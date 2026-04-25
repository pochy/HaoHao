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

tenant_slug="p5-smoke-$(date +%s)-$$"

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d "{\"slug\":\"$tenant_slug\",\"displayName\":\"P5 Smoke\"}" \
  "$BASE_URL/api/v1/admin/tenants" | rg "\"slug\":\"$tenant_slug\"" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/admin/tenants/$tenant_slug" | rg "\"slug\":\"$tenant_slug\"" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d '{"userEmail":"demo@example.com","roleCode":"todo_user"}' \
  "$BASE_URL/api/v1/admin/tenants/$tenant_slug/memberships" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/admin/tenants/$tenant_slug" | rg '"roleCode":"todo_user"' >/dev/null

echo "tenant-admin smoke ok: $BASE_URL"
