package bot

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"

	"gameap-webmon-bot/internal/alerts"
	"gameap-webmon-bot/internal/config"
	"gameap-webmon-bot/internal/gameap"
	"gameap-webmon-bot/internal/query"
)

// Bot coordinates Discord gateway events, dashboard message updates, and alert monitoring.
type Bot struct {
	mu            sync.Mutex
	cfg           *config.Config
	session       *discordgo.Session
	gameapClient  *gameap.Client
	state         *State
	alerts        *alerts.Tracker
	cmdHandler    *CommandHandler
	refreshChan   chan struct{}
	pollErrCount  int
	lastPollErr   string
	queryFailures map[uint64]int
}

// NewBot constructs a configured Bot instance.
func NewBot(cfg *config.Config, stateFile string) (*Bot, error) {
	if cfg.Bot.Token == "" {
		return nil, fmt.Errorf("bot token is required (specify in config.json or DISCORD_BOT_TOKEN)")
	}

	session, err := discordgo.New("Bot " + strings.TrimSpace(cfg.Bot.Token))
	if err != nil {
		return nil, fmt.Errorf("creating discord session: %w", err)
	}

	if cfg.Bot.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.Bot.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("parsing proxy_url %q: %w", cfg.Bot.ProxyURL, err)
		}
		log.Printf("[DISCORD] Using proxy for Discord API & Gateway: %s (%s)", proxyURL.Redacted(), proxyURL.Scheme)

		// Configure HTTP client for Discord REST API requests
		session.Client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}

		// Configure WebSocket Dialer for Discord Gateway connection
		if session.Dialer == nil {
			session.Dialer = &websocket.Dialer{
				HandshakeTimeout: 45 * time.Second,
			}
		}
		session.Dialer.Proxy = http.ProxyURL(proxyURL)
	}

	state := LoadState(stateFile)
	gameapClient := gameap.NewClient(cfg.GameAP.BaseURL, cfg.GameAP.APIToken, time.Duration(cfg.GameAP.TimeoutSeconds)*time.Second)
	if cfg.GameAP.BaseURL == "" && state.BaseURL != "" {
		gameapClient.UpdateCredentials(state.BaseURL, state.APIToken)
	} else if cfg.GameAP.BaseURL != "" {
		_ = state.SetCredentials(cfg.GameAP.BaseURL, cfg.GameAP.APIToken)
	}

	refreshChan := make(chan struct{}, 1)
	cmdHandler := NewCommandHandler(cfg, gameapClient, state, refreshChan)
	alertsTracker := alerts.NewTracker(cfg, session)

	// Apply state overrides for alerts if previously configured via Discord
	activeAlertsEnabled, activeAlertsCh, activeAlertsRole := state.GetAlerts(cfg.Notifications.Enabled, cfg.Notifications.AlertsChannelID, cfg.Notifications.GetResolvedAlertRoles())
	alertsTracker.UpdateAlertsConfig(activeAlertsEnabled, activeAlertsCh, activeAlertsRole)

	cmdHandler.SetAlertsTracker(alertsTracker)

	b := &Bot{
		cfg:           cfg,
		session:       session,
		gameapClient:  gameapClient,
		state:         state,
		alerts:        alertsTracker,
		cmdHandler:    cmdHandler,
		refreshChan:   refreshChan,
		queryFailures: make(map[uint64]int),
	}

	cmdHandler.SetRefreshFunc(b.RefreshDashboard)
	cmdHandler.SetRefreshNodesFunc(b.RefreshNodesStatus)
	cmdHandler.SetPurgeFunc(func(channelID string) (int, error) {
		return b.PurgeChannelBotMessages(channelID, 50)
	})

	// Register event handlers
	session.AddHandler(b.onReady)
	session.AddHandler(cmdHandler.HandleInteraction)

	return b, nil
}

