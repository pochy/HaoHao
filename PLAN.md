# HaoHao 初期実装計画

## Summary
- [CONCEPT.md](/Users/pochy/Projects/HaoHao/CONCEPT.md) を正本にし、最初の到達点を「基盤構築 + `Session/Profile` の縦切り 1 本」に固定する。
- 実装順は `リポジトリ骨格 → API契約生成 → DB → 認証/セッション → Frontend接続 → 縦切り機能 → 配信/CI` にする。依存の向きと一致しており、後戻りが最も少ない。
- 最初の完成条件は、`go run` で API が起動し、`openapi/openapi.yaml` と生成 client を更新でき、Vue から Cookie 認証付きで `GET /api/v1/session` と `GET /api/v1/profile` が動き、`FakeAuthProvider` を使ったローカル/CI の `login -> callback -> session bootstrap -> profile -> logout` smoke test、CSRF smoke test、SPA fallback smoke test が通ること。

## この計画の読み方
- 各 step は `この step の実作業順序` を先頭に置き、その下を実際の作業順に読む。見出しが `最初に打つコマンド`、`先に手で置くコード`、`次に打つコマンド`、`最後に打つコマンド` に分かれている場合は、その順に進めればよい。
- 先に固定するものは「後で変えると再配線コストが高いもの」に限る。具体的には `ディレクトリ構成`、`OpenAPI 生成経路`、`users` の ID 方針、`認証境界`、`transport wrapper` である。
- 逆に後回しにするものは「土台がないと正しく判断できないもの」に限る。具体的には `SPA 埋め込み配信`、`CI の詳細`、`Zitadel 実接続` である。
- サンプルコードは、直前に書いたファイル名へ置く断片を意味する。コマンドで生成されるファイルは、その step 内で「このコマンドで生成される」と明記する。
- `go run` や `npm run dev` のようにプロセスが張り付くコマンドは、別ターミナルで打つ前提で書く。その場合は `Terminal A`、`Terminal B` と明記する。

## プレースホルダの意味
- `<MODULE_PATH>` は `go mod init` に渡す Go module path を指す。例: `github.com/pochy/haohao/backend`
- `<seq>` は `migrate create -seq ...` が自動で付ける連番を指す。初回 migration なら通常は `000001` で、次は `000002` になる。
- `create_users` のような末尾の名前は migration 名であり、連番とは別である。

## 最短で進める実装順序
1. まず repo の骨格と起動点を固定する。
2. 次に `GET /api/v1/health` だけ持つ API を立ち上げる。
3. その API から OpenAPI を export できるようにする。
4. `users` migration と `sqlc` 生成を通す。
5. `AuthProvider` interface と `FakeAuthProvider` を先に実装する。
6. `SessionStore` と `CurrentUserResolver` を実装し、`GET /api/v1/session` を通す。
7. `GET /api/v1/profile` と `POST /api/v1/logout` を通す。
8. frontend の generated client と transport wrapper を接続する。
9. `Login`、`Home`、`Profile` の 3 画面だけ作る。
10. ここまで通ってから `ZitadelProvider` を差し込む。
11. 最後に `embed.FS` と CI を固める。

この順序にする理由は、毎ステップの成果物が次のステップの入力になるからである。`health` がないと OpenAPI を export できず、OpenAPI がないと frontend client を生成できず、`users` がないと callback 後の profile 表示を成立させられない。先に UI から始めると、見た目だけ進んで根幹の責務分離が崩れる。

## Implementation Steps

### 1. リポジトリ骨格を固定する

この step の実作業順序:
1. まず repo root に `backend/`、`db/`、`openapi/`、`frontend/` の土台を作る。
2. 次に `go.work`、`backend/go.mod`、Vite app をコマンドで生成する。
3. その後で `Makefile` と `compose.yaml` を手で置く。
4. 最後に Go workspace、backend package 解決、frontend build、DB/Redis 起動確認を行う。

最初に打つコマンド:

```sh
export MODULE_PATH=github.com/pochy/haohao/backend

mkdir -p backend/cmd \
  backend/internal/{api,app,auth,config,middleware,service} \
  backend/web/dist \
  db/{migrations,queries} \
  openapi

cat > go.work <<'EOF'
go 1.25.0

use ./backend
EOF

cd backend
go mod init "$MODULE_PATH"
go get github.com/danielgtaylor/huma/v2 \
  github.com/danielgtaylor/huma/v2/adapters/humagin \
  github.com/gin-gonic/gin \
  github.com/jackc/pgx/v5/pgxpool \
  github.com/redis/go-redis/v9

cd ..
npm create vite@latest frontend -- --template vue-ts
npm --prefix frontend install
npm --prefix frontend install pinia vue-router
npm --prefix frontend install -D @hey-api/openapi-ts
```

