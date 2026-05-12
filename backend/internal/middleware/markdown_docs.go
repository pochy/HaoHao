package middleware

import (
	"bytes"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	ghtml "github.com/yuin/goldmark/renderer/html"
)

type MarkdownDocsConfig struct {
	FS         fs.FS
	PathPrefix string
	Title      string
}

type markdownDoc struct {
	ID      string
	Path    string
	Title   string
	Snippet string
}

type markdownDocsPage struct {
	Title        string
	Docs         []markdownDoc
	Doc          markdownDoc
	Content      template.HTML
	SearchActive bool
	SearchQuery  string
}

type markdownDocFile struct {
	Doc  markdownDoc
	Data []byte
}

type markdownDocSearchMatch struct {
	Doc   markdownDoc
	Score int
}

var (
	markdownDocsLinkPattern = regexp.MustCompile(`\]\(([^)\s]+\.md(?:#[^)]+)?)\)`)
	markdownDocsMermaidCode = regexp.MustCompile(`(?s)<pre><code class="language-mermaid">(.*?)</code></pre>`)
	markdownDocsRenderer    = goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(ghtml.WithXHTML()),
	)
	markdownDocsTemplate = template.Must(template.New("markdown-docs").Parse(markdownDocsHTMLTemplate))
)

func MarkdownDocs(cfg MarkdownDocsConfig) gin.HandlerFunc {
	docsFS := cfg.FS
	pathPrefix := strings.TrimRight(strings.TrimSpace(cfg.PathPrefix), "/")
	if pathPrefix == "" {
		pathPrefix = "/docs"
	}
	title := strings.TrimSpace(cfg.Title)
	if title == "" {
		title = "HaoHao Docs"
	}

	return func(c *gin.Context) {
		if docsFS == nil || (c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead) {
			c.Next()
			return
		}

		requestPath := path.Clean(c.Request.URL.Path)
		switch {
		case requestPath == pathPrefix:
			renderMarkdownDocsIndex(c, docsFS, title)
		case strings.HasPrefix(requestPath, pathPrefix+"/"):
			docID := strings.TrimPrefix(requestPath, pathPrefix+"/")
			if strings.HasPrefix(strings.ToLower(docID), "openapi") {
				c.Next()
				return
			}
			renderMarkdownDoc(c, docsFS, title, docID)
		default:
			c.Next()
		}
	}
}

func renderMarkdownDocsIndex(c *gin.Context, docsFS fs.FS, title string) {
	searchQuery := strings.TrimSpace(c.Query("q"))
	var (
		docs []markdownDoc
		err  error
	)
	if searchQuery == "" {
		docs, err = markdownDocsCatalog(docsFS)
	} else {
		docs, err = searchMarkdownDocs(docsFS, searchQuery)
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to read docs")
		c.Abort()
		return
	}
	renderMarkdownDocsHTML(c, http.StatusOK, markdownDocsPage{
		Title:        title,
		Docs:         docs,
		SearchActive: searchQuery != "",
		SearchQuery:  searchQuery,
	})
}

func renderMarkdownDoc(c *gin.Context, docsFS fs.FS, title, rawDocID string) {
	docID, err := url.PathUnescape(rawDocID)
	if err != nil || !validMarkdownDocID(docID) {
		c.Status(http.StatusNotFound)
		c.Abort()
		return
	}

	filePath := docID + ".md"
	data, err := fs.ReadFile(docsFS, filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			c.Status(http.StatusNotFound)
			c.Abort()
			return
		}
		c.String(http.StatusInternalServerError, "failed to read doc")
		c.Abort()
		return
	}

	content, err := renderMarkdownDocContent(docsFS, docID, data)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to render doc")
		c.Abort()
		return
	}

	doc := markdownDoc{
		ID:    docID,
		Path:  filePath,
		Title: firstMarkdownHeading(data),
	}
	if doc.Title == "" {
		doc.Title = strings.TrimSuffix(path.Base(filePath), ".md")
	}

	renderMarkdownDocsHTML(c, http.StatusOK, markdownDocsPage{
		Title:   doc.Title + " - " + title,
		Doc:     doc,
		Content: template.HTML(content),
	})
}

func renderMarkdownDocsHTML(c *gin.Context, status int, page markdownDocsPage) {
	var body bytes.Buffer
	if err := markdownDocsTemplate.Execute(&body, page); err != nil {
		c.String(http.StatusInternalServerError, "failed to render docs")
		c.Abort()
		return
	}

	c.Header("Content-Security-Policy", "default-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'; img-src 'self' data:; style-src 'unsafe-inline'; script-src https://cdn.jsdelivr.net 'unsafe-inline'")
	c.Data(status, "text/html; charset=utf-8", body.Bytes())
	c.Abort()
}

