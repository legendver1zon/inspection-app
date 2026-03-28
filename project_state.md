# 📋 PROJECT STATE — inspection-app

> **Единый источник правды о проекте. Обновлять после каждого изменения кода.**
> Последнее обновление: 2026-03-28 (рефакторинг системы фото — блоки A/B/C/D реализованы, коммит `170b6a0`)

---

## 1. 📌 Общая информация

| Поле | Значение |
|------|----------|
| **Название** | inspection-app |
| **Назначение** | Веб-приложение для составления актов осмотра квартир: ввод дефектов, фото, генерация PDF |
| **Стек** | Go 1.25, Gin, GORM, PostgreSQL 16, FPDF, Яндекс Диск REST API, Redis |
| **Статус** | Production-ready MVP |
| **Docker** | `ghcr.io/legendver1zon/inspection-app:latest` |
| **Порт** | 8080 |

---

## 2. 🗂 Полная структура проекта

```
inspection-app/
├── .github/workflows/
│   └── docker.yml                  # CI/CD: сборка и push Docker image в ghcr.io
├── .env                            # Переменные окружения (gitignored)
├── .env.example                    # Шаблон переменных
├── Dockerfile                      # Multi-stage build: Go builder + Alpine runtime
├── docker-compose.yml              # PostgreSQL 16 + Redis 7 + приложение
├── go.mod                          # Go 1.25.4, 14 зависимостей
├── go.sum
│
├── cmd/server/
│   └── main.go                     # Точка входа: роутер, template functions, инициализация
│
├── internal/
│   ├── auth/
│   │   ├── auth.go                 # JWT (24ч), bcrypt, Claims{UserID, Role}
│   │   ├── auth_test.go            # Тесты: хэши, токены (13 тестов, без БД)
│   │   └── middleware.go           # RequireAuth, RequireAdmin middleware
│   │
│   ├── cloudstorage/
│   │   ├── storage.go              # Интерфейс FileStorage (EnsurePath, UploadFile, PublishFolder, PublishFile, FolderExists, MoveFolder)
│   │   ├── yandex.go               # Реализация Яндекс Диска через REST API (+ FolderExists, MoveFolder)
│   │   └── yandex_test.go          # Тесты FolderExists/MoveFolder через httptest.Server (6 тестов, без БД)
│   │
│   ├── handlers/
│   │   ├── auth.go                 # GetLogin, PostLogin, GetRegister, PostRegister, PostLogout
│   │   ├── auth_test.go            # Тесты auth handlers (13 тестов, требуют TEST_DATABASE_URL)
│   │   ├── inspections.go          # GetInspections, GetNewInspection, GetInspection (+archivedDefects), GetEditInspection, PostEditInspection (+EnsureFolder горутина), PostUploadPlan, PostDeleteInspection, GetUploadStatus
│   │   ├── inspections_test.go     # Тесты инспекций (30 тестов, требуют TEST_DATABASE_URL)
│   │   ├── documents.go            # PostGenerateDocument (только EnsureFolder + PDF, без upload-триггера), GetDownloadDocument, PostDeleteDocument
│   │   ├── photos.go               # PostUploadPhoto (+Redis push сразу), DeletePhoto, EnsureInspectionFolder (+автомиграция ID→ActNumber), UploadInspectionPhotos, SyncInspectionPhotos
│   │   ├── photos_test.go          # Тесты фото (13 тестов, требуют TEST_DATABASE_URL)
│   │   ├── photos_ensure_test.go   # Тесты EnsureInspectionFolder, buildUploadTask, buildDefectInfoMap (8 тестов)
│   │   ├── profile.go              # GetProfile, PostProfile, PostUploadAvatar
│   │   ├── admin.go                # GetAdminUsers, PostAdminChangeRole, GetAdminEditUser, PostAdminEditUser, DeleteAdminUser
│   │   ├── reset.go                # GetForgotPassword, PostForgotPassword, GetResetPassword, PostResetPassword
│   │   └── setup_test.go           # Test utilities и fixtures
│   │
│   ├── queue/
│   │   └── redis.go                # RedisQueue (RPUSH/BLPOP), Push/Pop/Len/Close, NewFromEnv
│   │
│   ├── worker/
│   │   └── uploader.go             # Uploader: Start/Stop, loop (BLPOP), processJob, recoverOnStartup, requeueFailed
│   │
│   ├── models/
│   │   └── models.go               # GORM модели: User, Inspection, InspectionRoom, RoomDefect, DefectTemplate, Document, Photo
│   │
│   ├── pdf/
│   │   ├── generator.go            # Генерация PDF (FPDF, QR-код, кириллица)
│   │   └── fonts/                  # Embedded шрифты Liberation Sans (go:embed)
│   │
│   ├── mailer/
│   │   └── mailer.go               # SMTP отправка (STARTTLS/TLS), восстановление пароля
│   │
│   ├── seed/
│   │   └── defects.go              # 48 шаблонов дефектов по 6 секциям
│   │
│   └── storage/
│       └── db.go                   # PostgreSQL подключение, AutoMigrate 7 моделей
│
└── web/
    ├── static/
    │   ├── css/style.css           # Полный дизайн + dark mode + адаптивность (768px, 480px)
    │   ├── js/main.js              # Минимальный JS (подтверждение удаления)
    │   ├── favicon.svg             # SVG иконка
    │   ├── documents/              # Сгенерированные PDF (runtime, gitignored)
    │   └── uploads/                # Загруженные файлы (runtime, gitignored)
    │       ├── photos/{inspectionID}/{defectID}/photo_*.jpg
    │       └── avatar_*.jpg, plan_*.jpg
    │
    └── templates/
        ├── partials/
        │   ├── base.html           # DOCTYPE, head, meta, скрипты
        │   └── navbar.html         # Навигация, меню профиля, переключатель темы
        ├── auth/
        │   ├── login.html          # Форма входа
        │   ├── register.html       # Регистрация (ФИО, email, пароль)
        │   ├── profile.html        # Профиль (ФИО, инициалы, пароль, аватар)
        │   ├── forgot_password.html
        │   └── reset_password.html
        ├── inspections/
        │   ├── list.html           # Список актов (draft/completed/all, фильтры)
        │   ├── new.html            # Редирект на edit нового акта
        │   ├── edit.html           # Редактор акта: шапка, помещения, дефекты, план
        │   └── view.html           # Просмотр, генерация PDF, документы
        └── admin/
            ├── users.html          # Список пользователей
            └── edit_user.html      # Редактирование пользователя
```

---

## 3. 🧩 Реализованный функционал

### Аутентификация
- Вход / Выход (`/internal/handlers/auth.go`)
- Регистрация: валидация ФИО (2-3 слова), политика пароля (≥6 симв. + заглавная + цифра + спецсимвол), уникальность email
- Первый пользователь → `admin`, остальные → `inspector`
- Автогенерация инициалов: `"Иванов Иван Иванович"` → `"Иванов И. И."`
- JWT в httpOnly cookie (24 часа) (`/internal/auth/auth.go`)
- Middleware: `RequireAuth`, `RequireAdmin` (`/internal/auth/middleware.go`)
- **Rate limiting**: 5 неудачных попыток входа / 15 мин с IP; 3 регистрации / час с IP; 3 сброса пароля / час с IP
- **Logging**: каждый вход, выход, регистрация, ошибка — `[SECURITY] event=... ip=...`

### Восстановление пароля
- 6-значный код на `crypto/rand` (криптографически безопасный) (`/internal/handlers/reset.go`)
- Срок действия кода: 15 минут
- SMTP: STARTTLS (587) и implicit TLS (465) (`/internal/mailer/mailer.go`)
- **Важно**: mailer не настроен, требует SMTP-конфигурации

### Профиль
- Редактирование ФИО, инициалов (`/internal/handlers/profile.go`)
- Смена пароля (с проверкой текущего)
- Загрузка аватара

### Осмотры (Акты)
- CRUD с поддержкой черновиков (`draft`) и завершённых (`completed`) (`/internal/handlers/inspections.go`)
- **Нейминг актов**: `{N}-{DDMMYY}` (пример: `18-270326` = акт №18 от 27.03.26)
- **Статус** меняется прямо на странице просмотра (`POST /inspections/:id/status`), не в редакторе
- **Дата** редактируется в форме редактирования (input type="date")
- До 10 помещений на осмотр
- Для каждого помещения:
  - Замеры: длина/ширина/высота, до 5 окон (высота/ширина), дверь
  - Тип окон: ПВХ / Алюминий / Дерево
  - Тип стен: Окраска / Плитка / ГКЛ (множественный выбор)
  - 6 секций дефектов: окна, потолок, стены (4 стены отдельно), пол, двери, сантехника
  - 48 преднастроенных шаблонов дефектов с порогами и единицами (`/internal/seed/defects.go`)
  - Поле "Прочее" для любой секции (с поддержкой фото — исправлен баг)