次に手で置くコード:

以下の Go snippet 中の `<MODULE_PATH>` は Step 1 で決めた module path に置換する。

`Makefile`

```make
.PHONY: api gen

api:
	go run ./backend/cmd

gen:
	go run ./backend/cmd openapi > openapi/openapi.yaml
	npm --prefix frontend exec openapi-ts -- -i ../openapi/openapi.yaml -o src/api/generated
```

`compose.yaml`

```yaml
services:
  db:
    image: postgres:18
    ports:
      - "5432:5432"
    environment:
      POSTGRES_USER: app
      POSTGRES_PASSWORD: app
      POSTGRES_DB: haohao

  redis:
    image: redis:8
    ports:
      - "6379:6379"
```

最後に打つコマンド:

```sh
go work edit -json
go list ./backend/...
docker-compose up -d db redis
docker-compose ps db redis
npm --prefix frontend run build
test -f frontend/dist/index.html
```

なぜこの順序か:
- `go.work` を最初に置くのは、repo root から `go run ./backend/cmd` を一貫して使うためである。ここがぶれると Makefile、CI、README、IDE 設定が全部ズレる。
- `npm --prefix frontend ...` を使うのは、root に Node の package 管理を持ち込まず、frontend 依存を `frontend/` に閉じ込めるためである。モノレポでも runtime を混ぜない方が事故が少ない。
- `@hey-api/openapi-ts` を frontend の devDependencies に入れるのは、毎回 `npx` でネットワーク解決させないためである。生成系ツールは repo ローカル依存にした方が再現性が高い。

この step の完了条件:
- `go.work` と `backend/go.mod` があり、repo root から `go work edit -json` と `go list ./backend/...` が通る。
- `docker-compose up -d db redis` の後に `docker-compose ps db redis` を実行し、PostgreSQL と Redis の両方が `running` になる。
- `npm --prefix frontend run build` が通り、`test -f frontend/dist/index.html` で Vite の build 成果物を確認できる。

### 2. Backend の最小起動と OpenAPI export を通す

この step の実作業順序:
1. まず `backend/internal/app/app.go` に Huma/Gin の最小構成と `GET /api/v1/health` を置く。
2. 次に `backend/cmd/main.go` に server 起動と `openapi` export の分岐を置く。
3. 最後に API を起動し、`/api/v1/health` と `openapi/openapi.yaml` を確認する。

先に手で置くコード:

`backend/internal/app/app.go`

```go
package app

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
)

type App struct {
	Router *gin.Engine
	API    huma.API
}

type HealthOutput struct {
	Body struct {
		OK bool `json:"ok"`
	}
}

func Build() *App {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	config := huma.DefaultConfig("HaoHao API", "0.1.0")
	api := humagin.New(router, config)

	huma.Get(api, "/api/v1/health", func(ctx context.Context, input *struct{}) (*HealthOutput, error) {
		out := &HealthOutput{}
		out.Body.OK = true
		return out, nil
	})

	return &App{Router: router, API: api}
}
```

`backend/cmd/main.go`

```go
package main

import (
	"log"
	"os"

	"<MODULE_PATH>/internal/app"
)

func main() {
	built := app.Build()

	if len(os.Args) > 1 && os.Args[1] == "openapi" {
		b, err := built.API.OpenAPI().YAML()
		if err != nil {
			log.Fatal(err)
		}
		if _, err := os.Stdout.Write(b); err != nil {
			log.Fatal(err)
		}
		return
	}

	log.Fatal(built.Router.Run(":8080"))
}
```

最後に打つコマンド:

```sh
go run ./backend/cmd openapi > openapi/openapi.yaml

# Terminal A
go run ./backend/cmd

# Terminal B
curl -s http://localhost:8080/api/v1/health
head -20 openapi/openapi.yaml
```

