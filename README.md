# Акты осмотра

Веб-приложение для формирования и ведения актов осмотра объектов недвижимости. Инспекторы фиксируют дефекты по каждому помещению, прикрепляют фотографии, которые асинхронно выгружаются в Яндекс Диск, и формируют PDF-документ с QR-кодом.

## Возможности

- **Создание актов осмотра** — номер акта, дата, время, адрес, этаж, площадь, температура, влажность, имена сторон
- **Помещения и дефекты** — до 10 помещений на акт; по каждому фиксируются дефекты окон, потолка, стен (4 стены), пола, дверей, сантехники
- **Замеры** — длина, ширина, высота помещения, параметры до 5 окон и двери
- **Загрузка плана** — фото/план помещений прикрепляется к акту
- **Фото дефектов** — jpg, jpeg, png, webp; до 30 штук на дефект; MIME-валидация по содержимому файла
- **Асинхронная загрузка в облако** — при генерации PDF папка создаётся и публикуется мгновенно (для QR-кода), фото загружаются в фоне через Redis-очередь
- **Прогресс-бар** — JS-поллинг каждые 3 секунды показывает ход загрузки фото в облако
- **Яндекс Диск** — структура `inspections/{id}/{Комната}/{Секция}/{Дефект}/photo_N.jpg`; опционально — без `YADISK_TOKEN` фото хранятся только локально
- **Генерация PDF** — акт в формате A4 с таблицами дефектов, замерами и блоком подписей
- **QR-код в PDF** — ссылка на публичную папку с фото на Яндекс Диске
- **Поиск и фильтрация** — по адресу, собственнику, инспектору (admin), диапазону дат, статусу; пагинация по 20 записей
- **Роли** — `admin` (все акты + управление пользователями) и `inspector` (только свои акты)
- **Безопасность** — rate limiting (5 попыток входа / 15 мин с IP), политика паролей (≥6 симв. + заглавная + цифра + спецсимвол), MIME-валидация загрузок, crypto/rand для кодов сброса пароля
- **Восстановление пароля** — 6-значный код на email, действует 15 минут
- **Тёмная тема** — переключатель в navbar, сохраняется в localStorage

## Технологии

