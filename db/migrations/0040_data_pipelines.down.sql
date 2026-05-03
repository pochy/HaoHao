ALTER TABLE data_pipeline_runs
    DROP CONSTRAINT IF EXISTS data_pipeline_runs_schedule_id_fkey;

DROP TABLE IF EXISTS data_pipeline_schedules;
DROP TABLE IF EXISTS data_pipeline_run_steps;
DROP TABLE IF EXISTS data_pipeline_runs;

ALTER TABLE data_pipelines
    DROP CONSTRAINT IF EXISTS data_pipelines_published_version_id_fkey;

DROP TABLE IF EXISTS data_pipeline_versions;
DROP TABLE IF EXISTS data_pipelines;

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
        'gold_publish'
    ));

DELETE FROM tenant_role_overrides
WHERE role_id IN (
    SELECT id
    FROM roles
    WHERE code = 'data_pipeline_user'
);

DELETE FROM tenant_memberships
WHERE role_id IN (
    SELECT id
    FROM roles
    WHERE code = 'data_pipeline_user'
);

DELETE FROM user_roles
WHERE role_id IN (
    SELECT id
    FROM roles
    WHERE code = 'data_pipeline_user'
);

DELETE FROM roles
WHERE code = 'data_pipeline_user';
