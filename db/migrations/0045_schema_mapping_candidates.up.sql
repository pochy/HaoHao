CREATE TABLE data_pipeline_schema_columns (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    domain TEXT NOT NULL DEFAULT '',
    schema_type TEXT NOT NULL DEFAULT '',
    target_column TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    aliases JSONB NOT NULL DEFAULT '[]'::jsonb,
    examples JSONB NOT NULL DEFAULT '[]'::jsonb,
    language TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT data_pipeline_schema_columns_public_id_key UNIQUE (public_id),
    CONSTRAINT data_pipeline_schema_columns_target_column_check CHECK (btrim(target_column) <> ''),
    CONSTRAINT data_pipeline_schema_columns_aliases_array_check CHECK (jsonb_typeof(aliases) = 'array'),
    CONSTRAINT data_pipeline_schema_columns_examples_array_check CHECK (jsonb_typeof(examples) = 'array'),
    CONSTRAINT data_pipeline_schema_columns_key UNIQUE (tenant_id, domain, schema_type, target_column, version)
);

CREATE INDEX data_pipeline_schema_columns_lookup_idx
    ON data_pipeline_schema_columns(tenant_id, domain, schema_type, target_column, version)
    WHERE archived_at IS NULL;

CREATE TABLE data_pipeline_mapping_examples (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    pipeline_id BIGINT NOT NULL REFERENCES data_pipelines(id) ON DELETE CASCADE,
    version_id BIGINT REFERENCES data_pipeline_versions(id) ON DELETE SET NULL,
    schema_column_id BIGINT NOT NULL REFERENCES data_pipeline_schema_columns(id) ON DELETE RESTRICT,
    source_column TEXT NOT NULL,
    sheet_name TEXT NOT NULL DEFAULT '',
    sample_values JSONB NOT NULL DEFAULT '[]'::jsonb,
    neighbor_columns JSONB NOT NULL DEFAULT '[]'::jsonb,
    decision TEXT NOT NULL CHECK (decision IN ('accepted', 'rejected')),
    decided_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    decided_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    shared_scope TEXT NOT NULL DEFAULT 'private' CHECK (shared_scope IN ('private', 'tenant')),
    shared_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    shared_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT data_pipeline_mapping_examples_public_id_key UNIQUE (public_id),
    CONSTRAINT data_pipeline_mapping_examples_source_column_check CHECK (btrim(source_column) <> ''),
    CONSTRAINT data_pipeline_mapping_examples_sample_values_array_check CHECK (jsonb_typeof(sample_values) = 'array'),
    CONSTRAINT data_pipeline_mapping_examples_neighbor_columns_array_check CHECK (jsonb_typeof(neighbor_columns) = 'array'),
    CONSTRAINT data_pipeline_mapping_examples_share_shape_check CHECK (
        (shared_scope = 'private' AND shared_at IS NULL)
        OR (shared_scope = 'tenant' AND shared_at IS NOT NULL)
    )
);

CREATE UNIQUE INDEX data_pipeline_mapping_examples_unique_version_idx
    ON data_pipeline_mapping_examples(tenant_id, pipeline_id, version_id, source_column, schema_column_id, decision)
    WHERE version_id IS NOT NULL;

CREATE UNIQUE INDEX data_pipeline_mapping_examples_unique_null_version_idx
    ON data_pipeline_mapping_examples(tenant_id, pipeline_id, source_column, schema_column_id, decision)
    WHERE version_id IS NULL;

CREATE INDEX data_pipeline_mapping_examples_schema_column_idx
    ON data_pipeline_mapping_examples(tenant_id, schema_column_id, decision, decided_at DESC);

CREATE INDEX data_pipeline_mapping_examples_pipeline_idx
    ON data_pipeline_mapping_examples(tenant_id, pipeline_id, decided_at DESC);
