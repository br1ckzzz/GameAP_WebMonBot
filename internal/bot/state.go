package bot

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// CustomServer represents an external game server added manually by IP:Port.
type CustomServer struct {
	ID        uint64    `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Port      int       `json:"port"`
	GameCode  string    `json:"game_code"`
	GameName  string    `json:"game_name"`
	IconURL   string    `json:"icon_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// State stores persistent state across bot restarts.
type State struct {
	mu           sync.RWMutex
	filePath     string
	GuildID        string                  `json:"guild_id"`
	ChannelID      string                  `json:"channel_id"`
	GameChannels   map[string]string       `json:"game_channels,omitempty"`   // key: normalized game_code or game_name, value: channel_id
	ServerChannels map[uint64]string       `json:"server_channels,omitempty"` // key: server_id, value: channel_id
	CustomServers  map[string]CustomServer `json:"custom_servers,omitempty"`  // key: "address:port"
	BaseURL        string                  `json:"base_url,omitempty"`
	APIToken       string                  `json:"api_token,omitempty"`
	AlertsEnabled  *bool                   `json:"alerts_enabled,omitempty"`
	AlertsChannelID string                 `json:"alerts_channel_id,omitempty"`
	AlertRoleID    string                  `json:"alert_role_id,omitempty"`
	ForumChannelID string                  `json:"forum_channel_id,omitempty"`
	ForumThreadIDs map[string]string       `json:"forum_thread_ids,omitempty"` // key: normalized game_code or game_name, value: thread_id
	NodesChannelID string                  `json:"nodes_channel_id,omitempty"` // dedicated channel for host nodes status
	NodesMessageID string                  `json:"nodes_message_id,omitempty"` // message ID of active nodes status embed
	MessageIDs     map[string]string       `json:"message_ids"`                 // key: game_code or "summary", value: discord message ID
	ManagerRoleIDs []string                `json:"manager_role_ids,omitempty"`  // roles allowed to manage the bot
	LastUpdated    time.Time               `json:"last_updated"`
}

// LoadState reads state from the specified file path, or initializes an empty state.
func LoadState(filePath string) *State {
	if filePath == "" {
		filePath = "state.json"
	}

	s := &State{
		filePath:       filePath,
		GameChannels:   make(map[string]string),
		ServerChannels: make(map[uint64]string),
		CustomServers:  make(map[string]CustomServer),
		ForumThreadIDs: make(map[string]string),
		MessageIDs:     make(map[string]string),
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return s
	}

	if err := json.Unmarshal(data, s); err != nil {
		log.Printf("[WARN] Failed to parse state file %s: %v", filePath, err)
		corruptPath := filePath + ".corrupt"
		if writeErr := os.WriteFile(corruptPath, data, 0644); writeErr != nil {
			log.Printf("[WARN] Failed to back up corrupted state file to %s: %v", corruptPath, writeErr)
		} else {
			log.Printf("[WARN] Backed up corrupted state file to %s", corruptPath)
		}
	}
	if s.GameChannels == nil {
		s.GameChannels = make(map[string]string)
	}
	if s.ServerChannels == nil {
		s.ServerChannels = make(map[uint64]string)
	}
	if s.CustomServers == nil {
		s.CustomServers = make(map[string]CustomServer)
	}
	if s.ForumThreadIDs == nil {
		s.ForumThreadIDs = make(map[string]string)
	}
	if s.MessageIDs == nil {
		s.MessageIDs = make(map[string]string)
	}

	return s
}

// Save writes the current state to disk atomically.
func (s *State) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.SaveLocked()
}

// SaveLocked writes state to disk without acquiring the mutex (caller must hold s.mu).
func (s *State) SaveLocked() error {
	if s.filePath == "" {
		return nil
	}
	s.LastUpdated = time.Now()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpFile, s.filePath)
}

// GetMessageID returns the Discord message ID for a given key.
func (s *State) GetMessageID(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.MessageIDs[key]
}

// SetMessageID records a Discord message ID for a key and saves state.
func (s *State) SetMessageID(key, messageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MessageIDs[key] = messageID
	return s.SaveLocked()
}

// RemoveMessageID deletes a message ID from state.
func (s *State) RemoveMessageID(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.MessageIDs, key)
	return s.SaveLocked()
}

