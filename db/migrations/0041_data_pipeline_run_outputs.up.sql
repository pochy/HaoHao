CREATE TABLE data_pipeline_run_outputs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    run_id BIGINT NOT NULL REFERENCES data_pipeline_runs(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'skipped')),
    output_work_table_id BIGINT REFERENCES dataset_work_tables(id) ON DELETE SET NULL,
    row_count BIGINT NOT NULL DEFAULT 0 CHECK (row_count >= 0),
    error_summary TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT data_pipeline_run_outputs_node_id_check CHECK (btrim(node_id) <> ''),
    CONSTRAINT data_pipeline_run_outputs_run_node_key UNIQUE (run_id, node_id)
);

CREATE INDEX data_pipeline_run_outputs_run_idx
    ON data_pipeline_run_outputs(run_id, id);
