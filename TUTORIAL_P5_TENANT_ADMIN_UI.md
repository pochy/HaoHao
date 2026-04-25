# P5 tenant 管理 UI 実装チュートリアル

## この文書の目的

この文書は、`deep-research-report.md` の **P5: tenant 管理 UI** を、現在の HaoHao に実装できる順番に分解したチュートリアルです。

P1 では app header に active tenant selector を追加し、P2 では tenant-aware TODO を実装しました。P3 では重要な状態変更を `audit_events` に残し、P4 では metrics / tracing / alerting を追加しました。P5 では、その上に **tenant 作成、tenant 更新、membership 管理、tenant role grant / revoke を browser から扱える管理 UI** を追加します。

このチュートリアルで扱う tenant 管理 UI は、既存の active tenant selector とは別の責務を持ちます。

- active tenant selector: ログイン中 user が「自分の作業対象 tenant」を切り替える UI
- tenant 管理 UI: tenant admin が「tenant 自体」と「user の tenant role」を管理する UI

既存の tenant selector は引き続き `/api/v1/tenants` と `/api/v1/session/tenant` だけを使います。P5 では admin 用 API を `/api/v1/admin/tenants` 以下に分け、通常 user 向けの tenant selector と混ぜないようにします。

## この文書が前提にしている現在地

このチュートリアルを始める前の repository は、少なくとも次の状態にある前提で進めます。

- `tenants`、`tenant_memberships`、`tenant_role_overrides` table が存在する
- `roles` には `docs_reader`、`external_api_user`、`machine_client_admin`、`todo_user` が存在する
- `GET /api/v1/tenants` と `POST /api/v1/session/tenant` が存在する
- `frontend/src/components/TenantSelector.vue` が app header に接続済み
- `frontend/src/api/client.ts` が Cookie / CSRF / generated SDK の共通 transport を持っている
- `frontend/src/components/AdminAccessDenied.vue` が 403 UI として使える
- `AuditService` が mutation と同じ transaction で audit event を保存できる
- `/metrics` と request log で P5 の API も観測できる
- `make gen` が sqlc / OpenAPI / frontend generated SDK をまとめて更新する
- `make binary` で single binary を作れる
- `make smoke-operability` と `make smoke-observability` が既存 server に対して確認できる

この P5 では、global role の管理 UI までは作りません。tenant 管理 UI を使える user は、先に DB seed、provider claim、または将来の user admin 画面で global role `tenant_admin` を持っている前提にします。

## 完成条件

このチュートリアルの完了条件は次です。

- global role `tenant_admin` が追加される
- `tenant_admin` を持つ user だけが tenant 管理 API / UI を使える
- `GET /api/v1/admin/tenants` で tenant list と member count を取得できる
- `POST /api/v1/admin/tenants` で tenant を作成できる
- `GET /api/v1/admin/tenants/{tenantSlug}` で tenant detail と membership を取得できる
- `PUT /api/v1/admin/tenants/{tenantSlug}` で display name / active を更新できる
- `DELETE /api/v1/admin/tenants/{tenantSlug}` で tenant を deactivate できる
- `POST /api/v1/admin/tenants/{tenantSlug}/memberships` で既存 user に tenant role を grant できる
- `DELETE /api/v1/admin/tenants/{tenantSlug}/memberships/{userPublicId}/roles/{roleCode}` で local tenant role を revoke できる
- provider claim / SCIM 由来の membership は UI 上で source が分かる
- provider claim / SCIM 由来の role は、P5 初期版では「source of truth 側で変更する」扱いにし、local override と混同しない
- tenant / membership / role 変更は `audit_events` に残る
- role 不足時は 403 の JSON 画面に飛ばず、frontend 上で `tenant_admin` が必要だと表示される
- active tenant selector と tenant 管理 UI の導線が分かれている
- `make gen`、`go test ./backend/...`、`npm --prefix frontend run build`、`make binary` が通る
- single binary を `:8080` で起動した状態で smoke と manual browser 確認ができる

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | `db/migrations/0010_tenant_admin_role.*.sql`, seed | `tenant_admin` role を導入する |
| Step 2 | `db/queries/tenant_admin.sql` | tenant 管理用 SQL を追加する |
| Step 3 | `backend/internal/service/tenant_admin_service.go` | tenant admin service と audit 方針を実装する |
| Step 4 | `backend/internal/api/tenant_admin.go` | Huma admin API を追加する |
| Step 5 | backend wiring / OpenAPI | service を runtime / OpenAPI export に接続する |
| Step 6 | `frontend/src/api/tenant-admin.ts`, store | generated SDK wrapper と Pinia store を追加する |
| Step 7 | `frontend/src/views/*`, router, nav | tenant 管理画面を追加する |
| Step 8 | smoke / audit 確認 | API と audit event を確認する |
| Step 9 | build / binary / browser 確認 | frontend と single binary で回帰を確認する |