// RemoveMessagesForChannel deletes and returns all tracked message IDs for a specific channel.
func (s *State) RemoveMessagesForChannel(channelID string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := make(map[string]string)
	for k, v := range s.MessageIDs {
		if strings.HasSuffix(k, "_"+channelID) || (k == "summary" && s.ChannelID == channelID) {
			removed[k] = v
			delete(s.MessageIDs, k)
		}
	}
	if len(removed) > 0 {
		_ = s.SaveLocked()
	}
	return removed
}

// GetChannelMessageIDs returns a snapshot of all message IDs tracked for the specified channel.
func (s *State) GetChannelMessageIDs(channelID string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make(map[string]string)
	for k, v := range s.MessageIDs {
		if strings.HasSuffix(k, "_"+channelID) || (k == "summary" && s.ChannelID == channelID) {
			res[k] = v
		}
	}
	return res
}

// GetAllMessageIDs returns a snapshot of all message IDs tracked across all channels.
func (s *State) GetAllMessageIDs() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make(map[string]string, len(s.MessageIDs))
	for k, v := range s.MessageIDs {
		res[k] = v
	}
	return res
}

// GetChannelID returns the bound channel ID.
func (s *State) GetChannelID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ChannelID
}

// GetGuildID returns the bound guild ID.
func (s *State) GetGuildID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.GuildID
}

// BindChannel assigns a target guild and channel for the general dashboard.
func (s *State) BindChannel(guildID, channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if guildID != "" {
		s.GuildID = guildID
	}
	oldChannelID := s.ChannelID
	s.ChannelID = channelID
	delete(s.MessageIDs, "summary")

	// If channel changed, remove old channel game message IDs
	if oldChannelID != "" && oldChannelID != channelID {
		for k := range s.MessageIDs {
			if strings.HasSuffix(k, "_"+oldChannelID) {
				delete(s.MessageIDs, k)
			}
		}
	}

	return s.SaveLocked()
}

// BindGameChannel maps a game code or name to a specific channel.
func (s *State) BindGameChannel(guildID, gameKey, channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if guildID != "" {
		s.GuildID = guildID
	}
	normKey := strings.ToLower(strings.TrimSpace(gameKey))
	normKey = strings.ReplaceAll(normKey, " ", "_")
	normKey = strings.ReplaceAll(normKey, "-", "_")

	if s.GameChannels == nil {
		s.GameChannels = make(map[string]string)
	}
	s.GameChannels[normKey] = channelID

	// Clear any previous message IDs for this game across all channels so it creates fresh
	prefix := "game_" + normKey
	for k := range s.MessageIDs {
		if k == prefix || strings.HasPrefix(k, prefix+"_") {
			delete(s.MessageIDs, k)
		}
	}

	return s.SaveLocked()
}

// UnbindGameChannel removes a dedicated game channel mapping.
func (s *State) UnbindGameChannel(gameKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	normKey := strings.ToLower(strings.TrimSpace(gameKey))
	normKey = strings.ReplaceAll(normKey, " ", "_")
	normKey = strings.ReplaceAll(normKey, "-", "_")

	delete(s.GameChannels, normKey)
	return s.SaveLocked()
}

// GetGameChannel returns the dedicated channel ID for a game key, if any.
func (s *State) GetGameChannel(gameKey string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	normKey := strings.ToLower(strings.TrimSpace(gameKey))
	normKey = strings.ReplaceAll(normKey, " ", "_")
	normKey = strings.ReplaceAll(normKey, "-", "_")
	return s.GameChannels[normKey]
}

// GetGameChannels returns a copy of all active game channel mappings.
func (s *State) GetGameChannels() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copyMap := make(map[string]string, len(s.GameChannels))
	for k, v := range s.GameChannels {
		copyMap[k] = v
	}
	return copyMap
}

// BindServerChannel maps a specific server ID to a dedicated channel.
func (s *State) BindServerChannel(guildID string, serverID uint64, channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if guildID != "" {
		s.GuildID = guildID
	}
	if s.ServerChannels == nil {
		s.ServerChannels = make(map[uint64]string)
	}
	s.ServerChannels[serverID] = channelID

	// Clear any previous message IDs for this server across all channels
	prefix := fmt.Sprintf("server_%d", serverID)
	for k := range s.MessageIDs {
		if k == prefix || strings.HasPrefix(k, prefix+"_") {
			delete(s.MessageIDs, k)
		}
	}

	return s.SaveLocked()
}

