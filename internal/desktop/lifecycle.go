package desktop

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"pneuma/internal/store/sqlite/desktopdb"
)

const (
	ThumbnailsCacheDir = "thumbs"
	ThumbnailsTempDir  = "pneuma-thumbs"
	// Use a local-only HTTP server on a random port for streaming local files.
	LocalHTTPServerAddr = "127.0.0.1:0"
)

// Startup is called when the app is starting up.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	if db, err := openAppDB(); err != nil {
		slog.Warn("failed to open database, state will not be persisted", "err", err)
	} else {
		a.appDB = db
		a.dq = desktopdb.New(db)
	}

	if cacheDir, err := os.UserCacheDir(); err == nil {
		a.thumbDir = filepath.Join(cacheDir, DesktopAppDirName, ThumbnailsCacheDir)
	} else {
		a.thumbDir = filepath.Join(os.TempDir(), ThumbnailsTempDir)
		slog.Warn("UserCacheDir unavailable, using temp dir for thumbnails", "dir", a.thumbDir)
	}
	if err := os.MkdirAll(a.thumbDir, 0o755); err != nil {
		slog.Error("failed to create thumbnail cache dir", "dir", a.thumbDir, "err", err)
	}

	listener, err := net.Listen("tcp", LocalHTTPServerAddr)
	if err != nil {
		slog.Error("local stream listener failed", "err", err)
		return
	}
	a.localPort = listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/local/stream", a.handleLocalStream)
	mux.HandleFunc("/local/art", a.handleLocalArt)
	mux.HandleFunc("/local/playlist-art", a.handlePlaylistArt)

	a.localSrv = &http.Server{Handler: mux}
	go func() {
		if err := a.localSrv.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Error("local stream server error", "err", err)
		}
	}()

	a.initLocalWatcher()

	slog.Info("pneuma desktop started", "local_stream_port", a.localPort)
}

// Shutdown is called when the app is closing.
func (a *App) Shutdown(_ context.Context) {
	a.mu.Lock()
	if a.stopRefresh != nil {
		a.stopRefresh()
	}
	a.mu.Unlock()

	a.stopLocalWatcher()
	if a.localSrv != nil {
		a.localSrv.Close()
	}
	a.closeAppDB()
}

// SecondInstanceLaunch is called by Wails when a second instance of the app is launched.
func (a *App) SecondInstanceLaunch(data options.SecondInstanceData) {
	slog.Info("second instance launched", "args", data.Args)
	slog.Info("opened from directory", "dir", data.WorkingDirectory)

	runtime.WindowUnminimise(a.ctx)
	runtime.Show(a.ctx)

	go runtime.EventsEmit(a.ctx, "launchArgs", data.Args)
}
