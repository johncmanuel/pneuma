package scanner

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"pneuma/internal/fingerprint/chromaprint"
	"pneuma/internal/library"
	"pneuma/internal/metadata/parser"
)

// audioExts is the set of recognised audio file extensions.
var audioExts = map[string]bool{
	".mp3": true, ".flac": true, ".ogg": true, ".opus": true,
	".m4a": true, ".aac": true, ".wav": true, ".aiff": true,
	".wv": true, ".ape": true,
}

// EventBus is any type that can publish library change events.
type EventBus interface {
	Publish(eventType string, payload any)
}

// Watcher monitors directories for audio file changes using OS-level events.
type Watcher struct {
	lib     *library.Service
	parser  *parser.Parser
	fpcalc  *chromaprint.Service
	bus     EventBus
	watcher *fsnotify.Watcher
	mu      sync.Mutex
	pending map[string]time.Time
	log     *slog.Logger
}

// NewWatcher creates a Watcher.
func NewWatcher(lib *library.Service, p *parser.Parser, fp *chromaprint.Service, bus EventBus) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		lib:     lib,
		parser:  p,
		fpcalc:  fp,
		bus:     bus,
		watcher: fw,
		pending: make(map[string]time.Time),
		log:     slog.Default().With("component", "watcher"),
	}, nil
}

// Add registers a directory with the OS watcher.
func (w *Watcher) Add(dir string) error {
	return w.watcher.Add(dir)
}

// Start begins processing file events. It blocks until ctx is cancelled.
func (w *Watcher) Start(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	defer w.watcher.Close()
	w.log.Info("watcher started")

	for {
		select {
		case <-ctx.Done():
			w.log.Info("watcher stopped")
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.log.Error("fsnotify error", "err", err)
		case <-ticker.C:
			w.flush(ctx)
		}
	}
}

func (w *Watcher) handleEvent(e fsnotify.Event) {
	path := e.Name
	ext := strings.ToLower(filepath.Ext(path))
	if !audioExts[ext] {
		return
	}

	switch {
	case e.Op&(fsnotify.Create|fsnotify.Write) != 0:
		w.mu.Lock()
		if _, exists := w.pending[path]; !exists {
			w.pending[path] = time.Now()
		}
		w.mu.Unlock()
	case e.Op&fsnotify.Remove != 0:
		w.removeFile(context.Background(), path)
	case e.Op&fsnotify.Rename != 0:
		w.removeFile(context.Background(), path)
	}
}

func (w *Watcher) flush(ctx context.Context) {
	w.mu.Lock()
	now := time.Now()
	ready := make([]string, 0)
	for path, t := range w.pending {
		if now.Sub(t) >= time.Second {
			ready = append(ready, path)
			delete(w.pending, path)
		}
	}
	w.mu.Unlock()

	for _, path := range ready {
		w.ingestFile(ctx, path)
	}
}

func (w *Watcher) ingestFile(ctx context.Context, path string) {
	track, err := w.parser.ParseFile(ctx, path)
	if err != nil {
		w.log.Error("parse failed", "path", path, "err", err)
		return
	}

	existing, err := w.lib.TrackByPath(ctx, path)
	if err != nil {
		w.log.Error("lookup failed", "path", path, "err", err)
		return
	}
	isNew := existing == nil
	if existing != nil {
		track.ID = existing.ID
		track.CreatedAt = existing.CreatedAt
	}

	// ── Dedup: SHA-256 content hash (exact copies across folders) ────────────
	if hash, hashErr := contentHash(path); hashErr == nil {
		track.Fingerprint = hash
		if dup, _ := w.lib.TrackByFingerprint(ctx, hash); dup != nil && dup.Path != path {
			w.log.Info("skipping duplicate (content hash match)", "path", path, "existing", dup.Path)
			return
		}
	}

	// ── Dedup: acoustic fingerprint (perceptually-same songs) ─────────────────
	if w.fpcalc != nil && w.fpcalc.Available() {
		if res, fpErr := w.fpcalc.Fingerprint(ctx, path); fpErr == nil && res.Fingerprint != "" {
			if dup, _ := w.lib.TrackByFingerprint(ctx, res.Fingerprint); dup != nil && dup.Path != path {
				w.log.Info("skipping duplicate (acoustic match)", "path", path, "existing", dup.Path)
				return
			}
		}
	}

	if err := w.lib.UpsertTrack(ctx, track); err != nil {
		w.log.Error("upsert failed", "path", path, "err", err)
		return
	}
	w.log.Info("ingested", "path", path, "title", track.Title)
	if isNew {
		w.bus.Publish("track.added", track)
	} else {
		w.bus.Publish("track.updated", track)
	}
}

func (w *Watcher) removeFile(ctx context.Context, path string) {
	if err := w.lib.RemoveByPath(ctx, path); err != nil {
		w.log.Error("remove failed", "path", path, "err", err)
		return
	}
	w.log.Info("removed", "path", path)
	w.bus.Publish("track.removed", map[string]string{"path": path})
}
