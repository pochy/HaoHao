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
- `todo_user`

この 3 つは本文の Phase 3 以降で local role の写像先として使います。ここで先に名前を固定しておくと、後で role 名がぶれません。

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

この時点では browser login 用 application だけを作れば十分です。**external user bearer app, SCIM client, M2M app はまだ作らなくて構いません**。それらは `Phase 3`, `Phase 5`, `Phase 6` で追加します。

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

## Phase 3 Exact Snapshot

Phase 0-2 の exact snapshot はそのまま使い、Phase 3 で変わった file だけをここに追加します。

- この節の block は、現在の Phase 3 実装に合わせた **exact delta** です
- `backend/internal/db/*.go`, `db/schema.sql`, `openapi/openapi.yaml`, `frontend/src/api/generated/*` は `make db-schema` と `make gen` の生成物です
- `go.work.sum` は tool 実行で再生成されることがありますが、この repo の正本には含めません

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

---

## Phase 5. Provisioning と Tenant-Aware Auth Context

### 目的

just-in-time login だけに依存せず、enterprise provisioning と org / tenant-aware な auth context を成立させます。

### この Phase の前提

- browser auth, external bearer API, delegated auth がある
- tenant sync 用 claim contract が `groups` に固定されている
- generic JWT bearer verifier がある

### この Phase の完了条件

- SCIM / provisioning が generic bearer verifier を使って保護されている
- create / update / deactivate / reconcile がある
- deprovisioning が session / delegated grant とつながっている
- browser と bearer token の両方で tenant-aware auth context を構築できる
- delegated grant schema が tenant-aware に昇格している

### Step 5.1. SCIM / enterprise provisioning を入れる

ここでは Phase 6 の M2M verifier を待ちません。Phase 3 で作った generic JWT bearer verifier を `audience / scope` 違いで再利用します。

#### Zitadel Actions と SCIM 用 client の設定

Phase 5 に入ったら、Zitadel Console 側でも次を用意してください。

- `groups` claim を出すための Action
- SCIM bearer 用の dedicated client

#### `groups` claim を出す理由

このチュートリアルでは app code が `organization_id` / `organization_ids` を見ません。tenant membership claim contract は **top-level の `groups`** に固定しています。

そのため、provider 側では Zitadel Actions などを使って、必要な membership 情報を `groups` claim に寄せてから backend へ渡してください。

#### 公式参照

- Actions overview  
  https://zitadel.com/docs/apis/actions/introduction  
  用途: Action の概念と flow を確認する

- Claims in ZITADEL  
  https://zitadel.com/docs/apis/openidoauth/claims  
  用途: custom claims と reserved claims の挙動を確認する

#### SCIM 用 client の設定

- browser app / external user bearer app とは **別 client**
- audience は `scim-provisioning`
- required scope は `scim:provision`

#### `.env` との対応

- SCIM audience → `SCIM_BEARER_AUDIENCE`
- SCIM required scope → `SCIM_REQUIRED_SCOPE`

#### 追加する設定

`.env.example` に少なくとも次を足します。

```dotenv
SCIM_BASE_PATH=/api/scim/v2
SCIM_BEARER_AUDIENCE=scim-provisioning
SCIM_REQUIRED_SCOPE=scim:provision
SCIM_RECONCILE_CRON=0 3 * * *
```

#### 追加するファイル

- `backend/internal/api/scim.go`
- `backend/internal/service/provisioning_service.go`
- `backend/internal/jobs/provisioning_reconcile.go`

#### まず実装する endpoint

完全な SCIM 全面実装ではなく、最初は次で十分です。

```text
POST   /api/scim/v2/Users
PUT    /api/scim/v2/Users/{id}
PATCH  /api/scim/v2/Users/{id}
GET    /api/scim/v2/Users/{id}
DELETE /api/scim/v2/Users/{id}
```

`DELETE` は physical delete にせず、**deactivate と同義**にしてください。user lifecycle を audit 可能に保つためです。

#### 認証の固定

- SCIM endpoint は generic JWT bearer verifier を使う
- `SCIM_BEARER_AUDIENCE` と `SCIM_REQUIRED_SCOPE` を検証する
- SCIM 用 bearer と external user bearer は path も audience も分ける

### Step 5.2. provisioning schema と deactivation 連携を入れる

#### 追加するテーブルと列

次の情報を保持できるようにしてください。

- `user_identities.external_id`
- `user_identities.provisioning_source`
- `users.deactivated_at`
- `provisioning_sync_state`

`db/queries/provisioning.sql` では少なくとも次が必要です。

- external ID で user / identity を引く
- external ID で upsert する
- deactivate / reactivate を切り替える
- 最終同期カーソルや同期時刻を保存する

#### failure handling

- SCIM payload の validation error は 400
- unknown external ID の update / patch は 404
- email collision など operator 対応が必要な問題は 409
- reconcile job は 1 user 失敗で全体を止めず、`provisioning_sync_state` に失敗件数を残す

#### deprovisioning と downstream grant revoke のつなぎ方

