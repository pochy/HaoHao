DROP INDEX IF EXISTS local_search_embeddings_hnsw_cosine_idx;
DROP INDEX IF EXISTS local_search_embeddings_hnsw_cosine_1024_idx;
DROP INDEX IF EXISTS local_search_embeddings_hnsw_cosine_768_idx;

ALTER TABLE local_search_embeddings
    ALTER COLUMN embedding TYPE vector;

ALTER TABLE local_search_embeddings
    ADD CONSTRAINT local_search_embeddings_embedding_dimension_match_check
        CHECK (embedding IS NULL OR dimension = vector_dims(embedding)) NOT VALID;

CREATE INDEX local_search_embeddings_hnsw_cosine_1024_idx
    ON local_search_embeddings USING hnsw ((embedding::vector(1024)) vector_cosine_ops)
    WHERE status = 'completed' AND embedding IS NOT NULL AND dimension = 1024;

CREATE INDEX local_search_embeddings_hnsw_cosine_768_idx
    ON local_search_embeddings USING hnsw ((embedding::vector(768)) vector_cosine_ops)
    WHERE status = 'completed' AND embedding IS NOT NULL AND dimension = 768;
