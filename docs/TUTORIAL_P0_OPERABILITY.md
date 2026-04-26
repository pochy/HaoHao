# P0 運用可能性を閉じるチュートリアル

## この文書の目的

この文書は、`deep-research-report.md` の **P0: 運用可能性を閉じる** を、実装できる順番に分解したチュートリアルです。

目的は、HaoHao の既存 backend surface を増やすことではありません。目的は、既にある認証・認可・SCIM・tenant・M2M・単一バイナリ配信を、運用時に判断しやすく、止めやすく、戻しやすくすることです。

このチュートリアルで扱う P0 は次の 5 点です。

- request id 付き structured logging
- `/healthz` / `/readyz`
- `ProvisioningReconcileJob` の scheduler 接続
- smoke script と Makefile target
- cutover / rollback runbook

`TUTORIAL.md` や `TUTORIAL_SINGLE_BINARY.md` と同じく、どのファイルを触り、どの順で確認するかを追える形にします。

## この文書が前提にしている現在地

このチュートリアルを始める前の repository は、少なくとも次の状態にある前提で進めます。

- `backend/cmd/main/main.go` が PostgreSQL / Redis / service / Gin router を組み立てている
- `backend/internal/app/app.go` が `gin.Logger()` / `gin.Recovery()` と Huma API を登録している
- `backend/internal/jobs/provisioning_reconcile.go` に `ProvisioningReconcileJob.RunOnce(ctx)` が存在する
- `backend/internal/config/config.go` に `SCIM_RECONCILE_CRON` があるが、runtime scheduler には接続されていない
- `TUTORIAL_SINGLE_BINARY.md` の実装により、`make binary` と `docker build -t haohao:dev -f docker/Dockerfile .` が通る
- `.env.example` は local / Zitadel / external bearer / M2M / downstream delegated auth / SCIM / cookie / docs auth の設定を持つ

この文書では、業務機能や UI は増やしません。tenant selector UI、machine client admin UI、TODO などは P1 以降で扱います。

## 完成条件

このチュートリアルの完了条件は次です。

- request ごとに `X-Request-ID` が付与され、structured log に同じ値が出る
- `gin.Logger()` が request id 付き structured request logger に置き換わっている
- `/healthz` が process liveness として `200 {"status":"ok"}` を返す
- `/readyz` が PostgreSQL / Redis を確認し、依存が正常なら `200` を返す
- `READINESS_CHECK_ZITADEL=true` の場合だけ Zitadel discovery も readiness に含める
- `ProvisioningReconcileJob.RunOnce(ctx)` が `time.Ticker` ベースの scheduler で実行される
- scheduler は no-overlap、timeout、shutdown、run-on-startup を扱う
- `scripts/smoke-operability.sh` と `make smoke-operability` がある
- `RUNBOOK_OPERABILITY.md` に cutover / rollback / smoke 手順がある
- CI で shell syntax と通常 build/test が確認される

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | `backend/internal/config/config.go`, `.env.example` | 運用設定の入口を固定する |
| Step 2 | `backend/internal/middleware/*`, `backend/internal/app/app.go` | request id と structured logging を入れる |
| Step 3 | `backend/internal/platform/*`, `backend/internal/app/*` | `/healthz` / `/readyz` を Gin route として追加する |
| Step 4 | `backend/internal/jobs/*`, `backend/cmd/main/main.go` | provisioning reconcile を scheduler に接続する |
| Step 5 | `scripts/smoke-operability.sh`, `Makefile` | local smoke の入口を固定する |
| Step 6 | `RUNBOOK_OPERABILITY.md` | cutover / rollback 手順を文書化する |
| Step 7 | `.github/workflows/ci.yml` | CI で P0 の崩れを検知する |

## Step 1. operational config を追加する

まず運用設定の入口を config に寄せます。ここで決める値は、runtime 起動後に変更しません。変更したい場合は process を再起動します。

### 1-1. Config に field を追加する

#### ファイル: `backend/internal/config/config.go`

`Config` に次を追加します。

