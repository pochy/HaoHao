ALTER TABLE drive_ocr_runs
    ADD COLUMN IF NOT EXISTS artifact_schema_version TEXT NOT NULL DEFAULT 'drive_image_pdf_v1',
    ADD COLUMN IF NOT EXISTS pipeline_config_hash TEXT NOT NULL DEFAULT '';

UPDATE drive_ocr_runs
SET pipeline_config_hash = 'legacy:' || engine || ':' || structured_extractor
WHERE btrim(pipeline_config_hash) = '';

ALTER TABLE drive_ocr_runs
    ALTER COLUMN pipeline_config_hash SET DEFAULT 'legacy';

ALTER TABLE drive_ocr_runs
    ADD CONSTRAINT drive_ocr_runs_artifact_schema_version_check
    CHECK (btrim(artifact_schema_version) <> ''),
    ADD CONSTRAINT drive_ocr_runs_pipeline_config_hash_check
    CHECK (btrim(pipeline_config_hash) <> '');

DROP INDEX IF EXISTS drive_ocr_runs_file_revision_provider_key;

CREATE UNIQUE INDEX drive_ocr_runs_file_revision_provider_key
    ON drive_ocr_runs(file_object_id, file_revision, content_sha256, engine, structured_extractor, pipeline_config_hash);
