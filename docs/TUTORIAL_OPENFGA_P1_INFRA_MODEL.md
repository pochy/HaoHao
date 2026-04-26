# Phase 1: OpenFGA infra / model 実装チュートリアル

## この文書の目的

この文書は、OpenFGA Drive 導入の最初のフェーズとして、OpenFGA runtime、backend config、authorization model、bootstrap script を追加する手順書です。

この Phase では Drive DB schema や API はまだ作りません。まず、HaoHao backend が OpenFGA に接続でき、repo 管理された model を明示的に登録できる状態を作ります。

## この Phase でやること / やらないこと

やること:

```text
OpenFGA runtime を compose に追加する
backend config に OpenFGA 接続設定を追加する
backend /readyz から OpenFGA /healthz を確認できるようにする
authorization model と model test を repo 管理する
store/model 登録用 bootstrap script を追加する
```

やらないこと:

```text
Drive DB schema
Drive API
Drive UI
OpenFGA Go SDK wrapper
DriveAuthorizationService
```

OpenFGA Go SDK は Phase 3 で追加します。Phase 1 では backend 起動時に model を自動作成・自動更新しません。

## 完成条件

- `compose.yaml` に OpenFGA 専用 PostgreSQL、`openfga-migrate`、`openfga` がある
- local OpenFGA HTTP API が `127.0.0.1:8088` に expose される
- production 相当では playground を無効化できる
- backend config に OpenFGA env が追加される
- `OPENFGA_ENABLED=true` のとき `/readyz` が OpenFGA HTTP `/healthz` を確認する
- `openfga/drive.fga` が存在する
- runtime 起動時に store/model を自動変更しない
- `scripts/openfga-bootstrap.sh` で store 作成と model 登録を行える
- `.env` に `OPENFGA_STORE_ID` と `OPENFGA_AUTHORIZATION_MODEL_ID` を設定できる

## 実装時の注意

この repository のローカル環境では `docker compose` ではなく `docker-compose` だけが有効な場合があります。手順内では `docker compose` を例示しますが、失敗する場合は `docker-compose` に置き換えます。

```bash
if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
else
  COMPOSE="docker-compose"
fi
```

また、PostgreSQL 18 image は `/var/lib/postgresql/data` ではなく `/var/lib/postgresql` に volume を mount します。`/var/lib/postgresql/data` に mount すると container が起動しないことがあります。

## OpenFGA CLI のインストール

Phase 1 の `make openfga-bootstrap` と `make test-openfga-model` は `fga` CLI を使います。

OpenFGA 公式の CLI install 方法は次です。

### macOS / Homebrew

```bash
brew install openfga/tap/fga
```

install 後に `fga` が見つからない場合は、Homebrew link と PATH を確認します。

```bash
brew list fga
brew link fga
which -a fga
```

Homebrew の cellar には入っているが symlink されていない場合、一時的には次の PATH 追加でも実行できます。

```bash
export PATH="/opt/homebrew/opt/fga/bin:$PATH"
```

### Go install

```bash
go install github.com/openfga/cli/cmd/fga@latest
```

`go install` を使う場合は、`$(go env GOPATH)/bin` が `PATH` に入っていることを確認します。

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

### Linux packages

`.deb`、`.rpm`、`.apk` は OpenFGA CLI releases page から download して install します。

```bash
sudo apt install ./fga_<version>_linux_<arch>.deb
sudo dnf install ./fga_<version>_linux_<arch>.rpm
sudo apk add --allow-untrusted ./fga_<version>_linux_<arch>.apk
```

### Docker で使う

local に CLI を入れずに一時実行する場合は Docker image を使います。

```bash
docker pull openfga/cli
docker run -it openfga/cli
```

この tutorial では repo mount と network 指定が必要なため、model test や store 作成の Docker 代替コマンドは後続 step に具体例を書きます。

### 確認

```bash
which -a fga
fga version
jq --version
```

確認例:

```text
fga version v`0.7.12`
```

`scripts/openfga-bootstrap.sh` は CLI output の parse に `jq` も使います。macOS では次で install できます。

```bash
brew install jq
```

## Step 1. compose に OpenFGA を追加する

### 対象ファイル

```text
compose.yaml
```

### 実装方針

HaoHao 本体の PostgreSQL と OpenFGA の PostgreSQL は分けます。

