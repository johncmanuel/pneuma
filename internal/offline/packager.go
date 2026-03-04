package offline

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"pneuma/internal/models"
	"pneuma/internal/store/sqlite"
)

const chunkSize = 512 * 1024 // 512 KiB

// EventBus can publish progress events.
type EventBus interface {
	Publish(eventType string, payload any)
}

// DownloadProgress is emitted during a download.
type DownloadProgress struct {
	TrackID    string  `json:"track_id"`
	Percent    float64 `json:"percent"`
	BytesTotal int64   `json:"bytes_total"`
	BytesDone  int64   `json:"bytes_done"`
}

// Packager handles offline sync operations.
type Packager struct {
	store    *sqlite.Store
	cacheDir string
	bus      EventBus
}

// New creates a Packager (called as offline.New in app.go / server).
func New(cacheDir string, store *sqlite.Store, bus EventBus) *Packager {
	return &Packager{
		store:    store,
		cacheDir: cacheDir,
		bus:      bus,
	}
}

// Download copies a track file into the offline cache with progress events.
// Signature: (ctx, track, userID) — matches router.go call site.
func (p *Packager) Download(ctx context.Context, track *models.Track, userID string) error {
	src, err := os.Open(track.Path)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return err
	}
	totalBytes := info.Size()

	dstDir := filepath.Join(p.cacheDir, userID)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	ext := filepath.Ext(track.Path)
	dstPath := filepath.Join(dstDir, track.ID+ext)

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer dst.Close()

	buf := make([]byte, chunkSize)
	var written int64
	for {
		select {
		case <-ctx.Done():
			os.Remove(dstPath)
			return ctx.Err()
		default:
		}
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, err := dst.Write(buf[:n]); err != nil {
				os.Remove(dstPath)
				return err
			}
			written += int64(n)
			if p.bus != nil {
				p.bus.Publish("download.progress", DownloadProgress{
					TrackID:    track.ID,
					Percent:    float64(written) / float64(totalBytes) * 100,
					BytesTotal: totalBytes,
					BytesDone:  written,
				})
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			os.Remove(dstPath)
			return readErr
		}
	}

	pack := &models.OfflinePack{
		ID:           track.ID,
		UserID:       userID,
		TrackID:      track.ID,
		LocalPath:    dstPath,
		DownloadedAt: time.Now(),
	}
	return p.store.UpsertOfflinePack(ctx, pack)
}

// Remove deletes an offline pack from disk and database.
// Signature: (ctx, userID, trackID) — matches router.go call site.
func (p *Packager) Remove(ctx context.Context, userID, trackID string) error {
	packs, err := p.store.ListOfflinePacks(ctx, userID)
	if err != nil {
		return err
	}
	for _, pk := range packs {
		if pk.TrackID == trackID {
			os.Remove(pk.LocalPath)
			break
		}
	}
	return p.store.DeleteOfflinePack(ctx, userID, trackID)
}

// ListPacks returns all offline packs for a user.
func (p *Packager) ListPacks(ctx context.Context, userID string) ([]*models.OfflinePack, error) {
	return p.store.ListOfflinePacks(ctx, userID)
}
