# P3 監査ログ実装チュートリアル

## この文書の目的

この文書は、`deep-research-report.md` の **P3: 監査ログ** を、現在の HaoHao に実装できる順番に分解したチュートリアルです。

P0 では request logging / readiness / scheduler / smoke を入れ、P1 では管理 UI を補完し、P2 では tenant 共有 TODO を縦切りで追加しました。P3 では、その上に **誰が、どの tenant で、何を、いつ変更したか** を残す監査ログを追加します。

この文書で扱う監査ログは、運用向けの request log ではありません。request log は「HTTP request がどう処理されたか」を追うものです。監査ログは「プロダクト上の重要な状態変更が何だったか」を DB に残すものです。

このチュートリアルでは、次を実装対象にします。

- `audit_events` table
- sqlc query
- `AuditService`
- request metadata を Huma handler から読めるようにする context helper
- session / tenant switch / integration / machine client / TODO mutation への audit event 追加
- 監査ログ失敗時の方針
- 生成物と確認コマンド

frontend の監査ログ閲覧 UI はこの P3 では追加しません。まず、失われると復元できない write-side の証跡を残すことを優先します。閲覧 UI は P5 tenant 管理 UI や admin dashboard と一緒に追加できます。

## この文書が前提にしている現在地

このチュートリアルを始める前の repository は、少なくとも次の状態にある前提で進めます。

- migration は `0008_todos` まで存在する
- `db/schema.sql` は tracked artifact として管理されている
- `backend/internal/db/*` は sqlc 生成物である
- `SessionService` / `DelegationService` / `MachineClientService` / `TodoService` が存在する
- `backend/internal/api/*` の Huma handler で browser session / CSRF / active tenant を検証している
- `middleware.RequestID()` と `middleware.RequestLogger()` が app に接続済み
- `make gen` が sqlc / OpenAPI / frontend generated SDK をまとめて更新する
- `make smoke-operability` は、既に `http://127.0.0.1:8080` で起動している server に対して確認する

この P3 では、SCIM / provisioning / external bearer / M2M の全 event までは広げません。まず browser session、tenant switch、delegated grant、machine client、TODO CRUD を対象にします。SCIM / provisioning / M2M は同じ `AuditService` に後から接続します。

## 完成条件

このチュートリアルの完了条件は次です。

- `0009_audit_events` migration で audit event table が追加される
- `actor_user_id`、`tenant_id`、`action`、`target_type`、`target_id`、`request_id`、`client_ip`、`user_agent`、`occurred_at` が保存される
- 将来の M2M / system actor に備えて `actor_type` と nullable な `actor_machine_client_id` も持つ
- mutation service から `AuditService` 経由で event を記録する
- DB mutation は mutation と audit event を同じ transaction に入れる
- session / external provider など transaction に入れられない side effect は失敗方針を明文化して扱う
- token、password、raw session id、refresh token、full TODO title などの secret / sensitive value を監査 metadata に入れない
- `make db-schema`、`make gen` が通る
- `go test ./backend/...` が通る
- `npm --prefix frontend run build` が通る
- single binary を `:8080` で起動した状態で `make smoke-operability` が通る
- manual smoke 後に `audit_events` に対象 event が残っている

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | `db/migrations/0009_audit_events.*.sql` | audit event schema を追加する |
| Step 2 | `db/queries/audit_events.sql` | sqlc query を追加する |
| Step 3 | `backend/internal/service/audit_service.go` | audit helper と失敗方針を実装する |
| Step 4 | `backend/internal/platform/request_meta.go`, middleware | Huma handler の `context.Context` から request metadata を読めるようにする |
| Step 5 | backend wiring | `AuditService` を runtime / OpenAPI wiring に接続する |
| Step 6 | service mutation | TODO / machine client / delegation / session に audit event を入れる |
| Step 7 | API handler | request metadata と actor / tenant を service に渡す |
| Step 8 | test / generation | sqlc、schema、OpenAPI、unit test を更新する |
| Step 9 | local smoke | 実際の mutation 後に `audit_events` を確認する |

## Step 1. audit event schema / migration を追加する

### 1-1. up migration を追加する

#### ファイル: `db/migrations/0009_audit_events.up.sql`

```sql
CREATE TABLE audit_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    actor_type TEXT NOT NULL CHECK (actor_type IN ('user', 'machine_client', 'system')),
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    actor_machine_client_id BIGINT REFERENCES machine_clients(id) ON DELETE SET NULL,
    tenant_id BIGINT REFERENCES tenants(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    request_id TEXT NOT NULL DEFAULT '',
    client_ip TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (btrim(action) <> ''),
    CHECK (btrim(target_type) <> ''),
    CHECK (btrim(target_id) <> '')
);

CREATE UNIQUE INDEX audit_events_public_id_idx
    ON audit_events(public_id);

CREATE INDEX audit_events_occurred_at_idx
    ON audit_events(occurred_at DESC, id DESC);

CREATE INDEX audit_events_tenant_occurred_at_idx
    ON audit_events(tenant_id, occurred_at DESC, id DESC);

CREATE INDEX audit_events_actor_user_occurred_at_idx
    ON audit_events(actor_user_id, occurred_at DESC, id DESC);

CREATE INDEX audit_events_target_idx
    ON audit_events(target_type, target_id, occurred_at DESC, id DESC);

CREATE INDEX audit_events_action_occurred_at_idx
    ON audit_events(action, occurred_at DESC, id DESC);
```

`actor_user_id` は user が存在する間は FK で追跡します。user が削除されても audit event 自体は消さないため、`ON DELETE SET NULL` にします。

`actor_machine_client_id` は P3 の最初の対象ではありませんが、M2M を audit 対象にしたときに同じ table を使えるように先に持たせます。

`target_id` は UUID / numeric ID / logical key を混ぜて扱うため `TEXT` にします。例えば TODO は `public_id`、machine client は local `id`、integration は `zitadel` のような resource server 名を入れます。

`metadata` は補助情報用です。ここに secret を入れてはいけません。入れるのは `changedFields`、`resourceServer`、`allowedScopeCount`、`titleLength` のような調査に必要な低感度情報だけです。

### 1-2. down migration を追加する

#### ファイル: `db/migrations/0009_audit_events.down.sql`

```sql
DROP TABLE IF EXISTS audit_events;
```

