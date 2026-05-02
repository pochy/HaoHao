CREATE TABLE dataset_work_table_export_schedules (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    work_table_id BIGINT NOT NULL REFERENCES dataset_work_tables(id) ON DELETE CASCADE,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    format TEXT NOT NULL DEFAULT 'csv',
    frequency TEXT NOT NULL,
    timezone TEXT NOT NULL,
    run_time TEXT NOT NULL,
    weekday SMALLINT,
    month_day SMALLINT,
    retention_days INTEGER NOT NULL DEFAULT 7,
    enabled BOOLEAN NOT NULL DEFAULT true,
    next_run_at TIMESTAMPTZ NOT NULL,
    last_run_at TIMESTAMPTZ,
    last_status TEXT,
    last_error_summary TEXT,
    last_export_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT dataset_work_table_export_schedules_public_id_key UNIQUE (public_id),
    CONSTRAINT dataset_work_table_export_schedules_format_check CHECK (format IN ('csv', 'json', 'parquet')),
    CONSTRAINT dataset_work_table_export_schedules_frequency_check CHECK (frequency IN ('daily', 'weekly', 'monthly')),
    CONSTRAINT dataset_work_table_export_schedules_run_time_check CHECK (run_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    CONSTRAINT dataset_work_table_export_schedules_weekday_check CHECK (weekday IS NULL OR weekday BETWEEN 1 AND 7),
    CONSTRAINT dataset_work_table_export_schedules_month_day_check CHECK (month_day IS NULL OR month_day BETWEEN 1 AND 28),
    CONSTRAINT dataset_work_table_export_schedules_retention_days_check CHECK (retention_days BETWEEN 1 AND 365),
    CONSTRAINT dataset_work_table_export_schedules_last_status_check CHECK (last_status IS NULL OR last_status IN ('created', 'skipped', 'failed', 'ready', 'disabled')),
    CONSTRAINT dataset_work_table_export_schedules_frequency_shape_check CHECK (
        (frequency = 'daily' AND weekday IS NULL AND month_day IS NULL)
        OR (frequency = 'weekly' AND weekday IS NOT NULL AND month_day IS NULL)
        OR (frequency = 'monthly' AND weekday IS NULL AND month_day IS NOT NULL)
    )
);

ALTER TABLE dataset_work_table_exports
    ADD COLUMN schedule_id BIGINT REFERENCES dataset_work_table_export_schedules(id) ON DELETE SET NULL,
    ADD COLUMN scheduled_for TIMESTAMPTZ;

ALTER TABLE dataset_work_table_export_schedules
    ADD CONSTRAINT dataset_work_table_export_schedules_last_export_id_fkey
    FOREIGN KEY (last_export_id) REFERENCES dataset_work_table_exports(id) ON DELETE SET NULL;

CREATE INDEX dataset_work_table_export_schedules_work_table_created_idx
    ON dataset_work_table_export_schedules(work_table_id, created_at DESC, id DESC);

CREATE INDEX dataset_work_table_export_schedules_due_idx
    ON dataset_work_table_export_schedules(next_run_at, id)
    WHERE enabled;

CREATE INDEX dataset_work_table_export_schedules_tenant_enabled_idx
    ON dataset_work_table_export_schedules(tenant_id, enabled, next_run_at);

CREATE INDEX dataset_work_table_exports_schedule_created_idx
    ON dataset_work_table_exports(schedule_id, created_at DESC, id DESC)
    WHERE schedule_id IS NOT NULL;
