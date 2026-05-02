CREATE TABLE dataset_gold_publications (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    source_work_table_id BIGINT NOT NULL REFERENCES dataset_work_tables(id) ON DELETE CASCADE,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    published_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    unpublished_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    archived_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    last_publish_run_id BIGINT,
    display_name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    gold_database TEXT NOT NULL,
    gold_table TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    row_count BIGINT NOT NULL DEFAULT 0 CHECK (row_count >= 0),
    total_bytes BIGINT NOT NULL DEFAULT 0 CHECK (total_bytes >= 0),
    schema_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    refresh_policy TEXT NOT NULL DEFAULT 'manual',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    unpublished_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,
    CONSTRAINT dataset_gold_publications_public_id_key UNIQUE (public_id),
    CONSTRAINT dataset_gold_publications_display_name_check CHECK (btrim(display_name) <> ''),
    CONSTRAINT dataset_gold_publications_gold_database_check CHECK (btrim(gold_database) <> ''),
    CONSTRAINT dataset_gold_publications_gold_table_check CHECK (btrim(gold_table) <> ''),
    CONSTRAINT dataset_gold_publications_status_check CHECK (status IN ('pending', 'active', 'failed', 'unpublished', 'archived')),
    CONSTRAINT dataset_gold_publications_refresh_policy_check CHECK (refresh_policy IN ('manual'))
);

CREATE TABLE dataset_gold_publish_runs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    publication_id BIGINT NOT NULL REFERENCES dataset_gold_publications(id) ON DELETE CASCADE,
    source_work_table_id BIGINT NOT NULL REFERENCES dataset_work_tables(id) ON DELETE CASCADE,
    requested_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    outbox_event_id BIGINT REFERENCES outbox_events(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    gold_database TEXT NOT NULL,
    gold_table TEXT NOT NULL,
    internal_database TEXT NOT NULL,
    internal_table TEXT NOT NULL,
    row_count BIGINT NOT NULL DEFAULT 0 CHECK (row_count >= 0),
    total_bytes BIGINT NOT NULL DEFAULT 0 CHECK (total_bytes >= 0),
    schema_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_summary TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT dataset_gold_publish_runs_public_id_key UNIQUE (public_id),
    CONSTRAINT dataset_gold_publish_runs_status_check CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    CONSTRAINT dataset_gold_publish_runs_gold_database_check CHECK (btrim(gold_database) <> ''),
    CONSTRAINT dataset_gold_publish_runs_gold_table_check CHECK (btrim(gold_table) <> ''),
    CONSTRAINT dataset_gold_publish_runs_internal_database_check CHECK (btrim(internal_database) <> ''),
    CONSTRAINT dataset_gold_publish_runs_internal_table_check CHECK (btrim(internal_table) <> '')
);

ALTER TABLE dataset_gold_publications
    ADD CONSTRAINT dataset_gold_publications_last_publish_run_id_fkey
    FOREIGN KEY (last_publish_run_id) REFERENCES dataset_gold_publish_runs(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX dataset_gold_publications_active_table_key
    ON dataset_gold_publications(tenant_id, gold_database, gold_table)
    WHERE archived_at IS NULL;

CREATE INDEX dataset_gold_publications_source_work_table_idx
    ON dataset_gold_publications(source_work_table_id, updated_at DESC, id DESC);

CREATE INDEX dataset_gold_publications_tenant_status_updated_idx
    ON dataset_gold_publications(tenant_id, status, updated_at DESC, id DESC);

CREATE INDEX dataset_gold_publications_last_publish_run_idx
    ON dataset_gold_publications(last_publish_run_id)
    WHERE last_publish_run_id IS NOT NULL;

CREATE UNIQUE INDEX dataset_gold_publish_runs_active_publication_key
    ON dataset_gold_publish_runs(publication_id)
    WHERE status IN ('pending', 'processing');

CREATE INDEX dataset_gold_publish_runs_publication_created_idx
    ON dataset_gold_publish_runs(publication_id, created_at DESC, id DESC);

CREATE INDEX dataset_gold_publish_runs_source_work_table_idx
    ON dataset_gold_publish_runs(source_work_table_id, created_at DESC, id DESC);

CREATE INDEX dataset_gold_publish_runs_outbox_event_idx
    ON dataset_gold_publish_runs(outbox_event_id)
    WHERE outbox_event_id IS NOT NULL;

CREATE INDEX dataset_gold_publish_runs_tenant_status_created_idx
    ON dataset_gold_publish_runs(tenant_id, status, created_at DESC, id DESC);
