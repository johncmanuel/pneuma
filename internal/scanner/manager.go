package scanner

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"pneuma/internal/library"
	"pneuma/internal/media"
	"pneuma/internal/metadata/parser"
)

// Manager is the single authority for translating filesystem state into
// library database state on the server side. It handles full lifecycle
// synchronization: scanning directories for new/updated files, pruning
// tracks that no longer exist on disk, and processing individual file
// events from the watcher.
type Manager struct {
	lib      *library.Service
	ingestor *Ingestor
	bus      EventBus
	log      *slog.Logger
}

// NewManager creates a Manager with the given library service, metadata
// parser, and event bus.
func NewManager(lib *library.Service, p *parser.Parser, bus EventBus) *Manager {
	return &Manager{
		lib:      lib,
		ingestor: NewIngestor(lib, p, bus),
		bus:      bus,
		log:      slog.Default().With("component", "scanner.manager"),
	}
}

// SyncFolder performs a full reconciliation scan of the directory, dir. It walks the
// directory tree, ingesting new and updated audio files, then prunes any
// tracks in the database whose files no longer exist on disk.
func (m *Manager) SyncFolder(ctx context.Context, dir string) (added, updated, removed int) {
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

		existing, err := m.lib.TrackByPath(ctx, path)
		if err != nil {
			return nil
		}
		if existing != nil && existing.Fingerprint != "" && !info.ModTime().After(existing.LastModified) {
			return nil
		}

		result, err := m.ingestor.Ingest(ctx, path, existing)
		if err != nil {
			m.log.Error("ingest error in sync", "path", path, "err", err)
			return nil
		}
		if result != nil && result.Skipped {
			m.log.Info("skipping duplicate (fingerprint match)", "path", path, "existing", result.DuplicatePath)
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
		m.log.Error("walk error", "dir", dir, "err", err)
	}

	// Prune tracks whose files were deleted while the server was offline.
	pruned := m.pruneStale(ctx, dir)
	removed = pruned

	return added, updated, removed
}

// HandleFileAdded processes a single newly created or modified audio file,
// parsing its metadata and upserting it into the library.
func (m *Manager) HandleFileAdded(ctx context.Context, path string) {
	result, err := m.ingestor.Ingest(ctx, path, nil)
	if err != nil {
		m.log.Error("ingest failed", "path", path, "err", err)
		return
	}
	if result != nil && result.Skipped {
		m.log.Info("skipping duplicate (fingerprint match)", "path", path, "existing", result.DuplicatePath)
		return
	}
	if result != nil && result.Track != nil {
		m.log.Info("ingested", "path", path, "title", result.Track.Title)
	}
}

// HandleFileRemoved deletes a track by its filesystem path and publishes
// a track.removed event.
func (m *Manager) HandleFileRemoved(ctx context.Context, path string) {
	track, _ := m.lib.TrackByPath(ctx, path)

	if err := m.lib.RemoveByPath(ctx, path); err != nil {
		m.log.Error("remove failed", "path", path, "err", err)
		return
	}
	m.log.Info("removed", "path", path)
	m.bus.Publish("track.removed", compactTrackEventPayload(track))
}

// ScanPath parses and upserts a single file, then publishes track.added or
// track.updated via the event bus. Used for post-upload metadata enrichment.
func (m *Manager) ScanPath(path string) {
	ctx := context.Background()

	result, err := m.ingestor.Ingest(ctx, path, nil)
	if err != nil {
		m.log.Error("ScanPath ingest error", "path", path, "err", err)
		return
	}
	if result != nil && result.Skipped {
		m.log.Info("ScanPath skipping duplicate (fingerprint match)", "path", path, "existing", result.DuplicatePath)
	}
}

// pruneStale queries all known paths under dir from the database, checks if
// each file still exists on disk, and removes the ones that don't. Returns the number
// of pruned files.
func (m *Manager) pruneStale(ctx context.Context, dir string) int {
	pattern := dir + string(filepath.Separator) + "%"
	paths, err := m.lib.ListPathsByPrefix(ctx, pattern)
	if err != nil {
		m.log.Error("pruneStale: list paths failed", "dir", dir, "err", err)
		return 0
	}

	pruned := 0
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			continue
		}

		track, _ := m.lib.TrackByPath(ctx, p)
		if err := m.lib.RemoveByPath(ctx, p); err != nil {
			m.log.Error("pruneStale: remove failed", "path", p, "err", err)
			continue
		}

		m.log.Info("pruned stale track", "path", p)
		m.bus.Publish("track.removed", compactTrackEventPayload(track))
		pruned++
	}

	return pruned
}
