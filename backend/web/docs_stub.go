//go:build !embed_frontend

package web

import (
	"io/fs"
	"os"
)

func MarkdownDocsFS() (fs.FS, error) {
	return os.DirFS("docs"), nil
}
