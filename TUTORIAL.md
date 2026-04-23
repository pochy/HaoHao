# `CONCEPT.md` をそのまま実装に落とすチュートリアル

## この文書の目的

この文書は、`CONCEPT.md` に書かれた方針を、実際に手を動かして組み立てていくためのチュートリアルに変換したものです。

目的は要約ではありません。目的は、**「今どのファイルを作るべきか」「そのファイルには何を書くべきか」「なぜその順番なのか」**を、迷わず追えるようにすることです。

特にこのチュートリアルでは、次の 3 点を重視します。

- 1 ファイルずつ進められること
- 各ファイルの役割が明確であること
- 生成物と手書きファイルの境界がはっきりしていること

## 動作環境

このチュートリアルは、次の環境で動かす前提で書いています。

### 必要環境

- Go `1.26.0`
- Node.js `22`
- npm
- Docker Engine / Docker Compose `v2`
- GNU Make
- `sqlc`
- `golang-migrate`
- `curl`
- `awk`
- macOS

### このチュートリアルで使うローカル構成

- backend: Go + Gin + Huma
- frontend: Vue 3 + Vite + TypeScript + Pinia
- database: PostgreSQL `18`
- session store: Redis `7.4`
- OpenAPI artifact: `openapi/openapi.yaml`
- generated SDK: `frontend/src/api/generated/`

### ローカルで使う URL / port

| 対象 | URL / 接続先 | 用途 |
| --- | --- | --- |
| frontend dev server | `http://127.0.0.1:5173` | Vite の開発画面 |
| backend HTTP | `http://127.0.0.1:8080` | API / docs / OpenAPI |
| PostgreSQL | `127.0.0.1:5432` | `DATABASE_URL` の接続先 |
| Redis | `127.0.0.1:6379` | セッションストア |

### この文書が前提にしている段階

このチュートリアルは foundation 段階に相当します。ここでいう foundation は、PostgreSQL、Redis、backend、frontend だけで最初の縦切り機能を完成させる段階です。

- 認証基盤はまだ Zitadel に寄せません
- login は PostgreSQL の `pgcrypto` を使う簡易実装で確認します
- Zitadel への移行は、この 1 周目が完成した後の段階として扱います

### 最初に確認しておくとよいコマンド

```bash
go version
node --version
npm --version
docker compose version
command -v make
command -v sqlc
command -v migrate
command -v curl
command -v awk
```

## このチュートリアルで作るもの

この文書は、`CONCEPT.md` の方針を「実際に手を動かせる順番」に並べ直した手順書です。

今回は最初の 1 本目として、次の縦切り機能を完成させます。

- `POST /api/v1/login`
- `GET /api/v1/session`
- `POST /api/v1/logout`
- Vue から Cookie 認証でこれらを呼び出す
- Huma から OpenAPI 3.1 を生成する
- OpenAPI から TypeScript SDK を生成する

つまり、`CONCEPT.md` の中核である次の流れを最小構成で一周させます。

1. SQL を書く
2. `sqlc` で Go コードを生成する
3. Huma で API 契約と実装をつなぐ
4. OpenAPI を export する
5. Vue から generated SDK を使う

この題材を選ぶ理由は単純です。`CONCEPT.md` の核になっている要素がほぼ全部入っているからです。

- Huma による OpenAPI 3.1 生成
- `sqlc` による SQL ベースのデータアクセス
- BFF + HttpOnly Cookie
- CSRF 対策
- 生成 client を使う Vue 側の接続
- 最終的な単一バイナリ配信

つまり、この最初の縦切り機能が作れれば、以後の業務機能は同じ型で増やしていけます。

## 完成イメージ

最終的な構成は、`CONCEPT.md` にある次の形をベースにします。

```text
my-enterprise-app/
├── docs/
├── openapi/
│   └── openapi.yaml
├── go.work
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   ├── views/
│   │   ├── composables/
│   │   ├── stores/
│   │   └── api/
│   ├── vite.config.ts
│   └── package.json
├── backend/
│   ├── cmd/
│   ├── internal/
│   │   ├── api/
│   │   ├── service/
│   │   ├── db/
│   │   ├── auth/
│   │   ├── config/
│   │   └── platform/
│   ├── go.mod
│   ├── sqlc.yaml
│   ├── web/
│   │   └── dist/
│   └── embed.go
├── db/
│   ├── migrations/
│   ├── queries/
│   └── schema.sql
├── compose.yaml
├── docker/
│   └── Dockerfile
├── scripts/
│   ├── gen.sh
│   └── seed-demo-user.sql
├── .github/workflows/
└── Makefile
```

## このチュートリアルの読み方

この文書は、上から順に読んで、そのまま順番にファイルを作っていく前提で書いています。

進め方のルールは次の通りです。

1. まず「土台になるファイル」を作る
2. 次に「DB と SQL の正本」を作る
3. その後に「Go 側の API 契約と実装」を作る
4. 生成物を出力する
5. 最後に「Vue 側から生成 client を使って接続する」

この順番を崩すと、途中で次の問題が起きやすくなります。

- フロントの型が先にできてしまい、OpenAPI とずれる
- API 実装が先に進み、DB 設計が後追いになる
- 生成物の出所が曖昧になり、手書き修正が入り込む

## 実装順の全体像

| フェーズ | 主な Step | 主な対象ファイル | このフェーズの目的 |
| --- | --- | --- | --- |
| 1 | Step 1-3 | `go.work`, `backend/go.mod`, `compose.yaml`, `Makefile`, `scripts/gen.sh` | リポジトリ全体の実行土台を作る |
| 2 | Step 4-5 | `db/migrations/*`, `db/schema.sql`, `db/queries/*`, `backend/sqlc.yaml` | DB と SQL を正本として固める |
| 3 | Step 6-12 | `backend/internal/*`, `backend/cmd/*`, `openapi/openapi.yaml` | Huma で API 契約と実装を結ぶ |
| 4 | Step 13-19 | `frontend/package.json`, `frontend/vite.config.ts`, `frontend/src/*` | 生成 client を transport wrapper 経由で使う |
| 5 | Step 20, 発展 1-3 | `backend/embed.go`, `docker/Dockerfile`, `.github/workflows/ci.yml` | 単一バイナリ配信と CI で仕上げる |

## 各フェーズの完了条件

### フェーズ 1 の完了条件

- `go.work` と `backend/go.mod` があり、Go 側の依存が解決できる
- `compose.yaml` で PostgreSQL と Redis が起動できる
- `Makefile` と `scripts/gen.sh` があり、標準コマンドの入口が決まっている

### フェーズ 2 の完了条件

- migration の `up` と `down` が動く
- `db/schema.sql` が migration 由来で再生成されている
- `db/queries/*.sql` が最小機能を表現できている
- `backend/sqlc.yaml` から `sqlc generate` が通る

### フェーズ 3 の完了条件

- `go run ./backend/cmd/main` で API が起動する
- docs / OpenAPI endpoint が確認できる
- `go run ./backend/cmd/openapi > openapi/openapi.yaml` が通る
- `openapi/openapi.yaml` に session 系 endpoint が出力される

### フェーズ 4 の完了条件

- `npm run dev` でフロントが起動する
- Vite の proxy 経由で `/api/v1/session` が呼べる
- 生成 client を直接ではなく wrapper 経由で使っている
- login / logout / current session の一連の導線が画面から確認できる

### フェーズ 5 の完了条件

- `frontend` を build すると `backend/web/dist/` に出力される
- Go バイナリ 1 つで API と SPA を返せる
- Docker build が通る
- CI で生成漏れと build 崩れを検知できる

## 先に理解しておくこと

このチュートリアルでは、次の 2 種類のファイルを明確に分けます。

### 手で書くファイル

- `db/migrations/*.sql`
- `db/queries/*.sql`
- `backend/internal/**/*.go`
- `frontend/src/**/*.ts`
- `frontend/src/**/*.vue`
- `compose.yaml`
- `Makefile`
- `scripts/*.sh`
- `scripts/*.sql`

### 生成されるファイル

- `db/schema.sql`
- `backend/internal/db/*`
- `openapi/openapi.yaml`
- `frontend/src/api/generated/*`
- `backend/go.mod`, `backend/go.sum`
- `frontend` の lockfile

重要なのは、**生成物を直接直さない**ことです。

- OpenAPI が変なら `backend/internal/api/*` を直す
- SQL 生成コードが変なら `db/queries/*` または `backend/sqlc.yaml` を直す
- `schema.sql` が変なら migration を直す

## 前提ツール

上の動作環境で挙げたもののうち、ローカルへ入れておく対象をここで再掲します。macOS を想定して書いていますが、ほかの環境でもインストール方法を読み替えれば進められます。

最低限必要なのは次です。

- Go 1.26.0
- Node.js 22 以上 (`npm` を含む)
- Docker Engine / Docker Compose v2
- GNU Make
- `sqlc`
- `golang-migrate`
- `curl`
- `awk`

macOS では `make`, `curl`, `awk` は通常プリインストールです。Docker Engine / Docker Compose v2 は Docker Desktop などで別途入れてください。

そのほかのツールは、たとえば次のように入れられます。

```bash
brew install go node sqlc golang-migrate
```

`CONCEPT.md` の方針どおり、DB は PostgreSQL 18、セッションストアは Redis を使います。

## 進め方

各ステップで、次の 5 点を必ず書きます。

- 何を作るか
- なぜその順番でやるのか
- 実行するコマンド
- 実際に書くファイル
- その段階で何を確認するか

上から順に進めてください。途中で順番を変えないほうが楽です。

---

## Step 1. 作業ディレクトリを作る

### この Step を最初にやる理由

この Step は単にフォルダを作るだけに見えますが、実際にはかなり重要です。ここでディレクトリ境界を先に固定しておかないと、後からファイルの置き場所がぶれます。

この構成では、置き場所のぶれがそのまま責務のぶれに直結します。たとえば API 契約が `backend/internal/api/` にまとまっていないと、どの Go ファイルが OpenAPI の正本なのか分からなくなります。`db/` と `backend/internal/db/` の区別が曖昧だと、どこが SQL の正本でどこが生成物かも崩れます。

つまりこの Step は、「空のディレクトリを作る作業」ではなく、**今後ずっと守る設計境界を最初に物理的に固定する作業**です。

### ここでやること

まず、`CONCEPT.md` にある monorepo の骨組みを作ります。

### 実行コマンド

```bash
mkdir -p openapi scripts
mkdir -p db/migrations db/queries
mkdir -p backend/cmd/main backend/cmd/openapi
mkdir -p backend/internal/app
mkdir -p backend/internal/api
mkdir -p backend/internal/auth
mkdir -p backend/internal/config
mkdir -p backend/internal/platform
mkdir -p backend/internal/service
mkdir -p backend/web/dist
```

### なぜ最初にこれをやるのか

