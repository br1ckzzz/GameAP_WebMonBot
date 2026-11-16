package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"gameap-webmon-bot/internal/config"
	"gameap-webmon-bot/internal/gameap"
)

// ParseHexColor converts "#2ecc71" or "2ecc71" to integer for Discord embed.
func ParseHexColor(hexStr string, fallback int) int {
	clean := strings.TrimPrefix(strings.TrimSpace(hexStr), "#")
	if val, err := strconv.ParseInt(clean, 16, 32); err == nil {
		return int(val)
	}
	return fallback
}

// FormatUptime converts seconds into a human-readable duration string (e.g. "14д 06ч").
func FormatUptime(seconds uint64) string {
	if seconds == 0 {
		return ""
	}
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	mins := (seconds % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%dд %02dч", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dч %02dм", hours, mins)
	}
	return fmt.Sprintf("%dм", mins)
}

// GenerateProgressBar creates a visual ASCII progress bar [██████░░░░].
func GenerateProgressBar(current, max, length int) string {
	if max <= 0 {
		return "[░░░░░░░░░░]"
	}
	if current < 0 {
		current = 0
	}
	if current > max {
		current = max
	}
	if length <= 0 {
		length = 10
	}

	filled := (current * length) / max
	if filled > length {
		filled = length
	}

	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < filled; i++ {
		sb.WriteString("█")
	}
	for i := filled; i < length; i++ {
		sb.WriteString("░")
	}
	sb.WriteString("]")
	return sb.String()
}

// RenderGameGroupEmbed builds a stylish Discord embed for a group of servers sharing the same game.
func RenderGameGroupEmbed(gameName string, servers []gameap.PublicServerDTO, cfg *config.Config, webURL string) *discordgo.MessageEmbed {
	onlineCount := 0
	primaryGameCode := ""
	for _, s := range servers {
		if s.Status == "online" {
			onlineCount++
		}
		if primaryGameCode == "" && s.GameCode != "" {
			primaryGameCode = s.GameCode
		}
	}

	onlineColorHex, offlineColorHex := "", ""
	if cfg != nil {
		onlineColorHex = cfg.Bot.EmbedColorOnline
		offlineColorHex = cfg.Bot.EmbedColorOffline
	}
	color := ParseHexColor(onlineColorHex, 0x2ecc71)
	if onlineCount == 0 && len(servers) > 0 {
		color = ParseHexColor(offlineColorHex, 0xe74c3c)
	}

	statusIcon := "🟢"
	if onlineCount == 0 {
		statusIcon = "🔴"
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s %s (Онлайн: %d/%d)", statusIcon, gameName, onlineCount, len(servers)),
		Description: fmt.Sprintf("Обновлено <t:%d:R>", time.Now().Unix()),
		Color:       color,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	if thumb := GetGameThumbnailURL(primaryGameCode); thumb != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: thumb,
		}
	}

	for _, srv := range servers {
		isOnline := srv.Status == "online"
		srvIcon := "🔴"
		statusText := "Недоступен (Offline)"
		if isOnline {
			srvIcon = "🟢"
			statusText = "В сети (Online)"
		}

		fieldTitle := fmt.Sprintf("%s %s", srvIcon, srv.Name)
		var valBuilder strings.Builder

		// Line 1: Address
		valBuilder.WriteString(fmt.Sprintf("> 📍 **Адрес:** `%s:%d`\n", srv.Address, srv.Port))

		// Line 2: Map and Players progress bar if available, or Status
		if isOnline && (srv.MaxPlayers > 0 || srv.Map != "") {
			var details []string
			if srv.Map != "" {
				details = append(details, fmt.Sprintf("🗺️ **Карта:** `%s`", srv.Map))
			}
			if srv.MaxPlayers > 0 {
				pct := (srv.Players * 100) / srv.MaxPlayers
				bar := GenerateProgressBar(srv.Players, srv.MaxPlayers, 8)
				details = append(details, fmt.Sprintf("👥 **Игроки:** `%d/%d` %s (%d%%)", srv.Players, srv.MaxPlayers, bar, pct))
			}
			valBuilder.WriteString(fmt.Sprintf("> %s\n", strings.Join(details, " • ")))
		} else {
			valBuilder.WriteString(fmt.Sprintf("> 📊 **Статус:** `%s`\n", statusText))
		}

		// Line 3: Category and Uptime if present
		var extraDetails []string
		if srv.Category != "" {
			extraDetails = append(extraDetails, fmt.Sprintf("🏷️ **Категория:** %s", srv.Category))
		}
		if srv.UptimeSeconds > 0 {
			if uStr := FormatUptime(srv.UptimeSeconds); uStr != "" {
				extraDetails = append(extraDetails, fmt.Sprintf("⏱️ **Uptime:** %s", uStr))
			}
		}
		if len(extraDetails) > 0 {
			valBuilder.WriteString(fmt.Sprintf("> %s\n", strings.Join(extraDetails, " • ")))
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fieldTitle,
			Value:  valBuilder.String(),
			Inline: false,
		})
	}

	return embed
}