- **Общие замечания по квартире** (не привязаны к помещению): Электричество, Вентиляция, Общие замечания
  - Хранятся в полях `Inspection.Electricity`, `Inspection.Ventilation`, `Inspection.GeneralNotes`
  - Отображаются в view, редактируются в edit, выводятся в PDF отдельной секцией

### Фильтрация и пагинация осмотров
- По статусу (draft/completed)
- По собственнику (LIKE), адресу (LIKE)
- По инспектору (только для admin)
- По диапазону дат
- Счётчики по вкладкам
- Inspector видит только свои осмотры, admin — все
- **Пагинация**: 20 записей на страницу, кнопки ← / →, счётчик "Показано X из Y · Страница N из M"
  - `const pageSize = 20` в `inspections.go`
  - Важно: `listQ` обязан иметь `.Model(&models.Inspection{})` для корректной работы `Count`
  - `listQ.Session(&gorm.Session{}).Count(&totalCount)` — клонирование, чтобы не сломать Preload
  - `prevPage`/`nextPage` вычисляются в Go-хендлере (не в шаблоне — Go templates передают int-константы как int64, что ломает функцию `add(a,b int)`)

### Безопасность загрузки файлов
- Аватар: проверка расширения + MIME по содержимому + лимит 5 МБ (`/internal/handlers/profile.go`)
- План помещений: проверка расширения + MIME по содержимому + лимит 20 МБ (`/internal/handlers/inspections.go`)
- Фото дефектов: расширение + лимит 20 МБ + максимум 30 штук на дефект (`/internal/handlers/photos.go`)

### Фотографии
- Загрузка к каждому дефекту: jpg, jpeg, png, webp (`/internal/handlers/photos.go`)
- Локальное хранение: `web/static/uploads/photos/{inspectionID}/{defectID}/`
- Иерархия в облаке: `inspections/{ActNumber}/{RoomName}/{Section}/{DefectName}/photo_{n}.jpg`
- Удаление с проверкой прав (инспектор — только свои, admin — любые)
- `upload_status` на каждом фото: `pending` → `uploading` → `done` / `failed`
- **Статусные иконки в view.html**: ⏳ (pending/uploading) и ✗ (failed) через CSS `::after` на `data-upload-status`; прогресс-бар удалён

### Асинхронная загрузка фото (Redis) — актуальная архитектура
- **Загрузка сразу при добавлении** (`PostUploadPhoto`): после сохранения файла сразу `queue.Push(inspectionID)` → воркер грузит немедленно
- **Fallback**: если Redis недоступен → `go SyncInspectionPhotos(inspectionID)` (горутина)
- **При генерации PDF** (`PostGenerateDocument`): только `EnsureInspectionFolder` + чтение актуального `PhotoFolderURL`; загрузка фото НЕ запускается (уже загружены)
- **При сохранении акта** (`PostEditInspection`): `go EnsureInspectionFolder(id)` в горутине — создаёт/переименовывает папку
- Воркер (`/internal/worker/uploader.go`): 5 горутин, BLPOP, graceful shutdown, recovery при рестарте
- Статус загрузки: `GET /inspections/:id/upload-status` → JSON `{total, pending, uploading, done, failed, all_done}` — используется для лёгкого JS polling; при `all_done=true` страница перезагружается

### Архив удалённых дефектов (view.html)
- После редактирования акта старые `RoomDefect` soft-delete-ятся (GORM, `deleted_at`)
- `GetInspection` собирает удалённые дефекты с фото через `.Unscoped()` → `[]ArchivedDefect`
- В `view.html` серый блок внизу (после помещений, до документов): `linear-gradient(135deg,#94a3b8,#64748b)`
- Только просмотр (нет кнопки 📷), фото с публичными ссылками на Яндекс Диск
- В PDF НЕ попадают; загрузка фото продолжается (воркер Unscoped)

### PDF генерация
- FPDF с поддержкой кириллицы, go:embed шрифты (`/internal/pdf/generator.go`)
- Страница 1: Шапка акта + план помещений + таблица замеров + подписи
- Следующие страницы: Дефекты по помещениям → Общие замечания (Электричество/Вентиляция/Общие замечания) → QR-код → Подписи
- Поле "Прочее" в PDF: отображается только текст замечания (без префикса "Прочее:")
- Единицы дефектов (мм и др.) НЕ добавляются автоматически к значениям в PDF
- QR-код со ссылкой на Яндекс Диск (если есть фото)
- Сохранение в `web/static/documents/`

### Облачное хранилище (Яндекс Диск)
- REST API (OAuth token) (`/internal/cloudstorage/yandex.go`)
- Опционально: без `YADISK_TOKEN` фото хранятся только локально
- Интерфейс `FileStorage`: `EnsurePath`, `UploadFile`, `PublishFolder`, `PublishFile`, `FolderExists`, `MoveFolder`
- **Нейминг папок**: `inspections/{ActNumber}/` (не по числовому ID)
- **Автомиграция**: если существует `inspections/{ID}` и нет `inspections/{ActNumber}` → `MoveFolder` при первом `EnsureInspectionFolder`
- Публикация папок (публичный URL → QR-код)

### Администрирование
- Управление пользователями: редактирование, удаление, смена роли (`/internal/handlers/admin.go`)
- Удаление осмотров (только admin)
- Просмотр всех осмотров системы

---

## 4. 🗄 База данных

### Таблицы и модели (`/internal/models/models.go`)

| Таблица | Ключевые поля |
|---------|--------------|
| `users` | id, email (UNIQUE), password_hash, full_name, initials, role (admin/inspector), avatar_url, reset_token, reset_expiry |
| `inspections` | id, act_number, user_id (FK), date, address, status (draft/completed), plan_image, photo_folder_url, rooms_count, floor, total_area, temp_outside, temp_inside, humidity, owner_name, developer_rep_name |
| `inspection_rooms` | id, inspection_id (FK), room_number, room_name, length/width/height, window_1-5_height/width, door_height/width, window_type, wall_type |
| `room_defects` | id, room_id (FK), defect_template_id (FK, NULLABLE), section, value, wall_number (0-4), notes |
| `defect_templates` | id, section (INDEX), name, threshold, unit, order_index |
| `photos` | id, defect_id (FK, INDEX), file_url, file_path, file_name, upload_status (pending\|uploading\|done\|failed, default:done) |
| `documents` | id, inspection_id (FK), format (pdf), file_path, generated_by (FK) |

### Связи
```
User          1:N  Inspection
User          1:N  Document
Inspection    1:N  InspectionRoom
Inspection    1:N  Document
InspectionRoom 1:N RoomDefect
RoomDefect    ?:1  DefectTemplate  (nullable)
RoomDefect    1:N  Photo
```

### Миграции
- AutoMigrate через GORM при старте (`/internal/storage/db.go`)
- Перед миграцией: `UPDATE room_defects SET defect_template_id = NULL WHERE defect_template_id = 0`

---

## 5. 🔗 Связи между компонентами

```
main.go
  ├── storage.ConnectFromEnv()   → internal/storage/db.go      → PostgreSQL
  ├── storage.Migrate()          → AutoMigrate всех моделей
  ├── seed.SeedDefectTemplates() → internal/seed/defects.go    → 48 записей в defect_templates
  ├── cloudstorage.NewYandexDisk() → internal/cloudstorage/yandex.go
  ├── queue.NewFromEnv()         → internal/queue/redis.go     → Redis (опционально)
  ├── worker.New(q).Start(ctx,5) → internal/worker/uploader.go → 5 горутин BLPOP
  └── Gin Router
        ├── RequireAuth          → internal/auth/middleware.go → internal/auth/auth.go (JWT)
        ├── RequireAdmin         → internal/auth/middleware.go
        ├── handlers/auth.go     → models + auth.go
        ├── handlers/inspections.go → models + storage.DB (+ GetUploadStatus)
        ├── handlers/photos.go   → models + storage.DB + cloudstorage (EnsureInspectionFolder, UploadInspectionPhotos)
        ├── handlers/documents.go → models + storage.DB + pdf.Generate() + queue.Push / SyncPhotos fallback
        ├── handlers/profile.go  → models + storage.DB
        ├── handlers/admin.go    → models + storage.DB
        └── handlers/reset.go    → models + mailer.go
```

