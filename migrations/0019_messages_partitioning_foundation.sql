CREATE TABLE IF NOT EXISTS messages_archive_partitioned (
    LIKE messages INCLUDING DEFAULTS INCLUDING CONSTRAINTS INCLUDING INDEXES
) PARTITION BY RANGE (created_at);

CREATE TABLE IF NOT EXISTS messages_archive_2026_01 PARTITION OF messages_archive_partitioned
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE IF NOT EXISTS messages_archive_2026_02 PARTITION OF messages_archive_partitioned
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE IF NOT EXISTS messages_archive_2026_03 PARTITION OF messages_archive_partitioned
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE IF NOT EXISTS messages_archive_2026_04 PARTITION OF messages_archive_partitioned
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE IF NOT EXISTS messages_archive_2026_05 PARTITION OF messages_archive_partitioned
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS messages_archive_2026_06 PARTITION OF messages_archive_partitioned
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE IF NOT EXISTS messages_archive_2026_07 PARTITION OF messages_archive_partitioned
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE IF NOT EXISTS messages_archive_2026_08 PARTITION OF messages_archive_partitioned
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE IF NOT EXISTS messages_archive_2026_09 PARTITION OF messages_archive_partitioned
    FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
CREATE TABLE IF NOT EXISTS messages_archive_2026_10 PARTITION OF messages_archive_partitioned
    FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');
CREATE TABLE IF NOT EXISTS messages_archive_2026_11 PARTITION OF messages_archive_partitioned
    FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
CREATE TABLE IF NOT EXISTS messages_archive_2026_12 PARTITION OF messages_archive_partitioned
    FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');

CREATE INDEX IF NOT EXISTS idx_messages_archive_partitioned_group_created
    ON messages_archive_partitioned (group_id, created_at DESC);
