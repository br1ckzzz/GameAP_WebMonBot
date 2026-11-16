package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"gameap-webmon-bot/internal/alerts"
	"gameap-webmon-bot/internal/catalog"
	"gameap-webmon-bot/internal/config"
	"gameap-webmon-bot/internal/gameap"
	"gameap-webmon-bot/internal/query"
)

// CommandHandler manages slash commands and interactive button components.
type CommandHandler struct {
	mu              sync.Mutex
	cfg             *config.Config
	gameapClient    *gameap.Client
	state           *State
	refreshChan     chan struct{}
	alertsTracker   *alerts.Tracker
	lastManual      time.Time
	refreshFunc      func(ctx context.Context) error
	refreshNodesFunc func(ctx context.Context) error
	purgeFunc        func(channelID string) (int, error)
	cachedServersMu sync.RWMutex
	cachedServers   map[uint64]gameap.PublicServerDTO
	cachedByAddr    map[string]gameap.PublicServerDTO
}

// NewCommandHandler creates a new command handler.
func NewCommandHandler(cfg *config.Config, gameapClient *gameap.Client, state *State, refreshChan chan struct{}) *CommandHandler {
	return &CommandHandler{
		cfg:           cfg,
		gameapClient:  gameapClient,
		state:         state,
		refreshChan:   refreshChan,
		cachedServers: make(map[uint64]gameap.PublicServerDTO),
		cachedByAddr:  make(map[string]gameap.PublicServerDTO),
	}
}

// UpdateCachedServers stores the latest snapshot of servers for fast in-memory lookups.
func (h *CommandHandler) UpdateCachedServers(servers []gameap.PublicServerDTO) {
	h.cachedServersMu.Lock()
	defer h.cachedServersMu.Unlock()

	h.cachedServers = make(map[uint64]gameap.PublicServerDTO, len(servers))
	h.cachedByAddr = make(map[string]gameap.PublicServerDTO, len(servers))

	for _, s := range servers {
		h.cachedServers[s.ID] = s
		if s.Address != "" && s.Port > 0 {
			key := strings.ToLower(fmt.Sprintf("%s:%d", strings.TrimSpace(s.Address), s.Port))
			h.cachedByAddr[key] = s
		}
	}
}

// GetCachedServer retrieves a server by its ID from memory cache.
func (h *CommandHandler) GetCachedServer(id uint64) (gameap.PublicServerDTO, bool) {
	h.cachedServersMu.RLock()
	defer h.cachedServersMu.RUnlock()
	if h.cachedServers == nil {
		return gameap.PublicServerDTO{}, false
	}
	s, ok := h.cachedServers[id]
	return s, ok
}

// GetCachedServerByAddr retrieves a server by address and port from memory cache.
func (h *CommandHandler) GetCachedServerByAddr(addr string, port int) (gameap.PublicServerDTO, bool) {
	h.cachedServersMu.RLock()
	defer h.cachedServersMu.RUnlock()
	if h.cachedByAddr == nil {
		return gameap.PublicServerDTO{}, false
	}
	key := strings.ToLower(fmt.Sprintf("%s:%d", strings.TrimSpace(addr), port))
	s, ok := h.cachedByAddr[key]
	return s, ok
}

// SetRefreshFunc assigns the callback for executing an immediate dashboard refresh.
func (h *CommandHandler) SetRefreshFunc(fn func(ctx context.Context) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.refreshFunc = fn
}

// SetPurgeFunc assigns the callback for purging old bot messages from a channel.
func (h *CommandHandler) SetPurgeFunc(fn func(channelID string) (int, error)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.purgeFunc = fn
}

// SetRefreshNodesFunc assigns the callback for executing an immediate nodes status refresh.
func (h *CommandHandler) SetRefreshNodesFunc(fn func(ctx context.Context) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.refreshNodesFunc = fn
}

// SetAlertsTracker sets the alert tracker reference for dynamic alert configuration.
func (h *CommandHandler) SetAlertsTracker(tracker *alerts.Tracker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.alertsTracker = tracker
}

// GetSlashCommands defines the application slash commands for Discord registration.
func (h *CommandHandler) GetSlashCommands() []*discordgo.ApplicationCommand {
	var defaultAdminPerms int64 = discordgo.PermissionManageServer
	return []*discordgo.ApplicationCommand{
		{
			Name:                     "mon",
			Description:              "Управление и мониторинг игровых серверов GameAP",
			DefaultMemberPermissions: &defaultAdminPerms,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "bind",
					Description: "Привязать табло серверов, конкретной игры или отдельного сервера к каналу",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionChannel,
							Name:        "channel",
							Description: "Текстовый канал для табло (по умолчанию: текущий)",
							Required:    false,
							ChannelTypes: []discordgo.ChannelType{
								discordgo.ChannelTypeGuildText,
							},
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "game",
							Description: "Код или название игры (valheim, romestead, cs2...). Если пусто — общее табло.",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "server",
							Description: "ID сервера для привязки персональной карточки (например 1)",
							Required:    false,
						},
					},
				},
				{
					Name:        "unbind",
					Description: "Отвязать отдельную игру, сервер или общее табло от канала",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "game",
							Description: "Игра для отвязки (valheim, romestead...). Если пусто или 'all' — общее табло.",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "server",
							Description: "ID сервера для отвязки персональной карточки (например 1)",
							Required:    false,
						},
					},
				},
				{
					Name:        "bindings",
					Description: "Показать текущие привязки каналов к играм",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
				},
				{
					Name:        "setup",
					Description: "Настроить адрес и токен API GameAP WebMonitoring",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "url",
							Description: "URL API WebMonitoring (например: https://gameap.domain.com/api/plugins/monitoring)",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "token",
							Description: "Секретный токен Bot API из панели GameAP (wm_...)",
							Required:    false,
						},
					},
				},
				{
					Name:        "status",
					Description: "Показать текущую сводку онлайна игровых серверов",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
				},
				{
					Name:        "refresh",
					Description: "Принудительно обновить информационное табло серверов",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
				},
				{
					Name:        "clean",
					Description: "Очистить зависшие/дублирующие карточки бота в текущем канале",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
				},
				{
					Name:        "alerts",
					Description: "Настроить канал и роль для оповещений о падении/запуске серверов",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionChannel,
							Name:        "channel",
							Description: "Канал для отправки алертов",
							Required:    false,
							ChannelTypes: []discordgo.ChannelType{
								discordgo.ChannelTypeGuildText,
							},
						},
						{
							Type:        discordgo.ApplicationCommandOptionRole,
							Name:        "role",
							Description: "Основная роль для пинга при падении сервера (offline)",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionRole,
							Name:        "role2",
							Description: "Вторая роль для пинга при падении сервера (опционально)",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "enabled",
							Description: "Включить (true) или выключить (false) оповещения",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "disable",
							Description: "Отключить оповещения",
							Required:    false,
						},
					},
				},
				{
					Name:        "forum",
					Description: "Привязать Forum Channel для создания тем по играм (Сценарий Б)",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionChannel,
							Name:        "channel",
							Description: "Канал форума Discord (Forum Channel)",
							Required:    false,
							ChannelTypes: []discordgo.ChannelType{
								discordgo.ChannelTypeGuildForum,
								discordgo.ChannelTypeGuildText,
							},
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "unbind",
							Description: "Отвязать канал форума",
							Required:    false,
						},
					},
				},
				{
					Name:        "ping",
					Description: "Проверить задержку Discord Gateway и GameAP API",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
				},
				{
					Name:        "add",
					Description: "Добавить внешний игровой сервер для мониторинга (любой IP:PORT)",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "address",
							Description: "Адрес сервера в формате IP:PORT (например: 185.12.34.56:7777)",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "game",
							Description: "Код или название игры (samp, bedrock, cs2, valheim, mc, quake3...)",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "name",
							Description: "Пользовательское название (если не указано — берётся с сервера)",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionChannel,
							Name:        "channel",
							Description: "Канал для карточки сервера (по умолчанию: текущий)",
							Required:    false,
							ChannelTypes: []discordgo.ChannelType{
								discordgo.ChannelTypeGuildText,
							},
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "icon",
							Description: "Прямая ссылка на логотип/иконку сервера (например: https://.../icon.png)",
							Required:    false,
						},
					},
				},
				{
					Name:        "remove",
					Description: "Удалить внешний сервер из мониторинга",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "address",
							Description: "Адрес удаляемого сервера (IP:PORT)",
							Required:    true,
						},
					},
				},
				{
					Name:        "list",
					Description: "Показать список добавленных внешних серверов",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
				},
				{
					Name:        "roles",
					Description: "Настройка ролей менеджеров мониторинга (Manager Roles)",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionRole,
							Name:        "add",
							Description: "Добавить роль с правами управления ботом",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionRole,
							Name:        "remove",
							Description: "Отозвать права управления у роли",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "list",
							Description: "Показать список разрешённых ролей",
							Required:    false,
						},
					},
				},
				{
					Name:        "games",
					Description: "Справочник 297 поддерживаемых игр DiscordGSM и быстрый поиск по кодам",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "search",
							Description: "Поиск игры по коду или названию (например: cs2, valheim, minecraft, quake)",
							Required:    false,
						},
					},
				},
				{
					Name:        "group",
					Description: "Отобразить или привязать единое групповое табло для выбранной игры",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "game",
							Description: "Код или название игры для группового табло (например cs2, valheim, samp)",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionChannel,
							Name:        "channel",
							Description: "Канал для группового табло (по умолчанию: текущий канал)",
							Required:    false,
							ChannelTypes: []discordgo.ChannelType{
								discordgo.ChannelTypeGuildText,
							},
						},
					},
				},
				{
					Name:        "nodes_channel",
					Description: "Привязать канал для мониторинга выделенных серверов (Нод)",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionChannel,
							Name:        "channel",
							Description: "Текстовый канал для статуса нод (по умолчанию: текущий)",
							Required:    false,
							ChannelTypes: []discordgo.ChannelType{
								discordgo.ChannelTypeGuildText,
							},
						},
						{
							Type:        discordgo.ApplicationCommandOptionBoolean,
							Name:        "unbind",
							Description: "Отвязать канал статуса нод",
							Required:    false,
						},
					},
				},
			},
		},
	}
}

