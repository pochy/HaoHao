package backend

import (
	"embed"
	"io/fs"
)

// FrontendDist holds the Vite build output that is embedded into the Go binary.
//
//go:embed web/dist
var FrontendDist embed.FS

func FrontendFS() (fs.FS, error) {
	return fs.Sub(FrontendDist, "web/dist")
}

