// Package ingestion provides a serial write queue for track uploads.
// The handler writes the file to a tmp directory and enqueues a job;
// a single background worker drains the queue and commits to the db.
package ingestion

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"pneuma/internal/library"
	"pneuma/internal/models"
	"pneuma/internal/store/sqlite/dbconv"
	"pneuma/internal/store/sqlite/serverdb"
)

// scanTrigger is satisfied by *scanner.Scheduler.
type scanTrigger interface {
	ScanPath(path string)
}

// eventPublisher is satisfied by *ws.Hub.
type eventPublisher interface {
	Publish(eventType string, payload any)
}

// Job describes a single uploaded track that needs to be committed to the DB.
type Job struct {
	TmpPath   string        // temporary file path (uploads/tmp/<uuid><ext>)
	FinalPath string        // final file path (uploads/<hash><ext>)
	Track     *models.Track // pre-populated from tag read in handler
	UserID    string
	Filename  string // original filename for audit log
}

// Queue is a serial write queue for uploaded tracks.
type Queue struct {
	ch      chan Job
	lib     *library.Service
	q       *serverdb.Queries
	hub     eventPublisher
	scanner scanTrigger
	log     *slog.Logger
}

// New creates an ingestion Queue with a buffered channel of the given capacity.
func New(lib *library.Service, q *serverdb.Queries, hub eventPublisher, sc scanTrigger, capacity int) *Queue {
	return &Queue{
		ch:      make(chan Job, capacity),
		lib:     lib,
		q:       q,
		hub:     hub,
		scanner: sc,
		log:     slog.Default().With("component", "ingestion"),
	}
}

// Enqueue adds a job to the queue. Returns an error if the queue is full.
func (iq *Queue) Enqueue(job Job) error {
	select {
	case iq.ch <- job:
		return nil
	default:
		return fmt.Errorf("ingestion queue full (%d)", cap(iq.ch))
	}
}

// Start drains the queue serially until ctx is cancelled.
func (iq *Queue) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-iq.ch:
			iq.process(ctx, job)
		}
	}
}

// process handles a single queue job.
func (iq *Queue) process(_ context.Context, job Job) {
	ctx := context.Background()

	// rename temp to final; atomic operation btw
	if err := os.Rename(job.TmpPath, job.FinalPath); err != nil {
		iq.log.Error("rename temp to final failed", "tmp", job.TmpPath, "final", job.FinalPath, "err", err)
		_ = os.Remove(job.TmpPath)
		return
	}

	job.Track.Path = job.FinalPath

	// upsert + audit in one logical step
	if err := iq.lib.UpsertTrack(ctx, job.Track); err != nil {
		iq.log.Error("upsert track failed", "path", job.FinalPath, "err", err)
		return
	}

	now := time.Now()
	_ = iq.q.InsertAuditEntry(ctx, serverdb.InsertAuditEntryParams{
		ID:         uuid.NewString(),
		UserID:     job.UserID,
		Action:     "upload",
		TargetType: "track",
		TargetID:   job.Track.ID,
		Detail:     dbconv.NullStr(job.Filename),
		CreatedAt:  dbconv.FormatTime(now),
	})

	iq.hub.Publish(string(models.EventTrackAdded), map[string]string{"id": job.Track.ID})

	go iq.scanner.ScanPath(job.FinalPath)

	iq.adoptStagedLrc(job)

	iq.log.Info("track ingested", "id", job.Track.ID, "path", job.FinalPath)
}

// adoptStagedLrc looks for a staged .lrc file in the uploads directory that
// matches the original filename of the uploaded track, and moves it next to
// the ingested audio file.
func (iq *Queue) adoptStagedLrc(job Job) {
	if job.Filename == "" {
		return
	}

	uploadsDir := filepath.Dir(job.FinalPath)

	// The staged .lrc filename matches the original upload filename with .lrc appended.
	// e.g., "song.flac" -> "song.flac.lrc" or "song.lrc"
	base := strings.TrimSuffix(job.Filename, filepath.Ext(job.Filename))

	candidates := []string{
		filepath.Join(uploadsDir, base+".lrc"),
		filepath.Join(uploadsDir, job.Filename+".lrc"),
	}

	lrcDest := strings.TrimSuffix(job.FinalPath, filepath.Ext(job.FinalPath)) + ".lrc"

	// find the first candidate that exists and move it to lrcDest
	for _, staged := range candidates {
		if _, err := os.Stat(staged); err == nil {
			if err := os.Rename(staged, lrcDest); err != nil {
				iq.log.Error("failed to adopt staged .lrc", "src", staged, "dst", lrcDest, "err", err)
			} else {
				iq.log.Info("adopted staged .lrc", "src", staged, "dst", lrcDest)
			}
			return
		}
	}
}

// CleanupTempUploads removes all files from the temp upload directory.
// Ensure this is called at server boot before starting the queue worker.
// Returns the number of files removed.
func CleanupTempUploads(tmpDir string) int {
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(tmpDir, e.Name())); err == nil {
			removed++
		}
	}
	return removed
}
