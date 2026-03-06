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
	"pneuma/internal/metadata/parser"
)

// Scheduler performs periodic full-directory reconciliation scans to catch any
// files that were missed by inotify (e.g. files added while the server was
// offline, or network-mounted paths that don't generate events).
type Scheduler struct {
	lib      *library.Service
	parser   *parser.Parser
	bus      EventBus
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

	track, err := sc.parser.ParseFile(ctx, path)
	if err != nil {
		sc.log.Error("ScanPath parse error", "path", path, "err", err)
		return
	}

	existing, err := sc.lib.TrackByPath(ctx, path)
	if err != nil {
		sc.log.Error("ScanPath db lookup error", "path", path, "err", err)
		return
	}

	isNew := existing == nil
	if existing != nil {
		// Preserve stable identity so the upsert overwrites rather than duplicates.
		track.ID = existing.ID
		track.CreatedAt = existing.CreatedAt
		track.UploadedByUserID = existing.UploadedByUserID
		// ParseFile doesn't compute content hashes — preserve the existing
		// fingerprint so we don't clobber upload-time SHA-256 values.
		if track.Fingerprint == "" && existing.Fingerprint != "" {
			track.Fingerprint = existing.Fingerprint
		}
	}

	if err := sc.lib.UpsertTrack(ctx, track); err != nil {
		sc.log.Error("ScanPath upsert error", "path", path, "err", err)
		return
	}

	if isNew {
		sc.bus.Publish("track.added", track)
	} else {
		sc.bus.Publish("track.updated", track)
	}
}

func (sc *Scheduler) scan(ctx context.Context) {
	sc.bus.Publish("scan.started", nil)
	start := time.Now()
	added, updated, removed := 0, 0, 0

	for _, dir := range sc.dirs {
		if _, err := os.Stat(dir); err != nil {
			sc.log.Warn("watch dir unavailable", "dir", dir, "err", err)
			continue
		}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if !audioExts[strings.ToLower(filepath.Ext(path))] {
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

			// Skip if file hasn't changed since last ingest.
			if existing != nil && !info.ModTime().After(existing.LastModified) {
				return nil
			}

			track, err := sc.parser.ParseFile(ctx, path)
			if err != nil {
				sc.log.Error("parse error in scan", "path", path, "err", err)
				return nil
			}

			isNew := existing == nil
			if existing != nil {
				// Preserve stable identity fields so the upsert matches on ID.
				track.ID = existing.ID
				track.CreatedAt = existing.CreatedAt
			}

			// ── Dedup: metadata match (title + artist + album + duration ±2 s) ────
			if dup, _ := sc.lib.DuplicateByMeta(ctx, track.Title, track.AlbumArtist, track.AlbumName, track.DurationMS, path); dup != nil {
				sc.log.Info("skipping duplicate (metadata match)", "path", path, "existing", dup.Path)
				return nil
			}

			if err := sc.lib.UpsertTrack(ctx, track); err != nil {
				sc.log.Error("upsert error in scan", "path", path, "err", err)
				return nil
			}

			if isNew {
				added++
				sc.bus.Publish("track.added", track)
			} else {
				updated++
				sc.bus.Publish("track.updated", track)
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
