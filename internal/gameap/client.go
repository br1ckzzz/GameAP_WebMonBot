package gameap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client interacts with the GameAP WebMonitoring Bot API.
type Client struct {
	baseURL         string
	apiToken        string
	httpClient      *http.Client
	stickyMu        sync.RWMutex
	stickyEndpoint  string
	stickyTelemetry string
}

// NewClient initializes a new GameAP API client.
func NewClient(baseURL, apiToken string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		apiToken: strings.TrimSpace(apiToken),
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				DisableCompression: true,
			},
		},
	}
}

// UpdateCredentials dynamically updates the target GameAP API endpoint and token.
func (c *Client) UpdateCredentials(baseURL, apiToken string) {
	c.stickyMu.Lock()
	defer c.stickyMu.Unlock()
	c.baseURL = strings.TrimRight(baseURL, "/")
	c.apiToken = strings.TrimSpace(apiToken)
	c.stickyEndpoint = ""
	c.stickyTelemetry = ""
}

// GetBaseURL returns the configured base URL.
func (c *Client) GetBaseURL() string {
	c.stickyMu.RLock()
	defer c.stickyMu.RUnlock()
	return c.baseURL
}

// FetchServers retrieves the current list of game servers from GameAP WebMonitoring.
// It uses sticky endpoint caching: once a working route is discovered, subsequent calls query
// only that endpoint to eliminate redundant HTTP retry storms and avoid UI delays.
func (c *Client) FetchServers(ctx context.Context) (*MonitoringResponse, error) {
	c.stickyMu.RLock()
	pinnedEndpoint := c.stickyEndpoint
	baseURL := c.baseURL
	c.stickyMu.RUnlock()

	// If we already discovered and pinned a working endpoint, use it directly
	if pinnedEndpoint != "" {
		resp, err := c.doFetch(ctx, pinnedEndpoint)
		if err == nil {
			return resp, nil
		}
		// If 404, route was changed / invalidated: clear pin and rediscover
		if strings.Contains(err.Error(), "HTTP 404") {
			log.Printf("[GameAP Client] Pinned endpoint %s returned 404, invalidating and rediscovering...", pinnedEndpoint)
			c.stickyMu.Lock()
			c.stickyEndpoint = ""
			c.stickyMu.Unlock()
		} else {
			// For 500, 401, 403, timeout or network drops, do NOT cycle through 12 candidate routes.
			// Fail fast so caller can immediately fall back to direct queries or cached state.
			return nil, err
		}
	}

	base := strings.TrimRight(baseURL, "/")
	var baseCandidates []string
	baseCandidates = append(baseCandidates, base)

	// GameAP Base32 ID normalization collision alias:
	// GameAP's CompactPluginID(ParsePluginID("monitoring")) normalizes "monitoring" to "monitorine".
	// If base URL contains "/monitoring", also generate candidate routes with "/monitorine".
	if strings.Contains(base, "/monitoring") {
		baseCandidates = append(baseCandidates, strings.Replace(base, "/monitoring", "/monitorine", 1))
	} else if strings.Contains(base, "/monitorine") {
		baseCandidates = append(baseCandidates, strings.Replace(base, "/monitorine", "/monitoring", 1))
	}

	var candidateEndpoints []string
	seen := make(map[string]bool)
	addCandidate := func(ep string) {
		if !seen[ep] && ep != "" {
			seen[ep] = true
			candidateEndpoints = append(candidateEndpoints, ep)
		}
	}

	for _, b := range baseCandidates {
		if strings.HasSuffix(b, "/api/servers") || strings.HasSuffix(b, "/bot/servers") {
			addCandidate(b + "?all=1")
			addCandidate(b)
			addCandidate(b + "/")
			addCandidate(strings.TrimSuffix(b, "/api/servers") + "/servers?all=1")
			addCandidate(strings.TrimSuffix(b, "/api/servers") + "/servers")
			addCandidate(strings.TrimSuffix(b, "/bot/servers") + "/servers?all=1")
			addCandidate(strings.TrimSuffix(b, "/bot/servers") + "/servers")
		} else if strings.HasSuffix(b, "/servers") {
			addCandidate(b + "?all=1")
			addCandidate(b)
			addCandidate(b + "/")
			addCandidate(strings.TrimSuffix(b, "/servers") + "/api/servers?all=1")
			addCandidate(strings.TrimSuffix(b, "/servers") + "/api/servers")
			addCandidate(strings.TrimSuffix(b, "/servers") + "?action=servers&all=1")
			addCandidate(strings.TrimSuffix(b, "/servers") + "?action=all_servers")
			addCandidate(strings.TrimSuffix(b, "/servers") + "?action=servers")
		} else {
			addCandidate(b + "?action=servers&all=1")
			addCandidate(b + "?action=all_servers")
			addCandidate(b + "?action=servers")
			addCandidate(b + "/servers?all=1")
			addCandidate(b + "/servers")
			addCandidate(b + "/servers/")
			addCandidate(b + "/api/servers?all=1")
			addCandidate(b + "/api/servers")
			addCandidate(b + "?all=1")
			addCandidate(b)
			addCandidate(b + "/")
		}
	}

	var lastErr error
	var authErr error
	for _, endpoint := range candidateEndpoints {
		resp, err := c.doFetch(ctx, endpoint)
		if err == nil {
			c.stickyMu.Lock()
			c.stickyEndpoint = endpoint
			c.stickyMu.Unlock()
			log.Printf("[GameAP Client] Discovered and pinned active endpoint: %s", endpoint)
			return resp, nil
		}
		// Remember explicit auth/permission error, but continue checking remaining candidate endpoints
		// (e.g. if an explicit /api/servers endpoint returned 403, generic /servers or /servers/ might succeed).
		if strings.Contains(err.Error(), "HTTP 401") || strings.Contains(err.Error(), "HTTP 403") {
			if authErr == nil {
				authErr = err
			}
		}
		lastErr = err
	}

	if authErr != nil {
		return nil, authErr
	}
	return nil, lastErr
}

