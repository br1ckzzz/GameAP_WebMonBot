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

func TestCleanupStaleMessages_PreservesGameGroup(t *testing.T) {
	tmpDir := t.TempDir()
	st := LoadState(tmpDir + "/state.json")
	_ = st.SetMessageID("game_group_cs2_123456", "msg_group_1")
	_ = st.SetMessageID("game_stale_no_msg", "")

	b := &Bot{
		cfg:   &config.Config{},
		state: st,
	}

	b.cleanupStaleMessages(map[string]bool{}, "", true)

	if got := st.GetMessageID("game_group_cs2_123456"); got != "msg_group_1" {
		t.Errorf("expected game_group_ key to be preserved, got %q", got)
	}
	if _, exists := st.GetAllMessageIDs()["game_stale_no_msg"]; exists {
		t.Errorf("expected stale non-game_group_ key to be removed")
	}
}

func TestStateUnbindAll(t *testing.T) {
	tmpDir := t.TempDir()
	st := LoadState(tmpDir + "/state.json")
	_ = st.BindChannel("g1", "ch_gen")
	_ = st.BindGameChannel("g1", "cs2", "ch_cs2")
	_ = st.BindServerChannel("g1", 1, "ch_srv1")
	_ = st.SetForumChannel("ch_forum")
	_ = st.SetNodesChannelID("ch_nodes")
	_ = st.SetMessageID("game_group_cs2_ch_cs2", "msg_grp")

	if err := st.UnbindAll(); err != nil {
		t.Fatalf("UnbindAll failed: %v", err)
	}
	if st.GetChannelID() != "" || len(st.GetGameChannels()) != 0 || len(st.GetServerChannels()) != 0 || st.GetForumChannel() != "" || st.GetNodesChannelID() != "" || len(st.GetAllMessageIDs()) != 0 {
		t.Errorf("expected all channel bindings and message IDs to be cleared after UnbindAll")
	}
}
