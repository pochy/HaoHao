# 単一バイナリ配信を完成させるチュートリアル

## この文書の目的

この文書は、現在の HaoHao repository を起点にして、`CONCEPT.md` が掲げている **単一バイナリ配信** を完成させるための手順書です。

ここでいう単一バイナリ配信とは、Vue の production build を Go binary に埋め込み、1 本の Go binary が次をまとめて返せる状態を指します。

- `/api/*`: backend API
- `/docs`: Huma docs
- `/openapi.yaml` / `/openapi.json`: OpenAPI
- `/`, `/login`, `/integrations`: Vue SPA
- `/assets/*`, `/favicon.svg`, `/icons.svg`: frontend build artifact

この文書は、既存の `TUTORIAL.md` / `TUTORIAL_ZITADEL.md` と同じように、どのファイルを作り、どの順番で確認するかを追える形にしています。

## この文書が前提にしている現在地

この repository は、少なくとも次の状態にある前提で進めます。

- `frontend/vite.config.ts` の build output が `../backend/web/dist` になっている
- `backend/web/dist/` は frontend build artifact として `.gitignore` されている
- `backend/cmd/main/main.go` は Gin / Huma backend を起動できる
- `go test ./backend/...` が build tag なしで通る
- `npm --prefix frontend run build` が通る
- `.github/workflows/ci.yml` と `.github/workflows/release.yml` は既に存在する

この文書では backend の API contract は変えません。OpenAPI schema も増やしません。追加するのは、Go binary に frontend dist を埋め込んで、Gin の `NoRoute` で SPA fallback を処理する部分です。

## 完成条件

このチュートリアルの完了条件は次です。

- `npm --prefix frontend run build` が `backend/web/dist/` を生成する
- `CGO_ENABLED=0 go build -buildvcs=false -trimpath -tags "embed_frontend nomsgpack" -ldflags "-s -w -buildid=" -o ./bin/haohao ./backend/cmd/main` が通る
- `./bin/haohao` 1 本で API / docs / OpenAPI / SPA を返せる
- `/`, `/login`, `/integrations` が `index.html` を返す
- `/api/*`, `/docs`, `/openapi.yaml` は SPA fallback されない
- 存在しない `/assets/*` は `index.html` ではなく `404` を返す
- `docker build -t haohao:dev -f docker/Dockerfile .` が通る
- runtime image は `scratch` base で、CA bundle と binary だけを含む
- binary は size reduction flags 付きで作られ、VCS metadata や debug 情報を含まない
- `./haohao` はカレントディレクトリまたは binary と同じ directory の `.env` を任意で読み込む
- embedded frontend build では dev 用 `FRONTEND_BASE_URL=http://127.0.0.1:5173` が残っていても `APP_BASE_URL` 側へ戻れる
- CI が生成物 drift、frontend build、embedded binary build、Docker build を検知する
- release workflow が OpenAPI artifact と single binary asset を release に載せられる

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | `frontend/vite.config.ts`, `.gitignore` | frontend build output の現在地を確認する |
| Step 2 | `backend/frontend.go`, `backend/frontend_embed.go`, `backend/frontend_stub.go`, `backend/frontend_test.go` | build tag 付きで frontend dist を embed する |
| Step 3 | `backend/internal/config/*` | 単一バイナリ用の `.env` 読み込みと frontend URL 補正を入れる |
| Step 4 | `backend/cmd/main/main.go` | SPA 配信を runtime に接続する |
| Step 5 | `Makefile` | build の入口を固定する |
| Step 6 | local binary | 1 本の binary で API と SPA を確認する |
| Step 7 | `docker/Dockerfile`, `.dockerignore` | production image を作る |
| Step 8 | `.github/workflows/ci.yml` | CI で embedded binary / Docker build を検知する |
| Step 9 | `.github/workflows/release.yml` | release asset に binary を追加する |

## Step 1. frontend build output を確認する

まず、frontend build の出力先を確認します。

#### ファイル: `frontend/vite.config.ts`

```ts
build: {
  outDir: '../backend/web/dist',
  emptyOutDir: true,
},
```

この設定により、次のコマンドで Vue の production bundle が `backend/web/dist/` に出ます。

```bash
npm --prefix frontend run build
```

`backend/web/dist/` は生成物です。source として編集しません。

#### ファイル: `.gitignore`

```gitignore
backend/web/dist
bin
```

`backend/web/dist` は commit しません。Go binary へ embed するための入力ではありますが、repository 上の正本ではありません。

正本は次です。

- frontend source: `frontend/src/*`
- frontend build config: `frontend/vite.config.ts`
- OpenAPI input: `openapi/openapi.yaml`
- generated SDK: `frontend/src/api/generated/*`

## Step 2. frontend embed の Go ファイルを追加する

ここでは root backend package に frontend 配信用の関数を追加します。

