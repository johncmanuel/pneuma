//go:build no_embed

package web

import "io/fs"

// FS returns nil when compiled with the no_embed build tag.
func FS() fs.FS {
	return nil
}