### 1-3. schema snapshot を更新する

```bash
make db-schema
```

`db/schema.sql` は生成物ですが、この repository では tracked artifact です。migration を追加したら差分が出るのが正しい状態です。

## Step 2. sqlc query を追加する

#### ファイル: `db/queries/audit_events.sql`

```sql
-- name: CreateAuditEvent :one
INSERT INTO audit_events (
    actor_type,
    actor_user_id,
    actor_machine_client_id,
    tenant_id,
    action,
    target_type,
    target_id,
    request_id,
    client_ip,
    user_agent,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11
)
RETURNING
    id,
    public_id,
    actor_type,
    actor_user_id,
    actor_machine_client_id,
    tenant_id,
    action,
    target_type,
    target_id,
    request_id,
    client_ip,
    user_agent,
    metadata,
    occurred_at,
    created_at;

-- name: ListRecentAuditEvents :many
SELECT
    id,
    public_id,
    actor_type,
    actor_user_id,
    actor_machine_client_id,
    tenant_id,
    action,
    target_type,
    target_id,
    request_id,
    client_ip,
    user_agent,
    metadata,
    occurred_at,
    created_at
FROM audit_events
ORDER BY occurred_at DESC, id DESC
LIMIT $1;

-- name: ListAuditEventsByTenantID :many
SELECT
    id,
    public_id,
    actor_type,
    actor_user_id,
    actor_machine_client_id,
    tenant_id,
    action,
    target_type,
    target_id,
    request_id,
    client_ip,
    user_agent,
    metadata,
    occurred_at,
    created_at
FROM audit_events
WHERE tenant_id = $1
ORDER BY occurred_at DESC, id DESC
LIMIT $2;

-- name: ListAuditEventsByTarget :many
SELECT
    id,
    public_id,
    actor_type,
    actor_user_id,
    actor_machine_client_id,
    tenant_id,
    action,
    target_type,
    target_id,
    request_id,
    client_ip,
    user_agent,
    metadata,
    occurred_at,
    created_at
FROM audit_events
WHERE target_type = $1
  AND target_id = $2
ORDER BY occurred_at DESC, id DESC
LIMIT $3;
```

`CreateAuditEvent` は runtime で使います。`List*` は P3 の manual smoke と、将来の admin UI / export 用の足場です。

query を追加したら生成します。

```bash
make sqlc
```

生成後、`backend/internal/db/models.go` に `AuditEvent` が追加され、`backend/internal/db/audit_events.sql.go` が作られます。

`metadata JSONB` の generated type は sqlc / pgx の設定で変わることがあります。この repository の構成では `[]byte` になる想定で進めます。もし `pgtype.JSONB` や `json.RawMessage` になった場合は、次の Step の `metadataPayload` の代入だけ generated type に合わせてください。

## Step 3. `AuditService` を追加する

#### ファイル: `backend/internal/service/audit_service.go`

```go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	AuditActorUser          = "user"
	AuditActorMachineClient = "machine_client"
	AuditActorSystem        = "system"
)

var ErrInvalidAuditEvent = errors.New("invalid audit event")

type AuditRequest struct {
	RequestID string
	ClientIP  string
	UserAgent string
}

type AuditContext struct {
	ActorType            string
	ActorUserID          *int64
	ActorMachineClientID *int64
	TenantID             *int64
	Request              AuditRequest
}

type AuditEventInput struct {
	AuditContext
	Action     string
	TargetType string
	TargetID   string
	Metadata   map[string]any
}

type AuditRecorder interface {
	Record(ctx context.Context, event AuditEventInput) error
	RecordWithQueries(ctx context.Context, queries *db.Queries, event AuditEventInput) error
	RecordBestEffort(ctx context.Context, event AuditEventInput)
}

type AuditService struct {
	queries *db.Queries
}

func NewAuditService(queries *db.Queries) *AuditService {
	return &AuditService{queries: queries}
}

func (s *AuditService) Record(ctx context.Context, event AuditEventInput) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("audit service is not configured")
	}
	return s.RecordWithQueries(ctx, s.queries, event)
}

func (s *AuditService) RecordWithQueries(ctx context.Context, queries *db.Queries, event AuditEventInput) error {
	if queries == nil {
		return fmt.Errorf("audit queries are not configured")
	}

	normalized, err := normalizeAuditEvent(event)
	if err != nil {
		return err
	}

	metadataPayload, err := json.Marshal(normalized.Metadata)
	if err != nil {
		return fmt.Errorf("encode audit metadata: %w", err)
	}

	if _, err := queries.CreateAuditEvent(ctx, db.CreateAuditEventParams{
		ActorType:            normalized.ActorType,
		ActorUserID:          auditInt8(normalized.ActorUserID),
		ActorMachineClientID: auditInt8(normalized.ActorMachineClientID),
		TenantID:             auditInt8(normalized.TenantID),
		Action:               normalized.Action,
		TargetType:           normalized.TargetType,
		TargetID:             normalized.TargetID,
		RequestID:            normalized.Request.RequestID,
		ClientIp:             normalized.Request.ClientIP,
		UserAgent:            normalized.Request.UserAgent,
		Metadata:             metadataPayload,
	}); err != nil {
		return fmt.Errorf("create audit event: %w", err)
	}

	return nil
}

func (s *AuditService) RecordBestEffort(ctx context.Context, event AuditEventInput) {
	if err := s.Record(ctx, event); err != nil {
		slog.WarnContext(ctx, "audit event failed", "error", err, "action", event.Action, "target_type", event.TargetType, "target_id", event.TargetID)
	}
}

func normalizeAuditEvent(event AuditEventInput) (AuditEventInput, error) {
	event.ActorType = strings.TrimSpace(event.ActorType)
	if event.ActorType == "" {
		event.ActorType = AuditActorUser
	}
	switch event.ActorType {
	case AuditActorUser:
		if event.ActorUserID == nil || *event.ActorUserID <= 0 {
			return AuditEventInput{}, fmt.Errorf("%w: actor user id is required", ErrInvalidAuditEvent)
		}
	case AuditActorMachineClient:
		if event.ActorMachineClientID == nil || *event.ActorMachineClientID <= 0 {
			return AuditEventInput{}, fmt.Errorf("%w: actor machine client id is required", ErrInvalidAuditEvent)
		}
	case AuditActorSystem:
	default:
		return AuditEventInput{}, fmt.Errorf("%w: unsupported actor type", ErrInvalidAuditEvent)
	}

	event.Action = strings.ToLower(strings.TrimSpace(event.Action))
	event.TargetType = strings.ToLower(strings.TrimSpace(event.TargetType))
	event.TargetID = strings.TrimSpace(event.TargetID)
	if event.Action == "" || event.TargetType == "" || event.TargetID == "" {
		return AuditEventInput{}, fmt.Errorf("%w: action, target type, and target id are required", ErrInvalidAuditEvent)
	}

	event.Request.RequestID = strings.TrimSpace(event.Request.RequestID)
	event.Request.ClientIP = strings.TrimSpace(event.Request.ClientIP)
	event.Request.UserAgent = strings.TrimSpace(event.Request.UserAgent)
	if event.Metadata == nil {
		event.Metadata = map[string]any{}
	}

	return event, nil
}

func UserAuditContext(userID int64, tenantID *int64, request AuditRequest) AuditContext {
	return AuditContext{
		ActorType:   AuditActorUser,
		ActorUserID: &userID,
		TenantID:    tenantID,
		Request:     request,
	}
}

func auditInt8(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *value, Valid: true}
}
```

