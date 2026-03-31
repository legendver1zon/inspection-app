# CLAUDE.md

Этот файл содержит инструкции для Claude Code (claude.ai/code) при работе с кодом в этом репозитории.

**Язык общения: русский.** Все ответы, объяснения, комментарии к коду и коммит-сообщения — на русском языке.

## Сборка и запуск

```bash
# Сборка
go build -o inspection-app ./cmd/server

# Запуск (требуется PostgreSQL, .env с DATABASE_URL)
./inspection-app

# Запуск через Docker (PostgreSQL + Redis + приложение)
docker compose up -d
```

## Тестирование

```bash
# Только unit-тесты (без БД) — быстрые, запускать в первую очередь
go test ./internal/auth/... ./internal/storage/... ./internal/security/... ./internal/seed/... ./internal/pdf/... ./internal/logger/... -v

# Интеграционные тесты (требуется PostgreSQL)
docker compose up postgres -d
TEST_DATABASE_URL=postgres://inspection:secret@localhost:5432/inspection_test?sslmode=disable \
  go test ./... -v

# Запуск одного теста
go test ./internal/pdf/... -run TestGenerate_CreatesFile -v

# Покрытие
go test ./internal/auth/... ./internal/pdf/... -cover
```

## Архитектура

Монолит на Go 1.25: HTTP-фреймворк Gin, ORM GORM, PostgreSQL 16, Redis 7.

**Поток запроса:** `main.go` (роутер + template-функции) → `auth/middleware.go` (JWT из httpOnly cookie) → `handlers/*` (HTTP-логика) → `storage.DB` (глобальный GORM-инстанс) → PostgreSQL.

**Загрузка фото:** `PostUploadPhoto` сохраняет файл локально → пушит `inspectionID` в Redis-очередь → `worker/uploader.go` (5 горутин через BLPOP) → `UploadInspectionPhotos` загружает на Яндекс Диск → помечает `photo.upload_status` как done/failed. Фронтенд опрашивает `GET /inspections/:id/upload-status` каждые 3 сек.

**Генерация PDF:** синхронно в HTTP-обработчике. `EnsureInspectionFolder` создаёт/публикует папку на Яндекс Диске (для QR-кода) → `pdf.Generate()` создаёт PDF формата A4 через fpdf → возвращает файл для скачивания.

**Ключевые проектные решения:**
- `DefectTemplateID *uint` — nullable. `nil` означает запись «Прочее» (свободный текст)
- `WallNumber 0` = не стеновой дефект, `1-4` = стена 1-4
- Комнаты при редактировании мягко удаляются (GORM `deleted_at`). Архивные дефекты с фото показываются на странице просмотра, но не попадают в PDF
- `ActNumber` формируется из auto-increment ID (формат `ID-DDMMYY`, устанавливается в транзакции)
- Безопасность cookie: `SameSite=Lax` всегда, `Secure` только при `COOKIE_SECURE=true` (требует HTTPS)

## Важные переменные окружения

- `DATABASE_URL` — обязательна всегда
- `JWT_SECRET` — **обязательна** в production (`GIN_MODE=release`), приложение не запустится без неё
- `GIN_MODE=release` — production-режим (JSON-логи, тестовый пользователь не создаётся, JWT_SECRET обязателен)
- `COOKIE_SECURE=true` — устанавливать только при настроенном HTTPS
- `LOG_LEVEL` — debug/info/warn/error (по умолчанию: info)
- `YADISK_TOKEN` — опционально, без него фото хранятся только локально
- `REDIS_URL` — опционально, без него загрузка фото синхронная

## Соглашения по коду

- Весь пользовательский текст на русском языке
- Email всегда нормализуется: `strings.ToLower(strings.TrimSpace(email))` перед любым запросом к БД
- Логирование через `internal/logger` (на базе slog): `logger.Info(msg, key, val)`, `logger.Ctx(c.Request.Context()).Error(msg, ...)` для логов с контекстом запроса (request_id/user_id)
- События безопасности через `security.Log(event, ip, detail)` — сохранять этот паттерн для auth-логирования
- Пароли в DSN/логах маскируются через `maskDSN()` — никогда не логировать credentials в открытом виде
- Тестовый пользователь `test@example.com / Test1234!` создаётся только в dev-режиме (не при `GIN_MODE=release`)

## Production-сервер

Timeweb Cloud VPS, путь `/opt/inspection-app`. Обновление: `bash update.sh` (бэкап БД, pull, рестарт).
