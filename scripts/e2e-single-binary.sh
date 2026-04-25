#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${E2E_HTTP_PORT:-18080}"
BASE_URL="${E2E_BASE_URL:-http://127.0.0.1:${PORT}}"
DATABASE_URL="${DATABASE_URL:-postgres://haohao:haohao@127.0.0.1:5432/haohao?sslmode=disable}"
REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}"
FILE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/haohao-e2e-files.XXXXXX")"
LOG_FILE="$(mktemp "${TMPDIR:-/tmp}/haohao-e2e-server.XXXXXX")"
SERVER_PID=""

cleanup() {
  local status=$?
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
  rm -rf "$FILE_DIR"
  if [[ "$status" -eq 0 ]]; then
    rm -f "$LOG_FILE"
  else
    echo "E2E server log retained: $LOG_FILE" >&2
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
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/seed-e2e-users.sql

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
DATA_LIFECYCLE_ENABLED=false \
./bin/haohao >"$LOG_FILE" 2>&1 &
SERVER_PID="$!"

for attempt in {1..80}; do
  if curl -fsS "$BASE_URL/readyz" >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    echo "E2E server exited early. Log: $LOG_FILE" >&2
    sed -n '1,160p' "$LOG_FILE" >&2 || true
    exit 1
  fi
  if [[ "$attempt" == "80" ]]; then
    echo "E2E server did not become ready. Log: $LOG_FILE" >&2
    sed -n '1,160p' "$LOG_FILE" >&2 || true
    exit 1
  fi
  sleep 0.25
done

E2E_BASE_URL="$BASE_URL" npm --prefix frontend run e2e -- --project=chromium
