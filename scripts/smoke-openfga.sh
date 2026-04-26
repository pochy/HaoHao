#!/usr/bin/env bash
set -euo pipefail

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
DATABASE_URL="${DATABASE_URL:?DATABASE_URL is required for smoke-openfga user setup}"
OWNER_EMAIL="${OPENFGA_SMOKE_OWNER_EMAIL:-drive-owner@example.com}"
VIEWER_EMAIL="${OPENFGA_SMOKE_VIEWER_EMAIL:-drive-viewer@example.com}"
EDITOR_EMAIL="${OPENFGA_SMOKE_EDITOR_EMAIL:-drive-editor@example.com}"
PASSWORD="${OPENFGA_SMOKE_PASSWORD:-changeme123}"
TENANT_SLUG="${OPENFGA_SMOKE_TENANT_SLUG:-acme}"
RUN_ID="openfga-smoke-$(date +%s)-$$"

OWNER_COOKIE="$(mktemp)"
VIEWER_COOKIE="$(mktemp)"
EDITOR_COOKIE="$(mktemp)"
PUBLIC_COOKIE="$(mktemp)"
UPLOAD_FILE="$(mktemp)"
OVERWRITE_FILE="$(mktemp)"
GROUP_FILE="$(mktemp)"
BODY_FILE="$(mktemp)"
trap 'rm -f "$OWNER_COOKIE" "$VIEWER_COOKIE" "$EDITOR_COOKIE" "$PUBLIC_COOKIE" "$UPLOAD_FILE" "$OVERWRITE_FILE" "$GROUP_FILE" "$BODY_FILE"' EXIT

command -v psql >/dev/null
command -v curl >/dev/null
command -v rg >/dev/null

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO roles (code) VALUES ('todo_user') ON CONFLICT (code) DO NOTHING;
INSERT INTO tenants (slug, display_name)
VALUES ('$TENANT_SLUG', 'OpenFGA Smoke Tenant')
ON CONFLICT (slug) DO UPDATE SET active = true, updated_at = now();

INSERT INTO users (email, display_name, password_hash)
VALUES
  ('$OWNER_EMAIL', 'Drive Owner Smoke', crypt('$PASSWORD', gen_salt('bf'))),
  ('$VIEWER_EMAIL', 'Drive Viewer Smoke', crypt('$PASSWORD', gen_salt('bf'))),
  ('$EDITOR_EMAIL', 'Drive Editor Smoke', crypt('$PASSWORD', gen_salt('bf')))
ON CONFLICT (email) DO UPDATE
SET display_name = EXCLUDED.display_name,
    password_hash = EXCLUDED.password_hash,
    deactivated_at = NULL,
    updated_at = now();

UPDATE users
SET default_tenant_id = (SELECT id FROM tenants WHERE slug = '$TENANT_SLUG'),
    updated_at = now()
WHERE email IN ('$OWNER_EMAIL', '$VIEWER_EMAIL', '$EDITOR_EMAIL');

INSERT INTO tenant_memberships (user_id, tenant_id, role_id, source, active)
SELECT u.id, t.id, r.id, 'local_override', true
FROM users u
JOIN tenants t ON t.slug = '$TENANT_SLUG'
JOIN roles r ON r.code = 'todo_user'
WHERE u.email IN ('$OWNER_EMAIL', '$VIEWER_EMAIL', '$EDITOR_EMAIL')
ON CONFLICT (user_id, tenant_id, role_id, source) DO UPDATE
SET active = true,
    updated_at = now();
SQL

viewer_public_id="$(psql "$DATABASE_URL" -At -c "SELECT public_id FROM users WHERE email = '$VIEWER_EMAIL' LIMIT 1")"
editor_public_id="$(psql "$DATABASE_URL" -At -c "SELECT public_id FROM users WHERE email = '$EDITOR_EMAIL' LIMIT 1")"
if [[ -z "$viewer_public_id" || -z "$editor_public_id" ]]; then
  echo "failed to resolve smoke user public ids" >&2
  exit 1
fi

