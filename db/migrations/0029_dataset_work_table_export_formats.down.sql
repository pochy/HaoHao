ALTER TABLE dataset_work_table_exports
    DROP CONSTRAINT dataset_work_table_exports_format_check;

ALTER TABLE dataset_work_table_exports
    ADD CONSTRAINT dataset_work_table_exports_format_check
    CHECK (format IN ('csv'));
