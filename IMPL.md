# HaoHao 実装状況

調査日: 2026-04-25

対象:

- `CONCEPT.md`
- `TUTORIAL.md`
- `TUTORIAL_ZITADEL.md`
- `TUTORIAL_SINGLE_BINARY.md`
- `TUTORIAL_P0_OPERABILITY.md`
- `TUTORIAL_P1_ADMIN_UI.md`
- `TUTORIAL_P2_TODO.md`
- `RUNBOOK_OPERABILITY.md`
- 現在の repository 実装

## 全体像

現在の実装は、`CONCEPT.md` の基本方針である **OpenAPI 3.1 優先 + Monorepo + Go/Huma + Vue + PostgreSQL/sqlc + BFF Cookie 認証** をかなり広い範囲まで反映している。

`TUTORIAL.md` の foundation は、local password login / Cookie session / OpenAPI 生成 / frontend generated SDK 連携まで実装済み。さらに `TUTORIAL_ZITADEL.md` の Phase 1-6、`TUTORIAL_SINGLE_BINARY.md`、`TUTORIAL_P0_OPERABILITY.md`、`TUTORIAL_P1_ADMIN_UI.md`、`TUTORIAL_P2_TODO.md` まで進んでおり、Zitadel browser login、bearer API、delegated auth、SCIM、tenant-aware auth、M2M、単一バイナリ配信、運用確認、管理 UI、tenant 共有 TODO が存在する。

単一バイナリで SPA を配信する部分、Dockerfile、CI、release asset、P0 operability、P1 admin UI、P2 TODO 縦切りも追加済み。現時点で大きく残るのは、tenant 自体の CRUD 管理 UI、TODO より具体的な業務ドメインの拡張、metrics / tracing などの本番 observability の拡張。

## 方針との対応

| 項目 | 現状 |
| --- | --- |
| Monorepo | 実装済み。repo root に `go.work`、`frontend/`、`backend/`、`db/`、`openapi/` がある |
| Backend | Go module は `backend/`。Gin + Huma で API を構成 |
| OpenAPI | Huma から `openapi/openapi.yaml` を生成。OpenAPI は `3.1.0` |
| Frontend | Vue 3 + Vite + TypeScript + Pinia + Vue Router |
| Generated SDK | `@hey-api/openapi-ts` で `frontend/src/api/generated/` を生成 |
| DB | PostgreSQL 18 前提。migration + `db/schema.sql` + sqlc |
| Auth | local password login と Zitadel OIDC login の両対応 |
| Session | Redis に session / CSRF / OIDC state / delegation state を保存 |
| Single binary | 実装済み。frontend build output を `backend/web/dist/` に出し、`embed_frontend` build tag で Go binary に embed する |
| Operability | 実装済み。structured request logging、request id、health/readiness、SCIM reconcile scheduler、smoke script、runbook がある |
| Admin UI | 実装済み。tenant selector、integrations UX、machine client admin UI、docs access check がある |
| Business domain | 実装済み。tenant-aware TODO の schema / API / frontend があり、active tenant と tenant role `todo_user` で認可する |
| Docker / CI / release | 実装済み。`docker/Dockerfile`、`.dockerignore`、CI の embedded binary / Docker build、release asset upload がある |

## 開発基盤

実装済み:

- `go.work` は Go `1.26.0`、`use ./backend`。
- `compose.yaml` は PostgreSQL `18` と Redis `7.4` を起動する。
- `Makefile` に `up`, `down`, `db-up`, `db-down`, `db-schema`, `seed-demo-user`, `sqlc`, `openapi`, `gen`, `backend-dev`, `frontend-dev`, `frontend-build`, `binary`, `docker-build`, `smoke-operability` がある。
- `scripts/gen.sh` は `sqlc generate`、OpenAPI export、frontend SDK 生成をまとめて実行する。
- `.env.example` に local / Zitadel / external bearer / M2M / downstream delegated auth / SCIM / readiness / reconcile scheduler / cookie / docs auth の設定がそろっている。
- `dev/zitadel/` に self-hosted dev 用 Zitadel compose と `.env.example` がある。`make zitadel-up` 系の入口もある。
- `docker/Dockerfile` は Node builder、Go builder、`scratch` runtime の multi-stage build。
- `.github/workflows/ci.yml` は backend test、frontend build、embedded binary build、generated drift、DB schema drift、OpenAPI validate、Zitadel compose config、Docker build を確認する。
- CI は `bash -n scripts/smoke-operability.sh` で operability smoke script の syntax も確認する。
- `.github/workflows/release.yml` は OpenAPI artifact と embedded Linux amd64 binary tarball を GitHub Release に upload する。
- `scripts/smoke-operability.sh` と `make smoke-operability` があり、既存 server に対して `/healthz`, `/readyz`, `/api/v1/session`, `/openapi.yaml`, forced OIDC callback redirect を確認する。
- `RUNBOOK_OPERABILITY.md` に binary deploy、Docker deploy、rollback、Zitadel redirect URI、smoke test の手順がある。
- `scripts/seed-demo-user.sql` は `demo@example.com` に加え、Acme / Beta tenant、tenant role `docs_reader` / `todo_user`、global role `machine_client_admin` を idempotent に準備する。

注意点:

- backend 本体は環境変数を読み、補助として `.env` も任意で読み込む。読み込み候補はカレントディレクトリの `.env` と実行ファイル横の `.env`。
- 既に設定されている環境変数は `.env` で上書きしない。Docker/Kubernetes や shell から渡した値が優先される。
- `make backend-dev` は引き続き `.env` を source してから起動するため、従来の開発起動も動く。
- `make smoke-operability` は server を起動しない。既に動いている `BASE_URL`、既定では `http://127.0.0.1:8080`、に対して確認する。
- `dev/zitadel/.env` と root `.env` は実ファイルが存在するが、秘密値を含み得るため実装ドキュメントでは値を前提にしない。

## Database / sqlc

migration は `0001` から `0008` まである。

| migration | 内容 |
| --- | --- |
| `0001_init` | `pgcrypto`、`users`。`public_id UUID DEFAULT uuidv7()`、local password 用 `password_hash` |
| `0002_user_identities` | `user_identities`、外部 IdP identity。`password_hash` nullable 化 |
| `0003_roles` | `roles`, `user_roles`。初期 role は `docs_reader`, `external_api_user`, `todo_user` |
| `0004_downstream_grants` | delegated auth 用 `oauth_user_grants` |
| `0005_provisioning` | `deactivated_at`、SCIM/provisioning 用 identity columns、`provisioning_sync_state` |
| `0006_org_tenants` | `tenants`, `tenant_memberships`, `tenant_role_overrides`、user default tenant、grant tenant 化 |
| `0007_machine_clients` | `machine_clients` と `machine_client_admin` role |
| `0008_todos` | tenant 共有 TODO。`todos.public_id`, `tenant_id`, `created_by_user_id`, `title`, `completed`, timestamps と tenant scoped index |

sqlc:

- `backend/sqlc.yaml` は `db/schema.sql` と `db/queries/` を入力にする。
- 生成先は `backend/internal/db/`。
- `uuid` は `github.com/google/uuid.UUID` に override されている。
- `db/queries/` は users, identities, roles, tenants, downstream grants, provisioning, machine clients, todos を持つ。

注意点:

- `db/schema.sql` は migration 由来の snapshot として扱う前提。
- TODO は tenant 共有の最初の業務 table。list / get / update / delete は必ず `tenant_id` で絞り、tenant 外 TODO の存在を `404` として隠す。

## Backend

### 構成

主要な構成:

