DROP INDEX IF EXISTS local_search_embeddings_hnsw_cosine_idx;
DROP INDEX IF EXISTS local_search_embeddings_tenant_kind_model_status_idx;

ALTER TABLE local_search_embeddings
    ADD COLUMN embedding_v1 DOUBLE PRECISION[];

ALTER TABLE local_search_embeddings
    DROP COLUMN embedding;

ALTER TABLE local_search_embeddings
    RENAME COLUMN embedding_v1 TO embedding;

ALTER TABLE local_search_embeddings
    DROP COLUMN indexed_at,
    DROP COLUMN metadata,
    DROP COLUMN resource_public_id,
    DROP COLUMN resource_id,
    DROP COLUMN resource_kind,
    DROP COLUMN tenant_id;

ALTER TABLE local_search_index_jobs
    DROP CONSTRAINT local_search_index_jobs_resource_kind_check,
    ADD CONSTRAINT local_search_index_jobs_resource_kind_check
        CHECK (resource_kind IS NULL OR resource_kind IN ('drive_file', 'ocr_run', 'product_extraction', 'gold_table'));

ALTER TABLE local_search_documents
    DROP CONSTRAINT local_search_documents_resource_kind_check,
    ADD CONSTRAINT local_search_documents_resource_kind_check
        CHECK (resource_kind IN ('drive_file', 'ocr_run', 'product_extraction', 'gold_table'));

DROP EXTENSION IF EXISTS vector;
