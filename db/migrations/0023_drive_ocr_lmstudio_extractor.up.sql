ALTER TABLE drive_ocr_runs
    DROP CONSTRAINT IF EXISTS drive_ocr_runs_structured_extractor_check;

ALTER TABLE drive_ocr_runs
    ADD CONSTRAINT drive_ocr_runs_structured_extractor_check
    CHECK (structured_extractor IN ('rules', 'ollama', 'lmstudio', 'docling'));
