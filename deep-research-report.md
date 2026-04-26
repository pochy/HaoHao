# HaoHao deep research report

調査日: 2026-04-25
現状反映日: 2026-04-26 (Asia/Tokyo)

基準資料:

- `IMPL.md`
- 現在の repository 実装
- `TUTORIAL.md`
- `TUTORIAL_ZITADEL.md`
- `TUTORIAL_SINGLE_BINARY.md`
- `TUTORIAL_P0_OPERABILITY.md` から `TUTORIAL_P12_FILE_LIFECYCLE_PHYSICAL_DELETE.md`
- `RUNBOOK_OPERABILITY.md`
- `RUNBOOK_OBSERVABILITY.md`
- `RUNBOOK_DEPLOYMENT.md`

## エグゼクティブサマリ

HaoHao は、当初の foundation tutorial を超えて、**Go/Huma/Gin backend + Vue/Vite frontend + PostgreSQL/sqlc + Redis session + Zitadel 連携**を中核にした multi-tenant Web application 基盤まで進んでいる。

2026-04-26 時点では、P0 から P12 までの主要チュートリアルが実装済みである。local password login、Zitadel OIDC browser login、Cookie session、CSRF、external bearer API、delegated OAuth grant、SCIM provisioning、tenant-aware auth、M2M bearer API、machine client 管理、OpenAPI 生成、frontend generated SDK、単一バイナリ配信、Docker/CI/release asset に加えて、audit、metrics/tracing/alert rules、tenant 管理 UI、Customer Signals、Web サービス共通機能、OpenAPI 分割、Playwright E2E、P10 横断拡張、tenant rate limit runtime 連動、file body purge まで入っている。

現在の HaoHao は、B2B SaaS、社内管理ツール、tenant-aware な業務 CRUD アプリの土台として十分に使える段階にある。次の課題は foundation の作り直しではなく、実運用や具体サービス化に必要な **delivery provider、dashboard、外部配送 E2E、object storage、billing、HA/DR** などを選択的に足すことにある。

## 現在地

| 領域 | 状態 |
| --- | --- |
| Monorepo | 実装済み。repo root に `go.work`、`backend/`、`frontend/`、`db/`、`openapi/` がある |
| Backend | Go module は `backend/`。Gin + Huma で API を構成 |
| OpenAPI | `openapi/openapi.yaml`、`openapi/browser.yaml`、`openapi/external.yaml` の 3 artifact を Huma から生成 |
| Frontend | Vue 3 + Vite + TypeScript + Pinia + Vue Router |
| Generated SDK | `@hey-api/openapi-ts` で `openapi/browser.yaml` 由来の `frontend/src/api/generated/` を生成 |
| Database | PostgreSQL 18 前提。migration は `0014_file_lifecycle_physical_delete` まで進んでいる |
| Session | Redis に session / CSRF / OIDC state / delegation state を保存 |
| Browser auth | local password login と Zitadel OIDC login の両対応 |
| External API | OIDC JWKS 検証付き bearer API がある |
| SCIM | User create/list/get/replace/patch/delete subset がある |
| Tenant | tenant membership、active tenant session、tenant role、tenant admin UI/API がある |
| M2M | machine client table、管理 API/UI、M2M bearer middleware、self endpoint がある |
| Audit | `audit_events` に重要 mutation と request metadata を保存 |
| Observability | `/metrics`、OpenTelemetry tracing、alert rules、runbook がある |
| Business domain | tenant-aware TODO と Customer Signals がある |
| Common services | outbox、idempotency、notification、invitation、file upload、tenant settings、data export、data lifecycle がある |
| Cross-cutting extensions | webhooks、CSV import/export、saved filters、support access、entitlements がある |
| Runtime policy | tenant settings の browser API rate limit override を middleware が runtime 参照する |
| File lifecycle | soft-deleted local file body を retention 後に purge し、DB tombstone は残す |
| E2E | single binary に対する Playwright E2E がある |
| Docker / CI / release | `scratch` runtime image、CI、OpenAPI artifacts、release binary がある |

## 実装アーキテクチャ

