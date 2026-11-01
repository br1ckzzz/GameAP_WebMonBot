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
