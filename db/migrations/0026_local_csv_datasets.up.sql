ALTER TABLE file_objects
    DROP CONSTRAINT IF EXISTS file_objects_purpose_check;

ALTER TABLE file_objects
    ADD CONSTRAINT file_objects_purpose_check
    CHECK (purpose IN ('attachment', 'avatar', 'import', 'export', 'drive', 'dataset_source'));

CREATE TABLE datasets (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    source_file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    byte_size BIGINT NOT NULL CHECK (byte_size >= 0),
    raw_database TEXT NOT NULL,
    raw_table TEXT NOT NULL,
    work_database TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'importing', 'ready', 'failed', 'deleted')),
    row_count BIGINT NOT NULL DEFAULT 0 CHECK (row_count >= 0),
    error_summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    imported_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    CHECK (btrim(name) <> ''),
    CHECK (btrim(raw_database) <> ''),
    CHECK (btrim(raw_table) <> ''),
    CHECK (btrim(work_database) <> '')
);

CREATE UNIQUE INDEX datasets_public_id_key
    ON datasets(public_id);

CREATE UNIQUE INDEX datasets_tenant_raw_table_key
    ON datasets(tenant_id, raw_table);

CREATE INDEX datasets_tenant_created_idx
    ON datasets(tenant_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX datasets_source_file_idx
    ON datasets(source_file_object_id);

CREATE TABLE dataset_columns (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    dataset_id BIGINT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL CHECK (ordinal > 0),
    original_name TEXT NOT NULL,
    column_name TEXT NOT NULL,
    clickhouse_type TEXT NOT NULL DEFAULT 'Nullable(String)',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (btrim(column_name) <> ''),
    CHECK (btrim(clickhouse_type) <> '')
);

CREATE UNIQUE INDEX dataset_columns_dataset_ordinal_key
    ON dataset_columns(dataset_id, ordinal);

CREATE UNIQUE INDEX dataset_columns_dataset_column_name_key
    ON dataset_columns(dataset_id, column_name);

CREATE TABLE dataset_import_jobs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    dataset_id BIGINT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    source_file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE RESTRICT,
    requested_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    outbox_event_id BIGINT REFERENCES outbox_events(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    total_rows BIGINT NOT NULL DEFAULT 0 CHECK (total_rows >= 0),
    valid_rows BIGINT NOT NULL DEFAULT 0 CHECK (valid_rows >= 0),
    invalid_rows BIGINT NOT NULL DEFAULT 0 CHECK (invalid_rows >= 0),
    error_sample JSONB NOT NULL DEFAULT '[]'::jsonb,
    error_summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX dataset_import_jobs_public_id_key
    ON dataset_import_jobs(public_id);

CREATE INDEX dataset_import_jobs_tenant_created_idx
    ON dataset_import_jobs(tenant_id, created_at DESC, id DESC);

CREATE INDEX dataset_import_jobs_pending_idx
    ON dataset_import_jobs(created_at, id)
    WHERE status IN ('pending', 'processing');

CREATE TABLE dataset_query_jobs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    requested_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    statement TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'completed', 'failed')),
    result_columns JSONB NOT NULL DEFAULT '[]'::jsonb,
    result_rows JSONB NOT NULL DEFAULT '[]'::jsonb,
    row_count INTEGER NOT NULL DEFAULT 0 CHECK (row_count >= 0),
    error_summary TEXT,
    duration_ms BIGINT NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CHECK (btrim(statement) <> '')
);

CREATE UNIQUE INDEX dataset_query_jobs_public_id_key
    ON dataset_query_jobs(public_id);

CREATE INDEX dataset_query_jobs_tenant_created_idx
    ON dataset_query_jobs(tenant_id, created_at DESC, id DESC);
