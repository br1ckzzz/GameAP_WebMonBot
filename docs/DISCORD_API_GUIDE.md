# Документация и справочные материалы по Discord API

В этом документе собраны ключевые ссылки на официальную документацию Discord Developer Portal, а также критические технические ограничения API, которые необходимо соблюдать при создании мониторинг-бота.

---

## 1. Официальные разделы Discord Developer Documentation

- **Общее введение в Discord API:**  
  https://discord.com/developers/docs/intro
- **Слэш-команды и интеракции (Application Commands):**  
  https://discord.com/developers/docs/interactions/application-commands
- **Кнопки и компоненты сообщений (Message Components):**  
  https://discord.com/developers/docs/interactions/message-components
- **Каналы, ветки и треды (Channels & Threads Resource):**  
  https://discord.com/developers/docs/resources/channel
- **Форум-каналы (Forum Channels):**  
  https://discord.com/developers/docs/resources/channel#forum-channel-object
- **Вебхуки (Webhook Resource):**  
  https://discord.com/developers/docs/resources/webhook

---

## 2. Критические лимиты Discord API (Rate Limits & Constraints)

При разработке ботов реального времени нарушение лимитов приводит к коду ошибки `HTTP 429 Too Many Requests` или временной блокировке токена.

### 2.1. Лимиты на редактирование сообщений (Editing Messages)
- **Маршрут:** `PATCH /channels/{channel.id}/messages/{message.id}`
- **Лимит:** Не более **5 запросов за 5 секунд** на конкретный канал.
- **Рекомендация для бота:** Интервал опроса и редактирования сообщений серверов должен составлять **30–60 секунд**. Если серверов много (например, 10), их следует объединять в 1–2 сводных сообщения, чтобы не тратить лимит на каждое отдельное сообщение.

### 2.2. Лимиты на Embed-сообщения (Embed Limits)
- **Количество Embeds в одном сообщении:** Максимум 10.
- **Длина заголовка (Title):** До 256 символов.
- **Длина описания (Description):** До 4096 символов.
- **Количество полей (Fields):** До 25 полей в одном Embed.
- **Длина названия поля (Field Name):** До 256 символов.
- **Длина значения поля (Field Value):** До 1024 символов.
- **Общий суммарный объём текста во всех полях:** Не более 6000 символов.

### 2.3. Лимиты на кнопки и ссылки (Components)
- В одном сообщении может быть до 5 рядов кнопок (`ActionRow`), по 5 кнопок в каждом (всего до 25 кнопок).
- Кнопки типа `Link` (ссылки) поддерживают только протоколы `http://` и `https://`. Протоколы вроде `steam://` или `ts3server://` Discord отклоняет при валидации JSON payload.

---

## 3. Рекомендуемые библиотеки разработчика

### Для Go (Golang)
- **`bwmarrin/discordgo`** (GitHub: https://github.com/bwmarrin/discordgo)  
  Самая популярная и стабильная библиотека для Go. Поддерживает Gateway, WebSockets, слэш-команды и работу с компонентами.
- **`disgo`** (GitHub: https://github.com/disgoorg/disgo)  
  Современная высокопроизводительная библиотека для Go с отличной поддержкой Interaction API.

### Для Python
- **`disnake`** (GitHub: https://github.com/DisnakeDev/disnake)  
  Активный форк discord.py с полной поддержкой компонентов V2, форумов и модальных окон.
- **`discord.py`** (GitHub: https://github.com/Rapptz/discord.py)  
  Оригинальная классическая библиотека Python.