func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Printf("[DISCORD] Logged in as %s#%s (ID: %s)", r.User.Username, r.User.Discriminator, r.User.ID)

	// Set bot presence
	presence := b.cfg.Bot.StatusPresence
	if presence == "" {
		presence = "GameAP Monitoring"
	}
	_ = s.UpdateGameStatus(0, presence)

	// Register slash commands
	appID := b.cfg.Bot.ApplicationID
	if appID == "" {
		appID = r.User.ID
	}
	guildID := b.cfg.Discord.GuildID

	commands := b.cmdHandler.GetSlashCommands()

	// If a specific guild is configured, restrict command registration strictly to that guild
	if guildID != "" {
		for _, cmd := range commands {
			_, err := s.ApplicationCommandCreate(appID, guildID, cmd)
			if err != nil {
				log.Printf("[WARN] Failed to register slash command /%s in guild %s: %v", cmd.Name, guildID, err)
			} else {
				log.Printf("[INFO] Registered slash command /%s in target guild %s", cmd.Name, guildID)
			}
		}
		log.Printf("[SECURITY] Bot locked to guild %s. Skipped other guild and global command registration.", guildID)
		return
	}

	// Otherwise (multi-server mode), register for every connected guild and globally
	for _, g := range r.Guilds {
		for _, cmd := range commands {
			_, err := s.ApplicationCommandCreate(appID, g.ID, cmd)
			if err == nil {
				log.Printf("[INFO] Registered slash command /%s in guild %s", cmd.Name, g.ID)
			}
		}
	}

	for _, cmd := range commands {
		_, _ = s.ApplicationCommandCreate(appID, "", cmd)
	}
}

// Start opens the Discord WebSocket connection and begins the monitoring loop.
func (b *Bot) Start(ctx context.Context) error {
	log.Printf("[INFO] Connecting to Discord Gateway...")
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("opening discord session: %w", err)
	}

	// Trigger first checks immediately
	go b.pollServers(ctx)
	go b.pollNodes(ctx)

	// Polling worker loop
	interval := time.Duration(b.cfg.Bot.UpdateIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	nodesInterval := time.Duration(b.cfg.Discord.NodesUpdateIntervalSeconds) * time.Second
	if nodesInterval <= 0 {
		nodesInterval = 300 * time.Second
	}
	nodesTicker := time.NewTicker(nodesInterval)
	defer nodesTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[INFO] Monitoring worker shutting down...")
			return nil
		case <-ticker.C:
			b.pollServers(ctx)
		case <-nodesTicker.C:
			b.pollNodes(ctx)
		case <-b.refreshChan:
			log.Printf("[INFO] Immediate dashboard refresh triggered.")
			b.pollServers(ctx)
		}
	}
}

// Stop cleanly disconnects from Discord and persists state.
func (b *Bot) Stop() error {
	_ = b.state.Save()
	if b.session != nil {
		return b.session.Close()
	}
	return nil
}

// queryCustomServers polls external custom servers configured in state.
func (b *Bot) queryCustomServers(timeout time.Duration) []gameap.PublicServerDTO {
	customServers := b.state.GetCustomServers()
	if len(customServers) == 0 {
		return nil
	}

	results := make([]gameap.PublicServerDTO, len(customServers))
	var wg sync.WaitGroup

	for i, cs := range customServers {
		wg.Add(1)
		go func(idx int, srv CustomServer) {
			defer wg.Done()

			info, err := query.QueryServerAuto(srv.GameCode, srv.Address, srv.Port, timeout)
			status := "offline"
			players := 0
			maxPlayers := 0
			mapName := "—"
			displayName := srv.Name
			if displayName == "" {
				displayName = fmt.Sprintf("%s:%d", srv.Address, srv.Port)
			}

			if err == nil && info != nil {
				status = "online"
				players = info.Players
				maxPlayers = info.MaxPlayers
				if info.Map != "" {
					mapName = info.Map
				}
				if srv.Name == "" && info.Name != "" {
					displayName = info.Name
				}
			}

			connectURL := ""
			proto := query.ResolveProtocol(srv.GameCode)
			if proto == query.ProtoA2S {
				connectURL = fmt.Sprintf("steam://connect/%s:%d", srv.Address, srv.Port)
			}

			results[idx] = gameap.PublicServerDTO{
				ID:         srv.ID,
				Name:       displayName,
				GameCode:   srv.GameCode,
				GameName:   srv.GameName,
				Address:    srv.Address,
				Port:       srv.Port,
				Status:     status,
				Players:    players,
				MaxPlayers: maxPlayers,
				Map:        mapName,
				ConnectURL: connectURL,
				IconURL:    srv.IconURL,
			}
		}(i, cs)
	}

	wg.Wait()
	return results
}

