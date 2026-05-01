DROP TABLE IF EXISTS dataset_query_jobs;
DROP TABLE IF EXISTS dataset_import_jobs;
DROP TABLE IF EXISTS dataset_columns;
DROP TABLE IF EXISTS datasets;

UPDATE file_objects
SET purpose = 'import'
WHERE purpose = 'dataset_source';

ALTER TABLE file_objects
    DROP CONSTRAINT IF EXISTS file_objects_purpose_check;

ALTER TABLE file_objects
    ADD CONSTRAINT file_objects_purpose_check
    CHECK (purpose IN ('attachment', 'avatar', 'import', 'export', 'drive'));
