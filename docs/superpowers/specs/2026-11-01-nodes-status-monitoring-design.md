# Дизайн-документ: Мониторинг выделенных серверов и хост-нод (Nodes Status Monitoring)

- **Дата:** 2026-11-01
- **Проект:** GameAP WebMonBot (v1.0.0) / GameAP_WebMonitoring
- **Статус:** Одобрено к планированию
- **Референс дизайна:** `media_1793557622986.png`

---

## 1. Контекст и цели

### 1.1. Контекст
В боте **GameAP WebMonBot** мониторинг в текущей версии сфокусирован на отображении игровых серверов (общего списка и карточек отдельных серверов).
Для сетевых игровых проектов и хостинг-провайдеров критически важно информировать игроков о состоянии всей инфраструктуры:
- Физические хост-машины (ноды / Dedicated Servers).
- Доступность дата-центров и локаций (Германия, Россия, Казахстан, Нидерланды и др.).
- Уровень защиты от DDoS (L4/L7 Anti-DDoS Protection).
- Количество запущенных игровых серверов на каждой ноде.
- Нагрузка на процессоры (CPU usage) и сетевая задержка (Ping).

### 1.2. Референс визуала и эстетика
Пользователь предоставил эталонный скриншот (`media_1793557622986.png`):
- **Заголовок:** `🖥️ Состояние выделенных серверов (Нод)`
- **Описание:** `Статус работы хост-нод и физических серверов сети`
- **Карточка ноды (Field):**
  - Заголовок с флагом страны: `🇩🇪 [GE] Ryzen 9 7950X3D — Frankfurt`
  - Статус и серверы: `🟢 Система в норме | 👥 Игровых серверов: 4`
  - Метрики:
    - `📊 Нагрузка CPU: [████░░░░░░] 42%`
    - `🛡️ Защита: L4 Anti-DDoS Protection`
    - `📶 Пинг: 24 ms` (или `Защищён (DDoS Filter)`)
- **Блок дата-центров (Нижний field):**
  - `🌐 Дата-центры сети` с перечислением локаций, провайдеров и статуса защиты.
- **Интерактивные компоненты:**
  - Кнопка: `[ 🔄 Обновить статус ]` (`refresh_nodes_status`).
  - Ссылка на панель управления **убрана** по требованию пользователя.
- **Подвал (Footer):**
  - `🔄 Автообновление каждые 5 минут • GameAP Infrastructure` (или кастомный интервал).

### 1.3. Ключевые требования пользователя
1. **Источник данных:**
   - Основной: GameAP API через модуль `gameap-nodes` (`FindNodes`) в плагине `GameAP_WebMonitoring` или прямой опрос GameAP API.
   - Резервный: секция `nodes` в `config.json` и состояние `state.json` для автономного режима (Standalone).
2. **Авторитетность статуса ноды:**
   - Статус онлайна ноды определяется **панелью GameAP** (`Node.enabled` / связь с GDaemon).
   - Если прямой TCP/ICMP пинг до ноды не проходит (таймаут или блок фаерволом/Anti-DDoS), нода **НЕ должна** помечаться как офлайн. Вместо миллисекунд пинга отображается статус `Защищён (DDoS Filter)` или пингуется активный игровой порт на этой ноде, а состояние остаётся `🟢 Система в норме`.
3. **Автоматическое определение страны и флага:**
   - По тегу в имени ноды: `\[([A-Za-z]{2,3})\]` (например, `[GE]`, `[DE]`, `[RU]`, `[KZ]`, `[US]`, `[FR]`, `[NL]`, `[FI]`).
   - Фоллбэк: по строке `location` (например, "Germany", "Frankfurt", "Russia", "Moscow", "Kazakhstan", "Netherlands", "Helsinki").
   - Фоллбэк по умолчанию: `🌐`.
