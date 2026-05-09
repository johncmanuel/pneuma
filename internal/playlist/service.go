package playlist

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"pneuma/internal/library"
	"pneuma/internal/models"
	"pneuma/internal/store/sqlite/dbconv"
	"pneuma/internal/store/sqlite/serverdb"
)

// Service is the server-side playlist domain service.
type Service struct {
	q   *serverdb.Queries
	lib *library.Service
}

// New creates a new playlist service.
func New(q *serverdb.Queries, lib *library.Service) *Service {
	return &Service{q: q, lib: lib}
}

// Create makes a new playlist owned by the given user.
func (s *Service) Create(ctx context.Context, userID, name, description string) (*models.Playlist, error) {
	now := dbconv.FormatTime(time.Now())
	id := uuid.NewString()
	if err := s.q.CreatePlaylist(ctx, serverdb.CreatePlaylistParams{
		ID:          id,
		UserID:      userID,
		Name:        name,
		Description: description,
		ArtworkPath: "",
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		return nil, fmt.Errorf("create playlist: %w", err)
	}

	return &models.Playlist{
		ID:          id,
		UserID:      userID,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}, nil
}

// ListByUser returns all playlists owned by a user with aggregate counts.
func (s *Service) ListByUser(ctx context.Context, userID string) ([]*models.Playlist, error) {
	rows, err := s.q.ListPlaylistsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list playlists: %w", err)
	}
	return dbconv.PlaylistRowsToModels(rows), nil
}

// GetByID returns a single playlist by ID.
func (s *Service) GetByID(ctx context.Context, id string) (*models.Playlist, error) {
	p, err := s.q.GetPlaylistByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("playlist not found")
		}
		return nil, fmt.Errorf("get playlist: %w", err)
	}
	pl := dbconv.PlaylistToModel(p)

	// Populate aggregate fields.
	count, err := s.q.CountPlaylistItems(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("count items: %w", err)
	}
	pl.ItemCount = int(count)

	return pl, nil
}

// Update modifies a playlist's metadata (name, description, artwork).
func (s *Service) Update(ctx context.Context, id, name, description, artworkPath string) error {
	now := dbconv.FormatTime(time.Now())
	return s.q.UpdatePlaylist(ctx, serverdb.UpdatePlaylistParams{
		Name:        name,
		Description: description,
		ArtworkPath: artworkPath,
		UpdatedAt:   now,
		ID:          id,
	})
}

// Delete removes a playlist and all its items.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.q.DeletePlaylist(ctx, id)
}

// GetItems returns all items in a playlist, ordered by position.
func (s *Service) GetItems(ctx context.Context, playlistID string) ([]models.PlaylistItem, error) {
	rows, err := s.q.ListPlaylistItems(ctx, playlistID)
	if err != nil {
		return nil, fmt.Errorf("list playlist items: %w", err)
	}
	return dbconv.PlaylistItemsToModels(rows), nil
}

// SetItems replaces all items in a playlist (delete + re-insert in a logical batch).
func (s *Service) SetItems(ctx context.Context, playlistID string, items []models.PlaylistItem) error {
	if err := s.q.DeletePlaylistItems(ctx, playlistID); err != nil {
		return fmt.Errorf("delete old items: %w", err)
	}

	for i, item := range items {
		addedAt := dbconv.FormatTime(item.AddedAt)
		if item.AddedAt.IsZero() {
			addedAt = dbconv.FormatTime(time.Now())
		}
		trackID := sql.NullString{String: item.TrackID, Valid: item.TrackID != ""}
		if err := s.q.InsertPlaylistItem(ctx, serverdb.InsertPlaylistItemParams{
			PlaylistID:     playlistID,
			Position:       int64(i),
			Source:         string(item.Source),
			TrackID:        trackID,
			RefTitle:       item.RefTitle,
			RefAlbum:       item.RefAlbum,
			RefAlbumArtist: item.RefAlbumArtist,
			RefDurationMs:  item.RefDurationMS,
			AddedAt:        addedAt,
		}); err != nil {
			return fmt.Errorf("insert item %d: %w", i, err)
		}
	}

	now := dbconv.FormatTime(time.Now())
	return s.q.TouchPlaylist(ctx, serverdb.TouchPlaylistParams{
		UpdatedAt: now,
		ID:        playlistID,
	})
}