## 先に決める方針

### `tenant_admin` は global role として始める

P5 初期版では、tenant 管理 UI の利用条件を global role `tenant_admin` にします。

理由は、tenant 作成や tenant deactivate は特定 tenant の中だけでは完結しない操作だからです。tenant-scoped な `tenant_admin` は将来、既存 tenant の membership だけを管理できるロールとして追加できますが、最初から混ぜると bootstrap と権限境界が曖昧になります。

追加する方針は次です。

- `tenant_admin` は `roles` table に追加する
- `AuthzService.HasRole("tenant_admin")` で API を保護する
- local dev の `demo@example.com` には seed で `tenant_admin` を付ける
- provider claim からも同期できるように `supportedGlobalRoles` に `tenant_admin` を追加する

### tenant は deactivate し、物理削除しない

tenant を物理削除すると、membership、OAuth grant、TODO、audit event との関係が大きく動きます。P5 では `DELETE /api/v1/admin/tenants/{tenantSlug}` を物理削除ではなく deactivate として扱います。

deactivate された tenant は次の挙動になります。

- tenant selector の候補から消える
- tenant-aware API の active tenant として使えない
- 過去の TODO / audit event は残る
- 管理 UI では inactive tenant として表示できる

### P5 初期版は local override を管理する

既存 schema の `tenant_memberships.source` は次の値を持ちます。

- `provider_claim`
- `scim`
- `local_override`

P5 初期版の UI で直接追加 / revoke するのは `local_override` です。`provider_claim` と `scim` は外部 source of truth 由来なので、画面には表示しますが、同じボタンで無条件に削除しません。

将来、外部由来 role をアプリ側で明示 deny したい場合は `tenant_role_overrides` の `effect = 'deny'` を使います。最初の P5 では事故を避けるため、UI 上は source を見せ、local override と provider-managed role を分けて扱います。

### audit metadata に secret や長い値を入れない

tenant 管理は監査対象です。ただし、audit metadata に email 全文、token、長い入力値、自由記述の error message を入れません。

P5 の audit event は、低感度で調査に必要な情報だけを残します。

- `tenant.create`
- `tenant.update`
- `tenant.deactivate`
- `tenant_role.grant`
- `tenant_role.revoke`

metadata は `changedFields`、`roleCode`、`source`、`displayNameLength` 程度にします。target user は `userPublicId` を target id に含め、email は入れません。

### UI は作業用画面として密度を保つ

P5 の画面は SaaS の管理作業画面です。新しい landing page や説明用 hero は作りません。

既存の `panel`、`admin-table`、`field-input`、`primary-button`、`secondary-button` を使い、次を守ります。

- list は table 中心にする
- loading / empty / forbidden / error を明確に分ける
- destructive action は確認を挟む
- error は操作した場所の近くに出す
- role source は chip / text で見分けられるようにする
- tenant selector と tenant 管理 UI を同じ component にしない

## Step 1. `tenant_admin` role を追加する

### 1-1. migration を追加する

#### ファイル: `db/migrations/0010_tenant_admin_role.up.sql`

```sql
INSERT INTO roles (code)
VALUES ('tenant_admin')
ON CONFLICT (code) DO NOTHING;
```

#### ファイル: `db/migrations/0010_tenant_admin_role.down.sql`

```sql
DELETE FROM user_roles
WHERE role_id IN (
    SELECT id
    FROM roles
    WHERE code = 'tenant_admin'
);

DELETE FROM roles
WHERE code = 'tenant_admin';
```

role 追加だけなので schema の table 定義は変わりません。ただし migration と seed の整合を保つため、`make db-up` 後に local seed も更新します。

### 1-2. local seed を更新する

#### ファイル: `scripts/seed-demo-user.sql`

roles seed に `tenant_admin` を追加します。

```sql
INSERT INTO roles (code)
VALUES
    ('docs_reader'),
    ('machine_client_admin'),
    ('tenant_admin'),
    ('todo_user')
ON CONFLICT (code) DO NOTHING;
```

`demo@example.com` にも global role を付けます。

```sql
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.code IN ('machine_client_admin', 'tenant_admin')
WHERE u.email = 'demo@example.com'
ON CONFLICT (user_id, role_id) DO NOTHING;
```

### 1-3. provider claim 同期対象に追加する

#### ファイル: `backend/internal/service/authz_service.go`

`supportedGlobalRoles` に `tenant_admin` を追加します。

```go
var supportedGlobalRoles = map[string]struct{}{
	"docs_reader":          {},
	"external_api_user":    {},
	"machine_client_admin": {},
	"tenant_admin":         {},
	"todo_user":            {},
}
```

この変更により、Zitadel などの provider group に `tenant_admin` が入っている場合も local `user_roles` に同期できます。

## Step 2. tenant 管理用 SQL を追加する

