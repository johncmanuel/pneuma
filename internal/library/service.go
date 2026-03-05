package library

import (
	"context"
	"time"

	"github.com/google/uuid"

	"pneuma/internal/models"
	"pneuma/internal/store/sqlite"
)

// Service is the library domain service.
type Service struct {
	store *sqlite.Store
}

// New creates a library Service.
func New(store *sqlite.Store) *Service {
	return &Service{store: store}
}

// AllTracks returns every track in the database.
func (s *Service) AllTracks(ctx context.Context) ([]*models.Track, error) {
	return s.store.ListTracks(ctx)
}

// Search performs a text search.
func (s *Service) Search(ctx context.Context, q string) ([]*models.Track, error) {
	return s.store.SearchTracks(ctx, q)
}

// TrackByID returns a single track.
func (s *Service) TrackByID(ctx context.Context, id string) (*models.Track, error) {
	return s.store.TrackByID(ctx, id)
}

// TrackByPath returns a track by filesystem path.
func (s *Service) TrackByPath(ctx context.Context, path string) (*models.Track, error) {
	return s.store.TrackByPath(ctx, path)
}

// TrackByFingerprint returns a track matching the given acoustic fingerprint.
func (s *Service) TrackByFingerprint(ctx context.Context, fp string) (*models.Track, error) {
	return s.store.TrackByFingerprint(ctx, fp)
}

// TracksByIDs returns tracks for the given IDs.
func (s *Service) TracksByIDs(ctx context.Context, ids []string) ([]*models.Track, error) {
	return s.store.TracksByIDs(ctx, ids)
}

// UpsertTrack inserts or updates a track, auto-generating an ID if empty.
func (s *Service) UpsertTrack(ctx context.Context, t *models.Track) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	return s.store.UpsertTrack(ctx, t)
}

// RemoveByPath deletes a track by its filesystem path.
func (s *Service) RemoveByPath(ctx context.Context, path string) error {
	return s.store.DeleteTrackByPath(ctx, path)
}

// SoftDeleteTrack marks a track as deleted without removing it from the DB.
func (s *Service) SoftDeleteTrack(ctx context.Context, trackID string) error {
	return s.store.SoftDeleteTrack(ctx, trackID)
}

// RestoreTrack clears the soft-delete marker on a track.
func (s *Service) RestoreTrack(ctx context.Context, trackID string) error {
	return s.store.RestoreTrack(ctx, trackID)
}

// DeduplicateFingerprints removes duplicate tracks that share the same
// acoustic fingerprint, keeping only the earliest-created per fingerprint.
func (s *Service) DeduplicateFingerprints(ctx context.Context) (int, error) {
	return s.store.DeleteDuplicateFingerprints(ctx)
}

// EnsureArtist looks up an artist by name, creating if necessary.
func (s *Service) EnsureArtist(ctx context.Context, name string) (*models.Artist, error) {
	a, err := s.store.ArtistByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if a != nil {
		return a, nil
	}
	a = &models.Artist{
		ID:        uuid.NewString(),
		Name:      name,
		CreatedAt: time.Now(),
	}
	if err := s.store.UpsertArtist(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// EnsureAlbum looks up an album by title+artist, creating if necessary.
func (s *Service) EnsureAlbum(ctx context.Context, title, artistID string) (*models.Album, error) {
	a, err := s.store.AlbumByTitleArtist(ctx, title, artistID)
	if err != nil {
		return nil, err
	}
	if a != nil {
		return a, nil
	}
	a = &models.Album{
		ID:        uuid.NewString(),
		Title:     title,
		ArtistID:  artistID,
		CreatedAt: time.Now(),
	}
	if err := s.store.UpsertAlbum(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// AllAlbums returns every album in the database.
func (s *Service) AllAlbums(ctx context.Context) ([]*models.Album, error) {
	return s.store.ListAlbums(ctx)
}

// SetAlbumArtwork updates the artwork ID on an album.
func (s *Service) SetAlbumArtwork(ctx context.Context, albumID, artworkID string) error {
	al, err := s.store.AlbumByID(ctx, albumID)
	if err != nil || al == nil {
		return err
	}
	al.ArtworkID = artworkID
	return s.store.UpsertAlbum(ctx, al)
}

// AddWatchFolder records a new watch folder.
func (s *Service) AddWatchFolder(ctx context.Context, path, userID string) error {
	wf := &models.WatchFolder{
		ID:        uuid.NewString(),
		Path:      path,
		UserID:    userID,
		CreatedAt: time.Now(),
	}
	return s.store.UpsertWatchFolder(ctx, wf)
}

// WatchFolders returns all watch folder records.
func (s *Service) WatchFolders(ctx context.Context) ([]*models.WatchFolder, error) {
	return s.store.ListWatchFolders(ctx)
}
