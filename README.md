# GameAP WebMonBot 🎮🤖

[🇷🇺 Читать на русском](#-русская-версия) | [🇬🇧 Read in English](#-english-version)

---

## 🇷🇺 Русская версия

Автономный, легковесный Discord-бот для онлайн-мониторинга игровых серверов панели **GameAP** в режиме реального времени.

Работает напрямую через плагин **GameAP WebMonitoring** (по безопасному токену `X-WebMon-Token`). Не требует паролей от аккаунтов панели, обходит ограничения 2FA и не имеет прав управления серверами (RCON/Power), гарантируя 100% безопасность.

### ⚡ Преимущества:
- **Автономный бинарник:** Компилируется в один файл без внешних зависимостей. Потребление памяти всего **~15 МБ RAM**.
- **Мультипротокольный опрос:** Нативная поддержка Valve A2S (UDP), Minecraft Java & Bedrock (RakNet), SA-MP / CR-MP, Quake / GameSpy и FiveM для мгновенного получения карт и слотов игроков.
- **Внешние серверы (Standalone):** Добавление и мониторинг любых внешних серверов по команде `/mon add address:<ip:port> game:<game>` без обязательной привязки к GameAP.
- **Интерактивные карточки:** Прогресс-бар онлайна `[██████░░░░] 24/32`, категории, точный аптайм, официальные иконки игр (Steam CDN) и кнопки быстрого подключения.
- **Гибкая привязка каналов:** Вывод общего табло, каналов по конкретным играм (`/mon bind game:...`) или персональных карточек отдельных серверов (`/mon bind server:...`).
- **Своевременные алерты:** Мгновенные уведомления о падении (с защитой от ложных срабатываний), восстановлении и перезагрузке серверов.
- **Умное обновление:** Редактирует уже созданные сообщения (`state.json`), не засоряя канал дубликатами.

---

### 🚀 Быстрый запуск за 3 шага

#### Шаг 1. Получить токен в GameAP
1. Откройте **Панель GameAP ➔ Мониторинг**.
2. В блоке **«🤖 Интеграция с Discord / Bot API»**:
   - Отметьте: `Разрешить доступ для внешних ботов (Bot API)`.
   - Нажмите **«Сгенерировать токен»** и скопируйте его.
   - Нажмите **«Сохранить настройки»**.

#### Шаг 2. Создать Discord-бота
1. Перейдите на [Discord Developer Portal](https://discord.com/developers/applications) ➔ **New Application**.
2. Вкладка **Bot** ➔ Нажмите **Reset Token** и скопируйте токен.
3. Вкладка **OAuth2 ➔ URL Generator**:
   - Отметьте: `bot`, `applications.commands`.
   - Права: `Send Messages`, `Embed Links`, `Manage Messages`, `Use Slash Commands`.
   - Перейдите по сгенерированной ссылке и добавьте бота на свой сервер.

#### Шаг 3. Запуск и быстрая привязка прямо в Discord ⚡

Вам **НЕ НУЖНО** вручную искать цифровые ID сервера и каналов! Бот настраивается прямо через Discord:

1. Запустите бота:
   ```bash
   ./bin/webmonbot.exe
   ```
   *(На Linux: `./bin/webmonbot-linux-amd64`)*
   Бот сразу подключится к Discord и загорится зелёным 🟢.

2. Перейдите в текстовый канал Discord, где хотите видеть табло, и напишите команду:
    ```text
    /mon bind
    ```
    *(Или выберите канал: `/mon bind channel:#серверы`)*
    Бот привяжет этот канал, сохранит настройки и выведет живое табло мониторинга!

3. **Привязка конкретной игры или отдельного сервера к каналу** 🎯:
    ```text
    /mon bind game:cs2 channel:#cs2
    /mon bind server:1 channel:#сервер-1
    ```
    Бот будет автоматически транслировать карточку выбранной игры или персональную карточку сервера именно в назначенный канал!
    - Посмотреть все привязки: `/mon bindings`
    - Отвязать игру: `/mon unbind game:cs2`
    - Отвязать сервер: `/mon unbind server:1`

4. Очистить зависшие/дублирующие карточки бота в текущем канале:
    ```text
    /mon clean
    ```

5. Настроить или сменить адрес панели GameAP можно прямо в Discord:
    ```text
    /mon setup url:https://gameap.domain.com/api/plugins/monitoring token:wm_...
    ```

6. **Настройка оповещений (Alerts) о падении/запуске серверов**:
    ```text
    /mon alerts channel:#алерты role:@Администраторы
    ```
    - Проверить текущий статус алертов: `/mon alerts`
    - Включить/выключить: `/mon alerts enabled:true` или `/mon alerts disable:true`

7. **Привязка к Discord Forum Channels (Сценарий Б: темы по играм)**:
    ```text
    /mon forum channel:#игровые-серверы
    ```
    - Проверить текущие темы: `/mon forum`
    - Отвязать форум: `/mon forum unbind:true`

8. **Мониторинг внешних серверов (Standalone Servers)**:
    Вы можете добавить абсолютно любой игровой сервер по его `IP:PORT` (даже если он не заведён в панель GameAP):
    ```text
    /mon add address:185.12.34.56:7777 game:samp name:My SAMP Server
    /mon add address:mc.hypixel.net:25565 game:mc
    /mon add address:play.myserver.ru:19132 game:bedrock
    /mon add address:192.168.1.50:27015 game:cs2 channel:#серверы icon:https://example.com/logo.png
    ```
    - **Иконка игры**: бот автоматически подтягивает официальную обложку/логотип по указанному коду или названию игры (`minecraft`, `samp`, `cs2`, `valheim`, `quake3` и др.). Также можно задать свою прямую ссылку через опциональный параметр `icon:`.
    - **Канал табло**: по умолчанию карточка публикуется в текущий канал (либо в указанный в параметре `channel:`).
    - Посмотреть список добавленных внешних серверов: `/mon list`
    - Удалить внешний сервер из мониторинга: `/mon remove address:185.12.34.56:7777`

9. **Вспомогательные команды**:
    - `/mon status` — показать краткую сводку текущего онлайна серверов.
    - `/mon refresh` — принудительно обновить табло во всех каналах без ожидания интервала.
    - `/mon ping` — проверить задержку Discord Gateway и API GameAP.

---

### 🎮 Справочник поддерживаемых игр и кодов (Supported Games)

Бот поддерживает как полные названия игр, так и короткие сокращения (коды) при привязке каналов (`/mon bind game:...`).

👉 **[Полный алфавитный справочник игр и кодов смотрите в SUPPORTED_GAMES.md](SUPPORTED_GAMES.md)** (Counter-Strike 2, Valheim, Minecraft, Rust, 7 Days to Die, DayZ, Project Zomboid и др.).

---

### 🐧 Автозапуск на Linux VPS (Systemd)
Создайте `/etc/systemd/system/webmonbot.service`:
```ini
[Unit]
Description=GameAP WebMonBot
After=network.target

[Service]
Type=simple
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
---

## 🇬🇧 English Version

Standalone, ultra-lightweight Discord bot for real-time live monitoring of **GameAP** game servers.

Connects directly via the **GameAP WebMonitoring** plugin (using the secure `X-WebMon-Token`). Requires no GameAP admin credentials, bypasses 2FA restrictions, and has no server management permissions (RCON/Power), ensuring 100% security.

### ⚡ Highlights:
- **Standalone Binary:** Single self-contained executable with zero dependencies. Consumes only **~15 MB RAM**.
- **Multi-Protocol Direct Query:** Native out-of-the-box support for Valve A2S (UDP), Minecraft Java & Bedrock (RakNet), SA-MP / CR-MP, Quake / GameSpy, and FiveM for instant real-time maps and player slots.
- **Standalone Servers:** Add and monitor any external game server via `/mon add <address:port> <game>` without requiring GameAP.
- **Rich Interactive Cards:** Visual progress bars `[██████░░░░] 24/32`, categories, exact uptime, official game icons (Steam CDN), and quick-connect buttons.
- **Flexible Channel Binding:** Run a unified general dashboard, per-game channels (`/mon bind game:...`), or individual dedicated server channels (`/mon bind server:...`).
- **Real-Time Lifecycle Alerts:** Immediate alerts for server downtime (with anti-flapping filters), recoveries, and restarts.
- **Clean In-Place Updates:** Edits existing dashboard messages (`state.json`) instead of spamming duplicates.

---

### 🚀 Quick Start in 3 Steps

#### Step 1. Get Token in GameAP
1. Go to **GameAP Panel ➔ Web Monitoring**.
2. In the **"🤖 Discord / Bot API Integration"** card:
   - Check `Allow access for external bots (Bot API)`.
   - Click **"Generate Token"** and copy it.
   - Click **"Save Settings"**.

#### Step 2. Create Discord Bot
1. Navigate to the [Discord Developer Portal](https://discord.com/developers/applications) ➔ **New Application**.
2. Under **Bot** tab ➔ Click **Reset Token** and copy the bot token.
3. Under **OAuth2 ➔ URL Generator**:
   - Check `bot` and `applications.commands` scopes.
   - Permissions: `Send Messages`, `Embed Links`, `Manage Messages`, `Use Slash Commands`.
   - Open the generated link and invite the bot to your Discord server.

#### Step 3. Launch & Native Discord Binding ⚡

You **DO NOT** need to search for numerical channel IDs! The bot can be configured directly inside Discord:

1. Launch the bot:
   ```bash
   ./bin/webmonbot.exe
   ```
   *(On Linux: `./bin/webmonbot-linux-amd64`)*
   The bot connects to Discord immediately and turns green 🟢.

2. In the text channel where you want the dashboard, type:
   ```text
   /mon bind
   ```
   *(Or specify another channel: `/mon bind channel:#servers`)*
   The bot will automatically bind the channel, save the state, and publish the live dashboard!

3. **Bind a specific game or individual server to a channel** 🎯:
   ```text
   /mon bind game:cs2 channel:#cs2
   /mon bind server:1 channel:#server-1
   ```
   The bot will automatically broadcast cards for that game or a dedicated single-server card to the designated channel!
   - View all bindings: `/mon bindings`
   - Unbind game: `/mon unbind game:cs2`
   - Unbind server: `/mon unbind server:1`

4. Sweep stale or duplicate bot messages in the current channel:
   ```text
   /mon clean
   ```

5. You can also configure or update your GameAP endpoint right from Discord:
   ```text
   /mon setup url:https://my-gameap.com/api/plugins/monitoring token:wm_...
   ```

6. Configure server lifecycle alerts directly in Discord:
   ```text
   /mon alerts channel:#alerts role:@Admins
   ```
   - Check current alerts status: `/mon alerts`
   - Enable or disable: `/mon alerts enabled:true` or `/mon alerts disable:true`

7. Configure Discord Forum Channels (Scenario B: dedicated game threads):
   ```text
   /mon forum channel:#game-servers
   ```
   - Check active forum threads: `/mon forum`
   - Unbind forum: `/mon forum unbind:true`

8. **Monitor Standalone Servers (Custom IP:Port)**:
   Add any external game server directly without needing GameAP:
   ```text
   /mon add address:185.12.34.56:7777 game:samp name:My SAMP Server
   /mon add address:mc.hypixel.net:25565 game:mc
   /mon add address:play.myserver.com:19132 game:bedrock
   /mon add address:192.168.1.50:27015 game:cs2
   ```
   - View added external servers: `/mon list`
   - Remove an external server: `/mon remove address:185.12.34.56:7777`

9. **Utility Commands**:
   - `/mon status` — display a quick summary of server statuses in chat.
   - `/mon refresh` — force an immediate refresh across all bound channels.
   - `/mon ping` — test Discord Gateway and GameAP API latencies.

---

### 🎮 Supported Games Directory
The bot supports both full game names and short codes when binding channels (`/mon bind game:...`).

👉 **[See the complete alphabetical Supported Games Index in SUPPORTED_GAMES.md](SUPPORTED_GAMES.md)** (Counter-Strike 2, Valheim, Minecraft, Rust, 7 Days to Die, DayZ, Project Zomboid, etc.).

---

### 🐧 Linux VPS Service (Systemd)
Create `/etc/systemd/system/webmonbot.service`:
```ini
[Unit]
Description=GameAP WebMonBot
After=network.target

[Service]
Type=simple
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