`Record` は通常の fail-closed 用です。呼び出し側は audit 失敗を error として扱います。

`RecordWithQueries` は DB mutation と同じ transaction に入れるための method です。`qtx := queries.WithTx(tx)` を渡して、mutation と audit event を同時に commit / rollback します。

`RecordBestEffort` は transaction に入れられない side effect 用です。logout や外部 provider revoke のように「すでに外側の状態が変わった後」で rollback できない event だけに使います。

## Step 4. request metadata を request context に載せる

Huma handler が受け取るのは `context.Context` です。Gin の `*gin.Context` に set した値はそのままでは Huma handler から読めません。

そこで、request id / client ip / user agent を標準の request context に入れる小さな helper を追加します。

### 4-1. platform helper を追加する

#### ファイル: `backend/internal/platform/request_meta.go`

```go
package platform

import "context"

type requestMetadataContextKey struct{}

type RequestMetadata struct {
	RequestID string
	ClientIP  string
	UserAgent string
}

func ContextWithRequestMetadata(ctx context.Context, metadata RequestMetadata) context.Context {
	return context.WithValue(ctx, requestMetadataContextKey{}, metadata)
}

func RequestMetadataFromContext(ctx context.Context) RequestMetadata {
	metadata, _ := ctx.Value(requestMetadataContextKey{}).(RequestMetadata)
	return metadata
}
```

### 4-2. request id middleware を更新する

#### ファイル: `backend/internal/middleware/request_id.go`

```go
package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"example.com/haohao/backend/internal/platform"

	"github.com/gin-gonic/gin"
)

const RequestIDHeader = "X-Request-ID"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" {
			requestID = newRequestID()
		}

		c.Set("request_id", requestID)
		c.Header(RequestIDHeader, requestID)
		c.Request = c.Request.WithContext(platform.ContextWithRequestMetadata(c.Request.Context(), platform.RequestMetadata{
			RequestID: requestID,
			ClientIP:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		}))

		c.Next()
	}
}

func RequestIDFromContext(c *gin.Context) string {
	value, ok := c.Get("request_id")
	if !ok {
		return ""
	}

	requestID, _ := value.(string)
	return requestID
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}

	return hex.EncodeToString(b[:])
}
```

これで Huma handler から `platform.RequestMetadataFromContext(ctx)` で audit 用 metadata を読めます。

### 4-3. API package に変換 helper を置く

#### ファイル: `backend/internal/api/register.go`

`api` package の複数 handler で使うため、`register.go` の末尾か新しい `audit.go` に helper を追加します。ここでは `backend/internal/api/audit.go` を追加する形にします。

#### ファイル: `backend/internal/api/audit.go`

```go
package api

import (
	"context"

	"example.com/haohao/backend/internal/platform"
	"example.com/haohao/backend/internal/service"
)

func auditRequest(ctx context.Context) service.AuditRequest {
	metadata := platform.RequestMetadataFromContext(ctx)
	return service.AuditRequest{
		RequestID: metadata.RequestID,
		ClientIP:  metadata.ClientIP,
		UserAgent: metadata.UserAgent,
	}
}

func userAuditContext(ctx context.Context, userID int64, tenantID *int64) service.AuditContext {
	return service.UserAuditContext(userID, tenantID, auditRequest(ctx))
}
```

## Step 5. backend wiring を更新する

### 5-1. API dependencies に `AuditService` を追加する

#### ファイル: `backend/internal/api/register.go`

```go
type Dependencies struct {
	SessionService               *service.SessionService
	OIDCLoginService             *service.OIDCLoginService
	DelegationService            *service.DelegationService
	ProvisioningService          *service.ProvisioningService
	AuthzService                 *service.AuthzService
	AuditService                 *service.AuditService
	TodoService                  *service.TodoService
	MachineClientService         *service.MachineClientService
	AuthMode                     string
	EnableLocalPasswordLogin     bool
	SCIMBasePath                 string
	FrontendBaseURL              string
	ZitadelIssuer                string
	ZitadelClientID              string
	ZitadelPostLogoutRedirectURI string
	CookieSecure                 bool
	SessionTTL                   time.Duration
}
```

`AuditService` は API handler から直接使う箇所もあります。例えば logout のような best-effort event は service transaction に入れにくいため、API handler から `RecordBestEffort` を呼ぶ方が扱いやすいです。

### 5-2. App constructor に `AuditService` を通す

#### ファイル: `backend/internal/app/app.go`

`New` の引数に `auditService *service.AuditService` を追加し、`backendapi.Dependencies` に渡します。

