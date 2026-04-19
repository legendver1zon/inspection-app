-- Миграция: partial unique index на act_number.
-- Проблема: при soft-delete (deleted_at IS NOT NULL) старый уникальный индекс
-- продолжал резервировать act_number, из-за чего новые инспекции не могли
-- использовать тот же номер. Заменяем на partial unique index,
-- который учитывает только живые записи.
--
-- Запуск на VPS:
--   docker compose exec -T postgres psql -U inspection -d inspection_db \
--       < migrations/001_partial_unique_act_number.sql
--
-- Идемпотентно: можно запускать повторно.

BEGIN;

DROP INDEX IF EXISTS idx_inspections_act_number;

CREATE UNIQUE INDEX idx_inspections_act_number
    ON inspections(act_number)
    WHERE deleted_at IS NULL;

COMMIT;
