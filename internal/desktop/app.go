package desktop

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// App holds all desktop application state. It acts as a thin client —
// local file playback is always available; server connectivity is optional.
type App struct {
	ctx context.Context

	// App-local SQLite database for persisting desktop client state.
	appDB *sql.DB

	// Local stream server (serves local audio files to the <audio> element).
	localPort int
	localSrv  *http.Server

	// Directory for cached artwork thumbnails.
	thumbDir string

	// Optional server connection state (guarded by mu).
	mu          sync.RWMutex
	serverURL   string
	token       string
	stopRefresh context.CancelFunc

	// fsnotify watcher for local music folders.
	localWatcher *fsnotify.Watcher
	watchedRoots []string // root folders registered with the watcher; guarded by mu
	// pendingCreates debounces rapid Create events for the same path (Linux
	// inotify routinely fires Create+Write+Chmod in quick succession for a
	// single file move). Guarded by mu.
	pendingCreates map[string]*time.Timer
}

// NewApp creates a new App.
func NewApp() *App {
	return &App{}
}