login() {
  local email="$1"
  local jar="$2"
  curl -fsS -c "$jar" -b "$jar" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$email\",\"password\":\"$PASSWORD\"}" \
    "$BASE_URL/api/v1/login" >/dev/null
  curl -fsS -c "$jar" -b "$jar" "$BASE_URL/api/v1/csrf" >/dev/null
  local csrf
  csrf="$(awk '$6 == "XSRF-TOKEN" { print $7 }' "$jar" | tail -n 1)"
  if [[ -z "$csrf" ]]; then
    echo "missing csrf token for $email" >&2
    exit 1
  fi
  curl -fsS -c "$jar" -b "$jar" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $csrf" \
    -d "{\"tenantSlug\":\"$TENANT_SLUG\"}" \
    "$BASE_URL/api/v1/session/tenant" >/dev/null
  printf '%s' "$csrf"
}

extract_json_string() {
  local key="$1"
  sed -n "s/.*\"$key\":\"\\([^\"]*\\)\".*/\\1/p"
}

extract_json_number() {
  local key="$1"
  sed -n "s/.*\"$key\":\\([0-9][0-9]*\\).*/\\1/p"
}

expect_status() {
  local expected="$1"
  shift
  local status
  status="$(curl -sS -o "$BODY_FILE" -w '%{http_code}' "$@")"
  if [[ "$status" != "$expected" ]]; then
    echo "expected HTTP $expected but got $status: $(cat "$BODY_FILE")" >&2
    exit 1
  fi
}

owner_csrf="$(login "$OWNER_EMAIL" "$OWNER_COOKIE")"
viewer_csrf="$(login "$VIEWER_EMAIL" "$VIEWER_COOKIE")"
editor_csrf="$(login "$EDITOR_EMAIL" "$EDITOR_COOKIE")"

folder_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $owner_csrf" \
  -d "{\"name\":\"$RUN_ID folder\"}" \
  "$BASE_URL/api/v1/drive/folders")"
folder_id="$(printf '%s' "$folder_response" | extract_json_string publicId)"
if [[ -z "$folder_id" ]]; then
  echo "missing folder id: $folder_response" >&2
  exit 1
fi

printf 'hello from openfga smoke\n' > "$UPLOAD_FILE"
upload_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
  -H "X-CSRF-Token: $owner_csrf" \
  -F "parentFolderPublicId=$folder_id" \
  -F "file=@$UPLOAD_FILE;filename=$RUN_ID.txt;type=text/plain" \
  "$BASE_URL/api/v1/drive/files")"
file_id="$(printf '%s' "$upload_response" | extract_json_string publicId)"
if [[ -z "$file_id" ]]; then
  echo "missing file id: $upload_response" >&2
  exit 1
fi

expect_status 403 -c "$VIEWER_COOKIE" -b "$VIEWER_COOKIE" "$BASE_URL/api/v1/drive/files/$file_id/content"

share_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $owner_csrf" \
  -d "{\"subjectType\":\"user\",\"subjectPublicId\":\"$viewer_public_id\",\"role\":\"viewer\"}" \
  "$BASE_URL/api/v1/drive/files/$file_id/shares")"
share_id="$(printf '%s' "$share_response" | extract_json_string publicId)"
if [[ -z "$share_id" ]]; then
  echo "missing share id: $share_response" >&2
  exit 1
fi

curl -fsS -c "$VIEWER_COOKIE" -b "$VIEWER_COOKIE" \
  "$BASE_URL/api/v1/drive/files/$file_id/content" | rg 'hello from openfga smoke' >/dev/null

expect_status 403 -X DELETE -c "$VIEWER_COOKIE" -b "$VIEWER_COOKIE" \
  -H "X-CSRF-Token: $viewer_csrf" \
  "$BASE_URL/api/v1/drive/files/$file_id"

curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $owner_csrf" \
  -d "{\"subjectType\":\"user\",\"subjectPublicId\":\"$editor_public_id\",\"role\":\"editor\"}" \
  "$BASE_URL/api/v1/drive/files/$file_id/shares" >/dev/null

curl -fsS -c "$EDITOR_COOKIE" -b "$EDITOR_COOKIE" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $editor_csrf" \
  -X PATCH \
  -d "{\"originalFilename\":\"$RUN_ID-renamed.txt\"}" \
  "$BASE_URL/api/v1/drive/files/$file_id" | rg "\"originalFilename\":\"$RUN_ID-renamed.txt\"" >/dev/null

printf 'overwritten by editor\n' > "$OVERWRITE_FILE"
curl -fsS -c "$EDITOR_COOKIE" -b "$EDITOR_COOKIE" \
  -H "X-CSRF-Token: $editor_csrf" \
  -X PUT \
  -F "file=@$OVERWRITE_FILE;filename=$RUN_ID-overwrite.txt;type=text/plain" \
  "$BASE_URL/api/v1/drive/files/$file_id/content" | rg "\"originalFilename\":\"$RUN_ID-overwrite.txt\"" >/dev/null

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
UPDATE file_objects
SET locked_at = now(),
    locked_by_user_id = (SELECT id FROM users WHERE email = '$OWNER_EMAIL'),
    lock_reason = 'manual_lock',
    updated_at = now()
WHERE public_id = '$file_id'::uuid;
SQL

expect_status 409 -X PUT -c "$EDITOR_COOKIE" -b "$EDITOR_COOKIE" \
  -H "X-CSRF-Token: $editor_csrf" \
  -F "file=@$OVERWRITE_FILE;filename=$RUN_ID-locked.txt;type=text/plain" \
  "$BASE_URL/api/v1/drive/files/$file_id/content"

expect_status 409 -X DELETE -c "$EDITOR_COOKIE" -b "$EDITOR_COOKIE" \
  -H "X-CSRF-Token: $editor_csrf" \
  "$BASE_URL/api/v1/drive/files/$file_id"

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
UPDATE file_objects
SET locked_at = NULL,
    locked_by_user_id = NULL,
    lock_reason = NULL,
    updated_at = now()
WHERE public_id = '$file_id'::uuid;
SQL

group_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $owner_csrf" \
  -d "{\"name\":\"$RUN_ID group\"}" \
  "$BASE_URL/api/v1/drive/groups")"
group_id="$(printf '%s' "$group_response" | extract_json_string publicId)"
if [[ -z "$group_id" ]]; then
  echo "missing group id: $group_response" >&2
  exit 1
fi

curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $owner_csrf" \
  -d "{\"userPublicId\":\"$viewer_public_id\"}" \
  "$BASE_URL/api/v1/drive/groups/$group_id/members" >/dev/null

group_folder_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $owner_csrf" \
  -d "{\"name\":\"$RUN_ID group folder\"}" \
  "$BASE_URL/api/v1/drive/folders")"
group_folder_id="$(printf '%s' "$group_folder_response" | extract_json_string publicId)"

printf 'group inherited access\n' > "$GROUP_FILE"
group_upload_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
  -H "X-CSRF-Token: $owner_csrf" \
  -F "parentFolderPublicId=$group_folder_id" \
  -F "file=@$GROUP_FILE;filename=$RUN_ID-group.txt;type=text/plain" \
  "$BASE_URL/api/v1/drive/files")"
group_file_id="$(printf '%s' "$group_upload_response" | extract_json_string publicId)"

curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $owner_csrf" \
  -d "{\"subjectType\":\"group\",\"subjectPublicId\":\"$group_id\",\"role\":\"viewer\"}" \
  "$BASE_URL/api/v1/drive/folders/$group_folder_id/shares" >/dev/null

curl -fsS -c "$VIEWER_COOKIE" -b "$VIEWER_COOKIE" \
  "$BASE_URL/api/v1/drive/files/$group_file_id/content" | rg 'group inherited access' >/dev/null

curl -fsS -c "$VIEWER_COOKIE" -b "$VIEWER_COOKIE" \
  "$BASE_URL/api/v1/drive/search?q=$RUN_ID-group" | rg "\"publicId\":\"$group_file_id\"" >/dev/null

expires_at="$(date -u -v+1H '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -d '+1 hour' '+%Y-%m-%dT%H:%M:%SZ')"
link_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $owner_csrf" \
  -d "{\"canDownload\":false,\"expiresAt\":\"$expires_at\"}" \
  "$BASE_URL/api/v1/drive/files/$file_id/share-links")"
link_id="$(printf '%s' "$link_response" | extract_json_string publicId)"
link_token="$(printf '%s' "$link_response" | extract_json_string token)"
if [[ -z "$link_id" || -z "$link_token" ]]; then
  echo "missing share link id/token: $link_response" >&2
  exit 1
fi

curl -fsS "$BASE_URL/api/public/drive/share-links/$link_token" | rg "\"publicId\":\"$file_id\"" >/dev/null
expect_status 403 "$BASE_URL/api/public/drive/share-links/$link_token/content"

if [[ "${RUN_DRIVE_PASSWORD_LINK_SMOKE:-0}" == "1" ]]; then
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO tenant_settings (tenant_id, file_quota_bytes, notifications_enabled, features)
SELECT id, 104857600, true, '{"drive":{"linkSharingEnabled":true,"publicLinksEnabled":true,"passwordProtectedLinksEnabled":true,"requireShareLinkPassword":true,"maxShareLinkTTLHours":168,"viewerDownloadEnabled":true}}'::jsonb
FROM tenants
WHERE slug = '$TENANT_SLUG'
ON CONFLICT (tenant_id) DO UPDATE
SET features = EXCLUDED.features,
    updated_at = now();
SQL
  password_link_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d "{\"canDownload\":true,\"expiresAt\":\"$expires_at\",\"password\":\"$PASSWORD\"}" \
    "$BASE_URL/api/v1/drive/files/$file_id/share-links")"
  password_link_token="$(printf '%s' "$password_link_response" | extract_json_string token)"
  if [[ -z "$password_link_token" ]]; then
    echo "missing password share link token" >&2
    exit 1
  fi
  curl -fsS "$BASE_URL/api/public/drive/share-links/$password_link_token" | rg '"passwordRequired":true' >/dev/null
  expect_status 403 "$BASE_URL/api/public/drive/share-links/$password_link_token/content"
  curl -fsS -c "$PUBLIC_COOKIE" -b "$PUBLIC_COOKIE" \
    -H 'Content-Type: application/json' \
    -d "{\"password\":\"$PASSWORD\"}" \
    "$BASE_URL/api/public/drive/share-links/$password_link_token/password" | rg '"verified":true' >/dev/null
  curl -fsS -c "$PUBLIC_COOKIE" -b "$PUBLIC_COOKIE" \
    "$BASE_URL/api/public/drive/share-links/$password_link_token/content" | rg 'overwritten by editor' >/dev/null
fi

if [[ "${RUN_OPENFGA_EXTERNAL_SHARE_SMOKE:-0}" == "1" ]]; then
  expect_status 403 -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d "{\"inviteeEmail\":\"external@example.com\",\"role\":\"viewer\"}" \
    "$BASE_URL/api/v1/drive/files/$file_id/invitations"

  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO tenant_settings (tenant_id, file_quota_bytes, notifications_enabled, features)
SELECT id, 104857600, true, '{"drive":{"linkSharingEnabled":true,"publicLinksEnabled":true,"externalUserSharingEnabled":true,"allowedExternalDomains":["example.com"],"blockedExternalDomains":["blocked.example.com"],"requireExternalShareApproval":true,"maxShareLinkTTLHours":168,"viewerDownloadEnabled":true}}'::jsonb
FROM tenants
WHERE slug = '$TENANT_SLUG'
ON CONFLICT (tenant_id) DO UPDATE
SET features = EXCLUDED.features,
    updated_at = now();
SQL
  expect_status 403 -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d "{\"inviteeEmail\":\"person@blocked.example.com\",\"role\":\"viewer\"}" \
    "$BASE_URL/api/v1/drive/files/$file_id/invitations"
  approval_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d "{\"inviteeEmail\":\"person@example.com\",\"role\":\"viewer\"}" \
    "$BASE_URL/api/v1/drive/files/$file_id/invitations")"
  printf '%s' "$approval_response" | rg '"status":"pending_approval"' >/dev/null
fi

if [[ "${RUN_OPENFGA_WORKSPACE_SMOKE:-0}" == "1" ]]; then
  workspace_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d "{\"name\":\"$RUN_ID workspace\"}" \
    "$BASE_URL/api/v1/drive/workspaces")"
  workspace_id="$(printf '%s' "$workspace_response" | extract_json_string publicId)"
  if [[ -z "$workspace_id" ]]; then
    echo "missing workspace id: $workspace_response" >&2
    exit 1
  fi
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    "$BASE_URL/api/v1/drive/workspaces" | rg "\"publicId\":\"$workspace_id\"" >/dev/null
  workspace_folder_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d "{\"name\":\"$RUN_ID workspace folder\",\"workspacePublicId\":\"$workspace_id\"}" \
    "$BASE_URL/api/v1/drive/folders")"
  workspace_folder_id="$(printf '%s' "$workspace_folder_response" | extract_json_string publicId)"
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    "$BASE_URL/api/v1/drive/items?workspacePublicId=$workspace_id" | rg "\"publicId\":\"$workspace_folder_id\"" >/dev/null
fi

