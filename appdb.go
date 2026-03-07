package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// openAppDB opens (or creates) the app-local SQLite database used to persist
// desktop client state: local folder list, track cache, recent albums, etc.
// The database is stored in the OS user-cache directory so it survives app
// updates but is clearly separate from user documents.
//
// Linux:   ~/.cache/pneuma/app.db
// macOS:   ~/Library/Caches/pneuma/app.db
// Windows: %LocalAppData%\pneuma\app.db
func openAppDB() (*sql.DB, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("user cache dir: %w", err)
	}
	dir := filepath.Join(cacheDir, "pneuma")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir appdb: %w", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "app.db"))
	if err != nil {
		return nil, err
	}

	// Generic key-value table (settings, small blobs, etc.).
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS kv (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create kv table: %w", err)
	}

	// Migration: drop the old local_tracks table if it still has the
	// now-removed fingerprint / acoustic_fingerprint columns.  The table
	// is a pure scan cache so data loss is safe — next scan repopulates it.
	var hasOldSchema bool
	if err := db.QueryRow(
		`SELECT count(*) > 0 FROM pragma_table_info('local_tracks') WHERE name='fingerprint'`,
	).Scan(&hasOldSchema); err == nil && hasOldSchema {
		slog.Info("appdb: removing obsolete fingerprint columns — table will be rebuilt on next scan")
		_, _ = db.Exec(`DROP TABLE IF EXISTS local_tracks`)
		_, _ = db.Exec(`DELETE FROM kv WHERE key = 'local_dupes_cache'`)
	}
	// Also drop the old fingerprint indexes if they somehow survived.
	_, _ = db.Exec(`DROP INDEX IF EXISTS idx_lt_fp`)
	_, _ = db.Exec(`DROP INDEX IF EXISTS idx_lt_afp`)

	// Relational table for locally-scanned tracks (metadata only, no hashes).
	const createLocalTracks = `
CREATE TABLE IF NOT EXISTS local_tracks (
path         TEXT PRIMARY KEY,
folder       TEXT NOT NULL,
title        TEXT NOT NULL DEFAULT '',
artist       TEXT NOT NULL DEFAULT '',
album        TEXT NOT NULL DEFAULT '',
album_artist TEXT NOT NULL DEFAULT '',
genre        TEXT NOT NULL DEFAULT '',
year         INTEGER NOT NULL DEFAULT 0,
track_number INTEGER NOT NULL DEFAULT 0,
disc_number  INTEGER NOT NULL DEFAULT 0,
duration_ms  INTEGER NOT NULL DEFAULT 0,
has_artwork  INTEGER NOT NULL DEFAULT 0
)`
	if _, err = db.Exec(createLocalTracks); err != nil {
		db.Close()
		return nil, fmt.Errorf("create local_tracks table: %w", err)
	}

	// Index for fast per-folder deletion and loading.
	if _, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_lt_folder ON local_tracks(folder)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create index: %w", err)
	}

	// One-time migration: clean up old KV-blob track cache if present.
	_, _ = db.Exec(`DELETE FROM kv WHERE key = 'local_tracks_cache'`)

	return db, nil
}

// AppDBGet returns the stored value for key, or "" if the key does not exist.
func (a *App) AppDBGet(key string) string {
	if a.appDB == nil {
		return ""
	}
	var val string
	if err := a.appDB.QueryRow(`SELECT value FROM kv WHERE key=?`, key).Scan(&val); err != nil {
		return ""
	}
	return val
}