```go
func New(
	cfg config.Config,
	logger *slog.Logger,
	sessionService *service.SessionService,
	oidcLoginService *service.OIDCLoginService,
	delegationService *service.DelegationService,
	provisioningService *service.ProvisioningService,
	authzService *service.AuthzService,
	auditService *service.AuditService,
	todoService *service.TodoService,
	machineClientService *service.MachineClientService,
	bearerVerifier *auth.BearerVerifier,
	m2mVerifier *auth.M2MVerifier,
) *App {
	// ...

	backendapi.Register(api, backendapi.Dependencies{
		SessionService:               sessionService,
		OIDCLoginService:             oidcLoginService,
		DelegationService:            delegationService,
		ProvisioningService:          provisioningService,
		AuthzService:                 authzService,
		AuditService:                 auditService,
		TodoService:                  todoService,
		MachineClientService:         machineClientService,
		AuthMode:                     cfg.AuthMode,
		EnableLocalPasswordLogin:     cfg.EnableLocalPasswordLogin,
		SCIMBasePath:                 cfg.SCIMBasePath,
		FrontendBaseURL:              cfg.FrontendBaseURL,
		ZitadelIssuer:                cfg.ZitadelIssuer,
		ZitadelClientID:              cfg.ZitadelClientID,
		ZitadelPostLogoutRedirectURI: cfg.ZitadelPostLogoutRedirectURI,
		CookieSecure:                 cfg.CookieSecure,
		SessionTTL:                   cfg.SessionTTL,
	})

	// ...
}
```

### 5-3. runtime entrypoint に `AuditService` を作る

#### ファイル: `backend/cmd/main/main.go`

`queries := db.New(pool)` の直後に `AuditService` を作り、各 service constructor に渡します。

```go
queries := db.New(pool)
auditService := service.NewAuditService(queries)
sessionStore := auth.NewSessionStore(redisClient, cfg.SessionTTL)
sessionService := service.NewSessionService(queries, sessionStore, cfg.AuthMode, cfg.EnableLocalPasswordLogin, auditService)
authzService := service.NewAuthzService(pool, queries)
todoService := service.NewTodoService(pool, queries, auditService)
machineClientService := service.NewMachineClientService(pool, queries, cfg.M2MRequiredScopePrefix, auditService)
```

`DelegationService` にも audit recorder を渡します。

```go
delegationService = service.NewDelegationService(
	queries,
	delegatedOAuthClient,
	delegationStateStore,
	refreshTokenStore,
	cfg.AppBaseURL,
	cfg.DownstreamDefaultScopes,
	cfg.DownstreamRefreshTokenTTL,
	cfg.DownstreamAccessTokenSkew,
	auditService,
)
```

最後に `app.New` に `auditService` を渡します。

```go
application := app.New(
	cfg,
	logger,
	sessionService,
	oidcLoginService,
	delegationService,
	provisioningService,
	authzService,
	auditService,
	todoService,
	machineClientService,
	bearerVerifier,
	m2mVerifier,
)
```

### 5-4. OpenAPI export entrypoint を更新する

#### ファイル: `backend/cmd/openapi/main.go`

OpenAPI export では handler は実行されません。nil DB でも route registration できるように、constructor だけ合わせます。

```go
auditService := service.NewAuditService(nil)
application := app.New(
	cfg,
	nil,
	nil,
	nil,
	nil,
	nil,
	nil,
	auditService,
	service.NewTodoService(nil, nil, auditService),
	nil,
	nil,
	nil,
)
```

constructor signature を変えた service は、OpenAPI export 側も必ず同じ形に更新します。

## Step 6. mutation service に audit event を入れる

### 6-1. TODO service を transaction 化する

#### ファイル: `backend/internal/service/todo_service.go`

`TodoService` は現在 `queries *db.Queries` だけを持っています。audit event を TODO mutation と同じ transaction に入れるため、`pool` と `audit` を追加します。

```go
import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TodoService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	audit   AuditRecorder
}

func NewTodoService(pool *pgxpool.Pool, queries *db.Queries, audit AuditRecorder) *TodoService {
	return &TodoService{
		pool:    pool,
		queries: queries,
		audit:   audit,
	}
}
```

`List` は read-only なので audit event は残しません。`Create` / `Update` / `Delete` に入れます。

`Create` は次の形にします。

```go
func (s *TodoService) Create(ctx context.Context, tenantID, userID int64, title string, auditCtx AuditContext) (Todo, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return Todo{}, fmt.Errorf("todo service is not configured")
	}
	if s.audit == nil {
		return Todo{}, fmt.Errorf("audit recorder is not configured")
	}

	normalizedTitle, err := normalizeTodoTitle(title)
	if err != nil {
		return Todo{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Todo{}, fmt.Errorf("begin todo create transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	row, err := qtx.CreateTodo(ctx, db.CreateTodoParams{
		TenantID:        tenantID,
		CreatedByUserID: userID,
		Title:           normalizedTitle,
	})
	if err != nil {
		return Todo{}, fmt.Errorf("create todo: %w", err)
	}

	item := todoFromDB(row)
	auditCtx.TenantID = &tenantID
	if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       "todo.create",
		TargetType:   "todo",
		TargetID:     item.PublicID,
		Metadata: map[string]any{
			"titleLength": len([]rune(normalizedTitle)),
		},
	}); err != nil {
		return Todo{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Todo{}, fmt.Errorf("commit todo create transaction: %w", err)
	}
	return item, nil
}
```

`Update` は `changedFields` だけを metadata に入れます。TODO title の実値は監査 metadata に入れません。

```go
changedFields := make([]string, 0, 2)
if input.Title != nil {
	changedFields = append(changedFields, "title")
}
if input.Completed != nil {
	changedFields = append(changedFields, "completed")
}

if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
	AuditContext: auditCtx,
	Action:       "todo.update",
	TargetType:   "todo",
	TargetID:     item.PublicID,
	Metadata: map[string]any{
		"changedFields": changedFields,
	},
}); err != nil {
	return Todo{}, err
}
```

`Delete` は target id が request path の `publicID` です。delete 後は row が無いので、削除前に parse 済みの public id 文字列を使います。

```go
if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
	AuditContext: auditCtx,
	Action:       "todo.delete",
	TargetType:   "todo",
	TargetID:     parsedPublicID.String(),
}); err != nil {
	return err
}
```

この Step の重要点は、TODO row の変更と audit row の insert が同じ transaction に入ることです。audit insert が失敗した場合は TODO mutation も rollback します。

### 6-2. machine client service に audit event を入れる

#### ファイル: `backend/internal/service/machine_client_service.go`

`MachineClientService` も TODO と同じく `pool` と `audit` を持たせます。

