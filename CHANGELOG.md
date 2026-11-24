# Changelog

Все важные изменения в проекте GameAP WebMonBot документируются в этом файле.

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.0.0/), проект придерживается [Semantic Versioning](https://semver.org/).

## [1.0.1] - 2026-11-24

### Безопасность (Security)
- **Защита `/mon setup` от SSRF и несанкционированного доступа:** команда теперь требует прав администратора сервера (`isServerAdmin`), валидирует URL через `validateSetupURL` (блокируются не-`http`/`https` схемы, `localhost`, loopback, link-local / cloud metadata, RFC1918 и CGNAT-подсети) и санитизирует ошибки подключения без утечки тела ответа в Discord.
- **Защита HTTP-редиректов от SSRF и права на `state.json`:** в `gameap.NewClient` добавлена проверка `CheckRedirect` (блокируются кросс-доменные редиректы на `localhost`, приватные/служебные и CGNAT IP-адреса), а файлы `state.json.tmp` / `state.json.corrupt` записываются с правами `0600` (исключены из Git в `.gitignore`).

### Исправлено (Fixed)
- **Устранены гонки данных (Data Races):**
  - В `internal/alerts/tracker.go` в фоновые горутины отправки уведомлений передаётся копия `ServerState` по значению вместо указателя.
  - В `internal/bot/bot.go` чтение `lastKnownServers` в `UpdateNodesStatus` защищено мьютексом `b.mu`.
  - В `internal/gameap/client.go` чтение `baseURL` и `apiToken` в `GetBaseURL()`, `FetchServers()`, `doFetch()` и `PushTelemetry()` защищено через `stickyMu.RLock()`.
- **Соответствие Slash-команд документации (`/mon nodes`, `/mon unbind all:true`, `/mon refresh`):**
  - Подкоманда привязки нод переименована из `nodes_channel` в `/mon nodes` (с сохранением обратной совместимости по алиасу `nodes_channel`).
  - В `/mon unbind` добавлен булевый параметр `all:true` и метод `State.UnbindAll()` для полной очистки всех привязок (общее табло, игры, серверы, форум и ноды).
  - Команда `/mon refresh` теперь дополнительно обновляет и карточку нод (`refreshNodesFn`).
- **Реальный замер пинга серверов и автообновление `/mon group`:**
  - В `internal/query/` (`QueryServer` A2S, `QueryServerAuto` для всех протоколов) добавлен замер RTT в миллисекундах (`ServerInfo.Ping`), который пробрасывается в `PublicServerDTO.Ping` и передаётся в веб-плагин через `PushTelemetry`.
  - В `RefreshDashboard` устранён баг «серверов-призраков»: удалённые в панели GameAP серверы больше не удерживаются в `lastKnownServers` после успешного ответа API; для `/mon group` добавлено живое автообновление в `updateDashboard`.
- **Работа `/mon setup` без перезапуска:** `RefreshDashboard` и `UpdateNodesStatus` теперь проверяют `b.gameapClient.GetBaseURL() != ""` вместо статического `b.cfg.GameAP.BaseURL != ""`.
- **Надёжность `state.json`:** убрано преждевременное `os.Remove` перед `os.Rename` в `SaveLocked`; при повреждённом JSON `LoadState` создаёт резервную копию `.corrupt` вместо молчаливой перезаписи; `SetCredentials` сохраняет состояние под мьютексом через `SaveLocked()`.
- **Корректное завершение работы (Graceful Shutdown):** в `cmd/bot/main.go` убран резкий `os.Exit(0)` из обработчика сигналов — бот корректно дожидается завершения `b.Start(ctx)` и вызывает `b.Stop()`; флаг `-check` учитывает `cfg.GameAP.TimeoutSeconds`.
- **Поддержка IPv6:** во всех 6 протоколах `internal/query/` (`a2s`, `bedrock`, `fivem`, `minecraft`, `quake`, `samp`) ручное форматирование `%s:%d` заменено на `net.JoinHostPort` (устранены предупреждения `go vet`).
- **Защита от nil-разыменований:** добавлены проверки `cfg != nil` / `h.cfg != nil` в `renderer.go` и `commands.go`.
- **Discord UX и жизненный цикл сообщений:**
  - В `/mon bind` вызов `deferResponse` перенесён до `purgeFunc`, чтобы избежать таймаута взаимодействия 3 секунды.
  - Поля Embed ограничены лимитом Discord (25 полей) и длина описаний ограничена 4000 символами с индикатором переполнения в `/mon status`, `/mon list`, `RenderGameGroupEmbed` и `RenderNodesStatusEmbed`.
  - Сообщения `/mon group` (ключи `game_group_`) больше не удаляются функцией `cleanupStaleMessages`.
  - При пустом `guildID` slash-команда `/mon` регистрируется только глобально, без дублирования по гильдиям.
  - Исправлена логика пересоздания удалённых веток форума в `syncForumPost`, убраны лишние вызовы `ChannelMessageDelete` и повторный `state.Save()`.
  - Исправлена опечатка в подсказке `/mon roles` (`add:@Роль` / `remove:@Роль`), добавлена валидация схемы `icon` URL и нормализация `host` в `/mon add`.
- **Алерты:** `NewTracker` использует `GetResolvedAlertRoles()`, время в алертах выводится через Discord-тег `<t:...:T>` вместо хардкода `MSK`, ошибки отправки в Discord логируются.

### Улучшено (Changed / Performance)
- **Параллельный опрос нод:** `EnrichNodeStatuses` (`nodes_probe.go`) теперь опрашивает ноды конкурентно (семафор на 8 горутин) с поддержкой отмены контекста через `net.Dialer.DialContext`.
- **Ограничение конкурентности прямых опросов:** в `queryCustomServers`, `enrichServersWithHealthcheck` и `query.EnrichServers` добавлен семафор на 16 горутин.
- **Очистка клиента GameAP:** удалено неиспользуемое поле `stickyTelemetry` и заменён `strings.NewReader(string(data))` на `bytes.NewReader(data)`.