UPDATE drive_ocr_runs
SET structured_extractor = 'rules'
WHERE structured_extractor IN ('python', 'ginza', 'sudachipy');

ALTER TABLE drive_ocr_runs
    DROP CONSTRAINT IF EXISTS drive_ocr_runs_structured_extractor_check;

ALTER TABLE drive_ocr_runs
    ADD CONSTRAINT drive_ocr_runs_structured_extractor_check
    CHECK (structured_extractor IN ('rules', 'ollama', 'lmstudio', 'gemini', 'codex', 'claude', 'docling'));
