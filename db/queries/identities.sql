-- name: GetUserByProviderSubject :one
SELECT
    u.id,
    u.public_id,
    u.email,
    u.display_name
FROM user_identities ui
JOIN users u ON u.id = ui.user_id
WHERE ui.provider = $1
  AND ui.subject = $2
LIMIT 1;

-- name: CreateUserIdentity :exec
INSERT INTO user_identities (
    user_id,
    provider,
    subject,
    email,
    email_verified
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
);

-- name: UpdateUserIdentityProfile :exec
UPDATE user_identities
SET email = $3,
    email_verified = $4,
    updated_at = now()
WHERE provider = $1
  AND subject = $2;
