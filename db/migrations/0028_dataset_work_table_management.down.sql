DROP TABLE IF EXISTS dataset_work_table_exports;

ALTER TABLE datasets
    DROP CONSTRAINT IF EXISTS datasets_work_table_source_check,
    DROP CONSTRAINT IF EXISTS datasets_file_source_check,
    DROP CONSTRAINT IF EXISTS datasets_source_work_table_id_fkey;

DROP INDEX IF EXISTS datasets_source_work_table_idx;
DROP INDEX IF EXISTS dataset_work_tables_tenant_dataset_idx;
DROP INDEX IF EXISTS dataset_work_tables_tenant_updated_idx;
DROP INDEX IF EXISTS dataset_work_tables_active_table_key;
DROP INDEX IF EXISTS dataset_work_tables_public_id_key;

DROP TABLE IF EXISTS dataset_work_tables;

ALTER TABLE datasets
    DROP CONSTRAINT IF EXISTS datasets_source_kind_check,
    DROP COLUMN IF EXISTS source_work_table_id,
    DROP COLUMN IF EXISTS source_kind,
    ALTER COLUMN source_file_object_id SET NOT NULL;
