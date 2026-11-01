# Мониторинг выделенных серверов (Нод) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Реализовать отдельный канал в Discord для мониторинга выделенных серверов (нод) сети с автообновлением каждые 5 минут, автоопределением стран/флагов, защитой от ложных оффлайн-статусов из-за Anti-DDoS и интерактивной кнопкой обновления в дизайне референса `media_1793557622986.png`.

**Architecture:** Модуль обогащения нод (`internal/bot/nodes_probe.go`) сопоставляет ноды из GameAP API (или standalone config) с запущенными игровыми серверами, определяет страну/флаг ноды по тегу/локации и выполняет безопасный опрос задержки. Модуль визуализации (`internal/bot/nodes_renderer.go`) собирает премиальный Discord Embed с ASCII-барами нагрузки процессоров и кнопкой обновления. Сервисный цикл (`internal/bot/bot.go`) выполняет in-place редактирование сообщения по таймеру и по клику на кнопку с сохранением ID в `state.json`.

**Tech Stack:** Go 1.22+, `github.com/bwmarrin/discordgo`, GameAP API / Rust WASM companion plugin.

**Spec:** [docs/superpowers/specs/2026-11-01-nodes-status-monitoring-design.md](file:///f:/GameAP_WebMonBot/docs/superpowers/specs/2026-11-01-nodes-status-monitoring-design.md)

## Global Constraints

- Версия бота зафиксирована на `1.0.0` (не менять версию в баннерах и файлах).
- Никаких внешних демонов: бот компилируется в автономный статический исполняемый файл без CGo.
- Пинг-таймаут фаервола/DDoS НЕ должен помечать ноду как оффлайн: авторитетным источником статуса является только GameAP, а пинг выводит `Защищён (DDoS Filter)`.
- Кнопка перехода в панель управления убрана: оставляем только кнопку `[ 🔄 Обновить статус ]` (`refresh_nodes_status`).
- Флаги определяются автоматически по тегу `[TAG]` (например, `[GE]`, `[RU]`) с фоллбэком на анализ `location` и дефолт `🌐`.

## Review Focus

1. **Anti-DDoS Firewall Timeout:** При таймауте или сбросе TCP-соединения статус ноды обязан оставаться `🟢 Система в норме`, а поле пинга — `Защищён (DDoS Filter)`.
2. **Missing Tag in Node Name:** Если в названии ноды нет тега `[XX]`, но в `location` указан город/страна ("Frankfurt"), флаг обязан определиться корректно (`🇩🇪`).
3. **Unknown Location & Tag:** Если нет ни тега, ни распознанной локации ("Node-01"), парсер обязан вернуть дефолтный символ `🌐` без паники.
4. **GameAP API Down / Standalone Fallback:** При недоступности панели GameAP опрос нод переключается на массив `nodes` из `config.json` без падений бота.
5. **Deleted Discord Message Recovery:** Если сохранённое сообщение в Discord было удалено пользователем вручную, бот обязан перехватить ошибку 404/Unknown Message, создать новое сообщение и сохранить новый ID в `state.json`.

---

### Task 1: Конфигурация и модели данных (Config & DTO Models)

**Files:**
- Modify: `internal/config/config.go:30-80`
- Modify: `internal/gameap/models.go:1-50`
- Test: `internal/config/config_test.go`
- Test: `internal/gameap/models_test.go`

**Interfaces:**
- Consumes: Стандартные структуры `Config`, `DiscordConfig`, `MonitoringResponse`.
- Produces:
  - `config.DiscordConfig.NodesChannelID string`
  - `config.DiscordConfig.NodesUpdateIntervalSeconds int` (дефолт 300)
  - `config.NodeConfig` struct (`ID`, `Name`, `Location`, `CPUModel`, `CPUPercent`, `Protection`, `Host`, `Port`, `Enabled`)
  - `config.Config.Nodes []NodeConfig`
  - `gameap.NodeDTO` struct
  - `gameap.MonitoringResponse.Nodes []NodeDTO`

- [ ] **Step 1: Написать падающий тест для новых полей конфигурации и моделей**

```go
// internal/config/config_test.go
func TestNodesConfigParsing(t *testing.T) {
    jsonCfg := []byte(`{
        "discord": {
            "nodes_channel_id": "999888777",
            "nodes_update_interval_seconds": 300
        },
        "nodes": [
            {
                "id": 1,
                "name": "[GE] Ryzen 9 7950X3D — Frankfurt",
                "location": "Germany, Frankfurt",
                "cpu_model": "AMD Ryzen 9 7950X3D",
                "cpu_percent": 42,
                "protection": "L4 Anti-DDoS Protection",
                "host": "185.189.255.10",
                "port": 31717,
                "enabled": true
            }
        ]
    }`)
    cfg, err := LoadConfigFromBytes(jsonCfg)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if cfg.Discord.NodesChannelID != "999888777" {
        t.Errorf("expected nodes_channel_id 999888777, got %s", cfg.Discord.NodesChannelID)
    }
    if cfg.Discord.NodesUpdateIntervalSeconds != 300 {
        t.Errorf("expected 300 seconds, got %d", cfg.Discord.NodesUpdateIntervalSeconds)
    }
    if len(cfg.Nodes) != 1 || cfg.Nodes[0].CPUModel != "AMD Ryzen 9 7950X3D" {
        t.Errorf("expected 1 node with Ryzen 9, got %+v", cfg.Nodes)
    }
}
```

- [ ] **Step 2: Запустить тест и убедиться, что он падает**

Run: `go test -v ./internal/config`
Expected: FAIL (неизвестные поля `NodesChannelID`, `NodesUpdateIntervalSeconds`, `Nodes`)

- [ ] **Step 3: Реализовать расширения конфигурации и моделей**

В `internal/config/config.go` добавить `NodesChannelID`, `NodesUpdateIntervalSeconds` (с дефолтом 300 при 0), `NodeConfig` и `Nodes []NodeConfig`.
В `internal/gameap/models.go` добавить `NodeDTO` и поле `Nodes []NodeDTO` в `MonitoringResponse`.

- [ ] **Step 4: Запустить тесты и убедиться, что они проходят**

Run: `go test -v ./internal/config ./internal/gameap`
Expected: PASS

- [ ] **Step 5: Закоммитить изменения**

```bash
git add internal/config internal/gameap
git commit -m "feat(config): add nodes monitoring configuration and models"
```

---

### Task 2: Парсер флагов стран и безопасный опрос задержки (Nodes Probe)

**Files:**
- Create: `internal/bot/nodes_probe.go`
- Create: `internal/bot/nodes_probe_test.go`

**Interfaces:**
- Consumes: `gameap.NodeDTO`, `gameap.ServerStatusDTO`, `config.NodeConfig`.
- Produces:
  - `DetectCountryAndFlag(name, location string) (countryCode, flag string)`
  - `ProbeNodeLatency(host string, port int, timeout time.Duration) string`
  - `type NodeStatus struct { ... }`
  - `EnrichNodeStatuses(nodes []gameap.NodeDTO, servers []gameap.ServerStatusDTO, customNodes []config.NodeConfig) []NodeStatus`

- [ ] **Step 1: Написать падающие тесты для `DetectCountryAndFlag`, `ProbeNodeLatency` и `EnrichNodeStatuses`**

```go
// internal/bot/nodes_probe_test.go
package bot

import (
    "testing"
    "time"
)

func TestDetectCountryAndFlag(t *testing.T) {
    tests := []struct {
        name     string
        location string
        wantCode string
        wantFlag string
    }{
        {"[GE] Ryzen 9 7950X3D — Frankfurt", "Frankfurt", "DE", "🇩🇪"},
        {"[DE] Node 2", "", "DE", "🇩🇪"},
        {"[RU] Moscow Core Server", "Moscow", "RU", "🇷🇺"},
        {"[KZ] Almaty High-Freq", "Kazakhstan", "KZ", "🇰🇿"},
        {"No Tag Server", "Germany, Frankfurt", "DE", "🇩🇪"},
        {"Unknown Server", "Secret Bunker", "GLOBAL", "🌐"},
    }

    for _, tt := range tests {
        code, flag := DetectCountryAndFlag(tt.name, tt.location)
        if code != tt.wantCode || flag != tt.wantFlag {
            t.Errorf("DetectCountryAndFlag(%q, %q) = (%q, %q), want (%q, %q)",
                tt.name, tt.location, code, flag, tt.wantCode, tt.wantFlag)
        }
    }
}

func TestProbeNodeLatency_DDoSFilterFallback(t *testing.T) {
    // Недоступный порт не должен вызывать панику и должен возвращать статус защиты
    latency := ProbeNodeLatency("127.0.0.1", 59999, 50*time.Millisecond)
    if latency != "Защищён (DDoS Filter)" {
        t.Errorf("expected DDoS filter fallback, got %q", latency)
    }
}
```

- [ ] **Step 2: Запустить тест и убедиться, что он падает**

Run: `go test -v ./internal/bot -run TestDetectCountryAndFlag`
Expected: FAIL (функции не определены)

- [ ] **Step 3: Реализовать `nodes_probe.go`**

1. `DetectCountryAndFlag`: регулярка `\[([A-Za-z]{2,3})\]` с нормализацией (`GE` -> `DE`), словарь флагов (`DE`, `RU`, `KZ`, `NL`, `FI`, `US`, `FR`, `PL`, `UA`, `GB`), затем поиск по ключевым словам в `location` и `name`, дефолт `🌐`.
2. `ProbeNodeLatency`: TCP `net.DialTimeout`. При успехе: `fmt.Sprintf("%d ms", duration.Milliseconds())`. При ошибке или таймауте: `"Защищён (DDoS Filter)"`.
3. `EnrichNodeStatuses`: объединение `NodeDTO` и `NodeConfig`, подсчет игровых серверов для каждой ноды (`server.NodeID == node.ID` или совпадение IP), вызов `ProbeNodeLatency`, сохранение `Online: node.Enabled`.

- [ ] **Step 4: Запустить тесты и убедиться, что они проходят**

Run: `go test -v ./internal/bot -run "TestDetectCountryAndFlag|TestProbeNodeLatency"`
Expected: PASS

- [ ] **Step 5: Закоммитить изменения**

```bash
git add internal/bot/nodes_probe.go internal/bot/nodes_probe_test.go
git commit -m "feat(bot): implement country detection and safe latency probe for nodes"
```

---

### Task 3: Рендерер Discord Embed и интерактивной кнопки (Nodes Renderer)

**Files:**
- Create: `internal/bot/nodes_renderer.go`
- Create: `internal/bot/nodes_renderer_test.go`

**Interfaces:**
- Consumes: `[]NodeStatus`, `footerInterval int`.
- Produces:
  - `RenderNodesStatusEmbed(nodes []NodeStatus, footerInterval int) *discordgo.MessageEmbed`
  - `BuildNodesStatusComponents() []discordgo.MessageComponent`

- [ ] **Step 1: Написать падающий тест для рендеринга эмбеда и кнопок**

```go
// internal/bot/nodes_renderer_test.go
package bot

import (
    "strings"
    "testing"
)

func TestRenderNodesStatusEmbed(t *testing.T) {
    nodes := []NodeStatus{
        {
            ID:          1,
            Name:        "[GE] Ryzen 9 7950X3D — Frankfurt",
            Location:    "Frankfurt",
            CountryCode: "DE",
            Flag:        "🇩🇪",
            Online:      true,
            ServerCount: 4,
            Latency:     "24 ms",
            CPUModel:    "AMD Ryzen 9 7950X3D",
            CPUPercent:  42,
            Protection:  "L4 Anti-DDoS Protection",
        },
    }

    embed := RenderNodesStatusEmbed(nodes, 5)
    if embed.Title != "🖥️ Состояние выделенных серверов (Нод)" {
        t.Errorf("unexpected title: %s", embed.Title)
    }
    if len(embed.Fields) < 2 {
        t.Fatalf("expected at least 2 fields (node + datacenters), got %d", len(embed.Fields))
    }
    nodeField := embed.Fields[0]
    if !strings.Contains(nodeField.Name, "🇩🇪") || !strings.Contains(nodeField.Name, "Frankfurt") {
        t.Errorf("node field title missing flag or name: %s", nodeField.Name)
    }
    if !strings.Contains(nodeField.Value, "🟢 Система в норме") || !strings.Contains(nodeField.Value, "👥 Игровых серверов: 4") {
        t.Errorf("node field value missing status or servers count: %s", nodeField.Value)
    }
    if !strings.Contains(nodeField.Value, "📊 Нагрузка CPU:") || !strings.Contains(nodeField.Value, "42%") {
        t.Errorf("node field value missing CPU progress bar: %s", nodeField.Value)
    }
    if !strings.Contains(embed.Footer.Text, "5 минут") {
        t.Errorf("footer text missing update interval: %s", embed.Footer.Text)
    }

    comps := BuildNodesStatusComponents()
    if len(comps) != 1 {
        t.Fatalf("expected 1 action row component, got %d", len(comps))
    }
}
```

- [ ] **Step 2: Запустить тест и убедиться, что он падает**

Run: `go test -v ./internal/bot -run TestRenderNodesStatusEmbed`
Expected: FAIL (функции не определены)

- [ ] **Step 3: Реализовать `nodes_renderer.go`**

Использовать существующую функцию `GenerateProgressBar(current, max, length)` для ASCII-бара нагрузки CPU.
Сформировать эмбед с цветом:
- `0x2ecc71` если все ноды online;
- `0xe67e22` при частичном сбое;
- `0xe74c3c` если все ноды offline.
Добавить нижнее поле `🌐 Дата-центры сети` со списком локаций и статусом защиты.
В `BuildNodesStatusComponents()` вернуть одну строку действий (`ActionsRow`) с кнопкой `[ 🔄 Обновить статус ]` (`custom_id: "refresh_nodes_status"`).

- [ ] **Step 4: Запустить тесты и убедиться, что они проходят**

Run: `go test -v ./internal/bot -run TestRenderNodesStatusEmbed`
Expected: PASS

- [ ] **Step 5: Закоммитить изменения**

```bash
git add internal/bot/nodes_renderer.go internal/bot/nodes_renderer_test.go
git commit -m "feat(bot): implement discord embed and component renderer for nodes status"
```

---

### Task 4: Сохранение состояния и обработчики команд/кнопок (State & Commands)

**Files:**
- Modify: `internal/bot/state.go:1-60`
- Modify: `internal/bot/commands.go:100-300, 850-950`
- Test: `internal/bot/state_test.go`
- Test: `internal/bot/commands_test.go`

**Interfaces:**
- Consumes: `state.State`, `commands.go:handleMessageComponent`, `commands.go:handleApplicationCommand`.
- Produces:
  - `state.State.NodesChannelID string`
  - `state.State.NodesMessageID string`
  - Обработка слэш-команды `/mon nodes_channel [channel]`
  - Обработка кнопки `refresh_nodes_status` в `handleMessageComponent`

- [ ] **Step 1: Написать падающий тест для `State` и обработки кнопки нод**

```go
// internal/bot/state_test.go
func TestNodesStatePersistence(t *testing.T) {
    tmpFile := filepath.Join(t.TempDir(), "state.json")
    state := &State{
        FilePath:       tmpFile,
        NodesChannelID: "111222333",
        NodesMessageID: "444555666",
    }
    if err := state.Save(); err != nil {
        t.Fatalf("save error: %v", err)
    }

    loaded, err := LoadState(tmpFile)
    if err != nil {
        t.Fatalf("load error: %v", err)
    }
    if loaded.NodesChannelID != "111222333" || loaded.NodesMessageID != "444555666" {
        t.Errorf("state mismatch: %+v", loaded)
    }
}
```

- [ ] **Step 2: Запустить тест и убедиться, что он падает**

Run: `go test -v ./internal/bot -run TestNodesStatePersistence`
Expected: FAIL (поля `NodesChannelID`, `NodesMessageID` отсутствуют)

- [ ] **Step 3: Реализовать сохранение в `state.go` и обработчики в `commands.go`**

1. Добавить `NodesChannelID` и `NodesMessageID` в `State`.
2. В `commands.go` в определение слэш-команд добавить подкоманду `nodes_channel` для выбора канала вывода нод.
3. В `handleMessageComponent` добавить обработку:
   ```go
   case "refresh_nodes_status":
       // Acknowledge interaction (DeferredUpdateMessage)
       // Запустить асинхронное обновление нод
   ```

- [ ] **Step 4: Запустить тесты и убедиться, что они проходят**

Run: `go test -v ./internal/bot -run TestNodesStatePersistence`
Expected: PASS

- [ ] **Step 5: Закоммитить изменения**

```bash
git add internal/bot/state.go internal/bot/commands.go internal/bot/state_test.go
git commit -m "feat(bot): add nodes channel state persistence and interaction handlers"
```

---

### Task 5: Фоновый цикл опроса и синхронизация сообщений (Bot Engine Loop)

**Files:**
- Modify: `internal/bot/bot.go:50-250`
- Test: `internal/bot/bot_test.go`

**Interfaces:**
- Consumes: `nodes_probe.go`, `nodes_renderer.go`, `state.go`.
- Produces:
  - `Bot.UpdateNodesStatus(ctx context.Context) error`
  - `Bot.RefreshNodesStatus(ctx context.Context) error`
  - Фоновый таймер `nodesTicker` (дефолт 5 минут / 300 сек).

- [ ] **Step 1: Написать падающий тест для метода `UpdateNodesStatus`**

Проверить, что при валидном канале `UpdateNodesStatus` форматирует эмбед и вызывает синхронизацию сообщения, а при ошибке 404 восстанавливает отправку нового сообщения.

- [ ] **Step 2: Запустить тест и убедиться, что он падает**

Run: `go test -v ./internal/bot -run TestUpdateNodesStatus`
Expected: FAIL (метод `UpdateNodesStatus` не объявлен)

- [ ] **Step 3: Реализовать `UpdateNodesStatus` и запуск таймера в `bot.go`**

1. В `Bot.Start(ctx)` запустить отдельный горутин-тикер с интервалом `cfg.Discord.NodesUpdateIntervalSeconds` (дефолт 300 секунд).
2. Реализовать `UpdateNodesStatus`:
   - Определить целевой `channelID` из `state.NodesChannelID`, если пусто — из `cfg.Discord.NodesChannelID`. Если оба пусты — пропустить.
   - Получить список серверов и нод из GameAP (или из standalone конфига).
   - Вызвать `EnrichNodeStatuses(...)`.
   - Вызвать `RenderNodesStatusEmbed(...)` и `BuildNodesStatusComponents()`.
   - Если `state.NodesMessageID` не пустой: вызвать `s.ChannelMessageEditComplex(...)`.
   - Если редактирование вернуло ошибку 404 (сообщение удалено) или `NodesMessageID` пуст: отправить новое сообщение `s.ChannelMessageSendComplex(...)`, сохранить полученный ID в `state.NodesMessageID` и вызвать `state.Save()`.

- [ ] **Step 4: Запустить тесты и убедиться, что они проходят**

Run: `go test -v ./internal/bot`
Expected: PASS

- [ ] **Step 5: Закоммитить изменения**

```bash
git add internal/bot/bot.go internal/bot/bot_test.go
git commit -m "feat(bot): implement background nodes monitoring loop and message sync"
```

---

### Task 6: Поддержка нод в Companion WASM плагине (GameAP_WebMonitoring)

**Files:**
- Modify: `f:\GameAP_WebMonitoring\rust-plugin\src\models.rs`
- Modify: `f:\GameAP_WebMonitoring\rust-plugin\src\handlers.rs`

**Interfaces:**
- Consumes: GameAP host module `gameap-nodes` (`FindNodes`).
- Produces: `nodes: Vec<NodeDTO>` в JSON-ответе плагина `/servers` и `/api/servers`.

- [ ] **Step 1: Объявить `NodeDTO` в `models.rs` плагина**

```rust
#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct NodeDTO {
    pub id: u64,
    pub name: String,
    pub location: String,
    pub provider: String,
    pub ips: Vec<String>,
    pub gdaemon_host: String,
    pub gdaemon_port: i32,
    pub enabled: bool,
}
```
Добавить `pub nodes: Vec<NodeDTO>` в `MonitoringResponse`.

- [ ] **Step 2: Реализовать заполнение нод в `handlers.rs`**

Если GameAP host module `gameap-nodes` доступен, вызвать `FindNodes` и заполнить `nodes` в ответе. При отсутствии модуля или ошибке возвращать пустой вектор `vec![]`, не ломая отдачу списка серверов.

- [ ] **Step 3: Скомпилировать WASM плагин**

Run: `cargo build --target wasm32-wasip1 --release` в директории `f:\GameAP_WebMonitoring\rust-plugin`
Expected: успешная сборка `web_monitoring.wasm`.

- [ ] **Step 4: Закоммитить изменения плагина**

```bash
git add src/models.rs src/handlers.rs
git commit -m "feat(wasm): add nodes list to monitoring response"
```

---

### Task 7: Комплексная сборка, проверка и тестирование (Verification)

**Files:**
- Test all: `f:\GameAP_WebMonBot`
- Build: `f:\GameAP_WebMonBot\webmonbot.exe`

- [ ] **Step 1: Запустить полный набор тестов в боте**

Run: `go test -v -race ./...` в `f:\GameAP_WebMonBot`
Expected: ALL PASS

- [ ] **Step 2: Скомпилировать финальный бинарник бота**

Run: `go build -ldflags="-s -w" -o webmonbot.exe ./cmd/bot` в `f:\GameAP_WebMonBot`
Expected: успешная генерация `webmonbot.exe`.

- [ ] **Step 3: Проверить запуск с тестовой конфигурацией**

Проверить, что бот корректно инициализирует таймер нод (300 сек), не выдает паник при пустом канале нод и готов к работе.
