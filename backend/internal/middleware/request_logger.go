package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		latency := time.Since(startedAt)
		status := c.Writer.Status()
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		attrs := []any{
			"request_id", RequestIDFromContext(c),
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"latency_ms", float64(latency.Microseconds()) / 1000,
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}
		if value, ok := c.Get("error_type"); ok {
			attrs = append(attrs, "error_type", value)
		}
		if value, ok := c.Get("error_code"); ok {
			attrs = append(attrs, "error_code", value)
		}
		if value, ok := c.Get("error_detail"); ok {
			attrs = append(attrs, "error_detail", value)
		}
		spanContext := trace.SpanContextFromContext(c.Request.Context())
		if spanContext.IsValid() {
			attrs = append(attrs,
				"trace_id", spanContext.TraceID().String(),
				"span_id", spanContext.SpanID().String(),
			)
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
