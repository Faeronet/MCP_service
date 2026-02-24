-- Revert partitioning: recreate non-partitioned obs.logs_index and drop functions.
-- Data in explicit partitions is lost; default partition data is migrated back.

DO $$
DECLARE
    part_name TEXT;
BEGIN
    -- Recreate non-partitioned table
    CREATE TABLE IF NOT EXISTS obs.logs_index_old (
        id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        ts          TIMESTAMPTZ NOT NULL,
        level       TEXT,
        service     TEXT,
        request_id  TEXT,
        message     TEXT,
        log_id      TEXT,
        raw_ref     TEXT,
        created_at  TIMESTAMPTZ DEFAULT NOW()
    );
    -- Copy from default partition (holds most rows if we migrated from non-partitioned)
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'obs' AND table_name = 'logs_index_default') THEN
        INSERT INTO obs.logs_index_old (id, ts, level, service, request_id, message, log_id, raw_ref, created_at)
        SELECT id, ts, level, service, request_id, message, log_id, raw_ref, created_at FROM obs.logs_index_default;
    END IF;
    DROP TABLE IF EXISTS obs.logs_index;
    ALTER TABLE obs.logs_index_old RENAME TO logs_index;
EXCEPTION WHEN OTHERS THEN
    NULL;
END $$;

DROP FUNCTION IF EXISTS obs.drop_logs_partitions_older_than(int);
DROP FUNCTION IF EXISTS obs.create_logs_partition_for_ts(timestamptz);

-- Recreate indexes from 000003/000004
CREATE INDEX IF NOT EXISTS idx_logs_index_ts ON obs.logs_index(ts);
CREATE INDEX IF NOT EXISTS idx_logs_index_service ON obs.logs_index(service);
CREATE INDEX IF NOT EXISTS idx_logs_index_request_id ON obs.logs_index(request_id);
CREATE INDEX IF NOT EXISTS idx_logs_index_level ON obs.logs_index(level);