// enrichServersWithHealthcheck performs direct protocol probes on GameAP servers.
// If a server has 2 consecutive query failures, it is marked offline even if GameAP reports active.
func (b *Bot) enrichServersWithHealthcheck(servers []gameap.PublicServerDTO, timeout time.Duration) []gameap.PublicServerDTO {
	if len(servers) == 0 {
		return servers
	}

	res := make([]gameap.PublicServerDTO, len(servers))
	copy(res, servers)

	var wg sync.WaitGroup
	for i := range res {
		srv := &res[i]
		if strings.ToLower(srv.Status) != "online" {
			continue
		}

		wg.Add(1)
		go func(s *gameap.PublicServerDTO) {
			defer wg.Done()

			info, err := query.QueryServerAuto(s.GameCode, s.Address, s.Port, timeout)
			b.mu.Lock()
			defer b.mu.Unlock()

			if err == nil && info != nil {
				b.queryFailures[s.ID] = 0
				s.Players = info.Players
				s.MaxPlayers = info.MaxPlayers
				if info.Map != "" {
					s.Map = info.Map
				}
				if s.Name == "" && info.Name != "" {
					s.Name = info.Name
				}
			} else {
				b.queryFailures[s.ID]++
				// If 2 or more consecutive probe failures, mark as offline (dead node or crashed server process)
				if b.queryFailures[s.ID] >= 2 {
					s.Status = "offline"
				}
			}
		}(srv)
	}

	wg.Wait()
	return res
}

// RefreshDashboard queries the GameAP API, checks server alerts, and updates Discord messages.
func (b *Bot) RefreshDashboard(ctx context.Context) error {
	pollCtx, cancel := context.WithTimeout(ctx, time.Duration(b.cfg.GameAP.TimeoutSeconds)*time.Second)
	defer cancel()

	var servers []gameap.PublicServerDTO
	var gameapErr error

	if b.gameapClient != nil && b.cfg.GameAP.BaseURL != "" {
		resp, err := b.gameapClient.FetchServers(pollCtx)
		if err != nil && pollCtx.Err() == nil {
			// Transient DB lock / micro-glitch: wait 2 seconds and retry once
			select {
			case <-time.After(2 * time.Second):
				if pollCtx.Err() == nil {
					resp, err = b.gameapClient.FetchServers(pollCtx)
				}
			case <-pollCtx.Done():
			}
		}
		if err == nil && resp != nil {
			servers = resp.Servers
		} else {
			gameapErr = err
		}
	}

	timeout := time.Duration(b.cfg.GameAP.DirectQueryTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 3000 * time.Millisecond
	}

	if b.cfg.GameAP.DirectQueryEnabled && len(servers) > 0 {
		servers = b.enrichServersWithHealthcheck(servers, timeout)
	}

	// Merge with external custom servers
	customDTOs := b.queryCustomServers(timeout)
	if len(customDTOs) > 0 {
		servers = append(servers, customDTOs...)
	}

	gameapAvailable := (gameapErr == nil)

	if gameapErr != nil {
		if len(servers) == 0 {
			return fmt.Errorf("GameAP API: %w", gameapErr)
		}
		log.Printf("[WARN] GameAP API unreachable (%v), continuing with %d custom servers in standalone mode", gameapErr, len(servers))
	}

	// Push enriched telemetry back to GameAP WebMonitoring plugin if available and healthy
	if gameapAvailable && b.gameapClient != nil && b.cfg.GameAP.BaseURL != "" && len(servers) > 0 {
		_ = b.gameapClient.PushTelemetry(pollCtx, servers)
	}

	// Update in-memory server snapshot for instantaneous (0 ms) button interactions
	if b.cmdHandler != nil && len(servers) > 0 {
		b.cmdHandler.UpdateCachedServers(servers)
	}

	// Dispatch to alerting engine
	b.alerts.ProcessUpdates(servers)

	// Update Dashboard in designated or dynamically bound channel
	b.updateDashboard(servers, gameapAvailable)
	return nil
}