現在の構成は、次の境界で読むと整理しやすい。

```text
.
├─ backend/
│  ├─ cmd/main/                 # runtime entrypoint
│  ├─ cmd/openapi/              # OpenAPI export
│  ├─ internal/api/             # Huma operations
│  ├─ internal/app/             # Gin/Huma router wiring
│  ├─ internal/auth/            # Cookie, Redis stores, OIDC/JWT/M2M
│  ├─ internal/config/          # env, .env, frontend URL normalization
│  ├─ internal/db/              # sqlc generated package
│  ├─ internal/jobs/            # schedulers and workers
│  ├─ internal/middleware/      # request/auth/rate-limit/security middleware
│  ├─ internal/platform/        # logger, readiness, metrics, tracing
│  └─ internal/service/         # application services
├─ frontend/
│  ├─ src/
│  ├─ src/api/generated/        # browser OpenAPI generated client
│  └─ vite.config.ts            # build output: ../backend/web/dist
├─ db/
│  ├─ migrations/
│  ├─ queries/
│  └─ schema.sql
├─ openapi/
│  ├─ openapi.yaml              # full canonical spec
│  ├─ browser.yaml              # browser SDK spec
│  └─ external.yaml             # external bearer / M2M / SCIM spec
├─ e2e/
├─ ops/prometheus/alerts/
├─ docker/Dockerfile
└─ Makefile
```

frontend build output を repository の正本にはしていない。`backend/web/dist/` は生成物であり、production binary を作る直前に `npm --prefix frontend run build` で生成し、`embed_frontend` build tag 付きの Go binary に埋め込む。

## OpenAPI と API surface

P8 で OpenAPI は利用者と security boundary に沿って分割された。

| artifact | 用途 |
| --- | --- |
| `openapi/openapi.yaml` | full canonical spec。runtime `/openapi.yaml`、全体 docs、drift check の正本 |
| `openapi/browser.yaml` | Cookie session / CSRF 前提の browser API。frontend generated SDK の入力 |
| `openapi/external.yaml` | browser Cookie に依存しない external bearer / M2M / SCIM API |

`openapi/browser.yaml` には `/api/external/*`、`/api/m2m/*`、`/api/scim/*` と bearer security scheme を含めない。`openapi/external.yaml` には `/api/v1/*` と `cookieAuth` を含めない。CI でもこの境界を grep check している。

主な runtime endpoint は次の通り。

| 区分 | endpoint |
| --- | --- |
| Browser auth/session | `GET /api/v1/auth/settings`, `GET /api/v1/auth/login`, `GET /api/v1/auth/callback`, `POST /api/v1/login`, `GET /api/v1/session`, `GET /api/v1/csrf`, `POST /api/v1/session/refresh`, `POST /api/v1/logout` |
| Tenant selector | `GET /api/v1/tenants`, `POST /api/v1/session/tenant` |
| Tenant admin | `/api/v1/admin/tenants`, `/api/v1/admin/tenants/{tenantSlug}`, memberships, tenant roles, settings, invitations, exports, entitlements, webhooks, imports |
| Delegated auth | `/api/v1/integrations/*` |
| TODO | `/api/v1/todos`, `/api/v1/todos/{todoPublicId}` |
| Customer Signals | `/api/v1/customer-signals`, `/api/v1/customer-signals/{signalPublicId}`, `/api/v1/customer-signal-filters` |
| Files | `GET /api/v1/files`, raw `POST /api/v1/files`, raw `GET /api/v1/files/{filePublicId}`, `DELETE /api/v1/files/{filePublicId}` |
| Notifications | `/api/v1/notifications`, `/api/v1/notifications/{notificationPublicId}/read` |
| Support access | `/api/v1/support/access/start`, `/api/v1/support/access/current`, `/api/v1/support/access/end` |
| External bearer | `GET /api/external/v1/me` |
| SCIM | `/api/scim/v2/Users`, `/api/scim/v2/Users/{id}` |
| M2M | `GET /api/m2m/v1/self` |

OpenAPI には出ないが runtime に存在する endpoint:

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

