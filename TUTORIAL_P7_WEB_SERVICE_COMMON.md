# P7 Web サービス共通機能実装チュートリアル

## この文書の目的

この文書は、`deep-research-report.md` の **P7: Web サービス共通機能** を、現在の HaoHao に実装できる順番へ分解したチュートリアルです。

P3 では audit log、P4 では metrics / tracing / alerting、P5 では tenant 管理 UI、P6 では Customer Signals という業務ドメインを追加しました。P7 では、その上に多くの Web サービスで繰り返し必要になる共通機能を積みます。

このチュートリアルで扱う共通機能は次です。

- security hardening
- background job / outbox
- idempotency key
- email / in-app notification
- user invitation / onboarding
- tenant-aware file upload
- Redis based rate limit
- tenant settings / quota
- data retention / export / deletion
- Playwright E2E
- deployment IaC / secret management / backup restore drill の入口

ただし、P7 初期版ではすべてを大規模に作り込みません。まずは **security hardening と idempotency を土台にし、outbox を中心に、通知、招待、ファイル、rate limit、quota、E2E、data lifecycle を小さく縦に通す** ことを目的にします。

この文書は `TUTORIAL.md` / `TUTORIAL_SINGLE_BINARY.md` / `TUTORIAL_P4_OBSERVABILITY.md` / `TUTORIAL_P6_DOMAIN_EXPANSION.md` と同じように、対象ファイル、主要コード方針、確認コマンド、完了条件まで追える形にしています。

## この文書が前提にしている現在地

このチュートリアルを始める前の repository は、少なくとも次の状態にある前提で進めます。

- Cookie session + CSRF の browser API がある
- active tenant と tenant role による tenant-aware API がある
- `AuditService` が mutation と同じ transaction で audit event を保存できる
- `/metrics`、request log、trace context の土台がある
- SCIM reconcile 用の scheduler pattern が `backend/internal/jobs` にある
- `customer_signals` という tenant-aware 業務 table / API / UI がある
- `frontend/src/api/client.ts` が generated SDK の共通 transport を持っている
- `make gen` が sqlc / OpenAPI / frontend generated SDK をまとめて更新する
- `make smoke-operability`、`make smoke-observability`、`make smoke-tenant-admin`、`make smoke-customer-signals` が既存 server に対して確認できる
- `make binary` で single binary を作れる

この P7 では、決済、請求、大量ファイル、リアルタイム通信、複数 region 高可用性は扱いません。これらは P7 の共通機能を入れた後、具体的なサービス要件に合わせて個別設計します。

## 完成条件

このチュートリアルの完了条件は次です。

- browser response に CSP / HSTS / `X-Content-Type-Options` / `Referrer-Policy` / frame 制御が入る
- request body size limit、trusted proxy、CORS 方針が設定として明示される
- `outbox_events` table が追加される
- worker が `FOR UPDATE SKIP LOCKED` で pending outbox event を non-blocking に claim できる
- outbox event は retry / dead-letter 相当の状態を持つ
- mutation service から同一 transaction 内で outbox event を enqueue できる
- `idempotency_keys` table が追加され、重複 `POST` を安全に再実行できる
- `notifications` table が追加される
- email sender は初期状態では log delivery とし、SMTP は設定で有効化できる
- in-app notification の list / mark read API がある
- `tenant_invitations` table が追加され、招待作成 / revoke / accept ができる
- tenant-aware な `file_objects` table が追加される
- upload / download / soft delete API がある
- file metadata は DB、file body は local filesystem driver に分ける
- file upload は size / content type / tenant role / CSRF を検証する
- Redis based rate limit middleware が login / browser API / external API に入る
- rate limit の metrics が `/metrics` に出る
- `tenant_settings` で file quota / rate limit override / feature enablement の入口を持つ
- soft delete 済み data の purge job と tenant data export job の入口がある
- Playwright E2E が local single binary に対して login、tenant 選択、Customer Signals、file upload、notification を確認できる
- deployment / secret management / backup restore drill の最小 runbook が repository にある
- `make gen`、`go test ./backend/...`、`npm --prefix frontend run build`、`make binary` が通る
- single binary を起動した状態で既存 smoke と P7 smoke / E2E が通る

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | security hardening | security headers、body limit、trusted proxy、CORS 方針を固定する |
| Step 2 | config / `.env.example` | P7 共通機能の設定入口を固定する |
| Step 3 | `db/migrations/0012_web_service_common.*.sql` | outbox / idempotency / notifications / invitations / files / tenant settings / exports schema を追加する |
| Step 4 | `db/queries/*.sql` | sqlc 用 query を追加する |
| Step 5 | `backend/internal/service/outbox_service.go` | transaction 内 enqueue と worker claim / retry を実装する |
| Step 6 | idempotency service / middleware | `POST` 系 API の重複実行を防ぐ |
| Step 7 | `backend/internal/jobs/outbox_worker.go` | outbox worker を scheduler として起動する |
| Step 8 | email / notification service | log email sender と in-app notification を追加する |
| Step 9 | user invitation / onboarding | tenant invitation と accept flow を追加する |
| Step 10 | file service / storage driver | tenant-aware file upload / download を追加する |
| Step 11 | Redis rate limit middleware | login / API / external API の rate limit を追加する |
| Step 12 | tenant settings / quota | file quota、rate limit override、feature enablement の入口を追加する |
| Step 13 | Huma API / wiring / OpenAPI | API と worker を runtime に接続する |
| Step 14 | frontend notification / file / invitation UI | notification center、attachment、invitation、tenant settings UI を追加する |
| Step 15 | smoke / Playwright E2E | API smoke と browser E2E を追加する |
| Step 16 | deployment / secret runbook | 配備先に依存しない運用入口を追加する |
| Step 17 | data lifecycle | retention、purge、tenant data export を追加する |
| Step 18 | backup / restore drill | database / file storage / Redis の restore 確認を固定する |
| Step 19 | P7 後の拡張候補 | webhooks、import/export、search、support access、feature flags を分離する |

## 先に決める方針

### security hardening を最初に入れる

P7 の最初に security header、request body size limit、trusted proxy、CORS 方針を固定します。

理由は、file upload、invitation、email link、external API、rate limit を追加した後に security boundary を直すと、API ごとに挙動が揺れやすいからです。HaoHao は browser app + Cookie session + CSRF を前提にしているため、default deny に近い security header を先に入れます。

### idempotency key は `POST` 系 API の共通作法にする

browser の二重 click、mobile / flaky network、external client の retry、outbox worker の再実行では、同じ mutation が複数回届く前提で設計します。

P7 では `Idempotency-Key` header を導入し、次に適用します。

- file upload
- tenant invitation create / accept
- tenant data export request
- external API の `POST`
- outbox enqueue を伴う mutation

`Idempotency-Key` 自体、request body 全文、response body 全文は metrics label や audit metadata に入れません。DB には request hash と response summary だけを保存します。

### outbox を最初に入れる

P7 の中で最初に入れるのは background job / outbox です。

理由は、email、notification、file processing、external sync などがすべて「HTTP request の transaction 後に非同期で処理したい仕事」だからです。先に outbox を入れておくと、後続機能が request handler に外部送信処理を抱えなくて済みます。

### outbox は PostgreSQL、短期 queue は Redis に寄せる

P7 初期版では次のように使い分けます。

- outbox: DB mutation と同じ transaction で永続化したい job
- Redis rate limit: 短命な counter / TTL が欲しい制御
- scheduler goroutine: process 内 worker

別の queue system はまだ入れません。単一バイナリ配信と local 開発の単純さを保つためです。

### file body と metadata を分ける

file upload は、DB に body を入れません。

- DB: tenant、owner、content type、size、storage key、checksum、soft delete
- local storage: file body
- 将来の S3 / GCS: storage driver の差し替え

P7 初期版は local filesystem driver だけで十分です。ただし interface は `Storage` として切り、S3 に置き換えやすくします。

### email は log delivery から始める

最初から本物の SMTP / SES / SendGrid に接続すると、secret、domain verification、bounce handling が実装の主題になってしまいます。

P7 初期版では `EMAIL_DELIVERY_MODE=log` を default にします。

- local / test: log sender
- staging / production: SMTP sender

