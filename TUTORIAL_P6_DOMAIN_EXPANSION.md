# P6 業務ドメイン拡張実装チュートリアル

## この文書の目的

この文書は、`deep-research-report.md` の **P6: 業務ドメイン拡張** を、現在の HaoHao に実装できる順番に分解したチュートリアルです。

P2 では tenant-aware TODO を追加し、P3 では audit log、P4 では observability、P5 では tenant 管理 UI を追加しました。P6 では、その土台の上に **TODO ではない実用寄りの業務ドメイン** を追加します。

このチュートリアルでは、最初の業務ドメインとして **Customer Signals** を実装します。

Customer Signal は、顧客要望、問い合わせ、商談メモ、調査メモ、社内 feedback などを 1 つの tenant-aware な業務 record として扱う機能です。後続で Product Decisions を追加する場合も、Customer Signals を入力として紐付けられます。

この文書は `TUTORIAL.md` / `TUTORIAL_SINGLE_BINARY.md` / `TUTORIAL_P2_TODO.md` / `TUTORIAL_P5_TENANT_ADMIN_UI.md` と同じように、対象ファイル、主要コード方針、確認コマンド、完了条件まで追える形にしています。

## この文書が前提にしている現在地

このチュートリアルを始める前の repository は、少なくとも次の状態にある前提で進めます。

- `tenants`、`roles`、`tenant_memberships` が存在する
- `tenant_admin` role と tenant 管理 UI が存在する
- `AuditService` が mutation と同じ transaction で audit event を保存できる
- `/metrics` と request log で API route が観測できる
- `frontend/src/api/client.ts` が Cookie / CSRF / generated SDK の共通 transport を持っている
- `frontend/src/components/AdminAccessDenied.vue` が role 不足 UI として使える
- `frontend/src/components/ConfirmActionDialog.vue` が destructive action の確認 UI として使える
- `make gen` が sqlc / OpenAPI / frontend generated SDK をまとめて更新する
- `make smoke-operability`、`make smoke-observability`、`make smoke-tenant-admin` が既存 server に対して確認できる

この P6 では、Product Decisions までは実装しません。まず Customer Signals の list/detail/create/update/delete を縦に通し、次の業務ドメインを追加するときの型を作ります。

## 完成条件

このチュートリアルの完了条件は次です。

- tenant role `customer_signal_user` が追加される
- local seed の demo user が Acme / Beta で `customer_signal_user` を持つ
- P5 tenant 管理 UI から `customer_signal_user` を grant できる
- `customer_signals` table が追加される
- `customer_signals` は tenant-aware で、tenant 外の record を読めない
- list/detail/create/update/delete の API が `/api/v1/customer-signals` に追加される
- API は active tenant と tenant role `customer_signal_user` を必須にする
- mutation は CSRF token を必須にする
- create/update/delete が `audit_events` に残る
- audit metadata に顧客名、本文全文、長文自由入力、secret を入れない
- `/customer-signals` と `/customer-signals/{publicId}` の UI が追加される
- tenant を Acme / Beta で切り替えると Signals 一覧が混ざらない
- `make gen`、`go test ./backend/...`、`npm --prefix frontend run build`、`make binary` が通る
- single binary を起動した状態で operability / observability / customer-signals smoke が通る

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | role / seed / tenant admin UI | `customer_signal_user` を tenant role として使えるようにする |
| Step 2 | `db/migrations/0011_customer_signals.*.sql` | Customer Signals schema を追加する |
| Step 3 | `db/queries/customer_signals.sql` | sqlc 用 query を追加する |
| Step 4 | `backend/internal/service/customer_signal_service.go` | domain service、validation、audit 方針を実装する |
| Step 5 | `backend/internal/api/customer_signals.go` | Huma API を追加する |
| Step 6 | backend wiring / OpenAPI | runtime と OpenAPI export に service を接続する |
| Step 7 | `frontend/src/api/*`, store | generated SDK wrapper と Pinia store を追加する |
| Step 8 | `frontend/src/views/*`, router, nav | Signals list/detail UI を追加する |
| Step 9 | smoke / audit / metrics 確認 | API、audit、metrics を確認する |
| Step 10 | build / binary / browser 確認 | frontend と single binary で回帰を確認する |

