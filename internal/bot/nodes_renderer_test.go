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
			OS:           "🐧 Linux",
			CPUModel:     "AMD Ryzen 9 7950X3D",
			CPUPercent:   42,
			RAMPercent:   55,
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
			OS:           "🪟 Windows",
			CPUModel:     "Intel Core i9-14900K",
			CPUPercent:   15,
			RAMPercent:   29,
			Protection:   "StormWall L3/L4/L7",
			Latency:      "Защищён (DDoS Filter)",
			ServersCount: 2,
			IsOnline:     true,
			Host:         "185.189.255.20",
			Port:         31717,
		},
	}

	t.Run("Russian language embed", func(t *testing.T) {
		embed := RenderNodesStatusEmbed(nodes, 300, "#2ecc71", "#e74c3c", "ru")
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

		// Check fields (2 nodes + 1 summary)
		if len(embed.Fields) != 3 {
			t.Fatalf("expected 3 fields, got %d", len(embed.Fields))
		}

		node1Field := embed.Fields[0]
		if !strings.Contains(node1Field.Name, "🇩🇪") || !strings.Contains(node1Field.Name, "Ryzen 9") {
			t.Errorf("unexpected node 1 field name: %q", node1Field.Name)
		}
		if !strings.Contains(node1Field.Value, "🟢 Система в норме") {
			t.Errorf("expected online status in node 1, got: %s", node1Field.Value)
		}
		if !strings.Contains(node1Field.Value, "Локация:") || !strings.Contains(node1Field.Value, "Germany, Frankfurt") {
			t.Errorf("expected Location in node 1, got: %s", node1Field.Value)
		}
		if !strings.Contains(node1Field.Value, "Провайдер:") || !strings.Contains(node1Field.Value, "Datacenter Corp") {
			t.Errorf("expected Provider in node 1, got: %s", node1Field.Value)
		}
		if !strings.Contains(node1Field.Value, "ОС:") || !strings.Contains(node1Field.Value, "Linux") {
			t.Errorf("expected OS in node 1, got: %s", node1Field.Value)
		}
		if !strings.Contains(node1Field.Value, "IP / Хост:") || !strings.Contains(node1Field.Value, "185.189.255.10") {
			t.Errorf("expected Host in node 1, got: %s", node1Field.Value)
		}
		if !strings.Contains(node1Field.Value, "Память (RAM):") || !strings.Contains(node1Field.Value, "55%") {
			t.Errorf("expected RAM in node 1, got: %s", node1Field.Value)
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
		if !strings.Contains(node2Field.Value, "🪟 Windows") {
			t.Errorf("expected Windows OS in node 2, got: %s", node2Field.Value)
		}

		summaryField := embed.Fields[2]
		if !strings.Contains(summaryField.Name, "Инфраструктура сети") {
			t.Errorf("unexpected summary field name: %q", summaryField.Name)
		}
		if !strings.Contains(summaryField.Value, "2/2") {
			t.Errorf("expected 2/2 active nodes in summary, got: %s", summaryField.Value)
		}
	})

	t.Run("English language embed", func(t *testing.T) {
		embed := RenderNodesStatusEmbed(nodes, 300, "#2ecc71", "#e74c3c", "en")
		if embed == nil {
			t.Fatal("expected embed to be non-nil")
		}

		if embed.Title != "🖥️ Dedicated Servers Status (Nodes)" {
			t.Errorf("unexpected embed title: %q", embed.Title)
		}

		if !strings.Contains(embed.Description, "Operational status of network host nodes") {
			t.Errorf("unexpected embed description: %q", embed.Description)
		}

		node1Field := embed.Fields[0]
		if !strings.Contains(node1Field.Value, "🟢 System Normal") {
			t.Errorf("expected online status in node 1, got: %s", node1Field.Value)
		}
		if !strings.Contains(node1Field.Value, "Location:") || !strings.Contains(node1Field.Value, "Germany, Frankfurt") {
			t.Errorf("expected Location in node 1, got: %s", node1Field.Value)
		}
		if !strings.Contains(node1Field.Value, "Provider:") || !strings.Contains(node1Field.Value, "Datacenter Corp") {
			t.Errorf("expected Provider in node 1, got: %s", node1Field.Value)
		}
		if !strings.Contains(node1Field.Value, "OS:") || !strings.Contains(node1Field.Value, "Linux") {
			t.Errorf("expected OS in node 1, got: %s", node1Field.Value)
		}
		if !strings.Contains(node1Field.Value, "Host / IP:") || !strings.Contains(node1Field.Value, "185.189.255.10") {
			t.Errorf("expected Host in node 1, got: %s", node1Field.Value)
		}
		if !strings.Contains(node1Field.Value, "RAM Usage:") || !strings.Contains(node1Field.Value, "55%") {
			t.Errorf("expected RAM in node 1, got: %s", node1Field.Value)
		}
		if !strings.Contains(node1Field.Value, "Ping:") || !strings.Contains(node1Field.Value, "24 ms") {
			t.Errorf("expected Ping in node 1, got: %s", node1Field.Value)
		}

		summaryField := embed.Fields[2]
		if !strings.Contains(summaryField.Name, "Network Infrastructure") {
			t.Errorf("unexpected summary field name: %q", summaryField.Name)
		}
	})

	t.Run("Empty nodes list", func(t *testing.T) {
		embedRU := RenderNodesStatusEmbed([]NodeStatus{}, 300, "#2ecc71", "#e74c3c", "ru")
		if !strings.Contains(embedRU.Fields[0].Name, "Ожидание подключения нод") {
			t.Errorf("unexpected empty state RU field name: %q", embedRU.Fields[0].Name)
		}

		embedEN := RenderNodesStatusEmbed([]NodeStatus{}, 300, "#2ecc71", "#e74c3c", "en")
		if !strings.Contains(embedEN.Fields[0].Name, "Waiting for Node Connections") {
			t.Errorf("unexpected empty state EN field name: %q", embedEN.Fields[0].Name)
		}
	})

	t.Run("Only real data rendered - no fake CPU or RAM or Protection", func(t *testing.T) {
		minimalNode := []NodeStatus{
			{
				ID:           1,
				Name:         "Dedicated Node #1",
				Location:     "Moscow",
				CountryCode:  "RU",
				Flag:         "🇷🇺",
				Provider:     "",
				OS:           "🪟 Windows",
				CPUModel:     "",
				CPUPercent:   0,
				RAMPercent:   0,
				Protection:   "",
				Latency:      "15 ms",
				ServersCount: 1,
				IsOnline:     true,
				Host:         "127.0.0.1",
				Port:         31717,
			},
		}

		embed := RenderNodesStatusEmbed(minimalNode, 300, "#2ecc71", "#e74c3c", "ru")
		if embed == nil {
			t.Fatal("expected embed to be non-nil")
		}
		field := embed.Fields[0]
		if strings.Contains(field.Value, "Нагрузка CPU:") {
			t.Errorf("expected no CPU load bar when CPUPercent == 0, got: %s", field.Value)
		}
		if strings.Contains(field.Value, "Память (RAM):") {
			t.Errorf("expected no RAM usage bar when RAMPercent == 0, got: %s", field.Value)
		}
		if strings.Contains(field.Value, "Процессор:") {
			t.Errorf("expected no CPU model when empty, got: %s", field.Value)
		}
		if strings.Contains(field.Value, "Защита:") {
			t.Errorf("expected no Protection when empty, got: %s", field.Value)
		}
		if !strings.Contains(field.Value, "127.0.0.1") {
			t.Errorf("expected host in field value, got: %s", field.Value)
		}
		if !strings.Contains(field.Value, "🪟 Windows") {
			t.Errorf("expected OS in field value, got: %s", field.Value)
		}
		if !strings.Contains(field.Value, "15 ms") {
			t.Errorf("expected Ping in field value, got: %s", field.Value)
		}
		if !strings.Contains(field.Value, "Игровых серверов:** 1 активных") {
			t.Errorf("expected servers count in field value, got: %s", field.Value)
		}
	})
}

