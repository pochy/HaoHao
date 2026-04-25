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

go run ./backend/cmd/openapi -surface=full > openapi/openapi.yaml
go run ./backend/cmd/openapi -surface=browser > openapi/browser.yaml
go run ./backend/cmd/openapi -surface=external > openapi/external.yaml

(
  cd frontend
  npm run openapi-ts
)
