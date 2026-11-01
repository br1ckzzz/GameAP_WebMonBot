package bot

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestRenderNodesStatusEmbed(t *testing.T) {
	nodes := []NodeStatus{
		{
			ID:           1,
			Name:         "[GE] Ryzen 9 7950X3D — Frankfurt",
			Location:     "Germany, Frankfurt",
			CountryCode:  "DE",
			Flag:         "🇩🇪",
			Provider:     "Datacenter Corp",
			CPUModel:     "AMD Ryzen 9 7950X3D",
			CPUPercent:   42,
			Protection:   "L4 Anti-DDoS Protection",
			Latency:      "24 ms",
			ServersCount: 3,
			IsOnline:     true,
			Host:         "185.189.255.10",
			Port:         31717,
		},
		{
			ID:           2,
			Name:         "[RU] Core i9-14900K — Moscow",
			Location:     "Russia, Moscow",
			CountryCode:  "RU",
			Flag:         "🇷🇺",
			Provider:     "DataLine",
			CPUModel:     "Intel Core i9-14900K",
			CPUPercent:   15,
			Protection:   "StormWall L3/L4/L7",
			Latency:      "Защищён (DDoS Filter)",
			ServersCount: 2,
			IsOnline:     true,
			Host:         "185.189.255.20",
			Port:         31717,
		},
	}

	embed := RenderNodesStatusEmbed(nodes, 300, "#2ecc71", "#e74c3c")
	if embed == nil {
		t.Fatal("expected embed to be non-nil")
	}

	if embed.Title != "🖥️ Состояние выделенных серверов (Нод)" {
		t.Errorf("unexpected embed title: %q", embed.Title)
	}

	if !strings.Contains(embed.Description, "Статус работы хост-нод") {
		t.Errorf("unexpected embed description: %q", embed.Description)
	}

	// All online -> online color 0x2ecc71
	if embed.Color != 0x2ecc71 {
		t.Errorf("expected online color 0x2ecc71, got 0x%x", embed.Color)
	}

	// Check fields
	if len(embed.Fields) != 3 { // 2 node fields + 1 summary field
		t.Fatalf("expected 3 fields, got %d", len(embed.Fields))
	}

	node1Field := embed.Fields[0]
	if !strings.Contains(node1Field.Name, "🇩🇪") || !strings.Contains(node1Field.Name, "Ryzen 9") {
		t.Errorf("unexpected node 1 field name: %q", node1Field.Name)
	}
	if !strings.Contains(node1Field.Value, "🟢 Система в норме") {
		t.Errorf("expected online status in node 1, got: %s", node1Field.Value)
	}
	if !strings.Contains(node1Field.Value, "24 ms") {
		t.Errorf("expected 24 ms ping in node 1, got: %s", node1Field.Value)
	}
	if !strings.Contains(node1Field.Value, "L4 Anti-DDoS Protection") {
		t.Errorf("expected protection badge in node 1, got: %s", node1Field.Value)
	}

	node2Field := embed.Fields[1]
	if !strings.Contains(node2Field.Value, "Защищён (DDoS Filter)") {
		t.Errorf("expected DDoS filter message in node 2, got: %s", node2Field.Value)
	}

	summaryField := embed.Fields[2]
	if !strings.Contains(summaryField.Name, "Инфраструктура сети") {
		t.Errorf("unexpected summary field name: %q", summaryField.Name)
	}
	if !strings.Contains(summaryField.Value, "2/2") {
		t.Errorf("expected 2/2 active nodes in summary, got: %s", summaryField.Value)
	}
}

func TestBuildNodesStatusComponents(t *testing.T) {
	components := BuildNodesStatusComponents()
	if len(components) != 1 {
		t.Fatalf("expected 1 action row, got %d", len(components))
	}

	row, ok := components[0].(discordgo.ActionsRow)
	if !ok {
		t.Fatalf("expected discordgo.ActionsRow, got %T", components[0])
	}

	if len(row.Components) != 1 {
		t.Fatalf("expected exactly 1 button, got %d", len(row.Components))
	}

	btn, ok := row.Components[0].(discordgo.Button)
	if !ok {
		t.Fatalf("expected discordgo.Button, got %T", row.Components[0])
	}

	if btn.CustomID != "refresh_nodes_status" {
		t.Errorf("expected CustomID 'refresh_nodes_status', got %q", btn.CustomID)
	}
	if btn.Label != "Обновить статус" {
		t.Errorf("expected Label 'Обновить статус', got %q", btn.Label)
	}
}
