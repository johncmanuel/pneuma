package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	apws "pneuma/internal/api/ws"
	"pneuma/internal/artwork"
	"pneuma/internal/config"
	"pneuma/internal/fingerprint/chromaprint"
	"pneuma/internal/library"
	"pneuma/internal/metadata/parser"
	"pneuma/internal/offline"
	"pneuma/internal/playback"
	"pneuma/internal/scanner"
	"pneuma/internal/store/sqlite"
	"pneuma/internal/user"

	pneumahttp "pneuma/internal/api/http"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails application struct. Its exported methods are bound to the
// Svelte frontend via generated TypeScript stubs.
type App struct {
	ctx context.Context

	// Core services
	config  *config.Config
	store   *sqlite.Store
	hub     *apws.Hub
	libSvc  *library.Service
	userSvc *user.Service
	engine  *playback.Engine
	handoff *playback.Handoff
	offline *offline.Packager

	// Scanner
	watcher *scanner.Watcher
	sched   *scanner.Scheduler

	// Cancel signal for background goroutines
	cancel context.CancelFunc
}

// NewApp creates a new App.
func NewApp() *App {
	return &App{}
}

// startup is called by Wails when the application starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	bgCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	log := slog.Default()

	// ── Config ────────────────────────────────────────────────────────────────
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		log.Error("config load failed", "err", err)
		os.Exit(1)
	}
	a.config = cfg

	// ── Store ─────────────────────────────────────────────────────────────────
	store, err := sqlite.Open(cfg.Database.Path)
	if err != nil {
		log.Error("db open failed", "err", err)
		os.Exit(1)
	}
	a.store = store

	// ── Services ──────────────────────────────────────────────────────────────
	a.hub = apws.New()
	a.libSvc = library.New(store)
	a.userSvc = user.New(store)
	artworkFetcher := artwork.NewFetcher(cfg.Artwork.CacheDir, store)
	_ = artworkFetcher
	metaParser := parser.New(cfg.Transcoding.FFmpegPath)
	fpcalcSvc := chromaprint.New(cfg.Transcoding.FpcalcPath)
	a.engine = playback.New(store, a.hub, a.libSvc)
	a.handoff = playback.NewHandoff(store, a.engine)
	a.offline = offline.New(offlineDirFromCfg(cfg), store, a.hub)

	// ── Scanner ───────────────────────────────────────────────────────────────
	watcher, err := scanner.NewWatcher(a.libSvc, metaParser, fpcalcSvc, a.hub)
	if err != nil {
		log.Error("watcher init failed", "err", err)
	} else {
		for _, dir := range cfg.Library.WatchFolders {
			if err := watcher.Add(dir); err != nil {
				log.Warn("watch folder unavailable", "dir", dir, "err", err)
			}
		}
		a.watcher = watcher
		go watcher.Start(bgCtx)
	}

	a.sched = scanner.NewScheduler(a.libSvc, metaParser, fpcalcSvc, a.hub, cfg.Library.WatchFolders, 15*time.Minute)
	go a.sched.Start(bgCtx)

	// ── Embedded HTTP server (for WebSocket + audio streaming) ────────────────
	router := pneumahttp.NewRouter(pneumahttp.Services{
		Library:  a.libSvc,
		User:     a.userSvc,
		Playback: a.engine,
		Handoff:  a.handoff,
		Offline:  a.offline,
		Hub:      a.hub,
		Scanner:  a.sched,
	})
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		log.Info("embedded API listening", "addr", addr)
		if err := router.Start(addr); err != nil {
			log.Error("embedded API error", "err", err)
		}
	}()

	log.Info("pneuma desktop started")
}

// shutdown is called by Wails when the application is closing.
func (a *App) shutdown(ctx context.Context) {
	if a.cancel != nil {
		a.cancel()
	}
	if a.store != nil {
		a.store.Close() //nolint:errcheck
	}
}

// ─── Wails-bound methods (callable from Svelte) ───────────────────────────────

// GetServerPort returns the embedded API port for use in frontend fetch URLs.
func (a *App) GetServerPort() int {
	return a.config.Server.Port
}

// TriggerScan kicks off an ad-hoc library scan.
func (a *App) TriggerScan() {
	if a.sched != nil {
		go a.sched.ScanNow(context.Background())
	}
}

// Notify sends a desktop OS notification.
func (a *App) Notify(title, message string) {
	wailsruntime.LogInfo(a.ctx, fmt.Sprintf("[notify] %s: %s", title, message))
}

func offlineDirFromCfg(cfg *config.Config) string {
	dbPath := cfg.Database.Path
	const suffix = "pneuma.db"
	if len(dbPath) > len(suffix) {
		return dbPath[:len(dbPath)-len(suffix)] + "offline"
	}
	return dbPath + "-offline"
}
