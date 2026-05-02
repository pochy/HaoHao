# P10 横断拡張機能実装チュートリアル

## この文書の目的

この文書は、`deep-research-report.md` の **P10 チュートリアル作成** を、現在の HaoHao に実装できる順番へ分解したチュートリアルです。

P7 では、security hardening、outbox、idempotency、notification、invitation、file upload、tenant settings、tenant data export、data lifecycle、backup / restore smoke までを入れました。P8 では OpenAPI を `full` / `browser` / `external` に分け、P9 では browser E2E を追加しました。

P10 では、その上に載せる横断機能をまとめて扱います。P7 初期版へ詰め込まず、P7 で入れた `outbox_events`、`audit_events`、`tenant_settings`、`file_objects`、`tenant_data_exports`、metrics / tracing、P8 の `openapi/browser.yaml`、P9 の Playwright E2E を前提にして、次の機能を段階的に追加します。

- outbound webhooks
- Customer Signals の CSV import / CSV export
- Customer Signals の search / cursor pagination / saved filters
- support access / impersonation
- feature flags / entitlements

この文書は `TUTORIAL.md` / `TUTORIAL_SINGLE_BINARY.md` / `TUTORIAL_P7_WEB_SERVICE_COMMON.md` と同じように、対象ファイル、主要コード方針、確認コマンド、完了条件まで追える形にしています。

## この文書が前提にしている現在地

このチュートリアルを始める前の repository は、少なくとも次の状態にある前提で進めます。

- P7 の `0012_web_service_common` migration が適用済み
- `outbox_events` があり、worker が retry / dead-letter 相当の状態を扱える
- `audit_events` があり、service から audit event を残せる
- `tenant_settings` があり、file quota / rate limit override / features JSON の入口がある
- `file_objects` があり、local file storage driver で upload / download できる
- `tenant_data_exports` があり、tenant data export request / status / download の入口がある
- `/metrics`、request log、trace context がある
- Customer Signals の tenant-aware CRUD と UI がある
- Tenant Admin UI があり、tenant detail から settings / exports を操作できる
- `openapi/browser.yaml` が frontend generated SDK の入力になっている
- `make gen` が sqlc / full OpenAPI / browser OpenAPI / external OpenAPI / frontend SDK を更新できる
- `make e2e` が local single binary に対して Playwright を実行できる
- `make binary` で frontend を embed した single binary を作れる

この P10 では、決済、請求、pricing plan、S3 / GCS driver、外部 queue、realtime notification、multi-region delivery は扱いません。これらは entitlements と webhook / import / export の最小実装が安定した後で追加します。

## 完成条件

このチュートリアルの完了条件は次です。

- `0013_p10_cross_cutting_extensions` migration が追加される
- webhook endpoint、webhook delivery log、Customer Signals import job、saved filter、support access session、feature definition / tenant entitlement の schema がある
- `db/schema.sql` と sqlc generated code が更新される
- outbound webhook endpoint を tenant admin が作成 / 更新 / 削除 / secret rotate できる
- webhook delivery は HMAC-SHA256 signature を付ける
- webhook request に `X-HaoHao-Event-ID` と `X-HaoHao-Signature` が入る
- webhook delivery は retry、response preview、dead-letter、manual retry を持つ
- Customer Signals の CSV import job を request でき、status UI で成功 / validation error を確認できる
- Customer Signals の CSV export job を request でき、ready 後に file download できる
- Customer Signals list が `q`、filter、cursor、limit に対応する
- cursor pagination は作成日時降順で重複と欠落を起こさない
- saved filters は tenant と owner user で分離される
- support access は global role `support_agent`、理由、期限、明示 banner、終了操作、audit event を持つ
- support access 中の mutation は actor と impersonated user の両方を audit に残す
- feature flags / entitlements は `tenant_settings.features` から独立し、tenant ごとの有効化と上限値を扱える
- webhook、import/export、support access は entitlement で gate できる
- frontend generated SDK は `openapi/browser.yaml` 由来のまま更新される
- Tenant Admin UI から webhooks、imports、exports、entitlements を操作できる
- Playwright E2E で webhook 作成、CSV import/export、saved filter、support access banner、entitlement gate を確認できる
- `make gen`、`go test ./backend/...`、`npm --prefix frontend run build`、`make binary`、`make smoke-common-services`、`make smoke-p10`、`make e2e` が通る

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | P10 境界 | 命名、role、event、entitlement、route の方針を固定する |
| Step 2 | `db/migrations/0013_p10_cross_cutting_extensions.*.sql` | 横断拡張の schema を追加する |
| Step 3 | `db/queries/*.sql` | sqlc 用 query を追加する |
| Step 4 | webhook service / API / worker | endpoint、signature、delivery、retry、dead-letter を実装する |
| Step 5 | import / export jobs | Customer Signals CSV import / export と status UI を実装する |
| Step 6 | search / cursor / saved filters | Customer Signals list を検索と cursor pagination にする |
| Step 7 | support access | impersonation、reason、expiry、banner、audit を実装する |
| Step 8 | feature flags / entitlements | `tenant_settings.features` から独立した tenant entitlement を実装する |
| Step 9 | Huma / OpenAPI / frontend | API registration、browser spec、SDK、Vue UI を接続する |
| Step 10 | smoke / E2E / CI | `smoke-p10`、Playwright、CI drift check を追加する |

## 先に決める方針

### P10 は P7 の土台を置き換えない

P10 は、P7 の outbox、audit、file storage、tenant settings、metrics を置き換えません。P10 では、それらを使う側の機能を追加します。

- webhook delivery は `outbox_events` で retry する
- CSV import / export の input / output は `file_objects` を使う
- import / export の非同期処理は outbox handler に寄せる
- support access と entitlement 変更は audit に残す
- metrics label に tenant id、user id、email、webhook URL、cursor、signature は入れない

### browser API に限定する

P10 の API は browser session + Cookie + CSRF を前提にします。

- tenant admin API は `/api/v1/admin/tenants/{tenantSlug}/...`
- Customer Signals list / saved filters は `/api/v1/customer-signals...`
- support access は `/api/v1/support/access...`
- external bearer、M2M、SCIM には P10 初期版では追加しない

そのため、P8 の surface では P10 の新 API を `browser` に含め、`external` には含めません。

### role は最小追加にする

新しく追加する global role は `support_agent` だけです。

既存 role の使い分けは次です。

| role | 用途 |
| --- | --- |
| `tenant_admin` | tenant 内の webhooks、imports、exports、entitlements 管理 |
| `customer_signal_user` | Customer Signals の list / search / saved filters / import result 確認 |
| `support_agent` | support access の開始 / 終了 |

`support_agent` は tenant admin UI から grant できる tenant role ではありません。global role として seed / provider claim / 管理者用 DB 操作で付与します。

### entitlement は tenant settings から独立させる

P7 の `tenant_settings.features` は入口としては便利ですが、billing、pricing plan、support access、webhook のような機能 gate の正本にし続けると、型も履歴も曖昧になります。

P10 では次を正本にします。

- `feature_definitions`: feature の catalog
- `tenant_entitlements`: tenant ごとの override と limit

`tenant_settings.features` は互換用の読み取り元としてだけ扱います。P10 migration で既存 JSON を可能な範囲で `tenant_entitlements` に backfill し、以後の UI は `tenant_entitlements` を更新します。

### webhook secret は平文保存しない

webhook signature secret は delivery 時に復号が必要です。そのため hash だけでは足りません。

P10 では、既存の delegated refresh token 暗号化と同じ AES-GCM pattern を汎用化し、次の config を追加します。

