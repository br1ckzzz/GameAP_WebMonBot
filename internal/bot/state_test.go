package bot

import (
	"errors"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestStateChannelCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state_test.json")

	s := LoadState(stateFile)
	s.ChannelID = "ch_general"
	_ = s.SetMessageID("summary", "msg_sum_1")
	_ = s.SetMessageID("game_valheim_ch_general", "msg_valheim_1")
	_ = s.SetMessageID("game_palworld_ch_dedicated", "msg_palworld_1")

	// Verify GetChannelMessageIDs
	generalMsgs := s.GetChannelMessageIDs("ch_general")
	if len(generalMsgs) != 2 {
		t.Fatalf("expected 2 messages for ch_general, got %d", len(generalMsgs))
	}
	if generalMsgs["summary"] != "msg_sum_1" || generalMsgs["game_valheim_ch_general"] != "msg_valheim_1" {
		t.Errorf("unexpected message mapping: %+v", generalMsgs)
	}

	// Remove messages for ch_general
	removed := s.RemoveMessagesForChannel("ch_general")
	if len(removed) != 2 {
		t.Fatalf("expected 2 messages removed, got %d", len(removed))
	}

	// Verify only dedicated channel remains
	remGeneral := s.GetChannelMessageIDs("ch_general")
	if len(remGeneral) != 0 {
		t.Fatalf("expected 0 messages remaining for ch_general, got %d", len(remGeneral))
	}

	if s.GetMessageID("game_palworld_ch_dedicated") != "msg_palworld_1" {
		t.Errorf("dedicated channel message should have been preserved")
	}

	// Verify reloading from disk preserves state
	s2 := LoadState(stateFile)
	if s2.GetMessageID("game_palworld_ch_dedicated") != "msg_palworld_1" {
		t.Errorf("reloaded state missing preserved message ID")
	}
}

func TestBindGameChannelCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state_test.json")

	s := LoadState(stateFile)
	_ = s.SetMessageID("game_valheim_ch1", "msg_1")
	_ = s.SetMessageID("game_valheim_ch2", "msg_2")
	_ = s.SetMessageID("game_cs2_ch1", "msg_cs2")

	// Rebind valheim to a new channel
	_ = s.BindGameChannel("guild1", "valheim", "ch_new")

	if s.GetMessageID("game_valheim_ch1") != "" || s.GetMessageID("game_valheim_ch2") != "" {
		t.Errorf("expected all valheim message IDs to be cleared on rebind")
	}

	if s.GetMessageID("game_cs2_ch1") != "msg_cs2" {
		t.Errorf("other game message ID was unexpectedly cleared")
	}
}

func TestIsMessageDeletedError(t *testing.T) {
	// Nil error
	if isMessageDeletedError(nil) {
		t.Errorf("nil error should not be considered deleted")
	}

	// Standard 404 RESTError
	err404 := &discordgo.RESTError{
		Response: &http.Response{StatusCode: http.StatusNotFound},
	}
	if !isMessageDeletedError(err404) {
		t.Errorf("404 RESTError should be considered deleted")
	}

	// Discord error code 10008 (Unknown Message)
	err10008 := &discordgo.RESTError{
		Response: &http.Response{StatusCode: http.StatusBadRequest},
		Message:  &discordgo.APIErrorMessage{Code: 10008, Message: "Unknown Message"},
	}
	if !isMessageDeletedError(err10008) {
		t.Errorf("10008 code should be considered deleted")
	}

	// Rate limit error (HTTP 429) -> NOT deleted!
	err429 := &discordgo.RESTError{
		Response: &http.Response{StatusCode: http.StatusTooManyRequests},
	}
	if isMessageDeletedError(err429) {
		t.Errorf("429 RateLimit should NOT be considered deleted")
	}

	// Transient network error -> NOT deleted!
	netErr := errors.New("dial tcp: lookup discord.com: i/o timeout")
	if isMessageDeletedError(netErr) {
		t.Errorf("network timeout should NOT be considered deleted")
	}

	// String containing unknown message
	strErr := errors.New("HTTP 404 Not Found: Unknown Message")
	if !isMessageDeletedError(strErr) {
		t.Errorf("error string with 'Unknown Message' should be considered deleted")
	}
}

