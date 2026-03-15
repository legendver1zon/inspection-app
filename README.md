# Акты осмотра

Веб-приложение для формирования и ведения актов осмотра объектов недвижимости. Позволяет инспекторам фиксировать дефекты по каждому помещению, прикреплять фотографии, хранить их в Яндекс Диске и формировать PDF-документ с QR-кодом для просмотра фото.

## Возможности

- **Создание актов осмотра** — номер акта, дата, время, адрес объекта, этаж, площадь, температура, влажность
- **Помещения и дефекты** — до 10 помещений на акт; по каждому фиксируются дефекты окон, потолка, стен (4 стены), пола, дверей, сантехники
- **Замеры** — длина, ширина, высота помещения, параметры окон и дверей
- **Загрузка плана** — прикрепить фото/план помещений к акту
- **Фото дефектов** — загрузка фотографий к каждому дефекту; файлы хранятся локально до синхронизации с Яндекс Диском
- **Яндекс Диск** — при наличии `YADISK_TOKEN` фото автоматически выгружаются в облако с сохранением структуры папок: `инспекция / комната / секция / дефект / фото`
- **Генерация PDF** — автоматическое формирование акта в формате A4 с таблицами дефектов и блоком подписей
- **QR-код в PDF** — если к осмотру прикреплены фото, в акт вставляется QR-код со ссылкой на публичную папку Яндекс Диска
- **Поиск по актам** — фильтрация по адресу, собственнику, инспектору (только для admin), диапазону дат и статусу
- **Скачивание документов** — все сформированные PDF доступны для скачивания
- **Профиль пользователя** — ФИО и инициалы подставляются в документ автоматически
- **Роли** — `admin` (управление пользователями) и `inspector` (работа с осмотрами)
- **Управление пользователями** — смена роли, удаление аккаунтов (только для администратора)

## Технологии

