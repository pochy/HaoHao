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
- `TUTORIAL_P7_5_CROSS_CUTTING_EXTENSIONS.md`
- `TUTORIAL_P8_OPENAPI_SPLIT.md`
- `TUTORIAL_P9_UI_PLAYWRIGHT_E2E.md`
- `TUTORIAL_P10_CROSS_CUTTING_EXTENSIONS.md`
- `TUTORIAL_P11_TENANT_RATE_LIMIT_RUNTIME.md`
- `TUTORIAL_P12_FILE_LIFECYCLE_PHYSICAL_DELETE.md`
- `RUNBOOK_OPERABILITY.md`
- `RUNBOOK_OBSERVABILITY.md`
- `RUNBOOK_DEPLOYMENT.md`
- 現在の repository 実装

## 全体像

現在の実装は、`CONCEPT.md` の基本方針である **OpenAPI 3.1 優先 + Monorepo + Go/Huma + Vue + PostgreSQL/sqlc + BFF Cookie 認証** を広い範囲で反映している。

`TUTORIAL.md` の foundation、`TUTORIAL_ZITADEL.md` の Phase 1-6、`TUTORIAL_SINGLE_BINARY.md`、P0-P12 の各チュートリアルまで実装済み。HaoHao は、local password login / Zitadel browser login / Cookie session / CSRF / tenant-aware auth / delegated auth / SCIM / M2M / single binary / operability / observability / audit / tenant admin / TODO / Customer Signals / common services / OpenAPI split / Playwright E2E / cross-cutting extensions / tenant rate limit runtime / file lifecycle purge まで持つ状態になっている。

| Phase | 状態 | 主な実装 |
| --- | --- | --- |
| Foundation | 実装済み | Go/Huma/Gin backend、Vue/Vite frontend、PostgreSQL/sqlc、Redis session、OpenAPI generated SDK |
| Zitadel | 実装済み | OIDC browser login、bearer API、delegated auth、SCIM、provisioning reconcile |
| Single binary | 実装済み | frontend embed、SPA fallback、`scratch` Docker image |
| P0: Operability | 実装済み | request id、structured logging、health/readiness、runbook、smoke |
| P1: Admin UI | 実装済み | tenant selector、integrations UX、machine client UI、docs access check |
| P2: TODO | 実装済み | tenant-aware TODO CRUD、role、UI |
| P3: Audit | 実装済み | `audit_events`、重要 mutation の audit、request metadata |
| P4: Observability | 実装済み | `/metrics`、dependency metrics、tracing、alert rules、runbook |
| P5: Tenant Admin | 実装済み | tenant CRUD/deactivate、membership role grant/revoke、tenant admin UI |
| P6: Customer Signals | 実装済み | tenant-aware domain CRUD、role、UI、audit、metrics |
| P7: Common Services | 実装済み | security hardening、outbox、idempotency、notifications、invitations、files、settings、exports、lifecycle |
| P8: OpenAPI Split | 実装済み | full/browser/external spec、browser SDK 入力、CI boundary check |
| P9: Playwright E2E | 実装済み | single binary E2E、role不足 UI、SPA fallback |
| P10: Cross-cutting Extensions | 実装済み | webhooks、CSV import/export、saved filters、support access、entitlements |
| P11: Rate Limit Runtime | 実装済み | tenant settings の browser API rate limit override を runtime 反映 |
| P12: File Physical Purge | 実装済み | local file body purge、retry/lock、audit/metrics、smoke |

現時点で残る大きな領域は、SMTP/provider email sender、DB query tracing、Grafana dashboard、webhook 外部配送 E2E、support access E2E 強化、object storage / virus scan、billing、realtime、HA/DR、M2M/external bearer 用の業務 API である。

## 方針との対応

