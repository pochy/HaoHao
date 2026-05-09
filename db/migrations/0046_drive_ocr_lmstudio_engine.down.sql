UPDATE drive_ocr_runs
SET engine = 'tesseract'
WHERE engine = 'lmstudio';

ALTER TABLE drive_ocr_runs
    DROP CONSTRAINT IF EXISTS drive_ocr_runs_engine_check;

ALTER TABLE drive_ocr_runs
    ADD CONSTRAINT drive_ocr_runs_engine_check
    CHECK (engine IN ('tesseract', 'docling', 'paddleocr'));
