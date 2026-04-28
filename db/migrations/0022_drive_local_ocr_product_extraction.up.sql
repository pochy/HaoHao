CREATE TABLE drive_ocr_runs (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    file_revision TEXT NOT NULL DEFAULT '1',
    content_sha256 TEXT NOT NULL DEFAULT '',
    engine TEXT NOT NULL,
    languages TEXT[] NOT NULL DEFAULT ARRAY['jpn', 'eng'],
    structured_extractor TEXT NOT NULL DEFAULT 'rules',
    status TEXT NOT NULL DEFAULT 'pending',
    reason TEXT NOT NULL DEFAULT 'upload',
    page_count INTEGER NOT NULL DEFAULT 0,
    processed_page_count INTEGER NOT NULL DEFAULT 0,
    average_confidence NUMERIC(5,4),
    extracted_text TEXT NOT NULL DEFAULT '',
    error_code TEXT,
    error_message TEXT,
    requested_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    outbox_event_id BIGINT REFERENCES outbox_events(id) ON DELETE SET NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_ocr_runs_engine_check CHECK (engine IN ('tesseract', 'docling', 'paddleocr')),
    CONSTRAINT drive_ocr_runs_structured_extractor_check CHECK (structured_extractor IN ('rules', 'ollama', 'docling')),
    CONSTRAINT drive_ocr_runs_status_check CHECK (status IN ('pending', 'running', 'completed', 'failed', 'skipped')),
    CONSTRAINT drive_ocr_runs_page_count_check CHECK (page_count >= 0),
    CONSTRAINT drive_ocr_runs_processed_page_count_check CHECK (processed_page_count >= 0)
);

CREATE UNIQUE INDEX drive_ocr_runs_public_id_key
    ON drive_ocr_runs(public_id);
CREATE UNIQUE INDEX drive_ocr_runs_file_revision_provider_key
    ON drive_ocr_runs(file_object_id, file_revision, content_sha256, engine, structured_extractor);
CREATE INDEX drive_ocr_runs_pending_idx
    ON drive_ocr_runs(tenant_id, created_at, id)
    WHERE status IN ('pending', 'running');
CREATE INDEX drive_ocr_runs_file_idx
    ON drive_ocr_runs(tenant_id, file_object_id, created_at DESC);
CREATE INDEX drive_ocr_runs_status_idx
    ON drive_ocr_runs(tenant_id, status, created_at DESC);

CREATE TABLE drive_ocr_pages (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    ocr_run_id BIGINT NOT NULL REFERENCES drive_ocr_runs(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    page_number INTEGER NOT NULL,
    raw_text TEXT NOT NULL DEFAULT '',
    average_confidence NUMERIC(5,4),
    layout_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    boxes_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_ocr_pages_page_number_check CHECK (page_number > 0)
);

CREATE UNIQUE INDEX drive_ocr_pages_run_page_key
    ON drive_ocr_pages(ocr_run_id, page_number);
CREATE INDEX drive_ocr_pages_file_idx
    ON drive_ocr_pages(tenant_id, file_object_id, page_number);

CREATE TABLE drive_product_extraction_items (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    ocr_run_id BIGINT NOT NULL REFERENCES drive_ocr_runs(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    item_type TEXT NOT NULL,
    name TEXT NOT NULL,
    brand TEXT,
    manufacturer TEXT,
    model TEXT,
    sku TEXT,
    jan_code TEXT,
    category TEXT,
    description TEXT,
    price JSONB NOT NULL DEFAULT '{}'::jsonb,
    promotion JSONB NOT NULL DEFAULT '{}'::jsonb,
    availability JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_text TEXT NOT NULL DEFAULT '',
    evidence JSONB NOT NULL DEFAULT '[]'::jsonb,
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    confidence NUMERIC(5,4),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_product_extraction_items_item_type_check CHECK (item_type IN ('product', 'promotion', 'bundle', 'unknown'))
);

CREATE UNIQUE INDEX drive_product_extraction_items_public_id_key
    ON drive_product_extraction_items(public_id);
CREATE INDEX drive_product_extraction_items_file_idx
    ON drive_product_extraction_items(tenant_id, file_object_id, created_at DESC);
CREATE INDEX drive_product_extraction_items_run_idx
    ON drive_product_extraction_items(ocr_run_id, id);
CREATE INDEX drive_product_extraction_items_jan_code_idx
    ON drive_product_extraction_items(tenant_id, jan_code)
    WHERE jan_code IS NOT NULL;