if [[ "${RUN_OPENFGA_PUBLIC_EDITOR_LINK_SMOKE:-0}" == "1" ]]; then
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO tenant_settings (tenant_id, file_quota_bytes, notifications_enabled, features)
SELECT id, 104857600, true, '{"drive":{"linkSharingEnabled":true,"publicLinksEnabled":true,"passwordProtectedLinksEnabled":true,"anonymousEditorLinksEnabled":true,"anonymousEditorLinksRequirePassword":true,"anonymousEditorLinkMaxTTLMinutes":60,"maxShareLinkTTLHours":168,"viewerDownloadEnabled":true}}'::jsonb
FROM tenants
WHERE slug = '$TENANT_SLUG'
ON CONFLICT (tenant_id) DO UPDATE
SET features = EXCLUDED.features,
    updated_at = now();
SQL
  editor_link_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d "{\"role\":\"editor\",\"canDownload\":false,\"expiresAt\":\"$expires_at\",\"password\":\"$PASSWORD\"}" \
    "$BASE_URL/api/v1/drive/files/$file_id/share-links")"
  editor_link_token="$(printf '%s' "$editor_link_response" | extract_json_string token)"
  if [[ -z "$editor_link_token" ]]; then
    echo "missing public editor link token" >&2
    exit 1
  fi
  curl -fsS -c "$PUBLIC_COOKIE" -b "$PUBLIC_COOKIE" \
    -H 'Content-Type: application/json' \
    -d "{\"password\":\"$PASSWORD\"}" \
    "$BASE_URL/api/public/drive/share-links/$editor_link_token/password" | rg '"verified":true' >/dev/null
  printf 'anonymous editor update\n' > "$OVERWRITE_FILE"
  curl -fsS -c "$PUBLIC_COOKIE" -b "$PUBLIC_COOKIE" \
    -X PUT \
    -F "file=@$OVERWRITE_FILE;filename=$RUN_ID-public-editor.txt;type=text/plain" \
    "$BASE_URL/api/public/drive/share-links/$editor_link_token/content" | rg "\"originalFilename\":\"$RUN_ID-public-editor.txt\"" >/dev/null
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    "$BASE_URL/api/v1/drive/files/$file_id/content" | rg 'anonymous editor update' >/dev/null
fi

