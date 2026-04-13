package desktop

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"pneuma/internal/store/sqlite"
	"pneuma/internal/store/sqlite/desktopdb"
)

// sqlPlaceholders returns n "?" strings for use in SQL IN clauses.
func sqlPlaceholders(n int) []string {
	ph := make([]string, n)
	for i := range ph {
		ph[i] = "?"
	}
	return ph
}

// buildFolderIN appends a folder IN clause to conditions/args if folders is non-empty.
func buildFolderIN(conditions []string, args []any, folders []string) ([]string, []any) {
	if len(folders) == 0 {
		return conditions, args
	}

	ph := sqlPlaceholders(len(folders))
	conditions = append(conditions, "folder IN ("+strings.Join(ph, ",")+")")

	for _, f := range folders {
		args = append(args, f)
	}

	return conditions, args
}

const (
	minPaginationLimit = 50
	maxPaginationLimit = 200

	// localTrackCols is the shared column list for raw SQL queries against local_tracks.
	localTrackCols = `path, folder, title, artist, album, album_artist, genre,
						year, track_number, disc_number, duration_ms, has_artwork`

	// unknownArtist is the fallback artist name for tracks without an album artist.
	// Includes SQL single quotes for direct use in query concatenation.
	unknownArtist = "'Unknown Artist'"
)

// clampPagination constrains offset and limit to fixed ranges.
func clampPagination(offset, limit int) (int, int) {
	if limit <= 0 {
		limit = minPaginationLimit
	}
	if limit > maxPaginationLimit {
		limit = maxPaginationLimit
	}
	if offset < 0 {
		offset = 0
	}

	return offset, limit
}

// localTrackFromDB converts a desktopdb.LocalTrack to a desktop.LocalTrack.
func localTrackFromDB(row desktopdb.LocalTrack) LocalTrack {
	return LocalTrack{
		Path:        row.Path,
		Title:       row.Title,
		Artist:      row.Artist,
		Album:       row.Album,
		AlbumArtist: row.AlbumArtist,
		Genre:       row.Genre,
		Year:        int(row.Year),
		TrackNumber: int(row.TrackNumber),
		DiscNumber:  int(row.DiscNumber),
		DurationMs:  row.DurationMs,
		HasArtwork:  row.HasArtwork,
	}
}

// localTracksFromDB converts a slice of desktopdb.LocalTrack to []LocalTrack.
func localTracksFromDB(rows []desktopdb.LocalTrack) []LocalTrack {
	out := make([]LocalTrack, len(rows))
	for i, r := range rows {
		out[i] = localTrackFromDB(r)
	}
	return out
}

// upsertLocalTrack inserts or replaces a single track row.
func (a *App) upsertLocalTrack(lt LocalTrack, folder string) error {
	if a.dq == nil {
		return fmt.Errorf("appDB not initialised")
	}
	return a.dq.UpsertLocalTrack(context.Background(), desktopdb.UpsertLocalTrackParams{
		Path:        lt.Path,
		Folder:      folder,
		Title:       lt.Title,
		Artist:      lt.Artist,
		Album:       lt.Album,
		AlbumArtist: lt.AlbumArtist,
		Genre:       lt.Genre,
		Year:        int64(lt.Year),
		TrackNumber: int64(lt.TrackNumber),
		DiscNumber:  int64(lt.DiscNumber),
		DurationMs:  lt.DurationMs,
		HasArtwork:  lt.HasArtwork,
	})
}

// deleteLocalTracksByFolder removes every track whose folder column matches.
func (a *App) deleteLocalTracksByFolder(folder string) error {
	if a.dq == nil {
		return nil
	}
	return a.dq.DeleteLocalTracksByFolder(context.Background(), folder)
}

// deleteLocalTrackByPath removes a single track by its absolute file path.
func (a *App) deleteLocalTrackByPath(path string) error {
	if a.dq == nil {
		return nil
	}
	return a.dq.DeleteLocalTrackByPath(context.Background(), path)
}

// pruneStaleLocalTracks removes rows for the given folder whose paths aren't in
// livePaths. Returns the slice of deleted paths.
//
// This method combines sqlc ListPathsByFolder with DeleteLocalTracksByPaths
// to remove stale rows in one pass.
func (a *App) pruneStaleLocalTracks(folder string, livePaths map[string]struct{}) ([]string, error) {
	if a.dq == nil || len(livePaths) == 0 {
		return nil, nil
	}

	stored, err := a.dq.ListPathsByFolder(context.Background(), folder)
	if err != nil {
		return nil, err
	}

	// find paths that are in stored but not in livePaths
	// aka stale paths
	var stalePaths []string
	for _, p := range stored {
		if _, exists := livePaths[p]; !exists {
			stalePaths = append(stalePaths, p)
		}
	}

	if len(stalePaths) == 0 {
		return nil, nil
	}

	if _, err := a.dq.DeleteLocalTracksByPaths(context.Background(), stalePaths); err != nil {
		return nil, err
	}

	return stalePaths, nil
}

