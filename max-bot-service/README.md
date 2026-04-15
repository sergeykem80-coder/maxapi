# MAX Bot Integration Service

Микросервис интеграции MAX Bot API и 1С на Go.

## 🎯 Описание

Сервис предоставляет:
- Прием webhook событий от MAX Bot API
- Сохранение сессий пользователей в PostgreSQL
- REST API для 1С для получения данных о сессиях и отправки уведомлений

## 📋 Требования

- Go 1.22+
- Docker
- PostgreSQL 14+
- MAX Bot токен (получить в [MAX Platform](https://platform.max.ru/))

## 🏗️ Структура проекта

```
max-bot-service/
├── cmd/server/main.go           # Точка входа
├── internal/
│   ├── config/                  # Конфигурация
│   ├── handler/                 # HTTP handlers
│   │   ├── webhook.go           # MAX webhook handler
│   │   └── api_1c.go            # 1C API handlers
│   ├── service/                 # Бизнес-логика
│   │   └── session.go           # Session & Message services
│   ├── store/postgres/          # Репозиторий
│   │   └── session_store.go     # PostgreSQL session store
│   └── maxclient/               # MAX API client wrapper
├── pkg/logger/                  # Logger utility
├── configs/
│   ├── config.yaml.example      # Пример конфигурации
│   └── amvera.yaml              # Amvera deployment config
├── Dockerfile                   # Multi-stage build
├── go.mod                       # Go module definition
└── README.md                    # Этот файл
```

## ⚙️ Конфигурация

### Переменные окружения

| Переменная | Описание | Пример |
|------------|----------|--------|
| `MAX_BOT_TOKEN` | Токен бота из MAX Platform | `max_bot_abc123...` |
| `WEBHOOK_SECRET` | Секрет для валидации webhook | `a-zA-Z0-9_- 32+ символов` |
| `DATABASE_URL` | URL подключения к PostgreSQL | `postgres://user:pass@host:5432/dbname` |
| `ONEC_API_KEY` | Ключ авторизации для 1С API | `1c_secret_xyz...` |
| `SESSION_TTL` | Время жизни сессии | `24h` (по умолчанию) |
| `LOG_LEVEL` | Уровень логирования | `info`, `debug`, `error` |
| `PORT` | Порт HTTP сервера | `8080` (по умолчанию) |

### Пример .env файла

```bash
MAX_BOT_TOKEN=max_bot_your_token_here
WEBHOOK_SECRET=your_secure_webhook_secret_32chars
DATABASE_URL=postgres://user:password@localhost:5432/maxbot?sslmode=disable
ONEC_API_KEY=your_1c_api_key_here
SESSION_TTL=24h
LOG_LEVEL=info
PORT=8080
```

## 🚀 Запуск

### Локальная разработка

1. **Клонирование репозитория:**
```bash
git clone <repository-url>
cd max-bot-service
```

2. **Установка зависимостей:**
```bash
go mod download
```

3. **Запуск PostgreSQL (Docker):**
```bash
docker run -d \
  --name maxbot-postgres \
  -e POSTGRES_USER=user \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=maxbot \
  -p 5432:5432 \
  postgres:14-alpine
```

4. **Настройка переменных окружения:**
```bash
cp configs/config.yaml.example configs/config.yaml
# Отредактируйте configs/config.yaml или используйте .env файл
```

5. **Запуск сервиса:**
```bash
go run cmd/server/main.go
```

### Docker запуск

1. **Сборка образа:**
```bash
docker build -t max-bot-service .
```

2. **Запуск контейнера:**
```bash
docker run -d \
  --name max-bot-service \
  -p 8080:8080 \
  -e MAX_BOT_TOKEN=your_token \
  -e WEBHOOK_SECRET=your_secret \
  -e DATABASE_URL=postgres://user:pass@host:5432/dbname \
  -e ONEC_API_KEY=your_1c_key \
  max-bot-service
```

### Деплой на Amvera

1. **Установите CLI Amvera:**
```bash
curl -fsSL https://amvera.ru/install.sh | bash
```

2. **Авторизация:**
```bash
amvera login
```

3. **Деплой:**
```bash
amvera deploy
```

Или используйте переменные окружения при деплое:
```bash
amvera env set MAX_BOT_TOKEN=your_token
amvera env set WEBHOOK_SECRET=your_secret
amvera env set DATABASE_URL=postgres://...
amvera env set ONEC_API_KEY=your_key
amvera deploy
```

## 📡 API Endpoints

### Health Check

**GET /health**

Проверка работоспособности сервиса.

**Ответ:**
```json
{
  "status": "healthy",
  "timestamp": "2026-04-15T10:00:00Z"
}
```

---

### Webhook от MAX Bot

**POST /webhook**

Принимает события от MAX Bot API.

**Headers:**
- `X-Max-Bot-Api-Secret: {WEBHOOK_SECRET}`

**Тело запроса (bot_started событие):**
```json
{
  "update_id": 1,
  "type": "bot_started",
  "sender": {
    "id": 123456,
    "username": "test_user",
    "first_name": "Тест"
  },
  "chat": {
    "id": 123456
  },
  "payload": {
    "start": "secret_test123"
  }
}
```

**Ответ 200 OK:**
```json
{
  "success": true,
  "message": "session created"
}
```

---

### Получить сессию (для 1С)

**GET /api/v1/session?secret={SECRET}**

Получение данных о сессии пользователя.

**Headers:**
- `Authorization: Bearer {ONEC_API_KEY}`

**Параметры:**
- `secret` (query) - UUID сессии

**Ответ 200 OK:**
```json
{
  "success": true,
  "data": {
    "secret": "uuid",
    "user_id": 123456,
    "chat_id": 123456,
    "username": "user_name",
    "first_name": "Имя",
    "started_at": "2026-04-15T10:00:00Z",
    "is_active": true
  }
}
```

**Ответ 404 Not Found:**
```json
{
  "success": false,
  "error": "session not found"
}
```

---

### Отправить уведомление (для 1С)

**POST /api/v1/notify**

Отправка сообщения пользователю через MAX Bot.

**Headers:**
- `Authorization: Bearer {ONEC_API_KEY}`

**Тело запроса:**
```json
{
  "secret": "uuid",
  "text": "Текст уведомления (до 4000 символов)",
  "priority": "normal"
}
```

**Параметры:**
- `secret` - UUID сессии
- `text` - Текст сообщения (макс. 4000 символов)
- `priority` - Приоритет: `normal` или `urgent` (опционально)

**Ответ 200 OK:**
```json
{
  "success": true,
  "message_id": "msg_abc123",
  "delivered_at": "2026-04-15T10:05:00Z"
}
```

---

## 🗄️ База данных

### Схема sessions

```sql
CREATE TABLE sessions (
    secret VARCHAR(64) PRIMARY KEY,
    user_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,
    username VARCHAR(255),
    first_name VARCHAR(255),
    started_at TIMESTAMPTZ DEFAULT NOW(),
    last_activity TIMESTAMPTZ,
    is_active BOOLEAN DEFAULT TRUE
);

CREATE INDEX idx_sessions_secret_active ON sessions(secret, is_active);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_chat_id ON sessions(chat_id);
```

## 🔐 Безопасность

- **Webhook**: Валидация заголовка `X-Max-Bot-Api-Secret`
- **1C API**: Bearer-токен авторизация в заголовке `Authorization`
- **Rate Limiting**: 30 RPS (лимит MAX API)
- **Секреты**: Только через переменные окружения
- **Валидация**: Проверка всех входных данных

## 🧪 Тестирование

### Unit тесты

```bash
go test ./... -v
```

### Интеграционные тесты

```bash
go test ./internal/... -tags=integration -v
```

### Пример теста webhook

```bash
curl -X POST http://localhost:8080/webhook \
  -H "Content-Type: application/json" \
  -H "X-Max-Bot-Api-Secret: your_webhook_secret" \
  -d '{
    "update_id": 1,
    "type": "bot_started",
    "sender": {"id": 123456, "username": "test_user", "first_name": "Тест"},
    "chat": {"id": 123456},
    "payload": {"start": "secret_test123"}
  }'
```

### Пример запроса к 1C API

```bash
# Получить сессию
curl -X GET "http://localhost:8080/api/v1/session?secret=test123" \
  -H "Authorization: Bearer your_1c_api_key"

# Отправить уведомление
curl -X POST http://localhost:8080/api/v1/notify \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your_1c_api_key" \
  -d '{
    "secret": "test123",
    "text": "Привет! Это тестовое уведомление.",
    "priority": "normal"
  }'
```

## 📊 Мониторинг

### Health Check Endpoint

```bash
curl http://localhost:8080/health
```

### Логи

Логи выводятся в stdout в структурированном формате JSON:

```json
{
  "level": "info",
  "timestamp": "2026-04-15T10:00:00Z",
  "caller": "handler/webhook.go:45",
  "message": "processing bot_started event",
  "user_id": 123456,
  "chat_id": 123456,
  "username": "test_user",
  "secret": "test123"
}
```

## 🔧 Troubleshooting

### Ошибка подключения к БД

Проверьте правильность `DATABASE_URL`:
```bash
echo $DATABASE_URL
```

### Ошибка валидации токена MAX

Убедитесь, что токен корректен:
```bash
curl -X GET https://api.max.ru/bot/info \
  -H "Authorization: Bearer $MAX_BOT_TOKEN"
```

### Webhook не работает

1. Проверьте заголовок `X-Max-Bot-Api-Secret`
2. Убедитесь, что секрет совпадает с `WEBHOOK_SECRET`
3. Проверьте логи сервиса

## 📚 Ресурсы

- [MAX API документация](https://dev.max.ru/docs)
- [MAX примеры](https://relaya.ru/blog/max-api-overview)
- [Amvera документация](https://docs.amvera.ru/)
- [MAX Go клиент](https://github.com/max-messenger/max-bot-api-client-go)

## 📝 Лицензия

MIT
