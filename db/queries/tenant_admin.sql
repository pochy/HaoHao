-- name: ListTenantAdminTenants :many
SELECT
    t.id,
    t.slug,
    t.display_name,
    t.active,
    t.created_at,
    t.updated_at,
    COALESCE(COUNT(DISTINCT tm.user_id) FILTER (WHERE tm.active), 0)::bigint AS active_member_count
FROM tenants t
LEFT JOIN tenant_memberships tm ON tm.tenant_id = t.id
GROUP BY t.id
ORDER BY t.slug;

-- name: GetTenantAdminTenant :one
SELECT
    t.id,
    t.slug,
    t.display_name,
    t.active,
    t.created_at,
    t.updated_at,
    COALESCE(COUNT(DISTINCT tm.user_id) FILTER (WHERE tm.active), 0)::bigint AS active_member_count
FROM tenants t
LEFT JOIN tenant_memberships tm ON tm.tenant_id = t.id
WHERE t.slug = $1
GROUP BY t.id
LIMIT 1;

-- name: CreateTenantAdminTenant :one
INSERT INTO tenants (
    slug,
    display_name,
    active
) VALUES (
    $1,
    $2,
    true
)
RETURNING
    id,
    slug,
    display_name,
    active,
    created_at,
    updated_at;

-- name: UpdateTenantAdminTenant :one
UPDATE tenants
SET
    display_name = $2,
    active = $3,
    updated_at = now()
WHERE slug = $1
RETURNING
    id,
    slug,
    display_name,
    active,
    created_at,
    updated_at;

-- name: DeactivateTenantAdminTenant :one
UPDATE tenants
SET
    active = false,
    updated_at = now()
WHERE slug = $1
RETURNING
    id,
    slug,
    display_name,
    active,
    created_at,
    updated_at;

-- name: ListTenantAdminMembershipRows :many
SELECT
    u.id AS user_id,
    u.public_id AS user_public_id,
    u.email,
    u.display_name AS user_display_name,
    u.deactivated_at AS user_deactivated_at,
    t.id AS tenant_id,
    t.slug AS tenant_slug,
    t.display_name AS tenant_display_name,
    r.code AS role_code,
    tm.source,
    tm.active,
    tm.created_at,
    tm.updated_at
FROM tenant_memberships tm
JOIN users u ON u.id = tm.user_id
JOIN tenants t ON t.id = tm.tenant_id
JOIN roles r ON r.id = tm.role_id
WHERE t.slug = $1
ORDER BY u.email, r.code, tm.source;

-- name: UpsertTenantAdminLocalMembership :one
INSERT INTO tenant_memberships (
    user_id,
    tenant_id,
    role_id,
    source,
    active
) VALUES (
    $1,
    $2,
    $3,
    'local_override',
    true
)
ON CONFLICT (user_id, tenant_id, role_id, source) DO UPDATE
SET active = true,
    updated_at = now()
RETURNING
    user_id,
    tenant_id,
    role_id,
    source,
    active,
    created_at,
    updated_at;

-- name: DeactivateTenantAdminLocalMembershipRole :execrows
UPDATE tenant_memberships
SET
    active = false,
    updated_at = now()
WHERE user_id = $1
  AND tenant_id = $2
  AND role_id = $3
  AND source = 'local_override'
  AND active = true;

-- name: CountActiveTenantAdmins :one
SELECT COUNT(DISTINCT tm.user_id)::bigint
FROM tenant_memberships tm
JOIN roles r ON r.id = tm.role_id
WHERE tm.tenant_id = $1
  AND r.code = 'tenant_admin'
  AND tm.active = true;