追加する public interface は 1 つだけです。

```go
func RegisterFrontendRoutes(router *gin.Engine) error
```

build tag なしの通常 build では frontend を embed しません。これにより、次のコマンドは `backend/web/dist/` が無くても壊れません。

```bash
go test ./backend/...
go run ./backend/cmd/openapi
```

frontend を埋め込む production binary を作るときだけ、`embed_frontend` build tag を付けます。ここでは Gin の未使用 msgpack binding を外す `nomsgpack`、VCS metadata を埋め込まない `-buildvcs=false`、path 情報を削る `-trimpath`、debug / symbol 情報を削る `-ldflags "-s -w -buildid="` も付けます。

```bash
CGO_ENABLED=0 go build -buildvcs=false -trimpath -tags "embed_frontend nomsgpack" -ldflags "-s -w -buildid=" -o ./bin/haohao ./backend/cmd/main
```

この build は debugging より配布サイズを優先します。panic stack trace に関数名は残りますが、DWARF debug 情報や symbol table は削られるため、production artifact として扱います。debug しやすい binary が必要なときは、これらの size reduction flags を外して一時 build します。

### 2-1. 共通 route 実装を書く

#### ファイル: `backend/frontend.go`

```go
package backend

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

var ErrFrontendNotEmbedded = errors.New("frontend dist is not embedded")

func RegisterFrontendRoutes(router *gin.Engine) error {
	distFS, err := frontendDistFS()
	if err != nil {
		return err
	}

	return registerFrontendRoutes(router, distFS)
}

func registerFrontendRoutes(router *gin.Engine, distFS fs.FS) error {
	if _, err := fs.Stat(distFS, "index.html"); err != nil {
		return err
	}

	indexHTML, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		return err
	}

	fileSystem := http.FS(distFS)

	router.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}

		requestPath := cleanFrontendRequestPath(c.Request.URL.Path)
		if isReservedFrontendPath(requestPath) {
			c.Status(http.StatusNotFound)
			return
		}

		if requestPath == "index.html" {
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
			return
		}

		if fileInfo, err := fs.Stat(distFS, requestPath); err == nil && !fileInfo.IsDir() {
			c.FileFromFS(requestPath, fileSystem)
			return
		}

		if shouldFallbackToIndex(requestPath) {
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
			return
		}

		c.Status(http.StatusNotFound)
	})

	return nil
}

func cleanFrontendRequestPath(requestPath string) string {
	cleaned := strings.TrimPrefix(path.Clean(requestPath), "/")
	if cleaned == "." || cleaned == "" {
		return "index.html"
	}

	return cleaned
}

func isReservedFrontendPath(requestPath string) bool {
	return requestPath == "api" ||
		strings.HasPrefix(requestPath, "api/") ||
		requestPath == "docs" ||
		strings.HasPrefix(requestPath, "docs/") ||
		requestPath == "schemas" ||
		strings.HasPrefix(requestPath, "schemas/") ||
		requestPath == "openapi" ||
		strings.HasPrefix(requestPath, "openapi.") ||
		strings.HasPrefix(requestPath, "openapi-")
}

func shouldFallbackToIndex(requestPath string) bool {
	if strings.HasPrefix(requestPath, "assets/") {
		return false
	}

	return path.Ext(requestPath) == ""
}
```

ここで重要なのは、API / docs / OpenAPI を SPA fallback しないことです。

もし `/api/v1/session` を `index.html` で返してしまうと、frontend から見ると API error が HTML になり、問題の切り分けが難しくなります。reserved path は明示的に `404` にして、Huma / Gin 側の route にだけ処理させます。

同じ理由で、存在しない `/assets/*` や拡張子付き path も `index.html` へ fallback しません。missing JavaScript / CSS / SVG に HTML を返すと、browser 側では MIME mismatch になり原因が追いにくくなります。

### 2-2. embed ありの実装を書く

#### ファイル: `backend/frontend_embed.go`

```go
//go:build embed_frontend

package backend

import (
	"embed"
	"io/fs"
)

//go:embed web/dist
var embeddedFrontend embed.FS

func frontendDistFS() (fs.FS, error) {
	return fs.Sub(embeddedFrontend, "web/dist")
}
```

このファイルは `-tags embed_frontend` を付けた build でだけ使われます。

`//go:embed web/dist` は build 時点で `backend/web/dist/` が存在している必要があります。つまり、embedded binary を作る前に必ず frontend build を実行します。

```bash
npm --prefix frontend run build
CGO_ENABLED=0 go build -buildvcs=false -trimpath -tags "embed_frontend nomsgpack" -ldflags "-s -w -buildid=" -o ./bin/haohao ./backend/cmd/main
```

### 2-3. embed なしの stub を書く

#### ファイル: `backend/frontend_stub.go`

```go
//go:build !embed_frontend

package backend

import "io/fs"

func frontendDistFS() (fs.FS, error) {
	return nil, ErrFrontendNotEmbedded
}
```

