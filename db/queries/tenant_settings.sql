-- name: UpsertTenantSettings :one
INSERT INTO tenant_settings (
    tenant_id,
    file_quota_bytes,
    rate_limit_login_per_minute,
    rate_limit_browser_api_per_minute,
    rate_limit_external_api_per_minute,
    notifications_enabled,
    features
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (tenant_id) DO UPDATE
SET
    file_quota_bytes = EXCLUDED.file_quota_bytes,
    rate_limit_login_per_minute = EXCLUDED.rate_limit_login_per_minute,
    rate_limit_browser_api_per_minute = EXCLUDED.rate_limit_browser_api_per_minute,
    rate_limit_external_api_per_minute = EXCLUDED.rate_limit_external_api_per_minute,
    notifications_enabled = EXCLUDED.notifications_enabled,
    features = EXCLUDED.features,
    updated_at = now()
RETURNING *;

-- name: GetTenantSettings :one
SELECT *
FROM tenant_settings
WHERE tenant_id = $1;

-- name: SumActiveFileBytesForTenant :one
SELECT COALESCE(sum(byte_size), 0)::bigint AS byte_size
FROM file_objects
WHERE tenant_id = $1
  AND deleted_at IS NULL;
