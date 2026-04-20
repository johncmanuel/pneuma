// Package metrics provides disk usage measurement and periodic snapshotting.
package metrics

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v4/disk"

	"pneuma/internal/store/sqlite/dbconv"
	"pneuma/internal/store/sqlite/serverdb"
)

// Collector gathers disk usage metrics and persists them to the database.
type Collector struct {
	q              *serverdb.Queries
	dbPath         string // path to the main SQLite database file
	transDir       string // transcode cache directory
	playlistArtDir string // playlist artwork directory
	tracksArtDir   string // track artwork directory
}

// New creates a new disk metrics Collector.
func New(q *serverdb.Queries, dbPath, transcodeDir, playlistArtDir, tracksArtDir string) *Collector {
	return &Collector{q: q, dbPath: dbPath, transDir: transcodeDir, playlistArtDir: playlistArtDir, tracksArtDir: tracksArtDir}
}

// Snapshot takes a single disk usage measurement and writes it to the database.
func (c *Collector) Snapshot(ctx context.Context) error {
	trackBytes, err := c.q.SumTrackBytes(ctx)
	if err != nil {
		return err
	}

	// include WAL and SHM files too
	dbBytes := fileSize(c.dbPath) + fileSize(c.dbPath+"-wal") + fileSize(c.dbPath+"-shm")
	transBytes := dirSize(c.transDir)

	totalPlaylistArtDirBytes := dirSize(c.playlistArtDir)
	trackArtBytes := dirSize(c.tracksArtDir)

	// Subtract track art bytes from the total playlist art dir size so they are disjoint
	playlistArtBytes := totalPlaylistArtDirBytes - trackArtBytes
	if playlistArtBytes < 0 {
		playlistArtBytes = 0
	}

	var totalBytes, freeBytes uint64
	if usage, err := disk.UsageWithContext(ctx, filepath.Dir(c.dbPath)); err == nil {
		totalBytes = usage.Total
		freeBytes = usage.Free
	}

	return c.q.InsertDiskUsage(ctx, serverdb.InsertDiskUsageParams{
		ID:                  uuid.NewString(),
		TotalBytes:          int64(totalBytes),
		FreeBytes:           int64(freeBytes),
		TracksBytes:         trackBytes,
		DbBytes:             dbBytes,
		TranscodeCacheBytes: transBytes,
		ArtworkCacheBytes:   trackArtBytes,
		PlaylistArtBytes:    playlistArtBytes,
		RecordedAt:          dbconv.FormatTime(time.Now()),
	})
}

// RunPeriodically takes a snapshot every interval until ctx is cancelled.
// It also prunes old snapshots, keeping only the most recent maxHistory entries.
func (c *Collector) RunPeriodically(ctx context.Context, interval time.Duration, maxHistory int64) {
	if err := c.Snapshot(ctx); err != nil {
		slog.Warn("initial disk usage snapshot failed", "err", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.Snapshot(ctx); err != nil {
				slog.Warn("disk usage snapshot failed", "err", err)
			}
			if err := c.q.PruneDiskUsageHistory(ctx, maxHistory); err != nil {
				slog.Warn("disk usage prune failed", "err", err)
			}
		}
	}
}

// ClearCaches removes all files in the transcode and artwork cache directories.
func (c *Collector) ClearCaches() error {
	if err := ClearDir(c.transDir); err != nil {
		return err
	}
	return ClearDir(c.playlistArtDir)
}

// ClearDir removes all files within a directory (but keeps the directory itself).
func ClearDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// dirSize recursively calculates the total size of all files in a directory.
func dirSize(path string) int64 {
	var total int64
	filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// fileSize returns the size of a single file, or 0 if it doesn't exist.
func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
