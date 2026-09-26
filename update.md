# Inspection App — План развития и технический долг

**Последнее обновление:** 2026-09-26
**Текущий статус:** Production (Timeweb Cloud, 5.42.105.93:8080) — старый фронт;
ветка `design-frontend` — React-фронтенд + фиксы фото/сессии, готов к выкатке
**Версия:** backend: Go 1.25 / Gin / GORM / PostgreSQL 16 / Redis 7;
frontend: React 18 / TypeScript / Vite / Tailwind v4 / Framer Motion / TanStack Query

---

## Сессия 5 — React-фронтенд (2026-07-25, ветка design-frontend)

Фронтенд переписан на React по решению владельца. Монорепо: `backend/`
(Go, перенесён git mv) + `frontend/`. Дизайн — «Лента» (выбран из трёх
концептов): кремовая бумага, тёплый графит, синий акцент #3E68A8,
хронология выездов по дням.

**Экраны (все, с анимациями):** логин; лента осмотров (сводка числами,
живой поиск, фильтр по статусу, таймлайн по дням); просмотр акта
(параметры чипами, помещения с дефектами и фото, архив, документы,
PDF, статус завершения); форма редактирования (шапка с live-проверкой
номера, помещения аккордеоном, дефекты по разделам, фото с прогрессом
загрузки и удалением, кроп плана через cropperjs из npm, автосейв
с защитой от гонок); дашборд; профиль (аватар, пароль); админка
пользователей (роли, сброс пароля, удаление с предохранителями).

**Бэкенд:** JSON-API `/api/*` (cookie-JWT, 401/403 JSON, APIAdminOnly);
сохранение формы — в старый проверенный POST /inspections/:id/edit;
**перепривязка фото** при пересоздании дефектов (ключ: комната+раздел+
шаблон+стена) — фото больше не уходят в архив при каждом сохранении.

**Прод:** Go отдаёт `web/spa` (React) вместо HTML-страниц, NoRoute —
SPA-fallback; без сборки работают старые шаблоны (dev). Dockerfile
в корне: node build → go build → alpine. Не перенесены на React:
register / forgot-password / reset-password (старые страницы работают).

**Выкатка на VPS:** git merge design-frontend → main → push (CI соберёт
образ из корневого Dockerfile) → на сервере bash update.sh. Миграций БД нет.

---

## Сессия 6 — фото и сессия по логам прода (2026-09-26, ветка design-frontend)

Расследование жалобы «фото долго загружается и иногда не загружается»
(скрипт `backend/scripts/diag-photos.sh`, запускается на сервере, отчёт в
/tmp/photo-diag-*.txt). По логам за сентябрь: сервер принимает фото за
24–34 мс, выгрузка в Яндекс без ошибок, БД чистая. Причины на стороне
телефона и показа:

- оригиналы 2,4 МБ (макс 6,4 МБ) без сжатия, все файлы параллельно →
  минуты на мобильной сети, обрывы 408/499 (nginx client_body_timeout 60 с);
- сессия 24 ч без продления → 302 на /login посреди осмотра (25.09 — 14
  запросов, 5 фото потеряны), фронт показывал общий тост;
- показ: каждая миниатюра = вызов API Яндекса (p50 300 мс) + оригинал
  2–3 МБ, в актах по 200–435 фото;
- nginx `client_max_body_size 10M` — реальных 413 всего 4 за три месяца.

**Что сделано:**

- Бэкенд: сессия 7 дней с продлением при активности (`auth.SessionTTL`,
  `RenewIfNeeded` в RequireAuth и APIAuth); RequireAuth отвечает 401 JSON
  fetch/XHR-клиентам (`WantsJSON`: X-Requested-With, Accept, Sec-Fetch-Mode)
  вместо редиректа; JSON-API `/api/register`, `/api/forgot-password`,
  `/api/reset-password` на общей логике с HTML-обработчиками
  (`registerUser`, `requestPasswordReset`, `resetPassword`) и JSON-лимитами;
  миниатюры `GET /photos/:id/thumb` (пакет `internal/thumbs`: 480 px, EXIF,
  генерация при загрузке и лениво при первом показе, семафор на 2,
  Cache-Control на год, fallback на оригинал).
- Фронт: страницы регистрации и сброса пароля (`AuthShell`); очередь отправки
  фото `lib/uploadQueue.ts` — сжатие на телефоне до 1920 px/JPEG 0.82
  (`lib/compressImage.ts`), отправка по одному, повторы с паузой при обрывах,
  ожидание сети, пауза при 401 и продолжение после входа, счётчик в шапке,
  кнопка «Повторить»; после входа возврат на прежнюю страницу; миниатюры
  через `PhotoThumb`.
