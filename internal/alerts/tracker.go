package alerts

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"gameap-webmon-bot/internal/config"
	"gameap-webmon-bot/internal/gameap"
)

// ServerState records historical state of a single monitored server.
type ServerState struct {
	LastStatus        string
	OfflineFailCount  int
	StatusChangedAt   time.Time
	OfflineAlertSent  bool
	LastSeenOnlineAt  time.Time
	Name              string
	Address           string
	Port              int
	GameName          string
	LastVersion       string
	LastBuildID       string
	LastBlocked       bool
}

// Tracker monitors server life-cycle changes and emits Discord alerts.
type Tracker struct {
	mu           sync.Mutex
	cfg          *config.Config
	session      *discordgo.Session
	servers      map[uint64]*ServerState
	isFirstCheck bool

	enabled         bool
	alertsChannelID string
	alertRoleID     string
}

// NewTracker initializes a new server life-cycle tracker.
func NewTracker(cfg *config.Config, session *discordgo.Session) *Tracker {
	return &Tracker{
		cfg:             cfg,
		session:         session,
		servers:         make(map[uint64]*ServerState),
		isFirstCheck:    true,
		enabled:         cfg.Notifications.Enabled,
		alertsChannelID: cfg.Notifications.AlertsChannelID,
		alertRoleID:     cfg.Notifications.AlertRoleID,
	}
}

// UpdateAlertsConfig dynamically updates the active alert destination and ping role.
func (t *Tracker) UpdateAlertsConfig(enabled bool, channelID string, roleID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.enabled = enabled
	t.alertsChannelID = channelID
	t.alertRoleID = roleID
}

// GetAlertsConfig returns the current alert settings.
func (t *Tracker) GetAlertsConfig() (bool, string, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.enabled, t.alertsChannelID, t.alertRoleID
}

// ProcessUpdates checks the latest server snapshot and fires alerts when states transition.
func (t *Tracker) ProcessUpdates(servers []gameap.PublicServerDTO) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.enabled || t.alertsChannelID == "" {
		return
	}

	now := time.Now()

	for _, srv := range servers {
		state, exists := t.servers[srv.ID]
		if !exists {
			t.servers[srv.ID] = &ServerState{
				LastStatus:       srv.Status,
				OfflineFailCount: 0,
				StatusChangedAt:  now,
				OfflineAlertSent: false,
				LastSeenOnlineAt: now,
				Name:             srv.Name,
				Address:          srv.Address,
				Port:             srv.Port,
				GameName:         srv.GameName,
				LastVersion:      srv.Version,
				LastBuildID:      srv.BuildID,
				LastBlocked:      srv.Blocked,
			}
			continue
		}

		// Update server metadata
		state.Name = srv.Name
		state.Address = srv.Address
		state.Port = srv.Port
		state.GameName = srv.GameName

		// If this is initial baseline sync, don't spam alerts
		if t.isFirstCheck {
			state.LastStatus = srv.Status
			state.LastVersion = srv.Version
			state.LastBuildID = srv.BuildID
			state.LastBlocked = srv.Blocked
			continue
		}

		threshold := t.cfg.Notifications.OfflineConfirmThreshold
		if threshold < 1 {
			threshold = 1
		}

		if srv.Status == "offline" {
			state.OfflineFailCount++

			// Trigger offline alert only when flapping threshold is satisfied
			if state.OfflineFailCount >= threshold && !state.OfflineAlertSent {
				state.OfflineAlertSent = true
				state.StatusChangedAt = now
				if t.cfg.Notifications.NotifyOnOffline {
					go t.sendOfflineAlert(state, t.alertsChannelID, t.alertRoleID)
				}
			}
		} else if srv.Status == "online" {
			// Was it previously confirmed offline?
			if state.OfflineAlertSent {
				downtime := now.Sub(state.StatusChangedAt)
				state.OfflineAlertSent = false
				state.OfflineFailCount = 0
				state.StatusChangedAt = now
				state.LastSeenOnlineAt = now
				if t.cfg.Notifications.NotifyOnOnline {
					go t.sendOnlineRecoveryAlert(state, downtime, t.alertsChannelID)
				}
			} else if state.OfflineFailCount > 0 && state.OfflineFailCount < threshold {
				// Server dipped offline for 1 poll and quickly recovered: restart detected
				state.OfflineFailCount = 0
				state.LastSeenOnlineAt = now
				if t.cfg.Notifications.NotifyOnRestart {
					go t.sendRestartAlert(state, t.alertsChannelID)
				}
			}

			state.LastStatus = "online"
			state.LastSeenOnlineAt = now
		}

		// Detect game/server version or build ID update
		if t.cfg.Notifications.NotifyOnUpdate {
			if state.LastVersion != "" && srv.Version != "" && srv.Version != state.LastVersion {
				go t.sendUpdateAlert(state, state.LastVersion, srv.Version, t.alertsChannelID)
			} else if state.LastBuildID != "" && srv.BuildID != "" && srv.BuildID != state.LastBuildID {
				go t.sendUpdateAlert(state, fmt.Sprintf("Build %s", state.LastBuildID), fmt.Sprintf("Build %s", srv.BuildID), t.alertsChannelID)
			}
		}

		// Detect maintenance / blocked status transitions
		if srv.Blocked != state.LastBlocked {
			go t.sendMaintenanceAlert(state, srv.Blocked, t.alertsChannelID)
		}

		state.LastVersion = srv.Version
		state.LastBuildID = srv.BuildID
		state.LastBlocked = srv.Blocked
	}

	t.isFirstCheck = false
}

