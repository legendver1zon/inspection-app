#!/bin/bash
# Бэкап PostgreSQL — запускать через cron ежедневно
# crontab -e → 0 3 * * * /opt/inspection-app/scripts/backup.sh
#
# Хранит последние 7 бэкапов, старые удаляются автоматически.

set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-/opt/inspection-app/backups}"
KEEP_DAYS=7
CONTAINER_NAME="inspection-app-postgres-1"
DB_NAME="inspection_db"
DB_USER="inspection"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/backup_${TIMESTAMP}.sql.gz"

mkdir -p "$BACKUP_DIR"

echo "[$(date)] Начинаю бэкап БД..."

# pg_dump внутри контейнера → gzip на хосте
docker exec "$CONTAINER_NAME" pg_dump -U "$DB_USER" "$DB_NAME" | gzip > "$BACKUP_FILE"

SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
echo "[$(date)] Бэкап создан: $BACKUP_FILE ($SIZE)"

# Удаляем бэкапы старше KEEP_DAYS дней
DELETED=$(find "$BACKUP_DIR" -name "backup_*.sql.gz" -mtime +"$KEEP_DAYS" -print -delete | wc -l)
if [ "$DELETED" -gt 0 ]; then
    echo "[$(date)] Удалено старых бэкапов: $DELETED"
fi

echo "[$(date)] Бэкап завершён"
