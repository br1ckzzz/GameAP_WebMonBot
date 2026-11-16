package bot

import (
	"strings"
	"testing"

	"gameap-webmon-bot/internal/config"
	"gameap-webmon-bot/internal/gameap"
)

func TestGenerateProgressBar(t *testing.T) {
	tests := []struct {
		name     string
		current  int
		max      int
		length   int
		expected string
	}{
		{
			name:     "half full 10 segments",
			current:  16,
			max:      32,
			length:   10,
			expected: "[█████░░░░░]",
		},
		{
			name:     "empty",
			current:  0,
			max:      20,
			length:   10,
			expected: "[░░░░░░░░░░]",
		},
		{
			name:     "full",
			current:  20,
			max:      20,
			length:   10,
			expected: "[██████████]",
		},
		{
			name:     "overflow handled",
			current:  30,
			max:      20,
			length:   10,
			expected: "[██████████]",
		},
		{
			name:     "zero max",
			current:  5,
			max:      0,
			length:   10,
			expected: "[░░░░░░░░░░]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := GenerateProgressBar(tt.current, tt.max, tt.length)
			if actual != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, actual)
			}
		})
	}
}

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		input    string
		fallback int
		expected int
	}{
		{"#2ecc71", 0, 0x2ecc71},
		{"e74c3c", 0, 0xe74c3c},
		{"invalid", 12345, 12345},
		{"", 999, 999},
	}

	for _, tt := range tests {
		got := ParseHexColor(tt.input, tt.fallback)
		if got != tt.expected {
			t.Errorf("ParseHexColor(%s) = %d; expected %d", tt.input, got, tt.expected)
		}
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		seconds  uint64
		expected string
	}{
		{0, ""},
		{120, "2м"},
		{3660, "1ч 01м"},
		{90000, "1д 01ч"},
	}

	for _, tt := range tests {
		got := FormatUptime(tt.seconds)
		if got != tt.expected {
			t.Errorf("FormatUptime(%d) = %q, expected %q", tt.seconds, got, tt.expected)
		}
	}
}

func TestRenderGameGroupEmbed(t *testing.T) {
	cfg := config.DefaultConfig()

	servers := []gameap.PublicServerDTO{
		{
			ID:            1,
			Name:          "CS2 Mirage 24/7",
			GameCode:      "cs2",
			GameName:      "Counter-Strike 2",
			Address:       "127.0.0.1",
			Port:          27015,
			Status:        "online",
			Map:           "de_mirage",
			Players:       18,
			MaxPlayers:    32,
			Category:      "Public",
			UptimeSeconds: 90000,
		},
		{
			ID:       2,
			Name:     "CS2 Offline Test",
			GameCode: "cs2",
			GameName: "Counter-Strike 2",
			Address:  "127.0.0.1",
			Port:     27016,
			Status:   "offline",
		},
	}

	embed := RenderGameGroupEmbed("Counter-Strike 2", servers, cfg, "")

	if !strings.Contains(embed.Description, "<t:") || !strings.Contains(embed.Description, ":R>") {
		t.Errorf("expected Description to contain native relative timestamp <t:...:R>, got: %s", embed.Description)
	}

	if embed.Thumbnail == nil || embed.Thumbnail.URL == "" {
		t.Errorf("expected embed to have thumbnail URL for cs2")
	}

	if len(embed.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(embed.Fields))
	}

	// Server 1 (online with map, players, category, uptime)
	f1 := embed.Fields[0]
	if !strings.Contains(f1.Value, "127.0.0.1:27015") {
		t.Errorf("expected field to contain address, got: %s", f1.Value)
	}
	if !strings.Contains(f1.Value, "de_mirage") {
		t.Errorf("expected field to contain map name, got: %s", f1.Value)
	}
	if !strings.Contains(f1.Value, "18/32") {
		t.Errorf("expected field to contain player slots, got: %s", f1.Value)
	}
	if !strings.Contains(f1.Value, "Public") {
		t.Errorf("expected field to contain Category 'Public', got: %s", f1.Value)
	}
	if !strings.Contains(f1.Value, "1д 01ч") {
		t.Errorf("expected field to contain uptime, got: %s", f1.Value)
	}

	// Server 2 (offline)
	f2 := embed.Fields[1]
	if !strings.Contains(f2.Value, "Недоступен (Offline)") {
		t.Errorf("expected field 2 to contain offline status, got: %s", f2.Value)
	}
}