この stub により、build tag なしの通常 build でも `RegisterFrontendRoutes` 自体は compile できます。

通常 build では `RegisterFrontendRoutes` が `ErrFrontendNotEmbedded` を返すため、`cmd/main` 側では fatal にせず log に留めます。これにより、開発中の backend 単体起動や OpenAPI export は frontend dist に依存しません。

### 2-4. route behavior の test を追加する

#### ファイル: `backend/frontend_test.go`

`testing/fstest.MapFS` で `index.html` と `assets/app.js` だけを持つ小さな FS を作り、次を確認します。

- `/`, `/integrations` は `index.html` を返す
- `/assets/app.js` は静的 asset として返す
- `/assets/missing.js` は `index.html` ではなく `404` を返す
- `/api/v1/session`, `/docs`, `/openapi.yaml` は SPA fallback せず `404` を返す
- `/openapi-3.0.yaml` も SPA fallback せず `404` を返す
- `POST /login` は SPA fallback せず `404` を返す

これにより、frontend dist が無い通常 test でも SPA fallback の条件を検証できます。

## Step 3. 単一バイナリ用の runtime config を追加する

単一バイナリは `./haohao` だけで起動されることが多いため、`make backend-dev` のように shell 側で `.env` を `source` してくれるとは限りません。

この repository では、backend の config loader に `.env` の任意読み込みを追加します。

読み込み候補は次です。

- カレントディレクトリの `.env`
- 実行ファイルと同じ directory の `.env`

既に shell / Docker / Kubernetes などで設定されている環境変数は `.env` で上書きしません。これにより、local では `bin/.env` を置いて `./haohao` でき、本番では外部から注入した環境変数を優先できます。

### 3-1. `.env` loader を追加する

#### ファイル: `backend/internal/config/dotenv.go`

実装の要点は次です。

- `.env` が無ければ何もしない
- `KEY=value`, `KEY="quoted value"`, `export KEY=value` を読む
- `#` comment と空行を無視する
- 既存の `os.LookupEnv(key)` がある場合は `os.Setenv` しない

`config.Load()` の先頭でこの loader を呼びます。

#### ファイル: `backend/internal/config/config.go`

```go
func Load() (Config, error) {
	if err := loadDotEnvFiles(); err != nil {
		return Config{}, err
	}

	// 既存の duration parse と Config 構築に続く
}
```

#### ファイル: `backend/internal/config/dotenv_test.go`

test では次を確認します。

- カレントディレクトリの `.env` が読まれる
- 既存の環境変数は `.env` で上書きされない
- quote、空白を含む値、inline comment を parse できる

### 3-2. embedded frontend 用の URL 補正を追加する

開発時の `.env` には、Vite dev server 用に次の値が入ることがあります。

```dotenv
FRONTEND_BASE_URL=http://127.0.0.1:5173
ZITADEL_POST_LOGOUT_REDIRECT_URI=http://127.0.0.1:5173/login
```

単一バイナリでは Vite dev server を使わないため、この値のままだと Zitadel callback や logout 後に `127.0.0.1:5173` へ移動して `ERR_CONNECTION_REFUSED` になります。

embedded frontend build では、dev 用 `5173` URL が残っている場合だけ `APP_BASE_URL` 側へ補正します。明示的な production URL、たとえば `https://app.example.com` は維持します。

#### ファイル: `backend/internal/config/frontend_mode.go`

```go
//go:build embed_frontend

package config

const frontendEmbedded = true
```

#### ファイル: `backend/internal/config/frontend_mode_stub.go`

```go
//go:build !embed_frontend

package config

const frontendEmbedded = false
```

#### ファイル: `backend/internal/config/frontend_url.go`

このファイルでは次を実装します。

- build tag なしの default `FRONTEND_BASE_URL` は従来通り `http://127.0.0.1:5173`
- embedded build の default `FRONTEND_BASE_URL` は `APP_BASE_URL`
- embedded build で `FRONTEND_BASE_URL` が `http://127.0.0.1:5173` または `http://localhost:5173` なら `APP_BASE_URL` に補正
- embedded build で `ZITADEL_POST_LOGOUT_REDIRECT_URI` が `5173/login` なら `FRONTEND_BASE_URL + "/login"` に補正

#### ファイル: `backend/internal/config/frontend_url_test.go`

test では次を確認します。

- build tag なしの default は Vite dev server
- embedded build の default は `APP_BASE_URL`
- embedded build では dev 用 `5173` URL を補正する
- explicit production URL は補正せず維持する

## Step 4. `cmd/main` に SPA 配信を接続する

`app.New(...)` で router を作った直後に、frontend route を登録します。

#### ファイル: `backend/cmd/main/main.go`

import に root backend package を追加します。

