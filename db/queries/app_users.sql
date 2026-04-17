-- name: GetAppUserByPublicID :one
SELECT id, public_id, zitadel_subject, display_name, created_at, updated_at
FROM app_users
WHERE public_id = $1;

