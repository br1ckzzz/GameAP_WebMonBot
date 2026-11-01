# Дизайн-документ: Поддержка расширенных сетевых протоколов и внешних серверов (Standalone Servers)

- **Дата:** 2026-10-01
- **Проект:** GameAP WebMonBot (v1.0.0)
- **Статус:** Одобрено к планированию

---

## 1. Контекст и цели

### 1.1. Контекст
В боте **GameAP WebMonBot** мониторинг изначально базировался на получении списка серверов из панели управления GameAP через плагин `GameAP_WebMonitoring` с обогащением данных через Valve A2S и Minecraft Java SLP.
Для обеспечения паритета с возможностями DiscordGSM и расширения аудитории требуется:
1. Добавить опрос серверов по популярным протоколам:
   - **SA-MP / CR-MP** (San Andreas Multiplayer — UDP бинарный протокол).
   - **Minecraft Bedrock** (MCPE — UDP RakNet протокол).
   - **Quake 2/3 / GameSpy 1** (UDP `getstatus` — Call of Duty, RTCW, Medal of Honor, Quake 3).
   - **FiveM** (GTA V — HTTP JSON endpoints).
2. Реализовать добавление и удаление любых внешних игровых серверов по `IP:PORT` через слэш-команды Discord (`/mon add`, `/mon remove`, `/mon list`) с сохранением в локальном состоянии бота `state.json`.

### 1.2. Ограничения
- Версия бота строго зафиксирована на **1.0.0** (без изменения версии).
- Бот остаётся полностью автономным одиночным бинарником на чистом Go без сторонних внешних демонов (Node.js, Python) и без CGo.
- Плагин `GameAP_WebMonitoring` не затрагивается (функционал внешних серверов реализуется исключительно на стороне бота).

---

## 2. Архитектура сетевых протоколов (`internal/query`)

### 2.1. Модули опроса
В пакете `internal/query` создаются/расширяются следующие файлы:

1. **`samp.go` (SA-MP & CR-MP):**
   - **Транспорт:** UDP.
   - **Формат запроса:**
     - 4 байта: Сигнатура `SAMP` (`0x53, 0x41, 0x4D, 0x50`).
     - 4 байта: IP-адрес сервера (IPv4 octets).
     - 2 байта: Порт сервера (uint16, LittleEndian).
     - 1 байт: Опкод `'i'` (`0x69` — Server Information).
   - **Формат ответа:**
     - 11 байт префикс (сигнатура + IP + порт + опкод).
     - 1 байт: Наличие пароля (`password`).
     - 2 байта: Количество игроков (uint16, LittleEndian).
     - 2 байта: Максимум игроков (uint16, LittleEndian).
     - 4 байта (uint32 length) + bytes: Название сервера (`hostname`).
     - 4 байта (uint32 length) + bytes: Режим игры (`gamemode`).
     - 4 байта (uint32 length) + bytes: Карта (`mapname`).
   - Функция: `QuerySAMP(address string, port int, timeout time.Duration) (*ServerInfo, error)`.

2. **`bedrock.go` (Minecraft Bedrock Edition / MCPE):**
   - **Транспорт:** UDP RakNet.
   - **Формат запроса (Unconnected Ping):**
     - 1 байт: `0x01`.
     - 8 байт: Client Timestamp (int64, BigEndian).
     - 16 байт: RakNet Magic GUID (`0x00, 0xFF, 0xFF, 0x00, 0xFE, 0xFE, 0xFE, 0xFE, 0xFD, 0xFD, 0xFD, 0xFD, 0x12, 0x34, 0x56, 0x78`).
     - 8 байт: Client GUID (uint64).
   - **Формат ответа (Unconnected Pong):**
     - 1 байт: `0x1C`.
     - 8 байт: Timestamp.
     - 8 байт: Server GUID.
     - 16 байт: Magic GUID.
     - 2 байта: Длина строки данных (uint16, BigEndian).
     - Payload: Строка токенов, разделенная точкой с запятой `;`:
       - `[0]` Game Edition (`MCPE`)
       - `[1]` Server Name / MOTD
       - `[2]` Protocol Version
       - `[3]` Game Version (e.g. `1.20.50`)
       - `[4]` Online Players
       - `[5]` Max Players
       - `[7]` Map / Sub-MOTD
       - `[8]` Game Mode (`Survival`, `Creative`)
   - Функция: `QueryBedrock(address string, port int, timeout time.Duration) (*ServerInfo, error)`.

3. **`quake.go` (Quake 2/3, RTCW, CoD, GameSpy 1):**
   - **Транспорт:** UDP.
   - **Запрос:** `\xFF\xFF\xFF\xFFgetstatus\n`.
   - **Ответ:** Заголовок `\xFF\xFF\xFF\xFFstatusResponse\n` + строка CVAR'ов, разделенных слэшами `\`, за которой следуют строки игроков.
   - Извлечение CVAR: `hostname` / `sv_hostname`, `mapname`, `sv_maxclients`, подсчет количества активных игроков.
   - Функция: `QueryQuake(address string, port int, timeout time.Duration) (*ServerInfo, error)`.