- `backend/cmd/main/main.go`: runtime 起動、PostgreSQL/Redis 接続、service wiring、HTTP server。
- `backend/cmd/openapi/main.go`: Huma API から OpenAPI YAML を出力。
- `backend/internal/app/app.go`: Gin engine、middleware、Huma config、security schemes、route registration。
- `backend/internal/api/`: Huma operation と request / response model。
- `backend/internal/service/`: session, OIDC login, identity, authz, delegation, provisioning, machine client, TODO。
- `backend/internal/auth/`: Cookie、Redis stores、OIDC/OAuth client、JWT bearer verifier、M2M verifier、refresh token encryption。
- `backend/internal/middleware/`: docs auth、external CORS/auth、SCIM auth、M2M auth。
- `backend/internal/config/dotenv.go`: `.env` の任意読み込み。カレントディレクトリと実行ファイル横の `.env` を読み、既存環境変数は上書きしない。
- `backend/internal/config/frontend_url.go`: embedded frontend build では dev 用 `http://127.0.0.1:5173` / `http://localhost:5173` の frontend URL と post logout URL を `APP_BASE_URL` 側へ補正する。
- `backend/internal/platform/logger.go`: `log/slog` ベースの logger。`LOG_LEVEL` と `LOG_FORMAT` で制御する。
- `backend/internal/platform/readiness.go`: PostgreSQL / Redis / optional Zitadel discovery の readiness check helper。
- `backend/internal/middleware/request_id.go`: `X-Request-ID` の受け取り / 生成 / response header 設定。
- `backend/internal/middleware/request_logger.go`: request id 付き structured request logging。
- `backend/internal/app/health.go`: Gin route として `/healthz` / `/readyz` を登録する。
- `backend/internal/jobs/scheduler.go`: `ProvisioningReconcileJob.RunOnce(ctx)` を `time.Ticker` で interval 実行する scheduler。
- `backend/internal/service/todo_service.go`: tenant-aware TODO の list/create/update/delete と validation。
- `backend/internal/api/todos.go`: browser session + CSRF + active tenant role `todo_user` 前提の TODO Huma operation。
- `backend/frontend.go`: embedded frontend の static serving と SPA fallback。
- `backend/frontend_embed.go`: `embed_frontend` build tag 付きで `backend/web/dist` を `embed.FS` に埋め込む。
- `backend/frontend_stub.go`: build tag なしでは frontend 未埋め込みとして扱う。
- `backend/frontend_test.go`: SPA fallback、reserved path、missing asset の挙動をテストする。

### Operability

実装済み:

- `LOG_LEVEL` は `debug`, `info`, `warn`, `error` を扱う。
- `LOG_FORMAT` は `json` / `text` を扱い、既定は `json`。
- `gin.Logger()` は使わず、`RequestID()` と `RequestLogger(logger)` を middleware として登録する。
- request log は `request_id`, `method`, `path`, `status`, `latency_ms`, `client_ip`, `user_agent` を出す。
- `/healthz` は process liveness として常に `200 {"status":"ok"}` を返す。
- `/readyz` は PostgreSQL / Redis を ping し、`READINESS_CHECK_ZITADEL=true` の場合だけ Zitadel discovery も確認する。
- readiness timeout は `READINESS_TIMEOUT` で制御する。
- `ProvisioningReconcileJob` は `SCIM_RECONCILE_ENABLED=true` のとき scheduler から呼ばれる。
- scheduler は no-overlap、per-run timeout、shutdown context、run-on-startup を扱う。
- scheduler 設定は `SCIM_RECONCILE_INTERVAL`, `SCIM_RECONCILE_TIMEOUT`, `SCIM_RECONCILE_RUN_ON_STARTUP`。

注意点:

- `/healthz` / `/readyz` は Huma operation ではなく Gin route。OpenAPI には出ない。
- `READINESS_CHECK_ZITADEL=false` なら Zitadel が落ちていても DB / Redis が正常なら `/readyz` は通る。
- `SCIM_RECONCILE_ENABLED=false` が既定。local development では明示的に有効化しない限り reconcile は走らない。