| 項目 | 現状 |
| --- | --- |
| Monorepo | 実装済み。repo root に `go.work`、`frontend/`、`backend/`、`db/`、`openapi/` がある |
| Backend | Go module は `backend/`。Gin + Huma で API を構成 |
| OpenAPI | Huma から `openapi/openapi.yaml`、`openapi/browser.yaml`、`openapi/external.yaml` を生成 |
| Frontend | Vue 3 + Vite + TypeScript + Pinia + Vue Router |
| Generated SDK | `@hey-api/openapi-ts` で `openapi/browser.yaml` 由来の `frontend/src/api/generated/` を生成 |
| DB | PostgreSQL 18 前提。migration + `db/schema.sql` + sqlc |
| Auth | local password login と Zitadel OIDC login の両対応 |
| Session | Redis に session / CSRF / OIDC state / delegation state を保存 |
| Tenant | active tenant selector、tenant role、tenant admin API/UI を実装済み |
| Single binary | 実装済み。frontend build output を `backend/web/dist/` に出し、`embed_frontend` build tag で Go binary に embed する |
| Operability | 実装済み。structured request logging、request id、health/readiness、scheduler、smoke script、runbook がある |
| Observability | 実装済み。Prometheus metrics、dependency ping metrics、OpenTelemetry tracing、alert rules、observability runbook がある |
| Audit | 実装済み。重要 mutation を `audit_events` に記録する |
| Admin UI | 実装済み。tenant selector、integrations UX、machine client UI、tenant admin UI、docs access check がある |
| Business domain | 実装済み。tenant-aware TODO と Customer Signals がある |
| Web service common | 実装済み。security headers、body limit、CORS、rate limit、outbox、idempotency、notification、invitation、file upload、tenant settings、data export、data lifecycle がある |
| Cross-cutting extensions | 実装済み。webhooks、Customer Signals import/export、saved filters、support access、entitlements がある |
| Runtime policies | 実装済み。tenant settings の browser API rate limit override を middleware が参照する |
| File lifecycle | 実装済み。soft delete 後の local file body physical purge と DB tombstone がある |
| E2E | 実装済み。single binary に対する Playwright Chromium E2E がある |
| Docker / CI / release | 実装済み。`docker/Dockerfile`、`.dockerignore`、CI の embedded binary / Docker build、release asset upload がある |

## 開発基盤

実装済み:

- `go.work` は Go `1.26.0`、`use ./backend`。
- `compose.yaml` は PostgreSQL `18` と Redis `7.4` を起動する。
- `Makefile` に `up`, `down`, `db-up`, `db-down`, `db-schema`, `seed-demo-user`, `sqlc`, `openapi`, `gen`, `backend-dev`, `frontend-dev`, `frontend-build`, `binary`, `docker-build`, `e2e` がある。
- smoke target は `smoke-operability`, `smoke-observability`, `smoke-tenant-admin`, `smoke-customer-signals`, `smoke-common-services`, `smoke-p10`, `smoke-rate-limit-runtime`, `smoke-file-purge`, `smoke-backup-restore`。
- `scripts/gen.sh` は sqlc generate、full/browser/external OpenAPI export、frontend SDK 生成をまとめて実行する。
- `.env.example` には local / Zitadel / external bearer / M2M / downstream delegated auth / SCIM / readiness / reconcile scheduler / cookie / docs auth / metrics / tracing / security / outbox / idempotency / email / file / rate limit / tenant quota / data lifecycle / webhook / support access の設定がある。
- `dev/zitadel/` に self-hosted dev 用 Zitadel compose と `.env.example` がある。`make zitadel-up` 系の入口もある。
- `docker/Dockerfile` は Node builder、Go builder、`scratch` runtime の multi-stage build。
- `.github/workflows/ci.yml` は backend test、frontend build、embedded binary build、Playwright E2E、generated drift、DB schema drift、OpenAPI validate、Zitadel compose config、Docker build を確認する。
- `.github/workflows/release.yml` は OpenAPI artifacts と embedded Linux amd64 binary tarball を GitHub Release に upload する。
- `ops/prometheus/alerts/haohao.rules.yml` に初期 alert rules がある。
- `RUNBOOK_OPERABILITY.md`、`RUNBOOK_OBSERVABILITY.md`、`RUNBOOK_DEPLOYMENT.md` がある。

