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

// QuerySAMP queries a SA-MP or CR-MP server using the official UDP binary protocol.
func QuerySAMP(address string, port int, timeout time.Duration) (*ServerInfo, error) {
	if address == "" || port <= 0 {
		return nil, errors.New("invalid address or port")
	}

	target := net.JoinHostPort(address, strconv.Itoa(port))
	conn, err := net.DialTimeout("udp", target, timeout)
	if err != nil {
		return nil, fmt.Errorf("dialing samp %s: %w", target, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	// Resolve IPv4 address bytes
	udpAddr, err := net.ResolveUDPAddr("udp4", target)
	if err != nil {
		return nil, fmt.Errorf("resolving udp4 %s: %w", target, err)
	}
	ipBytes := udpAddr.IP.To4()
	if len(ipBytes) != 4 {
		// Fallback parse by dots
		parts := strings.Split(address, ".")
		if len(parts) == 4 {
			ipBytes = make([]byte, 4)
			for i := 0; i < 4; i++ {
				n, _ := strconv.Atoi(parts[i])
				ipBytes[i] = byte(n)
			}
		} else {
			ipBytes = []byte{127, 0, 0, 1}
		}
	}

	// 1. Build SAMP packet header
	// 4 bytes: "SAMP"
	// 4 bytes: IP address
	// 2 bytes: Port (LittleEndian)
	// 1 byte: Opcode ('i' = 0x69 for Information)
	var pkt bytes.Buffer
	pkt.WriteString("SAMP")
	pkt.Write(ipBytes)
	_ = binary.Write(&pkt, binary.LittleEndian, uint16(port))
	pkt.WriteByte('i')

	if _, err := conn.Write(pkt.Bytes()); err != nil {
		return nil, fmt.Errorf("writing samp packet: %w", err)
	}

	recvBuf := make([]byte, 2048)
	n, err := conn.Read(recvBuf)
	if err != nil {
		return nil, fmt.Errorf("reading samp response: %w", err)
	}

	if n < 11 {
		return nil, errors.New("samp response too short")
	}

	// Verify header
	if string(recvBuf[:4]) != "SAMP" || recvBuf[10] != 'i' {
		return nil, errors.New("invalid samp response header")
	}

	buf := bytes.NewReader(recvBuf[11:n])

	// 1 byte: password
	_, err = buf.ReadByte()
	if err != nil {
		return nil, err
	}

	// 2 bytes: players (LittleEndian)
	var players uint16
	if err := binary.Read(buf, binary.LittleEndian, &players); err != nil {
		return nil, err
	}

	// 2 bytes: max players (LittleEndian)
	var maxPlayers uint16
	if err := binary.Read(buf, binary.LittleEndian, &maxPlayers); err != nil {
		return nil, err
	}

	// 4 bytes: hostname length + hostname string
	var nameLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &nameLen); err != nil {
		return nil, err
	}
	if nameLen > 512 {
		nameLen = 512
	}
	nameBytes := make([]byte, nameLen)
	_, _ = buf.Read(nameBytes)
	name := strings.TrimSpace(string(nameBytes))

	// 4 bytes: gamemode length + gamemode string
	var modeLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &modeLen); err != nil {
		modeLen = 0
	}
	if modeLen > 256 {
		modeLen = 256
	}
	modeBytes := make([]byte, modeLen)
	_, _ = buf.Read(modeBytes)
	gamemode := strings.TrimSpace(string(modeBytes))

	// 4 bytes: mapname length + mapname string
	var mapLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &mapLen); err != nil {
		mapLen = 0
	}
	if mapLen > 256 {
		mapLen = 256
	}
	mapBytes := make([]byte, mapLen)
	_, _ = buf.Read(mapBytes)
	mapname := strings.TrimSpace(string(mapBytes))
	if mapname == "" {
		mapname = "San Andreas"
	}

	gameTitle := "San Andreas Multiplayer"
	if gamemode != "" {
		gameTitle = fmt.Sprintf("SA-MP (%s)", gamemode)
	}

	return &ServerInfo{
		Name:       name,
		Map:        mapname,
		Folder:     "samp",
		Game:       gameTitle,
		Players:    int(players),
		MaxPlayers: int(maxPlayers),
		Bots:       0,
	}, nil
}
