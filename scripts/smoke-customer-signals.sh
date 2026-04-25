#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
COOKIE_JAR="$(mktemp)"
trap 'rm -f "$COOKIE_JAR"' EXIT

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

title="P6 smoke signal $(date +%s)-$$"
create_response="$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d "{\"customerName\":\"Acme\",\"title\":\"$title\",\"body\":\"Created by smoke test\",\"source\":\"support\",\"priority\":\"medium\",\"status\":\"new\"}" \
  "$BASE_URL/api/v1/customer-signals")"

signal_public_id="$(printf '%s' "$create_response" | sed -n 's/.*"publicId":"\([^"]*\)".*/\1/p')"
if [[ -z "$signal_public_id" ]]; then
  echo "missing signal public id: $create_response" >&2
  exit 1
fi

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/customer-signals" | rg "\"publicId\":\"$signal_public_id\"" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/customer-signals/$signal_public_id" | rg "\"title\":\"$title\"" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -X PATCH \
  -d '{"priority":"high","status":"triaged","body":"Updated by smoke test"}' \
  "$BASE_URL/api/v1/customer-signals/$signal_public_id" | rg '"status":"triaged"' >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "X-CSRF-Token: $csrf" \
  -X DELETE \
  "$BASE_URL/api/v1/customer-signals/$signal_public_id" >/dev/null

if curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/customer-signals" | rg "\"publicId\":\"$signal_public_id\"" >/dev/null; then
  echo "deleted signal is still visible" >&2
  exit 1
fi

echo "customer-signals smoke ok: $BASE_URL"
