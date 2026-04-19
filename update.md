# Inspection App — План развития и технический долг

**Последнее обновление:** 2026-04-19
**Текущий статус:** Production (Timeweb Cloud, 5.42.105.93:8080)
**Версия:** Go 1.25 / Gin / GORM / PostgreSQL 16 / Redis 7

---

## ⚠️ Миграция перед деплоем (2026-04-19)

Перед `bash update.sh` на VPS **обязательно** прокатить SQL-миграцию — иначе
новый код не сможет переиспользовать `act_number` от удалённых осмотров:

```bash
ssh root@5.42.105.93
cd /opt/inspection-app
docker compose exec -T postgres psql -U inspection -d inspection_db \
    < migrations/001_partial_unique_act_number.sql
```

Миграция заменяет обычный unique-индекс на `act_number` на **partial unique
index** (`WHERE deleted_at IS NULL`). Идемпотентно — можно запускать повторно.

**Что чинит:** раньше soft-deleted записи блокировали переиспользование номера
акта; теперь удалённые осмотры не резервируют номер. Также добавлена
pre-validation в `PostEditInspection`, чтобы конфликт ловился **до**
удаления комнат — инцидент с потерей 10 помещений больше не воспроизводится.

---

## Метрики проекта

| Метрика | Значение |
|---------|----------|
| Go-файлов | 51 |
| Строк Go-кода | 11,051 |
| HTML-шаблонов | 14 |
| Строк HTML | 2,428 |
| Строк CSS | 1,761 |
| Строк JS | 406 |
| Тест-файлов | 22 |
| Тест-функций | 213 (83 unit + 114 integration + 16 load) |
| API endpoints | 36 |
| DB-моделей | 7 (User, Inspection, InspectionRoom, RoomDefect, DefectTemplate, Photo, Document) |
| Зависимостей (прямых) | 9 (+gorilla/websocket) |
| Docker-сервисов | 3 (postgres, redis, app) |

---

## Общая оценка (на 2026-03-31)

| Аспект | Оценка | Комментарий |
|--------|--------|-------------|
| Архитектура | 8/10 | templatefuncs вынесены, main.go ~220 строк. Global vars остаются |
| Код | 8/10 | Structured logging, транзакции, интерфейсы (RateLimiter, FileStorage) |
| UX/UI | 7.5/10 | Dark mode, toast, mobile responsive, dashboard, accordion, beforeunload |
| Производительность | 8/10 | Async фото через Redis, semaphore Yandex Disk, WebSocket upload-status |
| Безопасность | 8.5/10 | JWT+bcrypt+rate limiting (Redis)+trusted proxies+SameSite+security logging |
| Тесты | 6.5/10 | 213 тестов. Нет e2e, нет тестов конкурентности |
| Production readiness | 8/10 | Docker, CI/CD, graceful shutdown, healthz, backup скрипт |
| **ОБЩАЯ** | **7.5/10** | Крепкий MVP, готов для 100-200 пользователей |

---

## Что было сделано

### Начальный аудит (сессия 1)
- [x] JWT_SECRET обязателен в production (log.Fatal при GIN_MODE=release)
- [x] Тестовый admin не создаётся в production
- [x] Cookie: Secure (через COOKIE_SECURE) + SameSite=Lax + HttpOnly
- [x] Пароль БД маскируется в логах (maskDSN)
- [x] Защита от удаления последнего администратора
- [x] Content-Disposition санитизация имени файла
- [x] LIKE wildcards экранируются (escapeLike)
- [x] ActNumber через ID (race condition fix) + uniqueIndex
- [x] PostEditInspection обёрнут в DB.Transaction
- [x] N+1 запросы при удалении — subquery
- [x] Фото: DB update до удаления файла
- [x] Множественная загрузка фото (multiple file input)
- [x] Structured logging через log/slog (JSON в production)
- [x] Request ID middleware + Panic recovery middleware
- [x] Toast-уведомления вместо alert()
- [x] 83 unit-теста + cookie/auth/PDF/logger тесты

### Этап 1 — Стабилизация (сессия 2)
- [x] DB-индексы: `inspections.status`, `photos.upload_status`
- [x] Health-check endpoint `/healthz` (DB + disk space)
- [x] DB connection pool (MaxOpenConns=25, MaxIdle=5, ConnMaxLifetime=5m)
- [x] `beforeunload` предупреждение на edit page