email 本文の template engine は最初は `text/template` で十分です。HTML email、unsubscribe、bounce handling は後続で追加します。

### invitation は tenant 管理 UI の自然な続きとして入れる

P5 の tenant 管理 UI は既存 user に role を grant できます。しかし実サービスでは、まだ user として存在しない相手を tenant に招待する導線が必要です。

P7 では `tenant_invitations` を追加し、tenant admin が invite を作成し、招待された user が login 後に accept する流れを作ります。

招待 token は平文保存しません。DB には token hash だけを保存します。email / log / audit metadata に token を出しません。

### rate limit は user / session / IP の順で key を作る

browser API の rate limit key は、認証済みなら user public id、未認証なら IP を使います。

public external API は token subject / client id / IP の順で key を作ります。

metrics label には key の値を入れません。`policy` と `result` 程度に抑えます。

### tenant settings は quota と feature の置き場にする

P7 では billing / pricing までは実装しません。ただし、file upload、rate limit、notification、将来の feature flag は tenant ごとの差分が必ず必要になります。

最初は `tenant_settings` に次を置きます。

- file storage quota
- rate limit override
- notification enablement
- feature enablement JSON

後から billing を入れる場合は、`tenant_settings` を entitlement の cache / override として使い、pricing plan 自体は別 table に分けます。

### data lifecycle は runbook だけで終わらせない

監査ログ、outbox、notification、file metadata、soft deleted data は放置すると増え続けます。

P7 では retention policy を文書化するだけでなく、purge job と tenant data export job の入口を用意します。実際の保持期間は deploy 先の規約に合わせて設定できるようにします。

### Playwright は smoke を置き換えない

shell smoke は速く、CI の初期失敗を見つけるのに向いています。Playwright は browser behavior、CSRF、SPA routing、フォーム、確認 dialog を見るために追加します。

P7 では両方残します。

## Step 1. security hardening を追加する

P7 の他機能を足す前に、全 request / response に効く security boundary を固定します。

### 1-1. security config を追加する

#### ファイル: `.env.example`

```dotenv
SECURITY_HEADERS_ENABLED=true
SECURITY_CSP="default-src 'self'; base-uri 'self'; frame-ancestors 'none'; object-src 'none'"
SECURITY_HSTS_ENABLED=false
SECURITY_HSTS_MAX_AGE=31536000
MAX_REQUEST_BODY_BYTES=10485760
TRUSTED_PROXY_CIDRS=
CORS_ALLOWED_ORIGINS=
```

local HTTP では HSTS を default off にします。本番で HTTPS 配信が確定したら `SECURITY_HSTS_ENABLED=true` にします。

### 1-2. security middleware を追加する

#### ファイル: `backend/internal/middleware/security_headers.go`

追加する header は次です。

```text
Content-Security-Policy
X-Content-Type-Options: nosniff
Referrer-Policy: strict-origin-when-cross-origin
X-Frame-Options: DENY
Strict-Transport-Security
```

`Strict-Transport-Security` は `SECURITY_HSTS_ENABLED=true` のときだけ返します。

#### ファイル: `backend/internal/app/app.go`

Gin router の初期化直後、Huma route 登録より前に middleware を入れます。

```go
router.Use(middleware.SecurityHeaders(middleware.SecurityHeadersConfig{
	Enabled:        cfg.SecurityHeadersEnabled,
	CSP:            cfg.SecurityCSP,
	HSTSEnabled:    cfg.SecurityHSTSEnabled,
	HSTSMaxAge:     cfg.SecurityHSTSMaxAge,
}))
```

### 1-3. request body size limit を追加する

#### ファイル: `backend/internal/middleware/body_limit.go`

全体の default limit と、file upload 用の個別 limit を分けます。

- default API body: `MAX_REQUEST_BODY_BYTES`
- file upload body: `FILE_MAX_BYTES` + multipart overhead

`/healthz`、`/readyz`、`/metrics` は body を読まないので、limit middleware の影響はありません。

### 1-4. trusted proxy / CORS 方針を固定する

#### ファイル: `backend/cmd/main/main.go`

`TRUSTED_PROXY_CIDRS` が空なら、Gin の trusted proxy は loopback だけにします。本番で reverse proxy を使う場合だけ CIDR を明示します。

CORS は default deny です。browser app は same-origin single binary 配信を基本にし、別 origin の frontend dev server だけ local で許可します。

## Step 2. P7 config を追加する

### 2-1. `.env.example` に設定を追加する

#### ファイル: `.env.example`

```dotenv
IDEMPOTENCY_ENABLED=true
IDEMPOTENCY_TTL=24h

OUTBOX_WORKER_ENABLED=false
OUTBOX_WORKER_INTERVAL=5s
OUTBOX_WORKER_TIMEOUT=10s
OUTBOX_WORKER_BATCH_SIZE=25
OUTBOX_WORKER_MAX_ATTEMPTS=8

EMAIL_DELIVERY_MODE=log
SMTP_HOST=
SMTP_PORT=587
SMTP_USERNAME=
SMTP_PASSWORD=
SMTP_FROM_ADDRESS=no-reply@example.test
INVITATION_TTL=168h

FILE_STORAGE_DRIVER=local
FILE_LOCAL_DIR=.data/files
FILE_MAX_BYTES=10485760
FILE_ALLOWED_MIME_TYPES=image/png,image/jpeg,application/pdf,text/plain

RATE_LIMIT_ENABLED=true
RATE_LIMIT_LOGIN_PER_MINUTE=10
RATE_LIMIT_BROWSER_API_PER_MINUTE=120
RATE_LIMIT_EXTERNAL_API_PER_MINUTE=300

TENANT_DEFAULT_FILE_QUOTA_BYTES=1073741824

DATA_RETENTION_OUTBOX_DAYS=30
DATA_RETENTION_NOTIFICATIONS_DAYS=180
DATA_RETENTION_DELETED_FILES_DAYS=30
DATA_EXPORT_TTL=168h
```

`OUTBOX_WORKER_ENABLED` は local では `false` から始めます。実装確認時だけ明示的に `true` にして worker を動かします。

### 2-2. Config に field を追加する

#### ファイル: `backend/internal/config/config.go`

`Config` に次を追加します。

```go
SecurityHeadersEnabled bool
SecurityCSP            string
SecurityHSTSEnabled    bool
SecurityHSTSMaxAge     int
MaxRequestBodyBytes    int64
TrustedProxyCIDRs      []string
CORSAllowedOrigins     []string

IdempotencyEnabled bool
IdempotencyTTL     time.Duration

OutboxWorkerEnabled     bool
OutboxWorkerInterval    time.Duration
OutboxWorkerTimeout     time.Duration
OutboxWorkerBatchSize   int
OutboxWorkerMaxAttempts int

EmailDeliveryMode string
SMTPHost          string
SMTPPort          int
SMTPUsername      string
SMTPPassword      string
SMTPFromAddress   string
InvitationTTL     time.Duration

FileStorageDriver     string
FileLocalDir          string
FileMaxBytes          int64
FileAllowedMIMETypes  []string

RateLimitEnabled              bool
RateLimitLoginPerMinute       int
RateLimitBrowserAPIPerMinute  int
RateLimitExternalAPIPerMinute int

TenantDefaultFileQuotaBytes int64

DataRetentionOutboxDays        int
DataRetentionNotificationsDays int
DataRetentionDeletedFilesDays  int
DataExportTTL                  time.Duration
```

duration は既存の `getEnvPositiveDuration` を使います。size と rate limit は 0 以下なら fallback に丸めます。

```go
outboxWorkerInterval, err := getEnvPositiveDuration("OUTBOX_WORKER_INTERVAL", "5s")
if err != nil {
	return Config{}, err
}
outboxWorkerTimeout, err := getEnvPositiveDuration("OUTBOX_WORKER_TIMEOUT", "10s")
if err != nil {
	return Config{}, err
}
```

`FILE_ALLOWED_MIME_TYPES` は既存の `getEnvCSV` を使います。

## Step 3. DB schema を追加する

P7 では migration `0012_web_service_common` を追加します。

### 3-1. outbox schema

#### ファイル: `db/migrations/0012_web_service_common.up.sql`