// HandleInteraction processes both slash commands and button clicks.
func (h *CommandHandler) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		h.handleApplicationCommand(s, i)
	case discordgo.InteractionMessageComponent:
		h.handleMessageComponent(s, i)
	}
}

func (h *CommandHandler) handleApplicationCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if data.Name != "mon" {
		return
	}

	// 1. Strict Guild Lockdown check (if configured in config.json)
	if h.cfg != nil && h.cfg.Discord.GuildID != "" && i.GuildID != "" && i.GuildID != h.cfg.Discord.GuildID {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Этот бот настроен на работу только на авторизованном сервере.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// 2. Authorization check: only Administrator, Manage Server or configured Manager Roles
	if !h.isAuthorized(i) {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ **Отказано в доступе.**\nДля управления мониторингом требуются права **Администратора** / **Управление сервером** либо назначенная роль менеджера бота (`/mon roles`).",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	subCommand := ""
	if len(data.Options) > 0 {
		subCommand = data.Options[0].Name
	}

	switch subCommand {
	case "roles":
		var opts []*discordgo.ApplicationCommandInteractionDataOption
		if len(data.Options) > 0 {
			opts = data.Options[0].Options
		}
		h.handleMonRoles(s, i, opts)

	case "bind":
		targetChannelID := i.ChannelID
		gameArg := ""
		var serverArg uint64
		if len(data.Options) > 0 && len(data.Options[0].Options) > 0 {
			for _, opt := range data.Options[0].Options {
				if opt.Name == "channel" {
					if ch, ok := opt.Value.(string); ok && ch != "" {
						targetChannelID = ch
					}
				} else if opt.Name == "game" {
					if g, ok := opt.Value.(string); ok {
						gameArg = strings.TrimSpace(g)
					}
				} else if opt.Name == "server" {
					if srvID, ok := opt.Value.(float64); ok {
						serverArg = uint64(srvID)
					}
				}
			}
		}

		guildID := i.GuildID
		var msgText string
		if serverArg > 0 {
			_ = h.state.BindServerChannel(guildID, serverArg, targetChannelID)
			msgText = fmt.Sprintf("✅ Персональная карточка сервера **#%d** успешно привязана к каналу <#%s>!\nКарточка формируется и публикуется прямо сейчас...", serverArg, targetChannelID)
		} else if gameArg != "" && !strings.EqualFold(gameArg, "all") && !strings.EqualFold(gameArg, "все") {
			_ = h.state.BindGameChannel(guildID, gameArg, targetChannelID)
			msgText = fmt.Sprintf("✅ Серверы игры **%s** успешно привязаны к каналу <#%s>!\nТабло формируется и публикуется прямо сейчас...", gameArg, targetChannelID)
		} else {
			_ = h.state.BindChannel(guildID, targetChannelID)
			msgText = fmt.Sprintf("✅ Общее табло серверов успешно привязано к каналу <#%s>!\nТабло формируется и публикуется прямо сейчас...", targetChannelID)
		}

		h.deferResponse(s, i)

		// Purge old bot messages from the target channel to prevent duplicates
		if h.purgeFunc != nil {
			_, _ = h.purgeFunc(targetChannelID)
		}

		if warning := checkChannelPermissions(s, targetChannelID); warning != "" {
			msgText += "\n\n" + warning
		}

		h.editResponse(s, i, msgText)

		// Trigger immediate refresh so the card appears right away without needing /mon refresh
		h.mu.Lock()
		refreshFn := h.refreshFunc
		h.mu.Unlock()
		if refreshFn != nil {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_ = refreshFn(ctx)
			}()
		} else {
			h.triggerManualRefresh()
		}

	case "unbind":
		gameArg := ""
		var serverArg uint64
		if len(data.Options) > 0 && len(data.Options[0].Options) > 0 {
			for _, opt := range data.Options[0].Options {
				if opt.Name == "game" {
					if g, ok := opt.Value.(string); ok {
						gameArg = strings.TrimSpace(g)
					}
				} else if opt.Name == "server" {
					if srvID, ok := opt.Value.(float64); ok {
						serverArg = uint64(srvID)
					}
				}
			}
		}

		var msgText string
		if serverArg > 0 {
			_ = h.state.UnbindServerChannel(serverArg)
			msgText = fmt.Sprintf("🗑️ Привязка для сервера **#%d** успешно удалена.", serverArg)
		} else if gameArg != "" && !strings.EqualFold(gameArg, "all") && !strings.EqualFold(gameArg, "все") {
			_ = h.state.UnbindGameChannel(gameArg)
			msgText = fmt.Sprintf("🗑️ Привязка для игры **%s** успешно удалена.", gameArg)
		} else {
			_ = h.state.BindChannel(i.GuildID, "")
			msgText = "🗑️ Привязка общего табло успешно удалена."
		}

		h.mu.Lock()
		refreshFn := h.refreshFunc
		h.mu.Unlock()
		if refreshFn != nil {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_ = refreshFn(ctx)
			}()
		} else {
			h.triggerManualRefresh()
		}

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: msgText,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})

	case "bindings":
		generalCh := h.state.GetChannelID()
		if generalCh == "" && h.cfg != nil {
			generalCh = h.cfg.Discord.DashboardChannelID
		}

		generalText := "Не привязано (используйте `/mon bind`)"
		if generalCh != "" {
			generalText = fmt.Sprintf("<#%s>", generalCh)
		}

		gameChannels := h.state.GetGameChannels()
		var gameLines []string
		for g, ch := range gameChannels {
			gameLines = append(gameLines, fmt.Sprintf("• **%s** ➔ <#%s>", strings.ToUpper(g), ch))
		}
		if h.cfg != nil && h.cfg.Discord.GameChannels != nil {
			for g, ch := range h.cfg.Discord.GameChannels {
				normKey := strings.ToLower(strings.TrimSpace(g))
				normKey = strings.ReplaceAll(normKey, " ", "_")
				normKey = strings.ReplaceAll(normKey, "-", "_")
				if _, exists := gameChannels[normKey]; !exists {
					gameLines = append(gameLines, fmt.Sprintf("• **%s** ➔ <#%s> *(из config.json)*", strings.ToUpper(g), ch))
				}
			}
		}

		gamesText := "Отдельные каналы для игр пока не привязаны.\nПривязать игру: `/mon bind game:cs2 channel:#канал`"
		if len(gameLines) > 0 {
			gamesText = strings.Join(gameLines, "\n")
		}

		serverChannels := h.state.GetServerChannels()
		var serverLines []string
		for srvID, ch := range serverChannels {
			serverLines = append(serverLines, fmt.Sprintf("• **Сервер #%d** ➔ <#%s>", srvID, ch))
		}
		if h.cfg != nil && h.cfg.Discord.ServerChannels != nil {
			for sKey, ch := range h.cfg.Discord.ServerChannels {
				sID, _ := strconv.ParseUint(sKey, 10, 64)
				if _, exists := serverChannels[sID]; !exists {
					serverLines = append(serverLines, fmt.Sprintf("• **Сервер %s** ➔ <#%s> *(из config.json)*", sKey, ch))
				}
			}
		}
		serversText := "Персональные каналы серверов не привязаны.\nПривязать сервер: `/mon bind server:1 channel:#канал`"
		if len(serverLines) > 0 {
			serversText = strings.Join(serverLines, "\n")
		}

		embed := &discordgo.MessageEmbed{
			Title:       "📌 Текущие привязки каналов мониторинга",
			Color:       0x0ea5e9,
			Timestamp:   time.Now().Format(time.RFC3339),
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "📊 Общее табло (все серверы)",
					Value:  generalText,
					Inline: false,
				},
				{
					Name:   "🎮 Каналы по конкретным играм",
					Value:  gamesText,
					Inline: false,
				},
				{
					Name:   "🎯 Персональные каналы серверов (Single Server)",
					Value:  serversText,
					Inline: false,
				},
				{
					Name: "🖥️ Состояние нод (выделенных серверов)",
					Value: func() string {
						nodesCh := h.state.GetNodesChannelID()
						if nodesCh == "" && h.cfg != nil {
							nodesCh = h.cfg.Discord.NodesChannelID
						}
						if nodesCh != "" {
							return fmt.Sprintf("<#%s>", nodesCh)
						}
						return "Не привязано (используйте `/mon nodes_channel channel:#канал`)"
					}(),
					Inline: false,
				},
			},
		}

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})

	case "setup":
		if !h.isServerAdmin(i) {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "❌ Только **Администраторы сервера** (права Администратор / Управление сервером) могут изменять подключение к GameAP.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		h.deferResponse(s, i)

		apiURL := ""
		apiToken := ""
		if len(data.Options) > 0 {
			for _, opt := range data.Options[0].Options {
				if opt.Name == "url" {
					if u, ok := opt.Value.(string); ok {
						apiURL = strings.TrimSpace(u)
					}
				} else if opt.Name == "token" {
					if t, ok := opt.Value.(string); ok {
						apiToken = strings.TrimSpace(t)
					}
				}
			}
		}

		if err := validateSetupURL(apiURL); err != nil {
			h.editResponse(s, i, fmt.Sprintf("❌ Некорректный `url`: %s", err.Error()))
			return
		}

		h.gameapClient.UpdateCredentials(apiURL, apiToken)
		_ = h.state.SetCredentials(apiURL, apiToken)

		// Test connection immediately
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		resp, err := h.gameapClient.FetchServers(ctx)
		if err != nil {
			log.Printf("[WARN] /mon setup connection test failed: %v", err)
			h.editResponse(s, i, fmt.Sprintf("⚠️ Параметры сохранены, но тест связи не прошёл: %s.\nПроверьте доступность URL и правильность токена.", sanitizeSetupError(err)))
			return
		}

		h.triggerManualRefresh()
		h.editResponse(s, i, fmt.Sprintf("✅ Связь с GameAP успешно установлена!\nОбнаружено серверов: **%d** (в сети: **%d**).", resp.TotalServers, resp.OnlineCount))

	case "status":
		h.deferResponse(s, i)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := h.gameapClient.FetchServers(ctx)
		if err != nil {
			h.editResponse(s, i, fmt.Sprintf("❌ Ошибка получения данных от GameAP: `%s`", err.Error()))
			return
		}

		online := 0
		for _, srv := range resp.Servers {
			if srv.Status == "online" {
				online++
			}
		}

		embed := &discordgo.MessageEmbed{
			Title:       "📊 Сводка мониторинга GameAP",
			Description: fmt.Sprintf("Всего серверов: **%d** | В сети: **%d**", len(resp.Servers), online),
			Color:       0x0ea5e9,
			Timestamp:   time.Now().Format(time.RFC3339),
		}

		for _, srv := range resp.Servers {
			icon := "🔴"
			if srv.Status == "online" {
				icon = "🟢"
			}
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("%s [#%d] %s", icon, srv.ID, srv.Name),
				Value:  fmt.Sprintf("`%s:%d` (%s)", srv.Address, srv.Port, srv.GameName),
				Inline: true,
			})
		}

		for _, cs := range h.state.GetCustomServers() {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("🌐 [#%d] %s", cs.ID, cs.Name),
				Value:  fmt.Sprintf("`%s:%d` (%s) *(внешний)*", cs.Address, cs.Port, cs.GameName),
				Inline: true,
			})
		}

		embed.Fields = capEmbedFields(embed.Fields, 24, "серверов")
		h.editResponseEmbed(s, i, embed)

	case "refresh":
		h.deferResponse(s, i)

		h.mu.Lock()
		refreshFn := h.refreshFunc
		h.mu.Unlock()

		go func() {
			if refreshFn != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := refreshFn(ctx); err != nil {
					h.editResponse(s, i, fmt.Sprintf("⚠️ Не удалось обновить табло: %v", err))
					return
				}
			} else {
				h.triggerManualRefresh()
			}

			h.editResponse(s, i, "✅ Табло мониторинга успешно обновлено!")
			time.Sleep(4 * time.Second)
			appID := i.Interaction.AppID
			if appID == "" && h.cfg != nil {
				appID = h.cfg.Bot.ApplicationID
			}
			_ = s.WebhookMessageDelete(appID, i.Interaction.Token, "@original")
		}()

	case "clean":
		h.deferResponse(s, i)
		targetChannelID := i.ChannelID

		go func() {
			var deletedCount int
			if h.purgeFunc != nil {
				cnt, err := h.purgeFunc(targetChannelID)
				if err != nil {
					h.editResponse(s, i, fmt.Sprintf("⚠️ Ошибка при очистке: %v", err))
					return
				}
				deletedCount = cnt
			}

			if h.refreshFunc != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_ = h.refreshFunc(ctx)
			} else {
				h.triggerManualRefresh()
			}

			h.editResponse(s, i, fmt.Sprintf("🧹 Канал <#%s> очищен! Удалено старых карточек бота: %d.\nСформировано актуальное табло.", targetChannelID, deletedCount))
		}()

	case "ping":
		h.deferResponse(s, i)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var apiText string
		if h.gameapClient != nil && h.gameapClient.GetBaseURL() != "" {
			apiLatency, err := h.gameapClient.Ping(ctx)
			apiText = fmt.Sprintf("%d ms", apiLatency.Milliseconds())
			if err != nil {
				apiText = fmt.Sprintf("Ошибка (%s)", err.Error())
			}
		} else {
			apiText = "Отключено (Автономный режим)"
		}

		gwLatency := s.HeartbeatLatency().Milliseconds()
		h.editResponse(s, i, fmt.Sprintf("🏓 **Понг!**\n• Discord Gateway: `%d ms`\n• GameAP API: `%s`", gwLatency, apiText))

	case "alerts":
		var opts []*discordgo.ApplicationCommandInteractionDataOption
		if len(data.Options) > 0 && len(data.Options[0].Options) > 0 {
			opts = data.Options[0].Options
		}
		h.handleAlertsCommand(s, i, opts)

	case "forum":
		var opts []*discordgo.ApplicationCommandInteractionDataOption
		if len(data.Options) > 0 && len(data.Options[0].Options) > 0 {
			opts = data.Options[0].Options
		}
		h.handleForumCommand(s, i, opts)

	case "add":
		var opts []*discordgo.ApplicationCommandInteractionDataOption
		if len(data.Options) > 0 && len(data.Options[0].Options) > 0 {
			opts = data.Options[0].Options
		}
		h.handleMonAdd(s, i, opts)

	case "remove":
		var opts []*discordgo.ApplicationCommandInteractionDataOption
		if len(data.Options) > 0 && len(data.Options[0].Options) > 0 {
			opts = data.Options[0].Options
		}
		h.handleMonRemove(s, i, opts)

	case "list":
		h.handleMonList(s, i)

	case "games":
		var opts []*discordgo.ApplicationCommandInteractionDataOption
		if len(data.Options) > 0 && len(data.Options[0].Options) > 0 {
			opts = data.Options[0].Options
		}
		h.handleMonGames(s, i, opts)

	case "group":
		var opts []*discordgo.ApplicationCommandInteractionDataOption
		if len(data.Options) > 0 && len(data.Options[0].Options) > 0 {
			opts = data.Options[0].Options
		}
		h.handleMonGroup(s, i, opts)

	case "nodes_channel":
		var opts []*discordgo.ApplicationCommandInteractionDataOption
		if len(data.Options) > 0 && len(data.Options[0].Options) > 0 {
			opts = data.Options[0].Options
		}
		h.handleMonNodesChannel(s, i, opts)
	}
}

