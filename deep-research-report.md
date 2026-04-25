# HaoHao deep research report

調査日: 2026-04-25

基準資料:

- `IMPL.md`
- 現在の repository 実装
- `TUTORIAL.md`
- `TUTORIAL_ZITADEL.md`
- `TUTORIAL_SINGLE_BINARY.md`

## エグゼクティブサマリ

HaoHao は、当初の foundation tutorial を超えて、**Go/Huma/Gin backend + Vue/Vite frontend + PostgreSQL/sqlc + Redis session + Zitadel 連携**を中核にした認証・認可基盤アプリとしてかなり進んでいる。

現在の到達点は、単なる CRUD scaffold ではない。local password login、Zitadel OIDC browser login、Cookie session、CSRF、external bearer API、delegated OAuth grant、SCIM provisioning、tenant-aware auth、M2M bearer API、machine client backend CRUD、OpenAPI 生成、frontend generated SDK、単一バイナリ配信、`scratch` Docker image、CI / release asset まで実装済みである。

一方で、実装済み backend surface を「運用できるプロダクト」に閉じるための最後の作業が残っている。優先度が高いのは、`ProvisioningReconcileJob` の scheduler 接続、tenant selector UI、machine client admin UI、cutover / rollback runbook、observability、structured logs、metrics、smoke test 手順の固定である。

古いレポートは「Customer Signals / Product Decisions アプリをこれから作る」前提だったが、現 repository はそのドメイン機能を実装していない。したがって本レポートでは、HaoHao の現在地を **認証・プロビジョニング・配布基盤の実装レビュー**として整理し、Customer Signals / Product Decisions は将来の業務ドメイン例に降格する。

## 現在地

| 領域          | 状態                                                                                                       |
| ------------- | ---------------------------------------------------------------------------------------------------------- |
| Monorepo      | 実装済み。repo root に `go.work`、`backend/`、`frontend/`、`db/`、`openapi/` がある                        |
| Backend       | Go module は `backend/`。Gin + Huma で API を構成                                                          |
| OpenAPI       | Huma から `openapi/openapi.yaml` を生成。OpenAPI は `3.1.0`                                                |
| Frontend      | Vue 3 + Vite + TypeScript + Pinia + Vue Router                                                             |
| Generated SDK | `@hey-api/openapi-ts` で `frontend/src/api/generated/` を生成                                              |
| Database      | PostgreSQL 18 前提。migration、`db/schema.sql`、sqlc 生成物がある                                          |
| Session       | Redis に session / CSRF / OIDC state / delegation state を保存                                             |
| Browser auth  | local password login と Zitadel OIDC login の両対応                                                        |
| External API  | OIDC JWKS 検証付き bearer API がある                                                                       |
| SCIM          | User create/list/get/replace/patch/delete subset がある                                                    |
| Tenant        | provider group / SCIM group 由来の tenant membership と active tenant session がある                       |
| M2M           | machine client table、backend CRUD、M2M bearer middleware、self endpoint がある                            |
| Single binary | Vue production build を Go binary に embed して API と SPA を同一プロセスで返せる                          |
| Docker        | `scratch` runtime image。CA bundle と `/haohao` binary のみ                                                |
| CI / release  | backend test、frontend build、embedded binary build、Docker build、OpenAPI artifact、release binary がある |

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
│  ├─ internal/middleware/      # docs/external/SCIM/M2M middleware
│  └─ internal/service/         # session, identity, authz, delegation, provisioning, machine clients
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

`backend/cmd/main/main.go` は、PostgreSQL、Redis、各 service、Gin/Huma router を構成し、最後に frontend route を登録する。build tag なしでは frontend は embed されず、backend 単体開発や OpenAPI export は frontend dist に依存しない。

`backend/internal/app/app.go` では、Gin middleware と Huma API を一つの router に載せている。

| レイヤ         | 主な責務                                                                                           |
| -------------- | -------------------------------------------------------------------------------------------------- |
| Gin middleware | logging, recovery, docs auth, external CORS/auth, SCIM auth, M2M auth                              |
| Huma           | request/response schema、OpenAPI、operation registration、security schemes                         |
| Service        | session, OIDC login, identity linking, role/tenant authz, delegation, provisioning, machine client |
| Auth package   | Redis-backed stores、Cookie、OIDC/OAuth client、JWT verifier、refresh token encryption             |

