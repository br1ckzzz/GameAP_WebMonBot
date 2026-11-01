package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// BotConfig holds Discord bot credentials and tuning options.
type BotConfig struct {
	Token                 string `json:"token"`
	ApplicationID         string `json:"application_id"`
	UpdateIntervalSeconds int    `json:"update_interval_seconds"`
	StatusPresence        string `json:"status_presence"`
	EmbedColorOnline      string `json:"embed_color_online"`
	EmbedColorOffline     string `json:"embed_color_offline"`
	DefaultThumbnailURL   string `json:"default_thumbnail_url,omitempty"`
	ProxyURL              string `json:"proxy_url,omitempty"`
}

// GameAPConfig holds GameAP WebMonitoring endpoint details and optional direct server query.
type GameAPConfig struct {
	BaseURL              string `json:"base_url"`
	APIToken             string `json:"api_token"`
	TimeoutSeconds       int    `json:"timeout_seconds"`
	DirectQueryEnabled   bool   `json:"direct_query_enabled"`
	DirectQueryTimeoutMS int    `json:"direct_query_timeout_ms"`
}

// DiscordConfig defines where dashboard messages should be published.
type DiscordConfig struct {
	GuildID                    string            `json:"guild_id"`
	DashboardChannelID         string            `json:"dashboard_channel_id"`
	NodesChannelID             string            `json:"nodes_channel_id,omitempty"`
	NodesUpdateIntervalSeconds int               `json:"nodes_update_interval_seconds,omitempty"`
	GameChannels               map[string]string `json:"game_channels,omitempty"`
	ServerChannels             map[string]string `json:"server_channels,omitempty"` // key: server ID or address:port, value: channel ID
	ForumChannelID             string            `json:"forum_channel_id"`
	UseForumThreads            bool              `json:"use_forum_threads"`
	GroupByGame                bool              `json:"group_by_game"`
	ManagerRoleIDs             []string          `json:"manager_role_ids,omitempty"`
}

// NodeConfig defines a dedicated host node / server for hardware and infrastructure monitoring.
type NodeConfig struct {
	ID         uint64   `json:"id"`
	Name       string   `json:"name"`
	Location   string   `json:"location,omitempty"`
	Provider   string   `json:"provider,omitempty"`
	CPUModel   string   `json:"cpu_model,omitempty"`
	CPUPercent int      `json:"cpu_percent,omitempty"`
	Protection string   `json:"protection,omitempty"`
	Host       string   `json:"host,omitempty"`
	Port       int      `json:"port,omitempty"`
	IPs        []string `json:"ips,omitempty"`
	Enabled    bool     `json:"enabled"`
}

// NotificationsConfig specifies event alerting options.
type NotificationsConfig struct {
	Enabled                 bool     `json:"enabled"`
	AlertsChannelID         string   `json:"alerts_channel_id"`
	AlertRoleID             string   `json:"alert_role_id,omitempty"`
	AlertRoleIDs            []string `json:"alert_role_ids,omitempty"`
	NotifyOnOffline         bool     `json:"notify_on_offline"`
	NotifyOnOnline          bool     `json:"notify_on_online"`
	NotifyOnRestart         bool     `json:"notify_on_restart"`
	NotifyOnUpdate          bool     `json:"notify_on_update"`
	OfflineConfirmThreshold int      `json:"offline_confirm_threshold"`
}

// GetResolvedAlertRoles returns a consolidated comma-separated string of role IDs.
func (n *NotificationsConfig) GetResolvedAlertRoles() string {
	if len(n.AlertRoleIDs) > 0 {
		return strings.Join(n.AlertRoleIDs, ",")
	}
	return n.AlertRoleID
}

// Config represents the complete application configuration.
type Config struct {
	Bot           BotConfig           `json:"bot"`
	GameAP        GameAPConfig        `json:"gameap_api"`
	Discord       DiscordConfig       `json:"discord"`
	Notifications NotificationsConfig `json:"notifications"`
	Nodes         []NodeConfig        `json:"nodes,omitempty"`
}

// DefaultConfig returns safe fallback defaults.
func DefaultConfig() *Config {
	return &Config{
		Bot: BotConfig{
			Token:                 "",
			ApplicationID:         "",
			UpdateIntervalSeconds: 30,
			StatusPresence:        "GameAP Monitoring",
			EmbedColorOnline:      "#2ecc71",
			EmbedColorOffline:     "#e74c3c",
			DefaultThumbnailURL:   "https://raw.githubusercontent.com/br1ckzzz/gameap_monitoring_plugin/main/icon.png?v=1.0.0",
		},
		GameAP: GameAPConfig{
			BaseURL:              "http://localhost:8080/api/plugins/monitoring",
			APIToken:             "",
			TimeoutSeconds:       10,
			DirectQueryEnabled:   true,
			DirectQueryTimeoutMS: 1200,
		},
		Discord: DiscordConfig{
			GuildID:                    "",
			DashboardChannelID:         "",
			NodesChannelID:             "",
			NodesUpdateIntervalSeconds: 300,
			GameChannels:               make(map[string]string),
			ForumChannelID:             "",
			UseForumThreads:            false,
			GroupByGame:                true,
		},
		Notifications: NotificationsConfig{
			Enabled:                 true,
			AlertsChannelID:         "",
			AlertRoleID:             "",
			NotifyOnOffline:         true,
			NotifyOnOnline:          true,
			NotifyOnRestart:         true,
			NotifyOnUpdate:          true,
			OfflineConfirmThreshold: 2,
		},
		Nodes: make([]NodeConfig, 0),
	}
}

