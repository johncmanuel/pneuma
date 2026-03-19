package models

import "time"

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

// TrackAlbumGroup is an album group derived directly from the tracks table
// using GROUP BY. It does not require a separate albums table
// and always reflects the actual music files present.
type TrackAlbumGroup struct {
	Key          string `json:"key"`    // "name|||artist" or "__unorganized__"
	Name         string `json:"name"`   // album_name
	Artist       string `json:"artist"` // album_artist
	TrackCount   int    `json:"track_count"`
	FirstTrackID string `json:"first_track_id"` // any track in this album (for artwork)
	ArtworkID    string `json:"artwork_id"`
}

type Track struct {
	ID               string     `json:"id"`
	Path             string     `json:"path"`
	Title            string     `json:"title"`
	AlbumArtist      string     `json:"album_artist"`
	AlbumName        string     `json:"album_name"`
	Genre            string     `json:"genre"`
	Year             int        `json:"year"`
	TrackNumber      int        `json:"track_number"`
	DiscNumber       int        `json:"disc_number"`
	DurationMS       int64      `json:"duration_ms"`
	BitrateKbps      int        `json:"bitrate_kbps,omitempty"`
	SampleRateHz     int        `json:"sample_rate_hz,omitempty"`
	Codec            string     `json:"codec,omitempty"`
	FileSizeBytes    int64      `json:"file_size_bytes"`
	LastModified     time.Time  `json:"last_modified"`
	Fingerprint      string     `json:"fingerprint,omitempty"`
	ReplayGainTrack  float64    `json:"replay_gain_track,omitempty"`
	ReplayGainAlbum  float64    `json:"replay_gain_album,omitempty"`
	UploadedByUserID string     `json:"uploaded_by_user_id,omitempty"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// ItemSource distinguishes how a playlist item was added.
type ItemSource string

const (
	SourceRemote   ItemSource = "remote"    // server-hosted track (has track_id)
	SourceLocalRef ItemSource = "local_ref" // reference to a local file (metadata only)
)

// Playlist represents a user-created playlist that can mix remote and local tracks.
type Playlist struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	Name             string    `json:"name"`
	Description      string    `json:"description,omitempty"`
	ArtworkPath      string    `json:"artwork_path,omitempty"`
	RemotePlaylistID string    `json:"remote_playlist_id,omitempty"` // linked server playlist ID for sync
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	// Computed/aggregate fields (not stored, populated by service layer).
	ItemCount  int   `json:"item_count,omitempty"`
	DurationMS int64 `json:"duration_ms,omitempty"`
	TrackCount int   `json:"track_count,omitempty"`  // alias for ItemCount in list views
	TotalDurMS int64 `json:"total_dur_ms,omitempty"` // alias for DurationMS in list views
}

// PlaylistItem is a single entry in a playlist. It supports both remote tracks
// (identified by TrackID UUID) and local-only references (metadata for matching).
type PlaylistItem struct {
	PlaylistID string     `json:"playlist_id"`
	Position   int        `json:"position"`
	Source     ItemSource `json:"source"`

	// Remote tracks: the server-side track UUID.
	TrackID string `json:"track_id,omitempty"`

	// Local references: display/matching metadata (never contains file paths).
	RefTitle       string `json:"ref_title,omitempty"`
	RefAlbum       string `json:"ref_album,omitempty"`
	RefAlbumArtist string `json:"ref_album_artist,omitempty"`
	RefDurationMS  int64  `json:"ref_duration_ms,omitempty"`

	AddedAt time.Time `json:"added_at"`

	Resolved  bool   `json:"resolved"`             // true when a matching track was found
	Missing   bool   `json:"missing"`              // true when no match exists on this device
	LocalPath string `json:"local_path,omitempty"` // resolved local file path (client-only, never uploaded)
}

type WatchFolder struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type PlaybackSession struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	TrackID    string    `json:"track_id"`
	PositionMS int64     `json:"position_ms"`
	Queue      []string  `json:"queue"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type AuditEntry struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Action     string    `json:"action"`      // e.g. "upload", "edit", "delete"
	TargetType string    `json:"target_type"` // e.g. "track", "user"
	TargetID   string    `json:"target_id"`
	Detail     string    `json:"detail,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

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

	EventPlaylistCreated EventType = "playlist.created"
	EventPlaylistUpdated EventType = "playlist.updated"
	EventPlaylistDeleted EventType = "playlist.deleted"
)

type Event struct {
	Type    EventType `json:"type"`
	Payload any       `json:"payload"`
}