なぜこの順序か:
- 最初に `health` だけ通すのは、OpenAPI 生成の経路と router/huma 配線が正しいかを最小コストで確認するためである。
- `go run ./backend/cmd openapi` を `curl /openapi.yaml` ではなく custom command にするのは、CI で server boot、DB 接続、port 確保を不要にするためである。契約生成は runtime 依存を持たない方が強い。
- `Build()` を分けるのは、「server 起動」と「OpenAPI export」が同じ登録経路を通るようにするためである。これを分けると spec と runtime のドリフトが起きる。

この step の完了条件:
- `curl -s http://localhost:8080/api/v1/health` で `{"ok":true}` 相当の応答を確認できる。
- `go run ./backend/cmd openapi > openapi/openapi.yaml` が通り、`head -20 openapi/openapi.yaml` で spec の先頭を確認できる。
- API 登録は `Build()` に集約されている。

### 3. DB 基盤を `users` だけで通す

この step の実作業順序:
1. まず `brew install golang-migrate sqlc` で CLI を入れる。
2. 次に `backend/sqlc.yaml` と `db/queries/users.sql` を手で置く。
3. その後で `migrate create -ext sql -dir db/migrations -seq create_users` を実行し、migration file を生成する。
4. そのコマンドで生成された `db/migrations/<seq>_create_users.up.sql` と `db/migrations/<seq>_create_users.down.sql` を編集する。`<seq>` は初回なら通常 `000001` である。
5. 最後に DB を起動し、migration 適用、schema dump、`sqlc generate` の順で進める。

最初に打つコマンド:

```sh
brew install golang-migrate sqlc
```

先に手で置くコード:

`backend/sqlc.yaml`

```yaml
version: "2"
sql:
  - engine: postgresql
    schema: "../db/schema.sql"
    queries: "../db/queries"
    gen:
      go:
        package: db
        out: "internal/db"
        sql_package: "pgx/v5"
```

`db/queries/users.sql`

```sql
-- name: GetUserByIssuerAndSubject :one
SELECT id, public_id, issuer, subject, display_name, email, created_at, updated_at
FROM users
WHERE issuer = $1 AND subject = $2;

-- name: UpsertUserFromIdentity :one
INSERT INTO users (issuer, subject, display_name, email)
VALUES ($1, $2, $3, $4)
ON CONFLICT (issuer, subject)
DO UPDATE SET
  display_name = EXCLUDED.display_name,
  email = EXCLUDED.email,
  updated_at = now()
RETURNING id, public_id, issuer, subject, display_name, email, created_at, updated_at;
```

次に打つコマンド:

```sh
migrate create -ext sql -dir db/migrations -seq create_users
```

このコマンドで生成され、続けて編集するファイル:

`db/migrations/<seq>_create_users.up.sql`

```sql
CREATE TABLE users (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  public_id uuid NOT NULL DEFAULT uuidv7() UNIQUE,
  issuer text NOT NULL,
  subject text NOT NULL,
  display_name text NOT NULL,
  email text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (issuer, subject)
);
```

`db/migrations/<seq>_create_users.down.sql`

```sql
DROP TABLE IF EXISTS users;
```

最後に打つコマンド:

```sh
docker-compose up -d db
migrate -path db/migrations -database "postgres://app:app@localhost:5432/haohao?sslmode=disable" up
docker-compose exec -T db pg_dump --schema-only --no-owner --no-privileges -U app -d haohao > db/schema.sql
sqlc generate -f backend/sqlc.yaml
```

確認ポイント:

```sh
sqlc vet -f backend/sqlc.yaml
docker-compose exec -T db psql -U app -d haohao -c '\d users'
```

なぜこの順序か:
- `users` だけ先に作るのは、最初の縦切りが profile 表示までであり、他テーブルを足しても価値検証が前に進まないからである。
- `issuer + subject` を `users` に直載せするのは、初期スライスを軽く保ちつつ、`subject` 単独より安全に一意性を持てるからである。`auth_identities` 分離は多 IdP 要件が出た時でよい。
- `db/schema.sql` を migration 適用後の DB から再生成するのは、schema の正本を migration に寄せるためである。手書きで `schema.sql` を更新すると drift の温床になる。
- `pg_dump` と `psql` を host 側ではなく `db` コンテナ内で実行するのは、PostgreSQL client/server のメジャーバージョン不一致で詰まらないためである。schema dump は DB と同じメジャー版の client を使った方が安全である。