```dotenv
WEBHOOK_SECRET_ENCRYPTION_KEY=
WEBHOOK_SECRET_KEY_VERSION=1
```

`WEBHOOK_SECRET_ENCRYPTION_KEY` は base64 encoded 32 bytes です。DB には `secret_ciphertext` と `secret_key_version` だけを保存します。response、audit metadata、log、metrics には secret を出しません。

### CSV import は validation first にする

CSV import は、1 行ずつ即座に DB へ入れるのではなく、まず validation 結果を job に残します。

- header を検証する
- row ごとの validation error を `import_error_rows` に残す
- error がある場合は既定では DB へ反映しない
- `mode=validate_only` と `mode=insert` を分ける

最初は `upsert` ではなく `insert` だけにします。既存 Customer Signal の更新 import は、重複判定と監査の扱いが重くなるため後続にします。

### cursor pagination は offset を使わない

Customer Signals は増え続ける業務データです。P10 では offset pagination ではなく cursor pagination にします。

cursor は base64url encoded JSON とし、内容は次に固定します。

```json
{"createdAt":"2026-04-25T12:00:00Z","id":12345}
```

query は `ORDER BY created_at DESC, id DESC` とし、次 page は `(created_at, id) < (:created_at, :id)` で取得します。

### support access は必ず画面に出す

support access は、内部 user が別 user として tenant を操作できる強い機能です。成功条件は API が動くことではなく、操作者とレビュー者が後から説明できることです。

そのため、次を必須にします。

- start 時に reason が必須
- expiry が必須
- UI header に support access banner を表示する
- every request log / audit event に support access id を入れる
- end 操作を明示する
- expired session は自動的に通常 session へ戻す

## Step 1. P10 の境界と命名方針を固定する

最初に、追加する domain 名、event 名、route 名、feature code を固定します。

### 1-1. event type

P10 で追加する outbox event type は次です。

| event type | 用途 |
| --- | --- |
| `webhook.delivery_requested` | webhook delivery 1 件を送信する |
| `customer_signal_import.requested` | Customer Signals CSV import job を処理する |
| `customer_signal_export.requested` | Customer Signals CSV export job を処理する |

webhook の payload として外部に送る business event type は次から始めます。

| webhook event | 発火元 |
| --- | --- |
| `customer_signal.created` | Customer Signal 作成 |
| `customer_signal.updated` | Customer Signal 更新 |
| `customer_signal.deleted` | Customer Signal soft delete |
| `customer_signal_import.completed` | import job 完了 |
| `customer_signal_export.ready` | export job ready |

外部 webhook payload の `eventId` は `webhook_deliveries.public_id` を使います。内部 outbox event id は外部に出しません。

### 1-2. feature code

`feature_definitions` に seed する code は次です。

| feature code | default | 用途 |
| --- | --- | --- |
| `webhooks.enabled` | false | outbound webhook API / delivery |
| `customer_signals.import_export` | false | Customer Signals CSV import / export |
| `customer_signals.saved_filters` | true | saved filter UI |
| `support_access.enabled` | false | support access / impersonation |

Customer Signals の search / cursor pagination は基本機能として常に有効にします。検索 API を feature gate すると、list UI の default 挙動も複雑になるためです。

### 1-3. public API route

P10 の browser API は次にします。

| route | 用途 |
| --- | --- |
| `GET /api/v1/admin/tenants/{tenantSlug}/webhooks` | webhook endpoint 一覧 |
| `POST /api/v1/admin/tenants/{tenantSlug}/webhooks` | webhook endpoint 作成 |
| `GET /api/v1/admin/tenants/{tenantSlug}/webhooks/{webhookPublicId}` | webhook endpoint detail |
| `PUT /api/v1/admin/tenants/{tenantSlug}/webhooks/{webhookPublicId}` | webhook endpoint 更新 |
| `POST /api/v1/admin/tenants/{tenantSlug}/webhooks/{webhookPublicId}/rotate-secret` | secret rotate |
| `DELETE /api/v1/admin/tenants/{tenantSlug}/webhooks/{webhookPublicId}` | webhook endpoint 削除 |
| `GET /api/v1/admin/tenants/{tenantSlug}/webhooks/{webhookPublicId}/deliveries` | delivery log |
| `POST /api/v1/admin/tenants/{tenantSlug}/webhooks/{webhookPublicId}/deliveries/{deliveryPublicId}/retry` | manual retry |
| `GET /api/v1/admin/tenants/{tenantSlug}/imports` | import job 一覧 |
| `POST /api/v1/admin/tenants/{tenantSlug}/imports` | CSV import job 作成 |
| `GET /api/v1/admin/tenants/{tenantSlug}/imports/{importPublicId}` | import job detail |
| `GET /api/v1/admin/tenants/{tenantSlug}/exports` | export job 一覧。既存 tenant data export route を拡張する |
| `POST /api/v1/admin/tenants/{tenantSlug}/exports` | export job 作成。`resourceType` 省略時は既存互換で `tenant_data` |
| `GET /api/v1/admin/tenants/{tenantSlug}/exports/{exportPublicId}` | export job detail |
| `GET /api/v1/admin/tenants/{tenantSlug}/exports/{exportPublicId}/download` | export file download |
| `GET /api/v1/admin/tenants/{tenantSlug}/entitlements` | tenant entitlement 一覧 |
| `PUT /api/v1/admin/tenants/{tenantSlug}/entitlements` | tenant entitlement 更新 |
| `GET /api/v1/customer-signals` | `q` / filter / cursor / limit 対応 list |
| `GET /api/v1/customer-signal-filters` | saved filter 一覧 |
| `POST /api/v1/customer-signal-filters` | saved filter 作成 |
| `PUT /api/v1/customer-signal-filters/{filterPublicId}` | saved filter 更新 |
| `DELETE /api/v1/customer-signal-filters/{filterPublicId}` | saved filter 削除 |
| `POST /api/v1/support/access/start` | support access 開始 |
| `GET /api/v1/support/access/current` | 現在の support access 状態 |
| `POST /api/v1/support/access/end` | support access 終了 |

## Step 2. migration を追加する

### 2-1. up migration を追加する

#### ファイル: `db/migrations/0013_p10_cross_cutting_extensions.up.sql`

まず role と feature catalog を追加します。

```sql
INSERT INTO roles (code)
VALUES ('support_agent')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE feature_definitions (
    code TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    default_enabled BOOLEAN NOT NULL DEFAULT false,
    default_limit JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO feature_definitions (
    code,
    display_name,
    description,
    default_enabled,
    default_limit
) VALUES
    ('webhooks.enabled', 'Outbound webhooks', 'Deliver signed tenant events to external HTTP endpoints.', false, '{}'::jsonb),
    ('customer_signals.import_export', 'Customer Signals import/export', 'Import and export Customer Signals as CSV files.', false, '{}'::jsonb),
    ('customer_signals.saved_filters', 'Customer Signals saved filters', 'Save tenant scoped Customer Signals filter presets.', true, '{}'::jsonb),
    ('support_access.enabled', 'Support access', 'Allow audited support impersonation sessions.', false, '{}'::jsonb)
ON CONFLICT (code) DO UPDATE
SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    default_enabled = EXCLUDED.default_enabled,
    default_limit = EXCLUDED.default_limit,
    updated_at = now();
```

tenant entitlement は feature ごとの override です。`limit_value` は numeric だけでなく object を置けるように JSONB にします。最初は webhook endpoint 上限や import row 上限をここへ置けます。

