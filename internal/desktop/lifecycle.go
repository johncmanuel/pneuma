package desktop

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	ThumbnailsCacheDir    = "thumbs"
	ThumbnailsTempDirProd = "pneuma-thumbs"
	ThumbnailsTempDirDev  = "pneuma-dev-thumbs"
	// Use a local-only HTTP server on a random port for streaming local files.
	LocalHTTPServerAddr = "127.0.0.1:0"
)

// Startup is called when the app is starting up.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	profile := resolveDesktopProfile(ctx)

	buildType := runtime.Environment(ctx).BuildType
	slog.Info("desktop storage profile resolved", "profile", profile, "build_type", buildType)

	if db, err := openAppDB(profile); err != nil {
		slog.Warn("failed to open database, state will not be persisted", "err", err)
	} else {
		a.store = NewAppStore(db)
	}

	var thumbDir string
	if cacheDir, err := os.UserCacheDir(); err == nil {
		thumbDir = filepath.Join(cacheDir, desktopAppDir(profile), ThumbnailsCacheDir)
	} else {
		thumbDir = filepath.Join(os.TempDir(), thumbnailsTempDir(profile))
		slog.Warn("UserCacheDir unavailable, using temp dir for thumbnails", "dir", thumbDir)
	}
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		slog.Error("failed to create thumbnail cache dir", "dir", thumbDir, "err", err)
	}

	a.streamer = NewLocalStreamer(thumbDir)
	port, err := a.streamer.Start()
	if err != nil {
		slog.Error("local stream server failed to start", "err", err)
	}

	// create scanner once store and wails ctx are initialized
	if a.store != nil {
		a.scanner = NewScanner(a.ctx, a.store)
	}

	if w, err := NewLocalWatcher(a.ctx, a.store, a.scanner); err != nil {
		slog.Warn("local file watcher unavailable", "err", err)
	} else {
		a.watcher = w
	}

	slog.Info("pneuma desktop started", "local_stream_port", port)
}

// Shutdown is called when the app is closing.
func (a *App) Shutdown(_ context.Context) {
	a.mu.Lock()
	if a.stopRefresh != nil {
		a.stopRefresh()
	}
	a.mu.Unlock()

	if a.watcher != nil {
		a.watcher.Close()
	}
	if a.streamer != nil {
		a.streamer.Stop(context.Background())
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