この構成では、後から「どこに置くべきか」で迷うと一気に崩れます。

特に次の境界は最初から固定してください。

- DB 正本は `db/`
- Go 実装は `backend/internal/`
- OpenAPI artifact は `openapi/`
- フロント実装は `frontend/`

### 確認

```bash
find . -maxdepth 3 -type d | sort
```

`backend/internal/...`, `db/...`, `openapi/`, `scripts/` が見えていれば大丈夫です。

---

## Step 2. ルートの基本ファイルを作る

### この Step を次にやる理由

Step 1 で箱を作ったら、次はその箱をどう動かすかを決めます。ここで作るのは `.gitignore`, `go.work`, `.env.example`, `compose.yaml`, `scripts/gen.sh`, `Makefile` です。

これらは機能実装のファイルではありませんが、以後のすべての作業手順を決めるファイルです。先にこれらを作る理由は、後から人によってコマンドや起動方法がずれると、同じ実装でも再現手順が変わってしまうからです。

`CONCEPT.md` の肝は「OpenAPI 3.1 優先」と「生成物をレビュー可能に保つ」ことです。これを守るには、生成、起動、DB 初期化の入口が最初から統一されている必要があります。

このステップでは、monorepo 全体の土台になるファイルを作ります。

### 2-1. `.gitignore`

#### 何をするか

生成物とローカル専用ファイルを整理します。

#### ファイル: `.gitignore`

```gitignore
.DS_Store
.env

frontend/node_modules
frontend/dist

backend/web/dist

*.log
cookies.txt
```

#### 解説

ここで大事なのは、`openapi/openapi.yaml` と `frontend/src/api/generated/` を **ignore しない** ことです。`CONCEPT.md` の方針では、これらはレビュー対象だからです。

---

### 2-2. `go.work`

#### 何をするか

repo root から `backend/` module を扱えるようにします。

#### ファイル: `go.work`

```go
go 1.26.0

use ./backend
```

#### 解説

`CONCEPT.md` にあった「repo root から `go run ./backend/...` を呼ぶ」方針をここで固定します。

---

### 2-3. `.env.example`

#### 何をするか

バックエンドの起動設定を環境変数に寄せます。

#### ファイル: `.env.example`

```dotenv
APP_NAME="HaoHao API"
APP_VERSION=0.1.0
HTTP_PORT=8080

DATABASE_URL=postgres://haohao:haohao@127.0.0.1:5432/haohao?sslmode=disable

REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=
REDIS_DB=0

SESSION_TTL=24h
COOKIE_SECURE=false
```

#### 実行コマンド

```bash
cp .env.example .env
```

#### 解説

この段階では `dotenv` ライブラリは使いません。DB 操作系と backend 起動系の `Makefile` ターゲット、および `scripts/gen.sh` が `.env` を読む前提にして、生成手順と起動手順で設定源がずれないようにします。`make openapi` だけは `.env` なしで動かします。

---

### 2-4. `compose.yaml`

#### 何をするか

PostgreSQL 18 と Redis をローカルで起動できるようにします。

`5432` がすでに埋まっている環境では、host 側 port と `.env.example` の `DATABASE_URL` を同じ値へ揃えて変更してください。たとえば `5433:5432` にするなら、`DATABASE_URL` も `127.0.0.1:5433` に合わせます。

#### ファイル: `compose.yaml`

```yaml
services:
  postgres:
    image: postgres:18
    container_name: haohao-postgres
    environment:
      POSTGRES_DB: haohao
      POSTGRES_USER: haohao
      POSTGRES_PASSWORD: haohao
    ports:
      - "5432:5432"
    volumes:
      - postgres-data:/var/lib/postgresql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U haohao -d haohao"]
      interval: 5s
      timeout: 5s
      retries: 10

  redis:
    image: redis:7.4
    container_name: haohao-redis
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data

volumes:
  postgres-data:
  redis-data:
```

#### 解説

`CONCEPT.md` の「開発時は依存サービスを `compose.yaml` に寄せる」方針をそのまま使います。

---

### 2-5. `scripts/gen.sh`

#### 何をするか

生成手順を 1 つのスクリプトにまとめます。

#### ファイル: `scripts/gen.sh`

```bash
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
```

#### 実行コマンド

```bash
chmod +x scripts/gen.sh
```

#### 解説

この順番が大事です。

1. SQL から Go コードを作る
2. Go + Huma から OpenAPI を出す
3. OpenAPI から TypeScript SDK を作る

`cmd/openapi` は最終的に `service` や `internal/db` を含む backend 全体をコンパイルするので、`sqlc` を先に回しておかないと、手順どおりに進めた読者が途中で詰まります。`.env` はあってもなくても進められるようにしつつ、ローカルで export 手順に設定を足したくなったときにもスクリプト側だけで吸収できる形にしています。

---

### 2-6. `Makefile`

#### 何をするか

日常的に叩くコマンドを固定します。

#### ファイル: `Makefile`

```make
SHELL := /bin/bash

export-env = set -a && source .env && set +a
DOCKER_COMPOSE := $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; elif command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"; else echo "docker compose"; fi)

up:
	$(DOCKER_COMPOSE) up -d

down:
	$(DOCKER_COMPOSE) down

db-up:
	$(export-env) && migrate -path db/migrations -database "$$DATABASE_URL" up

db-down:
	$(export-env) && migrate -path db/migrations -database "$$DATABASE_URL" down 1

db-schema:
	$(DOCKER_COMPOSE) exec -T postgres pg_dump --schema-only --no-owner --no-privileges -U haohao -d haohao > db/schema.sql

seed-demo-user:
	$(DOCKER_COMPOSE) exec -T postgres psql -U haohao -d haohao < scripts/seed-demo-user.sql

sqlc:
	cd backend && sqlc generate

openapi:
	go run ./backend/cmd/openapi > openapi/openapi.yaml

gen:
	./scripts/gen.sh

backend-dev:
	$(export-env) && go run ./backend/cmd/main

frontend-dev:
	cd frontend && npm run dev
```

#### 解説

この `Makefile` は「便利コマンド集」ではありません。チームの標準手順です。

`make openapi` だけは `.env` なしでも動く形にしておきます。OpenAPI export は DB や Redis の接続を必要としないので、CI でもそのまま実行できたほうが扱いやすいからです。

`db-schema` と `seed-demo-user` は、container 内から `DATABASE_URL` をそのまま使わず、container 内で確実に解決できる `-U haohao -d haohao` で実行しています。host 側 port を変えてもここは影響を受けません。

特に重要なのは次の 4 つです。

- `make db-up`
- `make db-schema`
- `make seed-demo-user`
- `make gen`

---

## Step 3. backend module を作る

### この Step をここでやる理由

Go のコードを書く前に module を作るのは、import path と依存関係を先に固定するためです。先にコードを書いてから module path を決めると、後で import をまとめて直すことになります。

また、この段階で `huma`, `humagin`, `gin`, `pgx`, `go-redis` を入れておくと、このリポジトリの実装方針が Go 側にも明確に出ます。つまり「この backend は何を前提に設計されているか」を、文章ではなく `go.mod` で表現できます。

この Step は、**以後の Go ファイルがどの世界観の上で書かれるのかを確定する Step**です。

### ここでやること

Go module を作り、必要な依存を入れます。

### 実行コマンド

```bash
cd backend
go mod init example.com/haohao/backend
go get github.com/danielgtaylor/huma/v2
go get github.com/danielgtaylor/huma/v2/adapters/humagin
go get github.com/gin-gonic/gin
go get github.com/jackc/pgx/v5
go get github.com/jackc/pgx/v5/pgxpool
go get github.com/redis/go-redis/v9
go get github.com/google/uuid
go mod tidy
cd ..
```

### 解説

module path の `example.com/haohao/backend` は、あとでそのまま import path に入ります。GitHub 上の本当の module 名が決まっているなら、ここで差し替えてください。

### 確認

```bash
sed -n '1,120p' backend/go.mod
```

次の依存が入っていれば十分です。

- `huma/v2`
- `humagin`
- `gin`
- `pgx/v5`
- `go-redis/v9`

---

## Step 4. 最初の migration を作る

### この Step を先にやる理由

このチュートリアルでは、Go の handler や Vue の画面より先に migration を作ります。理由は単純で、`CONCEPT.md` の世界では SQL とスキーマが設計資産だからです。

`sqlc` を使う構成では、DB が後追いになると困ります。先に API や service を書いてしまうと、あとで SQL を書くときに「アプリが欲しい形」に無理やり DB を合わせることになりがちです。これを避けるため、まず DB が何を保存し、何を返せるのかを先に決めます。

今回は `sessions` テーブルではなく Redis をセッションストアにし、DB には `users` だけを置いています。これも理由があります。最初の 1 周目で確認したいのは「認証可能なユーザーがいて、Redis にセッションを持てること」だからです。責務を分けるために、ユーザーの永続化とセッションの寿命管理を別の層に分けています。

### ここでやること

最初の 1 つ目のテーブルとして `users` を作ります。

今回は login を一番早く動かしたいので、**開発環境だけで使うデモユーザーは seed に分離します**。

- email: `demo@example.com`
- password: `changeme123`

### 4-1. `db/migrations/0001_init.up.sql`

#### ファイル: `db/migrations/0001_init.up.sql`

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    email TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

#### 解説

ここで `CONCEPT.md` の方針を 2 つ反映しています。

- 内部 ID は `BIGINT`
- 外部公開に使える ID は `UUIDv7`

今回は `users.public_id` を外部公開用 ID として確保します。

パスワード照合は、このチュートリアルでは PostgreSQL の `pgcrypto` を使います。理由は単純で、**最初の 1 周目を最短で動かすため**です。最終的に `CONCEPT.md` どおり Zitadel へ寄せるのは次の段階です。

seed データを migration に混ぜないのも重要です。schema の履歴と開発用データを分けておかないと、共有環境や将来の本番環境へそのまま流れ込みやすくなります。

---

### 4-2. `db/migrations/0001_init.down.sql`

#### ファイル: `db/migrations/0001_init.down.sql`

```sql
DROP TABLE IF EXISTS users;
```

#### 解説

down migration も最初から用意してください。初期構成ほど作り直し回数が多いからです。

---

### 4-3. `scripts/seed-demo-user.sql`

#### ファイル: `scripts/seed-demo-user.sql`

```sql
INSERT INTO users (email, display_name, password_hash)
VALUES (
    'demo@example.com',
    'Demo User',
    crypt('changeme123', gen_salt('bf'))
)
ON CONFLICT (email) DO UPDATE
SET
    display_name = EXCLUDED.display_name,
    password_hash = EXCLUDED.password_hash,
    updated_at = now();
```

#### 解説

このファイルは **開発専用** です。versioned migration ではなく、ローカル確認を早くするための seed として扱ってください。

