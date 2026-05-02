package app

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

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
				"/api/v1/drive/items",
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
				"/api/v1/drive/items",
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
			blockPrefixes: []string{"/api/v1/", "/api/public/drive"},
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
			assertOpenAPIDocTags(t, spec, tt.surface)
			assertOpenAPIDocumentationContent(t, spec)
		})
	}
}

func TestNewOpenAPIExportRejectsInvalidSurface(t *testing.T) {
	_, err := NewOpenAPIExport(config.Config{}, backendapi.Surface("unknown"))
	if err == nil {
		t.Fatal("NewOpenAPIExport() error = nil")
	}
}

func TestRuntimeOpenAPIDocsRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	docsFS := fstest.MapFS{
		"CONCEPT.md":                   {Data: []byte("# Concept\n\n[Runbook](runbooks/drive-openfga-dr.md)")},
		"runbooks/drive-openfga-dr.md": {Data: []byte("# Drive OpenFGA DR\n\nBody")},
	}
	application := New(config.Config{
		AppName:      "HaoHao API",
		AppVersion:   "test",
		SCIMBasePath: "/api/scim/v2",
	}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, MarkdownDocsFS{FS: docsFS})

	tests := []struct {
		name          string
		path          string
		wantStatus    int
		wantContent   string
		wantFragments []string
	}{
		{
			name:        "docs html moved under docs openapi",
			path:        "/docs/openapi",
			wantStatus:  http.StatusOK,
			wantContent: "text/html",
			wantFragments: []string{
				`apiDescriptionUrl="/docs/openapi.yaml"`,
			},
		},
		{
			name:        "openapi json moved under docs",
			path:        "/docs/openapi.json",
			wantStatus:  http.StatusOK,
			wantContent: "application/openapi+json",
			wantFragments: []string{
				`"/api/v1/session"`,
				`"/api/v1/drive/items"`,
				`"/api/external/v1/me"`,
				`"/api/m2m/v1/self"`,
				`"/api/scim/v2/Users"`,
			},
		},
		{
			name:        "openapi yaml moved under docs",
			path:        "/docs/openapi.yaml",
			wantStatus:  http.StatusOK,
			wantContent: "application/openapi+yaml",
			wantFragments: []string{
				"/api/v1/session:",
				"/api/v1/drive/items:",
				"/api/external/v1/me:",
				"/api/m2m/v1/self:",
				"/api/scim/v2/Users:",
			},
		},
		{
			name:        "markdown docs index",
			path:        "/docs",
			wantStatus:  http.StatusOK,
			wantContent: "text/html",
			wantFragments: []string{
				"Markdown Docs",
				"Concept",
				"/docs/CONCEPT",
			},
		},
		{
			name:        "markdown docs document",
			path:        "/docs/CONCEPT",
			wantStatus:  http.StatusOK,
			wantContent: "text/html",
			wantFragments: []string{
				"<h1",
				"Concept",
				`href="/docs/runbooks/drive-openfga-dr"`,
			},
		},
		{
			name:        "markdown docs nested document",
			path:        "/docs/runbooks/drive-openfga-dr",
			wantStatus:  http.StatusOK,
			wantContent: "text/html",
			wantFragments: []string{
				"Drive OpenFGA DR",
			},
		},
		{
			name:       "old openapi json route removed",
			path:       "/openapi.json",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "old openapi yaml route removed",
			path:       "/openapi.yaml",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "old openapi 3 yaml route removed",
			path:       "/openapi-3.0.yaml",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			application.Router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if recorder.Code != tt.wantStatus {
				t.Fatalf("%s status = %d body = %s", tt.path, recorder.Code, recorder.Body.String())
			}
			if tt.wantContent != "" && !strings.HasPrefix(recorder.Header().Get("Content-Type"), tt.wantContent) {
				t.Fatalf("%s content-type = %q, want prefix %q", tt.path, recorder.Header().Get("Content-Type"), tt.wantContent)
			}
			body := recorder.Body.String()
			for _, want := range tt.wantFragments {
				if !strings.Contains(body, want) {
					t.Fatalf("%s is missing %s", tt.path, want)
				}
			}
		})
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