```sql
CREATE TABLE outbox_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT REFERENCES tenants(id) ON DELETE SET NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'sent', 'failed', 'dead')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 8 CHECK (max_attempts > 0),
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_at TIMESTAMPTZ,
    locked_by TEXT,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX outbox_events_public_id_key ON outbox_events (public_id);

CREATE INDEX outbox_events_pending_idx
    ON outbox_events (available_at, id)
    WHERE status IN ('pending', 'failed');

CREATE INDEX outbox_events_tenant_created_idx
    ON outbox_events (tenant_id, created_at DESC)
    WHERE tenant_id IS NOT NULL;
```

`pending` / `failed` だけを worker が引くので、partial index にします。worker は `available_at <= now()` も見るため、`available_at, id` の順にします。

### 3-2. idempotency schema

#### ファイル: `db/migrations/0012_web_service_common.up.sql`

```sql
CREATE TABLE idempotency_keys (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT REFERENCES tenants(id) ON DELETE CASCADE,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    scope TEXT NOT NULL,
    idempotency_key_hash TEXT NOT NULL,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'processing'
        CHECK (status IN ('processing', 'completed', 'failed')),
    response_status INTEGER,
    response_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idempotency_keys_scope_key_hash_key
    ON idempotency_keys (scope, idempotency_key_hash);

CREATE INDEX idempotency_keys_expires_idx
    ON idempotency_keys (expires_at)
    WHERE status IN ('completed', 'failed');
```

`scope` は method / route template / actor / tenant を service 側で正規化した文字列にします。raw URL、email、request body 全文、idempotency key 平文は保存しません。

### 3-3. notification schema

#### ファイル: `db/migrations/0012_web_service_common.up.sql`

```sql
CREATE TABLE notifications (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT REFERENCES tenants(id) ON DELETE CASCADE,
    recipient_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel TEXT NOT NULL DEFAULT 'in_app'
        CHECK (channel IN ('in_app', 'email')),
    template TEXT NOT NULL,
    subject TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'sent', 'failed', 'read', 'suppressed')),
    outbox_event_id BIGINT REFERENCES outbox_events(id) ON DELETE SET NULL,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX notifications_public_id_key ON notifications (public_id);

CREATE INDEX notifications_recipient_unread_idx
    ON notifications (recipient_user_id, created_at DESC)
    WHERE read_at IS NULL;

CREATE INDEX notifications_tenant_created_idx
    ON notifications (tenant_id, created_at DESC)
    WHERE tenant_id IS NOT NULL;
```

notification 本文は自由記述ではなく system-generated にします。顧客名や本文全文など、業務データの長文を通知 metadata にコピーしないようにします。

### 3-4. invitation schema

#### ファイル: `db/migrations/0012_web_service_common.up.sql`

```sql
CREATE TABLE tenant_invitations (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    invited_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    accepted_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    invitee_email_normalized TEXT NOT NULL,
    role_codes JSONB NOT NULL DEFAULT '[]'::jsonb,
    token_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'revoked', 'expired')),
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX tenant_invitations_public_id_key ON tenant_invitations (public_id);
CREATE UNIQUE INDEX tenant_invitations_token_hash_key ON tenant_invitations (token_hash);

CREATE INDEX tenant_invitations_pending_tenant_email_idx
    ON tenant_invitations (tenant_id, invitee_email_normalized)
    WHERE status = 'pending';

CREATE INDEX tenant_invitations_expires_idx
    ON tenant_invitations (expires_at)
    WHERE status = 'pending';
```

email は normalized value だけを DB に保存します。表示用の元 email が必要になった場合でも、audit metadata や metrics label には入れません。

### 3-5. file metadata schema

#### ファイル: `db/migrations/0012_web_service_common.up.sql`

```sql
CREATE TABLE file_objects (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    uploaded_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    purpose TEXT NOT NULL DEFAULT 'attachment'
        CHECK (purpose IN ('attachment', 'avatar', 'import', 'export')),
    attached_to_type TEXT,
    attached_to_id TEXT,
    original_filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    byte_size BIGINT NOT NULL CHECK (byte_size >= 0),
    sha256_hex TEXT NOT NULL,
    storage_driver TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'deleted')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX file_objects_public_id_key ON file_objects (public_id);
CREATE UNIQUE INDEX file_objects_storage_key_key ON file_objects (storage_key);

CREATE INDEX file_objects_tenant_created_idx
    ON file_objects (tenant_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX file_objects_attachment_idx
    ON file_objects (tenant_id, attached_to_type, attached_to_id, created_at DESC)
    WHERE deleted_at IS NULL;
```

`attached_to_type` / `attached_to_id` は polymorphic attachment として扱います。P7 初期版では `customer_signal` にだけ接続し、他の domain は後で増やします。

### 3-6. tenant settings schema

#### ファイル: `db/migrations/0012_web_service_common.up.sql`

```sql
CREATE TABLE tenant_settings (
    tenant_id BIGINT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    file_quota_bytes BIGINT NOT NULL CHECK (file_quota_bytes >= 0),
    rate_limit_login_per_minute INTEGER CHECK (rate_limit_login_per_minute IS NULL OR rate_limit_login_per_minute > 0),
    rate_limit_browser_api_per_minute INTEGER CHECK (rate_limit_browser_api_per_minute IS NULL OR rate_limit_browser_api_per_minute > 0),
    rate_limit_external_api_per_minute INTEGER CHECK (rate_limit_external_api_per_minute IS NULL OR rate_limit_external_api_per_minute > 0),
    notifications_enabled BOOLEAN NOT NULL DEFAULT true,
    features JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`features` は初期版では JSONB で十分です。頻繁に検索する feature が出たら、個別 column または entitlement table に分けます。

### 3-7. tenant data export schema

#### ファイル: `db/migrations/0012_web_service_common.up.sql`

```sql
CREATE TABLE tenant_data_exports (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    requested_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    outbox_event_id BIGINT REFERENCES outbox_events(id) ON DELETE SET NULL,
    file_object_id BIGINT REFERENCES file_objects(id) ON DELETE SET NULL,
    format TEXT NOT NULL DEFAULT 'json'
        CHECK (format IN ('json', 'csv')),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'ready', 'failed', 'deleted')),
    error_summary TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX tenant_data_exports_public_id_key ON tenant_data_exports (public_id);

