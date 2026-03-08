package desktop

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
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
}

// NewApp creates a new App.
func NewApp() *App {
	return &App{}
}