#### ファイル: `db/queries/tenant_admin.sql`

admin 用 query は既存の `db/queries/tenants.sql` と分けます。通常 user の tenant selector と、admin UI の一覧 / mutation を同じ query file に混ぜないためです。

```sql
-- name: ListTenantAdminTenants :many
SELECT
    t.id,
    t.slug,
    t.display_name,
    t.active,
    t.created_at,
    t.updated_at,
    COALESCE(COUNT(DISTINCT tm.user_id) FILTER (WHERE tm.active), 0)::bigint AS active_member_count
FROM tenants t
LEFT JOIN tenant_memberships tm ON tm.tenant_id = t.id
GROUP BY t.id
ORDER BY t.slug;

-- name: CreateTenantAdminTenant :one
INSERT INTO tenants (
    slug,
    display_name,
    active
) VALUES (
    $1,
    $2,
    true
)
RETURNING
    id,
    slug,
    display_name,
    active,
    created_at,
    updated_at;

-- name: UpdateTenantAdminTenant :one
UPDATE tenants
SET
    display_name = $2,
    active = $3,
    updated_at = now()
WHERE slug = $1
RETURNING
    id,
    slug,
    display_name,
    active,
    created_at,
    updated_at;

-- name: DeactivateTenantAdminTenant :one
UPDATE tenants
SET
    active = false,
    updated_at = now()
WHERE slug = $1
RETURNING
    id,
    slug,
    display_name,
    active,
    created_at,
    updated_at;

-- name: ListTenantAdminMembershipRows :many
SELECT
    u.id AS user_id,
    u.public_id AS user_public_id,
    u.email,
    u.display_name AS user_display_name,
    u.deactivated_at AS user_deactivated_at,
    t.id AS tenant_id,
    t.slug AS tenant_slug,
    t.display_name AS tenant_display_name,
    r.code AS role_code,
    tm.source,
    tm.active,
    tm.created_at,
    tm.updated_at
FROM tenant_memberships tm
JOIN users u ON u.id = tm.user_id
JOIN tenants t ON t.id = tm.tenant_id
JOIN roles r ON r.id = tm.role_id
WHERE t.slug = $1
ORDER BY u.email, r.code, tm.source;

-- name: UpsertTenantAdminLocalMembership :one
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
    'local_override',
    true
)
ON CONFLICT (user_id, tenant_id, role_id, source) DO UPDATE
SET active = true,
    updated_at = now()
RETURNING
    user_id,
    tenant_id,
    role_id,
    source,
    active,
    created_at,
    updated_at;

-- name: DeactivateTenantAdminLocalMembershipRole :execrows
UPDATE tenant_memberships
SET
    active = false,
    updated_at = now()
WHERE user_id = $1
  AND tenant_id = $2
  AND role_id = $3
  AND source = 'local_override'
  AND active = true;
```

ここでは provider / SCIM 由来の membership は更新しません。P5 初期版で admin UI から更新する source は `local_override` だけです。

query を追加したら sqlc を生成します。

```bash
cd backend && sqlc generate
```

## Step 3. `TenantAdminService` を追加する

#### ファイル: `backend/internal/service/tenant_admin_service.go`

tenant 管理の validation、transaction、audit event は service に寄せます。API handler は Cookie / CSRF / role check と DTO 変換だけを担当します。

公開 method は次の形にします。

```go
type TenantAdminService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	audit   AuditRecorder
}

func NewTenantAdminService(pool *pgxpool.Pool, queries *db.Queries, audit AuditRecorder) *TenantAdminService

func (s *TenantAdminService) ListTenants(ctx context.Context) ([]TenantAdminTenant, error)
func (s *TenantAdminService) GetTenant(ctx context.Context, tenantSlug string) (TenantAdminTenantDetail, error)
func (s *TenantAdminService) CreateTenant(ctx context.Context, input TenantAdminTenantInput, auditCtx AuditContext) (TenantAdminTenant, error)
func (s *TenantAdminService) UpdateTenant(ctx context.Context, tenantSlug string, input TenantAdminTenantInput, auditCtx AuditContext) (TenantAdminTenant, error)
func (s *TenantAdminService) DeactivateTenant(ctx context.Context, tenantSlug string, auditCtx AuditContext) (TenantAdminTenant, error)
func (s *TenantAdminService) GrantRole(ctx context.Context, tenantSlug string, input TenantRoleGrantInput, auditCtx AuditContext) (TenantAdminMembership, error)
func (s *TenantAdminService) RevokeLocalRole(ctx context.Context, tenantSlug, userPublicID, roleCode string, auditCtx AuditContext) error
```

domain type は API response に近い形にしておくと、frontend で扱いやすくなります。

