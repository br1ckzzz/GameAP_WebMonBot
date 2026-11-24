# GameAP WebMonBot 🎮🤖

[![Version](https://img.shields.io/badge/version-1.0.1-blue.svg)](https://github.com/br1ckzzz/GameAP_WebMonBot/releases)
[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8.svg)](https://golang.org)
[![Discord API](https://img.shields.io/badge/Discord%20API-v10-5865F2.svg)](https://discord.com/developers/docs)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Supported Games](https://img.shields.io/badge/games-297%20supported-orange.svg)](SUPPORTED_GAMES.md)

[🇷🇺 Инструкция на русском языке](#-русская-версия) | [🇬🇧 English Documentation](#-english-version)

---

## 🇷🇺 Русская версия

**GameAP WebMonBot** — это автономный, высокопроизводительный и невероятно легкий Discord-бот нового поколения для мониторинга игровых серверов и хост-машин (нод) панели управления **GameAP** в режиме реального времени.

Бот работает без тяжелых зависимостей (не требуется Node.js, Python или Docker), потребляет всего **~15 МБ оперативной памяти**, компилируется в один исполняемый файл и настраивается прямо через Discord слэш-команды за пару кликов.

---

### ✨ Главные преимущества

- 🪶 **Экстремальная легковесность:** Написан на чистом Go. Потребляет всего **~15 МБ RAM** и практически не нагружает процессор. Запускается на любом слабом VPS или на том же сервере, где крутится GameAP.
- 🖥️ **Мониторинг хост-нод (`/mon nodes`) [NEW в v1.0.0]:** Отдельное интерактивное табло инфраструктуры хост-нод: статус доступности, операционная система, локация хостинга с флагом страны, IP-адрес, число активных серверов и **прямой замер сетевого пинга** до каждого хоста!
- ⚡ **Мультипротокольный движок прямого опроса (Direct Query):** Встроенная поддержка опроса серверов напрямую по UDP/TCP:
  - **Valve A2S** (Counter-Strike 2, CS:GO, Rust, TF2, Left 4 Dead 2, Valheim, 7 Days to Die, ARK и сотни других);
  - **Minecraft Java** (Query & Server List Ping протокол);
  - **Minecraft Bedrock** (RakNet протокол);
  - **SA-MP / CR-MP** (San Andreas Multiplayer протокол);
  - **Quake / GameSpy** (Quake 3, RTCW, CoD, Unreal Tournament);
  - **FiveM** (GTA V / cfx.re JSON API).
- 🌐 **Мониторинг любых внешних серверов (Standalone):** Вы можете добавить абсолютно любой игровой сервер мира прямо в Discord командой `/mon add address:ip:port game:код` — бот будет опрашивать его самостоятельно, даже если он не добавлен в панель GameAP!
- 🔔 **Умная система оповещений (Alerts):** Мгновенно оповещает администраторов или игроков о падении сервера, его восстановлении или перезагрузке с защитой от ложных срабатываний (anti-flapping confirmation).
- 🎨 **Красивые интерактивные карточки:** Прогресс-бары слотов игроков `[██████░░░░] 24/32`, кнопки мгновенного подключения `[🔗 Подключиться]` и ручного обновления `[🔄 Обновить]`, официальные иконки и обложки игр из Steam CDN.
- 🎯 **Гибкая маршрутизация по каналам:**
  - Единое табло со всеми серверами (`/mon bind`);
  - Выделенный канал под конкретную игру (`/mon bind game:cs2`);
  - Выделенный канал под отдельный флагманский сервер (`/mon bind server:1`);
  - Форум-каналы Discord с авто-созданием тем под каждую игру (`/mon forum`);
  - Выделенный канал для мониторинга хост-нод и инфраструктуры (`/mon nodes`).
- 🛡️ **100% Безопасность и совместимость с 2FA:** Работает по изолированному токену плагина `X-WebMon-Token`. Бот не имеет доступа к консоли, RCON или файлам сервера, не может остановить сервер и не требует логинов/паролей от панели.
- 📚 **Каталог из 297 поддерживаемых игр:** Полный список кодов и игр доступен в файле [SUPPORTED_GAMES.md](SUPPORTED_GAMES.md).

---

### 🚀 Пошаговая настройка для новичков (Quick Start)

Даже если вы никогда раньше не настраивали Discord-ботов, просто выполните 5 понятных шагов ниже:

#### Шаг 1. Создание бота в Discord Developer Portal
1. Перейдите на сайт [Discord Developer Portal](https://discord.com/developers/applications) и войдите под своим аккаунтом Discord.
2. В правом верхнем углу нажмите синюю кнопку **«New Application»**, введите имя (например, `GameAP Monitor`) и нажмите **Create**.
3. Перейдите во вкладку слева **«Bot»**:
   - Нажмите кнопку **«Reset Token»**, подтвердите действие и нажмите **«Copy»** (сохраните этот токен, он понадобится в `config.json`).
   - *(Опционально)* Прокрутите ниже до блока **«Privileged Gateway Intents»** — для базовой работы слеш-команд они не требуются, но если планируете расширенные функции, можно включить *Server Members Intent* и *Message Content Intent*.
4. Перейдите во вкладку слева **«OAuth2» ➔ «URL Generator»**:
   - В блоке **SCOPES** отметьте галочками: `bot` и `applications.commands`.
   - В блоке **BOT PERMISSIONS** отметьте:
     - `Send Messages` (Отправка сообщений)
     - `Embed Links` (Встраивание ссылок)
     - `Manage Messages` (Управление сообщениями — нужно для удаления старых карточек)
     - `Read Message History` (Чтение истории сообщений)
     - `Use Slash Commands` (Использование слэш-команд)
5. Внизу страницы скопируйте получившуюся ссылку в поле **Generated URL**, откройте её в браузере и добавьте бота на свой Discord-сервер.

---

#### Шаг 2. Включение Bot API в панели GameAP
1. Откройте вашу веб-панель **GameAP ➔ Плагины ➔ Мониторинг** (или раздел Web Monitoring).
2. Найдите блок настроек **«🤖 Интеграция с Discord / Bot API»**:
   - Поставьте галочку **«Разрешить доступ для внешних ботов (Bot API)»**.
   - Нажмите кнопку **«Сгенерировать токен»** и скопируйте появившийся ключ (например, `wm_sec_...`).
   - Нажмите зеленую кнопку **«Сохранить настройки»**.
3. Запомните адрес вашей панели, например: `http://ваш-домен-или-ip:8080/api/plugins/monitoring`.

---

#### Шаг 3. Подготовка конфигурации `config.json`
1. Скачайте свежий архив релиза бота из раздела [Releases](https://github.com/br1ckzzz/GameAP_WebMonBot/releases).
2. В папке с ботом переименуйте файл `config.example.json` в `config.json` (или создайте его рядом с `bot.exe`).
3. Заполните токен бота и адрес панели GameAP:

```json
{
  "bot": {
    "token": "СЮДА_ВСТАВЬТЕ_ТОКЕН_БОТА_ИЗ_ШАГА_1",
    "application_id": "",
    "update_interval_seconds": 30,
    "status_presence": "GameAP Monitoring",
    "embed_color_online": "#2ecc71",
    "embed_color_offline": "#e74c3c"
  },
  "gameap_api": {
    "base_url": "http://127.0.0.1:8080/api/plugins/monitoring",
    "api_token": "СЮДА_ВСТАВЬТЕ_ТОКЕН_ПЛАГИНА_ИЗ_ШАГА_2",
    "timeout_seconds": 8,
    "direct_query_enabled": true,
    "direct_query_timeout_ms": 1500
  },
  "discord": {
    "guild_id": "",
    "dashboard_channel_id": "",
    "nodes_channel_id": "",
    "nodes_update_interval_seconds": 60,
    "group_by_game": true
  },
  "notifications": {
    "enabled": true,
    "alerts_channel_id": "",
    "notify_on_offline": true,
    "notify_on_online": true,
    "notify_on_restart": true
  }
}
```

> 💡 **Совет:** Поля `guild_id`, `dashboard_channel_id`, `nodes_channel_id` и `alerts_channel_id` можно оставить пустыми `""`! Вы сможете привязать любые каналы прямо в Discord через удобные слэш-команды за секунду.

---

#### Шаг 4. Запуск бота
- **В Windows:** Просто дважды кликните по `bot.exe` (или запустите в консоли PowerShell `.\bot.exe`).
- **В Linux:** Выдайте права на запуск и запустите:
  ```bash
  chmod +x webmonbot-linux-amd64
  ./webmonbot-linux-amd64
  ```
В консоли появится сообщение:
```text
[INFO] Discord session established successfully (Bot: GameAP Monitor#1234)
[INFO] Application slash commands registered globally
[INFO] Monitoring loop started (Interval: 30s)
```
Бот загорится зелёным индикатором в списке участников вашего сервера.

---

#### Шаг 5. Привязка табло в Discord в один клик
Перейдите в Discord и напишите команду в нужном текстовом канале:

1. **Главное табло игровых серверов:**
   Перейдите в канал (например, `#мониторинг`) и введите:
   ```text
   /mon bind
   ```
   *Бот привяжет этот канал и сразу опубликует красивое живое табло со статусом всех ваших игровых серверов!*

2. **Табло физических нод и хостов (Новинка!):**
   Перейдите в канал для администрации или инфо-канал (например, `#статус-нод`) и введите:
   ```text
   /mon nodes
   ```
   *Бот создаст отдельную карточку со списком ваших серверов-нод, их статусом, операционной системой, локациями, IP-адресами, количеством серверов и сетевым пингом.*

3. **Канал оповещений об авариях/падениях (Alerts):**
   Перейдите в канал оповещений (например, `#алерты`) и введите:
   ```text
   /mon alerts channel:#алерты role:@Администраторы
   ```

Поздравляем! Мониторинг полностью настроен и обновляется в реальном времени каждые 30 секунд.

---

### 📖 Полный справочник слэш-команд бота (`/mon`)

Все команды администрирования интуитивны и защищены: их могут вызывать только пользователи с правом `Manage Server` (Управление сервером) или назначенной ролью администратора.

| Слэш-команда | Параметры | Описание действия | Пример использования |
| :--- | :--- | :--- | :--- |
| **`/mon bind`** | `[channel]` `[game]` `[server]` | Привязать мониторинг к каналу. Без параметров привязывает общее табло к текущему каналу. С `game:` транслирует только указанную игру, с `server:` — персональную карточку одного сервера. | `/mon bind`<br>`/mon bind channel:#серверы`<br>`/mon bind game:cs2 channel:#cs2`<br>`/mon bind server:1 channel:#mirage` |
| **`/mon unbind`** | `[game]` `[server]` `[all]` | Отвязать мониторинг от канала. Можно отвязать определенную игру, конкретный сервер или снять вообще все привязки (`all:true`). | `/mon unbind`<br>`/mon unbind game:cs2`<br>`/mon unbind server:1`<br>`/mon unbind all:true` |
| **`/mon bindings`** | *нет* | Показать список всех активных привязок каналов, игр и персональных серверов. | `/mon bindings` |
| **`/mon nodes`** | `[channel]` `[unbind]` | Управление мониторингом хост-нод и инфраструктуры. Публикует и авто-обновляет сводку физических машин GameAP со статусом, локациями, ОС и сетевым пингом. | `/mon nodes`<br>`/mon nodes channel:#железо`<br>`/mon nodes unbind:true` |
| **`/mon alerts`** | `[channel]` `[role]` `[enabled]` `[disable]` | Настройка мгновенных уведомлений об авариях серверов (падение, запуск, перезапуск, смена карты). Поддерживает упоминание роли. | `/mon alerts`<br>`/mon alerts channel:#аварии role:@Техподдержка`<br>`/mon alerts disable:true` |
| **`/mon forum`** | `[channel]` `[unbind]` | Привязать мониторинг к Discord Forum Channel. Бот автоматически создаст отдельные темы по играм с живыми карточками. | `/mon forum channel:#игровые-серверы`<br>`/mon forum unbind:true` |
| **`/mon add`** | `address:` `game:` `[name]` `[channel]` `[icon]` | **Добавить внешний сервер (Standalone)** в мониторинг без GameAP. Опрашивается напрямую по протоколам A2S/Minecraft/SAMP/FiveM. | `/mon add address:185.12.34.56:27015 game:cs2`<br>`/mon add address:mc.hypixel.net:25565 game:mc name:Hypixel`<br>`/mon add address:185.12.34.56:7777 game:samp channel:#samp` |
| **`/mon remove`** | `address:` | Удалить ранее добавленный внешний сервер из мониторинга. | `/mon remove address:185.12.34.56:27015` |
| **`/mon list`** | *нет* | Показать интерактивный список всех добавленных внешних серверов. | `/mon list` |
| **`/mon clean`** | *нет* | Быстрая очистка канала от старых, дублирующихся или зависших сообщений бота. | `/mon clean` |
| **`/mon setup`** | `url:` `token:` | Сменить или настроить адрес API GameAP WebMonitoring и токен авторизации прямо из Discord. | `/mon setup url:https://panel.site.com/api/plugins/monitoring token:wm_sec_123` |
| **`/mon status`** | *нет* | Мгновенная компактная сводка текущего состояния серверов (видна только вам). | `/mon status` |
| **`/mon refresh`** | *нет* | Принудительно обновить все карточки серверов и нод прямо сейчас, не дожидаясь таймера. | `/mon refresh` |
| **`/mon ping`** | *нет* | Проверить сетевую задержку до Discord Gateway и задержку ответа API GameAP. | `/mon ping` |

---

### 🎮 Справочник поддерживаемых игр (297 игр)

Бот распознает как полные наименования игр, так и общепринятые сокращения для параметра `game:` в командах `/mon bind game:...` и `/mon add ... game:...`.

👉 **[Полный алфавитный справочник всех 297 игр смотрите в SUPPORTED_GAMES.md](SUPPORTED_GAMES.md)**.

Популярные примеры кодов:
- `cs2` / `csgo` — Counter-Strike 2 / Global Offensive
- `rust` — Rust
- `mc` / `minecraft` — Minecraft Java Edition
- `bedrock` — Minecraft Bedrock (PE)
- `samp` / `crmp` — San Andreas Multiplayer / CR-MP
- `valheim` — Valheim
- `7d2d` — 7 Days to Die
- `tf2` — Team Fortress 2
- `l4d2` — Left 4 Dead 2
- `arkse` / `asa` — ARK: Survival Evolved / Ascended
- `dayz` — DayZ Standalone
- `unturned` — Unturned
- `palworld` — Palworld
- `fivem` — FiveM (GTA V RP)
- `kf2` — Killing Floor 2
- `zomboid` — Project Zomboid

---

### 🐧 Автозапуск в Linux (Systemd Service)

Чтобы бот работал 24/7 в фоновом режиме на вашем Linux VPS, создайте systemd-сервис:

1. Скопируйте бинарник и конфиг, например, в `/opt/webmonbot`:
   ```bash
   mkdir -p /opt/webmonbot
   cp webmonbot-linux-amd64 /opt/webmonbot/
   cp config.json /opt/webmonbot/
   chmod +x /opt/webmonbot/webmonbot-linux-amd64
   ```

2. Создайте файл службы `/etc/systemd/system/webmonbot.service`:
   ```ini
   [Unit]
   Description=GameAP WebMonBot Service
   After=network.target

   [Service]
   Type=simple
   User=root
   WorkingDirectory=/opt/webmonbot
   ExecStart=/opt/webmonbot/webmonbot-linux-amd64 -config /opt/webmonbot/config.json
   Restart=always
   RestartSec=5

   [Install]
   WantedBy=multi-user.target
   ```

3. Активируйте и запустите сервис:
   ```bash
   systemctl daemon-reload
   systemctl enable --now webmonbot
   ```

4. Проверить статус службы:
   ```bash
   systemctl status webmonbot
   journalctl -u webmonbot -f
   ```

---
---

## 🇬🇧 English Version

**GameAP WebMonBot** is a standalone, ultra-lightweight, and next-generation Discord bot designed for real-time monitoring of game servers and physical infrastructure host nodes powered by the **GameAP** control panel.

Built with clean Go, it requires no heavy runtime environments (zero Node.js, Python, or Docker dependencies), operates at a featherweight footprint of only **~15 MB RAM**, ships as a single static binary, and can be entirely configured directly inside Discord using modern slash commands.

---

### ✨ Key Features & Highlights

- 🪶 **Ultra Lightweight & Fast:** Single statically-linked binary consuming only **~15 MB of RAM**. Minimal CPU usage, lightning-fast socket updates.
- 🖥️ **Host Nodes & Dedicated Infrastructure Monitoring (`/mon nodes`) [NEW in v1.0.0]:** Dedicated dashboard for physical host machines: online availability status, Operating System, hosting location with national country flag, IP/host address, active game servers count, and **real-time network ping latency** to each individual host!
- ⚡ **Multi-Protocol Direct Query Engine:** Instantaneous socket-level queries without external wrappers:
  - **Valve A2S** (CS2, Rust, TF2, L4D2, Valheim, 7D2D, ARK, and hundreds more);
  - **Minecraft Java** (Query & Server List Ping protocols);
  - **Minecraft Bedrock** (RakNet protocol);
  - **SA-MP / CR-MP** (San Andreas Multiplayer protocol);
  - **Quake / GameSpy** (Quake 3, RTCW, Call of Duty, Unreal Tournament);
  - **FiveM** (GTA V / cfx.re JSON API).
- 🌐 **Standalone External Servers Monitoring:** Add and monitor ANY game server in the world using `/mon add address:ip:port game:code` — no GameAP panel registration required!
- 🔔 **Intelligent Lifecycle Alerts:** Immediate downtime alerts with anti-flapping confirmation thresholds, recovery notifications, and server restart tracking.
- 🎨 **Modern Interactive Cards:** Clean visual progress bars `[██████░░░░] 24/32`, native Discord action buttons `[🔗 Connect]` and `[🔄 Refresh]`, official game icons and thumbnails fetched automatically from Steam CDN.
- 🎯 **Flexible Multi-Channel Routing:**
  - Master summary dashboard (`/mon bind`);
  - Per-game dedicated channels (`/mon bind game:cs2`);
  - Dedicated flagship single-server channels (`/mon bind server:1`);
  - Discord Forum Channels with automated per-game threads (`/mon forum`);
  - Dedicated host nodes and infrastructure status channel (`/mon nodes`).
- 🛡️ **100% Secure & 2FA-Immune:** Authenticates strictly via isolated plugin `X-WebMon-Token`. The bot has no RCON, console, power, or filesystem permissions and requires zero panel administrator passwords.
- 📚 **Comprehensive 297 Games Index:** Full list of games and short codes is available in [SUPPORTED_GAMES.md](SUPPORTED_GAMES.md).

---

### 🚀 Beginner-Friendly Setup Guide (5 Easy Steps)

#### Step 1. Create Bot in Discord Developer Portal
1. Navigate to the [Discord Developer Portal](https://discord.com/developers/applications) and sign in.
2. Click the blue **"New Application"** button in the top right corner, choose a name (e.g. `GameAP Monitor`), and click **Create**.
3. In the left navigation menu, click **"Bot"**:
   - Click **"Reset Token"**, confirm, and click **"Copy"** (save this token for `config.json`).
4. In the left menu, click **"OAuth2" ➔ "URL Generator"**:
   - Under **SCOPES**, check: `bot` and `applications.commands`.
   - Under **BOT PERMISSIONS**, check:
     - `Send Messages`
     - `Embed Links`
     - `Manage Messages`
     - `Read Message History`
     - `Use Slash Commands`
5. Copy the generated URL at the bottom, open it in your browser, select your Discord server, and click **Authorize**.

---

#### Step 2. Enable Bot API in GameAP Panel
1. Open your **GameAP Web Panel ➔ Plugins ➔ Web Monitoring** (or Monitoring section).
2. Locate the **"🤖 Discord / Bot API Integration"** settings block:
   - Check **"Allow access for external bots (Bot API)"**.
   - Click **"Generate Token"** and copy the secret key (`wm_sec_...`).
   - Click **"Save Settings"**.
3. Note your API endpoint URL, e.g.: `http://your-domain-or-ip:8080/api/plugins/monitoring`.

---

#### Step 3. Prepare `config.json`
1. Download the latest release from the [Releases](https://github.com/br1ckzzz/GameAP_WebMonBot/releases) page.
2. Rename `config.example.json` to `config.json`.
3. Paste your Discord Bot Token and GameAP API Token:

```json
{
  "bot": {
    "token": "PASTE_YOUR_DISCORD_BOT_TOKEN_HERE",
    "application_id": "",
    "update_interval_seconds": 30,
    "status_presence": "GameAP Monitoring",
    "embed_color_online": "#2ecc71",
    "embed_color_offline": "#e74c3c"
  },
  "gameap_api": {
    "base_url": "http://127.0.0.1:8080/api/plugins/monitoring",
    "api_token": "PASTE_YOUR_PLUGIN_TOKEN_HERE",
    "timeout_seconds": 8,
    "direct_query_enabled": true,
    "direct_query_timeout_ms": 1500
  },
  "discord": {
    "guild_id": "",
    "dashboard_channel_id": "",
    "nodes_channel_id": "",
    "nodes_update_interval_seconds": 60,
    "group_by_game": true
  },
  "notifications": {
    "enabled": true,
    "alerts_channel_id": "",
    "notify_on_offline": true,
    "notify_on_online": true,
    "notify_on_restart": true
  }
}
```

> 💡 **Tip:** You can leave `guild_id`, `dashboard_channel_id`, `nodes_channel_id`, and `alerts_channel_id` empty `""`! You can bind any channel on the fly via Discord slash commands.

---

#### Step 4. Run the Bot
- **Windows:** Double click `bot.exe` or execute `.\bot.exe` in PowerShell.
- **Linux:** Mark executable and run:
  ```bash
  chmod +x webmonbot-linux-amd64
  ./webmonbot-linux-amd64
  ```

The bot will connect to Discord and register all slash commands globally within seconds.

---

#### Step 5. Bind Channels in Discord
Go to your Discord server and run:

1. **Master Game Servers Dashboard:**
   Inside your preferred channel (e.g. `#servers-status`), type:
   ```text
   /mon bind
   ```
   *The bot binds the channel and publishes the live interactive dashboard!*

2. **Hardware & Host Nodes Monitor (New!):**
   Inside your staff or infrastructure channel (e.g. `#nodes-status`), type:
   ```text
   /mon nodes
   ```
   *The bot publishes a live status card listing all host machines with status, OS, location flags, IP, active game servers count, and network ping.*

3. **Downtime & Recovery Alerts:**
   Inside your alerts channel (e.g. `#alerts`), type:
   ```text
   /mon alerts channel:#alerts role:@Admins
   ```

---

### 📖 Slash Commands Reference (`/mon`)

All management commands are restricted to members with `Manage Server` permission or an administrator role.

| Command | Parameters | Description | Example |
| :--- | :--- | :--- | :--- |
| **`/mon bind`** | `[channel]` `[game]` `[server]` | Bind dashboard to channel. Without arguments, binds general dashboard to current channel. Specify `game:` for dedicated game card or `server:` for single-server card. | `/mon bind`<br>`/mon bind channel:#servers`<br>`/mon bind game:rust channel:#rust`<br>`/mon bind server:1 channel:#mirage` |
| **`/mon unbind`** | `[game]` `[server]` `[all]` | Unbind dashboard from channel, remove a game target, single server, or all bindings (`all:true`). | `/mon unbind`<br>`/mon unbind game:cs2`<br>`/mon unbind server:1`<br>`/mon unbind all:true` |
| **`/mon bindings`** | *none* | Display all active channel, game, and server bindings. | `/mon bindings` |
| **`/mon nodes`** | `[channel]` `[unbind]` | Manage physical host nodes and infrastructure monitoring. Publishes auto-refreshing status card with location, OS, active servers count, and network ping. | `/mon nodes`<br>`/mon nodes channel:#infrastructure`<br>`/mon nodes unbind:true` |
| **`/mon alerts`** | `[channel]` `[role]` `[enabled]` `[disable]` | Configure instant server alerts (down, up, restart, map change). Supports role mentions. | `/mon alerts`<br>`/mon alerts channel:#alerts role:@Admins`<br>`/mon alerts disable:true` |
| **`/mon forum`** | `[channel]` `[unbind]` | Bind to a Discord Forum Channel. Automatically creates dedicated per-game threads with live cards. | `/mon forum channel:#game-threads`<br>`/mon forum unbind:true` |
| **`/mon add`** | `address:` `game:` `[name]` `[channel]` `[icon]` | **Add a standalone external server** to monitor directly without GameAP via A2S/Minecraft/SAMP/FiveM protocols. | `/mon add address:185.12.34.56:27015 game:cs2`<br>`/mon add address:mc.hypixel.net:25565 game:mc name:Hypixel`<br>`/mon add address:185.12.34.56:7777 game:samp` |
| **`/mon remove`** | `address:` | Remove an external standalone server from monitoring. | `/mon remove address:185.12.34.56:27015` |
| **`/mon list`** | *none* | Display all added standalone external servers. | `/mon list` |
| **`/mon clean`** | *none* | Clean up duplicate, stale, or orphaned bot messages in the current channel. | `/mon clean` |
| **`/mon setup`** | `url:` `token:` | Configure or update GameAP WebMonitoring endpoint URL and token directly from Discord. | `/mon setup url:https://panel.site.com/api/plugins/monitoring token:wm_sec_123` |
| **`/mon status`** | *none* | Quick ephemeral status summary of all servers (visible only to you). | `/mon status` |
| **`/mon refresh`** | *none* | Force an immediate refresh of all bound dashboards and node cards. | `/mon refresh` |
| **`/mon ping`** | *none* | Check Discord Gateway latency and GameAP WebMonitoring API response time. | `/mon ping` |

---

### 🎮 Supported Games Index (297 Games)

👉 **[See the complete alphabetical Supported Games Index in SUPPORTED_GAMES.md](SUPPORTED_GAMES.md)**.

Common game codes:
- `cs2` / `csgo` — Counter-Strike 2 / Global Offensive
- `rust` — Rust
- `mc` / `minecraft` — Minecraft Java Edition
- `bedrock` — Minecraft Bedrock (PE)
- `samp` / `crmp` — San Andreas Multiplayer / CR-MP
- `valheim` — Valheim
- `7d2d` — 7 Days to Die
- `tf2` — Team Fortress 2
- `l4d2` — Left 4 Dead 2
- `arkse` / `asa` — ARK: Survival Evolved / Ascended
- `dayz` — DayZ Standalone
- `fivem` — FiveM (GTA V)
- `palworld` — Palworld
- `unturned` — Unturned

---

### 🐧 Linux Background Service (Systemd)

Create `/etc/systemd/system/webmonbot.service`:
```ini
[Unit]
Description=GameAP WebMonBot Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/webmonbot
ExecStart=/opt/webmonbot/webmonbot-linux-amd64 -config /opt/webmonbot/config.json
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```
```bash
systemctl daemon-reload && systemctl enable --now webmonbot
```

---

### 📄 License

Distributed under the **MIT License**. Free for personal and commercial use in community game hosting projects.
