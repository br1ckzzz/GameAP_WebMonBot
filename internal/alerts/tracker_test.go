package alerts

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	"gameap-webmon-bot/internal/config"
	"gameap-webmon-bot/internal/gameap"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

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

func TestTrackerConcurrentProcessUpdates(t *testing.T) {
	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("discordgo.New failed: %v", err)
	}
	session.Client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewBufferString(`{"id":"1"}`)),
			}, nil
		}),
	}

	cfg := config.DefaultConfig()
	cfg.Notifications.Enabled = true
	cfg.Notifications.AlertsChannelID = "ch_alerts_test"
	cfg.Notifications.AlertRoleID = "12345"
	cfg.Notifications.OfflineConfirmThreshold = 1
	cfg.Notifications.NotifyOnOffline = true
	cfg.Notifications.NotifyOnOnline = true
	cfg.Notifications.NotifyOnRestart = true
	cfg.Notifications.NotifyOnUpdate = true

	tracker := NewTracker(cfg, session)

	// Baseline update
	tracker.ProcessUpdates([]gameap.PublicServerDTO{
		{
			ID:       101,
			Name:     "Server-0",
			GameCode: "cs2",
			GameName: "Counter-Strike 2",
			Address:  "127.0.0.1",
			Port:     27015,
			Status:   "online",
			Version:  "1.0.0",
			BuildID:  "100",
			Blocked:  false,
		},
	})

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < 30; i++ {
				status := "online"
				if i%2 == 0 {
					status = "offline"
				}
				tracker.ProcessUpdates([]gameap.PublicServerDTO{
					{
						ID:       101,
						Name:     fmt.Sprintf("Server-%d-%d", workerID, i),
						GameCode: "cs2",
						GameName: "Counter-Strike 2",
						Address:  fmt.Sprintf("10.0.%d.%d", workerID, i%250),
						Port:     27015 + (i % 10),
						Status:   status,
						Version:  fmt.Sprintf("1.%d.%d", workerID, i),
						BuildID:  fmt.Sprintf("%d", 100+i),
						Blocked:  i%3 == 0,
					},
				})
			}
		}(g)
	}
	wg.Wait()
	// Allow spawned alert goroutines to finish reading their state snapshots
	time.Sleep(50 * time.Millisecond)
}