- schema を変えない
- 共有環境や本番では流さない
- ローカルで何度流しても壊れにくいように upsert にする

---

### 4-4. DB を起動して migration と seed を流す

#### 実行コマンド

```bash
make up
make db-up
make db-schema
make seed-demo-user
```

#### 解説

ここで `db/schema.sql` が作られ、動作確認用のユーザーも投入されます。このときも正本は migration です。`schema.sql` は **手で編集しません**。seed は schema ではなく開発用データなので、別ファイルに分けたままにします。

### 確認

```bash
sed -n '1,220p' db/schema.sql
```

`users` テーブルと `pgcrypto` extension が見えていれば大丈夫です。demo ユーザーは次で確認できます。

```bash
set -a && source .env && set +a && psql "$DATABASE_URL" -c "SELECT email, display_name FROM users;"
```

---

## Step 5. `sqlc` 用の設定と SQL を書く

### この Step を migration の直後にやる理由

migration でスキーマを確定したら、次は「そのスキーマに対してアプリが何をしたいか」を SQL で書きます。ここで先に SQL を書く理由は、service 層が欲しい操作の形を DB の言葉で固定するためです。

もしここを飛ばして先に service を書くと、service が仮の repository interface を持ち始めます。そうすると、あとで SQL を書いたときに repository と service のどちらを正とするかが曖昧になります。

`sqlc` は「SQL を書いたら Go の型安全なコードが得られる」道具なので、まず SQL を書き、その後に生成物を使う順番が自然です。つまりこの Step は、**service が依存する DB 操作を曖昧な interface ではなく SQL で明文化する Step**です。

### ここでやること

アプリケーションが DB に何を頼むのかを、先に SQL で固定します。

### 5-1. `backend/sqlc.yaml`

#### ファイル: `backend/sqlc.yaml`

```yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "../db/schema.sql"
    queries: "../db/queries"
    gen:
      go:
        package: "db"
        out: "internal/db"
        sql_package: "pgx/v5"
        emit_json_tags: true
        overrides:
          - db_type: "uuid"
            go_type:
              import: "github.com/google/uuid"
              type: "UUID"
```

#### 解説

`sqlc` は PostgreSQL の `uuid` を `pgtype.UUID` にもできますが、このチュートリアルでは扱いやすさを優先して `google/uuid` に寄せます。

---

### 5-2. `db/queries/users.sql`

#### ファイル: `db/queries/users.sql`

```sql
-- name: AuthenticateUser :one
SELECT id
FROM users
WHERE email = @email
  AND password_hash = crypt(@password, password_hash)
LIMIT 1;

-- name: GetUserByID :one
SELECT
    id,
    public_id,
    email,
    display_name
FROM users
WHERE id = $1
LIMIT 1;
```

#### 解説

ここでは `service` が欲しい操作単位で SQL を切っています。

- login 時に email + password で認証する
- session 復元時に user ID からユーザーを読む

`sqlc.arg` の短縮である `@email`, `@password` を使うことで、生成される Go の `Params` struct 名が読みやすくなります。

ここで `SELECT *` を避けているのも重要です。認証に不要な `password_hash` まで service 層へ持ち上げないように、返す列を明示しています。

---

### 5-3. `sqlc` 生成

#### 実行コマンド

```bash
cd backend
sqlc generate
sqlc vet
cd ..
```

#### 確認

```bash
find backend/internal/db -maxdepth 1 -type f | sort
```

`models.go`, `users.sql.go`, `db.go` のような生成ファイルが出ていれば成功です。

---

## Step 6. backend の設定読み込みと接続周りを書く

### この Step を service より前にやる理由

設定読み込みと接続初期化は、一見すると後回しにできそうですが、後回しにするとすぐにグローバル変数や ad-hoc な初期化コードが増えます。

ここで `config` と `platform` を先に切っておく理由は、以後の層に「どうやって環境変数を読むか」「どうやって DB/Redis へ接続するか」を漏らさないためです。service や API が接続の作り方を知り始めると、層の境界が崩れます。

この Step の目的は、単に接続を成功させることではありません。**設定の責務と接続の責務を、アプリ本体から切り離すこと**です。

このステップでは、HTTP サーバーそのものではなく、その前提になる部品を書きます。

### 6-1. `backend/internal/config/config.go`

#### ファイル: `backend/internal/config/config.go`

```go
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppName      string
	AppVersion   string
	HTTPPort     int
	DatabaseURL  string
	RedisAddr    string
	RedisPassword string
	RedisDB      int
	SessionTTL   time.Duration
	CookieSecure bool
}

func Load() (Config, error) {
	sessionTTL, err := time.ParseDuration(getEnv("SESSION_TTL", "24h"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppName:       getEnv("APP_NAME", "HaoHao API"),
		AppVersion:    getEnv("APP_VERSION", "0.1.0"),
		HTTPPort:      getEnvInt("HTTP_PORT", 8080),
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		RedisAddr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),
		SessionTTL:    sessionTTL,
		CookieSecure:  getEnvBool("COOKIE_SECURE", false),
	}, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}
```

#### 解説

`openapi` export コマンドでもこの設定を使いたいので、ここでは `DATABASE_URL must be set` のような強制終了はしていません。**必須チェックは runtime 側**でやります。

---

### 6-2. `backend/internal/platform/postgres.go`

#### ファイル: `backend/internal/platform/postgres.go`

```go
package platform

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPostgresPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}
```

---

### 6-3. `backend/internal/platform/redis.go`

#### ファイル: `backend/internal/platform/redis.go`

```go
package platform

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(ctx context.Context, addr, password string, db int) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}
```

#### 解説

`CONCEPT.md` ではセッションストアに Redis を推奨していました。ここでその前提を先にコード化します。

---

## Step 7. セッション管理の部品を書く

### この Step を独立させる理由

Cookie とセッションの扱いは、handler にも service にも関係します。そのため、どちらか一方に埋め込むと責務がすぐに肥大化します。

ここで `auth/cookies.go` と `auth/session_store.go` を独立させる理由は、Cookie の仕様と Redis 上のセッション寿命を横断関心事として切り出すためです。特に `SESSION_ID` と `XSRF-TOKEN` の扱いは、このリポジトリ全体で一貫している必要があります。

この Step を独立させておくと、あとでセッションストアを Redis から別方式に変えても、API や frontend の設計を大きく崩さずに済みます。

ここから「BFF + HttpOnly Cookie」を実装に落とします。

### 7-1. `backend/internal/auth/cookies.go`

#### ファイル: `backend/internal/auth/cookies.go`

```go
package auth

import (
	"net/http"
	"time"
)

const (
	SessionCookieName = "SESSION_ID"
	XSRFCookieName    = "XSRF-TOKEN"
)

func NewSessionCookie(value string, secure bool, ttl time.Duration) http.Cookie {
	return http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	}
}

func NewXSRFCookie(value string, secure bool, ttl time.Duration) http.Cookie {
	return http.Cookie{
		Name:     XSRFCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	}
}

func ExpiredSessionCookie(secure bool) http.Cookie {
	return http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
}

func ExpiredXSRFCookie(secure bool) http.Cookie {
	return http.Cookie{
		Name:     XSRFCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
}
```

#### 解説

`SESSION_ID` は HttpOnly にし、`XSRF-TOKEN` は JavaScript から読めるようにします。これが `CONCEPT.md` にあった cookie-to-header CSRF 対策です。

---

### 7-2. `backend/internal/auth/session_store.go`

#### ファイル: `backend/internal/auth/session_store.go`

```go
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrSessionNotFound = errors.New("session not found")

type SessionRecord struct {
	UserID    int64  `json:"userId"`
	CSRFToken string `json:"csrfToken"`
}

type SessionStore struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

func NewSessionStore(client *redis.Client, ttl time.Duration) *SessionStore {
	return &SessionStore{
		client: client,
		prefix: "session:",
		ttl:    ttl,
	}
}

func (s *SessionStore) Create(ctx context.Context, userID int64) (string, string, error) {
	sessionID, err := randomToken(32)
	if err != nil {
		return "", "", err
	}

	csrfToken, err := randomToken(32)
	if err != nil {
		return "", "", err
	}

	record := SessionRecord{
		UserID:    userID,
		CSRFToken: csrfToken,
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return "", "", err
	}

	if err := s.client.Set(ctx, s.key(sessionID), payload, s.ttl).Err(); err != nil {
		return "", "", fmt.Errorf("save session: %w", err)
	}

	return sessionID, csrfToken, nil
}

func (s *SessionStore) Get(ctx context.Context, sessionID string) (SessionRecord, error) {
	raw, err := s.client.Get(ctx, s.key(sessionID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return SessionRecord{}, ErrSessionNotFound
	}
	if err != nil {
		return SessionRecord{}, fmt.Errorf("get session: %w", err)
	}

	var record SessionRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return SessionRecord{}, fmt.Errorf("decode session: %w", err)
	}

	return record, nil
}

func (s *SessionStore) Delete(ctx context.Context, sessionID string) error {
	if err := s.client.Del(ctx, s.key(sessionID)).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *SessionStore) key(sessionID string) string {
	return s.prefix + sessionID
}

func randomToken(numBytes int) (string, error) {
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
```

#### 解説

このストアには、最小限の責務しか持たせません。

- セッション作成
- セッション取得
- セッション削除

認証ロジックはまだここに入れません。そこは `service` の責務です。

---

## Step 8. service 層を書く

### この Step を API より先にやる理由

`CONCEPT.md` の中で一番守るべきことの 1 つが、「operation に業務ロジックを書き込まない」です。その原則を実際のコードに落とすために、API を書く前に service を作ります。

service を先に作る理由は、HTTP がなくても成立する業務ルールを先に定義したいからです。login の認証失敗、current session の復元、logout 時の CSRF 照合は、本質的には HTTP の都合ではなく業務ルールです。

ここで service を先に作っておくと、Huma 側は「入力を受ける」「service を呼ぶ」「結果を返す」だけの薄い層にできます。これが、OpenAPI 契約と業務ロジックを混ぜないための基本形です。

### 8-1. `backend/internal/service/session_service.go`

#### ファイル: `backend/internal/service/session_service.go`

