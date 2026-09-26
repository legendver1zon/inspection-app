#!/usr/bin/env bash
# Диагностика загрузки и показа фото. Ничего не меняет, только читает.
#
# Запуск на сервере:
#   cd /opt/inspection-app && bash scripts/diag-photos.sh
# Отчёт попадает в /tmp/photo-diag-<дата>.txt — прислать его целиком.
#
# SINCE=72h bash scripts/diag-photos.sh  — сузить период логов приложения (по умолчанию 168h = 7 дней).

set -u
cd "$(dirname "$0")/.." || exit 1

SINCE="${SINCE:-168h}"
OUT="/tmp/photo-diag-$(date +%Y%m%d-%H%M).txt"
APPLOG="$(mktemp /tmp/app-log.XXXXXX)"
trap 'rm -f "$APPLOG"' EXIT
exec > >(tee "$OUT") 2>&1

section() { printf '\n\n######## %s\n' "$*"; }
run() { local cmd; cmd="$(cat)"; printf '\n$ %s\n' "$cmd"; eval "$cmd" || true; }
pct() { sort -n | awk '{a[NR]=$1} END{if(NR) printf "n=%d p50=%s p90=%s max=%s\n", NR, a[int(NR/2)+1], a[int(NR*0.9)+1], a[NR]; else print "n=0"}'; }
export -f pct

PSQL='docker compose exec -T postgres psql -U inspection -d inspection_db -c'
NGX_CONF='/etc/nginx/sites-available/inspection-app'

echo "photo-diag: $(date '+%F %T')  host=$(hostname)  since=$SINCE  pwd=$(pwd)"

section "0. Окружение и контейнеры"
run <<'EOF'
uptime; echo; git rev-parse --short HEAD 2>/dev/null || echo "no git"; echo; docker compose ps
EOF
run <<'EOF'
docker inspect "$(docker compose ps -q app)" --format 'image={{.Image}} started={{.State.StartedAt}} restarts={{.RestartCount}} oom={{.State.OOMKilled}} exit={{.State.ExitCode}}'
EOF
run <<'EOF'
docker images --digests ghcr.io/legendver1zon/inspection-app
EOF
run <<'EOF'
docker compose exec -T app env | grep -E '^(GODEBUG|GIN_MODE|LOG_LEVEL|YADISK_ROOT)=' ; docker compose exec -T app env | grep -oE '^(REDIS_URL|YADISK_TOKEN|JWT_SECRET|DATABASE_URL)=' | sed 's/$/<set>/'
EOF

section "1. Логи приложения: выгрузка за период (файл $APPLOG)"
docker compose logs app --since "$SINCE" --no-log-prefix > "$APPLOG" 2>/dev/null || docker compose logs app --since "$SINCE" > "$APPLOG" 2>/dev/null
run <<'EOF'
wc -l "$APPLOG"; head -1 "$APPLOG" | cut -c1-200; echo ...; tail -1 "$APPLOG" | cut -c1-200
EOF
run <<'EOF'
grep -E '"msg":"(server started|server shutting down|cloud storage enabled|cloud storage disabled|redis connected, worker started|redis unavailable, sync photo upload|redis not configured|self-heal loop started)"' "$APPLOG" | tail -20
EOF
run <<'EOF'
echo "стартов сервера: $(grep -c '"msg":"server started"' "$APPLOG")   паник: $(grep -cE '^panic:|goroutine [0-9]+ \[running\]|"msg":"panic' "$APPLOG")"
EOF

section "2. Отправка фото: POST /defects/:id/photos (лог приложения)"
run <<'EOF'
grep '"msg":"request"' "$APPLOG" | grep '"method":"POST"' | grep '"path":"/defects/' | grep -oE '"status":[0-9]+' | sort | uniq -c
EOF
run <<'EOF'
echo "content_length (байт) по успешным 200:"; grep '"method":"POST"' "$APPLOG" | grep '"path":"/defects/' | grep '"status":200' | grep -oE '"content_length":[0-9]+' | cut -d: -f2 | pct
EOF
run <<'EOF'
echo "latency_ms по успешным 200:"; grep '"method":"POST"' "$APPLOG" | grep '"path":"/defects/' | grep '"status":200' | grep -oE '"latency_ms":[0-9]+' | cut -d: -f2 | pct
EOF
run <<'EOF'
echo "тело > 10 МиБ (прошли nginx? быть не должно): $(grep '"method":"POST"' "$APPLOG" | grep '"path":"/defects/' | grep -oE '"content_length":[0-9]+' | cut -d: -f2 | awk '$1>10485760' | wc -l)"
EOF
run <<'EOF'
echo "все НЕ-200 ответы на загрузку (последние 40):"; grep '"method":"POST"' "$APPLOG" | grep '"path":"/defects/' | grep -v '"status":200' | tail -40
EOF
run <<'EOF'
echo "302 без user_id = истёкшая сессия: $(grep '"method":"POST"' "$APPLOG" | grep '"path":"/defects/' | grep '"status":302' | grep -vc user_id)"
EOF
run <<'EOF'
echo "загрузки по часам (сколько POST фото в час):"; grep '"method":"POST"' "$APPLOG" | grep '"path":"/defects/' | grep -oE '"time":"[0-9-]+ [0-9]{2}' | cut -d'"' -f4 | sort | uniq -c | tail -40
EOF
run <<'EOF'
grep '"msg":"redis push failed, fallback sync"' "$APPLOG" | tail -5; echo "redis push failed: $(grep -c '"msg":"redis push failed, fallback sync"' "$APPLOG")"
EOF

