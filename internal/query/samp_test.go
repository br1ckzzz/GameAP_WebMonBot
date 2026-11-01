package query

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestQuerySAMP(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer pc.Close()

	addr := pc.LocalAddr().(*net.UDPAddr)

	go func() {
		buf := make([]byte, 1024)
		n, clientAddr, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		if n >= 11 && string(buf[:4]) == "SAMP" && buf[10] == 'i' {
			var resp bytes.Buffer
			resp.Write(buf[:11]) // echo header
			resp.WriteByte(0)    // password: false
			_ = binary.Write(&resp, binary.LittleEndian, uint16(42))  // players
			_ = binary.Write(&resp, binary.LittleEndian, uint16(100)) // max players

			name := []byte("My SA-MP RolePlay")
			_ = binary.Write(&resp, binary.LittleEndian, uint32(len(name)))
			resp.Write(name)

			gamemode := []byte("Roleplay v2")
			_ = binary.Write(&resp, binary.LittleEndian, uint32(len(gamemode)))
			resp.Write(gamemode)

			mapname := []byte("San Andreas")
			_ = binary.Write(&resp, binary.LittleEndian, uint32(len(mapname)))
			resp.Write(mapname)

			_, _ = pc.WriteTo(resp.Bytes(), clientAddr)
		}
	}()

	info, err := QuerySAMP(addr.IP.String(), addr.Port, 1*time.Second)
	if err != nil {
		t.Fatalf("QuerySAMP failed: %v", err)
	}
	if info.Name != "My SA-MP RolePlay" {
		t.Errorf("expected name 'My SA-MP RolePlay', got '%s'", info.Name)
	}
	if info.Players != 42 || info.MaxPlayers != 100 {
		t.Errorf("expected players 42/100, got %d/%d", info.Players, info.MaxPlayers)
	}
	if info.Map != "San Andreas" {
		t.Errorf("expected map 'San Andreas', got '%s'", info.Map)
	}
}
