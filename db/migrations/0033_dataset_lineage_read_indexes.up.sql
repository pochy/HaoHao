CREATE INDEX dataset_work_tables_tenant_query_job_idx
    ON dataset_work_tables(tenant_id, created_from_query_job_id, updated_at DESC, id DESC)
    WHERE created_from_query_job_id IS NOT NULL;

CREATE INDEX dataset_sync_jobs_source_work_table_created_idx
    ON dataset_sync_jobs(source_work_table_id, created_at DESC, id DESC);
