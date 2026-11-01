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

// NodeDTO represents a host machine / dedicated node monitored by GameAP.
type NodeDTO struct {
	ID          uint64   `json:"id"`
	Name        string   `json:"name"`
	Location    string   `json:"location,omitempty"`
	Provider    string   `json:"provider,omitempty"`
	IPs         []string `json:"ips,omitempty"`
	GDaemonHost string   `json:"gdaemon_host,omitempty"`
	GDaemonPort int      `json:"gdaemon_port,omitempty"`
	CPUModel    string   `json:"cpu_model,omitempty"`
	CPUPercent  int      `json:"cpu_percent,omitempty"`
	Protection  string   `json:"protection,omitempty"`
	Enabled     bool     `json:"enabled"`
}

// MonitoringResponse is the root payload returned by GET /servers or /api/servers.
type MonitoringResponse struct {
	Success      bool              `json:"success"`
	TotalServers int               `json:"total_servers"`
	OnlineCount  int               `json:"online_count"`
	Servers      []PublicServerDTO `json:"servers"`
	Nodes        []NodeDTO         `json:"nodes,omitempty"`
	Announcement *AnnouncementDTO  `json:"announcement,omitempty"`
	Timestamp    int64             `json:"timestamp"`
	Error        string            `json:"error,omitempty"`
}