// UnbindServerChannel removes a dedicated server channel mapping.
func (s *State) UnbindServerChannel(serverID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.ServerChannels, serverID)
	return s.SaveLocked()
}

// GetServerChannel returns the dedicated channel ID for a server ID, if any.
func (s *State) GetServerChannel(serverID uint64) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ServerChannels == nil {
		return ""
	}
	return s.ServerChannels[serverID]
}

// GetServerChannels returns a copy of all active server channel mappings.
func (s *State) GetServerChannels() map[uint64]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copyMap := make(map[uint64]string, len(s.ServerChannels))
	for k, v := range s.ServerChannels {
		copyMap[k] = v
	}
	return copyMap
}

// SetCredentials stores the GameAP API endpoint and secret token in state.
func (s *State) SetCredentials(baseURL, apiToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.BaseURL = baseURL
	s.APIToken = apiToken
	return s.SaveLocked()
}

// SetAlerts updates the alerts configuration in persistent state.
func (s *State) SetAlerts(enabled *bool, channelID, roleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if enabled != nil {
		s.AlertsEnabled = enabled
	}
	if channelID != "" {
		if channelID == "none" || channelID == "clear" {
			s.AlertsChannelID = ""
		} else {
			s.AlertsChannelID = channelID
		}
	}
	if roleID != "" {
		if roleID == "none" || roleID == "clear" {
			s.AlertRoleID = ""
		} else {
			s.AlertRoleID = roleID
		}
	}

	return s.SaveLocked()
}

// GetAlerts returns the resolved alerts configuration, falling back to default values.
func (s *State) GetAlerts(defaultEnabled bool, defaultChannelID, defaultRoleID string) (bool, string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	enabled := defaultEnabled
	if s.AlertsEnabled != nil {
		enabled = *s.AlertsEnabled
	}

	channelID := defaultChannelID
	if s.AlertsChannelID != "" {
		channelID = s.AlertsChannelID
	}

	roleID := defaultRoleID
	if s.AlertRoleID != "" {
		roleID = s.AlertRoleID
	}

	return enabled, channelID, roleID
}

// SetForumChannel updates the dedicated forum channel ID.
func (s *State) SetForumChannel(channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if channelID == "none" || channelID == "clear" {
		s.ForumChannelID = ""
	} else {
		s.ForumChannelID = channelID
	}
	return s.SaveLocked()
}

// GetForumChannel returns the active forum channel ID.
func (s *State) GetForumChannel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ForumChannelID
}

// SetForumThreadID records the Discord thread ID for a game in a forum channel.
func (s *State) SetForumThreadID(gameKey, threadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	normKey := strings.ToLower(strings.TrimSpace(gameKey))
	normKey = strings.ReplaceAll(normKey, " ", "_")
	normKey = strings.ReplaceAll(normKey, "-", "_")
	if s.ForumThreadIDs == nil {
		s.ForumThreadIDs = make(map[string]string)
	}
	s.ForumThreadIDs[normKey] = threadID
	return s.SaveLocked()
}

// GetForumThreadID returns the thread ID for a given game key.
func (s *State) GetForumThreadID(gameKey string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	normKey := strings.ToLower(strings.TrimSpace(gameKey))
	normKey = strings.ReplaceAll(normKey, " ", "_")
	normKey = strings.ReplaceAll(normKey, "-", "_")
	if s.ForumThreadIDs == nil {
		return ""
	}
	return s.ForumThreadIDs[normKey]
}

// RemoveForumThread removes a game's thread ID from state.
func (s *State) RemoveForumThread(gameKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	normKey := strings.ToLower(strings.TrimSpace(gameKey))
	normKey = strings.ReplaceAll(normKey, " ", "_")
	normKey = strings.ReplaceAll(normKey, "-", "_")
	delete(s.ForumThreadIDs, normKey)
	return s.SaveLocked()
}

