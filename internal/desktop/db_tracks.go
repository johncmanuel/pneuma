package desktop

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"pneuma/internal/store/sqlite/dbconv"
	"pneuma/internal/store/sqlite/desktopdb"
)

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
		HasArtwork:  row.HasArtwork != 0,
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
		HasArtwork:  dbconv.BoolInt(lt.HasArtwork),
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

// pruneStaleLocalTracks removes rows for the given folder whose paths are NOT
// in livePaths. Returns the slice of deleted paths.
// NOTE: This is a dynamic SQL query (it uses IN with variable number of placeholders), so sqlc
// cannot be used to update this method.
func (a *App) pruneStaleLocalTracks(folder string, livePaths map[string]struct{}) ([]string, error) {
	if a.dq == nil || len(livePaths) == 0 {
		return nil, nil
	}
	// Fetch all stored paths for this folder.
	stored, err := a.dq.ListPathsByFolder(context.Background(), folder)
	if err != nil {
		return nil, err
	}
	var staleAny []any
	var stalePaths []string
	for _, p := range stored {
		if _, exists := livePaths[p]; !exists {
			staleAny = append(staleAny, p)
			stalePaths = append(stalePaths, p)
		}
	}
	if len(staleAny) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(staleAny))
	for i := range staleAny {
		placeholders[i] = "?"
	}
	_, err = a.appDB.Exec(
		`DELETE FROM local_tracks WHERE path IN (`+strings.Join(placeholders, ",")+`)`,
		staleAny...,
	)
	if err != nil {
		return nil, err
	}
	return stalePaths, nil
}

// deleteLocalTracksByPathPrefix removes all tracks whose path starts with
// prefix+"/" — used when an entire directory is moved or deleted.
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

	const cols = `path, folder, title, artist, album, album_artist, genre,
year, track_number, disc_number, duration_ms, has_artwork`

	placeholders := make([]string, len(folders))
	args := make([]any, len(folders))
	for i, f := range folders {
		placeholders[i] = "?"
		args[i] = f
	}
	query := `SELECT ` + cols + ` FROM local_tracks WHERE folder IN (` +
		strings.Join(placeholders, ",") + `) ORDER BY folder, path`
	rows, err := a.appDB.Query(query, args...)
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
func (a *App) getLocalTracksPage(folders []string, offset, limit int) ([]LocalTrack, int, error) {
	if a.dq == nil {
		return nil, 0, nil
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

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

	const cols = `path, folder, title, artist, album, album_artist, genre,
year, track_number, disc_number, duration_ms, has_artwork`

	placeholders := make([]string, len(folders))
	folderArgs := make([]any, len(folders))
	for i, f := range folders {
		placeholders[i] = "?"
		folderArgs[i] = f
	}
	in := strings.Join(placeholders, ",")
	countQuery := `SELECT COUNT(*) FROM local_tracks WHERE folder IN (` + in + `)`
	dataQuery := `SELECT ` + cols + ` FROM local_tracks WHERE folder IN (` + in + `) ORDER BY album COLLATE NOCASE, disc_number, track_number LIMIT ? OFFSET ?`

	var total int
	_ = a.appDB.QueryRow(countQuery, folderArgs...).Scan(&total)

	args := append(folderArgs, limit, offset)
	rows, err := a.appDB.Query(dataQuery, args...)
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

	const cols = `path, folder, title, artist, album, album_artist, genre,
year, track_number, disc_number, duration_ms, has_artwork`

	placeholders := make([]string, len(folders))
	folderArgs := make([]any, len(folders))
	for i, f := range folders {
		placeholders[i] = "?"
		folderArgs[i] = f
	}
	in := strings.Join(placeholders, ",")
	q := `SELECT ` + cols + ` FROM local_tracks
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
// NOTE: This is a dynamic SQL query (it uses IN with variable number of placeholders), so sqlc
// cannot be used to update this method.
func (a *App) getLocalTracksByPaths(paths []string) ([]LocalTrack, error) {
	if a.dq == nil || len(paths) == 0 {
		return nil, nil
	}

	const cols = `path, folder, title, artist, album, album_artist, genre,
year, track_number, disc_number, duration_ms, has_artwork`

	placeholders := make([]string, len(paths))
	args := make([]any, len(paths))
	for i, p := range paths {
		placeholders[i] = "?"
		args[i] = p
	}

	q := `SELECT ` + cols + ` FROM local_tracks WHERE path IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := a.appDB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLocalTrackRows(rows)
}

// getLocalAlbumGroups returns paginated album groups computed via SQL GROUP BY.
// NOTE: This is a dynamic SQL query (it uses IN with variable number of placeholders), so sqlc
// cannot be used to update this method.
func (a *App) getLocalAlbumGroups(folders []string, filter string, offset, limit int) (*LocalAlbumGroupsResult, error) {
	if a.dq == nil {
		return &LocalAlbumGroupsResult{}, nil
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	// Build optional WHERE clauses.
	var conditions []string
	var args []any

	if len(folders) > 0 {
		placeholders := make([]string, len(folders))
		for i, f := range folders {
			placeholders[i] = "?"
			args = append(args, f)
		}
		conditions = append(conditions, "folder IN ("+strings.Join(placeholders, ",")+")")
	}

	if filter != "" {
		conditions = append(conditions, "(album LIKE ? OR album_artist LIKE ? OR artist LIKE ?)")
		like := "%" + filter + "%"
		args = append(args, like, like, like)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total groups.
	countQ := `SELECT COUNT(*) FROM (
		SELECT 1 FROM local_tracks ` + where + `
		GROUP BY CASE WHEN TRIM(album) = '' THEN '__unorganized__' ELSE album || '|||' || COALESCE(NULLIF(album_artist,''), artist, 'Unknown Artist') END
	)`
	var total int
	_ = a.appDB.QueryRow(countQ, args...).Scan(&total)

	// Fetch the page of groups.
	dataQ := `SELECT
		CASE WHEN TRIM(album) = '' THEN '__unorganized__' ELSE album || '|||' || COALESCE(NULLIF(album_artist,''), artist, 'Unknown Artist') END AS grp_key,
		CASE WHEN TRIM(album) = '' THEN 'Unorganized' ELSE album END AS grp_name,
		CASE WHEN TRIM(album) = '' THEN 'Various' ELSE COALESCE(NULLIF(album_artist,''), artist, 'Unknown Artist') END AS grp_artist,
		COUNT(*) AS track_count,
		MIN(path) AS first_path
	FROM local_tracks ` + where + `
	GROUP BY grp_key
	ORDER BY CASE WHEN grp_key = '__unorganized__' THEN 1 ELSE 0 END, grp_name COLLATE NOCASE
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

	const cols = `path, folder, title, artist, album, album_artist, genre,
year, track_number, disc_number, duration_ms, has_artwork`

	var conditions []string
	var args []any

	if len(folders) > 0 {
		placeholders := make([]string, len(folders))
		for i, f := range folders {
			placeholders[i] = "?"
			args = append(args, f)
		}
		conditions = append(conditions, "folder IN ("+strings.Join(placeholders, ",")+")")
	}

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

	q := `SELECT ` + cols + ` FROM local_tracks ` + where + ` ORDER BY disc_number, track_number, title COLLATE NOCASE`
	rows, err := a.appDB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLocalTrackRows(rows)
}