## 先に決める方針

### 題材は Customer Signals にする

P6 の候補には Customer Signals、Product Decisions、Lightweight CRM、Internal approvals があります。最初は Customer Signals を選びます。

理由は次です。

- TODO より業務らしいが、CRM 全体より小さく実装できる
- tenant-aware CRUD、role、audit、metrics、UI smoke の練習に向いている
- Product Decisions を追加するときに関連元として使える
- 自由記述を含むため、audit metadata に何を入れないかを確認しやすい

### delete は soft delete として扱う

Customer Signal は業務 record です。誤作成を消せる UI は必要ですが、後から監査したい record でもあります。

P6 では `DELETE /api/v1/customer-signals/{signalPublicId}` を物理削除ではなく soft delete として扱います。

- `customer_signals.deleted_at` を持つ
- list/detail は `deleted_at IS NULL` だけを返す
- delete mutation は `deleted_at = now()` にする
- audit event は `customer_signal.delete` として残す

### role は tenant-scoped にする

Customer Signals は tenant の業務データです。API 利用条件は global role ではなく active tenant の tenant role `customer_signal_user` にします。

P5 で tenant 管理 UI が入ったので、local dev では次の 2 通りで role を付与できます。

- seed で `demo@example.com` に Acme / Beta の `customer_signal_user` を付ける
- tenant 管理 UI で既存 user に `customer_signal_user` を grant する

### metrics label に顧客名や signal ID を入れない

P4 の方針どおり、metrics label に tenant slug、user id、email、customer name、signal public id を入れません。

Customer Signals API も HTTP metrics の route template、method、status class で十分です。個別 record の追跡は audit log と request log に寄せます。

## Step 1. `customer_signal_user` role を追加する

### 1-1. role を migration に入れる

Customer Signals の schema migration に role seed も入れます。

#### ファイル: `db/migrations/0011_customer_signals.up.sql`

```sql
INSERT INTO roles (code)
VALUES ('customer_signal_user')
ON CONFLICT (code) DO NOTHING;
```

down migration では、この role を使う local membership を先に消してから role を消します。table を drop する前後の順序は、FK と参照関係に合わせて調整します。

#### ファイル: `db/migrations/0011_customer_signals.down.sql`

```sql
DELETE FROM tenant_memberships
WHERE role_id IN (
    SELECT id
    FROM roles
    WHERE code = 'customer_signal_user'
);

DELETE FROM roles
WHERE code = 'customer_signal_user';
```

### 1-2. local seed に role を追加する

#### ファイル: `scripts/seed-demo-user.sql`

roles seed に `customer_signal_user` を追加します。

```sql
INSERT INTO roles (code)
VALUES
    ('customer_signal_user'),
    ('docs_reader'),
    ('machine_client_admin'),
    ('tenant_admin'),
    ('todo_user')
ON CONFLICT (code) DO NOTHING;
```

Acme / Beta の tenant membership にも追加します。

```sql
INSERT INTO tenant_memberships (user_id, tenant_id, role_id, source)
SELECT u.id, t.id, r.id, 'local_override'
FROM users u
JOIN tenants t ON t.slug IN ('acme', 'beta')
JOIN roles r ON r.code IN ('customer_signal_user', 'docs_reader', 'todo_user')
WHERE u.email = 'demo@example.com'
ON CONFLICT (user_id, tenant_id, role_id, source) DO UPDATE
SET
    active = true,
    updated_at = now();
```

### 1-3. provider / tenant admin の許可 role に追加する

#### ファイル: `backend/internal/service/authz_service.go`

`supportedTenantRoles` に `customer_signal_user` を追加します。

```go
var supportedTenantRoles = map[string]struct{}{
	"customer_signal_user": {},
	"docs_reader":          {},
	"todo_user":            {},
}
```

provider から plain role としても同期したい場合は、`supportedGlobalRoles` にも追加できます。ただし Customer Signals API は global role ではなく active tenant role を見ます。

#### ファイル: `frontend/src/views/TenantAdminTenantDetailView.vue`

