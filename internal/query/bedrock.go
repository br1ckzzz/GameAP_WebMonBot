package query

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

var (
	// raknetMagic is the standard RakNet offline message ID sequence (16 bytes).
	raknetMagic = []byte{
		0x00, 0xFF, 0xFF, 0x00,
		0xFE, 0xFE, 0xFE, 0xFE,
		0xFD, 0xFD, 0xFD, 0xFD,
		0x12, 0x34, 0x56, 0x78,
	}
)

// QueryBedrock queries a Minecraft Bedrock Edition (MCPE) server via the UDP RakNet protocol.
func QueryBedrock(address string, port int, timeout time.Duration) (*ServerInfo, error) {
	if address == "" {
		return nil, errors.New("invalid address")
	}
	if port <= 0 {
		port = 19132 // Default Minecraft Bedrock port
	}

	target := net.JoinHostPort(address, strconv.Itoa(port))
	conn, err := net.DialTimeout("udp", target, timeout)
	if err != nil {
		return nil, fmt.Errorf("dialing bedrock %s: %w", target, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	// 1. Build Unconnected Ping packet (ID 0x01)
	// 1 byte: ID 0x01
	// 8 bytes: Client Timestamp (int64, BigEndian)
	// 16 bytes: RakNet Magic GUID
	// 8 bytes: Client GUID (int64)
	var pkt bytes.Buffer
	pkt.WriteByte(0x01)
	_ = binary.Write(&pkt, binary.BigEndian, time.Now().UnixNano()/int64(time.Millisecond))
	pkt.Write(raknetMagic)
	_ = binary.Write(&pkt, binary.BigEndian, uint64(0x0000000000000001))

	if _, err := conn.Write(pkt.Bytes()); err != nil {
		return nil, fmt.Errorf("writing bedrock ping: %w", err)
	}

	recvBuf := make([]byte, 2048)
	n, err := conn.Read(recvBuf)
	if err != nil {
		return nil, fmt.Errorf("reading bedrock pong: %w", err)
	}

	// Unconnected Pong packet:
	// 1 byte: ID 0x1C
	// 8 bytes: Client Timestamp
	// 8 bytes: Server GUID
	// 16 bytes: RakNet Magic GUID
	// 2 bytes: Payload length (BigEndian uint16)
	// payload bytes: Semi-colon separated strings
	if n < 35 {
		return nil, errors.New("bedrock response too short")
	}

	if recvBuf[0] != 0x1C {
		return nil, fmt.Errorf("unexpected packet id 0x%02X, expected 0x1C", recvBuf[0])
	}

	// Verify magic bytes at offset 17..33
	if !bytes.Equal(recvBuf[17:33], raknetMagic) {
		return nil, errors.New("invalid raknet magic in response")
	}

	var strLen uint16
	_ = binary.Read(bytes.NewReader(recvBuf[33:35]), binary.BigEndian, &strLen)

	data := recvBuf[35:n]
	if int(strLen) <= len(data) {
		data = data[:strLen]
	}

	payload := string(data)
	tokens := strings.Split(payload, ";")
	if len(tokens) < 6 {
		return nil, fmt.Errorf("insufficient bedrock payload tokens: %s", payload)
	}

	// Format:
	// [0] Edition: "MCPE"
	// [1] MOTD: Server Name
	// [2] Protocol version
	// [3] Version name: "1.20.50"
	// [4] Online players
	// [5] Max players
	// [6] Server Unique ID
	// [7] Sub-MOTD / Map name
	// [8] Game mode: "Survival"
	name := strings.TrimSpace(tokens[1])
	version := strings.TrimSpace(tokens[3])
	online, _ := strconv.Atoi(tokens[4])
	maxPlayers, _ := strconv.Atoi(tokens[5])

	mapName := "Bedrock World"
	if len(tokens) > 7 && strings.TrimSpace(tokens[7]) != "" {
		mapName = strings.TrimSpace(tokens[7])
	}

	gameTitle := "Minecraft Bedrock"
	if version != "" {
		gameTitle = fmt.Sprintf("Minecraft Bedrock %s", version)
	}

	return &ServerInfo{
		Name:       name,
		Map:        mapName,
		Folder:     "bedrock",
		Game:       gameTitle,
		Players:    online,
		MaxPlayers: maxPlayers,
		Bots:       0,
	}, nil
}