```sql
CREATE TABLE tenant_entitlements (
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    feature_code TEXT NOT NULL REFERENCES feature_definitions(code) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL,
    limit_value JSONB NOT NULL DEFAULT '{}'::jsonb,
    source TEXT NOT NULL DEFAULT 'manual'
        CHECK (source IN ('default', 'manual', 'billing', 'migration')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, feature_code)
);

CREATE INDEX tenant_entitlements_feature_idx
    ON tenant_entitlements(feature_code, tenant_id);
```

webhook endpoint と delivery log を追加します。

```sql
CREATE TABLE webhook_endpoints (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    event_types JSONB NOT NULL DEFAULT '[]'::jsonb,
    secret_ciphertext BYTEA NOT NULL,
    secret_key_version INTEGER NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX webhook_endpoints_public_id_key ON webhook_endpoints(public_id);

CREATE INDEX webhook_endpoints_tenant_active_idx
    ON webhook_endpoints(tenant_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE webhook_deliveries (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    webhook_endpoint_id BIGINT NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    outbox_event_id BIGINT REFERENCES outbox_events(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'succeeded', 'failed', 'dead')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 8 CHECK (max_attempts > 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    response_status INTEGER,
    response_body_preview TEXT,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX webhook_deliveries_public_id_key ON webhook_deliveries(public_id);

CREATE INDEX webhook_deliveries_endpoint_created_idx
    ON webhook_deliveries(webhook_endpoint_id, created_at DESC);

CREATE INDEX webhook_deliveries_pending_idx
    ON webhook_deliveries(next_attempt_at, id)
    WHERE status IN ('pending', 'failed');
```

Customer Signals import job と saved filter を追加します。CSV export は既存の `tenant_data_exports` を拡張して使います。

```sql
CREATE TABLE customer_signal_import_jobs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    requested_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    source_file_object_id BIGINT REFERENCES file_objects(id) ON DELETE SET NULL,
    error_file_object_id BIGINT REFERENCES file_objects(id) ON DELETE SET NULL,
    outbox_event_id BIGINT REFERENCES outbox_events(id) ON DELETE SET NULL,
    mode TEXT NOT NULL DEFAULT 'validate_only'
        CHECK (mode IN ('validate_only', 'insert')),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    total_rows INTEGER NOT NULL DEFAULT 0 CHECK (total_rows >= 0),
    valid_rows INTEGER NOT NULL DEFAULT 0 CHECK (valid_rows >= 0),
    invalid_rows INTEGER NOT NULL DEFAULT 0 CHECK (invalid_rows >= 0),
    inserted_rows INTEGER NOT NULL DEFAULT 0 CHECK (inserted_rows >= 0),
    error_summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX customer_signal_import_jobs_public_id_key
    ON customer_signal_import_jobs(public_id);

CREATE INDEX customer_signal_import_jobs_tenant_created_idx
    ON customer_signal_import_jobs(tenant_id, created_at DESC);

CREATE TABLE customer_signal_import_error_rows (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    import_job_id BIGINT NOT NULL REFERENCES customer_signal_import_jobs(id) ON DELETE CASCADE,
    row_number INTEGER NOT NULL CHECK (row_number > 0),
    error_code TEXT NOT NULL,
    error_message TEXT NOT NULL,
    raw_row JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX customer_signal_import_error_rows_job_idx
    ON customer_signal_import_error_rows(import_job_id, row_number, id);

CREATE TABLE customer_signal_saved_filters (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    filter_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX customer_signal_saved_filters_public_id_key
    ON customer_signal_saved_filters(public_id);

CREATE UNIQUE INDEX customer_signal_saved_filters_owner_name_key
    ON customer_signal_saved_filters(tenant_id, owner_user_id, lower(name))
    WHERE deleted_at IS NULL;
```

Customer Signals search 用の index を追加します。

```sql
CREATE INDEX customer_signals_search_idx
    ON customer_signals
    USING GIN (
        to_tsvector(
            'simple',
            coalesce(customer_name, '') || ' ' ||
            coalesce(title, '') || ' ' ||
            coalesce(body, '')
        )
    )
    WHERE deleted_at IS NULL;

CREATE INDEX customer_signals_tenant_cursor_idx
    ON customer_signals(tenant_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
```

既存 `tenant_data_exports` を generic export job として使えるように、resource type と filter を足します。

```sql
ALTER TABLE tenant_data_exports
    ADD COLUMN resource_type TEXT NOT NULL DEFAULT 'tenant_data'
        CHECK (resource_type IN ('tenant_data', 'customer_signals')),
    ADD COLUMN filter_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN saved_filter_id BIGINT REFERENCES customer_signal_saved_filters(id) ON DELETE SET NULL;

CREATE INDEX tenant_data_exports_resource_tenant_created_idx
    ON tenant_data_exports(resource_type, tenant_id, created_at DESC)
    WHERE deleted_at IS NULL;
```

support access session を追加します。

```sql
CREATE TABLE support_access_sessions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    support_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    impersonated_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'ended', 'expired')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX support_access_sessions_public_id_key
    ON support_access_sessions(public_id);

CREATE INDEX support_access_sessions_active_support_idx
    ON support_access_sessions(support_user_id, expires_at)
    WHERE status = 'active';

CREATE INDEX support_access_sessions_tenant_created_idx
    ON support_access_sessions(tenant_id, created_at DESC);
```

最後に `tenant_settings.features` から backfill します。JSON value が boolean のものだけを初期移行対象にします。

```sql
INSERT INTO tenant_entitlements (
    tenant_id,
    feature_code,
    enabled,
    limit_value,
    source
)
SELECT
    ts.tenant_id,
    fd.code,
    (ts.features ->> fd.code)::boolean,
    '{}'::jsonb,
    'migration'
FROM tenant_settings ts
JOIN feature_definitions fd
  ON ts.features ? fd.code
WHERE jsonb_typeof(ts.features -> fd.code) = 'boolean'
ON CONFLICT (tenant_id, feature_code) DO NOTHING;
```

### 2-2. down migration を追加する

#### ファイル: `db/migrations/0013_p10_cross_cutting_extensions.down.sql`

```sql
DROP TABLE IF EXISTS support_access_sessions;

ALTER TABLE tenant_data_exports
    DROP COLUMN IF EXISTS saved_filter_id,
    DROP COLUMN IF EXISTS filter_json,
    DROP COLUMN IF EXISTS resource_type;

DROP TABLE IF EXISTS customer_signal_saved_filters;
DROP TABLE IF EXISTS customer_signal_import_error_rows;
DROP TABLE IF EXISTS customer_signal_import_jobs;
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_endpoints;
DROP TABLE IF EXISTS tenant_entitlements;
DROP TABLE IF EXISTS feature_definitions;

DELETE FROM roles
WHERE code = 'support_agent'
  AND NOT EXISTS (
      SELECT 1
      FROM user_roles ur
      JOIN roles r ON r.id = ur.role_id
      WHERE r.code = 'support_agent'
  );
```

index は table / column drop と一緒に消えます。

### 2-3. schema snapshot を更新する

```bash
make db-up
make db-schema
```

`db/schema.sql` は生成物ですが、この repository では tracked artifact です。migration 追加後に差分が出るのが正しい状態です。

## Step 3. sqlc query を追加する

### 3-1. webhook query

#### ファイル: `db/queries/webhooks.sql`

追加する query は次です。