func TestStateAlertsConfig(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state_alerts_test.json")

	s := LoadState(stateFile)

	// Fallback to defaults when not set
	en, ch, r := s.GetAlerts(true, "ch_def", "role_def")
	if !en || ch != "ch_def" || r != "role_def" {
		t.Fatalf("expected defaults (true, ch_def, role_def), got (%v, %s, %s)", en, ch, r)
	}

	// Update alerts
	tr := true
	if err := s.SetAlerts(&tr, "ch_alert_1", "role_admin_1"); err != nil {
		t.Fatalf("SetAlerts failed: %v", err)
	}

	en, ch, r = s.GetAlerts(false, "ch_def", "role_def")
	if !en || ch != "ch_alert_1" || r != "role_admin_1" {
		t.Fatalf("expected state overrides (true, ch_alert_1, role_admin_1), got (%v, %s, %s)", en, ch, r)
	}

	// Disable alerts and clear role
	fl := false
	if err := s.SetAlerts(&fl, "", "clear"); err != nil {
		t.Fatalf("SetAlerts failed: %v", err)
	}

	en, ch, r = s.GetAlerts(true, "ch_def", "role_def")
	if en || ch != "ch_alert_1" || r != "role_def" {
		t.Fatalf("expected state overrides (false, ch_alert_1, role_def), got (%v, %s, %s)", en, ch, r)
	}

	// Reload from disk
	s2 := LoadState(stateFile)
	en2, ch2, r2 := s2.GetAlerts(true, "ch_def", "role_def")
	if en2 != false || ch2 != "ch_alert_1" || r2 != "role_def" {
		t.Fatalf("reloaded state mismatch: (%v, %s, %s)", en2, ch2, r2)
	}
}

func TestStateForumConfig(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state_forum_test.json")

	s := LoadState(stateFile)
	if s.GetForumChannel() != "" {
		t.Fatalf("expected empty forum channel, got %s", s.GetForumChannel())
	}

	if err := s.SetForumChannel("ch_forum_1"); err != nil {
		t.Fatalf("SetForumChannel failed: %v", err)
	}
	if s.GetForumChannel() != "ch_forum_1" {
		t.Fatalf("expected ch_forum_1, got %s", s.GetForumChannel())
	}

	_ = s.SetForumThreadID("Counter-Strike 2", "th_cs2")
	_ = s.SetForumThreadID("valheim", "th_valheim")

	if s.GetForumThreadID("counter_strike_2") != "th_cs2" {
		t.Fatalf("expected th_cs2, got %s", s.GetForumThreadID("counter_strike_2"))
	}
	if s.GetForumThreadID("valheim") != "th_valheim" {
		t.Fatalf("expected th_valheim, got %s", s.GetForumThreadID("valheim"))
	}

	_ = s.RemoveForumThread("valheim")
	if s.GetForumThreadID("valheim") != "" {
		t.Fatalf("expected empty thread after remove, got %s", s.GetForumThreadID("valheim"))
	}

	// Reload from disk
	s2 := LoadState(stateFile)
	if s2.GetForumChannel() != "ch_forum_1" {
		t.Fatalf("reloaded forum channel mismatch: %s", s2.GetForumChannel())
	}
	if s2.GetForumThreadID("Counter-Strike 2") != "th_cs2" {
		t.Fatalf("reloaded thread mismatch: %s", s2.GetForumThreadID("Counter-Strike 2"))
	}
}

func TestBindServerChannel(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state_server_test.json")

	s := LoadState(stateFile)
	if s.GetServerChannel(42) != "" {
		t.Fatalf("expected empty server channel initially")
	}

	_ = s.SetMessageID("server_42_ch_old", "msg_old")
	_ = s.BindServerChannel("guild1", 42, "ch_server_42")

	if s.GetServerChannel(42) != "ch_server_42" {
		t.Fatalf("expected ch_server_42, got %s", s.GetServerChannel(42))
	}
	if s.GetMessageID("server_42_ch_old") != "" {
		t.Errorf("expected previous message ID for server 42 to be cleared")
	}

	all := s.GetServerChannels()
	if all[42] != "ch_server_42" {
		t.Fatalf("GetServerChannels missing 42")
	}

	// Reload from disk
	s2 := LoadState(stateFile)
	if s2.GetServerChannel(42) != "ch_server_42" {
		t.Fatalf("reloaded server channel mismatch: %s", s2.GetServerChannel(42))
	}

	// Unbind
	_ = s2.UnbindServerChannel(42)
	if s2.GetServerChannel(42) != "" {
		t.Fatalf("expected empty after unbind")
	}
}

