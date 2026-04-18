package scanner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
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

	f, err := os.Open(path)
	if err != nil {
		sc.log.Error("ScanPath open error", "path", path, "err", err)
		return
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		sc.log.Error("ScanPath fingerprint error", "path", path, "err", err)
		return
	}

	fingerprint := hex.EncodeToString(h.Sum(nil))

	dup, err := sc.lib.TrackByFingerprint(ctx, fingerprint)
	if err != nil {
		sc.log.Error("ScanPath fingerprint lookup error", "path", path, "err", err)
		return
	}
	if dup != nil && dup.DeletedAt == nil && dup.Path != path {
		sc.log.Info("ScanPath skipping duplicate (fingerprint match)", "path", path, "existing", dup.Path)
		return
	}

	track.Fingerprint = fingerprint

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

		// preserve existing title if the parser fell back to the filename
		// (e.g. hash filename for uploads)
		baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if track.Title == baseName && existing.Title != "" {
			track.Title = existing.Title
		}
	}

	if err := sc.lib.UpsertTrack(ctx, track); err != nil {
		sc.log.Error("ScanPath upsert error", "path", path, "err", err)
		return
	}

	if isNew {
		sc.bus.Publish("track.added", compactTrackEventPayload(track))
	} else {
		sc.bus.Publish("track.updated", compactTrackEventPayload(track))
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

			track, err := sc.parser.ParseFile(ctx, path)
			if err != nil {
				sc.log.Error("parse error in scan", "path", path, "err", err)
				return nil
			}

			f, err := os.Open(path)
			if err != nil {
				sc.log.Error("ScanPath open error", "path", path, "err", err)
				return nil
			}
			defer f.Close()

			h := sha256.New()
			if _, err := io.Copy(h, f); err != nil {
				sc.log.Error("ScanPath fingerprint error", "path", path, "err", err)
				return nil
			}

			fingerprint := hex.EncodeToString(h.Sum(nil))
			track.Fingerprint = fingerprint
			isNew := existing == nil

			// Preserve stable identity fields so the upsert matches on ID.
			if existing != nil {
				track.ID = existing.ID
				track.CreatedAt = existing.CreatedAt
				track.UploadedByUserID = existing.UploadedByUserID

				baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
				if track.Title == baseName && existing.Title != "" {
					track.Title = existing.Title
				}
			}

			dup, err := sc.lib.TrackByFingerprint(ctx, fingerprint)
			if err != nil {
				sc.log.Error("fingerprint lookup error in scan", "path", path, "err", err)
				return nil
			}
			if dup != nil && dup.DeletedAt == nil && dup.Path != path {
				sc.log.Info("skipping duplicate (fingerprint match)", "path", path, "existing", dup.Path)
				return nil
			}

			if err := sc.lib.UpsertTrack(ctx, track); err != nil {
				sc.log.Error("upsert error in scan", "path", path, "err", err)
				return nil
			}

			if isNew {
				added++
				sc.bus.Publish("track.added", compactTrackEventPayload(track))
			} else {
				updated++
				sc.bus.Publish("track.updated", compactTrackEventPayload(track))
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
