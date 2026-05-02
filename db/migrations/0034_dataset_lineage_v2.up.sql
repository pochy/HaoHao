CREATE TABLE dataset_lineage_change_sets (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    query_job_id BIGINT REFERENCES dataset_query_jobs(id) ON DELETE SET NULL,
    root_resource_type TEXT NOT NULL,
    root_resource_public_id UUID,
    source_kind TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    published_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    rejected_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    rejected_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,
    CONSTRAINT dataset_lineage_change_sets_public_id_key UNIQUE (public_id),
    CONSTRAINT dataset_lineage_change_sets_source_kind_check CHECK (source_kind IN ('parser', 'manual')),
    CONSTRAINT dataset_lineage_change_sets_status_check CHECK (status IN ('draft', 'published', 'rejected', 'archived')),
    CONSTRAINT dataset_lineage_change_sets_root_resource_type_check CHECK (btrim(root_resource_type) <> ''),
    CONSTRAINT dataset_lineage_change_sets_title_check CHECK (btrim(title) <> '')
);

CREATE TABLE dataset_lineage_nodes (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    change_set_id BIGINT NOT NULL REFERENCES dataset_lineage_change_sets(id) ON DELETE CASCADE,
    node_key TEXT NOT NULL,
    node_kind TEXT NOT NULL,
    source_kind TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_public_id UUID,
    parent_node_key TEXT,
    column_name TEXT,
    label TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    position_x DOUBLE PRECISION,
    position_y DOUBLE PRECISION,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT dataset_lineage_nodes_public_id_key UNIQUE (public_id),
    CONSTRAINT dataset_lineage_nodes_change_set_node_key_key UNIQUE (change_set_id, node_key),
    CONSTRAINT dataset_lineage_nodes_node_kind_check CHECK (node_kind IN ('resource', 'column', 'custom')),
    CONSTRAINT dataset_lineage_nodes_source_kind_check CHECK (source_kind IN ('parser', 'manual')),
    CONSTRAINT dataset_lineage_nodes_node_key_check CHECK (btrim(node_key) <> ''),
    CONSTRAINT dataset_lineage_nodes_resource_type_check CHECK (btrim(resource_type) <> ''),
    CONSTRAINT dataset_lineage_nodes_label_check CHECK (btrim(label) <> ''),
    CONSTRAINT dataset_lineage_nodes_resource_shape_check CHECK (
        (node_kind = 'resource' AND resource_public_id IS NOT NULL)
        OR (node_kind = 'column' AND column_name IS NOT NULL AND btrim(column_name) <> '')
        OR (node_kind = 'custom' AND resource_public_id IS NULL)
    )
);

CREATE TABLE dataset_lineage_edges (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    change_set_id BIGINT NOT NULL REFERENCES dataset_lineage_change_sets(id) ON DELETE CASCADE,
    edge_key TEXT NOT NULL,
    source_node_key TEXT NOT NULL,
    target_node_key TEXT NOT NULL,
    relation_type TEXT NOT NULL,
    source_kind TEXT NOT NULL,
    confidence TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    expression TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT dataset_lineage_edges_public_id_key UNIQUE (public_id),
    CONSTRAINT dataset_lineage_edges_change_set_edge_key_key UNIQUE (change_set_id, edge_key),
    CONSTRAINT dataset_lineage_edges_source_node_fkey FOREIGN KEY (change_set_id, source_node_key) REFERENCES dataset_lineage_nodes(change_set_id, node_key) ON DELETE CASCADE,
    CONSTRAINT dataset_lineage_edges_target_node_fkey FOREIGN KEY (change_set_id, target_node_key) REFERENCES dataset_lineage_nodes(change_set_id, node_key) ON DELETE CASCADE,
    CONSTRAINT dataset_lineage_edges_source_kind_check CHECK (source_kind IN ('parser', 'manual')),
    CONSTRAINT dataset_lineage_edges_confidence_check CHECK (confidence IN ('parser_exact', 'parser_partial', 'manual')),
    CONSTRAINT dataset_lineage_edges_edge_key_check CHECK (btrim(edge_key) <> ''),
    CONSTRAINT dataset_lineage_edges_relation_type_check CHECK (btrim(relation_type) <> ''),
    CONSTRAINT dataset_lineage_edges_no_self_loop_check CHECK (source_node_key <> target_node_key)
);

CREATE TABLE dataset_lineage_parse_runs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    query_job_id BIGINT NOT NULL REFERENCES dataset_query_jobs(id) ON DELETE CASCADE,
    change_set_id BIGINT REFERENCES dataset_lineage_change_sets(id) ON DELETE SET NULL,
    requested_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'processing',
    table_ref_count INTEGER NOT NULL DEFAULT 0 CHECK (table_ref_count >= 0),
    column_edge_count INTEGER NOT NULL DEFAULT 0 CHECK (column_edge_count >= 0),
    error_summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT dataset_lineage_parse_runs_public_id_key UNIQUE (public_id),
    CONSTRAINT dataset_lineage_parse_runs_status_check CHECK (status IN ('processing', 'completed', 'failed'))
);

CREATE INDEX dataset_lineage_change_sets_tenant_status_idx
    ON dataset_lineage_change_sets(tenant_id, status, updated_at DESC, id DESC);

CREATE INDEX dataset_lineage_change_sets_root_idx
    ON dataset_lineage_change_sets(tenant_id, root_resource_type, root_resource_public_id, status, updated_at DESC, id DESC);

CREATE INDEX dataset_lineage_nodes_change_set_idx
    ON dataset_lineage_nodes(change_set_id, id);

CREATE INDEX dataset_lineage_nodes_resource_idx
    ON dataset_lineage_nodes(tenant_id, resource_type, resource_public_id)
    WHERE resource_public_id IS NOT NULL;

CREATE INDEX dataset_lineage_edges_change_set_idx
    ON dataset_lineage_edges(change_set_id, id);

CREATE INDEX dataset_lineage_parse_runs_query_job_idx
    ON dataset_lineage_parse_runs(tenant_id, query_job_id, created_at DESC, id DESC);