```go
type TenantAdminTenant struct {
	ID                int64
	Slug              string
	DisplayName       string
	Active            bool
	ActiveMemberCount int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type TenantAdminTenantDetail struct {
	Tenant      TenantAdminTenant
	Memberships []TenantAdminMembership
}

type TenantAdminMembership struct {
	UserPublicID string
	Email        string
	DisplayName  string
	Deactivated  bool
	Roles        []TenantAdminRoleBinding
}

type TenantAdminRoleBinding struct {
	RoleCode string
	Source   string
	Active   bool
}

type TenantAdminTenantInput struct {
	Slug        string
	DisplayName string
	Active      *bool
}

type TenantRoleGrantInput struct {
	UserEmail string
	RoleCode  string
}
```

validation 方針は次です。

- slug は lowercase の `a-z0-9-` のみ
- slug は 3 文字以上 64 文字以下
- display name は trim 後 1 文字以上 120 文字以下
- role code は `supportedTenantRoles` に存在するものだけ
- grant 対象 user は既存 user に限定する
- deactivated user には新規 grant しない
- deactivate tenant は物理削除しない

`supportedTenantRoles` は `authz_service.go` の private map なので、P5 で次の helper を追加すると service から使いやすくなります。

```go
func IsSupportedTenantRole(roleCode string) bool {
	_, ok := supportedTenantRoles[strings.ToLower(strings.TrimSpace(roleCode))]
	return ok
}
```

### audit event の例

tenant 作成は mutation と audit event を同じ transaction に入れます。

```go
if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
	AuditContext: auditCtx,
	Action:       "tenant.create",
	TargetType:   "tenant",
	TargetID:     tenant.Slug,
	Metadata: map[string]any{
		"displayNameLength": len([]rune(tenant.DisplayName)),
	},
}); err != nil {
	return TenantAdminTenant{}, err
}
```

role grant は target user の public id と role を target id に含めます。

```go
targetID := tenant.Slug + ":" + user.PublicID.String() + ":" + role.Code
if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
	AuditContext: auditCtx,
	Action:       "tenant_role.grant",
	TargetType:   "tenant_role",
	TargetID:     targetID,
	Metadata: map[string]any{
		"roleCode": role.Code,
		"source":   "local_override",
	},
}); err != nil {
	return TenantAdminMembership{}, err
}
```

`auditCtx.TenantID` は対象 tenant の ID にします。これにより、後で tenant 単位の audit event を追いやすくなります。

## Step 4. Huma admin API を追加する

#### ファイル: `backend/internal/api/tenant_admin.go`

operation は admin namespace に分けます。

| Operation ID | Method / Path | 用途 |
| --- | --- | --- |
| `listTenantAdminTenants` | `GET /api/v1/admin/tenants` | tenant 一覧 |
| `createTenantAdminTenant` | `POST /api/v1/admin/tenants` | tenant 作成 |
| `getTenantAdminTenant` | `GET /api/v1/admin/tenants/{tenantSlug}` | tenant detail |
| `updateTenantAdminTenant` | `PUT /api/v1/admin/tenants/{tenantSlug}` | tenant 更新 |
| `deactivateTenantAdminTenant` | `DELETE /api/v1/admin/tenants/{tenantSlug}` | tenant deactivate |
| `grantTenantAdminRole` | `POST /api/v1/admin/tenants/{tenantSlug}/memberships` | tenant role grant |
| `revokeTenantAdminRole` | `DELETE /api/v1/admin/tenants/{tenantSlug}/memberships/{userPublicId}/roles/{roleCode}` | local tenant role revoke |

DTO は次のように始めます。

```go
type TenantAdminTenantBody struct {
	ID                int64     `json:"id" example:"1"`
	Slug              string    `json:"slug" example:"acme"`
	DisplayName       string    `json:"displayName" example:"Acme"`
	Active            bool      `json:"active" example:"true"`
	ActiveMemberCount int64     `json:"activeMemberCount" example:"3"`
	CreatedAt         time.Time `json:"createdAt" format:"date-time"`
	UpdatedAt         time.Time `json:"updatedAt" format:"date-time"`
}

type TenantAdminRoleBindingBody struct {
	RoleCode string `json:"roleCode" example:"todo_user"`
	Source   string `json:"source" example:"local_override"`
	Active   bool   `json:"active" example:"true"`
}

type TenantAdminMembershipBody struct {
	UserPublicID string                         `json:"userPublicId" format:"uuid"`
	Email        string                         `json:"email" format:"email"`
	DisplayName  string                         `json:"displayName" example:"Demo User"`
	Deactivated  bool                           `json:"deactivated" example:"false"`
	Roles        []TenantAdminRoleBindingBody   `json:"roles"`
}
```

`requireTenantAdmin()` は machine client admin と同じ形にします。