---

## 6. 🌐 HTTP Маршруты

### Публичные
| Метод | Путь | Handler |
|-------|------|---------|
| GET | `/` | Редирект на /login или /inspections |
| GET/POST | `/login` | GetLogin / PostLogin |
| GET/POST | `/register` | GetRegister / PostRegister |
| GET/POST | `/forgot-password` | GetForgotPassword / PostForgotPassword |
| GET/POST | `/reset-password` | GetResetPassword / PostResetPassword |

### Защищённые (RequireAuth)
| Метод | Путь | Handler |
|-------|------|---------|
| POST | `/logout` | PostLogout |
| GET | `/inspections` | GetInspections |
| GET | `/inspections/new` | GetNewInspection |
| GET | `/inspections/:id` | GetInspection |
| GET/POST | `/inspections/:id/edit` | GetEditInspection / PostEditInspection |
| POST | `/inspections/:id/upload-plan` | PostUploadPlan |
| POST | `/inspections/:id/generate` | PostGenerateDocument |
| GET | `/inspections/:id/upload-status` | GetUploadStatus (JSON статус загрузки фото) |
| GET | `/documents/:id/download` | GetDownloadDocument |
| POST | `/documents/:id/delete` | PostDeleteDocument |
| POST | `/defects/:id/photos` | PostUploadPhoto |
| POST | `/photos/:id/delete` | DeletePhoto |
| GET/POST | `/profile` | GetProfile / PostProfile |
| POST | `/profile/avatar` | PostUploadAvatar |

### Admin Only (RequireAdmin)
| Метод | Путь | Handler |
|-------|------|---------|
| GET | `/admin/users` | GetAdminUsers |
| GET/POST | `/admin/users/:id/edit` | GetAdminEditUser / PostAdminEditUser |
| POST | `/admin/users/:id/role` | PostAdminChangeRole |
| POST/DELETE | `/admin/users/:id/delete` | DeleteAdminUser |
| POST | `/admin/inspections/:id/delete` | PostDeleteInspection |

---

## 7. 🧪 Тесты

### Расположение
```
internal/auth/auth_test.go              # 13 unit-тестов, НЕ требуют БД
internal/seed/defects_test.go           # 11 unit-тестов, НЕ требуют БД
internal/security/security_test.go      # 13 unit-тестов, НЕ требуют БД
internal/handlers/setup_test.go         # Общие fixtures, helpers, роутер для тестов
internal/handlers/auth_test.go          # 13 интеграционных, требуют TEST_DATABASE_URL
internal/handlers/admin_test.go         # 12 интеграционных, требуют TEST_DATABASE_URL
internal/handlers/profile_test.go       # 8 интеграционных, требуют TEST_DATABASE_URL
internal/handlers/reset_test.go         # 10 интеграционных, требуют TEST_DATABASE_URL
internal/handlers/documents_test.go     # 9 интеграционных, требуют TEST_DATABASE_URL
internal/handlers/inspections_test.go   # 30 интеграционных, требуют TEST_DATABASE_URL
internal/handlers/photos_test.go        # 13 интеграционных, требуют TEST_DATABASE_URL
internal/handlers/photos_load_test.go   # 8 нагрузочных, требуют TEST_DATABASE_URL
internal/handlers/security_test.go      # 11 интеграционных (rate limit, MIME, policy), требуют TEST_DATABASE_URL
```

### Покрытие
| Пакет | Тестов | Тип | Статус |
|-------|--------|-----|--------|
| `internal/auth` | 13 | Unit (без БД) | ✅ Всегда запускаются |
| `internal/seed` | 11 | Unit (без БД) | ✅ Всегда запускаются |
| `internal/security` | 13 | Unit (без БД) | ✅ Всегда запускаются |
| `internal/handlers` (auth) | 13 | Интеграционные | ⚠️ Требуют `TEST_DATABASE_URL` |
| `internal/handlers` (admin) | 12 | Интеграционные | ⚠️ Требуют `TEST_DATABASE_URL` |
| `internal/handlers` (profile) | 8 | Интеграционные | ⚠️ Требуют `TEST_DATABASE_URL` |
| `internal/handlers` (reset) | 10 | Интеграционные | ⚠️ Требуют `TEST_DATABASE_URL` |
| `internal/handlers` (documents) | 9 | Интеграционные | ⚠️ Требуют `TEST_DATABASE_URL` |
| `internal/handlers` (inspections) | 30 | Интеграционные | ⚠️ Требуют `TEST_DATABASE_URL` |
| `internal/handlers` (photos) | 13 | Интеграционные | ⚠️ Требуют `TEST_DATABASE_URL` |
| `internal/handlers` (photos_load) | 8 | Нагрузочные | ⚠️ Требуют `TEST_DATABASE_URL` |
| `internal/handlers` (security) | 11 | Интеграционные | ⚠️ Требуют `TEST_DATABASE_URL` |
| **Итого** | **151** | | |

### Запуск тестов
```bash
# Только unit-тесты (без БД)
go test ./internal/auth/... ./internal/seed/... -v

# Все тесты (нужна БД)
docker compose up postgres -d
TEST_DATABASE_URL=postgres://inspection:secret@localhost:5432/inspection_test?sslmode=disable go test ./... -v
```

### НЕ покрыто тестами
- `internal/pdf` — PDF генерация (требует реального FPDF, сложно изолировать)
- `internal/mailer` — SMTP (требует реального сервера)
- `internal/cloudstorage` — Яндекс Диск (требует OAuth токен)
- `internal/storage` — подключение к БД
- `internal/queue` — Redis очередь (требует реального Redis)
- `internal/worker` — фоновый воркер (требует Redis + интеграции)
- `handlers/documents.go` — `PostGenerateDocument` (вызывает PDF генератор)

---

## 8. ⚙️ Конфигурация (переменные окружения)

| Переменная | Обязательная | Описание |
|------------|-------------|----------|
| `DATABASE_URL` | ✅ | `postgres://user:pass@host:5432/db?sslmode=disable` |
| `JWT_SECRET` | ✅ | Секрет для подписи JWT |
| `YADISK_TOKEN` | ❌ | OAuth токен Яндекс Диска |
| `YADISK_ROOT` | ❌ | Корневая папка (default: `disk:/inspection-app`) |
| `SMTP_HOST` | ❌ | SMTP сервер |
| `SMTP_PORT` | ❌ | 587 (STARTTLS) или 465 (TLS) |
| `SMTP_USER` | ❌ | Логин SMTP |
| `SMTP_PASS` | ❌ | Пароль SMTP |
| `SMTP_FROM` | ❌ | Email отправителя |
| `REDIS_URL` | ❌ | `redis://localhost:6379` — без этой переменной фото загружаются синхронно |
| `GIN_MODE` | ❌ | `release` для production |

---

## 9. 🕓 История изменений (Git)

