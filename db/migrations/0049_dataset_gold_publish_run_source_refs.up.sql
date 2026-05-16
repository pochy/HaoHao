ALTER TABLE dataset_gold_publish_runs
    ADD COLUMN source_data_pipeline_run_id BIGINT REFERENCES data_pipeline_runs(id) ON DELETE SET NULL,
    ADD COLUMN source_data_pipeline_run_output_id BIGINT REFERENCES data_pipeline_run_outputs(id) ON DELETE SET NULL;

CREATE INDEX dataset_gold_publish_runs_source_pipeline_run_idx
    ON dataset_gold_publish_runs(source_data_pipeline_run_id)
    WHERE source_data_pipeline_run_id IS NOT NULL;

CREATE INDEX dataset_gold_publish_runs_source_pipeline_output_idx
    ON dataset_gold_publish_runs(source_data_pipeline_run_output_id)
    WHERE source_data_pipeline_run_output_id IS NOT NULL;
