-- name: StarDriveItem :exec
INSERT INTO drive_starred_items (
    tenant_id,
    user_id,
    resource_type,
    resource_id
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(user_id),
    sqlc.arg(resource_type),
    sqlc.arg(resource_id)
)
ON CONFLICT (tenant_id, user_id, resource_type, resource_id)
WHERE deleted_at IS NULL
DO UPDATE SET updated_at = now();

-- name: UnstarDriveItem :exec
UPDATE drive_starred_items
SET deleted_at = COALESCE(deleted_at, now()),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND user_id = sqlc.arg(user_id)
  AND resource_type = sqlc.arg(resource_type)
  AND resource_id = sqlc.arg(resource_id)
  AND deleted_at IS NULL;

-- name: IsDriveItemStarredByUser :one
SELECT EXISTS (
    SELECT 1
    FROM drive_starred_items
    WHERE tenant_id = sqlc.arg(tenant_id)
      AND user_id = sqlc.arg(user_id)
      AND resource_type = sqlc.arg(resource_type)
      AND resource_id = sqlc.arg(resource_id)
      AND deleted_at IS NULL
)::boolean AS ok;

-- name: ListDriveStarredResourceRefs :many
SELECT resource_type, resource_id
FROM drive_starred_items
WHERE tenant_id = sqlc.arg(tenant_id)
  AND user_id = sqlc.arg(user_id)
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit_count);

-- name: ListDriveSharedResourceRefs :many
SELECT DISTINCT ON (share.resource_type, share.resource_id)
    share.resource_type,
    share.resource_id,
    share.role
FROM drive_resource_shares share
LEFT JOIN drive_group_members member
    ON member.group_id = share.subject_id
   AND member.user_id = sqlc.arg(user_id)
   AND member.deleted_at IS NULL
WHERE share.tenant_id = sqlc.arg(tenant_id)
  AND share.status = 'active'
  AND (
      (share.subject_type = 'user' AND share.subject_id = sqlc.arg(user_id))
      OR (share.subject_type = 'group' AND member.id IS NOT NULL)
  )
ORDER BY share.resource_type, share.resource_id, share.created_at DESC, share.id DESC
LIMIT sqlc.arg(limit_count);

-- name: RecordDriveItemActivity :exec
INSERT INTO drive_item_activities (
    tenant_id,
    actor_user_id,
    resource_type,
    resource_id,
    action,
    metadata
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.narg(actor_user_id),
    sqlc.arg(resource_type),
    sqlc.arg(resource_id),
    sqlc.arg(action),
    COALESCE(sqlc.narg(metadata), '{}'::jsonb)
);

-- name: ListDriveRecentResourceRefs :many
WITH latest AS (
    SELECT DISTINCT ON (resource_type, resource_id)
        resource_type,
        resource_id,
        action,
        created_at
    FROM drive_item_activities
    WHERE tenant_id = sqlc.arg(tenant_id)
      AND actor_user_id = sqlc.arg(actor_user_id)
    ORDER BY resource_type, resource_id, created_at DESC, id DESC
)
SELECT resource_type, resource_id, action, created_at
FROM latest
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count);

-- name: ListDriveItemActivities :many
SELECT
    activity.public_id,
    activity.resource_type,
    activity.resource_id,
    activity.action,
    activity.metadata,
    activity.created_at,
    COALESCE(actor.public_id::text, '') AS actor_public_id,
    COALESCE(actor.display_name, '') AS actor_display_name
FROM drive_item_activities activity
LEFT JOIN users actor ON actor.id = activity.actor_user_id
WHERE activity.tenant_id = sqlc.arg(tenant_id)
  AND activity.resource_type = sqlc.arg(resource_type)
  AND activity.resource_id = sqlc.arg(resource_id)
ORDER BY activity.created_at DESC, activity.id DESC
LIMIT sqlc.arg(limit_count);