```go
package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"

	db "example.com/haohao/backend/internal/db"
	"example.com/haohao/backend/internal/auth"

	"github.com/jackc/pgx/v5"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrInvalidCSRFToken   = errors.New("invalid csrf token")
)

type User struct {
	ID          int64
	PublicID    string
	Email       string
	DisplayName string
}

type SessionService struct {
	queries *db.Queries
	store   *auth.SessionStore
}

func NewSessionService(queries *db.Queries, store *auth.SessionStore) *SessionService {
	return &SessionService{
		queries: queries,
		store:   store,
	}
}

func (s *SessionService) Login(ctx context.Context, email, password string) (User, string, string, error) {
	userID, err := s.queries.AuthenticateUser(ctx, db.AuthenticateUserParams{
		Email:    email,
		Password: password,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", "", ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", "", fmt.Errorf("authenticate user: %w", err)
	}

	user, err := s.loadUserByID(ctx, userID)
	if err != nil {
		return User{}, "", "", err
	}

	sessionID, csrfToken, err := s.store.Create(ctx, userID)
	if err != nil {
		return User{}, "", "", err
	}

	return user, sessionID, csrfToken, nil
}

func (s *SessionService) CurrentUser(ctx context.Context, sessionID string) (User, error) {
	session, err := s.store.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, err
	}

	return s.loadUserByID(ctx, session.UserID)
}

func (s *SessionService) loadUserByID(ctx context.Context, userID int64) (User, error) {
	record, err := s.queries.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, fmt.Errorf("load user by session: %w", err)
	}

	return User{
		ID:          record.ID,
		PublicID:    record.PublicID.String(),
		Email:       record.Email,
		DisplayName: record.DisplayName,
	}, nil
}

func (s *SessionService) Logout(ctx context.Context, sessionID, csrfHeader string) error {
	session, err := s.store.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return ErrUnauthorized
	}
	if err != nil {
		return err
	}

	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(csrfHeader)) != 1 {
		return ErrInvalidCSRFToken
	}

	return s.store.Delete(ctx, sessionID)
}
```

#### 重要な解説

ここが `CONCEPT.md` の「service は業務ルールを持つ」の実装です。

このファイルでは、HTTP のことを知りません。

- Cookie をどう返すかは知らない
- Huma の input/output は知らない
- SQL 文字列も書かない

知っているのは次だけです。

- login の業務ルール
- current session の復元
- logout 時の CSRF 照合

---

## Step 9. Huma の operation を書く

### この Step の意図

ここで初めて HTTP の形を定義します。理由は、service が先にできていると「何を受けて何を返すか」を落ち着いて決められるからです。

Huma を使う理由は、Go の input / output struct がそのまま OpenAPI 3.1 の正本になるからです。手書き YAML を別で持つと、実装と契約がずれます。Huma の operation を書くということは、単に handler を書くことではなく、**Go の型で公開契約を宣言すること**です。

この Step は、「HTTP 実装を書く Step」というより、「公開契約を Go の型として固定する Step」だと思って読んでください。

ここから、Go の型を OpenAPI 3.1 の正本として扱います。

### 9-1. `backend/internal/api/register.go`

#### ファイル: `backend/internal/api/register.go`

```go
package api

import (
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type Dependencies struct {
	SessionService *service.SessionService
	CookieSecure   bool
	SessionTTL     time.Duration
}

func Register(api huma.API, deps Dependencies) {
	registerSessionRoutes(api, deps)
}
```

#### 解説

`cmd/main` と `cmd/openapi` の両方がこの `Register()` を呼ぶようにします。これで「起動している API」と「export される OpenAPI」のずれを防ぎます。

---

### 9-2. `backend/internal/api/session.go`

#### ファイル: `backend/internal/api/session.go`

```go
package api

import (
	"context"
	"errors"
	"net/http"

	"example.com/haohao/backend/internal/auth"
	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type UserResponse struct {
	PublicID    string `json:"publicId" format:"uuid" example:"018f2f05-c6c9-7a49-b32d-04f4dd84ef4a"`
	Email       string `json:"email" format:"email" example:"demo@example.com"`
	DisplayName string `json:"displayName" example:"Demo User"`
}

type SessionBody struct {
	User UserResponse `json:"user"`
}

type GetSessionInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID" required:"true"`
}

type GetSessionOutput struct {
	Body SessionBody
}

type LoginInput struct {
	Body struct {
		Email    string `json:"email" format:"email" example:"demo@example.com"`
		Password string `json:"password" minLength:"8" example:"changeme123"`
	}
}

type LoginOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      SessionBody
}

type LogoutInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID" required:"true"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
}

type LogoutOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
}

func registerSessionRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "getSession",
		Method:      http.MethodGet,
		Path:        "/api/v1/session",
		Summary:     "現在のセッションを返す",
		Tags:        []string{"session"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *GetSessionInput) (*GetSessionOutput, error) {
		user, err := deps.SessionService.CurrentUser(ctx, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPError(err)
		}

		return &GetSessionOutput{
			Body: SessionBody{
				User: toUserResponse(user),
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/api/v1/login",
		Summary:     "ログインして Cookie セッションを払い出す",
		Tags:        []string{"session"},
	}, func(ctx context.Context, input *LoginInput) (*LoginOutput, error) {
		user, sessionID, csrfToken, err := deps.SessionService.Login(ctx, input.Body.Email, input.Body.Password)
		if err != nil {
			return nil, toHTTPError(err)
		}

		return &LoginOutput{
			SetCookie: []http.Cookie{
				auth.NewSessionCookie(sessionID, deps.CookieSecure, deps.SessionTTL),
				auth.NewXSRFCookie(csrfToken, deps.CookieSecure, deps.SessionTTL),
			},
			Body: SessionBody{
				User: toUserResponse(user),
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "logout",
		Method:        http.MethodPost,
		Path:          "/api/v1/logout",
		Summary:       "セッションを破棄する",
		Tags:          []string{"session"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *LogoutInput) (*LogoutOutput, error) {
		if err := deps.SessionService.Logout(ctx, input.SessionCookie.Value, input.CSRFToken); err != nil {
			return nil, toHTTPError(err)
		}

		return &LogoutOutput{
			SetCookie: []http.Cookie{
				auth.ExpiredSessionCookie(deps.CookieSecure),
				auth.ExpiredXSRFCookie(deps.CookieSecure),
			},
		}, nil
	})
}

func toUserResponse(user service.User) UserResponse {
	return UserResponse{
		PublicID:    user.PublicID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
	}
}

func toHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		return huma.Error401Unauthorized("invalid credentials")
	case errors.Is(err, service.ErrUnauthorized):
		return huma.Error401Unauthorized("missing or expired session")
	case errors.Is(err, service.ErrInvalidCSRFToken):
		return huma.Error403Forbidden("invalid csrf token")
	default:
		return huma.Error500InternalServerError("internal server error")
	}
}
```

#### 重要な解説

ここで大事なのは、**operation が薄いこと**です。

- request / response struct を定義する
- service を呼ぶ
- Cookie を返す
- service error を HTTP error に変換する

業務ロジックは `service` に置いたままです。

この形にしておくと、OpenAPI 3.1 が Go 型から自然に生成されます。

---

## Step 10. Huma を組み立てる

### この Step を分ける理由

`app.New()` のような配線コードを独立させる理由は、handler の実装とアプリケーションの起動順序を分けるためです。起動コードに operation 定義が混ざると、OpenAPI export 用コマンドとサーバー起動コマンドで登録漏れが起きやすくなります。

ここで `app` 層を置くことで、`cmd/main` と `cmd/openapi` の両方が同じ組み立て手順を使えます。これが `CONCEPT.md` で言っていた「実装と API 契約のドリフトを最小化する」の具体策です。

つまりこの Step は、**サーバーの配線を 1 か所に固定して、起動経路の違いによるずれを防ぐ Step**です。

### 10-1. `backend/internal/app/app.go`

#### ファイル: `backend/internal/app/app.go`

```go
package app

import (
	"example.com/haohao/backend/internal/auth"
	backendapi "example.com/haohao/backend/internal/api"
	"example.com/haohao/backend/internal/config"
	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
)

type App struct {
	Router *gin.Engine
	API    huma.API
}

func New(cfg config.Config, sessionService *service.SessionService) *App {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	humaConfig := huma.DefaultConfig(cfg.AppName, cfg.AppVersion)
	humaConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"cookieAuth": {
			Type: "apiKey",
			In:   "cookie",
			Name: auth.SessionCookieName,
		},
	}

	api := humagin.New(router, humaConfig)

	backendapi.Register(api, backendapi.Dependencies{
		SessionService: sessionService,
		CookieSecure:   cfg.CookieSecure,
		SessionTTL:     cfg.SessionTTL,
	})

	return &App{
		Router: router,
		API:    api,
	}
}
```

#### 解説

このファイルが「Gin と Huma をどう組み合わせるか」の答えです。

`CONCEPT.md` にあった cookie auth の OpenAPI 定義もここで入れています。

---

### 10-2. `backend/cmd/main/main.go`

#### ファイル: `backend/cmd/main/main.go`

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/haohao/backend/internal/app"
	"example.com/haohao/backend/internal/auth"
	"example.com/haohao/backend/internal/config"
	db "example.com/haohao/backend/internal/db"
	"example.com/haohao/backend/internal/platform"
	"example.com/haohao/backend/internal/service"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	pool, err := platform.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	redisClient, err := platform.NewRedisClient(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatal(err)
	}
	defer redisClient.Close()

	queries := db.New(pool)
	sessionStore := auth.NewSessionStore(redisClient, cfg.SessionTTL)
	sessionService := service.NewSessionService(queries, sessionStore)

	application := app.New(cfg, sessionService)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           application.Router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on http://127.0.0.1:%d", cfg.HTTPPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-shutdownCtx.Done()

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctxWithTimeout); err != nil {
		log.Fatal(err)
	}
}
```

#### 解説

このファイルは「配線」だけをします。

- 設定を読む
- Postgres / Redis に接続する
- service を作る
- app を作る
- HTTP server を起動する

---

### 10-3. `backend/cmd/openapi/main.go`

#### ファイル: `backend/cmd/openapi/main.go`

```go
package main

import (
	"fmt"
	"log"

	"example.com/haohao/backend/internal/app"
	"example.com/haohao/backend/internal/config"
	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	application := app.New(cfg, nil)

	spec, err := application.API.OpenAPI().YAML()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(string(spec))
}
```

#### 解説

ここで `sessionService` に `nil` を渡しているのは意図的です。OpenAPI export では handler は実行されず、operation 登録だけ必要だからです。

`gin.SetMode(gin.ReleaseMode)` を入れているのは、標準出力へ debug log が混ざると `openapi/openapi.yaml` が壊れるためです。artifact として保存する経路では、YAML 以外を stdout に出さないことが重要です。

---

## Step 11. バックエンド単体で動作確認する

### この Step を frontend より先にやる理由

ここで先に backend 単体を確認する理由は、問題の切り分けを簡単にするためです。frontend をまだつながない状態で login / session / logout が通るなら、以後の不具合はブラウザ側に絞れます。

逆にここを飛ばして先に Vue をつなぐと、失敗時に「Vite proxy が悪いのか」「Cookie の付与が悪いのか」「backend の認証が悪いのか」が一気に分からなくなります。

この Step は、**サーバー側の責務だけを先に閉じる Step**です。E2E を急がず、まず backend 単体の正しさを確認します。

### 11-1. サーバー起動

#### 実行コマンド

```bash
make backend-dev
```

