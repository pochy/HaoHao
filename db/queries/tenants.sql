-- name: UpsertTenantBySlug :one
INSERT INTO tenants (
    slug,
    display_name,
    active
) VALUES (
    $1,
    $2,
    true
)
ON CONFLICT (slug) DO UPDATE
SET display_name = EXCLUDED.display_name,
    active = true,
    updated_at = now()
RETURNING
    id,
    slug,
    display_name,
    active,
    created_at,
    updated_at;

-- name: GetTenantBySlug :one
SELECT
    id,
    slug,
    display_name,
    active,
    created_at,
    updated_at
FROM tenants
WHERE slug = $1
LIMIT 1;

-- name: GetTenantByID :one
SELECT
    id,
    slug,
    display_name,
    active,
    created_at,
    updated_at
FROM tenants
WHERE id = $1
LIMIT 1;

-- name: DeleteTenantMembershipsByUserSource :exec
DELETE FROM tenant_memberships
WHERE user_id = $1
  AND source = $2;

-- name: UpsertTenantMembership :exec
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
    $4,
    true
)
ON CONFLICT (user_id, tenant_id, role_id, source) DO UPDATE
SET active = true,
    updated_at = now();

-- name: ListTenantMembershipRowsByUserID :many
SELECT
    t.id AS tenant_id,
    t.slug AS tenant_slug,
    t.display_name AS tenant_display_name,
    t.active AS tenant_active,
    r.code AS role_code,
    tm.source,
    tm.active AS membership_active
FROM tenant_memberships tm
JOIN tenants t ON t.id = tm.tenant_id
JOIN roles r ON r.id = tm.role_id
WHERE tm.user_id = $1
ORDER BY t.slug, r.code, tm.source;

-- name: ListTenantRoleOverridesByUserID :many
SELECT
    t.id AS tenant_id,
    t.slug AS tenant_slug,
    r.code AS role_code,
    tro.effect
FROM tenant_role_overrides tro
JOIN tenants t ON t.id = tro.tenant_id
JOIN roles r ON r.id = tro.role_id
WHERE tro.user_id = $1
ORDER BY t.slug, r.code, tro.effect;

-- name: UserHasActiveTenant :one
SELECT EXISTS (
    SELECT 1
    FROM tenant_memberships tm
    JOIN tenants t ON t.id = tm.tenant_id
    WHERE tm.user_id = $1
      AND t.id = $2
      AND t.active = true
      AND tm.active = true
)::boolean AS ok;