func (h *CommandHandler) handleMessageComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID

	if customID == "refresh_nodes_status" {
		lang := "ru"
		if h.cfg != nil && h.cfg.Bot.Language == "en" {
			lang = "en"
		}

		h.mu.Lock()
		now := time.Now()
		if now.Sub(h.lastManual) < 4*time.Second {
			h.mu.Unlock()
			cooldownMsg := "⏳ Пожалуйста, подождите несколько секунд перед повторным обновлением."
			if lang == "en" {
				cooldownMsg = "⏳ Please wait a few seconds before refreshing again."
			}
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: cooldownMsg,
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
		h.lastManual = now
		refreshNodesFn := h.refreshNodesFunc
		h.mu.Unlock()

		h.deferResponse(s, i)

		go func() {
			if refreshNodesFn != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				if err := refreshNodesFn(ctx); err != nil {
					errMsg := fmt.Sprintf("⚠️ Не удалось обновить статус нод: %v", err)
					if lang == "en" {
						errMsg = fmt.Sprintf("⚠️ Failed to refresh nodes status: %v", err)
					}
					h.editResponse(s, i, errMsg)
					return
				}
			}
			successMsg := "✅ Статус выделенных серверов (нод) успешно обновлен!"
			if lang == "en" {
				successMsg = "✅ Dedicated servers (nodes) status refreshed successfully!"
			}
			h.editResponse(s, i, successMsg)
			time.Sleep(4 * time.Second)
			appID := i.Interaction.AppID
			if appID == "" && h.cfg != nil {
				appID = h.cfg.Bot.ApplicationID
			}
			_ = s.WebhookMessageDelete(appID, i.Interaction.Token, "@original")
		}()
		return
	}

	if customID == "refresh_dashboard" {
		h.mu.Lock()
		now := time.Now()
		if now.Sub(h.lastManual) < 4*time.Second {
			h.mu.Unlock()
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "⏳ Пожалуйста, подождите несколько секунд перед повторным обновлением.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}
		h.lastManual = now
		refreshFn := h.refreshFunc
		h.mu.Unlock()

		h.deferResponse(s, i)

		go func() {
			if refreshFn != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := refreshFn(ctx); err != nil {
					if errors.Is(err, ErrGameAPDegraded) {
						h.editResponse(s, i, "⚠️ Панель GameAP временно недоступна, но табло успешно обновлено прямым опросом игровых серверов!")
					} else {
						h.editResponse(s, i, fmt.Sprintf("⚠️ Не удалось обновить табло: %v", err))
						return
					}
				} else {
					h.editResponse(s, i, "✅ Табло мониторинга успешно обновлено!")
				}
			} else {
				h.triggerManualRefresh()
				h.editResponse(s, i, "✅ Табло мониторинга успешно обновлено!")
			}

			time.Sleep(4 * time.Second)
			appID := i.Interaction.AppID
			if appID == "" && h.cfg != nil {
				appID = h.cfg.Bot.ApplicationID
			}
			_ = s.WebhookMessageDelete(appID, i.Interaction.Token, "@original")
		}()
		return
	}

	if strings.HasPrefix(customID, "connect:") {
		h.deferResponse(s, i)

		payload := strings.TrimPrefix(customID, "connect:")
		var serverID uint64
		var fallbackAddr, fallbackGame string
		var fallbackPort int

		if strings.Contains(payload, "|") {
			parts := strings.Split(payload, "|")
			if len(parts) >= 1 {
				serverID, _ = strconv.ParseUint(parts[0], 10, 64)
			}
			if len(parts) >= 3 {
				fallbackAddr = strings.TrimSpace(parts[1])
				fallbackPort, _ = strconv.Atoi(parts[2])
			}
			if len(parts) >= 4 {
				fallbackGame = strings.TrimSpace(parts[3])
			}
		} else if strings.Count(payload, ":") >= 2 {
			parts := strings.Split(payload, ":")
			if len(parts) >= 1 {
				serverID, _ = strconv.ParseUint(parts[0], 10, 64)
			}
			if len(parts) >= 3 {
				fallbackAddr = strings.TrimSpace(parts[1])
				fallbackPort, _ = strconv.Atoi(parts[2])
			}
			if len(parts) >= 4 {
				fallbackGame = strings.TrimSpace(parts[3])
			}
		} else {
			serverID, _ = strconv.ParseUint(payload, 10, 64)
		}

		var targetSrv *gameap.PublicServerDTO

		// 1. Instant lookup in in-memory server cache by ID (0 ms)
		if serverID > 0 {
			if srv, ok := h.GetCachedServer(serverID); ok {
				targetSrv = &srv
			}
		}

		// 2. Lookup in in-memory cache by address and port if ID lookup missed
		if targetSrv == nil && fallbackAddr != "" && fallbackPort > 0 {
			if srv, ok := h.GetCachedServerByAddr(fallbackAddr, fallbackPort); ok {
				targetSrv = &srv
			}
		}

		// 3. Lookup in custom servers from state.json
		if targetSrv == nil && serverID > 0 {
			for _, cs := range h.state.GetCustomServers() {
				if cs.ID == serverID {
					targetSrv = &gameap.PublicServerDTO{
						ID:       cs.ID,
						Name:     cs.Name,
						GameCode: cs.GameCode,
						GameName: cs.GameName,
						Address:  cs.Address,
						Port:     cs.Port,
					}
					break
				}
			}
		}

		// 4. Fail-safe payload reconstruction: if address & port are in button CustomID
		if targetSrv == nil && fallbackAddr != "" && fallbackPort > 0 {
			gameTitle := strings.ToUpper(fallbackGame)
			if g := catalog.GetGame(fallbackGame); g != nil {
				gameTitle = g.Name
			}
			targetSrv = &gameap.PublicServerDTO{
				ID:       serverID,
				Name:     gameTitle,
				GameCode: fallbackGame,
				GameName: gameTitle,
				Address:  fallbackAddr,
				Port:     fallbackPort,
			}
		}

		// 5. Fallback network poll if cache was somehow cold
		if targetSrv == nil && serverID > 0 && h.gameapClient != nil && h.gameapClient.GetBaseURL() != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			resp, _ := h.gameapClient.FetchServers(ctx)
			if resp != nil {
				for _, srv := range resp.Servers {
					if srv.ID == serverID {
						targetSrv = &srv
						break
					}
				}
			}
		}

		if targetSrv == nil {
			h.editResponse(s, i, "❌ Сервер не найден или в данный момент недоступен.")
			return
		}

		// Generate direct connect URL using 297 games catalog
		connectURL := targetSrv.ConnectURL
		if connectURL == "" {
			g := catalog.GetGame(targetSrv.GameCode)
			if g != nil && g.DirectConnect != "" {
				connectURL = strings.ReplaceAll(g.DirectConnect, "{address}", targetSrv.Address)
				connectURL = strings.ReplaceAll(connectURL, "{port}", strconv.Itoa(targetSrv.Port))
			} else if query.ResolveProtocol(targetSrv.GameCode) == query.ProtoA2S || (g != nil && g.SteamAppID > 0) {
				connectURL = fmt.Sprintf("steam://connect/%s:%d", targetSrv.Address, targetSrv.Port)
			}
		}

		var msg strings.Builder
		msg.WriteString(fmt.Sprintf("🎮 **Подключение к серверу:** %s\n", targetSrv.Name))
		msg.WriteString(fmt.Sprintf("📍 **Адрес:** `%s:%d`\n", targetSrv.Address, targetSrv.Port))
		if connectURL != "" {
			msg.WriteString(fmt.Sprintf("🔗 **Прямой запуск игры:** <%s>\n", connectURL))
		}
		msg.WriteString(fmt.Sprintf("📋 **Консольная команда:** `connect %s:%d`\n", targetSrv.Address, targetSrv.Port))

		h.editResponse(s, i, msg.String())
		return
	}
}

