package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"pneuma/internal/models"
)

// ─── Track ───────────────────────────────────────────────────────────────────

// UpsertTrack inserts or replaces a track record.
func (s *Store) UpsertTrack(ctx context.Context, t *models.Track) error {
	const q = `
	INSERT INTO tracks (
		id, path, title, artist_id, album_id, album_artist, album_name, genre, year,
		track_number, disc_number, duration_ms, bitrate_kbps, sample_rate_hz,
		codec, file_size_bytes, last_modified, fingerprint, mb_recording_id,
		replay_gain_track, replay_gain_album, artwork_id, uploaded_by_user_id,
		enriched_at, created_at, updated_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	ON CONFLICT(path) DO UPDATE SET
		title=excluded.title,
		artist_id=excluded.artist_id, album_id=excluded.album_id,
		album_artist=excluded.album_artist, album_name=excluded.album_name,
		genre=excluded.genre,
		year=excluded.year, track_number=excluded.track_number,
		disc_number=excluded.disc_number, duration_ms=excluded.duration_ms,
		bitrate_kbps=excluded.bitrate_kbps, sample_rate_hz=excluded.sample_rate_hz,
		codec=excluded.codec, file_size_bytes=excluded.file_size_bytes,
		last_modified=excluded.last_modified, fingerprint=excluded.fingerprint,
		mb_recording_id=excluded.mb_recording_id,
		replay_gain_track=excluded.replay_gain_track,
		replay_gain_album=excluded.replay_gain_album,
		artwork_id=excluded.artwork_id,
		uploaded_by_user_id=excluded.uploaded_by_user_id,
		enriched_at=excluded.enriched_at,
		updated_at=excluded.updated_at`
	// Note: id, created_at, and deleted_at are intentionally NOT updated —
	// they are stable identity fields that must not change once a track is first inserted.

	var enrichedAt *string
	if t.EnrichedAt != nil {
		s := t.EnrichedAt.UTC().Format(time.RFC3339)
		enrichedAt = &s
	}
	_, err := s.db.ExecContext(ctx, q,
		t.ID, t.Path, t.Title, nullStr(t.ArtistID), nullStr(t.AlbumID),
		t.AlbumArtist, t.AlbumName, t.Genre, t.Year, t.TrackNumber, t.DiscNumber,
		t.DurationMS, t.BitrateKbps, t.SampleRateHz, t.Codec,
		t.FileSizeBytes, t.LastModified.UTC().Format(time.RFC3339),
		t.Fingerprint, t.MBRecordingID,
		t.ReplayGainTrack, t.ReplayGainAlbum,
		nullStr(t.ArtworkID), nullStr(t.UploadedByUserID),
		enrichedAt,
		t.CreatedAt.UTC().Format(time.RFC3339),
		t.UpdatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// TrackByPath returns a Track given its filesystem path, or nil if not found.
func (s *Store) TrackByPath(ctx context.Context, path string) (*models.Track, error) {
	const q = `SELECT ` + trackColumns + ` FROM tracks WHERE path=? LIMIT 1`
	row := s.db.QueryRowContext(ctx, q, path)
	return scanTrack(row)
}

// TrackByID returns a Track by its ID.
func (s *Store) TrackByID(ctx context.Context, id string) (*models.Track, error) {
	const q = `SELECT ` + trackColumns + ` FROM tracks WHERE id=? LIMIT 1`
	row := s.db.QueryRowContext(ctx, q, id)
	return scanTrack(row)
}

// ListTracks returns all non-deleted tracks ordered by title.
func (s *Store) ListTracks(ctx context.Context) ([]*models.Track, error) {
	const q = `SELECT ` + trackColumns + ` FROM tracks WHERE deleted_at IS NULL ORDER BY title COLLATE NOCASE`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTracks(rows)
}

// SearchTracks performs a case-insensitive substring search on title, genre, artist name, album title.
func (s *Store) SearchTracks(ctx context.Context, query string) ([]*models.Track, error) {
	q := `SELECT ` + trackColumns + ` FROM tracks
		LEFT JOIN artists ON artists.id = tracks.artist_id
		LEFT JOIN albums ON albums.id = tracks.album_id
		WHERE tracks.deleted_at IS NULL
		  AND (tracks.title LIKE ? OR tracks.genre LIKE ? OR artists.name LIKE ? OR albums.title LIKE ?)
		ORDER BY tracks.title COLLATE NOCASE LIMIT 200`
	like := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx, q, like, like, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTracks(rows)
}

// TrackByFingerprint returns the first track matching the given acoustic
// fingerprint, or nil if none exists. Empty fingerprints are never matched.
func (s *Store) TrackByFingerprint(ctx context.Context, fp string) (*models.Track, error) {
	if fp == "" {
		return nil, nil
	}
	const q = `SELECT ` + trackColumns + ` FROM tracks WHERE fingerprint=? AND fingerprint!='' LIMIT 1`
	row := s.db.QueryRowContext(ctx, q, fp)
	return scanTrack(row)
}

// TracksByIDs returns tracks for the given IDs. The order is not guaranteed.
func (s *Store) TracksByIDs(ctx context.Context, ids []string) ([]*models.Track, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	q := `SELECT ` + trackColumns + ` FROM tracks WHERE id IN (` + joinStrings(placeholders, ",") + `)`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTracks(rows)
}

// DeleteTrackByPath removes a track record by path.
func (s *Store) DeleteTrackByPath(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM tracks WHERE path=?`, path)
	return err
}

// SoftDeleteTrack sets the deleted_at timestamp on a track (soft-delete).
func (s *Store) SoftDeleteTrack(ctx context.Context, trackID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE tracks SET deleted_at=?,updated_at=? WHERE id=?`,
		time.Now().UTC().Format(time.RFC3339),
		time.Now().UTC().Format(time.RFC3339),
		trackID,
	)
	return err
}

// RestoreTrack clears the deleted_at timestamp on a track.
func (s *Store) RestoreTrack(ctx context.Context, trackID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE tracks SET deleted_at=NULL,updated_at=? WHERE id=?`,
		time.Now().UTC().Format(time.RFC3339),
		trackID,
	)
	return err
}

// DeleteDuplicateFingerprints removes duplicate tracks that share the same
// fingerprint, keeping only the first-inserted track per fingerprint.
// Uses rowid ordering rather than created_at to avoid timestamp collisions.
func (s *Store) DeleteDuplicateFingerprints(ctx context.Context) (int, error) {
	const q = `DELETE FROM tracks
		WHERE fingerprint != '' AND fingerprint IS NOT NULL
		AND rowid NOT IN (
			SELECT MIN(rowid) FROM tracks
			WHERE fingerprint != '' AND fingerprint IS NOT NULL
			GROUP BY fingerprint
		)`
	result, err := s.db.ExecContext(ctx, q)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// ─── Album ───────────────────────────────────────────────────────────────────

// UpsertAlbum inserts or updates an album.
func (s *Store) UpsertAlbum(ctx context.Context, a *models.Album) error {
	const q = `
	INSERT INTO albums (id, title, artist_id, year, mb_release_id, artwork_id, created_at)
	VALUES (?,?,?,?,?,?,?)
	ON CONFLICT(id) DO UPDATE SET
		title=excluded.title, artist_id=excluded.artist_id, year=excluded.year,
		mb_release_id=excluded.mb_release_id, artwork_id=excluded.artwork_id`
	_, err := s.db.ExecContext(ctx, q,
		a.ID, a.Title, nullStr(a.ArtistID), a.Year,
		a.MBReleaseID, nullStr(a.ArtworkID),
		a.CreatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// AlbumByID returns an album by ID.
func (s *Store) AlbumByID(ctx context.Context, id string) (*models.Album, error) {
	const q = `SELECT id,title,COALESCE(artist_id,''),year,COALESCE(mb_release_id,''),COALESCE(artwork_id,''),created_at FROM albums WHERE id=?`
	row := s.db.QueryRowContext(ctx, q, id)
	return scanAlbum(row)
}

// AlbumByTitleArtist finds an album by title+artist (used during ingest dedup).
func (s *Store) AlbumByTitleArtist(ctx context.Context, title, artistID string) (*models.Album, error) {
	const q = `SELECT id,title,COALESCE(artist_id,''),year,COALESCE(mb_release_id,''),COALESCE(artwork_id,''),created_at
			   FROM albums WHERE title=? AND COALESCE(artist_id,'')=? LIMIT 1`
	row := s.db.QueryRowContext(ctx, q, title, artistID)
	return scanAlbum(row)
}

// ListAlbums returns all albums.
func (s *Store) ListAlbums(ctx context.Context) ([]*models.Album, error) {
	const q = `SELECT id,title,COALESCE(artist_id,''),year,COALESCE(mb_release_id,''),COALESCE(artwork_id,''),created_at FROM albums ORDER BY title COLLATE NOCASE`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Album
	for rows.Next() {
		a, err := scanAlbum(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ─── Artist ──────────────────────────────────────────────────────────────────

// UpsertArtist inserts or updates an artist.
func (s *Store) UpsertArtist(ctx context.Context, a *models.Artist) error {
	const q = `
	INSERT INTO artists (id, name, mb_artist_id, created_at)
	VALUES (?,?,?,?)
	ON CONFLICT(id) DO UPDATE SET name=excluded.name, mb_artist_id=excluded.mb_artist_id`
	_, err := s.db.ExecContext(ctx, q,
		a.ID, a.Name, a.MBArtistID,
		a.CreatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// ArtistByName looks up an artist by exact name (case-sensitive).
func (s *Store) ArtistByName(ctx context.Context, name string) (*models.Artist, error) {
	const q = `SELECT id,name,COALESCE(mb_artist_id,''),created_at FROM artists WHERE name=? LIMIT 1`
	row := s.db.QueryRowContext(ctx, q, name)
	var a models.Artist
	if err := row.Scan(&a.ID, &a.Name, &a.MBArtistID, (*timeStr)(&a.CreatedAt)); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// ─── Watch Folders ────────────────────────────────────────────────────────────

// UpsertWatchFolder adds a watch folder record.
func (s *Store) UpsertWatchFolder(ctx context.Context, wf *models.WatchFolder) error {
	const q = `INSERT OR IGNORE INTO watch_folders (id,path,user_id,created_at) VALUES (?,?,?,?)`
	_, err := s.db.ExecContext(ctx, q, wf.ID, wf.Path, wf.UserID, wf.CreatedAt.UTC().Format(time.RFC3339))
	return err
}

// ListWatchFolders returns all watch folder records.
func (s *Store) ListWatchFolders(ctx context.Context) ([]*models.WatchFolder, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,path,user_id,created_at FROM watch_folders`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.WatchFolder
	for rows.Next() {
		var wf models.WatchFolder
		if err := rows.Scan(&wf.ID, &wf.Path, &wf.UserID, (*timeStr)(&wf.CreatedAt)); err != nil {
			return nil, err
		}
		out = append(out, &wf)
	}
	return out, rows.Err()
}

// ─── Artwork ──────────────────────────────────────────────────────────────────

// UpsertArtwork inserts or updates an artwork record.
func (s *Store) UpsertArtwork(ctx context.Context, a *models.Artwork) error {
	const q = `INSERT INTO artworks (id,path,width,height,format,created_at) VALUES (?,?,?,?,?,?)
		ON CONFLICT(path) DO UPDATE SET width=excluded.width,height=excluded.height,format=excluded.format`
	_, err := s.db.ExecContext(ctx, q, a.ID, a.Path, a.Width, a.Height, a.Format, a.CreatedAt.UTC().Format(time.RFC3339))
	return err
}

// ArtworkByPath returns an artwork given its cache path.
func (s *Store) ArtworkByPath(ctx context.Context, path string) (*models.Artwork, error) {
	const q = `SELECT id,path,COALESCE(width,0),COALESCE(height,0),COALESCE(format,''),created_at FROM artworks WHERE path=? LIMIT 1`
	row := s.db.QueryRowContext(ctx, q, path)
	var a models.Artwork
	if err := row.Scan(&a.ID, &a.Path, &a.Width, &a.Height, &a.Format, (*timeStr)(&a.CreatedAt)); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

const trackColumns = `id,path,title,
	COALESCE(artist_id,''),COALESCE(album_id,''),
	COALESCE((SELECT name FROM artists WHERE id=tracks.artist_id),'') AS artist_name,
	album_artist,album_name,genre,year,
	track_number,disc_number,duration_ms,bitrate_kbps,sample_rate_hz,
	codec,file_size_bytes,last_modified,fingerprint,mb_recording_id,
	replay_gain_track,replay_gain_album,COALESCE(artwork_id,''),
	COALESCE(uploaded_by_user_id,''),deleted_at,
	enriched_at,created_at,updated_at`

type scanner interface {
	Scan(dest ...any) error
}

func scanTrack(row scanner) (*models.Track, error) {
	var t models.Track
	var enrichedAt, deletedAt sql.NullString
	err := row.Scan(
		&t.ID, &t.Path, &t.Title,
		&t.ArtistID, &t.AlbumID, &t.ArtistName, &t.AlbumArtist, &t.AlbumName, &t.Genre, &t.Year,
		&t.TrackNumber, &t.DiscNumber, &t.DurationMS, &t.BitrateKbps,
		&t.SampleRateHz, &t.Codec, &t.FileSizeBytes,
		(*timeStr)(&t.LastModified), &t.Fingerprint, &t.MBRecordingID,
		&t.ReplayGainTrack, &t.ReplayGainAlbum, &t.ArtworkID,
		&t.UploadedByUserID, &deletedAt,
		&enrichedAt,
		(*timeStr)(&t.CreatedAt), (*timeStr)(&t.UpdatedAt),
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanTrack: %w", err)
	}
	if enrichedAt.Valid {
		ts, _ := time.Parse(time.RFC3339, enrichedAt.String)
		t.EnrichedAt = &ts
	}
	if deletedAt.Valid {
		ts, _ := time.Parse(time.RFC3339, deletedAt.String)
		t.DeletedAt = &ts
	}
	return &t, nil
}

func scanAlbum(row scanner) (*models.Album, error) {
	var a models.Album
	err := row.Scan(&a.ID, &a.Title, &a.ArtistID, &a.Year, &a.MBReleaseID, &a.ArtworkID, (*timeStr)(&a.CreatedAt))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &a, err
}

func collectTracks(rows *sql.Rows) ([]*models.Track, error) {
	var out []*models.Track
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func joinStrings(elems []string, sep string) string {
	if len(elems) == 0 {
		return ""
	}
	out := elems[0]
	for _, e := range elems[1:] {
		out += sep + e
	}
	return out
}

// timeStr implements sql.Scanner for RFC3339 strings → time.Time.
type timeStr time.Time

func (t *timeStr) Scan(src any) error {
	switch v := src.(type) {
	case string:
		ts, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return err
		}
		*t = timeStr(ts)
	case nil:
		*t = timeStr(time.Time{})
	default:
		return fmt.Errorf("timeStr: unexpected type %T", src)
	}
	return nil
}
