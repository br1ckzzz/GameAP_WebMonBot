package query

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestQueryBedrock(t *testing.T) {
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
		if n >= 33 && buf[0] == 0x01 { // Unconnected Ping
			var resp bytes.Buffer
			resp.WriteByte(0x1C) // Unconnected Pong
			resp.Write(buf[1:9]) // echo timestamp
			_ = binary.Write(&resp, binary.BigEndian, uint64(123456789))
			resp.Write(raknetMagic)

			payload := "MCPE;Survival Bedrock Server;589;1.20.50;15;50;123456789;Bedrock World;Survival;1;19132;19133"
			_ = binary.Write(&resp, binary.BigEndian, uint16(len(payload)))
			resp.WriteString(payload)

			_, _ = pc.WriteTo(resp.Bytes(), clientAddr)
		}
	}()

	info, err := QueryBedrock(addr.IP.String(), addr.Port, 1*time.Second)
	if err != nil {
		t.Fatalf("QueryBedrock failed: %v", err)
	}
	if info.Name != "Survival Bedrock Server" {
		t.Errorf("expected name 'Survival Bedrock Server', got '%s'", info.Name)
	}
	if info.Players != 15 || info.MaxPlayers != 50 {
		t.Errorf("expected players 15/50, got %d/%d", info.Players, info.MaxPlayers)
	}
	if info.Map != "Bedrock World" {
		t.Errorf("expected map 'Bedrock World', got '%s'", info.Map)
	}
}