func (h *CommandHandler) deferResponse(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
}

func (h *CommandHandler) editResponse(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
}

func (h *CommandHandler) editResponseEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) {
	embeds := []*discordgo.MessageEmbed{embed}
	_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &embeds,
	})
}

func (h *CommandHandler) triggerManualRefresh() {
	select {
	case h.refreshChan <- struct{}{}:
	default:
	}
}

func formatRoleMentions(roleStr string) string {
	if roleStr == "" {
		return "Не назначена (без пинга)"
	}
	parts := strings.FieldsFunc(roleStr, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	})
	var mentions []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			mentions = append(mentions, fmt.Sprintf("<@&%s>", p))
		}
	}
	if len(mentions) == 0 {
		return "Не назначена (без пинга)"
	}
	return strings.Join(mentions, ", ")
}

func (h *CommandHandler) handleAlertsCommand(s *discordgo.Session, i *discordgo.InteractionCreate, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	if len(opts) == 0 {
		// Display current alerts status
		var cfgEnabled bool
		var cfgChannelID, cfgRoleID string
		if h.cfg != nil {
			cfgEnabled = h.cfg.Notifications.Enabled
			cfgChannelID = h.cfg.Notifications.AlertsChannelID
			cfgRoleID = h.cfg.Notifications.GetResolvedAlertRoles()
		}
		enabled, channelID, roleID := h.state.GetAlerts(cfgEnabled, cfgChannelID, cfgRoleID)

		statusText := "🟢 Включены"
		if !enabled {
			statusText = "🔴 Отключены"
		}

		channelText := "Не настроен (используйте `/mon alerts channel:#канал`)"
		if channelID != "" {
			channelText = fmt.Sprintf("<#%s>", channelID)
		}

		roleText := formatRoleMentions(roleID)

		embed := &discordgo.MessageEmbed{
			Title:       "🔔 Настройки системы оповещений (Alerts)",
			Description: "Оповещения отправляются при падении, восстановлении или перезапуске серверов.",
			Color:       0x3498db,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "📊 Статус",
					Value:  statusText,
					Inline: true,
				},
				{
					Name:   "📢 Канал оповещений",
					Value:  channelText,
					Inline: true,
				},
				{
					Name:   "👥 Роли для пинга (при падении)",
					Value:  roleText,
					Inline: false,
				},
				{
					Name:   "💡 Примеры команд",
					Value:  "• `/mon alerts channel:#алерты` — привязать канал\n• `/mon alerts role:@Администраторы` — установить роль для пинга\n• `/mon alerts role:@Администраторы role2:@Модераторы` — пинговать 2 роли сразу\n• `/mon alerts role:none` — отключить пинг ролей\n• `/mon alerts disable:true` — отключить оповещения\n• `/mon alerts enabled:true` — включить оповещения",
					Inline: false,
				},
			},
			Timestamp: time.Now().Format(time.RFC3339),
			Footer: &discordgo.MessageEmbedFooter{
				Text: "GameAP Alerts Management",
			},
		}

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	var newChannel string
	var newRole string
	var newRole2 string
	var enabledPtr *bool

	for _, opt := range opts {
		switch opt.Name {
		case "channel":
			if ch, ok := opt.Value.(string); ok {
				newChannel = strings.TrimSpace(ch)
			}
		case "role":
			if r, ok := opt.Value.(string); ok {
				newRole = strings.TrimSpace(r)
			}
		case "role2":
			if r, ok := opt.Value.(string); ok {
				newRole2 = strings.TrimSpace(r)
			}
		case "enabled":
			if en, ok := opt.Value.(bool); ok {
				enabledPtr = &en
			}
		case "disable":
			if dis, ok := opt.Value.(bool); ok && dis {
				f := false
				enabledPtr = &f
			}
		}
	}

	// If channel was specified but enabled was not explicitly set, turn enabled on by default
	if newChannel != "" && enabledPtr == nil {
		t := true
		enabledPtr = &t
	}

	roleToSet := ""
	if newRole != "" || newRole2 != "" {
		if strings.EqualFold(newRole, "none") || strings.EqualFold(newRole, "clear") {
			roleToSet = "clear"
		} else {
			var roles []string
			if newRole != "" {
				roles = append(roles, newRole)
			}
			if newRole2 != "" {
				roles = append(roles, newRole2)
			}
			roleToSet = strings.Join(roles, ",")
		}
	}

	_ = h.state.SetAlerts(enabledPtr, newChannel, roleToSet)

	var cfgEnabled bool
	var cfgChannelID, cfgRoleID string
	if h.cfg != nil {
		cfgEnabled = h.cfg.Notifications.Enabled
		cfgChannelID = h.cfg.Notifications.AlertsChannelID
		cfgRoleID = h.cfg.Notifications.GetResolvedAlertRoles()
	}
	activeEnabled, activeChannel, activeRole := h.state.GetAlerts(cfgEnabled, cfgChannelID, cfgRoleID)

	h.mu.Lock()
	if h.alertsTracker != nil {
		h.alertsTracker.UpdateAlertsConfig(activeEnabled, activeChannel, activeRole)
	}
	h.mu.Unlock()

	statusText := "🟢 Включены"
	if !activeEnabled {
		statusText = "🔴 Отключены"
	}

	channelText := "Не настроен"
	if activeChannel != "" {
		channelText = fmt.Sprintf("<#%s>", activeChannel)
	}

	roleText := formatRoleMentions(activeRole)

	embed := &discordgo.MessageEmbed{
		Title:       "✅ Настройки оповещений обновлены",
		Color:       0x2ecc71,
		Description: "Параметры успешно сохранены в `state.json` и применены в реальном времени.",
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "📊 Статус",
				Value:  statusText,
				Inline: true,
			},
			{
				Name:   "📢 Канал",
				Value:  channelText,
				Inline: true,
			},
			{
				Name:   "👥 Роли для пинга",
				Value:  roleText,
				Inline: false,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "GameAP Alerts Management",
		},
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}

