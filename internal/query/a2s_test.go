package query

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"gameap-webmon-bot/internal/gameap"
)

func TestParseA2SResponse_Source(t *testing.T) {
	// Construct a valid Source A2S_INFO packet
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF}) // Header
	buf.WriteByte(0x49)                        // Type: Source
	buf.WriteByte(0x11)                        // Protocol: 17
	buf.WriteString("Counter-Strike: Source Oscar-57\x00")
	buf.WriteString("de_dust2\x00")
	buf.WriteString("cstrike\x00")
	buf.WriteString("Counter-Strike: Source\x00")
	binary.Write(&buf, binary.LittleEndian, uint16(240)) // AppID
	buf.WriteByte(5)                                     // Players: 5
	buf.WriteByte(32)                                    // MaxPlayers: 32
	buf.WriteByte(0)                                     // Bots: 0
	buf.WriteByte('d')                                   // Dedicated
	buf.WriteByte('l')                                   // Linux
	buf.WriteByte(0)                                     // Visibility: Public
	buf.WriteByte(1)                                     // VAC: Secured

	info, err := ParseA2SResponse(buf.Bytes())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.Name != "Counter-Strike: Source Oscar-57" {
		t.Errorf("expected name 'Counter-Strike: Source Oscar-57', got '%s'", info.Name)
	}
	if info.Map != "de_dust2" {
		t.Errorf("expected map 'de_dust2', got '%s'", info.Map)
	}
	if info.Players != 5 {
		t.Errorf("expected players 5, got %d", info.Players)
	}
	if info.MaxPlayers != 32 {
		t.Errorf("expected max players 32, got %d", info.MaxPlayers)
	}
}

func TestParseA2SResponse_GoldSource(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF}) // Header
	buf.WriteByte(0x6D)                        // Type: GoldSource
	buf.WriteString("192.168.1.1:27015\x00")   // Address
	buf.WriteString("CS 1.6 Classic\x00")
	buf.WriteString("de_inferno\x00")
	buf.WriteString("cstrike\x00")
	buf.WriteString("Counter-Strike\x00")
	buf.WriteByte(12) // Players: 12
	buf.WriteByte(24) // MaxPlayers: 24

	info, err := ParseA2SResponse(buf.Bytes())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.Name != "CS 1.6 Classic" {
		t.Errorf("expected name 'CS 1.6 Classic', got '%s'", info.Name)
	}
	if info.Map != "de_inferno" {
		t.Errorf("expected map 'de_inferno', got '%s'", info.Map)
	}
	if info.Players != 12 {
		t.Errorf("expected players 12, got %d", info.Players)
	}
	if info.MaxPlayers != 24 {
		t.Errorf("expected max players 24, got %d", info.MaxPlayers)
	}
}

func TestParseA2SResponse_Errors(t *testing.T) {
	// Too short
	_, err := ParseA2SResponse([]byte{0xFF, 0xFF})
	if err == nil {
		t.Error("expected error for short packet, got nil")
	}

	// Invalid header
	_, err = ParseA2SResponse([]byte{0x00, 0x00, 0x00, 0x00, 0x49})
	if err == nil {
		t.Error("expected error for invalid header, got nil")
	}

	// Unsupported type
	_, err = ParseA2SResponse([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x99})
	if err == nil {
		t.Error("expected error for unsupported type, got nil")
	}
}

func TestEnrichServers_Fallback(t *testing.T) {
	servers := []gameap.PublicServerDTO{
		{
			ID:      1,
			Name:    "Offline Server",
			Address: "127.0.0.1",
			Port:    12345,
			Status:  "offline",
			Map:     "original_map",
		},
	}

	enriched := EnrichServers(servers, 100*time.Millisecond)
	if len(enriched) != 1 {
		t.Fatalf("expected 1 server, got %d", len(enriched))
	}
	if enriched[0].Map != "original_map" {
		t.Errorf("offline server map should not be changed, got '%s'", enriched[0].Map)
	}
}

func TestLiveQuery(t *testing.T) {
	info, err := QueryServer("46.138.244.41", 27315, 3*time.Second)
	if err != nil {
		t.Skipf("live server not reachable (network/firewall): %v", err)
	}
	if info.Map == "" {
		t.Errorf("expected map to not be empty")
	}
	t.Logf("Live query successful: Server='%s', Map='%s', Players=%d/%d", info.Name, info.Map, info.Players, info.MaxPlayers)
}

func TestGetQueryPort(t *testing.T) {
	tests := []struct {
		game     string
		port     int
		expected int
	}{
		{"valheim", 17800, 17801},
		{"Valheim", 2456, 2457},
		{"7d2d", 26900, 26901},
		{"cs2", 27015, 27015},
		{"minecraft", 25565, 25565},
		{"enshrouded", 15636, 15637},
	}

	for _, tt := range tests {
		got := GetQueryPort(tt.game, tt.port)
		if got != tt.expected {
			t.Errorf("GetQueryPort(%q, %d) = %d, expected %d", tt.game, tt.port, got, tt.expected)
		}
	}
}

func TestQueryServer_IPv6JoinHostPort(t *testing.T) {
	pc, err := net.ListenPacket("udp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 UDP loopback not available: %v", err)
	}
	defer pc.Close()

	go func() {
		buf := make([]byte, 2048)
		_, remote, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		var resp bytes.Buffer
		resp.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
		resp.WriteByte(0x49)
		resp.WriteByte(0x11)
		resp.WriteString("IPv6 A2S Server\x00")
		resp.WriteString("de_mirage\x00")
		resp.WriteString("csgo\x00")
		resp.WriteString("Counter-Strike 2\x00")
		_ = binary.Write(&resp, binary.LittleEndian, uint16(730))
		resp.WriteByte(3)
		resp.WriteByte(10)
		resp.WriteByte(0)
		resp.WriteByte('d')
		resp.WriteByte('l')
		resp.WriteByte(0)
		resp.WriteByte(1)
		_, _ = pc.WriteTo(resp.Bytes(), remote)
	}()

	port := pc.LocalAddr().(*net.UDPAddr).Port
	info, err := QueryServer("::1", port, 1*time.Second)
	if err != nil {
		t.Fatalf("QueryServer with IPv6 ::1 failed: %v", err)
	}
	if info.Name != "IPv6 A2S Server" {
		t.Errorf("expected 'IPv6 A2S Server', got %q", info.Name)
	}
}