### 11-2. OpenAPI と docs の確認

別ターミナルで次を叩いてください。

```bash
curl -i http://127.0.0.1:8080/openapi.yaml
curl -i http://127.0.0.1:8080/openapi.json
```

ブラウザで見るなら次です。

- `http://127.0.0.1:8080/docs`

### 11-3. login / session / logout の確認

この確認に入る前に、Step 4 で `make seed-demo-user` まで流しておいてください。

#### login

```bash
curl -i \
  -X POST http://127.0.0.1:8080/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"changeme123"}' \
  -c cookies.txt
```

#### session

```bash
curl -i \
  http://127.0.0.1:8080/api/v1/session \
  -b cookies.txt
```

#### logout

`curl -c` が出力する `cookies.txt` の 7 列目が cookie 値です。`XSRF-TOKEN` を取り出してヘッダーに載せます。

```bash
XSRF_TOKEN=$(awk '$6 == "XSRF-TOKEN" { print $7 }' cookies.txt)

curl -i \
  -X POST http://127.0.0.1:8080/api/v1/logout \
  -H "X-CSRF-Token: ${XSRF_TOKEN}" \
  -b cookies.txt \
  -c cookies.txt
```

#### 解説

ここが最初の大きな関門です。

この 3 つが通れば、次のことが確認できています。

- DB のユーザー認証
- Redis のセッション保存
- HttpOnly Cookie の払い出し
- XSRF-TOKEN Cookie の払い出し
- CSRF ヘッダーの照合
- Huma の OpenAPI 生成

---

## Step 12. OpenAPI artifact を出力する

### この Step を独立させる理由

OpenAPI をサーバーの副産物として眺めるだけでは不十分です。`CONCEPT.md` の方針では、`openapi/openapi.yaml` はレビュー対象であり、frontend 生成物の入力でもあります。

そのため、この Step では「サーバーが動いている」こととは別に、「artifact として正しく保存できる」ことを確認します。これは deploy とは別の責務です。

この Step を独立させることで、API 実装者も frontend 実装者も、同じ契約ファイルを基準に作業できます。

### 実行コマンド

```bash
make openapi
sed -n '1,220p' openapi/openapi.yaml
```

### ここで確認すること

- `paths` に `/api/v1/login`, `/api/v1/session`, `/api/v1/logout` がある
- `components.securitySchemes.cookieAuth` がある
- request / response schema が Go 側の型と一致している

### 解説

この `openapi/openapi.yaml` は frontend と公開契約の中間地点です。ここで差分レビューできる形にしておくのが `CONCEPT.md` の重要ポイントでした。

---

## Step 13. frontend を Vite で作る

### この Step をこのタイミングでやる理由

frontend をここで始めるのは、backend の契約が `openapi/openapi.yaml` として固まった後だからです。先に画面から作ると、API 名や payload 形状を仮で決めてしまい、あとで generated SDK と食い違います。

また、Vite の scaffold を使う理由もあります。ここで大事なのは UI の見た目ではなく、「Vue + TypeScript + Vite の最低限の土台を短時間で正しく作る」ことです。土台を自作すると、チュートリアルの主題が framework の初期設定にずれてしまいます。

この Step は、**frontend を API 契約の利用者として正しく立ち上げる Step**です。

### 13-1. Vite の初期化

#### 実行コマンド

```bash
npm create vite@latest frontend -- --template vue-ts
cd frontend
npm install pinia vue-router
npm install -D @hey-api/openapi-ts
cd ..
```

### 解説

ここでは `package.json` を手書きしません。Vite の初期 scaffold を使い、その上に必要な依存だけ足します。

こうすると TypeScript / Vue / Vite の最低限の配線を自分で書かずに済みます。

`CONCEPT.md` では frontend を `shared/`, `features/`, `pages/` のハイブリッドで切る方針でしたが、最初の 1 周目は Vite scaffold に寄せて `views/`, `stores/`, `api/` で始めます。login 導線が一周したあとで、必要に応じて feature-based な構成へ寄せていくほうが移行理由を説明しやすいからです。

---

### 13-2. `frontend/openapi-ts.config.ts`

#### ファイル: `frontend/openapi-ts.config.ts`

```ts
import { defineConfig } from '@hey-api/openapi-ts';

export default defineConfig({
  input: '../openapi/openapi.yaml',
  output: 'src/api/generated',
});
```

#### 解説

公式 README のとおり、`@hey-api/openapi-ts` はデフォルトで TypeScript interfaces と SDK を生成します。ここではまず最小構成で始めます。

---

### 13-3. `frontend/package.json` に script を足す

#### 何をするか

`scripts` に `openapi-ts` を足します。

#### 編集例

`frontend/package.json` の `scripts` を次の形にしてください。

```json
{
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc --noEmit && vite build",
    "preview": "vite preview",
    "openapi-ts": "openapi-ts"
  }
}
```

#### 解説

これで `scripts/gen.sh` から `npm run openapi-ts` を呼べるようになります。

---

### 13-4. `frontend/vite.config.ts`

#### ファイル: `frontend/vite.config.ts`

```ts
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/openapi': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/docs': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: '../backend/web/dist',
    emptyOutDir: true,
  },
});
```

#### 解説

この設定で、開発時は Vite からバックエンドへ `/api` を proxy し、本番 build 時は `backend/web/dist` に出力します。これは `CONCEPT.md` の「開発時は分離、本番時は Go に埋め込む」をそのまま再現しています。

---

## Step 14. generated SDK を作る

### この Step を wrapper より前にやる理由

wrapper は generated SDK の形に合わせて書くべきです。逆ではありません。先に wrapper を想像で書くと、生成される関数名やオプション形状とずれて、すぐに手修正が必要になります。

そのため、この Step ではまず `sdk.gen.ts`, `client.gen.ts`, `types.gen.ts` を見て、「実際に何が生成されたか」を把握します。これにより、その後の wrapper や store が生成物に自然に沿った形になります。

この Step は、**frontend 側の契約利用が推測ではなく実物に基づいていることを確認する Step**です。

### 実行コマンド

```bash
make gen
find frontend/src/api/generated -maxdepth 2 -type f | sort
```

### ここで確認すること

少なくとも次のようなファイルが見えるはずです。

- `client.gen.ts`
- `sdk.gen.ts`
- `types.gen.ts`

今回の確認では、`operationId` がそのまま SDK 名になっていることも見てください。

```bash
sed -n '1,220p' frontend/src/api/generated/sdk.gen.ts
```

`getSession`, `login`, `logout` が export されていれば成功です。

---

## Step 15. frontend の API wrapper を書く

### この Step を必ず挟む理由

generated SDK を画面から直接呼ばないのは、認証や CSRF の共通処理を UI から追い出すためです。`CONCEPT.md` で transport wrapper を強調していた理由はここにあります。

もし画面ごとに `credentials: "include"` を書いたり、毎回 `XSRF-TOKEN` を Cookie から読んだりすると、数が増えるほど揺れます。1 画面だけヘッダー付与を忘れた、といった事故も起きやすくなります。

この Step は、**generated SDK をそのまま使うのではなく、アプリケーションの接続ルールを 1 か所に閉じ込める Step**です。

ここから Vue を組み立てます。

### 15-1. `frontend/src/api/client.ts`

#### ファイル: `frontend/src/api/client.ts`

```ts
import { client } from './generated/client.gen';

function readCookie(name: string): string | undefined {
  const prefix = `${name}=`;
  return document.cookie
    .split(';')
    .map((part) => part.trim())
    .find((part) => part.startsWith(prefix))
    ?.slice(prefix.length);
}

client.setConfig({
  baseUrl: '',
  credentials: 'include',
  responseStyle: 'data',
  throwOnError: true,
  fetch: async (input, init) => {
    const headers = new Headers(init?.headers ?? {});
    headers.set('Accept', 'application/json');

    const method = (init?.method ?? 'GET').toUpperCase();
    if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
      const token = readCookie('XSRF-TOKEN');
      if (token) {
        headers.set('X-CSRF-Token', token);
      }
    }

    return fetch(input, {
      ...init,
      credentials: 'include',
      headers,
    });
  },
});
```

#### 解説

これが `CONCEPT.md` にあった transport wrapper です。

ここで 1 か所に寄せているのは次です。

- `credentials: "include"`
- `XSRF-TOKEN` の読み取り
- `X-CSRF-Token` の自動付与

Vue の各画面は、この先ここを意識しません。

---

### 15-2. `frontend/src/api/session.ts`

#### ファイル: `frontend/src/api/session.ts`

```ts
import { readCookie } from './client';
import { getSession, login, logout } from './generated/sdk.gen';

export async function fetchCurrentSession() {
  return getSession({});
}

export async function loginWithPassword(email: string, password: string) {
  return login({
    body: {
      email,
      password,
    },
  });
}

export async function logoutCurrentSession() {
  return logout({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
  });
}
```

#### 解説

画面から generated SDK を直接呼ばず、一段薄い adapter を挟みます。

`@hey-api/openapi-ts` の生成結果は version によって細部が変わることがあります。今回の構成では `getSession({})` のように空 options が必要で、`logout` は header を明示で渡す形でした。Step 14 で実際の `sdk.gen.ts` を見てから wrapper を合わせる、という順番の意味はここにあります。

このファイルの役割は 2 つです。

- generated SDK の import 先を 1 か所にする
- UI から見た API 名に揃える

---

## Step 16. Pinia store と router を書く

### この Step を view の前にやる理由

画面を先に作ると、画面コンポーネントが API 呼び出し、認証状態、遷移条件を全部持ち始めます。そうすると、後から共通化したくなって大きく崩れます。

先に store と router を作る理由は、画面の責務を「表示と入力」に限定するためです。Pinia store にはセッション状態と API 呼び出しを集約し、router には画面遷移条件を集約します。

この Step を挟むことで、view は薄く保てます。つまりこの Step は、**UI を薄く保つための土台作り**です。

### 16-1. `frontend/src/stores/session.ts`

#### ファイル: `frontend/src/stores/session.ts`

```ts
import { defineStore } from 'pinia';

import {
  fetchCurrentSession,
  loginWithPassword,
  logoutCurrentSession,
} from '../api/session';

type User = {
  publicId: string;
  email: string;
  displayName: string;
};

type AuthStatus = 'idle' | 'loading' | 'authenticated' | 'anonymous';

export const useSessionStore = defineStore('session', {
  state: () => ({
    status: 'idle' as AuthStatus,
    user: null as User | null,
    errorMessage: '',
  }),

  actions: {
    async bootstrap() {
      if (this.status !== 'idle') {
        return;
      }

      this.status = 'loading';
      this.errorMessage = '';

      try {
        const data = await fetchCurrentSession();
        this.user = data.user;
        this.status = 'authenticated';
      } catch {
        this.user = null;
        this.status = 'anonymous';
      }
    },

    async login(email: string, password: string) {
      this.status = 'loading';
      this.errorMessage = '';

      try {
        const data = await loginWithPassword(email, password);
        this.user = data.user;
        this.status = 'authenticated';
      } catch (error) {
        this.user = null;
        this.status = 'anonymous';
        this.errorMessage = error instanceof Error ? error.message : 'ログインに失敗しました';
        throw error;
      }
    },

    async logout() {
      try {
        await logoutCurrentSession();
      } finally {
        this.user = null;
        this.status = 'anonymous';
        this.errorMessage = '';
      }
    },
  },
});
```