### API surface

`openapi/openapi.yaml` に出ている主な endpoint:

- Browser/session:
  - `GET /api/v1/auth/settings`
  - `GET /api/v1/auth/login`
  - `GET /api/v1/auth/callback`
  - `POST /api/v1/login`
  - `GET /api/v1/session`
  - `GET /api/v1/csrf`
  - `POST /api/v1/session/refresh`
  - `POST /api/v1/logout`
- Tenant:
  - `GET /api/v1/tenants`
  - `POST /api/v1/session/tenant`
- Delegated auth integrations:
  - `GET /api/v1/integrations`
  - `GET /api/v1/integrations/{resourceServer}/connect`
  - `GET /api/v1/integrations/{resourceServer}/callback`
  - `POST /api/v1/integrations/{resourceServer}/verify`
  - `DELETE /api/v1/integrations/{resourceServer}/grant`
- External bearer:
  - `GET /api/external/v1/me`
- SCIM:
  - `/api/scim/v2/Users`
  - `/api/scim/v2/Users/{id}`
- Machine client management:
  - `GET /api/v1/machine-clients`
  - `POST /api/v1/machine-clients`
  - `GET /api/v1/machine-clients/{id}`
  - `PUT /api/v1/machine-clients/{id}`
  - `DELETE /api/v1/machine-clients/{id}`
- TODO:
  - `GET /api/v1/todos`
  - `POST /api/v1/todos`
  - `PATCH /api/v1/todos/{todoPublicId}`
  - `DELETE /api/v1/todos/{todoPublicId}`
- M2M:
  - `GET /api/m2m/v1/self`

OpenAPI には出ないが runtime に存在する endpoint:

- `GET /healthz`
- `GET /readyz`

### Browser auth / session

実装済み:

- local password login は `users.password_hash` と `crypt()` で検証する。
- `AUTH_MODE=zitadel` または `ENABLE_LOCAL_PASSWORD_LOGIN=false` の場合、password login は無効化される。
- session は Redis に保存する。
- `SESSION_ID` は HttpOnly Cookie。
- `XSRF-TOKEN` は frontend が読める Cookie。
- mutation 系 endpoint は `X-CSRF-Token` header を要求する。
- `GET /api/v1/csrf` で CSRF token を再発行できる。
- `POST /api/v1/session/refresh` で session ID と CSRF token を rotate できる。
- logout は local session を削除し、Zitadel mode では post logout URL を返す。

### Zitadel browser login

実装済み:

- `AUTH_MODE=zitadel` 時に `ZITADEL_ISSUER`, `ZITADEL_CLIENT_ID`, `ZITADEL_CLIENT_SECRET` が必須。
- OIDC discovery を使って provider を構成する。
- authorization code + PKCE + nonce を使う。
- login state は Redis に保存し、callback で consume する。
- ID token を検証し、userinfo から email / name / groups を取得する。
- local user は `(provider, subject)` を正として `user_identities` に紐付ける。
- email verified の既存 user がいれば identity を結び、なければ password なし user を作る。
- provider groups から global role と tenant membership を同期する。

### External bearer API

実装済み:

- `BearerVerifier` が OIDC discovery の JWKS を使って JWT 署名、issuer、audience、scope prefix を検証する。
- Zitadel role claim を `groups` / project role claim から取り出す。
- `/api/external/` は middleware で bearer token を検証する。
- `GET /api/external/v1/me` は bearer principal、local user、tenant context を返す。
- CORS は `EXTERNAL_ALLOWED_ORIGINS` に明示された origin のみ許可する。

注意点:

- external bearer API は `ZITADEL_ISSUER` が空だと verifier が構成されず、service unavailable になる。
- default の `EXTERNAL_REQUIRED_ROLE` は `external_api_user`。

### Downstream delegated auth

実装済み:

