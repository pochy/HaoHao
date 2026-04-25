package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func BrowserCORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		if len(allowed) == 0 {
			c.Next()
			return
		}

		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" && originAllowed(origin, allowed) {
			header := c.Writer.Header()
			header.Set("Access-Control-Allow-Origin", origin)
			header.Set("Access-Control-Allow-Credentials", "true")
			header.Add("Vary", "Origin")
			header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-CSRF-Token, Idempotency-Key")
			header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			header.Set("Access-Control-Max-Age", "600")
		}

		if c.Request.Method == http.MethodOptions {
			if origin == "" || !originAllowed(origin, allowed) {
				writeProblem(c, http.StatusForbidden, "origin is not allowed")
				return
			}
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}

		c.Next()
	}
}