```go
LogLevel                  string
LogFormat                 string
ReadinessTimeout          time.Duration
ReadinessCheckZitadel     bool
SCIMReconcileEnabled      bool
SCIMReconcileInterval     time.Duration
SCIMReconcileTimeout      time.Duration
SCIMReconcileRunOnStartup bool
```

`Load()` では duration を parse します。0 以下の値は `time.NewTicker` や HTTP timeout と相性が悪いため、起動前に error にします。

```go
readinessTimeout, err := getEnvPositiveDuration("READINESS_TIMEOUT", "2s")
if err != nil {
	return Config{}, err
}
scimReconcileInterval, err := getEnvPositiveDuration("SCIM_RECONCILE_INTERVAL", "1h")
if err != nil {
	return Config{}, err
}
scimReconcileTimeout, err := getEnvPositiveDuration("SCIM_RECONCILE_TIMEOUT", "30s")
if err != nil {
	return Config{}, err
}
```

helper は次です。

```go
func getEnvPositiveDuration(key, fallback string) (time.Duration, error) {
	value := getEnv(key, fallback)
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}

	return parsed, nil
}
```

`Config` の return には次を追加します。

```go
LogLevel:                  getEnv("LOG_LEVEL", "info"),
LogFormat:                 getEnv("LOG_FORMAT", "json"),
ReadinessTimeout:          readinessTimeout,
ReadinessCheckZitadel:     getEnvBool("READINESS_CHECK_ZITADEL", false),
SCIMReconcileEnabled:      getEnvBool("SCIM_RECONCILE_ENABLED", false),
SCIMReconcileInterval:     scimReconcileInterval,
SCIMReconcileTimeout:      scimReconcileTimeout,
SCIMReconcileRunOnStartup: getEnvBool("SCIM_RECONCILE_RUN_ON_STARTUP", false),
```

既存の `SCIM_RECONCILE_CRON` は使いません。cron parser の dependency は追加せず、`time.Ticker` の interval 実行に寄せます。

移行時は、`SCIMReconcileCron` field と `.env.example` の `SCIM_RECONCILE_CRON` を削除します。互換用に残すと、cron で動くのか interval で動くのかが読み手に伝わりにくくなるためです。

### 1-2. `.env.example` を更新する

#### ファイル: `.env.example`

既存の `SCIM_RECONCILE_CRON=0 3 * * *` を削除し、次を追加します。

```dotenv
LOG_LEVEL=info
LOG_FORMAT=json
READINESS_TIMEOUT=2s
READINESS_CHECK_ZITADEL=false

SCIM_RECONCILE_ENABLED=false
SCIM_RECONCILE_INTERVAL=1h
SCIM_RECONCILE_TIMEOUT=30s
SCIM_RECONCILE_RUN_ON_STARTUP=false
```

local では `SCIM_RECONCILE_ENABLED=false` のままで構いません。SCIM smoke や staging で明示的に有効化します。

### 1-3. config test を更新する

#### ファイル: `backend/internal/config/dotenv_test.go`

`.env` parser の test が `SCIM_RECONCILE_CRON` を使っている場合は、新しい key に置き換えます。

例:

```dotenv
SCIM_RECONCILE_INTERVAL=1h
```

追加で `Load()` の duration parse を確認する test を入れる場合は、次を確認します。

- `READINESS_TIMEOUT=3s` が `3 * time.Second` になる
- `SCIM_RECONCILE_INTERVAL=15m` が `15 * time.Minute` になる
- invalid duration と 0 以下の duration は `Load()` が error を返す

## Step 2. request id と structured logging を追加する

次に request ごとの追跡性を上げます。外部 logger dependency は追加せず、Go 標準の `log/slog` を使います。

### 2-1. logger package を追加する

#### ファイル: `backend/internal/platform/logger.go`

`LOG_FORMAT` は `json` と `text` だけを許可します。production / Docker は `json` を default にします。

```go
package platform

import (
	"io"
	"log/slog"
	"strings"
)

func NewLogger(level, format string, out io.Writer) *slog.Logger {
	var slogLevel slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	options := &slog.HandlerOptions{Level: slogLevel}
	if strings.EqualFold(format, "text") {
		return slog.New(slog.NewTextHandler(out, options))
	}
	return slog.New(slog.NewJSONHandler(out, options))
}
```