// LoadConfigFromBytes parses configuration from raw JSON bytes with safe defaults and env overrides.
func LoadConfigFromBytes(data []byte) (*Config, error) {
	cfg := DefaultConfig()

	if len(data) > 0 {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config JSON: %w", err)
		}
	}

	applyEnvOverrides(cfg)
	sanitizeConfig(cfg)

	return cfg, nil
}

// Load reads and parses configuration from the given path, with fallback to environment variables.
func Load(configPath string) (*Config, error) {
	// If path not provided, check standard candidates
	if configPath == "" {
		if _, err := os.Stat("config.json"); err == nil {
			configPath = "config.json"
		} else if _, err := os.Stat("../config.json"); err == nil {
			configPath = "../config.json"
		} else if _, err := os.Stat("config.example.json"); err == nil {
			configPath = "config.example.json"
		} else if _, err := os.Stat("../config.example.json"); err == nil {
			configPath = "../config.example.json"
		}
	}

	var data []byte
	if configPath != "" {
		var err error
		data, err = os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("reading config file %s: %w", configPath, err)
		}
	}

	return LoadConfigFromBytes(data)
}

func applyEnvOverrides(cfg *Config) {
	if envToken := os.Getenv("DISCORD_BOT_TOKEN"); envToken != "" {
		cfg.Bot.Token = envToken
	}
	if envAppID := os.Getenv("DISCORD_APP_ID"); envAppID != "" {
		cfg.Bot.ApplicationID = envAppID
	}
	if envURL := os.Getenv("GAMEAP_API_URL"); envURL != "" {
		cfg.GameAP.BaseURL = envURL
	}
	if envAPIToken := os.Getenv("GAMEAP_API_TOKEN"); envAPIToken != "" {
		cfg.GameAP.APIToken = envAPIToken
	}
	if envGuild := os.Getenv("DISCORD_GUILD_ID"); envGuild != "" {
		cfg.Discord.GuildID = envGuild
	}
	if envChannel := os.Getenv("DISCORD_DASHBOARD_CHANNEL"); envChannel != "" {
		cfg.Discord.DashboardChannelID = envChannel
	}
	if envNodesChannel := os.Getenv("DISCORD_NODES_CHANNEL"); envNodesChannel != "" {
		cfg.Discord.NodesChannelID = envNodesChannel
	}
	if envInterval := os.Getenv("UPDATE_INTERVAL_SECONDS"); envInterval != "" {
		if v, err := strconv.Atoi(envInterval); err == nil && v > 0 {
			cfg.Bot.UpdateIntervalSeconds = v
		}
	}
	if envNodesInterval := os.Getenv("NODES_UPDATE_INTERVAL_SECONDS"); envNodesInterval != "" {
		if v, err := strconv.Atoi(envNodesInterval); err == nil && v > 0 {
			cfg.Discord.NodesUpdateIntervalSeconds = v
		}
	}
	if envProxy := os.Getenv("DISCORD_PROXY_URL"); envProxy != "" {
		cfg.Bot.ProxyURL = envProxy
	} else if envProxy := os.Getenv("ALL_PROXY"); envProxy != "" {
		cfg.Bot.ProxyURL = envProxy
	} else if envProxy := os.Getenv("HTTPS_PROXY"); envProxy != "" {
		cfg.Bot.ProxyURL = envProxy
	} else if envProxy := os.Getenv("HTTP_PROXY"); envProxy != "" {
		cfg.Bot.ProxyURL = envProxy
	}
}

func sanitizeConfig(cfg *Config) {
	cfg.Bot.ProxyURL = strings.TrimSpace(cfg.Bot.ProxyURL)
	cfg.GameAP.BaseURL = strings.TrimRight(cfg.GameAP.BaseURL, "/")
	if cfg.Bot.UpdateIntervalSeconds < 5 {
		cfg.Bot.UpdateIntervalSeconds = 5 // Prevent aggressive spam
	}
	if cfg.Discord.NodesUpdateIntervalSeconds <= 0 {
		cfg.Discord.NodesUpdateIntervalSeconds = 300 // Default 5 minutes
	} else if cfg.Discord.NodesUpdateIntervalSeconds < 10 {
		cfg.Discord.NodesUpdateIntervalSeconds = 10
	}
	if cfg.Notifications.OfflineConfirmThreshold < 1 {
		cfg.Notifications.OfflineConfirmThreshold = 1
	}
	if cfg.GameAP.DirectQueryTimeoutMS <= 0 {
		cfg.GameAP.DirectQueryTimeoutMS = 1200
	}
}