if [[ "${RUN_DRIVE_SCAN_SMOKE:-0}" == "1" ]]; then
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO tenant_settings (tenant_id, file_quota_bytes, notifications_enabled, features)
SELECT id, 104857600, true, '{"drive":{"linkSharingEnabled":true,"publicLinksEnabled":true,"contentScanEnabled":true,"blockDownloadUntilScanComplete":true,"blockShareUntilScanComplete":true,"maxShareLinkTTLHours":168,"viewerDownloadEnabled":true}}'::jsonb
FROM tenants
WHERE slug = '$TENANT_SLUG'
ON CONFLICT (tenant_id) DO UPDATE
SET features = EXCLUDED.features,
    updated_at = now();

UPDATE file_objects
SET scan_status = 'pending',
    updated_at = now()
WHERE public_id = '$file_id'::uuid;
SQL
  expect_status 403 -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" "$BASE_URL/api/v1/drive/files/$file_id/content"
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
UPDATE file_objects
SET scan_status = 'clean',
    scanned_at = now(),
    updated_at = now()
WHERE public_id = '$file_id'::uuid;
SQL
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    "$BASE_URL/api/v1/drive/files/$file_id/content" >/dev/null
fi

if [[ "${RUN_DRIVE_PLAN_ENFORCEMENT_SMOKE:-0}" == "1" ]]; then
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO tenant_settings (tenant_id, file_quota_bytes, notifications_enabled, features)
SELECT id, 104857600, true, '{"drive":{"linkSharingEnabled":true,"publicLinksEnabled":true,"maxPublicLinkCount":1,"maxShareLinkTTLHours":168,"viewerDownloadEnabled":true,"planCode":"standard"}}'::jsonb
FROM tenants
WHERE slug = '$TENANT_SLUG'
ON CONFLICT (tenant_id) DO UPDATE
SET features = EXCLUDED.features,
    updated_at = now();
SQL
  expect_status 403 -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d "{\"canDownload\":true,\"expiresAt\":\"$expires_at\"}" \
    "$BASE_URL/api/v1/drive/files/$file_id/share-links"
fi

if [[ "${RUN_DRIVE_ADMIN_CONTENT_ACCESS_SMOKE:-0}" == "1" || "${RUN_DRIVE_DRIFT_SMOKE:-0}" == "1" || "${RUN_DRIVE_STORAGE_CONSISTENCY_SMOKE:-0}" == "1" || "${RUN_DRIVE_STORAGE_DRIVER_SMOKE:-0}" == "1" ]]; then
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO roles (code) VALUES ('tenant_admin'), ('drive_content_admin') ON CONFLICT (code) DO NOTHING;
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.code IN ('tenant_admin', 'drive_content_admin')
WHERE u.email = '$OWNER_EMAIL'
ON CONFLICT DO NOTHING;

INSERT INTO tenant_settings (tenant_id, file_quota_bytes, notifications_enabled, features)
SELECT id, 104857600, true, '{"drive":{"linkSharingEnabled":true,"publicLinksEnabled":true,"adminContentAccessMode":"break_glass","maxShareLinkTTLHours":168,"viewerDownloadEnabled":true}}'::jsonb
FROM tenants
WHERE slug = '$TENANT_SLUG'
ON CONFLICT (tenant_id) DO UPDATE
SET features = EXCLUDED.features,
    updated_at = now();
SQL
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    "$BASE_URL/api/v1/admin/tenants/$TENANT_SLUG/drive/operations/health" | rg '"workspaceCount":' >/dev/null
  if [[ "${RUN_DRIVE_ADMIN_CONTENT_ACCESS_SMOKE:-0}" == "1" ]]; then
    curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
      -H 'Content-Type: application/json' \
      -H "X-CSRF-Token: $owner_csrf" \
      -d '{"reason":"smoke verification","reasonCategory":"manual"}' \
      "$BASE_URL/api/v1/admin/tenants/$TENANT_SLUG/drive/content-access-sessions" | rg '"reasonCategory":"manual"' >/dev/null
    curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
      "$BASE_URL/api/v1/admin/tenants/$TENANT_SLUG/drive/files/$file_id/metadata" | rg "\"publicId\":\"$file_id\"" >/dev/null
  fi
fi

