package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// RenderNodesStatusEmbed builds a polished Discord embed representing dedicated nodes/servers.
// Supports bilingual rendering: "ru" (Russian, default) and "en" (English).
func RenderNodesStatusEmbed(nodes []NodeStatus, updateIntervalSeconds int, onlineColorHex, offlineColorHex, lang string) *discordgo.MessageEmbed {
	if strings.ToLower(strings.TrimSpace(lang)) != "en" {
		lang = "ru"
	}
	isEn := lang == "en"

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
		var statusText string
		if isEn {
			if n.IsOnline {
				statusText = "🟢 System Normal"
			} else {
				statusText = "🔴 Offline / Maintenance"
			}
		} else {
			if n.IsOnline {
				statusText = "🟢 Система в норме"
			} else {
				statusText = "🔴 Недоступен / Техработы"
			}
		}

		fieldName := fmt.Sprintf("%s %s", n.Flag, n.Name)
		if n.Flag == "" {
			fieldName = fmt.Sprintf("🖥️ %s", n.Name)
		}

		valLines := make([]string, 0, 12)
		if isEn {
			valLines = append(valLines, fmt.Sprintf("> **Status:** %s", statusText))
			if n.Location != "" {
				valLines = append(valLines, fmt.Sprintf("> **Location:** %s", n.Location))
			}
			if n.Provider != "" {
				valLines = append(valLines, fmt.Sprintf("> **Provider:** %s", n.Provider))
			}
			if n.OS != "" {
				valLines = append(valLines, fmt.Sprintf("> **OS:** %s", n.OS))
			}
			if n.Host != "" {
				valLines = append(valLines, fmt.Sprintf("> **Host / IP:** `%s`", n.Host))
			}
			valLines = append(valLines, fmt.Sprintf("> **Game Servers:** %d active", n.ServersCount))
			if n.CPUModel != "" {
				valLines = append(valLines, fmt.Sprintf("> **CPU:** %s", n.CPUModel))
			}
			if n.CPUPercent > 0 {
				cpuPercent := n.CPUPercent
				if cpuPercent > 100 {
					cpuPercent = 100
				}
				cpuBar := GenerateProgressBar(cpuPercent, 100, 10)
				valLines = append(valLines, fmt.Sprintf("> **CPU Load:** `%s %d%%`", cpuBar, cpuPercent))
			}
			if n.RAMPercent > 0 {
				ramPercent := n.RAMPercent
				if ramPercent > 100 {
					ramPercent = 100
				}
				ramBar := GenerateProgressBar(ramPercent, 100, 10)
				valLines = append(valLines, fmt.Sprintf("> **RAM Usage:** `%s %d%%`", ramBar, ramPercent))
			}
			if n.Protection != "" {
				valLines = append(valLines, fmt.Sprintf("> **Protection:** 🛡️ %s", n.Protection))
			}
			valLines = append(valLines, fmt.Sprintf("> **Ping:** 📶 %s", n.Latency))
		} else {
			valLines = append(valLines, fmt.Sprintf("> **Статус:** %s", statusText))
			if n.Location != "" {
				valLines = append(valLines, fmt.Sprintf("> **Локация:** %s", n.Location))
			}
			if n.Provider != "" {
				valLines = append(valLines, fmt.Sprintf("> **Провайдер:** %s", n.Provider))
			}
			if n.OS != "" {
				valLines = append(valLines, fmt.Sprintf("> **ОС:** %s", n.OS))
			}
			if n.Host != "" {
				valLines = append(valLines, fmt.Sprintf("> **IP / Хост:** `%s`", n.Host))
			}
			valLines = append(valLines, fmt.Sprintf("> **Игровых серверов:** %d активных", n.ServersCount))
			if n.CPUModel != "" {
				valLines = append(valLines, fmt.Sprintf("> **Процессор:** %s", n.CPUModel))
			}
			if n.CPUPercent > 0 {
				cpuPercent := n.CPUPercent
				if cpuPercent > 100 {
					cpuPercent = 100
				}
				cpuBar := GenerateProgressBar(cpuPercent, 100, 10)
				valLines = append(valLines, fmt.Sprintf("> **Нагрузка CPU:** `%s %d%%`", cpuBar, cpuPercent))
			}
			if n.RAMPercent > 0 {
				ramPercent := n.RAMPercent
				if ramPercent > 100 {
					ramPercent = 100
				}
				ramBar := GenerateProgressBar(ramPercent, 100, 10)
				valLines = append(valLines, fmt.Sprintf("> **Память (RAM):** `%s %d%%`", ramBar, ramPercent))
			}
			if n.Protection != "" {
				valLines = append(valLines, fmt.Sprintf("> **Защита:** 🛡️ %s", n.Protection))
			}
			valLines = append(valLines, fmt.Sprintf("> **Пинг:** 📶 %s", n.Latency))
		}

		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fieldName,
			Value:  strings.Join(valLines, "\n"),
			Inline: false,
		})
	}

	if totalNodes == 0 {
		var emptyName, emptyVal string
		if isEn {
			emptyName = "ℹ️ Waiting for Node Connections"
			emptyVal = "> Network nodes are currently synchronizing with GameAP.\n> You can configure the node list in config.json (\"nodes\": [...]) or they will be detected automatically when active servers appear."
		} else {
			emptyName = "ℹ️ Ожидание подключения нод"
			emptyVal = "> В данный момент ноды сети синхронизируются с GameAP.\n> Вы можете настроить список нод в config.json (\"nodes\": [...]) либо они определятся автоматически при появлении активных серверов."
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   emptyName,
			Value:  emptyVal,
			Inline: false,
		})
	}

	locationsCount := len(uniqueLocationsMap)
	if locationsCount == 0 && totalNodes > 0 {
		locationsCount = 1
	}

	var summaryName, summaryValue, footerText, title, description string
	if isEn {
		title = "🖥️ Dedicated Servers Status (Nodes)"
		description = "Operational status of network host nodes and physical servers"
		summaryName = "🌐 Network Infrastructure"
		summaryValue = fmt.Sprintf("Total Locations: %d | Active Nodes: %d/%d\nUpdated every %d min",
			locationsCount, activeNodes, totalNodes, intervalMins)
		footerText = fmt.Sprintf("GameAP Monitoring • Synchronized • Updates every %d min", intervalMins)
	} else {
		title = "🖥️ Состояние выделенных серверов (Нод)"
		description = "Статус работы хост-нод и физических серверов сети"
		summaryName = "🌐 Инфраструктура сети"
		summaryValue = fmt.Sprintf("Всего локаций: %d | Активных нод: %d/%d\nОбновление каждые %d мин",
			locationsCount, activeNodes, totalNodes, intervalMins)
		footerText = fmt.Sprintf("GameAP Monitoring • Синхронизировано • Обновляется раз в %d мин", intervalMins)
	}

	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   summaryName,
		Value:  summaryValue,
		Inline: false,
	})

	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       embedColor,
		Fields:      fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: footerText,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// BuildNodesStatusComponents constructs the single interactive button for nodes monitoring.
// Supports bilingual label ("Обновить статус" / "Refresh Status").
func BuildNodesStatusComponents(lang string) []discordgo.MessageComponent {
	label := "Обновить статус"
	if strings.ToLower(strings.TrimSpace(lang)) == "en" {
		label = "Refresh Status"
	}

	btn := discordgo.Button{
		Label:    label,
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