```go
type MachineClientService struct {
	pool                *pgxpool.Pool
	queries             *db.Queries
	requiredScopePrefix string
	audit               AuditRecorder
}

func NewMachineClientService(pool *pgxpool.Pool, queries *db.Queries, requiredScopePrefix string, audit AuditRecorder) *MachineClientService {
	return &MachineClientService{
		pool:                pool,
		queries:             queries,
		requiredScopePrefix: strings.TrimSpace(requiredScopePrefix),
		audit:               audit,
	}
}
```

admin mutation method の signature に `auditCtx AuditContext` を追加します。

```go
func (s *MachineClientService) Create(ctx context.Context, input MachineClientInput, auditCtx AuditContext) (MachineClient, error)
func (s *MachineClientService) Update(ctx context.Context, id int64, input MachineClientInput, auditCtx AuditContext) (MachineClient, error)
func (s *MachineClientService) Disable(ctx context.Context, id int64, auditCtx AuditContext) (MachineClient, error)
```

action は次に揃えます。

| Method | action | target_type | target_id |
| --- | --- | --- | --- |
| `Create` | `machine_client.create` | `machine_client` | created local id |
| `Update` | `machine_client.update` | `machine_client` | local id |
| `Disable` | `machine_client.disable` | `machine_client` | local id |

metadata には secret を入れません。machine client の `provider_client_id` は secret ではありませんが、監査ログでは local id で追えるので必須ではありません。

```go
Metadata: map[string]any{
	"provider":          item.Provider,
	"defaultTenantID":   itemDefaultTenantID(item),
	"allowedScopeCount": len(item.AllowedScopes),
	"active":            item.Active,
}
```

`allowedScopes` の中身まで残すかはプロダクト判断です。最初は count に留めると、scope 名に外部 system の情報が混ざる場合でも漏えい面を小さくできます。

### 6-3. delegation service に audit event を入れる

#### ファイル: `backend/internal/service/delegation_service.go`

constructor に `audit AuditRecorder` を追加します。

```go
type DelegationService struct {
	queries       *db.Queries
	oauthClient   *auth.DelegatedOAuthClient
	stateStore    *auth.DelegationStateStore
	tokenStore    *auth.RefreshTokenStore
	appBaseURL    string
	defaultScopes []string
	refreshTTL    time.Duration
	accessSkew    time.Duration
	audit         AuditRecorder
}
```

対象 event は次です。

| Method | action | 失敗方針 |
| --- | --- | --- |
| `StartConnectForTenant` | `integration.connect_start` | fail-closed |
| `SaveGrantFromCallback` | `integration.connect_finish` | best-effort または fail-closed |
| `VerifyAccessTokenForTenant` | `integration.verify` | fail-closed |
| `DeleteGrantForTenant` | `integration.revoke` | provider revoke 後は best-effort |

`StartConnectForTenant` は state を Redis に作った後に authorize URL を返します。audit 失敗で URL を返さなければ user の外部 consent は始まらないため、fail-closed で構いません。

`DeleteGrantForTenant` は外部 provider の revoke が先に成功した後、audit insert だけ失敗する可能性があります。この場合、外部 revoke を戻せないため `RecordBestEffort` にします。ただし DB の grant delete は成功 / 失敗を通常通り error handling します。

metadata は次程度に留めます。

```go
Metadata: map[string]any{
	"resourceServer": resource.resourceServer,
	"provider":       resource.provider,
	"scopeCount":     len(resource.scopes),
}
```

refresh token、access token、authorization code、raw state は絶対に入れません。

### 6-4. session service / handler に audit event を入れる

session は Redis side effect が中心で、DB transaction に入れられません。ここでは次の方針にします。

| Event | action | 方針 |
| --- | --- | --- |
| login success | `session.login` | session 発行後に audit、失敗したら新 session を削除して error |
| logout | `session.logout` | session 削除後に best-effort |
| CSRF reissue | 記録しない | high-volume で監査価値が低い |
| session refresh | `session.refresh` | rotate 後に audit、失敗したら new session を削除して error |
| active tenant switch | `session.tenant_switch` | `SetActiveTenant` 成功後に audit、失敗したら error |

#### ファイル: `backend/internal/service/session_service.go`

constructor に audit recorder を追加します。

```go
type SessionService struct {
	queries                  *db.Queries
	store                    *auth.SessionStore
	authMode                 string
	enableLocalPasswordLogin bool
	audit                    AuditRecorder
}

func NewSessionService(queries *db.Queries, store *auth.SessionStore, authMode string, enableLocalPasswordLogin bool, audit AuditRecorder) *SessionService {
	return &SessionService{
		queries:                  queries,
		store:                    store,
		authMode:                 strings.ToLower(strings.TrimSpace(authMode)),
		enableLocalPasswordLogin: enableLocalPasswordLogin,
		audit:                    audit,
	}
}
```

`Login` / `RefreshSession` / `SetActiveTenant` の signature に `auditRequest AuditRequest` を追加します。

```go
func (s *SessionService) Login(ctx context.Context, email, password string, auditRequest AuditRequest) (User, string, string, error)
func (s *SessionService) RefreshSession(ctx context.Context, sessionID, csrfHeader string, auditRequest AuditRequest) (string, string, error)
func (s *SessionService) SetActiveTenant(ctx context.Context, sessionID, csrfHeader string, tenantID int64, auditRequest AuditRequest) error
```

login success の audit は、認証成功後、session を作った後に入れます。audit が失敗したら発行済み session を削除します。

```go
sessionID, csrfToken, err := s.IssueSession(ctx, userID)
if err != nil {
	return User{}, "", "", err
}

if s.audit != nil {
	if err := s.audit.Record(ctx, AuditEventInput{
		AuditContext: UserAuditContext(user.ID, user.DefaultTenantID, auditRequest),
		Action:       "session.login",
		TargetType:   "session",
		TargetID:     "browser",
	}); err != nil {
		_ = s.store.Delete(ctx, sessionID)
		return User{}, "", "", err
	}
}
```

raw session id は secret と同じ扱いにします。`target_id` に raw session id を入れてはいけません。必要なら `hashSessionID(sessionID)` のような一方向 hash を使いますが、最初は `"browser"` で十分です。

logout は既存 API handler が unauthorized を成功扱いにしています。user が取れる場合だけ、session 削除後に best-effort で残します。

