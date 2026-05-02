package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

func Recovery(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			path := c.FullPath()
			if path == "" && c.Request != nil && c.Request.URL != nil {
				path = c.Request.URL.Path
			}
			if path == "" {
				path = "unmatched"
			}

			attrs := []any{
				"log_type", "panic",
				"request_id", RequestIDFromContext(c),
				"method", c.Request.Method,
				"path", path,
				"panic", fmt.Sprint(recovered),
				"stack", string(debug.Stack()),
			}
			spanContext := trace.SpanContextFromContext(c.Request.Context())
			if spanContext.IsValid() {
				attrs = append(attrs,
					"trace_id", spanContext.TraceID().String(),
					"span_id", spanContext.SpanID().String(),
				)
			}

			logger.ErrorContext(c.Request.Context(), "panic recovered", attrs...)
			c.AbortWithStatus(http.StatusInternalServerError)
		}()

		c.Next()
	}
}