追加する service:

```text
openfga-postgres
openfga-migrate
openfga
```

local 開発では OpenFGA HTTP API を `127.0.0.1:8088` に公開します。OpenFGA gRPC や playground は必要になってから公開します。

### 追加イメージ

`compose.yaml` の既存 `postgres` / `redis` service と同じ network 上に追加します。

```yaml
  openfga-postgres:
    image: postgres:18
    environment:
      POSTGRES_USER: openfga
      POSTGRES_PASSWORD: openfga
      POSTGRES_DB: openfga
    volumes:
      - openfga-postgres-data:/var/lib/postgresql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U openfga -d openfga"]
      interval: 5s
      timeout: 3s
      retries: 20

  openfga-migrate:
    image: openfga/openfga:v1.11.6
    depends_on:
      openfga-postgres:
        condition: service_healthy
    command: migrate
    environment:
      OPENFGA_DATASTORE_ENGINE: postgres
      OPENFGA_DATASTORE_URI: postgres://openfga:openfga@openfga-postgres:5432/openfga?sslmode=disable

  openfga:
    image: openfga/openfga:v1.11.6
    depends_on:
      openfga-migrate:
        condition: service_completed_successfully
    command: run
    environment:
      OPENFGA_DATASTORE_ENGINE: postgres
      OPENFGA_DATASTORE_URI: postgres://openfga:openfga@openfga-postgres:5432/openfga?sslmode=disable
      OPENFGA_HTTP_ADDR: 0.0.0.0:8080
      OPENFGA_PLAYGROUND_ENABLED: ${OPENFGA_PLAYGROUND_ENABLED:-true}
    ports:
      - "127.0.0.1:8088:8080"
    healthcheck:
      test: ["CMD", "/usr/local/bin/grpc_health_probe", "-addr=openfga:8081"]
      interval: 5s
      timeout: 30s
      retries: 3
```

`volumes:` にも OpenFGA 用 volume を追加します。

```yaml
volumes:
  openfga-postgres-data:
```

OpenFGA image には shell / curl が無いため、Docker healthcheck は image に含まれる `grpc_health_probe` を使います。これは container 自身の healthcheck 用です。HaoHao backend の `/readyz` は HTTP `/healthz` を確認します。

production では `OPENFGA_PLAYGROUND_ENABLED=false` とし、API token や network boundary で保護します。

### 確認

```bash
$COMPOSE up -d openfga-postgres openfga-migrate openfga
curl -fsS http://127.0.0.1:8088/healthz
```

期待値:

```json
{"status":"SERVING"}
```

## Step 2. backend config を追加する

### 対象ファイル

```text
backend/internal/config/*
backend/internal/platform/readiness.go
backend/internal/app/health_test.go
backend/cmd/main/main.go
.env.example
```

### 追加する env

```dotenv
OPENFGA_ENABLED=false
OPENFGA_API_URL=http://127.0.0.1:8088
OPENFGA_STORE_ID=
OPENFGA_AUTHORIZATION_MODEL_ID=
OPENFGA_API_TOKEN=
OPENFGA_TIMEOUT=2s
OPENFGA_FAIL_CLOSED=true
```

### 実装方針

`OPENFGA_ENABLED=true` の場合は次を必須にします。

- `OPENFGA_API_URL`
- `OPENFGA_STORE_ID`
- `OPENFGA_AUTHORIZATION_MODEL_ID`
- `OPENFGA_TIMEOUT`

`OPENFGA_FAIL_CLOSED=false` は local debugging 用に残してもよいですが、production profile では禁止または warning にします。

### Go config の形

既存の config style に合わせて、次のような構造を追加します。

```go
type OpenFGAConfig struct {
	Enabled              bool
	APIURL               string
	StoreID              string
	AuthorizationModelID string
	APIToken             string
	Timeout              time.Duration
	FailClosed           bool
}
```

既存 config の validation と同じ場所で、`Enabled` 時の必須値を検査します。

`OPENFGA_ENABLED=true` の場合、HaoHao `/readyz` は `OPENFGA_API_URL + "/healthz"` を確認します。`OPENFGA_API_TOKEN` が設定されている場合だけ `Authorization: Bearer ...` を付与します。

validation の期待動作:

