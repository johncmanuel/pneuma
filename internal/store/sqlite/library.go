// there are some queries that can't be expressed as static sqlc queries,
// so we implement them manually here via database/sql.
// these are mostly for track-derived album groups,
// which require GROUP BY on expressions.

package sqlite

import (
	"context"
	"strings"

	"pneuma/internal/models"
)

// UnorganizedAlbum is the special album name for unorganized tracks.
const UnorganizedAlbum = "'__unorganized__'"

// These functions query materialized album group aggregates from
// track_album_groups.

// ListTrackAlbumGroupsPage returns paginated album groups from materialized cache.
func (s *Store) ListTrackAlbumGroupsPage(ctx context.Context, filter string, offset, limit int) ([]*models.TrackAlbumGroup, error) {
	query := `SELECT key, name, artist, track_count, first_track_id
	FROM track_album_groups`

	args := make([]any, 0, 4)
	if strings.TrimSpace(filter) != "" {
		like := "%" + filter + "%"
		query += ` WHERE name LIKE ? OR artist LIKE ?`
		args = append(args, like, like)
	}

	query += ` ORDER BY name COLLATE NOCASE LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
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

// CountTrackAlbumGroups returns the total number of album groups in the materialized cache.
func (s *Store) CountTrackAlbumGroups(ctx context.Context, filter string) (int, error) {
	var n int
	var err error

	if strings.TrimSpace(filter) != "" {
		like := "%" + filter + "%"
		err = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM track_album_groups WHERE name LIKE ? OR artist LIKE ?`,
			like, like).Scan(&n)
	} else {
		err = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM track_album_groups`).Scan(&n)
	}
	return n, err
}

// SearchTrackIDs performs token/prefix search against the FTS index and
// returns matching track IDs in relevance order.
func (s *Store) SearchTrackIDs(ctx context.Context, query string, limit int) ([]string, error) {
	if strings.TrimSpace(query) == "" {
		return []string{}, nil
	}

	if limit <= 0 {
		limit = 200
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id
		FROM tracks_fts
		JOIN tracks t ON t.rowid = tracks_fts.rowid
		WHERE t.deleted_at IS NULL
		  AND tracks_fts MATCH ?
		ORDER BY bm25(tracks_fts), t.title COLLATE NOCASE
		LIMIT ?`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}
