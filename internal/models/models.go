package models

import "time"

// ─── Users & Devices ─────────────────────────────────────────────────────────

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	IsAdmin      bool      `json:"is_admin"`
	CanUpload    bool      `json:"can_upload"`
	CanEdit      bool      `json:"can_edit"`
	CanDelete    bool      `json:"can_delete"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Device struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ─── Library ─────────────────────────────────────────────────────────────────

type Artist struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	MBArtistID string    `json:"mb_artist_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type Album struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	ArtistID    string    `json:"artist_id,omitempty"`
	Year        int       `json:"year,omitempty"`
	MBReleaseID string    `json:"mb_release_id,omitempty"`
	ArtworkID   string    `json:"artwork_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Track struct {
	ID                  string     `json:"id"`
	Path                string     `json:"path"`
	Title               string     `json:"title"`
	ArtistID            string     `json:"artist_id,omitempty"`
	AlbumID             string     `json:"album_id,omitempty"`
	ArtistName          string     `json:"artist_name,omitempty"` // resolved from artists table
	AlbumArtist         string     `json:"album_artist"`
	AlbumName           string     `json:"album_name"`
	Genre               string     `json:"genre"`
	Year                int        `json:"year"`
	TrackNumber         int        `json:"track_number"`
	DiscNumber          int        `json:"disc_number"`
	DurationMS          int64      `json:"duration_ms"`
	BitrateKbps         int        `json:"bitrate_kbps,omitempty"`
	SampleRateHz        int        `json:"sample_rate_hz,omitempty"`
	Codec               string     `json:"codec,omitempty"`
	FileSizeBytes       int64      `json:"file_size_bytes"`
	LastModified        time.Time  `json:"last_modified"`
	Fingerprint         string     `json:"fingerprint,omitempty"`
	AcousticFingerprint string     `json:"acoustic_fingerprint,omitempty"`
	MBRecordingID       string     `json:"mb_recording_id,omitempty"`
	ReplayGainTrack     float64    `json:"replay_gain_track,omitempty"`
	ReplayGainAlbum     float64    `json:"replay_gain_album,omitempty"`
	ArtworkID           string     `json:"artwork_id,omitempty"`
	UploadedByUserID    string     `json:"uploaded_by_user_id,omitempty"`
	DeletedAt           *time.Time `json:"deleted_at,omitempty"`
	EnrichedAt          *time.Time `json:"enriched_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type Artwork struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Width     int       `json:"width,omitempty"`
	Height    int       `json:"height,omitempty"`
	Format    string    `json:"format,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ─── Playlists ───────────────────────────────────────────────────────────────

type Playlist struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PlaylistTrack struct {
	PlaylistID string `json:"playlist_id"`
	TrackID    string `json:"track_id"`
	Position   int    `json:"position"`
}

// ─── Watch Folders ───────────────────────────────────────────────────────────

type WatchFolder struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ─── Playback ────────────────────────────────────────────────────────────────

type PlaybackSession struct {
	ID         string    `json:"id"`
	DeviceID   string    `json:"device_id"`
	UserID     string    `json:"user_id"`
	TrackID    string    `json:"track_id"`
	PositionMS int64     `json:"position_ms"`
	Queue      []string  `json:"queue"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ─── Offline ─────────────────────────────────────────────────────────────────

type OfflinePack struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	TrackID      string    `json:"track_id"`
	LocalPath    string    `json:"local_path"`
	DownloadedAt time.Time `json:"downloaded_at"`
}

// ─── Events ──────────────────────────────────────────────────────────────────

// ─── Audit ───────────────────────────────────────────────────────────────────

type AuditEntry struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Action     string    `json:"action"`      // e.g. "upload", "edit", "delete"
	TargetType string    `json:"target_type"` // e.g. "track", "user"
	TargetID   string    `json:"target_id"`
	Detail     string    `json:"detail,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ─── Event Types ─────────────────────────────────────────────────────────────

type EventType string

const (
	EventTrackAdded       EventType = "track.added"
	EventTrackUpdated     EventType = "track.updated"
	EventTrackRemoved     EventType = "track.removed"
	EventPlaybackChanged  EventType = "playback.changed"
	EventQueueChanged     EventType = "queue.changed"
	EventScanStarted      EventType = "scan.started"
	EventScanCompleted    EventType = "scan.completed"
	EventDownloadProgress EventType = "download.progress"
)

type Event struct {
	Type    EventType `json:"type"`
	Payload any       `json:"payload"`
}