`LOG_LEVEL` が未知の値なら `info` 扱いにします。起動失敗にはしません。logging の設定 typo で service が落ちるより、`info` で起動する方が運用しやすいためです。

### 2-2. request id middleware を追加する

#### ファイル: `backend/internal/middleware/request_id.go`

`X-Request-ID` が来ていればそれを使い、無ければ生成します。

```go
package middleware

import (
	"crypto/rand"
	"encoding/hex"

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

生成に失敗した場合は空文字でも構いません。request logging は空の `request_id` をそのまま出します。

### 2-3. structured request logger を追加する

#### ファイル: `backend/internal/middleware/request_logger.go`

```go
package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		latency := time.Since(startedAt)
		status := c.Writer.Status()

		attrs := []any{
			"request_id", RequestIDFromContext(c),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"latency_ms", float64(latency.Microseconds()) / 1000,
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		switch {
		case status >= 500:
			logger.ErrorContext(c.Request.Context(), "http request", attrs...)
		case status >= 400:
			logger.WarnContext(c.Request.Context(), "http request", attrs...)
		default:
			logger.InfoContext(c.Request.Context(), "http request", attrs...)
		}
	}
}
```

session id、CSRF token、Authorization header、refresh token、client secret は log に出しません。

### 2-4. app wiring を更新する

#### ファイル: `backend/internal/app/app.go`

`New` の signature に logger を追加します。

```go
func New(
	cfg config.Config,
	logger *slog.Logger,
	sessionService *service.SessionService,
	// ...
) *App {
```

middleware の順番は次にします。

```go
router.Use(
	middleware.RequestID(),
	middleware.RequestLogger(logger),
	gin.Recovery(),
	middleware.DocsAuth(...),
	// ...
)
```

`gin.Logger()` は外します。request log が二重に出るためです。

#### ファイル: `backend/cmd/main/main.go`

起動直後に logger を作ります。

```go
logger := platform.NewLogger(cfg.LogLevel, cfg.LogFormat, os.Stdout)
slog.SetDefault(logger)
if os.Getenv(gin.EnvGinMode) == "" {
	gin.SetMode(gin.ReleaseMode)
}
```

以後の `log.Fatal` / `log.Printf` は段階的に `logger.Error` / `logger.Info` に寄せます。まず P0 では、起動失敗と listen log だけ移行すれば十分です。

## Step 3. `/healthz` / `/readyz` を追加する

health / readiness は OpenAPI に載せません。Huma operation ではなく Gin route として追加します。

理由は、probe endpoint は product API ではなく runtime の運用面だからです。docs auth や browser auth の影響も受けないようにします。

### 3-1. readiness checker を追加する

#### ファイル: `backend/internal/platform/readiness.go`

```go
package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type PingFunc func(context.Context) error

type ReadinessChecker struct {
	PostgresPing  PingFunc
	RedisPing     PingFunc
	ZitadelIssuer string
	CheckZitadel  bool
	HTTPClient    *http.Client
}

type ReadinessResult struct {
	Status string                    `json:"status"`
	Checks map[string]ReadinessCheck `json:"checks"`
}

type ReadinessCheck struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func (c ReadinessChecker) Check(ctx context.Context) ReadinessResult {
	result := ReadinessResult{
		Status: "ok",
		Checks: map[string]ReadinessCheck{},
	}

	c.checkPing(ctx, &result, "postgres", c.PostgresPing)
	c.checkPing(ctx, &result, "redis", c.RedisPing)

	if c.CheckZitadel {
		if err := c.checkZitadel(ctx); err != nil {
			result.Status = "error"
			result.Checks["zitadel"] = ReadinessCheck{Status: "error", Error: err.Error()}
		} else {
			result.Checks["zitadel"] = ReadinessCheck{Status: "ok"}
		}
	}

	return result
}

func (c ReadinessChecker) checkPing(ctx context.Context, result *ReadinessResult, name string, ping PingFunc) {
	if ping == nil {
		result.Status = "error"
		result.Checks[name] = ReadinessCheck{Status: "error", Error: "ping function is not configured"}
		return
	}

	if err := ping(ctx); err != nil {
		result.Status = "error"
		result.Checks[name] = ReadinessCheck{Status: "error", Error: err.Error()}
		return
	}

	result.Checks[name] = ReadinessCheck{Status: "ok"}
}

func (c ReadinessChecker) checkZitadel(ctx context.Context) error {
	issuer := strings.TrimRight(c.ZitadelIssuer, "/")
	if issuer == "" {
		return fmt.Errorf("ZITADEL_ISSUER is empty")
	}

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discovery returned %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return err
	}
	if body["issuer"] == nil {
		return fmt.Errorf("discovery response missing issuer")
	}

	return nil
}

func ReadinessTimeoutClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}
```

Zitadel は default では readiness に含めません。OIDC discovery は外部 dependency なので、local や一時的な IdP 障害で application process を unready にしたくない環境があるためです。

### 3-2. Gin route を追加する

#### ファイル: `backend/internal/app/health.go`

```go
package app

import (
	"context"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/platform"

	"github.com/gin-gonic/gin"
)

func RegisterHealthRoutes(router *gin.Engine, readiness platform.ReadinessChecker, timeout time.Duration) {
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		result := readiness.Check(ctx)
		if result.Status != "ok" {
			c.JSON(http.StatusServiceUnavailable, result)
			return
		}

		c.JSON(http.StatusOK, result)
	})
}
```

### 3-3. main から接続する

#### ファイル: `backend/cmd/main/main.go`

`app.New(...)` の前後どちらでも構いませんが、`NoRoute` より前に登録します。`RegisterFrontendRoutes` より前に登録すれば、SPA fallback に拾われません。

```go
readinessChecker := platform.ReadinessChecker{
	PostgresPing:  pool.Ping,
	RedisPing:     func(ctx context.Context) error { return redisClient.Ping(ctx).Err() },
	ZitadelIssuer: cfg.ZitadelIssuer,
	CheckZitadel:  cfg.ReadinessCheckZitadel,
	HTTPClient:    platform.ReadinessTimeoutClient(cfg.ReadinessTimeout),
}

application := app.New(cfg, logger, ...)
app.RegisterHealthRoutes(application.Router, readinessChecker, cfg.ReadinessTimeout)
```

## Step 4. `ProvisioningReconcileJob` を scheduler に接続する

`ProvisioningReconcileJob` は既に `RunOnce(ctx)` を持っています。P0 ではこの job を runtime に接続します。

cron parser は追加しません。`time.Ticker` の interval 実行にします。

### 4-1. scheduler を追加する

#### ファイル: `backend/internal/jobs/scheduler.go`

```go
package jobs

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

type ReconcileSchedulerConfig struct {
	Enabled      bool
	Interval     time.Duration
	Timeout      time.Duration
	RunOnStartup bool
}

type ReconcileRunner interface {
	RunOnce(context.Context) error
}

type ReconcileScheduler struct {
	job     ReconcileRunner
	config  ReconcileSchedulerConfig
	logger  *slog.Logger
	running atomic.Bool
}

func NewReconcileScheduler(job ReconcileRunner, config ReconcileSchedulerConfig, logger *slog.Logger) *ReconcileScheduler {
	if logger == nil {
		logger = slog.Default()
	}

	return &ReconcileScheduler{
		job:    job,
		config: config,
		logger: logger,
	}
}

func (s *ReconcileScheduler) Start(ctx context.Context) {
	if s == nil || s.job == nil || !s.config.Enabled {
		return
	}
	if s.config.Interval <= 0 {
		s.logger.ErrorContext(ctx, "provisioning reconcile scheduler disabled because interval is not positive", "interval", s.config.Interval.String())
		return
	}
	if s.config.Timeout <= 0 {
		s.logger.ErrorContext(ctx, "provisioning reconcile scheduler disabled because timeout is not positive", "timeout", s.config.Timeout.String())
		return
	}

	if s.config.RunOnStartup {
		go s.runOnce(ctx, "startup")
	}

	ticker := time.NewTicker(s.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go s.runOnce(ctx, "interval")
		}
	}
}

