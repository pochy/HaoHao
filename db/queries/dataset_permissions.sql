-- name: EnsureTenantDataAccessScope :one
INSERT INTO tenant_data_access_scopes (tenant_id)
VALUES (sqlc.arg(tenant_id))
ON CONFLICT (tenant_id) DO UPDATE
SET updated_at = tenant_data_access_scopes.updated_at
RETURNING *;

-- name: GetTenantDataAccessScope :one
SELECT *
FROM tenant_data_access_scopes
WHERE tenant_id = sqlc.arg(tenant_id)
LIMIT 1;

-- name: EnsureDatasetManagersGroup :one
INSERT INTO dataset_permission_groups (
    tenant_id,
    name,
    description,
    system_key,
    created_by_user_id
) VALUES (
    sqlc.arg(tenant_id),
    'Dataset Managers',
    'System group for Dataset, Work table, and Data Pipeline owners.',
    'dataset_managers',
    sqlc.narg(created_by_user_id)
)
ON CONFLICT (tenant_id, system_key) WHERE system_key IS NOT NULL AND deleted_at IS NULL DO UPDATE
SET updated_at = dataset_permission_groups.updated_at
RETURNING *;

-- name: CreateDatasetPermissionGroup :one
INSERT INTO dataset_permission_groups (
    tenant_id,
    name,
    description,
    created_by_user_id
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(name),
    sqlc.arg(description),
    sqlc.narg(created_by_user_id)
)
RETURNING *;

-- name: ListDatasetPermissionGroups :many
SELECT *
FROM dataset_permission_groups
WHERE tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
ORDER BY system_key NULLS LAST, updated_at DESC, id DESC
LIMIT sqlc.arg(limit_count);

-- name: GetDatasetPermissionGroupByPublicIDForTenant :one
SELECT *
FROM dataset_permission_groups
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
  AND deleted_at IS NULL
LIMIT 1;

-- name: GetDatasetPermissionGroupByIDForTenant :one
SELECT *
FROM dataset_permission_groups
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
  AND deleted_at IS NULL
LIMIT 1;

-- name: UpdateDatasetPermissionGroup :one
UPDATE dataset_permission_groups
SET
    name = sqlc.arg(name),
    description = sqlc.arg(description),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
  AND system_key IS NULL
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteDatasetPermissionGroup :one
UPDATE dataset_permission_groups
SET deleted_at = COALESCE(deleted_at, now()),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
  AND system_key IS NULL
  AND deleted_at IS NULL
RETURNING *;

-- name: AddDatasetPermissionGroupMember :one
INSERT INTO dataset_permission_group_members (
    group_id,
    user_id,
    added_by_user_id
) VALUES (
    sqlc.arg(group_id),
    sqlc.arg(user_id),
    sqlc.narg(added_by_user_id)
)
ON CONFLICT (group_id, user_id) WHERE deleted_at IS NULL DO UPDATE
SET deleted_at = NULL
RETURNING *;

-- name: RemoveDatasetPermissionGroupMember :one
UPDATE dataset_permission_group_members
SET deleted_at = COALESCE(deleted_at, now())
WHERE group_id = sqlc.arg(group_id)
  AND user_id = sqlc.arg(user_id)
  AND deleted_at IS NULL
RETURNING *;

-- name: ListDatasetPermissionGroupMembers :many
SELECT
    m.id,
    m.group_id,
    m.user_id,
    u.public_id AS user_public_id,
    u.email,
    u.display_name,
    m.created_at
FROM dataset_permission_group_members m
JOIN users u ON u.id = m.user_id
WHERE m.group_id = sqlc.arg(group_id)
  AND m.deleted_at IS NULL
ORDER BY u.display_name, u.email, m.id;

-- name: AddTenantAdminsToDatasetManagersGroup :exec
INSERT INTO dataset_permission_group_members (
    group_id,
    user_id,
    added_by_user_id
)
SELECT
    sqlc.arg(group_id),
    admins.user_id,
    sqlc.narg(added_by_user_id)