func markdownDocsCatalog(docsFS fs.FS) ([]markdownDoc, error) {
	files, err := collectMarkdownDocFiles(docsFS)
	if err != nil {
		return nil, err
	}
	docs := make([]markdownDoc, 0, len(files))
	for _, file := range files {
		docs = append(docs, file.Doc)
	}
	return docs, nil
}

func collectMarkdownDocFiles(docsFS fs.FS) ([]markdownDocFile, error) {
	files := []markdownDocFile{}
	err := fs.WalkDir(docsFS, ".", func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if filePath != "." && strings.HasPrefix(path.Base(filePath), ".") {
				return fs.SkipDir
			}
			return nil
		}
		docID, ok := markdownDocIDFromPath(filePath)
		if !ok {
			return nil
		}
		data, err := fs.ReadFile(docsFS, filePath)
		if err != nil {
			return err
		}
		title := firstMarkdownHeading(data)
		if title == "" {
			title = strings.TrimSuffix(path.Base(filePath), ".md")
		}
		files = append(files, markdownDocFile{
			Doc: markdownDoc{
				ID:    docID,
				Path:  filePath,
				Title: title,
			},
			Data: data,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Doc.ID < files[j].Doc.ID
	})
	return files, nil
}

func searchMarkdownDocs(docsFS fs.FS, query string) ([]markdownDoc, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return markdownDocsCatalog(docsFS)
	}

	files, err := collectMarkdownDocFiles(docsFS)
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	matches := []markdownDocSearchMatch{}
	for _, file := range files {
		score, snippet, ok := scoreMarkdownDocSearchMatch(file.Doc, file.Data, query, queryLower)
		if !ok {
			continue
		}
		doc := file.Doc
		doc.Snippet = snippet
		matches = append(matches, markdownDocSearchMatch{Doc: doc, Score: score})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].Doc.ID < matches[j].Doc.ID
	})

	docs := make([]markdownDoc, 0, len(matches))
	for _, match := range matches {
		docs = append(docs, match.Doc)
	}
	return docs, nil
}

func scoreMarkdownDocSearchMatch(doc markdownDoc, data []byte, query, queryLower string) (int, string, bool) {
	if containsMarkdownSearchText(doc.Title, queryLower) {
		return 400, markdownSearchSnippet(doc.Title, query), true
	}
	if containsMarkdownSearchText(doc.Path, queryLower) {
		return 300, markdownSearchSnippet(doc.Path, query), true
	}
	for _, heading := range markdownHeadingTexts(data) {
		if containsMarkdownSearchText(heading, queryLower) {
			return 200, markdownSearchSnippet(heading, query), true
		}
	}
	body := string(data)
	if containsMarkdownSearchText(body, queryLower) {
		return 100, markdownSearchSnippet(body, query), true
	}
	return 0, "", false
}

func containsMarkdownSearchText(text, queryLower string) bool {
	return strings.Contains(strings.ToLower(text), queryLower)
}

func markdownHeadingTexts(data []byte) []string {
	headings := []string{}
	for _, line := range strings.Split(string(data), "\n") {
		heading, ok := markdownHeadingText(line)
		if ok {
			headings = append(headings, heading)
		}
	}
	return headings
}

func markdownHeadingText(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(trimmed) || trimmed[level] != ' ' {
		return "", false
	}
	return strings.TrimSpace(trimmed[level+1:]), true
}

func markdownSearchSnippet(text, query string) string {
	normalized := strings.Join(strings.Fields(text), " ")
	if normalized == "" {
		return ""
	}

	query = strings.TrimSpace(query)
	matchByte := strings.Index(strings.ToLower(normalized), strings.ToLower(query))
	if query == "" || matchByte < 0 || matchByte > len(normalized) || !utf8.ValidString(normalized[:matchByte]) {
		return truncateMarkdownSearchSnippet(normalized)
	}

	runes := []rune(normalized)
	matchRune := len([]rune(normalized[:matchByte]))
	queryRunes := len([]rune(query))
	start := matchRune - 72
	if start < 0 {
		start = 0
	}
	end := matchRune + queryRunes + 88
	if end > len(runes) {
		end = len(runes)
	}

	snippet := string(runes[start:end])
	if start > 0 {
		snippet = "..." + strings.TrimLeft(snippet, " ")
	}
	if end < len(runes) {
		snippet = strings.TrimRight(snippet, " ") + "..."
	}
	return snippet
}

func truncateMarkdownSearchSnippet(text string) string {
	runes := []rune(text)
	if len(runes) <= 180 {
		return text
	}
	return strings.TrimRight(string(runes[:180]), " ") + "..."
}