注意点:

- backend 本体は環境変数を読み、補助として `.env` も任意で読み込む。読み込み候補はカレントディレクトリの `.env` と実行ファイル横の `.env`。
- 既に設定されている環境変数は `.env` で上書きしない。Docker/Kubernetes や shell から渡した値が優先される。
- `make backend-dev` は引き続き `.env` を source してから起動する。
- `make smoke-*` の多くは server を起動しない。既に動いている `BASE_URL`、既定では `http://127.0.0.1:8080`、に対して確認する。
- `make e2e` と `make smoke-file-purge` は single binary を作り、一時 server を起動して確認する。

## Database / sqlc

現在の migration は `0001` から `0014` まである。

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
| `0010_tenant_admin_role` | tenant 管理用 global role |
| `0011_customer_signals` | Customer Signals。tenant-aware CRUD、soft delete、`customer_signal_user` role |
| `0012_web_service_common` | outbox、idempotency、notifications、tenant invitations、file objects、tenant settings、tenant data exports |
| `0013_p10_cross_cutting_extensions` | `support_agent` role、feature definitions、tenant entitlements、webhooks、deliveries、Customer Signal import jobs、saved filters、support access sessions |
| `0014_file_lifecycle_physical_delete` | `file_objects` の `purged_at`、purge attempts、lock、last error |

sqlc:

- `backend/sqlc.yaml` は `db/schema.sql` と `db/queries/` を入力にする。
- 生成先は `backend/internal/db/`。
- `uuid` は `github.com/google/uuid.UUID` に override されている。
- `db/queries/` は users, identities, roles, tenants, downstream grants, provisioning, machine clients, todos, audit events, tenant admin, customer signals, outbox, idempotency, notifications, tenant invitations, file objects, tenant settings, tenant data exports, entitlements, webhooks, customer signal imports, saved filters, support access を持つ。

設計上の注意点:

- tenant-aware table は `tenant_id` で絞る。tenant 外 record の存在は基本的に `404` として隠す。
- soft delete 対象は `deleted_at` や status を使い、通常 list/get では除外する。
- file body purge は DB row を hard delete せず、tombstone として `file_objects` row を残す。
- outbox claim と file purge claim は `FOR UPDATE SKIP LOCKED` を使う。
- metrics label には tenant id、user id、email、public id、idempotency key、raw path、storage key、webhook URL を入れない。
- audit metadata には secret、token、webhook signature、raw idempotency key を入れない。

## Backend

### 構成

主要な構成:

- `backend/cmd/main/main.go`: runtime 起動、PostgreSQL/Redis 接続、service wiring、worker 起動、HTTP server。
- `backend/cmd/openapi/main.go`: Huma API から full/browser/external OpenAPI YAML を出力。
- `backend/internal/app/app.go`: Gin engine、middleware、Huma config、security schemes、route registration、rate limit resolver。
- `backend/internal/api/`: Huma operation と request / response model。file upload/download の一部は raw Gin route。
- `backend/internal/service/`: session, OIDC login, identity, authz, delegation, provisioning, machine client, TODO, audit, tenant admin, Customer Signals, outbox, idempotency, notification, invitation, file, tenant settings, tenant export, entitlement, webhook, import, saved filter, support access。
- `backend/internal/auth/`: Cookie、Redis stores、OIDC/OAuth client、JWT bearer verifier、M2M verifier、refresh token encryption、generic secret box。
- `backend/internal/middleware/`: request id、request logger、docs auth、external CORS/auth、SCIM auth、M2M auth、tracing、security headers、body limit、browser CORS、rate limit。
- `backend/internal/platform/`: logger、readiness、metrics、tracing。
- `backend/internal/jobs/`: provisioning reconcile scheduler、outbox worker、data lifecycle job。
- `backend/frontend.go`: embedded frontend の static serving と SPA fallback。