この step の完了条件:
- `docker-compose exec -T db psql -U app -d haohao -c '\d users'` で `users` table を確認できる。
- `sqlc generate -f backend/sqlc.yaml` と `sqlc vet -f backend/sqlc.yaml` が通る。
- `backend/internal/db` 配下に `GetUserByIssuerAndSubject` と `UpsertUserFromIdentity` を含む生成コードができる。

### 4. 認証境界は Fake から先に通す

この step の実作業順序:
1. まず `Identity`、`AuthProvider`、`Session`、`SessionStore` の境界を型として固定する。
2. 次に `FakeAuthProvider` を置き、`/auth/login` と `/auth/callback` が本番と同じ経路で動くようにする。
3. その後で login/callback handler の骨格を置く。
4. 最後に Redis を起動し、redirect と test の確認を行う。

先に手で置くコード:

`backend/internal/auth/types.go`

```go
type Identity struct {
	Issuer  string
	Subject string
	Name    string
	Email   string
}

type AuthProvider interface {
	AuthorizeURL(ctx context.Context, state string) (string, error)
	ExchangeCallback(ctx context.Context, code string) (*Identity, error)
}

type Session struct {
	ID        string
	UserID    int64
	CSRFToken string
}

type SessionStore interface {
	Put(ctx context.Context, session *Session, ttl time.Duration) error
	Get(ctx context.Context, sessionID string) (*Session, error)
	Delete(ctx context.Context, sessionID string) error
}
```

`backend/internal/auth/fake.go`

```go
type FakeAuthProvider struct{}

func (p *FakeAuthProvider) AuthorizeURL(ctx context.Context, state string) (string, error) {
	return "/api/v1/auth/callback?code=fake-code&state=" + url.QueryEscape(state), nil
}

func (p *FakeAuthProvider) ExchangeCallback(ctx context.Context, code string) (*Identity, error) {
	return &Identity{
		Issuer:  "fake",
		Subject: "local-user",
		Name:    "Local User",
		Email:   "local@example.com",
	}, nil
}
```

`backend/internal/api/auth.go`

```go
// login handler の形
func Login(c *gin.Context) {
	url, err := authProvider.AuthorizeURL(c.Request.Context(), state)
	if err != nil {
		// problem details に変換
		return
	}
	c.Redirect(http.StatusFound, url)
}
```

最後に打つコマンド:

```sh
docker-compose up -d redis
go test ./backend/...

# backend server が別ターミナルで起動している前提
curl -I http://localhost:8080/api/v1/auth/login
curl -I "http://localhost:8080/api/v1/auth/callback?code=fake-code&state=test"
```

なぜこの順序か:
- 先に `FakeAuthProvider` を作るのは、callback 後の session 確立、DB upsert、Cookie 発行、frontend bootstrap を実 IdP なしで通すためである。最初の価値は「ログインできること」ではなく「ログイン後のアプリ導線が一貫していること」である。
- `dev-only login` を採らないのは、本番には存在しない入口を増やさないためである。`FakeAuthProvider` なら本番と同じ `/auth/login` と `/auth/callback` を使える。
- `POST /api/v1/logout` を local-only の処理に留めるのは、Single Logout を初期要件に含めないためである。ここで IdP の end-session まで抱えると論点が増えすぎる。

この step の完了条件:
- `curl -I http://localhost:8080/api/v1/auth/login` が 302 を返す。
- `curl -I "http://localhost:8080/api/v1/auth/callback?code=fake-code&state=test"` が session 確立後の redirect を返し、成功時 `/`、失敗時 `/login?error=...` に流れる。
- `go test ./backend/...` が通る。
- `POST`、`PUT`、`PATCH`、`DELETE` の CSRF middleware の骨格ができている。

### 5. Frontend は generated client ではなく wrapper から始める

この step の実作業順序:
1. まず `make gen` を実行し、OpenAPI から generated client を作る。
2. 次に `frontend/src/api/transport.ts` と `frontend/src/api/client.ts` を手で置く。
3. 最後に frontend build を通し、wrapper 経由の import/export が壊れていないことを確認する。

最初に打つコマンド:

```sh
make gen
```

次に手で置くコード:

`frontend/src/api/transport.ts`