- Compose: пути томов через `UPLOADS_DIR`/`DOCUMENTS_DIR` (на сервере
  `./web/static/*` как раньше, локально `./backend/web/static/*` в .env).

**Офлайн на объекте (2026-09-26, продолжение сессии 6):**

- Фронт: очередь фото `lib/uploadQueue.ts` хранится в IndexedDB
  (`idb-keyval`, БД inspection-app/uploads) и переживает перезагрузку и
  закрытие вкладки; фото адресуется ключом помещение/раздел/шаблон/стена
  (`POST /inspections/:id/photos`), а не id дефекта; `client_id` защищает
  от дублей при повторах. Черновик формы `lib/draftStore.ts` (БД drafts)
  пишется при каждой правке и стирается после успешного сохранения;
  при открытии редактора черновик главнее данных сервера, показывается
  баннер. Автосохранение ждёт сеть (`useOnline`), после сбоя повторяет
  каждые 8 с и по событию online. PWA через `vite-plugin-pwa`: оболочка
  и ответы GET /api/* кешируются Service Worker'ом (NetworkFirst),
  миниатюры /photos/:id/thumb — CacheFirst; манифест, иконки
  `public/icons`, «На главный экран». Баннер «Нет сети» в шапке.
- Бэкенд: `picked_{tpl}_{i}`, `picked_{tpl}_{i}_wall{w}`,
  `picked_notes_{sec}_{i}` в POST /inspections/:id/edit сохраняют
  выбранный дефект без значения (заготовка), чтобы фото жили до ввода
  значения; PDF пустые пропускает. Раздача sw.js/registerSW.js/
  manifest.webmanifest/icons из web/spa в SPA-режиме, CSP worker-src.
- Ограничение: отправка идёт, пока приложение открыто (или при следующем
  открытии); фоновой отправки при закрытом приложении (Background Sync)
  пока нет.

**После выкатки caff58d (2026-09-26):**

- Удаление акта со страницы просмотра (208b1c0): инспектор — свои
  черновики, администратор — любые; `can_delete` в API акта,
  `POST /inspections/:id/delete` отвечает JSON fetch-клиентам.
- Значения по стенам: в карточке стенового дефекта кнопка «Разные значения
  по стенам» раскрывает поле на каждую отмеченную стену (как в старом
  HTML-редакторе; PDF печатает таблицу по стенам). Если в акте значения уже
  различаются, режим включается сам; «Одно значение» сводит обратно
  с подтверждением. Сервер и PDF не менялись (`defect_{tpl}_{i}_wall{w}`).
- Фото без дефекта: «Общий вид помещения» в начале каждого помещения и фото
  к общим замечаниям (электрика, вентиляция, общие). Модель `Photo` стала
  самостоятельной: `inspection_id`, `kind` (defect | room | electricity |
  ventilation | general), `room_number`; `defect_id` теперь nullable.
  Миграция в `storage.Migrate`: `DROP NOT NULL` + заполнение `inspection_id`
  старым фото по цепочке дефект → помещение; хук `Photo.BeforeCreate`
  доопределяет осмотр, если фото создано только с `defect_id`. Все запросы
  фото по осмотру идут через `photos.inspection_id` (воркер, планировщик,
  карточки списка). Загрузка тем же `POST /inspections/:id/photos`:
  `section=overview` + `room_number`, либо `section=electricity|ventilation|
  general` без помещения. В облаке: `{акт}/{помещение}/Общий_вид` и
  `{акт}/Общие_замечания/{Электричество|Вентиляция|Общие}`. API просмотра и
  edit-data отдают `rooms[].photos` и `general_photos`. PDF не менялся
  (фото в него не входят).
- Форма сохранения передаёт `room_prev_{i}` — прежний номер помещения
  (0 — новое). По нему к пересозданным дефектам и помещению переезжают фото,
  поэтому удаление помещения из середины больше не путает фото соседей;
  фото исчезнувших помещений уходят в архив. Старая HTML-форма поля не шлёт —
  номера считаются неизменными.

**PDF и подписи (2026-09-26, релиз 1 из 2):**

- Переключатель «Температура и влажность в акте» в «Параметрах объекта»
  (`inspections.hide_climate`); выключен — строка t/RH не печатается, поля в
  форме скрыты. Новый акт наследует значение от последнего акта инспектора.
- Таблица замеров внизу первой страницы PDF убрана вместе с расчётом её
  высоты; план может занять всю первую страницу.
- Рукописные подписи: инспектор и собственник (застройщик подписывает
  бумагу, его строка пустая). Canvas-поле `edit/SignaturePad.tsx`, PNG
  data-URL уходит полями `signature_{role}`, `signature_{role}_at`,
  `signature_{role}_clear`, `signature_inspector_from_profile` формы
  сохранения (значит, работает без сети через автосохранение и живёт в
  черновике). Таблица `signatures` (inspection_id+role уникальны, файл в
  uploads/signatures с случайным именем, `tz_offset_min` — зона телефона,
  в PDF печатается местное время). Подпись профиля: `users.signature_path`,
  `GET/POST /api/profile/signature`, `POST /api/profile/signature/delete`;
  картинки акта — `GET /api/inspections/:id/signature/:role` (только
  владелец/админ). Подпись собственника блокирует акт: сервер отвергает
  сохранение формы, удаление фото и удаление акта (403
  «Акт подписан собственником…»), редактор показывает плашку и кнопку
  «Снять подпись» (поле `signature_owner_clear`). Загрузка фото из очереди
  при блокировке разрешена намеренно.
- Следующий релиз: разметка дефектов на плане (фигуры по разделам, номера
  помещений и стен, легенда под планом).

**Не сделано (следующий этап):** Background Sync при закрытом приложении, полный офлайн для актов
(локальная база, черновики без номера, API синхронизации, точечное
сохранение вместо пересоздания дефектов — сейчас 1699 фото в архиве
из-за пересохранений), оболочка Capacitor для Google Play.

**Выкатка:** merge design-frontend → main → push → CI → на сервере
`bash update.sh`. Миграций БД нет (AutoMigrate). На сервере отдельно:
nginx `client_max_body_size 25M`, `client_body_timeout 300s`,
`proxy_read_timeout 300s`, `proxy_send_timeout 300s`. Скрипт
`scripts/cleanup-orphans.sh` НЕ запускать: сравнивает host-пути с
контейнерными и удалит планы/аватары.

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
| Go-файлов | 60 |
| Строк Go-кода | 13,566 |
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

### Сессия 4 — Аудит качества и безопасности (2026-07-24)

**Безопасность:**
- [x] `act_number`: серверная валидация (`security.ValidateActNumber`, ≤64 символа,
      буквы/цифры/пробел/`._/-`, запрет `..`) в PostEditInspection и check-act-number API.
      Раньше редактируемый номер попадал в пути Яндекс.Диска без ограничений — path
      traversal / коллизии папок
- [x] `sanitizeFolderName` применён к ActNumber в EnsureInspectionFolder и
      buildDefectInfoMap (защита в глубину для уже существующих записей);
      имена из одних точек → fallback на ID
- [x] Rate limit на `POST /reset-password` (5/15мин, инкремент при неверном коде) —
      6-значный код перебирался без ограничений; вектор захвата аккаунта
- [x] Rate limit на `/admin/*` (60/мин на IP)
- [x] Content-Security-Policy (пока с `unsafe-inline` — в шаблонах inline-скрипты)
- [x] Лимит тела запроса: 10 МБ формы, 200 МБ маршруты загрузки фото/плана/аватара
- [x] `GetPhotoDownload`: `c.File` только из каталога `web/static/uploads`
- [x] `GetDownloadDocument`: `HasPrefix` с разделителем (не пропускает `documents-evil`)
- [x] `strconv.Atoi` для `:id` в admin.go и documents.go
- [x] `COOKIE_SECURE` задокументирован в `.env.example`

**Техдолг:**
- [x] A4: `buildInitials` → общий пакет `internal/textutil` (две идентичные копии удалены)
- [x] A6: worker больше не импортирует handlers — `worker.New(q, uploadFn)`, связывает main.go
- [x] D7: все 17 непроверенных `DB.First()` закрыты:
      - пользователь для рендера берётся из контекста (`handlers.CurrentUser`,
        auth-middleware кладёт его туда — минус лишний запрос на каждый запрос)
      - цепочка авторизации фото → `loadPhotoInspection` c Unscoped и явными 404.
        Попутно исправлен продовый баг: владелец-неадмин получал 403 при скачивании
        фото архивных (soft-deleted) дефектов
- [x] `MemoryLocker`: refcount + удаление записей — map мьютексов рос бесконечно
- [x] `sync_scheduler`: syncCtx, отмена debounce-таймеров при shutdown,
      `StartSelfHealLoop` возвращает wait (main дожидается цикла);
      `TriggerRetryForInspection` через ScheduleSync вместо голой горутины

**По итогам многоагентной ревизии диффа (найдено и исправлено до коммита):**
- [x] CSP блокировал Cropper.js с cdnjs (кроп плана) и QR-код с api.qrserver.com —
      добавлены в белый список; вендоринг зависимостей — задача U6
- [x] Валидация act_number применялась и к неизменённому legacy-номеру —
      блокировала бы сохранение всего акта; теперь валидируется только изменённый
- [x] Номер акта с «/» (легитимный формат «15/2026») ронял генерацию PDF —
      имя файла санитизируется в pdf.Generate
- [x] Перебор кода сброса лимитировался только по IP — добавлен лимит по email
      (распределённый перебор с многих IP упирается в счётчик аккаунта)
- [x] 429-страница reset-password теряла email из формы
- [x] Фото в `pending` без Redis после рестарта зависали навсегда —
      self-heal теперь пересинхронизирует stale pending (resyncStalePending)

**Тесты:**
- [x] Хрупкие тайминг-тесты bcrypt (>50мс wall-clock) → проверка `bcrypt.Cost`
- [x] Починены 2 давно сломанных интеграционных теста (падали и на HEAD):
      `TestGetDownloadDocument_FileMissing_DeletesRecord` (путь вне allowedDir → 403),
      `TestSyncPhotos_30Photos_SingleUser` (устаревшее ожидание publish-URL на файл)
- [x] Новые тесты: `ValidateActNumber`, `textutil.Initials`, mutual exclusion +
      очистка map в MemoryLocker (-race), скачивание/удаление фото архивных дефектов,
      404 для фото удалённого осмотра, отказ отдачи файла вне uploads,
      path traversal в документах (403 + запись не удаляется), rate limit
      reset-password (429, сброс при успехе, per-email лимит), cost dummyHash
- [x] `resetAllLimiters()` дополнен новыми лимитерами (иначе тесты копили счётчики)
- [x] Полный прогон unit + integration — зелёный

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

### Закрыто в сессии 4 (2026-07-24)
- ~~A4: Дублирование buildInitials~~ → internal/textutil
- ~~A6: Worker импортирует handlers~~ → инъекция функции загрузки из main.go
- ~~D7: DB.First() без проверки ошибки (17 мест)~~ → CurrentUser + loadPhotoInspection

### Остаётся

| # | Проблема | Где | Приоритет |
|---|---------|-----|-----------|
| A1 | handlers — 6500+ строк в одном пакете; облачная синхронизация фото заслуживает пакета photosync | internal/handlers/ | Средний |
| A2 | Глобальные переменные (storage.DB, cloudStore, uploadLocker, wsHub) | handlers, storage | Средний |
| A7 | Дублирование retry-логики: sync_scheduler.retryFailedPhotos ≈ worker.retryFailed | handlers + worker | Низкий |
| D3 | InspectionRoom — 20+ числовых полей (Window1-5) | models.go | Низкий |
| F3 | Локальные файлы без backup | web/static/uploads/ | Средний |
| U6 | Строгий CSP без unsafe-inline: вынести inline-скрипты и onclick из 14 шаблонов | web/templates/ | Средний |

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

## Security Review (обновлён 2026-07-24)

**Оценка: 9/10.** Критических уязвимостей нет.

### Исправлено (2026-05, коммит 546e944)
- [x] Пароль БД в docker-compose.yml → `${POSTGRES_PASSWORD:-secret}`
- [x] Trusted Proxies → `SetTrustedProxies(["127.0.0.1", "::1"])`
- [x] `os.Remove()` ошибки теперь логируются
- [x] Security-заголовки (X-Content-Type-Options, X-Frame-Options, Referrer-Policy, Permissions-Policy)
- [x] Path traversal в GetDownloadDocument, WebSocket CheckOrigin, timing attack при логине

### Исправлено (2026-07-24, сессия 4)
- [x] `act_number` в путях Яндекс.Диска — валидация + санитизация (был path traversal)
- [x] Rate limit на `/reset-password` (перебор 6-значного кода) и `/admin/*`
- [x] Content-Security-Policy (базовый, с unsafe-inline)
- [x] Лимиты размера тела запроса (MaxBytesReader)
- [x] `c.File` из проверенного каталога, `Atoi` для id-параметров

### Остаётся (средний/низкий приоритет)
- [ ] Строгий CSP — требует выноса inline-скриптов/onclick из шаблонов (U6)
- [ ] CSRF-токены — сейчас только SameSite=Lax (приемлемо: опасные операции POST)
- [ ] HTTPS + HSTS + `COOKIE_SECURE=true` — ждёт покупки домена
- [ ] Redis rate limiter fail-open при недоступности Redis (осознанный компромисс)

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