// deleteLocalTracksByPathPrefix removes all tracks whose path starts with
// prefix+"/". Used when an entire directory is moved or deleted.
// Returns the number of rows deleted.
func (a *App) deleteLocalTracksByPathPrefix(prefix string) (int64, error) {
	if a.dq == nil {
		return 0, nil
	}

	return a.dq.DeleteLocalTracksByPathPrefix(context.Background(), desktopdb.DeleteLocalTracksByPathPrefixParams{
		Path:   prefix,
		Path_2: prefix + string(filepath.Separator) + "%",
	})
}

// getLocalTracks returns all tracks whose folder is in the given list.
// If folders is empty it returns all rows.
//
// NOTE: This is a dynamic SQL query (it uses IN with variable number of placeholders), so sqlc
// cannot be used to update this method.
func (a *App) getLocalTracks(folders []string) ([]LocalTrack, error) {
	if a.dq == nil {
		return nil, nil
	}

	if len(folders) == 0 {
		rows, err := a.dq.ListAllLocalTracks(context.Background())
		if err != nil {
			return nil, err
		}

		return localTracksFromDB(rows), nil
	}

	ph := sqlPlaceholders(len(folders))
	args := make([]any, len(folders))
	for i, f := range folders {
		args[i] = f
	}

	q := `SELECT ` + localTrackCols + ` FROM local_tracks WHERE folder IN (` +
		strings.Join(ph, ",") + `) ORDER BY folder, path`

	rows, err := a.appDB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLocalTrackRows(rows)
}

// scanLocalTrackRows reads local_tracks rows into []LocalTrack.
func scanLocalTrackRows(rows *sql.Rows) ([]LocalTrack, error) {
	var tracks []LocalTrack

	for rows.Next() {
		var lt LocalTrack
		var folder string
		var hasArt int

		if err := rows.Scan(
			&lt.Path, &folder, &lt.Title, &lt.Artist, &lt.Album, &lt.AlbumArtist, &lt.Genre,
			&lt.Year, &lt.TrackNumber, &lt.DiscNumber, &lt.DurationMs, &hasArt,
		); err != nil {
			continue
		}

		lt.HasArtwork = hasArt != 0
		tracks = append(tracks, lt)
	}

	return tracks, rows.Err()
}