// GetForumThreadIDs returns a copy of all active forum thread mappings.
func (s *State) GetForumThreadIDs() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copyMap := make(map[string]string, len(s.ForumThreadIDs))
	for k, v := range s.ForumThreadIDs {
		copyMap[k] = v
	}
	return copyMap
}

// SetNodesChannelID updates the dedicated channel ID for host nodes status.
func (s *State) SetNodesChannelID(channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var newChannel string
	if channelID != "none" && channelID != "clear" {
		newChannel = strings.TrimSpace(channelID)
	}

	if s.NodesChannelID != newChannel {
		s.NodesChannelID = newChannel
		s.NodesMessageID = "" // reset message ID on channel change or unbind
	}

	return s.SaveLocked()
}

// GetNodesChannelID returns the active dedicated nodes channel ID.
func (s *State) GetNodesChannelID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.NodesChannelID
}

// SetNodesMessageID records the Discord message ID for the nodes status message.
func (s *State) SetNodesMessageID(msgID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.NodesMessageID = strings.TrimSpace(msgID)
	return s.SaveLocked()
}

// GetNodesMessageID returns the active Discord message ID for the nodes status embed.
func (s *State) GetNodesMessageID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.NodesMessageID
}

// generateCustomServerID generates a stable unique ID >= 100000 based on address and port.
func generateCustomServerID(addrPort string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(addrPort))
	return 100000 + (h.Sum64() % 899999)
}

// AddCustomServer adds or updates an external game server and saves state.
func (s *State) AddCustomServer(srv CustomServer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%d", strings.ToLower(strings.TrimSpace(srv.Address)), srv.Port)
	if srv.ID == 0 {
		srv.ID = generateCustomServerID(key)
	}
	if srv.CreatedAt.IsZero() {
		srv.CreatedAt = time.Now()
	}
	if s.CustomServers == nil {
		s.CustomServers = make(map[string]CustomServer)
	}

	s.CustomServers[key] = srv
	return s.SaveLocked()
}

// RemoveCustomServer removes an external server by "address:port" or "address" and saves state.
func (s *State) RemoveCustomServer(addrPort string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	clean := strings.ToLower(strings.TrimSpace(addrPort))
	if s.CustomServers == nil {
		return false
	}

	if _, ok := s.CustomServers[clean]; ok {
		delete(s.CustomServers, clean)
		_ = s.SaveLocked()
		return true
	}

	// Try finding matching by prefix or exact match
	for k := range s.CustomServers {
		if strings.EqualFold(k, clean) || strings.EqualFold(strings.Split(k, ":")[0], clean) {
			delete(s.CustomServers, k)
			_ = s.SaveLocked()
			return true
		}
	}

	return false
}

// GetCustomServers returns a list of all external custom servers.
func (s *State) GetCustomServers() []CustomServer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]CustomServer, 0, len(s.CustomServers))
	for _, srv := range s.CustomServers {
		list = append(list, srv)
	}
	return list
}

// GetCustomServer returns a custom server by address:port.
func (s *State) GetCustomServer(addrPort string) (CustomServer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clean := strings.ToLower(strings.TrimSpace(addrPort))
	srv, ok := s.CustomServers[clean]
	return srv, ok
}

// AddManagerRole adds a role ID to the list of authorized manager roles.
func (s *State) AddManagerRole(roleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return nil
	}
	for _, id := range s.ManagerRoleIDs {
		if id == roleID {
			return nil
		}
	}
	s.ManagerRoleIDs = append(s.ManagerRoleIDs, roleID)
	return s.SaveLocked()
}

// RemoveManagerRole removes a role ID from manager roles.
func (s *State) RemoveManagerRole(roleID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	roleID = strings.TrimSpace(roleID)
	found := false
	var updated []string
	for _, id := range s.ManagerRoleIDs {
		if id == roleID {
			found = true
			continue
		}
		updated = append(updated, id)
	}
	if found {
		s.ManagerRoleIDs = updated
		_ = s.SaveLocked()
	}
	return found
}

// GetManagerRoles returns a copy of authorized manager role IDs.
func (s *State) GetManagerRoles() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]string, len(s.ManagerRoleIDs))
	copy(res, s.ManagerRoleIDs)
	return res
}