#### 解説

ここで状態を 3 つに分けているのがポイントです。

- `idle`
- `loading`
- `authenticated` / `anonymous`

`idle` を持たないと、router guard で初回 bootstrap したかどうかが分かりにくくなります。

---

### 16-2. `frontend/src/router/index.ts`

#### ファイル: `frontend/src/router/index.ts`

```ts
import { createRouter, createWebHistory } from 'vue-router';

import HomeView from '../views/HomeView.vue';
import LoginView from '../views/LoginView.vue';
import { useSessionStore } from '../stores/session';

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
      meta: { requiresAuth: true },
    },
    {
      path: '/login',
      name: 'login',
      component: LoginView,
    },
  ],
});

router.beforeEach(async (to) => {
  const sessionStore = useSessionStore();
  await sessionStore.bootstrap();

  if (to.meta.requiresAuth && sessionStore.status !== 'authenticated') {
    return { name: 'login' };
  }

  if (to.name === 'login' && sessionStore.status === 'authenticated') {
    return { name: 'home' };
  }

  return true;
});

export default router;
```

#### 解説

認可判定は router だけで閉じませんが、初回画面表示の導線は router が一番分かりやすいのでここで bootstrap します。

---

## Step 17. Vue の入口と画面を書く

### この Step を最後に近づける理由

`main.ts`, `App.vue`, 各 View は、最終的にはすべてそれまでに作った土台の利用者です。だからこそ最後に書くのが自然です。

ここまでで router, store, API wrapper, generated SDK が揃っているので、View では「どの action を呼ぶか」「どの state を表示するか」に集中できます。これが、画面コンポーネントを太らせないための順番です。

つまりこの Step は、**これまでに作った backend と frontend の部品を、実際の画面として結線する Step**です。

### 17-1. `frontend/src/main.ts`

#### ファイル: `frontend/src/main.ts`

```ts
import { createApp } from 'vue';
import { createPinia } from 'pinia';

import App from './App.vue';
import router from './router';

const app = createApp(App);

app.use(createPinia());
app.use(router);
app.mount('#app');
```

---

### 17-2. `frontend/src/App.vue`

#### ファイル: `frontend/src/App.vue`

```vue
<script setup lang="ts">
import { computed } from 'vue';
import { useSessionStore } from './stores/session';

const sessionStore = useSessionStore();
const displayName = computed(() => sessionStore.user?.displayName ?? 'Guest');
</script>

<template>
  <div class="app-shell">
    <header class="app-header">
      <h1>HaoHao</h1>
      <p>{{ displayName }}</p>
    </header>

    <main class="app-main">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  max-width: 960px;
  margin: 0 auto;
  padding: 32px 16px;
  font-family: ui-sans-serif, system-ui, sans-serif;
}

.app-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 32px;
}
</style>
```

---

### 17-3. `frontend/src/views/LoginView.vue`

#### ファイル: `frontend/src/views/LoginView.vue`

```vue
<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';

import { useSessionStore } from '../stores/session';

const router = useRouter();
const sessionStore = useSessionStore();

const email = ref('demo@example.com');
const password = ref('changeme123');
const submitting = ref(false);

async function submit() {
  submitting.value = true;

  try {
    await sessionStore.login(email.value, password.value);
    await router.push({ name: 'home' });
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <section class="panel">
    <h2>Login</h2>
    <p>最初の動作確認用ユーザーは `demo@example.com / changeme123` です。</p>

    <form class="form" @submit.prevent="submit">
      <label>
        Email
        <input v-model="email" type="email" required />
      </label>

      <label>
        Password
        <input v-model="password" type="password" required minlength="8" />
      </label>

      <button :disabled="submitting" type="submit">
        {{ submitting ? 'Signing in...' : 'Sign in' }}
      </button>
    </form>

    <p v-if="sessionStore.errorMessage" class="error">
      {{ sessionStore.errorMessage }}
    </p>
  </section>
</template>

<style scoped>
.panel {
  border: 1px solid #d0d7de;
  border-radius: 12px;
  padding: 24px;
}

.form {
  display: grid;
  gap: 16px;
  margin-top: 20px;
}

label {
  display: grid;
  gap: 8px;
}

input {
  padding: 10px 12px;
}

.error {
  color: #b42318;
  margin-top: 16px;
}
</style>
```

---

### 17-4. `frontend/src/views/HomeView.vue`

#### ファイル: `frontend/src/views/HomeView.vue`

```vue
<script setup lang="ts">
import { useRouter } from 'vue-router';

import { useSessionStore } from '../stores/session';

const router = useRouter();
const sessionStore = useSessionStore();

async function signOut() {
  await sessionStore.logout();
  await router.push({ name: 'login' });
}
</script>

<template>
  <section class="panel">
    <h2>Current Session</h2>

    <pre>{{ sessionStore.user }}</pre>

    <button type="button" @click="signOut">
      Logout
    </button>
  </section>
</template>

<style scoped>
.panel {
  border: 1px solid #d0d7de;
  border-radius: 12px;
  padding: 24px;
}

pre {
  background: #f6f8fa;
  padding: 16px;
  border-radius: 8px;
  overflow: auto;
  margin: 16px 0 24px;
}
</style>
```

---

## Step 18. フロントエンドを起動する

### この Step の意味

ここで確認しているのは、単に画面が開くことではありません。ブラウザ環境でしか起きない問題をまとめて確認しています。

たとえば次のようなものです。

- Vite proxy が正しく backend に流れているか
- Cookie がブラウザに保存されるか
- XSRF-TOKEN を JavaScript から読めるか
- logout 時に header が付いているか

つまりこの Step は、backend 単体テストでは拾えない「ブラウザ特有の前提」が成立しているかを確認する Step です。

### 実行コマンド

バックエンドを起動したまま、別ターミナルで次を実行します。

```bash
make frontend-dev
```

### 確認

ブラウザで次を開いてください。

- `http://127.0.0.1:5173/login`

次の順番で確認します。

1. 画面が表示される
2. 既定値の `demo@example.com / changeme123` で login できる
3. home 画面へ遷移する
4. user 情報が見える
5. logout で login 画面へ戻る

### 解説

ここまで通れば、フロントから見た end-to-end が完成です。

- Vite proxy
- generated SDK
- Cookie 認証
- CSRF header 自動付与
- Pinia state 管理

---

## Step 19. `make gen` が回ることを確認する

### この Step を最後にもう一度やる理由

この Step は非常に重要です。手で一度全部作れたとしても、それを再生成手順で再現できなければ、この構成は運用できません。

`CONCEPT.md` の本質は「生成物を前提とした開発フロー」です。したがって、最後に `make gen` を叩いて差分がどう出るかを見ることは、単なる確認ではなく運用ルールの検証です。

この Step をやることで、「以後の変更も同じコマンドで再生成できる」ことが保証されます。

### 実行コマンド

```bash
make gen
```

### ここで確認するもの

```bash
git status --short
```

差分として出てよいのは、たとえば次です。

- `openapi/openapi.yaml`
- `frontend/src/api/generated/*`
- `backend/internal/db/*`

この 3 種類の差分が毎回再生成され、レビュー可能であることが `CONCEPT.md` の中心です。

---

## Step 20. このあと機能を増やすときの順番

### なぜこの順番を守るのか

この順番は見た目の好みではありません。責務の流れに沿っているからです。

先に migration を書くのは、保存構造が先だからです。次に `db/queries` と `sqlc` をやるのは、DB に何を頼めるかを確定するためです。その後に service と API を作るのは、業務ルールと公開契約をその土台の上に乗せるためです。最後に frontend を直すのは、frontend が契約の利用者だからです。

この順番を崩すと、次のような drift が起きます。

- frontend だけ先に payload を仮定する
- service が仮の repository を持つ
- OpenAPI が後追いになって artifact の差分が意味を失う

つまりこの順番は、**変更時にずれを起こさないための最小ルール**です。

このリポジトリでは、次の順番を毎回守ってください。

1. `db/migrations/*.sql` を増やす
2. DB に migration を流す
3. `make db-schema` で `db/schema.sql` を再生成する
4. `db/queries/*.sql` を増やす
5. `make sqlc` で Go の DB コードを更新する
6. `backend/internal/service/*` を増やす
7. `backend/internal/api/*` の Huma operation を増やす
8. `make gen` で `openapi/openapi.yaml` と frontend SDK をまとめて更新する
9. `frontend/src/stores/*` と `frontend/src/views/*` をつなぐ

この順番にすると、仕様と実装のドリフトが起きにくくなります。

---

## 発展 1. frontend を Go バイナリに埋め込む

最初の 1 周目ではここまでで十分です。ただし、`CONCEPT.md` の「単一バイナリ配信」まで進めたいなら次を追加します。

### `backend/embed.go`

```go
package backend

import "embed"

//go:embed web/dist
//go:embed web/dist/*
var Frontend embed.FS
```

### 何を追加でやるか

- `frontend` の build 結果を `backend/web/dist/` に出す
- `cmd/main` で `/api/*` 以外の静的配信を追加する
- SPA fallback を `index.html` に向ける

この部分は API と認証が動いてから手を付けたほうが楽です。

### 実際に build して確認するには

まず frontend の production bundle を作り、その出力を含んだ状態で Go バイナリを build します。

#### 実行コマンド

```bash
cd frontend
npm run build
cd ..

mkdir -p bin
go build -o ./bin/haohao ./backend/cmd/main
```

#### 実行コマンド

DB と Redis が起動している前提で、`.env` を読み込んでバイナリを起動します。`8080` が埋まっている環境では、一時的に別 port を使って構いません。

```bash
make up

set -a && source .env && set +a
HTTP_PORT=18080 ./bin/haohao
```

#### 確認

別ターミナルで、次を確認します。

```bash
curl -i http://127.0.0.1:18080/
curl -i http://127.0.0.1:18080/login
curl -i \
  -X POST http://127.0.0.1:18080/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"changeme123"}'
```

ここで見たいのは次です。

- `/` が `index.html` を返す
- `/login` も SPA fallback として `index.html` を返す
- `/api/v1/login` は API として JSON を返し、`Set-Cookie` が付く

つまり、「1 本の Go バイナリが SPA と API を両方返せているか」を確認します。

---

## 発展 2. Dockerfile を作る

