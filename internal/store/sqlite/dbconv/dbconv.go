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

// ─── User ────────────────────────────────────────────────────────────────────

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

// ─── Audit ───────────────────────────────────────────────────────────────────

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

// OptionalTime returns a NullString for an optional *time.Time.
func OptionalTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: FormatTime(*t), Valid: true}
}

// ─── Device ──────────────────────────────────────────────────────────────────

// DeviceToModel converts a serverdb.Device to a domain models.Device.
func DeviceToModel(d serverdb.Device) *models.Device {
	out := &models.Device{
		ID:        d.ID,
		UserID:    d.UserID,
		Name:      d.Name,
		CreatedAt: parseTime(d.CreatedAt),
	}
	if d.LastSeenAt.Valid {
		ts := parseTime(d.LastSeenAt.String)
		out.LastSeenAt = &ts
	}
	return out
}

// DevicesToModels converts a slice of serverdb.Device to domain models.
func DevicesToModels(rows []serverdb.Device) []*models.Device {
	out := make([]*models.Device, len(rows))
	for i, r := range rows {
		out[i] = DeviceToModel(r)
	}
	return out
}

// ─── Playback Session ────────────────────────────────────────────────────────

// sessionRow is the common shape of PlaybackSessionByDeviceRow and
// PlaybackSessionsByUserRow (identical fields after COALESCE).
type sessionRow struct {
	ID         string
	DeviceID   string
	UserID     string
	TrackID    string
	PositionMs sql.NullInt64
	QueueJson  sql.NullString
	UpdatedAt  string
}

