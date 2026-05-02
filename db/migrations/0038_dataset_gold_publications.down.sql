ALTER TABLE dataset_gold_publications
    DROP CONSTRAINT IF EXISTS dataset_gold_publications_last_publish_run_id_fkey;

DROP TABLE IF EXISTS dataset_gold_publish_runs;
DROP TABLE IF EXISTS dataset_gold_publications;
