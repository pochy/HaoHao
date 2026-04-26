#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

command -v fga >/dev/null 2>&1 || {
  echo "fga CLI is required. Install it from https://github.com/openfga/cli" >&2
  exit 1
}

command -v jq >/dev/null 2>&1 || {
  echo "jq is required to parse fga CLI output" >&2
  exit 1
}

OPENFGA_API_URL="${OPENFGA_API_URL:-http://127.0.0.1:8088}"
OPENFGA_API_TOKEN="${OPENFGA_API_TOKEN:-}"
OPENFGA_STORE_ID="${OPENFGA_STORE_ID:-}"
OPENFGA_STORE_NAME="${OPENFGA_STORE_NAME:-haohao-drive-dev}"
OPENFGA_MODEL_FILE="${OPENFGA_MODEL_FILE:-openfga/drive.fga}"

if [[ ! -f "$OPENFGA_MODEL_FILE" ]]; then
  echo "OpenFGA model file not found: $OPENFGA_MODEL_FILE" >&2
  exit 1
fi

export FGA_API_URL="$OPENFGA_API_URL"
if [[ -n "$OPENFGA_API_TOKEN" ]]; then
  export FGA_API_TOKEN="$OPENFGA_API_TOKEN"
else
  unset FGA_API_TOKEN
fi

if [[ -n "$OPENFGA_STORE_ID" ]]; then
  response="$(fga model write --store-id "$OPENFGA_STORE_ID" --file "$OPENFGA_MODEL_FILE")"
  store_id="$OPENFGA_STORE_ID"
  model_id="$(jq -r '.authorization_model_id // empty' <<<"$response")"
else
  response="$(fga store create --name "$OPENFGA_STORE_NAME" --model "$OPENFGA_MODEL_FILE")"
  store_id="$(jq -r '.store.id // .id // empty' <<<"$response")"
  model_id="$(jq -r '.model.authorization_model_id // .authorization_model_id // empty' <<<"$response")"
fi

if [[ -z "$store_id" || -z "$model_id" ]]; then
  echo "Failed to parse store/model IDs from fga CLI output:" >&2
  echo "$response" >&2
  exit 1
fi

cat <<EOF
OPENFGA_STORE_ID=$store_id
OPENFGA_AUTHORIZATION_MODEL_ID=$model_id
EOF
