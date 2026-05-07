CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE local_search_documents
    DROP CONSTRAINT local_search_documents_resource_kind_check,
    ADD CONSTRAINT local_search_documents_resource_kind_check
        CHECK (resource_kind IN ('drive_file', 'ocr_run', 'product_extraction', 'gold_table', 'schema_column', 'mapping_example'));

ALTER TABLE local_search_index_jobs
    DROP CONSTRAINT local_search_index_jobs_resource_kind_check,
    ADD CONSTRAINT local_search_index_jobs_resource_kind_check
        CHECK (resource_kind IS NULL OR resource_kind IN ('drive_file', 'ocr_run', 'product_extraction', 'gold_table', 'schema_column', 'mapping_example'));

ALTER TABLE local_search_embeddings
    ADD COLUMN tenant_id BIGINT REFERENCES tenants(id) ON DELETE CASCADE,
    ADD COLUMN resource_kind TEXT,
    ADD COLUMN resource_id BIGINT,
    ADD COLUMN resource_public_id UUID,
    ADD COLUMN metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN indexed_at TIMESTAMPTZ;

UPDATE local_search_embeddings e
SET
    tenant_id = d.tenant_id,
    resource_kind = d.resource_kind,
    resource_id = d.resource_id,
    resource_public_id = d.resource_public_id
FROM local_search_documents d
WHERE d.id = e.document_id;

UPDATE local_search_embeddings
SET
    status = 'pending',
    embedding = NULL,
    error_summary = NULL,
    updated_at = now()
WHERE embedding IS NOT NULL
   OR status = 'completed';

ALTER TABLE local_search_embeddings
    ADD COLUMN embedding_v2 vector(1024);

ALTER TABLE local_search_embeddings
    DROP COLUMN embedding;

ALTER TABLE local_search_embeddings
    RENAME COLUMN embedding_v2 TO embedding;

ALTER TABLE local_search_embeddings
    ALTER COLUMN tenant_id SET NOT NULL,
    ALTER COLUMN resource_kind SET NOT NULL,
    ALTER COLUMN resource_id SET NOT NULL,
    ALTER COLUMN resource_public_id SET NOT NULL;

CREATE INDEX local_search_embeddings_tenant_kind_model_status_idx
    ON local_search_embeddings(tenant_id, resource_kind, model, status, indexed_at DESC);

CREATE INDEX local_search_embeddings_hnsw_cosine_idx
    ON local_search_embeddings USING hnsw (embedding vector_cosine_ops)
    WHERE status = 'completed' AND embedding IS NOT NULL;