- refresh token は AES-GCM で暗号化して DB に保存する。
- encryption key は `DOWNSTREAM_TOKEN_ENCRYPTION_KEY`。
- delegated state は Redis に保存する。
- consent callback で refresh token を保存し、access token は backend 内で refresh して使う。
- refresh token revoke / invalid_grant / refresh token TTL の扱いがある。
- frontend に `/integrations` 画面があり、connect / verify / revoke を呼べる。

注意点:

- `DelegationService` は `AUTH_MODE=zitadel` かつ `DOWNSTREAM_TOKEN_ENCRYPTION_KEY` が設定されている場合だけ構成される。
- 現在の downstream resource は実装上 `zitadel` のみ。
- tenant-aware 化済みのため、integration 操作には active tenant が必要。

### Provisioning / SCIM / tenant

実装済み:

- SCIM subset として user create/list/get/replace/patch/delete がある。
- SCIM bearer は `SCIM_BEARER_AUDIENCE` と `SCIM_REQUIRED_SCOPE` で検証する。
- SCIM user は `user_identities.provider = 'scim'` と `external_id` を使って管理する。
- deactivation 時に user sessions と delegated grants を削除する。
- provider group grammar `tenant:<slug>:<role>` から tenant membership を同期する。
- browser session には active tenant を保存できる。
- `GET /api/v1/tenants` と `POST /api/v1/session/tenant` がある。
- tenant role override の DB と解決ロジックがある。
- `ProvisioningReconcileJob` は runtime scheduler に接続済み。

注意点:

- tenant selector UI はあるが、tenant 自体の CRUD 管理 UI は無い。tenant は provider claim / SCIM group から upsert される形。

### M2M

実装済み:

- `machine_clients` table と CRUD API がある。
- CRUD API は browser session + `machine_client_admin` role が必要。
- frontend に machine client admin UI がある。
- `/machine-clients` で一覧、`/machine-clients/new` で作成、`/machine-clients/:id` で detail / update / disable を扱う。
- role 不足時は 403 を raw JSON ではなく admin access denied UI として表示する。
- `/api/m2m/` は M2M middleware で bearer token を検証する。
- human user claim を持つ token は M2M として拒否する。
- token の `client_id` / `azp` を provider client ID として local `machine_clients` に照合する。
- allowed scopes と `M2M_REQUIRED_SCOPE_PREFIX` で scope を制限する。
- `GET /api/m2m/v1/self` が現在の machine client 情報を返す。

注意点:

- Zitadel 側の machine client/application 作成は別途 Console または手順に従う必要がある。
- machine client admin は tenant role ではなく global role `machine_client_admin` を見る。

### P2 TODO

実装済み:

- `todos` table と sqlc query がある。
- TODO は tenant 共有の業務データとして `tenant_id` を必須にする。
- 作成者追跡用に `created_by_user_id` を保持する。
- `TodoService` は HTTP / Cookie を知らず、API 層から認証済み user ID と active tenant ID を受け取る。
- `GET /api/v1/todos` は active tenant の TODO だけを返す。
- `POST /api/v1/todos` は active tenant に TODO を作成する。
- `PATCH /api/v1/todos/{todoPublicId}` は title / completed を更新する。
- `DELETE /api/v1/todos/{todoPublicId}` は active tenant の TODO を削除する。
- 未ログインは `401 application/problem+json`。
- active tenant が無い場合は `409 active tenant is required`。
- active tenant に tenant role `todo_user` が無い場合は `403 todo_user tenant role is required`。
- 空 title / whitespace-only title / 200 文字超過 title は `400 invalid todo title`。
- title と completed が両方未指定の patch は `400 invalid todo update`。
- tenant 外または存在しない TODO は `404 todo not found` として扱う。

注意点:

- `todo_user` は TODO API では global role としては見ない。active tenant の tenant role だけを見る。
- TODO API は browser session + Cookie + CSRF の縦切りに限定している。M2M / external bearer 用 TODO API は未実装。
- tenant 自体の CRUD 管理 UI は P2 には含めていない。

