package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// RenderNodesStatusEmbed builds a polished Discord embed representing dedicated nodes/servers.
func RenderNodesStatusEmbed(nodes []NodeStatus, updateIntervalSeconds int, onlineColorHex, offlineColorHex string) *discordgo.MessageEmbed {
	totalNodes := len(nodes)
	activeNodes := 0
	uniqueLocationsMap := make(map[string]bool)

	allOnline := true
	for _, n := range nodes {
		if n.IsOnline {
			activeNodes++
		} else {
			allOnline = false
		}
		locKey := n.CountryCode
		if locKey == "" || locKey == "GLOBAL" {
			locKey = n.Location
		}
		if locKey != "" {
			uniqueLocationsMap[locKey] = true
		}
	}

	embedColor := ParseHexColor(onlineColorHex, 0x2ecc71)
	if !allOnline && totalNodes > 0 {
		embedColor = ParseHexColor(offlineColorHex, 0xe74c3c)
	}

	intervalMins := updateIntervalSeconds / 60
	if intervalMins <= 0 {
		intervalMins = 5
	}

	fields := make([]*discordgo.MessageEmbedField, 0, totalNodes+1)

	for _, n := range nodes {
		statusText := "🟢 Система в норме"
		if !n.IsOnline {
			statusText = "🔴 Недоступен / Техработы"
		}

		cpuPercent := n.CPUPercent
		if cpuPercent < 0 {
			cpuPercent = 0
		}
		if cpuPercent > 100 {
			cpuPercent = 100
		}
		cpuBar := GenerateProgressBar(cpuPercent, 100, 10)

		fieldName := fmt.Sprintf("%s %s", n.Flag, n.Name)
		if n.Flag == "" {
			fieldName = fmt.Sprintf("🖥️ %s", n.Name)
		}

		valLines := []string{
			fmt.Sprintf("> **Статус:** %s", statusText),
			fmt.Sprintf("> **Игровых серверов:** %d активных", n.ServersCount),
			fmt.Sprintf("> **Процессор:** %s", n.CPUModel),
			fmt.Sprintf("> **Нагрузка CPU:** `%s %d%%`", cpuBar, cpuPercent),
			fmt.Sprintf("> **Защита:** 🛡️ %s", n.Protection),
			fmt.Sprintf("> **Пинг:** 📶 %s", n.Latency),
		}

		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fieldName,
			Value:  strings.Join(valLines, "\n"),
			Inline: false,
		})
	}

	locationsCount := len(uniqueLocationsMap)
	if locationsCount == 0 && totalNodes > 0 {
		locationsCount = 1
	}

	summaryValue := fmt.Sprintf("Всего локаций: %d | Активных нод: %d/%d\nОбновление каждые %d мин",
		locationsCount, activeNodes, totalNodes, intervalMins)

	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "🌐 Инфраструктура сети",
		Value:  summaryValue,
		Inline: false,
	})

	footerText := fmt.Sprintf("GameAP Monitoring • Синхронизировано • Обновляется раз в %d мин", intervalMins)

	return &discordgo.MessageEmbed{
		Title:       "🖥️ Состояние выделенных серверов (Нод)",
		Description: "Статус работы хост-нод и физических серверов сети",
		Color:       embedColor,
		Fields:      fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: footerText,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// BuildNodesStatusComponents constructs the single interactive button for nodes monitoring.
// Notice: The web panel link button is omitted as requested by the user.
func BuildNodesStatusComponents() []discordgo.MessageComponent {
	btn := discordgo.Button{
		Label:    "Обновить статус",
		Style:    discordgo.PrimaryButton,
		CustomID: "refresh_nodes_status",
		Emoji: &discordgo.ComponentEmoji{
			Name: "🔄",
		},
	}

	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{btn},
		},
	}
}