OpenAPI security schemes は `cookieAuth`、`bearerAuth`、`m2mBearerAuth` の 3 種類で、現状は `openapi/openapi.yaml` 一つに browser / external / SCIM / M2M の surface が含まれている。古いレポートでは browser spec と external spec の分離を提案していたが、現 repository の規模では単一 spec のままでも十分扱える。外部 API の公開範囲が大きくなった時点で分割を検討すればよい。

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
| M2M                  | `GET /api/m2m/v1/self`                                                                                                                                                                                                                           |

現時点で TODO や Customer Signals / Product Decisions のような業務 CRUD は存在しない。HaoHao はまず認証・認可・連携の土台を厚く作っている状態である。

## Database と sqlc

migration は `0001` から `0007` まで進んでいる。

| migration                | 内容                                                                                      |
| ------------------------ | ----------------------------------------------------------------------------------------- |
| `0001_init`              | `pgcrypto`、`users`。`public_id UUID DEFAULT uuidv7()`、local password 用 `password_hash` |
| `0002_user_identities`   | 外部 IdP identity を `user_identities` へ分離。`password_hash` nullable 化                |
| `0003_roles`             | `roles`, `user_roles`。初期 role は `docs_reader`, `external_api_user`, `todo_user`       |
| `0004_downstream_grants` | delegated auth 用 `oauth_user_grants`                                                     |
| `0005_provisioning`      | `deactivated_at`、SCIM/provisioning 用 identity columns、`provisioning_sync_state`        |
| `0006_org_tenants`       | `tenants`, `tenant_memberships`, `tenant_role_overrides`、default tenant、grant tenant 化 |
| `0007_machine_clients`   | `machine_clients` と `machine_client_admin` role                                          |

sqlc は `backend/sqlc.yaml` で、`db/schema.sql` と `db/queries/` を入力にして `backend/internal/db/` を生成する。UUID は `github.com/google/uuid.UUID` に寄せられている。

現在の DB は認証・認可・provisioning・tenant・machine client が中心で、業務ドメイン table はまだない。次に業務 CRUD を追加する場合は、既存の tenant / role / session モデルに最初から接続するのがよい。

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
- Home 画面は current session 表示、session refresh、logout、docs link を持つ。
- Integrations 画面は delegated auth の list/connect/verify/revoke を呼べる。
- fetch は `credentials: 'include'` で Cookie を送る。
- mutation 前に `XSRF-TOKEN` Cookie を読み、`X-CSRF-Token` header を付ける。
- `XSRF-TOKEN` が無い場合は `GET /api/v1/csrf` を先に呼ぶ。
- Vite dev server は `/api`, `/openapi`, `/docs` を backend `127.0.0.1:8080` に proxy する。
- production build output は `backend/web/dist` へ出力され、Go binary に embed される。

未実装:

- tenant selector UI。
- machine client admin UI。
- SCIM / provisioning status の管理 UI。
- 業務ドメイン CRUD UI。

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

- `/`, `/login`, `/integrations` は SPA fallback として `index.html` を返す。
- `/assets/*`, `/favicon.svg`, `/icons.svg` は frontend build artifact を返す。
- `/api/*`, `/docs`, `/schemas/*`, `/openapi.yaml`, `/openapi.json`, `/openapi-3.0.yaml`, `/openapi-3.0.json` は SPA fallback しない。
- 存在しない `/assets/*` や拡張子付き path は `index.html` ではなく `404` を返す。
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

- local `darwin/arm64` binary: `15,035,506 bytes`
- 変更前の debug 情報付き binary: 約 `21M`
- Docker image: `docker image ls` では `20MB`
- `docker history`: `/haohao` layer は `14.6MB`、CA bundle は `242kB`

## Docker と release

`docker/Dockerfile` は multi-stage build で、frontend build と backend embedded binary build を container 内で完結させる。