section "3. Показ фото: GET /photos/:id/download (лог приложения)"
run <<'EOF'
grep '"msg":"request"' "$APPLOG" | grep '/download"' | grep '"path":"/photos/' | grep -oE '"status":[0-9]+' | sort | uniq -c
EOF
run <<'EOF'
echo "latency_ms по 307 (= время ответа API Яндекса):"; grep '"path":"/photos/' "$APPLOG" | grep '/download"' | grep '"status":307' | grep -oE '"latency_ms":[0-9]+' | cut -d: -f2 | pct
EOF
run <<'EOF'
echo "latency_ms по 200 (локальный файл):"; grep '"path":"/photos/' "$APPLOG" | grep '/download"' | grep '"status":200' | grep -oE '"latency_ms":[0-9]+' | cut -d: -f2 | pct
EOF
run <<'EOF'
echo "залпы: секунды с наибольшим числом обращений к Яндексу за ссылкой:"; grep '"path":"/photos/' "$APPLOG" | grep '/download"' | grep '"status":307' | grep -oE '"time":"[^"]+"' | sort | uniq -c | sort -rn | head -10
EOF
run <<'EOF'
echo "ошибки ссылок из облака по типу:"; grep '"msg":"cloud download URL"' "$APPLOG" | grep -oE 'HTTP [0-9]{3}|Client.Timeout|connection refused|no such host|EOF' | sort | uniq -c; grep '"msg":"cloud download URL"' "$APPLOG" | tail -5
EOF
run <<'EOF'
echo "403/404/5xx на показ (последние 20):"; grep '"path":"/photos/' "$APPLOG" | grep '/download"' | grep -E '"status":(403|404|5[0-9]{2})' | tail -20
EOF
run <<'EOF'
echo "404 на /static/uploads (локальный файл пропал):"; grep '"path":"/static/uploads/' "$APPLOG" | grep -c '"status":404'
EOF

section "4. Выгрузка в Яндекс.Диск (воркер)"
run <<'EOF'
for m in "upload start" "upload complete" "upload file ok" "upload attempt failed" "upload failed permanently" "upload EnsurePath" "upload read file" "upload skip: local file missing" "upload skip: file_path empty" "upload lock busy, will retry later" "worker pop" "worker done" "worker skip, no pending" "worker recovered" "self-heal: reset stuck uploading→pending" "retrying failed photos" "retry: reset failed→pending" "worker retry: reset failed→pending" "soft retry: reset failed→pending on view" "SyncInspectionPhotos panic" "EnsureInspectionFolder MoveFolder" "cloud download URL"; do printf '%6d  %s\n' "$(grep -c "\"msg\":\"$m\"" "$APPLOG")" "$m"; done
EOF
run <<'EOF'
echo "ошибки попыток выгрузки по типу:"; grep -E '"msg":"(upload attempt failed|upload failed permanently|upload EnsurePath)"' "$APPLOG" | grep -oE 'HTTP [0-9]{3}|Client.Timeout|context deadline exceeded|connection reset|no such host|EOF|TLS handshake' | sort | uniq -c | sort -rn
EOF
run <<'EOF'
echo "последние ошибки выгрузки:"; grep -E '"msg":"(upload attempt failed|upload failed permanently|upload EnsurePath|upload skip: local file missing)"' "$APPLOG" | tail -15 | cut -c1-400
EOF
run <<'EOF'
echo "размер выгруженных файлов, size_kb:"; grep '"msg":"upload file ok"' "$APPLOG" | grep -oE '"size_kb":[0-9]+' | cut -d: -f2 | pct
EOF
run <<'EOF'
echo "время выгрузки одного файла (duration):"; grep '"msg":"upload file ok"' "$APPLOG" | grep -oE '"duration":"[^"]+"' | cut -d'"' -f4 | sort | uniq -c | sort -rn | head -10
EOF
run <<'EOF'
echo "время выгрузки пачки (upload complete, последние 20):"; grep '"msg":"upload complete"' "$APPLOG" | grep -oE '"inspection_id":[0-9]+|"photos":[0-9]+|"duration":"[^"]+"' | paste - - - | tail -20
EOF
run <<'EOF'
echo "хронология воркера (последние 40 строк):"; grep -E '"msg":"(upload start|upload complete|retry: reset failed→pending|worker retry: reset failed→pending|soft retry: reset failed→pending on view|self-heal: reset stuck uploading→pending|upload lock busy, will retry later)"' "$APPLOG" | tail -40 | cut -c1-220
EOF

