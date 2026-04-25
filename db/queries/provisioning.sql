-- name: UpsertProvisioningSyncState :exec
INSERT INTO provisioning_sync_state (
    source,
    cursor_text,
    last_synced_at,
    last_error_code,
    last_error_message,
    failed_count
) VALUES (
    $1,
    $2,
    now(),
    $3,
    $4,
    $5
)
ON CONFLICT (source) DO UPDATE
SET cursor_text = EXCLUDED.cursor_text,
    last_synced_at = EXCLUDED.last_synced_at,
    last_error_code = EXCLUDED.last_error_code,
    last_error_message = EXCLUDED.last_error_message,
    failed_count = EXCLUDED.failed_count,
    updated_at = now();

-- name: ListDeactivatedUsersWithActiveGrants :many
SELECT DISTINCT
    u.id,
    u.public_id,
    u.email,
    u.display_name,
    u.deactivated_at,
    u.default_tenant_id
FROM users u
JOIN oauth_user_grants g ON g.user_id = u.id
WHERE u.deactivated_at IS NOT NULL
  AND g.revoked_at IS NULL
ORDER BY u.id;