CREATE INDEX tenant_data_exports_tenant_created_idx
    ON tenant_data_exports (tenant_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX tenant_data_exports_pending_idx
    ON tenant_data_exports (created_at, id)
    WHERE status IN ('pending', 'processing');
```

export body は file storage に置き、DB には file metadata への参照だけを保存します。

### 3-8. down migration

#### ファイル: `db/migrations/0012_web_service_common.down.sql`

```sql
DROP TABLE IF EXISTS tenant_data_exports;
DROP TABLE IF EXISTS tenant_settings;
DROP TABLE IF EXISTS file_objects;
DROP TABLE IF EXISTS tenant_invitations;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS outbox_events;
```

順序は FK 参照の逆順にします。

## Step 4. sqlc query を追加する

### 4-1. outbox query

#### ファイル: `db/queries/outbox.sql`

```sql
-- name: CreateOutboxEvent :one
INSERT INTO outbox_events (
    tenant_id,
    aggregate_type,
    aggregate_id,
    event_type,
    payload,
    max_attempts
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: ClaimOutboxEvents :many
UPDATE outbox_events
SET
    status = 'processing',
    locked_at = now(),
    locked_by = sqlc.arg(worker_id),
    attempts = attempts + 1,
    updated_at = now()
WHERE id IN (
    SELECT id
    FROM outbox_events
    WHERE status IN ('pending', 'failed')
      AND available_at <= now()
      AND attempts < max_attempts
    ORDER BY available_at, id
    LIMIT sqlc.arg(batch_size)
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkOutboxEventSent :one
UPDATE outbox_events
SET
    status = 'sent',
    locked_at = NULL,
    locked_by = NULL,
    last_error = NULL,
    processed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkOutboxEventRetry :one
UPDATE outbox_events
SET
    status = 'failed',
    locked_at = NULL,
    locked_by = NULL,
    last_error = left(sqlc.arg(last_error), 1000),
    available_at = now() + sqlc.arg(backoff),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: MarkOutboxEventDead :one
UPDATE outbox_events
SET
    status = 'dead',
    locked_at = NULL,
    locked_by = NULL,
    last_error = left(sqlc.arg(last_error), 1000),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
```

worker queue は `FOR UPDATE SKIP LOCKED` を使います。複数 worker が同時に動いても、同じ row を待ち合わずに別 row を claim できます。

### 4-2. idempotency query

#### ファイル: `db/queries/idempotency.sql`

```sql
-- name: CreateIdempotencyKey :one
INSERT INTO idempotency_keys (
    tenant_id,
    actor_user_id,
    scope,
    idempotency_key_hash,
    method,
    path,
    request_hash,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetIdempotencyKeyForScope :one
SELECT *
FROM idempotency_keys
WHERE scope = $1
  AND idempotency_key_hash = $2
  AND expires_at > now();

-- name: CompleteIdempotencyKey :one
UPDATE idempotency_keys
SET
    status = 'completed',
    response_status = $2,
    response_summary = $3,
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: FailIdempotencyKey :one
UPDATE idempotency_keys
SET
    status = 'failed',
    response_status = $2,
    response_summary = $3,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteExpiredIdempotencyKeys :execrows
DELETE FROM idempotency_keys
WHERE expires_at <= now();
```

同じ key で request hash が違う場合は `409 Conflict` にします。同じ key / 同じ hash で completed の場合は保存済み response summary を使って同等の response を返します。

### 4-3. notification query

#### ファイル: `db/queries/notifications.sql`

```sql
-- name: CreateNotification :one
INSERT INTO notifications (
    tenant_id,
    recipient_user_id,
    channel,
    template,
    subject,
    body,
    metadata,
    outbox_event_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: ListNotificationsForUser :many
SELECT *
FROM notifications
WHERE recipient_user_id = $1
  AND ($2::bigint IS NULL OR tenant_id = $2)
ORDER BY created_at DESC, id DESC
LIMIT $3;

-- name: MarkNotificationRead :one
UPDATE notifications
SET
    status = 'read',
    read_at = COALESCE(read_at, now()),
    updated_at = now()
WHERE public_id = $1
  AND recipient_user_id = $2
RETURNING *;
```

list は active tenant で絞れるようにします。ただし global notification を将来追加できるよう、`tenant_id` は nullable にします。

### 4-4. invitation query

#### ファイル: `db/queries/tenant_invitations.sql`

```sql
-- name: CreateTenantInvitation :one
INSERT INTO tenant_invitations (
    tenant_id,
    invited_by_user_id,
    invitee_email_normalized,
    role_codes,
    token_hash,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: ListTenantInvitations :many
SELECT *
FROM tenant_invitations
WHERE tenant_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: GetPendingTenantInvitationByTokenHash :one
SELECT *
FROM tenant_invitations
WHERE token_hash = $1
  AND status = 'pending'
  AND expires_at > now();

-- name: AcceptTenantInvitation :one
UPDATE tenant_invitations
SET
    status = 'accepted',
    accepted_by_user_id = $2,
    accepted_at = now(),
    updated_at = now()
WHERE id = $1
  AND status = 'pending'
RETURNING *;

-- name: RevokeTenantInvitation :one
UPDATE tenant_invitations
SET
    status = 'revoked',
    revoked_at = now(),
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND status = 'pending'
RETURNING *;

-- name: ExpireTenantInvitations :execrows
UPDATE tenant_invitations
SET
    status = 'expired',
    updated_at = now()
WHERE status = 'pending'
  AND expires_at <= now();
```

accept 時は invitation を更新するだけでなく、同じ transaction で `tenant_memberships` に local override role を追加します。

### 4-5. file object query

#### ファイル: `db/queries/file_objects.sql`

```sql
-- name: CreateFileObject :one
INSERT INTO file_objects (
    tenant_id,
    uploaded_by_user_id,
    purpose,
    attached_to_type,
    attached_to_id,
    original_filename,
    content_type,
    byte_size,
    sha256_hex,
    storage_driver,
    storage_key
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: GetFileObjectForTenant :one
SELECT *
FROM file_objects
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL;

-- name: ListFileObjectsForAttachment :many
SELECT *
FROM file_objects
WHERE tenant_id = $1
  AND attached_to_type = $2
  AND attached_to_id = $3
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: SoftDeleteFileObjectForTenant :one
UPDATE file_objects
SET
    status = 'deleted',
    deleted_at = COALESCE(deleted_at, now()),
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
RETURNING *;
```

file body の削除は、DB transaction commit 後に storage driver 側で行います。初期版では soft delete だけでも構いません。物理削除は retention policy が固まってから追加します。

### 4-6. tenant settings query

#### ファイル: `db/queries/tenant_settings.sql`

```sql
-- name: UpsertTenantSettings :one
INSERT INTO tenant_settings (
    tenant_id,
    file_quota_bytes,
    rate_limit_login_per_minute,
    rate_limit_browser_api_per_minute,
    rate_limit_external_api_per_minute,
    notifications_enabled,
    features
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (tenant_id) DO UPDATE
SET
    file_quota_bytes = EXCLUDED.file_quota_bytes,
    rate_limit_login_per_minute = EXCLUDED.rate_limit_login_per_minute,
    rate_limit_browser_api_per_minute = EXCLUDED.rate_limit_browser_api_per_minute,
    rate_limit_external_api_per_minute = EXCLUDED.rate_limit_external_api_per_minute,
    notifications_enabled = EXCLUDED.notifications_enabled,
    features = EXCLUDED.features,
    updated_at = now()
RETURNING *;

-- name: GetTenantSettings :one
SELECT *
FROM tenant_settings
WHERE tenant_id = $1;

-- name: SumActiveFileBytesForTenant :one
SELECT COALESCE(sum(byte_size), 0)::bigint AS byte_size
FROM file_objects
WHERE tenant_id = $1
  AND deleted_at IS NULL;
```

quota 判定は upload 前に `SumActiveFileBytesForTenant` を使います。厳密な同時 upload 制御が必要になったら、tenant row lock または quota usage table を追加します。

### 4-7. tenant data export query

#### ファイル: `db/queries/tenant_data_exports.sql`

```sql
-- name: CreateTenantDataExport :one
INSERT INTO tenant_data_exports (
    tenant_id,
    requested_by_user_id,
    format,
    expires_at
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: ListTenantDataExports :many
SELECT *
FROM tenant_data_exports
WHERE tenant_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: MarkTenantDataExportProcessing :one
UPDATE tenant_data_exports
SET
    status = 'processing',
    outbox_event_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkTenantDataExportReady :one
UPDATE tenant_data_exports
SET
    status = 'ready',
    file_object_id = $2,
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkTenantDataExportFailed :one
UPDATE tenant_data_exports
SET
    status = 'failed',
    error_summary = left($2, 1000),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SoftDeleteExpiredTenantDataExports :execrows
UPDATE tenant_data_exports
SET
    status = 'deleted',
    deleted_at = COALESCE(deleted_at, now()),
    updated_at = now()
WHERE expires_at <= now()
  AND deleted_at IS NULL;
```

export job は tenant-aware query だけを使い、他 tenant の record を混ぜません。

## Step 5. outbox service を追加する

### 5-1. enqueue API

#### ファイル: `backend/internal/service/outbox_service.go`

```go
type OutboxService struct {
	pool        *pgxpool.Pool
	queries     *db.Queries
	maxAttempts int
}

type EnqueueOutboxInput struct {
	TenantID      *int64
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       map[string]any
}

func (s *OutboxService) Enqueue(ctx context.Context, tx pgx.Tx, input EnqueueOutboxInput) (db.OutboxEvent, error) {
	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return db.OutboxEvent{}, err
	}

	queries := s.queries
	if tx != nil {
		queries = s.queries.WithTx(tx)
	}

	return queries.CreateOutboxEvent(ctx, db.CreateOutboxEventParams{
		TenantID:       nullableInt8(input.TenantID),
		AggregateType:  input.AggregateType,
		AggregateID:    input.AggregateID,
		EventType:      input.EventType,
		Payload:        payload,
		MaxAttempts:    int32(s.maxAttempts),
	})
}
```

重要なのは、business mutation と同じ transaction に入れられることです。Customer Signal を更新してから notification job だけ失敗する、という状態を避けます。

### 5-2. worker claim / handle

#### ファイル: `backend/internal/service/outbox_service.go`

```go
type OutboxHandler interface {
	HandleOutboxEvent(context.Context, db.OutboxEvent) error
}

func (s *OutboxService) RunBatch(ctx context.Context, workerID string, batchSize int, handler OutboxHandler) error {
	events, err := s.queries.ClaimOutboxEvents(ctx, db.ClaimOutboxEventsParams{
		WorkerID:  pgtype.Text{String: workerID, Valid: true},
		BatchSize: int32(batchSize),
	})
	if err != nil {
		return err
	}

	for _, event := range events {
		if err := handler.HandleOutboxEvent(ctx, event); err != nil {
			backoff := nextOutboxBackoff(event.Attempts)
			if int(event.Attempts) >= s.maxAttempts {
				_, _ = s.queries.MarkOutboxEventDead(ctx, db.MarkOutboxEventDeadParams{
					ID:        event.ID,
					LastError: err.Error(),
				})
				continue
			}
			_, _ = s.queries.MarkOutboxEventRetry(ctx, db.MarkOutboxEventRetryParams{
				ID:        event.ID,
				LastError: err.Error(),
				Backoff:   pgtype.Interval{Microseconds: backoff.Microseconds(), Valid: true},
			})
			continue
		}

		_, _ = s.queries.MarkOutboxEventSent(ctx, event.ID)
	}

	return nil
}
```

`last_error` は長さを制限し、secret や provider response body 全文を入れません。error message は調査の入口に留め、詳細は structured log / provider dashboard を見ます。

## Step 6. idempotency key を追加する

### 6-1. service を追加する

#### ファイル: `backend/internal/service/idempotency_service.go`

`IdempotencyService` は request の開始時に key を reserve し、handler 成功後に response summary を保存します。

```go
type IdempotencyService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	ttl     time.Duration
}

type IdempotencyReserveInput struct {
	TenantID           *int64
	ActorUserID        *int64
	Scope              string
	IdempotencyKeyPlain string
	Method             string
	Path               string
	RequestHash        string
}
```

処理方針は次です。

- key が無い request は通常実行する
- 同じ scope / key / request hash の completed request は replay する
- 同じ scope / key で request hash が違う場合は `409 Conflict`
- processing のまま残っている key は TTL 内なら `409 Conflict`
- key 平文は保存せず hash だけ保存する

### 6-2. middleware を追加する

#### ファイル: `backend/internal/middleware/idempotency.go`

対象は `POST` / `PUT` / `PATCH` / `DELETE` のうち、mutation として扱う API です。

P7 初期版では、必須対象を次に絞ります。

- `POST /api/v1/files`
- `POST /api/v1/admin/tenants/:tenantSlug/invitations`
- `POST /api/v1/invitations/accept`
- `POST /api/v1/admin/tenants/:tenantSlug/exports`
- external API の `POST`

既存 browser form で key を付けない場合は通常実行にします。ただし frontend wrapper から mutation には自動で UUID key を付けます。

### 6-3. frontend transport に追加する

#### ファイル: `frontend/src/api/client.ts`

generated SDK wrapper の mutation request で、caller が指定しない場合に `Idempotency-Key` を付けます。

```ts
const idempotencyKey = options.idempotencyKey ?? crypto.randomUUID()
```

同じ user action の retry では同じ key を再利用し、別 action では新しい key を作ります。

## Step 7. outbox worker を追加する

### 7-1. worker scheduler

#### ファイル: `backend/internal/jobs/outbox_worker.go`

```go
type OutboxBatchRunner interface {
	RunBatch(ctx context.Context, workerID string, batchSize int, handler service.OutboxHandler) error
}

type OutboxWorkerConfig struct {
	Enabled   bool
	Interval  time.Duration
	Timeout   time.Duration
	BatchSize int
	WorkerID  string
}
```

実装は `ReconcileScheduler` と同じ形にします。

- `Enabled=false` なら何もしない
- `Interval` / `Timeout` が不正なら error log を出して停止
- 1 process 内では overlap しない
- `RunBatch` の duration / status を metrics に出す

outbox handler は `event_type` で dispatch します。

- `notification.email_requested`
- `tenant_invitation.created`
- `tenant_data_export.requested`

未知の `event_type` は retry せず `dead` にします。deploy mismatch を早く見つけるためです。

### 7-2. metrics を追加する

#### ファイル: `backend/internal/platform/metrics.go`

追加する metrics は次です。

```go
outboxRunsTotal    *prometheus.CounterVec
outboxDuration     *prometheus.HistogramVec
outboxEventsTotal  *prometheus.CounterVec
rateLimitHitsTotal *prometheus.CounterVec
```

label は抑えます。

- outbox run: `trigger`, `status`
- outbox event: `event_type`, `status`
- rate limit: `policy`, `result`

`tenant_id`、user id、file id、outbox id は label にしません。

## Step 8. email / notification service を追加する

### 8-1. email sender interface

#### ファイル: `backend/internal/service/email_sender.go`

```go
type EmailMessage struct {
	ToUserID int64
	Subject  string
	TextBody string
}

type EmailSender interface {
	Send(ctx context.Context, message EmailMessage) error
}
```

初期版では user id から email address を service 内で解決します。outbox payload に email address をコピーしない方針にします。

### 8-2. log email sender

#### ファイル: `backend/internal/service/email_sender.go`

```go
type LogEmailSender struct {
	logger *slog.Logger
}

func (s *LogEmailSender) Send(ctx context.Context, message EmailMessage) error {
	s.logger.InfoContext(ctx, "email delivery skipped by log sender",
		"to_user_id", message.ToUserID,
		"subject_length", len(message.Subject),
		"body_length", len(message.TextBody),
	)
	return nil
}
```

log には email address、本文全文、token、secret を出しません。

### 8-3. notification service

#### ファイル: `backend/internal/service/notification_service.go`

```go
type NotificationService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	outbox  *OutboxService
	audit   *AuditService
}
```

最初に追加する use case は次にします。

- Customer Signal の status が `planned` になったら作成者へ in-app notification を作る
- 同じ transaction で `notification.email_requested` outbox event を enqueue する
- worker が log email sender で配送扱いにする

P7 初期版では notification 自体の audit event は必須にしません。user が明示的に mark read した操作だけ `notification.read` を audit に残すかどうかを判断します。

## Step 9. user invitation / onboarding を追加する

### 9-1. invitation service

#### ファイル: `backend/internal/service/tenant_invitation_service.go`

`TenantInvitationService` は tenant admin が招待を作成し、招待された user が accept する流れを扱います。

主な use case は次です。

- `CreateInvitation`
- `ListInvitations`
- `RevokeInvitation`
- `AcceptInvitation`
- `ExpireInvitations`

作成時は random token を生成し、DB には token hash だけを保存します。token 平文は response に返さず、email delivery の payload にだけ渡します。

### 9-2. invitation email

invitation 作成時に `tenant_invitation.created` outbox event を enqueue します。

worker は log email sender または SMTP sender で招待 link を送ります。link は frontend route にします。

```text
/invitations/accept?token={token}
```

backend API には token を path に入れず、body で渡します。token が access log の path に残るのを避けるためです。

### 9-3. audit 方針

invitation は tenant membership に影響するため audit に残します。

- `tenant_invitation.create`
- `tenant_invitation.revoke`
- `tenant_invitation.accept`
- `tenant_invitation.expire`

metadata には `roleCodes`、`expiresInHours`、`source` 程度を入れます。email、token、招待 link は入れません。

## Step 10. tenant-aware file service を追加する

### 10-1. storage interface

#### ファイル: `backend/internal/service/file_storage.go`

```go
type FileStorage interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}
```

### 10-2. local storage driver

#### ファイル: `backend/internal/service/local_file_storage.go`

local driver は `FILE_LOCAL_DIR` 配下に保存します。

storage key は user input から作りません。

```text
tenants/{tenant_id}/files/{file_public_id}
```

original filename は metadata と response header にだけ使います。path join には使いません。

### 10-3. file service validation

#### ファイル: `backend/internal/service/file_service.go`

validation は service 層に寄せます。

- `byte_size <= FILE_MAX_BYTES`
- `content_type` が allow list に含まれる
- filename は空でない
- attached target は active tenant 内に存在する
- upload user は active tenant role を持つ
- tenant settings の file quota を超えない

P7 初期版では attachment target を `customer_signal` だけにします。Customer Signal の存在確認は `CustomerSignalService` か dedicated query で行います。

### 10-4. audit 方針

file upload / delete は audit に残します。

- `file.upload`
- `file.delete`

metadata に入れるのは低感度情報だけです。

```json
{
  "byteSize": 12345,
  "contentType": "application/pdf",
  "purpose": "attachment",
  "attachedToType": "customer_signal"
}
```

original filename は個人情報や機密語を含む可能性があるため、audit metadata には入れません。

## Step 11. Redis rate limit middleware を追加する

### 11-1. rate limiter service

#### ファイル: `backend/internal/middleware/rate_limit.go`

P7 初期版は fixed window で十分です。

key 例:

```text
rate_limit:login:ip:{ip}:{yyyymmddhhmm}
rate_limit:browser:user:{userPublicId}:{yyyymmddhhmm}
rate_limit:external:subject:{subject}:{yyyymmddhhmm}
```

実装は Redis `INCR` + `EXPIRE` を pipeline で実行します。

```go
count, err := client.Incr(ctx, key).Result()
if count == 1 {
	_ = client.Expire(ctx, key, window+time.Minute).Err()
}
if count > limit {
	return ErrRateLimited
}
```

厳密な sliding window が必要になったら Lua script に置き換えます。初期版は実装の単純さを優先します。

### 11-2. middleware の適用場所

#### ファイル: `backend/internal/app/app.go`

適用方針は次です。

- login callback / local login: `login` policy
- `/api/v1/*` browser API: `browser_api` policy
- external bearer API / SCIM / M2M: `external_api` policy
- `/healthz`、`/readyz`、`/metrics` は対象外

rate limit 超過時は `429` を返し、`Retry-After` header を付けます。

### 11-3. metrics

rate limit は次の metrics を出します。

```text
haohao_rate_limit_requests_total{policy="browser_api",result="allowed"}
haohao_rate_limit_requests_total{policy="browser_api",result="blocked"}
```

IP、user id、subject は metrics label にしません。

## Step 12. tenant settings / quota を追加する

### 12-1. service を追加する

#### ファイル: `backend/internal/service/tenant_settings_service.go`

`TenantSettingsService` は tenant ごとの quota / rate limit override / feature enablement を扱います。

初期版で持つ操作は次です。

- `GetSettings`
- `UpdateSettings`
- `ResolveEffectiveRateLimit`
- `CheckFileQuota`

tenant settings がまだ無い tenant では、config の default 値を使って response を返します。更新時に `tenant_settings` row を upsert します。

### 12-2. file quota

file upload 前に、active tenant の使用量と quota を確認します。

```text
current active file bytes + uploading file bytes <= file_quota_bytes
```

超過時は `409 Conflict` を返します。quota 超過は audit には残さず、metrics に `file_quota_exceeded` を count します。

### 12-3. rate limit override

rate limiter は policy ごとに tenant settings を参照します。

- tenant settings に override がある場合は override を使う
- override が null の場合は config default を使う
- 未認証 login は tenant が無いので config default だけを使う

settings lookup が Redis / DB 障害で失敗した場合は、fail-open ではなく config default に fallback します。

### 12-4. audit 方針

tenant settings 更新は audit に残します。

- `tenant_settings.update`

metadata は `changedFields` だけにします。quota 値や feature JSON 全体は audit metadata に入れません。

## Step 13. API / wiring / OpenAPI を追加する

### 13-1. notification API

#### ファイル: `backend/internal/api/notifications.go`

追加する API は次です。

```text
GET  /api/v1/notifications
POST /api/v1/notifications/{notificationPublicId}/read
```

条件は次です。

- session 必須
- active tenant は optional filter として使う
- mark read は CSRF 必須
- user は自分宛の notification だけ読める

### 13-2. invitation API

#### ファイル: `backend/internal/api/tenant_invitations.go`

追加する API は次です。

```text
GET    /api/v1/admin/tenants/{tenantSlug}/invitations
POST   /api/v1/admin/tenants/{tenantSlug}/invitations
DELETE /api/v1/admin/tenants/{tenantSlug}/invitations/{invitationPublicId}
POST   /api/v1/invitations/accept
```

条件は次です。

- list / create / revoke は global role `tenant_admin` 必須
- create / revoke / accept は CSRF 必須
- accept は login 済み user を対象にし、token の email と session user email が一致することを確認する
- accept 成功時に active tenant を invitation の tenant に切り替える

### 13-3. file API

#### ファイル: `backend/internal/api/files.go`

追加する API は次です。

```text
POST   /api/v1/files
GET    /api/v1/files/{filePublicId}
DELETE /api/v1/files/{filePublicId}
GET    /api/v1/files?attachedToType=customer_signal&attachedToId={publicId}
```

条件は次です。

- session 必須
- active tenant 必須
- upload / delete は CSRF 必須
- upload は `Idempotency-Key` 推奨
- tenant role は target domain に合わせる
  - Customer Signal attachment なら `customer_signal_user`
- download も同じ tenant role を必須にする

Huma の multipart upload が扱いづらい場合は、初期版では Gin route を直接追加しても構いません。ただし OpenAPI に出す API 契約は Huma 側に寄せる方針を保ちます。

### 13-4. tenant settings API

#### ファイル: `backend/internal/api/tenant_settings.go`

追加する API は次です。

```text
GET /api/v1/admin/tenants/{tenantSlug}/settings
PUT /api/v1/admin/tenants/{tenantSlug}/settings
```

条件は次です。

- global role `tenant_admin` 必須
- update は CSRF 必須
- response には effective setting を返す
- feature JSON は object のみ許可し、array / primitive は拒否する

### 13-5. tenant data export API

#### ファイル: `backend/internal/api/tenant_data_exports.go`

追加する API は次です。

```text
GET  /api/v1/admin/tenants/{tenantSlug}/exports
POST /api/v1/admin/tenants/{tenantSlug}/exports
GET  /api/v1/admin/tenants/{tenantSlug}/exports/{exportPublicId}/download
```

条件は次です。

- global role `tenant_admin` 必須
- create は CSRF 必須
- create は `Idempotency-Key` 推奨
- download は `ready` かつ期限内の export だけ許可する

### 13-6. backend wiring

#### ファイル: `backend/internal/api/register.go`

`Dependencies` に service を追加します。

```go
IdempotencyService      *service.IdempotencyService
NotificationService     *service.NotificationService
TenantInvitationService *service.TenantInvitationService
FileService             *service.FileService
TenantSettingsService   *service.TenantSettingsService
TenantDataExportService *service.TenantDataExportService
```

`Register` に route 登録を追加します。

```go
registerNotificationRoutes(api, deps)
registerTenantInvitationRoutes(api, deps)
registerFileRoutes(api, deps)
registerTenantSettingsRoutes(api, deps)
registerTenantDataExportRoutes(api, deps)
```

#### ファイル: `backend/cmd/main/main.go`

runtime で service と worker を組み立てます。

```go
idempotencyService := service.NewIdempotencyService(pool, queries, cfg.IdempotencyTTL)
outboxService := service.NewOutboxService(pool, queries, cfg.OutboxWorkerMaxAttempts)
emailSender := service.NewEmailSender(cfg, queries, logger)
notificationService := service.NewNotificationService(pool, queries, outboxService, auditService)
tenantInvitationService := service.NewTenantInvitationService(pool, queries, outboxService, auditService, cfg.InvitationTTL)
tenantSettingsService := service.NewTenantSettingsService(pool, queries, auditService, cfg.TenantDefaultFileQuotaBytes)
fileStorage := service.NewFileStorage(cfg)
fileService := service.NewFileService(pool, queries, fileStorage, tenantSettingsService, auditService, cfg.FileMaxBytes, cfg.FileAllowedMIMETypes)
tenantDataExportService := service.NewTenantDataExportService(pool, queries, outboxService, fileStorage, auditService, cfg.DataExportTTL)
outboxHandler := service.NewOutboxHandler(emailSender, notificationService, tenantInvitationService, tenantDataExportService)
```

outbox worker は HTTP server と同じ shutdown context で起動します。

```go
outboxWorker := jobs.NewOutboxWorker(outboxService, outboxHandler, jobs.OutboxWorkerConfig{
	Enabled:   cfg.OutboxWorkerEnabled,
	Interval:  cfg.OutboxWorkerInterval,
	Timeout:   cfg.OutboxWorkerTimeout,
	BatchSize: cfg.OutboxWorkerBatchSize,
	WorkerID:  cfg.AppName,
}, logger, metrics)

go outboxWorker.Start(shutdownCtx)
```

OpenAPI export command では nil service を渡しても API schema が出るようにします。

## Step 14. frontend UI を追加する

### 14-1. notification center

追加ファイル:

- `frontend/src/api/notifications.ts`
- `frontend/src/stores/notifications.ts`
- `frontend/src/views/NotificationsView.vue`

route:

```text
/notifications
```

App header には `Notifications` link を追加します。未読 badge は最初から realtime にせず、画面表示時に list API を呼ぶだけで十分です。

UI 方針:

- table または compact list で表示する
- unread / read を chip で分ける
- mark read は行内 button
- notification body は短く表示し、長文説明カードにしない

### 14-2. invitation UI

追加先:

- `frontend/src/api/tenant-invitations.ts`
- `frontend/src/stores/tenant-invitations.ts`
- `frontend/src/views/TenantAdminTenantDetailView.vue`
- `frontend/src/views/InvitationAcceptView.vue`

tenant admin detail 画面に invitation section を追加します。

操作:

- invitee email 入力
- role selection
- invitation 作成
- pending invitation list
- revoke

accept 用に `/invitations/accept` route を追加します。URL query の token は画面上に表示せず、accept API の request body にだけ渡します。

### 14-3. Customer Signal attachment UI

追加先:

- `frontend/src/api/files.ts`
- `frontend/src/stores/files.ts`
- `frontend/src/views/CustomerSignalDetailView.vue`

Customer Signal detail に attachment section を追加します。

操作:

- file 選択
- upload
- attachment list
- download
- delete

UI 方針:

- upload input は attachment section 内に置く
- file name が長い場合は折り返す
- delete は `ConfirmActionDialog` を使う
- content type / size / uploaded at を表示する

### 14-4. tenant settings / export UI

追加先:

- `frontend/src/api/tenant-settings.ts`
- `frontend/src/api/tenant-data-exports.ts`
- `frontend/src/stores/tenant-settings.ts`
- `frontend/src/stores/tenant-data-exports.ts`
- `frontend/src/views/TenantAdminTenantDetailView.vue`

tenant admin detail 画面に settings と export section を追加します。

操作:

- file quota の表示 / 更新
- rate limit override の表示 / 更新
- notifications enabled の切り替え
- tenant data export request
- export status list
- ready export download

feature JSON の直接編集 UI は P7 初期版では text area にしません。既知 feature を toggle として出すだけにします。

## Step 15. smoke と Playwright E2E を追加する

### 15-1. smoke script

#### ファイル: `scripts/smoke-common-services.sh`

確認すること:

1. security header が返る
2. local login
3. active tenant を Acme に切り替え
4. idempotency key 付きで Customer Signal または file upload を二重送信し、重複作成されないことを確認
5. tenant invitation を作成し、pending list に出ることを確認
6. Customer Signal を作成
7. 小さな text file を upload
8. attachment list に出ることを確認
9. download できることを確認
10. delete 後に list から消えることを確認
11. tenant settings の file quota / rate limit override API が読めることを確認
12. tenant data export を request し、outbox event が作られることを確認
13. `/metrics` に outbox / rate limit / file API route が出ることを確認

#### ファイル: `Makefile`

```make
smoke-common-services:
	bash scripts/smoke-common-services.sh
```

### 15-2. Playwright を追加する

#### ファイル: `frontend/package.json`

```json
{
  "scripts": {
    "e2e": "playwright test"
  },
  "devDependencies": {
    "@playwright/test": "^1.57.0"
  }
}
```

version は実装時点で lock されるので、既存の npm lock 更新を必ず commit します。

#### ファイル: `playwright.config.ts`

```ts
import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  use: {
    baseURL: process.env.BASE_URL ?? 'http://127.0.0.1:8080',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
})
```

#### ファイル: `e2e/customer-signals-common.spec.ts`

E2E は次を確認します。

- login
- tenant selector
- Customer Signal create
- file upload
- notification page 表示
- tenant invitation create / revoke
- tenant settings 表示
- tenant data export request
- delete confirmation

test data は timestamp を suffix に付け、既存 data と衝突しないようにします。

### 15-3. Makefile target

#### ファイル: `Makefile`

```make
e2e:
	cd frontend && npm run e2e
```

single binary に対して確認する場合:

```bash
BASE_URL=http://127.0.0.1:8080 make e2e
```

## Step 16. deployment / secret management の入口を追加する

P7 初期版では、特定 cloud の Terraform を完成させるより、必要な環境変数、secret、volume、migration 手順を repository に固定します。

### 16-1. deployment runbook

#### ファイル: `RUNBOOK_DEPLOYMENT.md`

書く内容:

- production 起動に必須の環境変数
- secret として扱う環境変数
- migration 実行順
- rollback 時の注意
- file local storage を使う場合の永続 volume
- SMTP 有効化時の secret
- rate limit Redis の要件
- outbox worker を有効化する process 数
- data retention policy
- backup / restore drill

### 16-2. env contract

#### ファイル: `deploy/env.production.example`

`.env.example` とは別に、本番で必須になる値だけをまとめます。

```dotenv
APP_BASE_URL=https://example.com
AUTH_MODE=zitadel
ZITADEL_ISSUER=
ZITADEL_CLIENT_ID=
ZITADEL_CLIENT_SECRET=
DATABASE_URL=
REDIS_ADDR=
COOKIE_SECURE=true

METRICS_ENABLED=true
OTEL_TRACING_ENABLED=true
OTEL_EXPORTER_OTLP_ENDPOINT=

OUTBOX_WORKER_ENABLED=true
EMAIL_DELIVERY_MODE=smtp
SMTP_HOST=
SMTP_USERNAME=
SMTP_PASSWORD=

FILE_STORAGE_DRIVER=local
FILE_LOCAL_DIR=/var/lib/haohao/files
RATE_LIMIT_ENABLED=true
DATA_RETENTION_OUTBOX_DAYS=30
DATA_RETENTION_NOTIFICATIONS_DAYS=180
DATA_RETENTION_DELETED_FILES_DAYS=30
```

### 16-3. secret 方針

この段階では secret manager の実装は cloud 依存にしません。runbook に次を明記します。

- `.env` は local / demo 用
- production secret は environment injection または platform secret store から渡す
- `ZITADEL_CLIENT_SECRET`、`SMTP_PASSWORD`、`DOWNSTREAM_TOKEN_ENCRYPTION_KEY` は repository に置かない
- file storage path は secret ではないが、backup / retention 対象

## Step 17. data lifecycle を追加する

### 17-1. retention policy

#### ファイル: `RUNBOOK_DEPLOYMENT.md`

初期 retention は次を default とします。

| 対象 | default | 方針 |
| --- | --- | --- |
| `audit_events` | 無期限 | 監査証跡なので自動 purge しない |
| `outbox_events` | 30 日 | `sent` / `dead` を purge 対象にする |
| `notifications` | 180 日 | read 済みを purge 対象にする |
| `idempotency_keys` | 24 時間 | TTL 切れを削除する |
| deleted `file_objects` | 30 日 | metadata と body を purge 対象にする |
| `tenant_data_exports` | 7 日 | ready export を期限切れ後に soft delete する |

### 17-2. purge job

#### ファイル: `backend/internal/jobs/data_lifecycle.go`

P7 初期版では 1 日 1 回の scheduler として実装します。

処理対象:

- expired idempotency key 削除
- expired invitation 更新
- outbox cleanup
- read notification cleanup
- deleted file body cleanup
- expired tenant data export cleanup

purge job は metrics に `haohao_data_lifecycle_runs_total` と `haohao_data_lifecycle_items_total` を出します。tenant id や file id は label にしません。

### 17-3. tenant data export job

tenant data export は outbox handler として実装します。

含める対象:

- tenant metadata
- memberships
- customer signals
- file metadata
- notifications metadata
- audit event summary

file body は P7 初期版では export に含めず、metadata と download link だけにします。大量 file archive は P7.5 以降に回します。

## Step 18. backup / restore drill を追加する

### 18-1. runbook

#### ファイル: `RUNBOOK_DEPLOYMENT.md`

backup 方針だけでなく、restore 手順も書きます。

- PostgreSQL: `pg_dump` / managed backup snapshot
- Redis: session / rate limit / outbox lock は再生成可能、永続 backup は必須にしない
- local file storage: volume backup 必須
- `.env` / secret: platform secret store から復元

### 18-2. smoke

#### ファイル: `scripts/smoke-backup-restore.sh`

local では production restore ではなく、次の最小確認にします。

1. `pg_dump` が取れる
2. file storage directory を archive できる
3. dump に `tenant_settings` / `file_objects` / `outbox_events` が含まれる
4. restore 手順の dry-run command が構文上成立する

#### ファイル: `Makefile`

```make
smoke-backup-restore:
	bash scripts/smoke-backup-restore.sh
```

## Step 19. P7 後の拡張候補を分離する

P7 初期版に入れないが、将来ほぼ必要になる候補を文末に分離しておきます。

- outbound webhooks: 署名付き delivery、retry、delivery log、dead letter
- import / export jobs: CSV import、CSV export、job status UI
- search / cursor pagination: full-text search、cursor pagination、saved filters
- support access / impersonation: 理由入力、時間制限、明示 banner、audit
- feature flags / entitlements: billing / pricing plan と接続する独立機能

これらは outbox、file storage、tenant settings、audit が入った後に実装する方が安全です。

## 生成と確認

### DB / generated artifacts

```bash
make db-up
make db-schema
make sqlc
make gen
```

`db/schema.sql`、`backend/internal/db/*.sql.go`、`openapi/openapi.yaml`、`frontend/src/api/generated/*` は生成物です。手で編集しません。

### backend / frontend

```bash
go test ./backend/...
npm --prefix frontend run build
make binary
```

### local single binary smoke

```bash
set -a
source .env
export HTTP_PORT=18082
export APP_BASE_URL=http://127.0.0.1:18082
export FRONTEND_BASE_URL=http://127.0.0.1:18082
export AUTH_MODE=local
export ENABLE_LOCAL_PASSWORD_LOGIN=true
export OUTBOX_WORKER_ENABLED=true
set +a
./bin/haohao
```

別 terminal で確認します。

```bash
BASE_URL=http://127.0.0.1:18082 make smoke-operability
BASE_URL=http://127.0.0.1:18082 make smoke-observability
BASE_URL=http://127.0.0.1:18082 make smoke-customer-signals
BASE_URL=http://127.0.0.1:18082 make smoke-common-services
BASE_URL=http://127.0.0.1:18082 make smoke-backup-restore
BASE_URL=http://127.0.0.1:18082 make e2e
```

### audit 確認

```bash
docker-compose exec -T postgres psql -U haohao -d haohao -c "
SELECT action, target_type, target_id, metadata
FROM audit_events
WHERE action IN (
  'file.upload',
  'file.delete',
  'notification.read',
  'tenant_invitation.create',
  'tenant_invitation.revoke',
  'tenant_invitation.accept',
  'tenant_settings.update',
  'tenant_data_export.create'
)
ORDER BY id DESC
LIMIT 20;
"
```

期待値:

- file upload / delete が残る
- invitation / tenant settings / data export の重要 mutation が残る
- metadata に original filename や file body は入らない
- metadata に email、token、招待 link、feature JSON 全文は入らない
- tenant_id / actor_user_id が入る

### outbox 確認

```bash
docker-compose exec -T postgres psql -U haohao -d haohao -c "
SELECT event_type, status, attempts, available_at, processed_at
FROM outbox_events
ORDER BY id DESC
LIMIT 20;
"
```

期待値:

- worker 有効時は `sent` になる
- provider 失敗時は `failed` になり、`available_at` が未来になる
- 最大試行回数を超えると `dead` になる

### metrics 確認

```bash
curl -sS http://127.0.0.1:18082/metrics \
  | rg 'haohao_outbox|haohao_rate_limit|haohao_data_lifecycle|/api/v1/files|/api/v1/notifications|/api/v1/admin/tenants/.*/settings'
```

期待値:

- outbox run / event metrics が出る
- rate limit allowed / blocked metrics が出る
- data lifecycle metrics が出る
- file / notification API の HTTP metrics が route template で出る

## 実装時の注意点

### migration は既存番号の次にする

P6 まで入っている repository では `0011_customer_signals` が存在します。P7 は `0012_web_service_common` として追加します。

別 branch で migration 番号が増えている場合は、必ず最新の番号に合わせます。

### outbox worker は idempotent にする

worker は crash / retry / duplicate delivery が起きる前提で書きます。

- provider call の前後で process が落ちる可能性がある
- `sent` にする前に落ちると再送される可能性がある
- notification / email handler は duplicate に耐える

P7 初期版では log email sender なので影響は小さいですが、SMTP に切り替える前に idempotency key を provider に渡せる設計にしておきます。

### idempotency key は secret として扱う

`Idempotency-Key` は再実行制御に使う値なので、平文保存しません。

- DB には hash だけを保存する
- metrics label に入れない
- audit metadata に入れない
- request hash が違う再利用は `409 Conflict` にする

### invitation token は URL path に入れない

招待 token は email link で user に渡りますが、backend API の path には入れません。

frontend route の query から受け取り、backend には JSON body で渡します。backend access log の path に token が残るのを避けるためです。

### file upload は request body を丸ごと memory に載せない

small file でも、handler は streaming 前提にします。

- `http.MaxBytesReader` または Huma/Gin の size limit を使う
- checksum は stream を読みながら計算する
- temp file に書いてから metadata transaction を commit する
- commit 後に final path へ move する

local filesystem では atomic rename を使えます。S3 では temp object / final object の扱いが変わるため、driver interface の中に閉じ込めます。

### rate limit は reverse proxy の IP header を雑に信じない

local では `RemoteAddr` で十分です。本番で `X-Forwarded-For` を使う場合は、trusted proxy の設定を追加してから使います。

最初から任意の `X-Forwarded-For` を信じると、client が rate limit key を偽装できます。

### notification body に業務データ全文を複製しない

notification は便利ですが、業務データの複製先にもなります。Customer Signal の本文全文や添付 file 名を notification body / metadata にコピーしないようにします。

notification には短い system message と target id だけを入れ、詳細は権限チェック付きの detail API から読ませます。

### tenant settings は entitlement の代替にしすぎない

P7 では tenant settings に quota / override / feature toggle を置きます。ただし billing / pricing plan を実装する段階では、plan と entitlement は別 table に分けます。

tenant settings は local override と runtime default の置き場として扱います。

### restore drill は CI で重くしすぎない

`smoke-backup-restore` は local / nightly 向けにし、通常 CI では command 構文と small dump の確認に留めます。

本番 backup の restore は platform 依存なので、runbook に実施頻度と責任者を書く方針にします。

## 最終確認チェックリスト

- `make db-up` が通る
- `make db-schema` 後の `db/schema.sql` が migration と一致する
- `make sqlc` が通る
- `make gen` が通る
- `go test ./backend/...` が通る
- `npm --prefix frontend run build` が通る
- `make binary` が通る
- `BASE_URL=... make smoke-common-services` が通る
- `BASE_URL=... make smoke-backup-restore` が通る
- `BASE_URL=... make e2e` が通る
- `audit_events` に `file.upload` / `file.delete` / invitation / tenant settings / export が残る
- `outbox_events` が worker により `sent` になる
- `/metrics` に outbox / rate limit / data lifecycle / file / notification の観測値が出る
- browser で `/notifications` が開ける
- tenant admin detail で invitation / tenant settings / data export を操作できる
- Customer Signal detail で file upload / download / delete ができる
- rate limit 超過時に `429` と `Retry-After` が返る
- production env contract、deployment runbook、backup / restore drill が repository にある
