# HaoHao deep research report

調査日: 2026-04-25

基準資料:

- `IMPL.md`
- 現在の repository 実装
- `TUTORIAL.md`
- `TUTORIAL_ZITADEL.md`
- `TUTORIAL_SINGLE_BINARY.md`
- `TUTORIAL_P0_OPERABILITY.md`
- `TUTORIAL_P1_ADMIN_UI.md`
- `TUTORIAL_P2_TODO.md`
- `RUNBOOK_OPERABILITY.md`

## エグゼクティブサマリ

HaoHao は、当初の foundation tutorial を超えて、**Go/Huma/Gin backend + Vue/Vite frontend + PostgreSQL/sqlc + Redis session + Zitadel 連携**を中核にした multi-tenant Web application 基盤まで進んでいる。

現在の到達点は、単なる CRUD scaffold ではない。local password login、Zitadel OIDC browser login、Cookie session、CSRF、external bearer API、delegated OAuth grant、SCIM provisioning、tenant-aware auth、M2M bearer API、machine client 管理、OpenAPI 生成、frontend generated SDK、単一バイナリ配信、`scratch` Docker image、CI / release asset、P0 operability、P1 admin UI、P2 tenant-aware TODO まで実装済みである。

したがって現在の HaoHao は、B2B SaaS、社内管理ツール、tenant-aware な業務 CRUD アプリの基礎としては十分に使える段階にある。次の課題は foundation を作り直すことではなく、本番 Web サービスとして横断的に必要になる **監査ログ、metrics / tracing、tenant 管理、より具体的な業務ドメイン、Web サービス共通機能** を順に積むことにある。

優先順位は「後から追加しづらく、複数機能に横断して効き、欠けると事故調査や運用が難しくなるもの」を上に置く。具体的には、次の順番を推奨する。

1. P3: 監査ログ
2. P4: metrics / tracing / alerting
3. P5: tenant 管理 UI
4. P6: 業務ドメイン拡張
5. P7: Web サービス共通機能

## 現在地

