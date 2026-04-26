DROP TABLE IF EXISTS drive_share_links;
DROP TABLE IF EXISTS drive_resource_shares;
DROP TABLE IF EXISTS drive_group_members;
DROP TABLE IF EXISTS drive_groups;

DROP INDEX IF EXISTS file_objects_drive_folder_idx;
DROP INDEX IF EXISTS file_objects_drive_children_idx;

ALTER TABLE file_objects
    DROP COLUMN IF EXISTS deleted_by_user_id,
    DROP COLUMN IF EXISTS inheritance_enabled,
    DROP COLUMN IF EXISTS lock_reason,
    DROP COLUMN IF EXISTS locked_by_user_id,
    DROP COLUMN IF EXISTS locked_at,
    DROP COLUMN IF EXISTS drive_folder_id;

DROP TABLE IF EXISTS drive_folders;

DELETE FROM file_objects
WHERE purpose = 'drive';

ALTER TABLE file_objects
    DROP CONSTRAINT IF EXISTS file_objects_purpose_check;

ALTER TABLE file_objects
    ADD CONSTRAINT file_objects_purpose_check
        CHECK (purpose IN ('attachment', 'avatar', 'import', 'export'));