```text
OPENFGA_ENABLED=false: store/model ID が空でも起動できる
OPENFGA_ENABLED=true: API URL / store ID / model ID / positive timeout が必須
OPENFGA_API_URL: 末尾 slash は除去する
OPENFGA_FAIL_CLOSED: default true
```

## Step 3. OpenFGA model を repo に追加する

### 対象ファイル

```text
openfga/drive.fga
```

### 内容

`OPENFGA_IMPLEMENTATION_PLAN.md` の初期 model をそのまま置きます。

```fga
model
  schema 1.1

type user

type group
  relations
    define member: [user]

type share_link

type folder
  relations
    define parent: [folder]
    define owner: [user, group#member] or owner from parent
    define editor: [user, group#member] or owner or editor from parent
    define viewer: [user, group#member, share_link with not_expired] or editor or viewer from parent
    define can_view: viewer
    define can_edit: editor
    define can_delete: owner
    define can_share: owner

type file
  relations
    define parent: [folder]
    define owner: [user, group#member] or owner from parent
    define editor: [user, group#member] or owner or editor from parent
    define viewer: [user, group#member, share_link with not_expired] or editor or viewer from parent
    define can_view: viewer
    define can_download: viewer
    define can_edit: editor
    define can_delete: owner
    define can_share: owner

condition not_expired(current_time: timestamp, expires_at: timestamp) {
  current_time < expires_at
}
```

### ID 方針

OpenFGA object ID は DB の `public_id` から作ります。

```text
user:<users.public_id>
group:<drive_groups.public_id>
folder:<drive_folders.public_id>
file:<file_objects.public_id>
share_link:<drive_share_links.public_id>
```

tenant ID は OpenFGA object ID に入れません。tenant 境界は backend が DB で先に確認します。

## Step 4. bootstrap script を追加する

### 対象ファイル

```text
scripts/openfga-bootstrap.sh
Makefile
```

### 実装方針

runtime backend は store 作成や model 更新を行いません。bootstrap は明示的な script と Make target で実行します。

script の責務:

1. OpenFGA API URL を受け取る
2. store を作成する、または既存 store ID を使う
3. `openfga/drive.fga` を JSON DSL に変換して authorization model を登録する
4. store ID と authorization model ID を stdout に出す
5. `.env` へ raw secret を勝手に書き込まない

### Make target

```makefile
openfga-bootstrap:
	bash scripts/openfga-bootstrap.sh
```

script は local dev のために次の env を読む形にします。

```bash
OPENFGA_API_URL="${OPENFGA_API_URL:-http://127.0.0.1:8088}"
OPENFGA_STORE_NAME="${OPENFGA_STORE_NAME:-haohao-drive-dev}"
OPENFGA_MODEL_FILE="${OPENFGA_MODEL_FILE:-openfga/drive.fga}"
```

OpenFGA DSL から JSON model への変換は `fga` CLI に任せます。script は `fga` と `jq` を依存コマンドとして検査します。

HaoHao env と CLI env の対応は次です。

```text
OPENFGA_API_URL -> FGA_API_URL
OPENFGA_API_TOKEN -> FGA_API_TOKEN
OPENFGA_STORE_ID -> fga --store-id
```

```bash
command -v fga >/dev/null 2>&1 || {
  echo "fga CLI is required" >&2
  exit 1
}
command -v jq >/dev/null 2>&1 || {
  echo "jq is required to parse fga CLI output" >&2
  exit 1
}
```

script は `.env` を自動編集しません。出力された `OPENFGA_STORE_ID` と `OPENFGA_AUTHORIZATION_MODEL_ID` を開発者が `.env` に反映します。

local に `fga` CLI がない環境で store/model 登録だけ確認したい場合は、OpenFGA CLI の Docker image を使います。

```bash
docker run --rm \
  --network haohao_default \
  -v "$PWD:/work" \
  -w /work \
  openfga/cli:latest \
  store create \
  --name haohao-drive-dev \
  --model openfga/drive.fga \
  --api-url http://openfga:8080
```

この Docker 代替は `scripts/openfga-bootstrap.sh` そのものの実行ではありません。CI や開発端末で script を確認するには local `fga` と `jq` を入れます。

local `fga` CLI で script が成功すると、stdout に次の形式で出力されます。