```go
func (s *SessionService) Logout(ctx context.Context, sessionID, csrfHeader string, auditRequest AuditRequest) (string, error) {
	session, err := s.store.Get(ctx, sessionID)
	// 既存の CSRF check は維持する

	user, userErr := s.loadUserByID(ctx, session.UserID)
	if err := s.store.Delete(ctx, sessionID); err != nil {
		return "", err
	}

	if userErr == nil && s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: UserAuditContext(user.ID, user.DefaultTenantID, auditRequest),
			Action:       "session.logout",
			TargetType:   "session",
			TargetID:     "browser",
		})
	}

	return session.ProviderIDTokenHint, nil
}
```

## Step 7. API handler から audit context を渡す

### 7-1. TODO API を更新する

#### ファイル: `backend/internal/api/todos.go`

`create` は既に `current` と `tenant` を持っています。`userAuditContext` を渡します。

```go
item, err := deps.TodoService.Create(
	ctx,
	tenant.ID,
	current.User.ID,
	input.Body.Title,
	userAuditContext(ctx, current.User.ID, &tenant.ID),
)
```

`update` / `delete` では、現在 `_` で捨てている current session を使うように変えます。

```go
current, tenant, err := requireTodoTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
if err != nil {
	return nil, err
}

item, err := deps.TodoService.Update(ctx, tenant.ID, input.TodoPublicID, service.TodoUpdateInput{
	Title:     input.Body.Title,
	Completed: input.Body.Completed,
}, userAuditContext(ctx, current.User.ID, &tenant.ID))
```

delete も同じです。

```go
current, tenant, err := requireTodoTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
if err != nil {
	return nil, err
}

if err := deps.TodoService.Delete(ctx, tenant.ID, input.TodoPublicID, userAuditContext(ctx, current.User.ID, &tenant.ID)); err != nil {
	return nil, toTodoHTTPError(err)
}
```

### 7-2. machine client API を更新する

#### ファイル: `backend/internal/api/machine_clients.go`

今の `requireMachineClientAdmin` は error だけ返します。audit には actor user が必要なので、current session も返すようにします。

```go
func requireMachineClientAdmin(ctx context.Context, deps Dependencies, sessionID, csrfToken string) (service.CurrentSession, error) {
	if deps.MachineClientService == nil {
		return service.CurrentSession{}, huma.Error503ServiceUnavailable("machine client service is not configured")
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
		if errors.Is(err, service.ErrUnauthorized) ||
			errors.Is(err, service.ErrInvalidCSRFToken) ||
			errors.Is(err, service.ErrAuthModeUnsupported) ||
			errors.Is(err, service.ErrInvalidCredentials) {
			return service.CurrentSession{}, toHTTPError(err)
		}
		return service.CurrentSession{}, err
	}
	if !authCtx.HasRole("machine_client_admin") {
		return service.CurrentSession{}, huma.Error403Forbidden("machine_client_admin role is required")
	}
	return current, nil
}
```

mutation handler では `current.User.ID` を audit actor にします。machine client admin は global role なので tenant は nil で構いません。

```go
current, err := requireMachineClientAdmin(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
if err != nil {
	return nil, err
}
item, err := deps.MachineClientService.Create(ctx, machineClientInputFromBody(input.Body), userAuditContext(ctx, current.User.ID, nil))
```

read-only の list / get は audit event を残しません。

### 7-3. tenant switch API を更新する

#### ファイル: `backend/internal/api/tenants.go`

`POST /api/v1/session/tenant` は active tenant を変える重要 mutation です。`SetActiveTenant` に request metadata を渡します。

```go
if err := deps.SessionService.SetActiveTenant(
	ctx,
	input.SessionCookie.Value,
	input.CSRFToken,
	tenant.ID,
	auditRequest(ctx),
); err != nil {
	return nil, toHTTPError(err)
}
```

`SessionService.SetActiveTenant` 側では、`current.User.ID` と `tenantID` を使って次の event を残します。

```go
if s.audit != nil {
	if err := s.audit.Record(ctx, AuditEventInput{
		AuditContext: UserAuditContext(current.User.ID, &tenantID, auditRequest),
		Action:       "session.tenant_switch",
		TargetType:   "tenant",
		TargetID:     fmt.FormatInt(tenantID, 10),
	}); err != nil {
		return err
	}
}
```

### 7-4. session API を更新する

#### ファイル: `backend/internal/api/session.go`

`login` / `refresh` / `logout` に `auditRequest(ctx)` を渡します。

```go
user, sessionID, csrfToken, err := deps.SessionService.Login(ctx, input.Body.Email, input.Body.Password, auditRequest(ctx))
```

```go
sessionID, csrfToken, err := deps.SessionService.RefreshSession(ctx, input.SessionCookie.Value, input.CSRFToken, auditRequest(ctx))
```

```go
idTokenHint, err := deps.SessionService.Logout(ctx, input.SessionCookie.Value, input.CSRFToken, auditRequest(ctx))
```

`GET /api/v1/session` と `GET /api/v1/csrf` は監査ログに入れません。session check や CSRF refresh は high-volume であり、プロダクト上の重要 mutation ではないためです。

### 7-5. integration API を更新する

#### ファイル: `backend/internal/api/integrations.go`

`StartConnectForTenant` / `SaveGrantFromCallback` / `VerifyAccessTokenForTenant` / `DeleteGrantForTenant` に `AuditContext` を渡します。

```go
location, err := deps.DelegationService.StartConnectForTenant(
	ctx,
	current.User,
	authCtx.ActiveTenant.ID,
	input.SessionCookie.Value,
	input.ResourceServer,
	userAuditContext(ctx, current.User.ID, &authCtx.ActiveTenant.ID),
)
```

callback は `currentSessionAuthContext` ではなく `CurrentUser` だけで処理しています。active tenant は state record に保存された `record.TenantID` が source of truth です。`SaveGrantFromCallback` の内部で record から tenant id を audit context に入れてください。

```go
if _, err := deps.DelegationService.SaveGrantFromCallback(
	ctx,
	user,
	input.SessionCookie.Value,
	input.ResourceServer,
	input.Code,
	input.State,
	service.UserAuditContext(user.ID, nil, auditRequest(ctx)),
); err != nil {
	return &IntegrationCallbackOutput{
		Location: integrationRedirect(deps.FrontendBaseURL, "error", "delegated_callback_failed"),
	}, nil
}
```