P5 tenant 管理 UI の grant 候補にも追加します。

```ts
const grantRoleCode = ref('customer_signal_user')
const tenantRoleOptions = ['customer_signal_user', 'docs_reader', 'todo_user']
```

既存 tenant に後から `customer_signal_user` を付けられるようにするためです。

## Step 2. Customer Signals schema / migration を追加する

### 2-1. up migration を追加する

#### ファイル: `db/migrations/0011_customer_signals.up.sql`

```sql
INSERT INTO roles (code)
VALUES ('customer_signal_user')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE customer_signals (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    customer_name TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'other'
        CHECK (source IN ('support', 'sales', 'customer_success', 'research', 'internal', 'other')),
    priority TEXT NOT NULL DEFAULT 'medium'
        CHECK (priority IN ('low', 'medium', 'high', 'urgent')),
    status TEXT NOT NULL DEFAULT 'new'
        CHECK (status IN ('new', 'triaged', 'planned', 'closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CHECK (btrim(customer_name) <> ''),
    CHECK (btrim(title) <> ''),
    CHECK (btrim(source) <> ''),
    CHECK (btrim(priority) <> ''),
    CHECK (btrim(status) <> '')
);

CREATE UNIQUE INDEX customer_signals_public_id_idx
    ON customer_signals(public_id);

CREATE INDEX customer_signals_tenant_created_at_idx
    ON customer_signals(tenant_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX customer_signals_tenant_status_created_at_idx
    ON customer_signals(tenant_id, status, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX customer_signals_tenant_open_priority_idx
    ON customer_signals(tenant_id, priority, created_at DESC, id DESC)
    WHERE deleted_at IS NULL
      AND status <> 'closed';

CREATE INDEX customer_signals_created_by_user_id_idx
    ON customer_signals(created_by_user_id)
    WHERE created_by_user_id IS NOT NULL;
```

`tenant_id` は必須です。すべての read / write query で `tenant_id` を条件に入れ、tenant 外の record を扱えないようにします。

`customer_name`、`title`、`body` は自由入力です。audit metadata には全文を入れません。

`customer_signals_tenant_open_priority_idx` は、よく見る「未完了 signal を優先度順で見る」画面に効く partial index です。closed record まで常に同じ index に入れる必要はありません。

### 2-2. down migration を追加する

#### ファイル: `db/migrations/0011_customer_signals.down.sql`

```sql
DROP TABLE IF EXISTS customer_signals;

DELETE FROM tenant_memberships
WHERE role_id IN (
    SELECT id
    FROM roles
    WHERE code = 'customer_signal_user'
);

DELETE FROM roles
WHERE code = 'customer_signal_user';
```

### 2-3. schema snapshot を更新する

```bash
make db-up
make db-schema
```

`db/schema.sql` は生成物ですが、この repository では tracked artifact です。migration を足したら差分が出るのが正しい状態です。

## Step 3. sqlc query を追加する

#### ファイル: `db/queries/customer_signals.sql`

```sql
-- name: ListCustomerSignalsByTenantID :many
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
WHERE tenant_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: GetCustomerSignalByPublicIDForTenant :one
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
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
LIMIT 1;

-- name: CreateCustomerSignal :one
INSERT INTO customer_signals (
    tenant_id,
    created_by_user_id,
    customer_name,
    title,
    body,
    source,
    priority,
    status
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
RETURNING
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
    deleted_at;

-- name: UpdateCustomerSignalByPublicIDForTenant :one
UPDATE customer_signals
SET
    customer_name = $3,
    title = $4,
    body = $5,
    source = $6,
    priority = $7,
    status = $8,
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
RETURNING
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
    deleted_at;

-- name: SoftDeleteCustomerSignalByPublicIDForTenant :execrows
UPDATE customer_signals
SET
    deleted_at = now(),
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL;
```

query を追加したら sqlc を生成します。

```bash
cd backend && sqlc generate
```

ここで `backend/internal/db/customer_signals.sql.go` が生成されます。

## Step 4. CustomerSignalService を追加する

#### ファイル: `backend/internal/service/customer_signal_service.go`

