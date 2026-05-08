package desktop

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// App holds all desktop application state. It acts as a thin composition root:
// local file playback is always available; server connectivity is optional.
type App struct {
	ctx context.Context

	// store is the LocalStore backed by the app-local SQLite database.
	store *AppStore

	// scanner handles filesystem traversal, tag parsing, and DB upserts for local files.
	scanner *Scanner

	// Local stream server that serves local audio files to the player.
	localPort int
	localSrv  *http.Server

	// Directory for cached artwork thumbnails.
	thumbDir string

	// Optional server connection state.
	// Mutex used to prevent race conditions when concurrently reading/writing data like watchedRoots or pendingCreates
	mu          sync.RWMutex
	serverURL   string
	token       string
	stopRefresh context.CancelFunc

	// fsnotify watcher for local music folders.
	localWatcher *fsnotify.Watcher
	watchedRoots []string // root folders registered with the watcher

	// pendingCreates debounces rapid Create events for the same path (Linux
	// inotify routinely fires Create+Write+Chmod in quick succession for a
	// single file move).
	pendingCreates map[string]*time.Timer
}

// NewApp creates a new App.
func NewApp() *App {
	return &App{}
}