func (s *ReconcileScheduler) runOnce(parent context.Context, trigger string) {
	if !s.running.CompareAndSwap(false, true) {
		s.logger.WarnContext(parent, "provisioning reconcile skipped because previous run is still active", "trigger", trigger)
		return
	}
	defer s.running.Store(false)

	ctx, cancel := context.WithTimeout(parent, s.config.Timeout)
	defer cancel()

	startedAt := time.Now()
	err := s.job.RunOnce(ctx)
	duration := time.Since(startedAt)
	attrs := []any{
		"trigger", trigger,
		"duration_ms", float64(duration.Microseconds()) / 1000,
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
		s.logger.ErrorContext(ctx, "provisioning reconcile failed", attrs...)
		return
	}

	s.logger.InfoContext(ctx, "provisioning reconcile completed", attrs...)
}
```

no-overlap は `atomic.Bool` で十分です。multi-process / multi-replica 環境で厳密に 1 process にしたい場合は、P0 後に PostgreSQL advisory lock か Redis lock を追加します。

### 4-2. main から scheduler を起動する

#### ファイル: `backend/cmd/main/main.go`

import に jobs を追加します。

```go
"example.com/haohao/backend/internal/jobs"
```

`provisioningService` の生成後に job と scheduler を作ります。

```go
reconcileJob := jobs.NewProvisioningReconcileJob(queries, sessionService, delegationService)
reconcileScheduler := jobs.NewReconcileScheduler(reconcileJob, jobs.ReconcileSchedulerConfig{
	Enabled:      cfg.SCIMReconcileEnabled,
	Interval:     cfg.SCIMReconcileInterval,
	Timeout:      cfg.SCIMReconcileTimeout,
	RunOnStartup: cfg.SCIMReconcileRunOnStartup,
}, logger)
```

server shutdown と同じ context で止められるように、`signal.NotifyContext` を server 起動前に作ります。

```go
shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

go reconcileScheduler.Start(shutdownCtx)
```

この形なら SIGINT / SIGTERM で scheduler loop は止まります。実行中の `RunOnce` は timeout または shutdown context に従います。

## Step 5. smoke script と Makefile target を追加する

運用 smoke は手順書だけではなく、script に固定します。

### 5-1. smoke script を追加する

#### ファイル: `scripts/smoke-operability.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
BODY_FILE="$(mktemp)"
HEADERS_FILE="$(mktemp)"

cleanup() {
  rm -f "$BODY_FILE" "$HEADERS_FILE"
}
trap cleanup EXIT

fail() {
  echo "smoke failed: $*" >&2
  exit 1
}

status_of() {
  curl -sS -o "$BODY_FILE" -w "%{http_code}" "$1"
}

expect_status() {
  local path="$1"
  local want="$2"
  local got
  got="$(status_of "${BASE_URL}${path}")"
  if [[ "$got" != "$want" ]]; then
    echo "response body:" >&2
    cat "$BODY_FILE" >&2 || true
    fail "${path}: want ${want}, got ${got}"
  fi
}

expect_status "/healthz" "200"
expect_status "/readyz" "200"

: > "$HEADERS_FILE"
session_status="$(curl -sS -D "$HEADERS_FILE" -o "$BODY_FILE" -w "%{http_code}" "${BASE_URL}/api/v1/session")"
if [[ "$session_status" != "401" ]]; then
  fail "/api/v1/session: want 401, got ${session_status}"
fi
grep -iq '^content-type: application/problem+json' "$HEADERS_FILE" || fail "/api/v1/session did not return application/problem+json"

openapi_status="$(status_of "${BASE_URL}/openapi.yaml")"
if [[ "$openapi_status" != "200" ]]; then
  fail "/openapi.yaml: want 200, got ${openapi_status}"
fi
grep -q "openapi: 3.1.0" "$BODY_FILE" || fail "/openapi.yaml does not look like OpenAPI 3.1 YAML"

