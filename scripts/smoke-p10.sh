#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
COOKIE_JAR="$(mktemp)"
CSV_FILE="$(mktemp)"
trap 'rm -f "$COOKIE_JAR" "$CSV_FILE"' EXIT

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"changeme123"}' \
  "$BASE_URL/api/v1/login" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" "$BASE_URL/api/v1/csrf" >/dev/null
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

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -X PUT \
  -d '{"items":[{"featureCode":"webhooks.enabled","enabled":true,"limitValue":{"maxEndpoints":5}},{"featureCode":"customer_signals.import_export","enabled":true,"limitValue":{"maxRows":1000}},{"featureCode":"customer_signals.saved_filters","enabled":true,"limitValue":{}},{"featureCode":"support_access.enabled","enabled":true,"limitValue":{}}]}' \
  "$BASE_URL/api/v1/admin/tenants/acme/entitlements" | rg '"featureCode":"webhooks.enabled"' >/dev/null

title="P10 smoke signal $(date +%s)-$$"
create_response="$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d "{\"customerName\":\"Acme\",\"title\":\"$title\",\"body\":\"P10 search and CSV smoke\",\"source\":\"support\",\"priority\":\"medium\",\"status\":\"new\"}" \
  "$BASE_URL/api/v1/customer-signals")"
signal_public_id="$(printf '%s' "$create_response" | sed -n 's/.*"publicId":"\([^"]*\)".*/\1/p')"
if [[ -z "$signal_public_id" ]]; then
  echo "missing signal public id: $create_response" >&2
  exit 1
fi

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/customer-signals?q=P10&limit=1" | rg "\"publicId\":\"$signal_public_id\"|\"nextCursor\"" >/dev/null

filter_response="$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d '{"name":"P10 smoke","query":"P10","filters":{"status":"new"}}' \
  "$BASE_URL/api/v1/customer-signal-filters")"
filter_public_id="$(printf '%s' "$filter_response" | sed -n 's/.*"publicId":"\([^"]*\)".*/\1/p')"
if [[ -z "$filter_public_id" ]]; then
  echo "missing saved filter public id: $filter_response" >&2
  exit 1
fi

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "X-CSRF-Token: $csrf" \
  -X DELETE \
  "$BASE_URL/api/v1/customer-signal-filters/$filter_public_id" >/dev/null

cat > "$CSV_FILE" <<CSV
customer_name,title,body,source,priority,status
Acme,P10 imported signal,Imported by P10 smoke,support,medium,new
CSV

upload_response="$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "X-CSRF-Token: $csrf" \
  -F "purpose=import" \
  -F "file=@$CSV_FILE;filename=p10-import.csv;type=text/csv" \
  "$BASE_URL/api/v1/files")"
file_public_id="$(printf '%s' "$upload_response" | sed -n 's/.*"publicId":"\([^"]*\)".*/\1/p')"
if [[ -z "$file_public_id" ]]; then
  echo "missing import file public id: $upload_response" >&2
  exit 1
fi

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d "{\"inputFilePublicId\":\"$file_public_id\"}" \
  "$BASE_URL/api/v1/admin/tenants/acme/imports" | rg '"status":"(processing|completed)"' >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d '{"format":"csv"}' \
  "$BASE_URL/api/v1/admin/tenants/acme/exports" | rg '"format":"csv"' >/dev/null

if [[ "${RUN_WEBHOOK_SMOKE:-0}" == "1" ]]; then
  webhook_url="${WEBHOOK_SMOKE_URL:-http://127.0.0.1:18083/webhook}"
  curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $csrf" \
    -d "{\"name\":\"P10 smoke\",\"url\":\"$webhook_url\",\"eventTypes\":[\"customer_signal.created\"],\"active\":true}" \
    "$BASE_URL/api/v1/admin/tenants/acme/webhooks" | rg '"secret":"whsec_' >/dev/null
else
  curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    "$BASE_URL/api/v1/admin/tenants/acme/webhooks" | rg '"items"' >/dev/null
fi

echo "p10 smoke ok: $BASE_URL"
