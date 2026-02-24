-- Partition obs.logs_index by RANGE (ts) for large log volume.
-- Monthly partitions + default for existing data; function to create new partitions and retention.

-- 1) Create partitioned table (same structure as obs.logs_index; message_tsv added in 000004)
CREATE TABLE IF NOT EXISTS obs.logs_index_new (
    id          UUID DEFAULT gen_random_uuid(),
    ts          TIMESTAMPTZ NOT NULL,
    level       TEXT,
    service     TEXT,
    request_id  TEXT,
    message     TEXT,
    log_id      TEXT,
    raw_ref     TEXT,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    message_tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple', coalesce(message, ''))) STORED,
    PRIMARY KEY (id, ts)
) PARTITION BY RANGE (ts);

-- 2) Default partition for existing rows and any ts not yet covered
CREATE TABLE IF NOT EXISTS obs.logs_index_default PARTITION OF obs.logs_index_new
    DEFAULT;

-- 3) Copy data from existing table (if it exists)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'obs' AND table_name = 'logs_index') THEN
        INSERT INTO obs.logs_index_new (id, ts, level, service, request_id, message, log_id, raw_ref, created_at)
        SELECT id, ts, level, service, request_id, message, log_id, raw_ref, created_at
        FROM obs.logs_index;
    END IF;
EXCEPTION WHEN OTHERS THEN
    NULL;
END $$;

-- 4) Replace table: drop old, rename new
DROP TABLE IF EXISTS obs.logs_index;
ALTER TABLE obs.logs_index_new RENAME TO logs_index;

-- 5) Indexes on partitioned table
CREATE INDEX IF NOT EXISTS idx_logs_index_ts ON obs.logs_index(ts);
CREATE INDEX IF NOT EXISTS idx_logs_index_ts_desc ON obs.logs_index(ts DESC);
CREATE INDEX IF NOT EXISTS idx_logs_index_service ON obs.logs_index(service);
CREATE INDEX IF NOT EXISTS idx_logs_index_request_id ON obs.logs_index(request_id);
CREATE INDEX IF NOT EXISTS idx_logs_index_level ON obs.logs_index(level);
CREATE INDEX IF NOT EXISTS idx_logs_index_message_tsv ON obs.logs_index USING GIN(message_tsv);

-- 6) Function: create monthly partition for given timestamp if not exists
CREATE OR REPLACE FUNCTION obs.create_logs_partition_for_ts(ts_val TIMESTAMPTZ)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    part_start DATE := date_trunc('month', ts_val)::date;
    part_end   DATE := part_start + interval '1 month';
    part_name  TEXT := 'logs_index_' || to_char(part_start, 'YYYY_MM');
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'obs' AND c.relname = part_name
    ) THEN
        EXECUTE format(
            'CREATE TABLE obs.%I PARTITION OF obs.logs_index FOR VALUES FROM (%L) TO (%L)',
            part_name, part_start, part_end
        );
    END IF;
END $$;

-- 7) Function: drop partitions older than N days (retention)
CREATE OR REPLACE FUNCTION obs.drop_logs_partitions_older_than(days int DEFAULT 30)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    r RECORD;
    part_cutoff DATE := (CURRENT_DATE - (days || ' days')::interval);
BEGIN
    FOR r IN
        SELECT c.relname
        FROM pg_inherits i
        JOIN pg_class c ON c.oid = i.inhrelid
        JOIN pg_class p ON p.oid = i.inhparent
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE p.relname = 'logs_index' AND n.nspname = 'obs' AND c.relname <> 'logs_index_default'
    LOOP
        IF r.relname ~ '^logs_index_\d{4}_\d{2}$' THEN
            BEGIN
                IF (to_date(substring(r.relname from 'logs_index_(\d{4}_\d{2})'), 'YYYY_MM') + interval '1 month')::date <= part_cutoff THEN
                    EXECUTE format('DROP TABLE IF EXISTS obs.%I', r.relname);
                END IF;
            EXCEPTION WHEN OTHERS THEN
                NULL;
            END;
        END IF;
    END LOOP;
END $$;

-- Create first explicit monthly partition for current month
SELECT obs.create_logs_partition_for_ts(NOW());
