package library

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	"pneuma/internal/models"
	"pneuma/internal/store/sqlite"
	"pneuma/internal/store/sqlite/dbconv"
	"pneuma/internal/store/sqlite/serverdb"
)

// Service is the library domain service.
type Service struct {
	q     *serverdb.Queries
	store *sqlite.Store // retained for dynamic queries
}

// New creates a library Service.
func New(q *serverdb.Queries, store *sqlite.Store) *Service {
	return &Service{q: q, store: store}
}

// AllTracks returns every track in the database.
func (s *Service) AllTracks(ctx context.Context) ([]*models.Track, error) {
	rows, err := s.q.ListTracks(ctx)
	if err != nil {
		return nil, err
	}
	return dbconv.ListTracksToModels(rows), nil
}

// AllTracksPage returns a paginated slice of tracks.
func (s *Service) AllTracksPage(ctx context.Context, offset, limit int) ([]*models.Track, error) {
	rows, err := s.q.ListTracksPage(ctx, serverdb.ListTracksPageParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, err
	}
	return dbconv.ListTracksPageToModels(rows), nil
}

// CountTracks returns the total number of non-deleted tracks.
func (s *Service) CountTracks(ctx context.Context) (int, error) {
	n, err := s.q.CountTracks(ctx)
	return int(n), err
}

// Search performs a text search.
func (s *Service) Search(ctx context.Context, q string) ([]*models.Track, error) {
	query := normalizeSearchQuery(q)
	if query == "" {
		return []*models.Track{}, nil
	}

	ids, err := s.store.SearchTrackIDs(ctx, query, 200)
	if err != nil {
		return nil, err
	}

	rows, err := s.q.SearchTracksByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]*models.Track, len(rows))
	for _, track := range dbconv.SearchTracksByIDsToModels(rows) {
		if track == nil {
			continue
		}

		byID[track.ID] = track
	}

	ordered := make([]*models.Track, 0, len(ids))
	for _, id := range ids {
		if track, ok := byID[id]; ok {
			ordered = append(ordered, track)
		}
	}

	return ordered, nil
}

// normalizeSearchQuery transforms a raw search query
// into a normalized form suitable for FTS prefix search.
func normalizeSearchQuery(query string) string {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return ""
	}

	tokens := strings.Fields(trimmed)
	if len(tokens) == 0 {
		return ""
	}

	prefixed := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		normalizedToken := strings.ReplaceAll(token, `"`, "")
		normalizedToken = strings.ReplaceAll(normalizedToken, "'", "")
		if normalizedToken == "" {
			continue
		}

		prefixed = append(prefixed, normalizedToken+"*")
	}

	return strings.Join(prefixed, " ")
}

// TrackByID returns a single track.
func (s *Service) TrackByID(ctx context.Context, id string) (*models.Track, error) {
	row, err := s.q.TrackByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return dbconv.TrackByIDToModel(row), nil
}

// TrackByPath returns a track by filesystem path.
func (s *Service) TrackByPath(ctx context.Context, path string) (*models.Track, error) {
	row, err := s.q.TrackByPath(ctx, path)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return dbconv.TrackByPathToModel(row), nil
}

// TrackByFingerprint returns a track matching the given acoustic fingerprint.
func (s *Service) TrackByFingerprint(ctx context.Context, fp string) (*models.Track, error) {
	row, err := s.q.TrackByFingerprint(ctx, dbconv.NullStr(fp))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return dbconv.TrackByFPToModel(row), nil
}

// TracksByIDs returns tracks for the given IDs.
func (s *Service) TracksByIDs(ctx context.Context, ids []string) ([]*models.Track, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := s.q.ListTracksByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	return dbconv.ListTracksByIDsToModels(rows), nil
}