### Docs / OpenAPI

実装済み:

- Huma default docs / OpenAPI endpoint が有効。
- security schemes は `cookieAuth`, `bearerAuth`, `m2mBearerAuth`。
- `DOCS_AUTH_REQUIRED=true` で `/docs`, `/openapi.json`, `/openapi.yaml` を `docs_reader` role 付き browser session に制限できる。
- `backend/cmd/openapi` から `openapi/openapi.yaml` を再生成できる。

### Single binary / SPA 配信

実装済み:

- `npm --prefix frontend run build` は Vue production build を `backend/web/dist/` に出力する。
- `CGO_ENABLED=0 go build -buildvcs=false -trimpath -tags "embed_frontend nomsgpack" -ldflags "-s -w -buildid=" -o ./bin/haohao ./backend/cmd/main` で frontend embedded binary を作る。
- build tag なしの `go test ./backend/...` や `go run ./backend/cmd/openapi` は frontend dist 不在でも壊れない。
- `cmd/main` は router 作成後に `backend.RegisterFrontendRoutes(application.Router)` を呼ぶ。
- `/`, `/login`, `/integrations` は SPA fallback として `index.html` を返す。
- `/machine-clients`, `/machine-clients/new`, `/machine-clients/:id` も SPA fallback として `index.html` を返す。
- `/todos` も SPA fallback として `index.html` を返す。
- `/assets/*`, `/favicon.svg`, `/icons.svg` は frontend build artifact として返す。
- `/api/*`, `/docs`, `/schemas/*`, `/openapi.yaml`, `/openapi.json`, `/openapi-3.0.yaml`, `/openapi-3.0.json` は SPA fallback しない。
- 存在しない `/assets/*` や拡張子付き path は `index.html` ではなく `404` を返す。

build / size:

- production binary は `CGO_ENABLED=0`, `nomsgpack`, `-buildvcs=false`, `-trimpath`, `-ldflags "-s -w -buildid="` で作る。
- この環境の `darwin/arm64` binary は `15,035,506 bytes`。変更前の debug 情報付き binary は約 `21M`。
- `nomsgpack` により Gin の未使用 msgpack binding を外し、Docker build 中の `github.com/ugorji/go/codec` compile memory pressure も避ける。
- `-buildvcs=false` により `go version -m` に `vcs.revision`, `vcs.time`, `vcs.modified` が出ない。
- `bin/haohao` と同じ directory に `.env` を置いて `cd bin && ./haohao` した場合、その `.env` も読み込まれる。
- embedded build で `.env` に `FRONTEND_BASE_URL=http://127.0.0.1:5173` が残っていても、callback redirect は `APP_BASE_URL` に戻る。

Docker:

- `docker/Dockerfile` は frontend build と backend embedded binary build を container 内で完結させる。
- runtime stage は `scratch`。CA bundle と `/haohao` binary だけを含む。
- image は shell / package manager を持たないため、調査は debug image または builder stage で行う。
- この環境では `docker image ls haohao:dev` は `20MB` と表示された。
- `docker history` 上の実体 layer は `/haohao` が `14.6MB`、CA bundle が `242kB`。

## Frontend

実装済み:

