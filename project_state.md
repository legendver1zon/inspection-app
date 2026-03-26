# 📋 PROJECT STATE — inspection-app

> **Единый источник правды о проекте. Обновлять после каждого изменения кода.**
> Последнее обновление: 2026-03-26 (пагинация + email case-insensitive)

---

## 1. 📌 Общая информация

| Поле | Значение |
|------|----------|
| **Название** | inspection-app |
| **Назначение** | Веб-приложение для составления актов осмотра квартир: ввод дефектов, фото, генерация PDF |
| **Стек** | Go 1.25, Gin, GORM, PostgreSQL 16, FPDF, Яндекс Диск REST API |
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
├── docker-compose.yml              # PostgreSQL 16 + приложение
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
│   │   ├── storage.go              # Интерфейс FileStorage (EnsurePath, UploadFile, PublishFolder, PublishFile)
│   │   └── yandex.go               # Реализация Яндекс Диска через REST API
│   │
│   ├── handlers/
│   │   ├── auth.go                 # GetLogin, PostLogin, GetRegister, PostRegister, PostLogout
│   │   ├── auth_test.go            # Тесты auth handlers (12 тестов, требуют TEST_DATABASE_URL)
│   │   ├── inspections.go          # GetInspections, GetNewInspection, GetInspection, GetEditInspection, PostEditInspection, PostUploadPlan, PostDeleteInspection
│   │   ├── inspections_test.go     # Тесты инспекций (30 тестов, требуют TEST_DATABASE_URL)
│   │   ├── documents.go            # PostGenerateDocument, GetDownloadDocument, PostDeleteDocument
│   │   ├── photos.go               # PostUploadPhoto, DeletePhoto, SyncInspectionPhotos
│   │   ├── photos_test.go          # Тесты фото (13 тестов, требуют TEST_DATABASE_URL)
│   │   ├── profile.go              # GetProfile, PostProfile, PostUploadAvatar
│   │   ├── admin.go                # GetAdminUsers, PostAdminChangeRole, GetAdminEditUser, PostAdminEditUser, DeleteAdminUser
│   │   ├── reset.go                # GetForgotPassword, PostForgotPassword, GetResetPassword, PostResetPassword
│   │   └── setup_test.go           # Test utilities и fixtures
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
- Регистрация: валидация ФИО (2-3 слова), min 6 символов пароля, уникальность email
- Первый пользователь → `admin`, остальные → `inspector`
- Автогенерация инициалов: `"Иванов Иван Иванович"` → `"Иванов И. И."`
- JWT в httpOnly cookie (24 часа) (`/internal/auth/auth.go`)
- Middleware: `RequireAuth`, `RequireAdmin` (`/internal/auth/middleware.go`)

### Восстановление пароля
- 6-значный код по SMTP-email (`/internal/handlers/reset.go`)
- Срок действия кода: 15 минут
- SMTP: STARTTLS (587) и implicit TLS (465) (`/internal/mailer/mailer.go`)

### Профиль
- Редактирование ФИО, инициалов (`/internal/handlers/profile.go`)
- Смена пароля (с проверкой текущего)
- Загрузка аватара

### Осмотры (Акты)
- CRUD с поддержкой черновиков (`draft`) и завершённых (`completed`) (`/internal/handlers/inspections.go`)
- До 10 помещений на осмотр
- Для каждого помещения:
  - Замеры: длина/ширина/высота, до 5 окон (высота/ширина), дверь
  - Тип окон: ПВХ / Алюминий / Дерево
  - Тип стен: Окраска / Плитка / ГКЛ (множественный выбор)
  - 6 секций дефектов: окна, потолок, стены (4 стены отдельно), пол, двери, сантехника
  - 48 преднастроенных шаблонов дефектов с порогами и единицами (`/internal/seed/defects.go`)
  - Поле "Прочие" для любой секции

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

### Фотографии
- Загрузка к каждому дефекту: jpg, jpeg, png, webp (`/internal/handlers/photos.go`)
- Локальное хранение: `web/static/uploads/photos/{inspectionID}/{defectID}/`
- Синхронизация с Яндекс Диском перед генерацией PDF
- Иерархия в облаке: `inspections/{id}/{RoomName}/{Section}/{DefectName}/photo_{n}.jpg`
- Удаление с проверкой прав (инспектор — только свои, admin — любые)

### PDF генерация
- FPDF с поддержкой кириллицы, go:embed шрифты (`/internal/pdf/generator.go`)
- Страница 1: Шапка акта + план помещений + таблица замеров + подписи
- Следующие страницы: Дефекты по помещениям
- QR-код со ссылкой на Яндекс Диск (если есть фото)
- Сохранение в `web/static/documents/`