FROM (
    SELECT tm.user_id
    FROM tenant_memberships tm
    JOIN roles r ON r.id = tm.role_id
    WHERE tm.tenant_id = sqlc.arg(tenant_id)
      AND tm.active = true
      AND r.code = 'tenant_admin'
    UNION
    SELECT ur.user_id
    FROM user_roles ur
    JOIN roles r ON r.id = ur.role_id
    WHERE r.code = 'tenant_admin'
) admins
ON CONFLICT (group_id, user_id) WHERE deleted_at IS NULL DO UPDATE
SET deleted_at = NULL;

-- name: ListDatasetPermissionGrants :many
SELECT
    g.id,
    g.tenant_id,
    g.resource_type,
    g.resource_public_id,
    g.subject_type,
    g.subject_user_id,
    u.public_id AS subject_user_public_id,
    u.email AS subject_user_email,
    u.display_name AS subject_user_display_name,
    g.subject_group_id,
    pg.public_id AS subject_group_public_id,
    pg.name AS subject_group_name,
    g.action,
    g.created_by_user_id,
    g.created_at
FROM dataset_permission_grants g
LEFT JOIN users u ON u.id = g.subject_user_id
LEFT JOIN dataset_permission_groups pg ON pg.id = g.subject_group_id
WHERE g.tenant_id = sqlc.arg(tenant_id)
  AND g.resource_type = sqlc.arg(resource_type)
  AND (
      (sqlc.narg(resource_public_id)::uuid IS NULL AND g.resource_public_id IS NULL)
      OR g.resource_public_id = sqlc.narg(resource_public_id)::uuid
  )
  AND g.revoked_at IS NULL
ORDER BY g.subject_type, COALESCE(u.display_name, pg.name), g.action;

-- name: RevokeDatasetPermissionGrantsForSubject :exec
UPDATE dataset_permission_grants
SET revoked_at = COALESCE(revoked_at, now())
WHERE tenant_id = sqlc.arg(tenant_id)
  AND resource_type = sqlc.arg(resource_type)
  AND (
      (sqlc.narg(resource_public_id)::uuid IS NULL AND resource_public_id IS NULL)
      OR resource_public_id = sqlc.narg(resource_public_id)::uuid
  )
  AND subject_type = sqlc.arg(subject_type)
  AND (
      (sqlc.narg(subject_user_id)::bigint IS NOT NULL AND subject_user_id = sqlc.narg(subject_user_id)::bigint)
      OR (sqlc.narg(subject_group_id)::bigint IS NOT NULL AND subject_group_id = sqlc.narg(subject_group_id)::bigint)
  )
  AND revoked_at IS NULL;

-- name: CreateDatasetPermissionGrant :one
INSERT INTO dataset_permission_grants (
    tenant_id,
    resource_type,
    resource_public_id,
    subject_type,
    subject_user_id,
    subject_group_id,
    action,
    created_by_user_id
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(resource_type),
    sqlc.narg(resource_public_id),
    sqlc.arg(subject_type),
    sqlc.narg(subject_user_id),
    sqlc.narg(subject_group_id),
    sqlc.arg(action),
    sqlc.narg(created_by_user_id)
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: ListTenantDataResourcesForPermissionBackfill :many
SELECT 'dataset'::text AS resource_type, d.public_id, d.created_by_user_id
FROM datasets d
WHERE d.tenant_id = sqlc.arg(backfill_tenant_id)
UNION ALL
SELECT 'work_table'::text AS resource_type, wt.public_id, wt.created_by_user_id
FROM dataset_work_tables wt
WHERE wt.tenant_id = sqlc.arg(backfill_tenant_id)
  AND wt.dropped_at IS NULL
UNION ALL
SELECT 'data_pipeline'::text AS resource_type, dp.public_id, dp.created_by_user_id
FROM data_pipelines dp
WHERE dp.tenant_id = sqlc.arg(backfill_tenant_id)
  AND dp.archived_at IS NULL
ORDER BY resource_type, public_id;