`SaveGrantFromCallback` の中で state record を検証した後、`auditCtx.TenantID = &record.TenantID` としてから `integration.connect_finish` を記録します。

## Step 8. action name と metadata の基準を固定する

action name は後から変えると検索や UI が割れます。最初に小さく決めて固定します。

| 対象 | action | target_type | target_id | metadata |
| --- | --- | --- | --- | --- |
| login | `session.login` | `session` | `browser` | empty |
| logout | `session.logout` | `session` | `browser` | empty |
| session refresh | `session.refresh` | `session` | `browser` | empty |
| tenant switch | `session.tenant_switch` | `tenant` | tenant id | empty |
| integration connect start | `integration.connect_start` | `integration` | resource server | provider, scopeCount |
| integration connect finish | `integration.connect_finish` | `integration` | resource server | provider, scopeCount |
| integration verify | `integration.verify` | `integration` | resource server | provider |
| integration revoke | `integration.revoke` | `integration` | resource server | provider |
| machine client create | `machine_client.create` | `machine_client` | local id | provider, allowedScopeCount, active |
| machine client update | `machine_client.update` | `machine_client` | local id | changedFields |
| machine client disable | `machine_client.disable` | `machine_client` | local id | empty |
| TODO create | `todo.create` | `todo` | public id | titleLength |
| TODO update | `todo.update` | `todo` | public id | changedFields |
| TODO delete | `todo.delete` | `todo` | public id | empty |

metadata に入れないものも明文化します。

- password
- raw session id
- CSRF token
- authorization code
- access token
- refresh token
- ID token
- downstream token ciphertext
- machine client secret
- full business text fields
- request body の丸ごと保存

監査ログは便利な debug dump ではありません。後から人が読む証跡であり、保持期間も長くなりやすいため、低感度で検索に必要な情報だけを残します。

## Step 9. test を追加する

### 9-1. AuditService の validation test

#### ファイル: `backend/internal/service/audit_service_test.go`

```go
package service

import (
	"errors"
	"testing"
)

func TestNormalizeAuditEventRequiresActorUser(t *testing.T) {
	_, err := normalizeAuditEvent(AuditEventInput{
		AuditContext: AuditContext{ActorType: AuditActorUser},
		Action:       "todo.create",
		TargetType:   "todo",
		TargetID:     "018f2f05-c6c9-7a49-b32d-04f4dd84ef4a",
	})
	if !errors.Is(err, ErrInvalidAuditEvent) {
		t.Fatalf("normalizeAuditEvent() error = %v, want %v", err, ErrInvalidAuditEvent)
	}
}

func TestNormalizeAuditEventDefaultsMetadata(t *testing.T) {
	userID := int64(1)
	got, err := normalizeAuditEvent(AuditEventInput{
		AuditContext: AuditContext{
			ActorUserID: &userID,
		},
		Action:     " TODO.Create ",
		TargetType: " TODO ",
		TargetID:   " target ",
	})
	if err != nil {
		t.Fatalf("normalizeAuditEvent() error = %v", err)
	}
	if got.Action != "todo.create" {
		t.Fatalf("Action = %q, want %q", got.Action, "todo.create")
	}
	if got.TargetType != "todo" {
		t.Fatalf("TargetType = %q, want %q", got.TargetType, "todo")
	}
	if got.TargetID != "target" {
		t.Fatalf("TargetID = %q, want %q", got.TargetID, "target")
	}
	if got.Metadata == nil {
		t.Fatal("Metadata = nil, want empty map")
	}
}
```

### 9-2. request metadata context test

#### ファイル: `backend/internal/platform/request_meta_test.go`

```go
package platform

import (
	"context"
	"testing"
)

func TestRequestMetadataContext(t *testing.T) {
	ctx := ContextWithRequestMetadata(context.Background(), RequestMetadata{
		RequestID: "req-1",
		ClientIP:  "127.0.0.1",
		UserAgent: "test-agent",
	})

	got := RequestMetadataFromContext(ctx)
	if got.RequestID != "req-1" || got.ClientIP != "127.0.0.1" || got.UserAgent != "test-agent" {
		t.Fatalf("RequestMetadataFromContext() = %#v", got)
	}
}
```

### 9-3. existing tests を signature 変更に合わせる

constructor signature を変えるため、既存 test や OpenAPI export で `NewTodoService(queries)` / `NewMachineClientService(queries, prefix)` を呼んでいる箇所を更新します。

検索します。

```bash
rg -n "NewTodoService|NewMachineClientService|NewSessionService|NewDelegationService" backend
```

test で service method を直接呼んでいる場合は、`AuditContext` を渡します。validation helper の unit test だけなら constructor は不要です。

## Step 10. generation と確認コマンド

まず DB を起動して migration / schema を更新します。

```bash
make up
make db-up
make db-schema
```

次に sqlc / OpenAPI / frontend SDK を生成します。

```bash
make gen
```

OpenAPI route を増やしていない場合でも、`make gen` は sqlc 生成物を更新するために実行します。

backend test を流します。

```bash
go test ./backend/...
```

frontend build も確認します。

```bash
npm --prefix frontend run build
```

single binary を確認します。

```bash
make binary
```

## Step 11. local smoke で audit event を確認する

### 11-1. local login で起動する

browser から確認しやすいように local password login を有効にして起動します。

```bash
AUTH_MODE=local ENABLE_LOCAL_PASSWORD_LOGIN=true make backend-dev
```

別 terminal で frontend を起動します。

```bash
make frontend-dev
```

`http://127.0.0.1:5173` を開き、`demo@example.com` / `changeme123` でログインします。

### 11-2. 操作する

次を browser から実行します。

- login
- tenant selector で Acme / Beta を切り替える
- `/todos` で TODO を create / update / delete する
- `/machine-clients` で machine client を create / update / disable する
- `/integrations` で connect / verify / revoke を実行する

Zitadel 連携が未設定の場合、integration の connect / verify / revoke は local smoke から外して構いません。その場合でも TODO / tenant switch / machine client / session の audit event は確認します。

### 11-3. DB で audit event を確認する

```bash
docker compose exec -T postgres psql -U haohao -d haohao -c "
SELECT
    id,
    actor_type,
    actor_user_id,
    tenant_id,
    action,
    target_type,
    target_id,
    request_id,
    occurred_at
FROM audit_events
ORDER BY id DESC
LIMIT 30;
"
```

