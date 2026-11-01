package alerts

import (
	"testing"
	"time"

	"gameap-webmon-bot/internal/config"
	"gameap-webmon-bot/internal/gameap"
)

func TestTrackerLifecycle(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Notifications.Enabled = true
	cfg.Notifications.AlertsChannelID = "ch_alerts_test"
	cfg.Notifications.OfflineConfirmThreshold = 2

	tracker := NewTracker(cfg, nil)

	// Tick 1: baseline check (should not fire alerts, sets baseline)
	srvBaseline := []gameap.PublicServerDTO{
		{
			ID:       101,
			Name:     "Test Server",
			GameCode: "cs2",
			GameName: "Counter-Strike 2",
			Address:  "127.0.0.1",
			Port:     27015,
			Status:   "online",
			Version:  "1.0.0",
			Blocked:  false,
		},
	}
	tracker.ProcessUpdates(srvBaseline)

	state, ok := tracker.servers[101]
	if !ok {
		t.Fatalf("expected server 101 to be tracked")
	}
	if state.LastStatus != "online" || state.LastVersion != "1.0.0" || state.LastBlocked != false {
		t.Errorf("unexpected baseline state: %+v", state)
	}

	// Tick 2: Version updated to 1.1.0
	srvUpdated := []gameap.PublicServerDTO{
		{
			ID:       101,
			Name:     "Test Server",
			GameCode: "cs2",
			GameName: "Counter-Strike 2",
			Address:  "127.0.0.1",
			Port:     27015,
			Status:   "online",
			Version:  "1.1.0",
			Blocked:  false,
		},
	}
	tracker.ProcessUpdates(srvUpdated)
	if state.LastVersion != "1.1.0" {
		t.Errorf("expected LastVersion to be updated to 1.1.0, got: %s", state.LastVersion)
	}

	// Tick 3: Maintenance mode enabled
	srvMaintenance := []gameap.PublicServerDTO{
		{
			ID:       101,
			Name:     "Test Server",
			GameCode: "cs2",
			GameName: "Counter-Strike 2",
			Address:  "127.0.0.1",
			Port:     27015,
			Status:   "online",
			Version:  "1.1.0",
			Blocked:  true,
		},
	}
	tracker.ProcessUpdates(srvMaintenance)
	if !state.LastBlocked {
		t.Errorf("expected LastBlocked to be true")
	}

	// Tick 4: 1st offline fail (anti-flapping, should NOT send offline alert yet)
	srvOffline1 := []gameap.PublicServerDTO{
		{
			ID:       101,
			Name:     "Test Server",
			GameCode: "cs2",
			GameName: "Counter-Strike 2",
			Address:  "127.0.0.1",
			Port:     27015,
			Status:   "offline",
			Version:  "1.1.0",
			Blocked:  true,
		},
	}
	tracker.ProcessUpdates(srvOffline1)
	if state.OfflineFailCount != 1 || state.OfflineAlertSent {
		t.Errorf("expected failCount=1, alertSent=false, got: failCount=%d, alertSent=%v", state.OfflineFailCount, state.OfflineAlertSent)
	}

	// Tick 5: 2nd offline fail (reaches threshold=2, triggers offline alert)
	tracker.ProcessUpdates(srvOffline1)
	if state.OfflineFailCount != 2 || !state.OfflineAlertSent {
		t.Errorf("expected failCount=2, alertSent=true, got: failCount=%d, alertSent=%v", state.OfflineFailCount, state.OfflineAlertSent)
	}

	// Tick 6: Recovered back online
	tracker.ProcessUpdates(srvUpdated)
	if state.OfflineAlertSent || state.OfflineFailCount != 0 || state.LastStatus != "online" {
		t.Errorf("expected reset state after recovery, got: alertSent=%v, failCount=%d, status=%s", state.OfflineAlertSent, state.OfflineFailCount, state.LastStatus)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration string
		expected string
	}{
		{"45s", "45 сек."},
		{"1m15s", "1 мин. 15 сек."},
		{"5m0s", "5 мин. 0 сек."},
	}

	for _, tt := range tests {
		d, _ := parseDuration(tt.duration)
		got := formatDuration(d)
		if got != tt.expected {
			t.Errorf("formatDuration(%s) = %s; want %s", tt.duration, got, tt.expected)
		}
	}
}

func parseDuration(s string) (timeDuration time.Duration, err error) {
	return time.ParseDuration(s)
}