```
170b6a0  feat: рефакторинг системы фото (блоки A–D) + тесты
af29c55  docs: уточнение блока C — серая плашка для архива удалённых дефектов
aa28525  docs: план рефакторинга системы фото (раздел 17) + фикс buildDefectInfoMap
65e83ea  fix:  buildDefectInfoMap uses Unscoped to find soft-deleted defects after edit
596054d  feat: статус в view, дата в edit, нейминг актов, общие замечания, фото Прочее, PDF без единиц
6255f5a  feat: security hardening, async photo upload via Redis, 151 tests
8962d07  feat: pagination, test account seed, email normalization, integration tests
8af0090  feat: PDF measurements table improvements
0e62f3d  feat: dark mode, UI fixes, PDF improvements
41e571e  ci:   GitHub Actions → Docker build + push в ghcr.io
a6e591b  feat: admin user editing, delete button, FIO validation
69da379  feat: Docker-контейнеризация
5e7e9ea  fix:  DefectTemplateID *uint, PDF text overflow, DB cleanup
53e0274  fix:  тесты используют отдельную БД inspection_test
42a3bac  fix:  убраны FK-предупреждения в тестах
6705aea  feat: миграция с SQLite на PostgreSQL
a204f94  feat: поиск по актам, фото дефектов на Яндекс Диск, QR-код в PDF
b075d4d  fix:  measurements table pinned to bottom of page 1
54d4756  fix:  page-break splitting defect name from value
4e81bdb  fix:  threshold display для Царапины, measurements overflow
ab4ec75  fix:  убран stray X, единицы для Царапин, заголовки окон
55fdb40  fix:  перенос длинного текста в PDF
120c0e7  feat: логирование в файл /var/log/inspection-app/app.log
c5f446c  fix:  кнопка показа пароля на входе
88c0def  fix:  favicon синий фон
8af5c27  fix:  favicon заполняет пространство
1435429  feat: SVG favicon
dfc6bd5  chore: удалён render.yaml, обновлён .gitignore для Timeweb
5a744ab  fix:  ширина шапки профиля, смена пароля в отдельной секции
5c47eb3  fix:  переключатель пароля, выравнивание профиля
24d9dac  feat: подтверждение пароля, показ/скрытие пароля
32ac056  feat: UI: avatar circle в navbar + правый drawer
744013e  fix:  PDF план пропорциональный, measurements на стр.1
42132fc  feat: динамические окна (до 5 на комнату)
d298fcb  feat: убрана страница new, переставлены блоки в редакторе
8ffb2f5  feat: автогенерация инициалов из ФИО при регистрации
7cddfc3  fix:  поворот в кроппере, наложение строк PDF-шапки
b8715fb  feat: редактирование шапки акта, правка кроппера
2f0d030  feat: Cropper.js, план на отдельной странице PDF
a6ed485  feat: удаление актов (admin) и PDF-документов
19982a6  fix:  tab counts, stale cookie redirect
62252c6  feat: landing redirect, password recovery, inspection tabs
0129e5b  fix:  unit 'мм' для двух дефектов стен
186f4bd  fix:  уборка ± кнопок, форматирование темп/влажности в PDF
d89baff  fix:  кнопка ± для отрицательной температуры
717e7d8  fix:  персистентная БД через DB_PATH
1875091  fix:  встраиваем шрифты через go:embed
951bf0a  feat: просмотр дефектов + улучшения PDF + чекбоксы стен
0f0663d  feat: генерация и скачивание PDF
ba0117d  feat: помещения как основная единица + все секции + PDF
2147330  feat: initial MVP — акты осмотра квартир
```

---

## 10. 🐛 Исправленные баги

### Email case-sensitivity (2026-03-26)

**Проблема:** email сравнивался с учётом регистра — `Test@mail.com` и `test@mail.com` считались разными пользователями. Привело к созданию дубликата аккаунта `LegendVerizon@gmail.com` (ID=4) поверх существующего `legendverizon@gmail.com` (ID=1).

**Merge дубликатов:**
- ID=4 (`LegendVerizon@gmail.com`, inspector, 2 осмотра) → данные перенесены в ID=1
- 2 осмотра перенесены: `UPDATE inspections SET user_id = 1 WHERE user_id = 4`
- ID=4 удалён из БД
- Все email в БД нормализованы через `UPDATE users SET email = LOWER(email)`

**Решение в коде** — `strings.ToLower(strings.TrimSpace(...))` добавлен в:
- `internal/handlers/auth.go` → `PostLogin`, `PostRegister`
- `internal/handlers/reset.go` → `PostForgotPassword`, `PostResetPassword`
- `internal/handlers/admin.go` → `PostAdminEditUser`

**Тесты** (`internal/handlers/auth_test.go`):
- `TestPostRegister_EmailStoredLowercase` — email с заглавными буквами сохраняется в lowercase
- `TestPostRegister_DuplicateEmailCaseInsensitive` — `TAKEN@TEST.COM` отклоняется если `taken@test.com` уже есть
- `TestPostLogin_EmailCaseInsensitive` — вход с `USER@TEST.COM` работает если зарегистрирован `user@test.com`

---

## 10a. ✅ Реализовано (история задач)

| Задача | Когда |
|--------|-------|
| Тесты для `handlers/admin.go` — 12 тестов | 2026-03-26 |
| Тесты для `handlers/profile.go` — 8 тестов | 2026-03-26 |
| Тесты для `handlers/reset.go` — 10 тестов | 2026-03-26 |
| Тесты для `handlers/documents.go` — 9 тестов | 2026-03-26 |
| Тесты для `seed/defects.go` — 11 unit-тестов | 2026-03-26 |
| Rate limiting (5 попыток входа/15 мин, 3 регистрации/час) | 2026-03-27 |
| Политика паролей (≥6 + заглавная + цифра + спецсимвол) | 2026-03-27 |
| MIME-валидация файлов (аватар, план, фото) | 2026-03-27 |
| `crypto/rand` для reset-кодов | 2026-03-27 |
| Логирование безопасности: `[SECURITY] event=...` | 2026-03-27 |
| Пагинация списка осмотров — 20 записей/страницу | 2026-03-27 |
| **Асинхронная загрузка фото через Redis** (раздел 14) | 2026-03-27 |
| Исправлен баг: `SyncInspectionPhotos` с JOIN UPDATE в PostgreSQL | 2026-03-27 |
| **Блок A**: переименование папок Я.Диска ID→ActNumber + автомиграция | 2026-03-28 |
| **Блок B**: загрузка фото сразу при добавлении (не при генерации PDF) | 2026-03-28 |
| **Блок C**: архив удалённых дефектов с фото в view.html (серая плашка) | 2026-03-28 |
| **Блок D**: иконки ⏳/✗ у фото вместо общего прогресс-бара | 2026-03-28 |
| `yandex_test.go`: 6 тестов FolderExists/MoveFolder через httptest | 2026-03-28 |
| `photos_ensure_test.go`: 8 тестов EnsureFolder/buildUploadTask/buildDefectInfoMap | 2026-03-28 |

## 10b. ⚠️ TODO / Не реализовано

### Функционал
- [ ] Тесты для `internal/queue/redis.go` (требуют реального Redis)
- [ ] Тесты для `internal/worker/uploader.go` (требуют Redis + интеграции)
- [ ] Тесты для `pdf/generator.go`, `mailer/mailer.go`
- [ ] Исправить `recoverOnStartup без Redis` (фото в `uploading` не сбрасываются при рестарте без Redis)
- [ ] Экспорт в Excel / другие форматы
- [ ] Уведомления (push / Telegram)
- [ ] Мобильное приложение

### Технический долг
- [ ] Gin работает в `debug` mode (нужно `GIN_MODE=release` через env)
- [ ] `You trusted all proxies` — нужно установить `gin.SetTrustedProxies()`
- [ ] Логирование в `/var/log/inspection-app/app.log` может не работать без root на prod
- [ ] CSRF токены (SHOULD HAVE)
- [ ] Email-верификация при регистрации (ожидает SMTP-конфигурации)

---

## 🔐 11. Тестовые аккаунты

### Постоянный тестовый аккаунт

| Поле | Значение |
|------|----------|
| **Email** | `test@example.com` |
| **Пароль** | `Test1234!` |
| **ФИО** | Тестов Тест Тестович |
| **Роль** | `admin` |
| **Инициалы** | Тестов Т. Т. |

**Где создаётся:** `internal/seed/users.go` → `SeedTestUser()`
**Когда создаётся:** при каждом старте приложения (вызов в `cmd/server/main.go` после `seed.SeedDefects()`)
**Защита от дублирования:** перед созданием выполняется `WHERE email = ?` — если найден, пропускается
**Константы:** `seed.TestUserEmail`, `seed.TestUserPassword`

### Использование в тестах

⚠️ Интеграционные тесты (handlers) **не используют** этот аккаунт напрямую, потому что:
- каждый тест сбрасывает схему: `DROP SCHEMA public CASCADE`
- `newUser()` создаёт изолированных пользователей внутри каждого теста
- это обеспечивает полную изоляцию тестов

Тестовый аккаунт используется для:
- **ручного тестирования** в браузере (`http://localhost:8080`)
- **локальной отладки** без необходимости регистрироваться

### Другие пользователи в prod-БД

| Email | Роль | Назначение |
|-------|------|-----------|
| `legendverizon@gmail.com` | admin | Основной admin-аккаунт проекта |
| `devtest@example.com` | inspector | Тестовый inspector-аккаунт |

---

---

## 🚀 13. Производительность и загрузка файлов

