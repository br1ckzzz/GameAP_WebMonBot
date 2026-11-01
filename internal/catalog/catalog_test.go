package catalog

import (
	"testing"
)

func TestCatalogInitialization(t *testing.T) {
	total := TotalGames()
	if total != 297 {
		t.Fatalf("expected 297 games in catalog, got %d", total)
	}

	// 1. Check Source/Valve game
	cs2 := GetGame("cs2")
	if cs2 == nil {
		t.Fatal("expected cs2 to be found")
	}
	if cs2.Protocol != "a2s" {
		t.Errorf("expected protocol a2s for cs2, got %s", cs2.Protocol)
	}
	if cs2.SteamAppID != 730 {
		t.Errorf("expected AppID 730 for cs2, got %d", cs2.SteamAppID)
	}

	// 2. Check Port Offset game
	valheim := GetGame("valheim")
	if valheim == nil {
		t.Fatal("expected valheim to be found")
	}
	if valheim.PortOffset != 1 {
		t.Errorf("expected port offset 1 for valheim, got %d", valheim.PortOffset)
	}
	if qp := GetQueryPort("valheim", 2456); qp != 2457 {
		t.Errorf("expected query port 2457 for valheim 2456, got %d", qp)
	}

	// 3. Check Enshrouded
	enshrouded := GetGame("enshrouded")
	if enshrouded == nil {
		t.Fatal("expected enshrouded to be found")
	}
	if enshrouded.SteamAppID != 1203620 {
		t.Errorf("expected Steam AppID 1203620 for enshrouded, got %d", enshrouded.SteamAppID)
	}
	if qp := GetQueryPort("enshrouded", 15636); qp != 15637 {
		t.Errorf("expected query port 15637 for enshrouded, got %d", qp)
	}

	// 4. Check Minecraft Java & Bedrock
	mc := GetGame("minecraft")
	if mc == nil {
		t.Fatal("expected minecraft to be found")
	}
	if mc.Protocol != "minecraft_java" {
		t.Errorf("expected minecraft_java, got %s", mc.Protocol)
	}

	bedrock := GetGame("minecraftpe")
	if bedrock == nil {
		t.Fatal("expected minecraftpe to be found")
	}
	if bedrock.Protocol != "minecraft_bedrock" {
		t.Errorf("expected minecraft_bedrock, got %s", bedrock.Protocol)
	}

	// 5. Check SA-MP
	samp := GetGame("samp")
	if samp == nil {
		t.Fatal("expected samp to be found")
	}
	if samp.Protocol != "samp" {
		t.Errorf("expected samp protocol, got %s", samp.Protocol)
	}

	// 6. Check Quake
	q3 := GetGame("quake3")
	if q3 == nil {
		t.Fatal("expected quake3 to be found")
	}
	if q3.Protocol != "quake" {
		t.Errorf("expected quake protocol, got %s", q3.Protocol)
	}

	// 7. Test Search
	results := SearchGames("strike", 5)
	if len(results) == 0 {
		t.Error("expected search results for 'strike'")
	}
}
