# `AUTH_MODE=zitadel` から `CONCEPT.md` の最終形まで進める standalone チュートリアル

## この文書の目的

この文書は、**いまの HaoHao リポジトリを起点にして、Zitadel の browser redirect / callback login から `CONCEPT.md` が想定する認証まわりの最終形まで進めるための続きの手順書**です。

この文書は単独で読めるように書いています。`TUTORIAL.md` を見返さなくても、次に何を作るべきか、どのファイルを触るべきか、なぜその順番なのかが分かる構成にしています。

## Phase と主題の対応表

この文書では、最初に宣言する 6 段階をそのまま `Phase 1-6` として実体化します。

| Phase | 主題 |
| --- | --- |
| Phase 1 | browser redirect / callback login を実装する |
| Phase 2 | browser 向けの Cookie session, logout, CSRF 再発行まで仕上げる |
| Phase 3 | external client 向け API を browser 向けとは別 surface の bearer token 方式で追加する |
| Phase 4 | downstream delegated auth と refresh token 管理を server side に閉じる |
| Phase 5 | identity lifecycle / provisioning と org / tenant-aware auth context を入れる |
| Phase 6 | machine-to-machine, docs / OpenAPI 公開, CI, release asset, cutover, 運用までそろえる |

## この文書が前提にしている現在地

この repo は、少なくとも次の状態にある前提で進めます。

- PostgreSQL と Redis を `compose.yaml` で起動できる
- `users` テーブルがある
- `GET /api/v1/session`, `POST /api/v1/login`, `POST /api/v1/logout` がある
- `GET /api/v1/auth/settings` がある
- `AUTH_MODE=zitadel` にすると frontend は password form を隠せる
- local password login の foundation は動いている

この文書の Phase 0-2 は、**TODO 機能を前提にしません**。もし repo に TODO API が無い場合でも、browser login, cookie session, logout, CSRF bootstrap の実装だけで十分に読み進められます。

## Phase 0. Zitadel 導入と初期設定

`Phase 0` は、本文の `Phase 1-6` に入る前の前提準備です。ここでは **Zitadel を未導入の状態から、Phase 1 を始められる状態まで持っていく**ことだけを扱います。

### 目的

HaoHao 本体の実装に入る前に、self-hosted な dev 用 Zitadel 環境を起動し、browser login 用 application を作り、`.env` に必要な値をそろえます。

### この Phase の前提

- Docker と Docker Compose が使える
- HaoHao 側の callback / redirect URL は `127.0.0.1` で統一する
- production-grade の運用設計ではなく、dev / 検証用 quickstart を対象にする

#### ホスト側で先に入れておくもの

この文書のコマンドは次が入っている前提です。

- Docker Engine
- `docker compose` plugin または `docker-compose` binary
- Go
- Node.js と npm
- `migrate` CLI
- `curl`

最低限の確認は次で足ります。

```bash
docker --version
docker compose version || docker-compose version
go version
npm --version
migrate -version
curl --version
```

### この Phase の完了条件

- self-hosted な dev 用 Zitadel が起動している
- Console に admin で入れる
- HaoHao 用 project と browser login 用 application が作成済みである
- `.env` に `ZITADEL_ISSUER`, `ZITADEL_CLIENT_ID`, `ZITADEL_CLIENT_SECRET` を転記できている
- `/.well-known/openid-configuration` が見える

### Step 0.1. self-hosted dev 環境として Zitadel を別起動する

このチュートリアルでは、**HaoHao 本体の `compose.yaml` に Zitadel を追加しません**。Zitadel は別の self-hosted dev 環境として起動します。

理由は 2 つです。

- HaoHao 本体の compose と認証基盤の compose を分けたほうが、認証基盤の差し替えと障害切り分けがしやすい
- production では Zitadel をアプリと別系統で運用する前提なので、dev でも役割を分けておくほうが自然

### Step 0.2. Git 管理された Docker Compose で Zitadel を起動する

self-hosted の最短導入は、Zitadel の公式 Docker Compose quickstart を使うのが安全です。この repo では quickstart を元にした dev 用 compose を `dev/zitadel` に置き、起動は `Makefile` から行います。

#### 公式参照

- Self-hosted deploy overview  
  https://zitadel.com/docs/self-hosting/deploy/overview  
  用途: self-hosted 全体像と、Docker Compose が quickstart の選択肢にあることを確認する

- Set up ZITADEL with Docker Compose  
  https://zitadel.com/docs/self-hosting/deploy/compose  
  用途: dev 用 quickstart の compose, `.env`, 初回 login 手順を確認する

#### ここで読者が得るべきもの

この Step のゴールは、次の 3 つです。

- issuer URL
- admin login できる Console
- 利用可能な OIDC endpoints

#### この repo で使う起動コマンド

`make zitadel-up` は `dev/zitadel/.env` が無ければ `dev/zitadel/.env.example` から作成し、その後に Zitadel stack を起動します。実 `.env` は Git 管理しません。

#### HaoHao の `:8080` と衝突しないようにする

公式 quickstart は Zitadel を `http://localhost:8080` に公開します。しかし、この repo の HaoHao backend も `http://127.0.0.1:8080` を使います。`dev/zitadel/.env.example` は、最初から Zitadel 側を `8081` にずらしています。

必要に応じて `dev/zitadel/.env` を編集しますが、通常は既定値のままで構いません。

```dotenv
PROXY_HTTP_PUBLISHED_PORT=8081
ZITADEL_EXTERNALPORT=8081
```

起動します。

```bash
make zitadel-up
```

起動後の最初の確認はこれで足ります。

```bash
curl -fsS http://localhost:8081/.well-known/openid-configuration
```

この文書では以後、local self-hosted quickstart の Zitadel は次で扱います。

```text
issuer:  http://localhost:8081
console: http://localhost:8081/ui/console
```

停止と再起動は次です。

```bash
make zitadel-down
make zitadel-up
make zitadel-logs
```

#### 起動後に確認すること

- browser で Console に入れる
- `/.well-known/openid-configuration` が見える
- issuer URL を控えられる

dev quickstart では Zitadel 自体が `localhost` で立ち上がる例が出ます。**`ZITADEL_ISSUER` は quickstart が expose した issuer をそのまま使い、HaoHao 側の callback / redirect 登録値だけを `127.0.0.1` で統一**してください。

quickstart 直後の初回 admin login は、公式 guide の既定値どおり次で入れます。

```text
login URL: http://localhost:8081/ui/console?login_hint=zitadel-admin@zitadel.localhost
password:  Password1!
```

この quickstart では bootstrap 用に `login-client.pat` も作られますが、**これだけでは HaoHao 用 project や browser application の作成権限は足りません**。実際の bootstrap は Console 上で admin として行う前提にしてください。

### Step 0.3. 初回 bootstrap を行う

admin で Console に入ったら、HaoHao 用の project と roles を先に作ります。

local self-hosted quickstart には、最初から `ZITADEL` organization と `ZITADEL` project があります。**local dev では既定 org の下に `haohao` project を追加するだけでも構いません**。この文書では分かりやすさのために `HaoHao` organization / `haohao` project を推奨していますが、最低限必要なのは「HaoHao 用の独立した project と roles」です。

#### 推奨の最小構成

- Organization: `HaoHao`
- Project: `haohao`

Project の role は先に次を作ってください。

- `docs_reader`
- `external_api_user`
- `machine_client_admin`
- `todo_user`

この 4 つは本文の Phase 3 以降で local role の写像先として使います。`machine_client_admin` は Phase 6 の machine client CRUD 管理用です。ここで先に名前を固定しておくと、後で role 名がぶれません。

### Step 0.4. browser login 用 application を作る

browser login 用 application は **Web application** に固定します。HaoHao は backend で token exchange を行い client secret を保持するため、SPA 単体向けの `User Agent` ではなく `Web` として扱うほうが設計に合います。

#### `Select Framework` で迷ったら

Zitadel Console に `Select Framework` が出る場合、**`Go` があれば `Go` を選んでください**。この repo は frontend に Vue を使っていますが、OIDC callback と token exchange を持つのは browser 上の SPA ではなく **Go backend** です。

- 推奨: `Go`
- `Go` が無ければ `Other` / `Custom`
- 選ばない: `React` / `Vue` / `SPA` 前提の framework

ここで重要なのは framework 名そのものではなく、**最終的な application type が `Web` であること**です。framework selection は Console の初期値を補助するためのもので、HaoHao 側のアーキテクチャを変えるものではありません。

#### 公式参照

- Applications overview  
  https://zitadel.com/docs/guides/manage/console/applications-overview  
  用途: application type と token type の違いを確認する

#### ここで固定する設定

- Application type: `Web`
- Change OIDC Settings では **`CODE` を選ぶ**
- `Authentication Method` は `Basic`
- `client secret` を発行させる
- redirect URI:

```text
http://127.0.0.1:8080/api/v1/auth/callback
```

- post logout redirect URI:

```text
http://127.0.0.1:5173/login
```

browser login の初期 scope は本文どおり次で固定します。

```text
openid profile email
```

Console では次の順で辿ると迷いにくいです。

1. `Project`
2. `Applications`
3. `New`
4. `Web`
5. `Change OIDC Settings`
6. `CODE`
7. redirect URI / post logout redirect URI を保存

#### `PKCE` カードを選ばない理由

現在の Zitadel Console では、`Change OIDC Settings` の `PKCE` カードは **public client** 向けの設定です。`Authentication Method: None` になり、画面にも出るとおり **client secret は発行されません**。

HaoHao は browser ではなく **Go backend が authorization code を exchange する confidential web app** なので、この tutorial では `PKCE` カードではなく **`CODE` カード** を選びます。

- `PKCE` カード: `Authentication Method = None`、client secret なし
- `CODE` カード: `Authentication Method = Basic`、client secret あり

ここで `CODE` を選んでも、backend 実装側では引き続き **PKCE challenge / verifier を送って構いません**。この文書で言う「PKCE を使う」は protocol 上の追加防御であり、Console 上の card 名 `PKCE` を選ぶこととは同義ではありません。

#### 取得して `.env` に入れる値

この Step で Console から取得して控える値は次です。

- issuer URL
- browser app client ID
- browser app client secret

`client ID` は application の詳細画面にあります。`client secret` は表示済みのものを控えるか、見失った場合は `Regenerate Client Secret` で再発行してください。

すでに `PKCE` カードで作ってしまっていた場合は、**application を作り直す必要はありません**。`Change OIDC Settings` で `CODE` に変更し、その後 `Regenerate Client Secret` を実行して `client secret` を発行すれば十分です。

### Step 0.5. `.env` と Console 設定の対応をそろえる

Console 上の値と `.env` の対応は次です。

| Console 上の値 | `.env` のキー |
| --- | --- |
| issuer URL | `ZITADEL_ISSUER` |
| browser app client ID | `ZITADEL_CLIENT_ID` |
| browser app client secret | `ZITADEL_CLIENT_SECRET` |
| browser redirect URI | `ZITADEL_REDIRECT_URI` |
| post logout redirect URI | `ZITADEL_POST_LOGOUT_REDIRECT_URI` |

browser app の `client ID` と `client secret` を入れ終わったら、HaoHao 側の `.env` では **`AUTH_MODE=zitadel` に切り替えてから backend を再起動**してください。`AUTH_MODE=local` のままだと browser redirect login は有効になりません。

現在の backend は、`AUTH_MODE=zitadel` なのに `ZITADEL_ISSUER`, `ZITADEL_CLIENT_ID`, `ZITADEL_CLIENT_SECRET` のいずれかが欠けている場合は **起動時に fail fast** します。mode だけ先に切り替えて値を後回しにしないでください。

この Phase で最低限そろえる `.env` は次です。

```dotenv
APP_BASE_URL=http://127.0.0.1:8080
FRONTEND_BASE_URL=http://127.0.0.1:5173

AUTH_MODE=zitadel
ZITADEL_ISSUER=http://localhost:8081
ZITADEL_CLIENT_ID=<browser-app-client-id>
ZITADEL_CLIENT_SECRET=<browser-app-client-secret>
ZITADEL_REDIRECT_URI=http://127.0.0.1:8080/api/v1/auth/callback
ZITADEL_POST_LOGOUT_REDIRECT_URI=http://127.0.0.1:5173/login
ZITADEL_SCOPES="openid profile email"

LOGIN_STATE_TTL=10m
```

`ZITADEL_CLIENT_SECRET` は repo に commit しないでください。local 開発では `.env` にだけ置きます。

この時点では browser login 用 application だけを作れば十分です。**external user bearer app, SCIM client, M2M credential はまだ作らなくて構いません**。それらは `Phase 3`, `Phase 5`, `Phase 6` で追加します。

### Step 0.6. Phase 1 に進む前の確認

Phase 1 に進む前に、最低限次を確認してください。

1. `/.well-known/openid-configuration` が見える
2. Console で browser app が作成済みである
3. `.env` に issuer / client id / secret を転記済みである
4. redirect URI と post logout redirect URI が `127.0.0.1` で登録されている

## 最初に固定する設計

実装に入る前に、後でぶれやすい点をここで固定します。

### 1. ローカル URL は `127.0.0.1` に統一する

OAuth/OIDC の redirect URI mismatch は `localhost` と `127.0.0.1` の混在で起きやすいです。この文書では、**backend, frontend, Zitadel application の登録値をすべて `127.0.0.1` に統一**します。

- backend: `http://127.0.0.1:8080`
- frontend: `http://127.0.0.1:5173`
- browser callback: `http://127.0.0.1:8080/api/v1/auth/callback`
- post logout redirect: `http://127.0.0.1:5173/login`

### 2. Zitadel application の役割を最初に分ける

この文書では Zitadel 側の application を少なくとも 3 系統に分けます。

- browser login 用 application: `Authorization Code + PKCE`
- external user bearer API 用 application: **JWT access token 必須**
- machine-to-machine 用 application: client credentials + **JWT access token 必須**

Zitadel の access token は既定では opaque token です。**Phase 3 と Phase 6 の local JWT 検証を成立させるため、external API 用と M2M 用 application は JWT access token を前提に固定**します。

### 3. browser login では `ID token` と `userinfo` を役割分担する

この文書では browser login 後の claim 取得元を次のように固定します。

- `ID token`: `sub`, `iss`, `aud`, `nonce` の検証用
- `userinfo`: `email`, `email_verified`, `name` など profile 情報の取得用

つまり、**browser login は「ID token を検証したうえで userinfo を取りに行く」構成**にします。`email` や `name` が必ず ID token に入る前提では書きません。

### 4. provider user の識別子は `email` ではなく `(provider, sub)` を使う

OIDC の stable identifier は `sub` です。`email` だけで既存 user と結びつけるのは危険です。

そのため DB では次を保持します。

- provider 名
- provider 側 subject
- email
- email_verified

### 5. browser から見える認証状態は local Cookie session に統一する

callback 後に provider token を frontend へ渡す構成にはしません。backend が token exchange, ID token 検証, userinfo 取得を済ませたら、その場で今まで通り `SESSION_ID` と `XSRF-TOKEN` を発行します。

### 6. 一時的な login state は Redis に置く

この repo にはすでに Redis があります。そこで OIDC の `state`, `nonce`, `code_verifier` を短命データとして Redis に保存します。

### 7. canonical logout は `POST /api/v1/logout` のままにする

現在の repo は `POST /api/v1/logout` を Cookie + CSRF で保護しています。ここは崩しません。

- local session の state change は引き続き `POST /api/v1/logout`
- provider logout が必要な場合だけ、POST 成功後に browser を provider の logout URL へ top-level navigation させる
- `GET` で state change する logout route は作らない

### 8. bearer token 検証は汎用 verifier を先に作って使い回す

Phase 3 で **generic JWT bearer verifier** を作り、それを次で再利用します。

- external user bearer API
- SCIM / provisioning bearer
- Phase 6 の M2M verifier

SCIM は Phase 6 の M2M verifier を待たず、Phase 3 の generic verifier を `audience / scope` 違いで再利用する構成にします。

### 9. tenant sync 用 provider claim は `groups` に固定する

tenant-aware auth context の claim contract は曖昧にしません。この文書では、**provider からアプリに見える tenant membership claim 名を top-level の `groups` に固定**します。

- application code は `organization_id`, `organization_ids`, `groups` のような複数候補を見ない
- Zitadel Action など provider 側の拡張で、必要な情報を **`groups` claim に寄せてから** backend へ渡す

### 10. 最終的な認可判断は operation ではなく service に置く

browser session でも bearer token でも、operation は「認証済みか」を見るだけに寄せます。最終的な role / tenant / scope の判定は `AuthContext` を受け取る service に集約します。

## この文書で作る最終形

最終的な認証フローは次の 4 系統にします。

### browser 向け

```text
1. user opens /login
2. frontend sees AUTH_MODE=zitadel
3. frontend sends browser to GET /api/v1/auth/login
4. backend creates state / nonce / PKCE verifier and stores them in Redis
5. backend redirects browser to Zitadel authorize endpoint
6. user signs in on Zitadel
7. Zitadel redirects browser to GET /api/v1/auth/callback?code=...&state=...
8. backend consumes state, exchanges code, verifies ID token
9. backend calls userinfo with the returned access token
10. backend finds or creates a local user
11. backend issues SESSION_ID and XSRF-TOKEN cookies
12. backend redirects browser back to frontend
13. frontend bootstrap calls GET /api/v1/session and becomes authenticated
```

### external user client 向け

```text
1. external client gets a JWT access token from Zitadel
2. client calls /api/external/v1/*
3. backend verifies the JWT locally with Zitadel JWKS
4. backend builds auth context from token claims and local policy
5. service makes the final authorization decision
```

### downstream delegated access

```text
1. browser user consents to a downstream resource server
2. backend stores the returned refresh token in encrypted server-side storage
3. service requests an access token only when a downstream call is needed
4. backend rotates refresh tokens and handles revocation / invalid_grant
5. browser never receives the downstream refresh token
```

### machine-to-machine 向け

```text
1. machine client gets a JWT access token from Zitadel via client credentials
2. client calls /api/m2m/v1/*
3. backend verifies the token as M2M-only and maps the provider client to a local machine identity
4. service applies tenant-aware policy and scope-based authorization
5. browser-facing API and human-user bearer API stay isolated from M2M traffic
```

## 主な編集対象

このチュートリアルで主に触るファイルは次です。

### config / docs

- `.env.example`
- `backend/internal/config/config.go`
- `openapi/openapi.yaml`
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`

### database

- `db/migrations/0002_user_identities.up.sql`
- `db/migrations/0002_user_identities.down.sql`
- `db/migrations/0003_roles.up.sql`
- `db/migrations/0003_roles.down.sql`
- `db/migrations/0004_downstream_grants.up.sql`
- `db/migrations/0004_downstream_grants.down.sql`
- `db/migrations/0005_scim_provisioning.up.sql`
- `db/migrations/0005_scim_provisioning.down.sql`
- `db/migrations/0006_org_tenants.up.sql`
- `db/migrations/0006_org_tenants.down.sql`
- `db/migrations/0007_machine_clients.up.sql`
- `db/migrations/0007_machine_clients.down.sql`
- `db/queries/users.sql`
- `db/queries/identities.sql`
- `db/queries/roles.sql`
- `db/queries/downstream_grants.sql`
- `db/queries/provisioning.sql`
- `db/queries/org_tenants.sql`
- `db/queries/machine_clients.sql`
- `db/schema.sql`
- `backend/sqlc.yaml`

### backend

- `backend/internal/auth/oidc_client.go`
- `backend/internal/auth/login_state_store.go`
- `backend/internal/auth/bearer_verifier.go`
- `backend/internal/auth/refresh_token_store.go`
- `backend/internal/auth/m2m_verifier.go`
- `backend/internal/service/identity_service.go`
- `backend/internal/service/oidc_login_service.go`
- `backend/internal/service/session_service.go`
- `backend/internal/service/authz_service.go`
- `backend/internal/service/delegation_service.go`
- `backend/internal/service/provisioning_service.go`
- `backend/internal/service/tenant_sync_service.go`
- `backend/internal/service/machine_client_service.go`
- `backend/internal/api/oidc.go`
- `backend/internal/api/docs.go`
- `backend/internal/api/scim.go`
- `backend/internal/api/external_*.go`
- `backend/internal/api/m2m_*.go`
- `backend/internal/api/register.go`
- `backend/internal/app/app.go`
- `backend/internal/middleware/*.go`
- `backend/internal/jobs/provisioning_reconcile.go`
- `backend/cmd/main/main.go`
- `backend/cmd/openapi/main.go`

### frontend

- `frontend/src/views/LoginView.vue`
- `frontend/src/views/HomeView.vue`
- `frontend/src/api/client.ts`
- `frontend/src/api/auth.ts`
- `frontend/src/stores/session.ts`

`frontend/src/stores/session.ts` と `frontend/src/api/session.ts` は土台として使えますが、この文書の後半では logout 導線と CSRF bootstrap に変更が入ります。

---

## Phase 1. Browser Redirect / Callback Login

### 目的

`AUTH_MODE=zitadel` で本物の browser login を通し、callback 後に local Cookie session を払い出せる状態まで持っていきます。

### この Phase の前提

- PostgreSQL と Redis が起動している
- 既存の local session login は動いている
- `AUTH_MODE=zitadel` で frontend が login form を隠せる

### この Phase の完了条件

- `/api/v1/auth/login` が authorize redirect を返す
- `/api/v1/auth/callback` が ID token 検証 + userinfo 取得を行える
- callback 後に `SESSION_ID` と `XSRF-TOKEN` を発行できる
- frontend から browser redirect login が通る

### Step 1.1. Zitadel 側の application と claim contract を先に固定する

backend を書く前に、あとで効いてくる provider 側の前提をそろえます。

#### browser login 用 application

- flow は `Authorization Code`
- Console の `Change OIDC Settings` では `CODE` を選ぶ
- backend 実装では PKCE challenge / verifier も送る
- `Select Framework` が出るなら `Go` を選ぶ
- application type は必ず `Web` に寄せる
- redirect URI は次で固定する

```text
http://127.0.0.1:8080/api/v1/auth/callback
```

- post logout redirect URI は次で固定する

```text
http://127.0.0.1:5173/login
```

- browser login の初期 scope は次で始める

```text
openid profile email
```

ここでは **`offline_access` を browser login に含めません**。browser login の目的は local session を作ることだけで、downstream delegated consent は Phase 4 で別導線にします。

#### external user bearer API 用 application

- user が bearer token を取得するための application を別に用意する
- **JWT access token を有効にする**
- expected audience はこの文書では `haohao-external` に固定する

#### machine-to-machine 用 application

- client credentials を使える application を別に用意する
- **JWT access token を有効にする**
- expected audience はこの文書では `haohao-m2m` に固定する

#### tenant sync 用 claim contract

Phase 5 では provider claim から tenant membership を同期します。ここは最初に固定しておきます。

- backend が読む provider claim 名は **`groups`** だけにする
- `organization_id` や `organization_ids` は application code で吸収しない
- Zitadel Action など provider 側の拡張で、必要な membership 情報を top-level の `groups` claim に寄せる

### Step 1.2. 設定を増やす

#### `.env`

ここは **tracked の `.env.example` ではなく、実際に Zitadel login を試すときの `.env`** に入れる値です。repo に置く `.env.example` の正本は後半の `Phase 0-2 Exact Snapshot` を優先してください。

```dotenv
APP_BASE_URL=http://127.0.0.1:8080
FRONTEND_BASE_URL=http://127.0.0.1:5173

AUTH_MODE=zitadel
ZITADEL_ISSUER=
ZITADEL_CLIENT_ID=
ZITADEL_CLIENT_SECRET=
ZITADEL_REDIRECT_URI=http://127.0.0.1:8080/api/v1/auth/callback
ZITADEL_POST_LOGOUT_REDIRECT_URI=http://127.0.0.1:5173/login
ZITADEL_SCOPES="openid profile email"

LOGIN_STATE_TTL=10m
```

`COOKIE_SECURE=false` はローカルではそのままで構いません。HTTPS 配下へ出すときだけ `true` にします。`ZITADEL_SCOPES` は `Makefile` が `.env` を `source` するので、**空白区切りの scope は必ず quotes で囲ってください**。

#### `backend/internal/config/config.go`

`Config` に次の項目を足します。

- `AppBaseURL`
- `FrontendBaseURL`
- `ZitadelClientSecret`
- `ZitadelRedirectURI`
- `ZitadelPostLogoutRedirectURI`
- `ZitadelScopes`
- `LoginStateTTL`

`ZitadelScopes` は **string のまま受け、OIDC client 側で `strings.Fields` する**形に固定してください。いまの `config.go` の流儀とも合います。

最小実装は次の形です。

```go
type Config struct {
	AppBaseURL                   string
	FrontendBaseURL              string
	AuthMode                     string
	ZitadelIssuer                string
	ZitadelClientID              string
	ZitadelClientSecret          string
	ZitadelRedirectURI           string
	ZitadelPostLogoutRedirectURI string
	ZitadelScopes                string
	LoginStateTTL                time.Duration
	SessionTTL                   time.Duration
	CookieSecure                 bool
}

func Load() (Config, error) {
	sessionTTL, err := time.ParseDuration(getEnv("SESSION_TTL", "24h"))
	if err != nil {
		return Config{}, err
	}
	loginStateTTL, err := time.ParseDuration(getEnv("LOGIN_STATE_TTL", "10m"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppBaseURL:                   getEnv("APP_BASE_URL", "http://127.0.0.1:8080"),
		FrontendBaseURL:              strings.TrimRight(getEnv("FRONTEND_BASE_URL", "http://127.0.0.1:5173"), "/"),
		AuthMode:                     getEnv("AUTH_MODE", "local"),
		ZitadelIssuer:                getEnv("ZITADEL_ISSUER", ""),
		ZitadelClientID:              getEnv("ZITADEL_CLIENT_ID", ""),
		ZitadelClientSecret:          getEnv("ZITADEL_CLIENT_SECRET", ""),
		ZitadelRedirectURI:           getEnv("ZITADEL_REDIRECT_URI", "http://127.0.0.1:8080/api/v1/auth/callback"),
		ZitadelPostLogoutRedirectURI: getEnv("ZITADEL_POST_LOGOUT_REDIRECT_URI", "http://127.0.0.1:5173/login"),
		ZitadelScopes:                getEnv("ZITADEL_SCOPES", "openid profile email"),
		LoginStateTTL:                loginStateTTL,
		SessionTTL:                   sessionTTL,
		CookieSecure:                 getEnvBool("COOKIE_SECURE", false),
	}, nil
}
```

#### `db/queries/users.sql`

```sql
-- name: AuthenticateUser :one
SELECT id
FROM users
WHERE email = @email
  AND password_hash IS NOT NULL
  AND password_hash = crypt(@password, password_hash)
LIMIT 1;

-- name: GetUserByEmail :one
SELECT
    id,
    public_id,
    email,
    display_name
FROM users
WHERE email = $1
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

-- name: CreateOIDCUser :one
INSERT INTO users (
    email,
    display_name,
    password_hash
) VALUES (
    $1,
    $2,
    NULL
)
RETURNING
    id,
    public_id,
    email,
    display_name;

-- name: UpdateUserProfile :one
UPDATE users
SET email = $2,
    display_name = $3,
    updated_at = now()
WHERE id = $1
RETURNING
    id,
    public_id,
    email,
    display_name;
```

### Step 1.3. DB に外部 identity を保存できるようにする

callback で user を作る前に、「何をキーに local user と結びつけるか」を DB に落とします。

今の `users.password_hash` は `NOT NULL` です。これだと Zitadel だけで作られた user を素直に保存できません。そこで次の 2 つを入れます。

1. `users.password_hash` を nullable にする
2. `user_identities` テーブルを追加する

この repo は `0001_init` までしか持っていない前提なので、外部 identity 用 migration は **`0002_*`** にしてください。`0003_*` は `0002_todos` のような先行 migration が既にある場合だけです。

#### `db/migrations/0002_user_identities.up.sql`

```sql
ALTER TABLE users
    ALTER COLUMN password_hash DROP NOT NULL;

CREATE TABLE user_identities (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    subject TEXT NOT NULL,
    email TEXT NOT NULL,
    email_verified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, subject)
);

CREATE INDEX user_identities_user_id_idx ON user_identities(user_id);
```

初期制約は **`UNIQUE (provider, subject)` のみ**にします。`UNIQUE (user_id, provider)` は入れません。もし「1 local user に同一 provider identity を何件まで link するか」を制御したいなら、schema ではなく service 層の link policy で扱ってください。

#### `db/migrations/0002_user_identities.down.sql`

down は開発用と割り切って、破壊的でもよいので確実に戻せる形にしてください。

```sql
DROP TABLE IF EXISTS user_identities;

DELETE FROM users
WHERE password_hash IS NULL;

ALTER TABLE users
    ALTER COLUMN password_hash SET NOT NULL;
```

#### `db/queries/users.sql`

local password login は nullable 化に合わせて絞り込みを追加してください。

```sql
-- name: AuthenticateUser :one
SELECT id
FROM users
WHERE email = @email
  AND password_hash IS NOT NULL
  AND password_hash = crypt(@password, password_hash)
LIMIT 1;
```

この query では、**`password_hash IS NULL` の user は password login 対象外**です。つまり「Zitadel で作った user は local password login では通さない」という振る舞いになります。

#### `db/queries/identities.sql`

最低限必要なのは次です。

```sql
-- name: GetUserByProviderSubject :one
SELECT
    u.id,
    u.public_id,
    u.email,
    u.display_name
FROM user_identities ui
JOIN users u ON u.id = ui.user_id
WHERE ui.provider = $1
  AND ui.subject = $2
LIMIT 1;

-- name: CreateUserIdentity :exec
INSERT INTO user_identities (
    user_id,
    provider,
    subject,
    email,
    email_verified
) VALUES ($1, $2, $3, $4, $5);

-- name: UpdateUserIdentityProfile :exec
UPDATE user_identities
SET email = $3,
    email_verified = $4,
    updated_at = now()
WHERE provider = $1
  AND subject = $2;
```

ここで大事なのは、**identity lookup は `(provider, subject)` で行う**ことです。

### Step 1.4. OIDC の一時 state を Redis に置く

authorize redirect を作るには `state`, `nonce`, `code_verifier` が必要です。これらは callback まで保持し、使い終わったら消す必要があります。session 本体とは寿命も用途も違うため、別ストアとして切り出します。

#### 追加するファイル

- `backend/internal/auth/login_state_store.go`

#### この store に持たせる情報

```go
type LoginStateRecord struct {
    CodeVerifier string `json:"codeVerifier"`
    Nonce        string `json:"nonce"`
    ReturnTo     string `json:"returnTo"`
}
```

`ReturnTo` は任意ですが、callback 後にどこへ戻すかを持たせたいなら入れておくと便利です。最初は固定で `/` に戻すだけでも構いません。

必要なメソッドは次で十分です。

- `Create(ctx, returnTo string) (state string, record LoginStateRecord, err error)`
- `Consume(ctx, state string) (LoginStateRecord, error)`

`Consume` は取得したあと削除してください。replay を避けるためです。

prefix は session と分けます。

```text
oidc-state:
```

TTL は `10m` くらいで十分です。

最小実装は次のままで動きます。

```go
type LoginStateRecord struct {
	CodeVerifier string `json:"codeVerifier"`
	Nonce        string `json:"nonce"`
	ReturnTo     string `json:"returnTo"`
}

type LoginStateStore struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

func NewLoginStateStore(client *redis.Client, ttl time.Duration) *LoginStateStore {
	return &LoginStateStore{
		client: client,
		prefix: "oidc-state:",
		ttl:    ttl,
	}
}

func (s *LoginStateStore) Create(ctx context.Context, returnTo string) (string, LoginStateRecord, error) {
	state, err := randomToken(32)
	if err != nil {
		return "", LoginStateRecord{}, err
	}
	codeVerifier, err := randomToken(32)
	if err != nil {
		return "", LoginStateRecord{}, err
	}
	nonce, err := randomToken(32)
	if err != nil {
		return "", LoginStateRecord{}, err
	}

	record := LoginStateRecord{
		CodeVerifier: codeVerifier,
		Nonce:        nonce,
		ReturnTo:     returnTo,
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return "", LoginStateRecord{}, err
	}
	if err := s.client.Set(ctx, s.prefix+state, payload, s.ttl).Err(); err != nil {
		return "", LoginStateRecord{}, fmt.Errorf("save login state: %w", err)
	}

	return state, record, nil
}

func (s *LoginStateStore) Consume(ctx context.Context, state string) (LoginStateRecord, error) {
	raw, err := s.client.GetDel(ctx, s.prefix+state).Bytes()
	if errors.Is(err, redis.Nil) {
		return LoginStateRecord{}, ErrLoginStateNotFound
	}
	if err != nil {
		return LoginStateRecord{}, fmt.Errorf("consume login state: %w", err)
	}

	var record LoginStateRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return LoginStateRecord{}, fmt.Errorf("decode login state: %w", err)
	}
	return record, nil
}
```

### Step 1.5. Zitadel と話す OIDC client を作る

state を保存できても、authorize URL 生成と token exchange がなければ login を閉じられません。ここで protocol 部分を 1 箇所へ寄せます。

#### 使う依存

Go 側では次が扱いやすいです。

```bash
go get github.com/coreos/go-oidc/v3/oidc@v3.18.0 golang.org/x/oauth2@v0.36.0
go mod tidy
```

#### 追加するファイル

- `backend/internal/auth/oidc_client.go`

#### この client に持たせる責務

1. issuer から discovery する
2. authorize URL を作る
3. authorization code を token endpoint へ exchange する
4. ID token を verify する
5. userinfo endpoint を呼ぶ
6. claims を必要最小限の Go struct に落とす

#### claims struct の例

```go
type IdentityClaims struct {
    Subject       string   `json:"sub"`
    Email         string   `json:"email"`
    EmailVerified bool     `json:"email_verified"`
    Name          string   `json:"name"`
    Groups        []string `json:"groups,omitempty"`
}
```

`Groups` は Phase 5 まで必須ではありませんが、claim contract は最初からここに寄せておくと後で楽です。

実際の返り値は、Phase 2 の logout で `id_token_hint` を使えるように raw ID token も持たせる形が扱いやすいです。

```go
type OIDCIdentity struct {
    Claims     IdentityClaims
    RawIDToken string
}
```

#### authorize URL に必ず入れるもの

- `response_type=code`
- `scope=openid profile email`
- `state`
- `nonce`
- `code_challenge`
- `code_challenge_method=S256`
- `redirect_uri`

#### callback で必ず検証するもの

- state が Redis に存在すること
- token exchange が成功すること
- ID token の signature と issuer と audience が正しいこと
- nonce が保存値と一致すること
- access token で userinfo を取得できること

#### 実装上の注意

- browser login の profile 取得元は **userinfo** に固定します
- `ID token` は `sub / iss / aud / nonce` 検証に使います
- `email` や `name` を ID token だけから取る前提では書きません

最小実装は次の形です。

```go
type IdentityClaims struct {
	Subject       string   `json:"sub"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	Name          string   `json:"name"`
	Groups        []string `json:"groups,omitempty"`
}

type OIDCIdentity struct {
	Claims     IdentityClaims
	RawIDToken string
}

func NewOIDCClient(ctx context.Context, issuer, clientID, clientSecret, redirectURI, scopes string) (*OIDCClient, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover oidc provider: %w", err)
	}

	oauthScopes := strings.Fields(scopes)
	if len(oauthScopes) == 0 {
		oauthScopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}

	return &OIDCClient{
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
		oauth2Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  redirectURI,
			Scopes:       oauthScopes,
		},
	}, nil
}

func (c *OIDCClient) AuthorizeURL(state, nonce, codeVerifier string) string {
	return c.oauth2Config.AuthCodeURL(
		state,
		oidc.Nonce(nonce),
		oauth2.SetAuthURLParam("code_challenge", pkceS256(codeVerifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

func (c *OIDCClient) ExchangeCode(ctx context.Context, code, codeVerifier, expectedNonce string) (OIDCIdentity, error) {
	token, err := c.oauth2Config.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return OIDCIdentity{}, fmt.Errorf("exchange authorization code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return OIDCIdentity{}, fmt.Errorf("id_token missing from token response")
	}

	idToken, err := c.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return OIDCIdentity{}, fmt.Errorf("verify id token: %w", err)
	}

	var verified struct {
		Subject string `json:"sub"`
		Nonce   string `json:"nonce"`
	}
	if err := idToken.Claims(&verified); err != nil {
		return OIDCIdentity{}, fmt.Errorf("decode id token claims: %w", err)
	}
	if verified.Nonce != expectedNonce {
		return OIDCIdentity{}, fmt.Errorf("oidc nonce mismatch")
	}

	userInfo, err := c.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return OIDCIdentity{}, fmt.Errorf("fetch userinfo: %w", err)
	}

	var claims IdentityClaims
	if err := userInfo.Claims(&claims); err != nil {
		return OIDCIdentity{}, fmt.Errorf("decode userinfo claims: %w", err)
	}

	claims.Subject = verified.Subject
	return OIDCIdentity{Claims: claims, RawIDToken: rawIDToken}, nil
}
```

### Step 1.6. provider identity と local user を結びつける service を作る

OIDC で user 情報が取れても、そのままではアプリ内 user になりません。ここで「provider identity を local user へどう対応づけるか」を 1 箇所へ固定します。

#### 追加するファイル

- `backend/internal/service/identity_service.go`

#### この service の入力

```go
type ExternalIdentity struct {
    Provider      string
    Subject       string
    Email         string
    EmailVerified bool
    DisplayName   string
}
```

#### この service の責務

1. `(provider, subject)` で既存 identity を探す
2. 見つかれば、その user を返す
3. 見つからず `email_verified=true` で同じ email の local user がいれば、その user に identity を紐づける
4. どちらもなければ、新しい user を作って identity を作る
5. email と display name は必要に応じて更新する

既存 user への自動リンクは、**verified email のときだけ**行ってください。未検証 email で既存 account と結びつけるのは危険です。

user 作成と identity 作成は分離できません。`sqlc` の generated query を tx 上で動かしてください。

最小実装は次の形です。

```go
type ExternalIdentity struct {
	Provider      string
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
}

func (s *IdentityService) ResolveOrCreateUser(ctx context.Context, identity ExternalIdentity) (User, error) {
	normalized, err := normalizeExternalIdentity(identity)
	if err != nil {
		return User{}, err
	}

	existing, err := s.queries.GetUserByProviderSubject(ctx, db.GetUserByProviderSubjectParams{
		Provider: normalized.Provider,
		Subject:  normalized.Subject,
	})
	if err == nil {
		_ = s.queries.UpdateUserIdentityProfile(ctx, db.UpdateUserIdentityProfileParams{
			Provider:      normalized.Provider,
			Subject:       normalized.Subject,
			Email:         normalized.Email,
			EmailVerified: normalized.EmailVerified,
		})
		return dbUser(existing.ID, existing.PublicID.String(), existing.Email, existing.DisplayName), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return User{}, fmt.Errorf("lookup identity by provider subject: %w", err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, fmt.Errorf("begin identity transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	qtx := s.queries.WithTx(tx)
	user, err := s.resolveUserForIdentity(ctx, qtx, normalized)
	if err != nil {
		return User{}, err
	}
	if err := qtx.CreateUserIdentity(ctx, db.CreateUserIdentityParams{
		UserID:        user.ID,
		Provider:      normalized.Provider,
		Subject:       normalized.Subject,
		Email:         normalized.Email,
		EmailVerified: normalized.EmailVerified,
	}); err != nil {
		return User{}, fmt.Errorf("create user identity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit identity transaction: %w", err)
	}

	return user, nil
}
```

### Step 1.7. callback 後に local session を払い出す service を作る

この repo の frontend は、provider token ではなく local Cookie session を前提に書かれています。ここを崩さないために、callback 後の session 発行を service として明示します。

#### 追加するファイル

- `backend/internal/service/oidc_login_service.go`

#### 依存関係

この service は次を受け取る形にすると分かりやすいです。

- OIDC client
- login state store
- identity service
- session store

#### ここでやる処理

1. `StartLogin(ctx)` で state / nonce / verifier を作る
2. OIDC client で authorize URL を作る
3. `FinishLogin(ctx, code, state)` で state を consume する
4. token exchange, ID token verify, userinfo 取得を行う
5. identity service で local user を得る
6. session store で `SESSION_ID` と `XSRF-TOKEN` を作る

#### `session_service.go` で見直す点

いまの `SessionService.Login()` は password login 専用です。そこで session 発行の中核だけを helper に切り出しておくと、password login と OIDC callback の両方で再利用できます。

例えば次のような内部関数を持たせます。

```go
func (s *SessionService) createSessionForUser(ctx context.Context, user User) (string, string, error)
```

あるいは `IssueSession(ctx, userID int64)` を public にしても構いません。大事なのは、**password 認証と session 発行を 1 メソッドに固定しすぎない**ことです。

Phase 2 で provider logout まで閉じるなら、ここで **raw ID token を session store に保存できる形**にしておくと後戻りしません。例えば `IssueSessionWithProviderHint(ctx, userID, rawIDToken)` のような形です。

最小実装は次の形です。

```go
type OIDCLoginResult struct {
	SessionID string
	CSRFToken string
	ReturnTo  string
}

func (s *OIDCLoginService) StartLogin(ctx context.Context, returnTo string) (string, error) {
	state, record, err := s.loginState.Create(ctx, sanitizeReturnTo(returnTo))
	if err != nil {
		return "", fmt.Errorf("create oidc login state: %w", err)
	}
	return s.oidcClient.AuthorizeURL(state, record.Nonce, record.CodeVerifier), nil
}

func (s *OIDCLoginService) FinishLogin(ctx context.Context, code, state string) (OIDCLoginResult, error) {
	loginState, err := s.loginState.Consume(ctx, state)
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("consume oidc login state: %w", err)
	}

	identity, err := s.oidcClient.ExchangeCode(ctx, code, loginState.CodeVerifier, loginState.Nonce)
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("finish oidc code exchange: %w", err)
	}

	user, err := s.identity.ResolveOrCreateUser(ctx, ExternalIdentity{
		Provider:      s.providerName,
		Subject:       identity.Claims.Subject,
		Email:         identity.Claims.Email,
		EmailVerified: identity.Claims.EmailVerified,
		DisplayName:   identity.Claims.Name,
	})
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("resolve local user for oidc identity: %w", err)
	}

	sessionID, csrfToken, err := s.sessionService.IssueSessionWithProviderHint(ctx, user.ID, identity.RawIDToken)
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("issue local session for oidc login: %w", err)
	}

	return OIDCLoginResult{
		SessionID: sessionID,
		CSRFToken: csrfToken,
		ReturnTo:  sanitizeReturnTo(loginState.ReturnTo),
	}, nil
}
```

### Step 1.8. Huma route を足す

backend の core がそろったので、最後に HTTP とつなぎます。

#### 追加するファイル

- `backend/internal/api/oidc.go`

#### 追加する endpoint

この repo では次の 2 本で十分です。

```text
GET /api/v1/auth/login
GET /api/v1/auth/callback
```

#### `GET /api/v1/auth/login`

やることは 1 つだけです。

1. `oidcLoginService.StartLogin()` から authorize URL を受け取る
2. `302 Found` と `Location` header を返す

#### `GET /api/v1/auth/callback`

やることは次です。

1. query から `code` と `state` を受け取る
2. `oidcLoginService.FinishLogin()` を呼ぶ
3. `Set-Cookie` に `SESSION_ID` と `XSRF-TOKEN` を入れる
4. `302 Found` で frontend へ戻す

frontend へ戻す先は、まずは固定で `cfg.FrontendBaseURL + "/"` で構いません。失敗時だけ `cfg.FrontendBaseURL + "/login?error=oidc_callback_failed"` に戻す形が扱いやすいです。

#### 変更が入る既存ファイル

- `backend/internal/api/register.go`
- `backend/internal/app/app.go`
- `backend/cmd/main/main.go`
- `backend/cmd/openapi/main.go`

`Dependencies` へ新しい service を足し、`app.New()` で注入してください。

route 登録の最小実装は次です。

```go
func registerOIDCRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID:   "startOIDCLogin",
		Method:        http.MethodGet,
		Path:          "/api/v1/auth/login",
		DefaultStatus: http.StatusFound,
	}, func(ctx context.Context, input *StartOIDCLoginInput) (*StartOIDCLoginOutput, error) {
		location, err := deps.OIDCLoginService.StartLogin(ctx, input.ReturnTo)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to start oidc login")
		}
		return &StartOIDCLoginOutput{Location: location}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "finishOIDCLogin",
		Method:        http.MethodGet,
		Path:          "/api/v1/auth/callback",
		DefaultStatus: http.StatusFound,
	}, func(ctx context.Context, input *OIDCCallbackInput) (*OIDCCallbackOutput, error) {
		if input.Error != "" {
			return &OIDCCallbackOutput{Location: oidcFailureRedirect(deps.FrontendBaseURL)}, nil
		}

		result, err := deps.OIDCLoginService.FinishLogin(ctx, input.Code, input.State)
		if err != nil {
			return &OIDCCallbackOutput{Location: oidcFailureRedirect(deps.FrontendBaseURL)}, nil
		}

		return &OIDCCallbackOutput{
			SetCookie: []http.Cookie{
				auth.NewSessionCookie(result.SessionID, deps.CookieSecure, deps.SessionTTL),
				auth.NewXSRFCookie(result.CSRFToken, deps.CookieSecure, deps.SessionTTL),
			},
			Location: oidcSuccessRedirect(deps.FrontendBaseURL, result.ReturnTo),
		}, nil
	})
}
```

### Step 1.9. frontend の login 画面を本実装へ寄せる

frontend は契約の利用者です。backend の route と redirect 先が固まったので、`AUTH_MODE=zitadel` のときは password form を出さず、browser redirect のボタンだけ出してください。

#### `frontend/src/views/LoginView.vue`

最小構成なら次で十分です。

```vue
<a class="primary-button" href="/api/v1/auth/login">
  Sign in with Zitadel
</a>
```

`/api` は Vite proxy を通るので、開発中でも相対 path のままで問題ありません。

#### error 表示

callback 失敗時に `/login?error=oidc_callback_failed` へ戻すなら、`route.query.error` を読んで簡単な文言を出してください。

#### `frontend/src/api/auth.ts`

既存の `fetchAuthSettings()` はそのまま使えます。今回の login 開始は SDK 経由の XHR ではなく browser redirect なので、新しい fetch wrapper を足す必要はありません。

#### この Phase で実際に触るファイル

- `db/migrations/0002_user_identities.up.sql`
- `db/migrations/0002_user_identities.down.sql`
- `db/queries/users.sql`
- `db/queries/identities.sql`
- `backend/internal/auth/login_state_store.go`
- `backend/internal/auth/oidc_client.go`
- `backend/internal/service/identity_service.go`
- `backend/internal/service/oidc_login_service.go`
- `backend/internal/api/oidc.go`
- `backend/internal/api/register.go`
- `backend/internal/app/app.go`
- `frontend/src/views/LoginView.vue`
- `openapi/openapi.yaml`

### Step 1.10. 生成物を更新して手動確認する

この repo の Makefile にはすでに `up`, `db-up`, `backend-dev`, `frontend-dev` があります。Phase 1 の最後にまとめて流します。

#### コマンド

```bash
# 依存がまだ無ければ 1 回だけ
npm --prefix frontend install

make up
make db-up
make db-schema
make gen
go test ./backend/...
npm --prefix frontend run build

make backend-dev
make frontend-dev
```

`backend-dev` と `frontend-dev` は別 terminal で起動してください。現在の repo は `frontend/vite.config.ts` で `host: '127.0.0.1'`, `port: 5173`, `strictPort: true` を固定してください。**5173 が埋まっていると Vite が 5174 へ逃げる構成のままだと、Zitadel の redirect / post logout redirect と食い違います。**

#### 確認順

1. `curl -sS http://127.0.0.1:8080/api/v1/auth/settings` を叩き、`"mode":"zitadel"` と `issuer` が返ることを確認する
2. `curl -sS -D - -o /dev/null http://127.0.0.1:8080/api/v1/auth/login` を叩き、`302 Found` と `Location: http://localhost:8081/oauth/v2/authorize?...` が返ることを確認する
3. browser で `http://127.0.0.1:5173/login` を開く
4. Zitadel login button が見える
5. button クリックで provider の login 画面へ遷移する
6. login 成功後に frontend へ戻る
7. `GET /api/v1/session` が authenticated を返す
8. 既存の session-based UI が従来通り動く

ここで見たい本質は、**login 方式は変わっても、login 後の業務 API 利用はまったく同じ**という点です。

補足として、Zitadel Login UI v2 では callback URL を一度 `fetch` しようとして、`connect-src 'self'` の CSP warning を console に出すことがあります。local 構成で callback が `http://127.0.0.1:8080`、login UI が `http://localhost:8081` のときに起きやすい挙動ですが、その後 **browser navigation へフォールバックして login 自体は成功**します。`GET /api/v1/auth/callback` が `302` を返し、最終的に frontend に戻れていれば、この warning だけで失敗とは判断しないでください。

---

## Phase 2. Browser Cookie Session, Logout, CSRF 再発行

### 目的

browser 向け auth foundation を仕上げます。logout, session refresh, CSRF bootstrap をここで固めます。

### この Phase の前提

- Phase 1 の browser login が通っている
- callback 後に local session cookie を発行できている

### この Phase の完了条件

- canonical logout が `POST /api/v1/logout` のまま閉じている
- provider logout が必要な場合でも `GET` で state change しない
- `GET /api/v1/csrf` と `POST /api/v1/session/refresh` が動く
- frontend wrapper が CSRF bootstrap を吸収する

### Step 2.1. browser 向け logout を最終形にする

Step 1 までで login は通りますが、provider 側 session が残ったままだと「logout したのに次回すぐ復帰する」挙動が起きます。ここを browser 向けに閉じます。

#### canonical endpoint

browser logout の正規経路は **`POST /api/v1/logout`** のままにしてください。

- local state change は `POST`
- CSRF 検証を必須にする
- `GET /api/v1/auth/logout` のような state-changing route は追加しない

#### 実装方針

- `POST /api/v1/logout` はまず local session を削除する
- `SESSION_ID` と `XSRF-TOKEN` は必ず expired cookie を返す
- response は **常に `200 OK`** に固定する
- provider logout が不要なら `postLogoutURL` を空で返す
- provider logout が必要なら、**POST 成功後に browser が遷移すべき `postLogoutURL` を返す**

#### 返り値の形

local-only logout でも provider logout 連携でも扱いやすいよう、最終形は次のような body に寄せると実装しやすいです。

```json
{
  "postLogoutURL": "https://<your-zitadel-domain>/oidc/v1/end_session?..."
}
```

`postLogoutURL` が空なら frontend はそのまま `/login` へ戻り、値があるときだけ `window.location.assign(postLogoutURL)` で top-level navigation してください。

#### server 側で持つ情報

- callback 時に **`id_token_hint` として使う raw ID token を session へ保持する**
- Zitadel の `end_session_endpoint` を使う場合は `post_logout_redirect_uri` を必ず registration 済みの値に合わせる

session record は最低限次の形にしておくと実装がぶれません。

```go
type SessionRecord struct {
    UserID              int64  `json:"userId"`
    CSRFToken           string `json:"csrfToken"`
    ProviderIDTokenHint string `json:"providerIdTokenHint,omitempty"`
}
```

session の作成と rotate は次の形にすると十分です。

```go
func (s *SessionStore) CreateWithProviderHint(ctx context.Context, userID int64, providerIDTokenHint string) (string, string, error) {
	sessionID, err := randomToken(32)
	if err != nil {
		return "", "", err
	}
	csrfToken, err := randomToken(32)
	if err != nil {
		return "", "", err
	}

	record := SessionRecord{
		UserID:              userID,
		CSRFToken:           csrfToken,
		ProviderIDTokenHint: providerIDTokenHint,
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

func (s *SessionStore) Rotate(ctx context.Context, sessionID string) (string, string, error) {
	record, _, err := s.loadWithTTL(ctx, sessionID)
	if err != nil {
		return "", "", err
	}

	newSessionID, err := randomToken(32)
	if err != nil {
		return "", "", err
	}
	newCSRFToken, err := randomToken(32)
	if err != nil {
		return "", "", err
	}

	record.CSRFToken = newCSRFToken
	if err := s.save(ctx, newSessionID, record, s.ttl); err != nil {
		return "", "", err
	}
	if err := s.Delete(ctx, sessionID); err != nil {
		return "", "", err
	}

	return newSessionID, newCSRFToken, nil
}
```

`client_id` だけでも `end_session_endpoint` は呼べますが、**Zitadel Login UI の logout 画面へフォールバックして止まることがあります**。browser から `Logout` を押したあと確実に RP initiated logout を閉じたいなら、`id_token_hint` を使って `end_session_endpoint` を組み立てるほうが安定します。

#### frontend 側

- `frontend/src/views/HomeView.vue` の logout 導線は、まず `POST /api/v1/logout` を呼ぶ
- `frontend/src/stores/session.ts` は、成功時だけ local state を `anonymous` に落とす
- `postLogoutURL` が返ったときだけ browser navigation を行う

handler 側の最小実装は次です。

```go
func buildPostLogoutURL(deps Dependencies, idTokenHint string) string {
	if deps.AuthMode != "zitadel" || deps.ZitadelIssuer == "" || deps.ZitadelClientID == "" || deps.ZitadelPostLogoutRedirectURI == "" {
		return ""
	}

	endSessionURL, err := url.Parse(deps.ZitadelIssuer)
	if err != nil {
		return ""
	}
	endSessionURL.Path = "/oidc/v1/end_session"

	query := endSessionURL.Query()
	if idTokenHint != "" {
		query.Set("id_token_hint", idTokenHint)
	} else {
		query.Set("client_id", deps.ZitadelClientID)
	}
	query.Set("post_logout_redirect_uri", deps.ZitadelPostLogoutRedirectURI)
	endSessionURL.RawQuery = query.Encode()

	return endSessionURL.String()
}
```

### Step 2.2. セッション再発行と CSRF bootstrap を整える

`CONCEPT.md` では、`XSRF-TOKEN` は login 成功時だけでなく、セッション再発行時にも払い出す前提です。また、SPA 初回ロード時に token が無いなら bootstrap endpoint で補う方針です。ここを詰めると browser 向け auth foundation が安定します。

#### 追加する endpoint

最小構成なら次の 2 本で十分です。

```text
GET /api/v1/csrf
POST /api/v1/session/refresh
```

#### `GET /api/v1/csrf`

- 有効な session がある場合だけ `XSRF-TOKEN` を再発行する
- body は空でも構いません
- frontend は token が無い場合だけ呼びます

#### `POST /api/v1/session/refresh`

- 現在の session を検証する
- session ID を rotate する
- `SESSION_ID` と `XSRF-TOKEN` を両方再発行する
- TTL を延長する

#### transport wrapper で固定する前後関係

`frontend/src/api/client.ts` では次の順序を固定してください。

1. state-changing request の前に `XSRF-TOKEN` cookie を確認する
2. token が無ければ、まず `GET /api/v1/csrf` を 1 回だけ試す
3. そのあとで元の request に `X-CSRF-Token` header を付ける
4. `POST /api/v1/session/refresh` 自身もこの規則に従う

つまり、**`session/refresh` だから特別扱いして CSRF を飛ばす、という分岐は作らない**構成にします。

#### 変更が入るファイル

- `backend/internal/auth/session_store.go`
- `backend/internal/service/session_service.go`
- `backend/internal/api/session.go`
- `frontend/src/api/client.ts`
- `frontend/src/views/HomeView.vue`
- `frontend/src/stores/session.ts`

route 登録の最小実装は次です。

```go
huma.Register(api, huma.Operation{
	OperationID:   "getCSRF",
	Method:        http.MethodGet,
	Path:          "/api/v1/csrf",
	DefaultStatus: http.StatusNoContent,
	Security:      []map[string][]string{{"cookieAuth": {}}},
}, func(ctx context.Context, input *GetCSRFInput) (*GetCSRFOutput, error) {
	csrfToken, err := deps.SessionService.ReissueCSRF(ctx, input.SessionCookie.Value)
	if err != nil {
		return nil, toHTTPError(err)
	}
	return &GetCSRFOutput{
		SetCookie: []http.Cookie{
			auth.NewXSRFCookie(csrfToken, deps.CookieSecure, deps.SessionTTL),
		},
	}, nil
})

huma.Register(api, huma.Operation{
	OperationID:   "refreshSession",
	Method:        http.MethodPost,
	Path:          "/api/v1/session/refresh",
	DefaultStatus: http.StatusNoContent,
	Security:      []map[string][]string{{"cookieAuth": {}}},
}, func(ctx context.Context, input *RefreshSessionInput) (*RefreshSessionOutput, error) {
	sessionID, csrfToken, err := deps.SessionService.RefreshSession(ctx, input.SessionCookie.Value, input.CSRFToken)
	if err != nil {
		return nil, toHTTPError(err)
	}
	return &RefreshSessionOutput{
		SetCookie: []http.Cookie{
			auth.NewSessionCookie(sessionID, deps.CookieSecure, deps.SessionTTL),
			auth.NewXSRFCookie(csrfToken, deps.CookieSecure, deps.SessionTTL),
		},
	}, nil
})

huma.Register(api, huma.Operation{
	OperationID:   "logout",
	Method:        http.MethodPost,
	Path:          "/api/v1/logout",
	DefaultStatus: http.StatusOK,
	Security:      []map[string][]string{{"cookieAuth": {}}},
}, func(ctx context.Context, input *LogoutInput) (*LogoutOutput, error) {
	idTokenHint, err := deps.SessionService.Logout(ctx, input.SessionCookie.Value, input.CSRFToken)
	if err != nil && !errors.Is(err, service.ErrUnauthorized) {
		return nil, toHTTPError(err)
	}

	return &LogoutOutput{
		SetCookie: []http.Cookie{
			auth.ExpiredSessionCookie(deps.CookieSecure),
			auth.ExpiredXSRFCookie(deps.CookieSecure),
		},
		Body: LogoutBody{
			PostLogoutURL: buildPostLogoutURL(deps, idTokenHint),
		},
	}, nil
})
```

#### この Phase の手動確認

1. login 後に `XSRF-TOKEN` cookie がある
2. Home 画面で `Logout` を押し、最終的に `http://127.0.0.1:5173/login` に戻る
3. もう一度 login する
4. browser devtools で `XSRF-TOKEN` cookie だけを消した状態で mutating request を打つ
5. Network タブで wrapper が `GET /api/v1/csrf` を先に呼び、その後の request が通ることを確認する
6. `POST /api/v1/session/refresh` 後に `SESSION_ID` と `XSRF-TOKEN` の両方が更新される
7. `POST /api/v1/logout` 後に `GET /api/v1/session` が `401` になる

authenticated session が無いときの smoke check は次で足ります。

```bash
curl -i http://127.0.0.1:8080/api/v1/csrf
curl -i -X POST http://127.0.0.1:8080/api/v1/session/refresh -H 'X-CSRF-Token: dummy'
```

どちらも cookie が無ければ `401` になり、route が存在することだけ確認できます。逆に `X-CSRF-Token` header を付けずに `POST /api/v1/session/refresh` を叩くと、先に request validation が走って `422` になります。

### Phase 0-2 を fresh clone から再現する最短手順

ここまでを最短で再現するなら、順番は次で十分です。

1. HaoHao の依存 service を起動する

```bash
make up
```

2. Zitadel quickstart を起動する

```bash
make zitadel-up
```

3. Zitadel Console で `haohao` project, roles, browser 用 `Web` application を作る

quickstart が作る `login-client.pat` だけでは project 作成権限が無いため、この Step は Console 上の admin 操作として扱います。

- roles: `docs_reader`, `external_api_user`, `todo_user`
- OIDC Settings: `CODE`
- redirect URI: `http://127.0.0.1:8080/api/v1/auth/callback`
- post logout redirect URI: `http://127.0.0.1:5173/login`

4. repo root の `.env` に browser app の client 情報を入れる

```dotenv
AUTH_MODE=zitadel
ZITADEL_ISSUER=http://localhost:8081
ZITADEL_CLIENT_ID=<browser-app-client-id>
ZITADEL_CLIENT_SECRET=<browser-app-client-secret>
ZITADEL_REDIRECT_URI=http://127.0.0.1:8080/api/v1/auth/callback
ZITADEL_POST_LOGOUT_REDIRECT_URI=http://127.0.0.1:5173/login
ZITADEL_SCOPES="openid profile email"
LOGIN_STATE_TTL=10m
```

5. migration, 生成物, backend test を流す

```bash
npm --prefix frontend install
make db-up
make db-schema
make gen
go test ./backend/...
npm --prefix frontend run build
```

6. backend と frontend を起動する

```bash
make backend-dev
make frontend-dev
```

`frontend-dev` は `5173` を使えないと失敗する設定にしてください。別の Vite dev server が残っていると Phase 1-2 の redirect URI 検証がずれます。

7. browser で login と logout を確認する

- `http://127.0.0.1:5173/login` を開く
- `Sign in with Zitadel` を押す
- Home まで戻る
- `Logout` を押す
- `http://127.0.0.1:5173/login` に戻る

---

## Phase 0-2 Exact Snapshot

ここまでの本文は「なぜその形にするか」を説明するために、意図的に簡略化した snippet も混ぜています。  
**いまの repo と同じ状態を `TUTORIAL_ZITADEL.md` だけで再現したい場合は、この節の code block を正本として使ってください。**

- この節の block は、現在の Phase 0-2 実装に合わせた **exact snapshot** です
- 上の本文にある簡略 snippet と食い違う場合は、この節を優先してください
- tracked の `.env.example` は local mode を default にしています。実際に Zitadel login を検証するときだけ、repo root の `.env` を `AUTH_MODE=zitadel` に切り替えてください
- `db/schema.sql` は `make db-schema` の生成物です
- `backend/go.sum` は `cd backend && go mod tidy` の生成物です
- `backend/internal/db/*.go`, `openapi/openapi.yaml`, `frontend/src/api/generated/*` は `make gen` の生成物なので、ここでは再掲しません

### Project Exact Files

#### `Makefile`

```make
SHELL := /bin/bash

export-env = set -a && source .env && set +a
DOCKER_COMPOSE := $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; elif command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"; else echo "docker compose"; fi)
ZITADEL_ENV_FILE := dev/zitadel/.env
ZITADEL_ENV_EXAMPLE := dev/zitadel/.env.example
ZITADEL_COMPOSE_FILE := dev/zitadel/docker-compose.yml
ZITADEL_COMPOSE := $(DOCKER_COMPOSE) --env-file $(ZITADEL_ENV_FILE) -f $(ZITADEL_COMPOSE_FILE)

up:
	$(DOCKER_COMPOSE) up -d

down:
	$(DOCKER_COMPOSE) down

zitadel-env:
	@test -f $(ZITADEL_ENV_FILE) || cp $(ZITADEL_ENV_EXAMPLE) $(ZITADEL_ENV_FILE)

zitadel-up: zitadel-env
	$(ZITADEL_COMPOSE) up -d --wait

zitadel-down: zitadel-env
	$(ZITADEL_COMPOSE) down

zitadel-ps: zitadel-env
	$(ZITADEL_COMPOSE) ps

zitadel-logs: zitadel-env
	$(ZITADEL_COMPOSE) logs -f

db-wait:
	@until $(DOCKER_COMPOSE) exec -T postgres pg_isready -U haohao -d haohao >/dev/null 2>&1; do sleep 1; done

db-up: db-wait
	$(export-env) && migrate -path db/migrations -database "$$DATABASE_URL" up

db-down:
	$(export-env) && migrate -path db/migrations -database "$$DATABASE_URL" down 1

db-schema: db-wait
	$(DOCKER_COMPOSE) exec -T postgres pg_dump --schema-only --no-owner --no-privileges -U haohao -d haohao | sed '/^\\restrict /d; /^\\unrestrict /d' > db/schema.sql

seed-demo-user: db-wait
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

#### `dev/zitadel/.env.example`

```dotenv
# ⚠️  INSECURE DEFAULTS — for local development only.
# Before exposing this stack to a network, change ZITADEL_MASTERKEY,
# POSTGRES_ADMIN_PASSWORD, and POSTGRES_ZITADEL_PASSWORD.
# See: https://zitadel.com/docs/self-hosting/deploy/compose#homelab

# -----------------------------
# Domain and external URL
# -----------------------------
ZITADEL_DOMAIN=localhost
# Used by the base (non-TLS) compose; TLS overlays also publish 80/443 and may still expose
# this port unless their ports are adjusted.
PROXY_HTTP_PUBLISHED_PORT=8081
ZITADEL_EXTERNALPORT=8081
ZITADEL_EXTERNALSECURE=false
# Overridden by TLS overlays (they hardcode X-Forwarded-Proto: https).
ZITADEL_PUBLIC_SCHEME=http

# -----------------------------
# Security/bootstrap
# -----------------------------
# Must be exactly 32 chars for new deployments.
ZITADEL_MASTERKEY=MasterkeyNeedsToHave32Characters
LOGIN_CLIENT_PAT_EXPIRATION=2099-01-01T00:00:00Z

# -----------------------------
# Pinned image tags
# To upgrade ZITADEL, bump ZITADEL_VERSION and run:
#   docker compose --env-file .env -f docker-compose.yml pull
#   docker compose --env-file .env -f docker-compose.yml up -d --wait
# -----------------------------
ZITADEL_VERSION=v4.13.0
TRAEFIK_IMAGE=traefik:v3.6.8
POSTGRES_IMAGE=postgres:17.2-alpine
REDIS_IMAGE=redis:7.4.2-alpine
OTEL_COLLECTOR_IMAGE=otel/opentelemetry-collector-contrib:0.114.0

# -----------------------------
# Proxy settings
# -----------------------------
TRAEFIK_DASHBOARD_ENABLED=false
TRAEFIK_LOG_LEVEL=INFO
TRAEFIK_ACCESSLOG_ENABLED=true
# Trusted proxy IPs for X-Forwarded-* headers (external-tls mode).
# Comma-separated CIDR ranges of your upstream load balancer / reverse proxy.
TRAEFIK_TRUSTED_IPS=10.0.0.0/8,172.16.0.0/12,192.168.0.0/16

# Let's Encrypt mode
LETSENCRYPT_EMAIL=ops@example.com

# -----------------------------
# Postgres settings
# -----------------------------
POSTGRES_DB=zitadel
POSTGRES_ADMIN_USER=postgres
POSTGRES_ADMIN_PASSWORD=postgres
# DSN used by ZITADEL to connect to PostgreSQL.
# The start-from-init command uses this DSN to create the database schema and for
# normal operation. When a DSN is configured, ZITADEL does not create or switch
# to a separate unprivileged database user; the user in this DSN is used directly
# and must already exist. For production, configure this DSN to use a non-superuser
# role with only the permissions ZITADEL needs.
# See https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
ZITADEL_DATABASE_POSTGRES_DSN=postgresql://postgres:postgres@postgres:5432/zitadel?sslmode=disable

# -----------------------------
# Access logs
# -----------------------------
ZITADEL_ACCESS_LOG_STDOUT_ENABLED=true

# -----------------------------
# Optional Redis cache settings
# Default is disabled.
# Enable with --profile cache and switch connectors to redis.
# -----------------------------
ZITADEL_CACHES_CONNECTORS_REDIS_ENABLED=false
# DSN used by ZITADEL to connect to Redis.
# See https://redis.io/docs/latest/develop/tools/cli/#host-port-password-and-database
ZITADEL_CACHES_CONNECTORS_REDIS_URL=redis://redis:6379/0
ZITADEL_CACHES_INSTANCE_CONNECTOR=
ZITADEL_CACHES_MILESTONES_CONNECTOR=
ZITADEL_CACHES_ORGANIZATION_CONNECTOR=

# -----------------------------
# Optional API tracing (OTEL)
# Default is disabled. Enable with --profile observability.
# The collector receives traces from ZITADEL on port 4317 (gRPC) and 4318 (HTTP)
# and logs them to stdout by default.
# To forward traces to your own backend (Grafana Tempo, Jaeger, OpenObserve, etc.),
# uncomment the otlp exporter in otel-collector-config.yaml and set:
# OTEL_BACKEND_ENDPOINT=http://your-backend:4317
# -----------------------------
ZITADEL_INSTRUMENTATION_SERVICENAME=zitadel-api
ZITADEL_INSTRUMENTATION_TRACE_EXPORTER_TYPE=none
ZITADEL_INSTRUMENTATION_TRACE_EXPORTER_ENDPOINT=otel-collector:4317
ZITADEL_INSTRUMENTATION_TRACE_EXPORTER_INSECURE=true

# -----------------------------
# Future Login OTEL placeholders
# Current login image may ignore these.
# -----------------------------
LOGIN_OTEL_SERVICE_NAME=zitadel-login
LOGIN_OTEL_EXPORTER_OTLP_ENDPOINT=
LOGIN_OTEL_EXPORTER_OTLP_PROTOCOL=grpc
```

#### `dev/zitadel/docker-compose.yml`

```yaml
name: zitadel

services:
  proxy:
    image: ${TRAEFIK_IMAGE}
    restart: unless-stopped
    command:
      - --providers.docker=true
      - --providers.docker.exposedbydefault=false
      - --providers.docker.network=zitadel
      - --entrypoints.web.address=:80
      - --entrypoints.websecure.address=:443
      - --api.dashboard=${TRAEFIK_DASHBOARD_ENABLED}
      - --api.insecure=false
      - --ping=true
      - --ping.entrypoint=web
      - --log.level=${TRAEFIK_LOG_LEVEL}
      - --accesslog=${TRAEFIK_ACCESSLOG_ENABLED}
    ports:
      - ${PROXY_HTTP_PUBLISHED_PORT}:80
    networks:
      - zitadel
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    depends_on:
      zitadel-api:
        condition: service_healthy
      zitadel-login:
        condition: service_healthy

  zitadel-api:
    image: ghcr.io/zitadel/zitadel:${ZITADEL_VERSION}
    restart: unless-stopped
    user: "0"
    command: start-from-init --masterkey "${ZITADEL_MASTERKEY}"
    environment:
      ZITADEL_PORT: 8080
      ZITADEL_EXTERNALDOMAIN: ${ZITADEL_DOMAIN}
      ZITADEL_EXTERNALPORT: ${ZITADEL_EXTERNALPORT}
      ZITADEL_EXTERNALSECURE: ${ZITADEL_EXTERNALSECURE}
      ZITADEL_TLS_ENABLED: false

      ZITADEL_DATABASE_POSTGRES_DSN: ${ZITADEL_DATABASE_POSTGRES_DSN}

      ZITADEL_FIRSTINSTANCE_ORG_HUMAN_PASSWORDCHANGEREQUIRED: false
      ZITADEL_FIRSTINSTANCE_LOGINCLIENTPATPATH: /zitadel/bootstrap/login-client.pat
      ZITADEL_FIRSTINSTANCE_ORG_LOGINCLIENT_MACHINE_USERNAME: login-client
      ZITADEL_FIRSTINSTANCE_ORG_LOGINCLIENT_MACHINE_NAME: Automatically Initialized IAM_LOGIN_CLIENT
      ZITADEL_FIRSTINSTANCE_ORG_LOGINCLIENT_PAT_EXPIRATIONDATE: ${LOGIN_CLIENT_PAT_EXPIRATION}

      ZITADEL_DEFAULTINSTANCE_FEATURES_LOGINV2_REQUIRED: true
      ZITADEL_DEFAULTINSTANCE_FEATURES_LOGINV2_BASEURI: ${ZITADEL_PUBLIC_SCHEME}://${ZITADEL_DOMAIN}:${ZITADEL_EXTERNALPORT}/ui/v2/login/
      ZITADEL_OIDC_DEFAULTLOGINURLV2: ${ZITADEL_PUBLIC_SCHEME}://${ZITADEL_DOMAIN}:${ZITADEL_EXTERNALPORT}/ui/v2/login/login?authRequest=
      ZITADEL_OIDC_DEFAULTLOGOUTURLV2: ${ZITADEL_PUBLIC_SCHEME}://${ZITADEL_DOMAIN}:${ZITADEL_EXTERNALPORT}/ui/v2/login/logout?post_logout_redirect=
      ZITADEL_SAML_DEFAULTLOGINURLV2: ${ZITADEL_PUBLIC_SCHEME}://${ZITADEL_DOMAIN}:${ZITADEL_EXTERNALPORT}/ui/v2/login/login?samlRequest=

      ZITADEL_LOGSTORE_ACCESS_STDOUT_ENABLED: ${ZITADEL_ACCESS_LOG_STDOUT_ENABLED}

      ZITADEL_INSTRUMENTATION_TRACE_EXPORTER_TYPE: ${ZITADEL_INSTRUMENTATION_TRACE_EXPORTER_TYPE}
      ZITADEL_INSTRUMENTATION_TRACE_EXPORTER_ENDPOINT: ${ZITADEL_INSTRUMENTATION_TRACE_EXPORTER_ENDPOINT}
      ZITADEL_INSTRUMENTATION_TRACE_EXPORTER_INSECURE: ${ZITADEL_INSTRUMENTATION_TRACE_EXPORTER_INSECURE}
      ZITADEL_INSTRUMENTATION_SERVICENAME: ${ZITADEL_INSTRUMENTATION_SERVICENAME}

      ZITADEL_CACHES_CONNECTORS_REDIS_ENABLED: ${ZITADEL_CACHES_CONNECTORS_REDIS_ENABLED}
      ZITADEL_CACHES_CONNECTORS_REDIS_URL: ${ZITADEL_CACHES_CONNECTORS_REDIS_URL}
      ZITADEL_CACHES_INSTANCE_CONNECTOR: ${ZITADEL_CACHES_INSTANCE_CONNECTOR}
      ZITADEL_CACHES_MILESTONES_CONNECTOR: ${ZITADEL_CACHES_MILESTONES_CONNECTOR}
      ZITADEL_CACHES_ORGANIZATION_CONNECTOR: ${ZITADEL_CACHES_ORGANIZATION_CONNECTOR}

    healthcheck:
      test:
        - CMD
        - /app/zitadel
        - ready
      interval: 10s
      timeout: 30s
      retries: 12
      start_period: 20s
    volumes:
      - zitadel-bootstrap:/zitadel/bootstrap:rw
    networks:
      - zitadel
    depends_on:
      postgres:
        condition: service_healthy
    labels:
      - traefik.enable=true
      - traefik.docker.network=zitadel

      - traefik.http.services.zitadel-api.loadbalancer.server.port=8080
      - traefik.http.services.zitadel-api.loadbalancer.server.scheme=h2c

      - traefik.http.middlewares.zitadel-strip-api.stripprefix.prefixes=/api
      - traefik.http.middlewares.zitadel-strip-api.stripprefix.forceSlash=false

      # Note: no dedicated gRPC router needed. All gRPC and Connect-RPC traffic
      # is handled by the catch-all router below since the backend already uses h2c.

      - traefik.http.routers.zitadel-api-alias-web.rule=Host(`${ZITADEL_DOMAIN}`) && PathPrefix(`/api`)
      - traefik.http.routers.zitadel-api-alias-web.entrypoints=web
      - traefik.http.routers.zitadel-api-alias-web.middlewares=zitadel-strip-api
      - traefik.http.routers.zitadel-api-alias-web.service=zitadel-api
      - traefik.http.routers.zitadel-api-alias-web.priority=200

      - traefik.http.routers.zitadel-api-alias-websecure.rule=Host(`${ZITADEL_DOMAIN}`) && PathPrefix(`/api`)
      - traefik.http.routers.zitadel-api-alias-websecure.entrypoints=websecure
      - traefik.http.routers.zitadel-api-alias-websecure.tls=true
      - traefik.http.routers.zitadel-api-alias-websecure.middlewares=zitadel-strip-api
      - traefik.http.routers.zitadel-api-alias-websecure.service=zitadel-api
      - traefik.http.routers.zitadel-api-alias-websecure.priority=200

      - traefik.http.routers.zitadel-canonical-web.rule=Host(`${ZITADEL_DOMAIN}`) && !PathPrefix(`/ui/v2/login`) && !PathPrefix(`/api`) && !Path(`/`)
      - traefik.http.routers.zitadel-canonical-web.entrypoints=web
      - traefik.http.routers.zitadel-canonical-web.service=zitadel-api
      - traefik.http.routers.zitadel-canonical-web.priority=100

      - traefik.http.routers.zitadel-canonical-websecure.rule=Host(`${ZITADEL_DOMAIN}`) && !PathPrefix(`/ui/v2/login`) && !PathPrefix(`/api`) && !Path(`/`)
      - traefik.http.routers.zitadel-canonical-websecure.entrypoints=websecure
      - traefik.http.routers.zitadel-canonical-websecure.tls=true
      - traefik.http.routers.zitadel-canonical-websecure.service=zitadel-api
      - traefik.http.routers.zitadel-canonical-websecure.priority=100

  zitadel-login:
    image: ghcr.io/zitadel/zitadel-login:${ZITADEL_VERSION}
    restart: unless-stopped
    user: "0"
    environment:
      ZITADEL_API_URL: http://zitadel-api:8080
      NEXT_PUBLIC_BASE_PATH: /ui/v2/login
      ZITADEL_SERVICE_USER_TOKEN_FILE: /zitadel/bootstrap/login-client.pat
      CUSTOM_REQUEST_HEADERS: Host:${ZITADEL_DOMAIN},X-Forwarded-Proto:${ZITADEL_PUBLIC_SCHEME}

      # Future-ready placeholders for upcoming Login OTEL support.
      OTEL_SERVICE_NAME: ${LOGIN_OTEL_SERVICE_NAME}
      OTEL_EXPORTER_OTLP_ENDPOINT: ${LOGIN_OTEL_EXPORTER_OTLP_ENDPOINT}
      OTEL_EXPORTER_OTLP_PROTOCOL: ${LOGIN_OTEL_EXPORTER_OTLP_PROTOCOL}
    healthcheck:
      test:
        - CMD
        - /bin/sh
        - -c
        - node /app/healthcheck.mjs http://localhost:3000/ui/v2/login/healthy
      interval: 10s
      timeout: 30s
      retries: 12
      start_period: 20s
    volumes:
      - zitadel-bootstrap:/zitadel/bootstrap:ro
    networks:
      - zitadel
    depends_on:
      zitadel-api:
        condition: service_healthy
    labels:
      - traefik.enable=true
      - traefik.docker.network=zitadel
      - traefik.http.services.zitadel-login.loadbalancer.server.port=3000

      - traefik.http.middlewares.zitadel-root-rewrite.replacepath.path=/ui/v2/login/

      - traefik.http.routers.zitadel-root-web.rule=Host(`${ZITADEL_DOMAIN}`) && Path(`/`)
      - traefik.http.routers.zitadel-root-web.entrypoints=web
      - traefik.http.routers.zitadel-root-web.middlewares=zitadel-root-rewrite
      - traefik.http.routers.zitadel-root-web.service=zitadel-login
      - traefik.http.routers.zitadel-root-web.priority=400

      - traefik.http.routers.zitadel-root-websecure.rule=Host(`${ZITADEL_DOMAIN}`) && Path(`/`)
      - traefik.http.routers.zitadel-root-websecure.entrypoints=websecure
      - traefik.http.routers.zitadel-root-websecure.tls=true
      - traefik.http.routers.zitadel-root-websecure.middlewares=zitadel-root-rewrite
      - traefik.http.routers.zitadel-root-websecure.service=zitadel-login
      - traefik.http.routers.zitadel-root-websecure.priority=400

      - traefik.http.routers.zitadel-login-web.rule=Host(`${ZITADEL_DOMAIN}`) && PathPrefix(`/ui/v2/login`)
      - traefik.http.routers.zitadel-login-web.entrypoints=web
      - traefik.http.routers.zitadel-login-web.service=zitadel-login
      - traefik.http.routers.zitadel-login-web.priority=250

      - traefik.http.routers.zitadel-login-websecure.rule=Host(`${ZITADEL_DOMAIN}`) && PathPrefix(`/ui/v2/login`)
      - traefik.http.routers.zitadel-login-websecure.entrypoints=websecure
      - traefik.http.routers.zitadel-login-websecure.tls=true
      - traefik.http.routers.zitadel-login-websecure.service=zitadel-login
      - traefik.http.routers.zitadel-login-websecure.priority=250

  postgres:
    image: ${POSTGRES_IMAGE}
    restart: unless-stopped
    environment:
      POSTGRES_PASSWORD: ${POSTGRES_ADMIN_PASSWORD}
      POSTGRES_USER: ${POSTGRES_ADMIN_USER}
      POSTGRES_DB: ${POSTGRES_DB}
    healthcheck:
      test:
        - CMD-SHELL
        - pg_isready -d ${POSTGRES_DB} -U ${POSTGRES_ADMIN_USER}
      interval: 10s
      timeout: 30s
      retries: 10
      start_period: 20s
    volumes:
      - postgres-data:/var/lib/postgresql/data:rw
    networks:
      - zitadel

  redis:
    image: ${REDIS_IMAGE}
    restart: unless-stopped
    profiles:
      - cache
    command:
      - --save
      - ""
      - --appendonly
      - "no"
    networks:
      - zitadel

  otel-collector:
    image: ${OTEL_COLLECTOR_IMAGE}
    restart: unless-stopped
    profiles:
      - observability
    command:
      - --config=/etc/otelcol/config.yaml
    volumes:
      - ./otel-collector-config.yaml:/etc/otelcol/config.yaml:ro
    networks:
      - zitadel

networks:
  zitadel:
    name: zitadel

volumes:
  postgres-data:
  zitadel-bootstrap:
```

#### `.env.example`

```dotenv
APP_NAME="HaoHao API"
APP_VERSION=0.1.0
HTTP_PORT=8080

APP_BASE_URL=http://127.0.0.1:8080
FRONTEND_BASE_URL=http://127.0.0.1:5173

DATABASE_URL=postgres://haohao:haohao@127.0.0.1:5432/haohao?sslmode=disable

AUTH_MODE=local
ZITADEL_ISSUER=
ZITADEL_CLIENT_ID=
ZITADEL_CLIENT_SECRET=
ZITADEL_REDIRECT_URI=http://127.0.0.1:8080/api/v1/auth/callback
ZITADEL_POST_LOGOUT_REDIRECT_URI=http://127.0.0.1:5173/login
ZITADEL_SCOPES="openid profile email"

REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=
REDIS_DB=0

SESSION_TTL=24h
LOGIN_STATE_TTL=10m
COOKIE_SECURE=false
```

#### `.gitignore`

```gitignore
.DS_Store
.env
.local/

frontend/node_modules
frontend/dist

backend/web/dist

*.log
cookies.txt
bin
```

### Database Exact Files

#### `db/migrations/0002_user_identities.up.sql`

```sql
ALTER TABLE users
    ALTER COLUMN password_hash DROP NOT NULL;

CREATE TABLE user_identities (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    subject TEXT NOT NULL,
    email TEXT NOT NULL,
    email_verified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, subject)
);

CREATE INDEX user_identities_user_id_idx ON user_identities(user_id);
```

#### `db/migrations/0002_user_identities.down.sql`

```sql
DROP TABLE IF EXISTS user_identities;

DELETE FROM users
WHERE password_hash IS NULL;

ALTER TABLE users
    ALTER COLUMN password_hash SET NOT NULL;
```

#### `db/queries/users.sql`

```sql
-- name: AuthenticateUser :one
SELECT id
FROM users
WHERE email = @email
  AND password_hash IS NOT NULL
  AND password_hash = crypt(@password, password_hash)
LIMIT 1;

-- name: GetUserByEmail :one
SELECT
    id,
    public_id,
    email,
    display_name
FROM users
WHERE email = $1
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

-- name: CreateOIDCUser :one
INSERT INTO users (
    email,
    display_name,
    password_hash
) VALUES (
    $1,
    $2,
    NULL
)
RETURNING
    id,
    public_id,
    email,
    display_name;

-- name: UpdateUserProfile :one
UPDATE users
SET email = $2,
    display_name = $3,
    updated_at = now()
WHERE id = $1
RETURNING
    id,
    public_id,
    email,
    display_name;
```

#### `db/queries/identities.sql`

```sql
-- name: GetUserByProviderSubject :one
SELECT
    u.id,
    u.public_id,
    u.email,
    u.display_name
FROM user_identities ui
JOIN users u ON u.id = ui.user_id
WHERE ui.provider = $1
  AND ui.subject = $2
LIMIT 1;

-- name: CreateUserIdentity :exec
INSERT INTO user_identities (
    user_id,
    provider,
    subject,
    email,
    email_verified
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
);

-- name: UpdateUserIdentityProfile :exec
UPDATE user_identities
SET email = $3,
    email_verified = $4,
    updated_at = now()
WHERE provider = $1
  AND subject = $2;
```

### Backend Exact Files

#### `backend/go.mod`

```go
module example.com/haohao/backend

go 1.25.0

require (
	github.com/coreos/go-oidc/v3 v3.18.0
	github.com/danielgtaylor/huma/v2 v2.37.3
	github.com/gin-gonic/gin v1.12.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.9.2
	github.com/redis/go-redis/v9 v9.18.0
	golang.org/x/oauth2 v0.36.0
)

require (
	github.com/bytedance/gopkg v0.1.3 // indirect
	github.com/bytedance/sonic v1.15.0 // indirect
	github.com/bytedance/sonic/loader v0.5.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/gin-contrib/sse v1.1.0 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.30.1 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.59.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.1 // indirect
	go.mongodb.org/mongo-driver/v2 v2.5.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/arch v0.24.0 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
```

#### `backend/internal/config/config.go`

```go
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName                      string
	AppVersion                   string
	HTTPPort                     int
	AppBaseURL                   string
	FrontendBaseURL              string
	DatabaseURL                  string
	AuthMode                     string
	ZitadelIssuer                string
	ZitadelClientID              string
	ZitadelClientSecret          string
	ZitadelRedirectURI           string
	ZitadelPostLogoutRedirectURI string
	ZitadelScopes                string
	RedisAddr                    string
	RedisPassword                string
	RedisDB                      int
	LoginStateTTL                time.Duration
	SessionTTL                   time.Duration
	CookieSecure                 bool
}

func Load() (Config, error) {
	sessionTTL, err := time.ParseDuration(getEnv("SESSION_TTL", "24h"))
	if err != nil {
		return Config{}, err
	}
	loginStateTTL, err := time.ParseDuration(getEnv("LOGIN_STATE_TTL", "10m"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppName:                      getEnv("APP_NAME", "HaoHao API"),
		AppVersion:                   getEnv("APP_VERSION", "0.1.0"),
		HTTPPort:                     getEnvInt("HTTP_PORT", 8080),
		AppBaseURL:                   strings.TrimRight(getEnv("APP_BASE_URL", "http://127.0.0.1:8080"), "/"),
		FrontendBaseURL:              strings.TrimRight(getEnv("FRONTEND_BASE_URL", "http://127.0.0.1:5173"), "/"),
		DatabaseURL:                  getEnv("DATABASE_URL", ""),
		AuthMode:                     getEnv("AUTH_MODE", "local"),
		ZitadelIssuer:                strings.TrimRight(getEnv("ZITADEL_ISSUER", ""), "/"),
		ZitadelClientID:              getEnv("ZITADEL_CLIENT_ID", ""),
		ZitadelClientSecret:          getEnv("ZITADEL_CLIENT_SECRET", ""),
		ZitadelRedirectURI:           getEnv("ZITADEL_REDIRECT_URI", "http://127.0.0.1:8080/api/v1/auth/callback"),
		ZitadelPostLogoutRedirectURI: getEnv("ZITADEL_POST_LOGOUT_REDIRECT_URI", "http://127.0.0.1:5173/login"),
		ZitadelScopes:                getEnv("ZITADEL_SCOPES", "openid profile email"),
		RedisAddr:                    getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:                getEnv("REDIS_PASSWORD", ""),
		RedisDB:                      getEnvInt("REDIS_DB", 0),
		LoginStateTTL:                loginStateTTL,
		SessionTTL:                   sessionTTL,
		CookieSecure:                 getEnvBool("COOKIE_SECURE", false),
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

#### `backend/internal/auth/session_store.go`

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
	UserID              int64  `json:"userId"`
	CSRFToken           string `json:"csrfToken"`
	ProviderIDTokenHint string `json:"providerIdTokenHint,omitempty"`
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
	return s.CreateWithProviderHint(ctx, userID, "")
}

func (s *SessionStore) CreateWithProviderHint(ctx context.Context, userID int64, providerIDTokenHint string) (string, string, error) {
	sessionID, err := randomToken(32)
	if err != nil {
		return "", "", err
	}

	csrfToken, err := randomToken(32)
	if err != nil {
		return "", "", err
	}

	record := SessionRecord{
		UserID:              userID,
		CSRFToken:           csrfToken,
		ProviderIDTokenHint: providerIDTokenHint,
	}
	if err := s.save(ctx, sessionID, record, s.ttl); err != nil {
		return "", "", err
	}

	return sessionID, csrfToken, nil
}

func (s *SessionStore) Get(ctx context.Context, sessionID string) (SessionRecord, error) {
	record, _, err := s.loadWithTTL(ctx, sessionID)
	return record, err
}

func (s *SessionStore) Delete(ctx context.Context, sessionID string) error {
	if err := s.client.Del(ctx, s.key(sessionID)).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *SessionStore) ReissueCSRF(ctx context.Context, sessionID string) (string, error) {
	record, ttl, err := s.loadWithTTL(ctx, sessionID)
	if err != nil {
		return "", err
	}

	csrfToken, err := randomToken(32)
	if err != nil {
		return "", err
	}

	record.CSRFToken = csrfToken
	if err := s.save(ctx, sessionID, record, ttl); err != nil {
		return "", err
	}

	return csrfToken, nil
}

func (s *SessionStore) Rotate(ctx context.Context, sessionID string) (string, string, error) {
	record, _, err := s.loadWithTTL(ctx, sessionID)
	if err != nil {
		return "", "", err
	}

	newSessionID, err := randomToken(32)
	if err != nil {
		return "", "", err
	}

	newCSRFToken, err := randomToken(32)
	if err != nil {
		return "", "", err
	}

	record.CSRFToken = newCSRFToken
	if err := s.save(ctx, newSessionID, record, s.ttl); err != nil {
		return "", "", err
	}
	if err := s.Delete(ctx, sessionID); err != nil {
		return "", "", err
	}

	return newSessionID, newCSRFToken, nil
}

func (s *SessionStore) key(sessionID string) string {
	return s.prefix + sessionID
}

func (s *SessionStore) loadWithTTL(ctx context.Context, sessionID string) (SessionRecord, time.Duration, error) {
	raw, err := s.client.Get(ctx, s.key(sessionID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return SessionRecord{}, 0, ErrSessionNotFound
	}
	if err != nil {
		return SessionRecord{}, 0, fmt.Errorf("get session: %w", err)
	}

	var record SessionRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return SessionRecord{}, 0, fmt.Errorf("decode session: %w", err)
	}

	ttl, err := s.client.TTL(ctx, s.key(sessionID)).Result()
	if err != nil {
		return SessionRecord{}, 0, fmt.Errorf("get session ttl: %w", err)
	}
	if ttl <= 0 {
		ttl = s.ttl
	}

	return record, ttl, nil
}

func (s *SessionStore) save(ctx context.Context, sessionID string, record SessionRecord, ttl time.Duration) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}

	if err := s.client.Set(ctx, s.key(sessionID), payload, ttl).Err(); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	return nil
}

func randomToken(numBytes int) (string, error) {
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
```

#### `backend/internal/auth/login_state_store.go`

```go
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrLoginStateNotFound = errors.New("login state not found")

type LoginStateRecord struct {
	CodeVerifier string `json:"codeVerifier"`
	Nonce        string `json:"nonce"`
	ReturnTo     string `json:"returnTo"`
}

type LoginStateStore struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

func NewLoginStateStore(client *redis.Client, ttl time.Duration) *LoginStateStore {
	return &LoginStateStore{
		client: client,
		prefix: "oidc-state:",
		ttl:    ttl,
	}
}

func (s *LoginStateStore) Create(ctx context.Context, returnTo string) (string, LoginStateRecord, error) {
	state, err := randomToken(32)
	if err != nil {
		return "", LoginStateRecord{}, err
	}

	codeVerifier, err := randomToken(32)
	if err != nil {
		return "", LoginStateRecord{}, err
	}

	nonce, err := randomToken(32)
	if err != nil {
		return "", LoginStateRecord{}, err
	}

	record := LoginStateRecord{
		CodeVerifier: codeVerifier,
		Nonce:        nonce,
		ReturnTo:     returnTo,
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return "", LoginStateRecord{}, err
	}

	if err := s.client.Set(ctx, s.prefix+state, payload, s.ttl).Err(); err != nil {
		return "", LoginStateRecord{}, fmt.Errorf("save login state: %w", err)
	}

	return state, record, nil
}

func (s *LoginStateStore) Consume(ctx context.Context, state string) (LoginStateRecord, error) {
	raw, err := s.client.GetDel(ctx, s.prefix+state).Bytes()
	if errors.Is(err, redis.Nil) {
		return LoginStateRecord{}, ErrLoginStateNotFound
	}
	if err != nil {
		return LoginStateRecord{}, fmt.Errorf("consume login state: %w", err)
	}

	var record LoginStateRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return LoginStateRecord{}, fmt.Errorf("decode login state: %w", err)
	}

	return record, nil
}
```

#### `backend/internal/auth/oidc_client.go`

```go
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type IdentityClaims struct {
	Subject       string   `json:"sub"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	Name          string   `json:"name"`
	Groups        []string `json:"groups,omitempty"`
}

type OIDCIdentity struct {
	Claims     IdentityClaims
	RawIDToken string
}

type OIDCClient struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	config   *oauth2.Config
}

func NewOIDCClient(ctx context.Context, issuer, clientID, clientSecret, redirectURI, scopes string) (*OIDCClient, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover oidc provider: %w", err)
	}

	oauthScopes := strings.Fields(scopes)
	if len(oauthScopes) == 0 {
		oauthScopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}

	return &OIDCClient{
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  redirectURI,
			Scopes:       oauthScopes,
		},
	}, nil
}

func (c *OIDCClient) AuthorizeURL(state, nonce, codeVerifier string) string {
	return c.config.AuthCodeURL(
		state,
		oidc.Nonce(nonce),
		oauth2.SetAuthURLParam("code_challenge", pkceS256(codeVerifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

func (c *OIDCClient) ExchangeCode(ctx context.Context, code, codeVerifier, expectedNonce string) (OIDCIdentity, error) {
	token, err := c.config.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return OIDCIdentity{}, fmt.Errorf("exchange authorization code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return OIDCIdentity{}, fmt.Errorf("id_token missing from token response")
	}

	idToken, err := c.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return OIDCIdentity{}, fmt.Errorf("verify id token: %w", err)
	}

	var verified struct {
		Subject string `json:"sub"`
		Nonce   string `json:"nonce"`
	}
	if err := idToken.Claims(&verified); err != nil {
		return OIDCIdentity{}, fmt.Errorf("decode id token claims: %w", err)
	}
	if verified.Nonce != expectedNonce {
		return OIDCIdentity{}, fmt.Errorf("oidc nonce mismatch")
	}

	userInfo, err := c.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return OIDCIdentity{}, fmt.Errorf("fetch userinfo: %w", err)
	}

	var claims IdentityClaims
	if err := userInfo.Claims(&claims); err != nil {
		return OIDCIdentity{}, fmt.Errorf("decode userinfo claims: %w", err)
	}

	claims.Subject = verified.Subject
	return OIDCIdentity{
		Claims:     claims,
		RawIDToken: rawIDToken,
	}, nil
}

func pkceS256(codeVerifier string) string {
	hash := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}
```

#### `backend/internal/service/session_service.go`

```go
package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"

	"example.com/haohao/backend/internal/auth"
	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInvalidCSRFToken    = errors.New("invalid csrf token")
	ErrAuthModeUnsupported = errors.New("auth mode unsupported")
)

type User struct {
	ID          int64
	PublicID    string
	Email       string
	DisplayName string
}

type SessionService struct {
	queries  *db.Queries
	store    *auth.SessionStore
	authMode string
}

func NewSessionService(queries *db.Queries, store *auth.SessionStore, authMode string) *SessionService {
	return &SessionService{
		queries:  queries,
		store:    store,
		authMode: strings.ToLower(strings.TrimSpace(authMode)),
	}
}

func (s *SessionService) Login(ctx context.Context, email, password string) (User, string, string, error) {
	if s.authMode == "zitadel" {
		return User{}, "", "", ErrAuthModeUnsupported
	}

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

	sessionID, csrfToken, err := s.IssueSession(ctx, userID)
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

func (s *SessionService) IssueSession(ctx context.Context, userID int64) (string, string, error) {
	return s.IssueSessionWithProviderHint(ctx, userID, "")
}

func (s *SessionService) IssueSessionWithProviderHint(ctx context.Context, userID int64, providerIDTokenHint string) (string, string, error) {
	sessionID, csrfToken, err := s.store.CreateWithProviderHint(ctx, userID, providerIDTokenHint)
	if err != nil {
		return "", "", fmt.Errorf("create session: %w", err)
	}
	return sessionID, csrfToken, nil
}

func (s *SessionService) Logout(ctx context.Context, sessionID, csrfHeader string) (string, error) {
	session, err := s.store.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", ErrUnauthorized
	}
	if err != nil {
		return "", err
	}

	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(csrfHeader)) != 1 {
		return "", ErrInvalidCSRFToken
	}

	if err := s.store.Delete(ctx, sessionID); err != nil {
		return "", err
	}

	return session.ProviderIDTokenHint, nil
}

func (s *SessionService) ReissueCSRF(ctx context.Context, sessionID string) (string, error) {
	if _, err := s.CurrentUser(ctx, sessionID); err != nil {
		return "", err
	}

	csrfToken, err := s.store.ReissueCSRF(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", ErrUnauthorized
	}
	if err != nil {
		return "", err
	}

	return csrfToken, nil
}

func (s *SessionService) RefreshSession(ctx context.Context, sessionID, csrfHeader string) (string, string, error) {
	session, err := s.store.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", "", ErrUnauthorized
	}
	if err != nil {
		return "", "", err
	}

	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(csrfHeader)) != 1 {
		return "", "", ErrInvalidCSRFToken
	}

	newSessionID, newCSRFToken, err := s.store.Rotate(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", "", ErrUnauthorized
	}
	if err != nil {
		return "", "", err
	}

	return newSessionID, newCSRFToken, nil
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
```

#### `backend/internal/service/identity_service.go`

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidExternalIdentity = errors.New("invalid external identity")

type ExternalIdentity struct {
	Provider      string
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
}

type IdentityService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewIdentityService(pool *pgxpool.Pool, queries *db.Queries) *IdentityService {
	return &IdentityService{
		pool:    pool,
		queries: queries,
	}
}

func (s *IdentityService) ResolveOrCreateUser(ctx context.Context, identity ExternalIdentity) (User, error) {
	normalized, err := normalizeExternalIdentity(identity)
	if err != nil {
		return User{}, err
	}

	existing, err := s.queries.GetUserByProviderSubject(ctx, db.GetUserByProviderSubjectParams{
		Provider: normalized.Provider,
		Subject:  normalized.Subject,
	})
	if err == nil {
		_ = s.queries.UpdateUserIdentityProfile(ctx, db.UpdateUserIdentityProfileParams{
			Provider:      normalized.Provider,
			Subject:       normalized.Subject,
			Email:         normalized.Email,
			EmailVerified: normalized.EmailVerified,
		})

		return s.syncUserProfile(ctx, s.queries, dbUser(existing.ID, existing.PublicID.String(), existing.Email, existing.DisplayName), normalized)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return User{}, fmt.Errorf("lookup identity by provider subject: %w", err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, fmt.Errorf("begin identity transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	user, err := s.resolveUserForIdentity(ctx, qtx, normalized)
	if err != nil {
		return User{}, err
	}

	if err := qtx.CreateUserIdentity(ctx, db.CreateUserIdentityParams{
		UserID:        user.ID,
		Provider:      normalized.Provider,
		Subject:       normalized.Subject,
		Email:         normalized.Email,
		EmailVerified: normalized.EmailVerified,
	}); err != nil {
		return User{}, fmt.Errorf("create user identity: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit identity transaction: %w", err)
	}

	return user, nil
}

func (s *IdentityService) resolveUserForIdentity(ctx context.Context, queries *db.Queries, identity ExternalIdentity) (User, error) {
	if identity.EmailVerified {
		existing, err := queries.GetUserByEmail(ctx, identity.Email)
		if err == nil {
			return s.syncUserProfile(ctx, queries, dbUser(existing.ID, existing.PublicID.String(), existing.Email, existing.DisplayName), identity)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return User{}, fmt.Errorf("lookup user by email: %w", err)
		}
	}

	created, err := queries.CreateOIDCUser(ctx, db.CreateOIDCUserParams{
		Email:       identity.Email,
		DisplayName: identity.DisplayName,
	})
	if err != nil {
		return User{}, fmt.Errorf("create oidc user: %w", err)
	}

	return dbUser(created.ID, created.PublicID.String(), created.Email, created.DisplayName), nil
}

func (s *IdentityService) syncUserProfile(ctx context.Context, queries *db.Queries, user User, identity ExternalIdentity) (User, error) {
	nextEmail := user.Email
	if identity.EmailVerified && identity.Email != "" {
		nextEmail = identity.Email
	}

	nextDisplayName := user.DisplayName
	if identity.DisplayName != "" {
		nextDisplayName = identity.DisplayName
	}

	if nextEmail == user.Email && nextDisplayName == user.DisplayName {
		return user, nil
	}

	updated, err := queries.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		ID:          user.ID,
		Email:       nextEmail,
		DisplayName: nextDisplayName,
	})
	if err != nil {
		return User{}, fmt.Errorf("update user profile: %w", err)
	}

	return dbUser(updated.ID, updated.PublicID.String(), updated.Email, updated.DisplayName), nil
}

func normalizeExternalIdentity(identity ExternalIdentity) (ExternalIdentity, error) {
	provider := strings.ToLower(strings.TrimSpace(identity.Provider))
	subject := strings.TrimSpace(identity.Subject)
	email := strings.ToLower(strings.TrimSpace(identity.Email))
	displayName := strings.TrimSpace(identity.DisplayName)

	if provider == "" || subject == "" || email == "" {
		return ExternalIdentity{}, ErrInvalidExternalIdentity
	}
	if displayName == "" {
		displayName = fallbackDisplayName(email, subject)
	}

	return ExternalIdentity{
		Provider:      provider,
		Subject:       subject,
		Email:         email,
		EmailVerified: identity.EmailVerified,
		DisplayName:   displayName,
	}, nil
}

func fallbackDisplayName(email, subject string) string {
	if email != "" {
		if head, _, ok := strings.Cut(email, "@"); ok && head != "" {
			return head
		}
		return email
	}
	return subject
}

func dbUser(id int64, publicID, email, displayName string) User {
	return User{
		ID:          id,
		PublicID:    publicID,
		Email:       email,
		DisplayName: displayName,
	}
}
```

#### `backend/internal/service/oidc_login_service.go`

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"example.com/haohao/backend/internal/auth"
)

type OIDCLoginResult struct {
	SessionID string
	CSRFToken string
	ReturnTo  string
}

type OIDCLoginService struct {
	providerName   string
	oidcClient     *auth.OIDCClient
	loginState     *auth.LoginStateStore
	identity       *IdentityService
	sessionService *SessionService
}

func NewOIDCLoginService(providerName string, oidcClient *auth.OIDCClient, loginState *auth.LoginStateStore, identity *IdentityService, sessionService *SessionService) *OIDCLoginService {
	return &OIDCLoginService{
		providerName:   providerName,
		oidcClient:     oidcClient,
		loginState:     loginState,
		identity:       identity,
		sessionService: sessionService,
	}
}

func (s *OIDCLoginService) StartLogin(ctx context.Context, returnTo string) (string, error) {
	if s == nil || s.oidcClient == nil || s.loginState == nil {
		return "", ErrAuthModeUnsupported
	}

	state, record, err := s.loginState.Create(ctx, sanitizeReturnTo(returnTo))
	if err != nil {
		return "", fmt.Errorf("create oidc login state: %w", err)
	}

	return s.oidcClient.AuthorizeURL(state, record.Nonce, record.CodeVerifier), nil
}

func (s *OIDCLoginService) FinishLogin(ctx context.Context, code, state string) (OIDCLoginResult, error) {
	if s == nil || s.oidcClient == nil || s.loginState == nil || s.identity == nil || s.sessionService == nil {
		return OIDCLoginResult{}, ErrAuthModeUnsupported
	}

	loginState, err := s.loginState.Consume(ctx, state)
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("consume oidc login state: %w", err)
	}

	identity, err := s.oidcClient.ExchangeCode(ctx, code, loginState.CodeVerifier, loginState.Nonce)
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("finish oidc code exchange: %w", err)
	}

	user, err := s.identity.ResolveOrCreateUser(ctx, ExternalIdentity{
		Provider:      s.providerName,
		Subject:       identity.Claims.Subject,
		Email:         identity.Claims.Email,
		EmailVerified: identity.Claims.EmailVerified,
		DisplayName:   identity.Claims.Name,
	})
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("resolve local user for oidc identity: %w", err)
	}

	sessionID, csrfToken, err := s.sessionService.IssueSessionWithProviderHint(ctx, user.ID, identity.RawIDToken)
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("issue local session for oidc login: %w", err)
	}

	return OIDCLoginResult{
		SessionID: sessionID,
		CSRFToken: csrfToken,
		ReturnTo:  sanitizeReturnTo(loginState.ReturnTo),
	}, nil
}

func sanitizeReturnTo(returnTo string) string {
	trimmed := strings.TrimSpace(returnTo)
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return "/"
	}
	return trimmed
}

func IsOIDCLoginFailure(err error) bool {
	return err != nil && (errors.Is(err, auth.ErrLoginStateNotFound) || errors.Is(err, ErrInvalidExternalIdentity))
}
```

#### `backend/internal/api/auth_settings.go`

```go
package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type AuthSettingsBody struct {
	Mode    string               `json:"mode" example:"local"`
	Zitadel *ZitadelSettingsBody `json:"zitadel,omitempty"`
}

type ZitadelSettingsBody struct {
	Issuer   string `json:"issuer" format:"uri" example:"http://localhost:8081"`
	ClientID string `json:"clientId" example:"312345678901234567"`
}

type GetAuthSettingsOutput struct {
	Body AuthSettingsBody
}

func registerAuthSettingsRoute(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "getAuthSettings",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/settings",
		Summary:     "現在の認証モード設定を返す",
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

		return &GetAuthSettingsOutput{Body: body}, nil
	})
}
```

#### `backend/internal/api/register.go`

```go
package api

import (
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type Dependencies struct {
	SessionService               *service.SessionService
	OIDCLoginService             *service.OIDCLoginService
	AuthMode                     string
	FrontendBaseURL              string
	ZitadelIssuer                string
	ZitadelClientID              string
	ZitadelPostLogoutRedirectURI string
	CookieSecure                 bool
	SessionTTL                   time.Duration
}

func Register(api huma.API, deps Dependencies) {
	registerAuthSettingsRoute(api, deps)
	registerOIDCRoutes(api, deps)
	registerSessionRoutes(api, deps)
}
```

#### `backend/internal/api/session.go`

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
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
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

type GetCSRFInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
}

type GetCSRFOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
}

type RefreshSessionInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
}

type RefreshSessionOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
}

type LogoutInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
}

type LogoutBody struct {
	PostLogoutURL string `json:"postLogoutURL,omitempty" format:"uri" example:"http://localhost:8081/oidc/v1/end_session?id_token_hint=..."`
}

type LogoutOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      LogoutBody
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
		OperationID:   "getCSRF",
		Method:        http.MethodGet,
		Path:          "/api/v1/csrf",
		Summary:       "CSRF token を再発行する",
		Tags:          []string{"session"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *GetCSRFInput) (*GetCSRFOutput, error) {
		csrfToken, err := deps.SessionService.ReissueCSRF(ctx, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPError(err)
		}

		return &GetCSRFOutput{
			SetCookie: []http.Cookie{
				auth.NewXSRFCookie(csrfToken, deps.CookieSecure, deps.SessionTTL),
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "refreshSession",
		Method:        http.MethodPost,
		Path:          "/api/v1/session/refresh",
		Summary:       "セッションを再発行する",
		Tags:          []string{"session"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *RefreshSessionInput) (*RefreshSessionOutput, error) {
		sessionID, csrfToken, err := deps.SessionService.RefreshSession(ctx, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, toHTTPError(err)
		}

		return &RefreshSessionOutput{
			SetCookie: []http.Cookie{
				auth.NewSessionCookie(sessionID, deps.CookieSecure, deps.SessionTTL),
				auth.NewXSRFCookie(csrfToken, deps.CookieSecure, deps.SessionTTL),
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "logout",
		Method:        http.MethodPost,
		Path:          "/api/v1/logout",
		Summary:       "セッションを破棄する",
		Tags:          []string{"session"},
		DefaultStatus: http.StatusOK,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *LogoutInput) (*LogoutOutput, error) {
		idTokenHint, err := deps.SessionService.Logout(ctx, input.SessionCookie.Value, input.CSRFToken)
		if err != nil && !errors.Is(err, service.ErrUnauthorized) {
			return nil, toHTTPError(err)
		}

		return &LogoutOutput{
			SetCookie: []http.Cookie{
				auth.ExpiredSessionCookie(deps.CookieSecure),
				auth.ExpiredXSRFCookie(deps.CookieSecure),
			},
			Body: LogoutBody{
				PostLogoutURL: buildPostLogoutURL(deps, idTokenHint),
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
	case errors.Is(err, service.ErrAuthModeUnsupported):
		return huma.Error501NotImplemented("password login is disabled for the current auth mode")
	default:
		return huma.Error500InternalServerError("internal server error")
	}
}
```

#### `backend/internal/api/oidc.go`

```go
package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"example.com/haohao/backend/internal/auth"

	"github.com/danielgtaylor/huma/v2"
)

type StartOIDCLoginInput struct {
	ReturnTo string `query:"returnTo"`
}

type StartOIDCLoginOutput struct {
	Location string `header:"Location"`
}

type OIDCCallbackInput struct {
	Code             string `query:"code"`
	State            string `query:"state"`
	Error            string `query:"error"`
	ErrorDescription string `query:"error_description"`
}

type OIDCCallbackOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Location  string        `header:"Location"`
}

func registerOIDCRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID:   "startOIDCLogin",
		Method:        http.MethodGet,
		Path:          "/api/v1/auth/login",
		Summary:       "OIDC login を開始する",
		Tags:          []string{"auth"},
		DefaultStatus: http.StatusFound,
	}, func(ctx context.Context, input *StartOIDCLoginInput) (*StartOIDCLoginOutput, error) {
		if deps.OIDCLoginService == nil {
			return nil, huma.Error501NotImplemented("oidc login is not configured")
		}

		location, err := deps.OIDCLoginService.StartLogin(ctx, input.ReturnTo)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to start oidc login")
		}

		return &StartOIDCLoginOutput{Location: location}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "finishOIDCLogin",
		Method:        http.MethodGet,
		Path:          "/api/v1/auth/callback",
		Summary:       "OIDC callback を完了する",
		Tags:          []string{"auth"},
		DefaultStatus: http.StatusFound,
	}, func(ctx context.Context, input *OIDCCallbackInput) (*OIDCCallbackOutput, error) {
		if input.Error != "" || deps.OIDCLoginService == nil {
			return &OIDCCallbackOutput{
				Location: oidcFailureRedirect(deps.FrontendBaseURL),
			}, nil
		}

		result, err := deps.OIDCLoginService.FinishLogin(ctx, input.Code, input.State)
		if err != nil {
			return &OIDCCallbackOutput{
				Location: oidcFailureRedirect(deps.FrontendBaseURL),
			}, nil
		}

		return &OIDCCallbackOutput{
			SetCookie: []http.Cookie{
				auth.NewSessionCookie(result.SessionID, deps.CookieSecure, deps.SessionTTL),
				auth.NewXSRFCookie(result.CSRFToken, deps.CookieSecure, deps.SessionTTL),
			},
			Location: oidcSuccessRedirect(deps.FrontendBaseURL, result.ReturnTo),
		}, nil
	})
}

func oidcFailureRedirect(frontendBaseURL string) string {
	return strings.TrimRight(frontendBaseURL, "/") + "/login?error=oidc_callback_failed"
}

func oidcSuccessRedirect(frontendBaseURL, returnTo string) string {
	base := strings.TrimRight(frontendBaseURL, "/")
	if returnTo == "" || !strings.HasPrefix(returnTo, "/") || strings.HasPrefix(returnTo, "//") {
		return base + "/"
	}
	return base + returnTo
}

func buildPostLogoutURL(deps Dependencies, idTokenHint string) string {
	if deps.AuthMode != "zitadel" || deps.ZitadelIssuer == "" || deps.ZitadelClientID == "" || deps.ZitadelPostLogoutRedirectURI == "" {
		return ""
	}

	endSessionURL, err := url.Parse(strings.TrimRight(deps.ZitadelIssuer, "/") + "/oidc/v1/end_session")
	if err != nil {
		return ""
	}

	query := endSessionURL.Query()
	if idTokenHint != "" {
		query.Set("id_token_hint", idTokenHint)
	} else {
		query.Set("client_id", deps.ZitadelClientID)
	}
	query.Set("post_logout_redirect_uri", deps.ZitadelPostLogoutRedirectURI)
	endSessionURL.RawQuery = query.Encode()

	return endSessionURL.String()
}
```

#### `backend/internal/app/app.go`

```go
package app

import (
	backendapi "example.com/haohao/backend/internal/api"
	"example.com/haohao/backend/internal/auth"
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

func New(cfg config.Config, sessionService *service.SessionService, oidcLoginService *service.OIDCLoginService) *App {
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
		SessionService:               sessionService,
		OIDCLoginService:             oidcLoginService,
		AuthMode:                     cfg.AuthMode,
		FrontendBaseURL:              cfg.FrontendBaseURL,
		ZitadelIssuer:                cfg.ZitadelIssuer,
		ZitadelClientID:              cfg.ZitadelClientID,
		ZitadelPostLogoutRedirectURI: cfg.ZitadelPostLogoutRedirectURI,
		CookieSecure:                 cfg.CookieSecure,
		SessionTTL:                   cfg.SessionTTL,
	})

	return &App{
		Router: router,
		API:    api,
	}
}
```

#### `backend/cmd/main/main.go`

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
	sessionService := service.NewSessionService(queries, sessionStore, cfg.AuthMode)

	var oidcLoginService *service.OIDCLoginService
	if cfg.AuthMode == "zitadel" {
		if cfg.ZitadelIssuer == "" || cfg.ZitadelClientID == "" || cfg.ZitadelClientSecret == "" {
			log.Fatal("ZITADEL_ISSUER, ZITADEL_CLIENT_ID, and ZITADEL_CLIENT_SECRET are required when AUTH_MODE=zitadel")
		}

		oidcClient, err := auth.NewOIDCClient(
			ctx,
			cfg.ZitadelIssuer,
			cfg.ZitadelClientID,
			cfg.ZitadelClientSecret,
			cfg.ZitadelRedirectURI,
			cfg.ZitadelScopes,
		)
		if err != nil {
			log.Fatal(err)
		}

		loginStateStore := auth.NewLoginStateStore(redisClient, cfg.LoginStateTTL)
		identityService := service.NewIdentityService(pool, queries)
		oidcLoginService = service.NewOIDCLoginService("zitadel", oidcClient, loginStateStore, identityService, sessionService)
	}

	application := app.New(cfg, sessionService, oidcLoginService)

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

#### `backend/cmd/openapi/main.go`

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

	application := app.New(cfg, nil, nil)

	spec, err := application.API.OpenAPI().YAML()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(string(spec))
}
```

### Frontend Exact Files

#### `frontend/src/api/client.ts`

```ts
import type { ErrorModel } from './generated/types.gen'
import { client } from './generated/client.gen'

type ProblemLike = Partial<Pick<ErrorModel, 'detail' | 'title'>> & {
  message?: string
}

export function readCookie(name: string): string | undefined {
  const prefix = `${name}=`
  return document.cookie
    .split(';')
    .map((part) => part.trim())
    .find((part) => part.startsWith(prefix))
    ?.slice(prefix.length)
}

export function toApiErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message) {
    return error.message
  }

  if (error && typeof error === 'object') {
    const problem = error as ProblemLike
    if (problem.detail) {
      return problem.detail
    }
    if (problem.title) {
      return problem.title
    }
    if (problem.message) {
      return problem.message
    }
  }

  return '認証処理に失敗しました'
}

let csrfBootstrapPromise: Promise<void> | null = null

async function ensureCSRFCookie() {
  if (typeof document === 'undefined' || readCookie('XSRF-TOKEN')) {
    return
  }

  if (!csrfBootstrapPromise) {
    csrfBootstrapPromise = (async () => {
      try {
        await fetch('/api/v1/csrf', {
          method: 'GET',
          credentials: 'include',
          headers: {
            Accept: 'application/json',
          },
        })
      } catch {
        // The original mutating request will surface the real failure.
      }
    })().finally(() => {
      csrfBootstrapPromise = null
    })
  }

  await csrfBootstrapPromise
}

client.setConfig({
  baseUrl: '',
  credentials: 'include',
  responseStyle: 'data',
  throwOnError: true,
  fetch: async (input, init) => {
    const request = input instanceof Request ? input : undefined
    const headers = new Headers(request?.headers ?? init?.headers ?? {})
    headers.set('Accept', 'application/json')

    const method = (init?.method ?? request?.method ?? 'GET').toUpperCase()
    const csrfHeader = headers.get('X-CSRF-Token')
    if (!['GET', 'HEAD', 'OPTIONS'].includes(method) && !csrfHeader) {
      await ensureCSRFCookie()
      const token = readCookie('XSRF-TOKEN')
      if (token) {
        headers.set('X-CSRF-Token', token)
      }
    }

    return fetch(input, {
      ...init,
      credentials: 'include',
      headers,
    })
  },
})
```

#### `frontend/src/api/auth.ts`

```ts
import './client'

import type { AuthSettingsBody } from './generated/types.gen'
import { getAuthSettings } from './generated/sdk.gen'

export async function fetchAuthSettings(): Promise<AuthSettingsBody> {
  return getAuthSettings({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<AuthSettingsBody>
}
```

#### `frontend/src/api/session.ts`

```ts
import type { LogoutBody, SessionBody } from './generated/types.gen'
import { readCookie } from './client'
import { getSession, login, logout, refreshSession } from './generated/sdk.gen'

export async function fetchCurrentSession(): Promise<SessionBody> {
  return getSession({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<SessionBody>
}

export async function loginWithPassword(email: string, password: string): Promise<SessionBody> {
  return login({
    body: {
      email,
      password,
    },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<SessionBody>
}

export async function logoutCurrentSession(): Promise<LogoutBody> {
  return logout({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<LogoutBody>
}

export async function refreshCurrentSession(): Promise<void> {
  await refreshSession({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    responseStyle: 'data',
    throwOnError: true,
  })
}
```

#### `frontend/src/stores/session.ts`

```ts
import { defineStore } from 'pinia'

import { toApiErrorMessage } from '../api/client'
import type { UserResponse } from '../api/generated/types.gen'
import {
  fetchCurrentSession,
  loginWithPassword,
  logoutCurrentSession,
} from '../api/session'

type AuthStatus = 'idle' | 'loading' | 'authenticated' | 'anonymous'

export const useSessionStore = defineStore('session', {
  state: () => ({
    status: 'idle' as AuthStatus,
    user: null as UserResponse | null,
    errorMessage: '',
  }),

  actions: {
    async bootstrap() {
      if (this.status !== 'idle') {
        return
      }

      this.status = 'loading'
      this.errorMessage = ''

      try {
        const data = await fetchCurrentSession()
        this.user = data.user
        this.status = 'authenticated'
      } catch {
        this.user = null
        this.status = 'anonymous'
      }
    },

    async login(email: string, password: string) {
      this.status = 'loading'
      this.errorMessage = ''

      try {
        const data = await loginWithPassword(email, password)
        this.user = data.user
        this.status = 'authenticated'
      } catch (error) {
        this.user = null
        this.status = 'anonymous'
        this.errorMessage = toApiErrorMessage(error)
        throw error
      }
    },

    async logout() {
      this.errorMessage = ''

      try {
        const data = await logoutCurrentSession()
        this.user = null
        this.status = 'anonymous'
        return data.postLogoutURL ?? ''
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      }
    },
  },
})
```

#### `frontend/src/views/LoginView.vue`

```vue
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { fetchAuthSettings } from '../api/auth'
import { useSessionStore } from '../stores/session'

type AuthMode = 'local' | 'zitadel'

const route = useRoute()
const router = useRouter()
const sessionStore = useSessionStore()

const authMode = ref<AuthMode>('local')
const zitadelIssuer = ref('')
const email = ref('demo@example.com')
const password = ref('changeme123')
const submitting = ref(false)
const loadingSettings = ref(true)

const callbackErrorMessage = computed(() => {
  if (route.query.error === 'oidc_callback_failed') {
    return 'Zitadel callback の処理に失敗しました。設定値と redirect URI を確認してください。'
  }
  return ''
})

onMounted(async () => {
  try {
    const settings = await fetchAuthSettings()
    authMode.value = settings.mode as AuthMode
    zitadelIssuer.value = settings.zitadel?.issuer ?? ''
  } catch {
    authMode.value = 'local'
    zitadelIssuer.value = ''
  } finally {
    loadingSettings.value = false
  }
})

async function submit() {
  submitting.value = true

  try {
    await sessionStore.login(email.value, password.value)
    await router.push({ name: 'home' })
  } catch {
    // The store exposes the error message for the current view.
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <section class="split-grid">
    <div class="panel stack">
      <div class="stack intro">
        <span class="status-pill">Cookie Session</span>
        <h2>Login</h2>
        <p>
          認証 mode を backend から読み、<code>local</code> なら password form、
          <code>zitadel</code> なら browser redirect login を表示します。
        </p>
      </div>

      <p v-if="loadingSettings">Loading auth settings...</p>

      <form v-else-if="authMode === 'local'" class="stack" @submit.prevent="submit">
        <label class="field">
          <span class="field-label">Email</span>
          <input
            v-model="email"
            class="field-input"
            type="email"
            required
            autocomplete="username"
          />
        </label>

        <label class="field">
          <span class="field-label">Password</span>
          <input
            v-model="password"
            class="field-input"
            type="password"
            required
            minlength="8"
            autocomplete="current-password"
          />
        </label>

        <button class="primary-button" :disabled="submitting" type="submit">
          {{ submitting ? 'Signing in...' : 'Sign in' }}
        </button>
      </form>

      <div v-else class="stack">
        <p>
          <code>AUTH_MODE=zitadel</code> が有効です。browser は backend の
          <code>/api/v1/auth/login</code> へ遷移し、callback 後に local Cookie session を受け取ります。
        </p>
        <p v-if="zitadelIssuer">
          Issuer: <code>{{ zitadelIssuer }}</code>
        </p>
        <a class="primary-button zitadel-button" href="/api/v1/auth/login">
          Sign in with Zitadel
        </a>
      </div>

      <p v-if="callbackErrorMessage" class="error-message">
        {{ callbackErrorMessage }}
      </p>
      <p v-if="sessionStore.errorMessage" class="error-message">
        {{ sessionStore.errorMessage }}
      </p>
    </div>

    <aside class="panel stack">
      <h2>Routes</h2>
      <p>backend は Huma から OpenAPI 3.1 を生成し、frontend は generated SDK を使います。</p>
      <div class="stack detail-list">
        <div>
          <strong>Settings</strong>
          <p><code>GET /api/v1/auth/settings</code></p>
        </div>
        <div>
          <strong>OIDC</strong>
          <p><code>GET /api/v1/auth/login</code></p>
        </div>
        <div>
          <strong>Callback</strong>
          <p><code>GET /api/v1/auth/callback</code></p>
        </div>
        <div>
          <strong>Session</strong>
          <p><code>GET /api/v1/session</code></p>
        </div>
      </div>
    </aside>
  </section>
</template>

<style scoped>
.intro {
  gap: 10px;
}

.detail-list {
  gap: 14px;
}

.detail-list p,
.detail-list strong {
  margin: 0;
}

.zitadel-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  text-decoration: none;
}
</style>
```

#### `frontend/src/views/HomeView.vue`

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import { toApiErrorMessage } from '../api/client'
import { refreshCurrentSession } from '../api/session'
import { useSessionStore } from '../stores/session'

const router = useRouter()
const sessionStore = useSessionStore()

const userJson = computed(() => JSON.stringify(sessionStore.user, null, 2))
const refreshing = ref(false)
const refreshMessage = ref('')
const refreshErrorMessage = ref('')

async function signOut() {
  try {
    const postLogoutURL = await sessionStore.logout()
    if (postLogoutURL) {
      window.location.assign(postLogoutURL)
      return
    }
    await router.push({ name: 'login' })
  } catch {
    // The store exposes the error message for the current view.
  }
}

async function rotateSession() {
  refreshing.value = true
  refreshMessage.value = ''
  refreshErrorMessage.value = ''

  try {
    await refreshCurrentSession()
    refreshMessage.value = 'Session ID と CSRF token を再発行しました。'
  } catch (error) {
    refreshErrorMessage.value = toApiErrorMessage(error)
  } finally {
    refreshing.value = false
  }
}
</script>

<template>
  <section class="stack">
    <div class="split-grid">
      <section class="panel stack">
        <span class="status-pill">Authenticated</span>
        <h2>Current Session</h2>
        <p>Cookie セッションが復元できていれば、現在ユーザーがここに表示されます。</p>
        <pre class="json-card">{{ userJson }}</pre>

        <div class="action-row">
          <button class="secondary-button" :disabled="refreshing" type="button" @click="rotateSession">
            {{ refreshing ? 'Refreshing...' : 'Refresh Session' }}
          </button>
          <button class="secondary-button" type="button" @click="signOut">
            Logout
          </button>
          <a class="secondary-button docs-link" href="/docs" target="_blank" rel="noreferrer">
            Open Docs
          </a>
        </div>

        <p v-if="refreshMessage">{{ refreshMessage }}</p>
        <p v-if="refreshErrorMessage" class="error-message">
          {{ refreshErrorMessage }}
        </p>
        <p v-if="sessionStore.errorMessage" class="error-message">
          {{ sessionStore.errorMessage }}
        </p>
      </section>

      <aside class="panel stack">
        <h2>Verification</h2>
        <p>この画面が出ていれば、frontend は generated SDK 経由で session を読めています。</p>
        <ul class="check-list">
          <li>Cookie が browser に保存される</li>
          <li><code>/api/v1/session</code> が 200 を返す</li>
          <li><code>POST /api/v1/session/refresh</code> が Cookie を rotate する</li>
          <li><code>/docs</code> で OpenAPI 由来の docs を確認できる</li>
        </ul>
      </aside>
    </div>
  </section>
</template>

<style scoped>
.docs-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  text-decoration: none;
}

.check-list {
  margin: 0;
  padding-left: 1.2rem;
}
</style>
```

#### `frontend/vite.config.ts`

```ts
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
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
})
```

## Phase 3. External User Bearer API と共通認可土台

### 目的

browser API と混ざらない external bearer API surface を追加し、同時に local role / auth context の土台を入れます。

### この Phase の前提

- Phase 1 と Phase 2 の browser auth foundation が安定している
- Zitadel 側で external API 用 application の JWT access token が有効化されている

### この Phase の完了条件

- global role を local DB に持てる
- `bearerAuth` を使う external API surface がある
- generic JWT bearer verifier が external API で使われている
- browser API と external API が path / security scheme / CORS / CSRF の全てで分離されている

### Step 3.1. local 認可コンテキストを global role から入れる

ここでは tenant-aware な複雑さをまだ入れません。まずは global role を local DB に持てるようにします。

#### 追加するテーブル

この段階では最小でも次を持っておくと扱いやすいです。

- `roles`
- `user_roles`

provider から来る group / role claim をそのまま業務権限に使い切るのではなく、**local role に写してから使う**ほうが安全です。

#### 追加するファイル

- `db/migrations/0003_roles.up.sql`
- `db/migrations/0003_roles.down.sql`
- `db/queries/roles.sql`
- `backend/internal/service/authz_service.go`

#### 方針

- callback 時に provider claims を読み、必要な group だけ local role へ同期する
- service は `AuthContext` を受けて最終判断する
- operation は「session があるか」「bearer token が有効か」までにとどめる

#### 最初に用意する role

最初から複雑にせず、例えば次だけで十分です。

- `docs_reader`
- `external_api_user`
- `todo_user`

#### tenant との関係

ここで作る `roles` と `user_roles` は **global role** として扱ってください。Phase 5 で `tenant_role_overrides` を追加したあとも、この global role はベースレイヤとして残し、tenant 単位の allow / deny はその上に重ねる構成にします。つまり、**Phase 5 のために `user_roles` へ最初から tenant_id を入れない**でください。

### Step 3.2. generic JWT bearer verifier を作る

`external API`, `SCIM`, `M2M` が別々に token 検証を実装するとぶれます。ここで共通部品を先に作ります。

#### 追加する設定

`.env.example` に少なくとも次を足します。

```dotenv
EXTERNAL_EXPECTED_AUDIENCE=haohao-external
EXTERNAL_REQUIRED_SCOPE_PREFIX=
EXTERNAL_REQUIRED_ROLE=external_api_user
EXTERNAL_ALLOWED_ORIGINS=
```

`EXTERNAL_ALLOWED_ORIGINS` は comma-separated string として扱い、external API の CORS allowlist にだけ使います。空のままなら browser からの cross-origin call は通しません。
`EXTERNAL_REQUIRED_SCOPE_PREFIX` は optional な scope gate です。Zitadel の human-user JWT access token では `scope` claim が出ないことがあるため、Phase 3 の strict gate は `EXTERNAL_REQUIRED_ROLE` と Zitadel project roles claim で行います。

#### 追加するファイル

- `backend/internal/auth/bearer_verifier.go`
- `backend/internal/middleware/external_auth.go`

#### verifier の責務

- Zitadel の JWKS を取得・キャッシュする
- JWT access token の署名を local verify する
- issuer, audience, expiry, optional scope を検証する
- `sub`, `azp` / `client_id`, `scope`, `groups`, Zitadel project roles claim など必要な claim を struct に落とす

#### OpenAPI / Huma の security scheme

browser 用と external bearer 用をここで分けます。

```go
humaConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
    "cookieAuth": {
        Type: "apiKey",
        In:   "cookie",
        Name: "SESSION_ID",
    },
    "bearerAuth": {
        Type:         "http",
        Scheme:       "bearer",
        BearerFormat: "JWT",
    },
}
```

### Step 3.2.5. Zitadel Console 側で external user bearer app を作る

external user bearer API 用の token は、browser login 用 app と別 application にしてください。

#### 公式参照

- Applications overview  
  https://zitadel.com/docs/guides/manage/console/applications-overview  
  用途: application type と token type の設定場所を確認する

#### ここで固定する設定

- browser login 用 app とは **別 application**
- human-user token を取得する用途の application にする
- token type は **JWT access token**
- expected audience は最初は `haohao-external` を方針値にする。ただし local quickstart では、実際の JWT の `aud` に project ID / client ID が入ることがあるため、動作確認時は decode した `aud` を `EXTERNAL_EXPECTED_AUDIENCE` に合わせる

#### `.env` との対応

- expected audience の運用値 → `EXTERNAL_EXPECTED_AUDIENCE`
- optional scope prefix の運用値 → `EXTERNAL_REQUIRED_SCOPE_PREFIX`
- required role の運用値 → `EXTERNAL_REQUIRED_ROLE`

#### Console で作る external app の具体例

Phase 3 の最初の疎通確認では、human user が browser で login して JWT access token を取得できれば十分です。Console では browser login 用 app とは別に、例えば次で作ります。

```text
Projects -> haohao -> Applications -> New
Name: haohao-external-dev
Type: User Agent
Auth method: PKCE / Code
Redirect URI: http://127.0.0.1:18080/callback
Development Mode: enabled
Access Token Type: JWT
Add user roles to the access token: enabled
```

`Access Token Type` が opaque / bearer のままだと、HaoHao backend は local JWT 検証できません。設定を保存したあと、必ず新しく token を取り直してください。

Application の Token Settings は次で固定します。

```text
Auth Token Type: JWT
Add user roles to the access token: ON
User roles inside ID Token: OFF
Include user's profile info in the ID Token: OFF
```

Project settings 側は次で固定します。

```text
Return user roles during authentication: ON
Only authorized users can authenticate: OFF
Authentication is restricted to users from organizations that have been granted access to this project: OFF
```

`Only authorized users can authenticate` は strict な login 制御なので、最初の検証では OFF のままで構いません。後で enterprise 向けに login 自体を project role で閉じたいときに検討してください。

#### JWT access token を手動取得する

external app の `Client ID` と、`haohao` project の `Project ID` を控えてから実行します。`CODE_VERIFIER` は token exchange まで同じ shell で保持してください。

```bash
CLIENT_ID='<haohao-external-dev client id>'
PROJECT_ID='<haohao project id>'
REDIRECT_URI='http://127.0.0.1:18080/callback'
SCOPE="openid profile email urn:zitadel:iam:org:project:id:${PROJECT_ID}:aud urn:zitadel:iam:org:projects:roles"

CODE_VERIFIER=$(openssl rand -base64 96 | tr '+/' '-_' | tr -d '=' | cut -c 1-128)
CODE_CHALLENGE=$(printf '%s' "$CODE_VERIFIER" | openssl dgst -sha256 -binary | openssl base64 -A | tr '+/' '-_' | tr -d '=')
STATE=$(openssl rand -hex 16)

ENC_REDIRECT=$(python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=""))' "$REDIRECT_URI")
ENC_SCOPE=$(python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=""))' "$SCOPE")

AUTH_URL="http://localhost:8081/oauth/v2/authorize?client_id=${CLIENT_ID}&redirect_uri=${ENC_REDIRECT}&response_type=code&scope=${ENC_SCOPE}&code_challenge=${CODE_CHALLENGE}&code_challenge_method=S256&state=${STATE}"

echo "$AUTH_URL"
open "$AUTH_URL"
```

browser login 後、`http://127.0.0.1:18080/callback?code=...` へ戻ります。callback server は不要です。接続エラー画面になっても、address bar の `code` をコピーしてください。

```bash
CODE='<address bar の code>'

curl -sS -X POST http://localhost:8081/oauth/v2/token \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode grant_type=authorization_code \
  --data-urlencode client_id="$CLIENT_ID" \
  --data-urlencode code="$CODE" \
  --data-urlencode redirect_uri="$REDIRECT_URI" \
  --data-urlencode code_verifier="$CODE_VERIFIER" \
  | tee /tmp/zitadel-token.json
```

成功すると `access_token` が返ります。使うのは `id_token` ではなく `access_token` です。JWT access token なら `.` が 2 つ入った 3 segment の文字列になります。

```bash
ACCESS_TOKEN=$(python3 -c 'import json; print(json.load(open("/tmp/zitadel-token.json"))["access_token"])')
echo "$ACCESS_TOKEN" | awk -F. '{print NF}'
```

`3` 以外なら external app の `Access Token Type` が JWT になっていないか、古い token を見ています。

#### JWT の `aud` と roles claim を `.env` に合わせる

まず payload を decode します。

```bash
python3 - "$ACCESS_TOKEN" <<'PY'
import base64, json, sys
token = sys.argv[1]
payload = token.split(".")[1]
payload += "=" * (-len(payload) % 4)
print(json.dumps(json.loads(base64.urlsafe_b64decode(payload)), indent=2, ensure_ascii=False))
PY
```

local quickstart では、`aud` が `haohao-external` ではなく project ID / client ID の配列になることがあります。その場合は、JWT の `aud` に実際に含まれている値を repo root の `.env` に設定してください。

```dotenv
EXTERNAL_EXPECTED_AUDIENCE=<JWT の aud に入っている値>
```

Zitadel project roles が有効なら、JWT に次のような claim が出ます。

```json
"urn:zitadel:iam:org:project:<PROJECT_ID>:roles": {
  "external_api_user": {
    "<ORG_ID>": "zitadel.localhost"
  }
}
```

HaoHao はこの role claim から role code を抽出し、`EXTERNAL_REQUIRED_ROLE` を満たすか確認します。

```dotenv
EXTERNAL_REQUIRED_SCOPE_PREFIX=
EXTERNAL_REQUIRED_ROLE=external_api_user
```

`EXTERNAL_REQUIRED_SCOPE_PREFIX` は空のままで構いません。将来 Zitadel 以外の provider や M2M で scope claim を使う場合だけ設定します。

`.env` を変えたら、起動中の backend は必ず再起動します。起動済みプロセスは古い環境変数を保持したままです。

```bash
make backend-dev
```

既に `:8080` を別の backend が使っている場合は、古い `make backend-dev` を停止してから起動し直してください。

### Step 3.3. external client 向け API を別 surface で追加する

この文書では次で固定します。

```text
browser API:  /api/v1/*
external API: /api/external/v1/*
```

#### 追加するファイル

- `backend/internal/api/external_*.go`

#### bearer token 検証でやること

- generic JWT bearer verifier を使う
- `EXTERNAL_EXPECTED_AUDIENCE` と optional `EXTERNAL_REQUIRED_SCOPE_PREFIX` を確認する
- Zitadel project roles claim から `EXTERNAL_REQUIRED_ROLE` を確認する
- token claims から external client 用 `AuthContext` を組み立てる
- local user が既に provision 済みなら `(provider, subject)` で引いて local role も載せる

#### browser API と分ける点

- Cookie は使わない
- CSRF は不要
- CORS は明示 allowlist にする
- rate limit を browser API と分離する

#### 最初の endpoint

最小構成なら、まず 1 本だけでも十分です。

```text
GET /api/external/v1/me
```

current repo の `/api/external/v1/me` は、最低でも `provider`, `subject`, `scopes`, `groups` を返し、local user が provision 済みなら `user` と `roles` も返します。これで token verify, auth context, OpenAPI security scheme, cross-origin 動作の全てを小さく一周できます。その後に TODO 一覧などの read API を足してください。外部向け一覧は `CONCEPT.md` どおり `cursor` pagination を優先します。

#### この Phase の手動確認

1. Zitadel から external API 用 JWT access token を取得し、payload を decode して `iss`, `sub`, `aud`, Zitadel roles claim を見る
2. `.env` の `EXTERNAL_EXPECTED_AUDIENCE` を JWT の `aud` に含まれる値へ合わせ、backend を再起動する
3. `.env` の `EXTERNAL_REQUIRED_ROLE=external_api_user` を確認し、`GET /api/external/v1/me` が `200` になることを確認する
4. `external_api_user` role が無い token は `403 invalid bearer role` になることを確認する
5. issuer / audience / optional scope mismatch を個別に拒否できることを確認する
6. browser API に bearer token を送っても Cookie 前提の route とは混ざらないことを確認する
7. browser から呼ぶなら、`Origin` を `EXTERNAL_ALLOWED_ORIGINS` に足したときだけ preflight が通ることを確認する

`external_api_user` role の有無を切り替えたら、**必ず新しい authorization code から access token を取り直してください**。既存の JWT は発行時点の role claim を持ち続けます。

最小 positive test は次です。

```bash
curl -i \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  http://127.0.0.1:8080/api/external/v1/me
```

期待値は `200 OK` で、`groups` と `roles` に `external_api_user` が含まれることです。

role なし negative test では、まず Zitadel Console で対象 user から `external_api_user` role を外し、新しい token を取得します。decode した JWT に `urn:zitadel:iam:org:project:*:roles` claim が無い、または `external_api_user` が無いことを確認してから同じ endpoint を叩きます。期待値は次です。

```text
HTTP/1.1 403 Forbidden
{"detail":"invalid bearer role","status":403,"title":"Forbidden"}
```

もし role claim が無い token で `200 OK` になるなら、local DB に残った `user_roles` だけで external bearer を許可してしまっています。external bearer の required role check は、local DB の cached role ではなく **その JWT に含まれる provider role claim** を見る必要があります。

よくある失敗は次です。

```text
401 invalid bearer audience
```

backend が読んでいる `EXTERNAL_EXPECTED_AUDIENCE` と JWT の `aud` が一致していません。`.env` を直しただけでは反映されないので、backend を再起動してください。

```text
403 invalid bearer scope
```

JWT の `scope` claim に `EXTERNAL_REQUIRED_SCOPE_PREFIX` で始まる scope が入っていません。Zitadel human-user bearer の Phase 3 では `EXTERNAL_REQUIRED_SCOPE_PREFIX=` のままにし、role gate を使います。

```text
403 invalid bearer role
```

JWT の Zitadel roles claim に `EXTERNAL_REQUIRED_ROLE` がありません。Console で user role assignment, project の `Return user roles during authentication`, application の `Add user roles to the access token` を確認し、新しい token を取り直してください。

```text
401 invalid bearer token
```

access token が JWT ではない、token が期限切れ、issuer / JWKS が一致していない、または `id_token` を送っています。`/tmp/zitadel-token.json` から `access_token` を使っていることを確認してください。

#### Phase 3 final negative checks

positive path と role なし negative test が通ったあと、最後に次の軽い境界確認を流します。backend は通常設定で起動しておきます。

Token なしは `401` になります。

```bash
curl -i http://127.0.0.1:8080/api/external/v1/me
```

期待値は次です。

```text
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Bearer realm="haohao-external"
{"detail":"missing bearer token","status":401,"title":"Unauthorized"}
```

browser session route に bearer token を送っても、Cookie session としては扱われません。

```bash
ACCESS_TOKEN=$(python3 -c 'import json; print(json.load(open("/tmp/zitadel-token.json"))["access_token"])')
curl -i -H "Authorization: Bearer $ACCESS_TOKEN" http://127.0.0.1:8080/api/v1/session
```

期待値は `401` で、detail は `missing or expired session` です。

`EXTERNAL_ALLOWED_ORIGINS=` のままなら、external API の preflight は拒否されます。

```bash
curl -i -X OPTIONS \
  -H "Origin: http://127.0.0.1:5173" \
  -H "Access-Control-Request-Method: GET" \
  -H "Access-Control-Request-Headers: Authorization" \
  http://127.0.0.1:8080/api/external/v1/me
```

期待値は `403 Forbidden` と `origin is not allowed` です。

audience mismatch は `.env` を編集せずに確認できます。ただし `EXTERNAL_EXPECTED_AUDIENCE=wrong-audience make backend-dev` は、Makefile が後から `.env` を source するため override になりません。一時 override は次の形で起動します。

```bash
bash -lc 'set -a; source .env; export EXTERNAL_EXPECTED_AUDIENCE=wrong-audience; set +a; go run ./backend/cmd/main'
```

別 terminal で叩きます。

```bash
ACCESS_TOKEN=$(python3 -c 'import json; print(json.load(open("/tmp/zitadel-token.json"))["access_token"])')
curl -i -H "Authorization: Bearer $ACCESS_TOKEN" http://127.0.0.1:8080/api/external/v1/me
```

期待値は `401 Unauthorized` と `invalid bearer audience` です。

CORS allow 側も確認するなら、同じく `.env` を編集せずに一時 override で起動します。

```bash
bash -lc 'set -a; source .env; export EXTERNAL_ALLOWED_ORIGINS=http://127.0.0.1:5173; set +a; go run ./backend/cmd/main'
```

同じ OPTIONS request の期待値は `204 No Content` と次の header です。

```text
Access-Control-Allow-Origin: http://127.0.0.1:5173
Access-Control-Allow-Headers: Authorization, Content-Type
Access-Control-Allow-Methods: GET, POST, OPTIONS
```

## Phase 3 Exact Snapshot

Phase 0-2 の exact snapshot はそのまま使い、Phase 3 で変わった file だけをここに追加します。

- この節の block は、現在の Phase 3 実装に合わせた **exact delta** です
- `backend/internal/db/*.go`, `openapi/openapi.yaml`, `frontend/src/api/generated/*` は `make gen` の生成物です
- `db/schema.sql` は `make db-schema` の生成物ですが、`sqlc generate` の入力でもあるため、この snapshot には Phase 3 時点の内容を載せます
- `go.work.sum` は tool 実行で再生成されることがありますが、この repo の正本には含めません

#### Clean worktree replay checklist

`../phase3-test` のような clean worktree で **この `TUTORIAL_ZITADEL.md` だけから同じ file / directory 構成へ戻す** 場合は、次の順に進めてください。ここでは手作業で snippet を copy せず、Markdown 内の exact file block を展開します。

この script は `Phase 0-2 Exact Snapshot` の `Project Exact Files` と、この `Phase 3 Exact Snapshot` の file block を読み取り、同じ path に書き出します。親 directory は自動作成されるため、`dev/zitadel` もこの手順で作られます。Phase 3 の block は Phase 0-2 の同名 file を上書きするため、最終状態が現在の Phase 3 実装になります。

```bash
python3 - <<'PY'
from pathlib import Path
import re

doc = Path("TUTORIAL_ZITADEL.md")
text = doc.read_text()

sections = [
    ("### Project Exact Files", "## Phase 3. External User Bearer API"),
    ("## Phase 3 Exact Snapshot", "## Phase 4."),
]

files = {}
for start, end in sections:
    start_match = re.search(rf"^{re.escape(start)}", text, re.M)
    if not start_match:
        raise SystemExit(f"section not found: {start}")
    start_index = start_match.start()
    end_match = re.search(rf"^{re.escape(end)}", text[start_index:], re.M)
    end_index = -1 if not end_match else start_index + end_match.start()
    section = text[start_index:] if end_index == -1 else text[start_index:end_index]
    for path, body in re.findall(r'^#### `([^`]+)`\n\n```[^\n]*\n(.*?)\n```', section, re.M | re.S):
        files[path] = body.rstrip("\n") + "\n"

for path, body in files.items():
    target = Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(body)
    print(f"wrote {target}")
PY
```

その後、生成物を現在の実装と同じ状態に戻します。`go test` は `make gen` の後に実行してください。`make gen` の前に test すると、`backend/internal/db/roles.sql.go` などが無くて build が失敗します。

```bash
npm --prefix frontend install
make gen
go test ./backend/...
docker-compose --env-file dev/zitadel/.env.example -f dev/zitadel/docker-compose.yml config --quiet
git diff --check
```

Zitadel compose の Git 管理対象は次の 2 file だけです。実ファイルの `dev/zitadel/.env` は `make zitadel-env` / `make zitadel-up` が `.env.example` から作りますが、Git には入れません。

```text
dev/zitadel/.env.example
dev/zitadel/docker-compose.yml
```

repo root の `.env` も Git には入れません。`.env.example` から作り、Zitadel Console で作った値と JWT decode 結果に合わせます。

Zitadel Console 内の project / application / role assignment は Git に入らない外部状態です。そこは本文の Step 0.3, Step 1.3, Step 3.2.5 の手順で作り直します。

#### `.env.example`

```dotenv
APP_NAME="HaoHao API"
APP_VERSION=0.1.0
HTTP_PORT=8080

APP_BASE_URL=http://127.0.0.1:8080
FRONTEND_BASE_URL=http://127.0.0.1:5173

DATABASE_URL=postgres://haohao:haohao@127.0.0.1:5432/haohao?sslmode=disable

AUTH_MODE=local
ZITADEL_ISSUER=
ZITADEL_CLIENT_ID=
ZITADEL_CLIENT_SECRET=
ZITADEL_REDIRECT_URI=http://127.0.0.1:8080/api/v1/auth/callback
ZITADEL_POST_LOGOUT_REDIRECT_URI=http://127.0.0.1:5173/login
ZITADEL_SCOPES="openid profile email"

REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=
REDIS_DB=0

SESSION_TTL=24h
LOGIN_STATE_TTL=10m

EXTERNAL_EXPECTED_AUDIENCE=haohao-external
EXTERNAL_REQUIRED_SCOPE_PREFIX=
EXTERNAL_REQUIRED_ROLE=external_api_user
EXTERNAL_ALLOWED_ORIGINS=

COOKIE_SECURE=false
```

#### `backend/internal/config/config.go`

```go
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName                      string
	AppVersion                   string
	HTTPPort                     int
	AppBaseURL                   string
	FrontendBaseURL              string
	DatabaseURL                  string
	AuthMode                     string
	ZitadelIssuer                string
	ZitadelClientID              string
	ZitadelClientSecret          string
	ZitadelRedirectURI           string
	ZitadelPostLogoutRedirectURI string
	ZitadelScopes                string
	ExternalExpectedAudience     string
	ExternalRequiredScopePrefix  string
	ExternalRequiredRole         string
	ExternalAllowedOrigins       []string
	RedisAddr                    string
	RedisPassword                string
	RedisDB                      int
	LoginStateTTL                time.Duration
	SessionTTL                   time.Duration
	CookieSecure                 bool
}

func Load() (Config, error) {
	sessionTTL, err := time.ParseDuration(getEnv("SESSION_TTL", "24h"))
	if err != nil {
		return Config{}, err
	}
	loginStateTTL, err := time.ParseDuration(getEnv("LOGIN_STATE_TTL", "10m"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppName:                      getEnv("APP_NAME", "HaoHao API"),
		AppVersion:                   getEnv("APP_VERSION", "0.1.0"),
		HTTPPort:                     getEnvInt("HTTP_PORT", 8080),
		AppBaseURL:                   strings.TrimRight(getEnv("APP_BASE_URL", "http://127.0.0.1:8080"), "/"),
		FrontendBaseURL:              strings.TrimRight(getEnv("FRONTEND_BASE_URL", "http://127.0.0.1:5173"), "/"),
		DatabaseURL:                  getEnv("DATABASE_URL", ""),
		AuthMode:                     getEnv("AUTH_MODE", "local"),
		ZitadelIssuer:                strings.TrimRight(getEnv("ZITADEL_ISSUER", ""), "/"),
		ZitadelClientID:              getEnv("ZITADEL_CLIENT_ID", ""),
		ZitadelClientSecret:          getEnv("ZITADEL_CLIENT_SECRET", ""),
		ZitadelRedirectURI:           getEnv("ZITADEL_REDIRECT_URI", "http://127.0.0.1:8080/api/v1/auth/callback"),
		ZitadelPostLogoutRedirectURI: getEnv("ZITADEL_POST_LOGOUT_REDIRECT_URI", "http://127.0.0.1:5173/login"),
		ZitadelScopes:                getEnv("ZITADEL_SCOPES", "openid profile email"),
		ExternalExpectedAudience:     getEnv("EXTERNAL_EXPECTED_AUDIENCE", "haohao-external"),
		ExternalRequiredScopePrefix:  getEnv("EXTERNAL_REQUIRED_SCOPE_PREFIX", ""),
		ExternalRequiredRole:         getEnv("EXTERNAL_REQUIRED_ROLE", "external_api_user"),
		ExternalAllowedOrigins:       getEnvCSV("EXTERNAL_ALLOWED_ORIGINS"),
		RedisAddr:                    getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:                getEnv("REDIS_PASSWORD", ""),
		RedisDB:                      getEnvInt("REDIS_DB", 0),
		LoginStateTTL:                loginStateTTL,
		SessionTTL:                   sessionTTL,
		CookieSecure:                 getEnvBool("COOKIE_SECURE", false),
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

func getEnvCSV(key string) []string {
	value := strings.TrimSpace(getEnv(key, ""))
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}

	return items
}
```

#### `db/migrations/0003_roles.up.sql`

```sql
CREATE TABLE roles (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_roles (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX user_roles_role_id_idx ON user_roles(role_id);

INSERT INTO roles (code)
VALUES
    ('docs_reader'),
    ('external_api_user'),
    ('todo_user')
ON CONFLICT (code) DO NOTHING;
```

#### `db/migrations/0003_roles.down.sql`

```sql
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS roles;
```

#### `db/schema.sql`

```sql
--
-- PostgreSQL database dump
--


-- Dumped from database version 18.3 (Debian 18.3-1.pgdg13+1)
-- Dumped by pg_dump version 18.3 (Debian 18.3-1.pgdg13+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: oauth_user_grants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.oauth_user_grants (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    provider text NOT NULL,
    resource_server text NOT NULL,
    provider_subject text NOT NULL,
    refresh_token_ciphertext bytea NOT NULL,
    refresh_token_key_version integer NOT NULL,
    scope_text text NOT NULL,
    granted_by_session_id text NOT NULL,
    granted_at timestamp with time zone DEFAULT now() NOT NULL,
    last_refreshed_at timestamp with time zone,
    revoked_at timestamp with time zone,
    last_error_code text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    tenant_id bigint NOT NULL
);


--
-- Name: oauth_user_grants_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.oauth_user_grants ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.oauth_user_grants_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: provisioning_sync_state; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.provisioning_sync_state (
    source text NOT NULL,
    cursor_text text,
    last_synced_at timestamp with time zone,
    last_error_code text,
    last_error_message text,
    failed_count integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.roles (
    id bigint NOT NULL,
    code text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: roles_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.roles ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.roles_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version bigint NOT NULL,
    dirty boolean NOT NULL
);


--
-- Name: tenant_memberships; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_memberships (
    user_id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    role_id bigint NOT NULL,
    source text NOT NULL,
    active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT tenant_memberships_source_check CHECK ((source = ANY (ARRAY['provider_claim'::text, 'scim'::text, 'local_override'::text])))
);


--
-- Name: tenant_role_overrides; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_role_overrides (
    user_id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    role_id bigint NOT NULL,
    effect text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT tenant_role_overrides_effect_check CHECK ((effect = ANY (ARRAY['allow'::text, 'deny'::text])))
);


--
-- Name: tenants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenants (
    id bigint NOT NULL,
    slug text NOT NULL,
    display_name text NOT NULL,
    active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: tenants_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.tenants ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.tenants_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: user_identities; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_identities (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    provider text NOT NULL,
    subject text NOT NULL,
    email text NOT NULL,
    email_verified boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    external_id text,
    provisioning_source text
);


--
-- Name: user_identities_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.user_identities ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.user_identities_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: user_roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_roles (
    user_id bigint NOT NULL,
    role_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    email text NOT NULL,
    display_name text NOT NULL,
    password_hash text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deactivated_at timestamp with time zone,
    default_tenant_id bigint
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.users ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: oauth_user_grants oauth_user_grants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_user_grants
    ADD CONSTRAINT oauth_user_grants_pkey PRIMARY KEY (id);


--
-- Name: oauth_user_grants oauth_user_grants_user_id_provider_resource_server_tenant_id_ke; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_user_grants
    ADD CONSTRAINT oauth_user_grants_user_id_provider_resource_server_tenant_id_ke UNIQUE (user_id, provider, resource_server, tenant_id);


--
-- Name: provisioning_sync_state provisioning_sync_state_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.provisioning_sync_state
    ADD CONSTRAINT provisioning_sync_state_pkey PRIMARY KEY (source);


--
-- Name: roles roles_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_code_key UNIQUE (code);


--
-- Name: roles roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: tenant_memberships tenant_memberships_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_memberships
    ADD CONSTRAINT tenant_memberships_pkey PRIMARY KEY (user_id, tenant_id, role_id, source);


--
-- Name: tenant_role_overrides tenant_role_overrides_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_role_overrides
    ADD CONSTRAINT tenant_role_overrides_pkey PRIMARY KEY (user_id, tenant_id, role_id, effect);


--
-- Name: tenants tenants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenants
    ADD CONSTRAINT tenants_pkey PRIMARY KEY (id);


--
-- Name: tenants tenants_slug_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenants
    ADD CONSTRAINT tenants_slug_key UNIQUE (slug);


--
-- Name: user_identities user_identities_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_identities
    ADD CONSTRAINT user_identities_pkey PRIMARY KEY (id);


--
-- Name: user_identities user_identities_provider_subject_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_identities
    ADD CONSTRAINT user_identities_provider_subject_key UNIQUE (provider, subject);


--
-- Name: user_roles user_roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_pkey PRIMARY KEY (user_id, role_id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: oauth_user_grants_provider_subject_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX oauth_user_grants_provider_subject_idx ON public.oauth_user_grants USING btree (provider, provider_subject);


--
-- Name: oauth_user_grants_resource_server_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX oauth_user_grants_resource_server_idx ON public.oauth_user_grants USING btree (resource_server);


--
-- Name: oauth_user_grants_tenant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX oauth_user_grants_tenant_id_idx ON public.oauth_user_grants USING btree (tenant_id);


--
-- Name: tenant_memberships_role_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tenant_memberships_role_id_idx ON public.tenant_memberships USING btree (role_id);


--
-- Name: tenant_memberships_tenant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tenant_memberships_tenant_id_idx ON public.tenant_memberships USING btree (tenant_id);


--
-- Name: tenant_role_overrides_tenant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tenant_role_overrides_tenant_id_idx ON public.tenant_role_overrides USING btree (tenant_id);


--
-- Name: user_identities_provider_external_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX user_identities_provider_external_id_key ON public.user_identities USING btree (provider, external_id) WHERE (external_id IS NOT NULL);


--
-- Name: user_identities_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_identities_user_id_idx ON public.user_identities USING btree (user_id);


--
-- Name: user_roles_role_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_roles_role_id_idx ON public.user_roles USING btree (role_id);


--
-- Name: oauth_user_grants oauth_user_grants_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_user_grants
    ADD CONSTRAINT oauth_user_grants_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: oauth_user_grants oauth_user_grants_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_user_grants
    ADD CONSTRAINT oauth_user_grants_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: tenant_memberships tenant_memberships_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_memberships
    ADD CONSTRAINT tenant_memberships_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id) ON DELETE CASCADE;


--
-- Name: tenant_memberships tenant_memberships_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_memberships
    ADD CONSTRAINT tenant_memberships_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: tenant_memberships tenant_memberships_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_memberships
    ADD CONSTRAINT tenant_memberships_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: tenant_role_overrides tenant_role_overrides_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_role_overrides
    ADD CONSTRAINT tenant_role_overrides_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id) ON DELETE CASCADE;


--
-- Name: tenant_role_overrides tenant_role_overrides_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_role_overrides
    ADD CONSTRAINT tenant_role_overrides_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: tenant_role_overrides tenant_role_overrides_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_role_overrides
    ADD CONSTRAINT tenant_role_overrides_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_identities user_identities_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_identities
    ADD CONSTRAINT user_identities_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_roles user_roles_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id) ON DELETE CASCADE;


--
-- Name: user_roles user_roles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: users users_default_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_default_tenant_id_fkey FOREIGN KEY (default_tenant_id) REFERENCES public.tenants(id) ON DELETE SET NULL;


--
-- PostgreSQL database dump complete
--
```

#### `db/queries/roles.sql`

```sql
-- name: GetRolesByCode :many
SELECT
    id,
    code
FROM roles
WHERE code = ANY($1::text[])
ORDER BY code;

-- name: ListRoleCodesByUserID :many
SELECT r.code
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
WHERE ur.user_id = $1
ORDER BY r.code;

-- name: DeleteUserRolesByUserID :exec
DELETE FROM user_roles
WHERE user_id = $1;

-- name: DeleteUserRolesExcluding :exec
DELETE FROM user_roles
WHERE user_id = $1
  AND NOT (role_id = ANY($2::bigint[]));

-- name: AssignUserRole :exec
INSERT INTO user_roles (
    user_id,
    role_id
) VALUES (
    $1,
    $2
)
ON CONFLICT (user_id, role_id) DO NOTHING;
```

#### `backend/internal/auth/bearer_verifier.go`

```go
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4/jwt"
)

var (
	ErrMissingBearerToken    = errors.New("missing bearer token")
	ErrInvalidBearerToken    = errors.New("invalid bearer token")
	ErrInvalidBearerIssuer   = errors.New("invalid bearer issuer")
	ErrInvalidBearerAudience = errors.New("invalid bearer audience")
	ErrInvalidBearerScope    = errors.New("invalid bearer scope")
	ErrInvalidBearerRole     = errors.New("invalid bearer role")
)

type BearerVerifier struct {
	issuer string
	keySet *oidc.RemoteKeySet
}

type BearerTokenClaims struct {
	jwt.Claims
	AuthorizedParty   string             `json:"azp,omitempty"`
	ClientID          string             `json:"client_id,omitempty"`
	Scope             spaceSeparatedList `json:"scope,omitempty"`
	Groups            claimStringList    `json:"groups,omitempty"`
	Roles             []string           `json:"-"`
	Email             string             `json:"email,omitempty"`
	Name              string             `json:"name,omitempty"`
	PreferredUsername string             `json:"preferred_username,omitempty"`
}

type bearerTokenClaimsJSON struct {
	jwt.Claims
	AuthorizedParty   string             `json:"azp,omitempty"`
	ClientID          string             `json:"client_id,omitempty"`
	Scope             spaceSeparatedList `json:"scope,omitempty"`
	Groups            claimStringList    `json:"groups,omitempty"`
	Email             string             `json:"email,omitempty"`
	Name              string             `json:"name,omitempty"`
	PreferredUsername string             `json:"preferred_username,omitempty"`
}

func (c *BearerTokenClaims) UnmarshalJSON(data []byte) error {
	var decoded bearerTokenClaimsJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	var rawClaims map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawClaims); err != nil {
		return err
	}

	c.Claims = decoded.Claims
	c.AuthorizedParty = strings.TrimSpace(decoded.AuthorizedParty)
	c.ClientID = strings.TrimSpace(decoded.ClientID)
	if c.AuthorizedParty == "" {
		c.AuthorizedParty = c.ClientID
	}
	c.Scope = decoded.Scope
	c.Groups = decoded.Groups
	c.Roles = extractZitadelRoleClaims(rawClaims)
	c.Email = decoded.Email
	c.Name = decoded.Name
	c.PreferredUsername = decoded.PreferredUsername

	return nil
}

func NewBearerVerifier(ctx context.Context, issuer string) (*BearerVerifier, error) {
	trimmedIssuer := strings.TrimRight(strings.TrimSpace(issuer), "/")
	if trimmedIssuer == "" {
		return nil, fmt.Errorf("issuer is required")
	}

	provider, err := oidc.NewProvider(ctx, trimmedIssuer)
	if err != nil {
		return nil, fmt.Errorf("discover oidc provider: %w", err)
	}

	var discovery struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := provider.Claims(&discovery); err != nil {
		return nil, fmt.Errorf("decode oidc discovery document: %w", err)
	}
	if strings.TrimSpace(discovery.JWKSURI) == "" {
		return nil, fmt.Errorf("jwks_uri missing from oidc discovery document")
	}

	return &BearerVerifier{
		issuer: trimmedIssuer,
		keySet: oidc.NewRemoteKeySet(ctx, discovery.JWKSURI),
	}, nil
}

func (v *BearerVerifier) Verify(ctx context.Context, rawToken, expectedAudience, requiredScopePrefix string) (BearerTokenClaims, error) {
	if strings.TrimSpace(rawToken) == "" {
		return BearerTokenClaims{}, ErrMissingBearerToken
	}
	if v == nil || v.keySet == nil {
		return BearerTokenClaims{}, fmt.Errorf("%w: verifier is not configured", ErrInvalidBearerToken)
	}

	payload, err := v.keySet.VerifySignature(ctx, rawToken)
	if err != nil {
		return BearerTokenClaims{}, fmt.Errorf("%w: verify signature: %v", ErrInvalidBearerToken, err)
	}

	var claims BearerTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return BearerTokenClaims{}, fmt.Errorf("%w: decode claims: %v", ErrInvalidBearerToken, err)
	}

	expected := jwt.Expected{
		Issuer: v.issuer,
		Time:   time.Now(),
	}
	if audience := strings.TrimSpace(expectedAudience); audience != "" {
		expected.AnyAudience = jwt.Audience{audience}
	}

	if err := claims.Claims.ValidateWithLeeway(expected, time.Minute); err != nil {
		switch {
		case errors.Is(err, jwt.ErrInvalidIssuer):
			return BearerTokenClaims{}, ErrInvalidBearerIssuer
		case errors.Is(err, jwt.ErrInvalidAudience):
			return BearerTokenClaims{}, ErrInvalidBearerAudience
		default:
			return BearerTokenClaims{}, fmt.Errorf("%w: %v", ErrInvalidBearerToken, err)
		}
	}

	if strings.TrimSpace(claims.Subject) == "" {
		return BearerTokenClaims{}, fmt.Errorf("%w: subject is required", ErrInvalidBearerToken)
	}
	if prefix := strings.TrimSpace(requiredScopePrefix); prefix != "" && !claims.HasScopePrefix(prefix) {
		return BearerTokenClaims{}, ErrInvalidBearerScope
	}

	return claims, nil
}

func (c BearerTokenClaims) ScopeValues() []string {
	return append([]string(nil), c.Scope...)
}

func (c BearerTokenClaims) GroupValues() []string {
	return append([]string(nil), c.Groups...)
}

func (c BearerTokenClaims) RoleValues() []string {
	return append([]string(nil), c.Roles...)
}

func (c BearerTokenClaims) HasScopePrefix(prefix string) bool {
	trimmedPrefix := strings.TrimSpace(prefix)
	if trimmedPrefix == "" {
		return true
	}

	for _, scope := range c.Scope {
		if scope == trimmedPrefix || strings.HasPrefix(scope, trimmedPrefix) {
			return true
		}
	}

	return false
}

func extractZitadelRoleClaims(rawClaims map[string]json.RawMessage) []string {
	roleSet := make(map[string]struct{})
	for name, raw := range rawClaims {
		if !isZitadelRoleClaim(name) {
			continue
		}

		for _, role := range roleNamesFromClaim(raw) {
			roleSet[role] = struct{}{}
		}
	}

	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}

func isZitadelRoleClaim(name string) bool {
	return name == "urn:zitadel:iam:org:project:roles" ||
		(strings.HasPrefix(name, "urn:zitadel:iam:org:project:") && strings.HasSuffix(name, ":roles"))
}

func roleNamesFromClaim(raw json.RawMessage) []string {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err == nil {
		roles := make([]string, 0, len(object))
		for role := range object {
			if trimmed := strings.TrimSpace(role); trimmed != "" {
				roles = append(roles, trimmed)
			}
		}
		return roles
	}

	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		roles := make([]string, 0, len(many))
		for _, role := range many {
			if trimmed := strings.TrimSpace(role); trimmed != "" {
				roles = append(roles, trimmed)
			}
		}
		return roles
	}

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if trimmed := strings.TrimSpace(single); trimmed != "" {
			return []string{trimmed}
		}
	}

	return nil
}

type spaceSeparatedList []string

func (s *spaceSeparatedList) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*s = append((*s)[:0], strings.Fields(single)...)
		return nil
	}

	var many []string
	if err := json.Unmarshal(data, &many); err == nil {
		items := make([]string, 0, len(many))
		for _, item := range many {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				items = append(items, trimmed)
			}
		}
		*s = items
		return nil
	}

	return fmt.Errorf("unsupported scope claim format")
}

type claimStringList []string

func (s *claimStringList) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		single = strings.TrimSpace(single)
		if single == "" {
			*s = nil
			return nil
		}
		*s = []string{single}
		return nil
	}

	var many []string
	if err := json.Unmarshal(data, &many); err == nil {
		items := make([]string, 0, len(many))
		for _, item := range many {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				items = append(items, trimmed)
			}
		}
		*s = items
		return nil
	}

	return fmt.Errorf("unsupported string list claim format")
}
```

#### `backend/internal/service/authz_service.go`

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"example.com/haohao/backend/internal/auth"
	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthContext struct {
	AuthenticatedBy string
	Provider        string
	Subject         string
	AuthorizedParty string
	Scopes          []string
	Groups          []string
	Roles           []string
	User            *User
}

type authContextKey struct{}

type AuthzService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewAuthzService(pool *pgxpool.Pool, queries *db.Queries) *AuthzService {
	return &AuthzService{
		pool:    pool,
		queries: queries,
	}
}

func ContextWithAuthContext(ctx context.Context, authCtx AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey{}, authCtx)
}

func AuthContextFromContext(ctx context.Context) (AuthContext, bool) {
	authCtx, ok := ctx.Value(authContextKey{}).(AuthContext)
	return authCtx, ok
}

func (a AuthContext) HasRole(role string) bool {
	needle := strings.ToLower(strings.TrimSpace(role))
	if needle == "" {
		return true
	}

	for _, item := range append(append([]string{}, a.Roles...), a.Groups...) {
		if strings.ToLower(strings.TrimSpace(item)) == needle {
			return true
		}
	}

	return false
}

func (a AuthContext) HasProviderRole(role string) bool {
	needle := strings.ToLower(strings.TrimSpace(role))
	if needle == "" {
		return true
	}

	for _, item := range a.Groups {
		if strings.ToLower(strings.TrimSpace(item)) == needle {
			return true
		}
	}

	return false
}

func (s *AuthzService) SyncGlobalRoles(ctx context.Context, userID int64, providerGroups []string) ([]string, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return nil, fmt.Errorf("authz service is not configured")
	}

	roleCodes := normalizeGlobalRoleCodes(providerGroups)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin role sync transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	if len(roleCodes) == 0 {
		if err := qtx.DeleteUserRolesByUserID(ctx, userID); err != nil {
			return nil, fmt.Errorf("delete user roles: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit empty role sync transaction: %w", err)
		}
		return nil, nil
	}

	roles, err := qtx.GetRolesByCode(ctx, roleCodes)
	if err != nil {
		return nil, fmt.Errorf("load roles by code: %w", err)
	}

	roleIDs := make([]int64, 0, len(roles))
	syncedCodes := make([]string, 0, len(roles))
	for _, role := range roles {
		roleIDs = append(roleIDs, role.ID)
		syncedCodes = append(syncedCodes, role.Code)
	}

	if err := qtx.DeleteUserRolesExcluding(ctx, db.DeleteUserRolesExcludingParams{
		UserID:  userID,
		Column2: roleIDs,
	}); err != nil {
		return nil, fmt.Errorf("delete stale user roles: %w", err)
	}

	for _, roleID := range roleIDs {
		if err := qtx.AssignUserRole(ctx, db.AssignUserRoleParams{
			UserID: userID,
			RoleID: roleID,
		}); err != nil {
			return nil, fmt.Errorf("assign user role: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit role sync transaction: %w", err)
	}

	return syncedCodes, nil
}

func (s *AuthzService) AuthContextFromBearer(ctx context.Context, provider string, claims auth.BearerTokenClaims) (AuthContext, error) {
	authCtx := AuthContext{
		AuthenticatedBy: "bearer",
		Provider:        strings.ToLower(strings.TrimSpace(provider)),
		Subject:         strings.TrimSpace(claims.Subject),
		AuthorizedParty: strings.TrimSpace(claims.AuthorizedParty),
		Scopes:          claims.ScopeValues(),
		Groups:          mergeClaimValues(claims.GroupValues(), claims.RoleValues()),
	}

	if s == nil || s.queries == nil || authCtx.Provider == "" || authCtx.Subject == "" {
		return authCtx, nil
	}

	user, err := s.queries.GetUserByProviderSubject(ctx, db.GetUserByProviderSubjectParams{
		Provider: authCtx.Provider,
		Subject:  authCtx.Subject,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authCtx, nil
		}
		return AuthContext{}, fmt.Errorf("lookup user by provider subject: %w", err)
	}

	localUser := dbUser(user.ID, user.PublicID.String(), user.Email, user.DisplayName)
	authCtx.User = &localUser

	if len(authCtx.Groups) > 0 {
		roleCodes, err := s.SyncGlobalRoles(ctx, localUser.ID, authCtx.Groups)
		if err != nil {
			return AuthContext{}, fmt.Errorf("sync global roles from bearer claims: %w", err)
		}
		authCtx.Roles = roleCodes
		return authCtx, nil
	}

	roleCodes, err := s.queries.ListRoleCodesByUserID(ctx, localUser.ID)
	if err != nil {
		return AuthContext{}, fmt.Errorf("list local roles by user id: %w", err)
	}
	authCtx.Roles = roleCodes

	return authCtx, nil
}

var supportedGlobalRoles = map[string]struct{}{
	"docs_reader":       {},
	"external_api_user": {},
	"todo_user":         {},
}

func normalizeGlobalRoleCodes(providerGroups []string) []string {
	set := make(map[string]struct{}, len(providerGroups))
	for _, group := range providerGroups {
		code := strings.ToLower(strings.TrimSpace(group))
		if _, ok := supportedGlobalRoles[code]; ok {
			set[code] = struct{}{}
		}
	}

	roleCodes := make([]string, 0, len(set))
	for code := range set {
		roleCodes = append(roleCodes, code)
	}
	sort.Strings(roleCodes)

	return roleCodes
}

func mergeClaimValues(values ...[]string) []string {
	set := make(map[string]struct{})
	for _, group := range values {
		for _, value := range group {
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				set[trimmed] = struct{}{}
			}
		}
	}

	merged := make([]string, 0, len(set))
	for value := range set {
		merged = append(merged, value)
	}
	sort.Strings(merged)

	return merged
}
```

#### `backend/internal/service/oidc_login_service.go`

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"example.com/haohao/backend/internal/auth"
)

type OIDCLoginResult struct {
	SessionID string
	CSRFToken string
	ReturnTo  string
}

type OIDCLoginService struct {
	providerName   string
	oidcClient     *auth.OIDCClient
	loginState     *auth.LoginStateStore
	identity       *IdentityService
	authzService   *AuthzService
	sessionService *SessionService
}

func NewOIDCLoginService(providerName string, oidcClient *auth.OIDCClient, loginState *auth.LoginStateStore, identity *IdentityService, authzService *AuthzService, sessionService *SessionService) *OIDCLoginService {
	return &OIDCLoginService{
		providerName:   providerName,
		oidcClient:     oidcClient,
		loginState:     loginState,
		identity:       identity,
		authzService:   authzService,
		sessionService: sessionService,
	}
}

func (s *OIDCLoginService) StartLogin(ctx context.Context, returnTo string) (string, error) {
	if s == nil || s.oidcClient == nil || s.loginState == nil {
		return "", ErrAuthModeUnsupported
	}

	state, record, err := s.loginState.Create(ctx, sanitizeReturnTo(returnTo))
	if err != nil {
		return "", fmt.Errorf("create oidc login state: %w", err)
	}

	return s.oidcClient.AuthorizeURL(state, record.Nonce, record.CodeVerifier), nil
}

func (s *OIDCLoginService) FinishLogin(ctx context.Context, code, state string) (OIDCLoginResult, error) {
	if s == nil || s.oidcClient == nil || s.loginState == nil || s.identity == nil || s.sessionService == nil {
		return OIDCLoginResult{}, ErrAuthModeUnsupported
	}

	loginState, err := s.loginState.Consume(ctx, state)
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("consume oidc login state: %w", err)
	}

	identity, err := s.oidcClient.ExchangeCode(ctx, code, loginState.CodeVerifier, loginState.Nonce)
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("finish oidc code exchange: %w", err)
	}

	user, err := s.identity.ResolveOrCreateUser(ctx, ExternalIdentity{
		Provider:      s.providerName,
		Subject:       identity.Claims.Subject,
		Email:         identity.Claims.Email,
		EmailVerified: identity.Claims.EmailVerified,
		DisplayName:   identity.Claims.Name,
	})
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("resolve local user for oidc identity: %w", err)
	}

	if s.authzService != nil {
		if _, err := s.authzService.SyncGlobalRoles(ctx, user.ID, identity.Claims.Groups); err != nil {
			return OIDCLoginResult{}, fmt.Errorf("sync local roles for oidc login: %w", err)
		}
	}

	sessionID, csrfToken, err := s.sessionService.IssueSessionWithProviderHint(ctx, user.ID, identity.RawIDToken)
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("issue local session for oidc login: %w", err)
	}

	return OIDCLoginResult{
		SessionID: sessionID,
		CSRFToken: csrfToken,
		ReturnTo:  sanitizeReturnTo(loginState.ReturnTo),
	}, nil
}

func sanitizeReturnTo(returnTo string) string {
	trimmed := strings.TrimSpace(returnTo)
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return "/"
	}
	return trimmed
}

func IsOIDCLoginFailure(err error) bool {
	return err != nil && (errors.Is(err, auth.ErrLoginStateNotFound) || errors.Is(err, ErrInvalidExternalIdentity))
}
```

#### `backend/internal/middleware/external_auth.go`

```go
package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"example.com/haohao/backend/internal/auth"
	"example.com/haohao/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func ExternalCORS(pathPrefix string, allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, pathPrefix) {
			c.Next()
			return
		}

		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" && originAllowed(origin, allowed) {
			header := c.Writer.Header()
			header.Set("Access-Control-Allow-Origin", origin)
			header.Add("Vary", "Origin")
			header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			header.Set("Access-Control-Max-Age", "600")
		}

		if c.Request.Method == http.MethodOptions {
			if origin == "" || !originAllowed(origin, allowed) {
				writeProblem(c, http.StatusForbidden, "origin is not allowed")
				return
			}

			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}

		c.Next()
	}
}

func ExternalAuth(pathPrefix string, verifier *auth.BearerVerifier, authzService *service.AuthzService, providerName, expectedAudience, requiredScopePrefix, requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, pathPrefix) || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		if verifier == nil || authzService == nil {
			writeProblem(c, http.StatusServiceUnavailable, "external bearer auth is not configured")
			return
		}

		rawToken, err := bearerTokenFromHeader(c.GetHeader("Authorization"))
		if err != nil {
			writeBearerProblem(c, http.StatusUnauthorized, err.Error())
			return
		}

		claims, err := verifier.Verify(c.Request.Context(), rawToken, expectedAudience, requiredScopePrefix)
		if err != nil {
			status := http.StatusUnauthorized
			switch {
			case err == auth.ErrInvalidBearerScope:
				status = http.StatusForbidden
			case err == auth.ErrInvalidBearerAudience, err == auth.ErrInvalidBearerIssuer, err == auth.ErrMissingBearerToken:
				status = http.StatusUnauthorized
			}
			writeBearerProblem(c, status, err.Error())
			return
		}

		authCtx, err := authzService.AuthContextFromBearer(c.Request.Context(), providerName, claims)
		if err != nil {
			writeProblem(c, http.StatusInternalServerError, "failed to build auth context")
			return
		}
		if !authCtx.HasProviderRole(requiredRole) {
			writeBearerProblem(c, http.StatusForbidden, auth.ErrInvalidBearerRole.Error())
			return
		}

		c.Request = c.Request.WithContext(service.ContextWithAuthContext(c.Request.Context(), authCtx))
		c.Next()
	}
}

func bearerTokenFromHeader(header string) (string, error) {
	trimmed := strings.TrimSpace(header)
	if trimmed == "" {
		return "", auth.ErrMissingBearerToken
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(trimmed, prefix) {
		return "", fmt.Errorf("%w: authorization header must use Bearer", auth.ErrInvalidBearerToken)
	}

	token := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	if token == "" {
		return "", auth.ErrMissingBearerToken
	}

	return token, nil
}

func originAllowed(origin string, allowed map[string]struct{}) bool {
	if len(allowed) == 0 {
		return false
	}
	_, ok := allowed[origin]
	return ok
}

func writeBearerProblem(c *gin.Context, status int, detail string) {
	c.Header("WWW-Authenticate", `Bearer realm="haohao-external"`)
	writeProblem(c, status, detail)
}

func writeProblem(c *gin.Context, status int, detail string) {
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(status, gin.H{
		"title":  http.StatusText(status),
		"status": status,
		"detail": detail,
	})
}
```

#### `backend/internal/api/external_me.go`

```go
package api

import (
	"context"
	"net/http"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type ExternalMeBody struct {
	Provider        string        `json:"provider" example:"zitadel"`
	Subject         string        `json:"subject" example:"312345678901234567"`
	AuthorizedParty string        `json:"authorizedParty,omitempty" example:"312345678901234568"`
	Scopes          []string      `json:"scopes,omitempty" example:"external:read"`
	Groups          []string      `json:"groups,omitempty" example:"external_api_user"`
	Roles           []string      `json:"roles,omitempty" example:"todo_user"`
	User            *UserResponse `json:"user,omitempty"`
}

type GetExternalMeOutput struct {
	Body ExternalMeBody
}

func registerExternalRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "getExternalMe",
		Method:      http.MethodGet,
		Path:        "/api/external/v1/me",
		Summary:     "現在の external bearer principal を返す",
		Tags:        []string{"external"},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *struct{}) (*GetExternalMeOutput, error) {
		authCtx, ok := service.AuthContextFromContext(ctx)
		if !ok {
			return nil, huma.Error500InternalServerError("missing auth context")
		}

		var user *UserResponse
		if authCtx.User != nil {
			res := toUserResponse(*authCtx.User)
			user = &res
		}

		return &GetExternalMeOutput{
			Body: ExternalMeBody{
				Provider:        authCtx.Provider,
				Subject:         authCtx.Subject,
				AuthorizedParty: authCtx.AuthorizedParty,
				Scopes:          authCtx.Scopes,
				Groups:          authCtx.Groups,
				Roles:           authCtx.Roles,
				User:            user,
			},
		}, nil
	})
}
```

#### `backend/internal/api/register.go`

```go
package api

import (
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type Dependencies struct {
	SessionService               *service.SessionService
	OIDCLoginService             *service.OIDCLoginService
	AuthMode                     string
	FrontendBaseURL              string
	ZitadelIssuer                string
	ZitadelClientID              string
	ZitadelPostLogoutRedirectURI string
	CookieSecure                 bool
	SessionTTL                   time.Duration
}

func Register(api huma.API, deps Dependencies) {
	registerAuthSettingsRoute(api, deps)
	registerOIDCRoutes(api, deps)
	registerSessionRoutes(api, deps)
	registerExternalRoutes(api, deps)
}
```

#### `backend/internal/app/app.go`

```go
package app

import (
	backendapi "example.com/haohao/backend/internal/api"
	"example.com/haohao/backend/internal/auth"
	"example.com/haohao/backend/internal/config"
	"example.com/haohao/backend/internal/middleware"
	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
)

type App struct {
	Router *gin.Engine
	API    huma.API
}

func New(cfg config.Config, sessionService *service.SessionService, oidcLoginService *service.OIDCLoginService, authzService *service.AuthzService, bearerVerifier *auth.BearerVerifier) *App {
	router := gin.New()
	router.Use(
		gin.Logger(),
		gin.Recovery(),
		middleware.ExternalCORS("/api/external/", cfg.ExternalAllowedOrigins),
		middleware.ExternalAuth("/api/external/", bearerVerifier, authzService, "zitadel", cfg.ExternalExpectedAudience, cfg.ExternalRequiredScopePrefix, cfg.ExternalRequiredRole),
	)

	humaConfig := huma.DefaultConfig(cfg.AppName, cfg.AppVersion)
	humaConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"cookieAuth": {
			Type: "apiKey",
			In:   "cookie",
			Name: auth.SessionCookieName,
		},
		"bearerAuth": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		},
	}

	api := humagin.New(router, humaConfig)

	backendapi.Register(api, backendapi.Dependencies{
		SessionService:               sessionService,
		OIDCLoginService:             oidcLoginService,
		AuthMode:                     cfg.AuthMode,
		FrontendBaseURL:              cfg.FrontendBaseURL,
		ZitadelIssuer:                cfg.ZitadelIssuer,
		ZitadelClientID:              cfg.ZitadelClientID,
		ZitadelPostLogoutRedirectURI: cfg.ZitadelPostLogoutRedirectURI,
		CookieSecure:                 cfg.CookieSecure,
		SessionTTL:                   cfg.SessionTTL,
	})

	return &App{
		Router: router,
		API:    api,
	}
}
```

#### `backend/cmd/main/main.go`

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
	sessionService := service.NewSessionService(queries, sessionStore, cfg.AuthMode)
	authzService := service.NewAuthzService(pool, queries)

	var oidcLoginService *service.OIDCLoginService
	var bearerVerifier *auth.BearerVerifier
	if cfg.AuthMode == "zitadel" {
		if cfg.ZitadelIssuer == "" || cfg.ZitadelClientID == "" || cfg.ZitadelClientSecret == "" {
			log.Fatal("ZITADEL_ISSUER, ZITADEL_CLIENT_ID, and ZITADEL_CLIENT_SECRET are required when AUTH_MODE=zitadel")
		}

		oidcClient, err := auth.NewOIDCClient(
			ctx,
			cfg.ZitadelIssuer,
			cfg.ZitadelClientID,
			cfg.ZitadelClientSecret,
			cfg.ZitadelRedirectURI,
			cfg.ZitadelScopes,
		)
		if err != nil {
			log.Fatal(err)
		}

		loginStateStore := auth.NewLoginStateStore(redisClient, cfg.LoginStateTTL)
		identityService := service.NewIdentityService(pool, queries)
		oidcLoginService = service.NewOIDCLoginService("zitadel", oidcClient, loginStateStore, identityService, authzService, sessionService)
	}

	if cfg.ZitadelIssuer != "" {
		bearerVerifier, err = auth.NewBearerVerifier(ctx, cfg.ZitadelIssuer)
		if err != nil {
			log.Fatal(err)
		}
	}

	application := app.New(cfg, sessionService, oidcLoginService, authzService, bearerVerifier)

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

#### `backend/cmd/openapi/main.go`

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

	application := app.New(cfg, nil, nil, nil, nil)

	spec, err := application.API.OpenAPI().YAML()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(string(spec))
}
```

---

## Phase 4. Downstream Delegated Auth と Refresh Token 管理

### 目的

backend が browser user の代理で downstream API を呼べるようにしつつ、refresh token を browser に出さない構成を完成させます。

### この Phase の前提

- browser 向け auth foundation が安定している
- external bearer API と generic JWT verifier がある

### この Phase の完了条件

- refresh token が server side の暗号化ストアに保存される
- browser は refresh token を一切見ない
- downstream grant schema が tenant 未導入でも読める
- token rotation / revoke / invalid_grant handling がある

### Step 4.1. delegated auth 用の設定を増やす

`.env.example` に少なくとも次を足します。

```dotenv
DOWNSTREAM_TOKEN_ENCRYPTION_KEY=
DOWNSTREAM_TOKEN_KEY_VERSION=1
DOWNSTREAM_REFRESH_TOKEN_TTL=2160h
DOWNSTREAM_ACCESS_TOKEN_SKEW=30s
DOWNSTREAM_DEFAULT_SCOPES=offline_access
```

- `DOWNSTREAM_TOKEN_ENCRYPTION_KEY` は 32 byte 相当の base64 文字列に固定します
- refresh token の暗号化は **application layer の AES-256-GCM** で行います
- key rotation のために ciphertext と一緒に `key_version` を保存します

### Step 4.2. grant schema は tenant 未導入段階で先に置く

tenant は Phase 5 で入ります。したがって、この Phase では `oauth_user_grants` を **tenant 非依存の形で先に置く**のが自然です。

#### 追加するテーブルと query

- `oauth_user_grants`
- `db/queries/downstream_grants.sql`

#### この Phase の最低限の列

- `user_id`
- `provider`
- `resource_server`
- `provider_subject`
- `refresh_token_ciphertext`
- `refresh_token_key_version`
- `scope_text`
- `granted_by_session_id`
- `granted_at`
- `last_refreshed_at`
- `revoked_at`
- `last_error_code`

#### 初期 unique key

この Phase では次で固定してください。

```text
(user_id, provider, resource_server)
```

`tenant_id` はまだ持ちません。tenant-aware 化は Phase 5 で別 migration として追加します。

### Step 4.3. refresh token store と delegated auth service を作る

#### 追加するファイル

- `backend/internal/auth/refresh_token_store.go`
- `backend/internal/service/delegation_service.go`
- `backend/internal/api/integrations.go`

#### API と service の形

この Phase では、最低限次の導線を用意してください。

```text
GET    /api/v1/integrations/{resourceServer}/connect
GET    /api/v1/integrations/{resourceServer}/callback
DELETE /api/v1/integrations/{resourceServer}/grant
```

- `connect` は downstream 用 consent 画面へ redirect する
- `callback` は code exchange 後に refresh token を暗号化保存する
- `grant delete` は local grant を削除し、provider revocation endpoint があれば upstream 側も revoke する

service は少なくとも次の 2 操作を持つ形にしてください。

```text
SaveGrantFromCallback(ctx, authContext, resourceServer, code, state)
GetAccessToken(ctx, authContext, resourceServer) -> access token
```

#### 振る舞いの固定

- refresh token は **絶対に browser へ返さない**
- access token も browser へ返さず、backend が必要な瞬間だけ取得して下流 API に付ける
- token endpoint から新しい refresh token が返ったら、同じ transaction 内で暗号化し直して上書きする
- `invalid_grant` を受けたら `revoked_at` と `last_error_code` を更新し、その grant は即座に無効化する
- browser logout では grant を自動削除しません。downstream 連携の切断は明示的な `DELETE` で行います

#### browser login との関係

browser login の scope は引き続き `openid profile email` のままです。`offline_access` は delegated consent 導線だけで要求してください。

#### この Phase の手動確認

1. consent 完了後に local grant が保存される
2. refresh token が平文で browser や frontend state に出ない
3. downstream access token を backend が必要な瞬間だけ取得する
4. `invalid_grant` を返したとき grant が revoke 状態になる
5. `DELETE /grant` で local revoke と upstream revoke が走る

### Step 4.4. この repo での Phase 4 実装形

この Phase では downstream resource をまず `zitadel` に固定します。`resourceServer` path parameter は allowlist で検証し、`zitadel` 以外は unsupported resource として扱います。

backend が追加する browser session 向け API は次です。どれも Cookie session 前提で、refresh token / access token は response に含めません。

```text
GET    /api/v1/integrations
GET    /api/v1/integrations/{resourceServer}/connect
GET    /api/v1/integrations/{resourceServer}/callback
POST   /api/v1/integrations/{resourceServer}/verify
DELETE /api/v1/integrations/{resourceServer}/grant
```

`POST /verify` は backend 内で refresh token から access token を取得できるかだけを確認します。返すのは `resourceServer`, `connected`, `scopes`, `accessExpiresAt`, `refreshedAt` だけです。

frontend は `/integrations` を追加します。画面からできることは次です。

```text
connect  -> /api/v1/integrations/zitadel/connect へ遷移
verify   -> POST /api/v1/integrations/zitadel/verify
revoke   -> DELETE /api/v1/integrations/zitadel/grant
status   -> GET /api/v1/integrations
```

### Step 4.5. Zitadel Console の追加設定

browser login 用に使っている application、つまり `.env` の `ZITADEL_CLIENT_ID` に対応する application を開きます。この Phase では同じ OAuth client を delegated consent にも使います。

まず redirect URI を 1 つ追加します。

```text
http://127.0.0.1:8080/api/v1/integrations/zitadel/callback
```

browser login callback は引き続き次のまま残します。

```text
http://127.0.0.1:8080/api/v1/auth/callback
```

次に application settings で **Refresh Token を有効化**します。これを有効にしないと、`offline_access` を要求しても token endpoint の response に `refresh_token` が入らず、backend は grant を保存できません。その場合、frontend は次のように戻ります。

```text
http://127.0.0.1:5173/integrations?error=delegated_callback_failed
```

`ZITADEL_SCOPES` は引き続き `openid profile email` のままです。`offline_access` は integrations の delegated consent でだけ要求します。

### Step 4.6. local env の追加

repo root の `.env` に次を追加します。`.env` は Git に入れません。

```dotenv
DOWNSTREAM_TOKEN_ENCRYPTION_KEY=<32 byte key encoded as base64>
DOWNSTREAM_TOKEN_KEY_VERSION=1
DOWNSTREAM_REFRESH_TOKEN_TTL=2160h
DOWNSTREAM_ACCESS_TOKEN_SKEW=30s
DOWNSTREAM_DEFAULT_SCOPES=offline_access
```

key は例えば次で作れます。

```bash
openssl rand -base64 32
```

`DOWNSTREAM_TOKEN_ENCRYPTION_KEY` が空のままだと integrations API は configured にならず、接続状態 API は 503 を返します。

### Step 4.7. Phase 4 exact delta

Phase 3 replay 後に Phase 4 で増える主な source delta は次です。generated files はこのあと `make gen` で揃えます。

```text
.env.example
backend/cmd/main/main.go
backend/cmd/openapi/main.go
backend/internal/api/integrations.go
backend/internal/api/register.go
backend/internal/app/app.go
backend/internal/auth/delegated_oauth_client.go
backend/internal/auth/delegation_state_store.go
backend/internal/auth/refresh_token_store.go
backend/internal/config/config.go
backend/internal/service/delegation_service.go
backend/internal/service/session_service.go
db/migrations/0004_downstream_grants.up.sql
db/migrations/0004_downstream_grants.down.sql
db/queries/downstream_grants.sql
db/queries/identities.sql
frontend/src/api/integrations.ts
frontend/src/router/index.ts
frontend/src/views/IntegrationsView.vue
frontend/src/App.vue
```

generated / snapshot delta は次です。

```text
backend/internal/db/downstream_grants.sql.go
backend/internal/db/identities.sql.go
backend/internal/db/models.go
db/schema.sql
openapi/openapi.yaml
frontend/src/api/generated/index.ts
frontend/src/api/generated/sdk.gen.ts
frontend/src/api/generated/types.gen.ts
```

Phase 4 の生成は次で揃えます。

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
git diff --check
if docker compose version >/dev/null 2>&1; then
  docker compose --env-file dev/zitadel/.env.example -f dev/zitadel/docker-compose.yml config --quiet
else
  docker-compose --env-file dev/zitadel/.env.example -f dev/zitadel/docker-compose.yml config --quiet
fi
```

手元で DB を見るときも `docker compose` plugin と `docker-compose` binary の差があります。`unknown shorthand flag: 'T' in -T` が出る環境では `docker compose` plugin が無く、`-T` が Docker 本体の option として解釈されています。その場合は hyphen 付きの `docker-compose exec -T ...` を使ってください。

この tutorial の確認コマンドは、必要なら先に次で compose command を固定してから実行します。

```bash
if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
else
  COMPOSE="docker-compose"
fi
```

manual smoke は次の順で見ます。ここでは Connect / Verify / Revoke を必須確認にします。`invalid_grant` は provider 側で refresh token を失効させる必要があるため、通常 smoke とは分けて任意の負テストとして扱います。

```text
1. backend / frontend を起動する前に :8080 が空いていることを確認する
2. /integrations を開き、zitadel integration が Disconnected で表示される
3. Connect で consent へ遷移し、callback 後に Connected へ戻る
4. 画面で zitadel integration が Connected になり、Scopes に offline_access が表示される
5. Connect 直後は Last refresh が Never であることを見る
6. oauth_user_grants.refresh_token_ciphertext が平文 token ではないことを DB で見る
7. Verify で access check が成功し、browser response に token が出ないことを DevTools で見る
8. Verify 後に Last refresh が更新されることを見る
9. Revoke で local grant が削除され、可能なら upstream revocation も成功することを見る
```

成功時の `/integrations` 画面は、`zitadel` card が `CONNECTED` になり、`Scopes` に `offline_access` が表示されます。Connect 直後はまだ backend が access token refresh を試していないため、`Last refresh` は `Never` のままで正常です。Verify 後に `Last refresh` が時刻表示へ変われば、backend-only の refresh token から access token を取得できています。

UI は browser の local timezone で表示し、PostgreSQL は `timestamptz` を UTC で返すことがあります。例えば DB の `2026-04-24 15:39:01+00` が、Asia/Tokyo の browser では `2026/04/25 0:39` と表示されても正常です。

Connect 後の DB 確認は次です。`ciphertext_hex` は長い hex 文字列になり、refresh token の平文には見えないことを確認します。

```bash
$COMPOSE exec -T postgres psql -U haohao -d haohao -c "
select
  user_id,
  provider,
  resource_server,
  provider_subject,
  refresh_token_key_version,
  encode(refresh_token_ciphertext, 'hex') as ciphertext_hex,
  scope_text,
  granted_at,
  revoked_at,
  last_error_code
from oauth_user_grants;
"
```

期待値は次です。

```text
provider = zitadel
resource_server = zitadel
scope_text = offline_access
refresh_token_key_version = 1
revoked_at is null
last_error_code is null
```

Verify 後は refresh が成功したことだけを確認します。token 本体は API response に出しません。

```bash
$COMPOSE exec -T postgres psql -U haohao -d haohao -c "
select
  resource_server,
  scope_text,
  last_refreshed_at,
  revoked_at,
  last_error_code
from oauth_user_grants;
"
```

期待値は `last_refreshed_at` に時刻が入り、`revoked_at` と `last_error_code` が空のままです。

Revoke 後は row が削除されることを確認します。

```bash
$COMPOSE exec -T postgres psql -U haohao -d haohao -c "
select count(*) from oauth_user_grants;
"
```

期待値は `0` です。

`invalid_grant` handling まで確認する場合は、Connect 後に Zitadel 側で該当 user / application の grant か refresh token を失効させてから Verify します。期待値は Verify が失敗し、local row は残ったまま `revoked_at` と `last_error_code = invalid_grant` が入ることです。DB の ciphertext を壊すテストは復号エラーの確認になり、provider からの `invalid_grant` 確認にはなりません。

`delegated_callback_failed` で戻った場合は、まず次を確認してください。

```text
1. Zitadel application settings で Refresh Token が有効化されている
2. delegated callback URI が application に登録されている
3. .env の DOWNSTREAM_TOKEN_ENCRYPTION_KEY が 32 byte base64 になっている
4. .env の DOWNSTREAM_DEFAULT_SCOPES が offline_access になっている
5. DB migration 0004 が適用され、oauth_user_grants table が存在する
```

DevTools の Network で `/api/v1/integrations/zitadel/callback` の URL を見たとき、`code=` と `state=` が付いているのに `delegated_callback_failed` になる場合は、backend の code exchange 後に失敗しています。Phase 4 では特に Refresh Token 未有効化による `refresh_token` 欠落を疑ってください。

---

## Phase 4 Exact Snapshot
Phase 0-2 の exact snapshot と Phase 3 の exact snapshot を先に使い、Phase 4 で変わった非生成 file だけをここで上書き / 追加します。
- この節の block は、現在の Phase 4 実装に合わせた **exact delta** です
- `backend/internal/db/*.go`, `openapi/openapi.yaml`, `frontend/src/api/generated/*` は `make gen` の生成物なので、この snapshot には再掲しません
- `db/schema.sql` は `make db-schema` の生成物ですが、`sqlc generate` の入力でもあるため、この snapshot には Phase 4 時点の内容を載せます
- `backend/web/dist/*` は frontend build artifact なので、この snapshot には含めません
- `go.work.sum` は tool 実行で再生成されることがありますが、この repo の正本には含めません

#### Clean worktree replay checklist

`../phase4-test` のような clean worktree で **この `TUTORIAL_ZITADEL.md` だけから Phase 4 の file / directory 構成へ戻す** 場合は、次の順に進めてください。Phase 4 の block は Phase 0-2 / Phase 3 の同名 file を上書きするため、最終状態が現在の Phase 4 実装になります。

```bash
python3 - <<'PY'
from pathlib import Path
import re

doc = Path("TUTORIAL_ZITADEL.md")
text = doc.read_text()

sections = [
    ("### Project Exact Files", "## Phase 3. External User Bearer API"),
    ("## Phase 3 Exact Snapshot", "## Phase 4."),
    ("## Phase 4 Exact Snapshot", "## Phase 5."),
]

files = {}
for start, end in sections:
    start_match = re.search(rf"^{re.escape(start)}", text, re.M)
    if not start_match:
        raise SystemExit(f"section not found: {start}")
    start_index = start_match.start()
    end_match = re.search(rf"^{re.escape(end)}", text[start_index:], re.M)
    end_index = -1 if not end_match else start_index + end_match.start()
    section = text[start_index:] if end_index == -1 else text[start_index:end_index]
    for path, body in re.findall(r'^#### `([^`]+)`\n\n```[^\n]*\n(.*?)\n```', section, re.M | re.S):
        files[path] = body.rstrip("\n") + "\n"

for path, body in files.items():
    target = Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(body)
    print(f"wrote {target}")
PY
```

その後、生成物と build artifact を現在の実装と同じ状態に戻します。`make gen` の前に `go test` すると、`backend/internal/db/downstream_grants.sql.go` などが無くて build が失敗します。

```bash
npm --prefix frontend install
make gen
go test ./backend/...
npm --prefix frontend run build
if docker compose version >/dev/null 2>&1; then
  docker compose --env-file dev/zitadel/.env.example -f dev/zitadel/docker-compose.yml config --quiet
else
  docker-compose --env-file dev/zitadel/.env.example -f dev/zitadel/docker-compose.yml config --quiet
fi
git diff --check
```

manual smoke の前には DB migration を Phase 4 まで適用してください。repo root の `.env` と `dev/zitadel/.env` は Git に入れません。

```bash
make db-up
```

Zitadel Console 内の redirect URI / application 設定 / role assignment は Git に入らない外部状態です。Phase 4 では Step 4.5 の delegated callback URI も追加してください。

#### `.env.example`

```dotenv
APP_NAME="HaoHao API"
APP_VERSION=0.1.0
HTTP_PORT=8080

APP_BASE_URL=http://127.0.0.1:8080
FRONTEND_BASE_URL=http://127.0.0.1:5173

DATABASE_URL=postgres://haohao:haohao@127.0.0.1:5432/haohao?sslmode=disable

AUTH_MODE=local
ZITADEL_ISSUER=
ZITADEL_CLIENT_ID=
ZITADEL_CLIENT_SECRET=
ZITADEL_REDIRECT_URI=http://127.0.0.1:8080/api/v1/auth/callback
ZITADEL_POST_LOGOUT_REDIRECT_URI=http://127.0.0.1:5173/login
ZITADEL_SCOPES="openid profile email"

REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=
REDIS_DB=0

SESSION_TTL=24h
LOGIN_STATE_TTL=10m

EXTERNAL_EXPECTED_AUDIENCE=haohao-external
EXTERNAL_REQUIRED_SCOPE_PREFIX=
EXTERNAL_REQUIRED_ROLE=external_api_user
EXTERNAL_ALLOWED_ORIGINS=

DOWNSTREAM_TOKEN_ENCRYPTION_KEY=
DOWNSTREAM_TOKEN_KEY_VERSION=1
DOWNSTREAM_REFRESH_TOKEN_TTL=2160h
DOWNSTREAM_ACCESS_TOKEN_SKEW=30s
DOWNSTREAM_DEFAULT_SCOPES=offline_access

COOKIE_SECURE=false
```

#### `backend/cmd/main/main.go`

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
	sessionService := service.NewSessionService(queries, sessionStore, cfg.AuthMode)
	authzService := service.NewAuthzService(pool, queries)

	var oidcLoginService *service.OIDCLoginService
	var delegationService *service.DelegationService
	var bearerVerifier *auth.BearerVerifier
	if cfg.AuthMode == "zitadel" {
		if cfg.ZitadelIssuer == "" || cfg.ZitadelClientID == "" || cfg.ZitadelClientSecret == "" {
			log.Fatal("ZITADEL_ISSUER, ZITADEL_CLIENT_ID, and ZITADEL_CLIENT_SECRET are required when AUTH_MODE=zitadel")
		}

		oidcClient, err := auth.NewOIDCClient(
			ctx,
			cfg.ZitadelIssuer,
			cfg.ZitadelClientID,
			cfg.ZitadelClientSecret,
			cfg.ZitadelRedirectURI,
			cfg.ZitadelScopes,
		)
		if err != nil {
			log.Fatal(err)
		}

		loginStateStore := auth.NewLoginStateStore(redisClient, cfg.LoginStateTTL)
		identityService := service.NewIdentityService(pool, queries)
		oidcLoginService = service.NewOIDCLoginService("zitadel", oidcClient, loginStateStore, identityService, authzService, sessionService)

		if cfg.DownstreamTokenEncryptionKey != "" {
			refreshTokenStore, err := auth.NewRefreshTokenStore(cfg.DownstreamTokenEncryptionKey, cfg.DownstreamTokenKeyVersion)
			if err != nil {
				log.Fatal(err)
			}

			delegatedOAuthClient, err := auth.NewDelegatedOAuthClient(ctx, cfg.ZitadelIssuer, cfg.ZitadelClientID, cfg.ZitadelClientSecret)
			if err != nil {
				log.Fatal(err)
			}

			delegationStateStore := auth.NewDelegationStateStore(redisClient, cfg.LoginStateTTL)
			delegationService = service.NewDelegationService(
				queries,
				delegatedOAuthClient,
				delegationStateStore,
				refreshTokenStore,
				cfg.AppBaseURL,
				cfg.DownstreamDefaultScopes,
				cfg.DownstreamRefreshTokenTTL,
				cfg.DownstreamAccessTokenSkew,
			)
		}
	}

	if cfg.ZitadelIssuer != "" {
		bearerVerifier, err = auth.NewBearerVerifier(ctx, cfg.ZitadelIssuer)
		if err != nil {
			log.Fatal(err)
		}
	}

	application := app.New(cfg, sessionService, oidcLoginService, delegationService, authzService, bearerVerifier)

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

#### `backend/cmd/openapi/main.go`

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

	application := app.New(cfg, nil, nil, nil, nil, nil)

	spec, err := application.API.OpenAPI().YAML()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(string(spec))
}
```

#### `backend/internal/api/integrations.go`

```go
package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type IntegrationStatusBody struct {
	ResourceServer  string     `json:"resourceServer" example:"zitadel"`
	Provider        string     `json:"provider" example:"zitadel"`
	Connected       bool       `json:"connected" example:"true"`
	Scopes          []string   `json:"scopes,omitempty" example:"offline_access"`
	GrantedAt       *time.Time `json:"grantedAt,omitempty" format:"date-time"`
	LastRefreshedAt *time.Time `json:"lastRefreshedAt,omitempty" format:"date-time"`
	RevokedAt       *time.Time `json:"revokedAt,omitempty" format:"date-time"`
	LastErrorCode   string     `json:"lastErrorCode,omitempty" example:"invalid_grant"`
}

type ListIntegrationsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
}

type ListIntegrationsBody struct {
	Items []IntegrationStatusBody `json:"items"`
}

type ListIntegrationsOutput struct {
	Body ListIntegrationsBody
}

type ConnectIntegrationInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	ResourceServer string      `path:"resourceServer" example:"zitadel"`
}

type ConnectIntegrationOutput struct {
	Location string `header:"Location"`
}

type IntegrationCallbackInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	ResourceServer   string      `path:"resourceServer" example:"zitadel"`
	Code             string      `query:"code"`
	State            string      `query:"state"`
	Error            string      `query:"error"`
	ErrorDescription string      `query:"error_description"`
}

type IntegrationCallbackOutput struct {
	Location string `header:"Location"`
}

type VerifyIntegrationInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	ResourceServer string      `path:"resourceServer" example:"zitadel"`
}

type VerifyIntegrationBody struct {
	ResourceServer  string     `json:"resourceServer" example:"zitadel"`
	Connected       bool       `json:"connected" example:"true"`
	Scopes          []string   `json:"scopes,omitempty" example:"offline_access"`
	AccessExpiresAt *time.Time `json:"accessExpiresAt,omitempty" format:"date-time"`
	RefreshedAt     *time.Time `json:"refreshedAt,omitempty" format:"date-time"`
}

type VerifyIntegrationOutput struct {
	Body VerifyIntegrationBody
}

type DeleteIntegrationGrantInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	ResourceServer string      `path:"resourceServer" example:"zitadel"`
}

type DeleteIntegrationGrantOutput struct{}

func registerIntegrationRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listIntegrations",
		Method:      http.MethodGet,
		Path:        "/api/v1/integrations",
		Summary:     "downstream integration の接続状態を返す",
		Tags:        []string{"integrations"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ListIntegrationsInput) (*ListIntegrationsOutput, error) {
		if deps.DelegationService == nil {
			return nil, huma.Error503ServiceUnavailable("delegated auth is not configured")
		}

		user, err := deps.SessionService.CurrentUser(ctx, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPError(err)
		}

		statuses, err := deps.DelegationService.ListIntegrations(ctx, user)
		if err != nil {
			return nil, toDelegationHTTPError(err)
		}

		out := &ListIntegrationsOutput{}
		out.Body.Items = make([]IntegrationStatusBody, 0, len(statuses))
		for _, status := range statuses {
			out.Body.Items = append(out.Body.Items, toIntegrationStatusBody(status))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "connectIntegration",
		Method:        http.MethodGet,
		Path:          "/api/v1/integrations/{resourceServer}/connect",
		Summary:       "downstream integration consent を開始する",
		Tags:          []string{"integrations"},
		DefaultStatus: http.StatusFound,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ConnectIntegrationInput) (*ConnectIntegrationOutput, error) {
		if deps.DelegationService == nil {
			return nil, huma.Error503ServiceUnavailable("delegated auth is not configured")
		}

		user, err := deps.SessionService.CurrentUser(ctx, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPError(err)
		}

		location, err := deps.DelegationService.StartConnect(ctx, user, input.SessionCookie.Value, input.ResourceServer)
		if err != nil {
			return nil, toDelegationHTTPError(err)
		}

		return &ConnectIntegrationOutput{Location: location}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "finishIntegrationConnect",
		Method:        http.MethodGet,
		Path:          "/api/v1/integrations/{resourceServer}/callback",
		Summary:       "downstream integration consent callback を完了する",
		Tags:          []string{"integrations"},
		DefaultStatus: http.StatusFound,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *IntegrationCallbackInput) (*IntegrationCallbackOutput, error) {
		if input.Error != "" || deps.DelegationService == nil {
			return &IntegrationCallbackOutput{
				Location: integrationRedirect(deps.FrontendBaseURL, "error", "delegated_callback_failed"),
			}, nil
		}

		user, err := deps.SessionService.CurrentUser(ctx, input.SessionCookie.Value)
		if err != nil {
			return &IntegrationCallbackOutput{
				Location: integrationRedirect(deps.FrontendBaseURL, "error", "missing_session"),
			}, nil
		}

		if _, err := deps.DelegationService.SaveGrantFromCallback(ctx, user, input.SessionCookie.Value, input.ResourceServer, input.Code, input.State); err != nil {
			return &IntegrationCallbackOutput{
				Location: integrationRedirect(deps.FrontendBaseURL, "error", "delegated_callback_failed"),
			}, nil
		}

		return &IntegrationCallbackOutput{
			Location: integrationRedirect(deps.FrontendBaseURL, "connected", normalizeIntegrationResource(input.ResourceServer)),
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "verifyIntegrationAccess",
		Method:      http.MethodPost,
		Path:        "/api/v1/integrations/{resourceServer}/verify",
		Summary:     "downstream access token を backend 内で取得できるか検証する",
		Tags:        []string{"integrations"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *VerifyIntegrationInput) (*VerifyIntegrationOutput, error) {
		if deps.DelegationService == nil {
			return nil, huma.Error503ServiceUnavailable("delegated auth is not configured")
		}

		user, err := deps.SessionService.CurrentUserWithCSRF(ctx, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, toHTTPError(err)
		}

		result, err := deps.DelegationService.VerifyAccessToken(ctx, user, input.ResourceServer)
		if err != nil {
			return nil, toDelegationHTTPError(err)
		}

		out := &VerifyIntegrationOutput{}
		out.Body.ResourceServer = result.ResourceServer
		out.Body.Connected = result.Connected
		out.Body.Scopes = result.Scopes
		out.Body.AccessExpiresAt = result.AccessExpiresAt
		out.Body.RefreshedAt = result.RefreshedAt
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteIntegrationGrant",
		Method:        http.MethodDelete,
		Path:          "/api/v1/integrations/{resourceServer}/grant",
		Summary:       "downstream integration grant を削除する",
		Tags:          []string{"integrations"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *DeleteIntegrationGrantInput) (*DeleteIntegrationGrantOutput, error) {
		if deps.DelegationService == nil {
			return nil, huma.Error503ServiceUnavailable("delegated auth is not configured")
		}

		user, err := deps.SessionService.CurrentUserWithCSRF(ctx, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, toHTTPError(err)
		}

		if err := deps.DelegationService.DeleteGrant(ctx, user, input.ResourceServer); err != nil {
			return nil, toDelegationHTTPError(err)
		}

		return &DeleteIntegrationGrantOutput{}, nil
	})
}

func toIntegrationStatusBody(status service.DelegationStatus) IntegrationStatusBody {
	return IntegrationStatusBody{
		ResourceServer:  status.ResourceServer,
		Provider:        status.Provider,
		Connected:       status.Connected,
		Scopes:          status.Scopes,
		GrantedAt:       status.GrantedAt,
		LastRefreshedAt: status.LastRefreshedAt,
		RevokedAt:       status.RevokedAt,
		LastErrorCode:   status.LastErrorCode,
	}
}

func toDelegationHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrDelegationNotConfigured):
		return huma.Error503ServiceUnavailable("delegated auth is not configured")
	case errors.Is(err, service.ErrDelegationUnsupportedResource):
		return huma.Error404NotFound("unsupported downstream resource")
	case errors.Is(err, service.ErrDelegationGrantNotFound):
		return huma.Error404NotFound("delegated grant not found")
	case errors.Is(err, service.ErrDelegationInvalidState):
		return huma.Error400BadRequest("invalid delegated auth state")
	case errors.Is(err, service.ErrDelegationIdentityNotFound):
		return huma.Error409Conflict("zitadel identity is required before connecting the integration")
	case errors.Is(err, service.ErrDelegationRefreshTokenMissing):
		return huma.Error502BadGateway("provider did not return a refresh token")
	default:
		return huma.Error500InternalServerError("internal server error")
	}
}

func integrationRedirect(frontendBaseURL, key, value string) string {
	base := strings.TrimRight(frontendBaseURL, "/")
	query := url.Values{}
	query.Set(key, value)
	return base + "/integrations?" + query.Encode()
}

func normalizeIntegrationResource(resourceServer string) string {
	return strings.ToLower(strings.TrimSpace(resourceServer))
}
```

#### `backend/internal/api/register.go`

```go
package api

import (
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type Dependencies struct {
	SessionService               *service.SessionService
	OIDCLoginService             *service.OIDCLoginService
	DelegationService            *service.DelegationService
	AuthMode                     string
	FrontendBaseURL              string
	ZitadelIssuer                string
	ZitadelClientID              string
	ZitadelPostLogoutRedirectURI string
	CookieSecure                 bool
	SessionTTL                   time.Duration
}

func Register(api huma.API, deps Dependencies) {
	registerAuthSettingsRoute(api, deps)
	registerOIDCRoutes(api, deps)
	registerSessionRoutes(api, deps)
	registerExternalRoutes(api, deps)
	registerIntegrationRoutes(api, deps)
}
```

#### `backend/internal/app/app.go`

```go
package app

import (
	backendapi "example.com/haohao/backend/internal/api"
	"example.com/haohao/backend/internal/auth"
	"example.com/haohao/backend/internal/config"
	"example.com/haohao/backend/internal/middleware"
	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
)

type App struct {
	Router *gin.Engine
	API    huma.API
}

func New(cfg config.Config, sessionService *service.SessionService, oidcLoginService *service.OIDCLoginService, delegationService *service.DelegationService, authzService *service.AuthzService, bearerVerifier *auth.BearerVerifier) *App {
	router := gin.New()
	router.Use(
		gin.Logger(),
		gin.Recovery(),
		middleware.ExternalCORS("/api/external/", cfg.ExternalAllowedOrigins),
		middleware.ExternalAuth("/api/external/", bearerVerifier, authzService, "zitadel", cfg.ExternalExpectedAudience, cfg.ExternalRequiredScopePrefix, cfg.ExternalRequiredRole),
	)

	humaConfig := huma.DefaultConfig(cfg.AppName, cfg.AppVersion)
	humaConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"cookieAuth": {
			Type: "apiKey",
			In:   "cookie",
			Name: auth.SessionCookieName,
		},
		"bearerAuth": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		},
	}

	api := humagin.New(router, humaConfig)

	backendapi.Register(api, backendapi.Dependencies{
		SessionService:               sessionService,
		OIDCLoginService:             oidcLoginService,
		DelegationService:            delegationService,
		AuthMode:                     cfg.AuthMode,
		FrontendBaseURL:              cfg.FrontendBaseURL,
		ZitadelIssuer:                cfg.ZitadelIssuer,
		ZitadelClientID:              cfg.ZitadelClientID,
		ZitadelPostLogoutRedirectURI: cfg.ZitadelPostLogoutRedirectURI,
		CookieSecure:                 cfg.CookieSecure,
		SessionTTL:                   cfg.SessionTTL,
	})

	return &App{
		Router: router,
		API:    api,
	}
}
```

#### `backend/internal/auth/delegated_oauth_client.go`

```go
package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type DelegatedOAuthClient struct {
	clientID           string
	clientSecret       string
	endpoint           oauth2.Endpoint
	revocationEndpoint string
}

type DelegatedToken struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
	Scopes       []string
}

func NewDelegatedOAuthClient(ctx context.Context, issuer, clientID, clientSecret string) (*DelegatedOAuthClient, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover delegated oauth provider: %w", err)
	}

	var metadata struct {
		RevocationEndpoint string `json:"revocation_endpoint"`
	}
	if err := provider.Claims(&metadata); err != nil {
		return nil, fmt.Errorf("decode delegated oauth provider metadata: %w", err)
	}

	return &DelegatedOAuthClient{
		clientID:           clientID,
		clientSecret:       clientSecret,
		endpoint:           provider.Endpoint(),
		revocationEndpoint: metadata.RevocationEndpoint,
	}, nil
}

func (c *DelegatedOAuthClient) AuthorizeURL(state, codeVerifier, redirectURI string, scopes []string) string {
	return c.config(redirectURI, scopes).AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("code_challenge", pkceS256(codeVerifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

func (c *DelegatedOAuthClient) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI string, scopes []string) (DelegatedToken, error) {
	token, err := c.config(redirectURI, scopes).Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return DelegatedToken{}, fmt.Errorf("exchange delegated authorization code: %w", err)
	}

	return delegatedTokenFromOAuth2(token, scopes), nil
}

func (c *DelegatedOAuthClient) Refresh(ctx context.Context, refreshToken, redirectURI string, scopes []string) (DelegatedToken, error) {
	token, err := c.config(redirectURI, scopes).TokenSource(ctx, &oauth2.Token{
		RefreshToken: refreshToken,
	}).Token()
	if err != nil {
		return DelegatedToken{}, fmt.Errorf("refresh delegated access token: %w", err)
	}

	return delegatedTokenFromOAuth2(token, scopes), nil
}

func (c *DelegatedOAuthClient) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	if c == nil || c.revocationEndpoint == "" || refreshToken == "" {
		return nil
	}

	form := url.Values{}
	form.Set("token", refreshToken)
	form.Set("token_type_hint", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.revocationEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build token revocation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.clientID, c.clientSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("revoke delegated refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("revoke delegated refresh token: provider returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func IsInvalidGrantError(err error) bool {
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) && retrieveErr.ErrorCode == "invalid_grant" {
		return true
	}
	return err != nil && strings.Contains(err.Error(), "invalid_grant")
}

func (c *DelegatedOAuthClient) config(redirectURI string, scopes []string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.clientID,
		ClientSecret: c.clientSecret,
		Endpoint:     c.endpoint,
		RedirectURL:  redirectURI,
		Scopes:       scopes,
	}
}

func delegatedTokenFromOAuth2(token *oauth2.Token, fallbackScopes []string) DelegatedToken {
	scopes := fallbackScopes
	if rawScope, ok := token.Extra("scope").(string); ok && strings.TrimSpace(rawScope) != "" {
		scopes = strings.Fields(rawScope)
	}

	return DelegatedToken{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry,
		Scopes:       scopes,
	}
}
```

#### `backend/internal/auth/delegation_state_store.go`

```go
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrDelegationStateNotFound = errors.New("delegation state not found")

type DelegationStateRecord struct {
	UserID         int64  `json:"userId"`
	ResourceServer string `json:"resourceServer"`
	CodeVerifier   string `json:"codeVerifier"`
	SessionHash    string `json:"sessionHash"`
}

type DelegationStateStore struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

func NewDelegationStateStore(client *redis.Client, ttl time.Duration) *DelegationStateStore {
	return &DelegationStateStore{
		client: client,
		prefix: "delegation-state:",
		ttl:    ttl,
	}
}

func (s *DelegationStateStore) Create(ctx context.Context, userID int64, resourceServer, sessionHash string) (string, DelegationStateRecord, error) {
	state, err := randomToken(32)
	if err != nil {
		return "", DelegationStateRecord{}, err
	}

	codeVerifier, err := randomToken(32)
	if err != nil {
		return "", DelegationStateRecord{}, err
	}

	record := DelegationStateRecord{
		UserID:         userID,
		ResourceServer: resourceServer,
		CodeVerifier:   codeVerifier,
		SessionHash:    sessionHash,
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return "", DelegationStateRecord{}, err
	}

	if err := s.client.Set(ctx, s.prefix+state, payload, s.ttl).Err(); err != nil {
		return "", DelegationStateRecord{}, fmt.Errorf("save delegation state: %w", err)
	}

	return state, record, nil
}

func (s *DelegationStateStore) Consume(ctx context.Context, state string) (DelegationStateRecord, error) {
	raw, err := s.client.GetDel(ctx, s.prefix+state).Bytes()
	if errors.Is(err, redis.Nil) {
		return DelegationStateRecord{}, ErrDelegationStateNotFound
	}
	if err != nil {
		return DelegationStateRecord{}, fmt.Errorf("consume delegation state: %w", err)
	}

	var record DelegationStateRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return DelegationStateRecord{}, fmt.Errorf("decode delegation state: %w", err)
	}

	return record, nil
}
```

#### `backend/internal/auth/refresh_token_store.go`

```go
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

var (
	ErrTokenEncryptionKeyNotConfigured = errors.New("token encryption key is not configured")
	ErrInvalidTokenEncryptionKey       = errors.New("invalid token encryption key")
	ErrUnsupportedTokenKeyVersion      = errors.New("unsupported token key version")
)

type RefreshTokenStore struct {
	aead       cipher.AEAD
	keyVersion int32
}

func NewRefreshTokenStore(encodedKey string, keyVersion int) (*RefreshTokenStore, error) {
	if encodedKey == "" {
		return nil, ErrTokenEncryptionKeyNotConfigured
	}
	if keyVersion < 1 {
		return nil, fmt.Errorf("%w: key version must be positive", ErrInvalidTokenEncryptionKey)
	}

	key, err := decodeTokenEncryptionKey(encodedKey)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTokenEncryptionKey, err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTokenEncryptionKey, err)
	}

	return &RefreshTokenStore{
		aead:       aead,
		keyVersion: int32(keyVersion),
	}, nil
}

func (s *RefreshTokenStore) KeyVersion() int32 {
	return s.keyVersion
}

func (s *RefreshTokenStore) Encrypt(plaintext string) ([]byte, int32, error) {
	if s == nil || s.aead == nil {
		return nil, 0, ErrTokenEncryptionKeyNotConfigured
	}
	if plaintext == "" {
		return nil, 0, errors.New("refresh token is empty")
	}

	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, 0, fmt.Errorf("generate token nonce: %w", err)
	}

	ciphertext := s.aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := make([]byte, 0, len(nonce)+len(ciphertext))
	payload = append(payload, nonce...)
	payload = append(payload, ciphertext...)

	return payload, s.keyVersion, nil
}

func (s *RefreshTokenStore) Decrypt(ciphertext []byte, keyVersion int32) (string, error) {
	if s == nil || s.aead == nil {
		return "", ErrTokenEncryptionKeyNotConfigured
	}
	if keyVersion != s.keyVersion {
		return "", ErrUnsupportedTokenKeyVersion
	}
	if len(ciphertext) <= s.aead.NonceSize() {
		return "", errors.New("refresh token ciphertext is malformed")
	}

	nonce := ciphertext[:s.aead.NonceSize()]
	payload := ciphertext[s.aead.NonceSize():]

	plaintext, err := s.aead.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt refresh token: %w", err)
	}

	return string(plaintext), nil
}

func decodeTokenEncryptionKey(encoded string) ([]byte, error) {
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}

	var decoded []byte
	var err error
	for _, encoding := range encodings {
		decoded, err = encoding.DecodeString(encoded)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("%w: expected base64", ErrInvalidTokenEncryptionKey)
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("%w: expected 32 bytes, got %d", ErrInvalidTokenEncryptionKey, len(decoded))
	}

	return decoded, nil
}
```

#### `backend/internal/auth/refresh_token_store_test.go`

```go
package auth

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"
)

func TestRefreshTokenStoreEncryptDecrypt(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	store, err := NewRefreshTokenStore(key, 3)
	if err != nil {
		t.Fatalf("NewRefreshTokenStore() error = %v", err)
	}

	ciphertext, keyVersion, err := store.Encrypt("refresh-token")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if keyVersion != 3 {
		t.Fatalf("Encrypt() keyVersion = %d, want 3", keyVersion)
	}
	if bytes.Contains(ciphertext, []byte("refresh-token")) {
		t.Fatal("ciphertext contains plaintext token")
	}

	plaintext, err := store.Decrypt(ciphertext, keyVersion)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plaintext != "refresh-token" {
		t.Fatalf("Decrypt() = %q, want refresh-token", plaintext)
	}
}

func TestRefreshTokenStoreRejectsWrongKeyVersion(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))
	store, err := NewRefreshTokenStore(key, 1)
	if err != nil {
		t.Fatalf("NewRefreshTokenStore() error = %v", err)
	}

	ciphertext, _, err := store.Encrypt("refresh-token")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	_, err = store.Decrypt(ciphertext, 2)
	if !errors.Is(err, ErrUnsupportedTokenKeyVersion) {
		t.Fatalf("Decrypt() error = %v, want ErrUnsupportedTokenKeyVersion", err)
	}
}

func TestRefreshTokenStoreRejectsInvalidKey(t *testing.T) {
	_, err := NewRefreshTokenStore(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 31)), 1)
	if !errors.Is(err, ErrInvalidTokenEncryptionKey) {
		t.Fatalf("NewRefreshTokenStore() error = %v, want ErrInvalidTokenEncryptionKey", err)
	}
}
```

#### `backend/internal/config/config.go`

```go
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName                      string
	AppVersion                   string
	HTTPPort                     int
	AppBaseURL                   string
	FrontendBaseURL              string
	DatabaseURL                  string
	AuthMode                     string
	ZitadelIssuer                string
	ZitadelClientID              string
	ZitadelClientSecret          string
	ZitadelRedirectURI           string
	ZitadelPostLogoutRedirectURI string
	ZitadelScopes                string
	ExternalExpectedAudience     string
	ExternalRequiredScopePrefix  string
	ExternalRequiredRole         string
	ExternalAllowedOrigins       []string
	DownstreamTokenEncryptionKey string
	DownstreamTokenKeyVersion    int
	DownstreamRefreshTokenTTL    time.Duration
	DownstreamAccessTokenSkew    time.Duration
	DownstreamDefaultScopes      string
	RedisAddr                    string
	RedisPassword                string
	RedisDB                      int
	LoginStateTTL                time.Duration
	SessionTTL                   time.Duration
	CookieSecure                 bool
}

func Load() (Config, error) {
	sessionTTL, err := time.ParseDuration(getEnv("SESSION_TTL", "24h"))
	if err != nil {
		return Config{}, err
	}
	loginStateTTL, err := time.ParseDuration(getEnv("LOGIN_STATE_TTL", "10m"))
	if err != nil {
		return Config{}, err
	}
	downstreamRefreshTokenTTL, err := time.ParseDuration(getEnv("DOWNSTREAM_REFRESH_TOKEN_TTL", "2160h"))
	if err != nil {
		return Config{}, err
	}
	downstreamAccessTokenSkew, err := time.ParseDuration(getEnv("DOWNSTREAM_ACCESS_TOKEN_SKEW", "30s"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppName:                      getEnv("APP_NAME", "HaoHao API"),
		AppVersion:                   getEnv("APP_VERSION", "0.1.0"),
		HTTPPort:                     getEnvInt("HTTP_PORT", 8080),
		AppBaseURL:                   strings.TrimRight(getEnv("APP_BASE_URL", "http://127.0.0.1:8080"), "/"),
		FrontendBaseURL:              strings.TrimRight(getEnv("FRONTEND_BASE_URL", "http://127.0.0.1:5173"), "/"),
		DatabaseURL:                  getEnv("DATABASE_URL", ""),
		AuthMode:                     getEnv("AUTH_MODE", "local"),
		ZitadelIssuer:                strings.TrimRight(getEnv("ZITADEL_ISSUER", ""), "/"),
		ZitadelClientID:              getEnv("ZITADEL_CLIENT_ID", ""),
		ZitadelClientSecret:          getEnv("ZITADEL_CLIENT_SECRET", ""),
		ZitadelRedirectURI:           getEnv("ZITADEL_REDIRECT_URI", "http://127.0.0.1:8080/api/v1/auth/callback"),
		ZitadelPostLogoutRedirectURI: getEnv("ZITADEL_POST_LOGOUT_REDIRECT_URI", "http://127.0.0.1:5173/login"),
		ZitadelScopes:                getEnv("ZITADEL_SCOPES", "openid profile email"),
		ExternalExpectedAudience:     getEnv("EXTERNAL_EXPECTED_AUDIENCE", "haohao-external"),
		ExternalRequiredScopePrefix:  getEnv("EXTERNAL_REQUIRED_SCOPE_PREFIX", ""),
		ExternalRequiredRole:         getEnv("EXTERNAL_REQUIRED_ROLE", "external_api_user"),
		ExternalAllowedOrigins:       getEnvCSV("EXTERNAL_ALLOWED_ORIGINS"),
		DownstreamTokenEncryptionKey: getEnv("DOWNSTREAM_TOKEN_ENCRYPTION_KEY", ""),
		DownstreamTokenKeyVersion:    getEnvInt("DOWNSTREAM_TOKEN_KEY_VERSION", 1),
		DownstreamRefreshTokenTTL:    downstreamRefreshTokenTTL,
		DownstreamAccessTokenSkew:    downstreamAccessTokenSkew,
		DownstreamDefaultScopes:      getEnv("DOWNSTREAM_DEFAULT_SCOPES", "offline_access"),
		RedisAddr:                    getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:                getEnv("REDIS_PASSWORD", ""),
		RedisDB:                      getEnvInt("REDIS_DB", 0),
		LoginStateTTL:                loginStateTTL,
		SessionTTL:                   sessionTTL,
		CookieSecure:                 getEnvBool("COOKIE_SECURE", false),
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

func getEnvCSV(key string) []string {
	value := strings.TrimSpace(getEnv(key, ""))
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}

	return items
}
```

#### `backend/internal/service/delegation_service.go`

```go
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"example.com/haohao/backend/internal/auth"
	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrDelegationNotConfigured       = errors.New("delegated auth is not configured")
	ErrDelegationUnsupportedResource = errors.New("unsupported downstream resource")
	ErrDelegationGrantNotFound       = errors.New("delegated grant not found")
	ErrDelegationInvalidState        = errors.New("invalid delegated auth state")
	ErrDelegationIdentityNotFound    = errors.New("delegated provider identity not found")
	ErrDelegationRefreshTokenMissing = errors.New("delegated refresh token missing")
)

type DelegationStatus struct {
	ResourceServer  string
	Provider        string
	Connected       bool
	Scopes          []string
	GrantedAt       *time.Time
	LastRefreshedAt *time.Time
	RevokedAt       *time.Time
	LastErrorCode   string
}

type DelegatedAccessToken struct {
	AccessToken string
	ExpiresAt   *time.Time
	Scopes      []string
}

type DelegationVerifyResult struct {
	ResourceServer  string
	Connected       bool
	Scopes          []string
	AccessExpiresAt *time.Time
	RefreshedAt     *time.Time
}

type delegationResource struct {
	resourceServer string
	provider       string
	redirectURI    string
	scopes         []string
}

type DelegationService struct {
	queries       *db.Queries
	oauthClient   *auth.DelegatedOAuthClient
	stateStore    *auth.DelegationStateStore
	tokenStore    *auth.RefreshTokenStore
	appBaseURL    string
	defaultScopes []string
	refreshTTL    time.Duration
	accessSkew    time.Duration
}

func NewDelegationService(queries *db.Queries, oauthClient *auth.DelegatedOAuthClient, stateStore *auth.DelegationStateStore, tokenStore *auth.RefreshTokenStore, appBaseURL, defaultScopes string, refreshTTL, accessSkew time.Duration) *DelegationService {
	return &DelegationService{
		queries:       queries,
		oauthClient:   oauthClient,
		stateStore:    stateStore,
		tokenStore:    tokenStore,
		appBaseURL:    strings.TrimRight(appBaseURL, "/"),
		defaultScopes: normalizeScopeList(strings.Fields(defaultScopes)),
		refreshTTL:    refreshTTL,
		accessSkew:    accessSkew,
	}
}

func (s *DelegationService) ListIntegrations(ctx context.Context, user User) ([]DelegationStatus, error) {
	if err := s.requireConfigured(); err != nil {
		return nil, err
	}

	rows, err := s.queries.ListOAuthUserGrantsByUserID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("list downstream grants: %w", err)
	}

	byResource := make(map[string]db.ListOAuthUserGrantsByUserIDRow, len(rows))
	for _, row := range rows {
		if row.Provider == "zitadel" {
			byResource[row.ResourceServer] = row
		}
	}

	statuses := make([]DelegationStatus, 0, 1)
	for _, resourceServer := range []string{"zitadel"} {
		resource, err := s.resource(resourceServer)
		if err != nil {
			return nil, err
		}

		status := DelegationStatus{
			ResourceServer: resource.resourceServer,
			Provider:       resource.provider,
			Scopes:         resource.scopes,
		}
		if row, ok := byResource[resource.resourceServer]; ok {
			status.Connected = !row.RevokedAt.Valid
			status.Scopes = normalizeScopeText(row.ScopeText)
			status.GrantedAt = timeFromPg(row.GrantedAt)
			status.LastRefreshedAt = timeFromPg(row.LastRefreshedAt)
			status.RevokedAt = timeFromPg(row.RevokedAt)
			if row.LastErrorCode.Valid {
				status.LastErrorCode = row.LastErrorCode.String
			}
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

func (s *DelegationService) StartConnect(ctx context.Context, user User, sessionID, resourceServer string) (string, error) {
	if err := s.requireConfigured(); err != nil {
		return "", err
	}

	resource, err := s.resource(resourceServer)
	if err != nil {
		return "", err
	}

	state, record, err := s.stateStore.Create(ctx, user.ID, resource.resourceServer, hashSessionID(sessionID))
	if err != nil {
		return "", fmt.Errorf("create delegated auth state: %w", err)
	}

	return s.oauthClient.AuthorizeURL(state, record.CodeVerifier, resource.redirectURI, resource.scopes), nil
}

func (s *DelegationService) SaveGrantFromCallback(ctx context.Context, user User, sessionID, resourceServer, code, state string) (DelegationStatus, error) {
	if err := s.requireConfigured(); err != nil {
		return DelegationStatus{}, err
	}

	resource, err := s.resource(resourceServer)
	if err != nil {
		return DelegationStatus{}, err
	}

	record, err := s.stateStore.Consume(ctx, state)
	if err != nil {
		return DelegationStatus{}, fmt.Errorf("%w: %v", ErrDelegationInvalidState, err)
	}
	if record.UserID != user.ID || record.ResourceServer != resource.resourceServer || record.SessionHash != hashSessionID(sessionID) {
		return DelegationStatus{}, ErrDelegationInvalidState
	}

	identity, err := s.queries.GetUserIdentityByUserIDProvider(ctx, db.GetUserIdentityByUserIDProviderParams{
		UserID:   user.ID,
		Provider: resource.provider,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DelegationStatus{}, ErrDelegationIdentityNotFound
	}
	if err != nil {
		return DelegationStatus{}, fmt.Errorf("load delegated provider identity: %w", err)
	}

	token, err := s.oauthClient.ExchangeCode(ctx, code, record.CodeVerifier, resource.redirectURI, resource.scopes)
	if err != nil {
		return DelegationStatus{}, err
	}
	if token.RefreshToken == "" {
		return DelegationStatus{}, ErrDelegationRefreshTokenMissing
	}

	ciphertext, keyVersion, err := s.tokenStore.Encrypt(token.RefreshToken)
	if err != nil {
		return DelegationStatus{}, fmt.Errorf("encrypt delegated refresh token: %w", err)
	}

	row, err := s.queries.UpsertOAuthUserGrant(ctx, db.UpsertOAuthUserGrantParams{
		UserID:                 user.ID,
		Provider:               resource.provider,
		ResourceServer:         resource.resourceServer,
		ProviderSubject:        identity.Subject,
		RefreshTokenCiphertext: ciphertext,
		RefreshTokenKeyVersion: keyVersion,
		ScopeText:              scopeText(token.Scopes),
		GrantedBySessionID:     hashSessionID(sessionID),
	})
	if err != nil {
		return DelegationStatus{}, fmt.Errorf("save delegated grant: %w", err)
	}

	return grantStatusFromRow(row), nil
}

func (s *DelegationService) GetAccessToken(ctx context.Context, user User, resourceServer string) (DelegatedAccessToken, error) {
	if err := s.requireConfigured(); err != nil {
		return DelegatedAccessToken{}, err
	}

	resource, err := s.resource(resourceServer)
	if err != nil {
		return DelegatedAccessToken{}, err
	}

	grant, err := s.queries.GetActiveOAuthUserGrant(ctx, db.GetActiveOAuthUserGrantParams{
		UserID:         user.ID,
		Provider:       resource.provider,
		ResourceServer: resource.resourceServer,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DelegatedAccessToken{}, ErrDelegationGrantNotFound
	}
	if err != nil {
		return DelegatedAccessToken{}, fmt.Errorf("load delegated grant: %w", err)
	}
	if s.refreshTokenExpired(grant) {
		_ = s.queries.MarkOAuthUserGrantRevoked(ctx, db.MarkOAuthUserGrantRevokedParams{
			UserID:         user.ID,
			Provider:       resource.provider,
			ResourceServer: resource.resourceServer,
			LastErrorCode:  pgtype.Text{String: "refresh_token_expired", Valid: true},
		})
		return DelegatedAccessToken{}, ErrDelegationGrantNotFound
	}

	refreshToken, err := s.tokenStore.Decrypt(grant.RefreshTokenCiphertext, grant.RefreshTokenKeyVersion)
	if err != nil {
		return DelegatedAccessToken{}, fmt.Errorf("decrypt delegated refresh token: %w", err)
	}

	token, err := s.oauthClient.Refresh(ctx, refreshToken, resource.redirectURI, normalizeScopeText(grant.ScopeText))
	if err != nil {
		if auth.IsInvalidGrantError(err) {
			_ = s.queries.MarkOAuthUserGrantRevoked(ctx, db.MarkOAuthUserGrantRevokedParams{
				UserID:         user.ID,
				Provider:       resource.provider,
				ResourceServer: resource.resourceServer,
				LastErrorCode:  pgtype.Text{String: "invalid_grant", Valid: true},
			})
			return DelegatedAccessToken{}, ErrDelegationGrantNotFound
		}
		return DelegatedAccessToken{}, err
	}

	nextRefreshToken := token.RefreshToken
	if nextRefreshToken == "" {
		nextRefreshToken = refreshToken
	}

	ciphertext, keyVersion, err := s.tokenStore.Encrypt(nextRefreshToken)
	if err != nil {
		return DelegatedAccessToken{}, fmt.Errorf("encrypt rotated refresh token: %w", err)
	}

	scopes := token.Scopes
	if len(scopes) == 0 {
		scopes = normalizeScopeText(grant.ScopeText)
	}

	if _, err := s.queries.UpdateOAuthUserGrantAfterRefresh(ctx, db.UpdateOAuthUserGrantAfterRefreshParams{
		UserID:                 user.ID,
		Provider:               resource.provider,
		ResourceServer:         resource.resourceServer,
		RefreshTokenCiphertext: ciphertext,
		RefreshTokenKeyVersion: keyVersion,
		ScopeText:              scopeText(scopes),
	}); err != nil {
		return DelegatedAccessToken{}, fmt.Errorf("update delegated grant after refresh: %w", err)
	}

	return DelegatedAccessToken{
		AccessToken: token.AccessToken,
		ExpiresAt:   expiresWithSkew(token.Expiry, s.accessSkew),
		Scopes:      scopes,
	}, nil
}

func (s *DelegationService) VerifyAccessToken(ctx context.Context, user User, resourceServer string) (DelegationVerifyResult, error) {
	token, err := s.GetAccessToken(ctx, user, resourceServer)
	if err != nil {
		return DelegationVerifyResult{}, err
	}

	now := time.Now().UTC()
	return DelegationVerifyResult{
		ResourceServer:  normalizeResourceServer(resourceServer),
		Connected:       true,
		Scopes:          token.Scopes,
		AccessExpiresAt: token.ExpiresAt,
		RefreshedAt:     &now,
	}, nil
}

func (s *DelegationService) DeleteGrant(ctx context.Context, user User, resourceServer string) error {
	if err := s.requireConfigured(); err != nil {
		return err
	}

	resource, err := s.resource(resourceServer)
	if err != nil {
		return err
	}

	grant, err := s.queries.GetActiveOAuthUserGrant(ctx, db.GetActiveOAuthUserGrantParams{
		UserID:         user.ID,
		Provider:       resource.provider,
		ResourceServer: resource.resourceServer,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load delegated grant for revoke: %w", err)
	}
	if err == nil {
		refreshToken, err := s.tokenStore.Decrypt(grant.RefreshTokenCiphertext, grant.RefreshTokenKeyVersion)
		if err != nil {
			return fmt.Errorf("decrypt delegated refresh token for revoke: %w", err)
		}
		if err := s.oauthClient.RevokeRefreshToken(ctx, refreshToken); err != nil {
			return err
		}
	}

	if err := s.queries.DeleteOAuthUserGrant(ctx, db.DeleteOAuthUserGrantParams{
		UserID:         user.ID,
		Provider:       resource.provider,
		ResourceServer: resource.resourceServer,
	}); err != nil {
		return fmt.Errorf("delete delegated grant: %w", err)
	}

	return nil
}

func (s *DelegationService) requireConfigured() error {
	if s == nil || s.queries == nil || s.oauthClient == nil || s.stateStore == nil || s.tokenStore == nil || s.appBaseURL == "" {
		return ErrDelegationNotConfigured
	}
	return nil
}

func (s *DelegationService) resource(resourceServer string) (delegationResource, error) {
	normalized := normalizeResourceServer(resourceServer)
	if normalized != "zitadel" {
		return delegationResource{}, ErrDelegationUnsupportedResource
	}

	scopes := s.defaultScopes
	if len(scopes) == 0 {
		scopes = []string{"offline_access"}
	}

	return delegationResource{
		resourceServer: "zitadel",
		provider:       "zitadel",
		redirectURI:    s.appBaseURL + "/api/v1/integrations/zitadel/callback",
		scopes:         scopes,
	}, nil
}

func normalizeResourceServer(resourceServer string) string {
	return strings.ToLower(strings.TrimSpace(resourceServer))
}

func hashSessionID(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return hex.EncodeToString(sum[:])
}

func scopeText(scopes []string) string {
	return strings.Join(normalizeScopeList(scopes), " ")
}

func normalizeScopeText(value string) []string {
	return normalizeScopeList(strings.Fields(value))
}

func normalizeScopeList(scopes []string) []string {
	set := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		trimmed := strings.TrimSpace(scope)
		if trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}

	normalized := make([]string, 0, len(set))
	for scope := range set {
		normalized = append(normalized, scope)
	}
	sort.Strings(normalized)
	return normalized
}

func timeFromPg(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

func expiresWithSkew(expiry time.Time, skew time.Duration) *time.Time {
	if expiry.IsZero() {
		return nil
	}
	expiresAt := expiry.Add(-skew).UTC()
	return &expiresAt
}

func (s *DelegationService) refreshTokenExpired(grant db.OauthUserGrant) bool {
	if s.refreshTTL <= 0 || !grant.GrantedAt.Valid {
		return false
	}

	base := grant.GrantedAt.Time
	if grant.LastRefreshedAt.Valid {
		base = grant.LastRefreshedAt.Time
	}

	return time.Now().After(base.Add(s.refreshTTL))
}

func grantStatusFromRow(row db.OauthUserGrant) DelegationStatus {
	return DelegationStatus{
		ResourceServer:  row.ResourceServer,
		Provider:        row.Provider,
		Connected:       !row.RevokedAt.Valid,
		Scopes:          normalizeScopeText(row.ScopeText),
		GrantedAt:       timeFromPg(row.GrantedAt),
		LastRefreshedAt: timeFromPg(row.LastRefreshedAt),
		RevokedAt:       timeFromPg(row.RevokedAt),
		LastErrorCode: func() string {
			if row.LastErrorCode.Valid {
				return row.LastErrorCode.String
			}
			return ""
		}(),
	}
}
```

#### `backend/internal/service/session_service.go`

```go
package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"

	"example.com/haohao/backend/internal/auth"
	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInvalidCSRFToken    = errors.New("invalid csrf token")
	ErrAuthModeUnsupported = errors.New("auth mode unsupported")
)

type User struct {
	ID          int64
	PublicID    string
	Email       string
	DisplayName string
}

type SessionService struct {
	queries  *db.Queries
	store    *auth.SessionStore
	authMode string
}

func NewSessionService(queries *db.Queries, store *auth.SessionStore, authMode string) *SessionService {
	return &SessionService{
		queries:  queries,
		store:    store,
		authMode: strings.ToLower(strings.TrimSpace(authMode)),
	}
}

func (s *SessionService) Login(ctx context.Context, email, password string) (User, string, string, error) {
	if s.authMode == "zitadel" {
		return User{}, "", "", ErrAuthModeUnsupported
	}

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

	sessionID, csrfToken, err := s.IssueSession(ctx, userID)
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

func (s *SessionService) CurrentUserWithCSRF(ctx context.Context, sessionID, csrfHeader string) (User, error) {
	session, err := s.store.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, err
	}

	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(csrfHeader)) != 1 {
		return User{}, ErrInvalidCSRFToken
	}

	return s.loadUserByID(ctx, session.UserID)
}

func (s *SessionService) IssueSession(ctx context.Context, userID int64) (string, string, error) {
	return s.IssueSessionWithProviderHint(ctx, userID, "")
}

func (s *SessionService) IssueSessionWithProviderHint(ctx context.Context, userID int64, providerIDTokenHint string) (string, string, error) {
	sessionID, csrfToken, err := s.store.CreateWithProviderHint(ctx, userID, providerIDTokenHint)
	if err != nil {
		return "", "", fmt.Errorf("create session: %w", err)
	}
	return sessionID, csrfToken, nil
}

func (s *SessionService) Logout(ctx context.Context, sessionID, csrfHeader string) (string, error) {
	session, err := s.store.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", ErrUnauthorized
	}
	if err != nil {
		return "", err
	}

	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(csrfHeader)) != 1 {
		return "", ErrInvalidCSRFToken
	}

	if err := s.store.Delete(ctx, sessionID); err != nil {
		return "", err
	}

	return session.ProviderIDTokenHint, nil
}

func (s *SessionService) ReissueCSRF(ctx context.Context, sessionID string) (string, error) {
	if _, err := s.CurrentUser(ctx, sessionID); err != nil {
		return "", err
	}

	csrfToken, err := s.store.ReissueCSRF(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", ErrUnauthorized
	}
	if err != nil {
		return "", err
	}

	return csrfToken, nil
}

func (s *SessionService) RefreshSession(ctx context.Context, sessionID, csrfHeader string) (string, string, error) {
	session, err := s.store.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", "", ErrUnauthorized
	}
	if err != nil {
		return "", "", err
	}

	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(csrfHeader)) != 1 {
		return "", "", ErrInvalidCSRFToken
	}

	newSessionID, newCSRFToken, err := s.store.Rotate(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", "", ErrUnauthorized
	}
	if err != nil {
		return "", "", err
	}

	return newSessionID, newCSRFToken, nil
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
```

#### `db/migrations/0004_downstream_grants.up.sql`

```sql
CREATE TABLE oauth_user_grants (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    resource_server TEXT NOT NULL,
    provider_subject TEXT NOT NULL,
    refresh_token_ciphertext BYTEA NOT NULL,
    refresh_token_key_version INTEGER NOT NULL,
    scope_text TEXT NOT NULL,
    granted_by_session_id TEXT NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_refreshed_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    last_error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, provider, resource_server)
);

CREATE INDEX oauth_user_grants_provider_subject_idx
    ON oauth_user_grants(provider, provider_subject);

CREATE INDEX oauth_user_grants_resource_server_idx
    ON oauth_user_grants(resource_server);
```

#### `db/migrations/0004_downstream_grants.down.sql`

```sql
DROP TABLE IF EXISTS oauth_user_grants;
```

#### `db/queries/downstream_grants.sql`

```sql
-- name: UpsertOAuthUserGrant :one
INSERT INTO oauth_user_grants (
    user_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
ON CONFLICT (user_id, provider, resource_server) DO UPDATE
SET provider_subject = EXCLUDED.provider_subject,
    refresh_token_ciphertext = EXCLUDED.refresh_token_ciphertext,
    refresh_token_key_version = EXCLUDED.refresh_token_key_version,
    scope_text = EXCLUDED.scope_text,
    granted_by_session_id = EXCLUDED.granted_by_session_id,
    granted_at = now(),
    last_refreshed_at = NULL,
    revoked_at = NULL,
    last_error_code = NULL,
    updated_at = now()
RETURNING
    id,
    user_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at;

-- name: GetOAuthUserGrant :one
SELECT
    id,
    user_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at
FROM oauth_user_grants
WHERE user_id = $1
  AND provider = $2
  AND resource_server = $3
LIMIT 1;

-- name: GetActiveOAuthUserGrant :one
SELECT
    id,
    user_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at
FROM oauth_user_grants
WHERE user_id = $1
  AND provider = $2
  AND resource_server = $3
  AND revoked_at IS NULL
LIMIT 1;

-- name: ListOAuthUserGrantsByUserID :many
SELECT
    id,
    user_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at
FROM oauth_user_grants
WHERE user_id = $1
ORDER BY resource_server, provider;

-- name: UpdateOAuthUserGrantAfterRefresh :one
UPDATE oauth_user_grants
SET refresh_token_ciphertext = $4,
    refresh_token_key_version = $5,
    scope_text = $6,
    last_refreshed_at = now(),
    last_error_code = NULL,
    updated_at = now()
WHERE user_id = $1
  AND provider = $2
  AND resource_server = $3
  AND revoked_at IS NULL
RETURNING
    id,
    user_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at;

-- name: MarkOAuthUserGrantRevoked :exec
UPDATE oauth_user_grants
SET revoked_at = now(),
    last_error_code = $4,
    updated_at = now()
WHERE user_id = $1
  AND provider = $2
  AND resource_server = $3;

-- name: DeleteOAuthUserGrant :exec
DELETE FROM oauth_user_grants
WHERE user_id = $1
  AND provider = $2
  AND resource_server = $3;
```

#### `db/queries/identities.sql`

```sql
-- name: GetUserByProviderSubject :one
SELECT
    u.id,
    u.public_id,
    u.email,
    u.display_name
FROM user_identities ui
JOIN users u ON u.id = ui.user_id
WHERE ui.provider = $1
  AND ui.subject = $2
LIMIT 1;

-- name: GetUserIdentityByUserIDProvider :one
SELECT
    id,
    user_id,
    provider,
    subject,
    email,
    email_verified,
    created_at,
    updated_at
FROM user_identities
WHERE user_id = $1
  AND provider = $2
LIMIT 1;

-- name: CreateUserIdentity :exec
INSERT INTO user_identities (
    user_id,
    provider,
    subject,
    email,
    email_verified
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
);

-- name: UpdateUserIdentityProfile :exec
UPDATE user_identities
SET email = $3,
    email_verified = $4,
    updated_at = now()
WHERE provider = $1
  AND subject = $2;
```

#### `db/schema.sql`

```sql
--
-- PostgreSQL database dump
--


-- Dumped from database version 18.3 (Debian 18.3-1.pgdg13+1)
-- Dumped by pg_dump version 18.3 (Debian 18.3-1.pgdg13+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: oauth_user_grants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.oauth_user_grants (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    provider text NOT NULL,
    resource_server text NOT NULL,
    provider_subject text NOT NULL,
    refresh_token_ciphertext bytea NOT NULL,
    refresh_token_key_version integer NOT NULL,
    scope_text text NOT NULL,
    granted_by_session_id text NOT NULL,
    granted_at timestamp with time zone DEFAULT now() NOT NULL,
    last_refreshed_at timestamp with time zone,
    revoked_at timestamp with time zone,
    last_error_code text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: oauth_user_grants_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.oauth_user_grants ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.oauth_user_grants_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.roles (
    id bigint NOT NULL,
    code text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: roles_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.roles ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.roles_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version bigint NOT NULL,
    dirty boolean NOT NULL
);


--
-- Name: user_identities; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_identities (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    provider text NOT NULL,
    subject text NOT NULL,
    email text NOT NULL,
    email_verified boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_identities_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.user_identities ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.user_identities_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: user_roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_roles (
    user_id bigint NOT NULL,
    role_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    email text NOT NULL,
    display_name text NOT NULL,
    password_hash text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.users ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: oauth_user_grants oauth_user_grants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_user_grants
    ADD CONSTRAINT oauth_user_grants_pkey PRIMARY KEY (id);


--
-- Name: oauth_user_grants oauth_user_grants_user_id_provider_resource_server_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_user_grants
    ADD CONSTRAINT oauth_user_grants_user_id_provider_resource_server_key UNIQUE (user_id, provider, resource_server);


--
-- Name: roles roles_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_code_key UNIQUE (code);


--
-- Name: roles roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: user_identities user_identities_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_identities
    ADD CONSTRAINT user_identities_pkey PRIMARY KEY (id);


--
-- Name: user_identities user_identities_provider_subject_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_identities
    ADD CONSTRAINT user_identities_provider_subject_key UNIQUE (provider, subject);


--
-- Name: user_roles user_roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_pkey PRIMARY KEY (user_id, role_id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: oauth_user_grants_provider_subject_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX oauth_user_grants_provider_subject_idx ON public.oauth_user_grants USING btree (provider, provider_subject);


--
-- Name: oauth_user_grants_resource_server_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX oauth_user_grants_resource_server_idx ON public.oauth_user_grants USING btree (resource_server);


--
-- Name: user_identities_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_identities_user_id_idx ON public.user_identities USING btree (user_id);


--
-- Name: user_roles_role_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_roles_role_id_idx ON public.user_roles USING btree (role_id);


--
-- Name: oauth_user_grants oauth_user_grants_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_user_grants
    ADD CONSTRAINT oauth_user_grants_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_identities user_identities_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_identities
    ADD CONSTRAINT user_identities_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_roles user_roles_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id) ON DELETE CASCADE;


--
-- Name: user_roles user_roles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--
```

#### `frontend/src/App.vue`

```vue
<script setup lang="ts">
import { computed } from 'vue'

import { useSessionStore } from './stores/session'

const sessionStore = useSessionStore()

const displayName = computed(() => sessionStore.user?.displayName ?? 'Guest')
const statusLabel = computed(() => {
  switch (sessionStore.status) {
    case 'authenticated':
      return 'Authenticated'
    case 'anonymous':
      return 'Anonymous'
    case 'loading':
      return 'Checking'
    default:
      return 'Idle'
  }
})
</script>

<template>
  <div class="app-shell">
    <header class="app-header">
      <div>
        <p class="eyebrow">Foundation Tutorial Build</p>
        <h1>HaoHao</h1>
        <nav class="app-nav" aria-label="Primary">
          <RouterLink to="/">Session</RouterLink>
          <RouterLink to="/integrations">Integrations</RouterLink>
        </nav>
      </div>

      <div class="identity-card">
        <span class="identity-label">Current identity</span>
        <strong>{{ displayName }}</strong>
        <span class="identity-status">{{ statusLabel }}</span>
      </div>
    </header>

    <main class="app-main">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  width: min(960px, calc(100vw - 32px));
  margin: 0 auto;
  padding: 40px 0 64px;
}

.app-header {
  display: flex;
  justify-content: space-between;
  align-items: end;
  gap: 24px;
  margin-bottom: 28px;
}

.eyebrow {
  margin: 0 0 10px;
  font-size: 0.78rem;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--muted);
}

h1 {
  margin: 0;
  font-size: clamp(2.5rem, 5vw, 4rem);
  line-height: 0.96;
}

.app-nav {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  margin-top: 18px;
}

.app-nav a {
  display: inline-flex;
  align-items: center;
  min-height: 36px;
  padding: 0 12px;
  border: 1px solid var(--border);
  border-radius: 999px;
  color: var(--muted);
  text-decoration: none;
}

.app-nav a.router-link-active {
  color: var(--accent-strong);
  background: rgba(11, 93, 91, 0.08);
}

.identity-card {
  min-width: 210px;
  padding: 14px 16px;
  border: 1px solid var(--border-strong);
  border-radius: 18px;
  background: rgba(248, 239, 227, 0.78);
  backdrop-filter: blur(12px);
}

.identity-card strong {
  display: block;
  color: var(--text-strong);
  font-size: 1.05rem;
}

.identity-label,
.identity-status {
  display: block;
  font-size: 0.76rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--muted);
}

.identity-label {
  margin-bottom: 6px;
}

.identity-status {
  margin-top: 8px;
}

@media (max-width: 720px) {
  .app-shell {
    width: min(100vw - 24px, 960px);
    padding-top: 24px;
  }

  .app-header {
    flex-direction: column;
    align-items: stretch;
  }

  .identity-card {
    min-width: 0;
  }
}
</style>
```

#### `frontend/src/api/integrations.ts`

```ts
import {
  deleteIntegrationGrant,
  listIntegrations,
  verifyIntegrationAccess,
} from './generated/sdk.gen'
import { readCookie } from './client'
import type { IntegrationStatusBody, VerifyIntegrationBody } from './generated/types.gen'

export async function fetchIntegrations(): Promise<IntegrationStatusBody[]> {
  const data = await listIntegrations({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items: IntegrationStatusBody[] | null }
  return data.items ?? []
}

export function startIntegrationConnect(resourceServer: string) {
  window.location.assign(`/api/v1/integrations/${encodeURIComponent(resourceServer)}/connect`)
}

export async function verifyIntegration(resourceServer: string): Promise<VerifyIntegrationBody> {
  return verifyIntegrationAccess({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    path: {
      resourceServer,
    },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<VerifyIntegrationBody>
}

export async function revokeIntegrationGrant(resourceServer: string): Promise<void> {
  await deleteIntegrationGrant({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    path: {
      resourceServer,
    },
    responseStyle: 'data',
    throwOnError: true,
  })
}
```

#### `frontend/src/router/index.ts`

```ts
import { createRouter, createWebHistory } from 'vue-router'

import { useSessionStore } from '../stores/session'
import HomeView from '../views/HomeView.vue'
import IntegrationsView from '../views/IntegrationsView.vue'
import LoginView from '../views/LoginView.vue'

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
    {
      path: '/integrations',
      name: 'integrations',
      component: IntegrationsView,
      meta: { requiresAuth: true },
    },
  ],
})

router.beforeEach(async (to) => {
  const sessionStore = useSessionStore()
  await sessionStore.bootstrap()

  if (to.meta.requiresAuth && sessionStore.status !== 'authenticated') {
    return { name: 'login' }
  }

  if (to.name === 'login' && sessionStore.status === 'authenticated') {
    return { name: 'home' }
  }

  return true
})

export default router
```

#### `frontend/src/views/IntegrationsView.vue`

```vue
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import {
  fetchIntegrations,
  revokeIntegrationGrant,
  startIntegrationConnect,
  verifyIntegration,
} from '../api/integrations'
import { toApiErrorMessage } from '../api/client'
import type { IntegrationStatusBody, VerifyIntegrationBody } from '../api/generated/types.gen'

const route = useRoute()
const router = useRouter()

const items = ref<IntegrationStatusBody[]>([])
const loading = ref(false)
const busyResource = ref('')
const errorMessage = ref('')
const verifyResult = ref<VerifyIntegrationBody | null>(null)

const callbackMessage = computed(() => {
  if (route.query.connected) {
    return `${route.query.connected} integration connected.`
  }
  if (route.query.error) {
    return 'Integration callback failed.'
  }
  return ''
})

async function loadIntegrations() {
  loading.value = true
  errorMessage.value = ''

  try {
    items.value = await fetchIntegrations()
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  } finally {
    loading.value = false
  }
}

function connect(resourceServer: string) {
  startIntegrationConnect(resourceServer)
}

async function verify(resourceServer: string) {
  busyResource.value = resourceServer
  errorMessage.value = ''
  verifyResult.value = null

  try {
    verifyResult.value = await verifyIntegration(resourceServer)
    await loadIntegrations()
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
    await loadIntegrations()
  } finally {
    busyResource.value = ''
  }
}

async function revoke(resourceServer: string) {
  busyResource.value = resourceServer
  errorMessage.value = ''
  verifyResult.value = null

  try {
    await revokeIntegrationGrant(resourceServer)
    await loadIntegrations()
  } catch (error) {
    errorMessage.value = toApiErrorMessage(error)
  } finally {
    busyResource.value = ''
  }
}

function formatDate(value?: string) {
  if (!value) {
    return 'Never'
  }
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
}

function clearCallbackQuery() {
  if (route.query.connected || route.query.error) {
    router.replace({ name: 'integrations' })
  }
}

onMounted(async () => {
  await loadIntegrations()
  clearCallbackQuery()
})
</script>

<template>
  <section class="stack">
    <section class="panel stack">
      <div class="section-header">
        <div>
          <span class="status-pill">Delegated Auth</span>
          <h2>Integrations</h2>
        </div>
        <button class="secondary-button" :disabled="loading" type="button" @click="loadIntegrations">
          {{ loading ? 'Refreshing...' : 'Refresh' }}
        </button>
      </div>

      <p v-if="callbackMessage" class="notice-message">
        {{ callbackMessage }}
      </p>
      <p v-if="errorMessage" class="error-message">
        {{ errorMessage }}
      </p>

      <div class="integration-list">
        <article v-for="item in items" :key="item.resourceServer" class="integration-card">
          <div class="integration-main">
            <div>
              <span class="field-label">{{ item.provider }}</span>
              <h3>{{ item.resourceServer }}</h3>
            </div>
            <span :class="['connection-state', item.connected ? 'connected' : 'disconnected']">
              {{ item.connected ? 'Connected' : 'Disconnected' }}
            </span>
          </div>

          <dl class="metadata-grid">
            <div>
              <dt>Scopes</dt>
              <dd>{{ item.scopes?.join(' ') || 'None' }}</dd>
            </div>
            <div>
              <dt>Granted</dt>
              <dd>{{ formatDate(item.grantedAt) }}</dd>
            </div>
            <div>
              <dt>Last refresh</dt>
              <dd>{{ formatDate(item.lastRefreshedAt) }}</dd>
            </div>
            <div>
              <dt>Last error</dt>
              <dd>{{ item.lastErrorCode || 'None' }}</dd>
            </div>
          </dl>

          <div class="action-row">
            <button class="primary-button" type="button" @click="connect(item.resourceServer)">
              {{ item.connected ? 'Reconnect' : 'Connect' }}
            </button>
            <button
              class="secondary-button"
              :disabled="!item.connected || busyResource === item.resourceServer"
              type="button"
              @click="verify(item.resourceServer)"
            >
              {{ busyResource === item.resourceServer ? 'Verifying...' : 'Verify' }}
            </button>
            <button
              class="secondary-button danger-button"
              :disabled="!item.connected || busyResource === item.resourceServer"
              type="button"
              @click="revoke(item.resourceServer)"
            >
              Revoke
            </button>
          </div>
        </article>
      </div>
    </section>

    <section v-if="verifyResult" class="panel stack">
      <span class="status-pill">Verified</span>
      <h2>Access Check</h2>
      <dl class="metadata-grid">
        <div>
          <dt>Resource</dt>
          <dd>{{ verifyResult.resourceServer }}</dd>
        </div>
        <div>
          <dt>Expires</dt>
          <dd>{{ formatDate(verifyResult.accessExpiresAt) }}</dd>
        </div>
        <div>
          <dt>Refreshed</dt>
          <dd>{{ formatDate(verifyResult.refreshedAt) }}</dd>
        </div>
        <div>
          <dt>Scopes</dt>
          <dd>{{ verifyResult.scopes?.join(' ') || 'None' }}</dd>
        </div>
      </dl>
    </section>
  </section>
</template>

<style scoped>
.section-header,
.integration-main {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 16px;
}

.integration-list {
  display: grid;
  gap: 16px;
}

.integration-card {
  display: grid;
  gap: 20px;
  padding: 20px;
  border: 1px solid var(--border);
  border-radius: 20px;
  background: rgba(255, 250, 243, 0.72);
}

h3 {
  margin: 4px 0 0;
  color: var(--text-strong);
  font-size: 1.3rem;
}

.connection-state {
  display: inline-flex;
  align-items: center;
  min-height: 32px;
  padding: 0 12px;
  border-radius: 999px;
  font-size: 0.8rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.connection-state.connected {
  color: var(--accent-strong);
  background: rgba(11, 93, 91, 0.1);
}

.connection-state.disconnected {
  color: var(--muted);
  background: rgba(124, 102, 88, 0.12);
}

.metadata-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
  margin: 0;
}

.metadata-grid div {
  min-width: 0;
}

.metadata-grid dt {
  margin-bottom: 4px;
  color: var(--muted);
  font-size: 0.78rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.metadata-grid dd {
  margin: 0;
  overflow-wrap: anywhere;
  color: var(--text-strong);
}

.notice-message {
  margin: 0;
  color: var(--accent-strong);
}

.danger-button {
  color: var(--danger);
}

@media (max-width: 720px) {
  .section-header,
  .integration-main {
    align-items: stretch;
    flex-direction: column;
  }

  .metadata-grid {
    grid-template-columns: 1fr;
  }
}
</style>
```

## Phase 5. Provisioning と Tenant-Aware Auth Context

### 目的

just-in-time login だけに依存せず、enterprise provisioning と org / tenant-aware な auth context を成立させます。この Phase は大きいので、実装上は **5A: SCIM / provisioning / deactivation** と **5B: tenant-aware auth context / delegated grant tenant migration** に分けます。

### この Phase の前提

- browser auth, external bearer API, delegated auth がある
- Phase 3 の generic JWT bearer verifier がある
- Phase 4 の `oauth_user_grants` は tenant 非依存で作られている
- tenant membership claim contract は `groups` に固定されている

### この Phase の完了条件

- SCIM / provisioning endpoint が generic bearer verifier で保護されている
- SCIM create / update / patch / deactivate が local user lifecycle に反映される
- deactivation で local session と delegated grant が失効方向へ動く
- browser session と external bearer token の両方で tenant-aware auth context を構築できる
- `oauth_user_grants` が tenant-aware schema に移行済みである
- この Phase の差分を `TUTORIAL_ZITADEL.md` だけから clean worktree に replay できる

### Step 5.1. tenant membership claim grammar を固定する

Phase 5 では provider から見える tenant membership を **top-level の `groups` claim** だけから読みます。application code は `organization_id`, `organization_ids`, provider 固有 claim 名を見ません。

`groups` の tenant membership 文字列は次に固定します。

```text
tenant:<tenant-slug>:<role-code>
```

例:

```text
tenant:acme:todo_user
tenant:acme:docs_reader
```

Phase 5 で tenant role として扱う role は次だけです。

```text
docs_reader
todo_user
```

`external_api_user` は external bearer API に入るための global / provider role のまま残します。tenant role にはしません。

malformed な group、unsupported role、`tenant:` prefix でない group は tenant membership としては無視します。global role sync は Phase 3 までと同じく `docs_reader`, `external_api_user`, `todo_user` を local global role に写像します。

#### Zitadel Action 側の考え方

Zitadel Action では、Zitadel 側の org / project / grant 情報を app code が読む claim 名に寄せます。この tutorial では `groups` に寄せます。

local smoke では、まず次のような groups が ID token / userinfo / JWT access token で見えることだけを確認できれば十分です。

```json
{
  "groups": ["tenant:acme:todo_user", "external_api_user"]
}
```

local smoke 用の固定 Action は次のようにします。

```js
function haohaoGroups(ctx, api) {
  api.v1.claims.setClaim("groups", [
    "tenant:acme:todo_user",
    "external_api_user"
  ]);
}
```

Zitadel Console の `Actions` で作る action 名と JavaScript function 名は一致させます。この例なら両方 `haohaoGroups` です。

作成した Action は `Complement Token` flow に接続します。

```text
Flow Type: Complement Token

Trigger: Pre Userinfo creation
Action:  haohaoGroups

Trigger: Pre access token creation
Action:  haohaoGroups
```

`Pre Userinfo creation` は browser login 後の userinfo / session tenant sync 用です。`Pre access token creation` は external bearer API に渡す JWT access token 用です。片方だけだと browser session と external bearer のどちらかだけが tenant-aware になります。

Zitadel Console の Flow 画面には、trigger card が一時的に見えても保存されていないことがあります。リロード後に消える場合は保存されていません。ブラウザ DevTools の Network で次の request が `200` になっていることを確認してください。

```text
POST /management/v1/flows/2/trigger/4
POST /management/v1/flows/2/trigger/5
```

Management API 上の番号は次です。

```text
2 = Complement Token
4 = Pre Userinfo Creation
5 = Pre Access Token Creation
```

UI で不安定な場合は Management API で確認します。

```bash
MGMT_TOKEN='<zitadel management api token>'

curl -fsS -X POST http://localhost:8081/management/v1/actions/_search \
  -H "Authorization: Bearer $MGMT_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{}' \
  | python3 -m json.tool
```

`name = haohaoGroups` の `id` を控えて、trigger action を設定します。

```bash
ACTION_ID='<haohaoGroups action id>'

curl -fsS -X POST http://localhost:8081/management/v1/flows/2/trigger/4 \
  -H "Authorization: Bearer $MGMT_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"actionIds\":[\"$ACTION_ID\"]}" \
  | python3 -m json.tool

curl -fsS -X POST http://localhost:8081/management/v1/flows/2/trigger/5 \
  -H "Authorization: Bearer $MGMT_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"actionIds\":[\"$ACTION_ID\"]}" \
  | python3 -m json.tool

curl -fsS http://localhost:8081/management/v1/flows/2 \
  -H "Authorization: Bearer $MGMT_TOKEN" \
  | python3 -m json.tool
```

`triggerActions` に `Pre Userinfo Creation` と `Pre Access Token Creation` が残っていれば保存済みです。Zitadel 側を変えたら、既存の browser session / JWT は使い回さず、必ず login と token 取得をやり直してください。JWT は発行時点の claim を持ち続けます。

### Step 5.2. SCIM / provisioning 用設定を追加する

`.env.example` に次を追加します。repo root `.env` は Git 管理しません。

```dotenv
SCIM_BASE_PATH=/api/scim/v2
SCIM_BEARER_AUDIENCE=scim-provisioning
SCIM_REQUIRED_SCOPE=scim:provision
SCIM_RECONCILE_CRON=0 3 * * *
```

SCIM 用 bearer は browser app / external user bearer app とは分けます。

- audience: `scim-provisioning`
- required scope: `scim:provision`
- path: `/api/scim/v2/*`

Phase 6 の M2M verifier はまだ使いません。Phase 3 の generic JWT bearer verifier を SCIM 用 audience / scope で再利用します。

`SCIM_TOKEN` は HaoHao の `/api/scim/v2/*` を叩くための Zitadel-signed JWT access token です。browser の `SESSION_ID` ではありません。HaoHao backend は token を次の順で検証します。

1. `Authorization: Bearer <token>` がある
2. Zitadel JWKS で JWT signature を検証できる
3. `iss` が `ZITADEL_ISSUER` と一致する
4. `aud` に `SCIM_BEARER_AUDIENCE` が含まれる
5. `scope` に `SCIM_REQUIRED_SCOPE` が含まれる

local smoke では、既存の JWT access token 用 app を使って token を取り、JWT payload の `aud` に実際に入っている値へ repo root `.env` の `SCIM_BEARER_AUDIENCE` を合わせても構いません。repo root `.env` は Git 管理しません。`.env.example` の `scim-provisioning` は contract の既定値であり、local Zitadel の token が別の `aud` を出す場合は local `.env` だけを合わせます。

Zitadel が `scope: scim:provision` を access token に出さない場合は、local smoke 用に Action で scope claim を足します。

```js
function haohaoScimScope(ctx, api) {
  api.v1.claims.setClaim("scope", "scim:provision");
}
```

この Action は `Complement Token` flow の `Pre access token creation` に接続します。既に `haohaoGroups` も同じ trigger に接続している場合、両方が active で残ることを確認してください。

```text
Flow Type: Complement Token
Trigger:   Pre access token creation
Actions:   haohaoGroups, haohaoScimScope
```

Management API の `SetTriggerActions` は trigger の action list を置き換えます。API で設定する場合は、既存の `haohaoGroups` action id を落とさず、両方の id を渡してください。

```bash
curl -fsS -X POST http://localhost:8081/management/v1/flows/2/trigger/5 \
  -H "Authorization: Bearer $MGMT_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"actionIds\":[\"$HAOHAO_GROUPS_ACTION_ID\",\"$HAOHAO_SCIM_SCOPE_ACTION_ID\"]}" \
  | python3 -m json.tool
```

### Step 5.3. provisioning schema を追加する

`0005_provisioning` では user lifecycle と provisioning state を追加します。

- `users.deactivated_at`
- `user_identities.external_id`
- `user_identities.provisioning_source`
- `provisioning_sync_state`

SCIM identity は `provider = scim` として保存します。Zitadel OIDC login の `provider = zitadel` identity とは分けます。これにより、SCIM の `externalId` と OIDC の `sub` が衝突せず、delegated auth が `zitadel` identity を引くときにも SCIM identity を誤って拾いません。

SCIM user id は `users.public_id` を使います。SCIM `externalId` は `user_identities.external_id` に保存します。

### Step 5.4. tenant schema と delegated grant tenant migration を追加する

`0006_org_tenants` では次を追加します。

- `tenants`
- `tenant_memberships`
- `tenant_role_overrides`
- `users.default_tenant_id`
- `oauth_user_grants.tenant_id`

`oauth_user_grants` の unique key は次に変えます。

```text
(user_id, provider, resource_server, tenant_id)
```

Phase 4 の historical grant は consent tenant を保持していません。そのため migration では次の順にします。

1. `oauth_user_grants.tenant_id` を nullable で追加する
2. `users.default_tenant_id` がある row だけ backfill する
3. `tenant_id` を解決できない row は削除し、再 consent 必須にする
4. `tenant_id` を `NOT NULL` にする
5. unique key を `(user_id, provider, resource_server, tenant_id)` に置き換える

ここで tenant を推測しません。tenant 導入前の historical grant を誤った tenant に紐づける事故を避けるためです。

### Step 5.5. SCIM endpoint を最小 subset で実装する

完全な SCIM 2.0 全面実装ではなく、Zitadel / local smoke に必要な subset に絞ります。

```text
POST   /api/scim/v2/Users
GET    /api/scim/v2/Users
GET    /api/scim/v2/Users/{id}
PUT    /api/scim/v2/Users/{id}
PATCH  /api/scim/v2/Users/{id}
DELETE /api/scim/v2/Users/{id}
```

`DELETE` と `active=false` は physical delete ではなく `users.deactivated_at` を設定します。

SCIM create / update は idempotent にします。

- `externalId` が既存なら同じ user を更新する
- `externalId` が無く、同じ verified email の local user がいれば SCIM identity を attach する
- どちらも無ければ OIDC user と同じく passwordless local user を作る

SCIM `groups[].value` に `tenant:<slug>:<role>` が入っていれば、source `scim` の tenant membership として同期します。

### Step 5.6. deactivation と session / grant cleanup をつなぐ

Redis session は user_id 逆引き index を持つ必要があります。そうしないと deprovisioning 時に「この user の active session」を確実に消せません。

session store は次を持ちます。

- `session:<session_id>`: session 本体
- `session-user:<user_id>`: user の session id set

deactivation 時は次を行います。

1. `users.deactivated_at` を設定する
2. Redis の user session index から local session を削除する
3. active delegated grant があれば refresh token を upstream revoke する
4. local `oauth_user_grants` を削除する

`backend/internal/jobs/provisioning_reconcile.go` は、取りこぼし補修用に deactivated user の active grant cleanup を再実行できる形にします。

### Step 5.7. tenant-aware AuthContext を入れる

`AuthContext` に次を追加します。

- `DefaultTenant`
- `ActiveTenant`
- `Tenants`

browser session は session record の `activeTenantId` を優先します。未選択なら `users.default_tenant_id` を使い、それも無ければ最初に見つかった effective tenant を default として扱います。

external bearer は `X-Tenant-ID` header で tenant slug を受けます。無ければ default tenant を使います。

```text
X-Tenant-ID: acme
```

service は tenant access を次の順で構築します。

1. `provider_claim` / `scim` の active membership を土台にする
2. `tenant_role_overrides.effect = deny` を適用して role を削る
3. `tenant_role_overrides.effect = allow` を適用して role を足す
4. requested tenant が user の effective tenant に無ければ拒否する

### Step 5.8. browser tenant API を追加する

Phase 5 では tenant 管理 UI は作りません。ただし session の active tenant を確認 / 切替できる browser API は追加します。

```text
GET  /api/v1/tenants
POST /api/v1/session/tenant
```

`POST /api/v1/session/tenant` は Cookie session + CSRF 必須です。body は tenant slug を受けます。

```json
{
  "tenantSlug": "acme"
}
```

### Step 5.9. external bearer API を tenant-aware にする

`/api/external/v1/me` は `X-Tenant-ID` を受け、response に `activeTenant`, `defaultTenant`, `tenants` を含めます。

manual smoke では、同じ user に対して browser session と external bearer token が同じ effective tenant role を得られることを確認してください。

### Step 5.10. manual smoke

manual smoke は次の順で進めます。

1. Zitadel Action / Flow が `groups` claim を出していることを確認する
2. external bearer API が `X-Tenant-ID` で tenant-aware context を返すことを確認する
3. browser login session が同じ tenant role を返すことを確認する
4. `/integrations` が tenant-aware `oauth_user_grants` で Connect / Verify / Revoke できることを確認する
5. SCIM create / patch / deactivate と deactivation side effect を確認する

#### external token を authorization code flow で取り直す

ここでいう authorization code flow は、browser で Zitadel に login し、callback URL の `code` を `/oauth/v2/token` に交換して新しい `access_token` を得る手順です。

Zitadel Action / Flow / role を変えても、既に発行済みの JWT access token の中身は変わりません。`groups` claim の確認前に必ず新しい token を取り直してください。

```bash
CLIENT_ID='<haohao-external-dev client id>'
PROJECT_ID='<haohao project id>'
REDIRECT_URI='http://127.0.0.1:18080/callback'
SCOPE="openid profile email urn:zitadel:iam:org:project:id:${PROJECT_ID}:aud urn:zitadel:iam:org:projects:roles"

CODE_VERIFIER=$(openssl rand -base64 96 | tr '+/' '-_' | tr -d '=' | cut -c 1-128)
CODE_CHALLENGE=$(printf '%s' "$CODE_VERIFIER" | openssl dgst -sha256 -binary | openssl base64 -A | tr '+/' '-_' | tr -d '=')
STATE=$(openssl rand -hex 16)

ENC_REDIRECT=$(python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=""))' "$REDIRECT_URI")
ENC_SCOPE=$(python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=""))' "$SCOPE")

AUTH_URL="http://localhost:8081/oauth/v2/authorize?client_id=${CLIENT_ID}&redirect_uri=${ENC_REDIRECT}&response_type=code&scope=${ENC_SCOPE}&code_challenge=${CODE_CHALLENGE}&code_challenge_method=S256&state=${STATE}"

echo "$AUTH_URL"
open "$AUTH_URL"
```

browser login 後、`http://127.0.0.1:18080/callback?code=...&state=...` に戻ります。callback server は不要です。接続エラー画面になっても address bar の `code` をコピーします。

```bash
CODE='<address bar の code>'

curl -sS -X POST http://localhost:8081/oauth/v2/token \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode grant_type=authorization_code \
  --data-urlencode client_id="$CLIENT_ID" \
  --data-urlencode code="$CODE" \
  --data-urlencode redirect_uri="$REDIRECT_URI" \
  --data-urlencode code_verifier="$CODE_VERIFIER" \
  | tee /tmp/zitadel-external-token.json

EXTERNAL_TOKEN=$(python3 -c 'import json; print(json.load(open("/tmp/zitadel-external-token.json"))["access_token"])')
echo "$EXTERNAL_TOKEN" | awk -F. '{print NF}'
```

`3` が出れば JWT access token です。payload を decode します。

```bash
python3 - "$EXTERNAL_TOKEN" <<'PY'
import base64, json, sys
payload = sys.argv[1].split(".")[1]
payload += "=" * (-len(payload) % 4)
print(json.dumps(json.loads(base64.urlsafe_b64decode(payload)), indent=2, ensure_ascii=False))
PY
```

期待値は次です。

```json
{
  "iss": "http://localhost:8081",
  "groups": [
    "tenant:acme:todo_user",
    "external_api_user"
  ]
}
```

`aud` は local Zitadel の project ID / client ID の配列になることがあります。`401 invalid bearer audience` になる場合は、JWT payload の `aud` に含まれる値を repo root `.env` の `EXTERNAL_EXPECTED_AUDIENCE` に合わせ、backend を再起動してください。

#### external bearer tenant context を確認する

```bash
curl -fsS http://127.0.0.1:8080/api/external/v1/me \
  -H "Authorization: Bearer $EXTERNAL_TOKEN" \
  -H "X-Tenant-ID: acme" \
  | python3 -m json.tool
```

期待値は次です。

```json
{
  "provider": "zitadel",
  "groups": [
    "external_api_user",
    "tenant:acme:todo_user"
  ],
  "roles": [
    "external_api_user"
  ],
  "activeTenant": {
    "slug": "acme",
    "roles": [
      "todo_user"
    ],
    "default": true,
    "selected": true
  },
  "defaultTenant": {
    "slug": "acme"
  },
  "tenants": [
    {
      "slug": "acme",
      "roles": [
        "todo_user"
      ]
    }
  ]
}
```

未知 tenant は拒否されます。

```bash
curl -i http://127.0.0.1:8080/api/external/v1/me \
  -H "Authorization: Bearer $EXTERNAL_TOKEN" \
  -H "X-Tenant-ID: unknown"
```

期待値は `403` です。

#### browser session tenant context を確認する

browser で logout / login し直してから、DevTools などで `SESSION_ID` cookie を控えます。

```bash
SESSION_ID='<browser SESSION_ID cookie>'

curl -fsS http://127.0.0.1:8080/api/v1/tenants \
  -H "Cookie: SESSION_ID=$SESSION_ID" \
  | python3 -m json.tool
```

期待値は次です。

```json
{
  "items": [
    {
      "slug": "acme",
      "roles": [
        "todo_user"
      ],
      "default": true,
      "selected": true
    }
  ],
  "activeTenant": {
    "slug": "acme",
    "roles": [
      "todo_user"
    ]
  },
  "defaultTenant": {
    "slug": "acme"
  }
}
```

active tenant を明示的に選ぶ場合は `XSRF-TOKEN` cookie も使います。

```bash
XSRF_TOKEN='<browser XSRF-TOKEN cookie>'

curl -i -X POST http://127.0.0.1:8080/api/v1/session/tenant \
  -H "Cookie: SESSION_ID=$SESSION_ID; XSRF-TOKEN=$XSRF_TOKEN" \
  -H "X-CSRF-Token: $XSRF_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"tenantSlug":"acme"}'
```

#### integrations の tenant-aware grant を確認する

browser で `/integrations` を開きます。

```text
http://127.0.0.1:5173/integrations
```

確認順は次です。

1. `Connect` で Zitadel consent に進む
2. callback 後に `Connected` になる
3. `Verify` で backend が access token を取得できる
4. token 本体が UI / browser response / frontend state に出ない
5. `Revoke` で local grant が消える

DB では grant が tenant に紐づいていることを確認します。

```bash
if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
else
  COMPOSE="docker-compose"
fi

$COMPOSE exec -T postgres psql -U haohao -d haohao -c "
select
  g.user_id,
  g.provider,
  g.resource_server,
  t.slug as tenant_slug,
  g.scope_text,
  g.last_refreshed_at,
  g.revoked_at,
  g.last_error_code
from oauth_user_grants g
join tenants t on t.id = g.tenant_id
order by g.id desc;
"
```

期待値は次です。

```text
 provider | resource_server | tenant_slug |   scope_text   | last_refreshed_at | revoked_at | last_error_code
----------+-----------------+-------------+----------------+-------------------+------------+-----------------
 zitadel  | zitadel         | acme        | offline_access | <verify timestamp>|            |
```

`Revoke` 後は次が `0` になることを確認します。

```bash
$COMPOSE exec -T postgres psql -U haohao -d haohao -c "
select count(*) from oauth_user_grants;
"
```

#### SCIM create / list / patch / deactivate を確認する

SCIM token を取得したら、まず payload を確認します。SCIM token は `aud` が `SCIM_BEARER_AUDIENCE`、`scope` が `SCIM_REQUIRED_SCOPE` を満たす JWT にします。

```bash
SCIM_TOKEN='<zitadel jwt access token for scim>'

python3 - "$SCIM_TOKEN" <<'PY'
import base64, json, sys
payload = sys.argv[1].split(".")[1]
payload += "=" * (-len(payload) % 4)
print(json.dumps(json.loads(base64.urlsafe_b64decode(payload)), indent=2, ensure_ascii=False))
PY
```

期待値は次です。

```json
{
  "iss": "http://localhost:8081",
  "scope": "scim:provision"
}
```

`aud` は local Zitadel の project ID / client ID の配列になることがあります。例:

```json
{
  "aud": [
    "369925947568160771",
    "370001282871656451",
    "369925841318051843"
  ],
  "client_id": "370001282871656451"
}
```

この場合は、`aud` に含まれる値のうち 1 つを repo root `.env` に設定し、backend を再起動します。local smoke では `client_id` と同じ値を使うと追いやすいです。

```dotenv
SCIM_BEARER_AUDIENCE=370001282871656451
SCIM_REQUIRED_SCOPE=scim:provision
```

token が middleware を通ることを確認します。

```bash
curl -i http://127.0.0.1:8080/api/scim/v2/Users \
  -H "Authorization: Bearer $SCIM_TOKEN"
```

期待値は `200 OK` です。

よくある失敗は次です。

```text
401 invalid bearer audience
```

`SCIM_BEARER_AUDIENCE` が JWT payload の `aud` に含まれていません。

```text
403 invalid bearer scope
```

JWT payload に `scope: scim:provision` がありません。`haohaoScimScope` Action が動いていないか、古い token を使っています。

```text
401 invalid bearer token
```

opaque token か、Zitadel app の `Access Token Type` が JWT ではありません。設定を直して token を取り直してください。

middleware を通ったら create を確認します。

```bash
curl -fsS -X POST http://127.0.0.1:8080/api/scim/v2/Users \
  -H "Authorization: Bearer $SCIM_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "externalId": "phase5-user-1",
    "userName": "phase5-user@example.com",
    "displayName": "Phase 5 User",
    "active": true,
    "groups": [
      {"value": "tenant:acme:todo_user"}
    ]
  }' | tee /tmp/scim-create.json | python3 -m json.tool

SCIM_USER_ID=$(python3 -c 'import json; print(json.load(open("/tmp/scim-create.json"))["id"])')
```

DB では次を確認します。

```bash
$COMPOSE exec -T postgres psql -U haohao -d haohao -c "
select
  u.email,
  u.deactivated_at,
  ui.provider,
  ui.external_id,
  ui.provisioning_source,
  t.slug,
  r.code,
  tm.source
from users u
join user_identities ui on ui.user_id = u.id
join tenant_memberships tm on tm.user_id = u.id
join tenants t on t.id = tm.tenant_id
join roles r on r.id = tm.role_id
where u.email = 'phase5-user@example.com';
"
```

期待値は次です。

```text
provider = scim
external_id = phase5-user-1
provisioning_source = scim
slug = acme
code = todo_user
source = scim
deactivated_at = NULL
```

list / filter / get も確認します。

```bash
curl -fsS 'http://127.0.0.1:8080/api/scim/v2/Users?filter=externalId%20eq%20%22phase5-user-1%22' \
  -H "Authorization: Bearer $SCIM_TOKEN" \
  | python3 -m json.tool

curl -fsS "http://127.0.0.1:8080/api/scim/v2/Users/$SCIM_USER_ID" \
  -H "Authorization: Bearer $SCIM_TOKEN" \
  | python3 -m json.tool
```

PATCH で profile と groups を更新します。malformed group と unsupported tenant role は tenant membership として無視されることも確認します。

```bash
curl -fsS -X PATCH "http://127.0.0.1:8080/api/scim/v2/Users/$SCIM_USER_ID" \
  -H "Authorization: Bearer $SCIM_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "Operations": [
      {"op": "Replace", "path": "displayName", "value": "Phase 5 User Updated"},
      {"op": "Replace", "path": "groups", "value": [
        {"value": "tenant:acme:todo_user"},
        {"value": "tenant:acme:docs_reader"},
        {"value": "tenant:bad"},
        {"value": "tenant:acme:external_api_user"}
      ]}
    ]
  }' | python3 -m json.tool
```

SCIM deactivate は次です。

```bash
curl -fsS -X PATCH http://127.0.0.1:8080/api/scim/v2/Users/$SCIM_USER_ID \
  -H "Authorization: Bearer $SCIM_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "Operations": [
      {"op": "Replace", "path": "active", "value": false}
    ]
  }'
```

期待値は次です。

- `users.deactivated_at` が入る
- 既存 local session が使えなくなる
- `oauth_user_grants` の該当 user row が削除される
- 以後の password login / OIDC login / session lookup / mapped external bearer context が拒否される

isolated SCIM user は login session を持たないため、session invalidation まで見る場合は disposable login user で確認します。管理者 user では実行しないでください。

まず Zitadel Console で disposable human user を作り、その user で HaoHao に browser login します。例:

```bash
TEST_EMAIL='phase5-deactivate@example.com'
TEST_NAME='Phase 5 Deactivate User'
```

login 後に DevTools などで `SESSION_ID` を控えます。

```bash
TEST_SESSION_ID='<disposable user SESSION_ID>'

curl -i http://127.0.0.1:8080/api/v1/tenants \
  -H "Cookie: SESSION_ID=$TEST_SESSION_ID"
```

期待値は deactivate 前なので `200 OK` です。

次に同じ email で SCIM create します。これにより既存 local user に `provider = scim` identity が attach されます。

```bash
SCIM_EXTERNAL_ID="scim-$TEST_EMAIL"

curl -fsS -X POST http://127.0.0.1:8080/api/scim/v2/Users \
  -H "Authorization: Bearer $SCIM_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{
    \"externalId\": \"$SCIM_EXTERNAL_ID\",
    \"userName\": \"$TEST_EMAIL\",
    \"displayName\": \"$TEST_NAME\",
    \"active\": true,
    \"groups\": [
      {\"value\": \"tenant:acme:todo_user\"}
    ]
  }" | tee /tmp/scim-session-user.json | python3 -m json.tool

SCIM_SESSION_USER_ID=$(python3 -c 'import json; print(json.load(open("/tmp/scim-session-user.json"))["id"])')
```

DB では同じ user に `zitadel` と `scim` の 2 identity が付いたことを確認します。

```bash
$COMPOSE exec -T postgres psql -U haohao -d haohao -c "
select
  u.id,
  u.public_id,
  u.email,
  u.deactivated_at,
  ui.provider,
  ui.external_id,
  ui.provisioning_source
from users u
join user_identities ui on ui.user_id = u.id
where u.email = '$TEST_EMAIL'
order by ui.provider;
"
```

期待値は `provider = zitadel` と `provider = scim` の 2 rows で、`deactivated_at` が空であることです。

その user を SCIM deactivate します。

```bash
curl -fsS -X PATCH "http://127.0.0.1:8080/api/scim/v2/Users/$SCIM_SESSION_USER_ID" \
  -H "Authorization: Bearer $SCIM_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "Operations": [
      {"op": "Replace", "path": "active", "value": false}
    ]
  }' | python3 -m json.tool
```

DB で deactivation を確認します。

```bash
$COMPOSE exec -T postgres psql -U haohao -d haohao -c "
select
  email,
  deactivated_at
from users
where email = '$TEST_EMAIL';
"
```

期待値は `deactivated_at` に timestamp が入ることです。

deactivate 前に控えた session は使えなくなります。

```bash
curl -i http://127.0.0.1:8080/api/v1/tenants \
  -H "Cookie: SESSION_ID=$TEST_SESSION_ID"
```

期待値は次です。

```text
HTTP/1.1 401 Unauthorized
{"detail":"missing or expired session", ...}
```

grant cleanup も確認します。

```bash
$COMPOSE exec -T postgres psql -U haohao -d haohao -c "
select count(*)
from oauth_user_grants g
join users u on u.id = g.user_id
where u.email = '$TEST_EMAIL';
"
```

期待値は `0` です。

### 自動確認

Phase 5 実装後は最低限これを回します。

```bash
make gen
make db-up
make db-schema
go test ./backend/...
npm --prefix frontend run build
if docker compose version >/dev/null 2>&1; then
  docker compose --env-file dev/zitadel/.env.example -f dev/zitadel/docker-compose.yml config --quiet
else
  docker-compose --env-file dev/zitadel/.env.example -f dev/zitadel/docker-compose.yml config --quiet
fi
git diff --check
```

---

## Phase 5 Exact Snapshot

Phase 0-2, Phase 3, Phase 4 の exact snapshot を先に使い、Phase 5 で変わった非生成 file だけをここで上書き / 追加します。

- `backend/internal/db/*.go`, `openapi/openapi.yaml`, `frontend/src/api/generated/*` は `make gen` の生成物なので、この snapshot には再掲しません
- `db/schema.sql` は `make db-schema` の生成物ですが、`sqlc generate` の入力でもあるため、この snapshot に含めます
- `backend/web/dist/*` は frontend build artifact なので、この snapshot には含めません
- repo root `.env` と `dev/zitadel/.env` は Git 管理しません

#### Clean worktree replay checklist

`../phase5-test` のような clean worktree で **この `TUTORIAL_ZITADEL.md` だけから Phase 5 の file / directory 構成へ戻す** 場合は、次の順に進めてください。Phase 5 の block は Phase 0-2 / Phase 3 / Phase 4 の同名 file を上書きするため、最終状態が現在の Phase 5 実装になります。

```bash
python3 - <<'PY'
from pathlib import Path
import re

doc = Path("TUTORIAL_ZITADEL.md")
text = doc.read_text()

sections = [
    ("### Project Exact Files", "## Phase 3. External User Bearer API"),
    ("## Phase 3 Exact Snapshot", "## Phase 4."),
    ("## Phase 4 Exact Snapshot", "## Phase 5."),
    ("## Phase 5 Exact Snapshot", "## Phase 6."),
]

files = {}
for start, end in sections:
    start_match = re.search(rf"^{re.escape(start)}", text, re.M)
    if not start_match:
        raise SystemExit(f"section not found: {start}")
    start_index = start_match.start()
    end_match = re.search(rf"^{re.escape(end)}", text[start_index:], re.M)
    end_index = -1 if not end_match else start_index + end_match.start()
    section = text[start_index:] if end_index == -1 else text[start_index:end_index]
    for path, body in re.findall(r'^#### `([^`]+)`\n\n```[^\n]*\n(.*?)\n```', section, re.M | re.S):
        files[path] = body.rstrip("\n") + "\n"

for path, body in files.items():
    target = Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(body)
    print(f"wrote {target}")
PY
```

その後、生成物と build artifact を現在の実装と同じ状態に戻します。

```bash
npm --prefix frontend install
make gen
go test ./backend/...
npm --prefix frontend run build
if docker compose version >/dev/null 2>&1; then
  docker compose --env-file dev/zitadel/.env.example -f dev/zitadel/docker-compose.yml config --quiet
else
  docker-compose --env-file dev/zitadel/.env.example -f dev/zitadel/docker-compose.yml config --quiet
fi
git diff --check
```

manual smoke の前には DB migration を Phase 5 まで適用してください。

```bash
make db-up
```

Zitadel Console 内の Action / SCIM client / token settings は Git に入らない外部状態です。Phase 5 では `groups` claim と SCIM bearer client を追加してください。

#### `.env.example`

```dotenv
APP_NAME="HaoHao API"
APP_VERSION=0.1.0
HTTP_PORT=8080

APP_BASE_URL=http://127.0.0.1:8080
FRONTEND_BASE_URL=http://127.0.0.1:5173

DATABASE_URL=postgres://haohao:haohao@127.0.0.1:5432/haohao?sslmode=disable

AUTH_MODE=local
ZITADEL_ISSUER=
ZITADEL_CLIENT_ID=
ZITADEL_CLIENT_SECRET=
ZITADEL_REDIRECT_URI=http://127.0.0.1:8080/api/v1/auth/callback
ZITADEL_POST_LOGOUT_REDIRECT_URI=http://127.0.0.1:5173/login
ZITADEL_SCOPES="openid profile email"

REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=
REDIS_DB=0

SESSION_TTL=24h
LOGIN_STATE_TTL=10m

EXTERNAL_EXPECTED_AUDIENCE=haohao-external
EXTERNAL_REQUIRED_SCOPE_PREFIX=
EXTERNAL_REQUIRED_ROLE=external_api_user
EXTERNAL_ALLOWED_ORIGINS=

DOWNSTREAM_TOKEN_ENCRYPTION_KEY=
DOWNSTREAM_TOKEN_KEY_VERSION=1
DOWNSTREAM_REFRESH_TOKEN_TTL=2160h
DOWNSTREAM_ACCESS_TOKEN_SKEW=30s
DOWNSTREAM_DEFAULT_SCOPES=offline_access

SCIM_BASE_PATH=/api/scim/v2
SCIM_BEARER_AUDIENCE=scim-provisioning
SCIM_REQUIRED_SCOPE=scim:provision
SCIM_RECONCILE_CRON=0 3 * * *

COOKIE_SECURE=false
```

#### `backend/go.mod`

```text
module example.com/haohao/backend

go 1.25.0

require (
	github.com/coreos/go-oidc/v3 v3.18.0
	github.com/danielgtaylor/huma/v2 v2.37.3
	github.com/gin-gonic/gin v1.12.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.9.2
	github.com/redis/go-redis/v9 v9.18.0
	golang.org/x/oauth2 v0.36.0
)

require (
	github.com/alicebob/miniredis/v2 v2.35.0 // indirect
	github.com/bytedance/gopkg v0.1.3 // indirect
	github.com/bytedance/sonic v1.15.0 // indirect
	github.com/bytedance/sonic/loader v0.5.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/gin-contrib/sse v1.1.0 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.30.1 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.59.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.1 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.mongodb.org/mongo-driver/v2 v2.5.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/arch v0.24.0 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
```

#### `backend/go.sum`

```text
github.com/alicebob/miniredis/v2 v2.35.0 h1:QwLphYqCEAo1eu1TqPRN2jgVMPBweeQcR21jeqDCONI=
github.com/alicebob/miniredis/v2 v2.35.0/go.mod h1:TcL7YfarKPGDAthEtl5NBeHZfeUQj6OXMm/+iu5cLMM=
github.com/bsm/ginkgo/v2 v2.12.0 h1:Ny8MWAHyOepLGlLKYmXG4IEkioBysk6GpaRTLC8zwWs=
github.com/bsm/ginkgo/v2 v2.12.0/go.mod h1:SwYbGRRDovPVboqFv0tPTcG1sN61LM1Z4ARdbAV9g4c=
github.com/bsm/gomega v1.27.10 h1:yeMWxP2pV2fG3FgAODIY8EiRE3dy0aeFYt4l7wh6yKA=
github.com/bsm/gomega v1.27.10/go.mod h1:JyEr/xRbxbtgWNi8tIEVPUYZ5Dzef52k01W3YH0H+O0=
github.com/bytedance/gopkg v0.1.3 h1:TPBSwH8RsouGCBcMBktLt1AymVo2TVsBVCY4b6TnZ/M=
github.com/bytedance/gopkg v0.1.3/go.mod h1:576VvJ+eJgyCzdjS+c4+77QF3p7ubbtiKARP3TxducM=
github.com/bytedance/sonic v1.15.0 h1:/PXeWFaR5ElNcVE84U0dOHjiMHQOwNIx3K4ymzh/uSE=
github.com/bytedance/sonic v1.15.0/go.mod h1:tFkWrPz0/CUCLEF4ri4UkHekCIcdnkqXw9VduqpJh0k=
github.com/bytedance/sonic/loader v0.5.0 h1:gXH3KVnatgY7loH5/TkeVyXPfESoqSBSBEiDd5VjlgE=
github.com/bytedance/sonic/loader v0.5.0/go.mod h1:AR4NYCk5DdzZizZ5djGqQ92eEhCCcdf5x77udYiSJRo=
github.com/cespare/xxhash/v2 v2.3.0 h1:UL815xU9SqsFlibzuggzjXhog7bL6oX9BbNZnL2UFvs=
github.com/cespare/xxhash/v2 v2.3.0/go.mod h1:VGX0DQ3Q6kWi7AoAeZDth3/j3BFtOZR5XLFGgcrjCOs=
github.com/cloudwego/base64x v0.1.6 h1:t11wG9AECkCDk5fMSoxmufanudBtJ+/HemLstXDLI2M=
github.com/cloudwego/base64x v0.1.6/go.mod h1:OFcloc187FXDaYHvrNIjxSe8ncn0OOM8gEHfghB2IPU=
github.com/coreos/go-oidc/v3 v3.18.0 h1:V9orjXynvu5wiC9SemFTWnG4F45v403aIcjWo0d41+A=
github.com/coreos/go-oidc/v3 v3.18.0/go.mod h1:DYCf24+ncYi+XkIH97GY1+dqoRlbaSI26KVTCI9SrY4=
github.com/danielgtaylor/huma/v2 v2.37.3 h1:6Av0Vj45Vk5lDxRVfoO2iPlEdvCvwLc7pl5nbqGOkYM=
github.com/danielgtaylor/huma/v2 v2.37.3/go.mod h1:OeHHtCEAaNiuVbAVdYu4IQ0UOmnb4x3yMUOShNlZ53g=
github.com/davecgh/go-spew v1.1.0/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
github.com/davecgh/go-spew v1.1.1 h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=
github.com/davecgh/go-spew v1.1.1/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f h1:lO4WD4F/rVNCu3HqELle0jiPLLBs70cWOduZpkS1E78=
github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f/go.mod h1:cuUVRXasLTGF7a8hSLbxyZXjz+1KgoB3wDUb6vlszIc=
github.com/fxamacker/cbor/v2 v2.9.0 h1:NpKPmjDBgUfBms6tr6JZkTHtfFGcMKsw3eGcmD/sapM=
github.com/fxamacker/cbor/v2 v2.9.0/go.mod h1:vM4b+DJCtHn+zz7h3FFp/hDAI9WNWCsZj23V5ytsSxQ=
github.com/gabriel-vasile/mimetype v1.4.13 h1:46nXokslUBsAJE/wMsp5gtO500a4F3Nkz9Ufpk2AcUM=
github.com/gabriel-vasile/mimetype v1.4.13/go.mod h1:d+9Oxyo1wTzWdyVUPMmXFvp4F9tea18J8ufA774AB3s=
github.com/gin-contrib/sse v1.1.0 h1:n0w2GMuUpWDVp7qSpvze6fAu9iRxJY4Hmj6AmBOU05w=
github.com/gin-contrib/sse v1.1.0/go.mod h1:hxRZ5gVpWMT7Z0B0gSNYqqsSCNIJMjzvm6fqCz9vjwM=
github.com/gin-gonic/gin v1.12.0 h1:b3YAbrZtnf8N//yjKeU2+MQsh2mY5htkZidOM7O0wG8=
github.com/gin-gonic/gin v1.12.0/go.mod h1:VxccKfsSllpKshkBWgVgRniFFAzFb9csfngsqANjnLc=
github.com/go-jose/go-jose/v4 v4.1.4 h1:moDMcTHmvE6Groj34emNPLs/qtYXRVcd6S7NHbHz3kA=
github.com/go-jose/go-jose/v4 v4.1.4/go.mod h1:x4oUasVrzR7071A4TnHLGSPpNOm2a21K9Kf04k1rs08=
github.com/go-playground/assert/v2 v2.2.0 h1:JvknZsQTYeFEAhQwI4qEt9cyV5ONwRHC+lYKSsYSR8s=
github.com/go-playground/assert/v2 v2.2.0/go.mod h1:VDjEfimB/XKnb+ZQfWdccd7VUvScMdVu0Titje2rxJ4=
github.com/go-playground/locales v0.14.1 h1:EWaQ/wswjilfKLTECiXz7Rh+3BjFhfDFKv/oXslEjJA=
github.com/go-playground/locales v0.14.1/go.mod h1:hxrqLVvrK65+Rwrd5Fc6F2O76J/NuW9t0sjnWqG1slY=
github.com/go-playground/universal-translator v0.18.1 h1:Bcnm0ZwsGyWbCzImXv+pAJnYK9S473LQFuzCbDbfSFY=
github.com/go-playground/universal-translator v0.18.1/go.mod h1:xekY+UJKNuX9WP91TpwSH2VMlDf28Uj24BCp08ZFTUY=
github.com/go-playground/validator/v10 v10.30.1 h1:f3zDSN/zOma+w6+1Wswgd9fLkdwy06ntQJp0BBvFG0w=
github.com/go-playground/validator/v10 v10.30.1/go.mod h1:oSuBIQzuJxL//3MelwSLD5hc2Tu889bF0Idm9Dg26cM=
github.com/goccy/go-json v0.10.5 h1:Fq85nIqj+gXn/S5ahsiTlK3TmC85qgirsdTP/+DeaC4=
github.com/goccy/go-json v0.10.5/go.mod h1:oq7eo15ShAhp70Anwd5lgX2pLfOS3QCiwU/PULtXL6M=
github.com/goccy/go-yaml v1.19.2 h1:PmFC1S6h8ljIz6gMRBopkjP1TVT7xuwrButHID66PoM=
github.com/goccy/go-yaml v1.19.2/go.mod h1:XBurs7gK8ATbW4ZPGKgcbrY1Br56PdM69F7LkFRi1kA=
github.com/google/go-cmp v0.7.0 h1:wk8382ETsv4JYUZwIsn6YpYiWiBsYLSJiTsyBybVuN8=
github.com/google/go-cmp v0.7.0/go.mod h1:pXiqmnSA92OHEEa9HXL2W4E7lf9JzCmGVUdgjX3N/iU=
github.com/google/gofuzz v1.0.0/go.mod h1:dBl0BpW6vV/+mYPU4Po3pmUjxk6FQPldtuIdl/M65Eg=
github.com/google/uuid v1.6.0 h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=
github.com/google/uuid v1.6.0/go.mod h1:TIyPZe4MgqvfeYDBFedMoGGpEw/LqOeaOT+nhxU+yHo=
github.com/jackc/pgpassfile v1.0.0 h1:/6Hmqy13Ss2zCq62VdNG8tM1wchn8zjSGOBJ6icpsIM=
github.com/jackc/pgpassfile v1.0.0/go.mod h1:CEx0iS5ambNFdcRtxPj5JhEz+xB6uRky5eyVu/W2HEg=
github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 h1:iCEnooe7UlwOQYpKFhBabPMi4aNAfoODPEFNiAnClxo=
github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761/go.mod h1:5TJZWKEWniPve33vlWYSoGYefn3gLQRzjfDlhSJ9ZKM=
github.com/jackc/pgx/v5 v5.9.2 h1:3ZhOzMWnR4yJ+RW1XImIPsD1aNSz4T4fyP7zlQb56hw=
github.com/jackc/pgx/v5 v5.9.2/go.mod h1:mal1tBGAFfLHvZzaYh77YS/eC6IX9OWbRV1QIIM0Jn4=
github.com/jackc/puddle/v2 v2.2.2 h1:PR8nw+E/1w0GLuRFSmiioY6UooMp6KJv0/61nB7icHo=
github.com/jackc/puddle/v2 v2.2.2/go.mod h1:vriiEXHvEE654aYKXXjOvZM39qJ0q+azkZFrfEOc3H4=
github.com/json-iterator/go v1.1.12 h1:PV8peI4a0ysnczrg+LtxykD8LfKY9ML6u2jnxaEnrnM=
github.com/json-iterator/go v1.1.12/go.mod h1:e30LSqwooZae/UwlEbR2852Gd8hjQvJoHmT4TnhNGBo=
github.com/klauspost/cpuid/v2 v2.3.0 h1:S4CRMLnYUhGeDFDqkGriYKdfoFlDnMtqTiI/sFzhA9Y=
github.com/klauspost/cpuid/v2 v2.3.0/go.mod h1:hqwkgyIinND0mEev00jJYCxPNVRVXFQeu1XKlok6oO0=
github.com/leodido/go-urn v1.4.0 h1:WT9HwE9SGECu3lg4d/dIA+jxlljEa1/ffXKmRjqdmIQ=
github.com/leodido/go-urn v1.4.0/go.mod h1:bvxc+MVxLKB4z00jd1z+Dvzr47oO32F/QSNjSBOlFxI=
github.com/mattn/go-isatty v0.0.20 h1:xfD0iDuEKnDkl03q4limB+vH+GxLEtL/jb4xVJSWWEY=
github.com/mattn/go-isatty v0.0.20/go.mod h1:W+V8PltTTMOvKvAeJH7IuucS94S2C6jfK/D7dTCTo3Y=
github.com/modern-go/concurrent v0.0.0-20180228061459-e0a39a4cb421/go.mod h1:6dJC0mAP4ikYIbvyc7fijjWJddQyLn8Ig3JB5CqoB9Q=
github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd h1:TRLaZ9cD/w8PVh93nsPXa1VrQ6jlwL5oN8l14QlcNfg=
github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd/go.mod h1:6dJC0mAP4ikYIbvyc7fijjWJddQyLn8Ig3JB5CqoB9Q=
github.com/modern-go/reflect2 v1.0.2 h1:xBagoLtFs94CBntxluKeaWgTMpvLxC4ur3nMaC9Gz0M=
github.com/modern-go/reflect2 v1.0.2/go.mod h1:yWuevngMOJpCy52FWWMvUC8ws7m/LJsjYzDa0/r8luk=
github.com/pelletier/go-toml/v2 v2.2.4 h1:mye9XuhQ6gvn5h28+VilKrrPoQVanw5PMw/TB0t5Ec4=
github.com/pelletier/go-toml/v2 v2.2.4/go.mod h1:2gIqNv+qfxSVS7cM2xJQKtLSTLUE9V8t9Stt+h56mCY=
github.com/pmezard/go-difflib v1.0.0 h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=
github.com/pmezard/go-difflib v1.0.0/go.mod h1:iKH77koFhYxTK1pcRnkKkqfTogsbg7gZNVY4sRDYZ/4=
github.com/quic-go/qpack v0.6.0 h1:g7W+BMYynC1LbYLSqRt8PBg5Tgwxn214ZZR34VIOjz8=
github.com/quic-go/qpack v0.6.0/go.mod h1:lUpLKChi8njB4ty2bFLX2x4gzDqXwUpaO1DP9qMDZII=
github.com/quic-go/quic-go v0.59.0 h1:OLJkp1Mlm/aS7dpKgTc6cnpynnD2Xg7C1pwL6vy/SAw=
github.com/quic-go/quic-go v0.59.0/go.mod h1:upnsH4Ju1YkqpLXC305eW3yDZ4NfnNbmQRCMWS58IKU=
github.com/redis/go-redis/v9 v9.18.0 h1:pMkxYPkEbMPwRdenAzUNyFNrDgHx9U+DrBabWNfSRQs=
github.com/redis/go-redis/v9 v9.18.0/go.mod h1:k3ufPphLU5YXwNTUcCRXGxUoF1fqxnhFQmscfkCoDA0=
github.com/stretchr/objx v0.1.0/go.mod h1:HFkY916IF+rwdDfMAkV7OtwuqBVzrE8GR6GFx+wExME=
github.com/stretchr/objx v0.4.0/go.mod h1:YvHI0jy2hoMjB+UWwv71VJQ9isScKT/TqJzVSSt89Yw=
github.com/stretchr/objx v0.5.0/go.mod h1:Yh+to48EsGEfYuaHDzXPcE3xhTkx73EhmCGUpEOglKo=
github.com/stretchr/objx v0.5.2/go.mod h1:FRsXN1f5AsAjCGJKqEizvkpNtU+EGNCLh3NxZ/8L+MA=
github.com/stretchr/testify v1.3.0/go.mod h1:M5WIy9Dh21IEIfnGCwXGc5bZfKNJtfHm1UVUgZn+9EI=
github.com/stretchr/testify v1.7.0/go.mod h1:6Fq8oRcR53rry900zMqJjRRixrwX3KX962/h/Wwjteg=
github.com/stretchr/testify v1.7.1/go.mod h1:6Fq8oRcR53rry900zMqJjRRixrwX3KX962/h/Wwjteg=
github.com/stretchr/testify v1.8.0/go.mod h1:yNjHg4UonilssWZ8iaSj1OCr/vHnekPRkoO+kdMU+MU=
github.com/stretchr/testify v1.8.4/go.mod h1:sz/lmYIOXD/1dqDmKjjqLyZ2RngseejIcXlSw2iwfAo=
github.com/stretchr/testify v1.10.0/go.mod h1:r2ic/lqez/lEtzL7wO/rwa5dbSLXVDPFyf8C91i36aY=
github.com/stretchr/testify v1.11.1 h1:7s2iGBzp5EwR7/aIZr8ao5+dra3wiQyKjjFuvgVKu7U=
github.com/stretchr/testify v1.11.1/go.mod h1:wZwfW3scLgRK+23gO65QZefKpKQRnfz6sD981Nm4B6U=
github.com/twitchyliquid64/golang-asm v0.15.1 h1:SU5vSMR7hnwNxj24w34ZyCi/FmDZTkS4MhqMhdFk5YI=
github.com/twitchyliquid64/golang-asm v0.15.1/go.mod h1:a1lVb/DtPvCB8fslRZhAngC2+aY1QWCk3Cedj/Gdt08=
github.com/ugorji/go/codec v1.3.1 h1:waO7eEiFDwidsBN6agj1vJQ4AG7lh2yqXyOXqhgQuyY=
github.com/ugorji/go/codec v1.3.1/go.mod h1:pRBVtBSKl77K30Bv8R2P+cLSGaTtex6fsA2Wjqmfxj4=
github.com/x448/float16 v0.8.4 h1:qLwI1I70+NjRFUR3zs1JPUCgaCXSh3SW62uAKT1mSBM=
github.com/x448/float16 v0.8.4/go.mod h1:14CWIYCyZA/cWjXOioeEpHeN/83MdbZDRQHoFcYsOfg=
github.com/yuin/gopher-lua v1.1.1 h1:kYKnWBjvbNP4XLT3+bPEwAXJx262OhaHDWDVOPjL46M=
github.com/yuin/gopher-lua v1.1.1/go.mod h1:GBR0iDaNXjAgGg9zfCvksxSRnQx76gclCIb7kdAd1Pw=
github.com/zeebo/xxh3 v1.0.2 h1:xZmwmqxHZA8AI603jOQ0tMqmBr9lPeFwGg6d+xy9DC0=
github.com/zeebo/xxh3 v1.0.2/go.mod h1:5NWz9Sef7zIDm2JHfFlcQvNekmcEl9ekUZQQKCYaDcA=
go.mongodb.org/mongo-driver/v2 v2.5.0 h1:yXUhImUjjAInNcpTcAlPHiT7bIXhshCTL3jVBkF3xaE=
go.mongodb.org/mongo-driver/v2 v2.5.0/go.mod h1:yOI9kBsufol30iFsl1slpdq1I0eHPzybRWdyYUs8K/0=
go.uber.org/atomic v1.11.0 h1:ZvwS0R+56ePWxUNi+Atn9dWONBPp/AUETXlHW0DxSjE=
go.uber.org/atomic v1.11.0/go.mod h1:LUxbIzbOniOlMKjJjyPfpl4v+PKK2cNJn91OQbhoJI0=
go.uber.org/mock v0.6.0 h1:hyF9dfmbgIX5EfOdasqLsWD6xqpNZlXblLB/Dbnwv3Y=
go.uber.org/mock v0.6.0/go.mod h1:KiVJ4BqZJaMj4svdfmHM0AUx4NJYO8ZNpPnZn1Z+BBU=
golang.org/x/arch v0.24.0 h1:qlJ3M9upxvFfwRM51tTg3Yl+8CP9vCC1E7vlFpgv99Y=
golang.org/x/arch v0.24.0/go.mod h1:dNHoOeKiyja7GTvF9NJS1l3Z2yntpQNzgrjh1cU103A=
golang.org/x/crypto v0.48.0 h1:/VRzVqiRSggnhY7gNRxPauEQ5Drw9haKdM0jqfcCFts=
golang.org/x/crypto v0.48.0/go.mod h1:r0kV5h3qnFPlQnBSrULhlsRfryS2pmewsg+XfMgkVos=
golang.org/x/net v0.51.0 h1:94R/GTO7mt3/4wIKpcR5gkGmRLOuE/2hNGeWq/GBIFo=
golang.org/x/net v0.51.0/go.mod h1:aamm+2QF5ogm02fjy5Bb7CQ0WMt1/WVM7FtyaTLlA9Y=
golang.org/x/oauth2 v0.36.0 h1:peZ/1z27fi9hUOFCAZaHyrpWG5lwe0RJEEEeH0ThlIs=
golang.org/x/oauth2 v0.36.0/go.mod h1:YDBUJMTkDnJS+A4BP4eZBjCqtokkg1hODuPjwiGPO7Q=
golang.org/x/sync v0.19.0 h1:vV+1eWNmZ5geRlYjzm2adRgW2/mcpevXNg50YZtPCE4=
golang.org/x/sync v0.19.0/go.mod h1:9KTHXmSnoGruLpwFjVSX0lNNA75CykiMECbovNTZqGI=
golang.org/x/sys v0.6.0/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
golang.org/x/sys v0.41.0 h1:Ivj+2Cp/ylzLiEU89QhWblYnOE9zerudt9Ftecq2C6k=
golang.org/x/sys v0.41.0/go.mod h1:OgkHotnGiDImocRcuBABYBEXf8A9a87e/uXjp9XT3ks=
golang.org/x/text v0.34.0 h1:oL/Qq0Kdaqxa1KbNeMKwQq0reLCCaFtqu2eNuSeNHbk=
golang.org/x/text v0.34.0/go.mod h1:homfLqTYRFyVYemLBFl5GgL/DWEiH5wcsQ5gSh1yziA=
google.golang.org/protobuf v1.36.11 h1:fV6ZwhNocDyBLK0dj+fg8ektcVegBBuEolpbTQyBNVE=
google.golang.org/protobuf v1.36.11/go.mod h1:HTf+CrKn2C3g5S8VImy6tdcUvCska2kB7j23XfzDpco=
gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405/go.mod h1:Co6ibVJAznAaIkqp8huTwlJQCZ016jof/cbN4VW5Yz0=
gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=
gopkg.in/yaml.v3 v3.0.1 h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=
gopkg.in/yaml.v3 v3.0.1/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=
```

#### `backend/cmd/main/main.go`

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
	sessionService := service.NewSessionService(queries, sessionStore, cfg.AuthMode)
	authzService := service.NewAuthzService(pool, queries)

	var oidcLoginService *service.OIDCLoginService
	var delegationService *service.DelegationService
	var bearerVerifier *auth.BearerVerifier
	if cfg.AuthMode == "zitadel" {
		if cfg.ZitadelIssuer == "" || cfg.ZitadelClientID == "" || cfg.ZitadelClientSecret == "" {
			log.Fatal("ZITADEL_ISSUER, ZITADEL_CLIENT_ID, and ZITADEL_CLIENT_SECRET are required when AUTH_MODE=zitadel")
		}

		oidcClient, err := auth.NewOIDCClient(
			ctx,
			cfg.ZitadelIssuer,
			cfg.ZitadelClientID,
			cfg.ZitadelClientSecret,
			cfg.ZitadelRedirectURI,
			cfg.ZitadelScopes,
		)
		if err != nil {
			log.Fatal(err)
		}

		loginStateStore := auth.NewLoginStateStore(redisClient, cfg.LoginStateTTL)
		identityService := service.NewIdentityService(pool, queries)
		oidcLoginService = service.NewOIDCLoginService("zitadel", oidcClient, loginStateStore, identityService, authzService, sessionService)

		if cfg.DownstreamTokenEncryptionKey != "" {
			refreshTokenStore, err := auth.NewRefreshTokenStore(cfg.DownstreamTokenEncryptionKey, cfg.DownstreamTokenKeyVersion)
			if err != nil {
				log.Fatal(err)
			}

			delegatedOAuthClient, err := auth.NewDelegatedOAuthClient(ctx, cfg.ZitadelIssuer, cfg.ZitadelClientID, cfg.ZitadelClientSecret)
			if err != nil {
				log.Fatal(err)
			}

			delegationStateStore := auth.NewDelegationStateStore(redisClient, cfg.LoginStateTTL)
			delegationService = service.NewDelegationService(
				queries,
				delegatedOAuthClient,
				delegationStateStore,
				refreshTokenStore,
				cfg.AppBaseURL,
				cfg.DownstreamDefaultScopes,
				cfg.DownstreamRefreshTokenTTL,
				cfg.DownstreamAccessTokenSkew,
			)
		}
	}

	if cfg.ZitadelIssuer != "" {
		bearerVerifier, err = auth.NewBearerVerifier(ctx, cfg.ZitadelIssuer)
		if err != nil {
			log.Fatal(err)
		}
	}

	provisioningService := service.NewProvisioningService(pool, queries, sessionService, delegationService, authzService)

	application := app.New(cfg, sessionService, oidcLoginService, delegationService, provisioningService, authzService, bearerVerifier)

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

#### `backend/cmd/openapi/main.go`

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

	application := app.New(cfg, nil, nil, nil, nil, nil, nil)

	spec, err := application.API.OpenAPI().YAML()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(string(spec))
}
```

#### `backend/internal/api/external_me.go`

```go
package api

import (
	"context"
	"net/http"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type ExternalMeBody struct {
	Provider        string        `json:"provider" example:"zitadel"`
	Subject         string        `json:"subject" example:"312345678901234567"`
	AuthorizedParty string        `json:"authorizedParty,omitempty" example:"312345678901234568"`
	Scopes          []string      `json:"scopes,omitempty" example:"external:read"`
	Groups          []string      `json:"groups,omitempty" example:"external_api_user"`
	Roles           []string      `json:"roles,omitempty" example:"todo_user"`
	User            *UserResponse `json:"user,omitempty"`
	ActiveTenant    *TenantBody   `json:"activeTenant,omitempty"`
	DefaultTenant   *TenantBody   `json:"defaultTenant,omitempty"`
	Tenants         []TenantBody  `json:"tenants,omitempty"`
}

type GetExternalMeInput struct {
	TenantID string `header:"X-Tenant-ID" doc:"tenant slug for tenant-aware bearer context" example:"acme"`
}

type GetExternalMeOutput struct {
	Body ExternalMeBody
}

func registerExternalRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "getExternalMe",
		Method:      http.MethodGet,
		Path:        "/api/external/v1/me",
		Summary:     "現在の external bearer principal を返す",
		Tags:        []string{"external"},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *GetExternalMeInput) (*GetExternalMeOutput, error) {
		authCtx, ok := service.AuthContextFromContext(ctx)
		if !ok {
			return nil, huma.Error500InternalServerError("missing auth context")
		}

		var user *UserResponse
		if authCtx.User != nil {
			res := toUserResponse(*authCtx.User)
			user = &res
		}

		var activeTenant *TenantBody
		if authCtx.ActiveTenant != nil {
			item := toTenantBody(*authCtx.ActiveTenant)
			activeTenant = &item
		}
		var defaultTenant *TenantBody
		if authCtx.DefaultTenant != nil {
			item := toTenantBody(*authCtx.DefaultTenant)
			defaultTenant = &item
		}

		return &GetExternalMeOutput{
			Body: ExternalMeBody{
				Provider:        authCtx.Provider,
				Subject:         authCtx.Subject,
				AuthorizedParty: authCtx.AuthorizedParty,
				Scopes:          authCtx.Scopes,
				Groups:          authCtx.Groups,
				Roles:           authCtx.Roles,
				User:            user,
				ActiveTenant:    activeTenant,
				DefaultTenant:   defaultTenant,
				Tenants:         toTenantBodies(authCtx.Tenants),
			},
		}, nil
	})
}
```

#### `backend/internal/api/integrations.go`

```go
package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type IntegrationStatusBody struct {
	ResourceServer  string     `json:"resourceServer" example:"zitadel"`
	Provider        string     `json:"provider" example:"zitadel"`
	Connected       bool       `json:"connected" example:"true"`
	Scopes          []string   `json:"scopes,omitempty" example:"offline_access"`
	GrantedAt       *time.Time `json:"grantedAt,omitempty" format:"date-time"`
	LastRefreshedAt *time.Time `json:"lastRefreshedAt,omitempty" format:"date-time"`
	RevokedAt       *time.Time `json:"revokedAt,omitempty" format:"date-time"`
	LastErrorCode   string     `json:"lastErrorCode,omitempty" example:"invalid_grant"`
}

type ListIntegrationsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
}

type ListIntegrationsBody struct {
	Items []IntegrationStatusBody `json:"items"`
}

type ListIntegrationsOutput struct {
	Body ListIntegrationsBody
}

type ConnectIntegrationInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	ResourceServer string      `path:"resourceServer" example:"zitadel"`
}

type ConnectIntegrationOutput struct {
	Location string `header:"Location"`
}

type IntegrationCallbackInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	ResourceServer   string      `path:"resourceServer" example:"zitadel"`
	Code             string      `query:"code"`
	State            string      `query:"state"`
	Error            string      `query:"error"`
	ErrorDescription string      `query:"error_description"`
}

type IntegrationCallbackOutput struct {
	Location string `header:"Location"`
}

type VerifyIntegrationInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	ResourceServer string      `path:"resourceServer" example:"zitadel"`
}

type VerifyIntegrationBody struct {
	ResourceServer  string     `json:"resourceServer" example:"zitadel"`
	Connected       bool       `json:"connected" example:"true"`
	Scopes          []string   `json:"scopes,omitempty" example:"offline_access"`
	AccessExpiresAt *time.Time `json:"accessExpiresAt,omitempty" format:"date-time"`
	RefreshedAt     *time.Time `json:"refreshedAt,omitempty" format:"date-time"`
}

type VerifyIntegrationOutput struct {
	Body VerifyIntegrationBody
}

type DeleteIntegrationGrantInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	ResourceServer string      `path:"resourceServer" example:"zitadel"`
}

type DeleteIntegrationGrantOutput struct{}

func registerIntegrationRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listIntegrations",
		Method:      http.MethodGet,
		Path:        "/api/v1/integrations",
		Summary:     "downstream integration の接続状態を返す",
		Tags:        []string{"integrations"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ListIntegrationsInput) (*ListIntegrationsOutput, error) {
		if deps.DelegationService == nil {
			return nil, huma.Error503ServiceUnavailable("delegated auth is not configured")
		}

		current, authCtx, err := currentSessionAuthContext(ctx, deps, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPError(err)
		}
		if authCtx.ActiveTenant == nil {
			return nil, huma.Error409Conflict("active tenant is required before connecting integrations")
		}

		statuses, err := deps.DelegationService.ListIntegrationsForTenant(ctx, current.User, authCtx.ActiveTenant.ID)
		if err != nil {
			return nil, toDelegationHTTPError(err)
		}

		out := &ListIntegrationsOutput{}
		out.Body.Items = make([]IntegrationStatusBody, 0, len(statuses))
		for _, status := range statuses {
			out.Body.Items = append(out.Body.Items, toIntegrationStatusBody(status))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "connectIntegration",
		Method:        http.MethodGet,
		Path:          "/api/v1/integrations/{resourceServer}/connect",
		Summary:       "downstream integration consent を開始する",
		Tags:          []string{"integrations"},
		DefaultStatus: http.StatusFound,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ConnectIntegrationInput) (*ConnectIntegrationOutput, error) {
		if deps.DelegationService == nil {
			return nil, huma.Error503ServiceUnavailable("delegated auth is not configured")
		}

		current, authCtx, err := currentSessionAuthContext(ctx, deps, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPError(err)
		}
		if authCtx.ActiveTenant == nil {
			return nil, huma.Error409Conflict("active tenant is required before connecting integrations")
		}

		location, err := deps.DelegationService.StartConnectForTenant(ctx, current.User, authCtx.ActiveTenant.ID, input.SessionCookie.Value, input.ResourceServer)
		if err != nil {
			return nil, toDelegationHTTPError(err)
		}

		return &ConnectIntegrationOutput{Location: location}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "finishIntegrationConnect",
		Method:        http.MethodGet,
		Path:          "/api/v1/integrations/{resourceServer}/callback",
		Summary:       "downstream integration consent callback を完了する",
		Tags:          []string{"integrations"},
		DefaultStatus: http.StatusFound,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *IntegrationCallbackInput) (*IntegrationCallbackOutput, error) {
		if input.Error != "" || deps.DelegationService == nil {
			return &IntegrationCallbackOutput{
				Location: integrationRedirect(deps.FrontendBaseURL, "error", "delegated_callback_failed"),
			}, nil
		}

		user, err := deps.SessionService.CurrentUser(ctx, input.SessionCookie.Value)
		if err != nil {
			return &IntegrationCallbackOutput{
				Location: integrationRedirect(deps.FrontendBaseURL, "error", "missing_session"),
			}, nil
		}

		if _, err := deps.DelegationService.SaveGrantFromCallback(ctx, user, input.SessionCookie.Value, input.ResourceServer, input.Code, input.State); err != nil {
			return &IntegrationCallbackOutput{
				Location: integrationRedirect(deps.FrontendBaseURL, "error", "delegated_callback_failed"),
			}, nil
		}

		return &IntegrationCallbackOutput{
			Location: integrationRedirect(deps.FrontendBaseURL, "connected", normalizeIntegrationResource(input.ResourceServer)),
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "verifyIntegrationAccess",
		Method:      http.MethodPost,
		Path:        "/api/v1/integrations/{resourceServer}/verify",
		Summary:     "downstream access token を backend 内で取得できるか検証する",
		Tags:        []string{"integrations"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *VerifyIntegrationInput) (*VerifyIntegrationOutput, error) {
		if deps.DelegationService == nil {
			return nil, huma.Error503ServiceUnavailable("delegated auth is not configured")
		}

		current, authCtx, err := currentSessionAuthContextWithCSRF(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, toHTTPError(err)
		}
		if authCtx.ActiveTenant == nil {
			return nil, huma.Error409Conflict("active tenant is required before verifying integrations")
		}

		result, err := deps.DelegationService.VerifyAccessTokenForTenant(ctx, current.User, authCtx.ActiveTenant.ID, input.ResourceServer)
		if err != nil {
			return nil, toDelegationHTTPError(err)
		}

		out := &VerifyIntegrationOutput{}
		out.Body.ResourceServer = result.ResourceServer
		out.Body.Connected = result.Connected
		out.Body.Scopes = result.Scopes
		out.Body.AccessExpiresAt = result.AccessExpiresAt
		out.Body.RefreshedAt = result.RefreshedAt
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteIntegrationGrant",
		Method:        http.MethodDelete,
		Path:          "/api/v1/integrations/{resourceServer}/grant",
		Summary:       "downstream integration grant を削除する",
		Tags:          []string{"integrations"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *DeleteIntegrationGrantInput) (*DeleteIntegrationGrantOutput, error) {
		if deps.DelegationService == nil {
			return nil, huma.Error503ServiceUnavailable("delegated auth is not configured")
		}

		current, authCtx, err := currentSessionAuthContextWithCSRF(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, toHTTPError(err)
		}
		if authCtx.ActiveTenant == nil {
			return nil, huma.Error409Conflict("active tenant is required before deleting integrations")
		}

		if err := deps.DelegationService.DeleteGrantForTenant(ctx, current.User, authCtx.ActiveTenant.ID, input.ResourceServer); err != nil {
			return nil, toDelegationHTTPError(err)
		}

		return &DeleteIntegrationGrantOutput{}, nil
	})
}

func toIntegrationStatusBody(status service.DelegationStatus) IntegrationStatusBody {
	return IntegrationStatusBody{
		ResourceServer:  status.ResourceServer,
		Provider:        status.Provider,
		Connected:       status.Connected,
		Scopes:          status.Scopes,
		GrantedAt:       status.GrantedAt,
		LastRefreshedAt: status.LastRefreshedAt,
		RevokedAt:       status.RevokedAt,
		LastErrorCode:   status.LastErrorCode,
	}
}

func toDelegationHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrDelegationNotConfigured):
		return huma.Error503ServiceUnavailable("delegated auth is not configured")
	case errors.Is(err, service.ErrDelegationUnsupportedResource):
		return huma.Error404NotFound("unsupported downstream resource")
	case errors.Is(err, service.ErrDelegationGrantNotFound):
		return huma.Error404NotFound("delegated grant not found")
	case errors.Is(err, service.ErrDelegationInvalidState):
		return huma.Error400BadRequest("invalid delegated auth state")
	case errors.Is(err, service.ErrDelegationIdentityNotFound):
		return huma.Error409Conflict("zitadel identity is required before connecting the integration")
	case errors.Is(err, service.ErrDelegationRefreshTokenMissing):
		return huma.Error502BadGateway("provider did not return a refresh token")
	default:
		return huma.Error500InternalServerError("internal server error")
	}
}

func currentSessionAuthContext(ctx context.Context, deps Dependencies, sessionID string) (service.CurrentSession, service.AuthContext, error) {
	current, err := deps.SessionService.CurrentSession(ctx, sessionID)
	if err != nil {
		return service.CurrentSession{}, service.AuthContext{}, err
	}
	if deps.AuthzService == nil {
		return current, service.AuthContext{}, huma.Error503ServiceUnavailable("tenant auth is not configured")
	}
	authCtx, err := deps.AuthzService.BuildBrowserContext(ctx, current.User, current.ActiveTenantID)
	if err != nil {
		return service.CurrentSession{}, service.AuthContext{}, err
	}
	return current, authCtx, nil
}

func currentSessionAuthContextWithCSRF(ctx context.Context, deps Dependencies, sessionID, csrfToken string) (service.CurrentSession, service.AuthContext, error) {
	current, err := deps.SessionService.CurrentSessionWithCSRF(ctx, sessionID, csrfToken)
	if err != nil {
		return service.CurrentSession{}, service.AuthContext{}, err
	}
	if deps.AuthzService == nil {
		return current, service.AuthContext{}, huma.Error503ServiceUnavailable("tenant auth is not configured")
	}
	authCtx, err := deps.AuthzService.BuildBrowserContext(ctx, current.User, current.ActiveTenantID)
	if err != nil {
		return service.CurrentSession{}, service.AuthContext{}, err
	}
	return current, authCtx, nil
}

func integrationRedirect(frontendBaseURL, key, value string) string {
	base := strings.TrimRight(frontendBaseURL, "/")
	query := url.Values{}
	query.Set(key, value)
	return base + "/integrations?" + query.Encode()
}

func normalizeIntegrationResource(resourceServer string) string {
	return strings.ToLower(strings.TrimSpace(resourceServer))
}
```

#### `backend/internal/api/register.go`

```go
package api

import (
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type Dependencies struct {
	SessionService               *service.SessionService
	OIDCLoginService             *service.OIDCLoginService
	DelegationService            *service.DelegationService
	ProvisioningService          *service.ProvisioningService
	AuthzService                 *service.AuthzService
	AuthMode                     string
	SCIMBasePath                 string
	FrontendBaseURL              string
	ZitadelIssuer                string
	ZitadelClientID              string
	ZitadelPostLogoutRedirectURI string
	CookieSecure                 bool
	SessionTTL                   time.Duration
}

func Register(api huma.API, deps Dependencies) {
	registerAuthSettingsRoute(api, deps)
	registerOIDCRoutes(api, deps)
	registerSessionRoutes(api, deps)
	registerExternalRoutes(api, deps)
	registerIntegrationRoutes(api, deps)
	registerTenantRoutes(api, deps)
	registerSCIMRoutes(api, deps)
}
```

#### `backend/internal/api/scim.go`

```go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

var externalIDFilterRE = regexp.MustCompile(`(?i)^\s*externalId\s+eq\s+"([^"]+)"\s*$`)

type SCIMUserBody struct {
	Schemas     []string        `json:"schemas,omitempty"`
	ID          string          `json:"id,omitempty" format:"uuid"`
	ExternalID  string          `json:"externalId,omitempty"`
	UserName    string          `json:"userName,omitempty" format:"email"`
	DisplayName string          `json:"displayName,omitempty"`
	Active      *bool           `json:"active,omitempty"`
	Groups      []SCIMGroupBody `json:"groups,omitempty"`
}

type SCIMGroupBody struct {
	Value string `json:"value"`
}

type SCIMListResponseBody struct {
	Schemas      []string       `json:"schemas"`
	TotalResults int            `json:"totalResults"`
	StartIndex   int32          `json:"startIndex"`
	ItemsPerPage int            `json:"itemsPerPage"`
	Resources    []SCIMUserBody `json:"Resources"`
}

type SCIMUserInput struct {
	Body SCIMUserBody
}

type SCIMUserByIDInput struct {
	ID string `path:"id" format:"uuid"`
}

type SCIMListUsersInput struct {
	Filter     string `query:"filter"`
	StartIndex int32  `query:"startIndex" minimum:"1" default:"1"`
	Count      int32  `query:"count" minimum:"1" maximum:"100" default:"100"`
}

type SCIMReplaceUserInput struct {
	ID   string `path:"id" format:"uuid"`
	Body SCIMUserBody
}

type SCIMPatchInput struct {
	ID   string `path:"id" format:"uuid"`
	Body struct {
		Schemas    []string             `json:"schemas,omitempty"`
		Operations []SCIMPatchOperation `json:"Operations"`
	}
}

type SCIMPatchOperation struct {
	Op    string          `json:"op"`
	Path  string          `json:"path,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

type SCIMUserOutput struct {
	Body SCIMUserBody
}

type SCIMListUsersOutput struct {
	Body SCIMListResponseBody
}

type SCIMDeleteUserOutput struct{}

func registerSCIMRoutes(api huma.API, deps Dependencies) {
	if deps.SCIMBasePath == "" {
		return
	}
	usersPath := deps.SCIMBasePath + "/Users"

	huma.Register(api, huma.Operation{
		OperationID: "scimCreateUser",
		Method:      http.MethodPost,
		Path:        usersPath,
		Summary:     "SCIM user を作成または upsert する",
		Tags:        []string{"scim"},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *SCIMUserInput) (*SCIMUserOutput, error) {
		if deps.ProvisioningService == nil {
			return nil, huma.Error503ServiceUnavailable("scim provisioning is not configured")
		}
		user, err := deps.ProvisioningService.UpsertUser(ctx, provisionedInputFromSCIM(input.Body))
		if err != nil {
			return nil, toSCIMHTTPError(err)
		}
		return &SCIMUserOutput{Body: scimUserFromProvisioned(user)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "scimListUsers",
		Method:      http.MethodGet,
		Path:        usersPath,
		Summary:     "SCIM user を list する",
		Tags:        []string{"scim"},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *SCIMListUsersInput) (*SCIMListUsersOutput, error) {
		if deps.ProvisioningService == nil {
			return nil, huma.Error503ServiceUnavailable("scim provisioning is not configured")
		}
		if externalID := externalIDFromFilter(input.Filter); externalID != "" {
			user, err := deps.ProvisioningService.GetUserByExternalID(ctx, externalID)
			if err == nil {
				body := scimUserFromProvisioned(user)
				return &SCIMListUsersOutput{Body: scimList(input.StartIndex, []SCIMUserBody{body})}, nil
			}
			if errors.Is(err, service.ErrUnauthorized) {
				return &SCIMListUsersOutput{Body: scimList(input.StartIndex, nil)}, nil
			}
			if !errors.Is(err, service.ErrInvalidSCIMUser) {
				return nil, toSCIMHTTPError(err)
			}
		}

		users, err := deps.ProvisioningService.ListUsers(ctx, input.StartIndex, input.Count)
		if err != nil {
			return nil, toSCIMHTTPError(err)
		}
		resources := make([]SCIMUserBody, 0, len(users))
		for _, user := range users {
			resources = append(resources, scimUserFromProvisioned(user))
		}
		return &SCIMListUsersOutput{Body: scimList(input.StartIndex, resources)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "scimGetUser",
		Method:      http.MethodGet,
		Path:        usersPath + "/{id}",
		Summary:     "SCIM user を取得する",
		Tags:        []string{"scim"},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *SCIMUserByIDInput) (*SCIMUserOutput, error) {
		if deps.ProvisioningService == nil {
			return nil, huma.Error503ServiceUnavailable("scim provisioning is not configured")
		}
		user, err := deps.ProvisioningService.GetUser(ctx, input.ID)
		if err != nil {
			return nil, toSCIMHTTPError(err)
		}
		return &SCIMUserOutput{Body: scimUserFromProvisioned(user)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "scimReplaceUser",
		Method:      http.MethodPut,
		Path:        usersPath + "/{id}",
		Summary:     "SCIM user を置換する",
		Tags:        []string{"scim"},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *SCIMReplaceUserInput) (*SCIMUserOutput, error) {
		if deps.ProvisioningService == nil {
			return nil, huma.Error503ServiceUnavailable("scim provisioning is not configured")
		}
		existing, err := deps.ProvisioningService.GetUser(ctx, input.ID)
		if err != nil {
			return nil, toSCIMHTTPError(err)
		}
		body := input.Body
		if strings.TrimSpace(body.ExternalID) == "" {
			body.ExternalID = existing.ExternalID
		}
		user, err := deps.ProvisioningService.UpsertUser(ctx, provisionedInputFromSCIM(body))
		if err != nil {
			return nil, toSCIMHTTPError(err)
		}
		return &SCIMUserOutput{Body: scimUserFromProvisioned(user)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "scimPatchUser",
		Method:      http.MethodPatch,
		Path:        usersPath + "/{id}",
		Summary:     "SCIM user を patch する",
		Tags:        []string{"scim"},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *SCIMPatchInput) (*SCIMUserOutput, error) {
		if deps.ProvisioningService == nil {
			return nil, huma.Error503ServiceUnavailable("scim provisioning is not configured")
		}
		existing, err := deps.ProvisioningService.GetUser(ctx, input.ID)
		if err != nil {
			return nil, toSCIMHTTPError(err)
		}
		next := SCIMUserBody{
			ExternalID:  existing.ExternalID,
			UserName:    existing.UserName,
			DisplayName: existing.DisplayName,
			Active:      &existing.Active,
		}
		for _, op := range input.Body.Operations {
			if err := applySCIMPatch(&next, op); err != nil {
				return nil, toSCIMHTTPError(err)
			}
		}
		user, err := deps.ProvisioningService.UpsertUser(ctx, provisionedInputFromSCIM(next))
		if err != nil {
			return nil, toSCIMHTTPError(err)
		}
		return &SCIMUserOutput{Body: scimUserFromProvisioned(user)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "scimDeleteUser",
		Method:        http.MethodDelete,
		Path:          usersPath + "/{id}",
		Summary:       "SCIM user を deactivate する",
		Tags:          []string{"scim"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *SCIMUserByIDInput) (*SCIMDeleteUserOutput, error) {
		if deps.ProvisioningService == nil {
			return nil, huma.Error503ServiceUnavailable("scim provisioning is not configured")
		}
		if _, err := deps.ProvisioningService.DeactivateUser(ctx, input.ID); err != nil {
			return nil, toSCIMHTTPError(err)
		}
		return &SCIMDeleteUserOutput{}, nil
	})
}

func provisionedInputFromSCIM(body SCIMUserBody) service.ProvisionedUserInput {
	active := true
	if body.Active != nil {
		active = *body.Active
	}
	groups := make([]string, 0, len(body.Groups))
	for _, group := range body.Groups {
		if strings.TrimSpace(group.Value) != "" {
			groups = append(groups, strings.TrimSpace(group.Value))
		}
	}
	if body.Groups == nil {
		groups = nil
	}
	return service.ProvisionedUserInput{
		ExternalID:  body.ExternalID,
		UserName:    body.UserName,
		DisplayName: body.DisplayName,
		Active:      active,
		Groups:      groups,
	}
}

func scimUserFromProvisioned(user service.ProvisionedUser) SCIMUserBody {
	active := user.Active
	return SCIMUserBody{
		Schemas:     []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		ID:          user.PublicID,
		ExternalID:  user.ExternalID,
		UserName:    user.UserName,
		DisplayName: user.DisplayName,
		Active:      &active,
	}
}

func scimList(startIndex int32, resources []SCIMUserBody) SCIMListResponseBody {
	if startIndex < 1 {
		startIndex = 1
	}
	return SCIMListResponseBody{
		Schemas:      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		TotalResults: len(resources),
		StartIndex:   startIndex,
		ItemsPerPage: len(resources),
		Resources:    resources,
	}
}

func applySCIMPatch(body *SCIMUserBody, op SCIMPatchOperation) error {
	if strings.ToLower(strings.TrimSpace(op.Op)) != "replace" {
		return service.ErrInvalidSCIMUser
	}
	path := strings.ToLower(strings.TrimSpace(op.Path))
	switch path {
	case "active":
		var active bool
		if err := json.Unmarshal(op.Value, &active); err != nil {
			return service.ErrInvalidSCIMUser
		}
		body.Active = &active
	case "displayname":
		var value string
		if err := json.Unmarshal(op.Value, &value); err != nil {
			return service.ErrInvalidSCIMUser
		}
		body.DisplayName = value
	case "username":
		var value string
		if err := json.Unmarshal(op.Value, &value); err != nil {
			return service.ErrInvalidSCIMUser
		}
		body.UserName = value
	case "groups":
		var groups []SCIMGroupBody
		if err := json.Unmarshal(op.Value, &groups); err != nil {
			return service.ErrInvalidSCIMUser
		}
		body.Groups = groups
	default:
		return service.ErrInvalidSCIMUser
	}
	return nil
}

func externalIDFromFilter(filter string) string {
	match := externalIDFilterRE.FindStringSubmatch(filter)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func toSCIMHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidSCIMUser):
		return huma.Error400BadRequest("invalid scim user")
	case errors.Is(err, service.ErrUnauthorized):
		return huma.Error404NotFound("scim user not found")
	default:
		return huma.Error500InternalServerError(fmt.Sprintf("scim operation failed: %v", err))
	}
}
```

#### `backend/internal/api/tenants.go`

```go
package api

import (
	"context"
	"net/http"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type TenantBody struct {
	ID          int64    `json:"id" example:"1"`
	Slug        string   `json:"slug" example:"acme"`
	DisplayName string   `json:"displayName" example:"acme"`
	Roles       []string `json:"roles,omitempty" example:"todo_user"`
	Default     bool     `json:"default" example:"true"`
	Selected    bool     `json:"selected" example:"true"`
}

type ListTenantsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
}

type ListTenantsBody struct {
	Items         []TenantBody `json:"items"`
	ActiveTenant  *TenantBody  `json:"activeTenant,omitempty"`
	DefaultTenant *TenantBody  `json:"defaultTenant,omitempty"`
}

type ListTenantsOutput struct {
	Body ListTenantsBody
}

type SelectTenantInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          struct {
		TenantSlug string `json:"tenantSlug" example:"acme"`
	}
}

type SelectTenantOutput struct {
	Body struct {
		ActiveTenant TenantBody `json:"activeTenant"`
	}
}

func registerTenantRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listTenants",
		Method:      http.MethodGet,
		Path:        "/api/v1/tenants",
		Summary:     "現在の user が利用できる tenants を返す",
		Tags:        []string{"tenants"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ListTenantsInput) (*ListTenantsOutput, error) {
		_, authCtx, err := currentSessionAuthContext(ctx, deps, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPError(err)
		}

		out := &ListTenantsOutput{}
		out.Body.Items = toTenantBodies(authCtx.Tenants)
		if authCtx.ActiveTenant != nil {
			body := toTenantBody(*authCtx.ActiveTenant)
			out.Body.ActiveTenant = &body
		}
		if authCtx.DefaultTenant != nil {
			body := toTenantBody(*authCtx.DefaultTenant)
			out.Body.DefaultTenant = &body
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "selectTenant",
		Method:      http.MethodPost,
		Path:        "/api/v1/session/tenant",
		Summary:     "現在の session の active tenant を切り替える",
		Tags:        []string{"tenants"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *SelectTenantInput) (*SelectTenantOutput, error) {
		current, err := deps.SessionService.CurrentSessionWithCSRF(ctx, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, toHTTPError(err)
		}
		if deps.AuthzService == nil {
			return nil, huma.Error503ServiceUnavailable("tenant auth is not configured")
		}

		tenant, err := deps.AuthzService.SelectTenant(ctx, current.User, input.Body.TenantSlug)
		if err != nil {
			return nil, toHTTPError(err)
		}
		if err := deps.SessionService.SetActiveTenant(ctx, input.SessionCookie.Value, input.CSRFToken, tenant.ID); err != nil {
			return nil, toHTTPError(err)
		}

		out := &SelectTenantOutput{}
		out.Body.ActiveTenant = toTenantBody(tenant)
		return out, nil
	})
}

func toTenantBodies(items []service.TenantAccess) []TenantBody {
	out := make([]TenantBody, 0, len(items))
	for _, item := range items {
		out = append(out, toTenantBody(item))
	}
	return out
}

func toTenantBody(item service.TenantAccess) TenantBody {
	return TenantBody{
		ID:          item.ID,
		Slug:        item.Slug,
		DisplayName: item.DisplayName,
		Roles:       item.Roles,
		Default:     item.Default,
		Selected:    item.Selected,
	}
}
```

#### `backend/internal/app/app.go`

```go
package app

import (
	backendapi "example.com/haohao/backend/internal/api"
	"example.com/haohao/backend/internal/auth"
	"example.com/haohao/backend/internal/config"
	"example.com/haohao/backend/internal/middleware"
	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
)

type App struct {
	Router *gin.Engine
	API    huma.API
}

func New(cfg config.Config, sessionService *service.SessionService, oidcLoginService *service.OIDCLoginService, delegationService *service.DelegationService, provisioningService *service.ProvisioningService, authzService *service.AuthzService, bearerVerifier *auth.BearerVerifier) *App {
	router := gin.New()
	router.Use(
		gin.Logger(),
		gin.Recovery(),
		middleware.ExternalCORS("/api/external/", cfg.ExternalAllowedOrigins),
		middleware.ExternalAuth("/api/external/", bearerVerifier, authzService, "zitadel", cfg.ExternalExpectedAudience, cfg.ExternalRequiredScopePrefix, cfg.ExternalRequiredRole),
		middleware.SCIMAuth(cfg.SCIMBasePath+"/", bearerVerifier, cfg.SCIMBearerAudience, cfg.SCIMRequiredScope),
	)

	humaConfig := huma.DefaultConfig(cfg.AppName, cfg.AppVersion)
	humaConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"cookieAuth": {
			Type: "apiKey",
			In:   "cookie",
			Name: auth.SessionCookieName,
		},
		"bearerAuth": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		},
	}

	api := humagin.New(router, humaConfig)

	backendapi.Register(api, backendapi.Dependencies{
		SessionService:               sessionService,
		OIDCLoginService:             oidcLoginService,
		DelegationService:            delegationService,
		ProvisioningService:          provisioningService,
		AuthzService:                 authzService,
		AuthMode:                     cfg.AuthMode,
		SCIMBasePath:                 cfg.SCIMBasePath,
		FrontendBaseURL:              cfg.FrontendBaseURL,
		ZitadelIssuer:                cfg.ZitadelIssuer,
		ZitadelClientID:              cfg.ZitadelClientID,
		ZitadelPostLogoutRedirectURI: cfg.ZitadelPostLogoutRedirectURI,
		CookieSecure:                 cfg.CookieSecure,
		SessionTTL:                   cfg.SessionTTL,
	})

	return &App{
		Router: router,
		API:    api,
	}
}
```

#### `backend/internal/auth/bearer_verifier.go`

```go
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4/jwt"
)

var (
	ErrMissingBearerToken    = errors.New("missing bearer token")
	ErrInvalidBearerToken    = errors.New("invalid bearer token")
	ErrInvalidBearerIssuer   = errors.New("invalid bearer issuer")
	ErrInvalidBearerAudience = errors.New("invalid bearer audience")
	ErrInvalidBearerScope    = errors.New("invalid bearer scope")
	ErrInvalidBearerRole     = errors.New("invalid bearer role")
)

type BearerVerifier struct {
	issuer string
	keySet *oidc.RemoteKeySet
}

type BearerTokenClaims struct {
	jwt.Claims
	AuthorizedParty   string             `json:"azp,omitempty"`
	ClientID          string             `json:"client_id,omitempty"`
	Scope             spaceSeparatedList `json:"scope,omitempty"`
	Groups            claimStringList    `json:"groups,omitempty"`
	Roles             []string           `json:"-"`
	Email             string             `json:"email,omitempty"`
	Name              string             `json:"name,omitempty"`
	PreferredUsername string             `json:"preferred_username,omitempty"`
}

type bearerTokenClaimsJSON struct {
	jwt.Claims
	AuthorizedParty   string             `json:"azp,omitempty"`
	ClientID          string             `json:"client_id,omitempty"`
	Scope             spaceSeparatedList `json:"scope,omitempty"`
	Groups            claimStringList    `json:"groups,omitempty"`
	Email             string             `json:"email,omitempty"`
	Name              string             `json:"name,omitempty"`
	PreferredUsername string             `json:"preferred_username,omitempty"`
}

func (c *BearerTokenClaims) UnmarshalJSON(data []byte) error {
	var decoded bearerTokenClaimsJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	var rawClaims map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawClaims); err != nil {
		return err
	}

	c.Claims = decoded.Claims
	c.AuthorizedParty = strings.TrimSpace(decoded.AuthorizedParty)
	c.ClientID = strings.TrimSpace(decoded.ClientID)
	if c.AuthorizedParty == "" {
		c.AuthorizedParty = c.ClientID
	}
	c.Scope = decoded.Scope
	c.Groups = decoded.Groups
	c.Roles = extractZitadelRoleClaims(rawClaims)
	c.Email = decoded.Email
	c.Name = decoded.Name
	c.PreferredUsername = decoded.PreferredUsername

	return nil
}

func NewBearerVerifier(ctx context.Context, issuer string) (*BearerVerifier, error) {
	trimmedIssuer := strings.TrimRight(strings.TrimSpace(issuer), "/")
	if trimmedIssuer == "" {
		return nil, fmt.Errorf("issuer is required")
	}

	provider, err := oidc.NewProvider(ctx, trimmedIssuer)
	if err != nil {
		return nil, fmt.Errorf("discover oidc provider: %w", err)
	}

	var discovery struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := provider.Claims(&discovery); err != nil {
		return nil, fmt.Errorf("decode oidc discovery document: %w", err)
	}
	if strings.TrimSpace(discovery.JWKSURI) == "" {
		return nil, fmt.Errorf("jwks_uri missing from oidc discovery document")
	}

	return &BearerVerifier{
		issuer: trimmedIssuer,
		keySet: oidc.NewRemoteKeySet(ctx, discovery.JWKSURI),
	}, nil
}

func (v *BearerVerifier) Verify(ctx context.Context, rawToken, expectedAudience, requiredScopePrefix string) (BearerTokenClaims, error) {
	if strings.TrimSpace(rawToken) == "" {
		return BearerTokenClaims{}, ErrMissingBearerToken
	}
	if v == nil || v.keySet == nil {
		return BearerTokenClaims{}, fmt.Errorf("%w: verifier is not configured", ErrInvalidBearerToken)
	}

	payload, err := v.keySet.VerifySignature(ctx, rawToken)
	if err != nil {
		return BearerTokenClaims{}, fmt.Errorf("%w: verify signature: %v", ErrInvalidBearerToken, err)
	}

	var claims BearerTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return BearerTokenClaims{}, fmt.Errorf("%w: decode claims: %v", ErrInvalidBearerToken, err)
	}

	expected := jwt.Expected{
		Issuer: v.issuer,
		Time:   time.Now(),
	}
	if audience := strings.TrimSpace(expectedAudience); audience != "" {
		expected.AnyAudience = jwt.Audience{audience}
	}

	if err := claims.Claims.ValidateWithLeeway(expected, time.Minute); err != nil {
		switch {
		case errors.Is(err, jwt.ErrInvalidIssuer):
			return BearerTokenClaims{}, ErrInvalidBearerIssuer
		case errors.Is(err, jwt.ErrInvalidAudience):
			return BearerTokenClaims{}, ErrInvalidBearerAudience
		default:
			return BearerTokenClaims{}, fmt.Errorf("%w: %v", ErrInvalidBearerToken, err)
		}
	}

	if strings.TrimSpace(claims.Subject) == "" {
		return BearerTokenClaims{}, fmt.Errorf("%w: subject is required", ErrInvalidBearerToken)
	}
	if prefix := strings.TrimSpace(requiredScopePrefix); prefix != "" && !claims.HasScopePrefix(prefix) {
		return BearerTokenClaims{}, ErrInvalidBearerScope
	}

	return claims, nil
}

func (c BearerTokenClaims) ScopeValues() []string {
	return append([]string(nil), c.Scope...)
}

func (c BearerTokenClaims) GroupValues() []string {
	return append([]string(nil), c.Groups...)
}

func (c BearerTokenClaims) RoleValues() []string {
	return append([]string(nil), c.Roles...)
}

func (c BearerTokenClaims) HasScopePrefix(prefix string) bool {
	trimmedPrefix := strings.TrimSpace(prefix)
	if trimmedPrefix == "" {
		return true
	}

	for _, scope := range c.Scope {
		if scope == trimmedPrefix || strings.HasPrefix(scope, trimmedPrefix) {
			return true
		}
	}

	return false
}

func (c BearerTokenClaims) HasScope(scope string) bool {
	needle := strings.TrimSpace(scope)
	if needle == "" {
		return true
	}

	for _, item := range c.Scope {
		if item == needle {
			return true
		}
	}
	return false
}

func extractZitadelRoleClaims(rawClaims map[string]json.RawMessage) []string {
	roleSet := make(map[string]struct{})
	for name, raw := range rawClaims {
		if !isZitadelRoleClaim(name) {
			continue
		}

		for _, role := range roleNamesFromClaim(raw) {
			roleSet[role] = struct{}{}
		}
	}

	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}

func isZitadelRoleClaim(name string) bool {
	return name == "urn:zitadel:iam:org:project:roles" ||
		(strings.HasPrefix(name, "urn:zitadel:iam:org:project:") && strings.HasSuffix(name, ":roles"))
}

func roleNamesFromClaim(raw json.RawMessage) []string {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err == nil {
		roles := make([]string, 0, len(object))
		for role := range object {
			if trimmed := strings.TrimSpace(role); trimmed != "" {
				roles = append(roles, trimmed)
			}
		}
		return roles
	}

	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		roles := make([]string, 0, len(many))
		for _, role := range many {
			if trimmed := strings.TrimSpace(role); trimmed != "" {
				roles = append(roles, trimmed)
			}
		}
		return roles
	}

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if trimmed := strings.TrimSpace(single); trimmed != "" {
			return []string{trimmed}
		}
	}

	return nil
}

type spaceSeparatedList []string

func (s *spaceSeparatedList) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*s = append((*s)[:0], strings.Fields(single)...)
		return nil
	}

	var many []string
	if err := json.Unmarshal(data, &many); err == nil {
		items := make([]string, 0, len(many))
		for _, item := range many {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				items = append(items, trimmed)
			}
		}
		*s = items
		return nil
	}

	return fmt.Errorf("unsupported scope claim format")
}

type claimStringList []string

func (s *claimStringList) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		single = strings.TrimSpace(single)
		if single == "" {
			*s = nil
			return nil
		}
		*s = []string{single}
		return nil
	}

	var many []string
	if err := json.Unmarshal(data, &many); err == nil {
		items := make([]string, 0, len(many))
		for _, item := range many {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				items = append(items, trimmed)
			}
		}
		*s = items
		return nil
	}

	return fmt.Errorf("unsupported string list claim format")
}
```

#### `backend/internal/auth/delegation_state_store.go`

```go
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrDelegationStateNotFound = errors.New("delegation state not found")

type DelegationStateRecord struct {
	UserID         int64  `json:"userId"`
	TenantID       int64  `json:"tenantId"`
	ResourceServer string `json:"resourceServer"`
	CodeVerifier   string `json:"codeVerifier"`
	SessionHash    string `json:"sessionHash"`
}

type DelegationStateStore struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

func NewDelegationStateStore(client *redis.Client, ttl time.Duration) *DelegationStateStore {
	return &DelegationStateStore{
		client: client,
		prefix: "delegation-state:",
		ttl:    ttl,
	}
}

func (s *DelegationStateStore) Create(ctx context.Context, userID, tenantID int64, resourceServer, sessionHash string) (string, DelegationStateRecord, error) {
	state, err := randomToken(32)
	if err != nil {
		return "", DelegationStateRecord{}, err
	}

	codeVerifier, err := randomToken(32)
	if err != nil {
		return "", DelegationStateRecord{}, err
	}

	record := DelegationStateRecord{
		UserID:         userID,
		TenantID:       tenantID,
		ResourceServer: resourceServer,
		CodeVerifier:   codeVerifier,
		SessionHash:    sessionHash,
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return "", DelegationStateRecord{}, err
	}

	if err := s.client.Set(ctx, s.prefix+state, payload, s.ttl).Err(); err != nil {
		return "", DelegationStateRecord{}, fmt.Errorf("save delegation state: %w", err)
	}

	return state, record, nil
}

func (s *DelegationStateStore) Consume(ctx context.Context, state string) (DelegationStateRecord, error) {
	raw, err := s.client.GetDel(ctx, s.prefix+state).Bytes()
	if errors.Is(err, redis.Nil) {
		return DelegationStateRecord{}, ErrDelegationStateNotFound
	}
	if err != nil {
		return DelegationStateRecord{}, fmt.Errorf("consume delegation state: %w", err)
	}

	var record DelegationStateRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return DelegationStateRecord{}, fmt.Errorf("decode delegation state: %w", err)
	}

	return record, nil
}
```

#### `backend/internal/auth/session_store.go`

```go
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrSessionNotFound = errors.New("session not found")

type SessionRecord struct {
	UserID              int64  `json:"userId"`
	CSRFToken           string `json:"csrfToken"`
	ProviderIDTokenHint string `json:"providerIdTokenHint,omitempty"`
	ActiveTenantID      int64  `json:"activeTenantId,omitempty"`
}

type SessionStore struct {
	client          *redis.Client
	prefix          string
	userIndexPrefix string
	ttl             time.Duration
}

func NewSessionStore(client *redis.Client, ttl time.Duration) *SessionStore {
	return &SessionStore{
		client:          client,
		prefix:          "session:",
		userIndexPrefix: "session-user:",
		ttl:             ttl,
	}
}

func (s *SessionStore) Create(ctx context.Context, userID int64) (string, string, error) {
	return s.CreateWithProviderHint(ctx, userID, "")
}

func (s *SessionStore) CreateWithProviderHint(ctx context.Context, userID int64, providerIDTokenHint string) (string, string, error) {
	sessionID, err := randomToken(32)
	if err != nil {
		return "", "", err
	}

	csrfToken, err := randomToken(32)
	if err != nil {
		return "", "", err
	}

	record := SessionRecord{
		UserID:              userID,
		CSRFToken:           csrfToken,
		ProviderIDTokenHint: providerIDTokenHint,
	}
	if err := s.save(ctx, sessionID, record, s.ttl); err != nil {
		return "", "", err
	}

	return sessionID, csrfToken, nil
}

func (s *SessionStore) Get(ctx context.Context, sessionID string) (SessionRecord, error) {
	record, _, err := s.loadWithTTL(ctx, sessionID)
	return record, err
}

func (s *SessionStore) Delete(ctx context.Context, sessionID string) error {
	record, _, err := s.loadWithTTL(ctx, sessionID)
	if err != nil && !errors.Is(err, ErrSessionNotFound) {
		return err
	}

	if err := s.client.Del(ctx, s.key(sessionID)).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	if err == nil {
		_ = s.client.SRem(ctx, s.userIndexKey(record.UserID), sessionID).Err()
	}
	return nil
}

func (s *SessionStore) ReissueCSRF(ctx context.Context, sessionID string) (string, error) {
	record, ttl, err := s.loadWithTTL(ctx, sessionID)
	if err != nil {
		return "", err
	}

	csrfToken, err := randomToken(32)
	if err != nil {
		return "", err
	}

	record.CSRFToken = csrfToken
	if err := s.save(ctx, sessionID, record, ttl); err != nil {
		return "", err
	}

	return csrfToken, nil
}

func (s *SessionStore) Rotate(ctx context.Context, sessionID string) (string, string, error) {
	record, _, err := s.loadWithTTL(ctx, sessionID)
	if err != nil {
		return "", "", err
	}

	newSessionID, err := randomToken(32)
	if err != nil {
		return "", "", err
	}

	newCSRFToken, err := randomToken(32)
	if err != nil {
		return "", "", err
	}

	record.CSRFToken = newCSRFToken
	if err := s.save(ctx, newSessionID, record, s.ttl); err != nil {
		return "", "", err
	}
	if err := s.Delete(ctx, sessionID); err != nil {
		return "", "", err
	}

	return newSessionID, newCSRFToken, nil
}

func (s *SessionStore) SetActiveTenant(ctx context.Context, sessionID string, tenantID int64) error {
	record, ttl, err := s.loadWithTTL(ctx, sessionID)
	if err != nil {
		return err
	}

	record.ActiveTenantID = tenantID
	if err := s.save(ctx, sessionID, record, ttl); err != nil {
		return err
	}

	return nil
}

func (s *SessionStore) DeleteUserSessions(ctx context.Context, userID int64) error {
	indexKey := s.userIndexKey(userID)
	sessionIDs, err := s.client.SMembers(ctx, indexKey).Result()
	if err != nil {
		return fmt.Errorf("list user sessions: %w", err)
	}

	if len(sessionIDs) > 0 {
		keys := make([]string, 0, len(sessionIDs)+1)
		for _, sessionID := range sessionIDs {
			keys = append(keys, s.key(sessionID))
		}
		keys = append(keys, indexKey)
		if err := s.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("delete user sessions: %w", err)
		}
		return nil
	}

	if err := s.client.Del(ctx, indexKey).Err(); err != nil {
		return fmt.Errorf("delete user session index: %w", err)
	}
	return nil
}

func (s *SessionStore) key(sessionID string) string {
	return s.prefix + sessionID
}

func (s *SessionStore) userIndexKey(userID int64) string {
	return s.userIndexPrefix + strconv.FormatInt(userID, 10)
}

func (s *SessionStore) loadWithTTL(ctx context.Context, sessionID string) (SessionRecord, time.Duration, error) {
	raw, err := s.client.Get(ctx, s.key(sessionID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return SessionRecord{}, 0, ErrSessionNotFound
	}
	if err != nil {
		return SessionRecord{}, 0, fmt.Errorf("get session: %w", err)
	}

	var record SessionRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return SessionRecord{}, 0, fmt.Errorf("decode session: %w", err)
	}

	ttl, err := s.client.TTL(ctx, s.key(sessionID)).Result()
	if err != nil {
		return SessionRecord{}, 0, fmt.Errorf("get session ttl: %w", err)
	}
	if ttl <= 0 {
		ttl = s.ttl
	}

	return record, ttl, nil
}

func (s *SessionStore) save(ctx context.Context, sessionID string, record SessionRecord, ttl time.Duration) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}

	if err := s.client.Set(ctx, s.key(sessionID), payload, ttl).Err(); err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	if err := s.client.SAdd(ctx, s.userIndexKey(record.UserID), sessionID).Err(); err != nil {
		return fmt.Errorf("index session by user: %w", err)
	}
	if err := s.client.Expire(ctx, s.userIndexKey(record.UserID), ttl).Err(); err != nil {
		return fmt.Errorf("expire user session index: %w", err)
	}

	return nil
}

func randomToken(numBytes int) (string, error) {
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
```

#### `backend/internal/auth/session_store_test.go`

```go
package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestSessionStoreIndexesAndDeletesByUser(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store := NewSessionStore(client, time.Hour)

	sessionID, _, err := store.Create(ctx, 42)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := store.Get(ctx, sessionID); err != nil {
		t.Fatalf("Get() after create error = %v", err)
	}

	if err := store.DeleteUserSessions(ctx, 42); err != nil {
		t.Fatalf("DeleteUserSessions() error = %v", err)
	}
	if _, err := store.Get(ctx, sessionID); err != ErrSessionNotFound {
		t.Fatalf("Get() after DeleteUserSessions error = %v, want %v", err, ErrSessionNotFound)
	}
}

func TestSessionStoreRotateUpdatesUserIndex(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store := NewSessionStore(client, time.Hour)

	oldSessionID, _, err := store.Create(ctx, 42)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	newSessionID, _, err := store.Rotate(ctx, oldSessionID)
	if err != nil {
		t.Fatalf("Rotate() error = %v", err)
	}

	if _, err := store.Get(ctx, oldSessionID); err != ErrSessionNotFound {
		t.Fatalf("old session Get() error = %v, want %v", err, ErrSessionNotFound)
	}
	if _, err := store.Get(ctx, newSessionID); err != nil {
		t.Fatalf("new session Get() error = %v", err)
	}
	if err := store.DeleteUserSessions(ctx, 42); err != nil {
		t.Fatalf("DeleteUserSessions() error = %v", err)
	}
	if _, err := store.Get(ctx, newSessionID); err != ErrSessionNotFound {
		t.Fatalf("new session after DeleteUserSessions error = %v, want %v", err, ErrSessionNotFound)
	}
}
```

#### `backend/internal/config/config.go`

```go
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName                      string
	AppVersion                   string
	HTTPPort                     int
	AppBaseURL                   string
	FrontendBaseURL              string
	DatabaseURL                  string
	AuthMode                     string
	ZitadelIssuer                string
	ZitadelClientID              string
	ZitadelClientSecret          string
	ZitadelRedirectURI           string
	ZitadelPostLogoutRedirectURI string
	ZitadelScopes                string
	ExternalExpectedAudience     string
	ExternalRequiredScopePrefix  string
	ExternalRequiredRole         string
	ExternalAllowedOrigins       []string
	DownstreamTokenEncryptionKey string
	DownstreamTokenKeyVersion    int
	DownstreamRefreshTokenTTL    time.Duration
	DownstreamAccessTokenSkew    time.Duration
	DownstreamDefaultScopes      string
	SCIMBasePath                 string
	SCIMBearerAudience           string
	SCIMRequiredScope            string
	SCIMReconcileCron            string
	RedisAddr                    string
	RedisPassword                string
	RedisDB                      int
	LoginStateTTL                time.Duration
	SessionTTL                   time.Duration
	CookieSecure                 bool
}

func Load() (Config, error) {
	sessionTTL, err := time.ParseDuration(getEnv("SESSION_TTL", "24h"))
	if err != nil {
		return Config{}, err
	}
	loginStateTTL, err := time.ParseDuration(getEnv("LOGIN_STATE_TTL", "10m"))
	if err != nil {
		return Config{}, err
	}
	downstreamRefreshTokenTTL, err := time.ParseDuration(getEnv("DOWNSTREAM_REFRESH_TOKEN_TTL", "2160h"))
	if err != nil {
		return Config{}, err
	}
	downstreamAccessTokenSkew, err := time.ParseDuration(getEnv("DOWNSTREAM_ACCESS_TOKEN_SKEW", "30s"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppName:                      getEnv("APP_NAME", "HaoHao API"),
		AppVersion:                   getEnv("APP_VERSION", "0.1.0"),
		HTTPPort:                     getEnvInt("HTTP_PORT", 8080),
		AppBaseURL:                   strings.TrimRight(getEnv("APP_BASE_URL", "http://127.0.0.1:8080"), "/"),
		FrontendBaseURL:              strings.TrimRight(getEnv("FRONTEND_BASE_URL", "http://127.0.0.1:5173"), "/"),
		DatabaseURL:                  getEnv("DATABASE_URL", ""),
		AuthMode:                     getEnv("AUTH_MODE", "local"),
		ZitadelIssuer:                strings.TrimRight(getEnv("ZITADEL_ISSUER", ""), "/"),
		ZitadelClientID:              getEnv("ZITADEL_CLIENT_ID", ""),
		ZitadelClientSecret:          getEnv("ZITADEL_CLIENT_SECRET", ""),
		ZitadelRedirectURI:           getEnv("ZITADEL_REDIRECT_URI", "http://127.0.0.1:8080/api/v1/auth/callback"),
		ZitadelPostLogoutRedirectURI: getEnv("ZITADEL_POST_LOGOUT_REDIRECT_URI", "http://127.0.0.1:5173/login"),
		ZitadelScopes:                getEnv("ZITADEL_SCOPES", "openid profile email"),
		ExternalExpectedAudience:     getEnv("EXTERNAL_EXPECTED_AUDIENCE", "haohao-external"),
		ExternalRequiredScopePrefix:  getEnv("EXTERNAL_REQUIRED_SCOPE_PREFIX", ""),
		ExternalRequiredRole:         getEnv("EXTERNAL_REQUIRED_ROLE", "external_api_user"),
		ExternalAllowedOrigins:       getEnvCSV("EXTERNAL_ALLOWED_ORIGINS"),
		DownstreamTokenEncryptionKey: getEnv("DOWNSTREAM_TOKEN_ENCRYPTION_KEY", ""),
		DownstreamTokenKeyVersion:    getEnvInt("DOWNSTREAM_TOKEN_KEY_VERSION", 1),
		DownstreamRefreshTokenTTL:    downstreamRefreshTokenTTL,
		DownstreamAccessTokenSkew:    downstreamAccessTokenSkew,
		DownstreamDefaultScopes:      getEnv("DOWNSTREAM_DEFAULT_SCOPES", "offline_access"),
		SCIMBasePath:                 strings.TrimRight(getEnv("SCIM_BASE_PATH", "/api/scim/v2"), "/"),
		SCIMBearerAudience:           getEnv("SCIM_BEARER_AUDIENCE", "scim-provisioning"),
		SCIMRequiredScope:            getEnv("SCIM_REQUIRED_SCOPE", "scim:provision"),
		SCIMReconcileCron:            getEnv("SCIM_RECONCILE_CRON", "0 3 * * *"),
		RedisAddr:                    getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:                getEnv("REDIS_PASSWORD", ""),
		RedisDB:                      getEnvInt("REDIS_DB", 0),
		LoginStateTTL:                loginStateTTL,
		SessionTTL:                   sessionTTL,
		CookieSecure:                 getEnvBool("COOKIE_SECURE", false),
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

func getEnvCSV(key string) []string {
	value := strings.TrimSpace(getEnv(key, ""))
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}

	return items
}
```

#### `backend/internal/jobs/provisioning_reconcile.go`

```go
package jobs

import (
	"context"
	"fmt"

	db "example.com/haohao/backend/internal/db"
	"example.com/haohao/backend/internal/service"
)

type ProvisioningReconcileJob struct {
	queries           *db.Queries
	sessionService    *service.SessionService
	delegationService *service.DelegationService
}

func NewProvisioningReconcileJob(queries *db.Queries, sessionService *service.SessionService, delegationService *service.DelegationService) *ProvisioningReconcileJob {
	return &ProvisioningReconcileJob{
		queries:           queries,
		sessionService:    sessionService,
		delegationService: delegationService,
	}
}

func (j *ProvisioningReconcileJob) RunOnce(ctx context.Context) error {
	if j == nil || j.queries == nil {
		return nil
	}

	users, err := j.queries.ListDeactivatedUsersWithActiveGrants(ctx)
	if err != nil {
		return fmt.Errorf("list deactivated users with active grants: %w", err)
	}

	var failed int32
	for _, user := range users {
		if j.sessionService != nil {
			if err := j.sessionService.DeleteUserSessions(ctx, user.ID); err != nil {
				failed++
				continue
			}
		}
		if j.delegationService != nil {
			if err := j.delegationService.DeleteAllGrantsForUser(ctx, user.ID); err != nil {
				failed++
				continue
			}
		}
		if err := j.queries.DeleteOAuthUserGrantsByUserID(ctx, user.ID); err != nil {
			failed++
		}
	}

	if err := j.queries.UpsertProvisioningSyncState(ctx, db.UpsertProvisioningSyncStateParams{
		Source:      "scim",
		FailedCount: failed,
	}); err != nil {
		return fmt.Errorf("update provisioning reconcile state: %w", err)
	}

	return nil
}
```

#### `backend/internal/middleware/external_auth.go`

```go
package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"example.com/haohao/backend/internal/auth"
	"example.com/haohao/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func ExternalCORS(pathPrefix string, allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, pathPrefix) {
			c.Next()
			return
		}

		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" && originAllowed(origin, allowed) {
			header := c.Writer.Header()
			header.Set("Access-Control-Allow-Origin", origin)
			header.Add("Vary", "Origin")
			header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Tenant-ID")
			header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			header.Set("Access-Control-Max-Age", "600")
		}

		if c.Request.Method == http.MethodOptions {
			if origin == "" || !originAllowed(origin, allowed) {
				writeProblem(c, http.StatusForbidden, "origin is not allowed")
				return
			}

			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}

		c.Next()
	}
}

func ExternalAuth(pathPrefix string, verifier *auth.BearerVerifier, authzService *service.AuthzService, providerName, expectedAudience, requiredScopePrefix, requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, pathPrefix) || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		if verifier == nil || authzService == nil {
			writeProblem(c, http.StatusServiceUnavailable, "external bearer auth is not configured")
			return
		}

		rawToken, err := bearerTokenFromHeader(c.GetHeader("Authorization"))
		if err != nil {
			writeBearerProblem(c, http.StatusUnauthorized, err.Error())
			return
		}

		claims, err := verifier.Verify(c.Request.Context(), rawToken, expectedAudience, requiredScopePrefix)
		if err != nil {
			status := http.StatusUnauthorized
			switch {
			case err == auth.ErrInvalidBearerScope:
				status = http.StatusForbidden
			case err == auth.ErrInvalidBearerAudience, err == auth.ErrInvalidBearerIssuer, err == auth.ErrMissingBearerToken:
				status = http.StatusUnauthorized
			}
			writeBearerProblem(c, status, err.Error())
			return
		}

		authCtx, err := authzService.AuthContextFromBearerWithTenant(c.Request.Context(), providerName, claims, c.GetHeader("X-Tenant-ID"))
		if err != nil {
			if err == service.ErrUnauthorized {
				writeBearerProblem(c, http.StatusForbidden, "tenant access denied")
				return
			}
			writeProblem(c, http.StatusInternalServerError, "failed to build auth context")
			return
		}
		if !authCtx.HasProviderRole(requiredRole) {
			writeBearerProblem(c, http.StatusForbidden, auth.ErrInvalidBearerRole.Error())
			return
		}

		c.Request = c.Request.WithContext(service.ContextWithAuthContext(c.Request.Context(), authCtx))
		c.Next()
	}
}

func SCIMAuth(pathPrefix string, verifier *auth.BearerVerifier, expectedAudience, requiredScope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, pathPrefix) || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		if verifier == nil {
			writeProblem(c, http.StatusServiceUnavailable, "scim bearer auth is not configured")
			return
		}

		rawToken, err := bearerTokenFromHeader(c.GetHeader("Authorization"))
		if err != nil {
			writeBearerProblem(c, http.StatusUnauthorized, err.Error())
			return
		}

		claims, err := verifier.Verify(c.Request.Context(), rawToken, expectedAudience, "")
		if err != nil {
			writeBearerProblem(c, http.StatusUnauthorized, err.Error())
			return
		}
		if !claims.HasScope(requiredScope) {
			writeBearerProblem(c, http.StatusForbidden, auth.ErrInvalidBearerScope.Error())
			return
		}

		c.Next()
	}
}

func bearerTokenFromHeader(header string) (string, error) {
	trimmed := strings.TrimSpace(header)
	if trimmed == "" {
		return "", auth.ErrMissingBearerToken
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(trimmed, prefix) {
		return "", fmt.Errorf("%w: authorization header must use Bearer", auth.ErrInvalidBearerToken)
	}

	token := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	if token == "" {
		return "", auth.ErrMissingBearerToken
	}

	return token, nil
}

func originAllowed(origin string, allowed map[string]struct{}) bool {
	if len(allowed) == 0 {
		return false
	}
	_, ok := allowed[origin]
	return ok
}

func writeBearerProblem(c *gin.Context, status int, detail string) {
	c.Header("WWW-Authenticate", `Bearer realm="haohao-external"`)
	writeProblem(c, status, detail)
}

func writeProblem(c *gin.Context, status int, detail string) {
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(status, gin.H{
		"title":  http.StatusText(status),
		"status": status,
		"detail": detail,
	})
}
```

#### `backend/internal/service/authz_service.go`

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"example.com/haohao/backend/internal/auth"
	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthContext struct {
	AuthenticatedBy string
	Provider        string
	Subject         string
	AuthorizedParty string
	Scopes          []string
	Groups          []string
	Roles           []string
	User            *User
	DefaultTenant   *TenantAccess
	ActiveTenant    *TenantAccess
	Tenants         []TenantAccess
}

type TenantAccess struct {
	ID          int64
	Slug        string
	DisplayName string
	Roles       []string
	Default     bool
	Selected    bool
}

type TenantRoleClaim struct {
	TenantSlug string
	RoleCode   string
}

type authContextKey struct{}

type AuthzService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewAuthzService(pool *pgxpool.Pool, queries *db.Queries) *AuthzService {
	return &AuthzService{
		pool:    pool,
		queries: queries,
	}
}

func ContextWithAuthContext(ctx context.Context, authCtx AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey{}, authCtx)
}

func AuthContextFromContext(ctx context.Context) (AuthContext, bool) {
	authCtx, ok := ctx.Value(authContextKey{}).(AuthContext)
	return authCtx, ok
}

func (a AuthContext) HasRole(role string) bool {
	needle := strings.ToLower(strings.TrimSpace(role))
	if needle == "" {
		return true
	}

	for _, item := range append(append([]string{}, a.Roles...), a.Groups...) {
		if strings.ToLower(strings.TrimSpace(item)) == needle {
			return true
		}
	}

	return false
}

func (a AuthContext) HasProviderRole(role string) bool {
	needle := strings.ToLower(strings.TrimSpace(role))
	if needle == "" {
		return true
	}

	for _, item := range a.Groups {
		if strings.ToLower(strings.TrimSpace(item)) == needle {
			return true
		}
	}

	return false
}

func (s *AuthzService) SyncGlobalRoles(ctx context.Context, userID int64, providerGroups []string) ([]string, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return nil, fmt.Errorf("authz service is not configured")
	}

	roleCodes := normalizeGlobalRoleCodes(providerGroups)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin role sync transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	if len(roleCodes) == 0 {
		if err := qtx.DeleteUserRolesByUserID(ctx, userID); err != nil {
			return nil, fmt.Errorf("delete user roles: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit empty role sync transaction: %w", err)
		}
		return nil, nil
	}

	roles, err := qtx.GetRolesByCode(ctx, roleCodes)
	if err != nil {
		return nil, fmt.Errorf("load roles by code: %w", err)
	}

	roleIDs := make([]int64, 0, len(roles))
	syncedCodes := make([]string, 0, len(roles))
	for _, role := range roles {
		roleIDs = append(roleIDs, role.ID)
		syncedCodes = append(syncedCodes, role.Code)
	}

	if err := qtx.DeleteUserRolesExcluding(ctx, db.DeleteUserRolesExcludingParams{
		UserID:  userID,
		Column2: roleIDs,
	}); err != nil {
		return nil, fmt.Errorf("delete stale user roles: %w", err)
	}

	for _, roleID := range roleIDs {
		if err := qtx.AssignUserRole(ctx, db.AssignUserRoleParams{
			UserID: userID,
			RoleID: roleID,
		}); err != nil {
			return nil, fmt.Errorf("assign user role: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit role sync transaction: %w", err)
	}

	return syncedCodes, nil
}

func (s *AuthzService) AuthContextFromBearer(ctx context.Context, provider string, claims auth.BearerTokenClaims) (AuthContext, error) {
	return s.AuthContextFromBearerWithTenant(ctx, provider, claims, "")
}

func (s *AuthzService) AuthContextFromBearerWithTenant(ctx context.Context, provider string, claims auth.BearerTokenClaims, requestedTenantSlug string) (AuthContext, error) {
	authCtx := AuthContext{
		AuthenticatedBy: "bearer",
		Provider:        strings.ToLower(strings.TrimSpace(provider)),
		Subject:         strings.TrimSpace(claims.Subject),
		AuthorizedParty: strings.TrimSpace(claims.AuthorizedParty),
		Scopes:          claims.ScopeValues(),
		Groups:          mergeClaimValues(claims.GroupValues(), claims.RoleValues()),
	}

	if s == nil || s.queries == nil || authCtx.Provider == "" || authCtx.Subject == "" {
		return authCtx, nil
	}

	user, err := s.queries.GetUserByProviderSubject(ctx, db.GetUserByProviderSubjectParams{
		Provider: authCtx.Provider,
		Subject:  authCtx.Subject,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authCtx, nil
		}
		return AuthContext{}, fmt.Errorf("lookup user by provider subject: %w", err)
	}
	if user.DeactivatedAt.Valid {
		return AuthContext{}, ErrUnauthorized
	}

	localUser := dbUser(user.ID, user.PublicID.String(), user.Email, user.DisplayName, user.DeactivatedAt, user.DefaultTenantID)
	authCtx.User = &localUser

	if len(authCtx.Groups) > 0 {
		roleCodes, err := s.SyncGlobalRoles(ctx, localUser.ID, authCtx.Groups)
		if err != nil {
			return AuthContext{}, fmt.Errorf("sync global roles from bearer claims: %w", err)
		}
		authCtx.Roles = roleCodes
		if _, err := s.SyncTenantMemberships(ctx, localUser.ID, "provider_claim", authCtx.Groups); err != nil {
			return AuthContext{}, fmt.Errorf("sync tenant memberships from bearer claims: %w", err)
		}
	} else {
		roleCodes, err := s.queries.ListRoleCodesByUserID(ctx, localUser.ID)
		if err != nil {
			return AuthContext{}, fmt.Errorf("list local roles by user id: %w", err)
		}
		authCtx.Roles = roleCodes
	}

	tenants, active, def, err := s.resolveTenantAccess(ctx, localUser.ID, localUser.DefaultTenantID, requestedTenantSlug)
	if err != nil {
		return AuthContext{}, err
	}
	authCtx.Tenants = tenants
	authCtx.ActiveTenant = active
	authCtx.DefaultTenant = def

	return authCtx, nil
}

var supportedGlobalRoles = map[string]struct{}{
	"docs_reader":       {},
	"external_api_user": {},
	"todo_user":         {},
}

var supportedTenantRoles = map[string]struct{}{
	"docs_reader": {},
	"todo_user":   {},
}

func (s *AuthzService) BuildBrowserContext(ctx context.Context, user User, activeTenantID *int64) (AuthContext, error) {
	if s == nil || s.queries == nil {
		return AuthContext{}, fmt.Errorf("authz service is not configured")
	}

	roleCodes, err := s.queries.ListRoleCodesByUserID(ctx, user.ID)
	if err != nil {
		return AuthContext{}, fmt.Errorf("list local roles by user id: %w", err)
	}

	requestedSlug := ""
	if activeTenantID != nil {
		tenant, err := s.queries.GetTenantByID(ctx, *activeTenantID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return AuthContext{}, ErrUnauthorized
			}
			return AuthContext{}, fmt.Errorf("load active tenant: %w", err)
		}
		requestedSlug = tenant.Slug
	}

	tenants, active, def, err := s.resolveTenantAccess(ctx, user.ID, user.DefaultTenantID, requestedSlug)
	if err != nil {
		return AuthContext{}, err
	}

	return AuthContext{
		AuthenticatedBy: "session",
		Roles:           roleCodes,
		User:            &user,
		Tenants:         tenants,
		ActiveTenant:    active,
		DefaultTenant:   def,
	}, nil
}

func (s *AuthzService) SyncTenantMemberships(ctx context.Context, userID int64, source string, providerGroups []string) ([]TenantAccess, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return nil, fmt.Errorf("authz service is not configured")
	}

	claims := ParseTenantRoleClaims(providerGroups)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tenant sync transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	if err := qtx.DeleteTenantMembershipsByUserSource(ctx, db.DeleteTenantMembershipsByUserSourceParams{
		UserID: userID,
		Source: source,
	}); err != nil {
		return nil, fmt.Errorf("delete stale tenant memberships: %w", err)
	}

	if len(claims) > 0 {
		roleCodes := tenantRoleCodes(claims)
		roles, err := qtx.GetRolesByCode(ctx, roleCodes)
		if err != nil {
			return nil, fmt.Errorf("load tenant roles by code: %w", err)
		}
		roleIDByCode := make(map[string]int64, len(roles))
		for _, role := range roles {
			roleIDByCode[role.Code] = role.ID
		}

		for _, claim := range claims {
			roleID, ok := roleIDByCode[claim.RoleCode]
			if !ok {
				continue
			}
			tenant, err := qtx.UpsertTenantBySlug(ctx, db.UpsertTenantBySlugParams{
				Slug:        claim.TenantSlug,
				DisplayName: claim.TenantSlug,
			})
			if err != nil {
				return nil, fmt.Errorf("upsert tenant: %w", err)
			}
			if err := qtx.UpsertTenantMembership(ctx, db.UpsertTenantMembershipParams{
				UserID:   userID,
				TenantID: tenant.ID,
				RoleID:   roleID,
				Source:   source,
			}); err != nil {
				return nil, fmt.Errorf("upsert tenant membership: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tenant sync transaction: %w", err)
	}

	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load user after tenant sync: %w", err)
	}
	access, err := s.ListTenantAccess(ctx, userID, optionalPgInt8(user.DefaultTenantID))
	if err != nil {
		return nil, err
	}
	if !user.DefaultTenantID.Valid && len(access) > 0 {
		if _, err := s.queries.SetUserDefaultTenant(ctx, db.SetUserDefaultTenantParams{
			ID:              userID,
			DefaultTenantID: pgtype.Int8{Int64: access[0].ID, Valid: true},
		}); err != nil {
			return nil, fmt.Errorf("set default tenant: %w", err)
		}
		access[0].Default = true
	}
	return access, nil
}

func (s *AuthzService) ListTenantAccess(ctx context.Context, userID int64, defaultTenantID *int64) ([]TenantAccess, error) {
	rows, err := s.queries.ListTenantMembershipRowsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list tenant memberships: %w", err)
	}
	overrides, err := s.queries.ListTenantRoleOverridesByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list tenant role overrides: %w", err)
	}

	tenants := tenantAccessFromRows(rows, overrides)
	for i := range tenants {
		if defaultTenantID != nil && tenants[i].ID == *defaultTenantID {
			tenants[i].Default = true
		}
	}
	return tenants, nil
}

func (s *AuthzService) SelectTenant(ctx context.Context, user User, tenantSlug string) (TenantAccess, error) {
	tenants, active, _, err := s.resolveTenantAccess(ctx, user.ID, user.DefaultTenantID, tenantSlug)
	if err != nil {
		return TenantAccess{}, err
	}
	if active == nil {
		if len(tenants) == 0 {
			return TenantAccess{}, ErrUnauthorized
		}
		return tenants[0], nil
	}
	return *active, nil
}

func (s *AuthzService) resolveTenantAccess(ctx context.Context, userID int64, defaultTenantID *int64, requestedTenantSlug string) ([]TenantAccess, *TenantAccess, *TenantAccess, error) {
	tenants, err := s.ListTenantAccess(ctx, userID, defaultTenantID)
	if err != nil {
		return nil, nil, nil, err
	}

	var defaultTenant *TenantAccess
	var activeTenant *TenantAccess
	for i := range tenants {
		if tenants[i].Default {
			defaultTenant = &tenants[i]
			break
		}
	}
	if defaultTenant == nil && len(tenants) > 0 {
		tenants[0].Default = true
		defaultTenant = &tenants[0]
	}

	requested := strings.ToLower(strings.TrimSpace(requestedTenantSlug))
	if requested != "" {
		for i := range tenants {
			if tenants[i].Slug == requested {
				tenants[i].Selected = true
				activeTenant = &tenants[i]
				return tenants, activeTenant, defaultTenant, nil
			}
		}
		return nil, nil, nil, ErrUnauthorized
	}

	if defaultTenant != nil {
		for i := range tenants {
			if tenants[i].ID == defaultTenant.ID {
				tenants[i].Selected = true
				activeTenant = &tenants[i]
				break
			}
		}
	}
	return tenants, activeTenant, defaultTenant, nil
}

func normalizeGlobalRoleCodes(providerGroups []string) []string {
	set := make(map[string]struct{}, len(providerGroups))
	for _, group := range providerGroups {
		code := strings.ToLower(strings.TrimSpace(group))
		if _, ok := supportedGlobalRoles[code]; ok {
			set[code] = struct{}{}
		}
	}

	roleCodes := make([]string, 0, len(set))
	for code := range set {
		roleCodes = append(roleCodes, code)
	}
	sort.Strings(roleCodes)

	return roleCodes
}

func ParseTenantRoleClaims(providerGroups []string) []TenantRoleClaim {
	set := make(map[TenantRoleClaim]struct{})
	for _, group := range providerGroups {
		parts := strings.Split(strings.ToLower(strings.TrimSpace(group)), ":")
		if len(parts) != 3 || parts[0] != "tenant" {
			continue
		}
		tenantSlug := strings.TrimSpace(parts[1])
		roleCode := strings.TrimSpace(parts[2])
		if tenantSlug == "" || roleCode == "" {
			continue
		}
		if _, ok := supportedTenantRoles[roleCode]; !ok {
			continue
		}
		set[TenantRoleClaim{TenantSlug: tenantSlug, RoleCode: roleCode}] = struct{}{}
	}

	claims := make([]TenantRoleClaim, 0, len(set))
	for claim := range set {
		claims = append(claims, claim)
	}
	sort.Slice(claims, func(i, j int) bool {
		if claims[i].TenantSlug == claims[j].TenantSlug {
			return claims[i].RoleCode < claims[j].RoleCode
		}
		return claims[i].TenantSlug < claims[j].TenantSlug
	})
	return claims
}

func tenantRoleCodes(claims []TenantRoleClaim) []string {
	set := make(map[string]struct{}, len(claims))
	for _, claim := range claims {
		set[claim.RoleCode] = struct{}{}
	}
	codes := make([]string, 0, len(set))
	for code := range set {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func tenantAccessFromRows(rows []db.ListTenantMembershipRowsByUserIDRow, overrides []db.ListTenantRoleOverridesByUserIDRow) []TenantAccess {
	type tenantState struct {
		access TenantAccess
		roles  map[string]struct{}
	}

	byID := make(map[int64]*tenantState)
	for _, row := range rows {
		if !row.TenantActive || !row.MembershipActive {
			continue
		}
		state, ok := byID[row.TenantID]
		if !ok {
			state = &tenantState{
				access: TenantAccess{
					ID:          row.TenantID,
					Slug:        row.TenantSlug,
					DisplayName: row.TenantDisplayName,
				},
				roles: make(map[string]struct{}),
			}
			byID[row.TenantID] = state
		}
		state.roles[row.RoleCode] = struct{}{}
	}

	for _, override := range overrides {
		state, ok := byID[override.TenantID]
		if !ok {
			state = &tenantState{
				access: TenantAccess{
					ID:   override.TenantID,
					Slug: override.TenantSlug,
				},
				roles: make(map[string]struct{}),
			}
			byID[override.TenantID] = state
		}
		switch override.Effect {
		case "deny":
			delete(state.roles, override.RoleCode)
		case "allow":
			state.roles[override.RoleCode] = struct{}{}
		}
	}

	tenants := make([]TenantAccess, 0, len(byID))
	for _, state := range byID {
		if len(state.roles) == 0 {
			continue
		}
		state.access.Roles = make([]string, 0, len(state.roles))
		for role := range state.roles {
			state.access.Roles = append(state.access.Roles, role)
		}
		sort.Strings(state.access.Roles)
		tenants = append(tenants, state.access)
	}
	sort.Slice(tenants, func(i, j int) bool {
		return tenants[i].Slug < tenants[j].Slug
	})
	return tenants
}

func mergeClaimValues(values ...[]string) []string {
	set := make(map[string]struct{})
	for _, group := range values {
		for _, value := range group {
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				set[trimmed] = struct{}{}
			}
		}
	}

	merged := make([]string, 0, len(set))
	for value := range set {
		merged = append(merged, value)
	}
	sort.Strings(merged)

	return merged
}
```

#### `backend/internal/service/authz_service_test.go`

```go
package service

import (
	"reflect"
	"testing"

	db "example.com/haohao/backend/internal/db"
)

func TestParseTenantRoleClaims(t *testing.T) {
	got := ParseTenantRoleClaims([]string{
		"tenant:acme:todo_user",
		"tenant:acme:todo_user",
		"tenant:beta:docs_reader",
		"external_api_user",
		"tenant:acme:external_api_user",
		"tenant::todo_user",
		"tenant:bad",
	})

	want := []TenantRoleClaim{
		{TenantSlug: "acme", RoleCode: "todo_user"},
		{TenantSlug: "beta", RoleCode: "docs_reader"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseTenantRoleClaims() = %#v, want %#v", got, want)
	}
}

func TestTenantAccessFromRowsAppliesOverrides(t *testing.T) {
	rows := []db.ListTenantMembershipRowsByUserIDRow{
		{TenantID: 1, TenantSlug: "acme", TenantDisplayName: "Acme", TenantActive: true, RoleCode: "todo_user", MembershipActive: true},
		{TenantID: 1, TenantSlug: "acme", TenantDisplayName: "Acme", TenantActive: true, RoleCode: "docs_reader", MembershipActive: true},
	}

	got := tenantAccessFromRows(rows, []db.ListTenantRoleOverridesByUserIDRow{
		{TenantID: 1, TenantSlug: "acme", RoleCode: "todo_user", Effect: "deny"},
		{TenantID: 1, TenantSlug: "acme", RoleCode: "docs_reader", Effect: "allow"},
	})

	want := []TenantAccess{{
		ID:          1,
		Slug:        "acme",
		DisplayName: "Acme",
		Roles:       []string{"docs_reader"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tenant access = %#v, want %#v", got, want)
	}
}
```

#### `backend/internal/service/delegation_service.go`

```go
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"example.com/haohao/backend/internal/auth"
	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrDelegationNotConfigured       = errors.New("delegated auth is not configured")
	ErrDelegationUnsupportedResource = errors.New("unsupported downstream resource")
	ErrDelegationGrantNotFound       = errors.New("delegated grant not found")
	ErrDelegationInvalidState        = errors.New("invalid delegated auth state")
	ErrDelegationIdentityNotFound    = errors.New("delegated provider identity not found")
	ErrDelegationRefreshTokenMissing = errors.New("delegated refresh token missing")
)

type DelegationStatus struct {
	TenantID        int64
	ResourceServer  string
	Provider        string
	Connected       bool
	Scopes          []string
	GrantedAt       *time.Time
	LastRefreshedAt *time.Time
	RevokedAt       *time.Time
	LastErrorCode   string
}

type DelegatedAccessToken struct {
	AccessToken string
	ExpiresAt   *time.Time
	Scopes      []string
}

type DelegationVerifyResult struct {
	ResourceServer  string
	Connected       bool
	Scopes          []string
	AccessExpiresAt *time.Time
	RefreshedAt     *time.Time
}

type delegationResource struct {
	resourceServer string
	provider       string
	redirectURI    string
	scopes         []string
}

type DelegationService struct {
	queries       *db.Queries
	oauthClient   *auth.DelegatedOAuthClient
	stateStore    *auth.DelegationStateStore
	tokenStore    *auth.RefreshTokenStore
	appBaseURL    string
	defaultScopes []string
	refreshTTL    time.Duration
	accessSkew    time.Duration
}

func NewDelegationService(queries *db.Queries, oauthClient *auth.DelegatedOAuthClient, stateStore *auth.DelegationStateStore, tokenStore *auth.RefreshTokenStore, appBaseURL, defaultScopes string, refreshTTL, accessSkew time.Duration) *DelegationService {
	return &DelegationService{
		queries:       queries,
		oauthClient:   oauthClient,
		stateStore:    stateStore,
		tokenStore:    tokenStore,
		appBaseURL:    strings.TrimRight(appBaseURL, "/"),
		defaultScopes: normalizeScopeList(strings.Fields(defaultScopes)),
		refreshTTL:    refreshTTL,
		accessSkew:    accessSkew,
	}
}

func (s *DelegationService) ListIntegrations(ctx context.Context, user User) ([]DelegationStatus, error) {
	tenantID, err := delegationTenantID(user)
	if err != nil {
		return nil, err
	}
	return s.ListIntegrationsForTenant(ctx, user, tenantID)
}

func (s *DelegationService) ListIntegrationsForTenant(ctx context.Context, user User, tenantID int64) ([]DelegationStatus, error) {
	if err := s.requireConfigured(); err != nil {
		return nil, err
	}

	rows, err := s.queries.ListOAuthUserGrantsByUserID(ctx, db.ListOAuthUserGrantsByUserIDParams{
		UserID:   user.ID,
		TenantID: tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("list downstream grants: %w", err)
	}

	byResource := make(map[string]db.ListOAuthUserGrantsByUserIDRow, len(rows))
	for _, row := range rows {
		if row.Provider == "zitadel" {
			byResource[row.ResourceServer] = row
		}
	}

	statuses := make([]DelegationStatus, 0, 1)
	for _, resourceServer := range []string{"zitadel"} {
		resource, err := s.resource(resourceServer)
		if err != nil {
			return nil, err
		}

		status := DelegationStatus{
			TenantID:       tenantID,
			ResourceServer: resource.resourceServer,
			Provider:       resource.provider,
			Scopes:         resource.scopes,
		}
		if row, ok := byResource[resource.resourceServer]; ok {
			status.Connected = !row.RevokedAt.Valid
			status.Scopes = normalizeScopeText(row.ScopeText)
			status.GrantedAt = timeFromPg(row.GrantedAt)
			status.LastRefreshedAt = timeFromPg(row.LastRefreshedAt)
			status.RevokedAt = timeFromPg(row.RevokedAt)
			if row.LastErrorCode.Valid {
				status.LastErrorCode = row.LastErrorCode.String
			}
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

func (s *DelegationService) StartConnect(ctx context.Context, user User, sessionID, resourceServer string) (string, error) {
	tenantID, err := delegationTenantID(user)
	if err != nil {
		return "", err
	}
	return s.StartConnectForTenant(ctx, user, tenantID, sessionID, resourceServer)
}

func (s *DelegationService) StartConnectForTenant(ctx context.Context, user User, tenantID int64, sessionID, resourceServer string) (string, error) {
	if err := s.requireConfigured(); err != nil {
		return "", err
	}

	resource, err := s.resource(resourceServer)
	if err != nil {
		return "", err
	}

	state, record, err := s.stateStore.Create(ctx, user.ID, tenantID, resource.resourceServer, hashSessionID(sessionID))
	if err != nil {
		return "", fmt.Errorf("create delegated auth state: %w", err)
	}

	return s.oauthClient.AuthorizeURL(state, record.CodeVerifier, resource.redirectURI, resource.scopes), nil
}

func (s *DelegationService) SaveGrantFromCallback(ctx context.Context, user User, sessionID, resourceServer, code, state string) (DelegationStatus, error) {
	if err := s.requireConfigured(); err != nil {
		return DelegationStatus{}, err
	}

	resource, err := s.resource(resourceServer)
	if err != nil {
		return DelegationStatus{}, err
	}

	record, err := s.stateStore.Consume(ctx, state)
	if err != nil {
		return DelegationStatus{}, fmt.Errorf("%w: %v", ErrDelegationInvalidState, err)
	}
	if record.UserID != user.ID || record.TenantID == 0 || record.ResourceServer != resource.resourceServer || record.SessionHash != hashSessionID(sessionID) {
		return DelegationStatus{}, ErrDelegationInvalidState
	}

	identity, err := s.queries.GetUserIdentityByUserIDProvider(ctx, db.GetUserIdentityByUserIDProviderParams{
		UserID:   user.ID,
		Provider: resource.provider,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DelegationStatus{}, ErrDelegationIdentityNotFound
	}
	if err != nil {
		return DelegationStatus{}, fmt.Errorf("load delegated provider identity: %w", err)
	}

	token, err := s.oauthClient.ExchangeCode(ctx, code, record.CodeVerifier, resource.redirectURI, resource.scopes)
	if err != nil {
		return DelegationStatus{}, err
	}
	if token.RefreshToken == "" {
		return DelegationStatus{}, ErrDelegationRefreshTokenMissing
	}

	ciphertext, keyVersion, err := s.tokenStore.Encrypt(token.RefreshToken)
	if err != nil {
		return DelegationStatus{}, fmt.Errorf("encrypt delegated refresh token: %w", err)
	}

	row, err := s.queries.UpsertOAuthUserGrant(ctx, db.UpsertOAuthUserGrantParams{
		UserID:                 user.ID,
		TenantID:               record.TenantID,
		Provider:               resource.provider,
		ResourceServer:         resource.resourceServer,
		ProviderSubject:        identity.Subject,
		RefreshTokenCiphertext: ciphertext,
		RefreshTokenKeyVersion: keyVersion,
		ScopeText:              scopeText(token.Scopes),
		GrantedBySessionID:     hashSessionID(sessionID),
	})
	if err != nil {
		return DelegationStatus{}, fmt.Errorf("save delegated grant: %w", err)
	}

	return grantStatusFromUpsertRow(row), nil
}

func (s *DelegationService) GetAccessToken(ctx context.Context, user User, resourceServer string) (DelegatedAccessToken, error) {
	tenantID, err := delegationTenantID(user)
	if err != nil {
		return DelegatedAccessToken{}, err
	}
	return s.GetAccessTokenForTenant(ctx, user, tenantID, resourceServer)
}

func (s *DelegationService) GetAccessTokenForTenant(ctx context.Context, user User, tenantID int64, resourceServer string) (DelegatedAccessToken, error) {
	if err := s.requireConfigured(); err != nil {
		return DelegatedAccessToken{}, err
	}

	resource, err := s.resource(resourceServer)
	if err != nil {
		return DelegatedAccessToken{}, err
	}

	grant, err := s.queries.GetActiveOAuthUserGrant(ctx, db.GetActiveOAuthUserGrantParams{
		UserID:         user.ID,
		TenantID:       tenantID,
		Provider:       resource.provider,
		ResourceServer: resource.resourceServer,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DelegatedAccessToken{}, ErrDelegationGrantNotFound
	}
	if err != nil {
		return DelegatedAccessToken{}, fmt.Errorf("load delegated grant: %w", err)
	}
	if s.refreshTokenExpired(grant.GrantedAt, grant.LastRefreshedAt) {
		_ = s.queries.MarkOAuthUserGrantRevoked(ctx, db.MarkOAuthUserGrantRevokedParams{
			UserID:         user.ID,
			TenantID:       tenantID,
			Provider:       resource.provider,
			ResourceServer: resource.resourceServer,
			LastErrorCode:  pgtype.Text{String: "refresh_token_expired", Valid: true},
		})
		return DelegatedAccessToken{}, ErrDelegationGrantNotFound
	}

	refreshToken, err := s.tokenStore.Decrypt(grant.RefreshTokenCiphertext, grant.RefreshTokenKeyVersion)
	if err != nil {
		return DelegatedAccessToken{}, fmt.Errorf("decrypt delegated refresh token: %w", err)
	}

	token, err := s.oauthClient.Refresh(ctx, refreshToken, resource.redirectURI, normalizeScopeText(grant.ScopeText))
	if err != nil {
		if auth.IsInvalidGrantError(err) {
			_ = s.queries.MarkOAuthUserGrantRevoked(ctx, db.MarkOAuthUserGrantRevokedParams{
				UserID:         user.ID,
				TenantID:       tenantID,
				Provider:       resource.provider,
				ResourceServer: resource.resourceServer,
				LastErrorCode:  pgtype.Text{String: "invalid_grant", Valid: true},
			})
			return DelegatedAccessToken{}, ErrDelegationGrantNotFound
		}
		return DelegatedAccessToken{}, err
	}

	nextRefreshToken := token.RefreshToken
	if nextRefreshToken == "" {
		nextRefreshToken = refreshToken
	}

	ciphertext, keyVersion, err := s.tokenStore.Encrypt(nextRefreshToken)
	if err != nil {
		return DelegatedAccessToken{}, fmt.Errorf("encrypt rotated refresh token: %w", err)
	}

	scopes := token.Scopes
	if len(scopes) == 0 {
		scopes = normalizeScopeText(grant.ScopeText)
	}

	if _, err := s.queries.UpdateOAuthUserGrantAfterRefresh(ctx, db.UpdateOAuthUserGrantAfterRefreshParams{
		UserID:                 user.ID,
		TenantID:               tenantID,
		Provider:               resource.provider,
		ResourceServer:         resource.resourceServer,
		RefreshTokenCiphertext: ciphertext,
		RefreshTokenKeyVersion: keyVersion,
		ScopeText:              scopeText(scopes),
	}); err != nil {
		return DelegatedAccessToken{}, fmt.Errorf("update delegated grant after refresh: %w", err)
	}

	return DelegatedAccessToken{
		AccessToken: token.AccessToken,
		ExpiresAt:   expiresWithSkew(token.Expiry, s.accessSkew),
		Scopes:      scopes,
	}, nil
}

func (s *DelegationService) VerifyAccessToken(ctx context.Context, user User, resourceServer string) (DelegationVerifyResult, error) {
	token, err := s.GetAccessToken(ctx, user, resourceServer)
	if err != nil {
		return DelegationVerifyResult{}, err
	}
	return delegationVerifyResult(resourceServer, token), nil
}

func (s *DelegationService) VerifyAccessTokenForTenant(ctx context.Context, user User, tenantID int64, resourceServer string) (DelegationVerifyResult, error) {
	token, err := s.GetAccessTokenForTenant(ctx, user, tenantID, resourceServer)
	if err != nil {
		return DelegationVerifyResult{}, err
	}
	return delegationVerifyResult(resourceServer, token), nil
}

func delegationVerifyResult(resourceServer string, token DelegatedAccessToken) DelegationVerifyResult {
	now := time.Now().UTC()
	return DelegationVerifyResult{
		ResourceServer:  normalizeResourceServer(resourceServer),
		Connected:       true,
		Scopes:          token.Scopes,
		AccessExpiresAt: token.ExpiresAt,
		RefreshedAt:     &now,
	}
}

func (s *DelegationService) DeleteGrant(ctx context.Context, user User, resourceServer string) error {
	tenantID, err := delegationTenantID(user)
	if err != nil {
		return err
	}
	return s.DeleteGrantForTenant(ctx, user, tenantID, resourceServer)
}

func (s *DelegationService) DeleteGrantForTenant(ctx context.Context, user User, tenantID int64, resourceServer string) error {
	if err := s.requireConfigured(); err != nil {
		return err
	}

	resource, err := s.resource(resourceServer)
	if err != nil {
		return err
	}

	grant, err := s.queries.GetActiveOAuthUserGrant(ctx, db.GetActiveOAuthUserGrantParams{
		UserID:         user.ID,
		TenantID:       tenantID,
		Provider:       resource.provider,
		ResourceServer: resource.resourceServer,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load delegated grant for revoke: %w", err)
	}
	if err == nil {
		refreshToken, err := s.tokenStore.Decrypt(grant.RefreshTokenCiphertext, grant.RefreshTokenKeyVersion)
		if err != nil {
			return fmt.Errorf("decrypt delegated refresh token for revoke: %w", err)
		}
		if err := s.oauthClient.RevokeRefreshToken(ctx, refreshToken); err != nil {
			return err
		}
	}

	if err := s.queries.DeleteOAuthUserGrant(ctx, db.DeleteOAuthUserGrantParams{
		UserID:         user.ID,
		TenantID:       tenantID,
		Provider:       resource.provider,
		ResourceServer: resource.resourceServer,
	}); err != nil {
		return fmt.Errorf("delete delegated grant: %w", err)
	}

	return nil
}

func (s *DelegationService) DeleteAllGrantsForUser(ctx context.Context, userID int64) error {
	if err := s.requireConfigured(); err != nil {
		return nil
	}

	grants, err := s.queries.ListActiveOAuthUserGrantsByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("list active delegated grants for user: %w", err)
	}
	for _, grant := range grants {
		refreshToken, err := s.tokenStore.Decrypt(grant.RefreshTokenCiphertext, grant.RefreshTokenKeyVersion)
		if err != nil {
			return fmt.Errorf("decrypt delegated refresh token for user revoke: %w", err)
		}
		if err := s.oauthClient.RevokeRefreshToken(ctx, refreshToken); err != nil {
			return err
		}
	}
	if err := s.queries.DeleteOAuthUserGrantsByUserID(ctx, userID); err != nil {
		return fmt.Errorf("delete delegated grants for user: %w", err)
	}
	return nil
}

func (s *DelegationService) requireConfigured() error {
	if s == nil || s.queries == nil || s.oauthClient == nil || s.stateStore == nil || s.tokenStore == nil || s.appBaseURL == "" {
		return ErrDelegationNotConfigured
	}
	return nil
}

func (s *DelegationService) resource(resourceServer string) (delegationResource, error) {
	normalized := normalizeResourceServer(resourceServer)
	if normalized != "zitadel" {
		return delegationResource{}, ErrDelegationUnsupportedResource
	}

	scopes := s.defaultScopes
	if len(scopes) == 0 {
		scopes = []string{"offline_access"}
	}

	return delegationResource{
		resourceServer: "zitadel",
		provider:       "zitadel",
		redirectURI:    s.appBaseURL + "/api/v1/integrations/zitadel/callback",
		scopes:         scopes,
	}, nil
}

func normalizeResourceServer(resourceServer string) string {
	return strings.ToLower(strings.TrimSpace(resourceServer))
}

func hashSessionID(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return hex.EncodeToString(sum[:])
}

func scopeText(scopes []string) string {
	return strings.Join(normalizeScopeList(scopes), " ")
}

func normalizeScopeText(value string) []string {
	return normalizeScopeList(strings.Fields(value))
}

func normalizeScopeList(scopes []string) []string {
	set := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		trimmed := strings.TrimSpace(scope)
		if trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}

	normalized := make([]string, 0, len(set))
	for scope := range set {
		normalized = append(normalized, scope)
	}
	sort.Strings(normalized)
	return normalized
}

func timeFromPg(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

func expiresWithSkew(expiry time.Time, skew time.Duration) *time.Time {
	if expiry.IsZero() {
		return nil
	}
	expiresAt := expiry.Add(-skew).UTC()
	return &expiresAt
}

func (s *DelegationService) refreshTokenExpired(grantedAt, lastRefreshedAt pgtype.Timestamptz) bool {
	if s.refreshTTL <= 0 || !grantedAt.Valid {
		return false
	}

	base := grantedAt.Time
	if lastRefreshedAt.Valid {
		base = lastRefreshedAt.Time
	}

	return time.Now().After(base.Add(s.refreshTTL))
}

func grantStatusFromUpsertRow(row db.UpsertOAuthUserGrantRow) DelegationStatus {
	return DelegationStatus{
		TenantID:        row.TenantID,
		ResourceServer:  row.ResourceServer,
		Provider:        row.Provider,
		Connected:       !row.RevokedAt.Valid,
		Scopes:          normalizeScopeText(row.ScopeText),
		GrantedAt:       timeFromPg(row.GrantedAt),
		LastRefreshedAt: timeFromPg(row.LastRefreshedAt),
		RevokedAt:       timeFromPg(row.RevokedAt),
		LastErrorCode: func() string {
			if row.LastErrorCode.Valid {
				return row.LastErrorCode.String
			}
			return ""
		}(),
	}
}

func delegationTenantID(user User) (int64, error) {
	if user.DefaultTenantID == nil || *user.DefaultTenantID == 0 {
		return 0, ErrDelegationGrantNotFound
	}
	return *user.DefaultTenantID, nil
}
```

#### `backend/internal/service/identity_service.go`

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidExternalIdentity = errors.New("invalid external identity")

type ExternalIdentity struct {
	Provider      string
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
}

type IdentityService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewIdentityService(pool *pgxpool.Pool, queries *db.Queries) *IdentityService {
	return &IdentityService{
		pool:    pool,
		queries: queries,
	}
}

func (s *IdentityService) ResolveOrCreateUser(ctx context.Context, identity ExternalIdentity) (User, error) {
	normalized, err := normalizeExternalIdentity(identity)
	if err != nil {
		return User{}, err
	}

	existing, err := s.queries.GetUserByProviderSubject(ctx, db.GetUserByProviderSubjectParams{
		Provider: normalized.Provider,
		Subject:  normalized.Subject,
	})
	if err == nil {
		if existing.DeactivatedAt.Valid {
			return User{}, ErrUnauthorized
		}
		_ = s.queries.UpdateUserIdentityProfile(ctx, db.UpdateUserIdentityProfileParams{
			Provider:      normalized.Provider,
			Subject:       normalized.Subject,
			Email:         normalized.Email,
			EmailVerified: normalized.EmailVerified,
		})

		return s.syncUserProfile(ctx, s.queries, dbUser(existing.ID, existing.PublicID.String(), existing.Email, existing.DisplayName, existing.DeactivatedAt, existing.DefaultTenantID), normalized)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return User{}, fmt.Errorf("lookup identity by provider subject: %w", err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, fmt.Errorf("begin identity transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	user, err := s.resolveUserForIdentity(ctx, qtx, normalized)
	if err != nil {
		return User{}, err
	}

	if err := qtx.CreateUserIdentity(ctx, db.CreateUserIdentityParams{
		UserID:        user.ID,
		Provider:      normalized.Provider,
		Subject:       normalized.Subject,
		Email:         normalized.Email,
		EmailVerified: normalized.EmailVerified,
	}); err != nil {
		return User{}, fmt.Errorf("create user identity: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit identity transaction: %w", err)
	}

	return user, nil
}

func (s *IdentityService) resolveUserForIdentity(ctx context.Context, queries *db.Queries, identity ExternalIdentity) (User, error) {
	if identity.EmailVerified {
		existing, err := queries.GetUserByEmail(ctx, identity.Email)
		if err == nil {
			if existing.DeactivatedAt.Valid {
				return User{}, ErrUnauthorized
			}
			return s.syncUserProfile(ctx, queries, dbUser(existing.ID, existing.PublicID.String(), existing.Email, existing.DisplayName, existing.DeactivatedAt, existing.DefaultTenantID), identity)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return User{}, fmt.Errorf("lookup user by email: %w", err)
		}
	}

	created, err := queries.CreateOIDCUser(ctx, db.CreateOIDCUserParams{
		Email:       identity.Email,
		DisplayName: identity.DisplayName,
	})
	if err != nil {
		return User{}, fmt.Errorf("create oidc user: %w", err)
	}

	return dbUser(created.ID, created.PublicID.String(), created.Email, created.DisplayName, created.DeactivatedAt, created.DefaultTenantID), nil
}

func (s *IdentityService) syncUserProfile(ctx context.Context, queries *db.Queries, user User, identity ExternalIdentity) (User, error) {
	nextEmail := user.Email
	if identity.EmailVerified && identity.Email != "" {
		nextEmail = identity.Email
	}

	nextDisplayName := user.DisplayName
	if identity.DisplayName != "" {
		nextDisplayName = identity.DisplayName
	}

	if nextEmail == user.Email && nextDisplayName == user.DisplayName {
		return user, nil
	}

	updated, err := queries.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		ID:          user.ID,
		Email:       nextEmail,
		DisplayName: nextDisplayName,
	})
	if err != nil {
		return User{}, fmt.Errorf("update user profile: %w", err)
	}

	return dbUser(updated.ID, updated.PublicID.String(), updated.Email, updated.DisplayName, updated.DeactivatedAt, updated.DefaultTenantID), nil
}

func normalizeExternalIdentity(identity ExternalIdentity) (ExternalIdentity, error) {
	provider := strings.ToLower(strings.TrimSpace(identity.Provider))
	subject := strings.TrimSpace(identity.Subject)
	email := strings.ToLower(strings.TrimSpace(identity.Email))
	displayName := strings.TrimSpace(identity.DisplayName)

	if provider == "" || subject == "" || email == "" {
		return ExternalIdentity{}, ErrInvalidExternalIdentity
	}
	if displayName == "" {
		displayName = fallbackDisplayName(email, subject)
	}

	return ExternalIdentity{
		Provider:      provider,
		Subject:       subject,
		Email:         email,
		EmailVerified: identity.EmailVerified,
		DisplayName:   displayName,
	}, nil
}

func fallbackDisplayName(email, subject string) string {
	if email != "" {
		if head, _, ok := strings.Cut(email, "@"); ok && head != "" {
			return head
		}
		return email
	}
	return subject
}

func dbUser(id int64, publicID, email, displayName string, deactivatedAt pgtype.Timestamptz, defaultTenantID pgtype.Int8) User {
	var deactivated *time.Time
	if deactivatedAt.Valid {
		value := deactivatedAt.Time
		deactivated = &value
	}

	return User{
		ID:              id,
		PublicID:        publicID,
		Email:           email,
		DisplayName:     displayName,
		DeactivatedAt:   deactivated,
		DefaultTenantID: optionalPgInt8(defaultTenantID),
	}
}
```

#### `backend/internal/service/oidc_login_service.go`

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"example.com/haohao/backend/internal/auth"
)

type OIDCLoginResult struct {
	SessionID string
	CSRFToken string
	ReturnTo  string
}

type OIDCLoginService struct {
	providerName   string
	oidcClient     *auth.OIDCClient
	loginState     *auth.LoginStateStore
	identity       *IdentityService
	authzService   *AuthzService
	sessionService *SessionService
}

func NewOIDCLoginService(providerName string, oidcClient *auth.OIDCClient, loginState *auth.LoginStateStore, identity *IdentityService, authzService *AuthzService, sessionService *SessionService) *OIDCLoginService {
	return &OIDCLoginService{
		providerName:   providerName,
		oidcClient:     oidcClient,
		loginState:     loginState,
		identity:       identity,
		authzService:   authzService,
		sessionService: sessionService,
	}
}

func (s *OIDCLoginService) StartLogin(ctx context.Context, returnTo string) (string, error) {
	if s == nil || s.oidcClient == nil || s.loginState == nil {
		return "", ErrAuthModeUnsupported
	}

	state, record, err := s.loginState.Create(ctx, sanitizeReturnTo(returnTo))
	if err != nil {
		return "", fmt.Errorf("create oidc login state: %w", err)
	}

	return s.oidcClient.AuthorizeURL(state, record.Nonce, record.CodeVerifier), nil
}

func (s *OIDCLoginService) FinishLogin(ctx context.Context, code, state string) (OIDCLoginResult, error) {
	if s == nil || s.oidcClient == nil || s.loginState == nil || s.identity == nil || s.sessionService == nil {
		return OIDCLoginResult{}, ErrAuthModeUnsupported
	}

	loginState, err := s.loginState.Consume(ctx, state)
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("consume oidc login state: %w", err)
	}

	identity, err := s.oidcClient.ExchangeCode(ctx, code, loginState.CodeVerifier, loginState.Nonce)
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("finish oidc code exchange: %w", err)
	}

	user, err := s.identity.ResolveOrCreateUser(ctx, ExternalIdentity{
		Provider:      s.providerName,
		Subject:       identity.Claims.Subject,
		Email:         identity.Claims.Email,
		EmailVerified: identity.Claims.EmailVerified,
		DisplayName:   identity.Claims.Name,
	})
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("resolve local user for oidc identity: %w", err)
	}

	if s.authzService != nil {
		if _, err := s.authzService.SyncGlobalRoles(ctx, user.ID, identity.Claims.Groups); err != nil {
			return OIDCLoginResult{}, fmt.Errorf("sync local roles for oidc login: %w", err)
		}
		if _, err := s.authzService.SyncTenantMemberships(ctx, user.ID, "provider_claim", identity.Claims.Groups); err != nil {
			return OIDCLoginResult{}, fmt.Errorf("sync tenant memberships for oidc login: %w", err)
		}
	}

	sessionID, csrfToken, err := s.sessionService.IssueSessionWithProviderHint(ctx, user.ID, identity.RawIDToken)
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("issue local session for oidc login: %w", err)
	}

	return OIDCLoginResult{
		SessionID: sessionID,
		CSRFToken: csrfToken,
		ReturnTo:  sanitizeReturnTo(loginState.ReturnTo),
	}, nil
}

func sanitizeReturnTo(returnTo string) string {
	trimmed := strings.TrimSpace(returnTo)
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return "/"
	}
	return trimmed
}

func IsOIDCLoginFailure(err error) bool {
	return err != nil && (errors.Is(err, auth.ErrLoginStateNotFound) || errors.Is(err, ErrInvalidExternalIdentity))
}
```

#### `backend/internal/service/provisioning_service.go`

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const scimIdentityProvider = "scim"

var ErrInvalidSCIMUser = errors.New("invalid scim user")

type ProvisionedUserInput struct {
	ExternalID  string
	UserName    string
	DisplayName string
	Active      bool
	Groups      []string
}

type ProvisionedUser struct {
	ID          int64
	PublicID    string
	ExternalID  string
	UserName    string
	DisplayName string
	Active      bool
}

type ProvisioningService struct {
	pool              *pgxpool.Pool
	queries           *db.Queries
	sessionService    *SessionService
	delegationService *DelegationService
	authzService      *AuthzService
}

func NewProvisioningService(pool *pgxpool.Pool, queries *db.Queries, sessionService *SessionService, delegationService *DelegationService, authzService *AuthzService) *ProvisioningService {
	return &ProvisioningService{
		pool:              pool,
		queries:           queries,
		sessionService:    sessionService,
		delegationService: delegationService,
		authzService:      authzService,
	}
}

func (s *ProvisioningService) UpsertUser(ctx context.Context, input ProvisionedUserInput) (ProvisionedUser, error) {
	normalized, err := normalizeProvisionedUser(input)
	if err != nil {
		return ProvisionedUser{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ProvisionedUser{}, fmt.Errorf("begin provisioning transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	user, err := qtx.GetProvisionedUserByExternalID(ctx, db.GetProvisionedUserByExternalIDParams{
		Provider:   scimIdentityProvider,
		ExternalID: pgtype.Text{String: normalized.ExternalID, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		created, err := s.createProvisionedUser(ctx, qtx, normalized)
		if err != nil {
			return ProvisionedUser{}, err
		}
		user = created
	} else if err != nil {
		return ProvisionedUser{}, fmt.Errorf("lookup provisioned user by external id: %w", err)
	} else {
		if err := s.updateProvisionedUser(ctx, qtx, user.ID, normalized); err != nil {
			return ProvisionedUser{}, err
		}
		user.Email = normalized.UserName
		user.DisplayName = normalized.DisplayName
		user.DeactivatedAt = deactivatedAtForActive(normalized.Active)
	}

	if err := tx.Commit(ctx); err != nil {
		return ProvisionedUser{}, fmt.Errorf("commit provisioning transaction: %w", err)
	}

	if s.authzService != nil && normalized.Groups != nil {
		if _, err := s.authzService.SyncTenantMemberships(ctx, user.ID, "scim", normalized.Groups); err != nil {
			return ProvisionedUser{}, err
		}
	}
	if !normalized.Active {
		if err := s.deactivateSideEffects(ctx, user.ID); err != nil {
			return ProvisionedUser{}, err
		}
	}

	return provisionedUserFromRow(user), nil
}

func (s *ProvisioningService) GetUser(ctx context.Context, publicID string) (ProvisionedUser, error) {
	id, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return ProvisionedUser{}, ErrInvalidSCIMUser
	}

	user, err := s.queries.GetProvisionedUserByPublicID(ctx, db.GetProvisionedUserByPublicIDParams{
		Provider: scimIdentityProvider,
		PublicID: id,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ProvisionedUser{}, ErrUnauthorized
	}
	if err != nil {
		return ProvisionedUser{}, fmt.Errorf("load provisioned user: %w", err)
	}
	return provisionedUserFromPublicIDRow(user), nil
}

func (s *ProvisioningService) GetUserByExternalID(ctx context.Context, externalID string) (ProvisionedUser, error) {
	user, err := s.queries.GetProvisionedUserByExternalID(ctx, db.GetProvisionedUserByExternalIDParams{
		Provider:   scimIdentityProvider,
		ExternalID: pgtype.Text{String: strings.TrimSpace(externalID), Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ProvisionedUser{}, ErrUnauthorized
	}
	if err != nil {
		return ProvisionedUser{}, fmt.Errorf("load provisioned user by external id: %w", err)
	}
	return provisionedUserFromRow(user), nil
}

func (s *ProvisioningService) ListUsers(ctx context.Context, startIndex, count int32) ([]ProvisionedUser, error) {
	if startIndex < 1 {
		startIndex = 1
	}
	if count <= 0 || count > 100 {
		count = 100
	}

	rows, err := s.queries.ListProvisionedUsers(ctx, db.ListProvisionedUsersParams{
		Provider: scimIdentityProvider,
		Limit:    count,
		Offset:   startIndex - 1,
	})
	if err != nil {
		return nil, fmt.Errorf("list provisioned users: %w", err)
	}

	users := make([]ProvisionedUser, 0, len(rows))
	for _, row := range rows {
		users = append(users, provisionedUserFromListRow(row))
	}
	return users, nil
}

func (s *ProvisioningService) DeactivateUser(ctx context.Context, publicID string) (ProvisionedUser, error) {
	user, err := s.GetUser(ctx, publicID)
	if err != nil {
		return ProvisionedUser{}, err
	}

	updated, err := s.queries.SetUserDeactivatedAt(ctx, db.SetUserDeactivatedAtParams{
		ID:            user.ID,
		DeactivatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		return ProvisionedUser{}, fmt.Errorf("deactivate provisioned user: %w", err)
	}
	if err := s.deactivateSideEffects(ctx, user.ID); err != nil {
		return ProvisionedUser{}, err
	}

	return ProvisionedUser{
		ID:          updated.ID,
		PublicID:    updated.PublicID.String(),
		ExternalID:  user.ExternalID,
		UserName:    updated.Email,
		DisplayName: updated.DisplayName,
		Active:      false,
	}, nil
}

func (s *ProvisioningService) createProvisionedUser(ctx context.Context, qtx *db.Queries, input ProvisionedUserInput) (db.GetProvisionedUserByExternalIDRow, error) {
	existing, err := qtx.GetUserByEmail(ctx, input.UserName)
	if err == nil {
		if err := qtx.CreateProvisionedUserIdentity(ctx, db.CreateProvisionedUserIdentityParams{
			UserID:             existing.ID,
			Provider:           scimIdentityProvider,
			Subject:            input.ExternalID,
			Email:              input.UserName,
			EmailVerified:      true,
			ExternalID:         pgtype.Text{String: input.ExternalID, Valid: true},
			ProvisioningSource: pgtype.Text{String: "scim", Valid: true},
		}); err != nil {
			return db.GetProvisionedUserByExternalIDRow{}, fmt.Errorf("create provisioned identity: %w", err)
		}
		if err := s.updateProvisionedUser(ctx, qtx, existing.ID, input); err != nil {
			return db.GetProvisionedUserByExternalIDRow{}, err
		}
		return rowFromUser(existing.ID, existing.PublicID.String(), input.UserName, input.DisplayName, deactivatedAtForActive(input.Active), existing.DefaultTenantID, input.ExternalID), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.GetProvisionedUserByExternalIDRow{}, fmt.Errorf("lookup user by email: %w", err)
	}

	created, err := qtx.CreateOIDCUser(ctx, db.CreateOIDCUserParams{
		Email:       input.UserName,
		DisplayName: input.DisplayName,
	})
	if err != nil {
		return db.GetProvisionedUserByExternalIDRow{}, fmt.Errorf("create provisioned user: %w", err)
	}
	if err := qtx.CreateProvisionedUserIdentity(ctx, db.CreateProvisionedUserIdentityParams{
		UserID:             created.ID,
		Provider:           scimIdentityProvider,
		Subject:            input.ExternalID,
		Email:              input.UserName,
		EmailVerified:      true,
		ExternalID:         pgtype.Text{String: input.ExternalID, Valid: true},
		ProvisioningSource: pgtype.Text{String: "scim", Valid: true},
	}); err != nil {
		return db.GetProvisionedUserByExternalIDRow{}, fmt.Errorf("create provisioned identity: %w", err)
	}
	if !input.Active {
		if _, err := qtx.SetUserDeactivatedAt(ctx, db.SetUserDeactivatedAtParams{
			ID:            created.ID,
			DeactivatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		}); err != nil {
			return db.GetProvisionedUserByExternalIDRow{}, fmt.Errorf("deactivate created provisioned user: %w", err)
		}
	}

	return rowFromUser(created.ID, created.PublicID.String(), input.UserName, input.DisplayName, deactivatedAtForActive(input.Active), created.DefaultTenantID, input.ExternalID), nil
}

func (s *ProvisioningService) updateProvisionedUser(ctx context.Context, qtx *db.Queries, userID int64, input ProvisionedUserInput) error {
	if _, err := qtx.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		ID:          userID,
		Email:       input.UserName,
		DisplayName: input.DisplayName,
	}); err != nil {
		return fmt.Errorf("update provisioned user profile: %w", err)
	}
	if _, err := qtx.SetUserDeactivatedAt(ctx, db.SetUserDeactivatedAtParams{
		ID:            userID,
		DeactivatedAt: deactivatedAtForActive(input.Active),
	}); err != nil {
		return fmt.Errorf("update provisioned user active state: %w", err)
	}
	if err := qtx.UpdateUserIdentityProvisioningProfile(ctx, db.UpdateUserIdentityProvisioningProfileParams{
		Provider:           scimIdentityProvider,
		Subject:            input.ExternalID,
		Email:              input.UserName,
		EmailVerified:      true,
		ExternalID:         pgtype.Text{String: input.ExternalID, Valid: true},
		ProvisioningSource: pgtype.Text{String: "scim", Valid: true},
	}); err != nil {
		return fmt.Errorf("update provisioned identity profile: %w", err)
	}
	return nil
}

func (s *ProvisioningService) deactivateSideEffects(ctx context.Context, userID int64) error {
	if s.sessionService != nil {
		if err := s.sessionService.DeleteUserSessions(ctx, userID); err != nil {
			return err
		}
	}
	if s.delegationService != nil {
		if err := s.delegationService.DeleteAllGrantsForUser(ctx, userID); err != nil {
			return err
		}
	}
	if err := s.queries.DeleteOAuthUserGrantsByUserID(ctx, userID); err != nil {
		return fmt.Errorf("delete deactivated user grants: %w", err)
	}
	return nil
}

func normalizeProvisionedUser(input ProvisionedUserInput) (ProvisionedUserInput, error) {
	externalID := strings.TrimSpace(input.ExternalID)
	userName := strings.ToLower(strings.TrimSpace(input.UserName))
	displayName := strings.TrimSpace(input.DisplayName)
	if externalID == "" || userName == "" {
		return ProvisionedUserInput{}, ErrInvalidSCIMUser
	}
	if displayName == "" {
		displayName = fallbackDisplayName(userName, externalID)
	}
	return ProvisionedUserInput{
		ExternalID:  externalID,
		UserName:    userName,
		DisplayName: displayName,
		Active:      input.Active,
		Groups:      input.Groups,
	}, nil
}

func deactivatedAtForActive(active bool) pgtype.Timestamptz {
	if active {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
}

func rowFromUser(id int64, publicID, email, displayName string, deactivatedAt pgtype.Timestamptz, defaultTenantID pgtype.Int8, externalID string) db.GetProvisionedUserByExternalIDRow {
	parsed, _ := uuid.Parse(publicID)
	return db.GetProvisionedUserByExternalIDRow{
		ID:                 id,
		PublicID:           parsed,
		Email:              email,
		DisplayName:        displayName,
		DeactivatedAt:      deactivatedAt,
		DefaultTenantID:    defaultTenantID,
		Provider:           scimIdentityProvider,
		Subject:            externalID,
		ExternalID:         pgtype.Text{String: externalID, Valid: true},
		ProvisioningSource: pgtype.Text{String: "scim", Valid: true},
	}
}

func provisionedUserFromRow(row db.GetProvisionedUserByExternalIDRow) ProvisionedUser {
	externalID := ""
	if row.ExternalID.Valid {
		externalID = row.ExternalID.String
	}
	return ProvisionedUser{
		ID:          row.ID,
		PublicID:    row.PublicID.String(),
		ExternalID:  externalID,
		UserName:    row.Email,
		DisplayName: row.DisplayName,
		Active:      !row.DeactivatedAt.Valid,
	}
}

func provisionedUserFromPublicIDRow(row db.GetProvisionedUserByPublicIDRow) ProvisionedUser {
	return provisionedUserFromRow(db.GetProvisionedUserByExternalIDRow(row))
}

func provisionedUserFromListRow(row db.ListProvisionedUsersRow) ProvisionedUser {
	return provisionedUserFromRow(db.GetProvisionedUserByExternalIDRow(row))
}
```

#### `backend/internal/service/session_service.go`

```go
package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"example.com/haohao/backend/internal/auth"
	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInvalidCSRFToken    = errors.New("invalid csrf token")
	ErrAuthModeUnsupported = errors.New("auth mode unsupported")
)

type User struct {
	ID              int64
	PublicID        string
	Email           string
	DisplayName     string
	DeactivatedAt   *time.Time
	DefaultTenantID *int64
}

type CurrentSession struct {
	User           User
	ActiveTenantID *int64
}

type SessionService struct {
	queries  *db.Queries
	store    *auth.SessionStore
	authMode string
}

func NewSessionService(queries *db.Queries, store *auth.SessionStore, authMode string) *SessionService {
	return &SessionService{
		queries:  queries,
		store:    store,
		authMode: strings.ToLower(strings.TrimSpace(authMode)),
	}
}

func (s *SessionService) Login(ctx context.Context, email, password string) (User, string, string, error) {
	if s.authMode == "zitadel" {
		return User{}, "", "", ErrAuthModeUnsupported
	}

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

	sessionID, csrfToken, err := s.IssueSession(ctx, userID)
	if err != nil {
		return User{}, "", "", err
	}

	return user, sessionID, csrfToken, nil
}

func (s *SessionService) CurrentUser(ctx context.Context, sessionID string) (User, error) {
	current, err := s.CurrentSession(ctx, sessionID)
	if err != nil {
		return User{}, err
	}
	return current.User, nil
}

func (s *SessionService) CurrentSession(ctx context.Context, sessionID string) (CurrentSession, error) {
	session, err := s.store.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return CurrentSession{}, ErrUnauthorized
	}
	if err != nil {
		return CurrentSession{}, err
	}

	user, err := s.loadUserByID(ctx, session.UserID)
	if err != nil {
		return CurrentSession{}, err
	}

	return CurrentSession{
		User:           user,
		ActiveTenantID: optionalInt64(session.ActiveTenantID),
	}, nil
}

func (s *SessionService) CurrentUserWithCSRF(ctx context.Context, sessionID, csrfHeader string) (User, error) {
	current, err := s.CurrentSessionWithCSRF(ctx, sessionID, csrfHeader)
	if err != nil {
		return User{}, err
	}
	return current.User, nil
}

func (s *SessionService) CurrentSessionWithCSRF(ctx context.Context, sessionID, csrfHeader string) (CurrentSession, error) {
	session, err := s.store.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return CurrentSession{}, ErrUnauthorized
	}
	if err != nil {
		return CurrentSession{}, err
	}

	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(csrfHeader)) != 1 {
		return CurrentSession{}, ErrInvalidCSRFToken
	}

	user, err := s.loadUserByID(ctx, session.UserID)
	if err != nil {
		return CurrentSession{}, err
	}

	return CurrentSession{
		User:           user,
		ActiveTenantID: optionalInt64(session.ActiveTenantID),
	}, nil
}

func (s *SessionService) IssueSession(ctx context.Context, userID int64) (string, string, error) {
	return s.IssueSessionWithProviderHint(ctx, userID, "")
}

func (s *SessionService) IssueSessionWithProviderHint(ctx context.Context, userID int64, providerIDTokenHint string) (string, string, error) {
	sessionID, csrfToken, err := s.store.CreateWithProviderHint(ctx, userID, providerIDTokenHint)
	if err != nil {
		return "", "", fmt.Errorf("create session: %w", err)
	}
	return sessionID, csrfToken, nil
}

func (s *SessionService) Logout(ctx context.Context, sessionID, csrfHeader string) (string, error) {
	session, err := s.store.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", ErrUnauthorized
	}
	if err != nil {
		return "", err
	}

	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(csrfHeader)) != 1 {
		return "", ErrInvalidCSRFToken
	}

	if err := s.store.Delete(ctx, sessionID); err != nil {
		return "", err
	}

	return session.ProviderIDTokenHint, nil
}

func (s *SessionService) ReissueCSRF(ctx context.Context, sessionID string) (string, error) {
	if _, err := s.CurrentUser(ctx, sessionID); err != nil {
		return "", err
	}

	csrfToken, err := s.store.ReissueCSRF(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", ErrUnauthorized
	}
	if err != nil {
		return "", err
	}

	return csrfToken, nil
}

func (s *SessionService) SetActiveTenant(ctx context.Context, sessionID, csrfHeader string, tenantID int64) error {
	if _, err := s.CurrentSessionWithCSRF(ctx, sessionID, csrfHeader); err != nil {
		return err
	}
	return s.store.SetActiveTenant(ctx, sessionID, tenantID)
}

func (s *SessionService) DeleteUserSessions(ctx context.Context, userID int64) error {
	return s.store.DeleteUserSessions(ctx, userID)
}

func (s *SessionService) RefreshSession(ctx context.Context, sessionID, csrfHeader string) (string, string, error) {
	session, err := s.store.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", "", ErrUnauthorized
	}
	if err != nil {
		return "", "", err
	}

	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(csrfHeader)) != 1 {
		return "", "", ErrInvalidCSRFToken
	}

	newSessionID, newCSRFToken, err := s.store.Rotate(ctx, sessionID)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return "", "", ErrUnauthorized
	}
	if err != nil {
		return "", "", err
	}

	return newSessionID, newCSRFToken, nil
}

func (s *SessionService) loadUserByID(ctx context.Context, userID int64) (User, error) {
	record, err := s.queries.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, fmt.Errorf("load user by session: %w", err)
	}
	if record.DeactivatedAt.Valid {
		return User{}, ErrUnauthorized
	}

	return User{
		ID:              record.ID,
		PublicID:        record.PublicID.String(),
		Email:           record.Email,
		DisplayName:     record.DisplayName,
		DefaultTenantID: optionalPgInt8(record.DefaultTenantID),
	}, nil
}

func optionalInt64(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}

func optionalPgInt8(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}
```

#### `db/migrations/0005_provisioning.up.sql`

```sql
ALTER TABLE users
    ADD COLUMN deactivated_at TIMESTAMPTZ;

ALTER TABLE user_identities
    ADD COLUMN external_id TEXT,
    ADD COLUMN provisioning_source TEXT;

CREATE UNIQUE INDEX user_identities_provider_external_id_key
    ON user_identities(provider, external_id)
    WHERE external_id IS NOT NULL;

CREATE TABLE provisioning_sync_state (
    source TEXT PRIMARY KEY,
    cursor_text TEXT,
    last_synced_at TIMESTAMPTZ,
    last_error_code TEXT,
    last_error_message TEXT,
    failed_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

#### `db/migrations/0005_provisioning.down.sql`

```sql
DROP TABLE IF EXISTS provisioning_sync_state;

DROP INDEX IF EXISTS user_identities_provider_external_id_key;

ALTER TABLE user_identities
    DROP COLUMN IF EXISTS provisioning_source,
    DROP COLUMN IF EXISTS external_id;

ALTER TABLE users
    DROP COLUMN IF EXISTS deactivated_at;
```

#### `db/migrations/0006_org_tenants.up.sql`

```sql
CREATE TABLE tenants (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE users
    ADD COLUMN default_tenant_id BIGINT REFERENCES tenants(id) ON DELETE SET NULL;

CREATE TABLE tenant_memberships (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    source TEXT NOT NULL CHECK (source IN ('provider_claim', 'scim', 'local_override')),
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, tenant_id, role_id, source)
);

CREATE INDEX tenant_memberships_tenant_id_idx
    ON tenant_memberships(tenant_id);

CREATE INDEX tenant_memberships_role_id_idx
    ON tenant_memberships(role_id);

CREATE TABLE tenant_role_overrides (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    effect TEXT NOT NULL CHECK (effect IN ('allow', 'deny')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, tenant_id, role_id, effect)
);

CREATE INDEX tenant_role_overrides_tenant_id_idx
    ON tenant_role_overrides(tenant_id);

ALTER TABLE oauth_user_grants
    ADD COLUMN tenant_id BIGINT REFERENCES tenants(id) ON DELETE CASCADE;

UPDATE oauth_user_grants g
SET tenant_id = u.default_tenant_id
FROM users u
WHERE g.user_id = u.id
  AND u.default_tenant_id IS NOT NULL;

DELETE FROM oauth_user_grants
WHERE tenant_id IS NULL;

ALTER TABLE oauth_user_grants
    ALTER COLUMN tenant_id SET NOT NULL,
    DROP CONSTRAINT oauth_user_grants_user_id_provider_resource_server_key,
    ADD CONSTRAINT oauth_user_grants_user_id_provider_resource_server_tenant_id_key
        UNIQUE (user_id, provider, resource_server, tenant_id);

CREATE INDEX oauth_user_grants_tenant_id_idx
    ON oauth_user_grants(tenant_id);
```

#### `db/migrations/0006_org_tenants.down.sql`

```sql
DROP INDEX IF EXISTS oauth_user_grants_tenant_id_idx;

ALTER TABLE oauth_user_grants
    DROP CONSTRAINT IF EXISTS oauth_user_grants_user_id_provider_resource_server_tenant_id_key,
    DROP COLUMN IF EXISTS tenant_id,
    ADD CONSTRAINT oauth_user_grants_user_id_provider_resource_server_key
        UNIQUE (user_id, provider, resource_server);

DROP TABLE IF EXISTS tenant_role_overrides;
DROP TABLE IF EXISTS tenant_memberships;

ALTER TABLE users
    DROP COLUMN IF EXISTS default_tenant_id;

DROP TABLE IF EXISTS tenants;
```

#### `db/queries/downstream_grants.sql`

```sql
-- name: UpsertOAuthUserGrant :one
INSERT INTO oauth_user_grants (
    user_id,
    tenant_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9
)
ON CONFLICT (user_id, provider, resource_server, tenant_id) DO UPDATE
SET provider_subject = EXCLUDED.provider_subject,
    refresh_token_ciphertext = EXCLUDED.refresh_token_ciphertext,
    refresh_token_key_version = EXCLUDED.refresh_token_key_version,
    scope_text = EXCLUDED.scope_text,
    granted_by_session_id = EXCLUDED.granted_by_session_id,
    granted_at = now(),
    last_refreshed_at = NULL,
    revoked_at = NULL,
    last_error_code = NULL,
    updated_at = now()
RETURNING
    id,
    user_id,
    tenant_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at;

-- name: GetOAuthUserGrant :one
SELECT
    id,
    user_id,
    tenant_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at
FROM oauth_user_grants
WHERE user_id = $1
  AND tenant_id = $2
  AND provider = $3
  AND resource_server = $4
LIMIT 1;

-- name: GetActiveOAuthUserGrant :one
SELECT
    id,
    user_id,
    tenant_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at
FROM oauth_user_grants
WHERE user_id = $1
  AND tenant_id = $2
  AND provider = $3
  AND resource_server = $4
  AND revoked_at IS NULL
LIMIT 1;

-- name: ListOAuthUserGrantsByUserID :many
SELECT
    id,
    user_id,
    tenant_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at
FROM oauth_user_grants
WHERE user_id = $1
  AND tenant_id = $2
ORDER BY resource_server, provider;

-- name: UpdateOAuthUserGrantAfterRefresh :one
UPDATE oauth_user_grants
SET refresh_token_ciphertext = $5,
    refresh_token_key_version = $6,
    scope_text = $7,
    last_refreshed_at = now(),
    last_error_code = NULL,
    updated_at = now()
WHERE user_id = $1
  AND tenant_id = $2
  AND provider = $3
  AND resource_server = $4
  AND revoked_at IS NULL
RETURNING
    id,
    user_id,
    tenant_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at;

-- name: MarkOAuthUserGrantRevoked :exec
UPDATE oauth_user_grants
SET revoked_at = now(),
    last_error_code = $5,
    updated_at = now()
WHERE user_id = $1
  AND tenant_id = $2
  AND provider = $3
  AND resource_server = $4;

-- name: DeleteOAuthUserGrant :exec
DELETE FROM oauth_user_grants
WHERE user_id = $1
  AND tenant_id = $2
  AND provider = $3
  AND resource_server = $4;

-- name: ListActiveOAuthUserGrantsByUserID :many
SELECT
    id,
    user_id,
    tenant_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at
FROM oauth_user_grants
WHERE user_id = $1
  AND revoked_at IS NULL
ORDER BY tenant_id, resource_server, provider;

-- name: DeleteOAuthUserGrantsByUserID :exec
DELETE FROM oauth_user_grants
WHERE user_id = $1;
```

#### `db/queries/identities.sql`

```sql
-- name: GetUserByProviderSubject :one
SELECT
    u.id,
    u.public_id,
    u.email,
    u.display_name,
    u.deactivated_at,
    u.default_tenant_id
FROM user_identities ui
JOIN users u ON u.id = ui.user_id
WHERE ui.provider = $1
  AND ui.subject = $2
LIMIT 1;

-- name: GetUserIdentityByUserIDProvider :one
SELECT
    id,
    user_id,
    provider,
    subject,
    email,
    email_verified,
    external_id,
    provisioning_source,
    created_at,
    updated_at
FROM user_identities
WHERE user_id = $1
  AND provider = $2
LIMIT 1;

-- name: CreateUserIdentity :exec
INSERT INTO user_identities (
    user_id,
    provider,
    subject,
    email,
    email_verified
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
);

-- name: CreateProvisionedUserIdentity :exec
INSERT INTO user_identities (
    user_id,
    provider,
    subject,
    email,
    email_verified,
    external_id,
    provisioning_source
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
);

-- name: UpdateUserIdentityProfile :exec
UPDATE user_identities
SET email = $3,
    email_verified = $4,
    updated_at = now()
WHERE provider = $1
  AND subject = $2;

-- name: UpdateUserIdentityProvisioningProfile :exec
UPDATE user_identities
SET email = $3,
    email_verified = $4,
    external_id = $5,
    provisioning_source = $6,
    updated_at = now()
WHERE provider = $1
  AND subject = $2;

-- name: GetProvisionedUserByExternalID :one
SELECT
    u.id,
    u.public_id,
    u.email,
    u.display_name,
    u.deactivated_at,
    u.default_tenant_id,
    ui.id AS identity_id,
    ui.provider,
    ui.subject,
    ui.external_id,
    ui.provisioning_source
FROM user_identities ui
JOIN users u ON u.id = ui.user_id
WHERE ui.provider = $1
  AND ui.external_id = $2
LIMIT 1;

-- name: GetProvisionedUserByPublicID :one
SELECT
    u.id,
    u.public_id,
    u.email,
    u.display_name,
    u.deactivated_at,
    u.default_tenant_id,
    ui.id AS identity_id,
    ui.provider,
    ui.subject,
    ui.external_id,
    ui.provisioning_source
FROM user_identities ui
JOIN users u ON u.id = ui.user_id
WHERE ui.provider = $1
  AND u.public_id = $2
LIMIT 1;

-- name: ListProvisionedUsers :many
SELECT
    u.id,
    u.public_id,
    u.email,
    u.display_name,
    u.deactivated_at,
    u.default_tenant_id,
    ui.id AS identity_id,
    ui.provider,
    ui.subject,
    ui.external_id,
    ui.provisioning_source
FROM user_identities ui
JOIN users u ON u.id = ui.user_id
WHERE ui.provider = $1
ORDER BY u.id
LIMIT $2
OFFSET $3;
```

#### `db/queries/provisioning.sql`

```sql
-- name: UpsertProvisioningSyncState :exec
INSERT INTO provisioning_sync_state (
    source,
    cursor_text,
    last_synced_at,
    last_error_code,
    last_error_message,
    failed_count
) VALUES (
    $1,
    $2,
    now(),
    $3,
    $4,
    $5
)
ON CONFLICT (source) DO UPDATE
SET cursor_text = EXCLUDED.cursor_text,
    last_synced_at = EXCLUDED.last_synced_at,
    last_error_code = EXCLUDED.last_error_code,
    last_error_message = EXCLUDED.last_error_message,
    failed_count = EXCLUDED.failed_count,
    updated_at = now();

-- name: ListDeactivatedUsersWithActiveGrants :many
SELECT DISTINCT
    u.id,
    u.public_id,
    u.email,
    u.display_name,
    u.deactivated_at,
    u.default_tenant_id
FROM users u
JOIN oauth_user_grants g ON g.user_id = u.id
WHERE u.deactivated_at IS NOT NULL
  AND g.revoked_at IS NULL
ORDER BY u.id;
```

#### `db/queries/tenants.sql`

```sql
-- name: UpsertTenantBySlug :one
INSERT INTO tenants (
    slug,
    display_name,
    active
) VALUES (
    $1,
    $2,
    true
)
ON CONFLICT (slug) DO UPDATE
SET display_name = EXCLUDED.display_name,
    active = true,
    updated_at = now()
RETURNING
    id,
    slug,
    display_name,
    active,
    created_at,
    updated_at;

-- name: GetTenantBySlug :one
SELECT
    id,
    slug,
    display_name,
    active,
    created_at,
    updated_at
FROM tenants
WHERE slug = $1
LIMIT 1;

-- name: GetTenantByID :one
SELECT
    id,
    slug,
    display_name,
    active,
    created_at,
    updated_at
FROM tenants
WHERE id = $1
LIMIT 1;

-- name: DeleteTenantMembershipsByUserSource :exec
DELETE FROM tenant_memberships
WHERE user_id = $1
  AND source = $2;

-- name: UpsertTenantMembership :exec
INSERT INTO tenant_memberships (
    user_id,
    tenant_id,
    role_id,
    source,
    active
) VALUES (
    $1,
    $2,
    $3,
    $4,
    true
)
ON CONFLICT (user_id, tenant_id, role_id, source) DO UPDATE
SET active = true,
    updated_at = now();

-- name: ListTenantMembershipRowsByUserID :many
SELECT
    t.id AS tenant_id,
    t.slug AS tenant_slug,
    t.display_name AS tenant_display_name,
    t.active AS tenant_active,
    r.code AS role_code,
    tm.source,
    tm.active AS membership_active
FROM tenant_memberships tm
JOIN tenants t ON t.id = tm.tenant_id
JOIN roles r ON r.id = tm.role_id
WHERE tm.user_id = $1
ORDER BY t.slug, r.code, tm.source;

-- name: ListTenantRoleOverridesByUserID :many
SELECT
    t.id AS tenant_id,
    t.slug AS tenant_slug,
    r.code AS role_code,
    tro.effect
FROM tenant_role_overrides tro
JOIN tenants t ON t.id = tro.tenant_id
JOIN roles r ON r.id = tro.role_id
WHERE tro.user_id = $1
ORDER BY t.slug, r.code, tro.effect;

-- name: UserHasActiveTenant :one
SELECT EXISTS (
    SELECT 1
    FROM tenant_memberships tm
    JOIN tenants t ON t.id = tm.tenant_id
    WHERE tm.user_id = $1
      AND t.id = $2
      AND t.active = true
      AND tm.active = true
)::boolean AS ok;
```

#### `db/queries/users.sql`

```sql
-- name: AuthenticateUser :one
SELECT id
FROM users
WHERE email = @email
  AND password_hash IS NOT NULL
  AND deactivated_at IS NULL
  AND password_hash = crypt(@password, password_hash)
LIMIT 1;

-- name: GetUserByEmail :one
SELECT
    id,
    public_id,
    email,
    display_name,
    deactivated_at,
    default_tenant_id
FROM users
WHERE email = $1
LIMIT 1;

-- name: GetUserByID :one
SELECT
    id,
    public_id,
    email,
    display_name,
    deactivated_at,
    default_tenant_id
FROM users
WHERE id = $1
LIMIT 1;

-- name: GetUserByPublicID :one
SELECT
    id,
    public_id,
    email,
    display_name,
    deactivated_at,
    default_tenant_id
FROM users
WHERE public_id = $1
LIMIT 1;

-- name: CreateOIDCUser :one
INSERT INTO users (
    email,
    display_name,
    password_hash
) VALUES (
    $1,
    $2,
    NULL
)
RETURNING
    id,
    public_id,
    email,
    display_name,
    deactivated_at,
    default_tenant_id;

-- name: UpdateUserProfile :one
UPDATE users
SET email = $2,
    display_name = $3,
    updated_at = now()
WHERE id = $1
RETURNING
    id,
    public_id,
    email,
    display_name,
    deactivated_at,
    default_tenant_id;

-- name: SetUserDeactivatedAt :one
UPDATE users
SET deactivated_at = $2,
    updated_at = now()
WHERE id = $1
RETURNING
    id,
    public_id,
    email,
    display_name,
    deactivated_at,
    default_tenant_id;

-- name: SetUserDefaultTenant :one
UPDATE users
SET default_tenant_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING
    id,
    public_id,
    email,
    display_name,
    deactivated_at,
    default_tenant_id;
```

#### `db/schema.sql`

```sql
--
-- PostgreSQL database dump
--


-- Dumped from database version 18.3 (Debian 18.3-1.pgdg13+1)
-- Dumped by pg_dump version 18.3 (Debian 18.3-1.pgdg13+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: oauth_user_grants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.oauth_user_grants (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    provider text NOT NULL,
    resource_server text NOT NULL,
    provider_subject text NOT NULL,
    refresh_token_ciphertext bytea NOT NULL,
    refresh_token_key_version integer NOT NULL,
    scope_text text NOT NULL,
    granted_by_session_id text NOT NULL,
    granted_at timestamp with time zone DEFAULT now() NOT NULL,
    last_refreshed_at timestamp with time zone,
    revoked_at timestamp with time zone,
    last_error_code text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    tenant_id bigint NOT NULL
);


--
-- Name: oauth_user_grants_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.oauth_user_grants ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.oauth_user_grants_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: provisioning_sync_state; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.provisioning_sync_state (
    source text NOT NULL,
    cursor_text text,
    last_synced_at timestamp with time zone,
    last_error_code text,
    last_error_message text,
    failed_count integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.roles (
    id bigint NOT NULL,
    code text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: roles_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.roles ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.roles_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version bigint NOT NULL,
    dirty boolean NOT NULL
);


--
-- Name: tenant_memberships; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_memberships (
    user_id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    role_id bigint NOT NULL,
    source text NOT NULL,
    active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT tenant_memberships_source_check CHECK ((source = ANY (ARRAY['provider_claim'::text, 'scim'::text, 'local_override'::text])))
);


--
-- Name: tenant_role_overrides; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_role_overrides (
    user_id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    role_id bigint NOT NULL,
    effect text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT tenant_role_overrides_effect_check CHECK ((effect = ANY (ARRAY['allow'::text, 'deny'::text])))
);


--
-- Name: tenants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenants (
    id bigint NOT NULL,
    slug text NOT NULL,
    display_name text NOT NULL,
    active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: tenants_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.tenants ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.tenants_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: user_identities; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_identities (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    provider text NOT NULL,
    subject text NOT NULL,
    email text NOT NULL,
    email_verified boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    external_id text,
    provisioning_source text
);


--
-- Name: user_identities_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.user_identities ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.user_identities_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: user_roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_roles (
    user_id bigint NOT NULL,
    role_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    email text NOT NULL,
    display_name text NOT NULL,
    password_hash text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deactivated_at timestamp with time zone,
    default_tenant_id bigint
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.users ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: oauth_user_grants oauth_user_grants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_user_grants
    ADD CONSTRAINT oauth_user_grants_pkey PRIMARY KEY (id);


--
-- Name: oauth_user_grants oauth_user_grants_user_id_provider_resource_server_tenant_id_ke; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_user_grants
    ADD CONSTRAINT oauth_user_grants_user_id_provider_resource_server_tenant_id_ke UNIQUE (user_id, provider, resource_server, tenant_id);


--
-- Name: provisioning_sync_state provisioning_sync_state_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.provisioning_sync_state
    ADD CONSTRAINT provisioning_sync_state_pkey PRIMARY KEY (source);


--
-- Name: roles roles_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_code_key UNIQUE (code);


--
-- Name: roles roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: tenant_memberships tenant_memberships_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_memberships
    ADD CONSTRAINT tenant_memberships_pkey PRIMARY KEY (user_id, tenant_id, role_id, source);


--
-- Name: tenant_role_overrides tenant_role_overrides_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_role_overrides
    ADD CONSTRAINT tenant_role_overrides_pkey PRIMARY KEY (user_id, tenant_id, role_id, effect);


--
-- Name: tenants tenants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenants
    ADD CONSTRAINT tenants_pkey PRIMARY KEY (id);


--
-- Name: tenants tenants_slug_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenants
    ADD CONSTRAINT tenants_slug_key UNIQUE (slug);


--
-- Name: user_identities user_identities_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_identities
    ADD CONSTRAINT user_identities_pkey PRIMARY KEY (id);


--
-- Name: user_identities user_identities_provider_subject_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_identities
    ADD CONSTRAINT user_identities_provider_subject_key UNIQUE (provider, subject);


--
-- Name: user_roles user_roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_pkey PRIMARY KEY (user_id, role_id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: oauth_user_grants_provider_subject_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX oauth_user_grants_provider_subject_idx ON public.oauth_user_grants USING btree (provider, provider_subject);


--
-- Name: oauth_user_grants_resource_server_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX oauth_user_grants_resource_server_idx ON public.oauth_user_grants USING btree (resource_server);


--
-- Name: oauth_user_grants_tenant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX oauth_user_grants_tenant_id_idx ON public.oauth_user_grants USING btree (tenant_id);


--
-- Name: tenant_memberships_role_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tenant_memberships_role_id_idx ON public.tenant_memberships USING btree (role_id);


--
-- Name: tenant_memberships_tenant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tenant_memberships_tenant_id_idx ON public.tenant_memberships USING btree (tenant_id);


--
-- Name: tenant_role_overrides_tenant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tenant_role_overrides_tenant_id_idx ON public.tenant_role_overrides USING btree (tenant_id);


--
-- Name: user_identities_provider_external_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX user_identities_provider_external_id_key ON public.user_identities USING btree (provider, external_id) WHERE (external_id IS NOT NULL);


--
-- Name: user_identities_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_identities_user_id_idx ON public.user_identities USING btree (user_id);


--
-- Name: user_roles_role_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_roles_role_id_idx ON public.user_roles USING btree (role_id);


--
-- Name: oauth_user_grants oauth_user_grants_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_user_grants
    ADD CONSTRAINT oauth_user_grants_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: oauth_user_grants oauth_user_grants_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_user_grants
    ADD CONSTRAINT oauth_user_grants_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: tenant_memberships tenant_memberships_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_memberships
    ADD CONSTRAINT tenant_memberships_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id) ON DELETE CASCADE;


--
-- Name: tenant_memberships tenant_memberships_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_memberships
    ADD CONSTRAINT tenant_memberships_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: tenant_memberships tenant_memberships_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_memberships
    ADD CONSTRAINT tenant_memberships_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: tenant_role_overrides tenant_role_overrides_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_role_overrides
    ADD CONSTRAINT tenant_role_overrides_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id) ON DELETE CASCADE;


--
-- Name: tenant_role_overrides tenant_role_overrides_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_role_overrides
    ADD CONSTRAINT tenant_role_overrides_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: tenant_role_overrides tenant_role_overrides_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_role_overrides
    ADD CONSTRAINT tenant_role_overrides_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_identities user_identities_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_identities
    ADD CONSTRAINT user_identities_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_roles user_roles_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id) ON DELETE CASCADE;


--
-- Name: user_roles user_roles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: users users_default_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_default_tenant_id_fkey FOREIGN KEY (default_tenant_id) REFERENCES public.tenants(id) ON DELETE SET NULL;


--
-- PostgreSQL database dump complete
--
```

---

## Phase 6. M2M, Docs / OpenAPI, CI, Release, Cutover, 運用

### 目的

人間 user bearer と分離した M2M API を追加し、docs / OpenAPI 公開、CI、release、cutover、運用まで最終形に合わせます。

### この Phase の前提

- browser auth, external bearer, delegated auth, provisioning, tenant-aware auth context がある
- docs_reader などの global role と tenant-aware authz がある

### この Phase の完了条件

- M2M 専用 API surface と `m2mBearerAuth` がある
- `/docs`, `/openapi.*` が本番で認証付き公開になっている
- local password login の本番 cutover が整理されている
- CI / E2E / release asset / runbook が最終形に追随している

### Step 6.1. machine-to-machine 専用の client credentials flow を追加する

Phase 3 の generic JWT bearer verifier をベースに、M2M 固有のルールだけを別 verifier で包みます。

#### Zitadel Console 側で M2M credential を作る

ここで必要なのは、HaoHao repo 内に何かを作ることではなく、**Zitadel が client credentials JWT access token を発行できる credential** です。

この文書では分かりやすさのために「M2M app」と呼ぶことがありますが、local smoke の推奨は **Service User + Client Secret** です。API application でも同じ形の JWT が取れるなら使えますが、まずは Service User で進めてください。

browser login 用 app と external user bearer app とは分けます。M2M token は human user token ではないため、`groups`, project roles, `email`, `preferred_username` などの human-user claim を入れないでください。

#### 公式参照

- Applications overview  
  https://zitadel.com/docs/guides/manage/console/applications-overview  
  用途: API application と client credentials 用設定を確認する
- Client credentials with service users
  https://zitadel.com/docs/guides/integrate/service-users/client-credentials
  用途: machine-to-machine token 取得手順を確認する

#### ここで固定する設定

- credential は Service User + Client Secret を推奨する
- API application を使う場合は client credentials を使える形にする
- token type は **JWT access token**
- expected audience の contract default は `haohao-m2m`
- local smoke では JWT payload の `aud` に実際に入る値へ `M2M_EXPECTED_AUDIENCE` を一時 override してよい
- required scope prefix は `m2m:`

#### `.env` との対応

- M2M audience → `M2M_EXPECTED_AUDIENCE`
- M2M required scope prefix → `M2M_REQUIRED_SCOPE_PREFIX`

#### 追加する設定

`.env.example` に少なくとも次を足します。

```dotenv
M2M_EXPECTED_AUDIENCE=haohao-m2m
M2M_REQUIRED_SCOPE_PREFIX=m2m:
```

#### 追加するテーブル

最小構成なら次を持てば十分です。

- `machine_clients`

必要な列は次です。

- `provider`
- `provider_client_id`
- `display_name`
- `default_tenant_id`
- `allowed_scopes`
- `active`

machine client CRUD は browser session 側の運用 API として追加します。mutating request は `SESSION_ID` と `X-CSRF-Token` を必須にし、global role `machine_client_admin` を持つ user だけ通します。

```text
GET    /api/v1/machine-clients
POST   /api/v1/machine-clients
GET    /api/v1/machine-clients/{id}
PUT    /api/v1/machine-clients/{id}
DELETE /api/v1/machine-clients/{id}
```

`DELETE` は物理削除ではなく `active=false` への soft disable にしてください。

#### 追加するファイル

- `db/queries/machine_clients.sql`
- `backend/internal/auth/m2m_verifier.go`
- `backend/internal/service/machine_client_service.go`
- `backend/internal/api/m2m_*.go`

#### token 検証の固定

- generic JWT bearer verifier で署名 / issuer / audience / expiry / scope を検証する
- `client_id` または `azp` を local machine client に map する
- human-user 前提の claim がある token は M2M endpoint では拒否する
- client が inactive なら 403 にする

#### OpenAPI / Huma の security scheme

M2M は human-user bearer と必ず分けます。

```go
humaConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
    "cookieAuth": {
        Type: "apiKey",
        In:   "cookie",
        Name: "SESSION_ID",
    },
    "bearerAuth": {
        Type:         "http",
        Scheme:       "bearer",
        BearerFormat: "JWT",
    },
    "m2mBearerAuth": {
        Type:         "http",
        Scheme:       "bearer",
        BearerFormat: "JWT",
    },
}
```

#### path と最初の endpoint

```text
user bearer API: /api/external/v1/*
M2M API:         /api/m2m/v1/*
```

最初は次の 1 本で十分です。

```text
GET /api/m2m/v1/self
```

返す内容は、local machine client ID, display name, default tenant, allowed scopes に限定してください。

#### M2M manual smoke

machine client CRUD は browser session + CSRF + `machine_client_admin` で保護されています。manual smoke だけを素早く通す場合は、実 browser login ではなく local password login で管理用 cookie を作って構いません。ただし、`POST /api/v1/login` は `AUTH_MODE=zitadel` の起動中は常に `501` になります。local password login で cookie を取る terminal では、必ず `AUTH_MODE=local` と `ENABLE_LOCAL_PASSWORD_LOGIN=true` を一時 override して backend を起動してください。

まず `:8080` の状態を確認します。

```bash
lsof -nP -iTCP:8080 -sTCP:LISTEN
curl -sS http://127.0.0.1:8080/api/v1/auth/settings | python3 -m json.tool
```

`mode` が `zitadel` の場合、次の password login smoke は `501` になります。起動中 backend を止め、local mode で起動し直します。

```bash
kill <PID from lsof>

bash -lc 'set -a; source .env; export AUTH_MODE=local ENABLE_LOCAL_PASSWORD_LOGIN=true; set +a; go run ./backend/cmd/main'
```

別 terminal で demo user に必要な role を付与し、cookie と CSRF token を取得します。

```bash
make db-up
make seed-demo-user

docker-compose exec -T postgres psql -U haohao -d haohao <<'SQL'
INSERT INTO roles (code)
VALUES ('machine_client_admin'), ('docs_reader')
ON CONFLICT (code) DO NOTHING;

UPDATE users
SET deactivated_at = NULL
WHERE email = 'demo@example.com';

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.code IN ('machine_client_admin', 'docs_reader')
WHERE u.email = 'demo@example.com'
ON CONFLICT DO NOTHING;
SQL

HAOHAO=http://127.0.0.1:8080

curl -fsS -c cookies.txt \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"changeme123"}' \
  "$HAOHAO/api/v1/login"

CSRF_TOKEN=$(awk '$6=="XSRF-TOKEN"{print $7}' cookies.txt)
echo "$CSRF_TOKEN"
```

ここで `curl: (22) ... 501` が返る場合は、backend がまだ `AUTH_MODE=zitadel` で動いています。`/api/v1/auth/settings` の `mode` を確認し、`AUTH_MODE=local` で再起動してください。

次に Zitadel Console で M2M credential を作ります。`M2M_CLIENT_ID` と `M2M_CLIENT_SECRET` は HaoHao の `.env` に入れる値ではありません。manual smoke で Zitadel の token endpoint へ投げる一時値です。

Zitadel Console では次のどちらかの形で client credentials を用意します。

- 推奨: `Service Users` で service user を作り、Actions から `Generate Client Secret` を実行する
- API application を使う場合: HaoHao project 配下に M2M 用 API application を作り、client credentials / Basic auth 用の client secret を発行する

発行画面で表示される値を控えます。

```bash
M2M_CLIENT_ID='<ClientID shown by Zitadel>'
M2M_CLIENT_SECRET='<ClientSecret shown by Zitadel>'
```

`M2M_CLIENT_SECRET` は再表示できない扱いなので、見失った場合は Zitadel Console で再生成してください。M2M token は JWT access token である必要があります。opaque token が返る場合は、対象の service user / application の token settings で access token type を JWT に切り替えてください。

token endpoint から M2M token を取得します。

```bash
set -a; source .env; set +a

M2M_SCOPE_REQUEST='openid profile m2m:read'

TOKEN_RESPONSE=$(curl -fsS -X POST "$ZITADEL_ISSUER/oauth/v2/token" \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'grant_type=client_credentials' \
  --data-urlencode "scope=$M2M_SCOPE_REQUEST" \
  --user "$M2M_CLIENT_ID:$M2M_CLIENT_SECRET")

M2M_TOKEN=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])' <<<"$TOKEN_RESPONSE")
```

M2M token を取ったら、まず payload を確認します。

```bash
python3 - "$M2M_TOKEN" <<'PY'
import base64, json, sys
payload = sys.argv[1].split(".")[1]
payload += "=" * (-len(payload) % 4)
print(json.dumps(json.loads(base64.urlsafe_b64decode(payload)), indent=2, sort_keys=True))
PY
```

payload で次を確認します。

- `aud` に `M2M_EXPECTED_AUDIENCE` が含まれる
- `scope` に `M2M_REQUIRED_SCOPE_PREFIX` で始まる scope がある
- `client_id` または `azp` が machine client の `provider_client_id` と一致する
- human user 用の `email`, `preferred_username`, `groups`, project roles claim が無い

local smoke で期待する payload は、最低限次のような形です。

```json
{
  "aud": ["haohao-m2m-local"],
  "client_id": "haohao-m2m-local",
  "iss": "http://localhost:8081",
  "scope": "m2m:read",
  "sub": "370087165826170883"
}
```

次のように `scope` が `scim:provision` だったり、`groups` が入っていたりする token は **M2M token としては NG** です。

```json
{
  "aud": ["haohao-m2m-local"],
  "client_id": "haohao-m2m-local",
  "groups": ["tenant:acme:todo_user", "external_api_user"],
  "scope": "scim:provision"
}
```

この場合は、Phase 5 の `haohaoGroups` / `haohaoScimScope` Action が M2M token にも効いています。local smoke では、M2M token を取る間だけ `Complement Token` flow の `Pre access token creation` を M2M 用 Action のみにしてください。

```js
function haohaoM2MScope(ctx, api) {
  api.v1.claims.setClaim("scope", "m2m:read");
}
```

M2M token を取り直したら、必ず payload を再確認します。既に発行済みの JWT の中身は変わりません。

JWT payload の `aud` に合わせて backend を起動します。repo root `.env` は編集せず、一時 override で構いません。

```bash
M2M_AUD=$(python3 - "$M2M_TOKEN" <<'PY'
import base64, json, sys
payload = sys.argv[1].split(".")[1]
payload += "=" * (-len(payload) % 4)
claims = json.loads(base64.urlsafe_b64decode(payload))
aud = claims.get("aud", [])
if isinstance(aud, str):
    print(aud)
else:
    print(aud[0])
PY
)

# backend terminal を止めてから:
bash -lc "set -a; source .env; export AUTH_MODE=local ENABLE_LOCAL_PASSWORD_LOGIN=true M2M_EXPECTED_AUDIENCE='$M2M_AUD' M2M_REQUIRED_SCOPE_PREFIX='m2m:'; set +a; go run ./backend/cmd/main"
```

machine client は API で登録します。以下は既に `machine_client_admin` を持つ browser session の cookie と CSRF token がある前提です。

```bash
MACHINE_PROVIDER_CLIENT_ID=$(python3 - "$M2M_TOKEN" <<'PY'
import base64, json, sys
payload = sys.argv[1].split(".")[1]
payload += "=" * (-len(payload) % 4)
claims = json.loads(base64.urlsafe_b64decode(payload))
print(claims.get("client_id") or claims.get("azp") or "")
PY
)

M2M_ALLOWED_SCOPE=$(python3 - "$M2M_TOKEN" <<'PY'
import base64, json, sys
payload = sys.argv[1].split(".")[1]
payload += "=" * (-len(payload) % 4)
claims = json.loads(base64.urlsafe_b64decode(payload))
for scope in claims.get("scope", "").split():
    if scope.startswith("m2m:"):
        print(scope)
        break
PY
)

curl -fsS -X POST http://127.0.0.1:8080/api/v1/machine-clients \
  -b cookies.txt \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -d "{
    \"providerClientId\": \"$MACHINE_PROVIDER_CLIENT_ID\",
    \"displayName\": \"local m2m smoke\",
    \"allowedScopes\": [\"$M2M_ALLOWED_SCOPE\"],
    \"active\": true
  }" | tee /tmp/machine-client-created.json

MACHINE_ID=$(python3 -c 'import json; print(json.load(open("/tmp/machine-client-created.json"))["id"])')
```

登録後、M2M self endpoint を確認します。

```bash
curl -fsS http://127.0.0.1:8080/api/m2m/v1/self \
  -H "Authorization: Bearer $M2M_TOKEN" \
  | python3 -m json.tool
```

拒否系は次を確認してください。

- human user bearer token で `/api/m2m/v1/self` が 2xx にならない
- `provider_client_id` 未登録 token が 403 になる
- machine client を `DELETE /api/v1/machine-clients/{id}` で inactive にした後、同じ M2M token が 403 になる
- `allowedScopes` に無い `m2m:*` scope を含む token が 403 になる

最低限の拒否系として inactive client を確認します。

```bash
curl -fsS -X DELETE "http://127.0.0.1:8080/api/v1/machine-clients/$MACHINE_ID" \
  -b cookies.txt \
  -H "X-CSRF-Token: $CSRF_TOKEN"

curl -sS -o /tmp/m2m-inactive.json -w '%{http_code}\n' \
  http://127.0.0.1:8080/api/m2m/v1/self \
  -H "Authorization: Bearer $M2M_TOKEN"
```

期待値は `403` です。smoke 後も同じ machine client を使うなら active に戻します。

```bash
curl -fsS -X PUT "http://127.0.0.1:8080/api/v1/machine-clients/$MACHINE_ID" \
  -b cookies.txt \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -d "{
    \"providerClientId\": \"$MACHINE_PROVIDER_CLIENT_ID\",
    \"displayName\": \"local m2m smoke\",
    \"allowedScopes\": [\"$M2M_ALLOWED_SCOPE\"],
    \"active\": true
  }"
```

M2M smoke のために `Complement Token` flow を `haohaoM2MScope` のみに変えた場合は、M2M token を取り終わったあとに Zitadel Console の Action / Flow 設定を必ず見直してください。Phase 5 の SCIM smoke には `haohaoScimScope` が必要で、external / browser tenant smoke には `haohaoGroups` が必要です。Zitadel Console の Action / Flow は Git 管理外の外部状態なので、どの smoke のためにどの Action を有効にしたかを作業メモに残してから切り替えると事故りにくいです。

### Step 6.2. docs / OpenAPI を認証付きで公開する

`CONCEPT.md` では、本番の `/docs`, `/openapi.json`, `/openapi.yaml` は認証付きで公開する前提です。ここで制御を入れます。

#### 追加する設定

`.env.example` に次を足します。

```dotenv
DOCS_AUTH_REQUIRED=false
```

本番では `true` にし、`docs_reader` role を持つ user だけ通す形にしてください。

#### 実装方針

- development では `DOCS_AUTH_REQUIRED=false` で従来通り見える
- production では session auth と role check を必須にする
- docs auth は browser 向け session を使う
- `bearerAuth` や `m2mBearerAuth` では docs を開けない方針にする

#### OpenAPI artifact の配布

repo commit に加えて、GitHub Release / release asset に `openapi/openapi.yaml` を載せます。release workflow は次を行うだけで十分です。

1. `make gen`
2. `openapi/openapi.yaml` を validate する
3. tag build 時に release asset として upload する

### Step 6.3. local password login を本番から切り離す

ここまで来ると、browser login も external API も Zitadel を基盤にできます。最後に local password login を恒久経路から外します。

#### 追加する設定

`.env.example` に次を足します。

```dotenv
ENABLE_LOCAL_PASSWORD_LOGIN=true
```

本番では `false` にし、移行直後の rollback escape hatch としてだけ残します。

#### やること

- production の `AUTH_MODE` は `zitadel` 固定にする
- `ENABLE_LOCAL_PASSWORD_LOGIN=false` を本番既定にする
- `demo@example.com / changeme123` の説明を本番向け画面と文書から消す
- `scripts/seed-demo-user.sql` は開発専用と明示する
- `POST /api/v1/login` は dev 限定にするか、最終的に削除する

cutover smoke では `.env` を編集せず一時 override で確認できます。`AUTH_MODE=zitadel` のときは `ENABLE_LOCAL_PASSWORD_LOGIN=true` でも password login は無効です。local password login の無効化だけを確認する場合は、`AUTH_MODE=local` のまま `ENABLE_LOCAL_PASSWORD_LOGIN=false` にしてください。

```bash
bash -lc 'set -a; source .env; export AUTH_MODE=local ENABLE_LOCAL_PASSWORD_LOGIN=false; set +a; go run ./backend/cmd/main'
```

この状態で `POST /api/v1/login` が `501` になり、frontend の password form が demo credential 前提で表示されないことを確認します。

#### Phase 6 manual smoke の完了チェック

この Phase の manual smoke は、最低限次が確認できれば完了でよいです。

```text
M2M token payload:
  aud       -> M2M_EXPECTED_AUDIENCE と一致
  scope     -> m2m: で始まる scope を含む
  client_id -> machine_clients.provider_client_id と一致
  groups / email / preferred_username / project roles claim が無い

M2M API:
  POST /api/v1/machine-clients が machine_client_admin session + CSRF で成功
  GET /api/m2m/v1/self が active machine client で 200
  DELETE /api/v1/machine-clients/{id} 後、同じ token で /api/m2m/v1/self が 403
  PUT /api/v1/machine-clients/{id} で active に戻せる

docs / cutover:
  DOCS_AUTH_REQUIRED=true で cookie 無し /openapi.yaml が 401
  docs_reader session 付き /openapi.yaml が 200
  ENABLE_LOCAL_PASSWORD_LOGIN=false で POST /api/v1/login が 501
```

最後に自動確認を通します。

```bash
go test ./backend/...
npm --prefix frontend run build
make gen
make db-schema
git diff --check
```

### Step 6.4. テスト、CI、運用を最終形へ合わせる

ここで初めて全体のテストと運用をまとめて扱います。refresh token, SCIM, tenant, M2M がそろう前に先回りして書きません。

#### unit test

- login state store の create / consume
- PKCE challenge 生成
- nonce mismatch の拒否
- identity service の link / create 分岐
- bearer token claim 解析
- authz service の role / tenant 判定
- refresh token の暗号化 / 復号 / key version 切替
- provisioning payload の idempotent upsert
- tenant sync の precedence 判定
- machine client の scope / audience 判定

#### integration test

- fake OIDC client を差し込んだ callback handler test
- callback 成功時に cookie が出ること
- callback 後に `GET /api/v1/session` が通ること
- external bearer API の token verify
- SCIM create / update / deactivate の一周
- downstream grant の保存と `invalid_grant` 時の失効
- provider claim 削除時の tenant membership 失効
- M2M bearer API の client mapping と拒否系
- docs auth の拒否 / 許可

#### browser E2E

- Zitadel login から Home 画面表示まで
- logout 後に login 画面へ戻ること
- CSRF 付き mutating request が通ること
- delegated integration connect / revoke が画面導線から確認できること
- 埋め込み配信バイナリでも同じ smoke test が通ること

#### CI で必ずやること

- OpenAPI 3.1 の export
- OpenAPI artifact の validate
- generated client の差分検知
- migration 適用からの `db/schema.sql` 再生成確認
- `go test ./backend/...`
- frontend の typecheck / build
- browser auth, external auth, delegated auth, provisioning, M2M の smoke test

初期の GitHub Actions では、実 Zitadel を使う live smoke までは必須にしません。まずは `go test`, frontend build, `make gen` drift, migration からの `db/schema.sql` drift, OpenAPI validate, Zitadel compose config check を offline CI として固定します。browser auth / external auth / SCIM / M2M の live smoke は、local runbook か self-hosted test environment が安定してから CI に昇格してください。

#### self-hosted Zitadel の運用チェック

`CONCEPT.md` では self-hosted Zitadel を採用するので、最低限次を runbook 化してください。

- backup / restore
- version upgrade 手順
- secret rotation
- 監視とアラート
- JWT signing key / client secret 変更時の影響確認
- 障害時の一時ログイン停止手順

#### Phase 6 後にやること

Zitadel integration としては Phase 6 が最終 Phase です。次は新しい実装 Phase ではなく、release / cutover / 運用固定です。

1. `handmaid2` を push し、GitHub Actions が通ることを確認する
2. PR を作り、OpenAPI / generated client / migration / workflow の差分を review する
3. merge 後に tag を切り、release workflow が `openapi/openapi.yaml` を release asset に載せることを確認する
4. production env を `AUTH_MODE=zitadel`, `ENABLE_LOCAL_PASSWORD_LOGIN=false`, `DOCS_AUTH_REQUIRED=true` に固定する
5. Zitadel の Action / Flow / Service User / SCIM client / external app 設定を runbook 化し、backup / restore / secret rotation / monitoring に組み込む

以後の業務機能は、browser Cookie session, human-user bearer, delegated downstream access, machine-to-machine の 4 系統の入口を崩さずに追加してください。

## Phase 6 Exact Snapshot

Phase 0-2, Phase 3, Phase 4, Phase 5 の exact snapshot を先に replay し、Phase 6 で増える非生成 file をここで上書き / 追加します。

- `backend/internal/db/*.go`, `openapi/openapi.yaml`, `frontend/src/api/generated/*` は `make gen` の生成物なので、この snapshot には再掲しません
- `db/schema.sql` は `make db-schema` の生成物ですが、`sqlc generate` の入力でもあるため、この snapshot に含めます
- `backend/web/dist/*` は frontend build artifact なので、この snapshot には含めません
- repo root `.env` と `dev/zitadel/.env` は Git 管理しません

#### Clean worktree replay checklist

1. Phase 0-2, Phase 3, Phase 4, Phase 5 の exact snapshot を順に replay する
2. Phase 6 で追加 / 更新された source file を適用する
3. DB migration を適用し、schema と生成物をそろえる

```bash
make db-up
make db-schema
make gen
go test ./backend/...
npm --prefix frontend run build
git diff --check
docker compose --env-file dev/zitadel/.env.example -f dev/zitadel/docker-compose.yml config --quiet
```

Phase 6 の source delta は次です。

```text
.env.example
.github/workflows/ci.yml
.github/workflows/release.yml
Makefile
backend/cmd/main/main.go
backend/cmd/openapi/main.go
backend/internal/api/auth_settings.go
backend/internal/api/m2m.go
backend/internal/api/machine_clients.go
backend/internal/api/register.go
backend/internal/app/app.go
backend/internal/auth/m2m_verifier.go
backend/internal/config/config.go
backend/internal/middleware/external_auth.go
backend/internal/service/authz_service.go
backend/internal/service/machine_client_service.go
backend/internal/service/session_service.go
db/migrations/0007_machine_clients.up.sql
db/migrations/0007_machine_clients.down.sql
db/queries/machine_clients.sql
db/schema.sql
frontend/src/views/LoginView.vue
scripts/seed-demo-user.sql
```

generated / artifact delta は次です。

```text
backend/internal/db/machine_clients.sql.go
backend/internal/db/models.go
openapi/openapi.yaml
frontend/src/api/generated/*
```

---

## 全体の完了条件

このチュートリアルが終わったとき、repo は次の状態になっていれば十分です。

- browser 向け login / callback / logout が Zitadel 前提で閉じている
- `SESSION_ID` と `XSRF-TOKEN` の発行、再発行、失効が整理されている
- `(provider, subject)` を DB に保存し、local user と role を解決できる
- external client 向け API は `/api/external/v1/*` の bearer token で動く
- external API / M2M API の bearer token は JWT として local verify できる
- downstream API 呼び出しに使う refresh token が backend-only に暗号化保存され、rotation / revoke できる
- SCIM / enterprise provisioning による create / update / deactivate と reconcile path がある
- org / tenant-aware な local auth context が browser と bearer token の両方で構築できる
- delegated grant schema が tenant-aware に移行済みである
- M2M 専用 API は `/api/m2m/v1/*` の bearer token で動く
- OpenAPI 3.1 artifact に `cookieAuth`, `bearerAuth`, `m2mBearerAuth` が反映される
- `/docs`, `/openapi.json`, `/openapi.yaml` は本番で認証付き公開になっている
- `openapi/openapi.yaml` が repo と GitHub Release の両方で配布される
- production で local password login が無効化されている
- CI と E2E が browser auth / external auth / delegated auth / provisioning / M2M / 生成物 drift を検知できる

ここまで来れば、`CONCEPT.md` が想定している認証まわりの最終形に到達できます。以後の業務機能は、browser 向け Cookie session、human-user bearer、delegated downstream access、machine-to-machine という 4 系統の入口を崩さずに増やしていけます。

---

## 付録. 現在の repo の `.env.example`

Phase 6 実装まで反映済みの **tracked file** は次です。Zitadel login / external bearer / SCIM / M2M を試すときは、この内容を `.env` へコピーしたあとで local の Zitadel Console と JWT payload に合わせて値を埋めてください。repo root `.env` は Git 管理しません。

```dotenv
APP_NAME="HaoHao API"
APP_VERSION=0.1.0
HTTP_PORT=8080

APP_BASE_URL=http://127.0.0.1:8080
FRONTEND_BASE_URL=http://127.0.0.1:5173

DATABASE_URL=postgres://haohao:haohao@127.0.0.1:5432/haohao?sslmode=disable

AUTH_MODE=local
ZITADEL_ISSUER=
ZITADEL_CLIENT_ID=
ZITADEL_CLIENT_SECRET=
ZITADEL_REDIRECT_URI=http://127.0.0.1:8080/api/v1/auth/callback
ZITADEL_POST_LOGOUT_REDIRECT_URI=http://127.0.0.1:5173/login
ZITADEL_SCOPES="openid profile email"

REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=
REDIS_DB=0

SESSION_TTL=24h
LOGIN_STATE_TTL=10m

EXTERNAL_EXPECTED_AUDIENCE=haohao-external
EXTERNAL_REQUIRED_SCOPE_PREFIX=
EXTERNAL_REQUIRED_ROLE=external_api_user
EXTERNAL_ALLOWED_ORIGINS=

M2M_EXPECTED_AUDIENCE=haohao-m2m
M2M_REQUIRED_SCOPE_PREFIX=m2m:

DOWNSTREAM_TOKEN_ENCRYPTION_KEY=
DOWNSTREAM_TOKEN_KEY_VERSION=1
DOWNSTREAM_REFRESH_TOKEN_TTL=2160h
DOWNSTREAM_ACCESS_TOKEN_SKEW=30s
DOWNSTREAM_DEFAULT_SCOPES=offline_access

SCIM_BASE_PATH=/api/scim/v2
SCIM_BEARER_AUDIENCE=scim-provisioning
SCIM_REQUIRED_SCOPE=scim:provision
SCIM_RECONCILE_CRON=0 3 * * *

COOKIE_SECURE=false
DOCS_AUTH_REQUIRED=false
ENABLE_LOCAL_PASSWORD_LOGIN=true
```

将来の Phase で増える環境変数は、**その Phase を実装した時点で** この付録へ追記してください。現時点で未実装の値を先回りで混ぜないでください。
