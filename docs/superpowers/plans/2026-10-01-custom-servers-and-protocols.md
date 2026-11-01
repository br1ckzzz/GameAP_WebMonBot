# Расширенные сетевые протоколы и внешние серверы (Custom Servers) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Реализовать поддержку сетевых протоколов (SA-MP/CR-MP, Minecraft Bedrock RakNet, Quake/GameSpy, FiveM), единый маршрутизатор опроса, и полнофункциональное добавление/удаление внешних игровых серверов в боте через слэш-команды `/mon add`, `/mon remove`, `/mon list`.

**Architecture:** В `internal/query` создаются чистые Go-реализации UDP и HTTP протоколов и диспетчер `QueryServerAuto`. В `internal/bot/state.go` добавляется модель `CustomServer` с потокобезопасным хранением в `state.json`. В `internal/bot/bot.go` опрос внешних серверов встраивается в общий цикл `RefreshDashboard`, а в `internal/bot/commands.go` регистрируются команды `/mon add`, `/mon remove`, `/mon list`.

**Tech Stack:** Go 1.22+, `net` (UDP/TCP), `net/http`, DiscordGo, `encoding/binary`, `encoding/json`.

**Spec:** [docs/superpowers/specs/2026-10-01-custom-servers-and-protocols-design.md](file:///f:/GameAP_WebMonBot/docs/superpowers/specs/2026-10-01-custom-servers-and-protocols-design.md)

## Global Constraints
- Версия бота строго зафиксирована на **1.0.0** (без изменения версии).
- Бот остаётся полностью автономным одиночным бинарником на чистом Go без сторонних внешних демонов (Node.js, Python) и без CGo (`CGO_ENABLED=0`).
- Плагин `GameAP_WebMonitoring` не затрагивается (функционал внешних серверов реализуется исключительно на стороне бота).
- Все существующие тесты `go test ./...` должны проходить успешно.

## Review Focus
- Некорректный IP:порт или недоступный хост при `/mon add` не должен приводить к зависанию бота или панике.
- Таймауты при опросе UDP пакетов (SA-MP, Bedrock, Quake) должны строго соблюдаться (не более указанного timeout).
- Внешние серверы должны корректно сохраняться и восстанавливаться из `state.json` после перезапуска бота.
- Недоступность панели GameAP не должна блокировать мониторинг внешних серверов.
- Подключение и кнопки `[🎮 Подключиться]` и `[🔄 Обновить]` должны работать для внешних серверов без ошибок.

---

### Task 1: SA-MP / CR-MP UDP Query Protocol

**Files:**
- Create: `internal/query/samp.go`
- Test: `internal/query/samp_test.go`

**Interfaces:**
- Produces: `func QuerySAMP(address string, port int, timeout time.Duration) (*ServerInfo, error)`

- [ ] **Step 1: Write the unit test with mock UDP server**

```go
package query

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestQuerySAMP(t *testing.T) {
	// Start mock SAMP UDP server
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer pc.Close()

	addr := pc.LocalAddr().(*net.UDPAddr)

	go func() {
		buf := make([]byte, 1024)
		n, clientAddr, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		if n >= 11 && string(buf[:4]) == "SAMP" && buf[10] == 'i' {
			var resp bytes.Buffer
			resp.Write(buf[:11]) // echo header
			resp.WriteByte(0)    // password: false
			_ = binary.Write(&resp, binary.LittleEndian, uint16(42))  // players
			_ = binary.Write(&resp, binary.LittleEndian, uint16(100)) // max players

			name := []byte("My SA-MP RolePlay")
			_ = binary.Write(&resp, binary.LittleEndian, uint32(len(name)))
			resp.Write(name)

			gamemode := []byte("Roleplay v2")
			_ = binary.Write(&resp, binary.LittleEndian, uint32(len(gamemode)))
			resp.Write(gamemode)

			mapname := []byte("San Andreas")
			_ = binary.Write(&resp, binary.LittleEndian, uint32(len(mapname)))
			resp.Write(mapname)

			_, _ = pc.WriteTo(resp.Bytes(), clientAddr)
		}
	}()

	info, err := QuerySAMP(addr.IP.String(), addr.Port, 1*time.Second)
	if err != nil {
		t.Fatalf("QuerySAMP failed: %v", err)
	}
	if info.Name != "My SA-MP RolePlay" {
		t.Errorf("expected name 'My SA-MP RolePlay', got '%s'", info.Name)
	}
	if info.Players != 42 || info.MaxPlayers != 100 {
		t.Errorf("expected players 42/100, got %d/%d", info.Players, info.MaxPlayers)
	}
	if info.Map != "San Andreas" {
		t.Errorf("expected map 'San Andreas', got '%s'", info.Map)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/query -run TestQuerySAMP`
Expected: FAIL with undefined: `QuerySAMP`

- [ ] **Step 3: Implement QuerySAMP**

Create `internal/query/samp.go` with UDP client parsing opcode `'i'`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/query -run TestQuerySAMP`
Expected: PASS

---

### Task 2: Minecraft Bedrock RakNet UDP Protocol

**Files:**
- Create: `internal/query/bedrock.go`
- Test: `internal/query/bedrock_test.go`

**Interfaces:**
- Produces: `func QueryBedrock(address string, port int, timeout time.Duration) (*ServerInfo, error)`

- [ ] **Step 1: Write the unit test with mock Bedrock UDP server**

```go
package query

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestQueryBedrock(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer pc.Close()

	addr := pc.LocalAddr().(*net.UDPAddr)

	go func() {
		buf := make([]byte, 1024)
		n, clientAddr, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		if n >= 33 && buf[0] == 0x01 { // Unconnected Ping
			var resp bytes.Buffer
			resp.WriteByte(0x1C) // Unconnected Pong
			resp.Write(buf[1:9]) // echo timestamp
			_ = binary.Write(&resp, binary.BigEndian, uint64(123456789))
			resp.Write(raknetMagic)

			payload := "MCPE;Survival Bedrock Server;589;1.20.50;15;50;123456789;Bedrock World;Survival;1;19132;19133"
			_ = binary.Write(&resp, binary.BigEndian, uint16(len(payload)))
			resp.WriteString(payload)

			_, _ = pc.WriteTo(resp.Bytes(), clientAddr)
		}
	}()

	info, err := QueryBedrock(addr.IP.String(), addr.Port, 1*time.Second)
	if err != nil {
		t.Fatalf("QueryBedrock failed: %v", err)
	}
	if info.Name != "Survival Bedrock Server" {
		t.Errorf("expected name 'Survival Bedrock Server', got '%s'", info.Name)
	}
	if info.Players != 15 || info.MaxPlayers != 50 {
		t.Errorf("expected players 15/50, got %d/%d", info.Players, info.MaxPlayers)
	}
	if info.Map != "Bedrock World" {
		t.Errorf("expected map 'Bedrock World', got '%s'", info.Map)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/query -run TestQueryBedrock`
Expected: FAIL with undefined: `QueryBedrock`

- [ ] **Step 3: Implement QueryBedrock**

Create `internal/query/bedrock.go` implementing RakNet Unconnected Ping `0x01` and parsing Unconnected Pong `0x1c`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/query -run TestQueryBedrock`
Expected: PASS

---

### Task 3: Quake 2/3 / GameSpy UDP Protocol

**Files:**
- Create: `internal/query/quake.go`
- Test: `internal/query/quake_test.go`

**Interfaces:**
- Produces: `func QueryQuake(address string, port int, timeout time.Duration) (*ServerInfo, error)`

- [ ] **Step 1: Write the unit test with mock Quake server**

```go
package query

import (
	"net"
	"testing"
	"time"
)

func TestQueryQuake(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer pc.Close()

	addr := pc.LocalAddr().(*net.UDPAddr)

	go func() {
		buf := make([]byte, 1024)
		_, clientAddr, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		resp := "\xFF\xFF\xFF\xFFstatusResponse\n\\hostname\\Quake Arena Server\\mapname\\q3dm6\\sv_maxclients\\16\\gamename\\baseq3\n15 50 \"PlayerOne\"\n20 40 \"PlayerTwo\"\n"
		_, _ = pc.WriteTo([]byte(resp), clientAddr)
	}()

	info, err := QueryQuake(addr.IP.String(), addr.Port, 1*time.Second)
	if err != nil {
		t.Fatalf("QueryQuake failed: %v", err)
	}
	if info.Name != "Quake Arena Server" {
		t.Errorf("expected name 'Quake Arena Server', got '%s'", info.Name)
	}
	if info.Map != "q3dm6" {
		t.Errorf("expected map 'q3dm6', got '%s'", info.Map)
	}
	if info.Players != 2 || info.MaxPlayers != 16 {
		t.Errorf("expected players 2/16, got %d/%d", info.Players, info.MaxPlayers)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/query -run TestQueryQuake`
Expected: FAIL with undefined: `QueryQuake`

- [ ] **Step 3: Implement QueryQuake**

Create `internal/query/quake.go` sending `\xFF\xFF\xFF\xFFgetstatus\n` and parsing CVAR dictionary and player lines.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/query -run TestQueryQuake`
Expected: PASS

---

### Task 4: FiveM HTTP Query Protocol

**Files:**
- Create: `internal/query/fivem.go`
- Test: `internal/query/fivem_test.go`

**Interfaces:**
- Produces: `func QueryFiveM(address string, port int, timeout time.Duration) (*ServerInfo, error)`

- [ ] **Step 1: Write the unit test with mock HTTP server**

```go
package query

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestQueryFiveM(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dynamic.json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"hostname":"Los Santos RP FiveM","mapname":"San Andreas","clients":64,"sv_maxclients":128,"gametype":"Roleplay"}`))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	parts := strings.Split(strings.TrimPrefix(ts.URL, "http://"), ":")
	port, _ := strconv.Atoi(parts[1])

	info, err := QueryFiveM(parts[0], port, 1*time.Second)
	if err != nil {
		t.Fatalf("QueryFiveM failed: %v", err)
	}
	if info.Name != "Los Santos RP FiveM" {
		t.Errorf("expected name 'Los Santos RP FiveM', got '%s'", info.Name)
	}
	if info.Players != 64 || info.MaxPlayers != 128 {
		t.Errorf("expected players 64/128, got %d/%d", info.Players, info.MaxPlayers)
	}
	if info.Map != "San Andreas" {
		t.Errorf("expected map 'San Andreas', got '%s'", info.Map)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/query -run TestQueryFiveM`
Expected: FAIL with undefined: `QueryFiveM`

- [ ] **Step 3: Implement QueryFiveM**

Create `internal/query/fivem.go` querying `/dynamic.json` with fallback to `/info.json` & `/players.json`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/query -run TestQueryFiveM`
Expected: PASS

---

### Task 5: Query Dispatcher & Game Code Router

**Files:**
- Create: `internal/query/dispatcher.go`
- Test: `internal/query/dispatcher_test.go`

**Interfaces:**
- Produces: `func QueryServerAuto(gameCode, address string, port int, timeout time.Duration) (*ServerInfo, error)`

- [ ] **Step 1: Write test for QueryServerAuto router**

Test routing for different game codes (`samp`, `bedrock`, `mc`, `quake3`, `fivem`, `cs2`).

- [ ] **Step 2: Implement dispatcher.go**

Route to appropriate protocol function, with automatic fallback to A2S query.

- [ ] **Step 3: Run tests to verify it passes**

Run: `go test ./internal/query -run TestQueryServerAuto`
Expected: PASS

---

### Task 6: Custom Server Data Model & State Persistence

**Files:**
- Modify: `internal/bot/state.go`
- Test: `internal/bot/state_test.go`

**Interfaces:**
- Produces:
  - `type CustomServer struct`
  - `func (s *State) AddCustomServer(srv CustomServer) error`
  - `func (s *State) RemoveCustomServer(addrPort string) bool`
  - `func (s *State) GetCustomServers() []CustomServer`

- [ ] **Step 1: Write test for CustomServer state persistence**

Verify adding, removing, and retrieving custom servers, ensuring `state.json` saves and reloads properly.

- [ ] **Step 2: Implement CustomServer methods in state.go**

Add `CustomServers map[string]CustomServer` and thread-safe CRUD methods with auto-save.

- [ ] **Step 3: Run test to verify it passes**

Run: `go test ./internal/bot -run TestCustomServerState`
Expected: PASS

---

### Task 7: Monitoring Loop Integration for Custom Servers

**Files:**
- Modify: `internal/bot/bot.go:160-230`

- [ ] **Step 1: Update RefreshDashboard in bot.go**

1. Query all `CustomServers` in parallel via `QueryServerAuto`.
2. Convert results into `gameap.PublicServerDTO` with `online` or `offline` status.
3. Merge custom servers into the overall server list before passing to `alerts.ProcessUpdates` and `updateDashboard`.
4. Allow `RefreshDashboard` to succeed even if GameAP API fails, provided custom servers are monitored.

- [ ] **Step 2: Run unit tests**

Run: `go test ./internal/bot/...`
Expected: PASS

---

### Task 8: Discord Slash Commands `/mon add`, `/mon remove`, `/mon list`

**Files:**
- Modify: `internal/bot/commands.go`

- [ ] **Step 1: Register new subcommands under /mon in GetSlashCommands**
  - `add`: Options `address` (required string), `game` (required string), `name` (optional string).
  - `remove`: Options `address` (required string).
  - `list`: No options.

- [ ] **Step 2: Implement command handlers**
  - `handleMonAdd`: Parse address:port, normalize game code/name, probe server immediately, save to state, trigger dashboard refresh, send success embed.
  - `handleMonRemove`: Remove by address:port from state, trigger dashboard refresh, send confirmation.
  - `handleMonList`: Show ephemeral embed listing all custom servers, their game, status, and address.

- [ ] **Step 3: Run unit tests**

Run: `go test ./internal/bot/...`
Expected: PASS

---

### Task 9: End-to-end Verification & Production Build

**Files:**
- Verify: all unit tests `go test ./...`
- Build: `bin/webmonbot.exe` (Windows) and `bin/webmonbot-linux-amd64` (Linux)

- [ ] **Step 1: Run all unit tests**

Run: `go test ./... -v`
Expected: 100% PASS

- [ ] **Step 2: Compile Windows binary**

Run: `go build -ldflags "-s -w" -o bin/webmonbot.exe ./cmd/bot`
Verify: `bin/webmonbot.exe` generated cleanly.

- [ ] **Step 3: Cross-compile Linux binary**

Run: `powershell -Command "$env:GOOS='linux'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'; go build -ldflags '-s -w' -o bin/webmonbot-linux-amd64 ./cmd/bot"`
Verify: `bin/webmonbot-linux-amd64` generated cleanly.