```dotenv
OPENFGA_STORE_ID=01...
OPENFGA_AUTHORIZATION_MODEL_ID=01...
```

実行例:

```bash
PATH="/opt/homebrew/opt/fga/bin:$PATH" \
OPENFGA_STORE_NAME="haohao-drive-cli-$(date +%s)" \
bash scripts/openfga-bootstrap.sh
```

## Step 5. model test を追加する

### 対象ファイル

```text
openfga/drive.fga.yaml
```

### 確認すること

- owner は viewer / editor / can_delete / can_share を満たす
- editor は viewer / can_edit を満たす
- viewer は can_view のみ満たす
- parent folder viewer は child folder / child file viewer になる
- parent folder editor は child folder / child file editor になる
- parent tuple を消すと継承が止まる
- group member は group share 経由で access できる
- expired share link は access できない

### Make target

```makefile
test-openfga-model:
	cd openfga && fga model test --tests drive.fga.yaml
```

## Phase 1 の完了確認

```bash
$COMPOSE config
$COMPOSE up -d openfga-postgres openfga-migrate openfga
curl -fsS http://127.0.0.1:8088/healthz
make openfga-bootstrap
make test-openfga-model
go test ./backend/internal/config ./backend/internal/platform ./backend/internal/app
```

local に `fga` CLI がない環境では、model test は次のように Docker CLI image で代替確認できます。

```bash
docker run --rm -v "$PWD:/work" -w /work/openfga openfga/cli:latest model test --tests drive.fga.yaml
```

期待値:

```text
# Test Summary #
Tests 5/5 passing
Checks 43/43 passing
```

`.env` には bootstrap 結果を反映します。

```dotenv
OPENFGA_ENABLED=true
OPENFGA_API_URL=http://127.0.0.1:8088
OPENFGA_STORE_ID=<bootstrap output>
OPENFGA_AUTHORIZATION_MODEL_ID=<bootstrap output>
OPENFGA_FAIL_CLOSED=true
```

OpenFGA を有効化した backend `/readyz` を実プロセスで確認する場合は、登録済み store/model ID を使って別 port で起動します。

```bash
HTTP_PORT=18080 \
DATABASE_URL='postgres://haohao:haohao@127.0.0.1:5432/haohao?sslmode=disable' \
REDIS_ADDR=127.0.0.1:6379 \
OPENFGA_ENABLED=true \
OPENFGA_API_URL=http://127.0.0.1:8088 \
OPENFGA_STORE_ID=<bootstrap output> \
OPENFGA_AUTHORIZATION_MODEL_ID=<bootstrap output> \
OPENFGA_FAIL_CLOSED=true \
go run ./backend/cmd/main
```

別 terminal で確認します。

```bash
curl -fsS http://127.0.0.1:18080/readyz
```

期待値:

```json
{"status":"ok","checks":{"openfga":{"status":"ok"},"postgres":{"status":"ok"},"redis":{"status":"ok"}}}
```

## よくある詰まりどころ

### `docker: unknown command: docker compose`

Docker Compose v1 環境です。`docker-compose` を使います。

```bash
docker-compose config
docker-compose up -d openfga-postgres openfga-migrate openfga
```

### `openfga-postgres` が PostgreSQL 18 の data directory error で落ちる

`openfga-postgres-data:/var/lib/postgresql/data` に mount していないか確認します。PostgreSQL 18 では `/var/lib/postgresql` に mount します。

古い失敗 volume が残っている場合は、OpenFGA 専用 container / volume だけ削除して作り直します。

```bash
docker rm -f haohao-openfga haohao-openfga-migrate haohao-openfga-postgres
docker volume rm haohao_openfga-postgres-data
```

### `make openfga-bootstrap` が `fga CLI is required` で失敗する

local に `fga` CLI がありません。Homebrew などで CLI を入れるか、上記の Docker CLI image 代替コマンドで model 登録互換を確認します。

Homebrew で install 済みなのに見つからない場合は、link されていない可能性があります。

```bash
brew list fga
brew link fga
export PATH="/opt/homebrew/opt/fga/bin:$PATH"
fga version
```

### backend `/readyz` に `openfga` が出ない

`OPENFGA_ENABLED=true` で起動しているか確認します。`OPENFGA_ENABLED=false` の場合、OpenFGA readiness check は実行されません。
