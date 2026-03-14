package desktop

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// GetLocalPort returns the local stream server port.
func (a *App) GetLocalPort() int {
	return a.localPort
}

// Notify sends a desktop OS notification (logging fallback).
func (a *App) Notify(title, message string) {
	runtime.LogInfo(a.ctx, fmt.Sprintf("[notify] %s: %s", title, message))
}

// ClearArtworkCache removes all cached thumbnail files from the thumbs
// directory and resets the in-memory artwork hash cache. The cache is
// rebuilt on demand when artwork is next requested.
func (a *App) ClearArtworkCache() error {
	entries, err := os.ReadDir(a.thumbDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, e := range entries {
		_ = os.Remove(filepath.Join(a.thumbDir, e.Name()))
	}

	// Purge the in-memory hash map so subsequent requests regenerate thumbnails.
	artworkHashCache.Range(func(k, _ any) bool {
		artworkHashCache.Delete(k)
		return true
	})

	return nil
}