-- name: GetDriveStorageUsage :one
SELECT
    COALESCE(sum(byte_size) FILTER (WHERE deleted_at IS NULL), 0)::bigint AS used_bytes,
    COALESCE(sum(byte_size) FILTER (WHERE deleted_at IS NOT NULL AND purged_at IS NULL), 0)::bigint AS trash_bytes,
    count(*) FILTER (WHERE deleted_at IS NULL)::bigint AS file_count,
    count(*) FILTER (WHERE deleted_at IS NOT NULL AND purged_at IS NULL)::bigint AS trash_file_count,
    COALESCE(max(storage_driver) FILTER (WHERE deleted_at IS NULL), '')::text AS storage_driver
FROM file_objects
WHERE tenant_id = sqlc.arg(tenant_id)
  AND purpose = 'drive';

-- name: ListDriveFolderTreeCandidates :many
SELECT *
FROM drive_folders
WHERE tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
ORDER BY COALESCE(parent_folder_id, 0), lower(name), id
LIMIT sqlc.arg(limit_count);

-- name: ListDriveTagsForItem :many
SELECT tag
FROM drive_item_tags
WHERE tenant_id = sqlc.arg(tenant_id)
  AND resource_type = sqlc.arg(resource_type)
  AND resource_id = sqlc.arg(resource_id)
ORDER BY lower(tag), tag;

-- name: DeleteDriveTagsForItem :exec
DELETE FROM drive_item_tags
WHERE tenant_id = sqlc.arg(tenant_id)
  AND resource_type = sqlc.arg(resource_type)
  AND resource_id = sqlc.arg(resource_id);

-- name: AddDriveItemTag :exec
INSERT INTO drive_item_tags (
    tenant_id,
    resource_type,
    resource_id,
    tag,
    normalized_tag,
    created_by_user_id
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(resource_type),
    sqlc.arg(resource_id),
    sqlc.arg(tag),
    lower(btrim(sqlc.arg(tag))),
    sqlc.narg(created_by_user_id)
)
ON CONFLICT (tenant_id, resource_type, resource_id, normalized_tag) DO NOTHING;

-- name: UpsertDriveFilePreviewState :exec
INSERT INTO drive_file_previews (
    tenant_id,
    file_object_id,
    status,
    thumbnail_storage_key,
    preview_storage_key,
    content_type,
    error_code,
    generated_at
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(file_object_id),
    sqlc.arg(status),
    sqlc.narg(thumbnail_storage_key),
    sqlc.narg(preview_storage_key),
    sqlc.narg(content_type),
    sqlc.narg(error_code),
    CASE WHEN sqlc.arg(status) = 'ready' THEN now() ELSE NULL END
)
ON CONFLICT (tenant_id, file_object_id)
DO UPDATE SET
    status = EXCLUDED.status,
    thumbnail_storage_key = EXCLUDED.thumbnail_storage_key,
    preview_storage_key = EXCLUDED.preview_storage_key,
    content_type = EXCLUDED.content_type,
    error_code = EXCLUDED.error_code,
    generated_at = EXCLUDED.generated_at,
    updated_at = now();

-- name: ListDriveShareTargets :many
SELECT 'user'::text AS target_type,
       u.public_id::text AS public_id,
       u.display_name AS display_name,
       u.email AS secondary
FROM users u
JOIN tenant_memberships tm ON tm.user_id = u.id
WHERE tm.tenant_id = sqlc.arg(tenant_id)
  AND tm.active = true
  AND u.deactivated_at IS NULL
  AND (
      sqlc.narg(query)::text IS NULL
      OR u.display_name ILIKE '%' || sqlc.narg(query)::text || '%'
      OR u.email ILIKE '%' || sqlc.narg(query)::text || '%'
      OR u.public_id::text ILIKE '%' || sqlc.narg(query)::text || '%'
  )
UNION ALL
SELECT 'group'::text AS target_type,
       g.public_id::text AS public_id,
       g.name AS display_name,
       g.description AS secondary
FROM drive_groups g
WHERE g.tenant_id = sqlc.arg(tenant_id)
  AND g.deleted_at IS NULL
  AND (
      sqlc.narg(query)::text IS NULL
      OR g.name ILIKE '%' || sqlc.narg(query)::text || '%'
      OR g.public_id::text ILIKE '%' || sqlc.narg(query)::text || '%'
  )
