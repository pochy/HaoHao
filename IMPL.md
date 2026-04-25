# HaoHao 実装状況

調査日: 2026-04-26

対象:

- `CONCEPT.md`
- `deep-research-report.md`
- `TUTORIAL.md`
- `TUTORIAL_ZITADEL.md`
- `TUTORIAL_SINGLE_BINARY.md`
- `TUTORIAL_P0_OPERABILITY.md`
- `TUTORIAL_P1_ADMIN_UI.md`
- `TUTORIAL_P2_TODO.md`
- `TUTORIAL_P3_AUDIT_LOG.md`
- `TUTORIAL_P4_OBSERVABILITY.md`
- `TUTORIAL_P5_TENANT_ADMIN_UI.md`
- `TUTORIAL_P6_DOMAIN_EXPANSION.md`
- `TUTORIAL_P7_WEB_SERVICE_COMMON.md`
- `RUNBOOK_OPERABILITY.md`
- `RUNBOOK_OBSERVABILITY.md`
- `RUNBOOK_DEPLOYMENT.md`
- 現在の repository 実装

## 全体像

現在の実装は、`CONCEPT.md` の基本方針である **OpenAPI 3.1 優先 + Monorepo + Go/Huma + Vue + PostgreSQL/sqlc + BFF Cookie 認証** を広い範囲で反映している。

`TUTORIAL.md` の foundation、`TUTORIAL_ZITADEL.md` の Phase 1-6、`TUTORIAL_SINGLE_BINARY.md`、P0-P7 の各チュートリアルまで実装済み。HaoHao は、local password login / Zitadel browser login / Cookie session / CSRF / tenant-aware auth / delegated auth / SCIM / M2M / single binary / operability / admin UI / TODO / audit log / observability / tenant admin / Customer Signals / Web サービス共通機能まで持つ状態になっている。

`deep-research-report.md` で次に積むべきとされていた P3-P7 は、次の形で実装済み。

| Phase | 状態 | 主な実装 |
| --- | --- | --- |
| P3: 監査ログ | 実装済み | `audit_events`、重要 mutation の audit、request metadata、audit 失敗方針 |
| P4: metrics / tracing / alerting | 実装済み | `/metrics`、HTTP/dependency/auth/scheduler metrics、OpenTelemetry tracing、observability runbook |
| P5: tenant 管理 UI | 実装済み | tenant CRUD/deactivate、membership role grant/revoke、tenant admin UI、audit |
| P6: 業務ドメイン拡張 | 実装済み | Customer Signals の tenant-aware CRUD、role、UI、audit、metrics smoke |
| P7: Web サービス共通機能 | 実装済み | security hardening、outbox、idempotency、notification、invitation、file upload、tenant settings/quota、data export、data lifecycle、backup/restore smoke |

現時点で残る大きな領域は、ブラウザ E2E の拡充、P7.5 以降の webhooks / import-export / search / support access / feature flags / entitlements、billing、大量ファイル、multi-region などである。

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
| Tenant | active tenant selector、tenant role、tenant admin API/UI を実装済み |
| Single binary | 実装済み。frontend build output を `backend/web/dist/` に出し、`embed_frontend` build tag で Go binary に embed する |
| Operability | 実装済み。structured request logging、request id、health/readiness、scheduler、smoke script、runbook がある |
| Observability | 実装済み。Prometheus metrics、dependency ping metrics、OpenTelemetry tracing、observability runbook がある |
| Audit | 実装済み。重要 mutation を `audit_events` に記録する |
| Admin UI | 実装済み。tenant selector、integrations UX、machine client admin UI、tenant admin UI、docs access check がある |
| Business domain | 実装済み。tenant-aware TODO と Customer Signals がある |
| Web service common | 実装済み。security headers、body limit、CORS、rate limit、outbox、idempotency、notification、invitation、file upload、tenant settings、data export、data lifecycle がある |
| Docker / CI / release | 実装済み。`docker/Dockerfile`、`.dockerignore`、CI の embedded binary / Docker build、release asset upload がある |

## 開発基盤

実装済み:

- `go.work` は Go `1.26.0`、`use ./backend`。
- `compose.yaml` は PostgreSQL `18` と Redis `7.4` を起動する。
- `Makefile` に `up`, `down`, `db-up`, `db-down`, `db-schema`, `seed-demo-user`, `sqlc`, `openapi`, `gen`, `backend-dev`, `frontend-dev`, `frontend-build`, `binary`, `docker-build` がある。
- smoke target は `smoke-operability`, `smoke-observability`, `smoke-tenant-admin`, `smoke-customer-signals`, `smoke-common-services`, `smoke-backup-restore`。
- `scripts/gen.sh` は sqlc generate、OpenAPI export、frontend SDK 生成をまとめて実行する。
- `.env.example` には local / Zitadel / external bearer / M2M / downstream delegated auth / SCIM / readiness / reconcile scheduler / cookie / docs auth / metrics / tracing / security / outbox / idempotency / email / file / rate limit / tenant quota / data lifecycle の設定がある。
- `dev/zitadel/` に self-hosted dev 用 Zitadel compose と `.env.example` がある。`make zitadel-up` 系の入口もある。
- `docker/Dockerfile` は Node builder、Go builder、`scratch` runtime の multi-stage build。
- `.github/workflows/ci.yml` は backend test、frontend build、embedded binary build、generated drift、DB schema drift、OpenAPI validate、Zitadel compose config、Docker build を確認する。
- `.github/workflows/release.yml` は OpenAPI artifact と embedded Linux amd64 binary tarball を GitHub Release に upload する。
- `RUNBOOK_OPERABILITY.md` は binary deploy、Docker deploy、rollback、Zitadel redirect URI、smoke test の手順を持つ。
- `RUNBOOK_OBSERVABILITY.md` は scrape down、5xx、latency、readiness failure、dependency ping、SCIM reconcile、auth failure spike の初動手順を持つ。
- `RUNBOOK_DEPLOYMENT.md` は P7 common services の secret、volume、migration、retention、backup / restore drill を持つ。

注意点:

- backend 本体は環境変数を読み、補助として `.env` も任意で読み込む。読み込み候補はカレントディレクトリの `.env` と実行ファイル横の `.env`。
- 既に設定されている環境変数は `.env` で上書きしない。Docker/Kubernetes や shell から渡した値が優先される。
- `make backend-dev` は引き続き `.env` を source してから起動する。
- `make smoke-*` は server を起動しない。既に動いている `BASE_URL`、既定では `http://127.0.0.1:8080`、に対して確認する。
- single binary smoke では、先に `./bin/haohao` を対象 port で起動する必要がある。

## Database / sqlc

現在の migration は `0001` から `0012` まである。

| migration | 内容 |
| --- | --- |
| `0001_init` | `pgcrypto`、`users`。`public_id UUID DEFAULT uuidv7()`、local password 用 `password_hash` |
| `0002_user_identities` | `user_identities`、外部 IdP identity。`password_hash` nullable 化 |
| `0003_roles` | `roles`, `user_roles`。初期 role は `docs_reader`, `external_api_user`, `todo_user` |
| `0004_downstream_grants` | delegated auth 用 `oauth_user_grants` |
| `0005_provisioning` | `deactivated_at`、SCIM/provisioning 用 identity columns、`provisioning_sync_state` |
| `0006_org_tenants` | `tenants`, `tenant_memberships`, `tenant_role_overrides`、user default tenant、grant tenant 化 |
| `0007_machine_clients` | `machine_clients` と `machine_client_admin` role |
| `0008_todos` | tenant 共有 TODO |
| `0009_audit_events` | `audit_events`。actor / tenant / action / target / request metadata / JSON metadata を保存 |
| `0010_tenant_admin_role` | tenant 管理用 global role と tenant admin API/UI のための補助 |
| `0011_customer_signals` | Customer Signals。tenant-aware CRUD、soft delete、`customer_signal_user` role |
| `0012_web_service_common` | outbox、idempotency、notifications、tenant invitations、file objects、tenant settings、tenant data exports |

sqlc:

