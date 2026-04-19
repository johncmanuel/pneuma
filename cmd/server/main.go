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
	"pneuma/internal/ingestion"
	"pneuma/internal/library"
	"pneuma/internal/media"
	"pneuma/internal/metadata/parser"
	"pneuma/internal/playback"
	"pneuma/internal/playlist"
	"pneuma/internal/scanner"
	"pneuma/internal/store/sqlite"
	"pneuma/internal/store/sqlite/serverdb"
	"pneuma/internal/user"
	"pneuma/web"
)

func main() {
	dataDir := flag.String("data", "", "path to data directory (default: $PNEUMA_DATA_DIR or ~/.pneuma)")
	cfgPath := flag.String("config", "", "path to config file (default: <data-dir>/config.toml)")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	dir := *dataDir
	if dir == "" {
		dir = config.DefaultDataDir()
	}

	cPath := *cfgPath
	if cPath == "" {
		cPath = filepath.Join(dir, config.ConfigFileName)
	}

	cfg, err := config.Load(cPath, dir)
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}

	store, err := sqlite.Open(cfg.Database.Path, cfg.Database.MaxOpenConns)
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
	transcoder := media.NewStreamTranscoder(media.TranscodeConfig{
		FFmpegPath:        cfg.Transcoding.FFmpegPath,
		CacheDir:          cfg.Transcoding.CacheDir,
		CacheMaxSizeMB:    cfg.Transcoding.CacheMaxSizeMB,
		MaxConcurrentJobs: cfg.Transcoding.MaxConcurrentJobs,
	})

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
	scanIntervalMinutes := cfg.Library.ScanIntervalMinutes
	if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = 120
	}
	scanInterval := time.Duration(scanIntervalMinutes) * time.Minute
	slog.Info("library scan interval configured", "minutes", scanIntervalMinutes)
	sched := scanner.NewScheduler(libSvc, metaParser, hub, cfg.Library.WatchFolders, scanInterval)

	// Clean up orphaned temp files from a previous crash
	tmpUploadsDir := filepath.Join(cfg.Upload.Dir, "tmp")
	if n := ingestion.CleanupTempUploads(tmpUploadsDir); n > 0 {
		slog.Info("cleaned up orphaned temp uploads", "count", n)
	} else {
		slog.Info("no temp uploads found, continuing")
	}

	iqQueue := ingestion.New(libSvc, queries, hub, sched, cfg.Upload.QueueCapacity)

	router := api.NewRouter(api.Services{
		Library:             libSvc,
		User:                userSvc,
		Playback:            playEngine,
		Playlist:            playlistSvc,
		Hub:                 hub,
		Queries:             queries,
		Scanner:             sched,
		JWTSecret:           cfg.Auth.SecretKey,
		UploadsDir:          cfg.Upload.Dir,
		ArtworkDir:          filepath.Join(dir, config.ConfigCachePlaylistArtDir),
		UploadMaxMB:         cfg.Upload.MaxSizeMB,
		IngestionQueue:      iqQueue,
		Transcoder:          transcoder,
		WebUI:               dashboard.FS(),
		WebPlayerUI:         web.FS(),
		RateLimitingEnabled: cfg.RateLimiting.Enabled,
	})

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go sched.Start(ctx)
	go watcher.Start(ctx)
	go store.RunOptimizePeriodically(ctx, 24*time.Hour)
	go iqQueue.Start(ctx)

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