`CustomerSignalService` は HTTP や Cookie を知らない domain service として作ります。認証済み user ID と active tenant ID は API 層から渡します。

公開する method は次を基本にします。

```go
func (s *CustomerSignalService) List(ctx context.Context, tenantID int64) ([]CustomerSignal, error)
func (s *CustomerSignalService) Get(ctx context.Context, tenantID int64, publicID string) (CustomerSignal, error)
func (s *CustomerSignalService) Create(ctx context.Context, tenantID, userID int64, input CustomerSignalCreateInput, auditCtx AuditContext) (CustomerSignal, error)
func (s *CustomerSignalService) Update(ctx context.Context, tenantID int64, publicID string, input CustomerSignalUpdateInput, auditCtx AuditContext) (CustomerSignal, error)
func (s *CustomerSignalService) Delete(ctx context.Context, tenantID int64, publicID string, auditCtx AuditContext) error
```

service 側の型は API response に依存させません。

```go
type CustomerSignal struct {
	PublicID     string
	CustomerName string
	Title        string
	Body         string
	Source       string
	Priority     string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CustomerSignalCreateInput struct {
	CustomerName string
	Title        string
	Body         string
	Source       string
	Priority     string
	Status       string
}

type CustomerSignalUpdateInput struct {
	CustomerName *string
	Title        *string
	Body         *string
	Source       *string
	Priority     *string
	Status       *string
}
```

validation は service に寄せます。

- `customerName`: trim 後 1-120 文字
- `title`: trim 後 1-200 文字
- `body`: trim 後 0-4000 文字
- `source`: `support` / `sales` / `customer_success` / `research` / `internal` / `other`
- `priority`: `low` / `medium` / `high` / `urgent`
- `status`: `new` / `triaged` / `planned` / `closed`
- update は 1 field 以上必須
- `publicID` は UUID として parse する

mutation は `TodoService` や `TenantAdminService` と同じく transaction に入れます。

```go
tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
if err != nil {
	return CustomerSignal{}, fmt.Errorf("begin customer signal transaction: %w", err)
}
defer func() {
	_ = tx.Rollback(context.Background())
}()

qtx := s.queries.WithTx(tx)
```

create/update/delete の audit event は同じ transaction で記録します。

```go
auditCtx.TenantID = &tenantID
if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
	AuditContext: auditCtx,
	Action:       "customer_signal.create",
	TargetType:   "customer_signal",
	TargetID:     item.PublicID,
	Metadata: map[string]any{
		"titleLength": len([]rune(input.Title)),
		"bodyLength":  len([]rune(input.Body)),
		"source":      item.Source,
		"priority":    item.Priority,
		"status":      item.Status,
	},
}); err != nil {
	return CustomerSignal{}, err
}
```

update metadata は `changedFields`、delete metadata は `status` 程度に留めます。`customerName` や `body` の全文は audit metadata に入れません。

## Step 5. Customer Signals API を追加する

#### ファイル: `backend/internal/api/customer_signals.go`

API は browser session + active tenant + tenant role の経路に限定します。M2M / external bearer は P6 初期版では扱いません。

追加する endpoint は次です。