## Database と sqlc

DB は認証・認可・provisioning・tenant・M2M・業務 CRUD・共通機能・横断拡張・file lifecycle を扱える状態まで進んでいる。

| migration | 内容 |
| --- | --- |
| `0001_init` | `pgcrypto`、`users`、local password |
| `0002_user_identities` | 外部 IdP identity |
| `0003_roles` | global role / user role |
| `0004_downstream_grants` | delegated auth refresh token |
| `0005_provisioning` | SCIM/provisioning state |
| `0006_org_tenants` | tenants、memberships、tenant role overrides |
| `0007_machine_clients` | machine clients |
| `0008_todos` | tenant 共有 TODO |
| `0009_audit_events` | audit log |
| `0010_tenant_admin_role` | tenant admin role |
| `0011_customer_signals` | Customer Signals |
| `0012_web_service_common` | outbox、idempotency、notifications、invitations、files、tenant settings、data exports |
| `0013_p10_cross_cutting_extensions` | webhooks、import jobs、saved filters、support access、feature definitions、tenant entitlements |
| `0014_file_lifecycle_physical_delete` | file body purge state、retry/lock columns |

sqlc は `backend/sqlc.yaml` で、`db/schema.sql` と `db/queries/` を入力にして `backend/internal/db/` を生成する。UUID は `github.com/google/uuid.UUID` に寄せられている。

設計方針:

- tenant-aware table は `tenant_id` で絞る。
- tenant 外 record の存在は基本的に `404` として隠す。
- soft delete は user-facing delete、purge は storage body deletion として分ける。
- outbox と file purge の claim は `FOR UPDATE SKIP LOCKED` を使う。
- metrics label に tenant id、user id、email、public id、idempotency key、storage key、webhook URL は入れない。
- audit metadata、log、metrics に secret / token / raw signature は出さない。

## Runtime の構成

`backend/cmd/main/main.go` は、PostgreSQL、Redis、各 service、Gin/Huma router、scheduler、outbox worker、data lifecycle job、frontend route を構成する。build tag なしでは frontend は embed されず、backend 単体開発や OpenAPI export は frontend dist に依存しない。

`backend/internal/app/app.go` では、Gin middleware と Huma API を一つの router に載せている。

| レイヤ | 主な責務 |
| --- | --- |
| Gin route / middleware | request id、security headers、body limit、CORS、rate limit、request logging、recovery、docs auth、external/SCIM/M2M auth |
| Huma | request/response schema、OpenAPI、operation registration、security schemes |
| Service | session、OIDC、identity、role/tenant authz、audit、tenant admin、domain CRUD、outbox、files、entitlements、support access |
| Auth package | Redis stores、Cookie、OIDC/OAuth client、JWT verifier、M2M verifier、AES-GCM secret box |
| Jobs | provisioning reconcile、outbox worker、data lifecycle/file purge |

P11 により、browser API の rate limit は active tenant の `rateLimitBrowserApiPerMinute` を runtime に反映する。override がない場合、active tenant がない場合、session/settings lookup に失敗した場合は config default に戻る。support access 中の bucket は impersonated user ではなく actor user を requester とする。

P12 により、retention を過ぎた soft-deleted local file body は data lifecycle job で purge される。DB row は hard delete せず tombstone として残し、`purged_at`、`purge_attempts`、lock、last error を記録する。

## 認証と認可

実装済み:

- local password login と Zitadel browser login。
- Cookie session、CSRF token、session refresh、logout。
- Redis session / CSRF / OIDC state / delegation state。
- provider group grammar `tenant:<slug>:<role>` から tenant membership を同期する。
- tenant role override の DB と解決ロジック。
- external bearer API。
- downstream delegated auth。
- SCIM user provisioning。
- machine client CRUD と M2M bearer middleware。
- support access は global role `support_agent`、理由、期限、明示 banner、audit を前提にする。

注意点:

- tenant selector はログイン中 user が所属する active tenant 候補だけを扱う。
- tenant 自体の管理は `/api/v1/admin/tenants` 以下と tenant admin UI に分離している。
- M2M / external bearer 用の TODO / Customer Signals API はまだない。