- `backend/sqlc.yaml` は `db/schema.sql` と `db/queries/` を入力にする。
- 生成先は `backend/internal/db/`。
- `uuid` は `github.com/google/uuid.UUID` に override されている。
- `db/queries/` は users, identities, roles, tenants, downstream grants, provisioning, machine clients, todos, audit events, tenant admin, customer signals, outbox, idempotency, notifications, tenant invitations, file objects, tenant settings, tenant data exports を持つ。

設計上の注意点:

- tenant-aware table は `tenant_id` で絞る。tenant 外 record の存在は基本的に `404` として隠す。
- soft delete 対象は `deleted_at` や status を使い、通常 list/get では除外する。
- outbox claim は `FOR UPDATE SKIP LOCKED` を使う。
- metrics label には tenant id、user id、email、public id、idempotency key、raw path を入れない。
- audit metadata には secret や token を入れない。

## Backend

### 構成

主要な構成:

- `backend/cmd/main/main.go`: runtime 起動、PostgreSQL/Redis 接続、service wiring、worker 起動、HTTP server。
- `backend/cmd/openapi/main.go`: Huma API から OpenAPI YAML を出力。
- `backend/internal/app/app.go`: Gin engine、middleware、Huma config、security schemes、route registration。
- `backend/internal/api/`: Huma operation と request / response model。file upload/download の一部は raw Gin route。
- `backend/internal/service/`: session, OIDC login, identity, authz, delegation, provisioning, machine client, TODO, audit, tenant admin, Customer Signals, outbox, idempotency, notification, invitation, file, tenant settings, tenant export。
- `backend/internal/auth/`: Cookie、Redis stores、OIDC/OAuth client、JWT bearer verifier、M2M verifier、refresh token encryption。
- `backend/internal/middleware/`: request id、request logger、docs auth、external CORS/auth、SCIM auth、M2M auth、tracing、security headers、body limit、browser CORS、rate limit。
- `backend/internal/platform/`: logger、readiness、metrics、tracing。
- `backend/internal/jobs/`: provisioning reconcile scheduler、outbox worker、data lifecycle job。
- `backend/frontend.go`: embedded frontend の static serving と SPA fallback。

### Security / request boundary

実装済み:

- request id middleware と structured request logging。
- trusted proxy CIDR 設定。
- security headers:
  - `Content-Security-Policy`
  - `Strict-Transport-Security` optional
  - `X-Content-Type-Options`
  - `Referrer-Policy`
  - frame 制御
- request body size limit。
- browser API CORS policy。
- Redis-backed fixed window rate limit。
- `/healthz`, `/readyz`, `/metrics` は body limit / rate limit の対象外。

注意点:

- HSTS は HTTPS 終端の後ろで `SECURITY_HSTS_ENABLED=true` にする。
- tenant settings に rate limit override は保存できるが、middleware の runtime limit は現在 config ベース。tenant ごとの runtime override 反映は後続候補。

### Operability / observability

実装済み:

- `/healthz` は process liveness として `200 {"status":"ok"}` を返す。
- `/readyz` は PostgreSQL / Redis を ping し、設定により Zitadel discovery も確認する。
- `/metrics` は Prometheus text format を返す。
- HTTP metrics:
  - `haohao_http_requests_total`
  - `haohao_http_request_duration_seconds`
- dependency metrics:
  - `haohao_dependency_ping_duration_seconds`
  - readiness failure counter
- scheduler / auth / outbox / rate limit / data lifecycle / file quota metrics。
- OpenTelemetry tracing は default off。`OTEL_TRACING_ENABLED=true` と OTLP endpoint で有効化する。
- request log には tracing 有効時に `trace_id` / `span_id` を出せる。
- alerting は実 alert rule ではなく、まず `RUNBOOK_OBSERVABILITY.md` に初動手順を固定している。

注意点:

- `/healthz` / `/readyz` / `/metrics` は Huma operation ではなく Gin route。OpenAPI には出ない。
- DB query span の詳細 instrumentation は未実装。P4 では HTTP / dependency / scheduler / auth failure を優先している。

### Audit log

実装済み:

- `audit_events` table と `AuditService`。
- actor は user / system / machine client を拡張可能な形で持つ。
- request id、client IP、user agent、occurred_at を保存する。
- metadata は JSONB。secret、token、email raw value、idempotency key は入れない方針。
- 重要 mutation では audit failure を mutation failure として扱う。
- high-volume read-only access は audit 対象にしない。

主な audit action:

- session: `session.login`, `session.logout`, `session.tenant_switch`
- TODO: `todo.create`, `todo.update`, `todo.delete`
- machine client: `machine_client.create`, `machine_client.update`, `machine_client.delete`
- tenant admin: `tenant.create`, `tenant.update`, `tenant.deactivate`, `tenant_role.grant`, `tenant_role.revoke`
- Customer Signals: `customer_signal.create`, `customer_signal.update`, `customer_signal.delete`
- P7 common: `tenant_invitation.create`, `tenant_invitation.accept`, `tenant_invitation.revoke`, `tenant_settings.update`, `tenant_data_export.create`, `file.upload`, `file.delete`, `notification.read`

### Auth / tenant / M2M

実装済み:

- local password login と Zitadel browser login。
- Cookie session、CSRF token、session refresh、logout。
- Redis session / CSRF / OIDC state / delegation state。
- active tenant selector API:
  - `GET /api/v1/tenants`
  - `POST /api/v1/session/tenant`
- provider group grammar `tenant:<slug>:<role>` から tenant membership を同期する。
- tenant role override の DB と解決ロジック。
- external bearer API。
- downstream delegated auth。
- SCIM user provisioning。
- machine client CRUD と M2M bearer middleware。

注意点:

- tenant selector はログイン中 user が所属する active tenant 候補だけを扱う。
- tenant 自体の管理は `/api/v1/admin/tenants` 以下と tenant admin UI に分離している。

### P2 TODO

実装済み:

- `todos` table と sqlc query。
- browser session + active tenant + `todo_user` tenant role。
- `GET /api/v1/todos`
- `POST /api/v1/todos`
- `PATCH /api/v1/todos/{todoPublicId}`
- `DELETE /api/v1/todos/{todoPublicId}`
- tenant 外または存在しない TODO は `404`。
- create / update / delete は audit に残る。

注意点:

- TODO API は browser session + Cookie + CSRF の縦切りに限定。M2M / external bearer 用 TODO API は未実装。

### P5 tenant admin

実装済み:

- global role `tenant_admin` 前提の tenant 管理 API/UI。
- `GET /api/v1/admin/tenants`
- `POST /api/v1/admin/tenants`
- `GET /api/v1/admin/tenants/{tenantSlug}`
- `PUT /api/v1/admin/tenants/{tenantSlug}`
- `DELETE /api/v1/admin/tenants/{tenantSlug}` は物理削除ではなく deactivate。
- `POST /api/v1/admin/tenants/{tenantSlug}/memberships`
- `DELETE /api/v1/admin/tenants/{tenantSlug}/memberships/{userPublicId}/roles/{roleCode}`
- local override role の grant / revoke。
- provider claim / SCIM 由来 role は表示するが、同じ UI で無条件に削除しない。
- destructive action は確認 dialog を挟む。
- mutation は audit に残る。

### P6 Customer Signals

実装済み:

- `customer_signals` table と sqlc query。
- tenant-aware CRUD。
- browser session + active tenant + `customer_signal_user` tenant role。
- `GET /api/v1/customer-signals`
- `POST /api/v1/customer-signals`
- `GET /api/v1/customer-signals/{signalPublicId}`
- `PATCH /api/v1/customer-signals/{signalPublicId}`
- `DELETE /api/v1/customer-signals/{signalPublicId}` は soft delete。
- create / update / delete は audit に残る。
- HTTP metrics は route template、method、status class で出る。

注意点:

- customer name、signal public id、tenant slug は metrics label にしない。
- M2M / external bearer 用 Customer Signals API は未実装。

### P7 Web service common

実装済み:

- outbox:
  - `outbox_events`
  - enqueue / claim / sent / retry / dead
  - worker interval / timeout / batch size / max attempts
  - `notification.email_requested`, `tenant_invitation.created`, `tenant_data_export.requested`