func (c *Client) doFetch(ctx context.Context, endpoint string) (*MonitoringResponse, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request to %s: %w", endpoint, err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GameAP-WebMonBot/1.0")

	c.stickyMu.RLock()
	apiToken := c.apiToken
	c.stickyMu.RUnlock()

	if apiToken != "" {
		req.Header.Set("X-WebMon-Token", apiToken)
	}

	resp, err := c.httpClient.Do(req)
	latency := time.Since(start).Round(time.Millisecond)
	if err != nil {
		log.Printf("[GameAP HTTP] GET %s -> ERROR: %v (latency: %v)", endpoint, err, latency)
		return nil, fmt.Errorf("http get %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[GameAP HTTP] GET %s -> HTTP %d, error reading body: %v (latency: %v)", endpoint, resp.StatusCode, err, latency)
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	// Prepare single-line error snippet for log observability
	snippet := string(bodyBytes)
	if len(snippet) > 250 {
		snippet = snippet[:250] + "..."
	}
	snippet = strings.ReplaceAll(snippet, "\n", " ")
	snippet = strings.ReplaceAll(snippet, "\r", "")

	if resp.StatusCode == http.StatusOK {
		log.Printf("[GameAP HTTP] GET %s -> HTTP 200 OK (latency: %v, bytes: %d)", endpoint, latency, len(bodyBytes))
	} else {
		log.Printf("[GameAP HTTP] GET %s -> HTTP %d (latency: %v, body snippet: %q)", endpoint, resp.StatusCode, latency, snippet)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized (HTTP 401): invalid or missing GameAP API Token (%s)", snippet)
	}

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("forbidden (HTTP 403): Bot API is disabled in plugin settings (%s)", snippet)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status HTTP %d: %s", resp.StatusCode, snippet)
	}

	var monitoringResp MonitoringResponse
	if err := json.Unmarshal(bodyBytes, &monitoringResp); err != nil {
		// Fallback: sometimes endpoints might return a raw JSON array of servers
		var serversList []PublicServerDTO
		if arrErr := json.Unmarshal(bodyBytes, &serversList); arrErr == nil {
			online := 0
			for _, s := range serversList {
				if s.Status == "online" {
					online++
				}
			}
			return &MonitoringResponse{
				Success:      true,
				TotalServers: len(serversList),
				OnlineCount:  online,
				Servers:      serversList,
				Timestamp:    time.Now().Unix(),
			}, nil
		}
		return nil, fmt.Errorf("unmarshaling monitoring JSON: %w (body snippet: %s)", err, snippet)
	}

	return &monitoringResp, nil
}

