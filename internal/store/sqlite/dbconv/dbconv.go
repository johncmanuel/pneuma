// Package dbconv provides conversion helpers between sqlc-generated serverdb
// types and the application's domain models.
package dbconv

import (
	"database/sql"
	"encoding/json"
	"time"

	"pneuma/internal/models"
	"pneuma/internal/store/sqlite/serverdb"
)

// UserToModel converts a generated serverdb.User row into a domain models.User.
func UserToModel(u serverdb.User) *models.User {
	return &models.User{
		ID:           u.ID,
		Username:     u.Username,
		PasswordHash: u.PasswordHash,
		IsAdmin:      u.IsAdmin != 0,
		CanUpload:    u.CanUpload != 0,
		CanEdit:      u.CanEdit != 0,
		CanDelete:    u.CanDelete != 0,
		CreatedAt:    parseTime(u.CreatedAt),
		UpdatedAt:    parseTime(u.UpdatedAt),
	}
}

// UsersToModels converts a slice of serverdb.User rows into domain model pointers.
func UsersToModels(rows []serverdb.User) []*models.User {
	out := make([]*models.User, len(rows))
	for i, r := range rows {
		out[i] = UserToModel(r)
	}
	return out
}

// BoolInt converts a Go bool to the int64 value stored in SQLite.
func BoolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// FormatTime formats a time.Time as RFC3339-UTC for storage in SQLite TEXT columns.
func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// AuditToModel converts a generated serverdb.AuditLog row into a domain models.AuditEntry.
func AuditToModel(a serverdb.AuditLog) models.AuditEntry {
	return models.AuditEntry{
		ID:         a.ID,
		UserID:     a.UserID,
		Action:     a.Action,
		TargetType: a.TargetType,
		TargetID:   a.TargetID,
		Detail:     a.Detail.String,
		CreatedAt:  parseTime(a.CreatedAt),
	}
}

// AuditsToModels converts a slice of serverdb.AuditLog rows into domain models.
func AuditsToModels(rows []serverdb.AuditLog) []models.AuditEntry {
	out := make([]models.AuditEntry, len(rows))
	for i, r := range rows {
		out[i] = AuditToModel(r)
	}
	return out
}

// NullStr wraps a string in sql.NullString (Valid = non-empty).
func NullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// NullInt64 wraps an int in sql.NullInt64.
func NullInt64(n int) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(n), Valid: true}
}

// NullFloat wraps a float64 in sql.NullFloat64.
func NullFloat(f float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: f, Valid: true}
}

// SessionByDeviceToModel converts a PlaybackSessionByDeviceRow to a domain model.
func SessionByDeviceToModel(r serverdb.PlaybackSessionByDeviceRow) *models.PlaybackSession {
	ps := &models.PlaybackSession{
		ID:         r.ID,
		UserID:     r.UserID,
		TrackID:    r.TrackID,
		PositionMS: r.PositionMs.Int64,
		QueueIndex: int(r.QueueIndex.Int64),
		RepeatMode: int(r.RepeatMode.Int64),
		Shuffle:    r.Shuffle.Int64 != 0,
		Playing:    r.Playing.Int64 != 0,
		UpdatedAt:  parseTime(r.UpdatedAt),
	}
	if r.QueueJson.Valid {
		if err := json.Unmarshal([]byte(r.QueueJson.String), &ps.Queue); err != nil {
			ps.Queue = []string{}
		}
	} else {
		ps.Queue = []string{}
	}
	return ps
}

// trackRow is the common field layout shared by every sqlc-generated Track*Row
// type. Because all track SELECT queries use the same column list, the
// generated row structs are structurally identical, allowing Go struct
// conversion: trackRow(anyRow).
type trackRow struct {
	ID               string
	Path             string
	Title            string
	AlbumArtist      string
	AlbumName        string
	Genre            string
	Year             int64
	TrackNumber      int64
	DiscNumber       int64
	DurationMs       int64
	BitrateKbps      int64
	SampleRateHz     int64
	Codec            string
	FileSizeBytes    int64
	LastModified     string
	Fingerprint      string
	UploadedByUserID string
	DeletedAt        sql.NullString
	CreatedAt        string
	UpdatedAt        string
}

func trackToModel(r trackRow) *models.Track {
	t := &models.Track{
		ID:               r.ID,
		Path:             r.Path,
		Title:            r.Title,
		AlbumArtist:      r.AlbumArtist,
		AlbumName:        r.AlbumName,
		Genre:            r.Genre,
		Year:             int(r.Year),
		TrackNumber:      int(r.TrackNumber),
		DiscNumber:       int(r.DiscNumber),
		DurationMS:       r.DurationMs,
		BitrateKbps:      int(r.BitrateKbps),
		SampleRateHz:     int(r.SampleRateHz),
		Codec:            r.Codec,
		FileSizeBytes:    r.FileSizeBytes,
		LastModified:     parseTime(r.LastModified),
		Fingerprint:      r.Fingerprint,
		UploadedByUserID: r.UploadedByUserID,
		CreatedAt:        parseTime(r.CreatedAt),
		UpdatedAt:        parseTime(r.UpdatedAt),
	}
	if r.DeletedAt.Valid {
		ts := parseTime(r.DeletedAt.String)
		t.DeletedAt = &ts
	}
	return t
}

