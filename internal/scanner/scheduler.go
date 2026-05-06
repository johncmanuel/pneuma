package scanner

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pneuma/internal/library"
	"pneuma/internal/media"
	"pneuma/internal/metadata/parser"
)

// Scheduler performs periodic full-directory reconciliation scans to catch any
// files that were missed by inotify (e.g. files added while the server was
// offline, or network-mounted paths that don't generate events).
type Scheduler struct {
	lib      *library.Service
	parser   *parser.Parser
	bus      EventBus
	ingestor *Ingestor
	dirs     []string
	interval time.Duration
	log      *slog.Logger
}

// NewScheduler creates a Scheduler that will scan dirs every interval.
func NewScheduler(lib *library.Service, p *parser.Parser, bus EventBus, dirs []string, interval time.Duration) *Scheduler {
	return &Scheduler{
		lib:      lib,
		parser:   p,
		bus:      bus,
		ingestor: NewIngestor(lib, p, bus),
		dirs:     dirs,
		interval: interval,
		log:      slog.Default().With("component", "scheduler"),
	}
}

// Start runs an immediate scan then repeats on interval until ctx is cancelled.
func (sc *Scheduler) Start(ctx context.Context) {
	sc.scan(ctx)
	ticker := time.NewTicker(sc.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sc.scan(ctx)
		}
	}
}

// ScanNow triggers an ad-hoc scan of all directories synchronously.
func (sc *Scheduler) ScanNow(ctx context.Context) {
	sc.scan(ctx)
}

// ScanAll triggers a scan using a background context (satisfies scanTrigger interface).
func (sc *Scheduler) ScanAll() {
	sc.scan(context.Background())
}

// ScanPath parses and upserts a single file, then publishes track.added or
// track.updated via the event bus. Used for post-upload metadata enrichment.
func (sc *Scheduler) ScanPath(path string) {
	ctx := context.Background()

	result, err := sc.ingestor.Ingest(ctx, path, nil)
	if err != nil {
		sc.log.Error("ScanPath ingest error", "path", path, "err", err)
		return
	}
	if result != nil && result.Skipped {
		sc.log.Info("ScanPath skipping duplicate (fingerprint match)", "path", path, "existing", result.DuplicatePath)
	}
}

// scan performs a full scan of all configured directories, parsing and upserting tracks as needed, and publishing events for added or updated tracks.
func (sc *Scheduler) scan(ctx context.Context) {
	sc.bus.Publish("scan.started", nil)
	start := time.Now()
	added, updated, removed := 0, 0, 0

	for _, dir := range sc.dirs {
		if _, err := os.Stat(dir); err != nil {
			sc.log.Warn("watch dir unavailable", "dir", dir, "err", err)
			continue
		}

		// TODO: Might not be memory efficient if the user has a large music directory since filepath.WalkDir
		// reads directories into memory before walking them. Find a more memory-efficient way to do this if it becomes an issue.
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}

			ext := strings.ToLower(filepath.Ext(path))
			if !media.IsSupportedAudio(ext) {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}

			info, err := d.Info()
			if err != nil {
				return nil
			}

			existing, err := sc.lib.TrackByPath(ctx, path)
			if err != nil {
				return nil
			}
			if existing != nil && existing.Fingerprint != "" && !info.ModTime().After(existing.LastModified) {
				return nil
			}

			result, err := sc.ingestor.Ingest(ctx, path, existing)
			if err != nil {
				sc.log.Error("ingest error in scan", "path", path, "err", err)
				return nil
			}
			if result != nil && result.Skipped {
				sc.log.Info("skipping duplicate (fingerprint match)", "path", path, "existing", result.DuplicatePath)
				return nil
			}

			if result != nil && result.IsNew {
				added++
			} else {
				updated++
			}

			return nil
		})
		if err != nil {
			sc.log.Error("walk error", "dir", dir, "err", err)
		}
	}

	sc.log.Info("scan complete",
		"duration", time.Since(start).Round(time.Millisecond),
		"added", added, "updated", updated, "removed", removed,
	)
	sc.bus.Publish("scan.completed", map[string]int{
		"added": added, "updated": updated, "removed": removed,
	})
}
