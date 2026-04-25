# P4 metrics / tracing / alerting 実装チュートリアル

## この文書の目的

この文書は、`deep-research-report.md` の **P4: metrics / tracing / alerting** を、現在の HaoHao に実装できる順番に分解したチュートリアルです。

P0 では request id 付き structured logging、`/healthz` / `/readyz`、scheduler、smoke を入れました。P3 では product data の重要な状態変更を `audit_events` に残しました。P4 では、その上に **運用品質を継続観測するための metrics / tracing / alerting** を追加します。

この文書で扱う observability は、監査ログの代替ではありません。

- 監査ログ: 誰が、どの tenant で、何を変更したかを DB に残す
- metrics: latency、error rate、dependency failure、scheduler failure を時系列で見る
- tracing: 1 request が middleware / handler / dependency をどう通ったかを追う
- alerting: metrics の異常を検知し、初動手順に接続する

このチュートリアルでは、次を実装対象にします。

- Prometheus 形式の `/metrics`
- HTTP request count / latency / status count
- PostgreSQL / Redis / Zitadel readiness dependency latency
- readiness failure count
- SCIM reconcile run count / failure count / duration / skipped count
- external bearer / M2M / SCIM bearer auth failure count
- OpenTelemetry trace propagation
- request log への `trace_id` / `span_id` 追加
- alert rule の初期セット
- runbook と smoke script

## この文書が前提にしている現在地

このチュートリアルを始める前の repository は、少なくとも次の状態にある前提で進めます。

- `backend/internal/middleware/request_id.go` が `X-Request-ID` を付与している
- `backend/internal/middleware/request_logger.go` が structured request log を出している
- `backend/internal/app/health.go` が `/healthz` / `/readyz` を登録している
- `backend/internal/platform/readiness.go` が PostgreSQL / Redis / optional Zitadel を確認している
- `backend/internal/jobs/scheduler.go` が SCIM reconcile を no-overlap / timeout 付きで実行している
- `backend/cmd/main/main.go` が PostgreSQL / Redis / service / Gin router / scheduler を組み立てている
- `make smoke-operability` が、既に起動している server に対して確認できる
- `make binary` で single binary を作れる

この P4 では dashboard UI は追加しません。まず backend process が metrics と trace を出し、local / Docker / single binary で確認できる状態を作ります。Grafana dashboard は P4 の後に追加してもよいですが、alert rule と runbook は P4 で最低限用意します。

## 完成条件

このチュートリアルの完了条件は次です。

- `/metrics` が Prometheus text format を返す
- HTTP request count / duration / status class が route template 単位で取れる
- `/metrics` 自身や raw URL / tenant id / public id などの高 cardinality label を増やさない
- PostgreSQL / Redis / optional Zitadel readiness check の latency と failure count が取れる
- `/readyz` の失敗が metrics から追える
- SCIM reconcile の run count / failure count / duration / skipped count が取れる
- external bearer / M2M / SCIM bearer auth failure count が取れる
- OpenTelemetry trace context を受け取り、下流へ伝播できる
- tracing 有効時、request log に `request_id` と `trace_id` / `span_id` が同時に出る
- local / single binary / Docker で `/metrics` を確認できる
- alert rule の初期セットが repository にある
- alert 発火時の初動 runbook がある
- `go test ./backend/...` が通る
- `npm --prefix frontend run build` が通る
- `make binary` が通る
- single binary を `:8080` で起動した状態で `make smoke-operability` と `make smoke-observability` が通る

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | Go dependencies | Prometheus / OpenTelemetry の依存を追加する |
| Step 2 | `backend/internal/config/config.go`, `.env.example` | observability 設定の入口を固定する |
| Step 3 | `backend/internal/platform/metrics.go` | metrics collector と helper を追加する |
| Step 4 | `backend/internal/app/app.go`, `backend/cmd/main/main.go` | `/metrics` と HTTP metrics middleware を接続する |
| Step 5 | readiness wiring | DB / Redis / Zitadel ping latency と failure count を記録する |
| Step 6 | `backend/internal/platform/tracing.go`, request logger | OpenTelemetry と log の trace id 対応を入れる |
| Step 7 | `backend/internal/jobs/scheduler.go` | SCIM reconcile metrics を追加する |
| Step 8 | auth middleware | external bearer / M2M / SCIM bearer auth failure metrics を追加する |
| Step 9 | `ops/prometheus/alerts/haohao.rules.yml`, `RUNBOOK_OBSERVABILITY.md` | alert rule と初動手順を追加する |
| Step 10 | `scripts/smoke-observability.sh`, `Makefile` | local smoke の入口を固定する |
| Step 11 | test / local smoke / binary smoke | 生成物と runtime を確認する |

