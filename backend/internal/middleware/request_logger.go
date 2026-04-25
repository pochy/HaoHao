package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
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