### Этап 2 — Надёжность (сессия 2)
- [x] PostDeleteInspection обёрнут в транзакцию + удаление фото
- [x] Trusted Proxies настроены (127.0.0.1, ::1)
- [x] Пароль БД вынесен из docker-compose.yml в .env (`${POSTGRES_PASSWORD}`)
- [x] Disk space в /healthz (warning >80%, critical >90%)
- [x] Скрипт backup.sh (pg_dump + gzip + ротация 7 дней)
- [x] Скрипт cleanup-orphans.sh (очистка осиротевших файлов)
- [ ] HTTPS (домен + Nginx + Let's Encrypt) — **ждёт покупки домена**

### Этап 3 — Рост (сессия 2)
- [x] Template functions из main.go → `internal/templatefuncs/` (~240 строк)
- [x] Inline JS из edit.html → `edit-rooms.js` + `edit-plan.js` (~250 строк)
- [x] Semaphore на Yandex Disk API (макс 3 параллельных запроса)
- [x] Поиск по номеру акта на странице осмотров
- [x] Dashboard со статистикой (`/dashboard`)
- [x] Redis-backed rate limiter (интерфейс `RateLimiter` + fallback на in-memory)
- [x] Accordion для комнат в форме редактирования
- [ ] PDF generation queue (фоновая задача) — **отложено, пока нет нагрузки**

### Этап 4 — Масштабирование (сессия 2, частично)
- [x] WebSocket для upload-status (gorilla/websocket + hub + fallback на polling)
- [x] Мобильная адаптация (touch 44px, стековые фильтры, адаптивные фото/dashboard)
- [ ] Dependency injection (struct App) — **делать при 2-м разработчике**
- [ ] S3-совместимое хранилище — **делать когда Yandex Disk мешает**
- [ ] CDN для статики — **делать вместе с доменом/HTTPS**
- [ ] Горизонтальное масштабирование — **не нужно до 500+ пользователей**
- [ ] PWA (offline-кеш) — **отложено**

### Этап 5 — Продукт (не начат)
- [ ] Multi-tenancy (несколько организаций)
- [ ] API для внешних интеграций
- [ ] Экспорт в Excel/Word
- [ ] Шаблоны актов (настройка под клиента)
- [ ] Роль "клиент" (просмотр своих актов)
- [ ] Интеграция с CRM

---

## Технический долг (актуальный)

### Закрыто в этой сессии
- ~~A3: main.go 435 строк~~ → 220 строк (templatefuncs вынесены)
- ~~A5: Дублирование windowTypeName/wallTypeName~~ → templatefuncs экспортирует
- ~~D1: Нет индексов на status~~ → добавлены
- ~~D2: Нет индекса на upload_status~~ → добавлен
- ~~D5: Нет DB connection pool~~ → MaxOpenConns=25, MaxIdle=5
- ~~D6: PostDeleteInspection без транзакции~~ → обёрнут в Transaction
- ~~F1: Нет cleanup orphan-файлов~~ → cleanup-orphans.sh
- ~~F2: Нет мониторинга disk space~~ → /healthz проверяет диск
- ~~U1: Inline JS в edit.html~~ → edit-rooms.js + edit-plan.js
- ~~U2: Нет beforeunload~~ → добавлен
- ~~U4: Нет dashboard~~ → /dashboard
- ~~U5: Форма перегружена~~ → accordion для комнат

### Остаётся

| # | Проблема | Где | Приоритет |
|---|---------|-----|-----------|
| A1 | handlers — 6000+ строк в одном пакете | internal/handlers/ | Средний |
| A2 | Глобальные переменные (storage.DB, cloudStore) | handlers, storage | Средний |
| A4 | Дублирование buildInitials | handlers/auth.go + seed/users.go | Низкий |
| A6 | Worker импортирует handlers (обратная зависимость) | worker/uploader.go | Средний |
| D3 | InspectionRoom — 20+ числовых полей (Window1-5) | models.go | Низкий |
| D7 | 9 мест с DB.First() без проверки ошибки | handlers/ | Средний |
| F3 | Локальные файлы без backup | web/static/uploads/ | Средний |

---

## Риски при росте

### 100 пользователей — ✅ закрыто
- ~~Yandex Disk rate limiting~~ → semaphore 3 req/s
- ~~Disk заполняется~~ → /healthz мониторинг
- ~~In-memory rate limiter~~ → Redis-backed
- LIKE-запросы на 5000+ записей → pg_trgm GIN-индекс (при необходимости)

### 1000 пользователей
| Риск | Вероятность | Решение |
|------|-------------|---------|
| 50+ PDF одновременно | Высокая | PDF queue (фоновая генерация) |
| Yandex Disk API заблокирован | Высокая | S3 storage |
| Один Go процесс | Средняя | 2+ инстанса + LB |

---

## Архитектура: текущая

```
cmd/server/main.go              (~220 строк — routing + init)
internal/
  templatefuncs/  (250 строк — template functions из main.go)
  handlers/       (6200 строк — обработчики + ws.go)
  models/         (144 строк — GORM модели)
  storage/        (80 строк — DB подключение + pool)
  auth/           (549 строк — JWT + cookie)
  security/       (650 строк — rate limit memory + Redis + валидация)
  pdf/            (1398 строк — PDF генерация)
  cloudstorage/   (470 строк — Yandex Disk + semaphore)
  queue/          (89 строк — Redis queue)
  worker/         (167 строк — background uploader)
  logger/         (298 строк — structured logging)
  seed/           (384 строк — seed данные)
  mailer/         (87 строк — SMTP)
scripts/
  backup.sh           — pg_dump + gzip + ротация 7 дней
  cleanup-orphans.sh  — очистка осиротевших файлов
```

---

## Коммиты проекта

```
abe1e20 feat: мобильная адаптация + WebSocket для upload-status
792b3d4 feat: этапы 1-3 — стабилизация, надёжность, рост
6db650f test: PDF integration тесты + logger тесты
6af7bbe feat: structured logging + UI system (toast, loading, transitions)
ec3a235 fix: убрать PublishFile для каждого фото + последовательное создание папок
8b53755 test: 34 новых unit-теста
fb2449e feat: множественная загрузка фото
23475f0 fix: три бага загрузки фото в облако
8fbcdda fix: cookie Secure=false по умолчанию (HTTP-совместимость)
813a869 security: аудит + исправления безопасности и надёжности
```

---

## Важные файлы проекта

| Файл | Назначение |
|------|------------|
| cmd/server/main.go | Точка входа, router (~220 строк) |
| internal/templatefuncs/funcs.go | Template functions для HTML |
| internal/handlers/inspections.go | CRUD осмотров + dashboard |
| internal/handlers/photos.go | Загрузка фото + cloud sync + WS notify |
| internal/handlers/ws.go | WebSocket hub для upload-status |
| internal/handlers/documents.go | PDF генерация и скачивание |
| internal/handlers/auth.go | Login, register, logout |
| internal/handlers/admin.go | Управление пользователями |
| internal/auth/cookie.go | Централизованное управление cookie |
| internal/security/ratelimit.go | Rate limiter (интерфейс + in-memory) |
| internal/security/ratelimit_redis.go | Rate limiter на Redis |
| internal/models/models.go | 7 GORM моделей |
| internal/pdf/generator.go | PDF генерация (1024 строки) |
| internal/cloudstorage/yandex.go | Yandex Disk API + semaphore |
| internal/worker/uploader.go | Background фото uploader |
| web/static/css/style.css | Стили (1280 строк + mobile responsive) |
| web/static/css/ui.css | UI система (toast, loading, transitions) |
| web/static/js/ui.js | Toast, button loading |
| web/static/js/edit-rooms.js | Управление комнатами + accordion |
| web/static/js/edit-plan.js | Кроп плана + beforeunload |
| web/templates/inspections/dashboard.html | Страница статистики |
| docker-compose.yml | 3 сервиса, пароль через .env |
| scripts/backup.sh | Бэкап БД (cron) |
| scripts/cleanup-orphans.sh | Очистка orphan-файлов (cron) |

---

## Среда и переменные окружения

| Переменная | Обязательность | Описание |
|------------|----------------|----------|
| DATABASE_URL | **да** | PostgreSQL DSN |
| JWT_SECRET | **да в production** | Секрет для JWT (min 32 символа) |
| GIN_MODE | рекомендовано | `release` для production |
| POSTGRES_PASSWORD | рекомендовано | Пароль PostgreSQL в docker-compose |
| YADISK_TOKEN | нет | OAuth-токен Яндекс Диска |
| YADISK_ROOT | нет | Корневая папка (default: disk:/inspection-app) |
| REDIS_URL | нет | Redis (без него — синхронные фото, in-memory rate limiter) |
| LOG_LEVEL | нет | debug/info/warn/error (default: info) |
| COOKIE_SECURE | нет | `true` при HTTPS |
| SMTP_HOST/PORT/USER/PASS/FROM | нет | SMTP для сброса пароля |

---

## Команда обновления сервера

```bash
ssh root@5.42.105.93
cd /opt/inspection-app
bash update.sh
docker compose logs app --tail=20
```

update.sh делает: pg_dump → git pull → docker pull → docker up → логи

---

## Security Review (2026-03-31)

**Оценка: 8.5/10.** Критических уязвимостей нет.

### Исправлено
- [x] Пароль БД в docker-compose.yml → `${POSTGRES_PASSWORD:-secret}`
- [x] Trusted Proxies → `SetTrustedProxies(["127.0.0.1", "::1"])`
- [x] `os.Remove()` ошибки теперь логируются

### Остаётся (средний/низкий приоритет)
- [ ] `c.Param("id")` в admin.go без `strconv.Atoi` — GORM prepared statements защищают
- [ ] `c.File(absPath)` из DB-поля без проверки директории — риск низкий
- [ ] Нет Content-Security-Policy заголовка — inline JS вынесен, можно добавить CSP
- [ ] Нет rate limit на admin-endpoints

### Что реализовано хорошо
| Защита | Статус |
|--------|--------|
| SQL injection | GORM prepared statements |
| XSS | Go templates автоэкранирование |
| CSRF | SameSite=Lax cookie |
| JWT | HS256, httpOnly, 24ч, секрет enforced в prod |
| Brute force | Rate limiting (Redis/memory) на login/register/forgot |
| File upload | MIME validation + size limits |
| Пароли | bcrypt, политика сложности |
| Credentials в логах | maskDSN() |
| Panic recovery | Middleware с логированием |

---

*Документ обновлён: 2026-03-31*