// getLocalTracksPage returns a paginated slice of tracks from the given folders.
//
// NOTE: This is a dynamic SQL query (it uses IN with variable number of placeholders), so sqlc
// cannot be used to update this method.
func (a *App) getLocalTracksPage(folders []string, offset, limit int) ([]LocalTrack, int, error) {
	if a.dq == nil {
		return nil, 0, nil
	}

	offset, limit = clampPagination(offset, limit)

	if len(folders) == 0 {
		total, err := a.dq.CountAllLocalTracks(context.Background())
		if err != nil {
			return nil, 0, err
		}

		rows, err := a.dq.AllLocalTracksPage(context.Background(), desktopdb.AllLocalTracksPageParams{
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			return nil, 0, err
		}

		return localTracksFromDB(rows), int(total), nil
	}

	folderArgs := make([]any, len(folders))
	for i, f := range folders {
		folderArgs[i] = f
	}

	in := strings.Join(sqlPlaceholders(len(folders)), ",")
	countQ := `SELECT COUNT(*) FROM local_tracks WHERE folder IN (` + in + `)`
	dataQ := `SELECT ` + localTrackCols + ` FROM local_tracks WHERE folder IN (` + in + `) ORDER BY album COLLATE NOCASE, disc_number, track_number LIMIT ? OFFSET ?`

	var total int
	_ = a.appDB.QueryRow(countQ, folderArgs...).Scan(&total)

	args := append(folderArgs, limit, offset)
	rows, err := a.appDB.Query(dataQ, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	tracks, err := scanLocalTrackRows(rows)
	return tracks, total, err
}

// searchLocalTracks performs a LIKE search on local tracks.
func (a *App) searchLocalTracks(folders []string, query string) ([]LocalTrack, error) {
	if a.dq == nil {
		return nil, nil
	}

	if query == "" {
		return nil, nil
	}

	like := "%" + query + "%"

	if len(folders) == 0 {
		rows, err := a.dq.SearchAllLocalTracks(context.Background(), desktopdb.SearchAllLocalTracksParams{
			Title:  like,
			Artist: like,
			Album:  like,
			Path:   like,
		})
		if err != nil {
			return nil, err
		}

		return localTracksFromDB(rows), nil
	}

	folderArgs := make([]any, len(folders))
	for i, f := range folders {
		folderArgs[i] = f
	}

	in := strings.Join(sqlPlaceholders(len(folders)), ",")
	q := `SELECT ` + localTrackCols + ` FROM local_tracks
	     WHERE folder IN (` + in + `)
	       AND (title LIKE ? OR artist LIKE ? OR album LIKE ? OR path LIKE ?)
	     ORDER BY title COLLATE NOCASE LIMIT 50`

	args := append(folderArgs, like, like, like, like)

	rows, err := a.appDB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLocalTrackRows(rows)
}

// getLocalTracksByPaths returns tracks for the given exact paths.
func (a *App) getLocalTracksByPaths(paths []string) ([]LocalTrack, error) {
	if a.dq == nil || len(paths) == 0 {
		return nil, nil
	}

	rows, err := a.dq.ListLocalTracksByPaths(context.Background(), paths)
	if err != nil {
		return nil, err
	}

	return localTracksFromDB(rows), nil
}

// getLocalAlbumGroups returns paginated album groups computed via SQL GROUP BY.
// NOTE: This is a dynamic SQL query (it uses IN with variable number of placeholders), so sqlc
// cannot be used to update this method.
func (a *App) getLocalAlbumGroups(folders []string, filter string, offset, limit int) (*LocalAlbumGroupsResult, error) {
	if a.dq == nil {
		return &LocalAlbumGroupsResult{}, nil
	}

	offset, limit = clampPagination(offset, limit)

	// Build optional WHERE clauses.
	var conditions []string
	var args []any

	conditions, args = buildFolderIN(conditions, args, folders)

	if filter != "" {
		conditions = append(conditions, "(album LIKE ? OR album_artist LIKE ? OR artist LIKE ?)")
		like := "%" + filter + "%"
		args = append(args, like, like, like)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// grpKeyExpr groups tracks by album + album_artist. Tracks with an empty
	// album are lumped into a single unorganized bucket.
	const grpKeyExpr = `CASE WHEN TRIM(album) = '' THEN ` + sqlite.UnorganizedAlbum + ` ` +
		`ELSE album || '|||' || COALESCE(NULLIF(album_artist,''), artist, ` + unknownArtist + `) END`

	// Count distinct album groups.
	countQ := `SELECT COUNT(*) FROM (
		SELECT 1 FROM local_tracks ` + where + `
		GROUP BY ` + grpKeyExpr + `
	)`

	var total int
	_ = a.appDB.QueryRow(countQ, args...).Scan(&total)

	// Fetch paginated album groups with track counts and first track paths.
	// It handles missing album information by aggregating loose tracks into a single 'unorganized' bucket.
	// The results are ordered alphabetically, meaning the unorganized bucket always appears last.
	dataQ := `SELECT
		` + grpKeyExpr + ` AS grp_key,
		CASE WHEN TRIM(album) = '' THEN ` + sqlite.UnorganizedAlbum + ` ELSE album END AS grp_name,
		CASE WHEN TRIM(album) = '' THEN 'Various' ELSE COALESCE(NULLIF(album_artist,''), artist, ` + unknownArtist + `) END AS grp_artist,
		COUNT(*) AS track_count,
		MIN(path) AS first_path
	FROM local_tracks ` + where + `
	GROUP BY grp_key
	ORDER BY CASE WHEN grp_key = ` + sqlite.UnorganizedAlbum + ` THEN 1 ELSE 0 END, grp_name COLLATE NOCASE
	LIMIT ? OFFSET ?`

	dataArgs := append(args, limit, offset)
	rows, err := a.appDB.Query(dataQ, dataArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []LocalAlbumGroup
	for rows.Next() {
		var g LocalAlbumGroup
		if err := rows.Scan(&g.Key, &g.Name, &g.Artist, &g.TrackCount, &g.FirstTrackPath); err != nil {
			continue
		}
		albums = append(albums, g)
	}

	return &LocalAlbumGroupsResult{Albums: albums, Total: total}, rows.Err()
}

// getLocalAlbumTracks returns tracks for a specific album group.
// NOTE: This is a dynamic SQL query (it uses IN with variable number of placeholders), so sqlc
// cannot be used to update this method.
func (a *App) getLocalAlbumTracks(folders []string, albumName, albumArtist string) ([]LocalTrack, error) {
	if a.dq == nil {
		return nil, nil
	}

	var conditions []string
	var args []any

	conditions, args = buildFolderIN(conditions, args, folders)

	// Handle unorganized albums
	if albumName == "Unorganized" || albumName == "" {
		conditions = append(conditions, "TRIM(album) = ''")
	} else {
		conditions = append(conditions, "album = ?")
		args = append(args, albumName)
		conditions = append(conditions, "(album_artist = ? OR (album_artist = '' AND artist = ?))")
		args = append(args, albumArtist, albumArtist)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Fetch tracks for a specific album group
	q := `SELECT ` + localTrackCols + ` FROM local_tracks ` + where + ` ORDER BY disc_number, track_number, title COLLATE NOCASE`

	rows, err := a.appDB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLocalTrackRows(rows)
}