本番用イメージを作るなら、多段階ビルドで十分です。

### `docker/Dockerfile`

```dockerfile
FROM node:22 AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend ./
RUN npm run build

FROM golang:1.26 AS backend-builder
WORKDIR /app
ENV CGO_ENABLED=0 GOFLAGS="-p=1 -tags=nomsgpack"
COPY go.work .
COPY backend ./backend
COPY db ./db
COPY openapi ./openapi
COPY scripts ./scripts
COPY .env.example ./.env.example
COPY --from=frontend-builder /app/frontend/../backend/web/dist ./backend/web/dist
WORKDIR /app/backend
RUN go build -o /tmp/haohao ./cmd/main

FROM debian:bookworm-slim
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*
COPY --from=backend-builder /tmp/haohao /usr/local/bin/haohao
EXPOSE 8080
CMD ["haohao"]
```

### 実際に image を build するには

```bash
docker build -t haohao:dev -f docker/Dockerfile .
```

### どうやってコンテナを起動するか

backend コンテナ単体では PostgreSQL / Redis に接続できないので、先に `compose.yaml` の依存サービスを起動します。

```bash
make up
```

その上で、backend コンテナを compose の network に参加させて起動します。既定の compose project 名なら network は `haohao_default` です。別名になっている場合は `docker network ls` で確認してください。

```bash
docker run --rm \
  --name haohao-app \
  --network haohao_default \
  -p 18081:8080 \
  -e APP_NAME='HaoHao API' \
  -e APP_VERSION='0.1.0' \
  -e HTTP_PORT='8080' \
  -e DATABASE_URL='postgres://haohao:haohao@postgres:5432/haohao?sslmode=disable' \
  -e REDIS_ADDR='redis:6379' \
  -e REDIS_PASSWORD='' \
  -e REDIS_DB='0' \
  -e SESSION_TTL='24h' \
  -e COOKIE_SECURE='false' \
  haohao:dev
```

### 確認

別ターミナルで次を叩きます。

```bash
curl -i http://127.0.0.1:18081/
curl -i \
  -X POST http://127.0.0.1:18081/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"changeme123"}'
```

ここで見たいのは次です。

- `/` が `index.html` を返す
- `POST /api/v1/login` が JSON と `Set-Cookie` を返す
- つまり Docker image 化した backend でも、SPA 配信と API の両方が成立する

もし container 名や network 名が分からなくなったら、次で確認できます。

```bash
docker network ls
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Ports}}'
```

---

## 発展 3. CI で最低限チェックするもの

CI では、少なくとも次を固定してください。

- migration が通る
- `db/schema.sql` 更新漏れがない
- `sqlc generate` が通る
- `go test ./...` が通る
- `make openapi` の差分がない
- `make gen` の差分がない
- `frontend` の `npm run build` が通る

この構成では、**生成漏れを CI で止めること**が一番大事です。

`CONCEPT.md` では frontend の lint / format check に `Biome` を想定していますが、このチュートリアルでは login 導線の完成を優先して設定手順を省略しています。導入する場合は、この CI 一覧へそのコマンドを追加してください。

---

## 発展 4. セッション導線の自動テストを入れる

foundation が一通り動いたら、次は壊れやすい導線を 1 本だけでも自動テストにしてください。

このチュートリアルでは、`backend/session_flow_test.go` で次の流れをまとめて検証します。

- login
- todo 作成
- 現在の session 取得
- todo 一覧取得
- todo 削除
- logout
- logout 後に `401 Unauthorized` を返すこと

このテストは mock を使わず、`httptest.NewServer` と実 Postgres / 実 Redis を使います。`go test ./backend/...` を repo root から実行したときに `DATABASE_URL` が未設定なら `../.env` を読み込み、Redis は通常利用と衝突しにくいように `DB 15` を使います。

### `backend/session_flow_test.go`

最低限の骨格は次です。

```go
func TestSessionFlow(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := platform.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	redisClient, err := platform.NewRedisClient(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		t.Fatalf("connect redis: %v", err)
	}
	defer redisClient.Close()

	queries := db.New(pool)
	sessionStore := auth.NewSessionStore(redisClient, cfg.SessionTTL)
	sessionService := service.NewSessionService(queries, sessionStore, cfg.AuthMode)
	todoService := service.NewTodoService(queries)

	application := app.New(cfg, sessionService, todoService)
	server := httptest.NewServer(application.Router)
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}

	email := fmt.Sprintf("session-flow-%d@example.com", time.Now().UnixNano())
	password := "changeme123"

	loginBody, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})
	loginResponse, err := client.Post(server.URL+"/api/v1/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("login request: %v", err)
	}

	createTodoBody, _ := json.Marshal(map[string]string{
		"title": "Write integration coverage",
	})
	createTodoResponse, err := client.Post(server.URL+"/api/v1/todos", "application/json", bytes.NewReader(createTodoBody))
	if err != nil {
		t.Fatalf("create todo request: %v", err)
	}

	var createdTodo struct {
		PublicID string `json:"publicId"`
	}
	_ = json.NewDecoder(createTodoResponse.Body).Decode(&createdTodo)

	cookieURL, _ := url.Parse(server.URL)
	var csrfToken string
	for _, cookie := range jar.Cookies(cookieURL) {
		if cookie.Name == auth.XSRFCookieName {
			csrfToken = cookie.Value
			break
		}
	}

	currentResponse, err := client.Get(server.URL + "/api/v1/session")
	if err != nil {
		t.Fatalf("current session request: %v", err)
	}

	todoListResponse, err := client.Get(server.URL + "/api/v1/todos")
	if err != nil {
		t.Fatalf("list todos request: %v", err)
	}

	deleteTodoRequest, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/todos/"+createdTodo.PublicID, nil)
	deleteTodoRequest.Header.Set("X-CSRF-Token", csrfToken)
	deleteTodoResponse, err := client.Do(deleteTodoRequest)
	if err != nil {
		t.Fatalf("delete todo request: %v", err)
	}

	logoutRequest, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/logout", nil)
	logoutRequest.Header.Set("X-CSRF-Token", csrfToken)
	logoutResponse, err := client.Do(logoutRequest)
	if err != nil {
		t.Fatalf("logout request: %v", err)
	}

	expiredResponse, err := client.Get(server.URL + "/api/v1/session")
	if err != nil {
		t.Fatalf("expired session request: %v", err)
	}

	_ = loginResponse
	_ = createTodoResponse
	_ = currentResponse
	_ = todoListResponse
	_ = deleteTodoResponse
	_ = logoutResponse
	_ = expiredResponse
}
```

### 実際にどう実行するか

まず依存 service を起動し、migration を最新まで当てます。

```bash
make up
make db-up
```

その上で、session 導線の統合テストだけを明示的に流すなら次です。

```bash
go test ./backend/... -run TestSessionFlow -count=1
```

backend パッケージ全体をまとめて流すなら次でも構いません。

```bash
go test ./backend/...
```

### 何が通ればよいか

成功時は `ok  	example.com/haohao/backend` のような出力になります。

このテストが通っていれば、少なくとも「Cookie を受け取る」「CSRF を付ける」「session を Redis で引く」「認証済み API を叩く」という最小導線は自動で守られます。

`POST /api/v1/login` だけの単発テストより、このように 1 本の縦切りを最後まで通す方が後戻りを減らせます。

---

## 発展 5. 次の縦切り機能として TODO を追加する

session の foundation ができたら、次は小さな業務機能を 1 本追加します。ここでは TODO を例にします。

追加の流れは次です。

1. migration を足す
2. `db/queries/` に SQL を書く
3. `make db-schema` と `make gen` を回す
4. service を足す
5. API endpoint を足す
6. frontend store / view まで繋ぐ

今回の構成では、主に次のファイルが増えます。

- `db/migrations/0002_todos.up.sql`
- `db/migrations/0002_todos.down.sql`
- `db/queries/todos.sql`
- `backend/internal/service/todo_service.go`
- `backend/internal/api/todos.go`
- `frontend/src/api/todos.ts`
- `frontend/src/stores/todos.ts`
- `frontend/src/views/HomeView.vue`

`db/queries/todos.sql` では、少なくとも次の 3 操作があれば十分です。

- `ListTodosByUserID`
- `CreateTodo`
- `DeleteTodoByPublicID`

API は次の 3 endpoint で構いません。

- `GET /api/v1/todos`
- `POST /api/v1/todos`
- `DELETE /api/v1/todos/{todoPublicId}`

### `db/migrations/0002_todos.up.sql`

```sql
CREATE TABLE todos (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    user_id BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX todos_user_id_created_at_idx
    ON todos (user_id, created_at DESC, id DESC);
```

### `db/queries/todos.sql`

```sql
-- name: ListTodosByUserID :many
SELECT
    id,
    public_id,
    title,
    completed,
    created_at,
    updated_at
FROM todos
WHERE user_id = $1
ORDER BY created_at DESC, id DESC;

-- name: CreateTodo :one
INSERT INTO todos (
    user_id,
    title
) VALUES (
    $1,
    $2
)
RETURNING
    id,
    public_id,
    title,
    completed,
    created_at,
    updated_at;

-- name: DeleteTodoByPublicID :execrows
DELETE FROM todos
WHERE public_id = $1
  AND user_id = $2;
```

### `backend/internal/api/todos.go`

```go
func registerTodoRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listTodos",
		Method:      http.MethodGet,
		Path:        "/api/v1/todos",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListTodosInput) (*ListTodosOutput, error) {
		user, err := deps.SessionService.CurrentUser(ctx, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPError(err)
		}

		items, err := deps.TodoService.ListByUser(ctx, user.ID)
		if err != nil {
			return nil, toHTTPError(err)
		}

		responseItems := make([]TodoResponse, 0, len(items))
		for _, item := range items {
			responseItems = append(responseItems, toTodoResponse(item))
		}

		return &ListTodosOutput{
			Body: TodoListBody{Items: responseItems},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "createTodo",
		Method:        http.MethodPost,
		Path:          "/api/v1/todos",
		DefaultStatus: http.StatusCreated,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *CreateTodoInput) (*CreateTodoOutput, error) {
		user, err := deps.SessionService.CurrentUser(ctx, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPError(err)
		}

		item, err := deps.TodoService.Create(ctx, user.ID, input.Body.Title)
		if err != nil {
			return nil, toHTTPError(err)
		}

		return &CreateTodoOutput{Body: toTodoResponse(item)}, nil
	})
}
```

### `frontend/src/api/todos.ts`

```ts
import './client';
import { createTodo, deleteTodo, listTodos } from './generated/sdk.gen';

export async function fetchTodos() {
  return listTodos({});
}

export async function createTodoItem(title: string) {
  return createTodo({
    body: {
      title,
    },
  });
}

export async function deleteTodoItem(todoPublicId: string) {
  return deleteTodo({
    path: {
      todoPublicId,
    },
  });
}
```

