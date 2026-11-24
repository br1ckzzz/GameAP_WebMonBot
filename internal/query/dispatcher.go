package query

import (
	"strings"
	"time"

	"gameap-webmon-bot/internal/catalog"
)

// QueryProtocol represents the protocol type used to query a game server.
type QueryProtocol string

const (
	ProtoA2S           QueryProtocol = "a2s"
	ProtoMinecraftJava QueryProtocol = "minecraft_java"
	ProtoBedrock       QueryProtocol = "minecraft_bedrock"
	ProtoSAMP          QueryProtocol = "samp"
	ProtoQuake         QueryProtocol = "quake"
	ProtoFiveM         QueryProtocol = "fivem"
)

// ResolveProtocol determines which query protocol to use based on game code or name.
func ResolveProtocol(gameCodeOrName string) QueryProtocol {
	proto := catalog.ResolveProtocol(gameCodeOrName)
	switch strings.ToLower(proto) {
	case "samp":
		return ProtoSAMP
	case "minecraft_bedrock":
		return ProtoBedrock
	case "minecraft_java":
		return ProtoMinecraftJava
	case "quake":
		return ProtoQuake
	case "fivem":
		return ProtoFiveM
	default:
		return ProtoA2S
	}
}

// QueryServerAuto routes the query to the proper protocol engine based on game code.
// For A2S servers, it handles query port offsets with automatic fallback.
func QueryServerAuto(gameCode, address string, port int, timeout time.Duration) (*ServerInfo, error) {
	proto := ResolveProtocol(gameCode)
	start := time.Now()
	var info *ServerInfo
	var err error

	switch proto {
	case ProtoSAMP:
		info, err = QuerySAMP(address, port, timeout)

	case ProtoBedrock:
		info, err = QueryBedrock(address, port, timeout)

	case ProtoMinecraftJava:
		info, err = QueryMinecraft(address, port, timeout)

	case ProtoQuake:
		info, err = QueryQuake(address, port, timeout)

	case ProtoFiveM:
		info, err = QueryFiveM(address, port, timeout)

	case ProtoA2S:
		fallthrough
	default:
		qPort := GetQueryPort(gameCode, port)
		info, err = QueryServer(address, qPort, timeout)
		if err != nil && qPort != port {
			// Fallback to primary port
			start = time.Now()
			info, err = QueryServer(address, port, timeout)
		}
	}

	if err == nil && info != nil && info.Ping <= 0 {
		ms := int(time.Since(start).Milliseconds())
		if ms <= 0 {
			ms = 1
		}
		info.Ping = ms
	}
	return info, err
}
