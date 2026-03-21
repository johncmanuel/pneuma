//go:build no_embed

package dashboard

import "io/fs"

// FS returns nil when compiled with the no_embed build tag.
// The server will not serve the built-in web UI; serve it externally instead.
func FS() fs.FS {
	return nil
}