deactivation 後の downstream grant revoke は **`backend/internal/jobs/provisioning_reconcile.go` にまとめて持たせる**と構成が単純です。

- missed SCIM update の補修
- deactivated user の local session 無効化
- deactivated user の downstream grant revoke / cleanup

この 3 つを同じ job に寄せると、「deprovisioning 後にどこで revoke するか」が曖昧になりません。

### Step 5.3. organization / tenant-aware auth context を入れる

role だけでは「どの tenant のどの権限か」が表現しきれません。ここで browser でも bearer token client でも同じ `AuthContext` を組み立てられるようにします。

#### 追加するテーブル

最小構成なら次を追加してください。

- `tenants`
- `tenant_memberships`
- `tenant_role_overrides`

`tenant_memberships` には source を持たせます。

- `provider_claim`
- `scim`
- `local_override`

#### provider claim の前提

このチュートリアルでは、provider から見える tenant membership claim を **`groups` のみ**に固定します。

- claim 名は `groups`
- 値の整形は Zitadel Action など provider 側で済ませる
- app code は複数候補の claim 名を見ない

#### precedence の固定

effective membership は必ず次の順で決めます。

1. 直近の login または provisioning sync で得た provider membership を土台にする
2. `local deny override` を適用して membership / role を削る
3. `local allow override` を適用して role を足す
4. active tenant が未選択なら `users.default_tenant_id` を使う
5. provider 側から membership が消えたら、その sync 成功時点で `provider_claim` source を inactive にする
6. ただし `local allow override` がある role だけは残せる

この順序を文書内で固定してください。実装者が独自に precedence を決めないようにするためです。

#### browser と bearer の auth context

- browser: session middleware が `user_id` を取り、`authzService.BuildBrowserContext(userID, activeTenantID)` を呼ぶ
- external user bearer: token から `subject` を取り、identity 解決後に `authzService.BuildBearerContext(userID, requestedTenantID)` を呼ぶ
- requested tenant は `X-Tenant-ID` header で受け、無ければ default tenant を使う
- tenant が無効なら 403 を返し、operation ではなく service で最終判断する

#### delegated grant schema を tenant-aware 化する migration

Phase 4 では `oauth_user_grants` を tenant 非依存で作りました。ここで tenant-aware に昇格させます。

##### migration 手順

1. `oauth_user_grants` に nullable な `tenant_id` を追加する
2. 既存 row を backfill する
3. unresolved row を revoke する
4. `tenant_id` を `NOT NULL` に上げる
5. unique key を `(user_id, provider, resource_server, tenant_id)` に置き換える

##### backfill のルール

- まず grant 作成時に保持していた consent tenant を使う
- 無ければ `users.default_tenant_id` を使う
- それでも解決できない row は **推測しない**
- 解決不能 row は `revoked_at` を入れて再 consent 必須にする

こうすると tenant 導入前の historical grant を危険に推測して別 tenant へ紐づける事故を防げます。

#### この Phase の手動確認

1. SCIM create が idempotent に upsert される
2. PATCH で display name / email / active が反映される
3. deactivate で local session と downstream grant が失効方向へ動く
4. provider claim 追加で tenant membership が増える
5. provider claim 削除で access が即時に落ちる
6. local allow / deny override の precedence が守られる
7. browser と bearer token client が同じ effective membership を得る
8. unresolved historical grant が revoke され、再 consent が必要になる

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

#### Zitadel Console 側で M2M app を作る

M2M 用 app は browser login 用 app と external user bearer app とは分けてください。

#### 公式参照

- Applications overview  
  https://zitadel.com/docs/guides/manage/console/applications-overview  
  用途: API application と client credentials 用設定を確認する

#### ここで固定する設定

- application type は API 系の machine-to-machine 用 application
- client credentials を使える形にする
- token type は **JWT access token**
- expected audience は `haohao-m2m`

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

- `provider_client_id`
- `display_name`
- `default_tenant_id`
- `allowed_scopes`
- `active`

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

#### self-hosted Zitadel の運用チェック

`CONCEPT.md` では self-hosted Zitadel を採用するので、最低限次を runbook 化してください。

- backup / restore
- version upgrade 手順
- secret rotation
- 監視とアラート
- JWT signing key / client secret 変更時の影響確認
- 障害時の一時ログイン停止手順

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

Phase 0-2 実装まで反映済みの **tracked file** は次です。Zitadel login を試すときは、この内容を `.env` へコピーしたあとで `AUTH_MODE=zitadel` と `ZITADEL_*` を埋めてください。

```dotenv
APP_NAME="HaoHao API"
APP_VERSION=0.1.0
HTTP_PORT=8080

DATABASE_URL=postgres://haohao:haohao@127.0.0.1:5432/haohao?sslmode=disable

APP_BASE_URL=http://127.0.0.1:8080
FRONTEND_BASE_URL=http://127.0.0.1:5173

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

将来の Phase で増える環境変数は、**その Phase を実装した時点で** この付録へ追記してください。現時点で未実装の値を先回りで混ぜないでください。