- Vue 3 + Vite + TypeScript。
- Pinia store は session state、tenant state、machine client state、TODO state を管理する。
- Vue Router で `/`, `/login`, `/integrations`, `/todos`, `/machine-clients`, `/machine-clients/new`, `/machine-clients/:id` を持つ。
- generated SDK は `frontend/src/api/generated/`。
- `frontend/src/api/client.ts` が generated client の共通 transport 設定を持つ。
- `frontend/src/api/client.ts` は Problem JSON の status を読み、403 を UI state として扱う helper も持つ。
- `frontend/src/api/tenants.ts`, `frontend/src/api/machine-clients.ts`, `frontend/src/api/docs.ts`, `frontend/src/api/todos.ts` が generated SDK の薄い wrapper を持つ。
- fetch は `credentials: 'include'` で Cookie を送る。
- mutation 前に `XSRF-TOKEN` Cookie を読み、`X-CSRF-Token` header を付ける。
- `XSRF-TOKEN` が無い場合は `GET /api/v1/csrf` を先に呼ぶ。
- Login 画面は `GET /api/v1/auth/settings` を見て local password form と Zitadel login link を切り替える。
- App header は authenticated user に tenant selector を表示する。
- Home 画面は current session 表示、session refresh、logout、docs access check 付き link を持つ。
- Integrations 画面は active tenant を表示し、tenant 切り替えに追従して delegated auth の list/connect/verify/revoke を呼ぶ。
- TODO 画面は active tenant を表示し、tenant 切り替えに追従して list/create/update/delete を呼ぶ。
- TODO 画面は active tenant に `todo_user` tenant role が無い場合、blank ではなく `TODO role required` の 403 UI を表示する。
- Machine client admin 画面は list/create/detail/update/disable を扱う。
- `AdminAccessDenied` は machine client admin と TODO の role 不足時の 403 を画面内で表示する。
- logout 時に tenant store を reset し、次 user に古い tenant state が残らないようにしている。
- Vite dev server は `/api`, `/openapi`, `/docs` を backend `127.0.0.1:8080` に proxy する。
- `npm --prefix frontend run build` の出力先は `backend/web/dist`。
- production build output は `embed_frontend` build tag 付き Go binary に埋め込まれ、単一バイナリで SPA と API を配信できる。

注意点:

- `frontend/src/api/generated/` には現在の `@hey-api/openapi-ts` 生成物がある。
- frontend build output `backend/web/dist/` は生成物であり commit しない。
- tenant selector はログイン中 user の `tenant_memberships` だけを表示する。Zitadel login user と local demo user は別 user として扱う。
- docs auth が有効な環境では `DocsLink` が `/docs` を開く前に access check し、401 / 403 を frontend 上の error message にする。

## 生成物

生成物として扱うべきもの:

- `db/schema.sql`
- `backend/internal/db/*`
- `openapi/openapi.yaml`
- `frontend/src/api/generated/*`
- `frontend/package-lock.json`
- build output の `backend/web/dist/*`
- local binary の `bin/haohao`
- Docker image `haohao:dev`
- release asset の `haohao-linux-amd64.tar.gz`

現在確認した状態:

- `go run ./backend/cmd/openapi > /tmp/haohao-openapi.yaml` と `openapi/openapi.yaml` に差分なし。
- `cd backend && sqlc generate` 後に `backend/internal/db` へ差分なし。
- `cd frontend && npm run openapi-ts` 後に `frontend/src/api/generated` へ差分なし。

## 未実装 / 未接続

`CONCEPT.md` / tutorial の最終形に対して残っている主な項目:

- tenant 自体の CRUD 管理 UI。
- TODO より具体的な業務ドメイン機能。P2 では tenant-aware CRUD の実証として TODO を追加済みだが、Customer Signals / Product Decisions のような本来の業務モデルは未実装。
- M2M / external bearer 経由の TODO API。P2 は browser session + Cookie + CSRF に限定している。
- 本番用 metrics / tracing / alerting。structured logging と readiness はあるが、metrics exporter や distributed tracing は未実装。
- SCIM / Zitadel provisioning の本番運用では、provider 側 claim / group / SCIM 設定の runbook と staging smoke の拡充が必要。

## 検証結果

実行済み:

```bash
go test ./backend/...
npm --prefix frontend run build
CGO_ENABLED=0 go build -buildvcs=false -trimpath -tags "embed_frontend nomsgpack" -ldflags "-s -w -buildid=" -o ./bin/haohao ./backend/cmd/main
make binary
make smoke-operability
docker build -t haohao:dev -f docker/Dockerfile .
go run ./backend/cmd/openapi > /tmp/haohao-openapi.yaml && /usr/bin/diff -u openapi/openapi.yaml /tmp/haohao-openapi.yaml
cd backend && sqlc generate
cd frontend && npm run openapi-ts
make gen
make seed-demo-user
```