```go
import (
	// 既存 import は省略

	backendroot "example.com/haohao/backend"
)
```

`application := app.New(...)` の直後に追加します。

```go
application := app.New(cfg, sessionService, oidcLoginService, delegationService, provisioningService, authzService, machineClientService, bearerVerifier, m2mVerifier)
if err := backendroot.RegisterFrontendRoutes(application.Router); err != nil {
	log.Printf("frontend routes unavailable: %v", err)
}
```

この時点で、build tag なしの backend 起動では次のような log が出ます。

```text
frontend routes unavailable: frontend dist is not embedded
```

これは正常です。開発時は Vite dev server を使い、本番用 binary を作るときだけ frontend を embed します。

## Step 5. build 用 Makefile target を追加する

毎回コマンドを手で覚えなくてよいように、Makefile に入口を追加します。

#### ファイル: `Makefile`

```makefile
GO_BINARY_BUILD_FLAGS := -buildvcs=false -trimpath -tags "embed_frontend nomsgpack" -ldflags "-s -w -buildid="

frontend-build:
	cd frontend && npm run build

binary: frontend-build
	mkdir -p bin
	CGO_ENABLED=0 go build $(GO_BINARY_BUILD_FLAGS) -o ./bin/haohao ./backend/cmd/main

docker-build:
	docker build -t haohao:dev -f docker/Dockerfile .
```

これで、local binary build は次だけで実行できます。

```bash
make binary
```

Docker image build は次です。

```bash
make docker-build
```

## Step 6. binary で手動確認する

まず DB と Redis を起動し、migration と seed を流します。

```bash
make up
make db-up
make seed-demo-user
```

次に frontend build と embedded binary build を実行します。

```bash
npm --prefix frontend run build
mkdir -p bin
CGO_ENABLED=0 go build -buildvcs=false -trimpath -tags "embed_frontend nomsgpack" -ldflags "-s -w -buildid=" -o ./bin/haohao ./backend/cmd/main
```

`./bin/haohao` は、環境変数に加えて、カレントディレクトリの `.env` と実行ファイル横の `.env` を任意で読みます。既に設定されている環境変数は `.env` で上書きしません。

通常の backend port と衝突しないように `18080` で起動します。demo user の local password login まで確認する場合は、`.env` が `AUTH_MODE=zitadel` でもこの起動だけ `AUTH_MODE=local` に上書きします。

単一バイナリでは Vite dev server の `http://127.0.0.1:5173` を使いません。`FRONTEND_BASE_URL` と `ZITADEL_POST_LOGOUT_REDIRECT_URI` が dev 用の `5173` のままなら、embedded frontend build では `APP_BASE_URL` 側へ補正します。それでも、本番用 `.env` では次のように明示しておくのが安全です。

```dotenv
APP_BASE_URL=http://127.0.0.1:18080
FRONTEND_BASE_URL=http://127.0.0.1:18080
ZITADEL_REDIRECT_URI=http://127.0.0.1:18080/api/v1/auth/callback
ZITADEL_POST_LOGOUT_REDIRECT_URI=http://127.0.0.1:18080/login
```

```bash
set -a && source .env && set +a
HTTP_PORT=18080 \
APP_BASE_URL=http://127.0.0.1:18080 \
FRONTEND_BASE_URL=http://127.0.0.1:18080 \
AUTH_MODE=local \
ENABLE_LOCAL_PASSWORD_LOGIN=true \
./bin/haohao
```

binary と同じ directory に `.env` を置いて起動する場合は、次の形でも動きます。

```bash
cd bin
./haohao
```

この場合、`bin/.env` が読み込まれます。shell から渡した環境変数は引き続き `.env` より優先されます。

別 terminal で確認します。

```bash
curl -i http://127.0.0.1:18080/
curl -i http://127.0.0.1:18080/login
curl -i http://127.0.0.1:18080/integrations
curl -i http://127.0.0.1:18080/api/v1/session
curl -i http://127.0.0.1:18080/openapi.yaml
curl -i http://127.0.0.1:18080/assets/missing.js
curl -i 'http://127.0.0.1:18080/api/v1/auth/callback?error=forced'
```

期待する結果は次です。

- `/` は `text/html` で `index.html` を返す
- `/login` は SPA fallback として `index.html` を返す
- `/integrations` も SPA fallback として `index.html` を返す
- `/api/v1/session` は API response を返す。未ログインなら JSON problem の `401`
- `/openapi.yaml` は OpenAPI YAML を返す
- `/assets/missing.js` は `index.html` ではなく `404`

local password login は次で確認します。

```bash
curl -i \
  -X POST http://127.0.0.1:18080/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"changeme123"}'
```

ここで `Set-Cookie` が返れば、embedded binary でも API は通常どおり動いています。

### 6-1. binary size を確認する

この repository では、サイズ優先の build flags を使ったあとで binary size も確認します。

