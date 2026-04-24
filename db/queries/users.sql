-- name: AuthenticateUser :one
SELECT id
FROM users
WHERE email = @email
  AND password_hash = crypt(@password, password_hash)
LIMIT 1;

-- name: GetUserByID :one
SELECT
    id,
    public_id,
    email,
    display_name
FROM users
WHERE id = $1
LIMIT 1;