func assertOpenAPIDocumentationContent(t *testing.T, spec *huma.OpenAPI) {
	t.Helper()

	if spec.Info == nil || strings.TrimSpace(spec.Info.Description) == "" {
		t.Fatal("OpenAPI info.description is empty")
	}
	for _, term := range []string{"SESSION_ID", "X-CSRF-Token", "Idempotency-Key", "Problem Details"} {
		if !strings.Contains(spec.Info.Description, term) {
			t.Fatalf("OpenAPI info.description is missing %q", term)
		}
	}

	commonParams := map[string]struct{}{
		"SESSION_ID":        {},
		"X-CSRF-Token":      {},
		"Idempotency-Key":   {},
		"tenantSlug":        {},
		"filePublicId":      {},
		"folderPublicId":    {},
		"workspacePublicId": {},
		"limit":             {},
		"offset":            {},
	}

	operations := map[string]*huma.Operation{}
	forEachOpenAPIOperation(spec, func(method, path string, op *huma.Operation) {
		context := method + " " + path
		operations[op.OperationID] = op

		if strings.TrimSpace(op.Description) == "" {
			t.Fatalf("%s (%s) description is empty", context, op.OperationID)
		}

		for _, param := range op.Parameters {
			if param == nil {
				continue
			}
			if _, ok := commonParams[param.Name]; !ok {
				continue
			}
			if strings.TrimSpace(param.Description) == "" {
				t.Fatalf("%s (%s) common parameter %s description is empty", context, op.OperationID, param.Name)
			}
			if param.Example == nil && len(param.Examples) == 0 {
				t.Fatalf("%s (%s) common parameter %s example is missing", context, op.OperationID, param.Name)
			}
		}

		if op.RequestBody != nil {
			for contentType, media := range op.RequestBody.Content {
				if !isJSONOpenAPIContentType(contentType) {
					continue
				}
				if media == nil || (media.Example == nil && len(media.Examples) == 0) {
					t.Fatalf("%s (%s) JSON request body example is missing for %s", context, op.OperationID, contentType)
				}
			}
		}

		defaultResponse := op.Responses["default"]
		if defaultResponse == nil {
			t.Fatalf("%s (%s) default error response is missing", context, op.OperationID)
		}
		if strings.TrimSpace(defaultResponse.Description) == "" {
			t.Fatalf("%s (%s) default error response description is empty", context, op.OperationID)
		}
		problemMedia := defaultResponse.Content["application/problem+json"]
		if problemMedia == nil {
			t.Fatalf("%s (%s) default error response application/problem+json content is missing", context, op.OperationID)
		}
		if problemMedia.Example == nil && len(problemMedia.Examples) == 0 {
			t.Fatalf("%s (%s) default error response example is missing", context, op.OperationID)
		}
	})

	assertOperationJSONRequestExampleField(t, operations, "login", "email")
	assertOperationJSONRequestExampleField(t, operations, "createCustomerSignal", "customerName")
	assertOperationJSONRequestExampleField(t, operations, "createDriveFolder", "name")
	assertOperationJSONRequestExampleField(t, operations, "createWebhook", "url")
}

func assertOperationJSONRequestExampleField(t *testing.T, operations map[string]*huma.Operation, operationID, field string) {
	t.Helper()

	op := operations[operationID]
	if op == nil || op.RequestBody == nil {
		return
	}
	media := op.RequestBody.Content["application/json"]
	if media == nil {
		t.Fatalf("%s application/json request body is missing", operationID)
	}
	example, ok := openAPIMediaExampleValue(media).(map[string]any)
	if !ok {
		t.Fatalf("%s request example = %#v, want object", operationID, openAPIMediaExampleValue(media))
	}
	if _, ok := example[field]; !ok {
		t.Fatalf("%s request example is missing field %q: %#v", operationID, field, example)
	}
}

