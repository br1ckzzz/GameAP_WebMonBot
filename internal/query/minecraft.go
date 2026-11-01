package query

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// MinecraftStatusResponse represents the JSON payload from a Minecraft 1.7+ SLP response.
type MinecraftStatusResponse struct {
	Version struct {
		Name     string `json:"name"`
		Protocol int    `json:"protocol"`
	} `json:"version"`
	Players struct {
		Max    int `json:"max"`
		Online int `json:"online"`
	} `json:"players"`
	Description any `json:"description"` // can be string or object
}

func writeVarInt(w io.Writer, val int) error {
	for {
		b := byte(val & 0x7F)
		val >>= 7
		if val != 0 {
			b |= 0x80
		}
		if _, err := w.Write([]byte{b}); err != nil {
			return err
		}
		if val == 0 {
			break
		}
	}
	return nil
}

func readVarInt(r io.Reader) (int, error) {
	var num int
	var shift uint
	for {
		var buf [1]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return 0, err
		}
		b := buf[0]
		num |= int(b&0x7F) << shift
		if (b & 0x80) == 0 {
			break
		}
		shift += 7
		if shift >= 35 {
			return 0, errors.New("varint too wide")
		}
	}
	return num, nil
}

// QueryMinecraft queries a Minecraft server via the TCP Server List Ping protocol.
func QueryMinecraft(address string, port int, timeout time.Duration) (*ServerInfo, error) {
	if port <= 0 {
		port = 25565
	}
	target := fmt.Sprintf("%s:%d", address, port)

	var d net.Dialer
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := d.DialContext(ctx, "tcp", target)
	if err != nil {
		return nil, fmt.Errorf("connecting to minecraft %s: %w", target, err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	// 1. Handshake Packet (ID 0x00)
	var handshakeBuf bytes.Buffer
	_ = writeVarInt(&handshakeBuf, 0x00) // Packet ID 0x00
	_ = writeVarInt(&handshakeBuf, 47)   // Protocol version (47 works on 1.8 - 1.21+)

	// Hostname string (prefixed with VarInt length)
	hostBytes := []byte(address)
	_ = writeVarInt(&handshakeBuf, len(hostBytes))
	_, _ = handshakeBuf.Write(hostBytes)

	// Port (2 bytes, BigEndian)
	_ = binary.Write(&handshakeBuf, binary.BigEndian, uint16(port))

	// Next State (1 = Status)
	_ = writeVarInt(&handshakeBuf, 1)

	// Send Handshake packet (prefixed with packet length)
	var sendHandshake bytes.Buffer
	_ = writeVarInt(&sendHandshake, handshakeBuf.Len())
	_, _ = sendHandshake.Write(handshakeBuf.Bytes())
	if _, err := conn.Write(sendHandshake.Bytes()); err != nil {
		return nil, fmt.Errorf("sending handshake: %w", err)
	}

	// 2. Status Request Packet (Length 1, Packet ID 0x00)
	var sendStatus bytes.Buffer
	_ = writeVarInt(&sendStatus, 1)
	_ = writeVarInt(&sendStatus, 0x00)
	if _, err := conn.Write(sendStatus.Bytes()); err != nil {
		return nil, fmt.Errorf("sending status request: %w", err)
	}

	// 3. Read Response Packet
	packetLen, err := readVarInt(conn)
	if err != nil {
		return nil, fmt.Errorf("reading packet length: %w", err)
	}
	if packetLen <= 0 || packetLen > 65535 {
		return nil, fmt.Errorf("invalid packet length %d", packetLen)
	}

	packetID, err := readVarInt(conn)
	if err != nil {
		return nil, fmt.Errorf("reading packet id: %w", err)
	}
	if packetID != 0x00 {
		return nil, fmt.Errorf("unexpected packet id 0x%02x, expected 0x00", packetID)
	}

	jsonLen, err := readVarInt(conn)
	if err != nil {
		return nil, fmt.Errorf("reading json length: %w", err)
	}
	if jsonLen <= 0 || jsonLen > 65535 {
		return nil, fmt.Errorf("invalid json length %d", jsonLen)
	}

	jsonBytes := make([]byte, jsonLen)
	if _, err := io.ReadFull(conn, jsonBytes); err != nil {
		return nil, fmt.Errorf("reading json body: %w", err)
	}

	var statusResp MinecraftStatusResponse
	if err := json.Unmarshal(jsonBytes, &statusResp); err != nil {
		return nil, fmt.Errorf("unmarshaling status json: %w", err)
	}

	// Extract description MOTD string
	var motd string
	switch desc := statusResp.Description.(type) {
	case string:
		motd = desc
	case map[string]any:
		if txt, ok := desc["text"].(string); ok {
			motd = txt
		}
	}
	motd = strings.TrimSpace(motd)

	gameTitle := "Minecraft"
	if versionName := strings.TrimSpace(statusResp.Version.Name); versionName != "" {
		if strings.EqualFold(versionName, "minecraft") {
			gameTitle = "Minecraft"
		} else {
			gameTitle = fmt.Sprintf("Minecraft (%s)", versionName)
		}
	}

	return &ServerInfo{
		Name:       motd,
		Map:        "Overworld",
		Folder:     "minecraft",
		Game:       gameTitle,
		Players:    statusResp.Players.Online,
		MaxPlayers: statusResp.Players.Max,
		Bots:       0,
	}, nil
}