| stage         | 内容                                                               |
| ------------- | ------------------------------------------------------------------ |
| `node:24`     | `npm --prefix frontend run build`                                  |
| `golang:1.26` | `embed_frontend nomsgpack` build tag 付きで `/tmp/haohao` を build |
| `scratch`     | CA bundle と `/haohao` だけを copy                                 |

runtime image には shell も package manager もない。調査が必要な場合は production image に入るのではなく、debug image か builder stage を使う。

release workflow は tag push 時に frontend build、embedded Linux amd64 binary build、tarball 化、GitHub Release upload を行う。現状の release asset は `linux/amd64` が中心で、複数 OS / architecture が必要になった時点で matrix 化すればよい。

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
go test ./backend/...
go test ./backend/internal/config
go test -tags embed_frontend ./backend/internal/config
npm --prefix frontend run build
CGO_ENABLED=0 go build -buildvcs=false -trimpath -tags "embed_frontend nomsgpack" -ldflags "-s -w -buildid=" -o ./bin/haohao ./backend/cmd/main
make binary
docker build -t haohao:dev -f docker/Dockerfile .
go run ./backend/cmd/openapi > /tmp/haohao-openapi.yaml
cd backend && sqlc generate
cd frontend && npm run openapi-ts
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
- binary smoke test では `/`, `/login`, `/integrations` が HTML、`/api/v1/session` が `401 application/problem+json`、`/openapi.yaml` が OpenAPI YAML、`/assets/missing.js` が `404`。
- `AUTH_MODE=local` / `ENABLE_LOCAL_PASSWORD_LOGIN=true` の local password login smoke test は `200 OK` と `Set-Cookie` を返した。
- Docker image smoke test では `/`, `/login`, `/api/v1/session`, `/openapi.yaml`, `/openapi-3.0.yaml`, `/assets/missing.js` を確認済み。
- `bin/haohao` と同じ directory に `.env` を置き、`cd <dir> && ./haohao` で `DATABASE_URL` などが読み込まれることを確認済み。
- dev 用 `FRONTEND_BASE_URL=http://127.0.0.1:5173` が残った `.env` でも、embedded binary の `/api/v1/auth/callback?error=forced` は `APP_BASE_URL` 側の `/login?error=oidc_callback_failed` へ redirect することを確認済み。

## 残リスク

| リスク                                                             | 影響                                                                   | 推奨対応                                                                                |
| ------------------------------------------------------------------ | ---------------------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| `ProvisioningReconcileJob` が runtime scheduler に接続されていない | SCIM / provider state の drift を自動修復できない                      | 起動時 wiring、interval 設定、lock、metrics、dry-run log を追加                         |
| tenant selector UI がない                                          | tenant-aware backend があっても利用者が active tenant を切り替えにくい | Home または dedicated settings に selector を追加                                       |
| machine client admin UI がない                                     | backend CRUD はあるが browser から管理できない                         | `machine_client_admin` role 前提の一覧・作成・更新・無効化画面を追加                    |
| cutover / rollback runbook がない                                  | release 後の復旧判断が属人化する                                       | migration 前後、binary 差し替え、Docker rollback、Zitadel redirect URI 更新手順を文書化 |
| observability が最低限                                             | 失敗時に原因追跡しづらい                                               | request id、structured logs、health/readiness、metrics、trace hooks を追加              |
| release asset が Linux amd64 中心                                  | 配布対象が増えると手作業になる                                         | 必要になった時点で GOOS/GOARCH matrix と checksum を追加                                |
| 業務ドメインが未実装                                               | 認証基盤としては厚いが、ユーザー価値の画面が少ない                     | TODO または Customer Signals など、1つの縦切り機能を追加                                |

## 次の実装計画

### P0: 運用可能性を閉じる

最初にやるべきことは、新しい機能追加よりも、既に作った backend surface を運用できる形に閉じること。

| 作業                   | 完了条件                                                                                                                   |
| ---------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| Provisioning scheduler | interval env、single-flight/lock、失敗 log、unit test がある                                                               |
| health / readiness     | DB/Redis/Zitadel discovery の最低限確認ができる                                                                            |
| structured logging     | request id、method、path、status、latency、principal/tenant の安全な情報が出る                                             |
| runbook                | migration、binary deploy、Docker deploy、rollback、Zitadel redirect URI 更新が文書化される                                 |
| smoke script           | binary / Docker image に対する `/`, `/api/v1/session`, `/openapi.yaml`, login callback error redirect の確認が固定化される |

