//go:build !embed_frontend

package backend

import "io/fs"

func frontendDistFS() (fs.FS, error) {
	return nil, ErrFrontendNotEmbedded
}