func TestCustomServerState(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state_custom_srv_test.json")

	s := LoadState(stateFile)
	if len(s.GetCustomServers()) != 0 {
		t.Fatalf("expected 0 custom servers initially")
	}

	srv := CustomServer{
		Address:  "127.0.0.1",
		Port:     7777,
		GameCode: "samp",
		GameName: "San Andreas Multiplayer",
		Name:     "Test SAMP RP",
	}

	err := s.AddCustomServer(srv)
	if err != nil {
		t.Fatalf("AddCustomServer failed: %v", err)
	}

	servers := s.GetCustomServers()
	if len(servers) != 1 {
		t.Fatalf("expected 1 custom server, got %d", len(servers))
	}
	if servers[0].Address != "127.0.0.1" || servers[0].Port != 7777 || servers[0].ID == 0 {
		t.Errorf("unexpected server data: %+v", servers[0])
	}

	// Reload from disk
	s2 := LoadState(stateFile)
	reloaded := s2.GetCustomServers()
	if len(reloaded) != 1 {
		t.Fatalf("expected 1 custom server on reload, got %d", len(reloaded))
	}
	if reloaded[0].Name != "Test SAMP RP" {
		t.Errorf("expected name 'Test SAMP RP', got '%s'", reloaded[0].Name)
	}

	// Remove
	removed := s2.RemoveCustomServer("127.0.0.1:7777")
	if !removed {
		t.Fatalf("expected RemoveCustomServer to return true")
	}
	if len(s2.GetCustomServers()) != 0 {
		t.Fatalf("expected 0 custom servers after removal")
	}
}

func TestNodesStatePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state_nodes_test.json")

	s := LoadState(stateFile)
	if s.GetNodesChannelID() != "" {
		t.Fatalf("expected empty nodes channel ID, got %s", s.GetNodesChannelID())
	}
	if s.GetNodesMessageID() != "" {
		t.Fatalf("expected empty nodes message ID, got %s", s.GetNodesMessageID())
	}

	if err := s.SetNodesChannelID("ch_nodes_100"); err != nil {
		t.Fatalf("SetNodesChannelID failed: %v", err)
	}
	if err := s.SetNodesMessageID("msg_nodes_200"); err != nil {
		t.Fatalf("SetNodesMessageID failed: %v", err)
	}

	if s.GetNodesChannelID() != "ch_nodes_100" {
		t.Errorf("expected ch_nodes_100, got %s", s.GetNodesChannelID())
	}
	if s.GetNodesMessageID() != "msg_nodes_200" {
		t.Errorf("expected msg_nodes_200, got %s", s.GetNodesMessageID())
	}

	// Changing channel ID should reset message ID
	if err := s.SetNodesChannelID("ch_nodes_new"); err != nil {
		t.Fatalf("SetNodesChannelID failed: %v", err)
	}
	if s.GetNodesChannelID() != "ch_nodes_new" {
		t.Errorf("expected ch_nodes_new, got %s", s.GetNodesChannelID())
	}
	if s.GetNodesMessageID() != "" {
		t.Errorf("expected empty message ID after channel change, got %s", s.GetNodesMessageID())
	}

	// Re-set message ID and reload from disk
	_ = s.SetNodesMessageID("msg_nodes_300")
	s2 := LoadState(stateFile)
	if s2.GetNodesChannelID() != "ch_nodes_new" {
		t.Errorf("reloaded channel ID mismatch: %s", s2.GetNodesChannelID())
	}
	if s2.GetNodesMessageID() != "msg_nodes_300" {
		t.Errorf("reloaded message ID mismatch: %s", s2.GetNodesMessageID())
	}

	// Clear channel ID
	if err := s2.SetNodesChannelID("clear"); err != nil {
		t.Fatalf("SetNodesChannelID clear failed: %v", err)
	}
	if s2.GetNodesChannelID() != "" {
		t.Errorf("expected empty channel ID after clear, got %s", s2.GetNodesChannelID())
	}
	if s2.GetNodesMessageID() != "" {
		t.Errorf("expected empty message ID after clear, got %s", s2.GetNodesMessageID())
	}
}
