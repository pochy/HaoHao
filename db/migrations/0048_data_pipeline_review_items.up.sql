CREATE TABLE data_pipeline_review_items (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    pipeline_id BIGINT NOT NULL REFERENCES data_pipelines(id) ON DELETE CASCADE,
    version_id BIGINT NOT NULL REFERENCES data_pipeline_versions(id) ON DELETE RESTRICT,
    run_id BIGINT NOT NULL REFERENCES data_pipeline_runs(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL,
    queue TEXT NOT NULL DEFAULT 'default',
    status TEXT NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'approved', 'rejected', 'needs_changes', 'closed')),
    reason JSONB NOT NULL DEFAULT '[]'::jsonb,
    source_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_fingerprint TEXT NOT NULL,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    assigned_to_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    decision_comment TEXT NOT NULL DEFAULT '',
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT data_pipeline_review_items_public_id_key UNIQUE (public_id),
    CONSTRAINT data_pipeline_review_items_source_key UNIQUE (tenant_id, run_id, node_id, source_fingerprint),
    CONSTRAINT data_pipeline_review_items_node_id_check CHECK (btrim(node_id) <> ''),
    CONSTRAINT data_pipeline_review_items_queue_check CHECK (btrim(queue) <> ''),
    CONSTRAINT data_pipeline_review_items_source_fingerprint_check CHECK (btrim(source_fingerprint) <> '')
);

CREATE INDEX data_pipeline_review_items_pipeline_status_idx
    ON data_pipeline_review_items(tenant_id, pipeline_id, status, created_at DESC, id DESC);

CREATE INDEX data_pipeline_review_items_run_idx
    ON data_pipeline_review_items(tenant_id, run_id, node_id, id DESC);

CREATE TABLE data_pipeline_review_item_comments (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    review_item_id BIGINT NOT NULL REFERENCES data_pipeline_review_items(id) ON DELETE CASCADE,
    author_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT data_pipeline_review_item_comments_public_id_key UNIQUE (public_id),
    CONSTRAINT data_pipeline_review_item_comments_body_check CHECK (btrim(body) <> '')
);

CREATE INDEX data_pipeline_review_item_comments_item_idx
    ON data_pipeline_review_item_comments(tenant_id, review_item_id, created_at ASC, id ASC);
