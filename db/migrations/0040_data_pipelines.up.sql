INSERT INTO roles (code)
VALUES ('data_pipeline_user')
ON CONFLICT (code) DO NOTHING;

ALTER TABLE medallion_pipeline_runs
    DROP CONSTRAINT IF EXISTS medallion_pipeline_runs_pipeline_type_check;

ALTER TABLE medallion_pipeline_runs
    ADD CONSTRAINT medallion_pipeline_runs_pipeline_type_check CHECK (pipeline_type IN (
        'dataset_import',
        'work_table_register',
        'work_table_promote',
        'dataset_sync',
        'drive_ocr',
        'product_extraction',
        'gold_publish',
        'data_pipeline'
    ));

CREATE TABLE data_pipelines (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'published', 'archived')),
    published_version_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at TIMESTAMPTZ,
    CONSTRAINT data_pipelines_public_id_key UNIQUE (public_id),
    CONSTRAINT data_pipelines_name_check CHECK (btrim(name) <> '')
);

CREATE INDEX data_pipelines_tenant_updated_idx
    ON data_pipelines(tenant_id, updated_at DESC, id DESC)
    WHERE archived_at IS NULL;

CREATE TABLE data_pipeline_versions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    pipeline_id BIGINT NOT NULL REFERENCES data_pipelines(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL CHECK (version_number > 0),
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'published', 'archived')),
    graph JSONB NOT NULL,
    validation_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    published_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    CONSTRAINT data_pipeline_versions_public_id_key UNIQUE (public_id),
    CONSTRAINT data_pipeline_versions_graph_object_check CHECK (jsonb_typeof(graph) = 'object'),
    CONSTRAINT data_pipeline_versions_pipeline_version_key UNIQUE (pipeline_id, version_number)
);

ALTER TABLE data_pipelines
    ADD CONSTRAINT data_pipelines_published_version_id_fkey
    FOREIGN KEY (published_version_id) REFERENCES data_pipeline_versions(id) ON DELETE SET NULL;

CREATE INDEX data_pipeline_versions_pipeline_created_idx
    ON data_pipeline_versions(pipeline_id, created_at DESC, id DESC);

CREATE INDEX data_pipeline_versions_tenant_status_idx
    ON data_pipeline_versions(tenant_id, status, created_at DESC, id DESC);

CREATE TABLE data_pipeline_runs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    pipeline_id BIGINT NOT NULL REFERENCES data_pipelines(id) ON DELETE CASCADE,
    version_id BIGINT NOT NULL REFERENCES data_pipeline_versions(id) ON DELETE RESTRICT,
    schedule_id BIGINT,
    requested_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    trigger_kind TEXT NOT NULL DEFAULT 'manual'
        CHECK (trigger_kind IN ('manual', 'scheduled')),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'skipped')),
    output_work_table_id BIGINT REFERENCES dataset_work_tables(id) ON DELETE SET NULL,
    outbox_event_id BIGINT REFERENCES outbox_events(id) ON DELETE SET NULL,
    row_count BIGINT NOT NULL DEFAULT 0 CHECK (row_count >= 0),
    error_summary TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT data_pipeline_runs_public_id_key UNIQUE (public_id)
);

CREATE INDEX data_pipeline_runs_pipeline_created_idx
    ON data_pipeline_runs(pipeline_id, created_at DESC, id DESC);

CREATE INDEX data_pipeline_runs_active_idx
    ON data_pipeline_runs(tenant_id, pipeline_id, created_at DESC, id DESC)
    WHERE status IN ('pending', 'processing');

CREATE INDEX data_pipeline_runs_schedule_active_idx
    ON data_pipeline_runs(tenant_id, schedule_id)
    WHERE schedule_id IS NOT NULL
      AND status IN ('pending', 'processing');

CREATE TABLE data_pipeline_run_steps (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    run_id BIGINT NOT NULL REFERENCES data_pipeline_runs(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL,
    step_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'skipped')),
    row_count BIGINT NOT NULL DEFAULT 0 CHECK (row_count >= 0),
    error_summary TEXT,
    error_sample JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT data_pipeline_run_steps_node_id_check CHECK (btrim(node_id) <> ''),
    CONSTRAINT data_pipeline_run_steps_run_node_key UNIQUE (run_id, node_id)
);

CREATE INDEX data_pipeline_run_steps_run_idx
    ON data_pipeline_run_steps(run_id, id);

CREATE TABLE data_pipeline_schedules (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    pipeline_id BIGINT NOT NULL REFERENCES data_pipelines(id) ON DELETE CASCADE,
    version_id BIGINT NOT NULL REFERENCES data_pipeline_versions(id) ON DELETE RESTRICT,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    frequency TEXT NOT NULL CHECK (frequency IN ('daily', 'weekly', 'monthly')),
    timezone TEXT NOT NULL,
    run_time TEXT NOT NULL CHECK (run_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    weekday SMALLINT CHECK (weekday IS NULL OR weekday BETWEEN 1 AND 7),
    month_day SMALLINT CHECK (month_day IS NULL OR month_day BETWEEN 1 AND 28),
    enabled BOOLEAN NOT NULL DEFAULT true,
    next_run_at TIMESTAMPTZ NOT NULL,
    last_run_at TIMESTAMPTZ,
    last_status TEXT,
    last_error_summary TEXT,
    last_run_id BIGINT REFERENCES data_pipeline_runs(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT data_pipeline_schedules_public_id_key UNIQUE (public_id),
    CONSTRAINT data_pipeline_schedules_frequency_shape_check CHECK (
        (frequency = 'daily' AND weekday IS NULL AND month_day IS NULL)
        OR (frequency = 'weekly' AND weekday IS NOT NULL AND month_day IS NULL)
        OR (frequency = 'monthly' AND weekday IS NULL AND month_day IS NOT NULL)
    )
);

ALTER TABLE data_pipeline_runs
    ADD CONSTRAINT data_pipeline_runs_schedule_id_fkey
    FOREIGN KEY (schedule_id) REFERENCES data_pipeline_schedules(id) ON DELETE SET NULL;

CREATE INDEX data_pipeline_schedules_due_idx
    ON data_pipeline_schedules(next_run_at, id)
    WHERE enabled;

CREATE INDEX data_pipeline_schedules_pipeline_idx
    ON data_pipeline_schedules(pipeline_id, updated_at DESC, id DESC);