func openAPIMediaExampleValue(media *huma.MediaType) any {
	if media == nil {
		return nil
	}
	if media.Example != nil {
		return media.Example
	}
	if media.Examples["default"] != nil {
		return media.Examples["default"].Value
	}
	for _, example := range media.Examples {
		if example != nil {
			return example.Value
		}
	}
	return nil
}

func isJSONOpenAPIContentType(contentType string) bool {
	return contentType == "application/json" || contentType == "application/problem+json" || strings.HasSuffix(contentType, "+json")
}

func assertOpenAPIDocTags(t *testing.T, spec *huma.OpenAPI, surface backendapi.Surface) {
	t.Helper()

	gotRootTags := openAPITagNames(spec.Tags)
	wantRootTags := backendapi.OpenAPIDocTagNames(surface)
	if !slices.Equal(gotRootTags, wantRootTags) {
		t.Fatalf("root tags for %s = %#v, want %#v", surface, gotRootTags, wantRootTags)
	}

	allowedTags := map[string]struct{}{}
	for _, tag := range gotRootTags {
		allowedTags[tag] = struct{}{}
		assertNoLegacyDocTag(t, "root", tag)
	}

	forEachOpenAPIOperation(spec, func(method, path string, op *huma.Operation) {
		if len(op.Tags) != 1 {
			t.Fatalf("%s %s tags = %#v, want exactly one docs category tag", method, path, op.Tags)
		}
		tag := op.Tags[0]
		if _, ok := allowedTags[tag]; !ok {
			t.Fatalf("%s %s tag %q is not declared in root tags %#v", method, path, tag, gotRootTags)
		}
		assertNoLegacyDocTag(t, method+" "+path, tag)
	})
}

func openAPITagNames(tags []*huma.Tag) []string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	return names
}

func forEachOpenAPIOperation(spec *huma.OpenAPI, fn func(method, path string, op *huma.Operation)) {
	for path, item := range spec.Paths {
		for _, operation := range []struct {
			method string
			op     *huma.Operation
		}{
			{http.MethodGet, item.Get},
			{http.MethodPost, item.Post},
			{http.MethodPut, item.Put},
			{http.MethodPatch, item.Patch},
			{http.MethodDelete, item.Delete},
			{http.MethodHead, item.Head},
			{http.MethodOptions, item.Options},
			{http.MethodTrace, item.Trace},
		} {
			if operation.op != nil {
				fn(operation.method, path, operation.op)
			}
		}
	}
}

func assertNoLegacyDocTag(t *testing.T, context, tag string) {
	t.Helper()

	legacyTags := map[string]struct{}{
		"auth": {}, "customer-signal-filters": {}, "customer-signal-imports": {},
		"customer-signals": {}, "datasets": {}, "drive": {}, "drive-ai": {},
		"drive-clean-room": {}, "drive-collaboration": {}, "drive-e2ee": {},
		"drive-ediscovery": {}, "drive-gateway": {}, "drive-hsm": {},
		"drive-legal": {}, "drive-marketplace": {}, "drive-mobile": {},
		"drive-ocr": {}, "drive-office": {}, "drive-public": {}, "drive-sync": {},
		"entitlements": {}, "external": {}, "external-drive": {}, "files": {},
		"integrations": {}, "m2m": {}, "machine-clients": {}, "notifications": {},
		"scim": {}, "session": {}, "support-access": {}, "tenant-admin": {},
		"tenant-admin-drive": {}, "tenant-data-exports": {}, "tenant-invitations": {},
		"tenant-settings": {}, "tenants": {}, "todos": {}, "webhooks": {},
	}
	if _, ok := legacyTags[tag]; ok {
		t.Fatalf("%s still uses legacy OpenAPI tag %q", context, tag)
	}
}