### OpenAPI

P8 により、OpenAPI は 3 artifact に分割済み。

| artifact | 役割 |
| --- | --- |
| `openapi/openapi.yaml` | full canonical spec。runtime `/openapi.yaml` と全体 docs の互換入口 |
| `openapi/browser.yaml` | Cookie session / CSRF 前提の browser API。frontend generated SDK の入力 |
| `openapi/external.yaml` | external bearer / M2M / SCIM API |

実装:

- `backend/internal/api/register.go` に `SurfaceFull`, `SurfaceBrowser`, `SurfaceExternal` と `RegisterSurface` がある。
- `backend/cmd/openapi/main.go` は `-surface=full|browser|external` を受け取る。
- `Makefile` の `openapi` target と `scripts/gen.sh` は 3 spec を生成する。
- `frontend/openapi-ts.config.ts` は `../openapi/browser.yaml` を読む。
- CI は browser spec に external/M2M/SCIM が混ざらないこと、external spec に `/api/v1/*` と `cookieAuth` が混ざらないことを確認する。

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
- tenant settings に保存した `rateLimitBrowserApiPerMinute` を browser API runtime policy に反映。
- support access 中の rate limit bucket は actor user を requester として使う。
- `/healthz`, `/readyz`, `/metrics` は body limit / rate limit の対象外。

注意点:

- HSTS は HTTPS 終端の後ろで `SECURITY_HSTS_ENABLED=true` にする。
- `login` と `external_api` の tenant-specific rate limit override はまだ設計していない。P11 の対象は `browser_api` だけ。
- Redis rate limit 障害時は既存方針通り fail-open。

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
- `ops/prometheus/alerts/haohao.rules.yml` に scrape down、5xx、latency、readiness、dependency ping、SCIM reconcile、auth failure の初期 alert rules がある。
- `RUNBOOK_OBSERVABILITY.md` は各 alert の初動手順を持つ。

注意点:

- `/healthz` / `/readyz` / `/metrics` は Huma operation ではなく Gin route。OpenAPI には出ない。
- DB query span の詳細 instrumentation はまだ薄い。P4 では HTTP / dependency / scheduler / auth failure を優先している。
- Grafana dashboard の標準 artifact はまだない。

### Audit log

実装済み:

- `audit_events` table と `AuditService`。
- actor は user / system / machine client を拡張可能な形で持つ。
- support access 中は actor user と impersonated user を区別できる。
- request id、client IP、user agent、occurred_at を保存する。
- metadata は JSONB。secret、token、email raw value、idempotency key、webhook signature は入れない方針。
- 重要 mutation では audit failure を mutation failure として扱う。
- high-volume read-only access は audit 対象にしない。

主な audit action:

- session: `session.login`, `session.logout`, `session.tenant_switch`
- TODO: `todo.create`, `todo.update`, `todo.delete`
- machine client: `machine_client.create`, `machine_client.update`, `machine_client.delete`
- tenant admin: `tenant.create`, `tenant.update`, `tenant.deactivate`, `tenant_role.grant`, `tenant_role.revoke`
- Customer Signals: `customer_signal.create`, `customer_signal.update`, `customer_signal.delete`
- P7 common: `tenant_invitation.create`, `tenant_invitation.accept`, `tenant_invitation.revoke`, `tenant_settings.update`, `tenant_data_export.create`, `file.upload`, `file.delete`, `notification.read`
- P10: `tenant_entitlement.update`, `webhook.create`, `webhook.update`, `webhook.secret_rotate`, `webhook.delete`, `webhook.delivery_retry`, `customer_signal_import.create`, `customer_signal_saved_filter.*`, `support_access.start`, `support_access.end`
- P12: `file.purge`

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
- global role `support_agent` による support access。

注意点:

