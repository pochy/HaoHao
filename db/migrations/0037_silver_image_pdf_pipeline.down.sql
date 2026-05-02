DROP INDEX IF EXISTS drive_ocr_runs_file_revision_provider_key;

CREATE UNIQUE INDEX drive_ocr_runs_file_revision_provider_key
    ON drive_ocr_runs(file_object_id, file_revision, content_sha256, engine, structured_extractor);

ALTER TABLE drive_ocr_runs
    DROP CONSTRAINT IF EXISTS drive_ocr_runs_pipeline_config_hash_check,
    DROP CONSTRAINT IF EXISTS drive_ocr_runs_artifact_schema_version_check,
    DROP COLUMN IF EXISTS pipeline_config_hash,
    DROP COLUMN IF EXISTS artifact_schema_version;
