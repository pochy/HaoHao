# local Zitadel

`v0.2 Auth` は local Zitadel 前提で開始する。`shared dev` はこの段階では採用しない。

## 対象範囲

- `compose.auth.yaml` での local Zitadel 起動
- OIDC application / role / test user / redirect 契約の固定
- backend が読む auth env の固定
- `#9` Redis session store の開始条件と session contract の固定

## 非対象範囲

- `SESSION_ID` cookie の発行・再発行・削除 (`#10`)
- `GET /api/v1/session` の本実装差し替え (`#11`)
- `XSRF-TOKEN` 発行と CSRF middleware (`#12`)
- login / logout / refresh endpoint (`#13`)
- docs / OpenAPI の閲覧制御実装 (`#15`-`#18`)

## 固定した境界条件

- browser auth は `BFF + Cookie session` のみを使う
- browser に access token / refresh token は保持させない
- external API は bearer token のまま据え置く
- `SESSION_ID` は opaque なランダム値とし、Redis lookup key は `haohao:session:{session_id}` に固定する
- session payload は `user_id`, `zitadel_subject`, `roles`, `created_at`, `expires_at`, `csrf_secret`
- session store の責務は `backend/internal/service/` に閉じる
- local user 同定契約は `Zitadel sub -> db.app_users.zitadel_subject`

## local auth 契約

- compose file: `compose.auth.yaml`
- compose env: `compose.auth.env`
- backend env: `.env.auth`
- issuer URL: `http://localhost:8081`
- project: `haohao-local`
- OIDC application: `haohao-browser-local`
- roles: `app:user`, `docs:read`
- test user: `haohao.dev@zitadel.localhost`
- redirect URI: `http://localhost:8080/auth/callback`
- post-logout redirect URI: `http://localhost:8080/auth/logout/callback`
- frontend origin: `http://localhost:5173`
- scopes: `openid profile email`
- session TTL: absolute `8h`

OIDC application は Web application とし、backend が `client_id` / `client_secret` を保持する confidential client として扱う。local の HTTP redirect を許可するため `devMode=true` に固定する。

## 手順

1. 初回だけ `compose.auth.env.example` を `compose.auth.env` にコピーする
2. `make compose-auth-up` を実行する
3. `make compose-auth-seed` を実行する
4. `curl http://localhost:8081/.well-known/openid-configuration` を確認する
5. `make backend` を実行する

`compose.auth.yaml` は `compose.auth.env` 前提なので、日常運用では `docker-compose -f compose.auth.yaml ...` を直接打たず `make compose-auth-*` を使う。

`make compose-auth-seed` は次を行う。

- bootstrap PAT で management API に接続する
- project / roles / OIDC application / test user / user grant を再現する
- `.env.auth` に `ZITADEL_CLIENT_ID` と `ZITADEL_CLIENT_SECRET` を書き出す
- 既存 app と `.env.auth` の client ID が一致する場合は、seed 再実行でも client secret を再生成せず再利用する
- client secret を再生成するのは、初回 app 作成時か `.env.auth` が欠落または app と不整合な場合だけにする

## ログイン確認

公式の Docker Compose quickstart では `http://localhost:8080` を開くと login screen が表示され、`/ui/console?login_hint=zitadel-admin@zitadel.localhost` で初期 admin login ができる。この repo では backend と衝突させないため公開 port を `8081` に変えているので、同じ導線を `http://localhost:8081` に読み替えて使う。

### login screen

- `http://localhost:8081/` を開く
- local compose では `/ui/v2/login` に redirect される
- `http://localhost:8081/.well-known/openid-configuration` の `authorization_endpoint` が `http://localhost:8081/oauth/v2/authorize` になっていれば、public URL 設定は期待通り

### admin console login

- URL: `http://localhost:8081/ui/console?login_hint=zitadel-admin@zitadel.localhost`
- login name: `zitadel-admin@zitadel.localhost`
- password: `compose.auth.env` の `ZITADEL_FIRSTINSTANCE_ORG_HUMAN_PASSWORD`
- example env の既定値: `Password1!`

この login は、project / application / user / role が seed 済みかを console 上で目視確認する用途に使う。local auth の正本は seed script なので、console で手作業した内容を canonical にはしない。

### Hosted Login を test user で確認する

`#13` の login start endpoint はまだ未実装なので、現時点では browser app 用 OIDC application に対して authorization request を直接作って確認する。

1. `make compose-auth-seed` を実行して `.env.auth` を最新化する
2. 次の URL を browser で開く

```text
http://localhost:8081/oauth/v2/authorize?client_id=<ZITADEL_CLIENT_ID>&response_type=code&scope=openid%20profile%20email&redirect_uri=http%3A%2F%2Flocalhost%3A8080%2Fauth%2Fcallback&state=dev-local&nonce=dev-local
```

`<ZITADEL_CLIENT_ID>` には `.env.auth` の `ZITADEL_CLIENT_ID` を入れる。local では request を投げると `http://localhost:8081/ui/v2/login/login?authRequest=...` に redirect される。

3. Hosted Login で次を入力する

- login name: `haohao.dev@zitadel.localhost`
- password: `compose.auth.env` の `HAOHAO_ZITADEL_TEST_USER_PASSWORD`
- example env の既定値: `Password1!`

4. 認証が通ると `http://localhost:8080/auth/callback?code=...&state=...` に戻る

現時点では callback handler 自体は未実装なので、ここで HaoHao 側の login 完了までは期待しない。この確認の目的は、local Zitadel の Hosted Login が起動し、seed 済み app / redirect URI / test user credential が有効であることを確かめること。

### よくある詰まりどころ

- `Instance not found` が出る場合は、アクセス先が `localhost:8081` からずれていないか確認する
- `compose.auth.yaml` 単体では env が足りないので、`make compose-auth-up` を使う
- `compose.auth.env` の `ZITADEL_FIRSTINSTANCE_*` は初回 setup 時だけ反映される。既存 instance の値を変えたい場合は console か management API で更新する

## 参考

- Official Docker Compose quickstart: `https://zitadel.com/docs/self-hosting/deploy/compose`
- Hosted Login overview: `https://zitadel.com/docs/guides/integrate/login/hosted-login`
- External access / `Instance not found`: `https://zitadel.com/docs/self-hosting/manage/custom-domain`

## #9 開始条件

次を満たしたら `#9 Redis session store` の実装を開始できる。

- local Zitadel が `http://localhost:8081` で起動している
- `haohao-browser-local` と test user が seed で再現できる
- issuer / app / user / redirect URI / logout URI / env var がこの文書と example env に固定されている
- session contract が次で未決定でない

`#9` の session contract:

- lookup unit: `SESSION_ID`
- Redis key: `haohao:session:{session_id}`
- payload fields: `user_id`, `zitadel_subject`, `roles`, `created_at`, `expires_at`, `csrf_secret`
- TTL: absolute `8h`
- idle timeout: なし
- refresh: `#10` 以降で明示 endpoint 経由のみ
- logout: server-side invalidation で `delete(session_id)`

## 受け入れ基準

- `make compose-auth-up` で local Zitadel が起動する
- `curl http://localhost:8081/.well-known/openid-configuration` が成功する
- `make compose-auth-seed` で `haohao-browser-local` と test user が再現できる
- `.env.auth` を見れば backend が読む auth env が 1 回で分かる
- Redis session store の置き換え点が `backend/internal/service/` に閉じている