## Frontend

frontend は Vue 3 + Vite + TypeScript + Pinia + Vue Router で構成されている。generated SDK は `openapi/browser.yaml` 由来で、`frontend/src/api/client.ts` が Cookie、CSRF、idempotency key を含む共通 transport を持つ。

実装済み:

- Login 画面は auth settings を見て local password form と Zitadel login link を切り替える。
- App header は authenticated user に tenant selector と主要 navigation を表示する。
- Home、Integrations、Notifications、Invitation accept、TODO、Customer Signals、Tenant Admin、Machine Clients の画面がある。
- Customer Signals は list/create/detail/update/delete、search、cursor pagination、saved filters、attachment upload/download/delete を扱う。
- Tenant Admin detail は memberships、invitations、settings、entitlements、support access、webhooks、tenant exports、Customer Signals import/export を扱う。
- Support access 中は banner を表示し、終了操作を提供する。
- role 不足時は blank ではなく `AdminAccessDenied` による 403 UI を出す。
- Vite dev server は `/api`, `/openapi`, `/docs` を backend `127.0.0.1:8080` に proxy する。
- production build output は `backend/web/dist` へ出力され、Go binary に embed される。

P9 により、single binary に対する Playwright E2E がある。対象は login、tenant 切り替え、Customer Signals、file upload、saved filter、Tenant Admin settings/entitlements/invitation/export/import、Notifications、role 不足 UI、SPA fallback である。

## 単一バイナリ配信

単一バイナリ配信は実装済みである。

```sh
make binary
```

実体は次の build に相当する。

```sh
npm --prefix frontend run build
CGO_ENABLED=0 go build -buildvcs=false -trimpath -tags "embed_frontend nomsgpack" -ldflags "-s -w -buildid=" -o ./bin/haohao ./backend/cmd/main
```

重要な挙動:

- `/`, `/login`, `/integrations`, `/notifications`, `/invitations/accept`, `/todos`, `/customer-signals`, `/tenant-admin`, `/machine-clients` などの SPA route は `index.html` に fallback する。
- `/assets/*`, `/favicon.svg`, `/icons.svg` は frontend build artifact を返す。
- `/api/*`, `/docs`, `/schemas/*`, `/openapi.yaml`, `/openapi.json`, `/openapi-3.0.yaml`, `/openapi-3.0.json`, `/healthz`, `/readyz`, `/metrics` は SPA fallback しない。
- 見つからない `/assets/*` や拡張子付き path は `index.html` ではなく `404` を返す。
- build tag なしの `go test ./backend/...` や `go run ./backend/cmd/openapi` は frontend dist 不在でも壊れない。

## CI と生成物

CI は次の品質ゲートを持っている。

| ゲート | 内容 |
| --- | --- |
| backend test | `go test ./backend/...` |
| frontend build | `npm --prefix frontend run build` |
| embedded binary build | `CGO_ENABLED=0 go build ... -tags "embed_frontend nomsgpack"` |
| UI E2E | `make e2e` |
| generated drift | sqlc / full OpenAPI / browser OpenAPI / external OpenAPI / frontend SDK |
| DB schema drift | migrations 適用後の `db/schema.sql` 差分検知 |
| OpenAPI validate | 3 spec の存在、OpenAPI 3.1、surface boundary grep |
| Zitadel config | `dev/zitadel/docker-compose.yml` の config check |
| Docker build | `docker build -t haohao:ci -f docker/Dockerfile .` |
| whitespace | `git diff --check` |

生成物として扱うもの:

- `db/schema.sql`
- `backend/internal/db/*`
- `openapi/openapi.yaml`
- `openapi/browser.yaml`
- `openapi/external.yaml`
- `frontend/src/api/generated/*`
- `frontend/package-lock.json`

通常 commit しない local artifact:

- `backend/web/dist/*`
- `bin/haohao`
- `.data/*`
- Docker image
- Playwright report / test results

## 検証済み事項

2026-04-26 の現状確認で実行済み:

