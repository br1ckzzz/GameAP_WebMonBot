package bot

import (
	"strings"

	"gameap-webmon-bot/internal/catalog"
)

// GetGameThumbnailURL returns an official game cover/thumbnail image URL from Steam CDN or catalog manifest.
func GetGameThumbnailURL(gameCode string) string {
	clean := strings.ToLower(strings.TrimSpace(gameCode))
	if clean == "" {
		return ""
	}

	// 1. Try unified 297-game catalog first
	if thumb := catalog.GetGameThumbnailURL(clean); thumb != "" && !strings.Contains(thumb, "gameap_monitoring_plugin/main/icon.png") {
		return thumb
	}

	// 2. If gameCode contains version/suffix like "Minecraft (26.3)", extract prefix
	if idx := strings.Index(clean, "("); idx > 0 {
		clean = strings.TrimSpace(clean[:idx])
	}
	if idx := strings.Index(clean, ":"); idx > 0 {
		clean = strings.TrimSpace(clean[:idx])
	}

	if thumb := catalog.GetGameThumbnailURL(clean); thumb != "" && !strings.Contains(thumb, "gameap_monitoring_plugin/main/icon.png") {
		return thumb
	}

	cleanAlpha := strings.ReplaceAll(clean, "_", "")
	cleanAlpha = strings.ReplaceAll(cleanAlpha, "-", "")
	cleanAlpha = strings.ReplaceAll(cleanAlpha, " ", "")

	if thumb := catalog.GetGameThumbnailURL(cleanAlpha); thumb != "" && !strings.Contains(thumb, "gameap_monitoring_plugin/main/icon.png") {
		return thumb
	}

	// 3. Fallback to hardcoded non-steam mappings if any
	if directURL, ok := nonSteamGameLogos[cleanAlpha]; ok {
		return directURL
	}
	if directURL, ok := nonSteamGameLogos[clean]; ok {
		return directURL
	}

	// 4. Fallback to default catalog thumbnail
	return catalog.GetGameThumbnailURL(clean)
}

// nonSteamGameLogos provides fallback logos for non-Steam games.
var nonSteamGameLogos = map[string]string{
	"minecraft": "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png/minecraft.png",
	"mc":        "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png/minecraft.png",
	"bedrock":   "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png/minecraft.png",
	"mcpe":      "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png/minecraft.png",
	"samp":      "https://cdn.cloudflare.steamstatic.com/steam/apps/12180/header.jpg",
	"gtasa":     "https://cdn.cloudflare.steamstatic.com/steam/apps/12180/header.jpg",
	"gta_sa":    "https://cdn.cloudflare.steamstatic.com/steam/apps/12180/header.jpg",
	"mta":       "https://cdn.cloudflare.steamstatic.com/steam/apps/12180/header.jpg",
	"fivem":     "https://cdn.steamstatic.com/steam/apps/271590/header.jpg",
	"gtav":      "https://cdn.steamstatic.com/steam/apps/271590/header.jpg",
	"gta5":      "https://cdn.steamstatic.com/steam/apps/271590/header.jpg",
	"quake":     "https://cdn.cloudflare.steamstatic.com/steam/apps/2200/header.jpg",
	"quake3":    "https://cdn.cloudflare.steamstatic.com/steam/apps/2200/header.jpg",
	"q3a":       "https://cdn.cloudflare.steamstatic.com/steam/apps/2200/header.jpg",
	"terraria":  "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png/terraria.png",
	"valheim":   "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png/valheim.png",
}