## 先に決める方針

### metrics label の方針

Prometheus label は検索性と cardinality のバランスが重要です。P4 では次だけを label にします。

- HTTP: `method`, `route`, `status_class`
- dependency: `dependency`, `status`
- scheduler: `trigger`, `status`
- auth failure: `kind`, `reason`

`request_id`、`trace_id`、raw path、TODO public id、tenant id、user id、email、token、error message 全文は label にしません。これらは cardinality が高く、metrics backend を壊しやすいためです。個別 request の追跡は log / trace / audit log で行います。

### tracing の方針

tracing は default off にします。

local や CI では collector が無いことが多いため、`OTEL_TRACING_ENABLED=false` でも app が普通に起動できるようにします。staging / production で OTLP collector を用意したら `OTEL_TRACING_ENABLED=true` にします。

trace context propagation は tracing off でも設定します。これにより、将来 collector を有効化したときに HTTP 境界の設計を変えずに済みます。

### alert の方針

alert は最初から多くしすぎません。P4 では「本番運用で即調査が必要なもの」に絞ります。

- process / scrape down
- 5xx rate 上昇
- latency 悪化
- readiness failure
- DB / Redis ping latency 悪化
- SCIM reconcile failure
- auth failure spike

通知先や severity の細分化は、実運用先の Alertmanager / PagerDuty / Slack などに合わせて後から調整します。

## Step 1. Go dependencies を追加する

Prometheus と OpenTelemetry の依存を追加します。version は repository の `go.mod` / `go.sum` に固定されるため、ここでは package path だけ指定します。

```bash
go get \
  github.com/prometheus/client_golang/prometheus \
  github.com/prometheus/client_golang/prometheus/collectors \
  github.com/prometheus/client_golang/prometheus/promauto \
  github.com/prometheus/client_golang/prometheus/promhttp

go get \
  go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin \
  go.opentelemetry.io/otel \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp \
  go.opentelemetry.io/otel/sdk
```

追加後に一度 test を走らせ、依存だけで壊れていないことを確認します。

```bash
go test ./backend/...
```

## Step 2. observability config を追加する

### 2-1. Config に field を追加する

#### ファイル: `backend/internal/config/config.go`

`Config` に次を追加します。

```go
MetricsEnabled          bool
MetricsPath             string
OTELTracingEnabled      bool
OTELServiceName         string
OTELExporterOTLPEndpoint string
OTELExporterOTLPInsecure bool
OTELTraceSampleRatio    float64
```

`Load()` では sampling ratio を parse します。0 未満は 0、1 より大きい値は 1 に丸めます。設定 typo で起動失敗させるより、明確な範囲に補正して動かす方針にします。

```go
otelTraceSampleRatio := getEnvFloat64("OTEL_TRACES_SAMPLER_RATIO", 0.1)
otelTraceSampleRatio = clampFloat64(otelTraceSampleRatio, 0, 1)
```

helper は次です。

```go
func getEnvFloat64(key string, fallback float64) float64 {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func clampFloat64(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
```

`Config` の return には次を追加します。

```go
MetricsEnabled:           getEnvBool("METRICS_ENABLED", true),
MetricsPath:              getEnv("METRICS_PATH", "/metrics"),
OTELTracingEnabled:       getEnvBool("OTEL_TRACING_ENABLED", false),
OTELServiceName:          getEnv("OTEL_SERVICE_NAME", "haohao"),
OTELExporterOTLPEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
OTELExporterOTLPInsecure: getEnvBool("OTEL_EXPORTER_OTLP_INSECURE", true),
OTELTraceSampleRatio:     otelTraceSampleRatio,
```

`METRICS_PATH` は基本的に `/metrics` のままにします。platform 側で別 path を要求される場合だけ変更します。

### 2-2. `.env.example` を更新する

#### ファイル: `.env.example`

observability 用の設定を追加します。

```dotenv
# Observability
METRICS_ENABLED=true
METRICS_PATH=/metrics

OTEL_TRACING_ENABLED=false
OTEL_SERVICE_NAME=haohao
OTEL_EXPORTER_OTLP_ENDPOINT=
OTEL_EXPORTER_OTLP_INSECURE=true
OTEL_TRACES_SAMPLER_RATIO=0.1
```

local default は tracing off です。collector を起動して確認する時だけ `OTEL_TRACING_ENABLED=true` にします。

## Step 3. metrics collector を追加する

#### ファイル: `backend/internal/platform/metrics.go`

Prometheus の registry と collector を `platform` package にまとめます。global default registry は使わず、app ごとに private registry を持たせます。test で collector の二重登録に悩まないようにするためです。