func (t *Tracker) sendOfflineAlert(state *ServerState, channelID, roleID string) {
	if channelID == "" {
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🔴 Сервер недоступен (Падение): %s", state.Name),
		Description: fmt.Sprintf("Сервер **%s** (%s) перестал отвечать на запросы мониторинга.", state.Name, state.GameName),
		Color:       0xe74c3c, // Red
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "📍 Адрес",
				Value:  fmt.Sprintf("`%s:%d`", state.Address, state.Port),
				Inline: true,
			},
			{
				Name:   "🕒 Время фиксации",
				Value:  time.Now().Format("15:04:05 MSK"),
				Inline: true,
			},
			{
				Name:   "⚠️ Статус",
				Value:  "Подтверждённое падение (offline)",
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "GameAP Alerts System",
		},
	}

	content := ""
	if roleID != "" {
		roles := strings.FieldsFunc(roleID, func(r rune) bool {
			return r == ',' || r == ';' || r == ' '
		})
		var mentions []string
		for _, r := range roles {
			r = strings.TrimSpace(r)
			if r != "" {
				mentions = append(mentions, fmt.Sprintf("<@&%s>", r))
			}
		}
		if len(mentions) > 0 {
			content = strings.Join(mentions, " ")
		}
	}

	_, _ = t.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: content,
		Embed:   embed,
	})
}

func (t *Tracker) sendOnlineRecoveryAlert(state *ServerState, downtime time.Duration, channelID string) {
	if channelID == "" {
		return
	}

	downtimeStr := formatDuration(downtime)
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🟢 Сервер снова доступен (Восстановление): %s", state.Name),
		Description: fmt.Sprintf("Сервер **%s** (%s) успешно запустился и принимает игроков.", state.Name, state.GameName),
		Color:       0x2ecc71, // Green
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "📍 Адрес",
				Value:  fmt.Sprintf("`%s:%d`", state.Address, state.Port),
				Inline: true,
			},
			{
				Name:   "⏱️ Время простоя (Downtime)",
				Value:  downtimeStr,
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "GameAP Alerts System",
		},
	}

	_, _ = t.session.ChannelMessageSendEmbed(channelID, embed)
}

func (t *Tracker) sendRestartAlert(state *ServerState, channelID string) {
	if channelID == "" {
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🔄 Перезагрузка сервера: %s", state.Name),
		Description: fmt.Sprintf("Обнаружен перезапуск игрового сервера **%s**.", state.Name),
		Color:       0xf1c40f, // Yellow
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "📍 Адрес",
				Value:  fmt.Sprintf("`%s:%d`", state.Address, state.Port),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "GameAP Alerts System",
		},
	}

	_, _ = t.session.ChannelMessageSendEmbed(channelID, embed)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	s := (d % time.Minute) / time.Second
	if m > 0 {
		return fmt.Sprintf("%d мин. %d сек.", m, s)
	}
	return fmt.Sprintf("%d сек.", s)
}

func (t *Tracker) sendUpdateAlert(state *ServerState, oldVer, newVer, channelID string) {
	if channelID == "" {
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📦 Обновление сервера: %s", state.Name),
		Description: fmt.Sprintf("Игровой сервер **%s** (%s) обновлён до новой версии!", state.Name, state.GameName),
		Color:       0x3498db, // Blue
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "📍 Адрес",
				Value:  fmt.Sprintf("`%s:%d`", state.Address, state.Port),
				Inline: true,
			},
			{
				Name:   "🏷️ Версия",
				Value:  fmt.Sprintf("`%s` ➔ `%s`", oldVer, newVer),
				Inline: true,
			},
			{
				Name:   "🕒 Время обновления",
				Value:  time.Now().Format("15:04:05 MSK"),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "GameAP Alerts System",
		},
	}

	_, _ = t.session.ChannelMessageSendEmbed(channelID, embed)
}

func (t *Tracker) sendMaintenanceAlert(state *ServerState, blocked bool, channelID string) {
	if channelID == "" {
		return
	}

	title := fmt.Sprintf("🛠️ Сервер переведён на техобслуживание: %s", state.Name)
	desc := fmt.Sprintf("Сервер **%s** (%s) временно заблокирован или переведён в режим техобслуживания.", state.Name, state.GameName)
	color := 0x9b59b6 // Purple
	statusVal := "Техработы / Обслуживание (Blocked)"

	if !blocked {
		title = fmt.Sprintf("🛠️ Сервер вернулся с техобслуживания: %s", state.Name)
		desc = fmt.Sprintf("Сервер **%s** (%s) разблокирован и возвращён в нормальный рабочий режим.", state.Name, state.GameName)
		color = 0x2ecc71 // Green
		statusVal = "Штатный режим (Unblocked)"
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: desc,
		Color:       color,
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "📍 Адрес",
				Value:  fmt.Sprintf("`%s:%d`", state.Address, state.Port),
				Inline: true,
			},
			{
				Name:   "⚠️ Состояние",
				Value:  statusVal,
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "GameAP Alerts System",
		},
	}

	_, _ = t.session.ChannelMessageSendEmbed(channelID, embed)
}

