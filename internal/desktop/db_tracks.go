package desktop

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
)

// upsertLocalTrack inserts or replaces a single track row.
func (a *App) upsertLocalTrack(lt LocalTrack, folder string) error {
	if a.appDB == nil {
		return fmt.Errorf("appDB not initialised")
	}
	hasArt := 0
	if lt.HasArtwork {
		hasArt = 1
	}
	_, err := a.appDB.Exec(`
INSERT INTO local_tracks (
path, folder, title, artist, album, album_artist, genre,
year, track_number, disc_number, duration_ms, has_artwork
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(path) DO UPDATE SET
folder=excluded.folder, title=excluded.title, artist=excluded.artist,
album=excluded.album, album_artist=excluded.album_artist, genre=excluded.genre,
year=excluded.year, track_number=excluded.track_number,
disc_number=excluded.disc_number, duration_ms=excluded.duration_ms,
has_artwork=excluded.has_artwork`,
		lt.Path, folder, lt.Title, lt.Artist, lt.Album, lt.AlbumArtist, lt.Genre,
		lt.Year, lt.TrackNumber, lt.DiscNumber, lt.DurationMs, hasArt,
	)
	return err
}

// deleteLocalTracksByFolder removes every track whose folder column matches.
func (a *App) deleteLocalTracksByFolder(folder string) error {
	if a.appDB == nil {
		return nil
	}
	_, err := a.appDB.Exec(`DELETE FROM local_tracks WHERE folder=?`, folder)
	return err
}

// deleteLocalTrackByPath removes a single track by its absolute file path.
func (a *App) deleteLocalTrackByPath(path string) error {
	if a.appDB == nil {
		return nil
	}
	_, err := a.appDB.Exec(`DELETE FROM local_tracks WHERE path=?`, path)
	return err
}

// pruneStaleLocalTracks removes rows for the given folder whose paths are NOT
// in livePaths. Returns the slice of deleted paths.
func (a *App) pruneStaleLocalTracks(folder string, livePaths map[string]struct{}) ([]string, error) {
	if a.appDB == nil || len(livePaths) == 0 {
		return nil, nil
	}
	// Fetch all stored paths for this folder.
	rows, err := a.appDB.Query(`SELECT path FROM local_tracks WHERE folder=?`, folder)
	if err != nil {
		return nil, err
	}
	var staleAny []any
	var stalePaths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			continue
		}
		if _, exists := livePaths[p]; !exists {
			staleAny = append(staleAny, p)
			stalePaths = append(stalePaths, p)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
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
	if a.appDB == nil {
		return 0, nil
	}
	// Match the directory itself (exact) or any file inside it.
	res, err := a.appDB.Exec(
		`DELETE FROM local_tracks WHERE path = ? OR path LIKE ?`,
		prefix, prefix+string(filepath.Separator)+"%",
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// getLocalTracks returns all tracks whose folder is in the given list.
// If folders is empty it returns all rows.
func (a *App) getLocalTracks(folders []string) ([]LocalTrack, error) {
	if a.appDB == nil {
		return nil, nil
	}

	const cols = `path, folder, title, artist, album, album_artist, genre,
year, track_number, disc_number, duration_ms, has_artwork`

	var rows *sql.Rows
	var err error
	if len(folders) == 0 {
		rows, err = a.appDB.Query(`SELECT ` + cols + ` FROM local_tracks ORDER BY folder, path`)
	} else {
		placeholders := make([]string, len(folders))
		args := make([]any, len(folders))
		for i, f := range folders {
			placeholders[i] = "?"
			args[i] = f
		}
		query := `SELECT ` + cols + ` FROM local_tracks WHERE folder IN (` +
			strings.Join(placeholders, ",") + `) ORDER BY folder, path`
		rows, err = a.appDB.Query(query, args...)
	}
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
	if a.appDB == nil {
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

	const cols = `path, folder, title, artist, album, album_artist, genre,
year, track_number, disc_number, duration_ms, has_artwork`

	var countQuery, dataQuery string
	var args []any

	if len(folders) == 0 {
		countQuery = `SELECT COUNT(*) FROM local_tracks`
		dataQuery = `SELECT ` + cols + ` FROM local_tracks ORDER BY album COLLATE NOCASE, disc_number, track_number LIMIT ? OFFSET ?`
		args = []any{limit, offset}
	} else {
		placeholders := make([]string, len(folders))
		folderArgs := make([]any, len(folders))
		for i, f := range folders {
			placeholders[i] = "?"
			folderArgs[i] = f
		}
		in := strings.Join(placeholders, ",")
		countQuery = `SELECT COUNT(*) FROM local_tracks WHERE folder IN (` + in + `)`
		dataQuery = `SELECT ` + cols + ` FROM local_tracks WHERE folder IN (` + in + `) ORDER BY album COLLATE NOCASE, disc_number, track_number LIMIT ? OFFSET ?`
		args = append(folderArgs, limit, offset)
	}

	var total int
	if len(folders) == 0 {
		_ = a.appDB.QueryRow(countQuery).Scan(&total)
	} else {
		folderArgs := make([]any, len(folders))
		for i, f := range folders {
			folderArgs[i] = f
		}
		_ = a.appDB.QueryRow(countQuery, folderArgs...).Scan(&total)
	}

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
	if a.appDB == nil {
		return nil, nil
	}
	if query == "" {
		return nil, nil
	}

	const cols = `path, folder, title, artist, album, album_artist, genre,
year, track_number, disc_number, duration_ms, has_artwork`

	like := "%" + query + "%"

	var q string
	var args []any

	if len(folders) == 0 {
		q = `SELECT ` + cols + ` FROM local_tracks
		     WHERE (title LIKE ? OR artist LIKE ? OR album LIKE ? OR path LIKE ?)
		     ORDER BY title COLLATE NOCASE LIMIT 50`
		args = []any{like, like, like, like}
	} else {
		placeholders := make([]string, len(folders))
		folderArgs := make([]any, len(folders))
		for i, f := range folders {
			placeholders[i] = "?"
			folderArgs[i] = f
		}
		in := strings.Join(placeholders, ",")
		q = `SELECT ` + cols + ` FROM local_tracks
		     WHERE folder IN (` + in + `)
		       AND (title LIKE ? OR artist LIKE ? OR album LIKE ? OR path LIKE ?)
		     ORDER BY title COLLATE NOCASE LIMIT 50`
		args = append(folderArgs, like, like, like, like)
	}

	rows, err := a.appDB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLocalTrackRows(rows)
}

// getLocalTracksByPaths returns tracks for the given exact paths.
func (a *App) getLocalTracksByPaths(paths []string) ([]LocalTrack, error) {
	if a.appDB == nil || len(paths) == 0 {
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
func (a *App) getLocalAlbumGroups(folders []string, filter string, offset, limit int) (*LocalAlbumGroupsResult, error) {
	if a.appDB == nil {
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
func (a *App) getLocalAlbumTracks(folders []string, albumName, albumArtist string) ([]LocalTrack, error) {
	if a.appDB == nil {
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