```sql
-- name: ListWebhookEndpointsByTenantID :many
SELECT *
FROM webhook_endpoints
WHERE tenant_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: GetWebhookEndpointByPublicIDForTenant :one
SELECT *
FROM webhook_endpoints
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
LIMIT 1;

-- name: CreateWebhookEndpoint :one
INSERT INTO webhook_endpoints (
    tenant_id,
    created_by_user_id,
    name,
    url,
    description,
    event_types,
    secret_ciphertext,
    secret_key_version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: UpdateWebhookEndpointByPublicIDForTenant :one
UPDATE webhook_endpoints
SET
    name = $3,
    url = $4,
    description = $5,
    event_types = $6,
    active = $7,
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: RotateWebhookEndpointSecret :one
UPDATE webhook_endpoints
SET
    secret_ciphertext = $3,
    secret_key_version = $4,
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteWebhookEndpointByPublicIDForTenant :execrows
UPDATE webhook_endpoints
SET
    deleted_at = now(),
    active = false,
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL;
```

delivery query は、作成、一覧、claim、成功、retry、dead、manual retry を分けます。

```sql
-- name: CreateWebhookDelivery :one
INSERT INTO webhook_deliveries (
    tenant_id,
    webhook_endpoint_id,
    event_type,
    payload,
    max_attempts
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: ListWebhookDeliveriesByEndpointID :many
SELECT *
FROM webhook_deliveries
WHERE webhook_endpoint_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: MarkWebhookDeliveryProcessing :one
UPDATE webhook_deliveries
SET
    status = 'processing',
    attempts = attempts + 1,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkWebhookDeliverySucceeded :one
UPDATE webhook_deliveries
SET
    status = 'succeeded',
    response_status = $2,
    response_body_preview = left($3, 2000),
    last_error = NULL,
    delivered_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkWebhookDeliveryRetry :one
UPDATE webhook_deliveries
SET
    status = 'failed',
    response_status = $2,
    response_body_preview = left($3, 2000),
    last_error = left($4, 1000),
    next_attempt_at = now() + $5,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkWebhookDeliveryDead :one
UPDATE webhook_deliveries
SET
    status = 'dead',
    response_status = $2,
    response_body_preview = left($3, 2000),
    last_error = left($4, 1000),
    updated_at = now()
WHERE id = $1
RETURNING *;
```

### 3-2. import / export query

#### ファイル: `db/queries/customer_signal_imports.sql`

```sql
-- name: CreateCustomerSignalImportJob :one
INSERT INTO customer_signal_import_jobs (
    tenant_id,
    requested_by_user_id,
    source_file_object_id,
    mode
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: ListCustomerSignalImportJobsByTenantID :many
SELECT *
FROM customer_signal_import_jobs
WHERE tenant_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: GetCustomerSignalImportJobByPublicIDForTenant :one
SELECT *
FROM customer_signal_import_jobs
WHERE public_id = $1
  AND tenant_id = $2
LIMIT 1;

-- name: MarkCustomerSignalImportJobProcessing :one
UPDATE customer_signal_import_jobs
SET
    status = 'processing',
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CompleteCustomerSignalImportJob :one
UPDATE customer_signal_import_jobs
SET
    status = 'completed',
    total_rows = $2,
    valid_rows = $3,
    invalid_rows = $4,
    inserted_rows = $5,
    error_file_object_id = $6,
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: FailCustomerSignalImportJob :one
UPDATE customer_signal_import_jobs
SET
    status = 'failed',
    error_summary = left($2, 1000),
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateCustomerSignalImportErrorRow :one
INSERT INTO customer_signal_import_error_rows (
    import_job_id,
    row_number,
    error_code,
    error_message,
    raw_row
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;
```

#### ファイル: `db/queries/tenant_data_exports.sql`

既存 query に `resource_type`、`filter_json`、`saved_filter_id` を含めます。

```sql
-- name: ListTenantDataExportsByTenantAndResource :many
SELECT *
FROM tenant_data_exports
WHERE tenant_id = $1
  AND resource_type = $2
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT $3;

-- name: CreateTenantDataExportForResource :one
INSERT INTO tenant_data_exports (
    tenant_id,
    requested_by_user_id,
    format,
    resource_type,
    filter_json,
    saved_filter_id,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;
```

### 3-3. Customer Signals search query

#### ファイル: `db/queries/customer_signals.sql`

既存 `ListCustomerSignalsByTenantID` は残し、P10 用に page query を追加します。既存 API が internal call している場合の互換性を壊さないためです。

```sql
-- name: ListCustomerSignalsPage :many
SELECT
    id,
    public_id,
    tenant_id,
    created_by_user_id,
    customer_name,
    title,
    body,
    source,
    priority,
    status,
    created_at,
    updated_at,
    deleted_at
FROM customer_signals
WHERE tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
  AND (
      sqlc.narg(q)::text IS NULL
      OR to_tsvector(
          'simple',
          coalesce(customer_name, '') || ' ' ||
          coalesce(title, '') || ' ' ||
          coalesce(body, '')
      ) @@ websearch_to_tsquery('simple', sqlc.narg(q)::text)
  )
  AND (
      sqlc.narg(status)::text IS NULL
      OR status = sqlc.narg(status)::text
  )
  AND (
      sqlc.narg(priority)::text IS NULL
      OR priority = sqlc.narg(priority)::text
  )
  AND (
      sqlc.narg(source)::text IS NULL
      OR source = sqlc.narg(source)::text
  )
  AND (
      sqlc.narg(cursor_created_at)::timestamptz IS NULL
      OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::bigint)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit);
```

service 側では `limit + 1` を query に渡し、1 件余った場合だけ `nextCursor` を返します。

### 3-4. saved filters / support access / entitlements query

#### ファイル: `db/queries/customer_signal_saved_filters.sql`

```sql
-- name: ListCustomerSignalSavedFilters :many
SELECT *
FROM customer_signal_saved_filters
WHERE tenant_id = $1
  AND owner_user_id = $2
  AND deleted_at IS NULL
ORDER BY lower(name), id;

-- name: CreateCustomerSignalSavedFilter :one
INSERT INTO customer_signal_saved_filters (
    tenant_id,
    owner_user_id,
    name,
    filter_json
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: UpdateCustomerSignalSavedFilter :one
UPDATE customer_signal_saved_filters
SET
    name = $3,
    filter_json = $4,
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND owner_user_id = sqlc.arg(owner_user_id)
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteCustomerSignalSavedFilter :execrows
UPDATE customer_signal_saved_filters
SET
    deleted_at = now(),
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND owner_user_id = sqlc.arg(owner_user_id)
  AND deleted_at IS NULL;
```

#### ファイル: `db/queries/support_access.sql`

```sql
-- name: CreateSupportAccessSession :one
INSERT INTO support_access_sessions (
    support_user_id,
    tenant_id,
    impersonated_user_id,
    reason,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetActiveSupportAccessSessionByPublicID :one
SELECT *
FROM support_access_sessions
WHERE public_id = $1
  AND status = 'active'
  AND expires_at > now()
LIMIT 1;

-- name: EndSupportAccessSession :one
UPDATE support_access_sessions
SET
    status = 'ended',
    ended_at = now(),
    updated_at = now()
WHERE public_id = $1
  AND support_user_id = $2
  AND status = 'active'
RETURNING *;

-- name: ExpireSupportAccessSessions :execrows
UPDATE support_access_sessions
SET
    status = 'expired',
    ended_at = now(),
    updated_at = now()
WHERE status = 'active'
  AND expires_at <= now();
```

#### ファイル: `db/queries/entitlements.sql`