func TrackByPathToModel(r serverdb.TrackByPathRow) *models.Track { return trackToModel(trackRow(r)) }
func TrackByIDToModel(r serverdb.TrackByIDRow) *models.Track     { return trackToModel(trackRow(r)) }
func TrackByFPToModel(r serverdb.TrackByFingerprintRow) *models.Track {
	return trackToModel(trackRow(r))
}
func TrackDuplicateToModel(r serverdb.TrackDuplicateByMetaRow) *models.Track {
	return trackToModel(trackRow(r))
}

func ListTracksToModels(rows []serverdb.ListTracksRow) []*models.Track {
	out := make([]*models.Track, len(rows))
	for i, r := range rows {
		out[i] = trackToModel(trackRow(r))
	}
	return out
}

func ListTracksPageToModels(rows []serverdb.ListTracksPageRow) []*models.Track {
	out := make([]*models.Track, len(rows))
	for i, r := range rows {
		out[i] = trackToModel(trackRow(r))
	}
	return out
}

func SearchTracksToModels(rows []serverdb.SearchTracksRow) []*models.Track {
	out := make([]*models.Track, len(rows))
	for i, r := range rows {
		out[i] = trackToModel(trackRow(r))
	}
	return out
}

func ListTracksByAlbumNameToModels(rows []serverdb.ListTracksByAlbumNameRow) []*models.Track {
	out := make([]*models.Track, len(rows))
	for i, r := range rows {
		out[i] = trackToModel(trackRow(r))
	}
	return out
}

func ListTracksByAlbumNameAndArtistToModels(rows []serverdb.ListTracksByAlbumNameAndArtistRow) []*models.Track {
	out := make([]*models.Track, len(rows))
	for i, r := range rows {
		out[i] = trackToModel(trackRow(r))
	}
	return out
}

func ListTracksByAlbumUnorganizedToModels(rows []serverdb.ListTracksByAlbumUnorganizedRow) []*models.Track {
	out := make([]*models.Track, len(rows))
	for i, r := range rows {
		out[i] = trackToModel(trackRow(r))
	}
	return out
}

func WatchFolderToModel(wf serverdb.WatchFolder) *models.WatchFolder {
	return &models.WatchFolder{
		ID:        wf.ID,
		Path:      wf.Path,
		UserID:    wf.UserID,
		CreatedAt: parseTime(wf.CreatedAt),
	}
}

func WatchFoldersToModels(rows []serverdb.WatchFolder) []*models.WatchFolder {
	out := make([]*models.WatchFolder, len(rows))
	for i, r := range rows {
		out[i] = WatchFolderToModel(r)
	}
	return out
}

func PlaylistToModel(p serverdb.Playlist) *models.Playlist {
	return &models.Playlist{
		ID:          p.ID,
		UserID:      p.UserID,
		Name:        p.Name,
		Description: p.Description,
		ArtworkPath: p.ArtworkPath,
		CreatedAt:   parseTime(p.CreatedAt),
		UpdatedAt:   parseTime(p.UpdatedAt),
	}
}

func PlaylistRowToModel(r serverdb.ListPlaylistsByUserRow) *models.Playlist {
	var dur int64
	switch v := r.TotalDurationMs.(type) {
	case int64:
		dur = v
	case float64:
		dur = int64(v)
	}
	return &models.Playlist{
		ID:          r.ID,
		UserID:      r.UserID,
		Name:        r.Name,
		Description: r.Description,
		ArtworkPath: r.ArtworkPath,
		CreatedAt:   parseTime(r.CreatedAt),
		UpdatedAt:   parseTime(r.UpdatedAt),
		ItemCount:   int(r.ItemCount),
		DurationMS:  dur,
	}
}

func PlaylistRowsToModels(rows []serverdb.ListPlaylistsByUserRow) []*models.Playlist {
	out := make([]*models.Playlist, len(rows))
	for i, r := range rows {
		out[i] = PlaylistRowToModel(r)
	}
	return out
}

func PlaylistItemToModel(r serverdb.ListPlaylistItemsRow) models.PlaylistItem {
	return models.PlaylistItem{
		PlaylistID:     r.PlaylistID,
		Position:       int(r.Position),
		Source:         models.ItemSource(r.Source),
		TrackID:        r.TrackID,
		RefTitle:       r.RefTitle,
		RefAlbum:       r.RefAlbum,
		RefAlbumArtist: r.RefAlbumArtist,
		RefDurationMS:  r.RefDurationMs,
		AddedAt:        parseTime(r.AddedAt),
		Missing:        r.Missing != 0,
	}
}

func PlaylistItemsToModels(rows []serverdb.ListPlaylistItemsRow) []models.PlaylistItem {
	out := make([]models.PlaylistItem, len(rows))
	for i, r := range rows {
		out[i] = PlaylistItemToModel(r)
	}
	return out
}

// ParseTime parses an RFC3339 string into a time.Time (exported for use by desktop layer).
func ParseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func parseTime(s string) time.Time {
	return ParseTime(s)
}
