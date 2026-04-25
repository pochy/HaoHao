-- name: AuthenticateUser :one
SELECT id
FROM users
WHERE email = @email
  AND password_hash IS NOT NULL
  AND deactivated_at IS NULL
  AND password_hash = crypt(@password, password_hash)
LIMIT 1;

-- name: GetUserByEmail :one
SELECT
    id,
    public_id,
    email,
    display_name,
    deactivated_at,
    default_tenant_id
FROM users
WHERE email = $1
LIMIT 1;

-- name: GetUserByID :one
SELECT
    id,
    public_id,
    email,
    display_name,
    deactivated_at,
    default_tenant_id
FROM users
WHERE id = $1
LIMIT 1;

-- name: GetUserByPublicID :one
SELECT
    id,
    public_id,
    email,
    display_name,
    deactivated_at,
    default_tenant_id
FROM users
WHERE public_id = $1
LIMIT 1;

-- name: CreateOIDCUser :one
INSERT INTO users (
    email,
    display_name,
    password_hash
) VALUES (
    $1,
    $2,
    NULL
)
RETURNING
    id,
    public_id,
    email,
    display_name,
    deactivated_at,
    default_tenant_id;

-- name: UpdateUserProfile :one
UPDATE users
SET email = $2,
    display_name = $3,
    updated_at = now()
WHERE id = $1
RETURNING
    id,
    public_id,
    email,
    display_name,
    deactivated_at,
    default_tenant_id;

-- name: SetUserDeactivatedAt :one
UPDATE users
SET deactivated_at = $2,
    updated_at = now()
WHERE id = $1
RETURNING
    id,
    public_id,
    email,
    display_name,
    deactivated_at,
    default_tenant_id;

-- name: SetUserDefaultTenant :one
UPDATE users
SET default_tenant_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING
    id,
    public_id,
    email,
    display_name,
    deactivated_at,
    default_tenant_id;