- idempotency:
  - `idempotency_keys`
  - `Idempotency-Key` header
  - request hash、response summary、TTL
  - Customer Signal create、tenant invitation create/accept、tenant data export request などに接続
- email:
  - log email sender
  - SMTP sender は未実装
- notifications:
  - `notifications`
  - `GET /api/v1/notifications`
  - `POST /api/v1/notifications/{notificationPublicId}/read`
- tenant invitations:
  - `tenant_invitations`
  - `GET /api/v1/admin/tenants/{tenantSlug}/invitations`
  - `POST /api/v1/admin/tenants/{tenantSlug}/invitations`
  - `DELETE /api/v1/admin/tenants/{tenantSlug}/invitations/{invitationPublicId}`
  - `POST /api/v1/invitations/accept`
  - token は hash 保存。local/dev smoke では response に token を返す。
- file upload:
  - `file_objects`
  - local file storage
  - `GET /api/v1/files`
  - `POST /api/v1/files` は multipart raw Gin route
  - `GET /api/v1/files/:filePublicId` は raw download route
  - `DELETE /api/v1/files/{filePublicId}`
  - P7 初期版の attachment target は `customer_signal`
- tenant settings / quota:
  - `tenant_settings`
  - `GET /api/v1/admin/tenants/{tenantSlug}/settings`
  - `PUT /api/v1/admin/tenants/{tenantSlug}/settings`
  - file quota、browser API rate limit override、notifications enabled、features JSON
- tenant data export:
  - `tenant_data_exports`
  - `GET /api/v1/admin/tenants/{tenantSlug}/exports`
  - `POST /api/v1/admin/tenants/{tenantSlug}/exports`
  - `GET /api/v1/admin/tenants/{tenantSlug}/exports/{exportPublicId}`
  - `GET /api/v1/admin/tenants/{tenantSlug}/exports/{exportPublicId}/download`
  - JSON export を file storage に生成
- data lifecycle:
  - expired idempotency key cleanup
  - expired invitations
  - processed outbox retention
  - read notification retention
  - expired tenant data exports
- backup / restore:
  - `scripts/smoke-backup-restore.sh` は schema dump に P7 table が含まれることを確認する

注意点:

- P7 初期版では file body の物理削除は未実装。metadata soft delete と retention 方針はある。
- tenant data export は JSON metadata 中心。大量 file archive は P7.5 以降。
- tenant settings の feature toggle は保存できるが、billing / pricing / entitlement との接続は未実装。

## API surface

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
- Tenant selector:
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
- Tenant admin:
  - `/api/v1/admin/tenants`
  - `/api/v1/admin/tenants/{tenantSlug}`
  - `/api/v1/admin/tenants/{tenantSlug}/memberships`
  - `/api/v1/admin/tenants/{tenantSlug}/memberships/{userPublicId}/roles/{roleCode}`
- Customer Signals:
  - `/api/v1/customer-signals`
  - `/api/v1/customer-signals/{signalPublicId}`
- P7 common:
  - `/api/v1/notifications`
  - `/api/v1/notifications/{notificationPublicId}/read`
  - `/api/v1/admin/tenants/{tenantSlug}/invitations`
  - `/api/v1/admin/tenants/{tenantSlug}/invitations/{invitationPublicId}`
  - `/api/v1/invitations/accept`
  - `/api/v1/files`
  - `/api/v1/files/{filePublicId}`
  - `/api/v1/admin/tenants/{tenantSlug}/settings`
  - `/api/v1/admin/tenants/{tenantSlug}/exports`
  - `/api/v1/admin/tenants/{tenantSlug}/exports/{exportPublicId}`
  - `/api/v1/admin/tenants/{tenantSlug}/exports/{exportPublicId}/download`
- M2M:
  - `GET /api/m2m/v1/self`

