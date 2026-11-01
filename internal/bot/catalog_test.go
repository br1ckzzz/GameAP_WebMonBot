package bot

import (
	"strings"
	"testing"
)

func TestGetGameThumbnailURL(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{"cs2", "https://cdn.cloudflare.steamstatic.com/steam/apps/730/header.jpg"},
		{"CS2", "https://cdn.cloudflare.steamstatic.com/steam/apps/730/header.jpg"},
		{"cstrike", "https://cdn.cloudflare.steamstatic.com/steam/apps/10/header.jpg"},
		{"rust", "https://cdn.cloudflare.steamstatic.com/steam/apps/252490/header.jpg"},
		{"palwrld", "https://cdn.cloudflare.steamstatic.com/steam/apps/1623730/header.jpg"},
		{"palworld", "https://cdn.cloudflare.steamstatic.com/steam/apps/1623730/header.jpg"},
		{"7d2d", "https://cdn.cloudflare.steamstatic.com/steam/apps/251570/header.jpg"},
		{"svencoop", "https://cdn.cloudflare.steamstatic.com/steam/apps/225840/header.jpg"},
		{"minecraft", "minecraft"},
		{"unknown_game_xyz", ""},
	}

	for _, tt := range tests {
		got := GetGameThumbnailURL(tt.code)
		if tt.expected == "" {
			if got != "" {
				t.Errorf("GetGameThumbnailURL(%q) = %q, expected empty", tt.code, got)
			}
		} else {
			if !strings.Contains(got, tt.expected) && got != tt.expected {
				t.Errorf("GetGameThumbnailURL(%q) = %q, expected to contain %q", tt.code, got, tt.expected)
			}
		}
	}
}