```ts
const UNSAFE_METHODS = new Set(["POST", "PUT", "PATCH", "DELETE"])

function readCookie(name: string): string | undefined {
  return document.cookie
    .split("; ")
    .find((row) => row.startsWith(`${name}=`))
    ?.split("=")[1]
}

export async function apiFetch(input: RequestInfo | URL, init: RequestInit = {}) {
  const headers = new Headers(init.headers)
  const method = (init.method ?? "GET").toUpperCase()

  if (UNSAFE_METHODS.has(method)) {
    const token = readCookie("XSRF-TOKEN")
    if (token) {
      headers.set("X-CSRF-Token", decodeURIComponent(token))
    }
  }

  return fetch(input, {
    ...init,
    credentials: "include",
    headers,
  })
}
```

`frontend/src/api/client.ts`

```ts
import { client } from "./generated/client"
import { apiFetch } from "./transport"

client.setConfig({
  baseUrl: "/api",
  fetch: apiFetch,
})
```

最後に打つコマンド:

```sh
npm --prefix frontend run build

# ブラウザで手動確認したい場合のみ
npm --prefix frontend run dev
```

なぜこの順序か:
- wrapper を先に置くのは、`credentials: "include"`、CSRF header、problem details 変換を画面ごとに書かないためである。ここを横着すると 2 画面目から確実にズレる。
- `baseUrl: "/api"` を wrapper 側で固定するのは、開発時 proxy と本番時同一オリジン配信の両方で同じ呼び出しコードを使うためである。
- generated client を page から直接 import しないのは、OpenAPI 再生成の破壊的変更を UI 全体に漏らさないためである。

この step の完了条件:
- `make gen` で spec と generated client を更新できる。
- `npm --prefix frontend run build` が通る。
- frontend から generated client ではなく wrapper 経由で API を叩く土台ができる。

### 6. `Session/Profile` の縦切りを通す

この step の実作業順序:
1. 先に backend で `GET /api/v1/session` を通す。
2. その後で frontend の Pinia store に `bootstrap()` を書く。
3. 次に backend で `GET /api/v1/profile` と `POST /api/v1/logout` を通す。
4. 最後に `Login`、`Home`、`Profile` ページを接続し、guest/login/logout の画面遷移を確認する。

先に手で置くコード:

`backend/internal/api/session.go`

```go
type SessionResponse struct {
	Authenticated bool       `json:"authenticated"`
	User          *UserBrief `json:"user,omitempty"`
	CSRFReady     bool       `json:"csrfReady"`
}
```

`frontend/src/stores/session.ts`

```ts
type SessionStatus = "unknown" | "guest" | "authenticated"

export const useSessionStore = defineStore("session", {
  state: () => ({
    status: "unknown" as SessionStatus,
    user: null as null | { publicId: string; displayName: string },
  }),
  actions: {
    async bootstrap() {
      const session = await getSession()
      this.status = session.authenticated ? "authenticated" : "guest"
      this.user = session.user ?? null
    },
  },
})
```

最後に打つコマンド:

```sh
# backend server が別ターミナルで起動している前提
curl -i http://localhost:8080/api/v1/session
curl -i http://localhost:8080/api/v1/profile

# ブラウザで画面遷移を確認したい場合のみ
npm --prefix frontend run dev
```

なぜこの順序か:
- 先に store を作るのではなく `GET /api/v1/session` を通すのは、frontend の状態遷移を backend 契約に合わせるためである。先に store から作ると、あとで DTO に合わせて壊しやすい。
- `session` と `profile` を分けるのは、header/bootstrap 用の最小データと画面表示用の詳細データを混ぜないためである。これを最初に分けておくと、後で header 用 API を削り直さずに済む。
- `csrfReady` を残すのは、「このレスポンス後に unsafe method を投げてよい」ことを画面側が明示的に扱えるようにするためである。

この step の完了条件:
- guest でも `GET /api/v1/session` が 200 を返し、`XSRF-TOKEN` bootstrap を担う。
- login 後に `Home` から `Profile` へ遷移できる。
- logout 後に `guest` 状態へ戻る。

### 7. Zitadel 実装は最後に差し替える

この step の実作業順序:
1. まず `AuthProvider` interface を満たす `ZitadelProvider` を追加する。
2. 次に DI/config で Fake と Zitadel を差し替えられるようにする。
3. 最後に backend test を流し、handler 層へ条件分岐が漏れていないことを確認する。

先に手で置くコード:

`backend/internal/auth/zitadel.go`

```go
type ZitadelProvider struct {
	// client, issuer, redirect URL など
}

func (p *ZitadelProvider) AuthorizeURL(ctx context.Context, state string) (string, error) {
	// PKCE 付き authorize URL を返す
}

func (p *ZitadelProvider) ExchangeCallback(ctx context.Context, code string) (*Identity, error) {
	// token exchange と userinfo/id token 解決
}
```

