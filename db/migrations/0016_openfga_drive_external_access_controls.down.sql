DROP INDEX IF EXISTS drive_folders_legal_hold_idx;
DROP INDEX IF EXISTS drive_folders_retention_idx;
DROP INDEX IF EXISTS file_objects_drive_legal_hold_idx;
DROP INDEX IF EXISTS file_objects_drive_retention_idx;

ALTER TABLE drive_folders
    DROP COLUMN IF EXISTS purge_block_reason,
    DROP COLUMN IF EXISTS legal_hold_reason,
    DROP COLUMN IF EXISTS legal_hold_by_user_id,
    DROP COLUMN IF EXISTS legal_hold_at,
    DROP COLUMN IF EXISTS retention_until,
    DROP COLUMN IF EXISTS deleted_parent_folder_id;

ALTER TABLE file_objects
    DROP COLUMN IF EXISTS purge_block_reason,
    DROP COLUMN IF EXISTS legal_hold_reason,
    DROP COLUMN IF EXISTS legal_hold_by_user_id,
    DROP COLUMN IF EXISTS legal_hold_at,
    DROP COLUMN IF EXISTS retention_until,
    DROP COLUMN IF EXISTS deleted_parent_folder_id;

DROP TABLE IF EXISTS drive_group_external_mappings;
DROP TABLE IF EXISTS drive_share_invitations;
DROP TABLE IF EXISTS drive_share_link_password_attempts;

DROP INDEX IF EXISTS drive_share_links_password_required_idx;

ALTER TABLE drive_share_links
    DROP COLUMN IF EXISTS password_updated_at,
    DROP COLUMN IF EXISTS password_required,
    DROP COLUMN IF EXISTS password_hash;
