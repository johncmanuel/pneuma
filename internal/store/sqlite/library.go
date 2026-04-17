// there are some queries that can't be expressed as static sqlc queries,
// so we implement them manually here via database/sql.
// these are mostly for track-derived album groups,
// which require GROUP BY on expressions.

package sqlite

import (
	"context"

	"pneuma/internal/models"
)

// UnorganizedAlbum is the special album name for unorganized tracks.
// Includes SQL single quotes for direct use in query concatenation.
const UnorganizedAlbum = "'__unorganized__'"

// These functions derive album groups directly from the tracks table using
// GROUP BY, so they work even when the albums table is empty or incomplete.

const trackAlbumGroupKey = `CASE WHEN TRIM(COALESCE(album_name,''))='' THEN ` + UnorganizedAlbum + `
	ELSE album_name || '|||' || COALESCE(album_artist,'') END`

// ListTrackAlbumGroupsPage returns paginated album groups derived from tracks.
func (s *Store) ListTrackAlbumGroupsPage(ctx context.Context, filter string, offset, limit int) ([]*models.TrackAlbumGroup, error) {
	var q string
	var args []any

	// The queries below dynamically group tracks into albums using a composite key of the
	// album name and artist. This allows the application to generate album listings directly
	// from track metadata, avoiding the need for a separate albums table.

	if filter != "" {
		q = `SELECT ` + trackAlbumGroupKey + ` AS grp_key,
			COALESCE(NULLIF(TRIM(album_name),''),'') AS album_name,
			COALESCE(album_artist,'') AS album_artist,
			COUNT(*) AS track_count,
			MIN(id) AS first_track_id
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
			MIN(id) AS first_track_id
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
		if err := rows.Scan(&g.Key, &g.Name, &g.Artist, &g.TrackCount, &g.FirstTrackID); err != nil {
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