```sql
-- name: ListTenantEntitlements :many
SELECT
    fd.code,
    fd.display_name,
    fd.description,
    fd.default_enabled,
    fd.default_limit,
    te.enabled,
    te.limit_value,
    te.source,
    te.updated_at
FROM feature_definitions fd
LEFT JOIN tenant_entitlements te
  ON te.feature_code = fd.code
 AND te.tenant_id = $1
ORDER BY fd.code;

-- name: UpsertTenantEntitlement :one
INSERT INTO tenant_entitlements (
    tenant_id,
    feature_code,
    enabled,
    limit_value,
    source
) VALUES (
    $1, $2, $3, $4, 'manual'
)
ON CONFLICT (tenant_id, feature_code) DO UPDATE
SET
    enabled = EXCLUDED.enabled,
    limit_value = EXCLUDED.limit_value,
    source = EXCLUDED.source,
    updated_at = now()
RETURNING *;
```

### 3-5. sqlc を実行する

```bash
make sqlc
```

生成された `backend/internal/db/*` は手で編集しません。

## Step 4. outbound webhook を実装する

### 4-1. config を追加する

#### ファイル: `.env.example`

```dotenv
WEBHOOK_SECRET_ENCRYPTION_KEY=
WEBHOOK_SECRET_KEY_VERSION=1
WEBHOOK_DELIVERY_TIMEOUT=5s
WEBHOOK_MAX_ATTEMPTS=8
WEBHOOK_RESPONSE_PREVIEW_BYTES=2000
WEBHOOK_ALLOWED_URL_SCHEMES=https
WEBHOOK_ALLOW_LOCALHOST=false
```

local smoke だけは `WEBHOOK_ALLOW_LOCALHOST=true` を上書きします。本番では localhost、private IP、link-local、metadata service IP への delivery を拒否します。

#### ファイル: `backend/internal/config/config.go`

`Config` に次を追加します。

```go
WebhookSecretEncryptionKey string
WebhookSecretKeyVersion    int
WebhookDeliveryTimeout     time.Duration
WebhookMaxAttempts         int32
WebhookResponsePreviewBytes int
WebhookAllowedURLSchemes   []string
WebhookAllowLocalhost      bool
```

duration は既存の positive duration helper を使います。`WebhookMaxAttempts` は 1 以上、preview bytes は 0 以上にします。

### 4-2. secret encryption helper を汎用化する

#### ファイル: `backend/internal/auth/secret_box.go`

既存の `RefreshTokenStore` と同じ AES-GCM 実装を汎用 helper に切り出します。

```go
type SecretBox struct {
    aead       cipher.AEAD
    keyVersion int32
}

func NewSecretBox(encodedKey string, keyVersion int) (*SecretBox, error)
func (s *SecretBox) Encrypt(plaintext string) ([]byte, int32, error)
func (s *SecretBox) Decrypt(ciphertext []byte, keyVersion int32) (string, error)
```

`RefreshTokenStore` は内部で `SecretBox` を使うようにして、既存 test を壊さないようにします。error 名は既存の token encryption error を再利用して構いません。

### 4-3. service を追加する

#### ファイル: `backend/internal/service/webhook_service.go`

`WebhookService` は次を担当します。

- endpoint CRUD
- secret 生成 / 暗号化 / rotate
- URL validation
- event type validation
- tenant entitlement check
- delivery record 作成
- outbox enqueue
- delivery 実行

public method は次にします。

```go
type WebhookService struct {
    pool        *pgxpool.Pool
    queries     *db.Queries
    secretBox   *auth.SecretBox
    entitlements *EntitlementService
    audit       AuditRecorder
    httpClient  *http.Client
    cfg         WebhookConfig
}

func (s *WebhookService) ListEndpoints(ctx context.Context, tenantID int64) ([]WebhookEndpoint, error)
func (s *WebhookService) CreateEndpoint(ctx context.Context, tenantID, actorUserID int64, input WebhookEndpointInput, auditCtx AuditContext) (WebhookEndpointWithSecret, error)
func (s *WebhookService) UpdateEndpoint(ctx context.Context, tenantID int64, publicID string, input WebhookEndpointInput, auditCtx AuditContext) (WebhookEndpoint, error)
func (s *WebhookService) RotateSecret(ctx context.Context, tenantID int64, publicID string, auditCtx AuditContext) (WebhookEndpointWithSecret, error)
func (s *WebhookService) DeleteEndpoint(ctx context.Context, tenantID int64, publicID string, auditCtx AuditContext) error
func (s *WebhookService) EnqueueTenantEvent(ctx context.Context, qtx *db.Queries, tenantID int64, eventType string, payload any) error
func (s *WebhookService) Deliver(ctx context.Context, deliveryID int64) error
func (s *WebhookService) RetryDelivery(ctx context.Context, tenantID int64, deliveryPublicID string, auditCtx AuditContext) error
```

secret は作成時と rotate 時だけ response に出します。list / detail response には `secretPreview` を出す場合でも末尾 4 文字程度にし、DB に preview 用 plaintext は保存しません。

### 4-4. signature を実装する

webhook HTTP request は次にします。

```http
POST <endpoint url>
Content-Type: application/json
User-Agent: HaoHao-Webhooks/1
X-HaoHao-Event-ID: <webhook_deliveries.public_id>
X-HaoHao-Event-Type: customer_signal.created
X-HaoHao-Signature: t=1777046400,v1=<hex hmac sha256>
```

署名対象文字列は次です。

```text
<unix timestamp>.<raw request body>
```

HMAC key は復号した webhook secret です。

#### ファイル: `backend/internal/service/webhook_signature.go`

```go
func SignWebhookPayload(secret string, now time.Time, body []byte) string
func VerifyWebhookSignature(secret string, header string, body []byte, tolerance time.Duration) bool
```

`VerifyWebhookSignature` は外部 receiver の test 用にも使えます。HaoHao 自体が inbound webhook を受けるわけではありませんが、unit test で署名仕様を固定できます。

### 4-5. outbox handler に接続する

#### ファイル: `backend/internal/service/outbox_handler.go`

`DefaultOutboxHandler` に `webhooks *WebhookService` を追加し、次を扱います。

```go
case "webhook.delivery_requested":
    var payload struct {
        DeliveryID int64 `json:"deliveryId"`
    }
    if err := json.Unmarshal(event.Payload, &payload); err != nil {
        return err
    }
    if h.webhooks == nil {
        return nil
    }
    return h.webhooks.Deliver(ctx, payload.DeliveryID)
```

Customer Signal の create / update / delete service では、audit と同じ transaction 内で webhook delivery を enqueue します。

```go
_ = s.webhooks.EnqueueTenantEvent(ctx, qtx, tenantID, "customer_signal.created", map[string]any{
    "customerSignal": item,
})
```

webhook enqueue が失敗した場合は mutation 全体を rollback します。外部通知が契約になっている tenant では、DB だけ成功して webhook delivery が欠落する方が危険だからです。

### 4-6. API を追加する

#### ファイル: `backend/internal/api/webhooks.go`

すべて browser Cookie + CSRF + tenant admin + entitlement gate にします。

```go
func registerWebhookRoutes(api huma.API, deps Dependencies)
```

operation tag は `webhooks` にします。request / response body は secret を含む create / rotate response と、secret を含まない通常 response を分けます。

主な HTTP error は次に固定します。

| 状態 | response |
| --- | --- |
| 未ログイン | `401` |
| active tenant なし | `409` |
| `tenant_admin` なし | `403` |
| entitlement disabled | `403` |
| invalid URL / event type | `400` |
| endpoint not found | `404` |

### 4-7. metrics を追加する

#### ファイル: `backend/internal/platform/metrics.go`

追加する metrics は次です。

