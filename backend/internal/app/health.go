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
