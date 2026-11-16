package bot

import (
	"context"
	"net"
	"testing"
	"time"

	"gameap-webmon-bot/internal/config"
	"gameap-webmon-bot/internal/gameap"
)

func TestDetectCountryAndFlag(t *testing.T) {
	tests := []struct {
		name         string
		nodeName     string
		location     string
		expectedCode string
		expectedFlag string
	}{
		{
			name:         "tag [GE] in name",
			nodeName:     "[GE] Ryzen 9 7950X3D — Frankfurt",
			location:     "",
			expectedCode: "DE",
			expectedFlag: "🇩🇪",
		},
		{
			name:         "tag [DE] in name",
			nodeName:     "[DE] Dedicated Server #1",
			location:     "",
			expectedCode: "DE",
			expectedFlag: "🇩🇪",
		},
		{
			name:         "tag [RU] in name",
			nodeName:     "Core i9-14900K [RU] Moscow",
			location:     "",
			expectedCode: "RU",
			expectedFlag: "🇷🇺",
		},
		{
			name:         "tag [KZ] in name",
			nodeName:     "[KZ] Xeon Gold - Almaty",
			location:     "",
			expectedCode: "KZ",
			expectedFlag: "🇰🇿",
		},
		{
			name:         "tag [FI] in name",
			nodeName:     "[FI] Node 03",
			location:     "",
			expectedCode: "FI",
			expectedFlag: "🇫🇮",
		},
		{
			name:         "tag [NL] in name",
			nodeName:     "[NL] Amsterdam",
			location:     "",
			expectedCode: "NL",
			expectedFlag: "🇳🇱",
		},
		{
			name:         "tag [US] in name",
			nodeName:     "[US] Dallas Central",
			location:     "",
			expectedCode: "US",
			expectedFlag: "🇺🇸",
		},
		{
			name:         "fallback location Germany, Frankfurt",
			nodeName:     "Server Frankfurt Node",
			location:     "Germany, Frankfurt",
			expectedCode: "DE",
			expectedFlag: "🇩🇪",
		},
		{
			name:         "fallback location Russia",
			nodeName:     "Moscow Node",
			location:     "Russia, DataLine",
			expectedCode: "RU",
			expectedFlag: "🇷🇺",
		},
		{
			name:         "fallback location Finland",
			nodeName:     "Helsinki Cloud",
			location:     "Finland, Helsinki",
			expectedCode: "FI",
			expectedFlag: "🇫🇮",
		},
		{
			name:         "no match fallback globe",
			nodeName:     "Internal Cluster #1",
			location:     "Private Rack",
			expectedCode: "GLOBAL",
			expectedFlag: "🌐",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, flag := DetectCountryAndFlag(tc.nodeName, tc.location)
			if code != tc.expectedCode {
				t.Errorf("expected code %s, got %s", tc.expectedCode, code)
			}
			if flag != tc.expectedFlag {
				t.Errorf("expected flag %s, got %s", tc.expectedFlag, flag)
			}
		})
	}
}

func TestFormatOS(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"windows", "🪟 Windows"},
		{"Windows Server 2022", "🪟 Windows"},
		{"linux", "🐧 Linux"},
		{"Ubuntu 22.04", "🐧 Linux"},
		{"FreeBSD", "🖥️ FreeBSD"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := FormatOS(tc.input)
			if got != tc.expected {
				t.Errorf("FormatOS(%q) = %q; expected %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestProbeNodeLatency(t *testing.T) {
	// 1. Successful local listener probe
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock listener: %v", err)
	}
	defer ln.Close()

	tcpAddr := ln.Addr().(*net.TCPAddr)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			conn.Close()
		}
	}()

	ctx := context.Background()
	latency := ProbeNodeLatency(ctx, "127.0.0.1", tcpAddr.Port, 1*time.Second, "ru")
	if latency == "" || latency == "Защищён (DDoS Filter)" {
		t.Errorf("expected valid latency for local server, got %q", latency)
	}

	// 2. Unreachable / filtered host returns "Защищён (DDoS Filter)" in Russian
	latencyTimeoutRU := ProbeNodeLatency(ctx, "nonexistent.node.invalid", 59999, 50*time.Millisecond, "ru")
	if latencyTimeoutRU != "Защищён (DDoS Filter)" {
		t.Errorf("expected DDoS Filter fallback on closed host (RU), got %q", latencyTimeoutRU)
	}

	// 3. Unreachable / filtered host returns "Protected (DDoS Filter)" in English
	latencyTimeoutEN := ProbeNodeLatency(ctx, "nonexistent.node.invalid", 59999, 50*time.Millisecond, "en")
	if latencyTimeoutEN != "Protected (DDoS Filter)" {
		t.Errorf("expected DDoS Filter fallback on closed host (EN), got %q", latencyTimeoutEN)
	}

	// 4. Empty host fallback
	emptyHostLatency := ProbeNodeLatency(ctx, "", 31717, 50*time.Millisecond, "ru")
	if emptyHostLatency != "Защищён (DDoS Filter)" {
		t.Errorf("expected DDoS Filter fallback on empty host, got %q", emptyHostLatency)
	}
}