section "5. База данных: состояние фото"
run <<'EOF'
$PSQL "SELECT upload_status, retry_count, (file_path<>'') AS local_file, count(*) FROM photos WHERE deleted_at IS NULL GROUP BY 1,2,3 ORDER BY 1,2,3;"
EOF
run <<'EOF'
$PSQL "SELECT count(*) AS zombie_pending_retry_ge_5 FROM photos WHERE deleted_at IS NULL AND upload_status='pending' AND retry_count>=5;"
EOF
run <<'EOF'
$PSQL "SELECT left(last_error,140) AS last_error, count(*) FROM photos WHERE deleted_at IS NULL AND last_error<>'' GROUP BY 1 ORDER BY 2 DESC LIMIT 15;"
EOF
run <<'EOF'
$PSQL "SELECT id, defect_id, upload_status, retry_count, last_attempt_at, left(last_error,100) AS err FROM photos WHERE deleted_at IS NULL AND upload_status<>'done' ORDER BY id DESC LIMIT 30;"
EOF
run <<'EOF'
$PSQL "SELECT date_trunc('day', created_at)::date AS day, upload_status, count(*) FROM photos WHERE deleted_at IS NULL AND created_at > now() - interval '14 days' GROUP BY 1,2 ORDER BY 1,2;"
EOF
run <<'EOF'
$PSQL "SELECT count(*) AS photos_on_deleted_defects FROM photos p JOIN room_defects d ON d.id=p.defect_id WHERE d.deleted_at IS NOT NULL AND p.deleted_at IS NULL;"
EOF
run <<'EOF'
$PSQL "SELECT p.inspection_id, count(*) AS photos FROM photos p WHERE p.deleted_at IS NULL GROUP BY 1 ORDER BY 2 DESC LIMIT 10;"
EOF
run <<'EOF'
$PSQL "SELECT count(*) AS total_photos, count(*) FILTER (WHERE file_url LIKE 'http%') AS public_url, count(*) FILTER (WHERE file_url LIKE '/static/%') AS static_url, count(*) FILTER (WHERE file_url LIKE 'inspections/%') AS disk_path FROM photos WHERE deleted_at IS NULL;"
EOF

section "6. Redis (очередь выгрузки)"
run <<'EOF'
docker compose exec -T redis redis-cli LLEN inspection_app:upload_jobs; docker compose exec -T redis redis-cli INFO stats | grep -E 'evicted_keys|rejected_connections'; docker compose exec -T redis redis-cli INFO memory | grep -E 'used_memory_human|maxmemory_human'
EOF
run <<'EOF'
docker compose logs redis --since "$SINCE" 2>/dev/null | grep -iE 'MISCONF|fsync|slow|OOM|error' | tail -5
EOF

