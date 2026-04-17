package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/pochy/haohao/backend/internal/config"
)

func DocsAuthStub(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Placeholder for future Zitadel-backed docs authorization.
		// If a static bearer token is configured we require it, otherwise the
		// route stays open in local development.
		if cfg.DocsBearerToken == "" {
			c.Header("X-Docs-Auth-Mode", "stub-pass")
			c.Next()
			return
		}

		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token != cfg.DocsBearerToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "docs authorization required",
			})
			return
		}

		c.Next()
	}
}