```bash
ls -lh bin/haohao
wc -c bin/haohao
go version -m bin/haohao | grep -E 'build|vcs'
```

このセッションの `darwin/arm64` build では、`bin/haohao` は `15,035,506 bytes`、`ls -lh` 表示では約 `14M` でした。変更前の debug 情報付き binary は約 `21M` だったため、`nomsgpack`, `-trimpath`, `-ldflags "-s -w -buildid="`, `-buildvcs=false` でおよそ 6 MB 減っています。

`go version -m` に `vcs.revision`, `vcs.time`, `vcs.modified` が出ていなければ、`-buildvcs=false` が効いています。

## Step 7. Dockerfile を追加する

次に、frontend build と embedded Go binary build を Docker image 内で完結させます。

### 7-1. Dockerfile を作る

#### ファイル: `docker/Dockerfile`

```dockerfile
FROM node:24 AS frontend-builder
WORKDIR /app
COPY frontend/package*.json ./frontend/
RUN npm --prefix frontend ci
COPY frontend ./frontend
RUN npm --prefix frontend run build

FROM golang:1.26 AS backend-builder
WORKDIR /app
COPY go.work ./
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download
COPY backend ./backend
COPY --from=frontend-builder /app/backend/web/dist ./backend/web/dist
RUN GOMEMLIMIT=512MiB GOGC=25 CGO_ENABLED=0 go build -p 1 -buildvcs=false -trimpath -tags "embed_frontend nomsgpack" -ldflags "-s -w -buildid=" -o /tmp/haohao ./backend/cmd/main

FROM scratch
COPY --from=backend-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=backend-builder /tmp/haohao /haohao
USER 65532:65532
EXPOSE 8080
CMD ["/haohao"]
```

この Dockerfile では、frontend build output を source tree から持ち込まず、container build の中で生成します。

そのため、local の `backend/web/dist/` が空でも Docker build は成立します。

`nomsgpack` は Gin の未使用 msgpack binding を外す build tag です。`-buildvcs=false`, `-trimpath`, `-ldflags "-s -w -buildid="` は VCS metadata、path 情報、symbol table、DWARF debug 情報、build id を削って binary size を下げます。Docker Desktop などの小さめの builder では `github.com/ugorji/go/codec` の compile が `signal: killed` になりやすいため、`-p 1`, `GOMEMLIMIT`, `GOGC` も指定して低メモリ環境で安定させます。

runtime stage は `scratch` にして、TLS 検証に必要な CA bundle だけを builder からコピーします。OIDC / OAuth 連携で HTTPS outbound access が必要なため、CA bundle は残します。

runtime image には shell も package manager も入りません。container の中に入って調査したい場合は、この production image ではなく、別途 debug 用 image を作るか builder stage を使って確認します。

### 7-2. `.dockerignore` を作る

#### ファイル: `.dockerignore`

```dockerignore
.git
.github
.DS_Store
.env
.local
cookies.txt
*.log
bin

frontend/node_modules
frontend/dist
frontend/openapi-ts-error-*.log

backend/web/dist
```

`backend/web/dist` は Docker build context から除外します。Docker image 内では `frontend-builder` stage が dist を作り、`backend-builder` stage がそれを受け取ります。

### 7-3. Docker image を build する

```bash
docker build -t haohao:dev -f docker/Dockerfile .
```

build 後に image size と layer breakdown を確認します。

```bash
docker image ls haohao:dev --format 'table {{.Repository}}:{{.Tag}}\t{{.Size}}\t{{.ID}}'
docker history haohao:dev --no-trunc --format 'table {{.Size}}\t{{.CreatedBy}}' | sed -n '1,8p'
```

このセッションでは、`docker image ls` の表示は `20MB` でした。`docker history` では実体の layer は `/haohao` が `14.6MB`、CA bundle が `242kB` で、残りの `CMD` / `EXPOSE` / `USER` は `0B` でした。

Docker の size 表示は Docker Desktop / builder / platform によって丸めや圧縮後サイズの見え方が異なります。ここで見るべきポイントは、runtime image が base OS を含まず、binary layer と CA bundle layer だけになっていることです。

compose の PostgreSQL / Redis に接続して起動する場合は、まず依存 service を起動します。

```bash
make up
```

既定の compose project 名なら network は `haohao_default` です。違う場合は次で確認します。

```bash
docker network ls
```

backend container を起動します。

```bash
docker run --rm \
  --name haohao-app \
  --network haohao_default \
  -p 18081:8080 \
  -e APP_NAME='HaoHao API' \
  -e APP_VERSION='0.1.0' \
  -e HTTP_PORT='8080' \
  -e APP_BASE_URL='http://127.0.0.1:18081' \
  -e FRONTEND_BASE_URL='http://127.0.0.1:18081' \
  -e DATABASE_URL='postgres://haohao:haohao@postgres:5432/haohao?sslmode=disable' \
  -e REDIS_ADDR='redis:6379' \
  -e REDIS_PASSWORD='' \
  -e REDIS_DB='0' \
  -e AUTH_MODE='local' \
  -e ENABLE_LOCAL_PASSWORD_LOGIN='true' \
  -e SESSION_TTL='24h' \
  -e COOKIE_SECURE='false' \
  haohao:dev
```