// Ping measures latency to the GameAP API.
func (c *Client) Ping(ctx context.Context) (time.Duration, error) {
	start := time.Now()
	_, err := c.FetchServers(ctx)
	if err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

// ServerTelemetryDTO represents single server telemetry sent to WebMonitoring.
type ServerTelemetryDTO struct {
	Online     bool   `json:"online"`
	Map        string `json:"map,omitempty"`
	Players    int    `json:"players"`
	MaxPlayers int    `json:"max_players,omitempty"`
	Ping       int    `json:"ping,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// QueryTelemetryPayload wraps the servers dictionary sent to POST /servers/query-update.
type QueryTelemetryPayload struct {
	Servers map[string]ServerTelemetryDTO `json:"servers"`
}

// PushTelemetry sends live query telemetry to GameAP WebMonitoring plugin,
// keeping web UI in sync and preventing stuck online status during node crashes.
func (c *Client) PushTelemetry(ctx context.Context, servers []PublicServerDTO) error {
	c.stickyMu.RLock()
	baseURL := c.baseURL
	apiToken := c.apiToken
	c.stickyMu.RUnlock()

	if baseURL == "" || len(servers) == 0 {
		return nil
	}

	payload := QueryTelemetryPayload{
		Servers: make(map[string]ServerTelemetryDTO),
	}

	for _, s := range servers {
		if s.ID == 0 {
			continue
		}
		isOnline := (s.Status == "online")
		payload.Servers[fmt.Sprintf("%d", s.ID)] = ServerTelemetryDTO{
			Online:     isOnline,
			Map:        s.Map,
			Players:    s.Players,
			MaxPlayers: s.MaxPlayers,
			Ping:       s.Ping,
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	base := strings.TrimRight(baseURL, "/")
	var baseCandidates []string
	baseCandidates = append(baseCandidates, base)
	if strings.Contains(base, "/monitoring") {
		baseCandidates = append(baseCandidates, strings.Replace(base, "/monitoring", "/monitorine", 1))
	} else if strings.Contains(base, "/monitorine") {
		baseCandidates = append(baseCandidates, strings.Replace(base, "/monitorine", "/monitoring", 1))
	}

	var endpoints []string
	seen := make(map[string]bool)
	addEP := func(ep string) {
		if !seen[ep] && ep != "" {
			seen[ep] = true
			endpoints = append(endpoints, ep)
		}
	}

	for _, b := range baseCandidates {
		if strings.Contains(b, "?") {
			addEP(b + "&action=query-update")
			addEP(b + "/query-update")
		} else {
			addEP(b + "?action=query-update")
			addEP(strings.TrimSuffix(b, "/servers") + "?action=query-update")
			addEP(b + "/servers/query-update")
			addEP(b + "/query-update")
		}
	}

	for _, ep := range endpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep, strings.NewReader(string(data)))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "GameAP-WebMonBot/1.0")
		if apiToken != "" {
			req.Header.Set("X-WebMon-Token", apiToken)
		}
		resp, err := c.httpClient.Do(req)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
	return nil
}