```sh
go test ./backend/...
npm --prefix frontend run build
go run ./backend/cmd/openapi -surface=full
go run ./backend/cmd/openapi -surface=browser
go run ./backend/cmd/openapi -surface=external
make e2e
make smoke-file-purge
```

加えて、一時 single binary server に対して次の smoke 本体が `ok` を返した。

```sh
scripts/smoke-common-services.sh
scripts/smoke-p10.sh
scripts/smoke-rate-limit-runtime.sh
```

結果:

- backend test は成功。
- frontend build は成功。
- full / browser / external OpenAPI 再生成結果は tracked artifact と差分なし。
- `make e2e` は Playwright 3 tests passed。
- `make smoke-file-purge` は upload -> soft delete -> retention -> body deletion -> `purged_at` 記録を確認して成功。
- common services / P10 / rate limit runtime smoke は成功。
- Docker build は今回の現状確認では実行していない。

## 残課題

| 課題 | 影響 | 推奨対応 |
| --- | --- | --- |
| SMTP / provider email sender がない | invitation / notification email は log sender 中心 | SMTP、SES、SendGrid など provider を選んで `EmailSender` 実装を追加する |
| DB query span の詳細 instrumentation が薄い | trace で遅い query の特定がしづらい | pgx instrumentation または service-level span を追加する |
| Grafana dashboard がない | metrics はあるが標準 dashboard がない | `ops/` に dashboard JSON または Jsonnet を追加する |
| webhook delivery の外部実配送 E2E が薄い | HMAC signature、retry、response preview の browser/worker 結合回帰を見落としやすい | `RUN_WEBHOOK_SMOKE=1` 用の local receiver を CI で走らせる |
| support access E2E が薄い | banner、actor/impersonated audit、終了 flow の UI 回帰を拾いにくい | Playwright に support access start/current/end flow を追加する |
| object storage driver がない | local filesystem 以外の本番 file storage に未対応 | S3/GCS driver、signed download、retention/purge policy を追加する |
| file security processing がない | 大量 file や untrusted file の安全性が弱い | virus scan、content inspection、async processing を設計する |
| billing / subscription がない | plan/entitlement はあるが請求と連動しない | pricing plan、subscription、invoice、tenant entitlement sync を追加する |
| realtime notification がない | UI は polling/手動更新中心 | SSE/WebSocket などを要件が固まった時点で追加する |
| HA / DR / multi-region が未設計 | single region 前提の運用になる | backup/restore drill の先に RPO/RTO、replica、failover を設計する |
| M2M / external bearer の業務 API がない | browser session 以外から業務データを扱えない | scope / tenant 解決方針を決めて必要な業務 API だけ追加する |

## 当時の判断メモ

2026-04-25 の初期調査では、P3 から P7 までを次の優先順位で進める判断を置いていた。

1. P3: 監査ログ
2. P4: metrics / tracing / alerting
3. P5: tenant 管理 UI
4. P6: 業務ドメイン拡張
5. P7: Web サービス共通機能

この判断は当時妥当だったが、現在は P3-P7 に加えて P8-P12 も実装済みである。したがって、今後の最短経路は OpenAPI 分割や Playwright 導入ではなく、provider integration、運用 dashboard、外部配送/impersonation の E2E、object storage、billing、HA/DR の具体化である。

また、当時は「OpenAPI spec 分割は今すぐ不要」とする判断もあったが、P8 で full / browser / external の 3 artifact に分割済みである。現在の正本は Huma operation registration と generation command であり、YAML を手編集しない方針は維持する。

## 結論

HaoHao は、認証・認可・provisioning・tenant・M2M・単一バイナリ配信に加えて、operability、observability、audit、tenant admin、Customer Signals、common services、OpenAPI 分割、Playwright E2E、webhooks/import-export/saved filters/support access/entitlements、tenant rate limit runtime、file body purge まで到達している。

現在は、B2B SaaS や社内向け multi-tenant 業務 CRUD アプリの土台として、主要な縦切りが実装・検証済みである。次は、実サービスの要件に合わせて provider、billing、object storage、dashboard、HA/DR を足していく段階である。
