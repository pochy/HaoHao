ALTER TABLE datasets
    ADD COLUMN source_kind TEXT NOT NULL DEFAULT 'file',
    ADD COLUMN source_work_table_id BIGINT,
    ALTER COLUMN source_file_object_id DROP NOT NULL;

ALTER TABLE datasets
    ADD CONSTRAINT datasets_source_kind_check
    CHECK (source_kind IN ('file', 'work_table'));

CREATE TABLE dataset_work_tables (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    source_dataset_id BIGINT REFERENCES datasets(id) ON DELETE SET NULL,
    created_from_query_job_id BIGINT REFERENCES dataset_query_jobs(id) ON DELETE SET NULL,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    work_database TEXT NOT NULL,
    work_table TEXT NOT NULL,
    display_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'dropped')),
    row_count BIGINT NOT NULL DEFAULT 0 CHECK (row_count >= 0),
    total_bytes BIGINT NOT NULL DEFAULT 0 CHECK (total_bytes >= 0),
    engine TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    dropped_at TIMESTAMPTZ,
    CHECK (btrim(work_database) <> ''),
    CHECK (btrim(work_table) <> ''),
    CHECK (btrim(display_name) <> '')
);

CREATE UNIQUE INDEX dataset_work_tables_public_id_key
    ON dataset_work_tables(public_id);

CREATE UNIQUE INDEX dataset_work_tables_active_table_key
    ON dataset_work_tables(tenant_id, work_database, work_table)
    WHERE status = 'active' AND dropped_at IS NULL;

CREATE INDEX dataset_work_tables_tenant_updated_idx
    ON dataset_work_tables(tenant_id, updated_at DESC, id DESC);

CREATE INDEX dataset_work_tables_tenant_dataset_idx
    ON dataset_work_tables(tenant_id, source_dataset_id, updated_at DESC, id DESC)
    WHERE source_dataset_id IS NOT NULL;

ALTER TABLE datasets
    ADD CONSTRAINT datasets_source_work_table_id_fkey
    FOREIGN KEY (source_work_table_id) REFERENCES dataset_work_tables(id) ON DELETE SET NULL;

ALTER TABLE datasets
    ADD CONSTRAINT datasets_file_source_check
    CHECK (source_kind <> 'file' OR source_file_object_id IS NOT NULL);

ALTER TABLE datasets
    ADD CONSTRAINT datasets_work_table_source_check
    CHECK (source_kind <> 'work_table' OR source_work_table_id IS NOT NULL);

CREATE INDEX datasets_source_work_table_idx
    ON datasets(source_work_table_id)
    WHERE source_work_table_id IS NOT NULL;

CREATE TABLE dataset_work_table_exports (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    work_table_id BIGINT NOT NULL REFERENCES dataset_work_tables(id) ON DELETE CASCADE,
    requested_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    file_object_id BIGINT REFERENCES file_objects(id) ON DELETE SET NULL,
    outbox_event_id BIGINT REFERENCES outbox_events(id) ON DELETE SET NULL,
    format TEXT NOT NULL DEFAULT 'csv'
        CHECK (format IN ('csv')),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'ready', 'failed', 'deleted')),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '7 days',
    error_summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX dataset_work_table_exports_public_id_key
    ON dataset_work_table_exports(public_id);

CREATE INDEX dataset_work_table_exports_work_table_created_idx
    ON dataset_work_table_exports(work_table_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX dataset_work_table_exports_pending_idx
    ON dataset_work_table_exports(created_at, id)
    WHERE status IN ('pending', 'processing');
