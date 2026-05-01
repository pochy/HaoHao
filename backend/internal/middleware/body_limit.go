package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type BodyLimitOverride struct {
	Method     string
	PathPrefix string
	MaxBytes   int64
}

func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return BodyLimitWithOverrides(maxBytes, nil)
}

func BodyLimitWithOverrides(maxBytes int64, overrides []BodyLimitOverride) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := bodyLimitForRequest(c, maxBytes, overrides)
		if limit <= 0 || c.Request.Body == nil {
			c.Next()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}

func bodyLimitForRequest(c *gin.Context, fallback int64, overrides []BodyLimitOverride) int64 {
	if c == nil || c.Request == nil {
		return fallback
	}
	method := strings.ToUpper(strings.TrimSpace(c.Request.Method))
	path := c.Request.URL.Path
	for _, override := range overrides {
		if override.MaxBytes <= 0 {
			continue
		}
		if override.Method != "" && strings.ToUpper(strings.TrimSpace(override.Method)) != method {
			continue
		}
		if override.PathPrefix != "" && !strings.HasPrefix(path, override.PathPrefix) {
			continue
		}
		return override.MaxBytes
	}
	return fallback
}