ORDER BY display_name ASC, public_id ASC
LIMIT sqlc.arg(limit_count);

-- name: UpdateDriveResourceShareRole :one
UPDATE drive_resource_shares
SET
    role = sqlc.arg(role),
    status = 'active',
    updated_at = now()
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND status <> 'revoked'
RETURNING *;

-- name: CopyDriveFileObject :one
INSERT INTO file_objects (
    tenant_id,
    uploaded_by_user_id,
    purpose,
    workspace_id,
    drive_folder_id,
    original_filename,
    description,
    content_type,
    byte_size,
    sha256_hex,
    content_sha256,
    storage_driver,
    storage_key,
    storage_bucket,
    etag,
    scan_status,
    inheritance_enabled
)
SELECT
    source.tenant_id,
    sqlc.narg(uploaded_by_user_id),
    'drive',
    sqlc.narg(workspace_id),
    sqlc.narg(drive_folder_id),
    sqlc.arg(original_filename),
    source.description,
    source.content_type,
    sqlc.arg(byte_size),
    sqlc.arg(sha256_hex),
    NULLIF(sqlc.arg(sha256_hex), ''),
    sqlc.arg(storage_driver),
    sqlc.arg(storage_key),
    sqlc.narg(storage_bucket),
    NULLIF(sqlc.arg(etag)::text, ''),
    source.scan_status,
    sqlc.arg(inheritance_enabled)
FROM file_objects source
WHERE source.id = sqlc.arg(source_id)
  AND source.tenant_id = sqlc.arg(tenant_id)
  AND source.purpose = 'drive'
  AND source.deleted_at IS NULL
RETURNING *;

-- name: CopyDriveFolder :one
INSERT INTO drive_folders (
    tenant_id,
    workspace_id,
    parent_folder_id,
    name,
    description,
    created_by_user_id,
    inheritance_enabled
)
SELECT
    source.tenant_id,
    sqlc.narg(workspace_id),
    sqlc.narg(parent_folder_id),
    sqlc.arg(name),
    source.description,
    sqlc.arg(created_by_user_id),
    sqlc.arg(inheritance_enabled)
FROM drive_folders source
WHERE source.id = sqlc.arg(source_id)
  AND source.tenant_id = sqlc.arg(tenant_id)
  AND source.deleted_at IS NULL
RETURNING *;

-- name: UpdateDriveFileOwner :one
UPDATE file_objects
SET
    uploaded_by_user_id = sqlc.arg(uploaded_by_user_id),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND purpose = 'drive'
  AND deleted_at IS NULL
RETURNING *;

-- name: UpdateDriveFolderOwner :one
UPDATE drive_folders
SET
    created_by_user_id = sqlc.arg(created_by_user_id),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
RETURNING *;

-- name: MarkDriveFilePermanentlyDeleted :one
UPDATE file_objects
SET
    purged_at = COALESCE(purged_at, now()),
    purge_locked_at = NULL,
    purge_locked_by = NULL,
    last_purge_error = NULL,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND purpose = 'drive'
  AND status = 'deleted'
  AND deleted_at IS NOT NULL
  AND purged_at IS NULL
  AND legal_hold_at IS NULL
  AND purge_block_reason IS NULL
  AND (retention_until IS NULL OR retention_until <= now())
RETURNING *;

-- name: CountDriveFolderChildrenAnyState :one
SELECT (
    (SELECT count(*) FROM drive_folders folder WHERE folder.tenant_id = sqlc.arg(tenant_id) AND folder.parent_folder_id = sqlc.arg(folder_id))
    +
    (SELECT count(*) FROM file_objects file WHERE file.tenant_id = sqlc.arg(tenant_id) AND file.purpose = 'drive' AND file.drive_folder_id = sqlc.arg(folder_id) AND file.purged_at IS NULL)
)::bigint AS child_count;

-- name: DeleteDriveFolderPermanently :one
DELETE FROM drive_folders
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NOT NULL
  AND legal_hold_at IS NULL
  AND purge_block_reason IS NULL
  AND (retention_until IS NULL OR retention_until <= now())
RETURNING *;