最後に打つコマンド:

```sh
go test ./backend/...
```

なぜこの順序か:
- ここを最後にするのは、OIDC 実装の難所を「アプリ本体の完成」から切り離すためである。先に Zitadel をやると、本当に詰まっているのが OIDC なのかアプリ構造なのか見分けがつかなくなる。
- `AuthProvider` interface の差し替えに留めるのは、Fake と本番の分岐を handler 層に漏らさないためである。

この step の完了条件:
- `go test ./backend/...` が通る。
- config/DI 上の `AuthProvider` 差し替えだけで Fake と Zitadel を切り替えられる。

### 8. 最後に配信と CI を固める

この step の実作業順序:
1. まず `frontend/vite.config.ts` で build 出力先を `backend/web/dist` に固定する。
2. 次に `backend/web/embed.go` と `backend/internal/app/spa.go` を置き、backend binary で静的配信できるようにする。
3. その後で `Makefile` に local/CI 共通の入口を置く。
4. 最後に `.github/workflows/ci.yaml` を置き、build・test・smoke を CI から同じ target で叩く。
5. 実装後に frontend build、backend run、SPA smoke、`make ci` の順で確認する。

先に手で置くコード:

`frontend/vite.config.ts`

```ts
import { resolve } from "node:path"
import { defineConfig } from "vite"
import vue from "@vitejs/plugin-vue"

export default defineConfig({
	plugins: [vue()],
	build: {
		outDir: resolve(__dirname, "../backend/web/dist"),
		emptyOutDir: true,
	},
})
```

`backend/web/embed.go`

```go
package web

import (
	"embed"
	"io/fs"
	"log"
)

//go:embed dist
var embedded embed.FS

func MustDistFS() fs.FS {
	dist, err := fs.Sub(embedded, "dist")
	if err != nil {
		log.Panic(err)
	}
	return dist
}
```

`backend/internal/app/spa.go`

```go
func RegisterSPARoutes(router *gin.Engine, dist fs.FS) {
	fileServer := http.FileServer(http.FS(dist))

	router.NoRoute(func(c *gin.Context) {
		path := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")

		switch {
		case strings.HasPrefix(path, "api/"):
			c.Status(http.StatusNotFound)
		case hasEmbeddedAsset(dist, path):
			fileServer.ServeHTTP(c.Writer, c.Request)
		case acceptsHTML(c.Request) && (c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead):
			c.Request.URL.Path = "/index.html"
			fileServer.ServeHTTP(c.Writer, c.Request)
		default:
			c.Status(http.StatusNotFound)
		}
	})
}
```

`Makefile`

```make
.PHONY: frontend-build backend-test frontend-check sqlc-check verify-generated smoke-spa ci

frontend-build:
	npm --prefix frontend run build

backend-test:
	go test ./backend/...

frontend-check:
	npm --prefix frontend run typecheck
	npm --prefix frontend run build

sqlc-check:
	sqlc generate -f backend/sqlc.yaml
	sqlc vet -f backend/sqlc.yaml

verify-generated: gen
	git diff --exit-code -- openapi/openapi.yaml frontend/src/api/generated

smoke-spa:
	go run ./backend/cmd >/tmp/haohao-smoke.log 2>&1 & pid=$$!; \
	trap 'kill $$pid' EXIT; \
	sleep 2; \
	curl -fsS http://localhost:8080/api/v1/health; \
	curl -fsSI -H 'Accept: text/html' http://localhost:8080/profile; \
	test "$$(curl -o /dev/null -sS -w '%{http_code}' -H 'Accept: application/javascript' http://localhost:8080/assets/not-found.js)" = "404"

ci: verify-generated backend-test frontend-check sqlc-check smoke-spa
```

`.github/workflows/ci.yaml`

```yaml
name: ci

on:
  pull_request:
  push:
    branches: [main]

jobs:
  verify:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:18
        env:
          POSTGRES_USER: app
          POSTGRES_PASSWORD: app
          POSTGRES_DB: haohao
        ports:
          - 5432:5432
        options: >-
          --health-cmd "pg_isready -U app -d haohao"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      redis:
        image: redis:8
        ports:
          - 6379:6379
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: backend/go.mod
      - uses: actions/setup-node@v4
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: frontend/package-lock.json
      - run: npm --prefix frontend ci
      - name: contract
        run: make verify-generated
      - name: backend
        run: make backend-test
      - name: frontend
        run: make frontend-check
      - name: sqlc
        run: make sqlc-check
      - name: smoke-spa
        run: make smoke-spa
```