if [[ "${RUN_DRIVE_SEARCH_SMOKE:-0}" == "1" || "${RUN_DRIVE_COLLAB_SMOKE:-0}" == "1" || "${RUN_DRIVE_DESKTOP_SYNC_SMOKE:-0}" == "1" || "${RUN_DRIVE_MOBILE_OFFLINE_SMOKE:-0}" == "1" || "${RUN_DRIVE_CMK_SMOKE:-0}" == "1" || "${RUN_DRIVE_RESIDENCY_SMOKE:-0}" == "1" || "${RUN_DRIVE_LEGAL_DISCOVERY_SMOKE:-0}" == "1" || "${RUN_DRIVE_CLEAN_ROOM_SMOKE:-0}" == "1" ]]; then
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO roles (code)
VALUES ('tenant_admin'), ('drive_security_admin'), ('drive_sync_admin'), ('legal_admin'), ('legal_exporter'), ('legal_reviewer'), ('clean_room_admin')
ON CONFLICT (code) DO NOTHING;

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.code IN ('tenant_admin', 'drive_security_admin', 'drive_sync_admin', 'legal_admin', 'legal_exporter', 'legal_reviewer', 'clean_room_admin')
WHERE u.email = '$OWNER_EMAIL'
ON CONFLICT DO NOTHING;
SQL
fi

if [[ "${RUN_DRIVE_SEARCH_SMOKE:-0}" == "1" ]]; then
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO tenant_settings (tenant_id, file_quota_bytes, notifications_enabled, features)
SELECT id, 104857600, true, '{"drive":{"linkSharingEnabled":true,"publicLinksEnabled":true,"searchEnabled":true,"maxShareLinkTTLHours":168,"viewerDownloadEnabled":true}}'::jsonb
FROM tenants
WHERE slug = '$TENANT_SLUG'
ON CONFLICT (tenant_id) DO UPDATE
SET features = EXCLUDED.features,
    updated_at = now();
SQL
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -X POST \
    "$BASE_URL/api/v1/admin/tenants/$TENANT_SLUG/drive/search/index/rebuild" | rg '"indexed":' >/dev/null
  if ! curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    "$BASE_URL/api/v1/drive/search/documents?q=anonymous" | rg "\"publicId\":\"$file_id\"" >/dev/null; then
    curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
      "$BASE_URL/api/v1/drive/search/documents?q=overwritten" | rg "\"publicId\":\"$file_id\"" >/dev/null
  fi
fi

if [[ "${RUN_DRIVE_COLLAB_SMOKE:-0}" == "1" ]]; then
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO tenant_settings (tenant_id, file_quota_bytes, notifications_enabled, features)
SELECT id, 104857600, true, '{"drive":{"linkSharingEnabled":true,"publicLinksEnabled":true,"searchEnabled":true,"collaborationEnabled":true,"contentScanEnabled":false,"maxShareLinkTTLHours":168,"viewerDownloadEnabled":true}}'::jsonb
FROM tenants
WHERE slug = '$TENANT_SLUG'
ON CONFLICT (tenant_id) DO UPDATE
SET features = EXCLUDED.features,
    updated_at = now();
SQL
  edit_session_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -X POST \
    "$BASE_URL/api/v1/drive/files/$file_id/edit-sessions")"
  edit_session_id="$(printf '%s' "$edit_session_response" | extract_json_string publicId)"
  base_revision="$(printf '%s' "$edit_session_response" | extract_json_number baseRevision)"
  if [[ -z "$edit_session_id" ]]; then
    echo "missing edit session id: $edit_session_response" >&2
    exit 1
  fi
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -X PUT \
    -d "{\"content\":\"collab update $RUN_ID\",\"expectedRevision\":${base_revision:-0},\"filename\":\"$RUN_ID-collab.txt\",\"contentType\":\"text/plain\"}" \
    "$BASE_URL/api/v1/drive/files/$file_id/edit-sessions/$edit_session_id/content" | rg '"revision":' >/dev/null
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    "$BASE_URL/api/v1/drive/files/$file_id/content" | rg "collab update $RUN_ID" >/dev/null
fi

if [[ "${RUN_DRIVE_DESKTOP_SYNC_SMOKE:-0}" == "1" ]]; then
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO tenant_settings (tenant_id, file_quota_bytes, notifications_enabled, features)
SELECT id, 104857600, true, '{"drive":{"linkSharingEnabled":true,"publicLinksEnabled":true,"syncEnabled":true,"maxShareLinkTTLHours":168,"viewerDownloadEnabled":true}}'::jsonb
FROM tenants
WHERE slug = '$TENANT_SLUG'
ON CONFLICT (tenant_id) DO UPDATE
SET features = EXCLUDED.features,
    updated_at = now();
