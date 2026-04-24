#!/usr/bin/env bash
set -euo pipefail

if [[ -f .env ]]; then
  set -a
  source .env
  set +a
fi

mkdir -p openapi

(
  cd backend
  sqlc generate
)

go run ./backend/cmd/openapi > openapi/openapi.yaml

(
  cd frontend
  npm run openapi-ts
)