| metric | label | 用途 |
| --- | --- | --- |
| `haohao_webhook_deliveries_total` | `result` | delivery 成功 / retry / dead |
| `haohao_webhook_delivery_duration_seconds` | `result` | delivery latency |
| `haohao_webhook_endpoints_total` | なし | active endpoint 数 gauge |

label に tenant slug、URL、event id、error message は入れません。

## Step 5. Customer Signals CSV import / export job を実装する

### 5-1. CSV format を固定する

Customer Signals import / export の CSV header は次に固定します。

```csv
customer_name,title,body,source,priority,status
```

validation は既存 `CustomerSignalService` の normalize / allowed values を共有します。

| column | 必須 | ルール |
| --- | --- | --- |
| `customer_name` | yes | 1 から 120 文字 |
| `title` | yes | 1 から 200 文字 |
| `body` | no | 4000 文字以内 |
| `source` | no | `support` / `sales` / `customer_success` / `research` / `internal` / `other` |
| `priority` | no | `low` / `medium` / `high` / `urgent` |
| `status` | no | `new` / `triaged` / `planned` / `closed` |

空の optional value は service default に寄せます。

### 5-2. import service を追加する

#### ファイル: `backend/internal/service/customer_signal_import_service.go`

`CustomerSignalImportService` は次を担当します。

- upload 済み `file_objects` を source として job を作る
- `customer_signal_import.requested` outbox event を enqueue する
- CSV を読み、header と row を validation する
- `mode=validate_only` の場合は DB insert しない
- `mode=insert` の場合、validation error が 0 件のときだけ insert する
- error row CSV を `file_objects` に保存する
- audit と metrics を残す

import job 作成時は、既存 file upload API を使って CSV file を先に upload します。`file_objects.purpose` は既存 enum に `import` があるため、それを使います。

### 5-3. export service を拡張する

#### ファイル: `backend/internal/service/tenant_data_export_service.go`

既存 `TenantDataExportService` を拡張し、`resourceType=customer_signals` と `format=csv` を扱えるようにします。

追加する input は次です。

```go
type TenantDataExportInput struct {
    Format       string
    ResourceType string
    Filter       CustomerSignalFilter
    SavedFilterID *int64
}
```

既存互換のため、`ResourceType` が空なら `tenant_data` として扱います。

Customer Signals CSV export は次を出力します。

```csv
public_id,customer_name,title,body,source,priority,status,created_at,updated_at
```

file body は既存 file storage driver に保存し、`tenant_data_exports.file_object_id` に紐付けます。

### 5-4. API を追加する

#### ファイル: `backend/internal/api/customer_signal_imports.go`

```text
GET  /api/v1/admin/tenants/{tenantSlug}/imports
POST /api/v1/admin/tenants/{tenantSlug}/imports
GET  /api/v1/admin/tenants/{tenantSlug}/imports/{importPublicId}
```

`POST` body は次です。

```json
{
  "sourceFilePublicId": "018f2f05-c6c9-7a49-b32d-04f4dd84ef4a",
  "mode": "validate_only"
}
```

#### ファイル: `backend/internal/api/tenant_data_exports.go`

既存 exports API を壊さず、`resourceType` と `filter` を追加します。

```json
{
  "format": "csv",
  "resourceType": "customer_signals",
  "filter": {
    "q": "enterprise",
    "status": "planned",
    "priority": "high"
  },
  "savedFilterPublicId": "018f2f05-c6c9-7a49-b32d-04f4dd84ef4a"
}
```

`resourceType` 省略時は既存の tenant data export と同じ挙動にします。

### 5-5. import / export metrics を追加する

| metric | label | 用途 |
| --- | --- | --- |
| `haohao_customer_signal_import_jobs_total` | `result` | completed / failed |
| `haohao_customer_signal_import_rows_total` | `result` | valid / invalid / inserted |
| `haohao_tenant_export_jobs_total` | `resource`, `result` | export result |

`resource` は `tenant_data` / `customer_signals` のような低 cardinality 値だけにします。

## Step 6. Customer Signals search / cursor pagination / saved filters を実装する

### 6-1. API response を拡張する

#### ファイル: `backend/internal/api/customer_signals.go`

`GET /api/v1/customer-signals` input に query parameter を追加します。

```go
type ListCustomerSignalsInput struct {
    SessionCookie http.Cookie `cookie:"SESSION_ID"`
    Q             string      `query:"q"`
    Status        string      `query:"status"`
    Priority      string      `query:"priority"`
    Source        string      `query:"source"`
    Cursor        string      `query:"cursor"`
    Limit         int         `query:"limit"`
}
```

response は `nextCursor` を追加します。

```go
type CustomerSignalListBody struct {
    Items      []CustomerSignalBody `json:"items"`
    NextCursor string               `json:"nextCursor,omitempty"`
}
```

`limit` は default `25`、maximum `100` にします。無効な cursor は `400` です。

### 6-2. cursor helper を追加する

#### ファイル: `backend/internal/service/cursor.go`

```go
type CreatedAtIDCursor struct {
    CreatedAt time.Time `json:"createdAt"`
    ID        int64     `json:"id"`
}

func EncodeCreatedAtIDCursor(cursor CreatedAtIDCursor) (string, error)
func DecodeCreatedAtIDCursor(raw string) (CreatedAtIDCursor, error)
```

encoding は `base64.RawURLEncoding` を使います。cursor は UI と log へそのまま出る可能性があるため、内部 ID 以外の tenant / user 情報は入れません。

### 6-3. saved filters service / API を追加する

#### ファイル: `backend/internal/service/customer_signal_saved_filter_service.go`

saved filter は active tenant と owner user で分離します。他 user の filter は見えません。

`filter_json` には次だけ保存します。

```json
{
  "q": "enterprise",
  "status": "planned",
  "priority": "high",
  "source": "support"
}
```

未知 key は service validation で拒否します。JSON をそのまま SQL fragment に変換しません。

#### ファイル: `backend/internal/api/customer_signal_saved_filters.go`

```text
GET    /api/v1/customer-signal-filters
POST   /api/v1/customer-signal-filters
PUT    /api/v1/customer-signal-filters/{filterPublicId}
DELETE /api/v1/customer-signal-filters/{filterPublicId}
```

saved filters は `customer_signal_user` tenant role を要求します。mutating endpoint は CSRF を要求します。

## Step 7. support access / impersonation を実装する

### 7-1. config を追加する

#### ファイル: `.env.example`

```dotenv
SUPPORT_ACCESS_MAX_DURATION=30m
SUPPORT_ACCESS_DEFAULT_DURATION=15m
```

default duration は max duration 以下でなければ起動時 error にします。

### 7-2. auth context を拡張する

#### ファイル: `backend/internal/service/session_service.go`

current session に optional support access context を追加します。

```go
type SupportAccessContext struct {
    PublicID string
    TenantID int64
    TenantSlug string
    SupportUserID int64
    SupportUserEmail string
    ImpersonatedUserID int64
    ImpersonatedUserEmail string
    Reason string
    ExpiresAt time.Time
}
```

browser session store には `SupportAccessPublicID` を保存します。session load 時に DB の `support_access_sessions` を確認し、expired なら session から外します。

### 7-3. effective user 方針

support access 中の認可は impersonated user の tenant membership を使います。audit は support user と impersonated user の両方を残します。

`AuditContext` に次を追加します。

```go
SupportAccessID *string
ImpersonatedUserID *int64
```

audit event の actor は support user にします。metadata に `impersonatedUserId`、`supportAccessId`、`reason` を入れます。reason は user 入力ですが、audit 用に 500 文字で切ります。

