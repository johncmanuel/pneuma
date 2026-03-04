package artwork

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Cache manages the artwork disk cache directory, enforcing a size ceiling via
// LRU eviction of the oldest (by mtime) cached files.
type Cache struct {
	dir      string
	maxBytes int64
	log      *slog.Logger
}

// NewCache creates a Cache rooted at dir with a maxMB megabyte ceiling.
func NewCache(dir string, maxMB int) *Cache {
	return &Cache{
		dir:      dir,
		maxBytes: int64(maxMB) << 20,
		log:      slog.Default().With("component", "artwork-cache"),
	}
}

// Evict removes the oldest cached artwork files until total cache size is
// below the configured ceiling. Call periodically (e.g. after batch ingests).
func (c *Cache) Evict() error {
	type entry struct {
		path  string
		size  int64
		mtime time.Time
	}

	var files []entry
	var total int64

	err := filepath.WalkDir(c.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files = append(files, entry{path: path, size: info.Size(), mtime: info.ModTime()})
		total += info.Size()
		return nil
	})
	if err != nil {
		return fmt.Errorf("artwork cache walk: %w", err)
	}

	if total <= c.maxBytes {
		return nil
	}

	// Sort oldest-first so we remove least-recently-used files first.
	sort.Slice(files, func(i, j int) bool {
		return files[i].mtime.Before(files[j].mtime)
	})

	for _, f := range files {
		if total <= c.maxBytes {
			break
		}
		if err := os.Remove(f.path); err != nil {
			c.log.Warn("evict remove failed", "path", f.path, "err", err)
			continue
		}
		total -= f.size
		c.log.Debug("evicted", "path", f.path, "freed_kb", f.size>>10)
	}

	c.log.Info("cache eviction complete", "remaining_mb", total>>20)
	return nil
}

// SizeBytes returns the current total byte size of the artwork cache.
func (c *Cache) SizeBytes() (int64, error) {
	var total int64
	err := filepath.WalkDir(c.dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total, err
}
