package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"

	backend "github.com/pochy/haohao/backend"
	browserv1 "github.com/pochy/haohao/backend/internal/api/browser/v1"
	"github.com/pochy/haohao/backend/internal/config"
	"github.com/pochy/haohao/backend/internal/middleware"
	"github.com/pochy/haohao/backend/internal/service"
)

type Application struct {
	Router *gin.Engine
	API    huma.API
}

func New(cfg config.Config) (*Application, error) {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	humaConfig := huma.DefaultConfig("HaoHao Browser API", cfg.Version)
	humaConfig.OpenAPIPath = ""
	humaConfig.DocsPath = ""
	humaConfig.Servers = []*huma.Server{
		{URL: "/", Description: "same-origin browser entrypoint"},
	}
	if humaConfig.Components == nil {
		humaConfig.Components = &huma.Components{}
	}
	humaConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"cookieAuth": {
			Type: "apiKey",
			In:   "cookie",
			Name: cfg.SessionCookieName,
		},
		"csrfHeader": {
			Type: "apiKey",
			In:   "header",
			Name: "X-CSRF-Token",
		},
	}

	api := humagin.New(router, humaConfig)
	registerBrowserAPI(api, cfg)
	registerDocsRoutes(router, api, cfg)
	registerFrontendRoutes(router)

	return &Application{
		Router: router,
		API:    api,
	}, nil
}

func registerBrowserAPI(api huma.API, cfg config.Config) {
	sessions := service.NewSessionService()

	browserv1.RegisterHealth(api, cfg)
	browserv1.RegisterSession(api, cfg, sessions)
}

func registerDocsRoutes(router *gin.Engine, api huma.API, cfg config.Config) {
	auth := middleware.DocsAuthStub(cfg)

	router.GET("/openapi.yaml", auth, func(c *gin.Context) {
		spec, err := api.OpenAPI().YAML()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Data(http.StatusOK, "application/yaml", spec)
	})

	router.GET("/openapi.json", auth, func(c *gin.Context) {
		spec, err := json.MarshalIndent(api.OpenAPI(), "", "  ")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Data(http.StatusOK, "application/json", spec)
	})

	router.GET("/docs", auth, func(c *gin.Context) {
		html := fmt.Sprintf(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>HaoHao API Docs</title>
    <style>
      body { font-family: ui-sans-serif, system-ui, sans-serif; margin: 2rem; line-height: 1.5; }
      code { background: #f4f4f5; padding: 0.2rem 0.35rem; border-radius: 0.25rem; }
    </style>
  </head>
  <body>
    <h1>HaoHao API Docs Placeholder</h1>
    <p>This route is reserved for authenticated OpenAPI documentation.</p>
    <p>Current mode: <code>%s</code></p>
    <p>Spec: <a href="/openapi.yaml">/openapi.yaml</a></p>
  </body>
</html>`, docsMode(cfg))

		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})
}

func registerFrontendRoutes(router *gin.Engine) {
	distFS, err := backend.FrontendFS()
	if err != nil {
		return
	}

	router.NoRoute(func(c *gin.Context) {
		if !acceptsHTML(c.Request) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		if path != "" {
			if body, ok := readAsset(distFS, path); ok {
				c.Data(http.StatusOK, detectContentType(path, body), body)
				return
			}
		}

		index, ok := readAsset(distFS, "index.html")
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "frontend bundle not found"})
			return
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
}

func docsMode(cfg config.Config) string {
	if cfg.DocsBearerToken == "" {
		return "stub-pass"
	}

	return "static-bearer"
}

func acceptsHTML(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}

	accept := r.Header.Get("Accept")
	return accept == "" || strings.Contains(accept, "text/html")
}

func readAsset(distFS fs.FS, name string) ([]byte, bool) {
	file, err := distFS.Open(name)
	if err != nil {
		return nil, false
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return nil, false
	}

	body, err := fs.ReadFile(distFS, name)
	if err != nil {
		return nil, false
	}

	return body, true
}

func detectContentType(name string, body []byte) string {
	if strings.HasSuffix(name, ".js") {
		return "text/javascript; charset=utf-8"
	}
	if strings.HasSuffix(name, ".css") {
		return "text/css; charset=utf-8"
	}
	if strings.HasSuffix(name, ".html") {
		return "text/html; charset=utf-8"
	}

	return http.DetectContentType(bytes.TrimSpace(body))
}