### Проблема (была)
- `SyncInspectionPhotos` вызывалась синхронно в HTTP-запросе `/inspections/:id/generate`
- Для каждого из 20+ фото выполнялось последовательно ~9 HTTP-запросов к Яндекс.Диску
- 20 фото = 180 последовательных запросов → таймаут → 503
- `EnsurePath` повторно создавала папки для каждого фото (нет кэша)

### Что реализовано (`internal/handlers/photos.go`)

| Улучшение | Детали |
|-----------|--------|
| **Worker pool** | 5 горутин параллельно, `sem := make(chan struct{}, syncWorkers)` |
| **Кэш папок** | `createdFolders map[string]bool` + mutex — `EnsurePath` вызывается 1 раз на уникальную папку вместо N раз |
| **Retry с backoff** | 3 попытки, задержка `attempt * 2s` (2s, 4s) при ошибках Яндекс.Диска |
| **Чтение в память** | `os.ReadFile` → `bytes.NewReader` — позволяет retry без повторного открытия файла |
| **Лимит размера** | max 20 МБ на файл (`maxPhotoSize = 20*1024*1024`) |
| **Лимит количества** | max 30 фото на дефект (`maxPhotosPerDefect = 30`) |

### HTTP Server timeouts (`cmd/server/main.go`)

| Параметр | Значение | Причина |
|----------|----------|---------|
| `ReadTimeout` | 2 мин | Защита от медленных клиентов |
| `WriteTimeout` | 5 мин | Генерация PDF + синхронизация фото |
| `IdleTimeout` | 2 мин | Keep-alive соединения |

### Производительность
- 20 фото в 1 комнате: было 180 запросов → стало ~25 (5 EnsurePath + 20×2 upload/publish)
- 20 фото в 4 разных секциях: ~40 запросов параллельно вместо 180 последовательных

---

## 🚨 12. Важные замечания для AI

- **НЕ СОЗДАВАТЬ** файлы, если они уже существуют — проверять эту карту
- **НЕ ДУБЛИРОВАТЬ** логику: PDF — `internal/pdf/generator.go`, фото — `handlers/photos.go`, облако — `cloudstorage/yandex.go`
- **ПЕРЕД ИЗМЕНЕНИЕМ БД** — обновить `models.go` и учесть что AutoMigrate НЕ удаляет колонки
- **ТЕСТЫ** для handlers требуют отдельной БД `inspection_test` через `TEST_DATABASE_URL`
- **DefectTemplateID** — nullable (`*uint`), NULL означает запись "Прочее"
- **wall_number** — 0 = не стена, 1-4 = стены (дефекты стен хранятся как отдельные RoomDefect)
- **Первый пользователь** в системе автоматически получает роль `admin`
- **Синхронизация с Яндекс Диском** — асинхронно через Redis-очередь + воркер; fallback на sync если Redis недоступен
- **ОБНОВЛЯТЬ этот файл** после каждого изменения кода

### Правила для AI: работа с БД и паролями

- **НИКОГДА не менять пароли** от реальных (не тестовых) аккаунтов без явного разрешения пользователя
- Для отладки через curl или API — использовать только `test@example.com` / `Test1234!`
- Если нужна проверка поведения под залогиненным пользователем — добавить debug-лог в код, не трогать данные в БД

### Правила для AI: email

- **ВСЕГДА** нормализовать email перед сохранением/поиском: `strings.ToLower(strings.TrimSpace(email))`
- Уже реализовано в auth.go, reset.go, admin.go — не нарушать это правило при изменениях

### Правила для AI: тестовые аккаунты

- **НЕ создавать** новые тестовые аккаунты — использовать `test@example.com`
- **ВСЕГДА** проверять существование по email перед созданием
- Если тест падает → исправлять код, а не создавать новый аккаунт
- Если нужен аккаунт для ручного теста → `test@example.com` / `Test1234!`
- Интеграционные тесты используют `newUser()` с изолированной БД — это правильно, не менять

---

## 📋 14. Асинхронная загрузка фото через Redis

> **Статус:** ✅ РЕАЛИЗОВАНО — 2026-03-27
> **Исправлен баг:** `SyncInspectionPhotos` / `retryFailed` — JOIN+UPDATE в PostgreSQL через подзапрос

---

### Контекст и проблема

Сейчас при нажатии «Сгенерировать PDF»:
1. Вызывается `SyncInspectionPhotos` — загружает ВСЕ фото на Яндекс.Диск синхронно
2. Только после завершения всех загрузок генерируется PDF
3. При 20+ фото и медленном Яндекс.Диске → HTTP timeout → 503

Текущие улучшения (worker pool, retry) снизили нагрузку, но не устранили корень: загрузка **блокирует HTTP-запрос**. При 100+ пользователях это не масштабируется.

---

### Ключевое архитектурное решение

Разделить на два независимых потока:

```
[СИНХРОННО, ~2-3 сек]          [АСИНХРОННО, фоновый воркер]
─────────────────────          ────────────────────────────
POST /generate                 Redis Queue → Worker
  ↓                              ↓
EnsureInspectionFolder()       UploadInspectionPhotos()
  • создать папку на Я.Диске     • загружать фото пачками
  • опубликовать → URL           • обновлять photo.upload_status
  ↓                              • pending → uploading → done/failed
pdf.Generate()
  • QR-код уже есть (URL папки)
  ↓
queue.Push(inspectionID)
  • пользователь получает PDF
```

**Почему это работает:** при создании папки на Яндекс.Диске публичная ссылка выдаётся сразу, до загрузки фото. QR-код указывает на папку — она уже доступна. Фото появляются в папке по мере загрузки воркером.

---

### Новые компоненты

#### 1. `internal/queue/redis.go` — очередь задач

```
Тип очереди: Redis List (RPUSH / BLPOP)
Ключ: "inspection_app:upload_jobs"
Значение: JSON {"inspection_id": 42, "enqueued_at": "2026-03-27T10:00:00Z"}

Интерфейс:
  Push(ctx, inspectionID uint) error
  Pop(ctx) (inspectionID uint, error)
  Len(ctx) (int64, error)
```

Библиотека: `github.com/redis/go-redis/v9`

#### 2. `internal/worker/uploader.go` — фоновый воркер

```
Start(ctx context.Context, n int)
  • запускает n горутин (рекомендуется 3-5)
  • каждая горутина: BLPOP → UploadInspectionPhotos → обновить статус
  • graceful shutdown через context

processJob(ctx, inspectionID)
  • проверить что есть фото с upload_status = "pending"
  • обновить upload_status = "uploading"
  • вызвать UploadInspectionPhotos (без изменения интерфейса)
  • обновить upload_status = "done" / "failed"
```

#### 3. Новое поле в модели Photo

```go
// в models.go добавить в struct Photo:
UploadStatus string `gorm:"default:done"` // "pending" | "uploading" | "done" | "failed"
```

Значение по умолчанию `done` — обратная совместимость со старыми записями.
При сохранении нового фото в `PostUploadPhoto` устанавливать `"pending"`.

#### 4. Разбить `SyncInspectionPhotos` на две функции

```
EnsureInspectionFolder(inspectionID uint) error
  • создаёт папку inspections/{id}/ на Яндекс.Диске
  • публикует её → получает photo_folder_url
  • сохраняет photo_folder_url в inspections таблице
  • БЫСТРО: 2-3 HTTP запроса к Яндекс.Диску
  • вызывается синхронно перед pdf.Generate()

UploadInspectionPhotos(inspectionID uint)
  • загружает все фото с upload_status = "pending"
  • параллельно (worker pool 5 горутин, уже реализован)
  • обновляет photo.upload_status по результату
  • вызывается только из воркера
```

#### 5. Новый HTTP-эндпоинт статуса

```
GET /inspections/:id/upload-status
Доступ: RequireAuth (тот же пользователь или admin)

Ответ JSON:
{
  "total":     30,
  "pending":   25,
  "uploading":  2,
  "done":       3,
  "failed":     0,
  "all_done":  false
}
```

#### 6. Индикатор прогресса на фронтенде

В `web/templates/inspections/view.html`:

```
Блок "Фото в облаке":
  [████░░░░░░░░░░] 3 из 30 загружено

JS: setInterval(pollUploadStatus, 3000)
  • если all_done = true → остановить опрос, показать "✓ Все фото загружены"
  • если failed > 0 → показать кнопку "Повторить загрузку"
```

---

### Изменения в существующих файлах