: > "$HEADERS_FILE"
curl -sS -D "$HEADERS_FILE" -o "$BODY_FILE" "${BASE_URL}/api/v1/auth/callback?error=forced" >/dev/null
location="$(awk 'tolower($0) ~ /^location:/ {gsub("\r", "", $0); sub(/^[Ll]ocation:[[:space:]]*/, "", $0); print}' "$HEADERS_FILE" | tail -n 1)"

if [[ -z "$location" ]]; then
  fail "callback response did not include a Location header"
fi
if [[ "$location" == http://127.0.0.1:5173* || "$location" == http://localhost:5173* ]]; then
  fail "callback redirected to Vite dev server: ${location}"
fi

echo "operability smoke ok: ${BASE_URL}"
```

この smoke は browser login 成功までは確認しません。P0 の smoke は、process、dependency、OpenAPI、未認証 session、callback error redirect の最低限に絞ります。

### 5-2. Makefile target を追加する

#### ファイル: `Makefile`

```makefile
smoke-operability:
	bash scripts/smoke-operability.sh
```

default の `BASE_URL` は `http://127.0.0.1:8080` です。通常は次だけで確認できます。

```bash
make smoke-operability
```

## Step 6. cutover / rollback runbook を追加する

P0 では実装だけでなく、切り替えと戻し方を repository に残します。

### 6-1. runbook を追加する

#### ファイル: `RUNBOOK_OPERABILITY.md`

最低限、次の章を作ります。

```markdown
# HaoHao operability runbook

## Preconditions

- target version
- database backup
- migration status
- Zitadel redirect URI
- secret / env source

## Binary deploy

1. Build or download release binary.
2. Place `.env` beside the binary or inject env from process manager.
3. Run migration before switching traffic.
4. Start the new process.
5. Run `make smoke-operability`.

## Docker deploy

1. Pull or build image.
2. Confirm env values.
3. Run migration job.
4. Start container.
5. Run smoke from outside the container.

## Rollback

1. Stop new process or remove new container from traffic.
2. Start previous binary or previous image.
3. Do not run down migration automatically.
4. Check `/healthz`, `/readyz`, `/api/v1/session`, `/openapi.yaml`.

## Zitadel redirect URI update

- Browser callback: `${APP_BASE_URL}/api/v1/auth/callback`
- Post logout: `${APP_BASE_URL}/login`
- Single binary frontend must not use `http://127.0.0.1:5173`.

## Smoke order

1. `/healthz`
2. `/readyz`
3. `/api/v1/session`
4. `/openapi.yaml`
5. forced callback error redirect
6. manual browser login if auth settings changed
```

本番 rollback では down migration を自動実行しません。DB schema は forward-compatible に作る前提にします。

## Step 7. CI と最終確認

### 7-1. CI に shell syntax check を追加する

#### ファイル: `.github/workflows/ci.yml`

`scripts/smoke-operability.sh` を追加したら、CI ではまず syntax check だけを必須にします。

```yaml
- name: Smoke script syntax
  run: bash -n scripts/smoke-operability.sh
```

live smoke は DB / Redis / Zitadel / binary 起動が絡むため、最初から GitHub-hosted CI の必須 gate にしません。local runbook か self-hosted environment が安定してから昇格します。

### 7-2. 最終確認コマンド

P0 実装後は次を実行します。

```bash
go test ./backend/...
go test ./backend/internal/config
go test ./backend/internal/middleware
go test ./backend/internal/jobs
go test ./backend/internal/platform ./backend/internal/app
bash -n scripts/smoke-operability.sh
make binary
```

単一バイナリ smoke は、default port の `8080` で確認します。`HTTP_PORT` と `APP_BASE_URL` / `FRONTEND_BASE_URL` の port は必ずそろえます。

```bash
HTTP_PORT=8080 \
APP_BASE_URL=http://127.0.0.1:8080 \
FRONTEND_BASE_URL=http://127.0.0.1:8080 \
AUTH_MODE=local \
ENABLE_LOCAL_PASSWORD_LOGIN=true \
./bin/haohao
```

別 terminal で実行します。

```bash
scripts/smoke-operability.sh
```

Docker image も確認します。

```bash
docker build -t haohao:dev -f docker/Dockerfile .
```

container smoke をする場合は、DB / Redis は host 側の compose を使い、container から host に到達できる `DATABASE_URL` / `REDIS_ADDR` を渡します。macOS Docker Desktop なら `host.docker.internal` を使います。

## Troubleshooting

### `/readyz` が `503` になる

response body の `checks` を見ます。

- `postgres` が error: `DATABASE_URL`、migration、PostgreSQL container を確認する
- `redis` が error: `REDIS_ADDR`、Redis container を確認する
- `zitadel` が error: `READINESS_CHECK_ZITADEL=false` に戻せるか、`ZITADEL_ISSUER` と discovery endpoint を確認する

local development では、Zitadel を readiness に含めないのが default です。

### request log が二重に出る

`gin.Logger()` が残っている可能性があります。`middleware.RequestLogger(logger)` に置き換え、`gin.Logger()` は削除します。

### `X-Request-ID` が response に無い

`middleware.RequestID()` が `middleware.RequestLogger(logger)` より前に登録されているか確認します。middleware の順序は次です。

```go
router.Use(
	middleware.RequestID(),
	middleware.RequestLogger(logger),
	gin.Recovery(),
	// auth middleware...
)
```

### provisioning reconcile が動かない

次を確認します。

- `SCIM_RECONCILE_ENABLED=true`
- `SCIM_RECONCILE_INTERVAL` が valid duration
- `SCIM_RECONCILE_TIMEOUT` が short すぎない
- scheduler が `shutdownCtx` で起動されている
- log に `provisioning reconcile completed` または `provisioning reconcile failed` が出ている

### provisioning reconcile が重複実行される

single process 内では `atomic.Bool` の no-overlap で防ぎます。multi-replica では各 replica が scheduler を持つため、重複実行されます。

multi-replica 本番で 1 回だけ実行したい場合は、次のどちらかを P0 後の追加作業にします。

- scheduler を 1 replica / 1 job process だけで有効化する
- PostgreSQL advisory lock または Redis lock を追加する

### smoke で callback が `127.0.0.1:5173` に飛ぶ

古い binary を起動しているか、single binary 用の `.env` が dev frontend のままです。

単一バイナリでは次をそろえます。

```dotenv
APP_BASE_URL=http://127.0.0.1:8080
FRONTEND_BASE_URL=http://127.0.0.1:8080
ZITADEL_REDIRECT_URI=http://127.0.0.1:8080/api/v1/auth/callback
ZITADEL_POST_LOGOUT_REDIRECT_URI=http://127.0.0.1:8080/login
```

## 完了チェックリスト

P0 実装の完了条件は次です。

- [ ] `.env.example` に logging / readiness / reconcile scheduler の設定がある
- [ ] `SCIM_RECONCILE_CRON` が削除され、interval 設定に置き換わっている
- [ ] `log/slog` ベースの logger がある
- [ ] request id middleware がある
- [ ] structured request logger がある
- [ ] `gin.Logger()` が削除されている
- [ ] `/healthz` が `200 {"status":"ok"}` を返す
- [ ] `/readyz` が PostgreSQL / Redis を確認する
- [ ] `READINESS_CHECK_ZITADEL=true` のときだけ Zitadel discovery を確認する
- [ ] `ProvisioningReconcileJob` が scheduler から呼ばれる
- [ ] scheduler は no-overlap / timeout / shutdown / run-on-startup を扱う
- [ ] `scripts/smoke-operability.sh` がある
- [ ] `make smoke-operability` がある
- [ ] `RUNBOOK_OPERABILITY.md` がある
- [ ] CI に `bash -n scripts/smoke-operability.sh` がある
- [ ] `go test ./backend/...` が通る
- [ ] `make binary` が通る
- [ ] local binary に対して `scripts/smoke-operability.sh` が通る

この P0 が終わると、次の P1 では tenant selector UI と machine client admin UI に進めます。
