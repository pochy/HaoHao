#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_ENV_FILE="${COMPOSE_ENV_FILE:-$ROOT_DIR/compose.auth.env}"

if [[ -f "$COMPOSE_ENV_FILE" ]]; then
  set -a
  . "$COMPOSE_ENV_FILE"
  set +a
fi

require_bin() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 is required" >&2
    exit 1
  fi
}

shell_quote() {
  printf "'%s'" "$(printf '%s' "$1" | sed "s/'/'\\\\''/g")"
}

auth_env_value() {
  local key=$1
  if [[ ! -f "$AUTH_ENV_FILE" ]]; then
    return 0
  fi

  (
    set -a
    . "$AUTH_ENV_FILE"
    set +a
    case "$key" in
      ZITADEL_CLIENT_ID)
        printf '%s' "${ZITADEL_CLIENT_ID:-}"
        ;;
      ZITADEL_CLIENT_SECRET)
        printf '%s' "${ZITADEL_CLIENT_SECRET:-}"
        ;;
      *)
        ;;
    esac
  )
}

require_bin curl
require_bin jq

: "${ZITADEL_DOMAIN:=localhost}"
: "${ZITADEL_EXTERNALPORT:=8081}"
: "${ZITADEL_PUBLIC_SCHEME:=http}"
: "${ZITADEL_BOOTSTRAP_DIR:=.cache/zitadel/bootstrap}"
: "${HAOHAO_ZITADEL_PROJECT_NAME:=haohao-local}"
: "${HAOHAO_ZITADEL_APP_NAME:=haohao-browser-local}"
: "${HAOHAO_ZITADEL_TEST_USER_EMAIL:=haohao.dev@zitadel.localhost}"
: "${HAOHAO_ZITADEL_TEST_USER_PASSWORD:=Password1!}"
: "${HAOHAO_ZITADEL_REDIRECT_URI:=http://localhost:8080/auth/callback}"
: "${HAOHAO_ZITADEL_POST_LOGOUT_REDIRECT_URI:=http://localhost:8080/auth/logout/callback}"
: "${HAOHAO_ZITADEL_FRONTEND_ORIGIN:=http://localhost:5173}"

