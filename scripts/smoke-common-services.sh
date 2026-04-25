#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
COOKIE_JAR="$(mktemp)"
UPLOAD_FILE="$(mktemp)"
trap 'rm -f "$COOKIE_JAR" "$UPLOAD_FILE"' EXIT

curl -sS -D /tmp/haohao-p7-headers.$$ -o /dev/null "$BASE_URL/api/v1/session" || true
rg 'Content-Security-Policy|X-Content-Type-Options|Referrer-Policy|X-Frame-Options' /tmp/haohao-p7-headers.$$ >/dev/null
rm -f /tmp/haohao-p7-headers.$$

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"changeme123"}' \
  "$BASE_URL/api/v1/login" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/csrf" >/dev/null

csrf="$(awk '$6 == "XSRF-TOKEN" { print $7 }' "$COOKIE_JAR" | tail -n 1)"
if [[ -z "$csrf" ]]; then
  echo "missing csrf token" >&2
  exit 1
fi

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d '{"tenantSlug":"acme"}' \
  "$BASE_URL/api/v1/session/tenant" | rg '"slug":"acme"' >/dev/null

title="P7 common smoke $(date +%s)-$$"
body="{\"customerName\":\"Acme\",\"title\":\"$title\",\"body\":\"Created by P7 smoke\",\"source\":\"support\",\"priority\":\"medium\",\"status\":\"new\"}"
idempotency_key="p7-smoke-$(date +%s)-$$"

create_response="$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -H "Idempotency-Key: $idempotency_key" \
  -d "$body" \
  "$BASE_URL/api/v1/customer-signals")"

duplicate_response="$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -H "Idempotency-Key: $idempotency_key" \
  -d "$body" \
  "$BASE_URL/api/v1/customer-signals")"

signal_public_id="$(printf '%s' "$create_response" | sed -n 's/.*"publicId":"\([^"]*\)".*/\1/p')"
duplicate_public_id="$(printf '%s' "$duplicate_response" | sed -n 's/.*"publicId":"\([^"]*\)".*/\1/p')"
if [[ -z "$signal_public_id" || "$signal_public_id" != "$duplicate_public_id" ]]; then
  echo "idempotency replay failed: $create_response / $duplicate_response" >&2
  exit 1
fi

printf 'hello from p7 smoke\n' > "$UPLOAD_FILE"
upload_response="$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "X-CSRF-Token: $csrf" \
  -F "purpose=attachment" \
  -F "attachedToType=customer_signal" \
  -F "attachedToId=$signal_public_id" \
  -F "file=@$UPLOAD_FILE;filename=p7-smoke.txt;type=text/plain" \
  "$BASE_URL/api/v1/files")"

file_public_id="$(printf '%s' "$upload_response" | sed -n 's/.*"publicId":"\([^"]*\)".*/\1/p')"
if [[ -z "$file_public_id" ]]; then
  echo "missing file public id: $upload_response" >&2
  exit 1
fi

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/files?attachedToType=customer_signal&attachedToId=$signal_public_id" \
  | rg "\"publicId\":\"$file_public_id\"" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/files/$file_public_id" | rg 'hello from p7 smoke' >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "X-CSRF-Token: $csrf" \
  -X DELETE \
  "$BASE_URL/api/v1/files/$file_public_id" >/dev/null

settings_response="$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" "$BASE_URL/api/v1/admin/tenants/acme/settings")"
quota="$(printf '%s' "$settings_response" | sed -n 's/.*"fileQuotaBytes":\([0-9][0-9]*\).*/\1/p')"
if [[ -z "$quota" ]]; then
  echo "missing tenant settings: $settings_response" >&2
  exit 1
fi

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -X PUT \
  -d "{\"fileQuotaBytes\":$quota,\"rateLimitBrowserApiPerMinute\":120,\"notificationsEnabled\":true,\"features\":{}}" \
  "$BASE_URL/api/v1/admin/tenants/acme/settings" | rg '"notificationsEnabled":true' >/dev/null

invite_response="$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d '{"email":"demo@example.com","roleCodes":["todo_user"]}' \
  "$BASE_URL/api/v1/admin/tenants/acme/invitations")"
invite_token="$(printf '%s' "$invite_response" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')"
if [[ -z "$invite_token" ]]; then
  echo "missing invitation token: $invite_response" >&2
  exit 1
fi

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d "{\"token\":\"$invite_token\"}" \
  "$BASE_URL/api/v1/invitations/accept" | rg '"status":"accepted"' >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d '{"format":"json"}' \
  "$BASE_URL/api/v1/admin/tenants/acme/exports" | rg '"status":"processing"|\"status\":\"ready\"' >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/notifications" | rg '"items"' >/dev/null

curl -sS "$BASE_URL/metrics" \
  | rg 'haohao_http_requests_total|haohao_rate_limit_total|haohao_outbox' >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "X-CSRF-Token: $csrf" \
  -X DELETE \
  "$BASE_URL/api/v1/customer-signals/$signal_public_id" >/dev/null

echo "common-services smoke ok: $BASE_URL"