func (b *Bot) pollServers(ctx context.Context) {
	if err := b.RefreshDashboard(ctx); err != nil {
		b.mu.Lock()
		b.pollErrCount++
		count := b.pollErrCount
		isFirst := b.lastPollErr == ""
		b.lastPollErr = err.Error()
		b.mu.Unlock()

		if isFirst {
			log.Printf("[WARN] %v (subsequent polling errors suppressed until connection recovers)", err)
		} else if count%10 == 0 {
			log.Printf("[WARN] GameAP API is still unreachable (%d consecutive failures): %v", count, err)
		}
	} else {
		b.mu.Lock()
		wasFailing := b.lastPollErr != ""
		count := b.pollErrCount
		b.lastPollErr = ""
		b.pollErrCount = 0
		b.mu.Unlock()

		if wasFailing {
			log.Printf("[INFO] GameAP API connection restored successfully after %d failed attempts", count)
		}
	}
}

func (b *Bot) getChannelForGame(gameName, gameCode string) string {
	nameKey := normalizeGameKey(gameName)
	codeKey := normalizeGameKey(gameCode)

	// 1. Check dynamic state
	if ch := b.state.GetGameChannel(codeKey); ch != "" {
		return ch
	}
	if ch := b.state.GetGameChannel(nameKey); ch != "" {
		return ch
	}

	// 2. Check config.json
	if b.cfg.Discord.GameChannels != nil {
		if ch := b.cfg.Discord.GameChannels[codeKey]; ch != "" {
			return ch
		}
		if ch := b.cfg.Discord.GameChannels[nameKey]; ch != "" {
			return ch
		}
	}

	return ""
}

func (b *Bot) getGeneralChannel() string {
	if b.cfg.Discord.DashboardChannelID != "" {
		return b.cfg.Discord.DashboardChannelID
	}
	return b.state.GetChannelID()
}

func (b *Bot) getForumChannel() string {
	if b.cfg.Discord.ForumChannelID != "" {
		return b.cfg.Discord.ForumChannelID
	}
	return b.state.GetForumChannel()
}

func (b *Bot) getNodesChannel() string {
	if b.state != nil {
		if ch := b.state.GetNodesChannelID(); ch != "" {
			return ch
		}
	}
	if b.cfg != nil {
		return b.cfg.Discord.NodesChannelID
	}
	return ""
}

func (b *Bot) getServerChannel(serverID uint64, address string, port int) string {
	// 1. Check dynamic state
	if ch := b.state.GetServerChannel(serverID); ch != "" {
		return ch
	}

	// 2. Check static config.json
	if b.cfg != nil && b.cfg.Discord.ServerChannels != nil {
		idStr := strconv.FormatUint(serverID, 10)
		if ch := b.cfg.Discord.ServerChannels[idStr]; ch != "" {
			return ch
		}
		if address != "" && port > 0 {
			addrStr := fmt.Sprintf("%s:%d", address, port)
			if ch := b.cfg.Discord.ServerChannels[addrStr]; ch != "" {
				return ch
			}
		}
	}

	return ""
}

