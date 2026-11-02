package bot

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gameap-webmon-bot/internal/config"
	"gameap-webmon-bot/internal/gameap"
)

// NodeStatus represents enriched runtime status for a dedicated node / host server.
type NodeStatus struct {
	ID           uint64
	Name         string
	Location     string
	CountryCode  string
	Flag         string
	Provider     string
	OS           string
	CPUModel     string
	CPUPercent   int
	RAMPercent   int
	Protection   string
	Latency      string
	ServersCount int
	IsOnline     bool
	Host         string
	Port         int
}

var (
	tagRegex = regexp.MustCompile(`\[([A-Za-z]{2,3})\]`)

	countryMap = map[string]struct {
		Code string
		Flag string
	}{
		"DE":  {Code: "DE", Flag: "🇩🇪"},
		"GE":  {Code: "DE", Flag: "🇩🇪"}, // Common gaming node tag for Germany
		"RU":  {Code: "RU", Flag: "🇷🇺"},
		"KZ":  {Code: "KZ", Flag: "🇰🇿"},
		"FI":  {Code: "FI", Flag: "🇫🇮"},
		"NL":  {Code: "NL", Flag: "🇳🇱"},
		"US":  {Code: "US", Flag: "🇺🇸"},
		"USA": {Code: "US", Flag: "🇺🇸"},
		"FR":  {Code: "FR", Flag: "🇫🇷"},
		"PL":  {Code: "PL", Flag: "🇵🇱"},
		"UA":  {Code: "UA", Flag: "🇺🇦"},
		"BY":  {Code: "BY", Flag: "🇧🇾"},
		"GB":  {Code: "GB", Flag: "🇬🇧"},
		"UK":  {Code: "GB", Flag: "🇬🇧"},
		"SE":  {Code: "SE", Flag: "🇸🇪"},
		"NO":  {Code: "NO", Flag: "🇳🇴"},
		"TR":  {Code: "TR", Flag: "🇹🇷"},
		"SG":  {Code: "SG", Flag: "🇸🇬"},
		"JP":  {Code: "JP", Flag: "🇯🇵"},
		"KR":  {Code: "KR", Flag: "🇰🇷"},
		"CN":  {Code: "CN", Flag: "🇨🇳"},
		"CA":  {Code: "CA", Flag: "🇨🇦"},
		"AU":  {Code: "AU", Flag: "🇦🇺"},
		"BR":  {Code: "BR", Flag: "🇧🇷"},
		"ES":  {Code: "ES", Flag: "🇪🇸"},
		"IT":  {Code: "IT", Flag: "🇮🇹"},
		"AT":  {Code: "AT", Flag: "🇦🇹"},
		"CH":  {Code: "CH", Flag: "🇨🇭"},
		"CZ":  {Code: "CZ", Flag: "🇨🇿"},
	}

	keywordLocations = []struct {
		Keywords []string
		Code     string
		Flag     string
	}{
		{Keywords: []string{"germany", "frankfurt", "berlin", "deutschland"}, Code: "DE", Flag: "🇩🇪"},
		{Keywords: []string{"russia", "moscow", "petersburg", "россия", "москва", "питер", "dataline", "мтс", "mts"}, Code: "RU", Flag: "🇷🇺"},
		{Keywords: []string{"kazakhstan", "almaty", "astana", "казахстан", "алматы", "астана"}, Code: "KZ", Flag: "🇰🇿"},
		{Keywords: []string{"finland", "helsinki", "финляндия", "хельсинки"}, Code: "FI", Flag: "🇫🇮"},
		{Keywords: []string{"netherlands", "amsterdam", "нидерланды", "амстердам"}, Code: "NL", Flag: "🇳🇱"},
		{Keywords: []string{"france", "paris", "roubaix", "франция", "париж"}, Code: "FR", Flag: "🇫🇷"},
		{Keywords: []string{"poland", "warsaw", "польша", "варшава"}, Code: "PL", Flag: "🇵🇱"},
		{Keywords: []string{"united states", "usa", "dallas", "chicago", "new york", "ashburn", "virginia"}, Code: "US", Flag: "🇺🇸"},
		{Keywords: []string{"united kingdom", "london", "england", "великобритания", "лондон"}, Code: "GB", Flag: "🇬🇧"},
		{Keywords: []string{"sweden", "stockholm", "швеция", "стокгольм"}, Code: "SE", Flag: "🇸🇪"},
		{Keywords: []string{"norway", "oslo", "норвегия"}, Code: "NO", Flag: "🇳🇴"},
		{Keywords: []string{"belarus", "minsk", "беларусь", "минск"}, Code: "BY", Flag: "🇧🇾"},
		{Keywords: []string{"ukraine", "kyiv", "украина", "киев"}, Code: "UA", Flag: "🇺🇦"},
	}
)

