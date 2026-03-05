//go:build !no_embed

package web

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var embedded embed.FS

// FS returns the embedded web UI filesystem rooted at "dist/".
// Returns nil if the dist directory is missing (build without frontend).
func FS() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil
	}
	return sub
}