func (b *Bot) updateDashboard(servers []gameap.PublicServerDTO, gameapAvailable bool) {
	generalChannel := b.getGeneralChannel()
	webURL := ""
	if b.gameapClient != nil {
		webURL = b.gameapClient.GetBaseURL()
	}

	// Keep track of keys active in this update cycle
	activeKeys := make(map[string]bool)

	// 0. Dispatch servers that have dedicated single-server channels
	for _, srv := range servers {
		srvChannel := b.getServerChannel(srv.ID, srv.Address, srv.Port)
		if srvChannel != "" {
			embed := RenderSingleServerEmbed(srv, b.cfg)
			components := BuildSingleServerButtons(srv)
			key := fmt.Sprintf("server_%d_%s", srv.ID, srvChannel)
			activeKeys[key] = true
			b.syncDiscordMessage(srvChannel, key, embed, components)
		}
	}

	// Group servers by game
	groups := make(map[string][]gameap.PublicServerDTO)
	gameCodes := make(map[string]string)
	for _, srv := range servers {
		gameName := srv.GameName
		if gameName == "" {
			gameName = strings.ToUpper(srv.GameCode)
		}
		groups[gameName] = append(groups[gameName], srv)
		if gameCodes[gameName] == "" && srv.GameCode != "" {
			gameCodes[gameName] = srv.GameCode
		}
	}

	// 1. Dispatch games that have dedicated channels
	dedicatedDispatched := make(map[string]bool)
	for gameName, srvs := range groups {
		destChannel := b.getChannelForGame(gameName, gameCodes[gameName])
		if destChannel != "" {
			dedicatedDispatched[gameName] = true
			for _, srv := range srvs {
				// Skip servers that already have their own dedicated single-server channel
				if b.getServerChannel(srv.ID, srv.Address, srv.Port) != "" {
					continue
				}
				embed := RenderSingleServerEmbed(srv, b.cfg)
				components := BuildSingleServerButtons(srv)
				key := fmt.Sprintf("server_%d_%s", srv.ID, destChannel)
				activeKeys[key] = true
				b.syncDiscordMessage(destChannel, key, embed, components)
			}
		}
	}

	// 2. Dispatch to general dashboard channel (if configured)
	if generalChannel != "" {
		for _, srv := range servers {
			// Skip servers that already have their own dedicated single-server channel
			if b.getServerChannel(srv.ID, srv.Address, srv.Port) != "" {
				continue
			}

			gameName := srv.GameName
			if gameName == "" {
				gameName = strings.ToUpper(srv.GameCode)
			}
			// Only display in general channel if this game does not have its own dedicated channel
			if !dedicatedDispatched[gameName] {
				embed := RenderSingleServerEmbed(srv, b.cfg)
				components := BuildSingleServerButtons(srv)
				key := fmt.Sprintf("server_%d_%s", srv.ID, generalChannel)
				activeKeys[key] = true
				b.syncDiscordMessage(generalChannel, key, embed, components)
			}
		}
	}

	// 3. Dispatch to Forum Channel (Scenario B: Forum Threads per game)
	forumChannel := b.getForumChannel()
	if forumChannel != "" {
		for gameName, srvs := range groups {
			embed := RenderGameGroupEmbed(gameName, srvs, b.cfg, webURL)
			components := BuildServerButtons(srvs)
			b.syncForumPost(forumChannel, gameName, embed, components)
		}
	}

	// 4. Clean up stale messages for games/servers that are no longer active
	b.cleanupStaleMessages(activeKeys, generalChannel, gameapAvailable)
}

func (b *Bot) cleanupStaleMessages(activeKeys map[string]bool, generalChannel string, gameapAvailable bool) {
	tracked := b.state.GetAllMessageIDs()

	for key, msgID := range tracked {
		if activeKeys[key] {
			continue
		}

		// If GameAP was temporarily unavailable / unreachable, do NOT delete GameAP server messages!
		if !gameapAvailable && strings.HasPrefix(key, "server_") {
			isCustom := false
			for _, cs := range b.state.GetCustomServers() {
				if strings.HasPrefix(key, fmt.Sprintf("server_%d_", cs.ID)) {
					isCustom = true
					break
				}
			}
			if !isCustom {
				continue
			}
		}

		// Find channel ID from key format game_<name>_<channel> or summary
		var channelID string
		if key == "summary" {
			channelID = generalChannel
		} else {
			parts := strings.Split(key, "_")
			if len(parts) >= 3 {
				channelID = parts[len(parts)-1]
			}
		}

		if channelID != "" && msgID != "" {
			log.Printf("[INFO] Removing obsolete dashboard message %s (key %s) from channel %s", msgID, key, channelID)
			_ = b.session.ChannelMessageDelete(channelID, msgID)
		}
		_ = b.state.RemoveMessageID(key)
	}
}

func (b *Bot) syncDiscordMessage(channelID, key string, embed *discordgo.MessageEmbed, components []discordgo.MessageComponent) {
	msgID := b.state.GetMessageID(key)

	if msgID != "" {
		// Attempt to edit existing message
		edit := &discordgo.MessageEdit{
			ID:         msgID,
			Channel:    channelID,
			Embeds:     &[]*discordgo.MessageEmbed{embed},
			Components: &components,
		}
		_, err := b.session.ChannelMessageEditComplex(edit)
		if err == nil {
			return // Successfully edited
		}

		// Check if error is because message was physically deleted (404 / 10008)
		if !isMessageDeletedError(err) {
			log.Printf("[WARN] Transient error editing message %s in channel %s: %v (skipping duplicate creation, will retry next tick)", msgID, channelID, err)
			return
		}

		log.Printf("[INFO] Existing dashboard message %s was deleted from channel %s, recreating...", msgID, channelID)
		_ = b.state.RemoveMessageID(key)
	}

	// Before creating a new message, ensure any old message ID for this key is deleted
	if msgID != "" {
		_ = b.session.ChannelMessageDelete(channelID, msgID)
	}

	// Send new message
	send := &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	}
	msg, err := b.session.ChannelMessageSendComplex(channelID, send)
	if err != nil {
		log.Printf("[ERROR] Failed to send dashboard message to channel %s: %v", channelID, err)
		return
	}

	_ = b.state.SetMessageID(key, msg.ID)
}

