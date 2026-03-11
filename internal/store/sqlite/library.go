// there are some queries that can't be expressed as static sqlc queries,
// so we implement them manually here via database/sql.
// these are mostly for track-derived album groups,
// which require dynamic IN-lists or GROUP BY on expressions.

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"pneuma/internal/models"
)

// TracksByIDs returns tracks for the given IDs (dynamic IN-list, cannot be expressed as a static sqlc query).
func (s *Store) TracksByIDs(ctx context.Context, ids []string) ([]*models.Track, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	q := `SELECT ` + trackColumns + ` FROM tracks WHERE id IN (` + placeholders + `)`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTracks(rows)
}

// ─── Track-derived album groups ───────────────────────────────────────────────
// These functions derive album groups directly from the tracks table using
// GROUP BY, so they work even when the albums table is empty or incomplete.

const trackAlbumGroupKey = `CASE WHEN TRIM(COALESCE(album_name,''))='' THEN '__unorganized__'
	ELSE album_name || '|||' || COALESCE(album_artist,'') END`

// ListTrackAlbumGroupsPage returns paginated album groups derived from tracks.
func (s *Store) ListTrackAlbumGroupsPage(ctx context.Context, filter string, offset, limit int) ([]*models.TrackAlbumGroup, error) {
	var q string
	var args []any
	if filter != "" {
		q = `SELECT ` + trackAlbumGroupKey + ` AS grp_key,
			COALESCE(NULLIF(TRIM(album_name),''),'') AS album_name,
			COALESCE(album_artist,'') AS album_artist,
			COUNT(*) AS track_count,
			MIN(id) AS first_track_id,
			'' AS artwork_id
		FROM tracks
		WHERE deleted_at IS NULL AND (album_name LIKE ? OR album_artist LIKE ?)
		GROUP BY grp_key
		ORDER BY album_name COLLATE NOCASE
		LIMIT ? OFFSET ?`
		like := "%" + filter + "%"
		args = []any{like, like, limit, offset}
	} else {
		q = `SELECT ` + trackAlbumGroupKey + ` AS grp_key,
			COALESCE(NULLIF(TRIM(album_name),''),'') AS album_name,
			COALESCE(album_artist,'') AS album_artist,
			COUNT(*) AS track_count,
			MIN(id) AS first_track_id,
			'' AS artwork_id
		FROM tracks
		WHERE deleted_at IS NULL
		GROUP BY grp_key
		ORDER BY album_name COLLATE NOCASE
		LIMIT ? OFFSET ?`
		args = []any{limit, offset}
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.TrackAlbumGroup
	for rows.Next() {
		var g models.TrackAlbumGroup
		if err := rows.Scan(&g.Key, &g.Name, &g.Artist, &g.TrackCount, &g.FirstTrackID, &g.ArtworkID); err != nil {
			return nil, err
		}
		out = append(out, &g)
	}
	return out, rows.Err()
}

// CountTrackAlbumGroups returns the total number of distinct album groups in tracks.
func (s *Store) CountTrackAlbumGroups(ctx context.Context, filter string) (int, error) {
	var n int
	var err error
	if filter != "" {
		like := "%" + filter + "%"
		err = s.db.QueryRowContext(ctx,
			`SELECT COUNT(DISTINCT `+trackAlbumGroupKey+`) FROM tracks WHERE deleted_at IS NULL AND (album_name LIKE ? OR album_artist LIKE ?)`,
			like, like).Scan(&n)
	} else {
		err = s.db.QueryRowContext(ctx,
			`SELECT COUNT(DISTINCT `+trackAlbumGroupKey+`) FROM tracks WHERE deleted_at IS NULL`).Scan(&n)
	}
	return n, err
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

const trackColumns = `id,path,title,
	COALESCE(album_artist,''),COALESCE(album_name,''),COALESCE(genre,''),COALESCE(year,0),
	COALESCE(track_number,0),COALESCE(disc_number,0),COALESCE(duration_ms,0),COALESCE(bitrate_kbps,0),COALESCE(sample_rate_hz,0),
	COALESCE(codec,''),COALESCE(file_size_bytes,0),last_modified,COALESCE(fingerprint,''),
	COALESCE(replay_gain_track,0),COALESCE(replay_gain_album,0),
	COALESCE(uploaded_by_user_id,''),deleted_at,created_at,updated_at`

type scanner interface {
	Scan(dest ...any) error
}

func scanTrack(row scanner) (*models.Track, error) {
	var t models.Track
	var deletedAt sql.NullString
	err := row.Scan(
		&t.ID, &t.Path, &t.Title,
		&t.AlbumArtist, &t.AlbumName, &t.Genre, &t.Year,
		&t.TrackNumber, &t.DiscNumber, &t.DurationMS, &t.BitrateKbps,
		&t.SampleRateHz, &t.Codec, &t.FileSizeBytes,
		(*timeStr)(&t.LastModified), &t.Fingerprint,
		&t.ReplayGainTrack, &t.ReplayGainAlbum,
		&t.UploadedByUserID, &deletedAt,
		(*timeStr)(&t.CreatedAt), (*timeStr)(&t.UpdatedAt),
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanTrack: %w", err)
	}
	if deletedAt.Valid {
		ts, _ := time.Parse(time.RFC3339, deletedAt.String)
		t.DeletedAt = &ts
	}
	return &t, nil
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
