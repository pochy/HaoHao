#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

START_APP=1
INSTALL_TOOLS=1
BACKEND_STARTED=0
BACKEND_REUSED=0
FRONTEND_STARTED=0
FRONTEND_REUSED=0

usage() {
  cat <<'EOF'
Usage: scripts/setup-dev-env.sh [options]

Options:
  --skip-app       Docker services, DB, OpenFGA, SeaweedFS, Zitadel だけ構築し、backend/frontend は起動しない
  --no-install     air / migrate / sqlc / fga / npm dependencies のインストールをスキップする
  -h, --help       このヘルプを表示する
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-app)
      START_APP=0
      shift
      ;;
    --no-install)
      INSTALL_TOOLS=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

info() {
  printf '\n\033[1;34m==>\033[0m %s\n' "$*"
}

ok() {
  printf '\033[1;32mOK\033[0m %s\n' "$*"
}

warn() {
  printf '\033[1;33mWARN\033[0m %s\n' "$*" >&2
}

fail() {
  printf '\033[1;31mERROR\033[0m %s\n' "$*" >&2
  exit 1
}

require_command() {
  local cmd="$1"
  local hint="${2:-}"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    if [[ -n "$hint" ]]; then
      fail "$cmd が見つかりません。$hint"
    fi
    fail "$cmd が見つかりません。"
  fi
}

go_bin_dir() {
  go env GOPATH 2>/dev/null | awk '{ print $1 "/bin" }'
}

export_go_bin_path() {
  local bin
  bin="$(go_bin_dir)"
  case ":$PATH:" in
    *":$bin:"*) ;;
    *) export PATH="$PATH:$bin" ;;
  esac
}

set_env_value() {
  local file="$1"
  local key="$2"
  local value="$3"
  if grep -qE "^${key}=" "$file"; then
    perl -0pi -e "s|^${key}=.*$|${key}=${value}|m" "$file"
  else
    printf '%s=%s\n' "$key" "$value" >> "$file"
  fi
}

wait_for_url() {
  local url="$1"
  local name="$2"
  local attempts="${3:-60}"
  local i
  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      ok "$name is ready: $url"
      return 0
    fi
    sleep 1
  done
  fail "$name が起動確認できませんでした: $url"
}

url_ready() {
  local url="$1"
  curl -fsS "$url" >/dev/null 2>&1
}

wait_for_head() {
  local url="$1"
  local name="$2"
  local attempts="${3:-60}"
  local i
  for ((i = 1; i <= attempts; i++)); do
    if curl -fsSI "$url" >/dev/null 2>&1; then
      ok "$name is ready: $url"
      return 0
    fi
    sleep 1
  done
  fail "$name が起動確認できませんでした: $url"
}

head_ready() {
  local url="$1"
  curl -fsSI "$url" >/dev/null 2>&1
}