| Method | Path | Operation ID | 用途 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/customer-signals` | `listCustomerSignals` | active tenant の signal list |
| `POST` | `/api/v1/customer-signals` | `createCustomerSignal` | signal 作成 |
| `GET` | `/api/v1/customer-signals/{signalPublicId}` | `getCustomerSignal` | signal detail |
| `PATCH` | `/api/v1/customer-signals/{signalPublicId}` | `updateCustomerSignal` | signal 更新 |
| `DELETE` | `/api/v1/customer-signals/{signalPublicId}` | `deleteCustomerSignal` | signal soft delete |

response body は frontend が一覧と詳細で同じ型を使えるようにします。

```go
type CustomerSignalBody struct {
	PublicID     string    `json:"publicId" format:"uuid"`
	CustomerName string    `json:"customerName" example:"Acme"`
	Title        string    `json:"title" example:"Export CSV from reports"`
	Body         string    `json:"body" example:"Customer asked for monthly report export."`
	Source       string    `json:"source" enum:"support,sales,customer_success,research,internal,other"`
	Priority     string    `json:"priority" enum:"low,medium,high,urgent"`
	Status       string    `json:"status" enum:"new,triaged,planned,closed"`
	CreatedAt    time.Time `json:"createdAt" format:"date-time"`
	UpdatedAt    time.Time `json:"updatedAt" format:"date-time"`
}
```

mutation input では CSRF header を必須にします。

```go
type CreateCustomerSignalInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          CreateCustomerSignalBody
}
```

active tenant と role check は TODO と同じ形で helper にします。

```go
func requireCustomerSignalTenant(ctx context.Context, deps Dependencies, sessionID, csrfToken string) (service.CurrentSession, service.TenantAccess, error) {
	if deps.CustomerSignalService == nil {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error503ServiceUnavailable("customer signal service is not configured")
	}

	var current service.CurrentSession
	var authCtx service.AuthContext
	var err error
	if csrfToken == "" {
		current, authCtx, err = currentSessionAuthContext(ctx, deps, sessionID)
	} else {
		current, authCtx, err = currentSessionAuthContextWithCSRF(ctx, deps, sessionID, csrfToken)
	}
	if err != nil {
		return service.CurrentSession{}, service.TenantAccess{}, toHTTPError(err)
	}
	if authCtx.ActiveTenant == nil {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error409Conflict("active tenant is required")
	}
	if !tenantHasRole(*authCtx.ActiveTenant, "customer_signal_user") {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error403Forbidden("customer_signal_user tenant role is required")
	}
	return current, *authCtx.ActiveTenant, nil
}
```

error mapping は service error を HTTP status に変換します。

- invalid input: `400`
- not found / tenant 外: `404`
- no active tenant: `409`
- role 不足: `403`
- 未ログイン: `401`

## Step 6. backend wiring と生成物を更新する

### 6-1. Dependencies に service を追加する

#### ファイル: `backend/internal/api/register.go`

```go
type Dependencies struct {
	// ...
	CustomerSignalService *service.CustomerSignalService
	// ...
}
```

`Register` で route を接続します。TODO の前後どちらでも構いませんが、domain API 同士が近いほうが読みやすいです。

```go
registerCustomerSignalRoutes(api, deps)
registerTodoRoutes(api, deps)
```

### 6-2. app wiring を更新する

#### ファイル: `backend/internal/app/app.go`

`New` の引数と `api.Dependencies` に `CustomerSignalService` を追加します。

#### ファイル: `backend/cmd/main/main.go`

runtime service を作ります。

```go
customerSignalService := service.NewCustomerSignalService(pool, queries, auditService)
```

`app.New(...)` に渡します。

#### ファイル: `backend/cmd/openapi/main.go`

OpenAPI export 用にも service を渡します。

```go
customerSignalService := service.NewCustomerSignalService(nil, nil, auditService)
```

### 6-3. tests の wiring を更新する

#### ファイル: `backend/internal/app/metrics_test.go`

`app.New` の引数が増えるため、test 側の呼び出しも更新します。

### 6-4. 生成する

```bash
make gen
```

期待する生成物は次です。

- `backend/internal/db/customer_signals.sql.go`
- `openapi/openapi.yaml`
- `frontend/src/api/generated/index.ts`
- `frontend/src/api/generated/sdk.gen.ts`
- `frontend/src/api/generated/types.gen.ts`

## Step 7. frontend API wrapper と store を追加する

### 7-1. generated SDK wrapper を追加する

#### ファイル: `frontend/src/api/customer-signals.ts`

generated SDK を直接 view から呼ばず、wrapper を挟みます。

```ts
import { readCookie } from './client'
import {
  createCustomerSignal,
  deleteCustomerSignal,
  getCustomerSignal,
  listCustomerSignals,
  updateCustomerSignal,
} from './generated/sdk.gen'
import type {
  CreateCustomerSignalBodyWritable,
  CustomerSignalBody,
  UpdateCustomerSignalBodyWritable,
} from './generated/types.gen'