期待する event 例です。

```text
session.login
session.tenant_switch
todo.create
todo.update
todo.delete
machine_client.create
machine_client.update
machine_client.disable
session.logout
```

metadata も確認します。

```bash
docker compose exec -T postgres psql -U haohao -d haohao -c "
SELECT action, target_type, target_id, metadata
FROM audit_events
ORDER BY id DESC
LIMIT 10;
"
```

`metadata` に token、password、raw session id、full TODO title が入っていないことを確認します。

### 11-4. single binary smoke を確認する

single binary でも middleware / Huma / service wiring が同じように動くことを確認します。

```bash
make binary
AUTH_MODE=local ENABLE_LOCAL_PASSWORD_LOGIN=true HTTP_PORT=8080 ./bin/haohao
```

別 terminal で既存 smoke を流します。

```bash
make smoke-operability
```

browser で `http://127.0.0.1:8080` を開き、login / TODO create を実行したあと、同じ SQL で `audit_events` を確認します。

## 失敗方針

P3 では失敗方針をコードと文書で揃えます。

### DB mutation は fail-closed

TODO create / update / delete、machine client create / update / disable のように Postgres row を変更する操作は、mutation と audit event を同じ DB transaction に入れます。

audit insert が失敗したら mutation も rollback します。

理由は、DB mutation が成功しているのに audit event が無い状態が最も危険だからです。監査ログが必要な操作では、audit failure は mutation failure と同じ扱いにします。

### session / external side effect は個別に扱う

Redis session や外部 provider revoke は Postgres transaction に入りません。

この P3 では次の方針にします。

- login: session 発行後に audit。audit 失敗時は発行済み session を削除して error
- session refresh: rotate 後に audit。audit 失敗時は new session を削除して error
- logout: session 削除後に best-effort audit。audit 失敗でも logout は成功扱い
- integration connect start: consent redirect 前なので fail-closed
- integration callback: grant save 成功後に audit。provider callback flow を壊したくない場合は best-effort
- integration revoke: provider revoke 後は rollback できないため best-effort

best-effort にした event は、`RecordBestEffort` が structured log に `audit event failed` を出します。P4 metrics / tracing で、この失敗を counter / alert に接続します。

### read-only request は audit しない

`GET /api/v1/session`、`GET /api/v1/todos`、`GET /api/v1/tenants`、machine client list / get は audit event に入れません。

read-only access log が必要になった場合は、監査ログとは別の access log / analytics / security log として設計します。P3 の監査ログは重要 mutation に絞ります。

## よくある詰まりどころ

### `metadata` の generated type が tutorial と違う

sqlc / pgx の version や override によって `jsonb` の型が変わることがあります。

`CreateAuditEventParams.Metadata` が `[]byte` なら、`json.Marshal` の結果をそのまま渡します。

`pgtype.JSONB` や `json.RawMessage` になっている場合は、その generated type に合わせて `metadataPayload` の代入だけ調整します。table 設計と呼び出し側の考え方は変えません。

### Huma handler で request id が空になる

`RequestID()` middleware が `c.Request = c.Request.WithContext(...)` を実行しているか確認します。`c.Set("request_id", ...)` だけでは Huma handler の `context.Context` から値を読めません。

### audit insert だけ別 transaction になっている

DB mutation service で `s.audit.Record(ctx, ...)` を呼ぶと、default queries が使われ、mutation transaction と分かれます。

transaction 内では必ず `s.audit.RecordWithQueries(ctx, qtx, ...)` を使います。

### audit failure で TODO が作成されてしまう

`defer tx.Rollback(...)` と `tx.Commit(ctx)` の順番を確認します。audit insert error で return すれば rollback される形にします。

### migration は通るが sqlc が落ちる

`db/schema.sql` が `0009_audit_events` を含んでいるか確認します。

```bash
rg -n "CREATE TABLE audit_events" db/schema.sql
```

含まれていない場合は、DB に migration を流してから `make db-schema` を実行します。

### audit log に sensitive value が入った

metadata を request body から丸ごと作っている可能性があります。metadata は action ごとに allowlist で作ります。

悪い例です。

```go
Metadata: map[string]any{
	"request": input.Body,
}
```

良い例です。

```go
Metadata: map[string]any{
	"changedFields": changedFields,
}
```

## 完了チェックリスト

- [ ] `db/migrations/0009_audit_events.up.sql` を追加した
- [ ] `db/migrations/0009_audit_events.down.sql` を追加した
- [ ] `db/queries/audit_events.sql` を追加した
- [ ] `make db-schema` で `db/schema.sql` を更新した
- [ ] `make sqlc` または `make gen` で `backend/internal/db/audit_events.sql.go` を生成した
- [ ] `AuditService` を追加した
- [ ] request metadata を `context.Context` から読めるようにした
- [ ] `AuditService` を `cmd/main` / `app.New` / API dependencies に接続した
- [ ] TODO create / update / delete が audit event を残す
- [ ] machine client create / update / disable が audit event を残す
- [ ] login / logout / session refresh が audit event を残す
- [ ] active tenant switch が audit event を残す
- [ ] integration connect / verify / revoke が audit event を残す
- [ ] DB mutation の audit は同じ transaction に入っている
- [ ] best-effort event の対象と理由を確認した
- [ ] metadata に token / password / raw session id / full business text が入っていない
- [ ] `go test ./backend/...` が通る
- [ ] `npm --prefix frontend run build` が通る
- [ ] `make binary` が通る
- [ ] single binary 起動後に `make smoke-operability` が通る
- [ ] manual smoke 後に `audit_events` を SQL で確認した

## ここまでで何ができているか

P3 完了時点で、HaoHao は重要 mutation の証跡を DB に残せる状態になります。

特に次が追えるようになります。

- どの user が login / logout / session refresh を行ったか
- どの user が active tenant を切り替えたか
- どの tenant で TODO が作成 / 更新 / 削除されたか
- machine client admin 操作を誰が行ったか
- delegated integration の connect / verify / revoke がいつ行われたか
- request log の `request_id` と audit event を突き合わせられること

これで、P4 metrics / tracing、P5 tenant 管理 UI、P6 業務ドメイン拡張を進める前に、横断的な変更証跡の土台ができます。
