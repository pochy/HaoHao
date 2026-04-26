# Phase 1: OpenFGA infra / model 実装チュートリアル

## この文書の目的

この文書は、OpenFGA Drive 導入の最初のフェーズとして、OpenFGA runtime、backend config、authorization model、bootstrap script を追加する手順書です。

この Phase では Drive DB schema や API はまだ作りません。まず、HaoHao backend が OpenFGA に接続でき、repo 管理された model を明示的に登録できる状態を作ります。

## 完成条件

- `compose.yaml` に OpenFGA 専用 PostgreSQL、`openfga-migrate`、`openfga` がある
- local OpenFGA HTTP API が `127.0.0.1:8088` に expose される
- production 相当では playground を無効化できる
- backend config に OpenFGA env が追加される
- `openfga/drive.fga` が存在する
- runtime 起動時に store/model を自動変更しない
- `scripts/openfga-bootstrap.sh` で store 作成と model 登録を行える
- `.env` に `OPENFGA_STORE_ID` と `OPENFGA_AUTHORIZATION_MODEL_ID` を設定できる

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
      - openfga-postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U openfga -d openfga"]
      interval: 5s
      timeout: 3s
      retries: 20

  openfga-migrate:
    image: openfga/openfga:latest
    depends_on:
      openfga-postgres:
        condition: service_healthy
    command: migrate
    environment:
      OPENFGA_DATASTORE_ENGINE: postgres
      OPENFGA_DATASTORE_URI: postgres://openfga:openfga@openfga-postgres:5432/openfga?sslmode=disable

  openfga:
    image: openfga/openfga:latest
    depends_on:
      openfga-migrate:
        condition: service_completed_successfully
    command: run
    environment:
      OPENFGA_DATASTORE_ENGINE: postgres
      OPENFGA_DATASTORE_URI: postgres://openfga:openfga@openfga-postgres:5432/openfga?sslmode=disable
      OPENFGA_HTTP_ADDR: 0.0.0.0:8080
      OPENFGA_PLAYGROUND_ENABLED: "true"
    ports:
      - "127.0.0.1:8088:8080"
```

`volumes:` にも OpenFGA 用 volume を追加します。

```yaml
volumes:
  openfga-postgres-data:
```

production では `OPENFGA_PLAYGROUND_ENABLED=false` とし、API token や network boundary で保護します。

### 確認

```bash
docker compose up -d openfga-postgres openfga-migrate openfga
curl -fsS http://127.0.0.1:8088/healthz
```

## Step 2. backend config を追加する

### 対象ファイル

```text
backend/internal/config/*
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

OpenFGA DSL から JSON model への変換は、導入する CLI に合わせます。CLI が未導入の開発者にも分かるよう、script の先頭で依存コマンドを検査します。

```bash
command -v fga >/dev/null 2>&1 || {
  echo "fga CLI is required for model transform" >&2
  exit 1
}
```

## Step 5. model test を追加する

### 対象ファイル

```text
openfga/drive_tests.yaml
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
	fga model test --tests openfga/drive_tests.yaml
```

## Phase 1 の完了確認

```bash
docker compose up -d openfga-postgres openfga-migrate openfga
curl -fsS http://127.0.0.1:8088/healthz
make openfga-bootstrap
make test-openfga-model
go test ./backend/internal/config ./backend/internal/service
```

`.env` には bootstrap 結果を反映します。

```dotenv
OPENFGA_ENABLED=true
OPENFGA_API_URL=http://127.0.0.1:8088
OPENFGA_STORE_ID=<bootstrap output>
OPENFGA_AUTHORIZATION_MODEL_ID=<bootstrap output>
OPENFGA_FAIL_CLOSED=true
```