```go
func requireTenantAdmin(ctx context.Context, deps Dependencies, sessionID, csrfToken string) (service.CurrentSession, error) {
	var current service.CurrentSession
	var authCtx service.AuthContext
	var err error
	if csrfToken == "" {
		current, authCtx, err = currentSessionAuthContext(ctx, deps, sessionID)
	} else {
		current, authCtx, err = currentSessionAuthContextWithCSRF(ctx, deps, sessionID, csrfToken)
	}
	if err != nil {
		return service.CurrentSession{}, toHTTPError(err)
	}
	if !authCtx.HasRole("tenant_admin") {
		return service.CurrentSession{}, huma.Error403Forbidden("tenant_admin role is required")
	}
	return current, nil
}
```

mutating operation では必ず CSRF token を要求します。

```go
type CreateTenantAdminTenantInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          TenantAdminTenantRequestBody
}
```

`tenantSlug` は path parameter として受け取ります。

```go
type TenantAdminBySlugInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
}
```

エラー変換は service error を明示的に mapping します。

```go
func toTenantAdminHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrTenantAdminInvalidInput):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, service.ErrTenantAdminTenantNotFound):
		return huma.Error404NotFound("tenant not found")
	case errors.Is(err, service.ErrTenantAdminUserNotFound):
		return huma.Error404NotFound("user not found")
	case errors.Is(err, service.ErrTenantAdminRoleNotFound):
		return huma.Error400BadRequest("unsupported tenant role")
	default:
		return huma.Error500InternalServerError("internal server error")
	}
}
```

## Step 5. backend wiring と OpenAPI を更新する

### 5-1. Dependencies に service を追加する

#### ファイル: `backend/internal/api/register.go`

```go
type Dependencies struct {
	SessionService       *service.SessionService
	// ...
	TenantAdminService   *service.TenantAdminService
}
```

`Register()` では tenant selector の後に admin route を登録します。

```go
registerTenantRoutes(api, deps)
registerTenantAdminRoutes(api, deps)
registerTodoRoutes(api, deps)
```

### 5-2. app wiring を追加する

#### ファイル: `backend/internal/app/app.go`

`app.New()` の引数に `tenantAdminService *service.TenantAdminService` を追加し、`backendapi.Dependencies` に渡します。

### 5-3. main wiring を追加する

#### ファイル: `backend/cmd/main/main.go`

`auditService`、`authzService` と同じ場所で service を作ります。

```go
tenantAdminService := service.NewTenantAdminService(pool, queries, auditService)
```

`app.New()` に渡します。

### 5-4. openapi export 側も忘れずに更新する

#### ファイル: `backend/cmd/openapi/main.go`

OpenAPI export は実 DB / Redis を使わないため、nil service の扱いに注意します。既存の pattern に合わせて `api.Register()` の dependencies に `TenantAdminService` field を追加します。

### 5-5. 生成する

```bash
make gen
```

この時点で、少なくとも次が更新されます。

- `backend/internal/db/tenant_admin.sql.go`
- `openapi/openapi.yaml`
- `frontend/src/api/generated/*`

OpenAPI に admin operation が出ていることを確認します。

```bash
rg -n "listTenantAdminTenants|grantTenantAdminRole|/api/v1/admin/tenants" openapi/openapi.yaml
```

## Step 6. frontend API wrapper と store を追加する

### 6-1. generated SDK wrapper を追加する

#### ファイル: `frontend/src/api/tenant-admin.ts`

view から generated SDK を直接呼ばず、既存の `machine-clients.ts` と同じ wrapper 層を作ります。

```ts
import { readCookie } from './client'
import {
  createTenantAdminTenant,
  deactivateTenantAdminTenant,
  getTenantAdminTenant,
  grantTenantAdminRole,
  listTenantAdminTenants,
  revokeTenantAdminRole,
  updateTenantAdminTenant,
} from './generated/sdk.gen'
import type {
  TenantAdminMembershipRequestBody,
  TenantAdminTenantBody,
  TenantAdminTenantDetailBody,
  TenantAdminTenantRequestBody,
} from './generated/types.gen'

function csrfHeaders() {
  return {
    'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
  }
}

export async function fetchTenantAdminTenants(): Promise<TenantAdminTenantBody[]> {
  const data = await listTenantAdminTenants({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items: TenantAdminTenantBody[] | null }

  return data.items ?? []
}

export async function fetchTenantAdminTenant(tenantSlug: string): Promise<TenantAdminTenantDetailBody> {
  return getTenantAdminTenant({
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantAdminTenantDetailBody>
}

export async function createTenantFromForm(body: TenantAdminTenantRequestBody): Promise<TenantAdminTenantBody> {
  return createTenantAdminTenant({
    headers: csrfHeaders(),
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantAdminTenantBody>
}

export async function updateTenantFromForm(
  tenantSlug: string,
  body: TenantAdminTenantRequestBody,
): Promise<TenantAdminTenantBody> {
  return updateTenantAdminTenant({
    headers: csrfHeaders(),
    path: { tenantSlug },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TenantAdminTenantBody>
}

export async function deactivateTenant(tenantSlug: string): Promise<void> {
  await deactivateTenantAdminTenant({
    headers: csrfHeaders(),
    path: { tenantSlug },
    responseStyle: 'data',
    throwOnError: true,
  })
}

export async function grantTenantRole(
  tenantSlug: string,
  body: TenantAdminMembershipRequestBody,
): Promise<void> {
  await grantTenantAdminRole({
    headers: csrfHeaders(),
    path: { tenantSlug },
    body,
    responseStyle: 'data',
    throwOnError: true,
  })
}

export async function revokeTenantRole(
  tenantSlug: string,
  userPublicId: string,
  roleCode: string,
): Promise<void> {
  await revokeTenantAdminRole({
    headers: csrfHeaders(),
    path: { tenantSlug, userPublicId, roleCode },
    responseStyle: 'data',
    throwOnError: true,
  })
}
```

