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
		IsAdmin:      u.IsAdmin,
		CanUpload:    u.CanUpload,
		CanEdit:      u.CanEdit,
		CanDelete:    u.CanDelete,
		CreatedAt:    ParseTime(u.CreatedAt),
		UpdatedAt:    ParseTime(u.UpdatedAt),
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
		CreatedAt:  ParseTime(a.CreatedAt),
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

// SessionByDeviceToModel converts a PlaybackSessionByDeviceRow to a domain model.
func SessionByDeviceToModel(r serverdb.PlaybackSessionByDeviceRow) *models.PlaybackSession {
	ps := &models.PlaybackSession{
		ID:         r.ID,
		UserID:     r.UserID,
		TrackID:    r.TrackID,
		PositionMS: r.PositionMs,
		QueueIndex: int(r.QueueIndex),
		RepeatMode: int(r.RepeatMode),
		Shuffle:    r.Shuffle,
		Playing:    r.Playing,
		UpdatedAt:  ParseTime(r.UpdatedAt),
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

type trackRows interface {
	serverdb.ListTracksRow |
		serverdb.ListTracksPageRow |
		serverdb.ListTracksByIDsRow |
		serverdb.SearchTracksRow |
		serverdb.ListTracksByAlbumNameRow |
		serverdb.ListTracksByAlbumNameAndArtistRow |
		serverdb.ListTracksByAlbumUnorganizedRow
}

func tracksToModels[T trackRows](rows []T) []*models.Track {
	out := make([]*models.Track, len(rows))
	for i, r := range rows {
		out[i] = trackToModel(trackRow(r))
	}
	return out
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
		LastModified:     ParseTime(r.LastModified),
		Fingerprint:      r.Fingerprint,
		UploadedByUserID: r.UploadedByUserID,
		CreatedAt:        ParseTime(r.CreatedAt),
		UpdatedAt:        ParseTime(r.UpdatedAt),
	}
	if r.DeletedAt.Valid {
		ts := ParseTime(r.DeletedAt.String)
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
	return tracksToModels(rows)
}

func ListTracksPageToModels(rows []serverdb.ListTracksPageRow) []*models.Track {
	return tracksToModels(rows)
}

func ListTracksByIDsToModels(rows []serverdb.ListTracksByIDsRow) []*models.Track {
	return tracksToModels(rows)
}

func SearchTracksToModels(rows []serverdb.SearchTracksRow) []*models.Track {
	return tracksToModels(rows)
}

func ListTracksByAlbumNameToModels(rows []serverdb.ListTracksByAlbumNameRow) []*models.Track {
	return tracksToModels(rows)
}

func ListTracksByAlbumNameAndArtistToModels(rows []serverdb.ListTracksByAlbumNameAndArtistRow) []*models.Track {
	return tracksToModels(rows)
}

func ListTracksByAlbumUnorganizedToModels(rows []serverdb.ListTracksByAlbumUnorganizedRow) []*models.Track {
	return tracksToModels(rows)
}

func WatchFolderToModel(wf serverdb.WatchFolder) *models.WatchFolder {
	return &models.WatchFolder{
		ID:        wf.ID,
		Path:      wf.Path,
		UserID:    wf.UserID,
		CreatedAt: ParseTime(wf.CreatedAt),
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
		CreatedAt:   ParseTime(p.CreatedAt),
		UpdatedAt:   ParseTime(p.UpdatedAt),
	}
}

func PlaylistRowToModel(r serverdb.ListPlaylistsByUserRow) *models.Playlist {
	return &models.Playlist{
		ID:          r.ID,
		UserID:      r.UserID,
		Name:        r.Name,
		Description: r.Description,
		ArtworkPath: r.ArtworkPath,
		CreatedAt:   ParseTime(r.CreatedAt),
		UpdatedAt:   ParseTime(r.UpdatedAt),
		ItemCount:   int(r.ItemCount),
		DurationMS:  r.TotalDurationMs,
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
		AddedAt:        ParseTime(r.AddedAt),
		Missing:        r.Missing,
	}
}

func PlaylistItemsToModels(rows []serverdb.ListPlaylistItemsRow) []models.PlaylistItem {
	out := make([]models.PlaylistItem, len(rows))
	for i, r := range rows {
		out[i] = PlaylistItemToModel(r)
	}
	return out
}

// ParseTime parses an RFC3339 string into a time.Time.
func ParseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
