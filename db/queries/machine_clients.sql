-- name: ListMachineClients :many
SELECT
    id,
    provider,
    provider_client_id,
    display_name,
    default_tenant_id,
    allowed_scopes,
    active,
    created_at,
    updated_at
FROM machine_clients
ORDER BY provider, display_name, id;

-- name: GetMachineClientByID :one
SELECT
    id,
    provider,
    provider_client_id,
    display_name,
    default_tenant_id,
    allowed_scopes,
    active,
    created_at,
    updated_at
FROM machine_clients
WHERE id = $1
LIMIT 1;

-- name: GetMachineClientByProviderClientID :one
SELECT
    id,
    provider,
    provider_client_id,
    display_name,
    default_tenant_id,
    allowed_scopes,
    active,
    created_at,
    updated_at
FROM machine_clients
WHERE provider = $1
  AND provider_client_id = $2
LIMIT 1;

-- name: CreateMachineClient :one
INSERT INTO machine_clients (
    provider,
    provider_client_id,
    display_name,
    default_tenant_id,
    allowed_scopes,
    active
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING
    id,
    provider,
    provider_client_id,
    display_name,
    default_tenant_id,
    allowed_scopes,
    active,
    created_at,
    updated_at;

-- name: UpdateMachineClient :one
UPDATE machine_clients
SET provider_client_id = $2,
    display_name = $3,
    default_tenant_id = $4,
    allowed_scopes = $5,
    active = $6,
    updated_at = now()
WHERE id = $1
RETURNING
    id,
    provider,
    provider_client_id,
    display_name,
    default_tenant_id,
    allowed_scopes,
    active,
    created_at,
    updated_at;

-- name: DisableMachineClient :one
UPDATE machine_clients
SET active = false,
    updated_at = now()
WHERE id = $1
RETURNING
    id,
    provider,
    provider_client_id,
    display_name,
    default_tenant_id,
    allowed_scopes,
    active,
    created_at,
    updated_at;
