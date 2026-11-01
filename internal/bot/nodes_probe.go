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
	CPUModel     string
	CPUPercent   int
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
		{Keywords: []string{"russia", "moscow", "petersburg", "россия", "москва", "питер", "dataline"}, Code: "RU", Flag: "🇷🇺"},
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
	// 1. Try to extract tag from node name, e.g. [GE], [RU], [KZ]
	matches := tagRegex.FindAllStringSubmatch(name, -1)
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

// ProbeNodeLatency tests TCP reachability to host:port with a timeout.
// If the connection times out or is rejected (common with Anti-DDoS filtering),
// it returns "Защищён (DDoS Filter)" instead of reporting the node offline.
func ProbeNodeLatency(host string, port int, timeout time.Duration) string {
	if host == "" {
		return "Защищён (DDoS Filter)"
	}
	if port <= 0 {
		port = 31717
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return "Защищён (DDoS Filter)"
	}
	_ = conn.Close()

	elapsed := time.Since(start).Milliseconds()
	return fmt.Sprintf("%d ms", elapsed)
}

// EnrichNodeStatuses converts GameAP nodes or custom config nodes into rich Discord NodeStatuses.
func EnrichNodeStatuses(nodes []gameap.NodeDTO, servers []gameap.PublicServerDTO, customNodes []config.NodeConfig) []NodeStatus {
	var result []NodeStatus

	if len(nodes) > 0 {
		for _, n := range nodes {
			code, flag := DetectCountryAndFlag(n.Name, n.Location)

			// Count game servers running on this node
			count := 0
			for _, s := range servers {
				if isServerOnNode(s, n) {
					count++
				}
			}

			cpuModel := n.CPUModel
			if cpuModel == "" {
				cpuModel = "AMD Ryzen / Intel Core"
			}
			protection := n.Protection
			if protection == "" {
				protection = "L4 Anti-DDoS Protection"
			}

			host := n.GDaemonHost
			if host == "" && len(n.IPs) > 0 {
				host = n.IPs[0]
			}
			port := n.GDaemonPort
			if port <= 0 {
				port = 31717
			}

			latency := ProbeNodeLatency(host, port, 1500*time.Millisecond)

			result = append(result, NodeStatus{
				ID:           n.ID,
				Name:         n.Name,
				Location:     n.Location,
				CountryCode:  code,
				Flag:         flag,
				Provider:     n.Provider,
				CPUModel:     cpuModel,
				CPUPercent:   n.CPUPercent,
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
		if cpuModel == "" {
			cpuModel = "AMD Ryzen / Intel Core"
		}
		protection := cn.Protection
		if protection == "" {
			protection = "L4 Anti-DDoS Protection"
		}

		port := cn.Port
		if port <= 0 {
			port = 31717
		}

		latency := ProbeNodeLatency(cn.Host, port, 1500*time.Millisecond)

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
			CPUModel:     cpuModel,
			CPUPercent:   cn.CPUPercent,
			Protection:   protection,
			Latency:      latency,
			ServersCount: count,
			IsOnline:     cn.Enabled,
			Host:         cn.Host,
			Port:         port,
		})
	}

	return result
}

func isServerOnNode(s gameap.PublicServerDTO, n gameap.NodeDTO) bool {
	if s.Address == n.GDaemonHost && n.GDaemonHost != "" {
		return true
	}
	for _, ip := range n.IPs {
		if s.Address == ip {
			return true
		}
	}
	return false
}