// AddItem appends a single item to the end of a playlist.
func (s *Service) AddItem(ctx context.Context, playlistID string, item models.PlaylistItem) error {
	count, err := s.q.CountPlaylistItems(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("count items: %w", err)
	}

	addedAt := dbconv.FormatTime(time.Now())
	trackID := sql.NullString{String: item.TrackID, Valid: item.TrackID != ""}
	if err := s.q.InsertPlaylistItem(ctx, serverdb.InsertPlaylistItemParams{
		PlaylistID:     playlistID,
		Position:       count,
		Source:         string(item.Source),
		TrackID:        trackID,
		RefTitle:       item.RefTitle,
		RefAlbum:       item.RefAlbum,
		RefAlbumArtist: item.RefAlbumArtist,
		RefDurationMs:  item.RefDurationMS,
		AddedAt:        addedAt,
	}); err != nil {
		return fmt.Errorf("insert item: %w", err)
	}

	now := dbconv.FormatTime(time.Now())
	return s.q.TouchPlaylist(ctx, serverdb.TouchPlaylistParams{
		UpdatedAt: now,
		ID:        playlistID,
	})
}

// PlaylistStats returns item count and total duration for a playlist.
func (s *Service) PlaylistStats(ctx context.Context, playlistID string) (int, int64, error) {
	count, err := s.q.CountPlaylistItems(ctx, playlistID)
	if err != nil {
		return 0, 0, fmt.Errorf("count playlist items: %w", err)
	}

	durationMS, err := s.q.SumPlaylistDuration(ctx, playlistID)
	if err != nil {
		return 0, 0, fmt.Errorf("sum playlist duration: %w", err)
	}

	return int(count), durationMS, nil
}

// GenerateRandom creates a new playlist filled with randomly selected tracks
// targeting the given duration in minutes.
// Track selection is delegated to the library via SQL ORDER BY RANDOM(),
// avoiding loading the full tracks table into memory.
func (s *Service) GenerateRandom(ctx context.Context, userID, name, description string, durationMinutes int) (*models.Playlist, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("playlist name is required")
	}
	if durationMinutes <= 0 {
		return nil, fmt.Errorf("duration must be at least 1 minute")
	}

	// Fetch a large random batch from the library. 200 tracks covers ~12 hours
	// of typical listening, which is more than enough for any target duration.
	candidates, err := s.lib.GetRandomTracks(ctx, 200)
	if err != nil {
		return nil, fmt.Errorf("get random tracks: %w", err)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no tracks available")
	}

	pl, err := s.Create(ctx, userID, name, description)
	if err != nil {
		return nil, fmt.Errorf("create playlist: %w", err)
	}

	targetMS := int64(durationMinutes) * 60 * 1000
	var cumulative int64
	for i, t := range candidates {
		if cumulative >= targetMS {
			break
		}
		if err := s.q.InsertPlaylistItem(ctx, serverdb.InsertPlaylistItemParams{
			PlaylistID:     pl.ID,
			TrackID:        sql.NullString{String: t.ID, Valid: true},
			Position:       int64(i),
			AddedAt:        dbconv.FormatTime(time.Now()),
			Source:         "remote",
			RefTitle:       t.Title,
			RefAlbum:       t.AlbumName,
			RefAlbumArtist: t.AlbumArtist,
			RefDurationMs:  t.DurationMS,
		}); err != nil {
			return nil, fmt.Errorf("add playlist item: %w", err)
		}
		cumulative += t.DurationMS
	}

	count, err := s.q.CountPlaylistItems(ctx, pl.ID)
	if err == nil {
		pl.ItemCount = int(count)
	}

	return pl, nil
}
