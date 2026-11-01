package bot

import (
	"context"
	"testing"

	"gameap-webmon-bot/internal/config"
)

func TestGetNodesChannel(t *testing.T) {
	cfg := &config.Config{
		Discord: config.DiscordConfig{
			NodesChannelID: "config_nodes_chan",
		},
	}
	st := &State{}

	b := &Bot{
		cfg:   cfg,
		state: st,
	}

	// 1. Fallback to config when state is empty
	if got := b.getNodesChannel(); got != "config_nodes_chan" {
		t.Errorf("expected config_nodes_chan, got %q", got)
	}

	// 2. Prioritize dynamic state over config
	st.SetNodesChannelID("state_nodes_chan")
	if got := b.getNodesChannel(); got != "state_nodes_chan" {
		t.Errorf("expected state_nodes_chan, got %q", got)
	}

	// 3. Unbound state returns empty string
	st.SetNodesChannelID("")
	cfg.Discord.NodesChannelID = ""
	if got := b.getNodesChannel(); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestUpdateNodesStatus_NoChannel(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{}
	st := LoadState(tmpDir + "/state.json")

	b := &Bot{
		cfg:   cfg,
		state: st,
	}

	err := b.UpdateNodesStatus(context.Background())
	if err != nil {
		t.Fatalf("expected nil error when channel not configured, got: %v", err)
	}
}
