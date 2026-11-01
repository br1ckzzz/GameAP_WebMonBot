package gameap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientFetchServers_Fallback(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Simulate first endpoint (/api/servers) failing with 500, second (/servers/) succeeding
		if r.URL.Path == "/test/api/servers" {
			http.Error(w, "plugin error", http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/test/servers/" || r.URL.Path == "/test/servers" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"success":true,"total_servers":1,"online_count":1,"servers":[{"id":1,"name":"Test","status":"online"}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	client := NewClient(ts.URL+"/test", "test_token", 5*time.Second)
	resp, err := client.FetchServers(context.Background())
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}

	if !resp.Success || len(resp.Servers) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestClientFetchServers_MetadataFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"success": true,
			"total_servers": 1,
			"online_count": 1,
			"servers": [
				{
					"id": 42,
					"name": "Mirage 24/7",
					"game_code": "cs2",
					"game_name": "Counter-Strike 2",
					"address": "192.168.1.50",
					"port": 27015,
					"status": "online",
					"category": "Main Cluster",
					"uptime_seconds": 86400
				}
			],
			"announcement": {
				"enabled": true,
				"text": "Server tournament on Saturday!",
				"type": "warning",
				"deadline": "2026-10-05T18:00:00Z"
			},
			"timestamp": 1727112000
		}`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "token", 2*time.Second)
	resp, err := client.FetchServers(context.Background())
	if err != nil {
		t.Fatalf("FetchServers failed: %v", err)
	}

	if len(resp.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(resp.Servers))
	}
	srv := resp.Servers[0]
	if srv.Category != "Main Cluster" {
		t.Errorf("expected category 'Main Cluster', got '%s'", srv.Category)
	}
	if srv.UptimeSeconds != 86400 {
		t.Errorf("expected uptime 86400, got %d", srv.UptimeSeconds)
	}
	if resp.Announcement == nil || !resp.Announcement.Enabled || resp.Announcement.Text != "Server tournament on Saturday!" {
		t.Errorf("announcement parsing failed: %+v", resp.Announcement)
	}
}