- tenant selector はログイン中 user が所属する active tenant 候補だけを扱う。
- tenant 自体の管理は `/api/v1/admin/tenants` 以下と tenant admin UI に分離している。
- M2M / external bearer 用の TODO / Customer Signals API はまだない。

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

- TODO API は browser session + Cookie + CSRF の縦切りに限定。M2M / external bearer 用 TODO API は未追加。

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
- list は `q`、cursor、limit、filter に対応。
- create / update / delete は audit に残る。
- HTTP metrics は route template、method、status class で出る。

注意点:

- customer name、signal public id、tenant slug は metrics label にしない。
- M2M / external bearer 用 Customer Signals API は未追加。

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
  - provider SMTP/SES/SendGrid sender は未追加
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
  - attachment target は `customer_signal`
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
  - `scripts/smoke-backup-restore.sh` は schema dump に common service table が含まれることを確認する

### P10 cross-cutting extensions

実装済み:

- feature flags / entitlements:
  - `feature_definitions`
  - `tenant_entitlements`
  - `GET /api/v1/admin/tenants/{tenantSlug}/entitlements`
  - `PUT /api/v1/admin/tenants/{tenantSlug}/entitlements`
  - `webhooks.enabled`, `customer_signals.import_export`, `customer_signals.saved_filters`, `support_access.enabled`
- outbound webhooks:
  - `webhook_endpoints`
  - `webhook_deliveries`
  - endpoint list/create/get/update/delete
  - secret rotate
  - delivery log
  - manual retry
  - AES-GCM encrypted secret
  - HMAC-SHA256 signature
  - outbox event `webhook.delivery_requested`
- Customer Signals import/export:
  - `customer_signal_import_jobs`
  - CSV import request/status
  - validation result
  - CSV export は tenant data export route の `format=csv`
  - outbox events `customer_signal_import.requested`, `customer_signal_export.requested`
- search / cursor / saved filters:
  - list query `q`, `cursor`, `limit`, filter
  - `customer_signal_saved_filters`
  - owner user + tenant scoped saved filters
- support access:
  - global role `support_agent`
  - `support_access_sessions`
  - `POST /api/v1/support/access/start`
  - `GET /api/v1/support/access/current`
  - `POST /api/v1/support/access/end`
  - reason、expiry、banner、audit

注意点:

- P10 の API は browser session + Cookie + CSRF 前提。external bearer / M2M / SCIM には追加していない。
- webhook secret、signature、URL は log / metrics / audit metadata に出さない。
- support access 中の destructive/admin-sensitive 操作の一部は拒否する。

### P11 tenant rate limit runtime

実装済み:

- `TenantSettingsService.ResolveEffectiveRateLimit`。
- `middleware.RateLimitResolver`。
- `backend/internal/app/app.go` で browser API resolver を wiring。
- `browser_api` policy は active tenant の `rateLimitBrowserApiPerMinute` を runtime に反映する。
- override が `nil`、active tenant がない、session/settings lookup 失敗時は config default に fallback。
- support access 中は actor user を requester として bucket を作る。
- metrics label は `policy` と `result` の低 cardinality を維持。
- `scripts/smoke-rate-limit-runtime.sh` で tenant override による `429` と `Retry-After` を確認する。

注意点:

- `login` と `external_api` は P11 では config default のまま。
- Redis failure は fail-open。

### P12 file lifecycle physical delete

実装済み:

- `0014_file_lifecycle_physical_delete` migration。
- `file_objects` に `purged_at`, `purge_attempts`, `purge_locked_at`, `purge_locked_by`, `last_purge_error`。
- deleted local file body の claim / mark success / mark failure query。
- `FileService.PurgeDeletedBodies`。
- `DataLifecycleJob` から file body purge を実行。
- file body deletion は DB transaction の外で実行。
- `LocalFileStorage.Delete` は file missing を成功扱いにするため retry 可能。
- purge 成功時に `file.purge` audit event と `haohao_data_lifecycle_items_total{kind="file_objects_body_purged"}`。
- `scripts/smoke-file-purge.sh` で upload -> soft delete -> retention -> body deletion -> `purged_at` を確認する。

