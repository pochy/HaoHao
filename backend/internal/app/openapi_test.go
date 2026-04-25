package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	backendapi "example.com/haohao/backend/internal/api"
	"example.com/haohao/backend/internal/config"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
)

func TestNewOpenAPIExportSurfaces(t *testing.T) {
	cfg := config.Config{
		AppName:      "HaoHao API",
		AppVersion:   "test",
		SCIMBasePath: "/api/scim/v2",
	}

	tests := []struct {
		name          string
		surface       backendapi.Surface
		wantPaths     []string
		blockPrefixes []string
		wantSchemes   []string
		blockSchemes  []string
	}{
		{
			name:    "full",
			surface: backendapi.SurfaceFull,
			wantPaths: []string{
				"/api/v1/session",
				"/api/external/v1/me",
				"/api/m2m/v1/self",
				"/api/scim/v2/Users",
			},
			wantSchemes: []string{"cookieAuth", "bearerAuth", "m2mBearerAuth"},
		},
		{
			name:    "browser",
			surface: backendapi.SurfaceBrowser,
			wantPaths: []string{
				"/api/v1/session",
				"/api/v1/csrf",
				"/api/v1/integrations",
				"/api/v1/machine-clients",
			},
			blockPrefixes: []string{"/api/external/", "/api/m2m/", "/api/scim/"},
			wantSchemes:   []string{"cookieAuth"},
			blockSchemes:  []string{"bearerAuth", "m2mBearerAuth"},
		},
		{
			name:    "external",
			surface: backendapi.SurfaceExternal,
			wantPaths: []string{
				"/api/external/v1/me",
				"/api/m2m/v1/self",
				"/api/scim/v2/Users",
			},
			blockPrefixes: []string{"/api/v1/"},
			wantSchemes:   []string{"bearerAuth", "m2mBearerAuth"},
			blockSchemes:  []string{"cookieAuth"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := NewOpenAPIExport(cfg, tt.surface)
			if err != nil {
				t.Fatalf("NewOpenAPIExport() error = %v", err)
			}
			if spec.OpenAPI != "3.1.0" {
				t.Fatalf("OpenAPI version = %q", spec.OpenAPI)
			}
			if len(spec.Paths) == 0 {
				t.Fatal("OpenAPI paths is empty")
			}

			for _, path := range tt.wantPaths {
				if _, ok := spec.Paths[path]; !ok {
					t.Fatalf("missing path %s", path)
				}
			}
			for _, prefix := range tt.blockPrefixes {
				if pathWithPrefix(spec.Paths, prefix) {
					t.Fatalf("blocked path prefix %s is present", prefix)
				}
			}
			for _, scheme := range tt.wantSchemes {
				if !hasSecurityScheme(spec, scheme) {
					t.Fatalf("missing security scheme %s", scheme)
				}
			}
			for _, scheme := range tt.blockSchemes {
				if hasSecurityScheme(spec, scheme) {
					t.Fatalf("blocked security scheme %s is present", scheme)
				}
			}
		})
	}
}

func TestNewOpenAPIExportRejectsInvalidSurface(t *testing.T) {
	_, err := NewOpenAPIExport(config.Config{}, backendapi.Surface("unknown"))
	if err == nil {
		t.Fatal("NewOpenAPIExport() error = nil")
	}
}

func TestRuntimeOpenAPIYAMLUsesFullSurface(t *testing.T) {
	gin.SetMode(gin.TestMode)

	application := New(config.Config{
		AppName:      "HaoHao API",
		AppVersion:   "test",
		SCIMBasePath: "/api/scim/v2",
	}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	application.Router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("/openapi.yaml status = %d body = %s", recorder.Code, recorder.Body.String())
	}

	body := recorder.Body.String()
	for _, want := range []string{"/api/v1/session:", "/api/external/v1/me:", "/api/m2m/v1/self:", "/api/scim/v2/Users:"} {
		if !strings.Contains(body, want) {
			t.Fatalf("/openapi.yaml is missing %s", want)
		}
	}
}

func pathWithPrefix(paths map[string]*huma.PathItem, prefix string) bool {
	for path := range paths {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func hasSecurityScheme(spec *huma.OpenAPI, name string) bool {
	return spec.Components != nil && spec.Components.SecuritySchemes != nil && spec.Components.SecuritySchemes[name] != nil
}
