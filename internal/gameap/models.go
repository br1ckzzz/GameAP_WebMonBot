package gameap

// PublicServerDTO represents an active or monitored game server.
type PublicServerDTO struct {
	ID         uint64 `json:"id"`
	Name       string `json:"name"`
	GameCode   string `json:"game_code"`
	GameName   string `json:"game_name"`
	Address    string `json:"address"`
	Port       int    `json:"port"`
	Status     string `json:"status"` // "online" or "offline"
	Installed  bool   `json:"installed"`
	Blocked    bool   `json:"blocked"`
	ConnectURL string `json:"connect_url"`
	Map        string `json:"map,omitempty"`
	Players    int    `json:"players,omitempty"`
	MaxPlayers int    `json:"max_players,omitempty"`
	Ping       int    `json:"ping,omitempty"`
	Version       string           `json:"version,omitempty"`
	BuildID       string           `json:"build_id,omitempty"`
	Category      string           `json:"category,omitempty"`
	UptimeSeconds uint64           `json:"uptime_seconds,omitempty"`
	IconURL       string           `json:"icon_url,omitempty"`
}

// AnnouncementDTO represents an active community announcement banner.
type AnnouncementDTO struct {
	Enabled  bool   `json:"enabled"`
	Text     string `json:"text"`
	Link     string `json:"link,omitempty"`
	Type     string `json:"type,omitempty"`     // "info", "warning", "success"
	Deadline string `json:"deadline,omitempty"` // ISO 8601 string
}

// MonitoringResponse is the root payload returned by GET /servers or /api/servers.
type MonitoringResponse struct {
	Success      bool              `json:"success"`
	TotalServers int               `json:"total_servers"`
	OnlineCount  int               `json:"online_count"`
	Servers      []PublicServerDTO `json:"servers"`
	Announcement *AnnouncementDTO  `json:"announcement,omitempty"`
	Timestamp    int64             `json:"timestamp"`
	Error        string            `json:"error,omitempty"`
}