生成された型名は Huma DTO 名に依存します。実装時は `frontend/src/api/generated/types.gen.ts` を見て、必要なら import 名を合わせます。

### 6-2. Pinia store を追加する

#### ファイル: `frontend/src/stores/tenant-admin.ts`

既存の `machine-clients.ts` と同じ status model にします。

```ts
import { defineStore } from 'pinia'

import { isApiForbidden, toApiErrorMessage } from '../api/client'
import {
  createTenantFromForm,
  deactivateTenant,
  fetchTenantAdminTenant,
  fetchTenantAdminTenants,
  grantTenantRole,
  revokeTenantRole,
  updateTenantFromForm,
} from '../api/tenant-admin'
import type {
  TenantAdminTenantBody,
  TenantAdminTenantDetailBody,
  TenantAdminTenantRequestBody,
  TenantAdminMembershipRequestBody,
} from '../api/generated/types.gen'

type TenantAdminStatus = 'idle' | 'loading' | 'ready' | 'forbidden' | 'error'

export const useTenantAdminStore = defineStore('tenantAdmin', {
  state: () => ({
    status: 'idle' as TenantAdminStatus,
    items: [] as TenantAdminTenantBody[],
    current: null as TenantAdminTenantDetailBody | null,
    errorMessage: '',
    saving: false,
  }),

  actions: {
    async loadList() {
      this.status = 'loading'
      this.errorMessage = ''
      try {
        this.items = await fetchTenantAdminTenants()
        this.status = 'ready'
      } catch (error) {
        this.items = []
        this.status = isApiForbidden(error) ? 'forbidden' : 'error'
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async loadOne(tenantSlug: string) {
      this.status = 'loading'
      this.errorMessage = ''
      try {
        this.current = await fetchTenantAdminTenant(tenantSlug)
        this.status = 'ready'
      } catch (error) {
        this.current = null
        this.status = isApiForbidden(error) ? 'forbidden' : 'error'
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async create(body: TenantAdminTenantRequestBody) {
      this.saving = true
      this.errorMessage = ''
      try {
        return await createTenantFromForm(body)
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },

    async update(tenantSlug: string, body: TenantAdminTenantRequestBody) {
      this.saving = true
      this.errorMessage = ''
      try {
        return await updateTenantFromForm(tenantSlug, body)
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },

    async deactivate(tenantSlug: string) {
      this.saving = true
      this.errorMessage = ''
      try {
        await deactivateTenant(tenantSlug)
        await this.loadList()
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },

    async grantRole(tenantSlug: string, body: TenantAdminMembershipRequestBody) {
      this.saving = true
      this.errorMessage = ''
      try {
        await grantTenantRole(tenantSlug, body)
        await this.loadOne(tenantSlug)
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },

    async revokeRole(tenantSlug: string, userPublicId: string, roleCode: string) {
      this.saving = true
      this.errorMessage = ''
      try {
        await revokeTenantRole(tenantSlug, userPublicId, roleCode)
        await this.loadOne(tenantSlug)
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },
  },
})
```

## Step 7. tenant 管理 UI を追加する

### 7-1. tenant list view を追加する

#### ファイル: `frontend/src/views/TenantAdminTenantsView.vue`

画面は tenant 一覧を first view にします。marketing copy ではなく、すぐ作業できる table にします。

表示項目は次です。

- tenant display name
- slug
- active / inactive
- active member count
- updated at
- detail link

`store.status === 'forbidden'` のときは既存 component を使います。

```vue
<AdminAccessDenied
  v-if="store.status === 'forbidden'"
  title="Tenant admin role required"
  message="この画面を使うには global role tenant_admin が必要です。"
  role-label="tenant_admin"
/>
```

empty state には 1 つだけ明確な action を置きます。

```vue
<div v-else-if="store.status === 'ready' && store.items.length === 0" class="empty-state">
  <p>Tenant はまだ登録されていません。</p>
  <RouterLink class="primary-button link-button" to="/tenant-admin/new">
    New Tenant
  </RouterLink>
</div>
```

