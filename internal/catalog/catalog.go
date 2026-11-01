package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

//go:embed games.json
var embeddedCatalogJSON []byte

// GameDef represents a single supported game server definition in the DiscordGSM standard.
type GameDef struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Aliases       []string `json:"aliases"`
	Protocol      string   `json:"protocol"`
	DefaultPort   int      `json:"default_port"`
	PortOffset    int      `json:"port_offset"`
	Banner        string   `json:"banner"`
	DirectConnect string   `json:"direct_connect"`
	SteamAppID    int      `json:"steam_appid,omitempty"`
}

// NodeInfo represents metadata of the hosting node.
type NodeInfo struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	TotalGames int    `json:"total_games"`
	Endpoint   string `json:"endpoint"`
}

// CatalogManifest defines the root schema for games.json.
type CatalogManifest struct {
	SchemaVersion string    `json:"schema_version"`
	UpdatedAt     string    `json:"updated_at"`
	Description   string    `json:"description"`
	NodeInfo      NodeInfo  `json:"node_info"`
	Games         []GameDef `json:"games"`
}

var (
	mu              sync.RWMutex
	currentManifest CatalogManifest
	gamesByID       map[string]*GameDef
	gamesByAlias    map[string]*GameDef
	alphaCleanRegex = regexp.MustCompile(`[^a-z0-9]`)
)

func init() {
	if err := loadCatalogBytes(embeddedCatalogJSON); err != nil {
		panic(fmt.Sprintf("failed to parse embedded games.json catalog: %v", err))
	}
}

// normalizeKey strips non-alphanumeric characters and converts to lowercase.
func normalizeKey(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	// If name contains parentheses or colons, consider prefix as well
	if idx := strings.Index(lower, "("); idx > 0 {
		lower = strings.TrimSpace(lower[:idx])
	}
	if idx := strings.Index(lower, ":"); idx > 0 {
		lower = strings.TrimSpace(lower[:idx])
	}
	return alphaCleanRegex.ReplaceAllString(lower, "")
}

func loadCatalogBytes(data []byte) error {
	var manifest CatalogManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return err
	}

	byID := make(map[string]*GameDef, len(manifest.Games))
	byAlias := make(map[string]*GameDef, len(manifest.Games)*4)

	for i := range manifest.Games {
		g := &manifest.Games[i]
		cleanID := strings.ToLower(strings.TrimSpace(g.ID))
		byID[cleanID] = g

		// Register normalized ID
		normID := normalizeKey(g.ID)
		if normID != "" {
			byAlias[normID] = g
		}

		// Register normalized Name
		normName := normalizeKey(g.Name)
		if normName != "" {
			byAlias[normName] = g
		}

		// Register aliases
		for _, alias := range g.Aliases {
			normAlias := normalizeKey(alias)
			if normAlias != "" {
				byAlias[normAlias] = g
			}
			byAlias[strings.ToLower(strings.TrimSpace(alias))] = g
		}
	}

	mu.Lock()
	currentManifest = manifest
	gamesByID = byID
	gamesByAlias = byAlias
	mu.Unlock()

	return nil
}

// TotalGames returns the count of registered games in the catalog.
func TotalGames() int {
	mu.RLock()
	defer mu.RUnlock()
	return len(currentManifest.Games)
}

// GetGame retrieves game definition by code, alias, or normalized name.
func GetGame(codeOrName string) *GameDef {
	mu.RLock()
	defer mu.RUnlock()

	clean := strings.ToLower(strings.TrimSpace(codeOrName))
	if g, ok := gamesByID[clean]; ok {
		return g
	}

	norm := normalizeKey(codeOrName)
	if g, ok := gamesByAlias[norm]; ok {
		return g
	}
	if g, ok := gamesByAlias[clean]; ok {
		return g
	}

	return nil
}

// ResolveProtocol determines query protocol for the given game.
func ResolveProtocol(codeOrName string) string {
	if g := GetGame(codeOrName); g != nil && g.Protocol != "" {
		return g.Protocol
	}

	norm := normalizeKey(codeOrName)
	switch norm {
	case "samp", "crmp", "gtasa", "sanandreas":
		return "samp"
	case "bedrock", "mcpe", "minecraftpe", "minecraftbe":
		return "minecraft_bedrock"
	case "minecraft", "mc", "paper", "spigot", "forge", "fabric":
		return "minecraft_java"
	case "fivem", "gtav", "gta5", "redm":
		return "fivem"
	case "quake1", "quake2", "quake3", "quake4", "quakelive", "q3", "q2", "cod", "cod2", "cod4", "rtcw", "wolfet", "mohaa", "sof2":
		return "quake"
	default:
		return "a2s"
	}
}

// GetQueryPort returns the port to query for a given game and base port.
func GetQueryPort(codeOrName string, basePort int) int {
	if g := GetGame(codeOrName); g != nil && g.PortOffset > 0 {
		return basePort + g.PortOffset
	}

	norm := normalizeKey(codeOrName)
	switch norm {
	case "valheim", "7d2d", "sevendaystodie", "sdtd", "asa", "arksurvivalascended",
		"theforest", "forest", "sonsoftheforest", "sonsforest", "sotf", "enshrouded",
		"abioticfactor", "abioticf", "vrising", "theisle", "theisleevrima":
		return basePort + 1
	default:
		return basePort
	}
}

// GetGameThumbnailURL returns official banner or icon for the game, or empty string if unknown.
func GetGameThumbnailURL(codeOrName string) string {
	if g := GetGame(codeOrName); g != nil && g.Banner != "" {
		return g.Banner
	}
	return ""
}

// SearchGames searches the catalog by substring match on ID, Name, or Aliases.
func SearchGames(query string, limit int) []GameDef {
	mu.RLock()
	defer mu.RUnlock()

	cleanQuery := strings.ToLower(strings.TrimSpace(query))
	normQuery := normalizeKey(query)

	if limit <= 0 {
		limit = 10
	}

	var results []GameDef
	for _, g := range currentManifest.Games {
		matched := false
		if cleanQuery == "" || cleanQuery == "*" {
			matched = true
		} else if strings.Contains(strings.ToLower(g.ID), cleanQuery) || strings.Contains(strings.ToLower(g.Name), cleanQuery) {
			matched = true
		} else if normQuery != "" && (strings.Contains(normalizeKey(g.ID), normQuery) || strings.Contains(normalizeKey(g.Name), normQuery)) {
			matched = true
		} else {
			for _, a := range g.Aliases {
				if strings.Contains(strings.ToLower(a), cleanQuery) || (normQuery != "" && strings.Contains(normalizeKey(a), normQuery)) {
					matched = true
					break
				}
			}
		}

		if matched {
			results = append(results, g)
			if len(results) >= limit {
				break
			}
		}
	}

	return results
}

// SyncFromNode downloads an updated catalog manifest from a remote node URL and applies it.
func SyncFromNode(nodeURL string, timeout time.Duration) error {
	if nodeURL == "" {
		return fmt.Errorf("node URL is empty")
	}

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", nodeURL, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", "GameAP-WebMonBot-NodeSync/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d from node", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	return loadCatalogBytes(body)
}