別 terminal で確認します。

```bash
curl -i http://127.0.0.1:18081/
curl -i http://127.0.0.1:18081/login
curl -i http://127.0.0.1:18081/api/v1/session
curl -i http://127.0.0.1:18081/openapi.yaml
curl -i \
  -X POST http://127.0.0.1:18081/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"changeme123"}'
```

確認後は container を停止します。

```bash
docker rm -f haohao-app
```

## Step 8. CI を更新する

現在の CI は生成物 drift、backend test、frontend build、DB schema drift、OpenAPI validate、Zitadel compose config を確認しています。

単一バイナリ配信を CI に載せるには、次を追加します。

- Go version を `go.work` に合わせて `1.26.0` にする
- frontend build 後に embedded binary build を実行する
- Docker build を実行する

#### ファイル: `.github/workflows/ci.yml`

全体例は次です。既存の branch 設定は維持しています。

```yaml
name: CI

on:
  pull_request:
  push:
    branches:
      - main
      - handmaid2

jobs:
  verify:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:18
        env:
          POSTGRES_DB: haohao
          POSTGRES_USER: haohao
          POSTGRES_PASSWORD: haohao
        ports:
          - 5432:5432
        options: >-
          --health-cmd "pg_isready -U haohao -d haohao"
          --health-interval 5s
          --health-timeout 5s
          --health-retries 20
    env:
      DATABASE_URL: postgres://haohao:haohao@127.0.0.1:5432/haohao?sslmode=disable
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.26.0"

      - uses: actions/setup-node@v4
        with:
          node-version: "24"
          cache: npm
          cache-dependency-path: frontend/package-lock.json

      - name: Install tools
        run: |
          sudo apt-get update
          sudo apt-get install -y postgresql-client
          go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.0
          go install github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.3

      - name: Install frontend dependencies
        run: npm --prefix frontend ci

      - name: Go tests
        run: go test ./backend/...

      - name: Frontend build
        run: npm --prefix frontend run build

      - name: Embedded binary build
        run: CGO_ENABLED=0 go build -buildvcs=false -trimpath -tags "embed_frontend nomsgpack" -ldflags "-s -w -buildid=" -o ./bin/haohao ./backend/cmd/main

      - name: Generated drift
        run: |
          make gen
          git diff --exit-code -- openapi/openapi.yaml frontend/src/api/generated backend/internal/db

      - name: DB schema drift
        run: |
          migrate -path db/migrations -database "$DATABASE_URL" up
          PGPASSWORD=haohao pg_dump --schema-only --no-owner --no-privileges -h 127.0.0.1 -U haohao -d haohao \
            | sed '/^\\restrict /d; /^\\unrestrict /d' \
            | perl -0pe 's/\n+\z/\n/' > db/schema.sql
          git diff --exit-code -- db/schema.sql

      - name: OpenAPI validate
        run: |
          test -s openapi/openapi.yaml
          grep -Eq '^openapi: "?(3\.1|3\.1\.)' openapi/openapi.yaml

      - name: Zitadel compose config
        run: docker compose --env-file dev/zitadel/.env.example -f dev/zitadel/docker-compose.yml config --quiet

      - name: Docker build
        run: docker build -t haohao:ci -f docker/Dockerfile .

      - name: Whitespace check
        run: git diff --check
```

ここで `go test ./backend/...` は build tag なしで実行します。これは frontend dist が無くても backend の通常テストが壊れないことを確認するためです。

一方、`Embedded binary build` は frontend build 後に `CGO_ENABLED=0`, `-buildvcs=false`, `-trimpath`, `-tags "embed_frontend nomsgpack"`, `-ldflags "-s -w -buildid="` 付きで実行します。これは production binary が実際に作れることと、CI / release / local build の artifact 特性がずれないことを確認するためです。

## Step 9. release workflow を更新する

release workflow は、tag push 時に OpenAPI artifact だけでなく embedded binary も release asset として upload します。

