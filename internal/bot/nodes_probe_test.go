package bot

import (
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

	latency := ProbeNodeLatency("127.0.0.1", tcpAddr.Port, 1*time.Second)
	if latency == "" || latency == "Защищён (DDoS Filter)" {
		t.Errorf("expected valid latency for local server, got %q", latency)
	}

	// 2. Unreachable / filtered port returns "Защищён (DDoS Filter)"
	// On Windows, 127.0.0.1 on an unlistened port immediately refuses connection with error (RST)
	// Or non-routable IP
	latencyTimeout := ProbeNodeLatency("127.0.0.1", 59999, 100*time.Millisecond)
	if latencyTimeout != "Защищён (DDoS Filter)" {
		t.Errorf("expected DDoS Filter fallback on closed port, got %q", latencyTimeout)
	}

}

func TestEnrichNodeStatuses(t *testing.T) {
	gameapNodes := []gameap.NodeDTO{
		{
			ID:          1,
			Name:        "[GE] Ryzen 9 7950X3D — Frankfurt",
			Location:    "Germany, Frankfurt",
			Provider:    "Datacenter AG",
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
			Name:        "[RU] Core i9-14900K — Moscow",
			Host:        "127.0.0.1",
			Port:        31717,
			CPUModel:    "Intel Core i9-14900K",
			CPUPercent:  18,
			Protection:  "StormWall L3/L4/L7",
			ServersCount: 5,
		},
	}

	// Case 1: GameAP provided nodes
	statuses := EnrichNodeStatuses(gameapNodes, servers, nil)
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

	// Case 2: GameAP has no nodes, fallback to customNodes
	statusesCustom := EnrichNodeStatuses(nil, nil, customNodes)
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
}