### P1: 管理 UI を補完する

backend は tenant と machine client の面が進んでいるため、次は frontend で操作可能にするのが費用対効果が高い。

| 作業                 | 完了条件                                                                                     |
| -------------------- | -------------------------------------------------------------------------------------------- |
| Tenant selector      | `GET /api/v1/tenants` と `POST /api/v1/session/tenant` を使い active tenant を切り替えられる |
| Machine client admin | list/create/detail/update/delete ができ、role 不足時の UI も扱う                             |
| Integrations UX      | connect / verify / revoke の成功・失敗・期限切れ表示を整理                                   |
| Docs link hardening  | docs auth required のときの遷移と 403 表示が自然                                             |

### P2: 業務ドメインの縦切りを追加する

HaoHao の土台を活かすには、認証済み user と tenant に紐づく小さな業務 CRUD を 1 つ追加するのがよい。候補は次のどちらか。

| 候補                                 | 向いている理由                                                                            |
| ------------------------------------ | ----------------------------------------------------------------------------------------- |
| TODO                                 | 既存 tutorial の文脈と合いやすく、tenant / role / CSRF / generated SDK の縦切り検証に向く |
| Customer Signals / Product Decisions | 意思決定ログとして実用寄りだが、最初の縦切りとしては範囲が広い                            |

最初は TODO の方がよい。理由は、今の課題はドメイン設計ではなく、既存の認証・tenant・generated client・SPA 配信が業務 CRUD でも破綻しないことを確認する段階だからである。

### P3: 配布を強化する

単一バイナリと Docker image は既にできている。次に必要になりやすいのは配布物の検証と追跡性である。

| 作業                      | 完了条件                                        |
| ------------------------- | ----------------------------------------------- |
| checksum                  | release asset に SHA256 を添付                  |
| multi-arch                | 必要な GOOS/GOARCH だけ matrix 化               |
| image tag strategy        | `:dev`, `:sha-...`, semver tag のルールを文書化 |
| SBOM / vulnerability scan | production image の確認方法を CI に追加         |

## 判断メモ

### Spec 分割は今すぐ不要

古いレポートでは browser API と external API の OpenAPI spec 分割を推奨していた。しかし現状の external API は小さく、SCIM / M2M も同一 Huma app に載っている。今は単一 `openapi/openapi.yaml` を正本にし、operation tag と security scheme で整理する方が保守しやすい。

### `.env` loader は direct binary 実行に必要

`./haohao` は `make backend-dev` と違い `.env` を自動で source しない。実行ファイル横の `.env` を読む実装は、single binary 配布の実用性に直結している。既存環境変数を優先する挙動も、本番運用と相性がよい。

### `scratch` runtime は妥当

現在の runtime image は shell を持たないため調査性は落ちるが、配布サイズと attack surface の点では妥当である。HTTPS outbound access が必要な OIDC / OAuth 連携のため、CA bundle だけは残す判断も正しい。

### UPX は現時点では不要

binary size は `darwin/arm64` で約 15 MB、Docker image は約 20 MB まで下がっている。UPX を使えばさらに小さくできる可能性はあるが、起動時間、debuggability、脆弱性 scanner、環境差の tradeoff がある。現時点では Go 標準 toolchain の size flags と `scratch` runtime で十分である。

## 結論

HaoHao は、認証・認可・provisioning・tenant・M2M・単一バイナリ配信まで到達しており、基盤としてはかなり実装が進んでいる。今から大きな新設計に戻る必要はない。

次の最短経路は、`ProvisioningReconcileJob` の scheduler 接続、tenant selector UI、machine client admin UI、runbook、observability を入れて、現在の backend surface を実運用できる形に閉じること。その後に TODO など小さな業務 CRUD を 1 つ追加すると、認証基盤が実アプリとして機能するかを最小コストで検証できる。
