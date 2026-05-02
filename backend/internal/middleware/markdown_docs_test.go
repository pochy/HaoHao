package middleware

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func TestMarkdownDocsCatalog(t *testing.T) {
	docsFS := fstest.MapFS{
		"b.md":                  {Data: []byte("No heading\n")},
		"a.md":                  {Data: []byte("# Alpha\n")},
		"runbooks/dr.md":        {Data: []byte("# Disaster Recovery\n")},
		"runbooks/ignored.txt":  {Data: []byte("ignored")},
		"openapi.md":            {Data: []byte("# Reserved\n")},
		".hidden/secret.md":     {Data: []byte("# Hidden\n")},
		"openapi-guide/doc.md":  {Data: []byte("# Reserved subtree\n")},
		"runbooks/openapi.md":   {Data: []byte("# Nested OpenAPI allowed\n")},
		"runbooks/no-title.md":  {Data: []byte("plain text")},
		"runbooks/sub/next.md":  {Data: []byte("# Next\n")},
		"runbooks/sub/skip.txt": {Data: []byte("skip")},
	}

	docs, err := markdownDocsCatalog(docsFS)
	if err != nil {
		t.Fatalf("markdownDocsCatalog() error = %v", err)
	}

	gotIDs := make([]string, 0, len(docs))
	gotTitles := map[string]string{}
	for _, doc := range docs {
		gotIDs = append(gotIDs, doc.ID)
		gotTitles[doc.ID] = doc.Title
	}
	wantIDs := []string{"a", "b", "runbooks/dr", "runbooks/no-title", "runbooks/openapi", "runbooks/sub/next"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("ids = %#v, want %#v", gotIDs, wantIDs)
	}
	if gotTitles["a"] != "Alpha" {
		t.Fatalf("title for a = %q", gotTitles["a"])
	}
	if gotTitles["b"] != "b" {
		t.Fatalf("fallback title for b = %q", gotTitles["b"])
	}
}

func TestMarkdownDocIDValidation(t *testing.T) {
	tests := []struct {
		filePath string
		wantID   string
		wantOK   bool
	}{
		{filePath: "CONCEPT.md", wantID: "CONCEPT", wantOK: true},
		{filePath: "runbooks/drive-openfga-dr.md", wantID: "runbooks/drive-openfga-dr", wantOK: true},
		{filePath: "../secret.md", wantOK: false},
		{filePath: "openapi.md", wantOK: false},
		{filePath: "openapi-guide/intro.md", wantOK: false},
		{filePath: "notes.txt", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			gotID, gotOK := markdownDocIDFromPath(tt.filePath)
			if gotOK != tt.wantOK || gotID != tt.wantID {
				t.Fatalf("markdownDocIDFromPath(%q) = %q, %t; want %q, %t", tt.filePath, gotID, gotOK, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestRenderMarkdownDocContent(t *testing.T) {
	docsFS := fstest.MapFS{
		"CONCEPT.md":       {Data: []byte("# Concept\n")},
		"guide/current.md": {Data: []byte("# Current\n")},
	}
	content := []byte(`# Title

| A | B |
| --- | --- |
| 1 | 2 |

` + "```go\nfmt.Println(\"hello\")\n```\n\n" + `<script>alert("x")</script>

[Concept](../CONCEPT.md)
[External](https://example.com/README.md)
`)

	html, err := renderMarkdownDocContent(docsFS, "guide/current", content)
	if err != nil {
		t.Fatalf("renderMarkdownDocContent() error = %v", err)
	}
	for _, want := range []string{"<h1", "<table>", "<pre><code", `href="/docs/CONCEPT"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered html is missing %q:\n%s", want, html)
		}
	}
	if strings.Contains(html, "<script>") {
		t.Fatalf("raw script tag was rendered:\n%s", html)
	}
	if !strings.Contains(html, "https://example.com/README.md") {
		t.Fatalf("external markdown link should remain unchanged:\n%s", html)
	}
}

func TestMarkdownDocsMiddlewareRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	docsFS := fstest.MapFS{
		"CONCEPT.md":                   {Data: []byte("# Concept\n\nBody")},
		"runbooks/drive-openfga-dr.md": {Data: []byte("# Drive DR\n\nBody")},
	}
	router := gin.New()
	router.Use(MarkdownDocs(MarkdownDocsConfig{FS: docsFS}))
	router.GET("/docs/openapi", func(c *gin.Context) {
		c.String(http.StatusOK, "openapi")
	})

	tests := []struct {
		path      string
		wantCode  int
		wantBody  string
		blockBody string
	}{
		{path: "/docs", wantCode: http.StatusOK, wantBody: "Concept"},
		{path: "/docs/CONCEPT", wantCode: http.StatusOK, wantBody: "<h1"},
		{path: "/docs/runbooks/drive-openfga-dr", wantCode: http.StatusOK, wantBody: "Drive DR"},
		{path: "/docs/openapi", wantCode: http.StatusOK, wantBody: "openapi", blockBody: "Markdown Docs"},
		{path: "/docs/missing", wantCode: http.StatusNotFound},
		{path: "/docs/../CONCEPT", wantCode: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if recorder.Code != tt.wantCode {
				t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
			}
			if tt.wantBody != "" && !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Fatalf("body is missing %q:\n%s", tt.wantBody, recorder.Body.String())
			}
			if tt.blockBody != "" && strings.Contains(recorder.Body.String(), tt.blockBody) {
				t.Fatalf("body contains blocked %q:\n%s", tt.blockBody, recorder.Body.String())
			}
		})
	}
}
