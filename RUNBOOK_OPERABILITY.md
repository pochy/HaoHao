# HaoHao operability runbook

## Preconditions

- 対象 version / commit / image tag を決める。
- PostgreSQL backup と restore 手順を確認する。
- migration の適用状態を確認する。
- secret / env の注入元を確認する。
- Zitadel redirect URI が single binary の origin を向いていることを確認する。

single binary では、frontend は Vite dev server ではなく Go process が返します。

```dotenv
APP_BASE_URL=https://app.example.com
FRONTEND_BASE_URL=https://app.example.com
ZITADEL_REDIRECT_URI=https://app.example.com/api/v1/auth/callback
ZITADEL_POST_LOGOUT_REDIRECT_URI=https://app.example.com/login
```

## Binary Deploy

1. Release binary を download するか、`make binary` で build する。
2. `.env` を binary と同じ directory に置くか、process manager から環境変数を注入する。
3. traffic を切り替える前に DB migration を適用する。
4. 新しい process を起動する。
5. `/healthz` と `/readyz` が `200` になることを確認する。
6. `BASE_URL=<app-url> make smoke-operability` を実行する。
7. browser login が変更対象なら、実 browser で login / logout を確認する。

## Docker Deploy

1. 対象 image を pull または build する。
2. container に渡す env を確認する。
3. migration job を app container とは別に実行する。
4. 新しい container を起動する。
5. container 外から `BASE_URL=<app-url> make smoke-operability` を実行する。

production image は `scratch` runtime なので shell がありません。container 内で調査する前提にせず、log、health endpoint、debug image、builder stage を使います。

## Rollback

1. 新しい process / container を traffic から外す。
2. 直前の binary / image を起動する。
3. down migration は自動実行しない。
4. `/healthz`, `/readyz`, `/api/v1/session`, `/openapi.yaml` を確認する。
5. `BASE_URL=<app-url> make smoke-operability` を実行する。
6. Zitadel redirect URI を変更していた場合は、以前の origin に戻す必要があるか確認する。

rollback しやすくするため、migration は forward-compatible に作ります。schema を戻す判断は、backup / restore と data loss risk を確認してから別作業として扱います。

## Zitadel Redirect URI Update

single binary deployment では次を登録します。

- Browser callback: `${APP_BASE_URL}/api/v1/auth/callback`
- Post logout: `${APP_BASE_URL}/login`

local smoke なら例は次です。

```dotenv
APP_BASE_URL=http://127.0.0.1:8080
FRONTEND_BASE_URL=http://127.0.0.1:8080
ZITADEL_REDIRECT_URI=http://127.0.0.1:8080/api/v1/auth/callback
ZITADEL_POST_LOGOUT_REDIRECT_URI=http://127.0.0.1:8080/login
```

`http://127.0.0.1:5173` は Vite dev server 用です。single binary / production の redirect URI には使いません。

## Smoke Order

1. `/healthz` が `200`。
2. `/readyz` が `200`。
3. 未ログインの `/api/v1/session` が `401 application/problem+json`。
4. `/openapi.yaml` が OpenAPI YAML。
5. `/api/v1/auth/callback?error=forced` が `APP_BASE_URL` 側の `/login?error=oidc_callback_failed` に redirect する。
6. 認証設定を変えた場合だけ、manual browser login / logout を確認する。

script で確認する場合は次を使います。

```bash
BASE_URL=https://app.example.com make smoke-operability
```

## Readiness

`/readyz` は PostgreSQL と Redis を確認します。

`READINESS_CHECK_ZITADEL=true` の場合だけ Zitadel discovery endpoint も確認します。local development と IdP 障害時の切り分けでは、まず `READINESS_CHECK_ZITADEL=false` に戻して DB / Redis / app process の問題かを分けます。

## Provisioning Reconcile

SCIM reconcile scheduler は default では無効です。

```dotenv
SCIM_RECONCILE_ENABLED=false
SCIM_RECONCILE_INTERVAL=1h
SCIM_RECONCILE_TIMEOUT=30s
SCIM_RECONCILE_RUN_ON_STARTUP=false
```

staging / production で有効化する場合は、まず 1 process だけで有効にします。multi-replica で全 replica が scheduler を持つと、同じ reconcile が複数 process から走ります。
