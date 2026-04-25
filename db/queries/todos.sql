-- name: ListTodosByTenantID :many
SELECT
    id,
    public_id,
    tenant_id,
    created_by_user_id,
    title,
    completed,
    created_at,
    updated_at
FROM todos
WHERE tenant_id = $1
ORDER BY created_at DESC, id DESC;

-- name: GetTodoByPublicIDForTenant :one
SELECT
    id,
    public_id,
    tenant_id,
    created_by_user_id,
    title,
    completed,
    created_at,
    updated_at
FROM todos
WHERE public_id = $1
  AND tenant_id = $2
LIMIT 1;

-- name: CreateTodo :one
INSERT INTO todos (
    tenant_id,
    created_by_user_id,
    title
) VALUES (
    $1,
    $2,
    $3
)
RETURNING
    id,
    public_id,
    tenant_id,
    created_by_user_id,
    title,
    completed,
    created_at,
    updated_at;

-- name: UpdateTodoByPublicIDForTenant :one
UPDATE todos
SET
    title = $3,
    completed = $4,
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
RETURNING
    id,
    public_id,
    tenant_id,
    created_by_user_id,
    title,
    completed,
    created_at,
    updated_at;

-- name: DeleteTodoByPublicIDForTenant :execrows
DELETE FROM todos
WHERE public_id = $1
  AND tenant_id = $2;
