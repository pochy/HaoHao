#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
BODY_FILE="$(mktemp)"

cleanup() {
  rm -f "$BODY_FILE"
}
trap cleanup EXIT

fail() {
  echo "observability smoke failed: $*" >&2
  exit 1
}

curl -sS -o /dev/null "${BASE_URL}/readyz"
curl -sS -o /dev/null "${BASE_URL}/api/v1/session" || true

status="$(curl -sS -o "$BODY_FILE" -w "%{http_code}" "${BASE_URL}/metrics")"
if [[ "$status" != "200" ]]; then
  cat "$BODY_FILE" >&2 || true
  fail "/metrics: want 200, got ${status}"
fi

grep -q '^# HELP haohao_http_requests_total ' "$BODY_FILE" || fail "missing http request counter"
grep -q '^# HELP haohao_http_request_duration_seconds ' "$BODY_FILE" || fail "missing http request duration histogram"
grep -q '^# HELP haohao_dependency_ping_duration_seconds ' "$BODY_FILE" || fail "missing dependency ping histogram"
grep -q 'haohao_http_requests_total' "$BODY_FILE" || fail "http request metrics not exported"
grep -q 'haohao_dependency_ping_duration_seconds' "$BODY_FILE" || fail "dependency ping metrics not exported"

echo "observability smoke ok: ${BASE_URL}"
