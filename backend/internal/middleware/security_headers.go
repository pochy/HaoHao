package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

type SecurityHeadersConfig struct {
	Enabled     bool
	CSP         string
	HSTSEnabled bool
	HSTSMaxAge  int
}

func SecurityHeaders(cfg SecurityHeadersConfig) gin.HandlerFunc {
	csp := strings.TrimSpace(cfg.CSP)
	if csp == "" {
		csp = "default-src 'self'; base-uri 'self'; frame-ancestors 'none'; object-src 'none'"
	}
	hsts := ""
	if cfg.HSTSEnabled && cfg.HSTSMaxAge > 0 {
		hsts = fmt.Sprintf("max-age=%d; includeSubDomains", cfg.HSTSMaxAge)
	}

	return func(c *gin.Context) {
		if cfg.Enabled {
			header := c.Writer.Header()
			header.Set("Content-Security-Policy", csp)
			header.Set("X-Content-Type-Options", "nosniff")
			header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			header.Set("X-Frame-Options", "DENY")
			if hsts != "" {
				header.Set("Strict-Transport-Security", hsts)
			}
		}
		c.Next()
	}
}