### 7-4. service を追加する

#### ファイル: `backend/internal/service/support_access_service.go`

`SupportAccessService` は次を担当します。

- global role `support_agent` の確認
- tenant / impersonated user の存在確認
- reason min length `10`
- duration upper bound
- active session 作成
- current session 取得
- explicit end
- expired session cleanup
- audit record

support access では次の操作を禁止します。

- support access API 自身を impersonated context で呼ぶこと
- machine client secret 作成 / rotate
- webhook secret rotate / delete
- entitlement update

これらは安全側に倒すためです。必要なら後続で allowlist を広げます。

### 7-5. API を追加する

#### ファイル: `backend/internal/api/support_access.go`

```text
POST /api/v1/support/access/start
GET  /api/v1/support/access/current
POST /api/v1/support/access/end
```

start body は次です。

```json
{
  "tenantSlug": "acme",
  "impersonatedUserEmail": "demo@example.com",
  "reason": "Investigating support ticket HD-1234",
  "durationMinutes": 15
}
```

start / end は CSRF を要求します。current は authenticated session だけで返せます。

### 7-6. frontend banner を追加する

#### ファイル: `frontend/src/components/SupportAccessBanner.vue`

app header の最上部に、support access 中だけ表示します。

表示する内容は次です。

- support access 中であること
- impersonated user email
- tenant slug
- expiry
- end button

banner は閉じられないようにします。support access 中であることを非表示にできる UI は作りません。

## Step 8. feature flags / entitlements を実装する

### 8-1. EntitlementService を追加する

#### ファイル: `backend/internal/service/entitlement_service.go`

```go
type EntitlementService struct {
    queries *db.Queries
    audit   AuditRecorder
}

func (s *EntitlementService) List(ctx context.Context, tenantID int64) ([]TenantEntitlement, error)
func (s *EntitlementService) Update(ctx context.Context, tenantID int64, input []TenantEntitlementInput, auditCtx AuditContext) ([]TenantEntitlement, error)
func (s *EntitlementService) IsEnabled(ctx context.Context, tenantID int64, featureCode string) (bool, error)
func (s *EntitlementService) Limit(ctx context.Context, tenantID int64, featureCode string) (map[string]any, error)
```

`IsEnabled` は row が無い場合に `feature_definitions.default_enabled` を返します。

### 8-2. tenant settings との互換

#### ファイル: `backend/internal/service/tenant_settings_service.go`

`tenant_settings.features` は response には残します。ただし、新 UI は entitlements API を使います。

互換方針は次です。

- `TenantSettingsService.Get` は既存 field を返す
- `TenantSettingsService.Update` は `features` の更新を受け付けるが deprecated として扱う
- P10 UI では `features` を編集しない
- runtime feature gate は `EntitlementService` を正本にする

### 8-3. API を追加する

#### ファイル: `backend/internal/api/entitlements.go`

```text
GET /api/v1/admin/tenants/{tenantSlug}/entitlements
PUT /api/v1/admin/tenants/{tenantSlug}/entitlements
```

`tenant_admin` と CSRF を要求します。

PUT body は次です。

```json
{
  "items": [
    {
      "featureCode": "webhooks.enabled",
      "enabled": true,
      "limitValue": {
        "maxEndpoints": 5
      }
    }
  ]
}
```

unknown feature code は `400` にします。

### 8-4. gate を接続する

次の service / API で entitlement を確認します。

| feature code | gate 対象 |
| --- | --- |
| `webhooks.enabled` | webhook endpoint CRUD、delivery retry、business event enqueue |
| `customer_signals.import_export` | import job create、customer_signals CSV export create |
| `customer_signals.saved_filters` | saved filter CRUD |
| `support_access.enabled` | support access start |

disabled の場合は `403` を返します。response message は `"feature is not enabled for this tenant"` に揃えます。

## Step 9. Huma / OpenAPI / frontend を接続する

### 9-1. Dependencies を拡張する

#### ファイル: `backend/internal/api/register.go`

`Dependencies` に次を追加します。

```go
WebhookService *service.WebhookService
CustomerSignalImportService *service.CustomerSignalImportService
CustomerSignalSavedFilterService *service.CustomerSignalSavedFilterService
SupportAccessService *service.SupportAccessService
EntitlementService *service.EntitlementService
```

`RegisterRoutes` で次を呼びます。

```go
registerWebhookRoutes(api, deps)
registerCustomerSignalImportRoutes(api, deps)
registerCustomerSignalSavedFilterRoutes(api, deps)
registerSupportAccessRoutes(api, deps)
registerEntitlementRoutes(api, deps)
```

OpenAPI export command では nil service でも schema が出るよう、handler 内で nil check して `503` にします。

### 9-2. runtime wiring を追加する

#### ファイル: `backend/cmd/main/main.go`

起動時に次を構成します。

- `auth.SecretBox` for webhook secret
- `EntitlementService`
- `WebhookService`
- `CustomerSignalImportService`
- `CustomerSignalSavedFilterService`
- `SupportAccessService`
- outbox handler への webhook / import / export handler 接続

`WEBHOOK_SECRET_ENCRYPTION_KEY` が空でも、webhook feature が全 tenant で disabled なら起動は許可します。ただし webhook endpoint 作成時は `503` ではなく `500` にせず、`400` または `503` で `"webhook secret encryption key is not configured"` を返します。

production では起動時に key 必須に寄せる運用でも構いません。その場合は runbook に明記します。

### 9-3. OpenAPI surface を更新する

#### ファイル: `backend/internal/api/surface.go`

P8 の browser surface に P10 API を含めます。

`external` surface には含めません。

確認 grep は次です。

```bash
make gen
rg "/api/v1/admin/tenants/\\{tenantSlug\\}/webhooks" openapi/browser.yaml
rg "/api/v1/support/access/start" openapi/browser.yaml
! rg "/api/v1/support/access/start" openapi/external.yaml
```

### 9-4. frontend API wrapper を追加する

generated SDK を view から直接呼ばないよう、既存 wrapper と同じ層に追加します。

追加する file は次です。

- `frontend/src/api/webhooks.ts`
- `frontend/src/api/customer-signal-imports.ts`
- `frontend/src/api/tenant-data-exports.ts` の拡張
- `frontend/src/api/customer-signal-filters.ts`
- `frontend/src/api/support-access.ts`
- `frontend/src/api/entitlements.ts`

mutation は `readCookie('XSRF-TOKEN')` と共通 transport の idempotency support を使います。

### 9-5. frontend stores / views を追加する

追加 / 更新する file は次です。

- `frontend/src/stores/webhooks.ts`
- `frontend/src/stores/customer-signal-imports.ts`
- `frontend/src/stores/customer-signal-filters.ts`
- `frontend/src/stores/support-access.ts`
- `frontend/src/stores/entitlements.ts`
- `frontend/src/components/SupportAccessBanner.vue`
- `frontend/src/views/TenantAdminTenantDetailView.vue`
- `frontend/src/views/CustomerSignalsView.vue`
- `frontend/src/App.vue`

Tenant Admin detail に追加する section は次です。

- Entitlements
- Webhooks
- Imports
- Exports

Customer Signals list に追加する controls は次です。

- search input
- status / priority / source filter
- saved filter menu
- next page button

UI は P9 の selector 方針に合わせ、曖昧な箇所だけ `data-testid` を追加します。

## Step 10. smoke / Playwright / CI を追加する

### 10-1. smoke script を追加する

#### ファイル: `scripts/smoke-p10.sh`

この script は既存 server に対して確認します。server は起動しません。

