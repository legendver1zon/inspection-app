#!/bin/bash
# Очистка осиротевших файлов — фото и документы без ссылок в БД
# Запускать через cron: 0 4 * * * /opt/inspection-app/scripts/cleanup-orphans.sh
#
# Принцип: получаем список файлов из БД, сравниваем с файлами на диске,
# удаляем файлы которых нет в БД (старше 1 дня, чтобы не удалить свежие загрузки).

set -euo pipefail

APP_DIR="${APP_DIR:-/opt/inspection-app}"
UPLOADS_DIR="$APP_DIR/web/static/uploads"
DOCS_DIR="$APP_DIR/web/static/documents"
CONTAINER_NAME="inspection-app-postgres-1"
DB_NAME="inspection_db"
DB_USER="inspection"
DRY_RUN="${DRY_RUN:-false}"

echo "[$(date)] Начинаю очистку orphan-файлов..."

# Получаем список файлов из БД
db_query() {
    docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "$1" 2>/dev/null
}

# --- Очистка фото ---
if [ -d "$UPLOADS_DIR" ]; then
    DB_PHOTOS=$(db_query "SELECT file_path FROM photos WHERE file_path IS NOT NULL AND file_path != '' AND deleted_at IS NULL")

    DELETED=0
    while IFS= read -r -d '' file; do
        # Пропускаем файлы младше 1 дня
        if [ "$(find "$file" -mtime +0 2>/dev/null)" = "" ]; then
            continue
        fi
        # Проверяем есть ли файл в БД
        if ! echo "$DB_PHOTOS" | grep -qF "$file"; then
            if [ "$DRY_RUN" = "true" ]; then
                echo "  [DRY RUN] Удалил бы: $file"
            else
                rm -f "$file"
                echo "  Удалён: $file"
            fi
            DELETED=$((DELETED + 1))
        fi
    done < <(find "$UPLOADS_DIR" -type f -print0 2>/dev/null)

    echo "[$(date)] Фото: удалено orphan-файлов: $DELETED"
fi

# --- Очистка документов ---
if [ -d "$DOCS_DIR" ]; then
    DB_DOCS=$(db_query "SELECT file_path FROM documents WHERE file_path IS NOT NULL AND file_path != '' AND deleted_at IS NULL")

    DELETED=0
    while IFS= read -r -d '' file; do
        if [ "$(find "$file" -mtime +0 2>/dev/null)" = "" ]; then
            continue
        fi
        if ! echo "$DB_DOCS" | grep -qF "$file"; then
            if [ "$DRY_RUN" = "true" ]; then
                echo "  [DRY RUN] Удалил бы: $file"
            else
                rm -f "$file"
                echo "  Удалён: $file"
            fi
            DELETED=$((DELETED + 1))
        fi
    done < <(find "$DOCS_DIR" -type f -print0 2>/dev/null)

    echo "[$(date)] Документы: удалено orphan-файлов: $DELETED"
fi

echo "[$(date)] Очистка завершена"
