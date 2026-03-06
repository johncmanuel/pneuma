package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	api "pneuma/internal/api/http"
	apws "pneuma/internal/api/ws"

	"pneuma/internal/config"
	"pneuma/internal/fingerprint/chromaprint"
	"pneuma/internal/library"
	"pneuma/internal/metadata/parser"
	"pneuma/internal/offline"
	"pneuma/internal/playback"
	"pneuma/internal/scanner"
	"pneuma/internal/store/sqlite"
	"pneuma/internal/user"
	"pneuma/web"
)

func main() {
	cfgPath := flag.String("config", config.DefaultPath(), "path to config.toml")
	flag.Parse()
	slog.SetDefault(slog.New(newConsoleHandler(os.Stdout, slog.LevelInfo)))

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}

	store, err := sqlite.Open(cfg.Database.Path)
	if err != nil {
		slog.Error("db open failed", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	hub := apws.New()
	libSvc := library.New(store)
	userSvc := user.New(store)
	metaParser := parser.New(cfg.Transcoding.FFmpegPath)
	fpcalcSvc := chromaprint.New(cfg.Transcoding.FpcalcPath)
	playEngine := playback.New(store, hub, libSvc)
	handoffSvc := playback.NewHandoff(store, playEngine)
	offlinePkg := offline.New(offlineDir(cfg), store, hub)

	watcher, err := scanner.NewWatcher(libSvc, metaParser, fpcalcSvc, hub)
	if err != nil {
		slog.Error("watcher init failed", "err", err)
		os.Exit(1)
	}
	for _, dir := range cfg.Library.WatchFolders {
		if err := watcher.Add(dir); err != nil {
			slog.Warn("watch folder unavailable", "dir", dir, "err", err)
		}
	}
	sched := scanner.NewScheduler(libSvc, metaParser, fpcalcSvc, hub, cfg.Library.WatchFolders, 15*time.Minute)

	router := api.NewRouter(api.Services{
		Library:    libSvc,
		User:       userSvc,
		Playback:   playEngine,
		Handoff:    handoffSvc,
		Offline:    offlinePkg,
		Hub:        hub,
		Store:      store,
		Scanner:    sched,
		JWTSecret:  cfg.Auth.SecretKey,
		UploadsDir: cfg.Upload.Dir,
		WebUI:      web.FS(),
	})

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{Addr: addr, Handler: router}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sched.Start(ctx)
	go watcher.Start(ctx)

	go func() {
		slog.Info("pneuma server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down...")
	cancel()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	srv.Shutdown(shutCtx) //nolint:errcheck
}

func offlineDir(cfg *config.Config) string {
	dbPath := cfg.Database.Path
	const suffix = "pneuma.db"
	if len(dbPath) > len(suffix) {
		return dbPath[:len(dbPath)-len(suffix)] + "offline"
	}
	return dbPath + "-offline"
}
