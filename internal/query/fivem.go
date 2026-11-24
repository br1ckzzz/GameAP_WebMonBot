package query

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

type fivemDynamicResponse struct {
	Hostname     string `json:"hostname"`
	Mapname      string `json:"mapname"`
	Clients      int    `json:"clients"`
	SvMaxClients int    `json:"sv_maxclients"`
	Gametype     string `json:"gametype"`
}

type fivemInfoResponse struct {
	Vars struct {
		SvHostname string `json:"sv_hostname"`
		SvMaxClients string `json:"sv_maxclients"`
		Gametype string `json:"gametype"`
		Mapname string `json:"mapname"`
	} `json:"vars"`
}

// QueryFiveM queries a FiveM (GTA V) server via its HTTP JSON endpoints.
func QueryFiveM(address string, port int, timeout time.Duration) (*ServerInfo, error) {
	if address == "" || port <= 0 {
		return nil, errors.New("invalid address or port")
	}

	client := &http.Client{
		Timeout: timeout,
	}

	hostPort := net.JoinHostPort(address, strconv.Itoa(port))

	// 1. Try /dynamic.json first
	dynamicURL := fmt.Sprintf("http://%s/dynamic.json", hostPort)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, dynamicURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "GameAP-WebMonBot/1.0.1")
		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var dyn fivemDynamicResponse
				if err := json.NewDecoder(resp.Body).Decode(&dyn); err == nil && (dyn.Hostname != "" || dyn.SvMaxClients > 0) {
					mapName := dyn.Mapname
					if mapName == "" {
						mapName = "San Andreas"
					}
					gameTitle := "FiveM"
					if dyn.Gametype != "" {
						gameTitle = fmt.Sprintf("FiveM (%s)", dyn.Gametype)
					}
					return &ServerInfo{
						Name:       cleanColorCodes(dyn.Hostname),
						Map:        mapName,
						Folder:     "fivem",
						Game:       gameTitle,
						Players:    dyn.Clients,
						MaxPlayers: dyn.SvMaxClients,
						Bots:       0,
					}, nil
				}
			}
		}
	}

	// 2. Fallback to /info.json and /players.json
	infoURL := fmt.Sprintf("http://%s/info.json", hostPort)
	reqInfo, err := http.NewRequestWithContext(context.Background(), http.MethodGet, infoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request for %s: %w", infoURL, err)
	}
	reqInfo.Header.Set("User-Agent", "GameAP-WebMonBot/1.0.1")
	respInfo, err := client.Do(reqInfo)
	if err != nil {
		return nil, fmt.Errorf("connecting to fivem %s: %w", infoURL, err)
	}
	defer respInfo.Body.Close()

	if respInfo.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fivem info endpoint returned %d", respInfo.StatusCode)
	}

	var infoResp fivemInfoResponse
	if err := json.NewDecoder(respInfo.Body).Decode(&infoResp); err != nil {
		return nil, fmt.Errorf("decoding fivem info json: %w", err)
	}

	// Try getting players count from /players.json
	playersURL := fmt.Sprintf("http://%s/players.json", hostPort)
	reqPlayers, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, playersURL, nil)
	playerCount := 0
	if reqPlayers != nil {
		reqPlayers.Header.Set("User-Agent", "GameAP-WebMonBot/1.0.1")
		if respPlayers, err := client.Do(reqPlayers); err == nil {
			defer respPlayers.Body.Close()
			if respPlayers.StatusCode == http.StatusOK {
				var playerList []any
				if err := json.NewDecoder(respPlayers.Body).Decode(&playerList); err == nil {
					playerCount = len(playerList)
				}
			}
		}
	}

	mapName := infoResp.Vars.Mapname
	if mapName == "" {
		mapName = "San Andreas"
	}
	hostname := infoResp.Vars.SvHostname
	if hostname == "" {
		hostname = fmt.Sprintf("FiveM Server (%s)", hostPort)
	}

	return &ServerInfo{
		Name:       cleanColorCodes(hostname),
		Map:        mapName,
		Folder:     "fivem",
		Game:       "FiveM",
		Players:    playerCount,
		MaxPlayers: 64,
		Bots:       0,
	}, nil
}
