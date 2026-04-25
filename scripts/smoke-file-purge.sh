#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${SMOKE_FILE_PURGE_HTTP_PORT:-18081}"
BASE_URL="${SMOKE_FILE_PURGE_BASE_URL:-http://127.0.0.1:${PORT}}"
DATABASE_URL="${DATABASE_URL:-postgres://haohao:haohao@127.0.0.1:5432/haohao?sslmode=disable}"
REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}"
FILE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/haohao-file-purge-files.XXXXXX")"
LOG_FILE="$(mktemp "${TMPDIR:-/tmp}/haohao-file-purge-server.XXXXXX")"
COOKIE_JAR="$(mktemp)"
UPLOAD_FILE="$(mktemp)"
SERVER_PID=""

cleanup() {
  local status=$?
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
  rm -rf "$FILE_DIR"
  rm -f "$COOKIE_JAR" "$UPLOAD_FILE"
  if [[ "$status" -eq 0 ]]; then
    rm -f "$LOG_FILE"
  else
    echo "file purge smoke server log retained: $LOG_FILE" >&2
  fi
  exit "$status"
}
trap cleanup EXIT

cd "$ROOT_DIR"

if [[ ! -x ./bin/haohao ]]; then
  echo "bin/haohao is missing. Run make binary first." >&2
  exit 1
fi

for attempt in {1..60}; do
  if psql "$DATABASE_URL" -c 'select 1' >/dev/null 2>&1; then
    break
  fi
  if [[ "$attempt" == "60" ]]; then
    echo "database is not reachable: $DATABASE_URL" >&2
    exit 1
  fi
  sleep 1
done

migrate -path db/migrations -database "$DATABASE_URL" up
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/seed-demo-user.sql

HTTP_PORT="$PORT" \
APP_BASE_URL="$BASE_URL" \
FRONTEND_BASE_URL="$BASE_URL" \
DATABASE_URL="$DATABASE_URL" \
REDIS_ADDR="$REDIS_ADDR" \
AUTH_MODE=local \
ENABLE_LOCAL_PASSWORD_LOGIN=true \
COOKIE_SECURE=false \
DOCS_AUTH_REQUIRED=false \
RATE_LIMIT_ENABLED=false \
FILE_LOCAL_DIR="$FILE_DIR" \
OUTBOX_WORKER_INTERVAL=200ms \
OUTBOX_WORKER_TIMEOUT=2s \
DATA_LIFECYCLE_ENABLED=true \
DATA_LIFECYCLE_INTERVAL=200ms \
DATA_LIFECYCLE_TIMEOUT=5s \
DATA_LIFECYCLE_RUN_ON_STARTUP=true \
FILE_DELETED_RETENTION=1s \
FILE_PURGE_BATCH_SIZE=10 \
FILE_PURGE_LOCK_TIMEOUT=2s \
./bin/haohao >"$LOG_FILE" 2>&1 &
SERVER_PID="$!"

for attempt in {1..80}; do
  if curl -fsS "$BASE_URL/readyz" >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    echo "file purge smoke server exited early. Log: $LOG_FILE" >&2
    sed -n '1,160p' "$LOG_FILE" >&2 || true
    exit 1
  fi
  if [[ "$attempt" == "80" ]]; then
    echo "file purge smoke server did not become ready. Log: $LOG_FILE" >&2
    sed -n '1,160p' "$LOG_FILE" >&2 || true
    exit 1
  fi
  sleep 0.25
done

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

title="P12 file purge smoke $(date +%s)-$$"
create_response="$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d "{\"customerName\":\"Acme\",\"title\":\"$title\",\"body\":\"Created by P12 file purge smoke\",\"source\":\"support\",\"priority\":\"medium\",\"status\":\"new\"}" \
  "$BASE_URL/api/v1/customer-signals")"
signal_public_id="$(printf '%s' "$create_response" | sed -n 's/.*"publicId":"\([^"]*\)".*/\1/p')"
if [[ -z "$signal_public_id" ]]; then
  echo "missing signal public id: $create_response" >&2
  exit 1
fi

printf 'hello from p12 file purge smoke\n' > "$UPLOAD_FILE"
upload_response="$(curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "X-CSRF-Token: $csrf" \
  -F "purpose=attachment" \
  -F "attachedToType=customer_signal" \
  -F "attachedToId=$signal_public_id" \
  -F "file=@$UPLOAD_FILE;filename=p12-purge.txt;type=text/plain" \
  "$BASE_URL/api/v1/files")"
file_public_id="$(printf '%s' "$upload_response" | sed -n 's/.*"publicId":"\([^"]*\)".*/\1/p')"
if [[ -z "$file_public_id" ]]; then
  echo "missing file public id: $upload_response" >&2
  exit 1
fi

storage_key="$(psql "$DATABASE_URL" -tA -c "SELECT storage_key FROM file_objects WHERE public_id = '$file_public_id'")"
file_path="$FILE_DIR/$storage_key"
if [[ -z "$storage_key" || ! -f "$file_path" ]]; then
  echo "expected uploaded file body at $file_path" >&2
  exit 1
fi

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "X-CSRF-Token: $csrf" \
  -X DELETE \
  "$BASE_URL/api/v1/files/$file_public_id" >/dev/null

download_status="$(curl -sS -o /dev/null -w '%{http_code}' -c "$COOKIE_JAR" -b "$COOKIE_JAR" "$BASE_URL/api/v1/files/$file_public_id")"
if [[ "$download_status" != "404" ]]; then
  echo "expected deleted file download to return 404, got $download_status" >&2
  exit 1
fi

purged=0
for _ in {1..100}; do
  purged_at="$(psql "$DATABASE_URL" -tA -c "SELECT COALESCE(purged_at::text, '') FROM file_objects WHERE public_id = '$file_public_id'")"
  if [[ -n "$purged_at" && ! -e "$file_path" ]]; then
    purged=1
    break
  fi
  sleep 0.25
done

if [[ "$purged" != "1" ]]; then
  echo "expected file body to be purged and purged_at to be set" >&2
  psql "$DATABASE_URL" -c "SELECT public_id, status, deleted_at, purged_at, purge_attempts, last_purge_error FROM file_objects WHERE public_id = '$file_public_id'" >&2 || true
  exit 1
fi

curl -sS "$BASE_URL/metrics" \
  | rg 'haohao_data_lifecycle_items_total\{[^}]*kind="file_objects_body_purged"[^}]*\} [1-9]' >/dev/null

echo "file-purge smoke ok: $BASE_URL"