4. **`fivem.go` (FiveM GTA V):**
   - **Транспорт:** HTTP GET.
   - **Запрос:** `http://<address>:<port>/dynamic.json` (с fallback на `http://<address>:<port>/info.json` и `/players.json`).
   - Извлечение: `hostname`, `mapname`, `clients` (онлайн), `sv_maxclients` (макс. онлайн), `gametype`.
   - Функция: `QueryFiveM(address string, port int, timeout time.Duration) (*ServerInfo, error)`.

5. **`dispatcher.go` (Единый маршрутизатор):**
   - Функция: `QueryServerAuto(gameCode, address, port, timeout time.Duration) (*ServerInfo, error)`.
   - Маршрутизация на основе нормализованного `gameCode` / имени игры:
     - `samp`, `crmp`, `gta_sa` $\to$ `QuerySAMP`
     - `minecraft_bedrock`, `mcpe`, `bedrock` $\to$ `QueryBedrock`
     - `minecraft`, `mc` $\to$ `QueryMinecraft` (SLP TCP)
     - `quake2`, `quake3`, `cod`, `rtcw`, `mohaa` $\to$ `QueryQuake`
     - `fivem`, `gtav` $\to$ `QueryFiveM`
     - Все остальные игры из справочника (`cs2`, `valheim`, `7d2d`, `rust`, `ark`, `tf2` и др.) $\to$ `QueryServer` (Valve A2S с учётом `GetQueryPort` и fallback).
   - Если специфичный протокол возвращает ошибку, диспетчер пробует опрос через A2S.

---

## 3. Хранение и интеграция внешних серверов

### 3.1. Модель данных `CustomServer`
В [internal/bot/state.go](file:///f:/GameAP_WebMonBot/internal/bot/state.go):
```go
type CustomServer struct {
    ID        uint64    `json:"id"`
    Name      string    `json:"name"`
    Address   string    `json:"address"`
    Port      int       `json:"port"`
    GameCode  string    `json:"game_code"`
    GameName  string    `json:"game_name"`
    CreatedAt time.Time `json:"created_at"`
}
```

### 3.2. Хранилище `State`
- Поле в структуре `State`: `CustomServers map[string]CustomServer` (ключ: `address:port`).
- Потокобезопасные методы:
  - `AddCustomServer(srv CustomServer) error`
  - `RemoveCustomServer(addrPort string) bool`
  - `GetCustomServers() []CustomServer`
- Данные атомарно сохраняются в файл `state.json`.

### 3.3. Интеграция в мониторинг (`internal/bot/bot.go`)
- В функции `RefreshDashboard(ctx)`:
  1. Опрос GameAP API (если настроен). Ошибки GameAP не блокируют опрос внешних серверов.
  2. Параллельный опрос всех `CustomServers` через `QueryServerAuto`.
  3. Конвертация ответов в `gameap.PublicServerDTO` со статусами `online` / `offline`.
  4. Объединение серверов GameAP и внешних серверов в единый срез.
  5. Передача в `alerts.ProcessUpdates` (алерты падения/поднятия работают и для внешних серверов).
  6. Передача в `updateDashboard` (отображение в общем канале или каналах игр).

---

## 4. Команды Discord (`/mon add`, `/mon remove`, `/mon list`)

### 4.1. Подкоманды
В [internal/bot/commands.go](file:///f:/GameAP_WebMonBot/internal/bot/commands.go):
1. `/mon add address:<ip:port> game:<code_or_name> [name:<custom_name>]`
   - Валидация адреса и порта.
   - Резолв кода игры по каталогу [SUPPORTED_GAMES.md](file:///f:/GameAP_WebMonBot/SUPPORTED_GAMES.md).
   - Мгновенный проверочный опрос сервера. Если `name` не задано — подстановка имени из ответа сервера.
   - Сохранение в `state.json` и запуск `RefreshDashboard`.
   - Ответ пользователю красивым информационным эмбедом.
2. `/mon remove address:<ip:port>`
   - Удаление сервера из `state.json`.
   - Запуск `RefreshDashboard`.
   - Подтверждение удаления.
3. `/mon list`
   - Отображение списка всех добавленных внешних серверов с их ID, адресами, играми и статусами.

---

## 5. План тестирования и валидации
1. Юнит-тесты протоколов с mock UDP/HTTP серверами:
   - `samp_test.go`
   - `bedrock_test.go`
   - `quake_test.go`
   - `fivem_test.go`
   - `dispatcher_test.go`
2. Юнит-тесты хранилища `State` для внешних серверов.
3. Прогон всех тестов `go test ./...`.
4. Сборка бинарников под Windows (`bin/webmonbot.exe`) и Linux (`bin/webmonbot-linux-amd64`).