// TracksByAlbum returns all tracks for a given album_name + album_artist, ordered by disc/track.
func (s *Service) TracksByAlbum(ctx context.Context, albumName, albumArtist string) ([]*models.Track, error) {
	if albumName == "" {
		rows, err := s.q.ListTracksByAlbumUnorganized(ctx)
		if err != nil {
			return nil, err
		}
		return dbconv.ListTracksByAlbumUnorganizedToModels(rows), nil
	}

	if albumArtist != "" {
		rows, err := s.q.ListTracksByAlbumNameAndArtist(ctx, serverdb.ListTracksByAlbumNameAndArtistParams{
			AlbumName:   sql.NullString{String: albumName, Valid: true},
			AlbumArtist: sql.NullString{String: albumArtist, Valid: true},
		})
		if err != nil {
			return nil, err
		}
		return dbconv.ListTracksByAlbumNameAndArtistToModels(rows), nil
	}

	rows, err := s.q.ListTracksByAlbumName(ctx, sql.NullString{String: albumName, Valid: true})
	if err != nil {
		return nil, err
	}
	return dbconv.ListTracksByAlbumNameToModels(rows), nil
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
	return s.q.UpsertTrack(ctx, serverdb.UpsertTrackParams{
		ID:               t.ID,
		Path:             t.Path,
		Title:            t.Title,
		AlbumArtist:      dbconv.NullStr(t.AlbumArtist),
		AlbumName:        dbconv.NullStr(t.AlbumName),
		Genre:            dbconv.NullStr(t.Genre),
		Year:             dbconv.NullInt64(t.Year),
		TrackNumber:      dbconv.NullInt64(t.TrackNumber),
		DiscNumber:       dbconv.NullInt64(t.DiscNumber),
		DurationMs:       sql.NullInt64{Int64: t.DurationMS, Valid: true},
		BitrateKbps:      dbconv.NullInt64(t.BitrateKbps),
		SampleRateHz:     dbconv.NullInt64(t.SampleRateHz),
		Codec:            dbconv.NullStr(t.Codec),
		FileSizeBytes:    sql.NullInt64{Int64: t.FileSizeBytes, Valid: true},
		LastModified:     dbconv.FormatTime(t.LastModified),
		Fingerprint:      dbconv.NullStr(t.Fingerprint),
		UploadedByUserID: dbconv.NullStr(t.UploadedByUserID),
		CreatedAt:        dbconv.FormatTime(t.CreatedAt),
		UpdatedAt:        dbconv.FormatTime(t.UpdatedAt),
	})
}

// RemoveByPath deletes a track by its filesystem path.
func (s *Service) RemoveByPath(ctx context.Context, path string) error {
	return s.q.DeleteTrackByPath(ctx, path)
}

// SoftDeleteTrack marks a track as deleted without removing it from the DB.
func (s *Service) SoftDeleteTrack(ctx context.Context, trackID string) error {
	now := dbconv.FormatTime(time.Now())
	return s.q.SoftDeleteTrack(ctx, serverdb.SoftDeleteTrackParams{
		DeletedAt: sql.NullString{String: now, Valid: true},
		UpdatedAt: now,
		ID:        trackID,
	})
}

// RestoreTrack clears the soft-delete marker on a track.
func (s *Service) RestoreTrack(ctx context.Context, trackID string) error {
	return s.q.RestoreTrack(ctx, serverdb.RestoreTrackParams{
		UpdatedAt: dbconv.FormatTime(time.Now()),
		ID:        trackID,
	})
}

// DeduplicateFingerprints removes duplicate tracks that share the same
// content fingerprint, keeping only the earliest-created per fingerprint.
func (s *Service) DeduplicateFingerprints(ctx context.Context) (int, error) {
	result, err := s.q.DeleteDuplicateFingerprints(ctx)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// DuplicateByMeta returns a track that is a metadata-based duplicate of the
// given fields (same title, albumArtist, albumName, and duration ±2 s) at a
// different path, or nil if no duplicate exists.
func (s *Service) DuplicateByMeta(ctx context.Context, title, albumArtist, albumName string, durationMS int64, excludePath string) (*models.Track, error) {
	row, err := s.q.TrackDuplicateByMeta(ctx, serverdb.TrackDuplicateByMetaParams{
		Title:       title,
		AlbumArtist: albumArtist,
		AlbumName:   albumName,
		DurationMs:  sql.NullInt64{Int64: durationMS, Valid: true},
		ExcludePath: excludePath,
	})

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return dbconv.TrackDuplicateToModel(row), nil
}

// AllTrackAlbumGroupsPage returns paginated album groups derived from the tracks table.
func (s *Service) AllTrackAlbumGroupsPage(ctx context.Context, filter string, offset, limit int) ([]*models.TrackAlbumGroup, error) {
	return s.store.ListTrackAlbumGroupsPage(ctx, filter, offset, limit)
}

// CountTrackAlbumGroups returns the total number of distinct album groups in tracks.
func (s *Service) CountTrackAlbumGroups(ctx context.Context, filter string) (int, error) {
	return s.store.CountTrackAlbumGroups(ctx, filter)
}

// AddWatchFolder records a new watch folder.
func (s *Service) AddWatchFolder(ctx context.Context, path, userID string) error {
	wf := &models.WatchFolder{
		ID:        uuid.NewString(),
		Path:      path,
		UserID:    userID,
		CreatedAt: time.Now(),
	}

	return s.q.UpsertWatchFolder(ctx, serverdb.UpsertWatchFolderParams{
		ID:        wf.ID,
		Path:      wf.Path,
		UserID:    wf.UserID,
		CreatedAt: dbconv.FormatTime(wf.CreatedAt),
	})
}

// WatchFolders returns all watch folder records.
func (s *Service) WatchFolders(ctx context.Context) ([]*models.WatchFolder, error) {
	rows, err := s.q.ListWatchFolders(ctx)
	if err != nil {
		return nil, err
	}
	return dbconv.WatchFoldersToModels(rows), nil
}
