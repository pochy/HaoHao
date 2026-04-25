//go:build embed_frontend

package backend

import (
	"embed"
	"io/fs"
)

//go:embed web/dist
var embeddedFrontend embed.FS

func frontendDistFS() (fs.FS, error) {
	return fs.Sub(embeddedFrontend, "web/dist")
}