| Слой | Стек |
|---|---|
| Язык | Go 1.25 |
| Web-фреймворк | [Gin](https://github.com/gin-gonic/gin) |
| БД | PostgreSQL 16 через [GORM](https://gorm.io) |
| Очередь задач | Redis 7 (RPUSH / BLPOP) |
| Аутентификация | JWT в httpOnly cookie (24ч) |
| PDF | [go-pdf/fpdf](https://github.com/go-pdf/fpdf) |
| QR-код | [skip2/go-qrcode](https://github.com/skip2/go-qrcode) |
| Облако | Яндекс Диск REST API |
| Шрифты | Liberation Sans (embedded via `go:embed`) |
| Фронтенд | HTML-шаблоны Go + CSS (без JS-фреймворков) |
| Хостинг | [Timeweb Cloud](https://timeweb.cloud) |

## Структура проекта

```
inspection-app/
├── cmd/server/         # Точка входа, роутер, template functions, graceful shutdown
├── internal/
│   ├── auth/           # JWT, bcrypt, middleware RequireAuth / RequireAdmin
│   ├── cloudstorage/   # Интерфейс FileStorage + адаптер Яндекс Диска
│   ├── handlers/       # HTTP-обработчики (inspections, auth, profile, admin, documents, photos)
│   ├── models/         # GORM-модели (User, Inspection, Room, Defect, Photo, Document)
│   ├── pdf/            # Генератор PDF (fpdf), embedded шрифты, QR-код
│   ├── queue/          # Redis-очередь задач загрузки фото (RPUSH / BLPOP)
│   ├── security/       # Rate limiter, валидация паролей, MIME, логирование безопасности
│   ├── seed/           # 48 шаблонов дефектов + тестовый аккаунт
│   ├── storage/        # Подключение к БД, AutoMigrate
│   └── worker/         # Фоновый воркер загрузки фото (5 горутин, graceful shutdown)
└── web/
    ├── static/
    │   ├── css/        # style.css — дизайн + dark mode + адаптив (768px / 480px)
    │   ├── js/
    │   └── uploads/    # Загруженные планы и фото дефектов (runtime)
    └── templates/
        ├── auth/       # login, register, profile, forgot/reset password
        ├── inspections/# list, edit, view (с прогресс-баром загрузки)
        ├── admin/      # users, edit_user
        └── partials/   # navbar, base
```

## Переменные окружения

| Переменная | Обязательность | Описание |
|---|---|---|
| `DATABASE_URL` | **да** | DSN для PostgreSQL |
| `YADISK_TOKEN` | нет | OAuth-токен Яндекс Диска. Без него фото хранятся только локально |
| `YADISK_ROOT` | нет | Корневая папка на диске (по умолчанию `disk:/inspection-app`) |
| `REDIS_URL` | нет | `redis://host:6379`. Без него фото загружаются синхронно |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_USER` / `SMTP_PASS` / `SMTP_FROM` | нет | SMTP для восстановления пароля |
| `GIN_MODE` | нет | `release` для production |

Пример `.env`:
```
DATABASE_URL=postgres://inspection:secret@localhost:5432/inspection_db?sslmode=disable
YADISK_TOKEN=your_token_here
REDIS_URL=redis://localhost:6379
```

## Запуск локально (через Docker)

```bash
git clone https://github.com/legendver1zon/inspection-app.git
cd inspection-app

cp .env.example .env
# Вставить YADISK_TOKEN при необходимости

docker compose up -d
# → http://localhost:8080
```

docker-compose поднимает PostgreSQL 16 + Redis 7 + приложение. Redis включён с `appendonly yes` — задачи не теряются при рестарте.

## Запуск локально (без Docker)

1. Установить и запустить PostgreSQL и Redis
2. Создать базу данных:
```sql
CREATE USER inspection WITH PASSWORD 'secret';
CREATE DATABASE inspection_db OWNER inspection;
```
3. Настроить `.env`:
```bash
cp .env.example .env
```
4. Собрать и запустить:
```bash
go build -o app ./cmd/server
./app
# → http://localhost:8080
```

## Деплой на Timeweb Cloud (Docker)

1. Создать VPS на [timeweb.cloud](https://timeweb.cloud) (Ubuntu 22.04+)
2. Установить Docker:
```bash
curl -fsSL https://get.docker.com | sh
```
3. Клонировать репозиторий:
```bash
git clone https://github.com/legendver1zon/inspection-app.git /opt/inspection-app
cd /opt/inspection-app
```
4. Создать `.env`:
```bash
echo "YADISK_TOKEN=your_token" > .env
```
5. Поднять PostgreSQL, Redis и приложение:
```bash
docker compose up -d --build
```

## Тесты

151 тест: unit-тесты (без БД) + интеграционные (PostgreSQL).

```bash
# Только unit (не требуют БД)
go test ./internal/auth/... ./internal/seed/... ./internal/security/... -v

# Все тесты
docker compose up postgres -d
TEST_DATABASE_URL=postgres://inspection:secret@localhost:5432/inspection_test?sslmode=disable \
  go test ./... -v
```

| Пакет | Тестов | Тип |
|---|---|---|
| `internal/auth` | 13 | Unit — JWT, bcrypt |
| `internal/seed` | 11 | Unit — шаблоны дефектов |
| `internal/security` | 13 | Unit — rate limiter, валидация паролей |
| `handlers/auth` | 13 | Интеграционные |
| `handlers/admin` | 12 | Интеграционные |
| `handlers/profile` | 8 | Интеграционные |
| `handlers/reset` | 10 | Интеграционные |
| `handlers/documents` | 9 | Интеграционные |
| `handlers/inspections` | 30 | Интеграционные |
| `handlers/photos` | 13 | Интеграционные |
| `handlers/photos_load` | 8 | Нагрузочные (конкурентная загрузка) |
| `handlers/security` | 11 | Интеграционные (rate limit, MIME, политика паролей) |
| **Итого** | **151** | |

## Роли пользователей

| Роль | Возможности |
|---|---|
| `inspector` | Создание и редактирование своих актов, генерация PDF, загрузка фото |
| `admin` | Всё то же + просмотр всех актов + управление пользователями + фильтр по инспектору + удаление любых актов |

Первый зарегистрированный пользователь получает роль `admin`, остальные — `inspector`.

**Тестовый аккаунт** (создаётся автоматически при старте):
- Email: `test@example.com`
- Пароль: `Test1234!`
- Роль: `admin`

## Справочник дефектов

При первом запуске автоматически заполняется справочник из 48 дефектов, сгруппированных по секциям:

- **Окна** (7 позиций) — отклонения, зазоры, трещины, фурнитура, уплотнители...
- **Потолок** (7 позиций) — перепады, трещины, пятна...
- **Стены** (8 позиций) — неровности, трещины, отслоения...
- **Пол** (8 позиций) — перепады, скрипы, сколы плитки...
- **Двери** (8 позиций) — зазоры, перекос, фурнитура...
- **Сантехника** (10 позиций) — подтёки, уклоны, крепления...

## Структура фото на Яндекс Диске

```
disk:/inspection-app/
  inspections/
    25/                         ← публичная папка (URL → QR-код в PDF)
      Кухня/
        Потолок/
          Трещины/
            photo_1.jpg
        Стены/
          Стена_1/
            Отклонение/
              photo_1.jpg
```

## Архитектура загрузки фото

```
POST /inspections/:id/generate
  │
  ├─ [синхронно, ~2-3 сек]
  │   EnsureInspectionFolder() → публичный URL папки
  │   pdf.Generate()           ← QR-код уже готов
  │   queue.Push(inspectionID) ─────────────────────────────┐
  │                                                         │
  └─ [асинхронно, фоновый воркер]                          │
      Redis Queue ←────────────────────────────────────────┘
        └─ 5 горутин (BLPOP)
            └─ UploadInspectionPhotos()
                upload_status: pending → uploading → done/failed

GET /inspections/:id/upload-status → {total, done, pending, uploading, failed, all_done}
```

Если Redis недоступен — фото загружаются синхронно (fallback).
