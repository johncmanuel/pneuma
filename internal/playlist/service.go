package playlist

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"pneuma/internal/models"
	"pneuma/internal/store/sqlite/dbconv"
	"pneuma/internal/store/sqlite/serverdb"
)

// Service is the server-side playlist domain service.
type Service struct {
	q *serverdb.Queries
}

// New creates a playlist Service.
func New(q *serverdb.Queries) *Service {
	return &Service{q: q}
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

// Delete removes a playlist and all its items (CASCADE).
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
