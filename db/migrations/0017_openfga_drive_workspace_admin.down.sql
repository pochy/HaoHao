DROP TABLE IF EXISTS drive_file_revisions;
DROP TABLE IF EXISTS drive_admin_content_access_sessions;

ALTER TABLE drive_share_links
    DROP CONSTRAINT IF EXISTS drive_share_links_role_check;

ALTER TABLE drive_share_links
    ADD CONSTRAINT drive_share_links_role_check
        CHECK (role = 'viewer');

DROP INDEX IF EXISTS file_objects_drive_scan_idx;
DROP INDEX IF EXISTS file_objects_drive_workspace_children_idx;
DROP INDEX IF EXISTS drive_folders_workspace_children_idx;

ALTER TABLE file_objects
    DROP COLUMN IF EXISTS upload_state,
    DROP COLUMN IF EXISTS dlp_blocked,
    DROP COLUMN IF EXISTS scanned_at,
    DROP COLUMN IF EXISTS scan_engine,
    DROP COLUMN IF EXISTS scan_reason,
    DROP COLUMN IF EXISTS scan_status,
    DROP COLUMN IF EXISTS etag,
    DROP COLUMN IF EXISTS content_sha256,
    DROP COLUMN IF EXISTS storage_version,
    DROP COLUMN IF EXISTS storage_bucket,
    DROP COLUMN IF EXISTS workspace_id;

ALTER TABLE drive_folders
    DROP COLUMN IF EXISTS workspace_id;

DROP TABLE IF EXISTS drive_workspaces;