func (h *CommandHandler) handleForumCommand(s *discordgo.Session, i *discordgo.InteractionCreate, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	if len(opts) == 0 {
		forumCh := h.state.GetForumChannel()
		if forumCh == "" && h.cfg != nil {
			forumCh = h.cfg.Discord.ForumChannelID
		}

		forumText := "Не привязан (используйте `/mon forum channel:#форум`)"
		if forumCh != "" {
			forumText = fmt.Sprintf("<#%s>", forumCh)
		}

		threadIDs := h.state.GetForumThreadIDs()
		var threadLines []string
		for game, thID := range threadIDs {
			threadLines = append(threadLines, fmt.Sprintf("• **%s** ➔ <#%s>", strings.ToUpper(game), thID))
		}
		threadsText := "Пока нет созданных тем"
		if len(threadLines) > 0 {
			threadsText = strings.Join(threadLines, "\n")
		}

		embed := &discordgo.MessageEmbed{
			Title:       "📂 Настройки Forum Channels (Сценарий Б)",
			Description: "В форуме бот создаёт отдельные темы по играм с живыми карточками серверов.",
			Color:       0x9b59b6,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "📢 Форум-канал",
					Value:  forumText,
					Inline: false,
				},
				{
					Name:   "🧵 Активные темы по играм",
					Value:  threadsText,
					Inline: false,
				},
				{
					Name:   "💡 Команды",
					Value:  "• `/mon forum channel:#игровые-серверы` — привязать форум\n• `/mon forum unbind:true` — отвязать форум",
					Inline: false,
				},
			},
			Timestamp: time.Now().Format(time.RFC3339),
			Footer: &discordgo.MessageEmbedFooter{
				Text: "GameAP Forum Integration",
			},
		}

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	var targetChannel string
	var unbind bool

	for _, opt := range opts {
		switch opt.Name {
		case "channel":
			if ch, ok := opt.Value.(string); ok {
				targetChannel = strings.TrimSpace(ch)
			}
		case "unbind":
			if un, ok := opt.Value.(bool); ok {
				unbind = un
			}
		}
	}

	if unbind {
		_ = h.state.SetForumChannel("")
		h.triggerManualRefresh()
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "🗑️ Привязка форума успешно удалена.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if targetChannel == "" {
		targetChannel = i.ChannelID
	}

	_ = h.state.SetForumChannel(targetChannel)
	h.triggerManualRefresh()

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("✅ Форум-канал успешно привязан: <#%s>!\nБот автоматически создаст темы по играм при ближайшем обновлении.", targetChannel),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func (h *CommandHandler) handleMonAdd(s *discordgo.Session, i *discordgo.InteractionCreate, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	h.deferResponse(s, i)

	var addressStr, gameStr, customName, customIcon string
	targetChannelID := i.ChannelID
	for _, opt := range opts {
		switch opt.Name {
		case "address":
			if str, ok := opt.Value.(string); ok {
				addressStr = strings.TrimSpace(str)
			}
		case "game":
			if str, ok := opt.Value.(string); ok {
				gameStr = strings.TrimSpace(str)
			}
		case "name":
			if str, ok := opt.Value.(string); ok {
				customName = strings.TrimSpace(str)
			}
		case "channel":
			if ch, ok := opt.Value.(string); ok && ch != "" {
				targetChannelID = ch
			}
		case "icon":
			if str, ok := opt.Value.(string); ok {
				customIcon = strings.TrimSpace(str)
			}
		}
	}

	if addressStr == "" || gameStr == "" {
		h.editResponse(s, i, "❌ Укажите обязательные параметры: `address` (IP:PORT) и `game` (код или название игры).")
		return
	}

	host, portStr, err := net.SplitHostPort(addressStr)
	if err != nil {
		h.editResponse(s, i, fmt.Sprintf("❌ Неверный формат адреса `%s`. Ожидается `IP:PORT` (например `127.0.0.1:7777` или `mc.example.com:25565`).", addressStr))
		return
	}
	host = strings.ToLower(strings.TrimSpace(host))
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		h.editResponse(s, i, fmt.Sprintf("❌ Неверный номер порта `%s`. Допустимый диапазон: 1–65535.", portStr))
		return
	}

	if customIcon != "" {
		parsedIcon, iconErr := url.ParseRequestURI(customIcon)
		scheme := ""
		if parsedIcon != nil {
			scheme = strings.ToLower(parsedIcon.Scheme)
		}
		if iconErr != nil || (scheme != "http" && scheme != "https") || parsedIcon.Host == "" {
			h.editResponse(s, i, "❌ Некорректный URL иконки (`icon`). Используйте ссылку с протоколом `http://` или `https://`.")
			return
		}
	}

	timeout := 2500 * time.Millisecond
	info, qErr := query.QueryServerAuto(gameStr, host, port, timeout)

	serverName := customName
	statusStr := "🔴 Недоступен (Offline при проверке)"
	playersStr := "—"
	mapStr := "—"

	if qErr == nil && info != nil {
		statusStr = "🟢 Доступен (Online)"
		if serverName == "" {
			serverName = info.Name
		}
		playersStr = fmt.Sprintf("%d / %d", info.Players, info.MaxPlayers)
		if info.Map != "" {
			mapStr = info.Map
		}
	}
	if serverName == "" {
		serverName = fmt.Sprintf("%s:%d", host, port)
	}

	gameName := strings.ToUpper(gameStr)
	cleanCode := strings.ToLower(gameStr)
	if g := catalog.GetGame(gameStr); g != nil {
		gameName = g.Name
		cleanCode = g.ID
		if customIcon == "" && g.Banner != "" {
			customIcon = g.Banner
		}
	} else if info != nil && info.Game != "" {
		gameName = info.Game
	}

	cs := CustomServer{
		ID:       generateCustomServerID(fmt.Sprintf("%s:%d", host, port)),
		Name:     serverName,
		Address:  host,
		Port:     port,
		GameCode: cleanCode,
		GameName: gameName,
		IconURL:  customIcon,
	}

	if err := h.state.AddCustomServer(cs); err != nil {
		h.editResponse(s, i, fmt.Sprintf("❌ Ошибка сохранения сервера в state.json: %v", err))
		return
	}

	// Automatically bind server card to the target channel so it publishes immediately
	_ = h.state.BindServerChannel(i.GuildID, cs.ID, targetChannelID)

	// Trigger immediate refresh to publish card to targetChannelID
	h.mu.Lock()
	refreshFn := h.refreshFunc
	h.mu.Unlock()

	if refreshFn != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = refreshFn(ctx)
		}()
	} else {
		h.triggerManualRefresh()
	}

	permWarning := checkChannelPermissions(s, targetChannelID)
	embedDesc := fmt.Sprintf("Сервер **%s** успешно зарегистрирован и опубликован в канале <#%s>!", serverName, targetChannelID)
	embedColor := 0x00FF88
	if permWarning != "" {
		embedDesc = fmt.Sprintf("Сервер **%s** успешно зарегистрирован, но **не может быть опубликован** в канале <#%s>:\n\n%s", serverName, targetChannelID, permWarning)
		embedColor = 0xFFA500
	}

	embed := &discordgo.MessageEmbed{
		Title:       "✅ Внешний игровой сервер добавлен",
		Description: embedDesc,
		Color:       embedColor,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "🌐 Адрес", Value: fmt.Sprintf("`%s:%d`", host, port), Inline: true},
			{Name: "🎮 Игра", Value: fmt.Sprintf("`%s`", gameName), Inline: true},
			{Name: "📡 Статус опроса", Value: statusStr, Inline: true},
			{Name: "👥 Игроки", Value: playersStr, Inline: true},
			{Name: "🗺️ Карта", Value: mapStr, Inline: true},
			{Name: "🆔 Системный ID", Value: fmt.Sprintf("`%d`", cs.ID), Inline: true},
			{Name: "📢 Канал табло", Value: fmt.Sprintf("<#%s>", targetChannelID), Inline: false},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Табло серверов обновляется автоматически каждые 30 секунд",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	thumbURL := customIcon
	if thumbURL == "" {
		thumbURL = GetGameThumbnailURL(cs.GameCode)
	}
	if thumbURL == "" {
		thumbURL = GetGameThumbnailURL(cs.GameName)
	}
	if thumbURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: thumbURL,
		}
	}

	h.editResponseEmbed(s, i, embed)
}

