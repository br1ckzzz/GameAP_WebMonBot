package query

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// QueryQuake queries a Quake 2, Quake 3, RTCW, Call of Duty, or GameSpy 1 server.
func QueryQuake(address string, port int, timeout time.Duration) (*ServerInfo, error) {
	if address == "" || port <= 0 {
		return nil, errors.New("invalid address or port")
	}

	target := fmt.Sprintf("%s:%d", address, port)
	conn, err := net.DialTimeout("udp", target, timeout)
	if err != nil {
		return nil, fmt.Errorf("dialing quake %s: %w", target, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	// Request: \xFF\xFF\xFF\xFFgetstatus\n
	req := []byte("\xFF\xFF\xFF\xFFgetstatus\n")
	if _, err := conn.Write(req); err != nil {
		return nil, fmt.Errorf("writing quake getstatus: %w", err)
	}

	recvBuf := make([]byte, 4096)
	n, err := conn.Read(recvBuf)
	if err != nil {
		return nil, fmt.Errorf("reading quake response: %w", err)
	}

	data := recvBuf[:n]
	if len(data) < 15 {
		return nil, errors.New("quake response too short")
	}

	// Check header
	if !bytes.HasPrefix(data, []byte("\xFF\xFF\xFF\xFFstatusResponse\n")) &&
		!bytes.HasPrefix(data, []byte("\xFF\xFF\xFF\xFFprint\n")) {
		return nil, errors.New("invalid quake response header")
	}

	// Split lines
	rawStr := string(data)
	lines := strings.Split(rawStr, "\n")
	if len(lines) < 2 {
		return nil, errors.New("no status lines found in quake response")
	}

	// Line 1 contains \key\value\key\value...
	cvarLine := lines[1]
	cvarTokens := strings.Split(cvarLine, "\\")
	cvars := make(map[string]string)
	for i := 1; i+1 < len(cvarTokens); i += 2 {
		k := strings.ToLower(strings.TrimSpace(cvarTokens[i]))
		v := strings.TrimSpace(cvarTokens[i+1])
		cvars[k] = v
	}

	// Hostname resolution
	name := cvars["sv_hostname"]
	if name == "" {
		name = cvars["hostname"]
	}
	if name == "" {
		name = cvars["name"]
	}

	mapName := cvars["mapname"]
	if mapName == "" {
		mapName = cvars["map"]
	}

	gameName := cvars["gamename"]
	if gameName == "" {
		gameName = "Quake"
	}

	maxPlayers, _ := strconv.Atoi(cvars["sv_maxclients"])
	if maxPlayers == 0 {
		maxPlayers, _ = strconv.Atoi(cvars["maxclients"])
	}

	// Count player lines (lines after line 1 that are not empty)
	playerCount := 0
	for i := 2; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			playerCount++
		}
	}

	return &ServerInfo{
		Name:       cleanColorCodes(name),
		Map:        mapName,
		Folder:     gameName,
		Game:       gameName,
		Players:    playerCount,
		MaxPlayers: maxPlayers,
		Bots:       0,
	}, nil
}

// cleanColorCodes strips Quake-style color codes (^1, ^2, etc.) from strings.
func cleanColorCodes(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '^' && i+1 < len(runes) && runes[i+1] >= '0' && runes[i+1] <= '9' {
			i++ // skip color escape
			continue
		}
		b.WriteRune(runes[i])
	}
	return strings.TrimSpace(b.String())
}
