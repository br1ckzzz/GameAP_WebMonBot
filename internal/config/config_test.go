package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Bot.UpdateIntervalSeconds != 30 {
		t.Errorf("expected default update interval 30, got %d", cfg.Bot.UpdateIntervalSeconds)
	}
	if !cfg.Notifications.Enabled {
		t.Errorf("expected notifications to be enabled by default")
	}
	if cfg.Notifications.OfflineConfirmThreshold != 2 {
		t.Errorf("expected default offline confirm threshold 2, got %d", cfg.Notifications.OfflineConfirmThreshold)
	}
}

func TestLoadWithEnvOverrides(t *testing.T) {
	_ = os.Setenv("DISCORD_BOT_TOKEN", "test_env_token_123")
	_ = os.Setenv("UPDATE_INTERVAL_SECONDS", "45")
	defer func() {
		_ = os.Unsetenv("DISCORD_BOT_TOKEN")
		_ = os.Unsetenv("UPDATE_INTERVAL_SECONDS")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.Bot.Token != "test_env_token_123" {
		t.Errorf("expected token from env, got %s", cfg.Bot.Token)
	}
	if cfg.Bot.UpdateIntervalSeconds != 45 {
		t.Errorf("expected interval from env (45), got %d", cfg.Bot.UpdateIntervalSeconds)
	}
}

func TestNodesConfigParsing(t *testing.T) {
	jsonCfg := []byte(`{
		"discord": {
			"nodes_channel_id": "999888777",
			"nodes_update_interval_seconds": 300
		},
		"nodes": [
			{
				"id": 1,
				"name": "[GE] Ryzen 9 7950X3D — Frankfurt",
				"location": "Germany, Frankfurt",
				"cpu_model": "AMD Ryzen 9 7950X3D",
				"cpu_percent": 42,
				"protection": "L4 Anti-DDoS Protection",
				"host": "185.189.255.10",
				"port": 31717,
				"enabled": true
			}
		]
	}`)
	cfg, err := LoadConfigFromBytes(jsonCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Discord.NodesChannelID != "999888777" {
		t.Errorf("expected nodes_channel_id 999888777, got %s", cfg.Discord.NodesChannelID)
	}
	if cfg.Discord.NodesUpdateIntervalSeconds != 300 {
		t.Errorf("expected 300 seconds, got %d", cfg.Discord.NodesUpdateIntervalSeconds)
	}
	if len(cfg.Nodes) != 1 || cfg.Nodes[0].CPUModel != "AMD Ryzen 9 7950X3D" {
		t.Errorf("expected 1 node with Ryzen 9, got %+v", cfg.Nodes)
	}
}
