CREATE TABLE medallion_assets (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    layer TEXT NOT NULL CHECK (layer IN ('bronze', 'silver', 'gold')),
    resource_kind TEXT NOT NULL CHECK (resource_kind IN (
        'drive_file',
        'dataset',
        'work_table',
        'ocr_run',
        'product_extraction',
        'gold_table'
    )),
    resource_id BIGINT NOT NULL,
    resource_public_id UUID NOT NULL,
    display_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'building', 'failed', 'skipped', 'archived')),
    row_count BIGINT CHECK (row_count IS NULL OR row_count >= 0),
    byte_size BIGINT CHECK (byte_size IS NULL OR byte_size >= 0),
    schema_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at TIMESTAMPTZ,
    CONSTRAINT medallion_assets_public_id_key UNIQUE (public_id),
    CONSTRAINT medallion_assets_resource_key UNIQUE (tenant_id, resource_kind, resource_id),
    CONSTRAINT medallion_assets_display_name_check CHECK (btrim(display_name) <> '')
);

CREATE INDEX medallion_assets_tenant_layer_updated_idx
    ON medallion_assets(tenant_id, layer, updated_at DESC, id DESC);

CREATE INDEX medallion_assets_tenant_resource_public_idx
    ON medallion_assets(tenant_id, resource_kind, resource_public_id);

CREATE TABLE medallion_asset_edges (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    source_asset_id BIGINT NOT NULL REFERENCES medallion_assets(id) ON DELETE CASCADE,
    target_asset_id BIGINT NOT NULL REFERENCES medallion_assets(id) ON DELETE CASCADE,
    relation_type TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT medallion_asset_edges_public_id_key UNIQUE (public_id),
    CONSTRAINT medallion_asset_edges_unique_key UNIQUE (tenant_id, source_asset_id, target_asset_id, relation_type),
    CONSTRAINT medallion_asset_edges_relation_type_check CHECK (btrim(relation_type) <> ''),
    CONSTRAINT medallion_asset_edges_no_self_loop_check CHECK (source_asset_id <> target_asset_id)
);

CREATE INDEX medallion_asset_edges_source_idx
    ON medallion_asset_edges(source_asset_id, created_at DESC, id DESC);

CREATE INDEX medallion_asset_edges_target_idx
    ON medallion_asset_edges(target_asset_id, created_at DESC, id DESC);

CREATE TABLE medallion_pipeline_runs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    pipeline_type TEXT NOT NULL CHECK (pipeline_type IN (
        'dataset_import',
        'work_table_register',
        'work_table_promote',
        'dataset_sync',
        'drive_ocr',
        'product_extraction',
        'gold_publish'
    )),
    run_key TEXT NOT NULL,
    source_resource_kind TEXT CHECK (source_resource_kind IS NULL OR source_resource_kind IN (
        'drive_file',
        'dataset',
        'work_table',
        'ocr_run',
        'product_extraction',
        'gold_table'
    )),
    source_resource_id BIGINT,
    source_resource_public_id UUID,
    target_resource_kind TEXT CHECK (target_resource_kind IS NULL OR target_resource_kind IN (
        'drive_file',
        'dataset',
        'work_table',
        'ocr_run',
        'product_extraction',
        'gold_table'
    )),
    target_resource_id BIGINT,
    target_resource_public_id UUID,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'skipped')),
    runtime TEXT NOT NULL DEFAULT '',
    trigger_kind TEXT NOT NULL DEFAULT 'system'
        CHECK (trigger_kind IN ('manual', 'upload', 'scheduled', 'system', 'read_repair')),
    retryable BOOLEAN NOT NULL DEFAULT false,
    error_summary TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    requested_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT medallion_pipeline_runs_public_id_key UNIQUE (public_id),
    CONSTRAINT medallion_pipeline_runs_key UNIQUE (tenant_id, pipeline_type, run_key),
    CONSTRAINT medallion_pipeline_runs_run_key_check CHECK (btrim(run_key) <> '')
);

CREATE INDEX medallion_pipeline_runs_tenant_created_idx
    ON medallion_pipeline_runs(tenant_id, created_at DESC, id DESC);

CREATE INDEX medallion_pipeline_runs_source_idx
    ON medallion_pipeline_runs(tenant_id, source_resource_kind, source_resource_id, created_at DESC)
    WHERE source_resource_kind IS NOT NULL AND source_resource_id IS NOT NULL;

CREATE INDEX medallion_pipeline_runs_target_idx
    ON medallion_pipeline_runs(tenant_id, target_resource_kind, target_resource_id, created_at DESC)
    WHERE target_resource_kind IS NOT NULL AND target_resource_id IS NOT NULL;

CREATE TABLE medallion_pipeline_run_assets (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    pipeline_run_id BIGINT NOT NULL REFERENCES medallion_pipeline_runs(id) ON DELETE CASCADE,
    asset_id BIGINT NOT NULL REFERENCES medallion_assets(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('source', 'target', 'related')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT medallion_pipeline_run_assets_unique_key UNIQUE (pipeline_run_id, asset_id, role)
);

CREATE INDEX medallion_pipeline_run_assets_asset_idx
    ON medallion_pipeline_run_assets(asset_id, created_at DESC, id DESC);
