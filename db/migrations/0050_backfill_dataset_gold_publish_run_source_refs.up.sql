WITH ranked_outputs AS (
    SELECT
        gpr.id AS publish_run_id,
        dpr.id AS source_data_pipeline_run_id,
        dpro.id AS source_data_pipeline_run_output_id,
        row_number() OVER (
            PARTITION BY gpr.id
            ORDER BY
                CASE
                    WHEN dpro.completed_at IS NOT NULL AND dpro.completed_at <= gpr.created_at THEN 0
                    ELSE 1
                END,
                dpro.completed_at DESC NULLS LAST,
                dpro.id DESC
        ) AS rank
    FROM dataset_gold_publish_runs gpr
    JOIN data_pipeline_run_outputs dpro
      ON dpro.tenant_id = gpr.tenant_id
     AND dpro.output_work_table_id = gpr.source_work_table_id
     AND dpro.status = 'completed'
    JOIN data_pipeline_runs dpr
      ON dpr.tenant_id = dpro.tenant_id
     AND dpr.id = dpro.run_id
    WHERE gpr.source_data_pipeline_run_id IS NULL
       OR gpr.source_data_pipeline_run_output_id IS NULL
)
UPDATE dataset_gold_publish_runs gpr
SET
    source_data_pipeline_run_id = ranked_outputs.source_data_pipeline_run_id,
    source_data_pipeline_run_output_id = ranked_outputs.source_data_pipeline_run_output_id,
    updated_at = now()
FROM ranked_outputs
WHERE ranked_outputs.publish_run_id = gpr.id
  AND ranked_outputs.rank = 1
  AND (
      gpr.source_data_pipeline_run_id IS NULL
      OR gpr.source_data_pipeline_run_output_id IS NULL
  );