func (h *CommandHandler) handleMonRemove(s *discordgo.Session, i *discordgo.InteractionCreate, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	h.deferResponse(s, i)

	var addressStr string
	for _, opt := range opts {
		if opt.Name == "address" {
			if str, ok := opt.Value.(string); ok {
				addressStr = strings.TrimSpace(str)
			}
		}
	}

	if addressStr == "" {
		h.editResponse(s, i, "❌ Укажите адрес сервера для удаления (IP:PORT).")
		return
	}

	var removedID uint64
	if cs, found := h.state.GetCustomServer(addressStr); found {
		removedID = cs.ID
	}

	if !h.state.RemoveCustomServer(addressStr) {
		h.editResponse(s, i, fmt.Sprintf("❌ Сервер с адресом `%s` не найден в списке внешних серверов.", addressStr))
		return
	}
	if removedID != 0 {
		_ = h.state.UnbindServerChannel(removedID)
	}

	h.mu.Lock()
	refreshFn := h.refreshFunc
	h.mu.Unlock()

	if refreshFn != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = refreshFn(ctx)
		}()
	} else {
		h.triggerManualRefresh()
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🗑️ Внешний сервер удалён",
		Description: fmt.Sprintf("Сервер `%s` успешно удалён из мониторинга.", addressStr),
		Color:       0xFFA500,
	}
	h.editResponseEmbed(s, i, embed)
}

