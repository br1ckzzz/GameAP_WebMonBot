# GameAP WebMonBot 1.0.0 Enhancements & Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement comprehensive bot enhancements for v1.0.0: Single Server Channel Binding (`/mon bind server:ID`), Game Icons Catalog Thumbnail, Direct Query Minecraft TCP Ping, models sync with GameAP WebMonitoring v1.0.9 (`category`, `uptime_seconds`, `announcement`), streamlined embed design without console command clutter and without webmonitoring links, and full documentation update in `README.md` and `DEV_BUGS_AND_FIXES.md`.

**Architecture:**
- `internal/gameap/models.go`: Add `Category`, `UptimeSeconds`, `Announcement` to DTOs.
- `internal/query/minecraft.go`: Implement lightweight TCP Server List Ping (SLP) for Minecraft servers (Handshake `0x00` + Request `0x00` or Legacy ping `0xFE`).
- `internal/query/catalog.go`: Add Game Catalog thumbnail resolver by `game_code`.
- `internal/bot/renderer.go`: Redesign Embeds with inline grid, official game icon thumbnail, status, map, players bar, uptime, category, and clean buttons; remove console command strings and web links.
- `internal/bot/state.go`: Add `ServerChannels map[uint64]string` to support binding individual servers to dedicated channels.
- `internal/bot/commands.go`: Update `/mon bind` (add `server` option), `/mon unbind` (add `server` option), `/mon bindings`, clean interaction responses.
- `internal/bot/bot.go`: Dispatch single-server cards to dedicated server channels, cleanup stale messages for server channels.
- `README.md` & `DEV_BUGS_AND_FIXES.md`: Document all features, command usages, and advantages for release 1.0.0.

**Tech Stack:** Go 1.22+, `github.com/bwmarrin/discordgo`, Standard Go TCP/UDP sockets, Standard testing suite.

---

### Task 1: 📦 Sync Models with WebMonitoring v1.0.9 (`Category`, `UptimeSeconds`, `Announcement`)

**Files:**
- Modify: `internal/gameap/models.go`
- Modify: `internal/gameap/client_test.go`

- [ ] Add `Category string `json:"category,omitempty"`` to `PublicServerDTO`.
- [ ] Add `UptimeSeconds uint64 `json:"uptime_seconds,omitempty"`` to `PublicServerDTO`.
- [ ] Add `AnnouncementDTO` struct and field `Announcement *AnnouncementDTO `json:"announcement,omitempty"`` to `MonitoringResponse`.
- [ ] Write unit test in `client_test.go` verifying JSON parsing of category, uptime, and announcement fields.
- [ ] Run `go test ./internal/gameap/...` to ensure all tests pass.

---

### Task 2: 🎮 Game Icons Catalog Resolver

**Files:**
- Create: `internal/bot/catalog.go`
- Create: `internal/bot/catalog_test.go`

- [ ] Implement `GetGameThumbnailURL(gameCode string) string` mapping known games (cs2, cstrike, tf2, rust, gmod, left4dead2, valheim, minecraft, palwrld/palworld, 7d2d, dod, cod4, ark, kf, etc.) to stable Steam CDN / public logos.
- [ ] Provide fallback placeholder icon for unknown games.
- [ ] Write unit tests in `catalog_test.go` verifying URL lookups and case-insensitivity.
- [ ] Run `go test ./internal/bot/...`.

---

### Task 3: ⛏️ Direct Query for Minecraft (TCP Server List Ping)

**Files:**
- Create: `internal/query/minecraft.go`
- Create: `internal/query/minecraft_test.go`
- Modify: `internal/query/a2s.go` (wire Minecraft into `EnrichServers`)

- [ ] Implement `QueryMinecraft(address string, port int, timeout time.Duration) (*ServerInfo, error)` using Minecraft TCP Server List Ping protocol.
- [ ] In `EnrichServers`, dispatch query based on `game_code`: call `QueryMinecraft` for `minecraft`/`mc`, call A2S for Valve Source/Rust games.
- [ ] Write unit tests for packet encoding/decoding and error handling.
- [ ] Run `go test ./internal/query/...`.