```go
package platform

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry *prometheus.Registry

	httpRequestsTotal       *prometheus.CounterVec
	httpRequestDuration     *prometheus.HistogramVec
	dependencyPingDuration  *prometheus.HistogramVec
	readinessFailuresTotal  *prometheus.CounterVec
	reconcileRunsTotal      *prometheus.CounterVec
	reconcileDuration       *prometheus.HistogramVec
	reconcileSkippedTotal   *prometheus.CounterVec
	authFailuresTotal       *prometheus.CounterVec
}

func NewMetrics(appVersion string) *Metrics {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	factory := promauto.With(registry)
	constLabels := prometheus.Labels{"app_version": appVersion}

	return &Metrics{
		registry: registry,
		httpRequestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "haohao",
			Name:        "http_requests_total",
			Help:        "Total number of HTTP requests.",
			ConstLabels: constLabels,
		}, []string{"method", "route", "status_class"}),
		httpRequestDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace:   "haohao",
			Name:        "http_request_duration_seconds",
			Help:        "HTTP request duration in seconds.",
			Buckets:     []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			ConstLabels: constLabels,
		}, []string{"method", "route", "status_class"}),
		dependencyPingDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace:   "haohao",
			Name:        "dependency_ping_duration_seconds",
			Help:        "Dependency ping duration in seconds.",
			Buckets:     []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2},
			ConstLabels: constLabels,
		}, []string{"dependency", "status"}),
		readinessFailuresTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "haohao",
			Name:        "readiness_failures_total",
			Help:        "Total number of readiness dependency failures.",
			ConstLabels: constLabels,
		}, []string{"dependency"}),
		reconcileRunsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "haohao",
			Name:        "scim_reconcile_runs_total",
			Help:        "Total number of SCIM reconcile runs.",
			ConstLabels: constLabels,
		}, []string{"trigger", "status"}),
		reconcileDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace:   "haohao",
			Name:        "scim_reconcile_duration_seconds",
			Help:        "SCIM reconcile run duration in seconds.",
			Buckets:     []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
			ConstLabels: constLabels,
		}, []string{"trigger", "status"}),
		reconcileSkippedTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "haohao",
			Name:        "scim_reconcile_skipped_total",
			Help:        "Total number of skipped SCIM reconcile runs.",
			ConstLabels: constLabels,
		}, []string{"trigger"}),
		authFailuresTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "haohao",
			Name:        "auth_failures_total",
			Help:        "Total number of bearer authentication failures.",
			ConstLabels: constLabels,
		}, []string{"kind", "reason"}),
	}
}

func (m *Metrics) Handler() http.Handler {
	if m == nil {
		return http.NotFoundHandler()
	}
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) HTTPMiddleware(metricsPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m == nil {
			c.Next()
			return
		}
		if c.Request.URL.Path == metricsPath {
			c.Next()
			return
		}

		startedAt := time.Now()
		c.Next()

		route := c.FullPath()
		if strings.TrimSpace(route) == "" {
			route = "unmatched"
		}
		statusClass := strconv.Itoa(c.Writer.Status()/100) + "xx"

		m.httpRequestsTotal.WithLabelValues(c.Request.Method, route, statusClass).Inc()
		m.httpRequestDuration.WithLabelValues(c.Request.Method, route, statusClass).Observe(time.Since(startedAt).Seconds())
	}
}

func (m *Metrics) InstrumentPing(dependency string, ping PingFunc) PingFunc {
	return func(ctx context.Context) error {
		startedAt := time.Now()
		var err error
		if ping == nil {
			err = fmt.Errorf("%s ping function is not configured", dependency)
		} else {
			err = ping(ctx)
		}

		if m != nil {
			m.ObserveDependencyPing(dependency, time.Since(startedAt), err)
		}

		return err
	}
}

func (m *Metrics) ObserveDependencyPing(dependency string, duration time.Duration, err error) {
	if m == nil {
		return
	}

	status := "ok"
	if err != nil {
		status = "error"
	}
	m.dependencyPingDuration.WithLabelValues(dependency, status).Observe(duration.Seconds())
	if err != nil {
		m.readinessFailuresTotal.WithLabelValues(dependency).Inc()
	}
}

func (m *Metrics) ObserveReconcileRun(trigger string, duration time.Duration, err error) {
	if m == nil {
		return
	}
	status := "ok"
	if err != nil {
		status = "error"
	}
	m.reconcileRunsTotal.WithLabelValues(trigger, status).Inc()
	m.reconcileDuration.WithLabelValues(trigger, status).Observe(duration.Seconds())
}

func (m *Metrics) IncReconcileSkipped(trigger string) {
	if m == nil {
		return
	}
	m.reconcileSkippedTotal.WithLabelValues(trigger).Inc()
}

func (m *Metrics) IncAuthFailure(kind, reason string) {
	if m == nil {
		return
	}
	m.authFailuresTotal.WithLabelValues(kind, reason).Inc()
}
```