func (h *CommandHandler) handleMonList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	h.deferResponse(s, i)

	servers := h.state.GetCustomServers()
	if len(servers) == 0 {
		h.editResponse(s, i, "ℹ️ Внешние серверы пока не добавлены. Используйте команду `/mon add address:<ip:port> game:<game>`.")
		return
	}

	var sb strings.Builder
	for idx, srv := range servers {
		entry := fmt.Sprintf("**%d. %s** (ID: `%d`)\n   • Адрес: `%s:%d`\n   • Игра: `%s` (`%s`)\n\n",
			idx+1, srv.Name, srv.ID, srv.Address, srv.Port, srv.GameName, srv.GameCode)
		if sb.Len()+len(entry) > 3950 {
			sb.WriteString(fmt.Sprintf("...и ещё %d серверов", len(servers)-idx))
			break
		}
		sb.WriteString(entry)
	}

	desc := truncateEmbedDescription(sb.String(), 4000, "\n...")
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📋 Список внешних серверов (%d)", len(servers)),
		Description: desc,
		Color:       0x5865F2,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Удалить сервер: /mon remove address:<ip:port>",
		},
	}

	h.editResponseEmbed(s, i, embed)
}

func checkChannelPermissions(s *discordgo.Session, channelID string) string {
	if s == nil || channelID == "" {
		return ""
	}
	botUserID := ""
	if s.State != nil && s.State.User != nil {
		botUserID = s.State.User.ID
	}
	if botUserID == "" {
		return ""
	}
	perms, err := s.UserChannelPermissions(botUserID, channelID)
	if err != nil {
		return ""
	}
	var missing []string
	if (perms & discordgo.PermissionViewChannel) == 0 {
		missing = append(missing, "Просмотр канала (View Channel)")
	}
	if (perms & discordgo.PermissionSendMessages) == 0 {
		missing = append(missing, "Отправка сообщений (Send Messages)")
	}
	if (perms & discordgo.PermissionEmbedLinks) == 0 {
		missing = append(missing, "Встраивание ссылок (Embed Links)")
	}
	if len(missing) > 0 {
		return fmt.Sprintf("⚠️ **Внимание: у бота отсутствуют необходимые права в канале <#%s>:**\n• %s\n\n👉 Добавьте эти права для роли бота в настройках канала (*Настройки канала -> Права доступа*), чтобы карточка смогла опубликоваться.", channelID, strings.Join(missing, "\n• "))
	}
	return ""
}

func (h *CommandHandler) isServerAdmin(i *discordgo.InteractionCreate) bool {
	if i.Member == nil {
		return false
	}
	adminPerms := int64(discordgo.PermissionAdministrator | discordgo.PermissionManageServer)
	return (i.Member.Permissions & adminPerms) != 0
}

func (h *CommandHandler) isAuthorized(i *discordgo.InteractionCreate) bool {
	if i.Member == nil {
		return false
	}

	// 1. Server Administrators & Managers have universal access
	if h.isServerAdmin(i) {
		return true
	}

	// 2. Check assigned manager roles (both from state.json and config.json)
	allowedRoles := make(map[string]bool)
	for _, rid := range h.state.GetManagerRoles() {
		allowedRoles[rid] = true
	}
	if h.cfg != nil {
		for _, rid := range h.cfg.Discord.ManagerRoleIDs {
			allowedRoles[rid] = true
		}
	}

	for _, userRole := range i.Member.Roles {
		if allowedRoles[userRole] {
			return true
		}
	}

	return false
}

var httpStatusRegexp = regexp.MustCompile(`HTTP (\d{3})`)

// validateSetupURL validates that the GameAP setup URL uses http/https and does not target local/private/metadata addresses.
func validateSetupURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return errors.New("не указан обязательный параметр url")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("неверный формат URL")
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return errors.New("поддерживаются только схемы http:// и https://")
	}

	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return errors.New("отсутствует хост в URL")
	}

	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".localhost") {
		return errors.New("запрещено использовать localhost")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isDisallowedSetupIP(ip) {
			return errors.New("запрещён доступ к локальным, приватным или служебным IP-адресам")
		}
		return nil
	}

	if ips, lookupErr := net.LookupIP(host); lookupErr == nil {
		for _, ip := range ips {
			if isDisallowedSetupIP(ip) {
				return errors.New("домен разрешается в локальный, приватный или служебный IP-адрес")
			}
		}
	}

	return nil
}

func isDisallowedSetupIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// Reject CGNAT 100.64.0.0/10
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
	}
	return false
}

// sanitizeSetupError maps GameAP client errors to safe user-facing messages without leaking raw HTTP response bodies.
func sanitizeSetupError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "превышен таймаут ожидания ответа"
	}

	errStr := err.Error()
	if strings.Contains(errStr, "HTTP 401") {
		return "ошибка авторизации (HTTP 401): неверный или отсутствующий токен"
	}
	if strings.Contains(errStr, "HTTP 403") {
		return "доступ запрещён (HTTP 403): Bot API отключён или недостаточно прав"
	}
	if matches := httpStatusRegexp.FindStringSubmatch(errStr); len(matches) == 2 {
		return fmt.Sprintf("сервер вернул ошибку (HTTP %s)", matches[1])
	}
	if strings.Contains(errStr, "unmarshaling") {
		return "сервер вернул некорректный JSON-ответ"
	}
	return "не удалось подключиться к указанному адресу"
}

