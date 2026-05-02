ALTER TABLE dataset_query_jobs
    ADD COLUMN dataset_id BIGINT REFERENCES datasets(id) ON DELETE SET NULL;

CREATE INDEX dataset_query_jobs_tenant_dataset_created_idx
    ON dataset_query_jobs(tenant_id, dataset_id, created_at DESC, id DESC)
    WHERE dataset_id IS NOT NULL;
