#!/usr/bin/env bash
set -euo pipefail

DOCKER_COMPOSE="${DOCKER_COMPOSE:-docker-compose}"
TMP_DUMP="$(mktemp)"
trap 'rm -f "$TMP_DUMP"' EXIT

"$DOCKER_COMPOSE" exec -T postgres pg_dump --schema-only --no-owner --no-privileges -U haohao -d haohao > "$TMP_DUMP"

for table in tenant_settings file_objects outbox_events tenant_data_exports; do
  rg "CREATE TABLE public\\.$table" "$TMP_DUMP" >/dev/null
done

echo "backup-restore smoke ok: schema dump contains P7 common service tables"