func (h *CommandHandler) handleMonRoles(s *discordgo.Session, i *discordgo.InteractionCreate, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	var addRoleID, removeRoleID string
	for _, opt := range opts {
		switch opt.Name {
		case "add":
			if r, ok := opt.Value.(string); ok {
				addRoleID = strings.TrimSpace(r)
			}
		case "remove":
			if r, ok := opt.Value.(string); ok {
				removeRoleID = strings.TrimSpace(r)
			}
		}
	}

	if addRoleID != "" || removeRoleID != "" {
		if !h.isServerAdmin(i) {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "❌ Только **Администраторы сервера** (права Администратор / Управление сервером) могут добавлять или удалять роли менеджеров бота.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		if addRoleID != "" {
			_ = h.state.AddManagerRole(addRoleID)
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("✅ Роль <@&%s> успешно добавлена в список менеджеров бота!\nУчастники с этой ролью теперь могут использовать все команды управления `/mon`.", addRoleID),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		if removeRoleID != "" {
			if h.state.RemoveManagerRole(removeRoleID) {
				_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("🗑️ Роль <@&%s> успешно удалена из списка менеджеров бота.", removeRoleID),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			} else {
				_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("ℹ️ Роль <@&%s> не была найдена в списке менеджеров.", removeRoleID),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			}
			return
		}
	}

	// Default: list all manager roles
	roles := h.state.GetManagerRoles()
	var configRoles []string
	if h.cfg != nil {
		configRoles = h.cfg.Discord.ManagerRoleIDs
	}

	var sb strings.Builder
	sb.WriteString("👑 **Права доступа к управлению мониторингом (/mon):**\n\n")
	sb.WriteString("• **Администраторы сервера** (*Administrator* / *Manage Server*) — полный доступ.\n")

	allRolesMap := make(map[string]bool)
	for _, r := range roles {
		allRolesMap[r] = true
	}
	for _, r := range configRoles {
		allRolesMap[r] = true
	}

	if len(allRolesMap) == 0 {
		sb.WriteString("• **Назначенные роли менеджеров:** *(пока не назначены)*\n\n")
		sb.WriteString("💡 *Чтобы выдать доступ модераторам или конкретной роли:* `/mon roles add:@Роль`")
	} else {
		sb.WriteString("• **Назначенные роли менеджеров:**\n")
		for r := range allRolesMap {
			sb.WriteString(fmt.Sprintf("  - <@&%s> (`%s`)\n", r, r))
		}
		sb.WriteString("\n*Удалить роль:* `/mon roles remove:@Роль`")
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: sb.String(),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func (h *CommandHandler) handleMonGames(s *discordgo.Session, i *discordgo.InteractionCreate, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	searchQuery := ""
	for _, opt := range opts {
		if opt.Name == "search" {
			if str, ok := opt.Value.(string); ok {
				searchQuery = strings.TrimSpace(str)
			}
		}
	}

	if searchQuery == "" {
		embed := &discordgo.MessageEmbed{
			Title:       "🎮 Справочник поддерживаемых игр (297 игр)",
			Description: "Бот полностью совместим со стандартом **DiscordGSM** и поддерживает **297 сетевых игр** с автоопределением протокола (A2S, Minecraft Java, Bedrock, SA-MP, Quake, FiveM) и смещений портов.",
			Color:       0x3498db,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "📖 Полный онлайн-справочник кодов игр",
					Value:  "[🔗 Открыть SUPPORTED_GAMES.md на GitHub](https://github.com/br1ckzzz/GameAP_WebMonBot/blob/main/SUPPORTED_GAMES.md)\n*Полный алфавитный перечень всех 297 игр с системными кодами и примерами команд.*",
					Inline: false,
				},
				{
					Name:   "🔍 Быстрый поиск в Discord",
					Value:  "Используйте команду с параметром поиска:\n• `/mon games search:cs2`\n• `/mon games search:valheim`\n• `/mon games search:quake`\n• `/mon games search:minecraft`",
					Inline: false,
				},
				{
					Name:   "🌐 Публичный узел ноды (Catalog API)",
					Value:  "`GET /api/v1/games` *(JSON-манифест схемы 1.0.0)*",
					Inline: false,
				},
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: "GameAP DiscordGSM Catalog Node",
			},
			Timestamp: time.Now().Format(time.RFC3339),
		}

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	matches := catalog.SearchGames(searchQuery, 8)
	if len(matches) == 0 {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("❌ По запросу **%s** игр не найдено.\n📖 Полный список из 297 игр смотрите в [SUPPORTED_GAMES.md](https://github.com/br1ckzzz/GameAP_WebMonBot/blob/main/SUPPORTED_GAMES.md).", searchQuery),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🔍 Результаты поиска: «%s» (найдено: %d)", searchQuery, len(matches)),
		Description: "Код игры можно использовать в командах `/mon add` и `/mon bind`:",
		Color:       0x2ecc71,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Для полного списка: /mon games или SUPPORTED_GAMES.md",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	for _, g := range matches {
		var meta []string
		meta = append(meta, fmt.Sprintf("Код: `%s`", g.ID))
		meta = append(meta, fmt.Sprintf("Протокол: `%s`", g.Protocol))
		if g.PortOffset > 0 {
			meta = append(meta, fmt.Sprintf("Смещение: `+%d`", g.PortOffset))
		}
		if g.DefaultPort > 0 {
			meta = append(meta, fmt.Sprintf("Порт: `%d`", g.DefaultPort))
		}
		if g.SteamAppID > 0 {
			meta = append(meta, fmt.Sprintf("Steam AppID: `%d`", g.SteamAppID))
		}

		exampleCmd := fmt.Sprintf("`/mon add address:IP:%d game:%s`", g.DefaultPort, g.ID)
		val := fmt.Sprintf("> %s\n> 💡 Пример: %s", strings.Join(meta, " • "), exampleCmd)

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("🎮 %s", g.Name),
			Value:  val,
			Inline: false,
		})
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}

func (h *CommandHandler) handleMonGroup(s *discordgo.Session, i *discordgo.InteractionCreate, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	h.deferResponse(s, i)

	gameArg := ""
	targetChannelID := i.ChannelID

	for _, opt := range opts {
		if opt.Name == "game" {
			if str, ok := opt.Value.(string); ok {
				gameArg = strings.TrimSpace(str)
			}
		} else if opt.Name == "channel" {
			if ch, ok := opt.Value.(string); ok && ch != "" {
				targetChannelID = ch
			}
		}
	}

	if gameArg == "" {
		h.editResponse(s, i, "❌ Укажите игру для группового табло (например: `/mon group game:cs2`).")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	var allServers []gameap.PublicServerDTO
	if h.gameapClient != nil && h.gameapClient.GetBaseURL() != "" {
		resp, _ := h.gameapClient.FetchServers(ctx)
		if resp != nil {
			allServers = resp.Servers
		}
	}

	// Also add matching custom servers
	for _, cs := range h.state.GetCustomServers() {
		allServers = append(allServers, gameap.PublicServerDTO{
			ID:       cs.ID,
			Name:     cs.Name,
			GameCode: cs.GameCode,
			GameName: cs.GameName,
			Address:  cs.Address,
			Port:     cs.Port,
			Status:   "online",
			IconURL:  cs.IconURL,
		})
	}

	normTarget := catalog.GetGame(gameArg)
	targetCode := strings.ToLower(gameArg)
	if normTarget != nil {
		targetCode = normTarget.ID
	}

	var matched []gameap.PublicServerDTO
	var titleGameName string
	for _, srv := range allServers {
		srvCode := strings.ToLower(srv.GameCode)
		if g := catalog.GetGame(srv.GameCode); g != nil {
			srvCode = g.ID
		}
		if srvCode == targetCode || strings.EqualFold(srv.GameName, gameArg) {
			matched = append(matched, srv)
			if titleGameName == "" {
				titleGameName = srv.GameName
			}
		}
	}

	if len(matched) == 0 {
		h.editResponse(s, i, fmt.Sprintf("ℹ️ Серверов для игры **%s** в мониторинге не обнаружено.", gameArg))
		return
	}

	if titleGameName == "" {
		if normTarget != nil {
			titleGameName = normTarget.Name
		} else {
			titleGameName = strings.ToUpper(gameArg)
		}
	}

	webURL := ""
	if h.gameapClient != nil {
		webURL = h.gameapClient.GetBaseURL()
	}
	embed := RenderGameGroupEmbed(titleGameName, matched, h.cfg, webURL)
	buttons := BuildServerButtons(matched)

	msg, err := s.ChannelMessageSendComplex(targetChannelID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: buttons,
	})
	if err != nil {
		h.editResponse(s, i, fmt.Sprintf("⚠️ Ошибка отправки табло в канал <#%s>: %v", targetChannelID, err))
		return
	}

	key := fmt.Sprintf("game_group_%s_%s", targetCode, targetChannelID)
	_ = h.state.SetMessageID(key, msg.ID)

	h.editResponse(s, i, fmt.Sprintf("✅ Групповое табло для **%s** успешно опубликовано в канале <#%s>!", titleGameName, targetChannelID))
}

func (h *CommandHandler) handleMonNodesChannel(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
	unbind := false
	targetChannelID := i.ChannelID

	for _, opt := range options {
		if opt.Name == "unbind" {
			if b, ok := opt.Value.(bool); ok {
				unbind = b
			}
		} else if opt.Name == "channel" {
			if ch, ok := opt.Value.(string); ok && ch != "" {
				targetChannelID = ch
			}
		}
	}

	if unbind {
		if err := h.state.SetNodesChannelID("clear"); err != nil {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("❌ Ошибка при отвязке канала нод: %v", err),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "🗑️ Канал мониторинга выделенных серверов (нод) успешно отвязан.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if err := h.state.SetNodesChannelID(targetChannelID); err != nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("❌ Ошибка при привязке канала нод: %v", err),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	msgText := fmt.Sprintf("✅ Канал для статуса выделенных серверов (нод) успешно привязан к <#%s>!\nКарточка формируется и публикуется прямо сейчас...", targetChannelID)
	if warning := checkChannelPermissions(s, targetChannelID); warning != "" {
		msgText += "\n\n" + warning
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msgText,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	h.mu.Lock()
	refreshNodesFn := h.refreshNodesFunc
	h.mu.Unlock()
	if refreshNodesFn != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = refreshNodesFn(ctx)
		}()
	}
}