func (b *Bot) syncForumPost(forumChannelID string, gameName string, embed *discordgo.MessageEmbed, components []discordgo.MessageComponent) {
	threadID := b.state.GetForumThreadID(gameName)

	if threadID != "" {
		// Attempt to update the starter message in the forum thread
		// (In Discord Forum Channels, starter message ID == threadID)
		edit := &discordgo.MessageEdit{
			Channel:    threadID,
			ID:         threadID,
			Embeds:     &[]*discordgo.MessageEmbed{embed},
			Components: &components,
		}

		_, err := b.session.ChannelMessageEditComplex(edit)
		if err == nil {
			return // Successfully updated existing forum post!
		}

		// Check if thread was archived: unarchive and retry
		unarchived := false
		if _, unarchErr := b.session.ChannelEditComplex(threadID, &discordgo.ChannelEdit{Archived: &unarchived}); unarchErr == nil {
			if _, retryErr := b.session.ChannelMessageEditComplex(edit); retryErr == nil {
				return
			}
		}

		// If thread/message was deleted (404 or 10008), reset thread ID to re-create
		if isMessageDeletedError(err) {
			log.Printf("[INFO] Forum thread for %s (%s) was deleted. Re-creating post...", gameName, threadID)
			_ = b.state.RemoveForumThread(gameName)
		} else {
			// Transient network or rate limit error: log warning and retry on next tick
			log.Printf("[WARN] Failed to edit forum post for %s in thread %s: %v", gameName, threadID, err)
			return
		}
	}

	// Create new post/thread in forum channel
	threadStart := &discordgo.ThreadStart{
		Name:                fmt.Sprintf("🎮 %s", gameName),
		AutoArchiveDuration: 10080, // 7 days (maximum allowed)
	}
	msgSend := &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	}

	th, err := b.session.ForumThreadStartComplex(forumChannelID, threadStart, msgSend)
	if err != nil {
		log.Printf("[ERROR] Failed to create forum thread for %s in %s: %v", gameName, forumChannelID, err)
		return
	}

	log.Printf("[INFO] Created forum thread '%s' (ID: %s) in forum channel %s", th.Name, th.ID, forumChannelID)
	_ = b.state.SetForumThreadID(gameName, th.ID)
}

