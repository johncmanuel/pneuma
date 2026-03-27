//go:build !no_embed

package web

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var embedded embed.FS

// FS returns the embedded web player filesystem rooted at "dist/".
func FS() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil
	}
	return sub
}
