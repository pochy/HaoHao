DROP INDEX IF EXISTS dataset_gold_publish_runs_source_pipeline_output_idx;
DROP INDEX IF EXISTS dataset_gold_publish_runs_source_pipeline_run_idx;

ALTER TABLE dataset_gold_publish_runs
    DROP COLUMN IF EXISTS source_data_pipeline_run_output_id,
    DROP COLUMN IF EXISTS source_data_pipeline_run_id;
