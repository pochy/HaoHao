DROP INDEX IF EXISTS file_objects_purge_lock_idx;
DROP INDEX IF EXISTS file_objects_purge_candidates_idx;

ALTER TABLE file_objects
    DROP COLUMN IF EXISTS last_purge_error,
    DROP COLUMN IF EXISTS purge_locked_by,
    DROP COLUMN IF EXISTS purge_locked_at,
    DROP COLUMN IF EXISTS purge_attempts,
    DROP COLUMN IF EXISTS purged_at;
