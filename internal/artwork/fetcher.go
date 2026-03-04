package artwork

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"pneuma/internal/models"
	"pneuma/internal/store/sqlite"
)

const (
	maxDownloadBytes = 10 << 20
	coverArtURL      = "https://coverartarchive.org/release/%s/front-500"
)

// Fetcher downloads artwork from the Cover Art Archive and stores embedded art.
type Fetcher struct {
	cacheDir string
	store    *sqlite.Store
	httpC    *http.Client
}

// NewFetcher creates an artwork Fetcher.
func NewFetcher(cacheDir string, store *sqlite.Store) *Fetcher {
	return &Fetcher{
		cacheDir: cacheDir,
		store:    store,
		httpC:    &http.Client{Timeout: 30 * time.Second},
	}
}

// FetchForRelease downloads the 500px front cover for a MusicBrainz release.
func (f *Fetcher) FetchForRelease(ctx context.Context, mbReleaseID string) (*models.Artwork, error) {
	u := fmt.Sprintf(coverArtURL, mbReleaseID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.httpC.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cover art archive: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover art archive %d for %s", resp.StatusCode, mbReleaseID)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadBytes))
	if err != nil {
		return nil, err
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	path := filepath.Join(f.cacheDir, hash[:2], hash+".jpg")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, err
	}

	now := time.Now()
	art := &models.Artwork{
		ID:        hash,
		Path:      path,
		Width:     500,
		Height:    500,
		Format:    "jpeg",
		CreatedAt: now,
	}
	if err := f.store.UpsertArtwork(ctx, art); err != nil {
		return nil, err
	}
	return art, nil
}

// StoreEmbedded stores an embedded picture from tags.
func (f *Fetcher) StoreEmbedded(ctx context.Context, data []byte) (*models.Artwork, error) {
	if len(data) == 0 {
		return nil, nil
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	path := filepath.Join(f.cacheDir, hash[:2], hash+".jpg")

	if _, err := os.Stat(path); err == nil {
		return &models.Artwork{ID: hash, Path: path}, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, err
	}

	now := time.Now()
	art := &models.Artwork{
		ID:        hash,
		Path:      path,
		Format:    "jpeg",
		CreatedAt: now,
	}
	if err := f.store.UpsertArtwork(ctx, art); err != nil {
		return nil, err
	}
	return art, nil
}