確認する流れは次です。

1. local login
2. active tenant を `acme` にする
3. entitlements で `webhooks.enabled` と `customer_signals.import_export` を有効化する
4. test webhook receiver を localhost で起動する
5. webhook endpoint を作成する
6. Customer Signal を作成し、delivery が `succeeded` になることを確認する
7. CSV file を upload する
8. import job を `validate_only` で作成し、`completed` になることを確認する
9. Customer Signals list を `q` と `limit` 付きで呼び、`nextCursor` が返ることを確認する
10. saved filter を作成 / 更新 / 削除する
11. Customer Signals CSV export を request し、ready または processing になることを確認する
12. `/metrics` に P10 metrics が出ることを確認する

webhook receiver は test 用に次のどちらかで実装します。

- `scripts/webhook-receiver.go` を追加して `go run` する
- `scripts/smoke-p10.sh` 内で `python3 -m http.server` ではなく、小さな HTTP receiver を使う

signature header の存在確認だけでなく、receiver 側で `X-HaoHao-Signature` を記録します。HMAC 検証は Go unit test に寄せます。

#### ファイル: `Makefile`

```make
smoke-p10:
	bash scripts/smoke-p10.sh
```

### 10-2. Go test を追加する

追加する test は次です。

| file | 確認 |
| --- | --- |
| `backend/internal/service/webhook_signature_test.go` | signature 生成 / 検証 / timestamp tolerance |
| `backend/internal/service/webhook_service_test.go` | URL validation、event type validation、secret が response に残らない |
| `backend/internal/service/customer_signal_import_service_test.go` | CSV validation、error row、insert mode |
| `backend/internal/service/cursor_test.go` | cursor encode / decode、invalid cursor |
| `backend/internal/service/support_access_service_test.go` | max duration、reason required、expiry |
| `backend/internal/service/entitlement_service_test.go` | default、override、unknown feature |
| `backend/internal/app/openapi_test.go` | browser / external surface の混入防止 |

### 10-3. Playwright E2E を更新する

#### ファイル: `e2e/browser-journey.spec.ts`

既存 journey に次を足します。

- Tenant Admin detail で entitlements を有効化する
- webhook endpoint を作成する
- Customer Signal を作成する
- webhook delivery log が表示される
- CSV import job を作成し、status が表示される
- saved filter を作成し、filter を適用する
- Customer Signals export を request し、status が表示される

#### ファイル: `e2e/access-and-fallback.spec.ts`

support access と entitlement gate を確認します。

- entitlement disabled の tenant では webhook section が disabled state になる
- `support_agent` user で support access を開始すると banner が出る
- support access end で banner が消える
- role 不足 user では support access start ができない

E2E 用 user は `scripts/seed-e2e-users.sql` に追加します。

| user | 目的 |
| --- | --- |
| `support@example.com` | global role `support_agent` を持つ |
| `limited@example.com` | entitlement / admin access denied 確認 |

### 10-4. CI を更新する

#### ファイル: `.github/workflows/ci.yml`

CI の generated drift check に次を含めます。

- `db/schema.sql`
- `backend/internal/db/*`
- `openapi/openapi.yaml`
- `openapi/browser.yaml`
- `openapi/external.yaml`
- `frontend/src/api/generated/*`

CI で追加する確認は次です。

```bash
make gen
git diff --exit-code -- db/schema.sql backend/internal/db openapi frontend/src/api/generated
go test ./backend/...
npm --prefix frontend run build
make binary
make smoke-p10
make e2e
```

`make smoke-p10` と `make e2e` は PostgreSQL / Redis / local single binary が必要です。P9 の CI pattern を流用します。

## 生成と確認

### DB / generated artifacts

```bash
make db-up
make db-schema
make sqlc
make gen
```

### backend test

```bash
go test ./backend/...
```

### frontend build

```bash
npm --prefix frontend run build
```

### single binary

```bash
make binary
```

### smoke

single binary を起動してから smoke を実行します。

```bash
AUTH_MODE=local \
ENABLE_LOCAL_PASSWORD_LOGIN=true \
HTTP_PORT=18082 \
APP_BASE_URL=http://127.0.0.1:18082 \
FRONTEND_BASE_URL=http://127.0.0.1:18082 \
WEBHOOK_ALLOW_LOCALHOST=true \
OUTBOX_WORKER_ENABLED=true \
./bin/haohao
```

別 terminal で実行します。

```bash
BASE_URL=http://127.0.0.1:18082 make smoke-common-services
BASE_URL=http://127.0.0.1:18082 make smoke-p10
BASE_URL=http://127.0.0.1:18082 make e2e
```

## 失敗時の見方

### webhook delivery が成功しない

まず outbox と delivery log を分けて確認します。

```sql
SELECT event_type, status, attempts, last_error
FROM outbox_events
WHERE event_type = 'webhook.delivery_requested'
ORDER BY created_at DESC
LIMIT 10;

SELECT status, attempts, response_status, last_error
FROM webhook_deliveries
ORDER BY created_at DESC
LIMIT 10;
```

outbox が `sent` で delivery が `failed` の場合は HTTP receiver 側の status / timeout を見ます。outbox が `dead` の場合は handler が delivery 実行前に error になっています。

### import job が processing のまま残る

`OUTBOX_WORKER_ENABLED=true` で起動しているか確認します。

```sql
SELECT event_type, status, attempts, last_error
FROM outbox_events
WHERE event_type = 'customer_signal_import.requested'
ORDER BY created_at DESC
LIMIT 10;
```

CSV validation error は job failure ではありません。validation error がある `validate_only` job は `completed` で、`invalid_rows > 0` になります。

### cursor pagination で重複する

query が `created_at` だけで page を切っていないか確認します。必ず `(created_at, id)` の pair で比較します。

```sql
ORDER BY created_at DESC, id DESC
```

次 page は次です。

```sql
AND (created_at, id) < (:cursor_created_at, :cursor_id)
```

### support access banner が消えない

session store の support access id と DB session status を確認します。

```sql
SELECT public_id, status, expires_at, ended_at
FROM support_access_sessions
ORDER BY created_at DESC
LIMIT 10;
```

DB が `ended` なのに UI が消えない場合は、`GET /api/v1/support/access/current` 後に Pinia store が clear されているかを確認します。

### entitlement が効かない

runtime gate が `tenant_settings.features` をまだ読んでいないか確認します。P10 以降の正本は `tenant_entitlements` です。

```sql
SELECT feature_code, enabled, limit_value, source
FROM tenant_entitlements
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'acme');
```

## P10 完了後の状態

このチュートリアルを最後まで進めると、HaoHao は次の状態になります。

- tenant admin は signed outbound webhook を管理できる
- webhook delivery の失敗は retry / dead-letter / delivery log で追える
- Customer Signals は CSV import / CSV export を非同期 job として扱える
- Customer Signals list は検索、filter、cursor pagination、saved filter を持つ
- support agent は理由と期限付きで tenant user を impersonate でき、UI と audit で明示される
- feature flags / entitlements は `tenant_settings` から独立した正本を持つ
- OpenAPI browser spec、generated SDK、Vue UI、smoke、Playwright E2E が P10 の主要 flow を検知できる

この時点で、HaoHao は多くの B2B SaaS に必要な横断機能の基礎を持ちます。次に billing / pricing plan を入れる場合は、`tenant_entitlements.source = 'billing'` と pricing plan table を接続します。S3 / GCS を入れる場合は、P7 の file storage interface に object storage driver を追加し、P10 の import / export / webhook payload には storage driver 差分を漏らさないようにします。