func TestEnrichNodeStatuses(t *testing.T) {
	gameapNodes := []gameap.NodeDTO{
		{
			ID:          1,
			Name:        "[GE] Ryzen 9 7950X3D — Frankfurt",
			Location:    "Germany, Frankfurt",
			Provider:    "Datacenter AG",
			OS:          "linux",
			RAMPercent:  55,
			IPs:         []string{"185.189.255.10"},
			GDaemonHost: "185.189.255.10",
			GDaemonPort: 31717,
			CPUModel:    "AMD Ryzen 9 7950X3D",
			CPUPercent:  42,
			Protection:  "L4 Anti-DDoS Protection",
			Enabled:     true,
		},
	}

	servers := []gameap.PublicServerDTO{
		{
			ID:      101,
			Name:    "CS2 Public #1",
			Address: "185.189.255.10",
			Port:    27015,
			Status:  "online",
		},
		{
			ID:      102,
			Name:    "CS2 Retake #1",
			Address: "185.189.255.10",
			Port:    27016,
			Status:  "online",
		},
	}

	customNodes := []config.NodeConfig{
		{
			Name:         "AMD Ryzen 7 7845HS Server",
			Location:     "[RU] Moscow",
			Provider:     "MTS",
			OS:           "windows",
			Host:         "127.0.0.1",
			Port:         31717,
			CPUModel:     "AMD Ryzen 7 7845HS",
			CPUPercent:   14,
			RAMPercent:   29,
			Protection:   "L4 Anti-DDoS Protection",
			ServersCount: 5,
		},
	}

	// Case 1: GameAP provided nodes
	ctx := context.Background()
	statuses := EnrichNodeStatuses(ctx, gameapNodes, servers, nil, "ru")
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	s := statuses[0]
	if s.Flag != "🇩🇪" || s.CountryCode != "DE" {
		t.Errorf("expected DE/🇩🇪, got %s/%s", s.CountryCode, s.Flag)
	}
	if s.ServersCount != 2 {
		t.Errorf("expected 2 servers on node, got %d", s.ServersCount)
	}
	if !s.IsOnline {
		t.Errorf("expected node to be online")
	}
	if s.OS != "🐧 Linux" {
		t.Errorf("expected formatted OS '🐧 Linux', got %q", s.OS)
	}
	if s.RAMPercent != 55 {
		t.Errorf("expected RAMPercent 55, got %d", s.RAMPercent)
	}

	// Case 2: GameAP has no nodes, fallback to customNodes
	statusesCustom := EnrichNodeStatuses(ctx, nil, nil, customNodes, "ru")
	if len(statusesCustom) != 1 {
		t.Fatalf("expected 1 custom status, got %d", len(statusesCustom))
	}
	sc := statusesCustom[0]
	if sc.Flag != "🇷🇺" || sc.CountryCode != "RU" {
		t.Errorf("expected RU/🇷🇺, got %s/%s", sc.CountryCode, sc.Flag)
	}
	if sc.ServersCount != 5 {
		t.Errorf("expected 5 servers for custom node, got %d", sc.ServersCount)
	}
	if sc.OS != "🪟 Windows" {
		t.Errorf("expected formatted OS '🪟 Windows', got %q", sc.OS)
	}
	if sc.RAMPercent != 29 {
		t.Errorf("expected RAMPercent 29, got %d", sc.RAMPercent)
	}
	if sc.Provider != "MTS" {
		t.Errorf("expected Provider 'MTS', got %q", sc.Provider)
	}

	// Case 3: Both nodes and customNodes are nil, fallback to server addresses
	statusesServers := EnrichNodeStatuses(ctx, nil, servers, nil, "ru")
	if len(statusesServers) != 1 {
		t.Fatalf("expected 1 aggregated server status, got %d", len(statusesServers))
	}
	ss := statusesServers[0]
	if ss.ServersCount != 2 {
		t.Errorf("expected 2 servers on aggregated host, got %d", ss.ServersCount)
	}
	if ss.Host != "185.189.255.10" {
		t.Errorf("expected host 185.189.255.10, got %s", ss.Host)
	}
}

func TestEnrichNodeStatuses_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	nodes := []gameap.NodeDTO{
		{ID: 1, Name: "Node 1", GDaemonHost: "192.0.2.1", GDaemonPort: 31717, Enabled: true},
		{ID: 2, Name: "Node 2", GDaemonHost: "192.0.2.2", GDaemonPort: 31717, Enabled: true},
	}

	start := time.Now()
	statuses := EnrichNodeStatuses(ctx, nodes, nil, nil, "en")
	elapsed := time.Since(start)

	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	for i, s := range statuses {
		if s.Latency != "Protected (DDoS Filter)" {
			t.Errorf("node %d: expected 'Protected (DDoS Filter)' on cancelled ctx, got %q", i, s.Latency)
		}
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("expected cancelled EnrichNodeStatuses to return quickly, took %v", elapsed)
	}
}

func TestEnrichNodeStatuses_ConcurrentProbing(t *testing.T) {
	// Start 4 local TCP listeners and probe them concurrently
	const n = 4
	nodes := make([]gameap.NodeDTO, 0, n)
	for i := 0; i < n; i++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen %d: %v", i, err)
		}
		defer ln.Close()
		go func(l net.Listener) {
			for {
				c, err := l.Accept()
				if err != nil {
					return
				}
				_ = c.Close()
			}
		}(ln)

		port := ln.Addr().(*net.TCPAddr).Port
		nodes = append(nodes, gameap.NodeDTO{
			ID:          uint64(i + 1),
			Name:        "Local Node",
			GDaemonHost: "127.0.0.1",
			GDaemonPort: port,
			Enabled:     true,
		})
	}

	statuses := EnrichNodeStatuses(context.Background(), nodes, nil, nil, "ru")
	if len(statuses) != n {
		t.Fatalf("expected %d statuses, got %d", n, len(statuses))
	}
	for i, s := range statuses {
		if s.ID != uint64(i+1) {
			t.Errorf("expected order preserved at index %d (ID %d), got ID %d", i, i+1, s.ID)
		}
		if s.Latency == "" || s.Latency == "Защищён (DDoS Filter)" {
			t.Errorf("node %d: expected valid latency, got %q", i, s.Latency)
		}
	}
}
