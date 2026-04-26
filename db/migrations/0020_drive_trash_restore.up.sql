CREATE INDEX file_objects_drive_trash_idx
    ON file_objects (tenant_id, deleted_at DESC, id DESC)
    WHERE purpose = 'drive'
      AND deleted_at IS NOT NULL
      AND purged_at IS NULL;

CREATE INDEX drive_folders_trash_idx
    ON drive_folders (tenant_id, deleted_at DESC, id DESC)
    WHERE deleted_at IS NOT NULL;
