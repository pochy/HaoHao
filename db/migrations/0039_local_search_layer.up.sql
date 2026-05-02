CREATE TABLE local_search_documents (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    resource_kind TEXT NOT NULL CHECK (resource_kind IN ('drive_file', 'ocr_run', 'product_extraction', 'gold_table')),
    resource_id BIGINT NOT NULL,
    resource_public_id UUID NOT NULL,
    file_object_id BIGINT REFERENCES file_objects(id) ON DELETE CASCADE,
    medallion_asset_id BIGINT REFERENCES medallion_assets(id) ON DELETE SET NULL,
    gold_publication_id BIGINT REFERENCES dataset_gold_publications(id) ON DELETE CASCADE,
    title TEXT NOT NULL DEFAULT '',
    body_text TEXT NOT NULL DEFAULT '',
    snippet TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    source_updated_at TIMESTAMPTZ,
    indexed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    search_vector TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(body_text, '')), 'B')
    ) STORED,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT local_search_documents_public_id_key UNIQUE (public_id),
    CONSTRAINT local_search_documents_resource_key UNIQUE (tenant_id, resource_kind, resource_id),
    CONSTRAINT local_search_documents_title_check CHECK (btrim(title) <> '')
);

CREATE INDEX local_search_documents_tenant_kind_idx
    ON local_search_documents(tenant_id, resource_kind, indexed_at DESC, id DESC);

CREATE INDEX local_search_documents_file_idx
    ON local_search_documents(tenant_id, file_object_id, indexed_at DESC)
    WHERE file_object_id IS NOT NULL;

CREATE INDEX local_search_documents_gold_publication_idx
    ON local_search_documents(tenant_id, gold_publication_id, indexed_at DESC)
    WHERE gold_publication_id IS NOT NULL;

CREATE INDEX local_search_documents_medallion_asset_idx
    ON local_search_documents(medallion_asset_id)
    WHERE medallion_asset_id IS NOT NULL;

CREATE INDEX local_search_documents_vector_idx
    ON local_search_documents USING gin(search_vector);

CREATE INDEX medallion_assets_local_search_idx
    ON medallion_assets USING gin(
        to_tsvector('simple', display_name || ' ' || metadata::text || ' ' || schema_summary::text)
    );

CREATE TABLE local_search_index_jobs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    resource_kind TEXT CHECK (resource_kind IS NULL OR resource_kind IN ('drive_file', 'ocr_run', 'product_extraction', 'gold_table')),
    resource_id BIGINT,
    resource_public_id UUID,
    outbox_event_id BIGINT REFERENCES outbox_events(id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT 'index_requested',
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'processing', 'completed', 'failed', 'skipped')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    indexed_count INTEGER NOT NULL DEFAULT 0 CHECK (indexed_count >= 0),
    skipped_count INTEGER NOT NULL DEFAULT 0 CHECK (skipped_count >= 0),
    failed_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
    last_error TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT local_search_index_jobs_public_id_key UNIQUE (public_id),
    CONSTRAINT local_search_index_jobs_reason_check CHECK (btrim(reason) <> '')
);

CREATE INDEX local_search_index_jobs_tenant_status_idx
    ON local_search_index_jobs(tenant_id, status, created_at DESC, id DESC);

CREATE INDEX local_search_index_jobs_resource_idx
    ON local_search_index_jobs(tenant_id, resource_kind, resource_id, created_at DESC)
    WHERE resource_kind IS NOT NULL AND resource_id IS NOT NULL;

CREATE INDEX local_search_index_jobs_outbox_event_idx
    ON local_search_index_jobs(outbox_event_id)
    WHERE outbox_event_id IS NOT NULL;

CREATE TABLE local_search_embeddings (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    document_id BIGINT NOT NULL REFERENCES local_search_documents(id) ON DELETE CASCADE,
    chunk_ordinal INTEGER NOT NULL CHECK (chunk_ordinal >= 0),
    source_text TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    dimension INTEGER NOT NULL DEFAULT 0 CHECK (dimension >= 0),
    content_hash TEXT NOT NULL DEFAULT '',
    embedding DOUBLE PRECISION[],
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'completed', 'failed', 'skipped')),
    error_summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT local_search_embeddings_public_id_key UNIQUE (public_id),
    CONSTRAINT local_search_embeddings_chunk_key UNIQUE (document_id, chunk_ordinal, model, content_hash)
);

CREATE INDEX local_search_embeddings_document_idx
    ON local_search_embeddings(document_id, chunk_ordinal);

CREATE INDEX local_search_embeddings_status_idx
    ON local_search_embeddings(status, created_at DESC, id DESC);