ISSUER_URL="${ZITADEL_ISSUER_URL:-${ZITADEL_PUBLIC_SCHEME}://${ZITADEL_DOMAIN}:${ZITADEL_EXTERNALPORT}}"
DISCOVERY_URL="${ISSUER_URL%/}/.well-known/openid-configuration"
MANAGEMENT_BASE="${ISSUER_URL%/}/management/v1"
BOOTSTRAP_DIR="$ZITADEL_BOOTSTRAP_DIR"
if [[ "$BOOTSTRAP_DIR" != /* ]]; then
  BOOTSTRAP_DIR="$ROOT_DIR/$BOOTSTRAP_DIR"
fi
PAT_FILE="${ZITADEL_BOOTSTRAP_PAT_FILE:-$BOOTSTRAP_DIR/admin.pat}"
AUTH_ENV_FILE="${HAOHAO_AUTH_ENV_FILE:-$ROOT_DIR/.env.auth}"

wait_for_discovery() {
  local attempt
  for attempt in $(seq 1 60); do
    if curl -fsS "$DISCOVERY_URL" >/dev/null; then
      return 0
    fi
    sleep 2
  done

  echo "timed out waiting for $DISCOVERY_URL" >&2
  exit 1
}

api() {
  local method=$1
  local path=$2
  local body=${3-}
  local -a args
  args=(-fsS -X "$method" -H "Authorization: Bearer $ADMIN_PAT" -H "Accept: application/json")
  if [[ -n "$body" ]]; then
    args+=(-H "Content-Type: application/json" --data "$body")
  fi
  curl "${args[@]}" "${MANAGEMENT_BASE}${path}"
}

search_project_id() {
  api POST "/projects/_search" "$(jq -n --arg name "$HAOHAO_ZITADEL_PROJECT_NAME" '{
    query: {limit: 1},
    queries: [{nameQuery: {name: $name, method: "TEXT_QUERY_METHOD_EQUALS"}}]
  }')" | jq -r '.result[0].id // empty'
}

ensure_project() {
  local project_id
  project_id="$(search_project_id)"
  if [[ -n "$project_id" ]]; then
    printf '%s' "$project_id"
    return 0
  fi

  api POST "/projects" "$(jq -n --arg name "$HAOHAO_ZITADEL_PROJECT_NAME" '{
    name: $name,
    projectRoleAssertion: true
  }')" | jq -r '.id'
}

ensure_role() {
  local project_id=$1
  local role_key=$2
  local display_name=$3

  local existing
  existing="$(api POST "/projects/${project_id}/roles/_search" "$(jq -n --arg key "$role_key" '{
    query: {limit: 1},
    queries: [{keyQuery: {key: $key}}]
  }')" | jq -r '.result[0].key // empty')"
  if [[ -n "$existing" ]]; then
    return 0
  fi

  api POST "/projects/${project_id}/roles" "$(jq -n --arg key "$role_key" --arg displayName "$display_name" '{
    roleKey: $key,
    displayName: $displayName,
    group: "haohao"
  }')" >/dev/null
}

search_app() {
  local project_id=$1
  api POST "/projects/${project_id}/apps/_search" "$(jq -n --arg name "$HAOHAO_ZITADEL_APP_NAME" '{
    query: {limit: 1},
    queries: [{nameQuery: {name: $name, method: "TEXT_QUERY_METHOD_EQUALS"}}]
  }')" | jq -c '.result[0] // empty'
}

create_or_update_app() {
  local project_id=$1
  local app_json
  local client_id
  local client_secret
  local needs_update
  local existing_auth_client_id
  local existing_auth_client_secret

  app_json="$(search_app "$project_id")"
  if [[ -z "$app_json" ]]; then
    app_json="$(api POST "/projects/${project_id}/apps/oidc" "$(jq -n \
      --arg name "$HAOHAO_ZITADEL_APP_NAME" \
      --arg redirectURI "$HAOHAO_ZITADEL_REDIRECT_URI" \
      --arg logoutURI "$HAOHAO_ZITADEL_POST_LOGOUT_REDIRECT_URI" \
      --arg frontendOrigin "$HAOHAO_ZITADEL_FRONTEND_ORIGIN" \
      '{
        name: $name,
        redirectUris: [$redirectURI],
        responseTypes: ["OIDC_RESPONSE_TYPE_CODE"],
        grantTypes: [
          "OIDC_GRANT_TYPE_AUTHORIZATION_CODE",
          "OIDC_GRANT_TYPE_REFRESH_TOKEN"
        ],
        appType: "OIDC_APP_TYPE_WEB",
        authMethodType: "OIDC_AUTH_METHOD_TYPE_BASIC",
        postLogoutRedirectUris: [$logoutURI],
        version: "OIDC_VERSION_1_0",
        devMode: true,
        accessTokenType: "OIDC_TOKEN_TYPE_BEARER",
        accessTokenRoleAssertion: true,
        idTokenRoleAssertion: true,
        additionalOrigins: [$frontendOrigin]
      }')")"
    client_id="$(printf '%s' "$app_json" | jq -r '.clientId')"
    client_secret="$(printf '%s' "$app_json" | jq -r '.clientSecret')"
    printf '%s\n%s\n' "$client_id" "$client_secret"
    return 0
  fi

  local app_id
  app_id="$(printf '%s' "$app_json" | jq -r '.id')"
  client_id="$(printf '%s' "$app_json" | jq -r '.oidcConfig.clientId')"

  needs_update="$(printf '%s' "$app_json" | jq -r \
    --arg redirectURI "$HAOHAO_ZITADEL_REDIRECT_URI" \
    --arg logoutURI "$HAOHAO_ZITADEL_POST_LOGOUT_REDIRECT_URI" \
    --arg frontendOrigin "$HAOHAO_ZITADEL_FRONTEND_ORIGIN" '
      if (.oidcConfig.redirectUris == [$redirectURI]
        and .oidcConfig.postLogoutRedirectUris == [$logoutURI]
        and .oidcConfig.responseTypes == ["OIDC_RESPONSE_TYPE_CODE"]
        and .oidcConfig.grantTypes == ["OIDC_GRANT_TYPE_AUTHORIZATION_CODE", "OIDC_GRANT_TYPE_REFRESH_TOKEN"]
        and .oidcConfig.devMode == true
        and .oidcConfig.accessTokenRoleAssertion == true
        and .oidcConfig.idTokenRoleAssertion == true
        and .oidcConfig.additionalOrigins == [$frontendOrigin])
      then "false"
      else "true"
      end
    ')"

  if [[ "$needs_update" == "true" ]]; then
    api PUT "/projects/${project_id}/apps/${app_id}/oidc_config" "$(jq -n \
      --arg redirectURI "$HAOHAO_ZITADEL_REDIRECT_URI" \
      --arg logoutURI "$HAOHAO_ZITADEL_POST_LOGOUT_REDIRECT_URI" \
      --arg frontendOrigin "$HAOHAO_ZITADEL_FRONTEND_ORIGIN" \
      '{
        redirectUris: [$redirectURI],
        responseTypes: ["OIDC_RESPONSE_TYPE_CODE"],
        grantTypes: [
          "OIDC_GRANT_TYPE_AUTHORIZATION_CODE",
          "OIDC_GRANT_TYPE_REFRESH_TOKEN"
        ],
        appType: "OIDC_APP_TYPE_WEB",
        authMethodType: "OIDC_AUTH_METHOD_TYPE_BASIC",
        postLogoutRedirectUris: [$logoutURI],
        devMode: true,
        accessTokenType: "OIDC_TOKEN_TYPE_BEARER",
        accessTokenRoleAssertion: true,
        idTokenRoleAssertion: true,
        additionalOrigins: [$frontendOrigin]
      }')" >/dev/null
  fi

  existing_auth_client_id="$(auth_env_value ZITADEL_CLIENT_ID)"
  existing_auth_client_secret="$(auth_env_value ZITADEL_CLIENT_SECRET)"
  if [[ "$existing_auth_client_id" == "$client_id" && -n "$existing_auth_client_secret" ]]; then
    echo "reusing existing client secret from $AUTH_ENV_FILE" >&2
    client_secret="$existing_auth_client_secret"
    printf '%s\n%s\n' "$client_id" "$client_secret"
    return 0
  fi

  echo "generating new client secret for $HAOHAO_ZITADEL_APP_NAME" >&2
  client_secret="$(api POST "/projects/${project_id}/apps/${app_id}/oidc_config/_generate_client_secret" '{}' | jq -r '.clientSecret')"
  printf '%s\n%s\n' "$client_id" "$client_secret"
}

search_user_id() {
  api POST "/users/_search" "$(jq -n --arg email "$HAOHAO_ZITADEL_TEST_USER_EMAIL" '{
    query: {limit: 1},
    queries: [{emailQuery: {emailAddress: $email, method: "TEXT_QUERY_METHOD_EQUALS_IGNORE_CASE"}}]
  }')" | jq -r '.result[0].id // empty'
}

ensure_user() {
  local user_id
  user_id="$(search_user_id)"
  if [[ -z "$user_id" ]]; then
    user_id="$(api POST "/users/human" "$(jq -n \
      --arg email "$HAOHAO_ZITADEL_TEST_USER_EMAIL" \
      --arg password "$HAOHAO_ZITADEL_TEST_USER_PASSWORD" \
      '{
        userName: $email,
        profile: {
          firstName: "HaoHao",
          lastName: "Developer",
          displayName: "HaoHao Developer",
          preferredLanguage: "en"
        },
        email: {
          email: $email,
          isEmailVerified: true
        },
        initialPassword: $password
      }' )" | jq -r '.userId')"
    printf '%s' "$user_id"
    return 0
  fi

  api POST "/users/${user_id}/password" "$(jq -n --arg password "$HAOHAO_ZITADEL_TEST_USER_PASSWORD" '{
    password: $password,
    noChangeRequired: true
  }')" >/dev/null
  printf '%s' "$user_id"
}

ensure_user_grant() {
  local user_id=$1
  local project_id=$2
  local grant_id
  local grant_json
  local existing_role_keys
  local target_role_keys="app:user,docs:read"

  grant_json="$(api POST "/users/grants/_search" "$(jq -n --arg userID "$user_id" --arg projectID "$project_id" '{
    query: {limit: 1},
    queries: [
      {userIdQuery: {userId: $userID}},
      {projectIdQuery: {projectId: $projectID}}
    ]
  }')")"
  grant_id="$(printf '%s' "$grant_json" | jq -r '.result[0].id // empty')"

  if [[ -z "$grant_id" ]]; then
    api POST "/users/${user_id}/grants" "$(jq -n --arg projectID "$project_id" '{
      projectId: $projectID,
      roleKeys: ["app:user", "docs:read"]
    }')" >/dev/null
    return 0
  fi

  existing_role_keys="$(printf '%s' "$grant_json" | jq -r '.result[0].roleKeys // [] | sort | join(",")')"
  if [[ "$existing_role_keys" == "$target_role_keys" ]]; then
    return 0
  fi

  api PUT "/users/${user_id}/grants/${grant_id}" '{
    "roleKeys": ["app:user", "docs:read"]
  }' >/dev/null
}

write_auth_env() {
  local client_id=$1
  local client_secret=$2

  cat >"$AUTH_ENV_FILE" <<EOF
ZITADEL_ISSUER_URL=$(shell_quote "$ISSUER_URL")
ZITADEL_CLIENT_ID=$(shell_quote "$client_id")
ZITADEL_CLIENT_SECRET=$(shell_quote "$client_secret")
ZITADEL_REDIRECT_URI=$(shell_quote "$HAOHAO_ZITADEL_REDIRECT_URI")
ZITADEL_POST_LOGOUT_REDIRECT_URI=$(shell_quote "$HAOHAO_ZITADEL_POST_LOGOUT_REDIRECT_URI")
ZITADEL_SCOPES='openid profile email'
FRONTEND_ORIGIN=$(shell_quote "$HAOHAO_ZITADEL_FRONTEND_ORIGIN")
SESSION_TTL='8h'
EOF
}

if [[ ! -f "$PAT_FILE" ]]; then
  echo "bootstrap PAT not found at $PAT_FILE" >&2
  echo "run 'make compose-auth-up' first" >&2
  exit 1
fi

wait_for_discovery
ADMIN_PAT="$(tr -d '\r\n' <"$PAT_FILE")"

PROJECT_ID="$(ensure_project)"
ensure_role "$PROJECT_ID" "app:user" "App User"
ensure_role "$PROJECT_ID" "docs:read" "Docs Reader"

mapfile -t APP_CREDS < <(create_or_update_app "$PROJECT_ID")
CLIENT_ID="${APP_CREDS[0]}"
CLIENT_SECRET="${APP_CREDS[1]}"

USER_ID="$(ensure_user)"
ensure_user_grant "$USER_ID" "$PROJECT_ID"
write_auth_env "$CLIENT_ID" "$CLIENT_SECRET"

echo "seeded local Zitadel"
echo "issuer: $ISSUER_URL"
echo "project: $HAOHAO_ZITADEL_PROJECT_NAME ($PROJECT_ID)"
echo "app: $HAOHAO_ZITADEL_APP_NAME"
echo "test user: $HAOHAO_ZITADEL_TEST_USER_EMAIL"
echo ".env.auth written to $AUTH_ENV_FILE"
