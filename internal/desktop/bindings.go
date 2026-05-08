package desktop

import (
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// GetLocalPort returns the local stream server port.
func (a *App) GetLocalPort() int {
	if a.streamer != nil {
		return a.streamer.Port()
	}
	return 0
}

// Notify sends a desktop OS notification (logging fallback).
func (a *App) Notify(title, message string) {
	runtime.LogInfo(a.ctx, fmt.Sprintf("[notify] %s: %s", title, message))
}

// ClearArtworkCache removes all cached thumbnail files from the thumbs
// directory and resets the in-memory artwork hash cache. The cache is
// rebuilt on demand when artwork is next requested.
func (a *App) ClearArtworkCache() error {
	if a.streamer != nil {
		return a.streamer.ClearArtworkCache()
	}
	return nil
}