`InstrumentPing` は `ping == nil` の時も error を返します。既存の `ReadinessChecker` が持っている「ping function が無い場合は readiness failure」という意味を維持するためです。大事なのは、dependency 名を固定 label にして latency / failure を記録することです。

## Step 4. `/metrics` と HTTP middleware を接続する

### 4-1. App wiring に Metrics を渡す

#### ファイル: `backend/internal/app/app.go`

`app.New` に `metrics *platform.Metrics` を追加します。

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
	metrics *platform.Metrics,
) *App {
```

middleware は request id、metrics、logging の順に通します。tracing は Step 6 で request id と metrics の間に追加します。

```go
handlers := []gin.HandlerFunc{
	middleware.RequestID(),
}
if cfg.MetricsEnabled && metrics != nil {
	handlers = append(handlers, metrics.HTTPMiddleware(cfg.MetricsPath))
}
handlers = append(handlers,
	middleware.RequestLogger(logger),
	gin.Recovery(),
	middleware.DocsAuth(cfg.DocsAuthRequired, sessionService, authzService),
	middleware.ExternalCORS("/api/external/", cfg.ExternalAllowedOrigins),
	middleware.ExternalAuth("/api/external/", bearerVerifier, authzService, "zitadel", cfg.ExternalExpectedAudience, cfg.ExternalRequiredScopePrefix, cfg.ExternalRequiredRole, metrics),
	middleware.M2MAuth("/api/m2m/", m2mVerifier, machineClientService, "zitadel", metrics),
	middleware.SCIMAuth(cfg.SCIMBasePath+"/", bearerVerifier, cfg.SCIMBearerAudience, cfg.SCIMRequiredScope, metrics),
)
router.Use(handlers...)
```

`cfg.MetricsEnabled` が true で `metrics` がある場合は `/metrics` を登録します。test で `metrics=nil` を渡すケースがあるなら、nil guard を入れて panic しないようにします。

```go
if cfg.MetricsEnabled && metrics != nil {
	router.GET(cfg.MetricsPath, gin.WrapH(metrics.Handler()))
}
```

`/metrics` は Huma API ではなく Gin route として登録します。OpenAPI に出す product API ではなく、platform が scrape する operational endpoint だからです。

### 4-2. main で Metrics を作る

#### ファイル: `backend/cmd/main/main.go`

config load 後、app を作る前に metrics を作ります。

```go
var metrics *platform.Metrics
if cfg.MetricsEnabled {
	metrics = platform.NewMetrics(cfg.AppVersion)
}
```

`app.New` に `metrics` を渡します。

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
	metrics,
)
```

## Step 5. readiness dependency metrics を接続する

#### ファイル: `backend/cmd/main/main.go`

`ReadinessChecker` に metrics を渡します。PostgreSQL / Redis は `checkPing` の中で、Zitadel は `checkZitadel` の前後で latency / failure を記録します。

```go
app.RegisterHealthRoutes(application.Router, platform.ReadinessChecker{
	PostgresPing:  pool.Ping,
	RedisPing:     func(ctx context.Context) error { return redisClient.Ping(ctx).Err() },
	ZitadelIssuer: cfg.ZitadelIssuer,
	CheckZitadel:  cfg.ReadinessCheckZitadel,
	HTTPClient:    platform.ReadinessTimeoutClient(cfg.ReadinessTimeout),
	Metrics:       metrics,
}, cfg.ReadinessTimeout)
```

`ReadinessChecker` に `Metrics *Metrics` を持たせます。

```go
type ReadinessChecker struct {
	PostgresPing  PingFunc
	RedisPing     PingFunc
	ZitadelIssuer string
	CheckZitadel  bool
	HTTPClient    *http.Client
	Metrics       *Metrics
}
```

`checkZitadel` の結果も `dependency_ping_duration_seconds{dependency="zitadel"}` と `readiness_failures_total{dependency="zitadel"}` に入るようにします。

## Step 6. OpenTelemetry tracing を追加する

### 6-1. tracing initializer を追加する

#### ファイル: `backend/internal/platform/tracing.go`

OpenTelemetry SDK の初期化を platform に寄せます。

```go
package platform

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type TracingConfig struct {
	Enabled      bool
	ServiceName  string
	AppVersion   string
	Endpoint     string
	Insecure     bool
	SampleRatio  float64
}

func InitTracing(ctx context.Context, cfg TracingConfig, logger *slog.Logger) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	options := []otlptracehttp.Option{}
	if cfg.Endpoint != "" {
		options = append(options, otlptracehttp.WithEndpointURL(cfg.Endpoint))
	}
	if cfg.Insecure {
		options = append(options, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(ctx, options...)
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRatio)),
		sdktrace.WithResource(resource.NewWithAttributes(
			"",
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.AppVersion),
		)),
	)

	otel.SetTracerProvider(provider)
	if logger != nil {
		logger.Info("tracing enabled", "service", cfg.ServiceName, "endpoint", cfg.Endpoint, "sample_ratio", cfg.SampleRatio)
	}

	return provider.Shutdown, nil
}
```

`WithEndpointURL` が使えない OpenTelemetry version の場合は、依存 version に合わせて `WithEndpoint` / `WithURLPath` へ置き換えます。設定値としては `OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318` のような OTLP HTTP endpoint を想定します。

### 6-2. Gin tracing middleware を追加する

#### ファイル: `backend/internal/middleware/tracing.go`

`app` package から OpenTelemetry の import 詳細を隠すため、middleware package に薄い wrapper を置きます。

```go
package middleware

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func Trace(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}
```

#### ファイル: `backend/internal/app/app.go`

Step 4 で作った middleware chain に tracing を挿入します。request id と trace id の対応を log に残したいので、`RequestID()` の直後、HTTP metrics / request logger の前に置きます。

```go
handlers := []gin.HandlerFunc{
	middleware.RequestID(),
}
if cfg.OTELTracingEnabled {
	handlers = append(handlers, middleware.Trace(cfg.OTELServiceName))
}
if cfg.MetricsEnabled && metrics != nil {
	handlers = append(handlers, metrics.HTTPMiddleware(cfg.MetricsPath))
}
```

### 6-3. main で tracing を初期化する

#### ファイル: `backend/cmd/main/main.go`

logger 初期化後に tracing を初期化します。

```go
shutdownTracing, err := platform.InitTracing(ctx, platform.TracingConfig{
	Enabled:     cfg.OTELTracingEnabled,
	ServiceName: cfg.OTELServiceName,
	AppVersion:  cfg.AppVersion,
	Endpoint:    cfg.OTELExporterOTLPEndpoint,
	Insecure:    cfg.OTELExporterOTLPInsecure,
	SampleRatio: cfg.OTELTraceSampleRatio,
}, logger)
if err != nil {
	fatal(logger, "initialize tracing", err)
}
defer func() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdownTracing(shutdownCtx); err != nil {
		logger.Warn("shutdown tracing", "error", err)
	}
}()
```

### 6-4. request log に trace id を追加する

#### ファイル: `backend/internal/middleware/request_logger.go`

OpenTelemetry の span context から `trace_id` / `span_id` を取り、structured log に足します。

```go
import (
	"go.opentelemetry.io/otel/trace"
)
```

`attrs` を作った後に追加します。

```go
spanContext := trace.SpanContextFromContext(c.Request.Context())
if spanContext.IsValid() {
	attrs = append(attrs,
		"trace_id", spanContext.TraceID().String(),
		"span_id", spanContext.SpanID().String(),
	)
}
```

これにより、1 行の request log から次を同時に辿れます。

- `request_id`: user / support / audit log との突合用
- `trace_id`: tracing backend で request 全体を見るための ID
- `span_id`: request span の識別子

## Step 7. SCIM reconcile metrics を追加する

#### ファイル: `backend/internal/jobs/scheduler.go`

scheduler から platform package へ直接依存させないため、interface を追加します。

```go
type ReconcileMetrics interface {
	ObserveReconcileRun(trigger string, duration time.Duration, err error)
	IncReconcileSkipped(trigger string)
}
```

`ReconcileScheduler` に field を追加します。

```go
type ReconcileScheduler struct {
	job     ReconcileRunner
	config  ReconcileSchedulerConfig
	logger  *slog.Logger
	metrics ReconcileMetrics
	running atomic.Bool
}
```

constructor に `metrics ReconcileMetrics` を追加します。

```go
func NewReconcileScheduler(job ReconcileRunner, config ReconcileSchedulerConfig, logger *slog.Logger, metrics ReconcileMetrics) *ReconcileScheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ReconcileScheduler{
		job:     job,
		config:  config,
		logger:  logger,
		metrics: metrics,
	}
}
```

skip 時に metric を増やします。

```go
if !s.running.CompareAndSwap(false, true) {
	if s.metrics != nil {
		s.metrics.IncReconcileSkipped(trigger)
	}
	s.logger.WarnContext(parent, "provisioning reconcile skipped because previous run is still active", "trigger", trigger)
	return
}
```

run 完了時に duration と status を記録します。

```go
err := s.job.RunOnce(ctx)
duration := time.Since(startedAt)
if s.metrics != nil {
	s.metrics.ObserveReconcileRun(trigger, duration, err)
}
```

#### ファイル: `backend/cmd/main/main.go`

constructor 呼び出しに `metrics` を渡します。

```go
reconcileScheduler := jobs.NewReconcileScheduler(reconcileJob, jobs.ReconcileSchedulerConfig{
	Enabled:      cfg.SCIMReconcileEnabled,
	Interval:     cfg.SCIMReconcileInterval,
	Timeout:      cfg.SCIMReconcileTimeout,
	RunOnStartup: cfg.SCIMReconcileRunOnStartup,
}, logger, metrics)
```

## Step 8. auth failure metrics を追加する

#### ファイル: `backend/internal/middleware/external_auth.go`

middleware package でも platform package へ直接依存させないため、interface を追加します。

```go
type AuthFailureMetrics interface {
	IncAuthFailure(kind, reason string)
}
```

`ExternalAuth` / `M2MAuth` / `SCIMAuth` の引数に `metrics AuthFailureMetrics` を追加します。

```go
func ExternalAuth(..., metrics AuthFailureMetrics) gin.HandlerFunc
func M2MAuth(..., metrics AuthFailureMetrics) gin.HandlerFunc
func SCIMAuth(..., metrics AuthFailureMetrics) gin.HandlerFunc
```

認証失敗時に、低 cardinality な reason で metric を増やします。

```go
func incAuthFailure(metrics AuthFailureMetrics, kind, reason string) {
	if metrics == nil {
		return
	}
	metrics.IncAuthFailure(kind, reason)
}
```

reason は固定値だけを使います。

```go
const (
	authFailureMissingToken  = "missing_token"
	authFailureInvalidToken  = "invalid_token"
	authFailureInvalidScope  = "invalid_scope"
	authFailureInvalidRole   = "invalid_role"
	authFailureTenantDenied  = "tenant_denied"
	authFailureInactiveClient = "inactive_client"
	authFailureNotConfigured = "not_configured"
)
```

例:

```go
rawToken, err := bearerTokenFromHeader(c.GetHeader("Authorization"))
if err != nil {
	incAuthFailure(metrics, "external_bearer", authFailureMissingToken)
	writeBearerProblem(c, http.StatusUnauthorized, err.Error())
	return
}

claims, err := verifier.Verify(c.Request.Context(), rawToken, expectedAudience, requiredScopePrefix)
if err != nil {
	reason := authFailureInvalidToken
	if errors.Is(err, auth.ErrInvalidBearerScope) {
		reason = authFailureInvalidScope
	}
	incAuthFailure(metrics, "external_bearer", reason)
	...
}
```

M2M は `kind="m2m"`、SCIM は `kind="scim"` にします。

error message 全文を label に入れてはいけません。token 内容や tenant 情報が混ざる可能性があり、cardinality も高くなります。

## Step 9. alert rule と runbook を追加する

### 9-1. Prometheus alert rule を追加する

#### ファイル: `ops/prometheus/alerts/haohao.rules.yml`

```yaml
groups:
  - name: haohao
    rules:
      - alert: HaoHaoScrapeDown
        expr: up{job="haohao"} == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "HaoHao metrics scrape is down"
          runbook: "RUNBOOK_OBSERVABILITY.md#haohaoscrapedown"

      - alert: HaoHaoHigh5xxRate
        expr: |
          sum(rate(haohao_http_requests_total{status_class="5xx"}[5m]))
          /
          clamp_min(sum(rate(haohao_http_requests_total[5m])), 1)
          > 0.02
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "HaoHao 5xx rate is high"
          runbook: "RUNBOOK_OBSERVABILITY.md#haohaohigh5xxrate"

      - alert: HaoHaoHighLatency
        expr: |
          histogram_quantile(
            0.95,
            sum by (le) (rate(haohao_http_request_duration_seconds_bucket[5m]))
          ) > 1
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "HaoHao p95 latency is high"
          runbook: "RUNBOOK_OBSERVABILITY.md#haohaohighlatency"

      - alert: HaoHaoReadinessFailure
        expr: increase(haohao_readiness_failures_total[5m]) > 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "HaoHao readiness dependency is failing"
          runbook: "RUNBOOK_OBSERVABILITY.md#haohaoreadinessfailure"

      - alert: HaoHaoDependencyPingSlow
        expr: |
          histogram_quantile(
            0.95,
            sum by (le, dependency) (rate(haohao_dependency_ping_duration_seconds_bucket[5m]))
          ) > 0.25
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "HaoHao dependency ping latency is high"
          runbook: "RUNBOOK_OBSERVABILITY.md#haohaodependencypingslow"

      - alert: HaoHaoSCIMReconcileFailure
        expr: increase(haohao_scim_reconcile_runs_total{status="error"}[15m]) > 0
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "HaoHao SCIM reconcile is failing"
          runbook: "RUNBOOK_OBSERVABILITY.md#haohaoscimreconcilefailure"

      - alert: HaoHaoAuthFailureSpike
        expr: sum by (kind) (increase(haohao_auth_failures_total[5m])) > 20
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "HaoHao bearer auth failures spiked"
          runbook: "RUNBOOK_OBSERVABILITY.md#haohaoauthfailurespike"
```

`job="haohao"` は Prometheus scrape config に合わせて変更します。alert rule 側に tenant / user / route の細かい条件を入れすぎず、まず異常を検知できる最小セットにします。

### 9-2. runbook を追加する

#### ファイル: `RUNBOOK_OBSERVABILITY.md`

runbook には alert ごとに、最初に見る metrics / logs / health endpoint を書きます。

```md
# Observability Runbook

## HaoHaoScrapeDown

1. `/healthz` と `/readyz` を確認する。
2. process / container が起動しているか確認する。
3. `METRICS_ENABLED` と `METRICS_PATH` を確認する。
4. Prometheus から HaoHao への network / service discovery を確認する。

## HaoHaoHigh5xxRate

1. `haohao_http_requests_total{status_class="5xx"}` を route 別に見る。
2. 同時間帯の structured log を `status>=500` で検索する。
3. log の `request_id` / `trace_id` から trace を確認する。
4. `/readyz` と dependency ping metrics を確認する。

## HaoHaoHighLatency

1. p95 latency が上がっている route を確認する。
2. DB / Redis / Zitadel ping latency を確認する。
3. trace で遅い span が handler / DB / external dependency のどこかを見る。

## HaoHaoReadinessFailure

1. `/readyz` の JSON body で failing dependency を確認する。
2. `haohao_readiness_failures_total` の dependency label を確認する。
3. PostgreSQL / Redis / Zitadel の疎通、credential、network を確認する。

## HaoHaoDependencyPingSlow

1. `dependency` label が postgres / redis / zitadel のどれかを確認する。
2. dependency 側の saturation、connection 数、network latency を確認する。
3. app restart ではなく dependency 側の状態を優先して確認する。

## HaoHaoSCIMReconcileFailure

1. scheduler log の `provisioning reconcile failed` を確認する。
2. `trigger` が startup / interval のどちらか確認する。
3. delegated grant / SCIM mapping / provider availability を確認する。

## HaoHaoAuthFailureSpike

1. `kind` が external_bearer / m2m / scim のどれか確認する。
2. `reason` が missing_token / invalid_scope / invalid_role / tenant_denied / client_not_found などのどれか確認する。
3. provider 側の client / audience / scope / role 設定変更がないか確認する。
4. token や secret は log / issue / chat に貼らない。
```

## Step 10. observability smoke を追加する

### 10-1. smoke script を追加する

#### ファイル: `scripts/smoke-observability.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
BODY_FILE="$(mktemp)"

cleanup() {
  rm -f "$BODY_FILE"
}
trap cleanup EXIT

fail() {
  echo "observability smoke failed: $*" >&2
  exit 1
}

curl -sS -o /dev/null "${BASE_URL}/readyz"
curl -sS -o /dev/null "${BASE_URL}/api/v1/session" || true

status="$(curl -sS -o "$BODY_FILE" -w "%{http_code}" "${BASE_URL}/metrics")"
if [[ "$status" != "200" ]]; then
  cat "$BODY_FILE" >&2 || true
  fail "/metrics: want 200, got ${status}"
fi

grep -q '^# HELP haohao_http_requests_total ' "$BODY_FILE" || fail "missing http request counter"
grep -q '^# HELP haohao_http_request_duration_seconds ' "$BODY_FILE" || fail "missing http request duration histogram"
grep -q '^# HELP haohao_dependency_ping_duration_seconds ' "$BODY_FILE" || fail "missing dependency ping histogram"
grep -q 'haohao_http_requests_total' "$BODY_FILE" || fail "http request metrics not exported"
grep -q 'haohao_dependency_ping_duration_seconds' "$BODY_FILE" || fail "dependency ping metrics not exported"

echo "observability smoke ok: ${BASE_URL}"
```

実装後に executable bit を付けます。

```bash
chmod +x scripts/smoke-observability.sh
```

### 10-2. Makefile target を追加する

#### ファイル: `Makefile`

```make
smoke-observability:
	bash scripts/smoke-observability.sh
```

## Step 11. test と local smoke を実行する

### 11-1. unit test

最低限、次の test を追加します。

- `backend/internal/platform/metrics_test.go`
  - `NewMetrics` が handler を返す
  - HTTP middleware が route template と status class で counter を増やす
  - dependency ping success / failure が histogram と failure counter を更新する
- `backend/internal/middleware/request_id_test.go`
  - tracing 有効時に request log に `trace_id` が含まれる
- `backend/internal/jobs/scheduler_test.go`
  - reconcile success / failure / skipped で metrics interface が呼ばれる
- `backend/internal/app/health_test.go`
  - `/metrics` が `200` を返す

実行します。

```bash
go test ./backend/...
npm --prefix frontend run build
make binary
```

### 11-2. local API で metrics を確認する

```bash
make up
make db-up
make seed-demo-user
make binary
```

別 terminal で single binary を起動します。

```bash
AUTH_MODE=local \
ENABLE_LOCAL_PASSWORD_LOGIN=true \
HTTP_PORT=8080 \
./bin/haohao
```

確認します。

```bash
curl -sS http://127.0.0.1:8080/metrics | head
curl -sS http://127.0.0.1:8080/readyz
curl -sS -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8080/api/v1/session
curl -sS http://127.0.0.1:8080/metrics | rg 'haohao_http_requests_total|haohao_dependency_ping_duration_seconds|haohao_readiness_failures_total'
make smoke-operability
make smoke-observability
```

### 11-3. tracing を local collector で確認する

OTLP HTTP collector がある環境では tracing を有効化して起動します。

```bash
OTEL_TRACING_ENABLED=true \
OTEL_SERVICE_NAME=haohao \
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318 \
OTEL_TRACES_SAMPLER_RATIO=1 \
AUTH_MODE=local \
ENABLE_LOCAL_PASSWORD_LOGIN=true \
HTTP_PORT=8080 \
./bin/haohao
```

request を投げます。

```bash
curl -sS -H 'X-Request-ID: trace-smoke-1' http://127.0.0.1:8080/readyz
curl -sS -H 'X-Request-ID: trace-smoke-2' http://127.0.0.1:8080/api/v1/session
```

structured log に `request_id` と `trace_id` が同じ行に出ていれば、log から trace backend へ辿れます。

```json
{
  "msg": "http request",
  "request_id": "trace-smoke-1",
  "trace_id": "00000000000000000000000000000000",
  "span_id": "0000000000000000",
  "method": "GET",
  "path": "/readyz",
  "status": 200
}
```

実際の `trace_id` / `span_id` は request ごとに変わります。

## CI で確認すること

CI では collector を起動しなくてもよいです。P4 の CI では次を確認します。

```bash
go test ./backend/...
npm --prefix frontend run build
make binary
```

`make smoke-observability` は起動中 server が必要な smoke なので、既存の `make smoke-operability` と同じ扱いにします。CI で single binary を起動する job があるなら、その job に追加します。

## P4 でやらないこと

P4 では次は扱いません。

- Grafana dashboard の完成版
- tenant / user / TODO ID を metrics label にすること
- audit log を metrics backend に送ること
- frontend の user behavior analytics
- alert 通知先の provider 固有設定
- distributed trace の全 DB query span 化

DB query span は将来 `pgx` instrumentation を入れて追加できます。P4 ではまず HTTP request、readiness dependency、scheduler、auth failure の横断観測を優先します。

## 最終確認チェックリスト

- `METRICS_ENABLED=true` で `/metrics` が `200` を返す
- `/metrics` に `haohao_http_requests_total` がある
- `/metrics` に `haohao_http_request_duration_seconds` がある
- `/metrics` に `haohao_dependency_ping_duration_seconds` がある
- `/readyz` を叩くと dependency ping metrics が増える
- `/api/v1/session` などの API を叩くと HTTP metrics が増える
- bearer auth failure を起こすと `haohao_auth_failures_total` が増える
- SCIM reconcile を有効化した環境で run / failure / skipped metrics が増える
- `OTEL_TRACING_ENABLED=false` でも app が起動する
- `OTEL_TRACING_ENABLED=true` で collector がある場合に trace が送られる
- request log に `request_id` と `trace_id` が同時に出る
- `ops/prometheus/alerts/haohao.rules.yml` がある
- `RUNBOOK_OBSERVABILITY.md` がある
- `scripts/smoke-observability.sh` が executable である
- `make smoke-observability` が通る
- `go test ./backend/...` が通る
- `npm --prefix frontend run build` が通る
- `make binary` が通る