結果:

- backend test は成功。
- frontend build は成功し、`backend/web/dist/` に出力された。
- embedded binary build は成功。
- `make binary` は成功。
- `make smoke-operability` は成功し、`operability smoke ok: http://127.0.0.1:8080` を確認済み。
- `docker build -t haohao:dev -f docker/Dockerfile .` は成功。
- OpenAPI 再生成結果は tracked artifact と差分なし。
- sqlc 再生成後の差分なし。
- frontend SDK 再生成後の差分なし。
- binary smoke test では `/`, `/login`, `/integrations` が HTML、`/api/v1/session` が `401 application/problem+json`、`/openapi.yaml` が OpenAPI YAML、`/assets/missing.js` が `404`。
- `/todos` は single binary の SPA fallback として `200 text/html` で `index.html` を返すことを確認済み。
- 未ログインの `/api/v1/todos` は `401 application/problem+json` と `missing or expired session` を返すことを確認済み。
- operability smoke test では `/healthz`, `/readyz`, `/api/v1/session`, `/openapi.yaml`, forced OIDC callback redirect を確認済み。
- P1 browser smoke では tenant selector の Acme / Beta 切り替え、Integrations の tenant 追従、machine client の作成 / detail / update / disable、role 不足時の 403 UI、docs link access check を確認済み。
- P2 API smoke では local mode で `demo@example.com` / `changeme123` に login し、TODO create / patch / delete と Acme / Beta の tenant 分離を確認済み。
- P2 browser smoke では `/todos` 画面で TODO 作成、完了 toggle、tenant selector による Acme / Beta 切り替え、削除を確認済み。
- `AUTH_MODE=local` / `ENABLE_LOCAL_PASSWORD_LOGIN=true` で起動した local password login smoke test は `200 OK` と `Set-Cookie` を返した。
- root `.env` が `AUTH_MODE=zitadel` の場合、demo password login は `501` になるため、P2 local demo smoke では `AUTH_MODE=local ENABLE_LOCAL_PASSWORD_LOGIN=true` を起動時に上書きして確認した。
- Docker image smoke test では `/`, `/login`, `/api/v1/session`, `/openapi.yaml`, `/openapi-3.0.yaml`, `/assets/missing.js` を確認済み。
- `bin/haohao` と同じ directory に `.env` を置き、`cd <dir> && ./haohao` で `DATABASE_URL` などが読み込まれることを確認済み。
- dev 用 `FRONTEND_BASE_URL=http://127.0.0.1:5173` が残った `.env` でも、embedded binary の `/api/v1/auth/callback?error=forced` は `APP_BASE_URL` 側の `/login?error=oidc_callback_failed` へ redirect することを確認済み。
- local binary size は `15,035,506 bytes`。
- Docker image は `docker image ls` で `20MB` 表示、`docker history` の runtime payload は binary `14.6MB` + CA bundle `242kB`。

## 現在地の要約

HaoHao は、foundation の login/session 縦切りを超えて、Zitadel を中心にした browser login、external bearer API、delegated auth、SCIM provisioning、tenant-aware auth、M2M まで backend 実装が入っている。さらに、frontend SPA を Go binary に埋め込む単一バイナリ配信、`scratch` runtime Docker image、CI / release artifact 生成、P0 operability、P1 admin UI、P2 tenant-aware TODO まで到達している。

次に優先すべきなのは、P2 TODO で確認した browser session + CSRF + active tenant role の縦切りを、より実際の業務ドメインへ広げること、または tenant 自体の CRUD 管理 UI と metrics / tracing を補うこと。認証・認可・tenant・operability・admin UI の土台の上で、最初の tenant-aware 業務 CRUD は動作確認済みの段階にいる。
