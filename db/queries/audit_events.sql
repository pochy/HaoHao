-- name: CreateAuditEvent :one
INSERT INTO audit_events (
    actor_type,
    actor_user_id,
    actor_machine_client_id,
    tenant_id,
    action,
    target_type,
    target_id,
    request_id,
    client_ip,
    user_agent,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11
)
RETURNING
    id,
    public_id,
    actor_type,
    actor_user_id,
    actor_machine_client_id,
    tenant_id,
    action,
    target_type,
    target_id,
    request_id,
    client_ip,
    user_agent,
    metadata,
    occurred_at,
    created_at;

-- name: ListRecentAuditEvents :many
SELECT
    id,
    public_id,
    actor_type,
    actor_user_id,
    actor_machine_client_id,
    tenant_id,
    action,
    target_type,
    target_id,
    request_id,
    client_ip,
    user_agent,
    metadata,
    occurred_at,
    created_at
FROM audit_events
ORDER BY occurred_at DESC, id DESC
LIMIT $1;

-- name: ListAuditEventsByTenantID :many
SELECT
    id,
    public_id,
    actor_type,
    actor_user_id,
    actor_machine_client_id,
    tenant_id,
    action,
    target_type,
    target_id,
    request_id,
    client_ip,
    user_agent,
    metadata,
    occurred_at,
    created_at
FROM audit_events
WHERE tenant_id = $1
ORDER BY occurred_at DESC, id DESC
LIMIT $2;

-- name: ListAuditEventsByTarget :many
SELECT
    id,
    public_id,
    actor_type,
    actor_user_id,
    actor_machine_client_id,
    tenant_id,
    action,
    target_type,
    target_id,
    request_id,
    client_ip,
    user_agent,
    metadata,
    occurred_at,
    created_at
FROM audit_events
WHERE target_type = $1
  AND target_id = $2
ORDER BY occurred_at DESC, id DESC
LIMIT $3;