func TestRenderSingleServerEmbed(t *testing.T) {
	cfg := config.DefaultConfig()

	srv := gameap.PublicServerDTO{
		ID:            10,
		Name:          "Survival Vanilla",
		GameCode:      "minecraft",
		GameName:      "Minecraft",
		Address:       "mc.example.com",
		Port:          25565,
		Status:        "online",
		Map:           "world",
		Players:       12,
		MaxPlayers:    50,
		Category:      "Survival",
		UptimeSeconds: 7200,
	}

	embed := RenderSingleServerEmbed(srv, cfg)

	if embed.Thumbnail == nil || !strings.Contains(embed.Thumbnail.URL, "minecraft") {
		t.Errorf("expected minecraft thumbnail, got %+v", embed.Thumbnail)
	}

	if !strings.Contains(embed.Title, "Survival Vanilla") {
		t.Errorf("expected title to contain server name, got: %s", embed.Title)
	}

	foundMap := false
	foundPlayers := false
	foundCategory := false
	foundUptime := false

	for _, f := range embed.Fields {
		if strings.Contains(f.Name, "Карта") && f.Value == "world" {
			foundMap = true
		}
		if strings.Contains(f.Name, "Игроки") && strings.Contains(f.Value, "12/50") {
			foundPlayers = true
		}
		if strings.Contains(f.Name, "Категория") && f.Value == "Survival" {
			foundCategory = true
		}
		if strings.Contains(f.Name, "Uptime") && strings.Contains(f.Value, "2ч") {
			foundUptime = true
		}
	}

	if !foundMap || !foundPlayers || !foundCategory || !foundUptime {
		t.Errorf("missing fields in single server embed: map=%v, players=%v, cat=%v, uptime=%v",
			foundMap, foundPlayers, foundCategory, foundUptime)
	}

	btns := BuildSingleServerButtons(srv)
	if len(btns) != 1 {
		t.Fatalf("expected 1 action row for buttons, got %d", len(btns))
	}
}

func TestFormatConnectCustomID(t *testing.T) {
	srv := gameap.PublicServerDTO{
		ID:       131208,
		Name:     "Enshrouded CrazyParty",
		GameCode: "enshrouded",
		Address:  "server.party-host.top",
		Port:     15636,
		Status:   "online",
	}

	btnID := FormatConnectCustomID(srv)
	expected := "connect:131208|server.party-host.top|15636|enshrouded"
	if btnID != expected {
		t.Errorf("FormatConnectCustomID = %q, expected %q", btnID, expected)
	}

	handler := NewCommandHandler(config.DefaultConfig(), nil, nil, nil)
	handler.UpdateCachedServers([]gameap.PublicServerDTO{srv})

	cached, ok := handler.GetCachedServer(131208)
	if !ok || cached.Name != "Enshrouded CrazyParty" {
		t.Errorf("GetCachedServer(131208) failed, got %v", cached)
	}

	byAddr, ok := handler.GetCachedServerByAddr("server.party-host.top", 15636)
	if !ok || byAddr.ID != 131208 {
		t.Errorf("GetCachedServerByAddr failed, got %v", byAddr)
	}
}

func TestRenderGameGroupEmbed_NilConfig(t *testing.T) {
	servers := []gameap.PublicServerDTO{
		{
			ID:         1,
			Name:       "CS2 Public",
			GameCode:   "cs2",
			GameName:   "Counter-Strike 2",
			Address:    "127.0.0.1",
			Port:       27015,
			Players:    10,
			MaxPlayers: 32,
			Status:     "online",
		},
	}

	embed := RenderGameGroupEmbed("Counter-Strike 2", servers, nil, "")
	if embed == nil {
		t.Fatal("expected non-nil embed")
	}
	if embed.Color != 0x2ecc71 {
		t.Errorf("expected default online color 0x2ecc71, got 0x%x", embed.Color)
	}

	servers[0].Status = "offline"
	offlineEmbed := RenderGameGroupEmbed("Counter-Strike 2", servers, nil, "")
	if offlineEmbed == nil {
		t.Fatal("expected non-nil offline embed")
	}
	if offlineEmbed.Color != 0xe74c3c {
		t.Errorf("expected default offline color 0xe74c3c, got 0x%x", offlineEmbed.Color)
	}
}

func TestRenderSingleServerEmbed_NilConfig(t *testing.T) {
	srv := gameap.PublicServerDTO{
		ID:         1,
		Name:       "Test Server",
		GameCode:   "cs2",
		GameName:   "Counter-Strike 2",
		Address:    "127.0.0.1",
		Port:       27015,
		Players:    5,
		MaxPlayers: 20,
		Status:     "online",
	}

	embed := RenderSingleServerEmbed(srv, nil)
	if embed == nil {
		t.Fatal("expected non-nil embed")
	}
	if embed.Color != 0x2ecc71 {
		t.Errorf("expected default online color 0x2ecc71, got 0x%x", embed.Color)
	}

	srv.Status = "offline"
	offlineEmbed := RenderSingleServerEmbed(srv, nil)
	if offlineEmbed == nil {
		t.Fatal("expected non-nil offline embed")
	}
	if offlineEmbed.Color != 0xe74c3c {
		t.Errorf("expected default offline color 0xe74c3c, got 0x%x", offlineEmbed.Color)
	}
}