func sessionToModel(r sessionRow) *models.PlaybackSession {
	ps := &models.PlaybackSession{
		ID:         r.ID,
		DeviceID:   r.DeviceID,
		UserID:     r.UserID,
		TrackID:    r.TrackID,
		PositionMS: r.PositionMs.Int64,
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

// SessionByDeviceToModel converts a PlaybackSessionByDeviceRow to a domain model.
func SessionByDeviceToModel(r serverdb.PlaybackSessionByDeviceRow) *models.PlaybackSession {
	return sessionToModel(sessionRow(r))
}

// SessionsByUserToModels converts PlaybackSessionsByUserRow slices.
func SessionsByUserToModels(rows []serverdb.PlaybackSessionsByUserRow) []*models.PlaybackSession {
	out := make([]*models.PlaybackSession, len(rows))
	for i, r := range rows {
		out[i] = sessionToModel(sessionRow(r))
	}
	return out
}

// ─── Offline Pack ────────────────────────────────────────────────────────────

// OfflinePackToModel converts a serverdb.OfflinePack to a domain model.
func OfflinePackToModel(op serverdb.OfflinePack) *models.OfflinePack {
	return &models.OfflinePack{
		ID:           op.ID,
		UserID:       op.UserID,
		TrackID:      op.TrackID,
		LocalPath:    op.LocalPath,
		DownloadedAt: parseTime(op.DownloadedAt),
	}
}

// OfflinePacksToModels converts a slice of serverdb.OfflinePack.
func OfflinePacksToModels(rows []serverdb.OfflinePack) []*models.OfflinePack {
	out := make([]*models.OfflinePack, len(rows))
	for i, r := range rows {
		out[i] = OfflinePackToModel(r)
	}
	return out
}

// ─── Track ───────────────────────────────────────────────────────────────────

// trackRow is the common field layout shared by every sqlc-generated Track*Row
// type. Because all track SELECT queries use the same column list, the
// generated row structs are structurally identical, allowing Go struct
// conversion: trackRow(anyRow).
type trackRow struct {
	ID                  string
	Path                string
	Title               string
	ArtistID            string
	AlbumID             string
	ArtistName          string
	AlbumArtist         sql.NullString
	AlbumName           sql.NullString
	Genre               sql.NullString
	Year                sql.NullInt64
	TrackNumber         sql.NullInt64
	DiscNumber          sql.NullInt64
	DurationMs          sql.NullInt64
	BitrateKbps         sql.NullInt64
	SampleRateHz        sql.NullInt64
	Codec               sql.NullString
	FileSizeBytes       sql.NullInt64
	LastModified        string
	Fingerprint         sql.NullString
	AcousticFingerprint string
	MbRecordingID       sql.NullString
	ReplayGainTrack     sql.NullFloat64
	ReplayGainAlbum     sql.NullFloat64
	ArtworkID           string
	UploadedByUserID    string
	DeletedAt           sql.NullString
	EnrichedAt          sql.NullString
	CreatedAt           string
	UpdatedAt           string
}

func trackToModel(r trackRow) *models.Track {
	t := &models.Track{
		ID:                  r.ID,
		Path:                r.Path,
		Title:               r.Title,
		ArtistID:            r.ArtistID,
		AlbumID:             r.AlbumID,
		ArtistName:          r.ArtistName,
		AlbumArtist:         r.AlbumArtist.String,
		AlbumName:           r.AlbumName.String,
		Genre:               r.Genre.String,
		Year:                int(r.Year.Int64),
		TrackNumber:         int(r.TrackNumber.Int64),
		DiscNumber:          int(r.DiscNumber.Int64),
		DurationMS:          r.DurationMs.Int64,
		BitrateKbps:         int(r.BitrateKbps.Int64),
		SampleRateHz:        int(r.SampleRateHz.Int64),
		Codec:               r.Codec.String,
		FileSizeBytes:       r.FileSizeBytes.Int64,
		LastModified:        parseTime(r.LastModified),
		Fingerprint:         r.Fingerprint.String,
		AcousticFingerprint: r.AcousticFingerprint,
		MBRecordingID:       r.MbRecordingID.String,
		ReplayGainTrack:     r.ReplayGainTrack.Float64,
		ReplayGainAlbum:     r.ReplayGainAlbum.Float64,
		ArtworkID:           r.ArtworkID,
		UploadedByUserID:    r.UploadedByUserID,
		CreatedAt:           parseTime(r.CreatedAt),
		UpdatedAt:           parseTime(r.UpdatedAt),
	}
	if r.DeletedAt.Valid {
		ts := parseTime(r.DeletedAt.String)
		t.DeletedAt = &ts
	}
	if r.EnrichedAt.Valid {
		ts := parseTime(r.EnrichedAt.String)
		t.EnrichedAt = &ts
	}
	return t
}

// --- Track row converters (one per sqlc query) ---

func TrackByPathToModel(r serverdb.TrackByPathRow) *models.Track { return trackToModel(trackRow(r)) }
func TrackByIDToModel(r serverdb.TrackByIDRow) *models.Track     { return trackToModel(trackRow(r)) }
func TrackByFPToModel(r serverdb.TrackByFingerprintRow) *models.Track {
	return trackToModel(trackRow(r))
}
func TrackByAcousticFPToModel(r serverdb.TrackByAcousticFingerprintRow) *models.Track {
	return trackToModel(trackRow(r))
}
func TrackDuplicateToModel(r serverdb.TrackDuplicateByMetaRow) *models.Track {
	return trackToModel(trackRow(r))
}
func SearchTrackToModel(r serverdb.SearchTracksRow) *models.Track {
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

// ─── Album ───────────────────────────────────────────────────────────────────

// albumRow is the common shape of all album query row types.
type albumRow struct {
	ID          string
	Title       string
	ArtistID    string
	Year        sql.NullInt64
	MbReleaseID string
	ArtworkID   string
	CreatedAt   string
}

func albumToModel(r albumRow) *models.Album {
	return &models.Album{
		ID:          r.ID,
		Title:       r.Title,
		ArtistID:    r.ArtistID,
		Year:        int(r.Year.Int64),
		MBReleaseID: r.MbReleaseID,
		ArtworkID:   r.ArtworkID,
		CreatedAt:   parseTime(r.CreatedAt),
	}
}

func AlbumByIDToModel(r serverdb.AlbumByIDRow) *models.Album { return albumToModel(albumRow(r)) }
func AlbumByTitleArtistToModel(r serverdb.AlbumByTitleArtistRow) *models.Album {
	return albumToModel(albumRow(r))
}

func ListAlbumsToModels(rows []serverdb.ListAlbumsRow) []*models.Album {
	out := make([]*models.Album, len(rows))
	for i, r := range rows {
		out[i] = albumToModel(albumRow(r))
	}
	return out
}

func ListAlbumsPageToModels(rows []serverdb.ListAlbumsPageRow) []*models.Album {
	out := make([]*models.Album, len(rows))
	for i, r := range rows {
		out[i] = albumToModel(albumRow(r))
	}
	return out
}

func ListAlbumsPageFilteredToModels(rows []serverdb.ListAlbumsPageFilteredRow) []*models.Album {
	out := make([]*models.Album, len(rows))
	for i, r := range rows {
		out[i] = albumToModel(albumRow(r))
	}
	return out
}

// ─── Artist ──────────────────────────────────────────────────────────────────

func ArtistByNameToModel(r serverdb.ArtistByNameRow) *models.Artist {
	return &models.Artist{
		ID:         r.ID,
		Name:       r.Name,
		MBArtistID: r.MbArtistID,
		CreatedAt:  parseTime(r.CreatedAt),
	}
}

// ─── Artwork ─────────────────────────────────────────────────────────────────

func ArtworkByPathToModel(r serverdb.ArtworkByPathRow) *models.Artwork {
	return &models.Artwork{
		ID:        r.ID,
		Path:      r.Path,
		Width:     int(r.Width),
		Height:    int(r.Height),
		Format:    r.Format,
		CreatedAt: parseTime(r.CreatedAt),
	}
}

// ─── Watch Folder ────────────────────────────────────────────────────────────

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

// ─── internal helpers ────────────────────────────────────────────────────────

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
