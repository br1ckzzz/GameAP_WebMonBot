package query

import (
	"net"
	"testing"
	"time"
)

func TestQueryQuake(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer pc.Close()

	addr := pc.LocalAddr().(*net.UDPAddr)

	go func() {
		buf := make([]byte, 1024)
		_, clientAddr, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		resp := "\xFF\xFF\xFF\xFFstatusResponse\n\\hostname\\Quake Arena Server\\mapname\\q3dm6\\sv_maxclients\\16\\gamename\\baseq3\n15 50 \"PlayerOne\"\n20 40 \"PlayerTwo\"\n"
		_, _ = pc.WriteTo([]byte(resp), clientAddr)
	}()

	info, err := QueryQuake(addr.IP.String(), addr.Port, 1*time.Second)
	if err != nil {
		t.Fatalf("QueryQuake failed: %v", err)
	}
	if info.Name != "Quake Arena Server" {
		t.Errorf("expected name 'Quake Arena Server', got '%s'", info.Name)
	}
	if info.Map != "q3dm6" {
		t.Errorf("expected map 'q3dm6', got '%s'", info.Map)
	}
	if info.Players != 2 || info.MaxPlayers != 16 {
		t.Errorf("expected players 2/16, got %d/%d", info.Players, info.MaxPlayers)
	}
}

func TestQueryQuake_IPv6JoinHostPort(t *testing.T) {
	pc, err := net.ListenPacket("udp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 UDP loopback not available: %v", err)
	}
	defer pc.Close()

	go func() {
		buf := make([]byte, 1024)
		_, clientAddr, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		resp := "\xFF\xFF\xFF\xFFstatusResponse\n\\hostname\\Quake IPv6\\mapname\\q3dm1\\sv_maxclients\\8\\gamename\\baseq3\n"
		_, _ = pc.WriteTo([]byte(resp), clientAddr)
	}()

	port := pc.LocalAddr().(*net.UDPAddr).Port
	info, err := QueryQuake("::1", port, 1*time.Second)
	if err != nil {
		t.Fatalf("QueryQuake with IPv6 ::1 failed: %v", err)
	}
	if info.Name != "Quake IPv6" {
		t.Errorf("expected 'Quake IPv6', got %q", info.Name)
	}
}