| Файл | Изменение |
|------|-----------|
| `internal/models/models.go` | + поле `UploadStatus string` в `Photo` |
| `internal/handlers/photos.go` | при `PostUploadPhoto` ставить `upload_status = "pending"` |
| `internal/handlers/photos.go` | выделить `EnsureInspectionFolder()` как отдельную функцию |
| `internal/handlers/documents.go` | убрать `SyncInspectionPhotos`, добавить `EnsureInspectionFolder` + `queue.Push` |
| `internal/handlers/inspections.go` | + обработчик `GET /inspections/:id/upload-status` |
| `cmd/server/main.go` | + подключение Redis, + `worker.Start(ctx, 5)`, + graceful shutdown |
| `docker-compose.yml` | + сервис Redis (image: `redis:7-alpine`) |
| `.env.example` | + `REDIS_URL=redis://localhost:6379` |
| `go.mod` | + `github.com/redis/go-redis/v9` |

---

### Восстановление после рестарта

При старте сервера в `main.go`:
```
1. Найти все Photo с upload_status = "uploading"
   → переставить в "pending" (воркер умер на полпути)

2. Найти все Inspection, у которых есть Photo с upload_status = "pending"
   → положить в Redis queue (задачи потерялись из памяти при рестарте)
```

---

### Защита от дублирования задач (идемпотентность)

- Перед постановкой в очередь: проверять `SELECT COUNT(*) FROM photos WHERE inspection_id=? AND upload_status='pending' > 0`
- Если 0 — не ставить задачу (все уже загружены)
- `UploadInspectionPhotos` обрабатывает только `WHERE upload_status = 'pending'` — повторный запуск безопасен

---

### Поведение при недоступности Яндекс.Диска

```
Попытка загрузки → ошибка
→ upload_status = "failed"
→ через 60 сек: requeuer-горутина
    SELECT inspections где есть photos с status="failed"
    → переставить в "pending", положить в очередь снова
→ максимум 5 попыток (после этого оставить "failed", не ретраить)
```

---

### Поведение при недоступности Redis

```
queue.Push() вернул ошибку
→ НЕ падать
→ лог: "Redis недоступен, синхронизируем синхронно (fallback)"
→ вызвать SyncInspectionPhotos синхронно (как сейчас)
→ алерт/метрика для мониторинга
```

---

### Инфраструктура: Redis в docker-compose

```yaml
# Добавить в docker-compose.yml:
redis:
  image: redis:7-alpine
  restart: unless-stopped
  ports:
    - "6379:6379"
  volumes:
    - redis_data:/data
  command: redis-server --appendonly yes  # persistence включена

volumes:
  redis_data:
```

`--appendonly yes` — Redis сохраняет очередь на диск. При рестарте контейнера задачи не теряются.

---

### Переменные окружения

```bash
# .env (добавить):
REDIS_URL=redis://localhost:6379

# В docker-compose (app сервис):
REDIS_URL=redis://redis:6379
```

---

### Тесты, которые нужно написать

```
internal/queue/redis_test.go
  • TestPush_Pop
  • TestPop_BlocksUntilItem
  • TestLen

internal/worker/uploader_test.go
  • TestWorker_ProcessesJob (mock queue + mock cloud)
  • TestWorker_GracefulShutdown
  • TestWorker_RetriesOnFailure
  • TestWorker_SkipsIfNoPhotos

internal/handlers/inspections_test.go (добавить)
  • TestGetUploadStatus_AllDone
  • TestGetUploadStatus_Pending
  • TestGetUploadStatus_Mixed

internal/handlers/photos_load_test.go (расширить)
  • TestEnsureInspectionFolder_CreatesAndPublishes
  • TestUploadInspectionPhotos_UpdatesStatus
```

---

### Реализованные шаги

```
✅ Шаг 1: Модель + миграция
  • UploadStatus в Photo (default: 'done', NOT NULL)
  • PostUploadPhoto устанавливает upload_status = "pending"

✅ Шаг 2: docker-compose + Redis
  • сервис redis:7-alpine с appendonly yes
  • REDIS_URL в .env.example и docker-compose app service

✅ Шаг 3: internal/queue/redis.go
  • Push(ctx, inspectionID), Pop(ctx), Len(ctx), Close()
  • NewFromEnv(): nil,nil если REDIS_URL не задан

✅ Шаг 4: Разбить SyncInspectionPhotos
  • EnsureInspectionFolder(id) (sync, 2-3 HTTP к Я.Диску)
  • UploadInspectionPhotos(id) (async, только pending-фото)
  • SyncInspectionPhotos — fallback: сбрасывает done→pending если file_path != ''

✅ Шаг 5: internal/worker/uploader.go
  • Start(ctx, 5), processJob, graceful shutdown через done chan
  • recoverOnStartup: uploading→pending, requeue при старте
  • requeueFailed: тикер 60с, failed→pending+requeue

✅ Шаг 6: documents.go
  • async path: EnsureInspectionFolder + queue.Push
  • fallback при Push error: SyncInspectionPhotos

✅ Шаг 7: GET /inspections/:id/upload-status → JSON {total,pending,uploading,done,failed,all_done}

✅ Шаг 8: Фронтенд прогресс-бар в view.html
  • setInterval(pollUploadStatus, 3000)
  • progress bar, ✓ когда all_done, кнопка "Повторить" при failed

✅ Шаг 9: Graceful shutdown в main.go
  • signal.NotifyContext + http.Server в горутине
  • <-ctx.Done() → srv.Shutdown(30s) + worker.Stop()

✅ Шаг 10 (баги): исправлен JOIN+UPDATE в PostgreSQL
  • SyncInspectionPhotos: подзапрос SELECT ids → UPDATE WHERE id IN (...)
  • worker.retryFailed: то же исправление
```

Шаг 10: Нагрузочные тесты
  • обновить photos_load_test.go
```

---

### Ожидаемый результат после реализации

| Метрика | Сейчас | После |
|---------|--------|-------|
| Время ответа на "Сгенерировать PDF" | 30-120 сек (блок) | 3-5 сек |
| PDF с QR-кодом | ✅ | ✅ |
| Фото в облаке сразу | ✅ (если не timeout) | ⏳ (загружаются в фоне) |
| 503 при 20+ фото | ❌ случается | ✅ устранено |
| Масштабирование до 100+ юзеров | ❌ | ✅ |
| Выживание при рестарте сервера | ❌ задачи теряются | ✅ Redis + БД |
| Видимость прогресса для пользователя | ❌ | ✅ прогресс-бар |

---

## 15. 🔐 Security (реализовано 2026-03-27)

### Что сделано

| Защита | Файл | Детали |
|--------|------|--------|
| `crypto/rand` для reset-кода | `handlers/reset.go` | Заменён `math/rand` → криптографически безопасный |
| Политика пароля | `security/validator.go` | ≥6 символов + заглавная + цифра + спецсимвол |
| Rate limit: вход | `security/ratelimit.go` + `main.go` | 5 неудачных попыток / 15 мин с IP; счётчик сбрасывается при успешном входе |
| Rate limit: регистрация | `security/ratelimit.go` + `main.go` | 3 попытки / час с IP; инкрементируется только при успехе |
| Rate limit: сброс пароля | `security/ratelimit.go` + `main.go` | 3 попытки / час с IP |
| Rate limit: создание актов | `security/ratelimit.go` + `inspections.go` | 20 актов / час на пользователя; admins без ограничений |
| MIME-валидация аватара | `handlers/profile.go` | расширение + реальный MIME (DetectContentType) + лимит 5 МБ |
| MIME-валидация плана | `handlers/inspections.go` | расширение + реальный MIME (DetectContentType) + лимит 20 МБ |
| Security logging | `security/logger.go` | Каждое событие → `[SECURITY] event=... ip=...` в stdout/app.log |

### Новые файлы

```
internal/security/
  ratelimit.go   — MemoryRateLimiter (sliding window), глобальные лимитеры, gin middleware
  logger.go      — Log(event, ip, detail) + константы событий
  validator.go   — ValidatePassword(), ValidateImage()
