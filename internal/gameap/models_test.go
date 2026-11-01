package gameap

import (
	"encoding/json"
	"testing"
)

func TestMonitoringResponseWithNodes(t *testing.T) {
	payload := []byte(`{
		"success": true,
		"total_servers": 2,
		"online_count": 2,
		"servers": [],
		"nodes": [
			{
				"id": 1,
				"name": "[GE] Ryzen 9 7950X3D — Frankfurt",
				"location": "Germany, Frankfurt",
				"provider": "Datacenter Corp",
				"ips": ["185.189.255.10"],
				"gdaemon_host": "185.189.255.10",
				"gdaemon_port": 31717,
				"cpu_model": "AMD Ryzen 9 7950X3D",
				"cpu_percent": 42,
				"protection": "L4 Anti-DDoS Protection",
				"enabled": true
			}
		],
		"timestamp": 1700000000
	}`)

	var resp MonitoringResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		t.Fatalf("failed to unmarshal MonitoringResponse with nodes: %v", err)
	}

	if len(resp.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(resp.Nodes))
	}

	node := resp.Nodes[0]
	if node.ID != 1 || node.Name != "[GE] Ryzen 9 7950X3D — Frankfurt" {
		t.Errorf("unexpected node: %+v", node)
	}
	if node.CPUPercent != 42 || node.Protection != "L4 Anti-DDoS Protection" {
		t.Errorf("unexpected node metrics: CPU=%d, Protection=%s", node.CPUPercent, node.Protection)
	}
}
