package query

import (
	"testing"
	"time"
)

func TestQueryServerAutoRouter(t *testing.T) {
	// Protocol resolution tests
	protoSAMP := ResolveProtocol("samp")
	if protoSAMP != ProtoSAMP {
		t.Errorf("expected ProtoSAMP, got %v", protoSAMP)
	}

	protoCRMP := ResolveProtocol("crmp")
	if protoCRMP != ProtoSAMP {
		t.Errorf("expected ProtoSAMP for crmp, got %v", protoCRMP)
	}

	protoBedrock := ResolveProtocol("bedrock")
	if protoBedrock != ProtoBedrock {
		t.Errorf("expected ProtoBedrock, got %v", protoBedrock)
	}

	protoMC := ResolveProtocol("minecraft")
	if protoMC != ProtoMinecraftJava {
		t.Errorf("expected ProtoMinecraftJava, got %v", protoMC)
	}

	protoQuake := ResolveProtocol("quake3")
	if protoQuake != ProtoQuake {
		t.Errorf("expected ProtoQuake, got %v", protoQuake)
	}

	protoFiveM := ResolveProtocol("fivem")
	if protoFiveM != ProtoFiveM {
		t.Errorf("expected ProtoFiveM, got %v", protoFiveM)
	}

	protoCS2 := ResolveProtocol("cs2")
	if protoCS2 != ProtoA2S {
		t.Errorf("expected ProtoA2S for cs2, got %v", protoCS2)
	}

	protoValheim := ResolveProtocol("valheim")
	if protoValheim != ProtoA2S {
		t.Errorf("expected ProtoA2S for valheim, got %v", protoValheim)
	}

	// Verify QueryServerAuto fails gracefully on unreachable IP with short timeout
	_, err := QueryServerAuto("cs2", "127.0.0.1", 54321, 50*time.Millisecond)
	if err == nil {
		t.Errorf("expected error on unreachable port, got nil")
	}
}
