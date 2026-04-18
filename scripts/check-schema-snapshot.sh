#!/bin/sh

set -eu

base_ref="${1:-}"
head_ref="${2:-HEAD}"

if [ -z "$base_ref" ]; then
  echo "usage: $0 <base-ref> [head-ref]" >&2
  exit 2
fi

if [ "$base_ref" = "0000000000000000000000000000000000000000" ]; then
  echo "Skipping db/schema.sql snapshot check: no comparison base commit."
  exit 0
fi

changed_files="$(git diff --name-only "$base_ref...$head_ref")"

if ! printf '%s\n' "$changed_files" | grep -q '^db/migrations/'; then
  exit 0
fi

if printf '%s\n' "$changed_files" | grep -q '^db/schema\.sql$'; then
  exit 0
fi

echo "db/migrations changed but db/schema.sql was not updated." >&2
exit 1