SQL
  sync_device_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d "{\"deviceName\":\"$RUN_ID desktop\",\"platform\":\"desktop\"}" \
    "$BASE_URL/api/v1/drive-sync/devices/register")"
  sync_device_id="$(printf '%s' "$sync_device_response" | extract_json_string publicId)"
  sync_device_token="$(printf '%s' "$sync_device_response" | extract_json_string token)"
  if [[ -z "$sync_device_id" || -z "$sync_device_token" ]]; then
    echo "missing sync device token: $sync_device_response" >&2
    exit 1
  fi
  curl -fsS -H "Authorization: Bearer $sync_device_token" \
    "$BASE_URL/api/v1/drive-sync/delta?cursor=0" | rg '"events":' >/dev/null
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d '{"reason":"smoke remote wipe"}' \
    "$BASE_URL/api/v1/drive-sync/devices/$sync_device_id/revoke" >/dev/null
  curl -fsS -H "Authorization: Bearer $sync_device_token" \
    "$BASE_URL/api/v1/drive-sync/delta?cursor=0" | rg '"remoteWipe":true' >/dev/null
fi

if [[ "${RUN_DRIVE_MOBILE_OFFLINE_SMOKE:-0}" == "1" ]]; then
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO tenant_settings (tenant_id, file_quota_bytes, notifications_enabled, features)
SELECT id, 104857600, true, '{"drive":{"linkSharingEnabled":true,"publicLinksEnabled":true,"syncEnabled":true,"mobileOfflineEnabled":true,"offlineCacheAllowed":true,"maxShareLinkTTLHours":168,"viewerDownloadEnabled":true}}'::jsonb
FROM tenants
WHERE slug = '$TENANT_SLUG'
ON CONFLICT (tenant_id) DO UPDATE
SET features = EXCLUDED.features,
    updated_at = now();
SQL
  mobile_device_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d "{\"deviceName\":\"$RUN_ID mobile\",\"platform\":\"mobile\"}" \
    "$BASE_URL/api/v1/drive-sync/devices/register")"
  mobile_device_token="$(printf '%s' "$mobile_device_response" | extract_json_string token)"
  curl -fsS -H "Authorization: Bearer $mobile_device_token" \
    -H 'Content-Type: application/json' \
    -d "{\"operations\":[{\"operationType\":\"rename_file\",\"resourceType\":\"file\",\"resourcePublicId\":\"$file_id\",\"baseRevision\":0,\"name\":\"$RUN_ID-mobile.txt\"}]}" \
    "$BASE_URL/api/v1/drive-mobile/offline/replay" | rg '"applied":1' >/dev/null
fi

if [[ "${RUN_DRIVE_CMK_SMOKE:-0}" == "1" ]]; then
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO tenant_settings (tenant_id, file_quota_bytes, notifications_enabled, features)
SELECT id, 104857600, true, '{"drive":{"linkSharingEnabled":true,"publicLinksEnabled":true,"cmkEnabled":true,"encryptionMode":"tenant_managed","maxShareLinkTTLHours":168,"viewerDownloadEnabled":true}}'::jsonb
FROM tenants
WHERE slug = '$TENANT_SLUG'
ON CONFLICT (tenant_id) DO UPDATE
SET features = EXCLUDED.features,
    updated_at = now();
SQL
  cmk_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -X PUT \
    -d '{"mode":"tenant_managed","provider":"local-fake","keyRef":"arn:aws:kms:ap-northeast-1:123456789012:key/smoke"}' \
    "$BASE_URL/api/v1/admin/tenants/$TENANT_SLUG/drive/security/encryption-policy")"
  kms_key_id="$(printf '%s' "$cmk_response" | extract_json_string keyPublicId)"
  if [[ -z "$kms_key_id" ]]; then
    echo "missing kms key id: $cmk_response" >&2
    exit 1
  fi
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" "$BASE_URL/api/v1/drive/files/$file_id/content" >/dev/null
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -X PATCH \
    -d '{"status":"unavailable"}' \
    "$BASE_URL/api/v1/admin/tenants/$TENANT_SLUG/drive/security/kms-keys/$kms_key_id" | rg '"keyStatus":"unavailable"' >/dev/null
  expect_status 403 -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" "$BASE_URL/api/v1/drive/files/$file_id/content"
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -X PATCH \
    -d '{"status":"active"}' \
    "$BASE_URL/api/v1/admin/tenants/$TENANT_SLUG/drive/security/kms-keys/$kms_key_id" | rg '"keyStatus":"active"' >/dev/null
fi

if [[ "${RUN_DRIVE_RESIDENCY_SMOKE:-0}" == "1" ]]; then
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO tenant_settings (tenant_id, file_quota_bytes, notifications_enabled, features)
SELECT id, 104857600, true, '{"drive":{"linkSharingEnabled":true,"publicLinksEnabled":true,"dataResidencyEnabled":true,"primaryRegion":"ap-northeast-1","allowedRegions":["ap-northeast-1"],"maxShareLinkTTLHours":168,"viewerDownloadEnabled":true}}'::jsonb
FROM tenants
WHERE slug = '$TENANT_SLUG'
ON CONFLICT (tenant_id) DO UPDATE
SET features = EXCLUDED.features,
    updated_at = now();
