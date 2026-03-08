package desktop

import (
	"encoding/json"
	"os/exec"
	"sync"
)

// ffprobePath is resolved once at init; empty means unavailable.
var ffprobePath string

// durationCache avoids re-running ffprobe for files that haven't changed.
// Key: "path|size|mtime_unix"  Value: duration in milliseconds.
var durationCache sync.Map

// artworkHashCache maps "path|size|mtime_unix" -> sha256 hex of the raw artwork bytes.
var artworkHashCache sync.Map

func init() {
	if p, err := exec.LookPath("ffprobe"); err == nil {
		ffprobePath = p
	}
}

// audioExts is the set of file extensions the local file dialog will accept.
var audioExts = map[string]bool{
	".mp3": true, ".flac": true, ".ogg": true, ".opus": true,
	".m4a": true, ".aac": true, ".wav": true, ".aiff": true,
	".wma": true, ".alac": true, ".ape": true, ".wv": true,
}

// thumbMaxDim is the maximum width/height for cached artwork thumbnails.
const thumbMaxDim = 400

// LocalTrack holds metadata read from a local audio file via embedded tags.
type LocalTrack struct {
	Path        string `json:"path"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	AlbumArtist string `json:"album_artist"`
	Genre       string `json:"genre"`
	Year        int    `json:"year"`
	TrackNumber int    `json:"track_number"`
	DiscNumber  int    `json:"disc_number"`
	DurationMs  int64  `json:"duration_ms"`
	HasArtwork  bool   `json:"has_artwork"`
}

// LocalAlbumGroup represents a group of tracks sharing the same album+artist.
type LocalAlbumGroup struct {
	Key            string `json:"key"`
	Name           string `json:"name"`
	Artist         string `json:"artist"`
	TrackCount     int    `json:"track_count"`
	FirstTrackPath string `json:"first_track_path"`
}

// LocalAlbumGroupsResult holds paginated album groups plus the total count.
type LocalAlbumGroupsResult struct {
	Albums []LocalAlbumGroup `json:"albums"`
	Total  int               `json:"total"`
}

// ConnectResult is returned on a successful server login.
type ConnectResult struct {
	User  json.RawMessage `json:"user"`
	Token string          `json:"token"`
}