```

### Архитектура rate limiter

- **Хранилище**: in-memory (`sync.Mutex` + `map[string]*limitWindow`)
- **Стратегия**: скользящее окно (sliding window)
- **Ключ**: IP-адрес для auth-маршрутов, `insp:{userID}` для актов
- **Cleanup**: фоновая горутина каждые 10 мин удаляет истёкшие окна
- **Интерфейс готов к замене на Redis**: достаточно реализовать те же методы поверх Redis (Section 14)
- **Admin bypass**: admins обходят лимит создания актов; для /login и /register bypass невозможен (пользователь ещё не аутентифицирован)

### Подробные сообщения об ошибках

При блокировке пользователь видит:
- Что произошло (превышен лимит попыток)
- Почему (защита от взлома/спама)
- Что делать (подождать + корректные данные)
- Сколько минут до разблокировки

### Что осталось (SHOULD HAVE / NICE TO HAVE)

| Задача | Зависимость | Приоритет |
|--------|-------------|-----------|
| Email verification при регистрации | Настроить mailer (SMTP) | SHOULD HAVE |
| Redis-backed rate limiter | Section 14 (Redis) | SHOULD HAVE |
| CSRF-токены в формах | — | SHOULD HAVE |
| JWT invalidation при logout (blacklist) | Redis | NICE TO HAVE |
| Лимит смены пароля (3 раза в сутки) | — | NICE TO HAVE |
| Настройка mailer для email verification | SMTP-провайдер | SHOULD HAVE |

### Email verification — план (не реализовано, ждёт mailer)

Когда mailer будет настроен:
1. Добавить поля в `User`: `IsVerified bool`, `VerificationToken string`, `VerificationExpiry *time.Time`
2. При регистрации: `IsVerified = false`, генерация токена, отправка письма
3. Новые пользователи видят баннер "Подтвердите email" до верификации, но не блокируются
4. Существующие пользователи (до внедрения) считаются верифицированными (`IsVerified = true` при миграции)
5. Endpoint `GET /verify-email?token=...` — обновляет `IsVerified = true`

---

## 16. 🎨 UI: стилизация "Общие замечания по квартире" (2026-03-27)

### Что сделано

Блок "Общие замечания по квартире" (Электричество, Вентиляция, Общие замечания) приведён к единой стилистике остальных секций приложения.

| Страница | До | После |
|----------|----|-------|
| `view.html` | `<div class="form-card">` с `<h2>` | `<div class="room-block">` с синей плашкой `room-header → room-title → room-label` |
| `edit.html` | Два отдельных `form-card` (поля + кнопка "Сохранить") | Один `room-block`: синяя плашка + поля с `padding:12px 16px` + кнопка "Сохранить" справа внутри блока |

### Детали реализации

**view.html** — поля отображаются как `defect-row` внутри `defects-section`:
- Электричество → `defect-name` / `defect-value`
- Вентиляция → `defect-name` / `defect-value`
- Общие замечания → отдельный `<p style="white-space:pre-wrap">` с заголовком `defect-name`

**edit.html** — поля формы обёрнуты в `<div style="padding:12px 16px">` внутри `room-block`; кнопка "Сохранить" выровнена по правому краю через `display:flex; justify-content:flex-end`.

---

## ✅ 17. РЕФАКТОРИНГ СИСТЕМЫ ФОТО (реализован 2026-03-28, коммит `170b6a0`)

> Все 4 блока реализованы, протестированы (ручные тесты + unit тесты), задеплоены на GitHub.

---

### Контекст: исправления и фоновые баги

**Баг buildDefectInfoMap** (`internal/handlers/photos.go`): ✅ исправлен ранее
- `PostEditInspection` soft-delete-ит старые room_defects → GORM добавлял `deleted_at IS NULL` → пустой infoMap → 0 задач → фото навсегда в `uploading`
- Фикс: `.Unscoped()` к обоим запросам в `buildDefectInfoMap`

**Баг recoverOnStartup без Redis**: ⚠️ не исправлен
- `recoverOnStartup` вызывается только если Redis доступен
- Без Redis фото в `uploading` никогда не сбрасываются в `pending`
- Обход: ручной SQL `UPDATE photos SET upload_status='pending' WHERE upload_status='uploading'`

---

### Блок A — Переименование папок Яндекс Диска (ID → ActNumber) ✅ РЕАЛИЗОВАН

**Было:** папки назывались `inspections/27`, **стало:** `inspections/18-280326`

**Файлы которые ТРОГАТЬ:**

`internal/cloudstorage/storage.go`:
- Добавить метод в интерфейс `FileStorage`:
  ```go
  MoveFolder(oldRelPath, newRelPath string) error
  ```

`internal/cloudstorage/yandex.go`:
- Реализовать `MoveFolder` через Яндекс Диск API:
  ```
  POST https://cloud-api.yandex.net/v1/disk/resources/move?from={oldFullPath}&path={newFullPath}&overwrite=false
  ```
- HTTP клиент с таймаутом уже есть (`y.client`), использовать его

`internal/handlers/photos.go` — функция `EnsureInspectionFolder`:
- Сейчас: создаёт папку `inspections/{inspectionID}`
- Было: `inspFolder := fmt.Sprintf("inspections/%d", inspectionID)`
- Стало: `inspFolder := fmt.Sprintf("inspections/%s", inspection.ActNumber)`
- Добавить автомиграцию ПЕРЕД созданием новой папки:
  ```
  Если exists("inspections/{ID}") И NOT exists("inspections/{ActNumber}")
    → MoveFolder("inspections/{ID}", "inspections/{ActNumber}")
  Иначе если exists("inspections/{ActNumber}")
    → ок, ничего не делать
  Иначе
    → создать новую папку
  ```
- Для проверки существования папки: GET `/resources?path=...` → 200 = exists, 404 = not exists
- Добавить вспомогательный метод `FolderExists(relPath string) bool` в yandex.go

`internal/handlers/photos.go` — функция `buildUploadTask`:
- Сейчас путь: `inspections/{inspectionID}/{RoomName}/{Section}/`
- Стало: `inspections/{actNumber}/{RoomName}/{Section}/`
- Нужно передавать actNumber в эту функцию. Изменить сигнатуру или добавить в `defectInfo`:
  ```go
  type defectInfo struct {
      ...
      ActNumber string  // добавить
  }
  ```
- В `buildDefectInfoMap` добавить запрос inspection по ID чтобы получить ActNumber

**Файлы которые НЕ ТРОГАТЬ:**
- `internal/models/models.go` — модели не меняются
- `internal/worker/uploader.go` — логика воркера не меняется
- `internal/queue/redis.go` — очередь не меняется
- `internal/pdf/generator.go` — PDF не меняется
- `internal/handlers/documents.go` — пока не трогать (изменится в Блоке B)

---

### Блок B — Загрузка фото сразу при добавлении ✅ РЕАЛИЗОВАН

**Было:** фото висели как `pending` до нажатия "Сформировать акт"

**Новая логика:**
```
Пользователь нажимает 📷 → фото сохраняется локально → сразу push в Redis → воркер грузит на диск
```

**Файлы которые ТРОГАТЬ:**

`internal/handlers/photos.go` — функция `PostUploadPhoto`:
- В конце, после `storage.DB.Create(&photo)`, добавить:
  ```go
  // Сразу ставим в очередь на загрузку
  if uploadQueue != nil {
      uploadQueue.Push(context.Background(), inspection.ID)
  } else if cloudStore != nil {
      go SyncInspectionPhotos(inspection.ID)  // горутина, не блокируем ответ
  }
  ```
- `uploadQueue` уже объявлен в `documents.go` — нужно сделать его доступным из `photos.go`
  ИЛИ переместить `uploadQueue` в отдельный пакет/файл `handlers/state.go`

`internal/handlers/documents.go` — функция `PostGenerateDocument`:
- УБРАТЬ блок загрузки фото (строки с `uploadQueue.Push` и `SyncInspectionPhotos`)
- Оставить только:
  1. `EnsureInspectionFolder` (папка нужна для QR-кода в PDF)
  2. Перечитать inspection с актуальным `PhotoFolderURL`
  3. `pdf.Generate()`
- Результат: PDF генерируется мгновенно, без ожидания загрузки фото

`internal/handlers/inspections.go` — функция `PostEditInspection`:
- В конце (после сохранения всех дефектов) добавить вызов:
  ```go
  if cloudStore != nil {
      go handlers.EnsureInspectionFolder(inspection.ID)
  }
  ```
- Это создаёт папку на Яндекс Диске при первом сохранении и записывает `PhotoFolderURL`
- QR-код в PDF становится доступен сразу после первого сохранения

**Важно:** `EnsureInspectionFolder` уже идемпотентная (проверяет `inspection.PhotoFolderURL != ""`), повторные вызовы безвредны

**Файлы которые НЕ ТРОГАТЬ:**
- `internal/worker/uploader.go` — воркер не меняется, логика та же
- `internal/models/models.go` — не меняется
- `internal/pdf/generator.go` — не меняется

---

### Блок C — Архив удалённых дефектов в view.html ✅ РЕАЛИЗОВАН

**Поведение:**
- После редактирования акта старые room_defects soft-delete-ятся
- Если у них есть фото (любой статус) — показывать отдельный блок внизу view.html
- Блок называется "Архив удалённых дефектов"
- Только просмотр, кнопки 📷 нет
- В PDF НЕ попадают
- Загрузка фото продолжается даже для soft-deleted дефектов (buildDefectInfoMap уже Unscoped)
- **Плашка заголовка — серая**, НЕ синяя. Синяя плашка используется только для помещений у которых есть дефекты в PDF. Архив — это информация только для просмотра на сайте, в PDF не включается, поэтому визуально отличается: `background: linear-gradient(135deg, #94a3b8, #64748b)` вместо синего градиента помещений

**Файлы которые ТРОГАТЬ:**

`cmd/server/main.go` — добавить template function:
```go
"deletedDefectsWithPhotos": func(inspectionID uint) []models.RoomDefect {
    var defects []models.RoomDefect
    storage.DB.Unscoped().
        Preload("Photos").
        Preload("DefectTemplate").
        Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
        Where("inspection_rooms.inspection_id = ? AND room_defects.deleted_at IS NOT NULL", inspectionID).
        Having("COUNT(photos.id) > 0").  // только если есть фото
        Find(&defects)
    return defects
}
```
Либо передавать эти данные через GetInspection handler — тогда добавить поле в шаблонный контекст.

`internal/handlers/inspections.go` — функция `GetInspection`:
- Добавить в контекст шаблона `deletedDefects []models.RoomDefect`:
  ```go
  var deletedDefects []models.RoomDefect
  storage.DB.Unscoped().
      Preload("Photos").
      Preload("DefectTemplate").
      Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
      Joins("LEFT JOIN photos ON photos.defect_id = room_defects.id AND photos.deleted_at IS NULL").
      Where("inspection_rooms.inspection_id = ? AND room_defects.deleted_at IS NOT NULL", inspection.ID).
      Group("room_defects.id").
      Having("COUNT(photos.id) > 0").
      Find(&deletedDefects)
  ```
- Добавить в `gin.H{}` → `"deletedDefects": deletedDefects`

`web/templates/inspections/view.html`:
- Добавить блок ПОСЛЕ секций помещений, ПЕРЕД блоком документов:
  ```html
  {{ if .deletedDefects }}
  <div class="room-block" style="border-color:#e2e8f0">
    <div class="room-header" style="background:linear-gradient(135deg,#94a3b8,#64748b)">
      <div class="room-title">
        <span class="room-label">Архив удалённых дефектов</span>
      </div>
    </div>
    <div style="padding:12px 16px">
      {{ range .deletedDefects }}
      <div style="background:#f8fafc;border:1px solid #e2e8f0;border-radius:8px;padding:12px;margin-bottom:8px;opacity:0.8">
        <div style="display:flex;align-items:center;gap:8px;margin-bottom:8px">
          <span style="background:#94a3b8;color:#fff;font-size:11px;padding:2px 8px;border-radius:4px;font-weight:600">УДАЛЁН</span>
          <span style="font-weight:500;color:#475569">{{ дефект название }}</span>
          <span style="color:#94a3b8;font-size:13px">— {{ комната }} · {{ секция }}</span>
        </div>
        <!-- миниатюры фото, только просмотр, без кнопки добавить -->
        {{ range .Photos }}
          <img src="{{ .FileURL }}" style="width:80px;height:60px;object-fit:cover;border-radius:4px;margin-right:4px">
        {{ end }}
        <!-- статус загрузки -->
        {{ if not all done }} <span>⏳ загружается на диск</span> {{ end }}
      </div>
      {{ end }}
    </div>
  </div>
  {{ end }}
  ```

**Файлы которые НЕ ТРОГАТЬ:**
- `internal/pdf/generator.go` — PDF не должен показывать архив
- `internal/models/models.go` — поле `deleted_at` уже есть (GORM soft delete)
- `web/templates/inspections/edit.html` — edit не меняется

---

### Блок D — Статус фото у миниатюр ✅ РЕАЛИЗОВАН

**Цель:** убрать большой прогресс-бар внизу view.html, показывать ⏳/✓/✗ у каждого фото

**Файлы которые ТРОГАТЬ:**

`web/templates/inspections/view.html`:
- Убрать блок `<div id="upload-status">` с прогресс-баром (строки ~206-216)
- Убрать JS функции `pollUploadStatus`, `startPoll`, `retryUpload`, `stopPoll`
- У каждой миниатюры фото добавить статусный бейдж на основе `upload_status` из данных шаблона:
  - `pending` или `uploading` → ⏳ маленькая иконка поверх фото
  - `done` → ничего (фото чистое)
  - `failed` → ✗ красная иконка + кнопка "повторить" (POST на новый endpoint или через Redis)
- Статус известен в момент рендера (Photo.UploadStatus передаётся через Preload)
- Для реального времени (pending → done) оставить лёгкий polling только если есть фото не в `done`

**Файлы которые НЕ ТРОГАТЬ:**
- `internal/handlers/inspections.go` → `GetUploadStatus` — оставить как есть (нужен для polling)
- Маршрут `GET /inspections/:id/upload-status` — оставить

---

### Итоги реализации и ручного тестирования (2026-03-28)

```
✅ Блок A: папка создана как inspections/TEST-32 (не inspections/32)
   photo_folder_url сохранён в БД, Яндекс Диск отвечает 201

✅ Блок B: POST /defects/237/photos → статус pending → uploading → done за ~7 сек
   Загрузка происходит без нажатия "Сформировать акт"

✅ Блок C: редактирование без дефекта → дефект soft-deleted → view.html
   показывает блок "Архив удалённых дефектов" с фото и ссылкой на Яндекс Диск

✅ Блок D: прогресс-бар удалён, CSS ::after иконки присутствуют,
   upload-notice и startUploadPoll работают
```

---

### Что КАТЕГОРИЧЕСКИ НЕ ТРОГАТЬ

| Файл/компонент | Причина |
|----------------|---------|
| `internal/auth/` | Полностью рабочий, покрыт тестами |
| `internal/models/models.go` | Поля не меняются (только читаем) |
| `internal/pdf/generator.go` | PDF не меняется |
| `internal/worker/uploader.go` | Воркер не меняется |
| `internal/queue/redis.go` | Очередь не меняется |
| `internal/seed/` | Шаблоны дефектов не меняются |
| `internal/mailer/` | SMTP не трогаем |
| `web/static/css/style.css` | CSS классы room-block и т.д. уже корректны |
| `web/templates/inspections/edit.html` | Форма не меняется (кроме вызова EnsureFolder) |
| Все тесты (`*_test.go`) | Не ломать существующие тесты |

---

### Известные риски

1. **MoveFolder на Яндекс Диске** — операция не атомарная: если переименование прервётся, папка может существовать с обоими именами или ни с одним. Решение: проверять exists после move.

2. **Параллельные push в Redis** — если пользователь добавляет 10 фото быстро, в очередь попадёт 10 задач для одного inspectionID. Воркер обработает дубли за счёт проверки `count pending == 0 → skip`. Это безопасно.

3. **EnsureFolder при каждом сохранении** — вызов идемпотентный (проверяет `PhotoFolderURL != ""`), но при первом вызове делает 2-3 HTTP запроса к Яндекс Диску. Запускать через `go` горутину, не блокировать HTTP ответ.

4. **Архив удалённых дефектов** — JOIN + GROUP BY + HAVING в GORM может быть неочевидным. Если сложно — использовать raw SQL через `storage.DB.Raw(...)`.

5. **recoverOnStartup без Redis** — всё ещё не исправлен. При рестарте сервера без Redis фото в `uploading` не сбросятся. Временное решение: добавить в `cmd/server/main.go` при старте:
   ```go
   // Сбрасываем застрявшие uploading независимо от Redis
   storage.DB.Model(&models.Photo{}).
       Where("upload_status = 'uploading'").
       Update("upload_status", "pending")
   ```
   Это безопасно — при рестарте всегда сбрасывать.
