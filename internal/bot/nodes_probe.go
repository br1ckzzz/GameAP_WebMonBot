package bot

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
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
func ProbeNodeLatency(ctx context.Context, host string, port int, timeout time.Duration, lang string) string {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.ToLower(lang) != "en" {
		lang = "ru"
	}
	protectedMsg := "Защищён (DDoS Filter)"
	if lang == "en" {
		protectedMsg = "Protected (DDoS Filter)"
	}

	if ctx.Err() != nil {
		return protectedMsg
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
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
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
	quickDialer := net.Dialer{Timeout: quickTimeout}
	for _, fbPort := range fallbackPorts {
		if ctx.Err() != nil {
			return protectedMsg
		}
		if fbPort == port {
			continue
		}
		fbAddr := net.JoinHostPort(host, strconv.Itoa(fbPort))
		fbStart := time.Now()
		fbConn, fbErr := quickDialer.DialContext(ctx, "tcp", fbAddr)
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

const maxNodeProbeConcurrency = 8

// EnrichNodeStatuses converts GameAP nodes or custom config nodes into rich Discord NodeStatuses.
func EnrichNodeStatuses(ctx context.Context, nodes []gameap.NodeDTO, servers []gameap.PublicServerDTO, customNodes []config.NodeConfig, lang string) []NodeStatus {
	if ctx == nil {
		ctx = context.Background()
	}

	if strings.ToLower(lang) != "en" {
		lang = "ru"
	}

	sem := make(chan struct{}, maxNodeProbeConcurrency)

	if len(nodes) > 0 {
		result := make([]NodeStatus, len(nodes))
		var wg sync.WaitGroup
		for i, n := range nodes {
			wg.Add(1)
			go func(idx int, node gameap.NodeDTO) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				code, flag := DetectCountryAndFlag(node.Name, node.Location)

				// Count game servers running on this node
				count := 0
				for _, s := range servers {
					if isServerOnNode(s, node, len(nodes)) {
						count++
					}
				}

				host := node.GDaemonHost
				if host == "" && len(node.IPs) > 0 {
					host = node.IPs[0]
				}
				port := node.GDaemonPort
				if port <= 0 {
					port = 31717
				}

				latency := ProbeNodeLatency(ctx, host, port, 1500*time.Millisecond, lang)

				result[idx] = NodeStatus{
					ID:           node.ID,
					Name:         node.Name,
					Location:     node.Location,
					CountryCode:  code,
					Flag:         flag,
					Provider:     node.Provider,
					OS:           FormatOS(node.OS),
					CPUModel:     node.CPUModel,
					CPUPercent:   node.CPUPercent,
					RAMPercent:   node.RAMPercent,
					Protection:   node.Protection,
					Latency:      latency,
					ServersCount: count,
					IsOnline:     node.Enabled,
					Host:         host,
					Port:         port,
				}
			}(i, n)
		}
		wg.Wait()
		return result
	}

	// Fallback to custom config nodes if GameAP returned no nodes
	if len(customNodes) > 0 {
		result := make([]NodeStatus, len(customNodes))
		var wg sync.WaitGroup
		for i, cn := range customNodes {
			wg.Add(1)
			go func(idx int, cfgNode config.NodeConfig) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				code, flag := DetectCountryAndFlag(cfgNode.Name, cfgNode.Location)

				count := cfgNode.ServersCount
				if count == 0 && len(servers) > 0 {
					for _, s := range servers {
						if s.Address == cfgNode.Host {
							count++
						}
					}
				}

				port := cfgNode.Port
				if port <= 0 {
					port = 31717
				}

				latency := ProbeNodeLatency(ctx, cfgNode.Host, port, 1500*time.Millisecond, lang)

				id := cfgNode.ID
				if id == 0 {
					id = uint64(idx + 1)
				}

				result[idx] = NodeStatus{
					ID:           id,
					Name:         cfgNode.Name,
					Location:     cfgNode.Location,
					CountryCode:  code,
					Flag:         flag,
					Provider:     cfgNode.Provider,
					OS:           FormatOS(cfgNode.OS),
					CPUModel:     cfgNode.CPUModel,
					CPUPercent:   cfgNode.CPUPercent,
					RAMPercent:   cfgNode.RAMPercent,
					Protection:   cfgNode.Protection,
					Latency:      latency,
					ServersCount: count,
					IsOnline:     cfgNode.Enabled,
					Host:         cfgNode.Host,
					Port:         port,
				}
			}(i, cn)
		}
		wg.Wait()
		return result
	}

	// Fallback: discover nodes from unique server addresses if both GameAP nodes and customNodes are empty
	if len(servers) > 0 {
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

		result := make([]NodeStatus, len(order))
		var wg sync.WaitGroup
		for idx, addr := range order {
			wg.Add(1)
			go func(i int, hostAddr string, srvCount int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				nodeName := fmt.Sprintf("GameAP Dedicated Host [%s]", hostAddr)
				code, flag := DetectCountryAndFlag(nodeName, "")
				latency := ProbeNodeLatency(ctx, hostAddr, 31717, 1500*time.Millisecond, lang)
				location := "Dedicated Server"
				if lang == "ru" {
					location = "Выделенный сервер"
				}
				result[i] = NodeStatus{
					ID:           uint64(i + 1),
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
					ServersCount: srvCount,
					IsOnline:     true,
					Host:         hostAddr,
					Port:         31717,
				}
			}(idx, addr, hostMap[addr].count)
		}
		wg.Wait()
		return result
	}

	return nil
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
