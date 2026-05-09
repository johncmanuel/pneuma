package desktop

import (
	"context"
)

// App holds all desktop application state. It acts as a thin composition root:
// local file playback is always available; server connectivity is optional.
type App struct {
	ctx context.Context

	// store is the LocalStore backed by the app-local SQLite database.
	store *AppStore

	// library manages filesystem synchronization, tag parsing, and DB upserts for local files.
	library *LibraryManager

	// streamer handles local audio streaming and artwork cache serving.
	streamer *LocalStreamer

	// watcher manages fsnotify events and library updates for watched folders.
	watcher *LocalWatcher

	// client manages the optional remote server connection and all outbound API calls.
	client *ServerClient

	// playlistManager owns all playlist business logic: CRUD, random generation, and remote syncing.
	playlistManager *PlaylistManager
}

// NewApp creates a new App.
func NewApp() *App {
	return &App{}
}