最後に打つコマンド:

```sh
npm --prefix frontend run build
test -f backend/web/dist/index.html

# Terminal A
go run ./backend/cmd

# Terminal B
curl -i http://localhost:8080/api/v1/health
curl -i -H 'Accept: text/html' http://localhost:8080/profile
curl -i -H 'Accept: application/javascript' http://localhost:8080/assets/not-found.js

# Terminal A を止めた後
make ci
```

なぜこの順序か:
- build 出力先を最初に固定するのは、`frontend/dist` と `backend/web/dist` の二重管理を避けるためである。配信基盤でいちばん壊れやすいのは artifact の置き場所のブレである。
- `embed.FS` の前に `npm build` を通すのは、`dist` が空のまま `go:embed` を評価して build failure になるのを避けるためである。ここは順序依存が強い。
- SPA fallback を `Accept: text/html` かつ `GET/HEAD` に限定するのは、存在しない `.js` や `.png` を `index.html` に誤変換しないためである。ここを雑にすると frontend の不具合追跡が極端に難しくなる。
- Makefile を先に固めてから workflow を書くのは、ローカルと CI で別々の手順書が育つのを防ぐためである。CI は shell script ではなく、repo 内の単一入口を叩くだけにした方が長持ちする。

この step の完了条件:
- `npm --prefix frontend run build` だけで `backend/web/dist/index.html` が毎回再生成される。
- frontend build を埋め込んだ backend が動き、`/api/v1/health`、`/profile`、存在しない asset の 3 パターンを明確にさばける。
- clean checkout の状態で `make ci` が通り、GitHub Actions も同じ `make` target 群だけを使う。
- contract 生成物、backend test、frontend typecheck/build、`sqlc`、SPA smoke のどこで壊れたかを workflow の step 名と `make` target 名だけで特定できる。

### 8 の完了後に毎回踏む手順

1. feature を追加したら、まず `make gen` を実行し、OpenAPI と generated client の差分を先に確定する。
2. 次に backend/frontend の実装を入れた後、PR 作成前に `make ci` をローカルで 1 回通す。失敗したら CI に投げる前に target 単位で原因を潰す。
3. PR では generated file の差分を隠さず commit する。`openapi/openapi.yaml` や `frontend/src/api/generated` を未更新のままにしない。
4. merge 前には required check を `verify` に固定し、その内側の step 名を `contract`、`backend`、`frontend`、`sqlc`、`smoke-spa` にする。レビューアが失敗箇所を UI 上で即読める名前にする。
5. main merge 後は clean checkout から binary を作り直し、`/api/v1/health`、`/profile`、`/assets/not-found.js` の smoke を再実行する。deploy 手順の正しさは unit test ではなく artifact で確認する。
6. 以後の機能追加も、毎回 `migration -> sqlc query -> service -> Huma operation -> openapi regen -> frontend adapter -> page -> make ci -> smoke` の順に戻る。配信と CI を最後に 1 回だけやる作業にしない。

## Public APIs / Important Interfaces
- `GET /api/v1/health`: liveness 確認用。最初の OpenAPI export と reverse proxy 疎通確認に使う。
- `GET /api/v1/auth/login`: Zitadel の authorize に 302 redirect する browser-only endpoint。Vue からは通常 API client ではなくブラウザ遷移で使う。
- `GET /api/v1/auth/callback`: `code/state` を処理して session cookie を確立し、成功時は `/`、失敗時は `/login?error=...` に 302 redirect する。
- `GET /api/v1/session`: `{ authenticated, user?, csrfReady }` のような軽量 DTO を返す。session 確認と CSRF bootstrap を兼任し、`csrfReady` はこのレスポンス後に CSRF token が利用可能であることを示す。
- `GET /api/v1/profile`: 画面表示用の profile DTO を返す。外部公開 ID は `users.public_id` の `uuidv7` を使い、内部 DB join は `users.id` の `bigint` を維持する。
- `POST /api/v1/logout`: CSRF 必須。Redis session を削除し、cookie を無効化する。
- `AuthProvider`: Zitadel 固有処理を隔離するための境界。handler/service から OIDC SDK を直接呼ばない。本番実装は `ZitadelProvider`、dev/CI/E2E は `FakeAuthProvider` にする。
- `SessionStore`: Redis 用の境界。DB repository と分けるのは、session が SQL 永続化と性質が異なるため。
- `DomainError`: service から返す機械可読エラー。operation が HTTP status / problem details に写像する。