4. **Периодичность обновления:**
   - Обновление сообщения каждые 5 минут (300 секунд по умолчанию, настраивается в конфигурации `discord.nodes_update_interval_seconds`).
   - Мгновенное обновление по нажатию на кнопку `[ 🔄 Обновить статус ]`.
5. **Сохранение и привязка:**
   - Канал задается через `config.json` (`nodes_channel_id`) либо через слэш-команду `/mon nodes_channel [channel]`.
   - ID сообщения нод сохраняется в `state.json` (`nodes_message_id`), бот редактирует существующее сообщение при перезапусках.

---

## 2. Архитектура решения

### 2.1. Схема взаимодействия
```
+-----------------------------------------------------------+
|                      GameAP Panel                         |
|  - Nodes Table (ID, Name, Location, GDaemon, Status)      |
|  - Game Servers Table (ds_id -> Node ID)                  |
+-----------------------------+-----------------------------+
                              |
                     WASM / REST API
                              |
+-----------------------------v-----------------------------+
|              GameAP_WebMonBot Engine                      |
|                                                           |
| 1. internal/gameap:                                       |
|    - MonitoringResponse includes []NodeDTO                |
|                                                           |
| 2. internal/bot/nodes_probe.go:                           |
|    - Parse Country & Flag (tag / location)                |
|    - Associate game servers count per node (by ds_id/IP)  |
|    - Safe TCP/GDaemon Latency Probe (with DDoS fallback)  |
|                                                           |
| 3. internal/bot/nodes_renderer.go:                        |
|    - Builds Discord Embed matching reference visual       |
|    - Builds ActionRow with [Refresh] button               |
|                                                           |
| 4. internal/bot/bot.go (Nodes Poller Loop):               |
|    - Ticker (default 300s / 5 min)                        |
|    - Edits existing nodes message in nodes_channel_id     |
|                                                           |
| 5. internal/bot/commands.go:                              |
|    - Handles button "refresh_nodes_status"                |
|    - Slash command /mon nodes_channel                     |
+-----------------------------+-----------------------------+
                              |
                        Discord API
                              |
+-----------------------------v-----------------------------+
|                    Discord Channel                        |
|   [ 🖥️ Состояние выделенных серверов (Нод) ]              |
|   [ 🔄 Обновить статус ] (Interactive Button)             |
+-----------------------------------------------------------+
```

---

## 3. Модели данных

### 3.1. DTO ноды (`internal/gameap/models.go`)
```go
type NodeDTO struct {
    ID          uint64 `json:"id"`
    Name        string `json:"name"`
    Location    string `json:"location"`
    Provider    string `json:"provider,omitempty"`
    IPs         []string `json:"ips,omitempty"`
    GDaemonHost string `json:"gdaemon_host,omitempty"`
    GDaemonPort int    `json:"gdaemon_port,omitempty"`
    Enabled     bool   `json:"enabled"`
    CPUModel    string `json:"cpu_model,omitempty"`
    CPUPercent  int    `json:"cpu_percent,omitempty"`
    Protection  string `json:"protection,omitempty"`
}
```

### 3.2. Внутренняя обогащенная модель статуса ноды (`internal/bot/nodes_probe.go`)
```go
type NodeStatus struct {
    ID          uint64
    Name        string
    Location    string
    CountryCode string
    Flag        string
    Online      bool
    ServerCount int
    Latency     string
    CPUModel    string
    CPUPercent  int
    Protection  string
    Host        string
}
```

### 3.3. Конфигурация нод в `config.json`
```json
{
  "discord": {
    "nodes_channel_id": "123456789012345678",
    "nodes_update_interval_seconds": 300
  },
  "nodes": [
    {
      "id": 1,
      "name": "[GE] Ryzen 9 7950X3D — Frankfurt",
      "location": "Germany, Frankfurt",
      "cpu_model": "AMD Ryzen 9 7950X3D",
      "protection": "L4 Anti-DDoS Protection",
      "host": "185.189.255.10",
      "port": 31717
    }
  ]
}
```