#### ファイル: `.github/workflows/release.yml`

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release-assets:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.26.0"

      - uses: actions/setup-node@v4
        with:
          node-version: "24"
          cache: npm
          cache-dependency-path: frontend/package-lock.json

      - name: Install tools
        run: go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.0

      - name: Install frontend dependencies
        run: npm --prefix frontend ci

      - name: Generate OpenAPI and SDK
        run: make gen

      - name: Validate OpenAPI artifact
        run: |
          test -s openapi/openapi.yaml
          grep -Eq '^openapi: "?(3\.1|3\.1\.)' openapi/openapi.yaml

      - name: Build frontend bundle
        run: npm --prefix frontend run build

      - name: Build embedded binary
        run: |
          mkdir -p dist
          CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath -tags "embed_frontend nomsgpack" -ldflags "-s -w -buildid=" -o dist/haohao-linux-amd64 ./backend/cmd/main
          tar -C dist -czf dist/haohao-linux-amd64.tar.gz haohao-linux-amd64

      - name: Upload release assets
        uses: softprops/action-gh-release@v2
        with:
          files: |
            openapi/openapi.yaml
            dist/haohao-linux-amd64.tar.gz
```

最初は `linux/amd64` だけで十分です。複数 OS / architecture に広げる場合は、release job を matrix 化します。

## 途中で迷わないための判断基準

### API route は SPA fallback しない

`/api/*`, `/docs`, `/openapi.yaml`, `/openapi.json`, `/openapi-3.0.yaml`, `/openapi-3.0.json`, `/schemas/*` は frontend route として扱いません。

存在しない API path は `index.html` ではなく `404` にします。API client が HTML を JSON として読もうとする状態を避けるためです。

また、存在しない `/assets/*` や拡張子付き path も `404` にします。SPA route として fallback するのは `/login` のような拡張子を持たない frontend route だけです。

### 単一バイナリでは frontend URL を backend URL に寄せる

開発時は Vite dev server の `http://127.0.0.1:5173` を frontend として使います。一方、単一バイナリでは Go process が frontend も返すため、frontend URL は `APP_BASE_URL` と同じ origin になります。

そのため、本番形の `.env` では次をそろえます。

```dotenv
APP_BASE_URL=https://app.example.com
FRONTEND_BASE_URL=https://app.example.com
ZITADEL_REDIRECT_URI=https://app.example.com/api/v1/auth/callback
ZITADEL_POST_LOGOUT_REDIRECT_URI=https://app.example.com/login
```

local smoke test なら host と port を合わせて `http://127.0.0.1:18080` のようにします。Zitadel 側に登録する redirect URI も同じ値に合わせます。

embedded build では dev 用 `5173` が残っている場合に限り `APP_BASE_URL` 側へ補正しますが、production deployment では `.env` / secret manager 側でも明示的にそろえるのが安全です。

### dist は commit しない

`backend/web/dist/` は build artifact です。

commit するものは次です。

- `backend/frontend.go`
- `backend/frontend_embed.go`
- `backend/frontend_stub.go`
- `backend/frontend_test.go`
- `backend/internal/config/config.go`
- `backend/internal/config/dotenv.go`
- `backend/internal/config/dotenv_test.go`
- `backend/internal/config/frontend_mode.go`
- `backend/internal/config/frontend_mode_stub.go`
- `backend/internal/config/frontend_url.go`
- `backend/internal/config/frontend_url_test.go`
- `backend/cmd/main/main.go`
- `Makefile`
- `docker/Dockerfile`
- `.dockerignore`
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`

commit しないものは次です。

- `backend/web/dist/*`
- `bin/haohao`
- Docker image
- release asset

### build tag なしの開発体験を壊さない

開発時は引き続き次の分離起動を使います。

```bash
make backend-dev
make frontend-dev
```

このとき frontend は Vite dev server が返します。Go backend は API / docs / OpenAPI に集中します。

本番形の確認だけ、次を使います。

```bash
make binary
./bin/haohao
```

### size reduction の限界を把握する

この手順では、Go 標準 toolchain の範囲で binary size を下げます。

- `nomsgpack`: Gin の未使用 msgpack binding を外す
- `CGO_ENABLED=0`: static binary にして `scratch` image で動かす
- `-buildvcs=false`: VCS metadata を埋め込まない
- `-trimpath`: local path を埋め込まない
- `-ldflags "-s -w -buildid="`: symbol table、DWARF debug 情報、build id を削る
- `scratch`: runtime image から OS layer をなくす

ここからさらに小さくするなら UPX のような binary packer が選択肢になりますが、起動時間、脆弱性 scanner、debuggability、環境による実行差の tradeoff が出ます。そのため、この手順では UPX は使いません。

## よくある詰まりどころ

### `DATABASE_URL is required` が出る

古い binary か、`.env` の場所が違う可能性があります。

この手順の実装後は、`./haohao` は次の順で `.env` を探します。

- 起動したカレントディレクトリの `.env`
- 実行ファイルと同じ directory の `.env`

たとえば `bin/haohao` と `bin/.env` を並べる場合は、次で起動できます。

```bash
cd bin
./haohao
```

既に shell から渡した環境変数は `.env` より優先されます。binary を更新していない場合は、先に `make binary` を実行します。

### login 後に `http://127.0.0.1:5173/` へ飛ぶ

`FRONTEND_BASE_URL` または `ZITADEL_POST_LOGOUT_REDIRECT_URI` が Vite dev server の値のままです。単一バイナリでは frontend も Go process が返すため、`APP_BASE_URL` と同じ origin にします。

```dotenv
APP_BASE_URL=http://127.0.0.1:8080
FRONTEND_BASE_URL=http://127.0.0.1:8080
ZITADEL_REDIRECT_URI=http://127.0.0.1:8080/api/v1/auth/callback
ZITADEL_POST_LOGOUT_REDIRECT_URI=http://127.0.0.1:8080/login
```

この手順の embedded build では、dev 用 `5173` が残っていれば `APP_BASE_URL` に補正します。それでもブラウザが 5173 へ移動する場合は、古い binary を起動している可能性があります。`make binary` 後に `bin/haohao` を差し替えてください。

redirect 先だけを軽く確認するには次を実行します。

```bash
curl -i 'http://127.0.0.1:8080/api/v1/auth/callback?error=forced'
```

`Location` が `http://127.0.0.1:8080/login?error=oidc_callback_failed` のように `APP_BASE_URL` 側なら正常です。

## 生成物として扱うファイル

この手順で増える生成物は次です。

- `backend/web/dist/*`
- `bin/haohao`
- Docker image `haohao:dev`
- GitHub Release asset `haohao-linux-amd64.tar.gz`

これらは source ではありません。問題があれば、生成物を直接直さず、次を直します。

- frontend の表示が違う: `frontend/src/*`
- frontend build output の場所が違う: `frontend/vite.config.ts`
- SPA fallback が違う: `backend/frontend.go`
- `.env` が読まれない: `backend/internal/config/dotenv.go`
- login callback / logout 後に `5173` へ飛ぶ: `backend/internal/config/frontend_url.go` または `.env` の `FRONTEND_BASE_URL`
- binary build が壊れる: `backend/frontend_embed.go` / `backend/cmd/main/main.go`
- Docker image が壊れる: `docker/Dockerfile`
- CI がずれる: `.github/workflows/ci.yml`

## 最終確認コマンド

local で最低限流す確認は次です。

```bash
go test ./backend/...
go test -tags embed_frontend ./backend/internal/config
npm --prefix frontend run build
CGO_ENABLED=0 go build -buildvcs=false -trimpath -tags "embed_frontend nomsgpack" -ldflags "-s -w -buildid=" -o ./bin/haohao ./backend/cmd/main
docker build -t haohao:dev -f docker/Dockerfile .
ls -lh bin/haohao
docker image ls haohao:dev
```

binary の smoke test は次です。

```bash
set -a && source .env && set +a
HTTP_PORT=18080 \
APP_BASE_URL=http://127.0.0.1:18080 \
FRONTEND_BASE_URL=http://127.0.0.1:18080 \
AUTH_MODE=local \
ENABLE_LOCAL_PASSWORD_LOGIN=true \
./bin/haohao
```

`./bin/haohao` は `.env` を自動で読むため、手元の `.env` の値をそのまま使うだけなら `source .env` は不要です。ここでは smoke test 用に port と auth mode を一時上書きするため、明示的に環境変数を渡しています。

別 terminal で実行します。

```bash
curl -i http://127.0.0.1:18080/
curl -i http://127.0.0.1:18080/login
curl -i http://127.0.0.1:18080/integrations
curl -i http://127.0.0.1:18080/api/v1/session
curl -i http://127.0.0.1:18080/openapi.yaml
curl -i http://127.0.0.1:18080/assets/missing.js
curl -i 'http://127.0.0.1:18080/api/v1/auth/callback?error=forced'
```

期待結果:

- frontend route は HTML を返す
- API route は JSON または problem response を返し、HTML fallback しない
- OpenAPI / docs は既存の Huma route として残る
- 存在しない asset は HTML fallback ではなく `404` になる
- OIDC callback failure redirect は `127.0.0.1:5173` ではなく `APP_BASE_URL` 側の `/login?error=oidc_callback_failed` へ戻る
- binary / Docker image は size reduction flags と `scratch` runtime で作られている
- CI では dist 不在の default Go test と、dist ありの embedded binary build の両方を検証する

## ここまでで何ができているか

ここまで終えると、HaoHao は `CONCEPT.md` の単一バイナリ配信に到達します。

- 開発時は backend と frontend を分離起動できる
- 本番形では Go binary 1 本で API / docs / OpenAPI / SPA を返せる
- Docker image も binary 1 本を実行するだけになる
- 配布 artifact は stripped static binary と `scratch` runtime image になる
- CI は generated artifact drift と production binary build の両方を検知できる
- release には OpenAPI artifact と embedded binary asset を載せられる

この次に進むなら、`IMPL.md` に残っている `ProvisioningReconcileJob` の scheduler 接続、tenant selector UI、machine client admin UI を順番に埋めていきます。
