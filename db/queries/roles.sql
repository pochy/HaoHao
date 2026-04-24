-- name: GetRolesByCode :many
SELECT
    id,
    code
FROM roles
WHERE code = ANY($1::text[])
ORDER BY code;

-- name: ListRoleCodesByUserID :many
SELECT r.code
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
WHERE ur.user_id = $1
ORDER BY r.code;

-- name: DeleteUserRolesByUserID :exec
DELETE FROM user_roles
WHERE user_id = $1;

-- name: DeleteUserRolesExcluding :exec
DELETE FROM user_roles
WHERE user_id = $1
  AND NOT (role_id = ANY($2::bigint[]));

-- name: AssignUserRole :exec
INSERT INTO user_roles (
    user_id,
    role_id
) VALUES (
    $1,
    $2
)
ON CONFLICT (user_id, role_id) DO NOTHING;