---

## 4. Алгоритм парсинга стран и флагов

1. **Поиск тега в квадратных скобках:**
   - Регулярное выражение: `\[([A-Za-z]{2,3})\]`.
   - Если найдено:
     - `GE` или `DE` -> `🇩🇪` (Германия)
     - `RU` -> `🇷🇺` (Россия)
     - `KZ` -> `🇰🇿` (Казахстан)
     - `NL` -> `🇳🇱` (Нидерланды)
     - `FI` -> `🇫🇮` (Финляндия)
     - `US` -> `🇺🇸` (США)
     - `FR` -> `🇫🇷` (Франция)
     - `PL` -> `🇵🇱` (Польша)
     - `UA` -> `🇺🇦` (Украина)
     - `GB` или `UK` -> `🇬🇧` (Великобритания)
     - `SE` -> `🇸🇪` (Швеция)
2. **Фоллбэк по тексту `location` или `name`:**
   - Проверка наличия ключевых слов (регистронезависимо):
     - `germany`, `frankfurt`, `германия`, `франкфурт` -> `🇩🇪`
     - `russia`, `moscow`, `россия`, `москва`, `питер`, `spb` -> `🇷🇺`
     - `kazakhstan`, `almaty`, `казахстан`, `алматы`, `астана` -> `🇰🇿`
     - `netherlands`, `amsterdam`, `нидерланды`, `амстердам` -> `🇳🇱`
     - `finland`, `helsinki`, `финляндия`, `хельсинки` -> `🇫🇮`
     - `france`, `paris`, `франция`, `париж` -> `🇫🇷`
3. **Фоллбэк по умолчанию:** `🌐`.

---

## 5. Безопасный опрос сетевой задержки (Safe Latency Probe)

- Проверяется TCP-соединение до порта GDaemon или первого доступного IP ноды с коротким таймаутом (800 мс).
- Если порт ответил: пинг форматируется как `XX ms`.
- Если порт не ответил (таймаут/connection refused из-за защитных политик / Anti-DDoS):
  - Нода **НЕ** помечается как `Offline`.
  - Поле пинга принимает значение `🛡️ Защищён (DDoS Filter)`.
  - Общий статус ноды остаётся `🟢 Система в норме`.
- Статус ноды становится `🔴 Недоступен` **только** если панель GameAP явно сообщает `enabled: false` или нода не отвечает самой панели.

---

## 6. Визуальный рендеринг Discord Embed

- **Цвет эмбеда:**
  - `0x2ecc71` (зеленый) — все ноды в норме.
  - `0xe67e22` (оранжевый) — частичная деградация / одна нода недоступна.
  - `0xe74c3c` (красный) — все ноды недоступны.
- **Поля эмбеда (Fields):**
  - Для каждой ноды отдельный блок поля:
    - Название поля: `{flag} {NodeName}`
    - Значение поля:
      ```
      🟢 Система в норме | 👥 Игровых серверов: {count}
      📊 Нагрузка CPU: [{bar}] {percent}% ({cpu_model})
      🛡️ Защита: {protection}
      📶 Пинг: {ping_or_ddos_status}
      ```
  - Поле дата-центров (внизу эмбеда):
    - Название: `🌐 Дата-центры сети`
    - Значение:
      ```
      • 🇩🇪 Франкфурт (Equinix FR2 / Maincubes) — L4/L7 DDoS Filter
      • 🇷🇺 Москва (DataPro / Selectel) — L4 Anti-DDoS Shield
      • 🇰🇿 Алматы (ТрансТелеКом) — Protected Low-Latency Route
      ```
- **Интерактивная кнопка:**
  - Стиль: `Secondary` (серый) или `Primary` (синий).
  - Эмодзи: `🔄`.
  - Лейбл: `Обновить статус`.
  - CustomID: `refresh_nodes_status`.