OpenAPI には出ないが runtime に存在する endpoint:

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`
- raw multipart upload `POST /api/v1/files`
- raw file download `GET /api/v1/files/:filePublicId`

## Frontend

実装済み:

- Vue 3 + Vite + TypeScript。
- Pinia store は session、tenant、machine clients、TODO、tenant admin、Customer Signals、files、notifications、tenant common settings/invitation/export を管理する。
- Vue Router route:
  - `/`
  - `/login`
  - `/integrations`
  - `/notifications`
  - `/invitations/accept`
  - `/todos`
  - `/customer-signals`
  - `/customer-signals/:signalPublicId`
  - `/tenant-admin`
  - `/tenant-admin/new`
  - `/tenant-admin/:tenantSlug`
  - `/machine-clients`
  - `/machine-clients/new`
  - `/machine-clients/:id`
- generated SDK は `frontend/src/api/generated/`。
- `frontend/src/api/client.ts` が generated client の共通 transport 設定を持つ。
- fetch は `credentials: 'include'` で Cookie を送る。
- mutation 前に `XSRF-TOKEN` Cookie を読み、`X-CSRF-Token` header を付ける。
- POST 系 API には frontend client が `Idempotency-Key` を付ける。
- Login 画面は local password form と Zitadel login link を切り替える。
- App header は authenticated user に tenant selector と主要 navigation を表示する。
- Home 画面は current session 表示、session refresh、logout、docs access check 付き link を持つ。
- Integrations 画面は active tenant を表示し、tenant 切り替えに追従する。
- TODO 画面は tenant-aware list/create/update/delete を扱う。
- Customer Signals 画面は list/create、detail/update/delete、attachment upload/download/delete を扱う。
- Tenant Admin 画面は tenant list/create/detail/update/deactivate、role grant/revoke、invitation、tenant settings、tenant data export を扱う。
- Notifications 画面は list と mark read を扱う。
- Invitation accept 画面は token から invite accept を行う。
- `AdminAccessDenied` は role 不足時の 403 を画面内で表示する。
- `ConfirmActionDialog` は destructive action の確認に使う。
- logout 時に tenant store を reset し、次 user に古い tenant state が残らないようにしている。
- Vite dev server は `/api`, `/openapi`, `/docs` を backend `127.0.0.1:8080` に proxy する。
- production build output は `backend/web/dist` に出力され、`embed_frontend` build tag 付き Go binary に埋め込まれる。

注意点:

- `frontend/src/api/generated/` は現在の `@hey-api/openapi-ts` 生成物。
- frontend build output `backend/web/dist/` は生成物であり commit しない。
- tenant selector は現在 user の active tenant 切り替え用。tenant 管理 UI とは責務を分けている。

## Single binary / SPA 配信

実装済み:

- `npm --prefix frontend run build` は Vue production build を `backend/web/dist/` に出力する。
- `make binary` は frontend build 後に `CGO_ENABLED=0 go build -buildvcs=false -trimpath -tags "embed_frontend nomsgpack" -ldflags "-s -w -buildid=" -o ./bin/haohao ./backend/cmd/main` を実行する。
- build tag なしの `go test ./backend/...` や `go run ./backend/cmd/openapi` は frontend dist 不在でも壊れない。
- `/`, `/login`, `/integrations`, `/notifications`, `/invitations/accept`, `/todos`, `/customer-signals`, `/tenant-admin`, `/machine-clients` などの SPA route は `index.html` に fallback する。
- `/api/*`, `/docs`, `/schemas/*`, `/openapi.yaml`, `/openapi.json`, `/openapi-3.0.yaml`, `/openapi-3.0.json`, `/healthz`, `/readyz`, `/metrics` は SPA fallback しない。
- 存在しない `/assets/*` や拡張子付き path は `index.html` ではなく `404` を返す。

## 生成物

生成物として扱うべきもの:

- `db/schema.sql`
- `backend/internal/db/*`
- `openapi/openapi.yaml`
- `frontend/src/api/generated/*`
- `frontend/package-lock.json`

commit しない local artifact:

- build output の `backend/web/dist/*`
- local binary の `bin/haohao`
- local file storage の `.data/*`
- Docker image `haohao:dev`
- release asset の `haohao-linux-amd64.tar.gz`

## 検証結果

直近で確認済み:

```bash
make db-up db-schema sqlc
go test ./backend/...
npm --prefix frontend run build
make gen
make binary
git diff --check
```

single binary を `18083` で起動して確認:

```bash
HTTP_PORT=18083 \
APP_BASE_URL=http://127.0.0.1:18083 \
FRONTEND_BASE_URL=http://127.0.0.1:18083 \
AUTH_MODE=local \
ENABLE_LOCAL_PASSWORD_LOGIN=true \
OUTBOX_WORKER_INTERVAL=1s \
DATA_LIFECYCLE_RUN_ON_STARTUP=true \
./bin/haohao
```

別 terminal で実行:

```bash
BASE_URL=http://127.0.0.1:18083 make smoke-operability smoke-observability smoke-tenant-admin smoke-customer-signals smoke-common-services smoke-backup-restore
```

結果:

- backend test は成功。
- frontend build は成功。
- OpenAPI / frontend SDK / sqlc 生成は成功。
- embedded binary build は成功。
- `smoke-operability` は成功。
- `smoke-observability` は成功。
- `smoke-tenant-admin` は成功。
- `smoke-customer-signals` は成功。
- `smoke-common-services` は成功。
- `smoke-backup-restore` は成功。
- `/metrics` に HTTP / outbox / rate limit metrics が出ることを確認済み。
- audit log に Customer Signals、file upload/delete、tenant invitation、tenant settings、tenant data export が出ることを確認済み。

ブラウザ確認済み:

- `http://127.0.0.1:18083/login`
- `demo@example.com` / `changeme123` で login
- active tenant が `Acme / acme` になる
- Notifications 画面表示
- Customer Signals の作成、詳細表示、添付 UI 表示
- Tenant Admin detail の invite / settings / quota / data export セクション表示

注意点:

- `make smoke-*` は server を起動しない。`BASE_URL` の先に app が起動していない場合は `curl: (7) Failed to connect` になる。
- local root `.env` が `AUTH_MODE=zitadel` の場合、demo password login は無効になるため、local smoke では起動時に `AUTH_MODE=local ENABLE_LOCAL_PASSWORD_LOGIN=true` を上書きする。

## 未実装 / 後続候補

P7 までの範囲で明確に残っているもの:

- ブラウザ E2E の拡充。shell smoke はあるが、Playwright などの UI regression test は未整備。
- tenant settings の rate limit override を middleware の runtime policy に反映すること。
- file lifecycle で local storage 上の実 file body を retention 後に物理削除すること。
- SMTP / provider email sender。現在は log email sender。
- DB query span などの詳細 tracing instrumentation。
- alert rule / dashboard の具体ファイル。現在は metrics と runbook まで。

P7.5 / P8 以降に回す候補:

- outbound webhooks。署名、retry、delivery log、dead letter。
- import / export jobs。CSV import、CSV export、job status UI。
- search / cursor pagination。Customer Signals が増えた段階で full-text search や saved filters を検討する。
- support access / impersonation。理由入力、時間制限、明示 banner、audit が前提。
- feature flags / entitlements。tenant settings から独立させ、billing / pricing plan と接続する。
- billing / subscription / invoice。
- 大量 file archive、object storage provider、virus scan。
- realtime notification。
- multi-region / HA / DR。
- M2M / external bearer 用の TODO / Customer Signals API。

## 現在地の要約

HaoHao は、foundation の login/session 縦切りを超えて、Zitadel を中心にした browser login、external bearer API、delegated auth、SCIM provisioning、tenant-aware auth、M2M、single binary 配信、Docker/CI/release、operability、observability、audit、admin UI、tenant 管理、tenant-aware TODO、Customer Signals、Web サービス共通機能まで到達している。

現時点では、B2B SaaS や社内向け multi-tenant 業務 CRUD アプリの土台として、認証・認可・tenant・運用・監査・観測・管理・業務 CRUD・共通サービスの主要な縦切りが動作確認済みである。次の優先は、UI E2E で回帰検知を固めること、または P7.5 として webhooks / import-export / search / support access / feature flags を足すことである。