### 実装後に流すコマンド

```bash
make db-up
make db-schema
make gen
go test ./backend/...
```

frontend も確認するなら、別 terminal で次を実行します。

```bash
cd frontend
npm run build
```

開発中に画面で確認するなら、backend と frontend dev server をそれぞれ起動します。

terminal 1:

```bash
make backend-dev
```

terminal 2:

```bash
cd frontend
npm run dev
```

### どう確認するか

まず demo user を入れていない場合は seed します。

```bash
make seed-demo-user
```

その後、browser で `http://localhost:5173/login` を開き、`demo@example.com / changeme123` で login します。

確認ポイントは次です。

- login 後に Home 画面へ遷移する
- 現在の session 情報が表示される
- TODO を追加すると一覧の先頭に増える
- TODO を削除すると一覧から消える

API を `curl` で確認したい場合は、次のように cookie jar と CSRF token を使います。

```bash
BASE_URL=http://localhost:8080

curl -sS -c /tmp/haohao.cookies \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"changeme123"}' \
  "$BASE_URL/api/v1/login"

XSRF_TOKEN=$(awk '$6=="XSRF-TOKEN"{print $7}' /tmp/haohao.cookies)

curl -sS -b /tmp/haohao.cookies \
  "$BASE_URL/api/v1/todos"

curl -sS -b /tmp/haohao.cookies \
  -H "X-CSRF-Token: $XSRF_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Review generated OpenAPI diff"}' \
  "$BASE_URL/api/v1/todos"
```

`POST /api/v1/todos` の応答には `publicId` が入るので、削除確認はそれを使います。

```bash
curl -i -b /tmp/haohao.cookies \
  -H "X-CSRF-Token: $XSRF_TOKEN" \
  -X DELETE \
  "$BASE_URL/api/v1/todos/<todoPublicId>"
```

この 1 本が通ると、migration -> sqlc -> service -> Huma -> OpenAPI -> generated SDK -> frontend store までの経路をもう一度実地で確認できます。

---

## 発展 6. Zitadel へ寄せる前の足場を入れる

`CONCEPT.md` の最終形は、local password login を恒久運用することではありません。ただし、いきなり browser redirect / callback / token exchange まで入れると切り分けが難しくなるので、その前に「切替可能な足場」を先に入れておきます。

まず `.env.example` に次を足します。

```dotenv
AUTH_MODE=local
ZITADEL_ISSUER=
ZITADEL_CLIENT_ID=
```

backend 側では `backend/internal/config/config.go` にこの設定を読み込ませ、`backend/internal/api/auth_settings.go` で公開用 endpoint を追加します。

```text
GET /api/v1/auth/settings
```

### `backend/internal/config/config.go`

```go
type Config struct {
	AppName         string
	AppVersion      string
	HTTPPort        int
	DatabaseURL     string
	AuthMode        string
	ZitadelIssuer   string
	ZitadelClientID string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	SessionTTL      time.Duration
	CookieSecure    bool
}

func Load() (Config, error) {
	sessionTTL, err := time.ParseDuration(getEnv("SESSION_TTL", "24h"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppName:         getEnv("APP_NAME", "HaoHao API"),
		AppVersion:      getEnv("APP_VERSION", "0.1.0"),
		HTTPPort:        getEnvInt("HTTP_PORT", 8080),
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		AuthMode:        getEnv("AUTH_MODE", "local"),
		ZitadelIssuer:   getEnv("ZITADEL_ISSUER", ""),
		ZitadelClientID: getEnv("ZITADEL_CLIENT_ID", ""),
		RedisAddr:       getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:   getEnv("REDIS_PASSWORD", ""),
		RedisDB:         getEnvInt("REDIS_DB", 0),
		SessionTTL:      sessionTTL,
		CookieSecure:    getEnvBool("COOKIE_SECURE", false),
	}, nil
}
```

### `backend/internal/api/auth_settings.go`

```go
type AuthSettingsBody struct {
	Mode    string               `json:"mode" example:"local"`
	Zitadel *ZitadelSettingsBody `json:"zitadel,omitempty"`
}

type ZitadelSettingsBody struct {
	Issuer   string `json:"issuer" format:"uri" example:"https://auth.example.com"`
	ClientID string `json:"clientId" example:"312345678901234567"`
}

func registerAuthSettingsRoute(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "getAuthSettings",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/settings",
		Tags:        []string{"auth"},
	}, func(ctx context.Context, input *struct{}) (*GetAuthSettingsOutput, error) {
		body := AuthSettingsBody{
			Mode: deps.AuthMode,
		}

		if deps.AuthMode == "zitadel" {
			body.Zitadel = &ZitadelSettingsBody{
				Issuer:   deps.ZitadelIssuer,
				ClientID: deps.ZitadelClientID,
			}
		}

		return &GetAuthSettingsOutput{
			Body: body,
		}, nil
	})
}
```

この endpoint は、frontend が login 画面をどの mode で描画するべきか判断するために使います。応答例は次です。

```json
{
  "mode": "zitadel",
  "zitadel": {
    "issuer": "https://auth.example.com",
    "clientId": "312345678901234567"
  }
}
```

frontend 側では `frontend/src/api/auth.ts` で generated SDK を包み、`frontend/src/views/LoginView.vue` で起動時に `GET /api/v1/auth/settings` を読んで画面を切り替えます。

```ts
import './client';
import { getAuthSettings } from './generated/sdk.gen';

export async function fetchAuthSettings() {
  return getAuthSettings({});
}
```

`LoginView.vue` では、例えば次のように mode を読んで描画を切り替えます。

```ts
onMounted(async () => {
  try {
    const settings = await fetchAuthSettings();
    authMode.value = settings.mode as 'local' | 'zitadel';
    zitadelIssuer.value = settings.zitadel?.issuer ?? '';
  } catch {
    authMode.value = 'local';
  }
});
```

- `AUTH_MODE=local` なら今まで通り email / password form を表示する
- `AUTH_MODE=zitadel` なら local form を出さず、Zitadel 用の案内を表示する

### 実装後に流すコマンド

新しい endpoint を足したので、この段階でも `make gen` は必須です。

```bash
make gen
go test ./backend/...

cd frontend
npm run build
```

### どう確認するか

`.env` を次のように変えます。

```dotenv
AUTH_MODE=zitadel
ZITADEL_ISSUER=https://<your-zitadel-domain>
ZITADEL_CLIENT_ID=<your-client-id>
```

その上で backend と frontend を起動します。

```bash
make backend-dev
```

別 terminal:

```bash
cd frontend
npm run dev
```

まず backend の公開設定が返ることを確認します。

```bash
curl -sS http://localhost:8080/api/v1/auth/settings
```

その後、browser で `http://localhost:5173/login` を開きます。ここで login form が消え、`AUTH_MODE=zitadel` が有効であることと issuer 情報が表示されれば、mode 切替の足場はできています。

### この段階ではまだ未実装のもの

この節で入れるのは足場だけです。次は別タスクとして、少なくとも次が必要になります。

- Zitadel 側の application 作成
- redirect URI の登録
- frontend からの browser redirect
- callback での code 受け取り
- backend での token 検証または exchange
- local session への橋渡し

つまり、この段階で `AUTH_MODE=zitadel` を有効にしても、まだ本番 login は完成していません。local login を誤って通してしまわないよう、未実装 mode は `501 Not Implemented` にして止める方が安全です。

---

## 途中で迷わないための判断基準

実装中に「この処理はどこに置くべきか」で迷ったら、次の基準で判断してください。

- HTTP の入出力、validation、OpenAPI metadata なら `backend/internal/api/`
- 業務ルールなら `backend/internal/service/`
- SQL の形に落ちるなら `db/queries/` と `backend/internal/db/`
- 認証、Cookie、CSRF の横断処理なら `backend/internal/auth/`
- フロントの API 接続共通処理なら `frontend/src/api/client.ts`
- 画面状態なら `frontend/src/stores/`
- 画面表示そのものなら `frontend/src/views/`

この境界を守ると、`CONCEPT.md` の中心思想である「責務分離」と「契約ドリフトの最小化」が自然に実装へ反映されます。

---

## 生成物として扱うファイル

次のファイルは原則として手で編集しません。

- `openapi/openapi.yaml`
- `frontend/src/api/generated/*`
- `backend/internal/db/*`
- `db/schema.sql`

これらに差分が出たときは、「このファイルを直す」のではなく「何を正本として変えるべきか」を考えてください。

- OpenAPI が変なら `backend/internal/api/*`
- SQL 生成コードが変なら `db/queries/*` か `backend/sqlc.yaml`
- schema が変なら `db/migrations/*`

---

## この順番を勧める理由

このチュートリアルは、見た目のわかりやすさよりも、後戻りの少なさを優先して順番を決めています。

まず DB と SQL を先に置くのは、`sqlc` を使う構成では SQL が設計資産だからです。次に Go 側で Huma operation を作るのは、OpenAPI 3.1 を公開契約として正しく出すためです。最後に Vue 側をつなぐのは、生成 client を受け取る側としてフロントを扱うと責務がぶれにくいからです。

逆に、先に画面から作り始めると、最初は速く見えても後で次のような揺れが出ます。

- path や payload 名が変わるたびに画面と API を両方手修正する
- generated client と wrapper の責務が混ざる
- OpenAPI artifact がレビューされない

---

## 最後に: 最小の 1 周目で目指す状態

最初の 1 周目では、全部を完成させる必要はありません。まずは次の状態を目指してください。

- PostgreSQL と Redis が `compose.yaml` で起動する
- migration と `sqlc` 生成が通る
- Huma で `session`, `login`, `logout` の OpenAPI が出る
- Vue から generated client + wrapper 経由で session API を呼べる
- frontend を build して Go から配信できる

ここまで到達すれば、このリポジトリは「概念だけの設計書」ではなく、**次の機能を同じ型で増やせる実装基盤**になります。

その後は、同じ順番で機能を増やしてください。

1. migration を追加する
2. `db/queries/*` を増やす
3. service を増やす
4. Huma operation を追加する
5. OpenAPI を再生成する
6. frontend store / view を増やす

この反復が安定して回るなら、`CONCEPT.md` の設計は正しく実装へ変換できています。

---

## ここまでで何ができているか

このチュートリアルを最後まで進めると、次の状態になります。

- PostgreSQL 18 と Redis が `compose.yaml` で起動する
- migration と `sqlc` の流れがある
- Huma の Go 型から OpenAPI 3.1 が出る
- OpenAPI から frontend SDK が生成される
- Vue + Pinia から Cookie セッション API を叩ける
- CSRF token を wrapper で自動付与できる

つまり、`CONCEPT.md` は単なる方針文書ではなく、**次の業務機能を同じ型で増やせる作業台**に変わっています。

以後は、この型を崩さずに機能を足していってください。
