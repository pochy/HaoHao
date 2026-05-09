DROP INDEX IF EXISTS local_search_embeddings_hnsw_cosine_768_idx;
DROP INDEX IF EXISTS local_search_embeddings_hnsw_cosine_1024_idx;

ALTER TABLE local_search_embeddings
    DROP CONSTRAINT IF EXISTS local_search_embeddings_embedding_dimension_match_check;

DELETE FROM local_search_embeddings
WHERE dimension <> 1024;

ALTER TABLE local_search_embeddings
    ALTER COLUMN embedding TYPE vector(1024);

CREATE INDEX local_search_embeddings_hnsw_cosine_idx
    ON local_search_embeddings USING hnsw (embedding vector_cosine_ops)
    WHERE status = 'completed' AND embedding IS NOT NULL;