// RenderSingleServerEmbed builds a dedicated 3-column inline grid embed for an individual server.
func RenderSingleServerEmbed(srv gameap.PublicServerDTO, cfg *config.Config) *discordgo.MessageEmbed {
	isOnline := srv.Status == "online"

	onlineColorHex, offlineColorHex := "", ""
	if cfg != nil {
		onlineColorHex = cfg.Bot.EmbedColorOnline
		offlineColorHex = cfg.Bot.EmbedColorOffline
	}
	color := ParseHexColor(onlineColorHex, 0x2ecc71)
	statusIcon := "🟢"
	statusText := "В сети (Online)"
	if !isOnline {
		color = ParseHexColor(offlineColorHex, 0xe74c3c)
		statusIcon = "🔴"
		statusText = "Недоступен (Offline)"
	}

	cleanName := strings.TrimSpace(srv.Name)
	if cleanName == "" {
		cleanName = fmt.Sprintf("Сервер #%d", srv.ID)
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s %s", statusIcon, cleanName),
		Description: fmt.Sprintf("Обновлено <t:%d:R>", time.Now().Unix()),
		Color:       color,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	thumb := strings.TrimSpace(srv.IconURL)
	if thumb == "" {
		thumb = GetGameThumbnailURL(srv.GameCode)
	}
	if thumb == "" {
		thumb = GetGameThumbnailURL(srv.GameName)
	}
	if thumb == "" {
		if cfg != nil && cfg.Bot.DefaultThumbnailURL != "" {
			thumb = cfg.Bot.DefaultThumbnailURL
		} else {
			thumb = "https://raw.githubusercontent.com/br1ckzzz/gameap_monitoring_plugin/main/icon.png?v=1.0.0"
		}
	}
	if thumb != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: thumb,
		}
	}

	gameTitle := srv.GameName
	if gameTitle == "" {
		gameTitle = strings.ToUpper(srv.GameCode)
	}

	mapName := srv.Map
	if mapName == "" {
		mapName = "—"
	}

	playersStr := "—"
	if isOnline && srv.MaxPlayers > 0 {
		pct := (srv.Players * 100) / srv.MaxPlayers
		bar := GenerateProgressBar(srv.Players, srv.MaxPlayers, 8)
		playersStr = fmt.Sprintf("%d/%d %s (%d%%)", srv.Players, srv.MaxPlayers, bar, pct)
	} else if srv.Players > 0 {
		playersStr = fmt.Sprintf("%d", srv.Players)
	}

	uptimeStr := "—"
	if srv.UptimeSeconds > 0 {
		if u := FormatUptime(srv.UptimeSeconds); u != "" {
			uptimeStr = u
		}
	}

	embed.Fields = []*discordgo.MessageEmbedField{
		{
			Name:   "📊 Статус",
			Value:  statusText,
			Inline: true,
		},
		{
			Name:   "📍 Адрес и порт",
			Value:  fmt.Sprintf("`%s:%d`", srv.Address, srv.Port),
			Inline: true,
		},
		{
			Name:   "🎮 Игра",
			Value:  gameTitle,
			Inline: true,
		},
		{
			Name:   "🗺️ Карта",
			Value:  mapName,
			Inline: true,
		},
		{
			Name:   "👥 Игроки",
			Value:  playersStr,
			Inline: true,
		},
		{
			Name:   "⏱️ Uptime",
			Value:  uptimeStr,
			Inline: true,
		},
	}

	if srv.Category != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "🏷️ Категория",
			Value:  srv.Category,
			Inline: true,
		})
	}

	return embed
}

// FormatConnectCustomID encodes server ID, address, port, and gameCode into a compact CustomID.
func FormatConnectCustomID(srv gameap.PublicServerDTO) string {
	if srv.Address != "" && srv.Port > 0 {
		cleanGame := strings.TrimSpace(srv.GameCode)
		if cleanGame == "" {
			cleanGame = "a2s"
		}
		btnID := fmt.Sprintf("connect:%d|%s|%d|%s", srv.ID, srv.Address, srv.Port, cleanGame)
		if len(btnID) <= 100 {
			return btnID
		}
	}
	return fmt.Sprintf("connect:%d", srv.ID)
}

// BuildSingleServerButtons builds action row components for a single server card.
func BuildSingleServerButtons(srv gameap.PublicServerDTO) []discordgo.MessageComponent {
	var row []discordgo.MessageComponent

	if srv.Status == "online" {
		row = append(row, discordgo.Button{
			Label:    "Подключиться",
			Style:    discordgo.SecondaryButton,
			CustomID: FormatConnectCustomID(srv),
			Emoji: &discordgo.ComponentEmoji{
				Name: "🎮",
			},
		})
	}

	row = append(row, discordgo.Button{
		Label:    "Обновить",
		Style:    discordgo.PrimaryButton,
		CustomID: "refresh_dashboard",
		Emoji: &discordgo.ComponentEmoji{
			Name: "🔄",
		},
	})

	return []discordgo.MessageComponent{discordgo.ActionsRow{Components: row}}
}

// BuildServerButtons builds action row components: Connect buttons and Refresh button.
func BuildServerButtons(servers []gameap.PublicServerDTO) []discordgo.MessageComponent {
	var rows []discordgo.MessageComponent

	// First row: direct connect buttons for up to 5 servers
	var connectBtns []discordgo.MessageComponent
	for i, srv := range servers {
		if i >= 5 {
			break
		}
		if srv.Status == "online" {
			connectBtns = append(connectBtns, discordgo.Button{
				Label:    fmt.Sprintf("Подключиться #%d", i+1),
				Style:    discordgo.SecondaryButton,
				CustomID: FormatConnectCustomID(srv),
				Emoji: &discordgo.ComponentEmoji{
					Name: "🎮",
				},
			})
		}
	}

	// Action row for refresh button
	actionBtns := []discordgo.MessageComponent{
		discordgo.Button{
			Label:    "Обновить",
			Style:    discordgo.PrimaryButton,
			CustomID: "refresh_dashboard",
			Emoji: &discordgo.ComponentEmoji{
				Name: "🔄",
			},
		},
	}

	if len(connectBtns) > 0 {
		rows = append(rows, discordgo.ActionsRow{Components: connectBtns})
	}
	rows = append(rows, discordgo.ActionsRow{Components: actionBtns})

	return rows
}