| 領域                  | 状態                                                                                                             |
| --------------------- | ---------------------------------------------------------------------------------------------------------------- |
| Monorepo              | 実装済み。repo root に `go.work`、`backend/`、`frontend/`、`db/`、`openapi/` がある                              |
| Backend               | Go module は `backend/`。Gin + Huma で API を構成                                                                |
| OpenAPI               | Huma から `openapi/openapi.yaml` を生成。OpenAPI は `3.1.0`                                                      |
| Frontend              | Vue 3 + Vite + TypeScript + Pinia + Vue Router                                                                   |
| Generated SDK         | `@hey-api/openapi-ts` で `frontend/src/api/generated/` を生成                                                    |
| Database              | PostgreSQL 18 前提。migration は `0008_todos` まで進んでいる                                                     |
| Session               | Redis に session / CSRF / OIDC state / delegation state を保存                                                   |
| Browser auth          | local password login と Zitadel OIDC login の両対応                                                              |
| External API          | OIDC JWKS 検証付き bearer API がある                                                                             |
| SCIM                  | User create/list/get/replace/patch/delete subset がある                                                          |
| Tenant                | tenant membership、active tenant session、tenant role がある                                                     |
| M2M                   | machine client table、管理 API/UI、M2M bearer middleware、self endpoint がある                                   |
| Single binary         | Vue production build を Go binary に embed して API と SPA を同一プロセスで返せる                                |
| Operability           | structured request logging、request id、health/readiness、SCIM reconcile scheduler、smoke script、runbook がある |
| Admin UI              | tenant selector、integrations UX、machine client admin UI、docs access check がある                              |
| Business domain       | tenant-aware TODO の schema / API / frontend がある                                                              |
| Docker / CI / release | `scratch` runtime image、CI、OpenAPI artifact、release binary がある                                             |

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
│  ├─ internal/jobs/            # reconcile scheduler
│  ├─ internal/middleware/      # request id/logging, docs/external/SCIM/M2M middleware
│  ├─ internal/platform/        # slog logger and readiness helpers
│  └─ internal/service/         # session, identity, authz, delegation, provisioning, machine clients, TODO
├─ frontend/
│  ├─ src/
│  ├─ src/api/generated/        # openapi-ts generated client
│  └─ vite.config.ts            # build output: ../backend/web/dist
├─ db/
│  ├─ migrations/
│  ├─ queries/
│  └─ schema.sql
├─ openapi/openapi.yaml
├─ docker/Dockerfile
└─ Makefile
```

重要なのは、frontend build output を repository の正本にしていない点である。`backend/web/dist/` は生成物であり、production binary を作る直前に `npm --prefix frontend run build` で生成し、`embed_frontend` build tag 付きの Go binary に埋め込む。

## Runtime の構成

`backend/cmd/main/main.go` は、PostgreSQL、Redis、各 service、Gin/Huma router、SCIM reconcile scheduler、frontend route を構成する。build tag なしでは frontend は embed されず、backend 単体開発や OpenAPI export は frontend dist に依存しない。

`backend/internal/app/app.go` では、Gin middleware と Huma API を一つの router に載せている。

| レイヤ                 | 主な責務                                                                                                               |
| ---------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| Gin route / middleware | request id、structured request logging、recovery、health/readiness、docs auth、external CORS/auth、SCIM auth、M2M auth |
| Huma                   | request/response schema、OpenAPI、operation registration、security schemes                                             |
| Service                | session、OIDC login、identity linking、role/tenant authz、delegation、provisioning、machine client、TODO               |
| Auth package           | Redis-backed stores、Cookie、OIDC/OAuth client、JWT verifier、refresh token encryption                                 |

OpenAPI security schemes は `cookieAuth`、`bearerAuth`、`m2mBearerAuth` の 3 種類で、現状は `openapi/openapi.yaml` 一つに browser / external / SCIM / M2M の surface が含まれている。外部 API の公開範囲が大きくなった時点で、browser spec と external spec の分割を検討すればよい。

## API surface

現在 OpenAPI に出ている主な endpoint は次の通り。

| 区分                 | endpoint                                                                                                                                                                                                                                         |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Browser auth/session | `GET /api/v1/auth/settings`, `GET /api/v1/auth/login`, `GET /api/v1/auth/callback`, `POST /api/v1/login`, `GET /api/v1/session`, `GET /api/v1/csrf`, `POST /api/v1/session/refresh`, `POST /api/v1/logout`                                       |
| Tenant               | `GET /api/v1/tenants`, `POST /api/v1/session/tenant`                                                                                                                                                                                             |
| Delegated auth       | `GET /api/v1/integrations`, `GET /api/v1/integrations/{resourceServer}/connect`, `GET /api/v1/integrations/{resourceServer}/callback`, `POST /api/v1/integrations/{resourceServer}/verify`, `DELETE /api/v1/integrations/{resourceServer}/grant` |
| External bearer      | `GET /api/external/v1/me`                                                                                                                                                                                                                        |
| SCIM                 | `/api/scim/v2/Users`, `/api/scim/v2/Users/{id}`                                                                                                                                                                                                  |
| Machine clients      | `GET /api/v1/machine-clients`, `POST /api/v1/machine-clients`, `GET /api/v1/machine-clients/{id}`, `PUT /api/v1/machine-clients/{id}`, `DELETE /api/v1/machine-clients/{id}`                                                                     |
| TODO                 | `GET /api/v1/todos`, `POST /api/v1/todos`, `PATCH /api/v1/todos/{todoPublicId}`, `DELETE /api/v1/todos/{todoPublicId}`                                                                                                                           |
| M2M                  | `GET /api/m2m/v1/self`                                                                                                                                                                                                                           |

OpenAPI には出ないが runtime に存在する endpoint:

- `GET /healthz`
- `GET /readyz`

TODO API は browser session + Cookie + CSRF の tenant-aware CRUD として実装されている。active tenant が必須で、active tenant に tenant role `todo_user` がない場合は `403` になる。tenant 外または見つからない TODO は `404` として扱い、他 tenant の存在を漏らさない。

## Database と sqlc

DB は認証・認可・provisioning・tenant・machine client・TODO を扱える状態まで進んでいる。

| 領域               | 内容                                                                                                      |
| ------------------ | --------------------------------------------------------------------------------------------------------- |
| users / identities | local password user と外部 IdP identity を分離                                                            |
| roles              | global role と user role                                                                                  |
| downstream grants  | delegated auth refresh token を暗号化保存                                                                 |
| provisioning       | SCIM / provider sync state と deactivation                                                                |
| tenants            | tenant、membership、tenant role override、default tenant                                                  |
| machine clients    | M2M client 管理と allowed scopes                                                                          |
| todos              | tenant 共有 TODO。`public_id`、`tenant_id`、`created_by_user_id`、`title`、`completed`、timestamps を持つ |

sqlc は `backend/sqlc.yaml` で、`db/schema.sql` と `db/queries/` を入力にして `backend/internal/db/` を生成する。UUID は `github.com/google/uuid.UUID` に寄せられている。

TODO は tenant 共有の最初の業務 table である。list / get / update / delete は必ず `tenant_id` で絞り、tenant 外 TODO の存在は `404` として隠す。この方針は今後の業務 table でも踏襲するのがよい。

## 認証と認可

### Browser session

実装済み:

- local password login は `users.password_hash` と `crypt()` で検証する。
- `AUTH_MODE=zitadel` または `ENABLE_LOCAL_PASSWORD_LOGIN=false` の場合、password login は無効化される。
- session は Redis に保存する。
- `SESSION_ID` は HttpOnly Cookie。
- `XSRF-TOKEN` は frontend が読める Cookie。
- mutation 系 endpoint は `X-CSRF-Token` header を要求する。
- `GET /api/v1/csrf` で CSRF token を再発行できる。
- `POST /api/v1/session/refresh` で session ID と CSRF token を rotate できる。

### Zitadel browser login

実装済み:

- authorization code + PKCE + nonce。
- OIDC discovery と ID token 検証。
- Redis-backed login state。
- userinfo から email / name / groups を取得。
- `(provider, subject)` を正として `user_identities` へ紐付け。
- email verified の既存 user との identity linking。
- provider groups から global role と tenant membership を同期。

### External bearer / SCIM / M2M

実装済み:

- external bearer API は OIDC discovery の JWKS で JWT 署名、issuer、audience、scope prefix を検証する。
- `/api/external/` は bearer token middleware を通る。
- SCIM は bearer token の audience / scope を検証し、user provisioning を実行する。
- M2M は human user claim を持つ token を拒否し、`client_id` / `azp` を local `machine_clients` と照合する。
- M2M token は allowed scopes と `M2M_REQUIRED_SCOPE_PREFIX` で制限する。

## Frontend

frontend は Vue 3 + Vite + TypeScript + Pinia + Vue Router で構成されている。generated SDK は `frontend/src/api/generated/` にあり、`frontend/src/api/client.ts` が共通 transport を持つ。

実装済み:

- Login 画面は `GET /api/v1/auth/settings` を見て local password form と Zitadel login link を切り替える。
- App header は authenticated user に tenant selector を表示する。
- Home 画面は current session 表示、session refresh、logout、docs link、TODO 導線を持つ。
- Integrations 画面は delegated auth の list/connect/verify/revoke を呼べる。
- Machine clients 画面は list/create/detail/update/delete を提供する。
- TODO 画面は list/create/update/delete、完了 toggle、active tenant 切り替えへの追従を提供する。
- role 不足時は blank ではなく `AdminAccessDenied` による 403 UI を出す。
- fetch は `credentials: 'include'` で Cookie を送る。
- mutation 前に `XSRF-TOKEN` Cookie を読み、`X-CSRF-Token` header を付ける。
- `XSRF-TOKEN` が無い場合は `GET /api/v1/csrf` を先に呼ぶ。
- Vite dev server は `/api`, `/openapi`, `/docs` を backend `127.0.0.1:8080` に proxy する。
- production build output は `backend/web/dist` へ出力され、Go binary に embed される。

注意点:

- tenant selector はログイン中 user の tenant membership だけを表示する。
- tenant 自体の作成、membership 管理、tenant role 付与の UI はまだない。
- `todo_user` は TODO API では global role としては見ない。active tenant の tenant role だけを見る。
- root `.env` が `AUTH_MODE=zitadel` の場合、demo password login は `501` になる。local demo smoke では `AUTH_MODE=local ENABLE_LOCAL_PASSWORD_LOGIN=true` を起動時に上書きする。

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

- `/`, `/login`, `/integrations`, `/machine-clients`, `/todos` は SPA fallback として `index.html` を返す。
- `/assets/*`, `/favicon.svg`, `/icons.svg` は frontend build artifact を返す。
- `/api/*`, `/docs`, `/schemas/*`, `/openapi.yaml`, `/openapi.json`, `/openapi-3.0.yaml`, `/openapi-3.0.json` は SPA fallback しない。
- 見つからない `/assets/*` や拡張子付き path は `index.html` ではなく `404` を返す。
- build tag なしの `go test ./backend/...` や `go run ./backend/cmd/openapi` は frontend dist 不在でも壊れない。

サイズ削減も入っている。

| 設定                         | 目的                                                                                 |
| ---------------------------- | ------------------------------------------------------------------------------------ |
| `CGO_ENABLED=0`              | static binary 化し、`scratch` runtime で動かす                                       |
| `nomsgpack`                  | Gin の未使用 msgpack binding を外し、binary size と compile memory pressure を下げる |
| `-buildvcs=false`            | VCS metadata を binary に入れない                                                    |
| `-trimpath`                  | local path 情報を削る                                                                |
| `-ldflags "-s -w -buildid="` | symbol table、DWARF debug 情報、build id を削る                                      |

確認済みの値:

- local `darwin/arm64` binary: 約 15 MB
- 変更前の debug 情報付き binary: 約 21 MB
- Docker image: 約 20 MB
- `docker history`: `/haohao` layer は約 14.6 MB、CA bundle は約 242 kB

## Docker と release

`docker/Dockerfile` は multi-stage build で、frontend build と backend embedded binary build を container 内で完結させる。

| stage         | 内容                                                               |
| ------------- | ------------------------------------------------------------------ |
| `node:24`     | `npm --prefix frontend run build`                                  |
| `golang:1.26` | `embed_frontend nomsgpack` build tag 付きで `/tmp/haohao` を build |
| `scratch`     | CA bundle と `/haohao` だけを copy                                 |

runtime image には shell も package manager もない。調査が必要な場合は production image に入るのではなく、debug image か builder stage を使う。

release workflow は tag push 時に frontend build、embedded Linux amd64 binary build、tarball 化、GitHub Release upload を行う。現状の release asset は `linux/amd64` が中心で、複数 OS / architecture が必要になった時点で matrix 化すればよい。

## Operability

P0 operability は実装済みである。

| 領域               | 現状                                                                                                      |
| ------------------ | --------------------------------------------------------------------------------------------------------- |
| request id         | `X-Request-ID` の受け取り / 生成 / response header 設定                                                   |
| structured logging | Go 標準 `log/slog`。`LOG_LEVEL` と `LOG_FORMAT` で制御                                                    |
| request logging    | `request_id`, `method`, `path`, `status`, `latency_ms`, `client_ip`, `user_agent` を出す                  |
| liveness           | `/healthz` は process liveness として `200 {"status":"ok"}` を返す                                        |
| readiness          | `/readyz` は PostgreSQL / Redis を ping し、設定時だけ Zitadel discovery も確認                           |
| scheduler          | `ProvisioningReconcileJob.RunOnce(ctx)` を `time.Ticker` で interval 実行                                 |
| smoke              | `scripts/smoke-operability.sh` と `make smoke-operability`                                                |
| runbook            | `RUNBOOK_OPERABILITY.md` に binary / Docker deploy、rollback、Zitadel redirect URI、smoke test 手順がある |

`/healthz` / `/readyz` は Huma operation ではなく Gin route である。OpenAPI には出ない。

## `.env` と frontend URL

単一バイナリは `make backend-dev` と違い、shell が `.env` を source してくれるとは限らない。そのため backend config loader は補助として `.env` を任意で読む。

読み込み候補:

- カレントディレクトリの `.env`
- 実行ファイルと同じ directory の `.env`

既に設定されている環境変数は `.env` で上書きしない。これにより、local では `bin/.env` を置いて `cd bin && ./haohao` でき、本番では Docker/Kubernetes/systemd/secret manager から渡した値を優先できる。

embedded frontend build では、開発用の `FRONTEND_BASE_URL=http://127.0.0.1:5173` または `http://localhost:5173` が残っていても、frontend URL と post logout redirect は `APP_BASE_URL` 側へ補正される。これは、単一バイナリでは Vite dev server ではなく Go process が frontend も返すためである。

本番形では、補正に頼らず次の値を同一 origin にそろえる。

```dotenv
APP_BASE_URL=https://app.example.com
FRONTEND_BASE_URL=https://app.example.com
ZITADEL_REDIRECT_URI=https://app.example.com/api/v1/auth/callback
ZITADEL_POST_LOGOUT_REDIRECT_URI=https://app.example.com/login
```

## CI と生成物

CI は次の品質ゲートを持っている。

| ゲート                | 内容                                                          |
| --------------------- | ------------------------------------------------------------- |
| backend test          | `go test ./backend/...`                                       |
| frontend build        | `npm --prefix frontend run build`                             |
| embedded binary build | `CGO_ENABLED=0 go build ... -tags "embed_frontend nomsgpack"` |
| generated drift       | sqlc / OpenAPI / frontend SDK の差分検知                      |
| DB schema drift       | `db/schema.sql` の drift 検知                                 |
| OpenAPI validate      | OpenAPI artifact の検証                                       |
| Zitadel config        | `dev/zitadel/docker-compose.yml` の config check              |
| Docker build          | `docker build -t haohao:ci -f docker/Dockerfile .`            |
| operability script    | `bash -n scripts/smoke-operability.sh`                        |

生成物として扱うもの:

- `db/schema.sql`
- `backend/internal/db/*`
- `openapi/openapi.yaml`
- `frontend/src/api/generated/*`
- `frontend/package-lock.json`
- `backend/web/dist/*`
- `bin/haohao`
- Docker image `haohao:dev`
- release asset `haohao-linux-amd64.tar.gz`

`backend/web/dist/*` と `bin/haohao` は local build artifact であり、通常は commit しない。

## 検証済み事項

実行済み:

```sh
make db-up
make db-schema
cd backend && sqlc generate
go run ./backend/cmd/openapi > /tmp/haohao-openapi.yaml
/usr/bin/diff -u openapi/openapi.yaml /tmp/haohao-openapi.yaml
npm --prefix frontend run openapi-ts
go test ./backend/...
bash -n scripts/smoke-operability.sh
npm --prefix frontend run build
make seed-demo-user
make binary
make smoke-operability
docker build -t haohao:dev -f docker/Dockerfile .
```

結果:

- backend test は成功。
- frontend build は成功し、`backend/web/dist/` に出力された。
- embedded binary build は成功。
- `make binary` は成功。
- Docker build は成功。
- OpenAPI 再生成結果は tracked artifact と差分なし。
- sqlc 再生成後の差分なし。
- frontend SDK 再生成後の差分なし。
- operability smoke は `http://127.0.0.1:8080` に対して成功。
- binary smoke では `/`, `/login`, `/integrations`, `/machine-clients`, `/todos` が SPA fallback で HTML を返すことを確認済み。
- `/api/v1/session` と未ログインの `/api/v1/todos` は `401 application/problem+json` を返す。
- `/openapi.yaml` は OpenAPI YAML を返す。
- `/assets/missing.js` は `404` を返す。
- P1 browser smoke では tenant selector の Acme / Beta 切り替え、Integrations の tenant 追従、machine client の作成 / detail / update / disable、role 不足時の 403 UI、docs link access check を確認済み。
- P2 API smoke では local mode で `demo@example.com` / `changeme123` に login し、TODO create / patch / delete と Acme / Beta の tenant 分離を確認済み。
- P2 browser smoke では `/todos` 画面で TODO 作成、完了 toggle、tenant selector による Acme / Beta 切り替え、削除を確認済み。

## 残課題

| 課題                                    | 影響                                                                                              | 推奨対応                                                                              |
| --------------------------------------- | ------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------- |
| 監査ログがない                          | admin 操作、tenant 変更、machine client 操作、業務 CRUD の証跡を後から追えない                    | 重要 mutation に audit event を保存する                                               |
| metrics / tracing / alerting がない     | latency、error rate、DB/Redis/Zitadel 依存、scheduler 失敗を継続観測しづらい                      | Prometheus metrics と OpenTelemetry trace の導入を検討する                            |
| tenant 管理 UI がない                   | tenant 作成、membership 管理、tenant role 付与を UI で自走できない                                | tenant admin role 前提の管理 UI/API を追加する                                        |
| 業務ドメインが TODO に留まっている      | 基盤の縦切り確認は済んだが、実用的なサービス価値はまだ薄い                                        | Customer Signals / Product Decisions などの業務モデルを追加する                       |
| Web サービス共通機能が薄い              | file upload、email、background job、rate limit などが必要なサービスにすぐ展開できない             | 共通機能を小さく追加し、tenant-aware な実装規約を固定する                             |
| M2M / external bearer の業務 API がない | browser session 以外から業務データを扱えない                                                      | 必要になった時点で scope / tenant 解決方針を決めて追加する                            |
| E2E test が薄い                         | login、tenant 切り替え、role 不足 UI、single binary SPA fallback の回帰を UI レベルで検知しづらい | Playwright smoke を追加する                                                           |
| deploy IaC / secret management がない   | 実配備先ごとの構築が手作業になりやすい                                                            | 対象 platform を決めて Terraform / compose / systemd / Kubernetes manifest を追加する |

## 次の実装優先順位

### P3: 監査ログ

最優先は監査ログである。

理由は、multi-tenant、admin 操作、machine client、role 変更、業務 CRUD が入った時点で「誰が、どの tenant で、何を、いつ変更したか」を失うのが最も危険だからである。監査ログは後から schema や UI を足すことはできても、過去に発生した操作の証跡は復元できない。したがって、機能追加を広げる前に入れる価値が高い。

対象:

- login / logout / session refresh の重要 event
- active tenant 切り替え
- delegated grant の connect / verify / revoke
- machine client の create / update / disable / delete
- TODO CRUD
- 将来の業務 CRUD
- tenant / membership / role 変更

完了条件:

- audit event table がある。
- actor user、tenant、action、target type/id、request id、client ip、user agent、occurred_at が保存される。
- mutation service から一貫した helper 経由で記録される。
- 監査ログの失敗方針が明文化されている。

### P4: metrics / tracing / alerting

次は metrics / tracing / alerting である。

理由は、P0 で structured log と readiness は入っているが、本番運用では latency、error rate、DB/Redis/Zitadel 依存の異常、scheduler の失敗を時系列で継続観測できる必要があるからである。監査ログがプロダクトデータの証跡なら、metrics / tracing は運用品質の証跡であり、どちらも全機能に横断して効く。

P3 の次に置く理由は、監査ログは失われると過去分を復元できない一方、metrics / tracing は導入後から観測を開始できるためである。ただし、本番運用前には必須に近い。

対象:

- HTTP request count / latency / status count
- DB / Redis ping latency
- readiness failure count
- SCIM reconcile run count / failure count / duration
- external bearer / M2M auth failure count
- OpenTelemetry trace propagation
- alert rule の初期セット

完了条件:

- `/metrics` または platform に合った exporter がある。
- request id と trace id の対応が log で追える。
- local / single binary / Docker で metrics を確認できる。
- alert の最小セットが runbook に書かれている。

### P5: tenant 管理 UI

次は tenant 管理 UI である。

理由は、tenant selector は既にあるが、tenant 作成、membership 追加/削除、tenant role 付与を UI で扱えないと B2B SaaS として自走しづらいからである。現状は provider claim / SCIM group 由来の tenant provisioning に強く、アプリ内管理者が tenant を調整する導線が弱い。

P4 の後に置く理由は、tenant / role 変更 UI は事故時の影響が大きく、監査と観測なしに広げると調査が難しくなるためである。

対象:

- tenant list / detail
- tenant create / update / deactivate
- tenant membership list / add / remove
- tenant role grant / revoke
- tenant admin role の導入
- 403 UI と role 不足時の導線

完了条件:

- tenant admin が UI から tenant と membership を管理できる。
- tenant role 変更は監査ログに残る。
- active tenant selector と tenant 管理 UI の責務が分かれている。

### P6: 業務ドメイン拡張

次は TODO を超えた業務ドメインの拡張である。

理由は、P2 TODO で browser session + CSRF + active tenant role + generated SDK + SPA fallback の縦切りは確認済みだからである。次は Customer Signals / Product Decisions のような実用寄りの業務モデルを追加し、HaoHao を「認証基盤」から「実際に使うアプリ」へ進める段階である。

P5 の後に置く理由は、tenant / role 管理が整うと、複数業務ドメインを追加しても運用しやすいからである。

候補:

- Customer Signals: 顧客要望、問い合わせ、商談メモ、重要度、状態管理。
- Product Decisions: 意思決定ログ、背景、選択肢、決定者、関連 signal。
- Lightweight CRM: account、contact、activity、note。
- Internal approvals: request、approval step、comment、audit trail。

完了条件:

- tenant-aware な業務 table が TODO とは別にある。
- role / permission が業務単位で整理されている。
- list/detail/create/update/delete の UI がある。
- audit log、metrics、E2E smoke の対象に入る。

### P7: Web サービス共通機能

最後に、様々な Web サービスへ展開するための共通機能を積む。

理由は、file upload、email、notification、background job / outbox、rate limit、E2E test、deployment IaC は多くの Web サービスで必要になる一方、サービス内容によって必要度が変わるためである。P3-P6 で運用・管理・業務の骨格を固めた後、利用シナリオに合わせて足すのがよい。

推奨順:

1. background job / outbox
2. email / notification
3. file upload
4. rate limit
5. Playwright E2E
6. deployment IaC / secret management

この順番にする理由は、background job / outbox が email、notification、file processing、external sync の土台になるためである。rate limit は public exposure が増える前に必要になる。Playwright E2E は画面数が増えた段階で効果が大きい。IaC は配備先が固まってから具体化する方が無駄が少ない。

## 判断メモ

### 今の基盤で作りやすい Web サービス

HaoHao は、次のようなサービスの基礎として特に向いている。

- B2B SaaS の MVP
- 社内管理ツール
- tenant-aware な業務 CRUD
- Zitadel 前提のログイン付きアプリ
- Cookie session + CSRF の browser app
- M2M API を少し持つ管理サービス
- single binary で配る小規模アプリ

逆に、決済・請求、大量ファイル、リアルタイム通信、高トラフィック public API、複数 region 高可用性は、現時点の土台だけではまだ足りない。これらは P7 以降で個別に設計するのがよい。

### Spec 分割は今すぐ不要

古いレポートでは browser API と external API の OpenAPI spec 分割を推奨していた。しかし現状の external API は小さく、SCIM / M2M も同一 Huma app に載っている。今は単一 `openapi/openapi.yaml` を正本にし、operation tag と security scheme で整理する方が保守しやすい。

### `.env` loader は direct binary 実行に必要

`./haohao` は `make backend-dev` と違い `.env` を自動で source しない。実行ファイル横の `.env` を読む実装は、single binary 配布の実用性に直結している。既存環境変数を優先する挙動も、本番運用と相性がよい。

### `scratch` runtime は妥当

現在の runtime image は shell を持たないため調査性は落ちるが、配布サイズと attack surface の点では妥当である。HTTPS outbound access が必要な OIDC / OAuth 連携のため、CA bundle だけは残す判断も正しい。

### UPX は現時点では不要

binary size は local `darwin/arm64` で約 15 MB、Docker image は約 20 MB まで下がっている。UPX を使えばさらに小さくできる可能性はあるが、起動時間、debuggability、脆弱性 scanner、環境差の tradeoff がある。現時点では Go 標準 toolchain の size flags と `scratch` runtime で十分である。

## 結論

HaoHao は、認証・認可・provisioning・tenant・M2M・単一バイナリ配信に加えて、P0 operability、P1 admin UI、P2 tenant-aware TODO まで到達している。今から大きな新設計に戻る必要はない。

次の最短経路は、監査ログ、metrics / tracing、tenant 管理 UI を入れて、複数の Web サービスに共通する運用・管理の基礎を固めること。その後に Customer Signals / Product Decisions などの実用的な業務ドメインを追加すると、HaoHao は認証基盤から実アプリ基盤へ自然に進められる。

## 追記: P3-P7 実装後の現在地と次アクション

この追記は、P3 から P7 までを実装した後の現在地を整理し、次にやるべきことを決めるためのものである。上の本文は当時の調査記録として残し、ここでは実装後の差分を明確にする。

### P3-P7 の実装状況

| Phase | 主な内容 | 現在地 |
|-------|----------|--------|
| P3: 監査ログ | audit log、主要操作の audit event、metadata、request context | 実装済み |
| P4: metrics / tracing / alerting | `/metrics`、HTTP metrics、dependency metrics、outbox / rate limit metrics、tracing middleware、runbook | 実装済み |
| P5: tenant 管理 UI | tenant 一覧 / 詳細 / 作成、membership grant / revoke、tenant admin role、403 UI | 実装済み |
| P6: 業務ドメイン拡張 | Customer Signals の tenant-aware CRUD、UI、audit、metrics | 実装済み |
| P7: Web サービス共通機能 | security hardening、outbox、idempotency、notification、invitation、file upload、tenant settings、data export、data lifecycle、backup / restore smoke | 実装済み |

P7 までで、shell smoke、API、DB schema、metrics、audit log の縦切りはかなり強くなった。`smoke-operability`、`smoke-observability`、`smoke-tenant-admin`、`smoke-customer-signals`、`smoke-common-services`、`smoke-backup-restore` によって、backend と運用系の主要な壊れ方は検知できる。

一方で、UI 回帰検知はまだ薄い。調査時点では次の状態である。

- `frontend/package.json` に Playwright dependency と `e2e` script がない。
- repository root に `playwright.config.ts` がない。
- repository root に `e2e/` directory がない。
- `Makefile` に `e2e` target がない。
- `TUTORIAL_P7_WEB_SERVICE_COMMON.md` には Playwright E2E の Step があるが、実装はまだ入っていない。

したがって、次に最も効果が高いのは **P7 UI の Playwright E2E チュートリアル作成 / 実装** である。

### 次の優先順位

1. P7 UI Playwright E2E チュートリアル作成 / 実装
2. P7.5 チュートリアル作成
3. tenant settings の rate limit runtime 連動
4. file lifecycle の物理削除
5. P7.5 / P8 候補の順次実装

#### 1. P7 UI Playwright E2E

最優先は、P7 で増えた UI と既存の login / tenant flow を browser レベルで確認することである。

最低限の E2E 対象は次にする。

- Login
- tenant 選択
- Customer Signals 作成
- file upload
- notifications 表示
- Tenant Admin settings 更新
- tenant data export request
- SPA fallback / role 不足 UI の最低限確認

理由は、現在の shell smoke は API と運用面には強いが、フォーム、route、generated SDK の呼び出し、Cookie / CSRF、画面遷移、確認 dialog、access denied 表示の回帰を直接見ないためである。P7 まで進んだ今は、画面数と権限分岐が増えており、ここを自動化する効果が大きい。

#### 2. P7.5 チュートリアル作成

P7.5 は、P7 の上に載せる横断機能を分離して設計するのがよい。P7 初期版に詰め込むより、outbox、audit、tenant settings、file storage、metrics が入った状態で独立 tutorial として扱う方が実装と検証が安定する。

候補は次である。

- outbound webhooks: 署名、retry、delivery log、dead letter
- import / export jobs: CSV import、CSV export、job status UI
- search / cursor pagination: Customer Signals の full-text search、cursor pagination、saved filters
- support access / impersonation: 理由入力、時間制限、明示 banner、audit
- feature flags / entitlements: tenant settings から独立させ、billing / pricing plan に接続できる形へ拡張

#### 3. tenant settings の rate limit runtime 連動

P7 では tenant settings に rate limit override の入口がある。一方で、middleware 側の runtime policy はまだ config ベースである。

次の実装では、tenant settings の browser API rate limit override を実際の rate limit decision に反映する。ただし、metrics label に tenant id、user id、email、idempotency key などの高 cardinality / sensitive value は入れない方針を維持する。

#### 4. file lifecycle の物理削除

現状は file metadata の soft delete と data lifecycle の入口が中心である。運用として締めるなら、retention 経過後に local storage 上の実 file body も削除する purge を追加する。

このとき、DB transaction と filesystem delete の順序、失敗時の retry、audit / metrics、削除対象の再確認を設計する必要がある。まずは local storage driver で実装し、S3 などの object storage driver は後続でよい。

### 推奨する直近作業

すぐ次に進むなら、**P7 UI の Playwright E2E チュートリアル作成 / 実装** を行うのがよい。

理由は、P7 の API / smoke / metrics / audit はすでに十分通っている一方、UI regression test が未整備だからである。Login から tenant 選択、Customer Signals、file upload、Tenant Admin settings / export までを 1 本の browser E2E として通すことで、P0-P7 の browser app と single binary 配信の回帰検知が一段強くなる。
