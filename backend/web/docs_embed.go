//go:build embed_frontend

package web

import (
	"embed"
	"io/fs"
)

//go:embed dist/_docs
var embeddedMarkdownDocs embed.FS

func MarkdownDocsFS() (fs.FS, error) {
	return fs.Sub(embeddedMarkdownDocs, "dist/_docs")
}
