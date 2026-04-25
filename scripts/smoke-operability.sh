#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
BODY_FILE="$(mktemp)"
HEADERS_FILE="$(mktemp)"

cleanup() {
  rm -f "$BODY_FILE" "$HEADERS_FILE"
}
trap cleanup EXIT

fail() {
  echo "smoke failed: $*" >&2
  exit 1
}

status_of() {
  curl -sS -o "$BODY_FILE" -w "%{http_code}" "$1"
}

expect_status() {
  local path="$1"
  local want="$2"
  local got
  got="$(status_of "${BASE_URL}${path}")"
  if [[ "$got" != "$want" ]]; then
    echo "response body:" >&2
    cat "$BODY_FILE" >&2 || true
    fail "${path}: want ${want}, got ${got}"
  fi
}

expect_status "/healthz" "200"
expect_status "/readyz" "200"

: > "$HEADERS_FILE"
session_status="$(curl -sS -D "$HEADERS_FILE" -o "$BODY_FILE" -w "%{http_code}" "${BASE_URL}/api/v1/session")"
if [[ "$session_status" != "401" ]]; then
  fail "/api/v1/session: want 401, got ${session_status}"
fi
grep -iq '^content-type: application/problem+json' "$HEADERS_FILE" || fail "/api/v1/session did not return application/problem+json"

openapi_status="$(status_of "${BASE_URL}/openapi.yaml")"
if [[ "$openapi_status" != "200" ]]; then
  fail "/openapi.yaml: want 200, got ${openapi_status}"
fi
grep -q "openapi: 3.1.0" "$BODY_FILE" || fail "/openapi.yaml does not look like OpenAPI 3.1 YAML"

: > "$HEADERS_FILE"
curl -sS -D "$HEADERS_FILE" -o "$BODY_FILE" "${BASE_URL}/api/v1/auth/callback?error=forced" >/dev/null
location="$(awk 'tolower($0) ~ /^location:/ {gsub("\r", "", $0); sub(/^[Ll]ocation:[[:space:]]*/, "", $0); print}' "$HEADERS_FILE" | tail -n 1)"

if [[ -z "$location" ]]; then
  fail "callback response did not include a Location header"
fi
if [[ "$location" == http://127.0.0.1:5173* || "$location" == http://localhost:5173* ]]; then
  fail "callback redirected to Vite dev server: ${location}"
fi

echo "operability smoke ok: ${BASE_URL}"