export async function fetchCustomerSignals(): Promise<CustomerSignalBody[]> {
  const data = await listCustomerSignals({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items?: CustomerSignalBody[] | null }

  return data.items ?? []
}

export async function fetchCustomerSignal(signalPublicId: string): Promise<CustomerSignalBody> {
  return getCustomerSignal({
    path: { signalPublicId },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<CustomerSignalBody>
}

export async function createCustomerSignalItem(body: CreateCustomerSignalBodyWritable): Promise<CustomerSignalBody> {
  return createCustomerSignal({
    headers: { 'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '' },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<CustomerSignalBody>
}

export async function updateCustomerSignalItem(
  signalPublicId: string,
  body: UpdateCustomerSignalBodyWritable,
): Promise<CustomerSignalBody> {
  return updateCustomerSignal({
    headers: { 'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '' },
    path: { signalPublicId },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<CustomerSignalBody>
}

export async function deleteCustomerSignalItem(signalPublicId: string): Promise<void> {
  await deleteCustomerSignal({
    headers: { 'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '' },
    path: { signalPublicId },
    responseStyle: 'data',
    throwOnError: true,
  })
}
```

### 7-2. Pinia store を追加する

#### ファイル: `frontend/src/stores/customer-signals.ts`

状態は TODO store と同じ粒度で始めます。

```ts
type CustomerSignalStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'forbidden' | 'error'
```

store が持つ責務は次です。

- list を読み込む
- detail を読み込む
- create/update/delete を実行する
- `403` を `forbidden` に寄せる
- tenant switch 時に reset できる

`isApiForbidden(error)` と `toApiErrorMessage(error)` は既存の `frontend/src/api/client.ts` を使います。

### 7-3. role 不足判定を更新する

#### ファイル: `frontend/src/api/client.ts`

`customer_signal_user` を frontend の forbidden 判定に追加します。

```ts
return /machine_client_admin|tenant_admin|todo_user|customer_signal_user/.test(message)
```

## Step 8. Customer Signals UI を追加する

### 8-1. 一覧画面を追加する

#### ファイル: `frontend/src/views/CustomerSignalsView.vue`

この画面では active tenant の signal list と作成 form を出します。

UI 方針:

- SaaS の作業画面なので hero や landing page にしない
- `panel`、`admin-table`、`field-input`、`primary-button`、`secondary-button` を使う
- active tenant、件数、status 内訳を上部に表示する
- create form は customer name、title、source、priority、body を入力できる
- list は table にする
- status / priority は chip で視認できるようにする
- role 不足時は `AdminAccessDenied` を表示する

role 不足 UI は次のようにします。

```vue
<AdminAccessDenied
  v-if="signalStore.status === 'forbidden'"
  title="Customer Signals role required"
  message="この画面を使うには active tenant の customer_signal_user role が必要です。"
  role-label="customer_signal_user"
/>
```

tenant switch への追従は TODO と同じです。

```ts
watch(
  () => tenantStore.activeTenant?.slug,
  async (slug) => {
    signalStore.reset()
    if (slug) {
      await signalStore.load()
    }
  },
  { immediate: true },
)
```

### 8-2. 詳細 / 編集画面を追加する

#### ファイル: `frontend/src/views/CustomerSignalDetailView.vue`

detail では次を扱います。

- customer name
- title
- body
- source
- priority
- status
- save
- delete

delete は `ConfirmActionDialog` を使います。`window.confirm` は使いません。

```vue
<ConfirmActionDialog
  v-model:open="deleteDialogOpen"
  title="Delete customer signal"
  confirm-label="Delete"
  @confirm="deleteSignal"
>
  <p>この signal を削除します。audit log は残ります。</p>
</ConfirmActionDialog>
```

delete 成功後は `/customer-signals` に戻します。

### 8-3. router と nav を追加する

#### ファイル: `frontend/src/router/index.ts`

```ts
{
  path: '/customer-signals',
  name: 'customer-signals',
  component: () => import('../views/CustomerSignalsView.vue'),
},
{
  path: '/customer-signals/:signalPublicId',
  name: 'customer-signal-detail',
  component: () => import('../views/CustomerSignalDetailView.vue'),
},
```

#### ファイル: `frontend/src/App.vue`

main nav に `Signals` を追加します。

```vue
<RouterLink to="/customer-signals">Signals</RouterLink>
```

### 8-4. style を追加する

#### ファイル: `frontend/src/style.css`

既存 class を優先し、不足分だけ追加します。

候補:

- `.status-chip`
- `.priority-chip`
- `.signal-form-grid`
- `.signal-detail-grid`
- `.textarea-input`

色は既存 palette に寄せます。顧客名、本文、button text が狭い viewport で重ならないよう、grid は mobile で 1 column に落とします。

## Step 9. smoke / audit / metrics を確認する

### 9-1. smoke script を追加する

#### ファイル: `scripts/smoke-customer-signals.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
COOKIE_JAR="$(mktemp)"
trap 'rm -f "$COOKIE_JAR"' EXIT

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"changeme123"}' \
  "$BASE_URL/api/v1/login" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/csrf" >/dev/null

csrf="$(awk '$6 == "XSRF-TOKEN" { print $7 }' "$COOKIE_JAR" | tail -n 1)"
if [[ -z "$csrf" ]]; then
  echo "missing csrf token" >&2
  exit 1
fi

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d '{"tenantSlug":"acme"}' \
  "$BASE_URL/api/v1/session/tenant" >/dev/null

title="P6 smoke signal $(date +%s)-$$"

created="$(
  curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -H 'Content-Type: application/json' \
    -H "X-CSRF-Token: $csrf" \
    -d "{\"customerName\":\"Acme\",\"title\":\"$title\",\"body\":\"Smoke test body\",\"source\":\"support\",\"priority\":\"high\",\"status\":\"new\"}" \
    "$BASE_URL/api/v1/customer-signals"
)"

echo "$created" | rg "\"title\":\"$title\"" >/dev/null
signal_public_id="$(echo "$created" | sed -n 's/.*"publicId":"\([^"]*\)".*/\1/p')"
if [[ -z "$signal_public_id" ]]; then
  echo "missing signal public id" >&2
  exit 1
fi

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/customer-signals" | rg "\"title\":\"$title\"" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/customer-signals/$signal_public_id" | rg "\"publicId\":\"$signal_public_id\"" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d '{"status":"triaged","priority":"urgent"}' \
  "$BASE_URL/api/v1/customer-signals/$signal_public_id" | rg '"status":"triaged"' >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H "X-CSRF-Token: $csrf" \
  -X DELETE \
  "$BASE_URL/api/v1/customer-signals/$signal_public_id" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/customer-signals" | rg -v "\"publicId\":\"$signal_public_id\"" >/dev/null

echo "customer-signals smoke ok: $BASE_URL"
```

#### ファイル: `Makefile`

```make
smoke-customer-signals:
	bash scripts/smoke-customer-signals.sh
```

### 9-2. audit event を確認する

smoke 後に `audit_events` を確認します。

```bash
docker-compose exec -T postgres psql -U haohao -d haohao -c "
SELECT action, target_type, target_id, metadata
FROM audit_events
WHERE action LIKE 'customer_signal%'
ORDER BY id DESC
LIMIT 20;
"
```

期待する event は少なくとも次です。

- `customer_signal.create`
- `customer_signal.update`
- `customer_signal.delete`

metadata に `body` 全文や `customerName` 全文が入っていないことも確認します。

### 9-3. metrics を確認する

`make smoke-observability` と P6 smoke を実行した後に metrics を見ます。

```bash
curl -sS http://127.0.0.1:8080/metrics \
  | rg 'haohao_http_requests_total.*customer-signals|haohao_http_request_duration_seconds.*customer-signals'
```

期待すること:

- route label が `/api/v1/customer-signals` や `/api/v1/customer-signals/{signalPublicId}` のような template になる
- tenant slug、customer name、public id が metrics label に入らない

## Step 10. build と browser で確認する

### 10-1. backend / frontend の通常確認

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
make binary
```

### 10-2. local dev server で確認する

terminal 1:

```bash
make backend-dev
```

terminal 2:

```bash
make frontend-dev
```

browser で `http://127.0.0.1:5173` を開きます。

確認項目:

- `demo@example.com` / `changeme123` で login できる
- nav に `Signals` が表示される
- Acme を active tenant にすると `/customer-signals` が表示される
- signal を作成できる
- detail で status / priority / body を更新できる
- delete で確認 dialog が出る
- delete 後に list から消える
- Beta に切り替えると Acme の signal が表示されない
- role 不足時は 403 JSON ではなく `AdminAccessDenied` が表示される

### 10-3. single binary で確認する

```bash
make binary
AUTH_MODE=local ENABLE_LOCAL_PASSWORD_LOGIN=true HTTP_PORT=8080 ./bin/haohao
```

別 terminal で smoke を実行します。

```bash
make smoke-operability
make smoke-observability
make smoke-tenant-admin
make smoke-customer-signals
```

browser で `http://127.0.0.1:8080/customer-signals` を開き、SPA fallback でも Signals 画面が表示されることを確認します。

## 実装時の注意点

### tenant 条件を query から外さない

Customer Signals は tenant データです。`public_id` が分かっていても、`tenant_id` なしで detail / update / delete してはいけません。

正しい query は次の形です。

```sql
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
```

`public_id` だけで更新できる query は作りません。

### audit metadata に自由入力全文を入れない

Customer Signal には顧客名、問い合わせ本文、商談メモなどが入ります。audit metadata は調査補助用であり、業務本文のコピー先ではありません。

入れてよい例:

- `changedFields`
- `source`
- `priority`
- `status`
- `titleLength`
- `bodyLength`

避ける例:

- `customerName`
- `title`
- `body`
- email 全文
- token / password / raw session id

### role 不足と active tenant 不足を分ける

`customer_signal_user` が無い場合は `403` です。active tenant が無い場合は `409` です。

この区別があると、frontend で次を出し分けできます。

- `403`: tenant admin に role 付与を依頼する
- `409`: tenant selector / membership seed / login 状態を確認する

### delete は UI 上では削除、DB 上では soft delete

利用者にとっては delete ですが、DB では `deleted_at` を入れます。list/detail からは消える一方で、audit event と DB record は調査可能な形で残ります。

将来 hard delete が必要になった場合は、retention policy と別 job として実装します。

### tenant 管理 UI の role option 更新を忘れない

`supportedTenantRoles` に追加しても、P5 の UI に role option が出なければ browser から grant できません。

P6 では次の 3 箇所をセットで更新します。

- `roles` seed / migration
- `supportedTenantRoles`
- `TenantAdminTenantDetailView.vue` の role option

### generated SDK を手で編集しない

`frontend/src/api/generated/*` は OpenAPI 由来の生成物です。型や SDK のずれを手修正で直さず、Huma API struct と `make gen` で直します。

## 最終確認チェックリスト

- `customer_signal_user` role が migration / seed に入っている
- `supportedTenantRoles` に `customer_signal_user` が入っている
- tenant 管理 UI の grant option に `customer_signal_user` が出る
- `customer_signals` table が tenant-aware で追加されている
- list/detail/update/delete query が `tenant_id` と `deleted_at IS NULL` を条件にしている
- create/update/delete が transaction 内で audit event を保存する
- audit metadata に顧客名や本文全文が入っていない
- `GET /api/v1/customer-signals` が active tenant の record だけを返す
- `POST /api/v1/customer-signals` が CSRF 必須で作成できる
- `GET /api/v1/customer-signals/{signalPublicId}` が detail を返す
- `PATCH /api/v1/customer-signals/{signalPublicId}` が更新できる
- `DELETE /api/v1/customer-signals/{signalPublicId}` が soft delete する
- role 不足時は frontend で `AdminAccessDenied` が表示される
- `/customer-signals` と detail route が single binary の SPA fallback で表示される
- `make gen` が通る
- `go test ./backend/...` が通る
- `npm --prefix frontend run build` が通る
- `make binary` が通る
- `make smoke-operability` が通る
- `make smoke-observability` が通る
- `make smoke-tenant-admin` が通る
- `make smoke-customer-signals` が通る
