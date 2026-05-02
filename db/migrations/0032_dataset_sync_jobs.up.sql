CREATE TABLE dataset_sync_jobs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    dataset_id BIGINT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    source_work_table_id BIGINT NOT NULL REFERENCES dataset_work_tables(id) ON DELETE CASCADE,
    requested_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    outbox_event_id BIGINT REFERENCES outbox_events(id) ON DELETE SET NULL,
    mode TEXT NOT NULL DEFAULT 'full_refresh',
    status TEXT NOT NULL DEFAULT 'pending',
    old_raw_database TEXT NOT NULL,
    old_raw_table TEXT NOT NULL,
    new_raw_database TEXT NOT NULL,
    new_raw_table TEXT NOT NULL,
    row_count BIGINT NOT NULL DEFAULT 0 CHECK (row_count >= 0),
    total_bytes BIGINT NOT NULL DEFAULT 0 CHECK (total_bytes >= 0),
    error_summary TEXT,
    cleanup_status TEXT,
    cleanup_error_summary TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT dataset_sync_jobs_public_id_key UNIQUE (public_id),
    CONSTRAINT dataset_sync_jobs_mode_check CHECK (mode IN ('full_refresh')),
    CONSTRAINT dataset_sync_jobs_status_check CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    CONSTRAINT dataset_sync_jobs_cleanup_status_check CHECK (cleanup_status IS NULL OR cleanup_status IN ('completed', 'failed', 'skipped')),
    CONSTRAINT dataset_sync_jobs_old_raw_database_check CHECK (btrim(old_raw_database) <> ''),
    CONSTRAINT dataset_sync_jobs_old_raw_table_check CHECK (btrim(old_raw_table) <> ''),
    CONSTRAINT dataset_sync_jobs_new_raw_database_check CHECK (btrim(new_raw_database) <> ''),
    CONSTRAINT dataset_sync_jobs_new_raw_table_check CHECK (btrim(new_raw_table) <> '')
);

CREATE UNIQUE INDEX dataset_sync_jobs_active_dataset_key
    ON dataset_sync_jobs(dataset_id)
    WHERE status IN ('pending', 'processing');

CREATE INDEX dataset_sync_jobs_dataset_created_idx
    ON dataset_sync_jobs(dataset_id, created_at DESC, id DESC);

CREATE INDEX dataset_sync_jobs_tenant_status_idx
    ON dataset_sync_jobs(tenant_id, status, created_at DESC, id DESC);
