package query

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestQueryFiveM(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dynamic.json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"hostname":"Los Santos RP FiveM","mapname":"San Andreas","clients":64,"sv_maxclients":128,"gametype":"Roleplay"}`))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	parts := strings.Split(strings.TrimPrefix(ts.URL, "http://"), ":")
	port, _ := strconv.Atoi(parts[1])

	info, err := QueryFiveM(parts[0], port, 1*time.Second)
	if err != nil {
		t.Fatalf("QueryFiveM failed: %v", err)
	}
	if info.Name != "Los Santos RP FiveM" {
		t.Errorf("expected name 'Los Santos RP FiveM', got '%s'", info.Name)
	}
	if info.Players != 64 || info.MaxPlayers != 128 {
		t.Errorf("expected players 64/128, got %d/%d", info.Players, info.MaxPlayers)
	}
	if info.Map != "San Andreas" {
		t.Errorf("expected map 'San Andreas', got '%s'", info.Map)
	}
}

func TestQueryFiveM_IPv6JoinHostPort(t *testing.T) {
	ln, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback not available: %v", err)
	}
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dynamic.json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"hostname":"Los Santos IPv6","mapname":"San Andreas","clients":12,"sv_maxclients":64,"gametype":"Roleplay"}`))
			return
		}
		http.NotFound(w, r)
	}))
	ts.Listener = ln
	ts.Start()
	defer ts.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	info, err := QueryFiveM("::1", port, 1*time.Second)
	if err != nil {
		t.Fatalf("QueryFiveM with IPv6 ::1 failed: %v", err)
	}
	if info.Name != "Los Santos IPv6" {
		t.Errorf("expected name 'Los Santos IPv6', got %q", info.Name)
	}
}