stop_pid_file() {
  local pid_file="$1"
  local name="$2"
  if [[ ! -f "$pid_file" ]]; then
    return 0
  fi
  local pid
  pid="$(cat "$pid_file")"
  if [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1; then
    info "$name を再起動します (pid: $pid)"
    kill "$pid" >/dev/null 2>&1 || true
    sleep 1
  fi
  rm -f "$pid_file"
}

start_background() {
  local name="$1"
  local pid_file="$2"
  local log_file="$3"
  shift 3

  stop_pid_file "$pid_file" "$name"
  info "$name をバックグラウンド起動します"
  mkdir -p "$(dirname "$pid_file")" "$(dirname "$log_file")"
  ( "$@" >"$log_file" 2>&1 & echo $! > "$pid_file" )
  ok "$name started: pid=$(cat "$pid_file"), log=$log_file"
}

info "前提コマンドを確認します"
require_command go "Go 1.26.0 をインストールしてください。"
require_command node "Node.js 22 をインストールしてください。"
require_command npm "Node.js 22 付属の npm を利用してください。"
require_command docker "Docker Desktop を起動・インストールしてください。"
require_command make "GNU Make をインストールしてください。"
require_command curl "curl をインストールしてください。"
require_command jq "jq をインストールしてください。"
export_go_bin_path

if [[ "$INSTALL_TOOLS" == "1" ]]; then
  info "Go 開発ツールを確認・インストールします"
  if ! command -v air >/dev/null 2>&1; then
    go install github.com/air-verse/air@latest
  fi
  if ! command -v migrate >/dev/null 2>&1; then
    go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
  fi
  if ! command -v sqlc >/dev/null 2>&1; then
    go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.0
  fi
  export_go_bin_path

  info "OpenFGA CLI を確認します"
  if ! command -v fga >/dev/null 2>&1; then
    require_command brew "fga CLI を自動インストールするには Homebrew が必要です。手動の場合は https://github.com/openfga/cli を参照してください。"
    brew install openfga/tap/fga
  fi

  info "フロントエンド依存を npm ci で揃えます"
  ( cd frontend && npm ci )
else
  info "--no-install 指定のためインストール処理をスキップします"
  require_command air "air が必要です。--no-install を外すか go install github.com/air-verse/air@latest を実行してください。"
  require_command migrate "migrate が必要です。--no-install を外すか go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest を実行してください。"
  require_command sqlc "sqlc が必要です。--no-install を外すか go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.0 を実行してください。"
  require_command fga "fga CLI が必要です。--no-install を外すか brew install openfga/tap/fga を実行してください。"
fi

info ".env を準備します"
if [[ ! -f .env ]]; then
  cp .env.example .env
  ok ".env を .env.example から作成しました"
else
  ok ".env は既に存在します"
fi
set -a
# shellcheck disable=SC1091
source .env
set +a

info "Docker 開発サービスを起動します (PostgreSQL / Redis / ClickHouse / OpenFGA)"
make up

info "DB migration とデモユーザー seed を適用します"
make db-up
make seed-demo-user

info "生成物を更新します (sqlc + OpenAPI + frontend SDK)"
make gen

info "OpenFGA store/model を bootstrap します"
openfga_store_id_before="${OPENFGA_STORE_ID:-}"
if [[ -n "$openfga_store_id_before" ]]; then
  openfga_output="$(OPENFGA_STORE_ID="$openfga_store_id_before" make openfga-bootstrap)"
else
  openfga_output="$(make openfga-bootstrap)"
fi
printf '%s\n' "$openfga_output"
openfga_store_id="$(printf '%s\n' "$openfga_output" | awk -F= '$1 == "OPENFGA_STORE_ID" { print $2 }' | tail -n 1)"
openfga_model_id="$(printf '%s\n' "$openfga_output" | awk -F= '$1 == "OPENFGA_AUTHORIZATION_MODEL_ID" { print $2 }' | tail -n 1)"
[[ -n "$openfga_store_id" ]] || fail "OPENFGA_STORE_ID を取得できませんでした"
[[ -n "$openfga_model_id" ]] || fail "OPENFGA_AUTHORIZATION_MODEL_ID を取得できませんでした"
set_env_value .env OPENFGA_ENABLED true
set_env_value .env OPENFGA_STORE_ID "$openfga_store_id"
set_env_value .env OPENFGA_AUTHORIZATION_MODEL_ID "$openfga_model_id"
ok "OpenFGA 設定を .env に反映しました"

info "SeaweedFS を起動し、Drive 用 bucket を作成します"
make seaweedfs-up
docker exec haohao-seaweedfs sh -lc \
  'printf "s3.bucket.create -name haohao-drive-dev\ns3.bucket.list\n" | weed shell -master=localhost:9333 -filer=localhost:8888' || true
set_env_value .env FILE_STORAGE_DRIVER seaweedfs_s3
set_env_value .env FILE_S3_ENDPOINT http://127.0.0.1:8333
set_env_value .env FILE_S3_REGION us-east-1
set_env_value .env FILE_S3_BUCKET haohao-drive-dev
set_env_value .env FILE_S3_ACCESS_KEY_ID haohao
set_env_value .env FILE_S3_SECRET_ACCESS_KEY haohao-secret
set_env_value .env FILE_S3_FORCE_PATH_STYLE true
ok "SeaweedFS S3 設定を .env に反映しました"

info "Zitadel を起動します"
make zitadel-env
set_env_value dev/zitadel/.env ZITADEL_DEFAULT_REDIRECT_URI http://127.0.0.1:8080/api/v1/auth/callback
make zitadel-up

if [[ "$START_APP" == "1" ]]; then
  info "HaoHao backend/frontend を起動します"
  mkdir -p .dev/logs .dev/pids
  if url_ready "http://127.0.0.1:8080/readyz"; then
    ok "HaoHao backend は既に起動しています: http://127.0.0.1:8080"
    BACKEND_REUSED=1
  else
    start_background "backend" ".dev/pids/backend.pid" ".dev/logs/backend.log" env "PATH=$PATH" make backend-dev
    wait_for_url "http://127.0.0.1:8080/readyz" "HaoHao backend"
    BACKEND_STARTED=1
  fi

  if head_ready "http://127.0.0.1:5173/"; then
    ok "HaoHao frontend は既に起動しています: http://127.0.0.1:5173/"
    FRONTEND_REUSED=1
  else
    start_background "frontend" ".dev/pids/frontend.pid" ".dev/logs/frontend.log" make frontend-dev
    wait_for_head "http://127.0.0.1:5173/" "HaoHao frontend"
    FRONTEND_STARTED=1
  fi
else
  info "--skip-app 指定のため backend/frontend 起動をスキップします"
fi

info "追加サービスの疎通確認を行います"
curl -fsS "http://127.0.0.1:8088/healthz" >/dev/null
ok "OpenFGA health: http://127.0.0.1:8088/healthz"
curl -fsS "http://127.0.0.1:9333/cluster/status?pretty=y" >/dev/null
ok "SeaweedFS master: http://127.0.0.1:9333"
curl -fsSI "http://127.0.0.1:8888/" >/dev/null
ok "SeaweedFS filer: http://127.0.0.1:8888"
curl -fsS "http://localhost:8081/.well-known/openid-configuration" >/dev/null
ok "Zitadel discovery: http://localhost:8081/.well-known/openid-configuration"

cat <<EOF

================================================================
HaoHao 開発環境構築が完了しました
================================================================

App:
  Frontend:  http://127.0.0.1:5173/
  Backend:   http://127.0.0.1:8080
  Readiness: http://127.0.0.1:8080/readyz

HaoHao local login:
  email:    demo@example.com
  password: changeme123

OpenFGA:
  API / Playground: http://127.0.0.1:8088
  OPENFGA_STORE_ID=$openfga_store_id
  OPENFGA_AUTHORIZATION_MODEL_ID=$openfga_model_id

SeaweedFS:
  Master UI:   http://127.0.0.1:9333
  Filer UI:    http://127.0.0.1:8888
  S3 endpoint: http://127.0.0.1:8333
  S3 bucket:   haohao-drive-dev
  S3 access:   haohao / haohao-secret

Zitadel:
  Issuer:    http://localhost:8081
  Console:   http://localhost:8081/ui/console?login_hint=zitadel-admin@zitadel.localhost
  Admin PW:  Password1!

Logs and PIDs:
EOF

if [[ "$START_APP" == "1" ]]; then
  if [[ "$BACKEND_STARTED" == "1" ]]; then
    cat <<'EOF'
  Backend log:  .dev/logs/backend.log
  Backend pid:  .dev/pids/backend.pid
EOF
  elif [[ "$BACKEND_REUSED" == "1" ]]; then
    cat <<'EOF'
  Backend: 既存プロセスを利用しています。今回更新した .env を反映するには backend を再起動してください。
EOF
  fi

  if [[ "$FRONTEND_STARTED" == "1" ]]; then
    cat <<'EOF'
  Frontend log: .dev/logs/frontend.log
  Frontend pid: .dev/pids/frontend.pid
EOF
  elif [[ "$FRONTEND_REUSED" == "1" ]]; then
    cat <<'EOF'
  Frontend: 既存プロセスを利用しています。
EOF
  fi

  if [[ "$BACKEND_STARTED" == "1" || "$FRONTEND_STARTED" == "1" ]]; then
    cat <<'EOF'
Stop app servers:
  test -f .dev/pids/backend.pid && kill $(cat .dev/pids/backend.pid)
  test -f .dev/pids/frontend.pid && kill $(cat .dev/pids/frontend.pid)
EOF
  fi
else
  cat <<'EOF'
  backend/frontend は --skip-app により起動していません。
  起動する場合:
    make backend-dev
    make frontend-dev
EOF
fi

cat <<'EOF'
Stop Docker services:
  make down
  make zitadel-down

Note:
  HaoHao は初期状態では AUTH_MODE=local です。
  Zitadel 認証に切り替える場合は、Zitadel Console で HaoHao 用 Project / OIDC application を作成し、
  .env に AUTH_MODE=zitadel, ZITADEL_ISSUER, ZITADEL_CLIENT_ID, ZITADEL_CLIENT_SECRET を設定してください。

EOF