### Облачное хранилище (Яндекс Диск)
- REST API (OAuth token) (`/internal/cloudstorage/yandex.go`)
- Опционально: без `YADISK_TOKEN` фото хранятся только локально
- Автосинхронизация при генерации PDF
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
| `photos` | id, defect_id (FK, INDEX), file_url, file_path, file_name |
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
  ├── storage.ConnectFromEnv()   → internal/storage/db.go    → PostgreSQL
  ├── storage.Migrate()          → AutoMigrate всех моделей
  ├── seed.SeedDefectTemplates() → internal/seed/defects.go  → 48 записей в defect_templates
  ├── cloudstorage.NewYandexDisk() → internal/cloudstorage/yandex.go
  └── Gin Router
        ├── RequireAuth          → internal/auth/middleware.go → internal/auth/auth.go (JWT)
        ├── RequireAdmin         → internal/auth/middleware.go
        ├── handlers/auth.go     → models + auth.go
        ├── handlers/inspections.go → models + storage.DB
        ├── handlers/photos.go   → models + storage.DB + cloudstorage
        ├── handlers/documents.go → models + storage.DB + pdf.Generate() + SyncPhotos
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
internal/handlers/setup_test.go         # Общие fixtures, helpers, роутер для тестов
internal/handlers/auth_test.go          # 13 интеграционных, требуют TEST_DATABASE_URL
internal/handlers/admin_test.go         # 12 интеграционных, требуют TEST_DATABASE_URL
internal/handlers/profile_test.go       # 8 интеграционных, требуют TEST_DATABASE_URL
internal/handlers/reset_test.go         # 10 интеграционных, требуют TEST_DATABASE_URL
internal/handlers/documents_test.go     # 9 интеграционных, требуют TEST_DATABASE_URL
internal/handlers/inspections_test.go   # 30 интеграционных, требуют TEST_DATABASE_URL
internal/handlers/photos_test.go        # 13 интеграционных, требуют TEST_DATABASE_URL
```

### Покрытие
| Пакет | Тестов | Тип | Статус |
|-------|--------|-----|--------|
| `internal/auth` | 13 | Unit (без БД) | ✅ Всегда запускаются |
| `internal/seed` | 11 | Unit (без БД) | ✅ Всегда запускаются |
| `internal/handlers` (auth) | 13 | Интеграционные | ⚠️ Требуют `TEST_DATABASE_URL` |
| `internal/handlers` (admin) | 12 | Интеграционные | ⚠️ Требуют `TEST_DATABASE_URL` |
| `internal/handlers` (profile) | 8 | Интеграционные | ⚠️ Требуют `TEST_DATABASE_URL` |
| `internal/handlers` (reset) | 10 | Интеграционные | ⚠️ Требуют `TEST_DATABASE_URL` |
| `internal/handlers` (documents) | 9 | Интеграционные | ⚠️ Требуют `TEST_DATABASE_URL` |
| `internal/handlers` (inspections) | 30 | Интеграционные | ⚠️ Требуют `TEST_DATABASE_URL` |
| `internal/handlers` (photos) | 13 | Интеграционные | ⚠️ Требуют `TEST_DATABASE_URL` |
| **Итого** | **119** | | |

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
| `GIN_MODE` | ❌ | `release` для production |

---

## 9. 🕓 История изменений (Git)

```
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

## 10a. ⚠️ TODO / Не реализовано

### Функционал
- [x] Тесты для `handlers/admin.go` — 12 тестов ✅
- [x] Тесты для `handlers/profile.go` — 8 тестов ✅
- [x] Тесты для `handlers/reset.go` — 10 тестов ✅
- [x] Тесты для `handlers/documents.go` — 9 тестов (без PostGenerateDocument) ✅
- [x] Тесты для `seed/defects.go` — 11 unit-тестов ✅
- [ ] Тесты для `pdf/generator.go`, `mailer/mailer.go`, `cloudstorage/yandex.go`
- [x] Пагинация списка осмотров — 20 записей на страницу, с фильтрами ✅
- [ ] Экспорт в Excel / другие форматы
- [ ] Уведомления (push / Telegram)
- [ ] Мобильное приложение

### Технический долг
- [ ] Gin работает в `debug` mode (нужно `GIN_MODE=release` через env)
- [ ] `You trusted all proxies` — нужно установить `gin.SetTrustedProxies()`
- [ ] Логирование в `/var/log/inspection-app/app.log` может не работать без root на prod

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

## 🚨 12. Важные замечания для AI

- **НЕ СОЗДАВАТЬ** файлы, если они уже существуют — проверять эту карту
- **НЕ ДУБЛИРОВАТЬ** логику: PDF — `internal/pdf/generator.go`, фото — `handlers/photos.go`, облако — `cloudstorage/yandex.go`
- **ПЕРЕД ИЗМЕНЕНИЕМ БД** — обновить `models.go` и учесть что AutoMigrate НЕ удаляет колонки
- **ТЕСТЫ** для handlers требуют отдельной БД `inspection_test` через `TEST_DATABASE_URL`
- **DefectTemplateID** — nullable (`*uint`), NULL означает запись "Прочее"
- **wall_number** — 0 = не стена, 1-4 = стены (дефекты стен хранятся как отдельные RoomDefect)
- **Первый пользователь** в системе автоматически получает роль `admin`
- **Синхронизация с Яндекс Диском** происходит автоматически перед генерацией PDF
- **ОБНОВЛЯТЬ этот файл** после каждого изменения кода

### Правила для AI: email

- **ВСЕГДА** нормализовать email перед сохранением/поиском: `strings.ToLower(strings.TrimSpace(email))`
- Уже реализовано в auth.go, reset.go, admin.go — не нарушать это правило при изменениях

### Правила для AI: тестовые аккаунты

- **НЕ создавать** новые тестовые аккаунты — использовать `test@example.com`
- **ВСЕГДА** проверять существование по email перед созданием
- Если тест падает → исправлять код, а не создавать новый аккаунт
- Если нужен аккаунт для ручного теста → `test@example.com` / `Test1234!`
- Интеграционные тесты используют `newUser()` с изолированной БД — это правильно, не менять