// DetectCountryAndFlag identifies the ISO country code and flag emoji from node name or location.
func DetectCountryAndFlag(name, location string) (countryCode, flag string) {
	// 1. Try to extract tag from node name or location, e.g. [GE], [RU], [KZ]
	matches := tagRegex.FindAllStringSubmatch(name+" "+location, -1)
	for _, m := range matches {
		if len(m) > 1 {
			upper := strings.ToUpper(m[1])
			if match, ok := countryMap[upper]; ok {
				return match.Code, match.Flag
			}
		}
	}

	// 2. Try to match keywords in location or name
	combined := strings.ToLower(name + " " + location)
	for _, entry := range keywordLocations {
		for _, kw := range entry.Keywords {
			if strings.Contains(combined, kw) {
				return entry.Code, entry.Flag
			}
		}
	}

	// 3. Fallback to global icon
	return "GLOBAL", "🌐"
}

// FormatOS formats operating system name with modern OS emoji.
func FormatOS(osName string) string {
	clean := strings.ToLower(strings.TrimSpace(osName))
	switch {
	case strings.Contains(clean, "win"):
		return "🪟 Windows"
	case strings.Contains(clean, "linux") || strings.Contains(clean, "ubuntu") || strings.Contains(clean, "debian") || strings.Contains(clean, "centos") || strings.Contains(clean, "alpine") || strings.Contains(clean, "rhel"):
		return "🐧 Linux"
	case clean != "":
		return fmt.Sprintf("🖥️ %s", strings.TrimSpace(osName))
	default:
		return ""
	}
}

// ProbeNodeLatency tests TCP reachability to host:port with a timeout.
// On success, it returns formatted ping in ms (e.g. "21 ms").
// If the connection times out or fails (common with Anti-DDoS filtering),
// it returns a localized protected filter indicator instead of marking the node offline.
func ProbeNodeLatency(host string, port int, timeout time.Duration, lang string) string {
	if strings.ToLower(lang) != "en" {
		lang = "ru"
	}
	protectedMsg := "Защищён (DDoS Filter)"
	if lang == "en" {
		protectedMsg = "Protected (DDoS Filter)"
	}

	host = strings.TrimSpace(host)
	if host == "" {
		return protectedMsg
	}
	if port <= 0 {
		port = 31717
	}
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err == nil {
		_ = conn.Close()
		elapsed := time.Since(start).Milliseconds()
		if elapsed <= 0 {
			elapsed = 1
		}
		return fmt.Sprintf("%d ms", elapsed)
	}

	// Attempt quick fallback ports in case gdaemon port is internal or behind firewall
	fallbackPorts := []int{80, 443, 22, 27015, 25565, 8080}
	quickTimeout := 350 * time.Millisecond
	for _, fbPort := range fallbackPorts {
		if fbPort == port {
			continue
		}
		fbAddr := net.JoinHostPort(host, strconv.Itoa(fbPort))
		fbStart := time.Now()
		fbConn, fbErr := net.DialTimeout("tcp", fbAddr, quickTimeout)
		if fbErr == nil {
			_ = fbConn.Close()
			elapsed := time.Since(fbStart).Milliseconds()
			if elapsed <= 0 {
				elapsed = 1
			}
			return fmt.Sprintf("%d ms", elapsed)
		}
	}

	return protectedMsg
}