---

### Task 4: 🎨 Redesign Embeds (Clean Grid, Thumbnails, No Console Command Clutter, No Web Links)

**Files:**
- Modify: `internal/bot/renderer.go`
- Modify: `internal/bot/renderer_test.go`

- [ ] Update `RenderGameGroupEmbed`:
  - Add game thumbnail via `GetGameThumbnailURL`.
  - Remove console command strings (`connect IP:PORT`) from description and fields.
  - Remove webmonitoring link from description/fields.
  - Display server status, address `` `ip:port` ``, map, player progress bar, uptime (`⏱️ 14д 06ч`), category (`🏷️ Public`).
  - Set footer to clean relative timestamp `<t:TIMESTAMP:R>`.
- [ ] Implement `RenderSingleServerEmbed(server gameap.PublicServerDTO, cfg *config.Config) *discordgo.MessageEmbed` for individual server channels.
- [ ] In `BuildServerButtons`, ensure buttons are clean: `[ 🎮 Подключиться ]` and `[ 🔄 Обновить ]`.
- [ ] In `commands.go`, clean up the click interaction modal/response so it shows the connection info cleanly without clutter.
- [ ] Update and expand `renderer_test.go`.
- [ ] Run `go test ./internal/bot/...`.

---

### Task 5: 🎯 Single Server Channel Binding (`/mon bind server:ID`)

**Files:**
- Modify: `internal/bot/state.go`
- Modify: `internal/bot/state_test.go`
- Modify: `internal/bot/commands.go`
- Modify: `internal/bot/bot.go`

- [ ] In `State`:
  - Add `ServerChannels map[uint64]string `json:"server_channels,omitempty"`` (maps `server.id` -> `channel_id`).
  - Add methods `BindServerChannel(guildID string, serverID uint64, channelID string) error`, `UnbindServerChannel(serverID uint64) error`, `GetServerChannel(serverID uint64) string`, `GetServerChannels() map[uint64]string`.
- [ ] In `commands.go`:
  - Add option `server` (Integer) to `/mon bind` subcommand.
  - Add option `server` (Integer) to `/mon unbind` subcommand.
  - Update `/mon bindings` to list both game bindings and individual server bindings.
- [ ] In `bot.go`:
  - In `updateDashboard`, dispatch `RenderSingleServerEmbed` to channels bound via `ServerChannels`.
  - Clean up stale messages for server channels when servers are removed or re-bound.
- [ ] Update `state_test.go` with server channel test cases.
- [ ] Run `go test ./internal/bot/...`.

---

### Task 6: 🌐 Clean Canonical Web URL & Polish

**Files:**
- Modify: `internal/bot/bot.go`
- Modify: `internal/config/config.go`

- [ ] Remove `strings.Replace(webURL, "/api/plugins/monitoring", "/api/plugins/monitoring/view", 1)` in `bot.go`.
- [ ] Ensure all channel sync and purge operations handle rate limits cleanly.
- [ ] Run full test suite: `go test -v ./...`.

---

### Task 7: 📚 Update Documentation (`README.md`, `DEV_BUGS_AND_FIXES.md`) & Build Verification

**Files:**
- Modify: `README.md`
- Modify: `DEV_BUGS_AND_FIXES.md`

- [ ] In `README.md`:
  - Document `/mon bind server:1 channel:#cs2-mirage` (индивидуальные серверные каналы).
  - Document Direct Query for Minecraft & Source.
  - Document game thumbnails and metadata support (`uptime`, `category`).
  - Synchronize English and Russian sections.
  - Keep version strictly as **1.0.0**.
- [ ] In `DEV_BUGS_AND_FIXES.md`:
  - Add summary of all v1.0.0 enhancements and optimizations.
- [ ] Run build test via `build.ps1` (or `go build ./cmd/bot`) to ensure binary compiles cleanly without warnings.
