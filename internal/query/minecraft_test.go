package query

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func TestVarIntReadWrite(t *testing.T) {
	testValues := []int{0, 1, 127, 128, 255, 25565, 65535}
	for _, val := range testValues {
		var buf bytes.Buffer
		if err := writeVarInt(&buf, val); err != nil {
			t.Fatalf("writeVarInt(%d) failed: %v", val, err)
		}
		got, err := readVarInt(&buf)
		if err != nil {
			t.Fatalf("readVarInt failed for %d: %v", val, err)
		}
		if got != val {
			t.Errorf("readVarInt got %d, expected %d", got, val)
		}
	}
}

func TestQueryMinecraft_MockServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on local port: %v", err)
	}
	defer ln.Close()

	mockJSON := `{"version":{"name":"1.21.1","protocol":767},"players":{"max":50,"online":12},"description":"CyberZone Minecraft Server"}`

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read Handshake
		hsLen, _ := readVarInt(conn)
		hsData := make([]byte, hsLen)
		_, _ = conn.Read(hsData)

		// Read Status Request
		reqLen, _ := readVarInt(conn)
		reqData := make([]byte, reqLen)
		_, _ = conn.Read(reqData)

		// Send Response: PacketID 0x00 + JSON string
		var payload bytes.Buffer
		_ = writeVarInt(&payload, 0x00) // Packet ID 0x00
		_ = writeVarInt(&payload, len(mockJSON))
		payload.WriteString(mockJSON)

		var packet bytes.Buffer
		_ = writeVarInt(&packet, payload.Len())
		packet.Write(payload.Bytes())

		_, _ = conn.Write(packet.Bytes())
	}()

	addr := ln.Addr().(*net.TCPAddr)
	info, err := QueryMinecraft(addr.IP.String(), addr.Port, 2*time.Second)
	if err != nil {
		t.Fatalf("QueryMinecraft returned error: %v", err)
	}

	if info.Players != 12 {
		t.Errorf("expected 12 players, got %d", info.Players)
	}
	if info.MaxPlayers != 50 {
		t.Errorf("expected 50 max players, got %d", info.MaxPlayers)
	}
	if info.Game != "Minecraft (1.21.1)" {
		t.Errorf("expected game 'Minecraft (1.21.1)', got '%s'", info.Game)
	}
	if info.Name != "CyberZone Minecraft Server" {
		t.Errorf("expected name 'CyberZone Minecraft Server', got '%s'", info.Name)
	}
}

