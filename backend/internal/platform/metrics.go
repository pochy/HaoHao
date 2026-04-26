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
	outboxRunsTotal         *prometheus.CounterVec
	outboxDuration          *prometheus.HistogramVec
	outboxEventsTotal       *prometheus.CounterVec
	rateLimitTotal          *prometheus.CounterVec
	dataLifecycleRunsTotal  *prometheus.CounterVec
	dataLifecycleItemsTotal *prometheus.CounterVec
	fileQuotaExceededTotal  *prometheus.CounterVec
	openFGARequestsTotal    *prometheus.CounterVec
	openFGARequestDuration  *prometheus.HistogramVec
	driveAuthzDeniedTotal   *prometheus.CounterVec
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
		outboxRunsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "haohao",
			Name:        "outbox_runs_total",
			Help:        "Total number of outbox worker runs.",
			ConstLabels: constLabels,
		}, []string{"trigger", "status"}),
		outboxDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace:   "haohao",
			Name:        "outbox_duration_seconds",
			Help:        "Outbox worker run duration in seconds.",
			Buckets:     []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			ConstLabels: constLabels,
		}, []string{"trigger", "status"}),
		outboxEventsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "haohao",
			Name:        "outbox_events_total",
			Help:        "Total number of handled outbox events.",
			ConstLabels: constLabels,
		}, []string{"event_type", "status"}),
		rateLimitTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "haohao",
			Name:        "rate_limit_total",
			Help:        "Total number of rate limit decisions.",
			ConstLabels: constLabels,
		}, []string{"policy", "result"}),
		dataLifecycleRunsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "haohao",
			Name:        "data_lifecycle_runs_total",
			Help:        "Total number of data lifecycle job runs.",
			ConstLabels: constLabels,
		}, []string{"trigger", "status"}),
		dataLifecycleItemsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "haohao",
			Name:        "data_lifecycle_items_total",
			Help:        "Total number of data lifecycle affected rows.",
			ConstLabels: constLabels,
		}, []string{"kind"}),
		fileQuotaExceededTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "haohao",
			Name:        "file_quota_exceeded_total",
			Help:        "Total number of file uploads blocked by tenant quota.",
			ConstLabels: constLabels,
		}, []string{"purpose"}),
		openFGARequestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "haohao",
			Name:        "openfga_requests_total",
			Help:        "Total number of OpenFGA client requests.",
			ConstLabels: constLabels,
		}, []string{"operation", "result"}),
		openFGARequestDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace:   "haohao",
			Name:        "openfga_request_duration_seconds",
			Help:        "OpenFGA client request duration in seconds.",
			Buckets:     []float64{0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2},
			ConstLabels: constLabels,
		}, []string{"operation", "result"}),
		driveAuthzDeniedTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "haohao",
			Name:        "drive_authz_denied_total",
			Help:        "Total number of denied Drive authorization decisions.",
			ConstLabels: constLabels,
		}, []string{"operation", "resource_type"}),
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

func (m *Metrics) ObserveOutboxRun(trigger string, duration time.Duration, err error) {
	if m == nil {
		return
	}

	status := "ok"
	if err != nil {
		status = "error"
	}
	m.outboxRunsTotal.WithLabelValues(trigger, status).Inc()
	m.outboxDuration.WithLabelValues(trigger, status).Observe(duration.Seconds())
}

func (m *Metrics) IncOutboxEvent(eventType, status string) {
	if m == nil {
		return
	}
	m.outboxEventsTotal.WithLabelValues(eventType, status).Inc()
}

func (m *Metrics) IncRateLimit(policy, result string) {
	if m == nil {
		return
	}
	m.rateLimitTotal.WithLabelValues(policy, result).Inc()
}

func (m *Metrics) IncDataLifecycleRun(trigger string, err error) {
	if m == nil {
		return
	}
	status := "ok"
	if err != nil {
		status = "error"
	}
	m.dataLifecycleRunsTotal.WithLabelValues(trigger, status).Inc()
}

func (m *Metrics) IncDataLifecycleItems(kind string, count int64) {
	if m == nil || count <= 0 {
		return
	}
	m.dataLifecycleItemsTotal.WithLabelValues(kind).Add(float64(count))
}

func (m *Metrics) IncFileQuotaExceeded(purpose string) {
	if m == nil {
		return
	}
	m.fileQuotaExceededTotal.WithLabelValues(purpose).Inc()
}

func (m *Metrics) ObserveOpenFGARequest(operation string, duration time.Duration, err error) {
	if m == nil {
		return
	}
	switch operation {
	case "openfga_check", "openfga_write", "openfga_delete", "openfga_list_objects":
	default:
		operation = "openfga_check"
	}
	result := "ok"
	if err != nil {
		result = "error"
	}
	m.openFGARequestsTotal.WithLabelValues(operation, result).Inc()
	m.openFGARequestDuration.WithLabelValues(operation, result).Observe(duration.Seconds())
}

func (m *Metrics) IncDriveAuthzDenied(operation, resourceType string) {
	if m == nil {
		return
	}
	switch operation {
	case "can_view", "can_download", "can_edit", "can_delete", "can_share":
	default:
		operation = "unknown"
	}
	switch resourceType {
	case "file", "folder", "share_link":
	default:
		resourceType = "unknown"
	}
	m.driveAuthzDeniedTotal.WithLabelValues(operation, resourceType).Inc()
}
