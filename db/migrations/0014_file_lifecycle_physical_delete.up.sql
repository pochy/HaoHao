ALTER TABLE file_objects
    ADD COLUMN purged_at TIMESTAMPTZ,
    ADD COLUMN purge_attempts INTEGER NOT NULL DEFAULT 0 CHECK (purge_attempts >= 0),
    ADD COLUMN purge_locked_at TIMESTAMPTZ,
    ADD COLUMN purge_locked_by TEXT,
    ADD COLUMN last_purge_error TEXT;

CREATE INDEX file_objects_purge_candidates_idx
    ON file_objects (deleted_at, id)
    WHERE deleted_at IS NOT NULL
      AND purged_at IS NULL;

CREATE INDEX file_objects_purge_lock_idx
    ON file_objects (purge_locked_at)
    WHERE purge_locked_at IS NOT NULL
      AND purged_at IS NULL;
