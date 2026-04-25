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

tenant_slug="p11-rl-$(date +%s)-$$"

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d "{\"slug\":\"$tenant_slug\",\"displayName\":\"P11 Rate Limit\"}" \
  "$BASE_URL/api/v1/admin/tenants" | rg "\"slug\":\"$tenant_slug\"" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d '{"userEmail":"demo@example.com","roleCode":"customer_signal_user"}' \
  "$BASE_URL/api/v1/admin/tenants/$tenant_slug/memberships" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d "{\"tenantSlug\":\"$tenant_slug\"}" \
  "$BASE_URL/api/v1/session/tenant" | rg "\"slug\":\"$tenant_slug\"" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -X PUT \
  -d '{"fileQuotaBytes":104857600,"rateLimitBrowserApiPerMinute":2,"notificationsEnabled":true,"features":{}}' \
  "$BASE_URL/api/v1/admin/tenants/$tenant_slug/settings" \
  | rg '"rateLimitBrowserApiPerMinute":2' >/dev/null

blocked=0
for _ in 1 2 3 4; do
  headers="$(mktemp)"
  status="$(curl -sS -o /dev/null -D "$headers" -w '%{http_code}' -c "$COOKIE_JAR" -b "$COOKIE_JAR" "$BASE_URL/api/v1/customer-signals")"
  if [[ "$status" == "429" ]]; then
    rg -i '^Retry-After:' "$headers" >/dev/null
    blocked=1
    rm -f "$headers"
    break
  fi
  rm -f "$headers"
done

if [[ "$blocked" != "1" ]]; then
  echo "expected browser API rate limit to block after tenant override" >&2
  exit 1
fi

curl -sS "$BASE_URL/metrics" \
  | rg 'haohao_rate_limit_total\{[^}]*policy="browser_api"[^}]*result="blocked"' >/dev/null

echo "rate-limit-runtime smoke ok: $BASE_URL"
