package scanner

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"pneuma/internal/library"
	"pneuma/internal/media"
	"pneuma/internal/metadata/parser"
	"pneuma/internal/models"
)

// trackEventPayload represents the payload for track-related events, containing only the track ID for simplicity.
type trackEventPayload struct {
	// ID is the unique identifier of the track associated with the event.
	ID string `json:"id"`
}

// compactTrackEventPayload creates a trackEventPayload with only the ID field populated, used for events where only the track ID is needed.
func compactTrackEventPayload(track *models.Track) trackEventPayload {
	if track == nil {
		return trackEventPayload{}
	}
	return trackEventPayload{ID: track.ID}
}

// EventBus is any type that can publish library change events.
type EventBus interface {
	Publish(eventType string, payload any)
}

// Watcher monitors directories for audio file changes using OS-level events.
type Watcher struct {
	lib      *library.Service
	parser   *parser.Parser
	bus      EventBus
	ingestor *Ingestor
	watcher  *fsnotify.Watcher
	mu       sync.Mutex
	pending  map[string]time.Time
	log      *slog.Logger
}

// NewWatcher creates a Watcher.
func NewWatcher(lib *library.Service, p *parser.Parser, bus EventBus) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		lib:      lib,
		parser:   p,
		bus:      bus,
		ingestor: NewIngestor(lib, p, bus),
		watcher:  fw,
		pending:  make(map[string]time.Time),
		log:      slog.Default().With("component", "watcher"),
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

// handleEvent processes a single fsnotify event, adding create/write events
// to the pending map and handling deletes/renames immediately.
func (w *Watcher) handleEvent(e fsnotify.Event) {
	path := e.Name

	ext := strings.ToLower(filepath.Ext(path))
	if !media.IsSupportedAudio(ext) {
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

// flush processes pending file events that have been stable for at least 1 second.
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

// ingestFile parses and stores a track, handling duplicates and errors.
func (w *Watcher) ingestFile(ctx context.Context, path string) {
	result, err := w.ingestor.Ingest(ctx, path, nil)
	if err != nil {
		w.log.Error("ingest failed", "path", path, "err", err)
		return
	}
	if result != nil && result.Skipped {
		w.log.Info("skipping duplicate (fingerprint match)", "path", path, "existing", result.DuplicatePath)
		return
	}
	if result != nil && result.Track != nil {
		w.log.Info("ingested", "path", path, "title", result.Track.Title)
	}
}

// removeFile deletes a track by path and publishes an event.
func (w *Watcher) removeFile(ctx context.Context, path string) {
	track, _ := w.lib.TrackByPath(ctx, path)

	if err := w.lib.RemoveByPath(ctx, path); err != nil {
		w.log.Error("remove failed", "path", path, "err", err)
		return
	}
	w.log.Info("removed", "path", path)
	w.bus.Publish("track.removed", compactTrackEventPayload(track))
}
