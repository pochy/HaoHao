package backend

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

var ErrFrontendNotEmbedded = errors.New("frontend dist is not embedded")

func RegisterFrontendRoutes(router *gin.Engine) error {
	distFS, err := frontendDistFS()
	if err != nil {
		return err
	}

	return registerFrontendRoutes(router, distFS)
}

func registerFrontendRoutes(router *gin.Engine, distFS fs.FS) error {
	if _, err := fs.Stat(distFS, "index.html"); err != nil {
		return err
	}

	indexHTML, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		return err
	}

	fileSystem := http.FS(distFS)

	router.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}

		requestPath := cleanFrontendRequestPath(c.Request.URL.Path)
		if isReservedFrontendPath(requestPath) {
			c.Status(http.StatusNotFound)
			return
		}

		if requestPath == "index.html" {
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
			return
		}

		if fileInfo, err := fs.Stat(distFS, requestPath); err == nil && !fileInfo.IsDir() {
			c.FileFromFS(requestPath, fileSystem)
			return
		}

		if shouldFallbackToIndex(requestPath) {
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
			return
		}

		c.Status(http.StatusNotFound)
	})

	return nil
}

func cleanFrontendRequestPath(requestPath string) string {
	cleaned := strings.TrimPrefix(path.Clean(requestPath), "/")
	if cleaned == "." || cleaned == "" {
		return "index.html"
	}

	return cleaned
}

func isReservedFrontendPath(requestPath string) bool {
	return requestPath == "api" ||
		strings.HasPrefix(requestPath, "api/") ||
		requestPath == "docs" ||
		strings.HasPrefix(requestPath, "docs/") ||
		requestPath == "schemas" ||
		strings.HasPrefix(requestPath, "schemas/") ||
		requestPath == "openapi" ||
		strings.HasPrefix(requestPath, "openapi.") ||
		strings.HasPrefix(requestPath, "openapi-") ||
		requestPath == "_docs" ||
		strings.HasPrefix(requestPath, "_docs/")
}

func shouldFallbackToIndex(requestPath string) bool {
	if strings.HasPrefix(requestPath, "assets/") {
		return false
	}

	return path.Ext(requestPath) == ""
}