| Слой | Стек |
|---|---|
| Язык | Go 1.25 |
| Web-фреймворк | [Gin](https://github.com/gin-gonic/gin) |
| БД | PostgreSQL через [GORM](https://gorm.io) |
| Аутентификация | JWT + сессии (gin-contrib/sessions) |
| PDF | [go-pdf/fpdf](https://github.com/go-pdf/fpdf) |
| QR-код | [skip2/go-qrcode](https://github.com/skip2/go-qrcode) |
| Облако | Яндекс Диск REST API |
| Шрифты | Liberation Sans (embedded via `go:embed`) |
| Фронтенд | HTML-шаблоны Go + CSS (без JS-фреймворков) |
| Хостинг | [Timeweb Cloud](https://timeweb.cloud) |

## Структура проекта

```
inspection-app/
├── cmd/server/         # Точка входа, роутер, template functions
├── internal/
│   ├── auth/           # JWT middleware, хэширование паролей
│   ├── cloudstorage/   # Интерфейс FileStorage + адаптер Яндекс Диска
│   ├── handlers/       # HTTP-обработчики (inspections, auth, profile, admin, documents, photos)
│   ├── models/         # GORM-модели (User, Inspection, Room, Defect, Photo...)
│   ├── pdf/            # Генератор PDF (fpdf), embedded шрифты, QR-код
│   ├── seed/           # Начальное заполнение справочника дефектов (48 записей)
│   └── storage/        # Подключение к БД, AutoMigrate
└── web/
    ├── static/
    │   ├── css/        # style.css — дизайн + адаптив (768px / 480px)
    │   ├── js/
    │   └── uploads/    # Загруженные планы и фото дефектов
    └── templates/
        ├── auth/       # login.html, register.html, profile.html
        ├── inspections/# list, new, edit, view
        ├── admin/      # users.html
        └── partials/   # navbar, base
```

## Переменные окружения

| Переменная | Обязательность | Описание |
|---|---|---|
| `DATABASE_URL` | **да** | DSN для подключения к PostgreSQL |
| `YADISK_TOKEN` | нет | OAuth-токен Яндекс Диска. Без него фото хранятся только локально |
| `YADISK_ROOT` | нет | Корневая папка на диске (по умолчанию `disk:/inspection-app`) |

Пример `.env`:
```
DATABASE_URL=postgres://inspection:secret@localhost:5432/inspection_db?sslmode=disable
YADISK_TOKEN=your_token_here
```

## Запуск локально (через Docker)

```bash
git clone https://github.com/legendver1zon/inspection-app.git
cd inspection-app

# Скопировать конфиг
cp .env.example .env
# Отредактировать .env — вставить YADISK_TOKEN при необходимости

# Поднять PostgreSQL + приложение
docker compose up -d

# → http://localhost:8080
```

## Запуск локально (без Docker)

1. Установить и запустить PostgreSQL
2. Создать базу данных:
```sql
CREATE USER inspection WITH PASSWORD 'secret';
CREATE DATABASE inspection_db OWNER inspection;
```
3. Настроить `.env`:
```bash
cp .env.example .env
# Указать свой DATABASE_URL
```
4. Собрать и запустить:
```bash
go build -o app ./cmd/server
./app
# → http://localhost:8080
```

> **Windows:** шрифты подхватываются автоматически из `C:/Windows/Fonts/arial.ttf`
> **Linux:** нужен пакет `fonts-liberation` (`apt-get install -y fonts-liberation`)

## Деплой на Timeweb Cloud

1. Создать VPS на [timeweb.cloud](https://timeweb.cloud) (Ubuntu 22.04)
2. Установить зависимости:
```bash
apt-get install -y golang fonts-liberation postgresql
```
3. Создать пользователя и базу PostgreSQL:
```bash
sudo -u postgres psql -c "CREATE USER inspection WITH PASSWORD 'yourpassword';"
sudo -u postgres psql -c "CREATE DATABASE inspection_db OWNER inspection;"
```
4. Клонировать репозиторий и собрать:
```bash
git clone https://github.com/legendver1zon/inspection-app.git /opt/inspection-app
cd /opt/inspection-app
go build -o inspection-app-bin ./cmd/server
```
5. Добавить переменные окружения в systemd-сервис `/etc/systemd/system/inspection-app.service`:
```ini
[Service]
Environment=DATABASE_URL=postgres://inspection:yourpassword@localhost:5432/inspection_db?sslmode=disable
Environment=YADISK_TOKEN=your_token
ExecStart=/opt/inspection-app/inspection-app-bin
```
6. Перезапустить сервис:
```bash
systemctl daemon-reload
systemctl restart inspection-app
```

## Тесты

Проект покрыт интеграционными тестами на базе PostgreSQL.
Тесты требуют переменную окружения `TEST_DATABASE_URL`.

```bash
# Поднять тестовую БД (достаточно только postgres)
docker compose up postgres -d

# Запустить тесты
TEST_DATABASE_URL=postgres://inspection:secret@localhost:5432/inspection_test?sslmode=disable \
  go test ./internal/handlers/... -v
```

Каждый тест полностью сбрасывает и пересоздаёт схему — тесты изолированы друг от друга.
Без `TEST_DATABASE_URL` тесты пропускаются (`t.Skip`).

```bash
go test ./internal/handlers/... -v
```

Что покрыто:

| Пакет | Тесты |
|---|---|
| `handlers/auth_test.go` | Логин, регистрация, выход, первый пользователь → admin, валидация |
| `handlers/inspections_test.go` | Авторизация, изоляция данных, вкладки draft/completed, фильтры по адресу / собственнику / инспектору / дате, комбинированные фильтры, счётчики |
| `handlers/photos_test.go` | Загрузка фото (успех, forbidden, неверный формат, без файла), удаление фото, синхронизация с Яндекс Диском (mock) |

## Роли пользователей

| Роль | Возможности |
|---|---|
| `inspector` | Создание и редактирование своих актов, генерация PDF, загрузка фото |
| `admin` | Всё то же + просмотр всех актов + управление пользователями + фильтр по инспектору |

Первый зарегистрированный пользователь получает роль `admin`, остальные — `inspector`.

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
    25/                              ← публичная папка (URL → QR-код в PDF)
      Кухня/
        Потолок/
          Трещины/
            photo_1.jpg
            photo_2.jpg
        Стены/
          Стена_1/
            Отклонение/
              photo_1.jpg
```