注意点:

- P12 は `file_objects` row を hard delete しない。
- S3 / GCS driver はまだない。対象は `storage_driver = 'local'`。
- purge dead-letter UI はまだない。失敗時は `last_purge_error` を残して retry する。

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
  - `/api/v1/admin/tenants/{tenantSlug}/settings`
  - `/api/v1/admin/tenants/{tenantSlug}/invitations`
  - `/api/v1/admin/tenants/{tenantSlug}/exports`
  - `/api/v1/admin/tenants/{tenantSlug}/entitlements`
  - `/api/v1/admin/tenants/{tenantSlug}/webhooks`
  - `/api/v1/admin/tenants/{tenantSlug}/imports`
- Customer Signals:
  - `/api/v1/customer-signals`
  - `/api/v1/customer-signals/{signalPublicId}`
  - `/api/v1/customer-signal-filters`
  - `/api/v1/customer-signal-filters/{filterPublicId}`
- P7/P10 common:
  - `/api/v1/notifications`
  - `/api/v1/notifications/{notificationPublicId}/read`
  - `/api/v1/invitations/accept`
  - `/api/v1/files`
  - `/api/v1/files/{filePublicId}`
  - `/api/v1/support/access/start`
  - `/api/v1/support/access/current`
  - `/api/v1/support/access/end`
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
- Pinia store は session、tenant、machine clients、TODO、tenant admin、Customer Signals、files、notifications、tenant common settings/invitation/export/import/entitlement/webhook を管理する。
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
- generated SDK は `frontend/src/api/generated/`。入力は `openapi/browser.yaml`。
- `frontend/src/api/client.ts` が generated client の共通 transport 設定を持つ。
- fetch は `credentials: 'include'` で Cookie を送る。
- mutation 前に `XSRF-TOKEN` Cookie を読み、`X-CSRF-Token` header を付ける。
- POST 系 API には frontend client が `Idempotency-Key` を付ける。
- Login 画面は local password form と Zitadel login link を切り替える。
- App header は authenticated user に tenant selector と主要 navigation を表示する。
- Support access 中は `SupportAccessBanner` を表示する。
- Home 画面は current session 表示、session refresh、logout、docs access check 付き link を持つ。
- Integrations 画面は active tenant を表示し、tenant 切り替えに追従する。
- TODO 画面は tenant-aware list/create/update/delete を扱う。
- Customer Signals 画面は list/search/cursor/create/saved filters、detail/update/delete、attachment upload/download/delete、CSV export request を扱う。
- Tenant Admin 画面は tenant list/create/detail/update/deactivate、role grant/revoke、invitation、tenant settings、entitlements、support access、webhooks、import/export を扱う。
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

## Playwright E2E

実装済み:

- `frontend/package.json` に `@playwright/test` と `e2e` scripts がある。
- root に `playwright.config.ts` がある。
- root に `e2e/` directory がある。
- `scripts/e2e-single-binary.sh` は migration、seed、single binary 起動、Playwright 実行、cleanup を行う。
- `scripts/seed-e2e-users.sql` は role 不足確認用 user を用意する。
- `make e2e` は `make binary` 後に single binary に対して Playwright を実行する。
- CI は Chromium Playwright E2E を実行し、失敗時に report / trace / screenshot / video を artifact にする。

現在の test:

- `e2e/browser-journey.spec.ts`
  - login
  - tenant 切り替え
  - Customer Signals create/search/saved filter/detail
  - file upload
  - tenant settings update
  - entitlements update
  - invitation
  - tenant export / CSV export / CSV import
  - notification mark read
- `e2e/access-and-fallback.spec.ts`
  - limited user の role-specific access denied UI
  - single binary SPA fallback と API/assets 404 boundary

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
- `openapi/browser.yaml`
- `openapi/external.yaml`
- `frontend/src/api/generated/*`
- `frontend/package-lock.json`

