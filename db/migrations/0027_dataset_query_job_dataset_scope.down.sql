DROP INDEX IF EXISTS dataset_query_jobs_tenant_dataset_created_idx;

ALTER TABLE dataset_query_jobs
    DROP COLUMN IF EXISTS dataset_id;