func markdownDocIDFromPath(filePath string) (string, bool) {
	cleaned := path.Clean(filePath)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) || path.Ext(cleaned) != ".md" {
		return "", false
	}
	docID := strings.TrimSuffix(cleaned, ".md")
	if !validMarkdownDocID(docID) {
		return "", false
	}
	return docID, true
}

func validMarkdownDocID(docID string) bool {
	cleaned := path.Clean(docID)
	if docID == "" || cleaned != docID || strings.HasPrefix(docID, "../") || strings.Contains(docID, "\\") {
		return false
	}
	segments := strings.Split(docID, "/")
	if strings.HasPrefix(strings.ToLower(segments[0]), "openapi") {
		return false
	}
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func firstMarkdownHeading(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return ""
}

func renderMarkdownDocContent(docsFS fs.FS, docID string, data []byte) (string, error) {
	markdown := rewriteMarkdownDocLinks(docsFS, docID, string(data))
	var body bytes.Buffer
	if err := markdownDocsRenderer.Convert([]byte(markdown), &body); err != nil {
		return "", err
	}
	return renderMarkdownMermaidBlocks(body.String()), nil
}

func renderMarkdownMermaidBlocks(html string) string {
	return markdownDocsMermaidCode.ReplaceAllString(html, `<div class="mermaid">$1</div>`)
}

func rewriteMarkdownDocLinks(docsFS fs.FS, docID, markdown string) string {
	return markdownDocsLinkPattern.ReplaceAllStringFunc(markdown, func(match string) string {
		target := strings.TrimSuffix(strings.TrimPrefix(match, "]("), ")")
		rewritten, ok := resolveMarkdownDocLink(docsFS, docID, target)
		if !ok {
			return match
		}
		return "](" + rewritten + ")"
	})
}

func resolveMarkdownDocLink(docsFS fs.FS, currentDocID, target string) (string, bool) {
	parsed, err := url.Parse(target)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || strings.HasPrefix(parsed.Path, "/") {
		return "", false
	}

	currentDir := path.Dir(currentDocID)
	if currentDir == "." {
		currentDir = ""
	}
	targetPath := path.Clean(path.Join(currentDir, parsed.Path))
	docID, ok := markdownDocIDFromPath(targetPath)
	if !ok {
		return "", false
	}
	if _, err := fs.Stat(docsFS, docID+".md"); err != nil {
		return "", false
	}

	docURL := "/docs/" + escapeMarkdownDocID(docID)
	if parsed.Fragment != "" {
		docURL += "#" + url.PathEscape(parsed.Fragment)
	}
	return docURL, true
}

func escapeMarkdownDocID(docID string) string {
	segments := strings.Split(docID, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

const markdownDocsHTMLTemplate = `<!doctype html>
<html lang="ja">
  <head>
    <meta charset="utf-8">
    <meta name="referrer" content="no-referrer">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{ .Title }}</title>
    <style>
      :root { color-scheme: light; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #24272e; background: #f6f7fb; }
      body { margin: 0; }
      a { color: #2457c5; text-decoration: none; }
      a:hover { text-decoration: underline; }
      .docs-shell { max-width: 1040px; margin: 0 auto; padding: 40px 24px 72px; }
      .docs-topbar { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 28px; }
      .docs-brand { font-size: 14px; font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; color: #5d6678; }
      .docs-links { display: flex; flex-wrap: wrap; gap: 12px; font-size: 14px; }
      .docs-card { background: #fff; border: 1px solid #dfe3ec; border-radius: 8px; box-shadow: 0 1px 2px rgba(20, 28, 45, 0.05); }
      .docs-index { padding: 28px; }
      .docs-title { margin: 0 0 10px; font-size: 32px; line-height: 1.2; }
      .docs-subtitle { margin: 0 0 24px; color: #5d6678; line-height: 1.6; }
      .docs-search { display: grid; gap: 8px; margin: 0 0 24px; }
      .docs-search label { font-size: 13px; font-weight: 700; color: #4d5668; }
      .docs-search-row { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; }
      .docs-search input { flex: 1 1 280px; min-width: 0; height: 40px; padding: 0 12px; border: 1px solid #cbd3e1; border-radius: 6px; color: #24272e; background: #fff; font: inherit; }
      .docs-search button, .docs-search-clear { display: inline-flex; align-items: center; justify-content: center; min-height: 40px; padding: 0 14px; border-radius: 6px; font: inherit; font-weight: 700; }
      .docs-search button { border: 1px solid #2457c5; color: #fff; background: #2457c5; cursor: pointer; }
      .docs-search-clear { border: 1px solid #dfe3ec; color: #42526b; background: #fbfcff; }
      .docs-search-count { margin: 0 0 16px; color: #4d5668; }
      .docs-empty { margin: 0; padding: 16px; border: 1px dashed #cbd3e1; border-radius: 8px; color: #5d6678; background: #fbfcff; }
      .docs-list { display: grid; gap: 10px; margin: 0; padding: 0; list-style: none; }
      .docs-list a { display: grid; gap: 4px; padding: 14px 16px; border: 1px solid #e5e8f0; border-radius: 8px; background: #fbfcff; }
      .docs-list strong { color: #24272e; }
      .docs-list span { color: #697386; font-size: 13px; }
      .docs-list .docs-snippet { color: #4d5668; font-size: 14px; line-height: 1.5; }
      .docs-article { padding: 32px; line-height: 1.75; overflow-wrap: anywhere; }
      .docs-article h1, .docs-article h2, .docs-article h3 { line-height: 1.25; margin: 1.8em 0 0.6em; }
      .docs-article h1:first-child { margin-top: 0; }
      .docs-article pre { overflow-x: auto; padding: 16px; border-radius: 8px; background: #111827; color: #f9fafb; }
      .docs-article code { font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace; font-size: 0.92em; }
      .docs-article :not(pre) > code { padding: 0.15em 0.35em; border-radius: 4px; background: #eef2f8; color: #172033; }
      .docs-article .mermaid { overflow-x: auto; margin: 1.5em 0; padding: 18px; border: 1px solid #dfe3ec; border-radius: 8px; background: #fbfcff; text-align: center; }
      .docs-article table { width: 100%; border-collapse: collapse; display: block; overflow-x: auto; }
      .docs-article th, .docs-article td { border: 1px solid #dfe3ec; padding: 8px 10px; text-align: left; }
      .docs-article blockquote { margin: 1.5em 0; padding-left: 1em; border-left: 4px solid #c5cede; color: #4d5668; }
      .docs-breadcrumb { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; margin-bottom: 16px; color: #697386; font-size: 14px; }
      @media (max-width: 700px) { .docs-shell { padding: 24px 14px 48px; } .docs-topbar { align-items: flex-start; flex-direction: column; } .docs-index, .docs-article { padding: 20px; } .docs-title { font-size: 26px; } }
    </style>
  </head>
  <body>
    <main class="docs-shell">
      <header class="docs-topbar">
        <div class="docs-brand">HaoHao Docs</div>
        <nav class="docs-links" aria-label="Documentation links">
          <a href="/docs">Markdown Docs</a>
          <a href="/docs/openapi">OpenAPI Reference</a>
        </nav>
      </header>
      {{ if .Content }}
        <nav class="docs-breadcrumb" aria-label="Breadcrumb">
          <a href="/docs">Docs</a>
          <span aria-hidden="true">/</span>
          <span>{{ .Doc.Path }}</span>
        </nav>
        <article class="docs-card docs-article">
          {{ .Content }}
        </article>
      {{ else }}
        <section class="docs-card docs-index">
          <h1 class="docs-title">Markdown Docs</h1>
          <p class="docs-subtitle">Repository の <code>docs/</code> 配下にある Markdown document の目次です。Markdown file を追加すると、次回 request または rebuild 後に自動でここへ表示されます。</p>
          <form class="docs-search" action="/docs" method="get" role="search">
            <label for="docs-search-query">Search Markdown docs</label>
            <div class="docs-search-row">
              <input id="docs-search-query" name="q" type="search" value="{{ .SearchQuery }}" placeholder="Search by title, path, heading, or body">
              <button type="submit">Search</button>
              {{ if .SearchActive }}<a class="docs-search-clear" href="/docs">Clear</a>{{ end }}
            </div>
          </form>
          {{ if .SearchActive }}
            <p class="docs-search-count">{{ len .Docs }} {{ if eq (len .Docs) 1 }}result{{ else }}results{{ end }} for <strong>{{ .SearchQuery }}</strong></p>
          {{ end }}
          {{ if .Docs }}
            <ul class="docs-list">
              {{ range .Docs }}
                <li>
                  <a href="/docs/{{ .ID }}">
                    <strong>{{ .Title }}</strong>
                    <span>{{ .Path }}</span>
                    {{ if .Snippet }}<span class="docs-snippet">{{ .Snippet }}</span>{{ end }}
                  </a>
                </li>
              {{ end }}
            </ul>
          {{ else if .SearchActive }}
            <p class="docs-empty">No Markdown docs matched your search.</p>
          {{ else }}
            <p class="docs-empty">No Markdown docs are available.</p>
          {{ end }}
        </section>
      {{ end }}
    </main>
    <script src="https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js"></script>
    <script>if (window.mermaid) { window.mermaid.initialize({ startOnLoad: true, securityLevel: "strict", theme: "default" }); }</script>
  </body>
</html>
`