// EnrichNodeStatuses converts GameAP nodes or custom config nodes into rich Discord NodeStatuses.
func EnrichNodeStatuses(nodes []gameap.NodeDTO, servers []gameap.PublicServerDTO, customNodes []config.NodeConfig, lang string) []NodeStatus {
	var result []NodeStatus

	if strings.ToLower(lang) != "en" {
		lang = "ru"
	}

	if len(nodes) > 0 {
		for _, n := range nodes {
			code, flag := DetectCountryAndFlag(n.Name, n.Location)

			// Count game servers running on this node
			count := 0
			for _, s := range servers {
				if isServerOnNode(s, n, len(nodes)) {
					count++
				}
			}

			cpuModel := n.CPUModel
			protection := n.Protection

			host := n.GDaemonHost
			if host == "" && len(n.IPs) > 0 {
				host = n.IPs[0]
			}
			port := n.GDaemonPort
			if port <= 0 {
				port = 31717
			}

			latency := ProbeNodeLatency(host, port, 1500*time.Millisecond, lang)

			result = append(result, NodeStatus{
				ID:           n.ID,
				Name:         n.Name,
				Location:     n.Location,
				CountryCode:  code,
				Flag:         flag,
				Provider:     n.Provider,
				OS:           FormatOS(n.OS),
				CPUModel:     cpuModel,
				CPUPercent:   n.CPUPercent,
				RAMPercent:   n.RAMPercent,
				Protection:   protection,
				Latency:      latency,
				ServersCount: count,
				IsOnline:     n.Enabled,
				Host:         host,
				Port:         port,
			})
		}
		return result
	}

	// Fallback to custom config nodes if GameAP returned no nodes
	for i, cn := range customNodes {
		code, flag := DetectCountryAndFlag(cn.Name, cn.Location)

		count := cn.ServersCount
		if count == 0 && len(servers) > 0 {
			for _, s := range servers {
				if s.Address == cn.Host {
					count++
				}
			}
		}

		cpuModel := cn.CPUModel
		protection := cn.Protection

		port := cn.Port
		if port <= 0 {
			port = 31717
		}

		latency := ProbeNodeLatency(cn.Host, port, 1500*time.Millisecond, lang)

		id := cn.ID
		if id == 0 {
			id = uint64(i + 1)
		}

		result = append(result, NodeStatus{
			ID:           id,
			Name:         cn.Name,
			Location:     cn.Location,
			CountryCode:  code,
			Flag:         flag,
			Provider:     cn.Provider,
			OS:           FormatOS(cn.OS),
			CPUModel:     cpuModel,
			CPUPercent:   cn.CPUPercent,
			RAMPercent:   cn.RAMPercent,
			Protection:   protection,
			Latency:      latency,
			ServersCount: count,
			IsOnline:     cn.Enabled,
			Host:         cn.Host,
			Port:         port,
		})
	}

	// Fallback: discover nodes from unique server addresses if both GameAP nodes and customNodes are empty
	if len(result) == 0 && len(servers) > 0 {
		type hostInfo struct{ count int }
		hostMap := make(map[string]*hostInfo)
		var order []string
		for _, s := range servers {
			addr := strings.TrimSpace(s.Address)
			if addr == "" {
				continue
			}
			if info, exists := hostMap[addr]; exists {
				info.count++
			} else {
				hostMap[addr] = &hostInfo{count: 1}
				order = append(order, addr)
			}
		}
		for idx, addr := range order {
			info := hostMap[addr]
			nodeName := fmt.Sprintf("GameAP Dedicated Host [%s]", addr)
			code, flag := DetectCountryAndFlag(nodeName, "")
			latency := ProbeNodeLatency(addr, 31717, 1500*time.Millisecond, lang)
			location := "Dedicated Server"
			if lang == "ru" {
				location = "Выделенный сервер"
			}
			result = append(result, NodeStatus{
				ID:           uint64(idx + 1),
				Name:         nodeName,
				Location:     location,
				CountryCode:  code,
				Flag:         flag,
				Provider:     "GameAP Host",
				OS:           "",
				CPUModel:     "",
				CPUPercent:   0,
				RAMPercent:   0,
				Protection:   "",
				Latency:      latency,
				ServersCount: info.count,
				IsOnline:     true,
				Host:         addr,
				Port:         31717,
			})
		}
	}

	return result
}

func isServerOnNode(s gameap.PublicServerDTO, n gameap.NodeDTO, totalNodes int) bool {
	if s.NodeID > 0 && n.ID > 0 && s.NodeID == n.ID {
		return true
	}
	if totalNodes == 1 {
		return true
	}
	if s.Address == n.GDaemonHost && n.GDaemonHost != "" {
		return true
	}
	for _, ip := range n.IPs {
		if s.Address == ip && ip != "" {
			return true
		}
	}
	return false
}