## Test Plan
- `make gen` 実行後に `openapi/openapi.yaml` と `frontend` 生成 client に差分が残らないこと。
- Huma handler の test で、未認証 `GET /api/v1/profile` が 401、`POST`、`PUT`、`PATCH`、`DELETE` の CSRF 不一致 request が 403 になること。
- `GET /api/v1/session` の test で、guest でも `XSRF-TOKEN` bootstrap を担い、レスポンス後に CSRF token が利用可能になること。
- `sqlc` クエリの integration test で、初回ログイン時に `users` が upsert され、再ログイン時に重複作成されないこと。identity 解決は `issuer + subject` で行うこと。
- Frontend test で、transport wrapper が `credentials: "include"` を付け、`XSRF-TOKEN` から `X-CSRF-Token` を自動付与すること。
- E2E smoke は `FakeAuthProvider` 前提で、`login -> callback -> session bootstrap -> profile 表示 -> logout` が通ること。
- 埋め込み配信 test で、`/api/v1/health` は API に届き、`/profile` は `index.html`、存在しないアセットは 404 になること。
- CI は最低でも `contract`、`backend`、`frontend`、`sqlc`、`smoke-spa` の 5 step に分けて失敗箇所を見える化する。workflow の中身は individual command ではなく `make` target に寄せる。
- `contract` step は `make gen` の実行と generated file の diff 検査を担う。差分が出たら merge blocker にする。
- `backend` step は `go test ./backend/...`、`frontend` step は `npm --prefix frontend run typecheck && npm --prefix frontend run build`、`sqlc` step は `sqlc generate` と `sqlc vet` を担当する。
- `smoke-spa` step は build 済み backend を起動し、`/api/v1/health`、`Accept: text/html` 付き `/profile`、存在しない asset の 404 を `curl` で確認する。
- `sqlc verify` は `sqlc Cloud` 導入後に追加する。

## Assumptions / Defaults
- 現在の repo は実装コードがなく、初回 scaffold から始める前提。
- `MODULE_PATH` は一度決めたら途中で変えない。Go import path は後からの張り替えコストが高いため、最初に固定する。
- `compose.yaml` は最初は PostgreSQL 18 と Redis を持ち、Zitadel は環境変数で接続する外部依存として扱う。ローカル E2E と CI は `FakeAuthProvider` を使い、本番コードの入口は `AuthProvider` に統一する。`dev-only login` は採用しない。
- 生成物である `openapi/openapi.yaml` と `frontend` client はコミット対象にする。
- 以後の業務機能は、毎回 `migration -> sqlc query -> service -> Huma operation -> openapi regen -> frontend adapter -> page` の順で追加する。
- callback 失敗時の query parameter 名は `error` に固定する。
- Single Logout や IdP の end-session 連携は初期スライスの対象外にする。
- `sqlc verify` は `sqlc Cloud` を導入するまで CI 必須項目に含めない。
- `@hey-api/openapi-ts`、`sqlc`、Go、Node のバージョン固定は将来追加候補とし、今回は方針レベルに留める。
- 参考: Huma OpenAPI/CLI/adapter は公式 docs を前提にする。https://huma.rocks/features/openapi-generation/ , https://huma.rocks/features/cli/ , https://huma.rocks/features/bring-your-own-router/
- 参考: `sqlc` の CI 運用と `verify` の前提は公式 docs に合わせる。https://docs.sqlc.dev/en/stable/howto/ci-cd.html , https://docs.sqlc.dev/en/latest/howto/verify.html
- 参考: `@hey-api/openapi-ts` は OpenAPI 3.1 入力を扱える現行 CLI を使う。https://www.npmjs.com/package/%40hey-api/openapi-ts
- `sqlc` install docs: https://docs.sqlc.dev/en/stable/overview/install.html
- `golang-migrate` CLI install docs: https://pkg.go.dev/github.com/golang-migrate/migrate/v4/cmd/migrate
- PostgreSQL `pg_dump` docs: https://www.postgresql.org/docs/17/app-pgdump.html