SQL
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -X PUT \
    -d '{"primaryRegion":"ap-northeast-1","allowedRegions":["ap-northeast-1"],"replicationMode":"none","indexRegion":"same_as_primary","backupRegion":"same_jurisdiction","status":"active"}' \
    "$BASE_URL/api/v1/admin/tenants/$TENANT_SLUG/drive/residency-policy" | rg '"primaryRegion":"ap-northeast-1"' >/dev/null
fi

if [[ "${RUN_DRIVE_LEGAL_DISCOVERY_SMOKE:-0}" == "1" ]]; then
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO tenant_settings (tenant_id, file_quota_bytes, notifications_enabled, features)
SELECT id, 104857600, true, '{"drive":{"linkSharingEnabled":true,"publicLinksEnabled":true,"legalDiscoveryEnabled":true,"maxShareLinkTTLHours":168,"viewerDownloadEnabled":true}}'::jsonb
FROM tenants
WHERE slug = '$TENANT_SLUG'
ON CONFLICT (tenant_id) DO UPDATE
SET features = EXCLUDED.features,
    updated_at = now();
SQL
  legal_case_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d "{\"name\":\"$RUN_ID case\",\"description\":\"smoke\"}" \
    "$BASE_URL/api/v1/admin/tenants/$TENANT_SLUG/drive/legal/cases")"
  legal_case_id="$(printf '%s' "$legal_case_response" | extract_json_string publicId)"
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d "{\"filePublicId\":\"$file_id\",\"reason\":\"smoke legal hold\"}" \
    "$BASE_URL/api/v1/admin/tenants/$TENANT_SLUG/drive/legal/cases/$legal_case_id/files" >/dev/null
  expect_status 409 -X DELETE -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H "X-CSRF-Token: $owner_csrf" \
    "$BASE_URL/api/v1/drive/files/$file_id"
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -X POST \
    "$BASE_URL/api/v1/admin/tenants/$TENANT_SLUG/drive/legal/cases/$legal_case_id/exports" | rg '"status":"pending_approval"' >/dev/null
fi

if [[ "${RUN_DRIVE_CLEAN_ROOM_SMOKE:-0}" == "1" ]]; then
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 >/dev/null <<SQL
INSERT INTO tenant_settings (tenant_id, file_quota_bytes, notifications_enabled, features)
SELECT id, 104857600, true, '{"drive":{"linkSharingEnabled":true,"publicLinksEnabled":true,"cleanRoomEnabled":true,"cleanRoomRawExportEnabled":false,"maxShareLinkTTLHours":168,"viewerDownloadEnabled":true}}'::jsonb
FROM tenants
WHERE slug = '$TENANT_SLUG'
ON CONFLICT (tenant_id) DO UPDATE
SET features = EXCLUDED.features,
    updated_at = now();
SQL
  clean_room_response="$(curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d "{\"name\":\"$RUN_ID clean room\"}" \
    "$BASE_URL/api/v1/admin/tenants/$TENANT_SLUG/drive/clean-rooms")"
  clean_room_id="$(printf '%s' "$clean_room_response" | extract_json_string publicId)"
  if [[ -z "$clean_room_id" ]]; then
    echo "missing clean room id: $clean_room_response" >&2
    exit 1
  fi
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d "{\"userPublicId\":\"$viewer_public_id\",\"role\":\"participant\"}" \
    "$BASE_URL/api/v1/admin/tenants/$TENANT_SLUG/drive/clean-rooms/$clean_room_id/participants" >/dev/null
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d "{\"filePublicId\":\"$file_id\"}" \
    "$BASE_URL/api/v1/admin/tenants/$TENANT_SLUG/drive/clean-rooms/$clean_room_id/datasets" | rg '"status":"submitted"' >/dev/null
  curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $owner_csrf" \
    -d '{"rawDatasetExport":true}' \
    "$BASE_URL/api/v1/admin/tenants/$TENANT_SLUG/drive/clean-rooms/$clean_room_id/exports" | rg '"status":"denied"' >/dev/null
fi

curl -fsS -c "$OWNER_COOKIE" -b "$OWNER_COOKIE" \
  -H "X-CSRF-Token: $owner_csrf" \
  -X DELETE \
  "$BASE_URL/api/v1/drive/share-links/$link_id" >/dev/null
expect_status 404 "$BASE_URL/api/public/drive/share-links/$link_token"

audit_count="$(psql "$DATABASE_URL" -At -c "SELECT count(*) FROM audit_events WHERE action IN ('drive.share.create','drive.share_link.create','drive.authz.denied') AND occurred_at > now() - interval '10 minutes'")"
if [[ "${audit_count:-0}" -le 0 ]]; then
  echo "missing expected Drive audit events" >&2
  exit 1
fi

curl -fsS "$BASE_URL/metrics" | rg 'haohao_openfga_requests_total|haohao_drive_authz_denied_total' >/dev/null

echo "openfga smoke ok: $BASE_URL"
