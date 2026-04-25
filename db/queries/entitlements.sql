-- name: ListTenantEntitlements :many
SELECT
    fd.code,
    fd.display_name,
    fd.description,
    COALESCE(te.enabled, fd.default_enabled)::boolean AS enabled,
    COALESCE(te.limit_value, fd.default_limit)::jsonb AS limit_value,
    COALESCE(te.source, 'default')::text AS source,
    COALESCE(te.updated_at, fd.updated_at) AS updated_at
FROM feature_definitions fd
LEFT JOIN tenant_entitlements te
    ON te.feature_code = fd.code
   AND te.tenant_id = $1
ORDER BY fd.code;

-- name: GetTenantEntitlement :one
SELECT
    fd.code,
    fd.display_name,
    fd.description,
    COALESCE(te.enabled, fd.default_enabled)::boolean AS enabled,
    COALESCE(te.limit_value, fd.default_limit)::jsonb AS limit_value,
    COALESCE(te.source, 'default')::text AS source,
    COALESCE(te.updated_at, fd.updated_at) AS updated_at
FROM feature_definitions fd
LEFT JOIN tenant_entitlements te
    ON te.feature_code = fd.code
   AND te.tenant_id = $1
WHERE fd.code = $2
LIMIT 1;

-- name: UpsertTenantEntitlement :one
INSERT INTO tenant_entitlements (
    tenant_id,
    feature_code,
    enabled,
    limit_value,
    source
) VALUES (
    $1, $2, $3, $4, $5
)
ON CONFLICT (tenant_id, feature_code) DO UPDATE
SET enabled = EXCLUDED.enabled,
    limit_value = EXCLUDED.limit_value,
    source = EXCLUDED.source,
    updated_at = now()
RETURNING *;
