DROP INDEX IF EXISTS dataset_work_table_exports_schedule_created_idx;
DROP INDEX IF EXISTS dataset_work_table_export_schedules_tenant_enabled_idx;
DROP INDEX IF EXISTS dataset_work_table_export_schedules_due_idx;
DROP INDEX IF EXISTS dataset_work_table_export_schedules_work_table_created_idx;

ALTER TABLE dataset_work_table_export_schedules
    DROP CONSTRAINT IF EXISTS dataset_work_table_export_schedules_last_export_id_fkey;

ALTER TABLE dataset_work_table_exports
    DROP COLUMN IF EXISTS scheduled_for,
    DROP COLUMN IF EXISTS schedule_id;

DROP TABLE IF EXISTS dataset_work_table_export_schedules;