commit しない local artifact:

- build output の `backend/web/dist/*`
- local binary の `bin/haohao`
- local file storage の `.data/*`
- Docker image `haohao:dev`
- release asset の `haohao-linux-amd64.tar.gz`
- Playwright `test-results/`
- Playwright `playwright-report/`

## 検証結果

2026-04-26 の現状確認で実行済み:

```bash
go test ./backend/...
npm --prefix frontend run build
go run ./backend/cmd/openapi -surface=full
go run ./backend/cmd/openapi -surface=browser
go run ./backend/cmd/openapi -surface=external
make e2e
make smoke-file-purge
```

加えて、一時 single binary server に対して次の smoke 本体が `ok` を返した。

```bash
scripts/smoke-common-services.sh
scripts/smoke-p10.sh
scripts/smoke-rate-limit-runtime.sh
```

結果:

- backend test は成功。
- frontend build は成功。
- full / browser / external OpenAPI 再生成結果は tracked artifact と差分なし。
- embedded binary build は `make e2e` と `make smoke-file-purge` の中で成功。
- `make e2e` は Playwright 3 tests passed。
- `make smoke-file-purge` は upload -> soft delete -> retention -> body deletion -> `purged_at` 記録を確認して成功。
- common services / P10 / rate limit runtime smoke は成功。
- `git diff --check` は成功。
- Docker build は今回の現状確認では実行していない。

注意点:

- `make smoke-*` は server を起動しないものがある。`BASE_URL` の先に app が起動していない場合は `curl: (7) Failed to connect` になる。
- local root `.env` が `AUTH_MODE=zitadel` の場合、demo password login は無効になるため、local smoke では起動時に `AUTH_MODE=local ENABLE_LOCAL_PASSWORD_LOGIN=true` を上書きする。
- ローカル Docker CLI は環境によって `docker compose` ではなく `docker-compose` の場合がある。CI は `docker compose` を使う。

## 残課題 / 次アクション

現在の明確な次アクション:

- SMTP / provider email sender。現在は log email sender 中心。
- DB query span などの詳細 tracing instrumentation。
- Grafana dashboard artifact。alert rules は `ops/prometheus/alerts/haohao.rules.yml` にある。
- webhook delivery の外部実配送 E2E。`RUN_WEBHOOK_SMOKE=1` 用の local receiver を CI へ足す価値がある。
- support access E2E 強化。banner、actor/impersonated audit、終了 flow を Playwright に追加する。
- object storage driver。S3 / GCS、signed download、retention/purge policy。
- file security processing。virus scan、content inspection、async processing。
- billing / subscription / invoice。tenant entitlements と pricing plan の接続。
- realtime notification。SSE/WebSocket は要件が固まってから追加する。
- multi-region / HA / DR。RPO/RTO、replica、failover、restore drill の拡張。
- M2M / external bearer 用の TODO / Customer Signals API。scope / tenant 解決方針を決めてから追加する。

## 現在地の要約

HaoHao は、foundation の login/session 縦切りを超えて、Zitadel を中心にした browser login、external bearer API、delegated auth、SCIM provisioning、tenant-aware auth、M2M、single binary 配信、Docker/CI/release、operability、observability、audit、admin UI、tenant 管理、tenant-aware TODO、Customer Signals、Web サービス共通機能、OpenAPI 分割、Playwright E2E、P10 横断拡張、tenant rate limit runtime、file lifecycle physical purge まで到達している。

現時点では、B2B SaaS や社内向け multi-tenant 業務 CRUD アプリの土台として、認証・認可・tenant・運用・監査・観測・管理・業務 CRUD・共通サービス・横断機能の主要な縦切りが動作確認済みである。次の優先は、具体サービス要件に合わせて email provider、dashboard、外部配送 E2E、object storage、billing、HA/DR を追加することである。