### 7-2. tenant form view を追加する

#### ファイル: `frontend/src/views/TenantAdminTenantFormView.vue`

`/tenant-admin/new` で tenant を作成します。

入力は最小にします。

- slug
- display name

slug は作成後に変更しません。backend service でも update 時は slug を変更しない設計にします。

### 7-3. tenant detail view を追加する

#### ファイル: `frontend/src/views/TenantAdminTenantDetailView.vue`

detail view は 3 つの領域に分けます。

- tenant settings: display name / active update
- membership grant form: user email + role code
- membership table: user / role / source / active / action

role option は P5 初期版では固定値で十分です。

```ts
const tenantRoleOptions = ['docs_reader', 'todo_user']
```

role binding の source が `local_override` のものだけ revoke button を有効にします。

```vue
<button
  class="secondary-button"
  type="button"
  :disabled="role.source !== 'local_override' || store.saving"
  @click="confirmRevoke(member.userPublicId, role.roleCode)"
>
  Revoke
</button>
```

`provider_claim` や `scim` の role は表示だけにし、source of truth 側で変更する導線を明記します。

```vue
<span v-if="role.source !== 'local_override'" class="cell-subtle">
  Managed by {{ role.source }}
</span>
```

tenant deactivate と role revoke は destructive action なので、再利用できる確認 dialog を挟みます。P5 では `frontend/src/components/ConfirmActionDialog.vue` を追加し、native `<dialog>` または導入済みの accessible primitive を使って keyboard / focus の挙動を任せます。`window.confirm()` は画面内の error 表示や文脈説明を置きにくいため使いません。

dialog には次を表示します。

- 実行する操作名
- 対象 tenant slug
- 対象 user / role
- 操作後に戻す方法
- `Cancel` と destructive action の明確な button

### 7-4. router を追加する

#### ファイル: `frontend/src/router/index.ts`

```ts
import TenantAdminTenantDetailView from '../views/TenantAdminTenantDetailView.vue'
import TenantAdminTenantFormView from '../views/TenantAdminTenantFormView.vue'
import TenantAdminTenantsView from '../views/TenantAdminTenantsView.vue'
```

routes を追加します。

```ts
{
  path: '/tenant-admin',
  name: 'tenant-admin',
  component: TenantAdminTenantsView,
  meta: { requiresAuth: true },
},
{
  path: '/tenant-admin/new',
  name: 'tenant-admin-new',
  component: TenantAdminTenantFormView,
  meta: { requiresAuth: true },
},
{
  path: '/tenant-admin/:tenantSlug',
  name: 'tenant-admin-detail',
  component: TenantAdminTenantDetailView,
  meta: { requiresAuth: true },
},
```

### 7-5. app nav に追加する

#### ファイル: `frontend/src/App.vue`

primary nav に tenant 管理画面への導線を追加します。

```vue
<RouterLink to="/tenant-admin">Tenants</RouterLink>
```

active tenant selector はそのまま header tool として残します。tenant 管理 UI の list/detail に selector の責務を移しません。

### 7-6. style を最小追加する

#### ファイル: `frontend/src/style.css`

既存の `panel`、`admin-table`、`status-pill`、`error-message` を優先して使います。足りない場合だけ小さく追加します。

```css
.source-chip {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 0 10px;
  border: 1px solid var(--border);
  border-radius: 999px;
  color: var(--muted);
  background: rgba(255, 255, 255, 0.62);
  font-size: 0.82rem;
}

.source-chip.local {
  color: var(--accent-strong);
  border-color: rgba(11, 93, 91, 0.28);
}
```

新しい色や大きな装飾は増やしません。

## Step 8. smoke と audit を確認する

### 8-1. migration と seed を流す

```bash
make up
make db-up
make seed-demo-user
```

`demo@example.com` に `tenant_admin` が付いていることを確認します。

```bash
docker-compose exec -T postgres psql -U haohao -d haohao -c "
SELECT u.email, r.code
FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN roles r ON r.id = ur.role_id
WHERE u.email = 'demo@example.com'
ORDER BY r.code;
"
```

### 8-2. API smoke script を追加する

#### ファイル: `scripts/smoke-tenant-admin.sh`

local password login で admin API を叩きます。既存 server が `:8080` で起動している前提にします。

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

tenant_slug="p5-smoke-$(date +%s)"

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d "{\"slug\":\"$tenant_slug\",\"displayName\":\"P5 Smoke\"}" \
  "$BASE_URL/api/v1/admin/tenants" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/admin/tenants/$tenant_slug" | rg "\"slug\":\"$tenant_slug\"" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $csrf" \
  -d '{"userEmail":"demo@example.com","roleCode":"todo_user"}' \
  "$BASE_URL/api/v1/admin/tenants/$tenant_slug/memberships" >/dev/null

curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE_URL/api/v1/admin/tenants/$tenant_slug" | rg '"roleCode":"todo_user"' >/dev/null

echo "tenant-admin smoke ok: $BASE_URL"
```

#### ファイル: `Makefile`

```make
smoke-tenant-admin:
	bash scripts/smoke-tenant-admin.sh
```

### 8-3. audit event を確認する

smoke 後に `audit_events` を確認します。

```bash
docker-compose exec -T postgres psql -U haohao -d haohao -c "
SELECT action, target_type, target_id, metadata
FROM audit_events
WHERE action LIKE 'tenant%'
ORDER BY id DESC
LIMIT 20;
"
```

期待する event は少なくとも次です。

- `tenant.create`
- `tenant_role.grant`

## Step 9. build と browser で確認する

### 9-1. backend / frontend の通常確認

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
make binary
```

### 9-2. local dev server で確認する

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
- nav に `Tenants` が表示される
- `/tenant-admin` で tenant list が表示される
- `New Tenant` から tenant を作成できる
- detail で display name を変更できる
- `demo@example.com` に `todo_user` を grant できる
- local override role を revoke できる
- tenant を deactivate すると list 上で inactive になる
- `/api/v1/tenants` の active tenant selector には inactive tenant が出ない

### 9-3. role 不足 UI を確認する

DB で一時的に `tenant_admin` を外します。

```bash
docker-compose exec -T postgres psql -U haohao -d haohao -c "
DELETE FROM user_roles ur
USING users u, roles r
WHERE ur.user_id = u.id
  AND ur.role_id = r.id
  AND u.email = 'demo@example.com'
  AND r.code = 'tenant_admin';
"
```

browser を reload し、`/tenant-admin` が 403 JSON ではなく `AdminAccessDenied` を表示することを確認します。

確認後、seed で戻します。

```bash
make seed-demo-user
```

### 9-4. single binary で確認する

```bash
make binary
AUTH_MODE=local ENABLE_LOCAL_PASSWORD_LOGIN=true HTTP_PORT=8080 ./bin/haohao
```

別 terminal で smoke を実行します。

```bash
make smoke-operability
make smoke-observability
make smoke-tenant-admin
```

browser で `http://127.0.0.1:8080/tenant-admin` を開き、SPA fallback でも tenant 管理画面が表示されることを確認します。

## 実装時の注意点

### active tenant selector を admin list に置き換えない

`frontend/src/components/TenantSelector.vue` は、現在の session の active tenant を切り替えるための UI です。tenant 管理 UI が tenant list を持っても、この selector の責務は変えません。

P5 の admin API は `/api/v1/admin/tenants` に閉じます。`GET /api/v1/tenants` は、ログイン中 user が使える active tenant 候補だけを返し続けます。

### provider / SCIM 由来 role の扱いを明確にする

`provider_claim` と `scim` は外部同期で再作成される可能性があります。UI で削除したように見せても、次の login / reconcile で戻ると利用者にとって危険です。

P5 初期版では:

- local override は UI から grant / revoke できる
- provider / SCIM 由来 role は source を表示する
- provider / SCIM 由来 role の恒久変更は source of truth 側で行う

明示 deny を UI に追加する場合は、`tenant_role_overrides` の `deny` を別 action として実装し、通常の revoke と文言を分けます。

### tenant deactivate は影響が大きい

deactivate は active tenant selector から tenant を消します。deactivate 前に確認を挟み、UI 上では次を表示します。

- tenant slug
- active member count
- deactivate は物理削除ではないこと
- existing data は残ること

### audit を optional にしない

tenant / membership / role 変更は後から復元できない運用上の証跡です。P5 の mutation service では `AuditRecorder` が nil の場合に失敗させます。

`MachineClientService` や `TodoService` と同じように、DB mutation と audit event は同じ transaction で commit します。

### metrics label に tenant slug や user id を入れない

P4 の方針どおり、metrics label に tenant slug、user id、email、request id を入れません。

tenant 管理 API も HTTP metrics の route template、method、status class だけで十分です。個別の tenant / user 追跡は audit log と request log で行います。

## 最終確認チェックリスト

- `tenant_admin` role が migration / seed / provider sync 対象に入っている
- `tenant_admin` なしでは `/api/v1/admin/tenants` が `403` になる
- frontend は `403` を `AdminAccessDenied` として表示する
- tenant 作成、更新、deactivate が UI からできる
- existing user に `docs_reader` / `todo_user` を local override として grant できる
- local override role を revoke できる
- provider / SCIM 由来 role の source が UI で分かる
- tenant 管理 mutation が `audit_events` に残る
- active tenant selector は admin 管理画面から独立して動く
- `make gen` が通る
- `go test ./backend/...` が通る
- `npm --prefix frontend run build` が通る
- `make binary` が通る
- `make smoke-operability` が通る
- `make smoke-observability` が通る
- `make smoke-tenant-admin` が通る