// PurgeChannelBotMessages finds and deletes bot-authored dashboard messages in a channel.
func (b *Bot) PurgeChannelBotMessages(channelID string, limit int) (int, error) {
	if limit <= 0 {
		limit = 50
	}
	messages, err := b.session.ChannelMessages(channelID, limit, "", "", "")
	if err != nil {
		return 0, err
	}

	botUserID := ""
	if b.session.State != nil && b.session.State.User != nil {
		botUserID = b.session.State.User.ID
	}

	deleted := 0
	for _, m := range messages {
		if m.Author == nil {
			continue
		}
		isBotAuthor := (botUserID != "" && m.Author.ID == botUserID) || m.Author.Bot
		if !isBotAuthor {
			continue
		}

		isDashboardMsg := false
		for _, emb := range m.Embeds {
			if emb.Footer != nil && strings.Contains(emb.Footer.Text, "GameAP WebMonitoring") {
				isDashboardMsg = true
				break
			}
		}

		if isDashboardMsg || (botUserID != "" && m.Author.ID == botUserID) {
			if err := b.session.ChannelMessageDelete(channelID, m.ID); err == nil {
				deleted++
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Remove all tracked state messages for this channel
	b.state.RemoveMessagesForChannel(channelID)

	return deleted, nil
}

func (b *Bot) pollNodes(ctx context.Context) {
	if err := b.RefreshNodesStatus(ctx); err != nil {
		log.Printf("[WARN] Failed to refresh nodes status: %v", err)
	}
}

// RefreshNodesStatus triggers a manual or scheduled refresh of the host nodes monitoring embed.
func (b *Bot) RefreshNodesStatus(ctx context.Context) error {
	return b.UpdateNodesStatus(ctx)
}

// UpdateNodesStatus fetches nodes from GameAP, enriches latency, and updates Discord message.
func (b *Bot) UpdateNodesStatus(ctx context.Context) error {
	channelID := b.getNodesChannel()
	if channelID == "" {
		return nil
	}

	timeoutSec := 10
	if b.cfg != nil && b.cfg.GameAP.TimeoutSeconds > 0 {
		timeoutSec = b.cfg.GameAP.TimeoutSeconds
	}
	pollCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	var nodes []gameap.NodeDTO
	var servers []gameap.PublicServerDTO

	if b.gameapClient != nil && b.cfg != nil && b.cfg.GameAP.BaseURL != "" {
		resp, err := b.gameapClient.FetchServers(pollCtx)
		if err == nil && resp != nil {
			nodes = resp.Nodes
			servers = resp.Servers
		} else if err != nil {
			log.Printf("[WARN] Failed to fetch nodes from GameAP API: %v (falling back to config nodes if present)", err)
		}
	}

	var configNodes []config.NodeConfig
	var updateSec int
	var onlineColor, offlineColor string
	if b.cfg != nil {
		configNodes = b.cfg.Nodes
		updateSec = b.cfg.Discord.NodesUpdateIntervalSeconds
		onlineColor = b.cfg.Bot.EmbedColorOnline
		offlineColor = b.cfg.Bot.EmbedColorOffline
	}
	enriched := EnrichNodeStatuses(nodes, servers, configNodes)
	if len(enriched) == 0 {
		return nil
	}

	embed := RenderNodesStatusEmbed(enriched, updateSec, onlineColor, offlineColor)
	components := BuildNodesStatusComponents()

	return b.syncNodesDiscordMessage(channelID, embed, components)
}

func (b *Bot) syncNodesDiscordMessage(channelID string, embed *discordgo.MessageEmbed, components []discordgo.MessageComponent) error {
	if b.session == nil {
		return nil
	}

	var msgID string
	if b.state != nil {
		msgID = b.state.GetNodesMessageID()
	}

	if msgID != "" {
		edit := &discordgo.MessageEdit{
			ID:         msgID,
			Channel:    channelID,
			Embeds:     &[]*discordgo.MessageEmbed{embed},
			Components: &components,
		}
		_, err := b.session.ChannelMessageEditComplex(edit)
		if err == nil {
			return nil
		}

		if !isMessageDeletedError(err) {
			log.Printf("[WARN] Transient error editing nodes message %s in channel %s: %v", msgID, channelID, err)
			return err
		}

		log.Printf("[INFO] Existing nodes status message %s was deleted from channel %s, recreating...", msgID, channelID)
		if b.state != nil {
			_ = b.state.SetNodesMessageID("")
		}
	}

	send := &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	}
	msg, err := b.session.ChannelMessageSendComplex(channelID, send)
	if err != nil {
		log.Printf("[ERROR] Failed to send nodes status message to channel %s: %v", channelID, err)
		return fmt.Errorf("sending nodes status message: %w", err)
	}

	if b.state != nil {
		_ = b.state.SetNodesMessageID(msg.ID)
		_ = b.state.Save()
	}
	return nil
}

func isMessageDeletedError(err error) bool {
	if err == nil {
		return false
	}
	if restErr, ok := err.(*discordgo.RESTError); ok {
		if restErr.Response != nil && restErr.Response.StatusCode == 404 {
			return true
		}
		if restErr.Message != nil && restErr.Message.Code == 10008 {
			return true
		}
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "unknown message") || strings.Contains(errStr, "404")
}

func sanitizeKey(name string) string {
	lower := strings.ToLower(name)
	return strings.ReplaceAll(lower, " ", "_")
}

func normalizeGameKey(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	lower = strings.ReplaceAll(lower, " ", "_")
	lower = strings.ReplaceAll(lower, "-", "_")
	return lower
}