func TestBuildNodesStatusComponents(t *testing.T) {
	tests := []struct {
		lang          string
		expectedLabel string
	}{
		{lang: "ru", expectedLabel: "Обновить статус"},
		{lang: "en", expectedLabel: "Refresh Status"},
		{lang: "", expectedLabel: "Обновить статус"},
	}

	for _, tt := range tests {
		components := BuildNodesStatusComponents(tt.lang)
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
		if btn.Label != tt.expectedLabel {
			t.Errorf("for lang %q expected Label %q, got %q", tt.lang, tt.expectedLabel, btn.Label)
		}
	}
}

func TestRenderNodesStatusEmbed_FieldCap(t *testing.T) {
	var nodes []NodeStatus
	for i := 1; i <= 30; i++ {
		nodes = append(nodes, NodeStatus{
			ID:       uint64(i),
			Name:     "Node",
			IsOnline: true,
			Latency:  "10 ms",
		})
	}

	embed := RenderNodesStatusEmbed(nodes, 300, "#2ecc71", "#e74c3c", "ru")
	if len(embed.Fields) > 25 {
		t.Fatalf("expected at most 25 fields, got %d", len(embed.Fields))
	}
	if len(embed.Fields) != 25 {
		t.Fatalf("expected 25 fields (23 nodes + 1 overflow + 1 summary), got %d", len(embed.Fields))
	}
	overflow := embed.Fields[23]
	if !strings.Contains(overflow.Name, "и ещё 7") && !strings.Contains(overflow.Value, "и ещё 7") {
		t.Errorf("expected overflow indicator for 7 nodes, got name=%q value=%q", overflow.Name, overflow.Value)
	}
	summary := embed.Fields[24]
	if !strings.Contains(summary.Name, "Инфраструктура") {
		t.Errorf("expected summary field last, got %q", summary.Name)
	}

	embedEn := RenderNodesStatusEmbed(nodes, 300, "#2ecc71", "#e74c3c", "en")
	if len(embedEn.Fields) != 25 {
		t.Fatalf("expected 25 fields for EN, got %d", len(embedEn.Fields))
	}
	if !strings.Contains(embedEn.Fields[23].Name, "and 7 more") && !strings.Contains(embedEn.Fields[23].Value, "and 7 more") {
		t.Errorf("expected EN overflow indicator for 7 nodes, got name=%q value=%q", embedEn.Fields[23].Name, embedEn.Fields[23].Value)
	}
}
