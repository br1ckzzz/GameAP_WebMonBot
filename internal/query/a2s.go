package query

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"gameap-webmon-bot/internal/catalog"
	"gameap-webmon-bot/internal/gameap"
)

var (
	a2sHeader      = []byte{0xFF, 0xFF, 0xFF, 0xFF}
	a2sInfoRequest = []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x54, 'S', 'o', 'u', 'r', 'c', 'e', ' ', 'E', 'n', 'g', 'i', 'n', 'e', ' ', 'Q', 'u', 'e', 'r', 'y', 0x00}
)

// ServerInfo holds parsed results from a game server query probe.
type ServerInfo struct {
	Name       string
	Map        string
	Folder     string
	Game       string
	Players    int
	MaxPlayers int
	Bots       int
	Ping       int
}

func readNullString(buf *bytes.Buffer) (string, error) {
	str, err := buf.ReadString(0x00)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(str, "\x00"), nil
}

// ParseA2SResponse parses a raw A2S_INFO response buffer into ServerInfo.
func ParseA2SResponse(data []byte) (*ServerInfo, error) {
	if len(data) < 5 {
		return nil, errors.New("packet too short")
	}

	if !bytes.Equal(data[:4], a2sHeader) {
		return nil, errors.New("invalid header")
	}

	headerType := data[4]
	buf := bytes.NewBuffer(data[5:])

	switch headerType {
	case 0x49: // Source Engine (CS:S, CS2, TF2, Rust, etc.)
		// Protocol byte
		if _, err := buf.ReadByte(); err != nil {
			return nil, err
		}
		name, err := readNullString(buf)
		if err != nil {
			return nil, err
		}
		mapName, err := readNullString(buf)
		if err != nil {
			return nil, err
		}
		folder, err := readNullString(buf)
		if err != nil {
			return nil, err
		}
		game, err := readNullString(buf)
		if err != nil {
			return nil, err
		}

		// AppID (2 bytes)
		var appID uint16
		if err := binary.Read(buf, binary.LittleEndian, &appID); err != nil {
			return nil, err
		}

		players, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}
		maxPlayers, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}
		bots, _ := buf.ReadByte()

		return &ServerInfo{
			Name:       name,
			Map:        mapName,
			Folder:     folder,
			Game:       game,
			Players:    int(players),
			MaxPlayers: int(maxPlayers),
			Bots:       int(bots),
		}, nil

	case 0x6D: // GoldSource (CS 1.6, Half-Life 1)
		// Address string (IP:port)
		if _, err := readNullString(buf); err != nil {
			return nil, err
		}
		name, err := readNullString(buf)
		if err != nil {
			return nil, err
		}
		mapName, err := readNullString(buf)
		if err != nil {
			return nil, err
		}
		folder, err := readNullString(buf)
		if err != nil {
			return nil, err
		}
		game, err := readNullString(buf)
		if err != nil {
			return nil, err
		}

		players, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}
		maxPlayers, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}

		return &ServerInfo{
			Name:       name,
			Map:        mapName,
			Folder:     folder,
			Game:       game,
			Players:    int(players),
			MaxPlayers: int(maxPlayers),
			Bots:       0,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported response type 0x%02X", headerType)
	}
}

// QueryServer sends a UDP A2S_INFO query to target address and port with challenge support.
func QueryServer(address string, port int, timeout time.Duration) (*ServerInfo, error) {
	if address == "" || port <= 0 {
		return nil, errors.New("invalid address or port")
	}

	target := net.JoinHostPort(address, strconv.Itoa(port))
	conn, err := net.DialTimeout("udp", target, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	// 1. Send initial A2S_INFO query
	rttStart := time.Now()
	if _, err := conn.Write(a2sInfoRequest); err != nil {
		return nil, err
	}

	recvBuf := make([]byte, 2048)
	n, err := conn.Read(recvBuf)
	if err != nil {
		return nil, err
	}

	data := recvBuf[:n]

	// 2. Check for challenge response (0x41)
	if len(data) >= 9 && bytes.Equal(data[:4], a2sHeader) && data[4] == 0x41 {
		challenge := data[5:9]
		reqWithChallenge := append(append([]byte{}, a2sInfoRequest...), challenge...)

		rttStart = time.Now()
		if _, err := conn.Write(reqWithChallenge); err != nil {
			return nil, err
		}

		n, err = conn.Read(recvBuf)
		if err != nil {
			return nil, err
		}
		data = recvBuf[:n]
	}

	rttMs := int(time.Since(rttStart).Milliseconds())
	if rttMs <= 0 {
		rttMs = 1
	}

	info, err := ParseA2SResponse(data)
	if err != nil {
		return nil, err
	}
	info.Ping = rttMs
	return info, nil
}

// GetQueryPort returns the recommended query port for a given game and base game port.
func GetQueryPort(gameCode string, port int) int {
	return catalog.GetQueryPort(gameCode, port)
}

// EnrichServers probes online servers in parallel with A2S_INFO queries, updating Map, Players, and Ping.
func EnrichServers(servers []gameap.PublicServerDTO, timeout time.Duration) []gameap.PublicServerDTO {
	if len(servers) == 0 {
		return servers
	}

	result := make([]gameap.PublicServerDTO, len(servers))
	copy(result, servers)

	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)

	for i := range result {
		// Only probe online servers with valid IP and port
		if result[i].Status != "online" || result[i].Address == "" || result[i].Port <= 0 {
			continue
		}

		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			srv := &result[idx]
			info, err := QueryServerAuto(srv.GameCode, srv.Address, srv.Port, timeout)

			if err == nil && info != nil {
				if info.Map != "" {
					srv.Map = info.Map
				}
				if info.MaxPlayers > 0 {
					srv.MaxPlayers = info.MaxPlayers
					srv.Players = info.Players
				}
				if info.Ping > 0 {
					srv.Ping = info.Ping
				}
			}
		}(i)
	}

	wg.Wait()
	return result
}
