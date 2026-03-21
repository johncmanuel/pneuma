package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	api "pneuma/internal/api/http"
	apws "pneuma/internal/api/ws"

	"pneuma/dashboard"
	"pneuma/internal/config"
	"pneuma/internal/library"
	"pneuma/internal/metadata/parser"
	"pneuma/internal/playback"
	"pneuma/internal/playlist"
	"pneuma/internal/scanner"
	"pneuma/internal/store/sqlite"
	"pneuma/internal/store/sqlite/serverdb"
	"pneuma/internal/user"
)

func main() {
	dataDir := flag.String("data", "", "path to data directory (default: $PNEUMA_DATA_DIR or ~/.pneuma)")
	cfgPath := flag.String("config", "", "path to config.toml (default: <data-dir>/config.toml)")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	dir := *dataDir
	if dir == "" {
		dir = config.DefaultDataDir()
	}

	cPath := *cfgPath
	if cPath == "" {
		cPath = filepath.Join(dir, "config.toml")
	}

	cfg, err := config.Load(cPath, dir)
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

	slog.Info("database opened", "path", cfg.Database.Path)

	hub := apws.New()
	queries := serverdb.New(store.DB())
	libSvc := library.New(queries, store)
	userSvc := user.New(queries)
	metaParser := parser.New(cfg.Transcoding.FFmpegPath)
	playEngine := playback.New(queries, hub, libSvc)
	playlistSvc := playlist.New(queries)

	watcher, err := scanner.NewWatcher(libSvc, metaParser, hub)
	if err != nil {
		slog.Error("watcher init failed", "err", err)
		os.Exit(1)
	}
	for _, dir := range cfg.Library.WatchFolders {
		if err := watcher.Add(dir); err != nil {
			slog.Warn("watch folder unavailable", "dir", dir, "err", err)
		}
	}
	sched := scanner.NewScheduler(libSvc, metaParser, hub, cfg.Library.WatchFolders, 15*time.Minute)

	router := api.NewRouter(api.Services{
		Library:     libSvc,
		User:        userSvc,
		Playback:    playEngine,
		Playlist:    playlistSvc,
		Hub:         hub,
		Queries:     queries,
		Scanner:     sched,
		JWTSecret:   cfg.Auth.SecretKey,
		UploadsDir:  cfg.Upload.Dir,
		UploadMaxMB: cfg.Upload.MaxSizeMB,
		WebUI:       dashboard.FS(),
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
	srv.Shutdown(shutCtx)
}