// AppDBSet stores or replaces value for key (upsert).
func (a *App) AppDBSet(key, value string) error {
	if a.appDB == nil {
		return fmt.Errorf("appDB not initialised")
	}
	_, err := a.appDB.Exec(
		`INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		key, value,
	)
	return err
}

// AppDBDelete removes key from the store. It is a no-op when the key does not exist.
func (a *App) AppDBDelete(key string) error {
	if a.appDB == nil {
		return nil
	}
	_, err := a.appDB.Exec(`DELETE FROM kv WHERE key=?`, key)
	return err
}

// closeAppDB is called from shutdown.
func (a *App) closeAppDB() {
	if a.appDB != nil {
		if err := a.appDB.Close(); err != nil {
			slog.Warn("appDB close error", "err", err)
		}
		a.appDB = nil
	}
}

// ─── local_tracks helpers ────────────────────────────────────────────────────

// LocalDuplicateGroup is a set of local tracks that share the same metadata
// (title, album, album_artist) and are therefore considered likely duplicates.
type LocalDuplicateGroup struct {
	Key    string       `json:"key"`    // "title|album|album_artist" (lower-cased)
	Tracks []LocalTrack `json:"tracks"` // 2+ copies
}

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

// findLocalDuplicates returns groups of tracks that share the same metadata.
// Only groups with 2+ members are returned.  A single CTE query does all the work in SQLite.
func (a *App) findLocalDuplicates(folders []string) ([]LocalDuplicateGroup, error) {
	if a.appDB == nil {
		return nil, nil
	}

	// Build optional WHERE clause for folder filtering.
	folderWhere := ""
	var folderArgs []any
	if len(folders) > 0 {
		placeholders := make([]string, len(folders))
		for i, f := range folders {
			placeholders[i] = "?"
			folderArgs = append(folderArgs, f)
		}
		folderWhere = "WHERE folder IN (" + strings.Join(placeholders, ",") + ")"
	}

	// CTE finds (title, album, album_artist, track_number, disc_number, duration_ms) combos
	// with >=2 rows, then the outer query retrieves the full metadata in one pass.
	query := `
WITH dup_keys AS (
SELECT lower(title) AS t, lower(album) AS al, lower(album_artist) AS aa,
       track_number AS tn, disc_number AS dn, duration_ms AS dur
FROM local_tracks ` + folderWhere + `
GROUP BY lower(title), lower(album), lower(album_artist), track_number, disc_number, duration_ms
HAVING count(*) >= 2
)
SELECT lt.path, lt.folder, lt.title, lt.artist, lt.album, lt.album_artist,
       lt.genre, lt.year, lt.track_number, lt.disc_number, lt.duration_ms, lt.has_artwork
FROM local_tracks lt
INNER JOIN dup_keys dk
       ON lower(lt.title)        = dk.t
      AND lower(lt.album)        = dk.al
      AND lower(lt.album_artist) = dk.aa
      AND lt.track_number       = dk.tn
      AND lt.disc_number        = dk.dn
      AND lt.duration_ms        = dk.dur
` + folderWhere + `
ORDER BY dk.t, dk.al, dk.aa, dk.tn, dk.dn, dk.dur, lt.path`

	// folderArgs used twice: once in the CTE, once in the outer WHERE.
	args := append(folderArgs, folderArgs...)

	rows, err := a.appDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []LocalDuplicateGroup
	var curKey string
	for rows.Next() {
		var lt LocalTrack
		var folder string
		var hasArt int
		if err := rows.Scan(
			&lt.Path, &folder, &lt.Title, &lt.Artist, &lt.Album, &lt.AlbumArtist,
			&lt.Genre, &lt.Year, &lt.TrackNumber, &lt.DiscNumber, &lt.DurationMs, &hasArt,
		); err != nil {
			continue
		}
		lt.HasArtwork = hasArt != 0

		key := strings.ToLower(lt.Title) + "|" + strings.ToLower(lt.Album) + "|" + strings.ToLower(lt.AlbumArtist) + "|" + fmt.Sprintf("%d|%d|%d", lt.TrackNumber, lt.DiscNumber, lt.DurationMs)
		if key != curKey {
			groups = append(groups, LocalDuplicateGroup{Key: key})
			curKey = key
		}
		groups[len(groups)-1].Tracks = append(groups[len(groups)-1].Tracks, lt)
	}
	return groups, rows.Err()
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