section "7. nginx: конфиг и логи"
run <<'EOF'
nginx -T 2>/dev/null | grep -nE 'client_max_body_size|client_body_timeout|client_body_buffer_size|proxy_request_buffering|proxy_(read|send|connect)_timeout|send_timeout|proxy_http_version|Upgrade|limit_req|limit_conn|access_log|error_log|http2|gzip' | grep -v '^\s*#'
EOF
run <<'EOF'
echo "--- $NGX_CONF ---"; grep -vE '^\s*(#|$)' "$NGX_CONF"
EOF
run <<'EOF'
echo "статусы POST фото в nginx (включая ротированные логи):"; zgrep -hE 'POST /defects/[0-9]+/photos' /var/log/nginx/access.log* 2>/dev/null | grep -v curl | awk '{print $9}' | sort | uniq -c
EOF
run <<'EOF'
echo "413 — сколько и каких размеров (error.log):"; zgrep -h 'client intended to send too large body' /var/log/nginx/error.log* 2>/dev/null | grep -oE '[0-9]+ bytes' | awk '{printf "%.1f MB\n", $1/1048576}' | sort -n | uniq -c
EOF
run <<'EOF'
echo "413 по дням:"; zgrep -h 'client intended to send too large body' /var/log/nginx/error.log* 2>/dev/null | cut -d' ' -f1 | sort | uniq -c
EOF
run <<'EOF'
echo "обрывы: 408 (стоп >client_body_timeout), 499 (браузер оборвал), 502/504 (последние 30):"; zgrep -hE 'POST /defects/[0-9]+/photos' /var/log/nginx/access.log* 2>/dev/null | awk '$9==408||$9==499||$9==502||$9==504' | tail -30
EOF
run <<'EOF'
echo "статусы показа фото в nginx:"; zgrep -hE 'GET /photos/[0-9]+/download' /var/log/nginx/access.log* 2>/dev/null | awk '{print $9}' | sort | uniq -c
EOF
run <<'EOF'
echo "прочие ошибки nginx (последние 20):"; zgrep -hvE 'too large body' /var/log/nginx/error.log* 2>/dev/null | tail -20
EOF
run <<'EOF'
echo "user-agent на загрузках фото (какие телефоны):"; zgrep -hE 'POST /defects/[0-9]+/photos' /var/log/nginx/access.log* 2>/dev/null | grep -oE '"[^"]*(iPhone|Android|Mobile)[^"]*"$' | sed -E 's/AppleWebKit.*Mobile/… Mobile/' | sort | uniq -c | sort -rn | head -5
EOF

section "8. Ресурсы сервера"
run <<'EOF'
df -h / /opt /var/lib/docker 2>/dev/null; df -i / | tail -1
EOF
run <<'EOF'
du -sh web/static/uploads web/static/documents /var/lib/docker /var/log/nginx 2>/dev/null
EOF
run <<'EOF'
free -m; nproc; docker stats --no-stream
EOF
run <<'EOF'
dmesg -T 2>/dev/null | grep -iE 'out of memory|killed process' | tail -5; journalctl -k --since "-7d" 2>/dev/null | grep -iE 'out of memory|killed process' | tail -5
EOF
run <<'EOF'
curl -s http://127.0.0.1:8080/healthz; echo
EOF

section "9. Локальные файлы фото на диске сервера"
run <<'EOF'
echo "файлов: $(find web/static/uploads/photos -type f 2>/dev/null | wc -l)   больше 10 МиБ: $(find web/static/uploads/photos -type f -size +10M 2>/dev/null | wc -l)   больше 20 МиБ: $(find web/static/uploads/photos -type f -size +20M 2>/dev/null | wc -l)"
EOF
run <<'EOF'
echo "размеры локальных фото (KB):"; find web/static/uploads/photos -type f -printf '%k\n' 2>/dev/null | pct
EOF
run <<'EOF'
echo "расширения:"; find web/static/uploads/photos -type f 2>/dev/null | sed -E 's/.*\.//' | tr 'A-Z' 'a-z' | sort | uniq -c
EOF
run <<'EOF'
echo "самые старые невыгруженные (10 шт):"; find web/static/uploads/photos -type f -printf '%TY-%Tm-%Td %TH:%TM %kK %p\n' 2>/dev/null | sort | head -10
EOF
run <<'EOF'
docker compose exec -T app sh -c 'id; touch /app/web/static/uploads/.wtest && echo WRITE_OK && rm -f /app/web/static/uploads/.wtest'
EOF

section "10. cron и cleanup-orphans"
run <<'EOF'
crontab -l 2>/dev/null; ls -la /etc/cron.d 2>/dev/null; grep -rl cleanup-orphans /etc/cron* /var/spool/cron 2>/dev/null
EOF
run <<'EOF'
DRY_RUN=true bash scripts/cleanup-orphans.sh 2>&1 | head -15
EOF

section "11. Яндекс.Диск: токен и место"
run <<'EOF'
tok="$(grep -E '^YADISK_TOKEN=' .env | cut -d= -f2- | tr -d "\"'")"; if [ -n "$tok" ]; then curl -s -o /tmp/yd.json -w 'HTTP %{http_code}\n' -H "Authorization: OAuth $tok" https://cloud-api.yandex.net/v1/disk/; grep -oE '"(total_space|used_space|trash_size)":[0-9]+' /tmp/yd.json | awk -F: '{printf "%s: %.2f GB\n", $1, $2/1073741824}'; rm -f /tmp/yd.json; else echo "YADISK_TOKEN не задан в .env"; fi
EOF

section "ГОТОВО"
echo "Отчёт: $OUT  ($(wc -l < "$OUT") строк). Пришлите файл целиком."
