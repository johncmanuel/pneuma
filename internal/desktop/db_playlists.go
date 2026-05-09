package desktop

import "fmt"

// LocalPlaylistSummary is the list-view representation of a local playlist.
type LocalPlaylistSummary struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	ArtworkPath      string `json:"artwork_path"`
	RemotePlaylistID string `json:"remote_playlist_id"`
	ItemCount        int    `json:"item_count"`
	TotalDurationMS  int64  `json:"total_duration_ms"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// LocalPlaylistItem is the frontend-facing representation of a single item
// within a playlist.
type LocalPlaylistItem struct {
	Position       int    `json:"position"`
	Source         string `json:"source"` // "remote" | "local_ref"
	TrackID        string `json:"track_id,omitempty"`
	LocalPath      string `json:"local_path,omitempty"`
	RefTitle       string `json:"ref_title"`
	RefAlbum       string `json:"ref_album"`
	RefAlbumArtist string `json:"ref_album_artist"`
	RefDurationMS  int64  `json:"ref_duration_ms"`
	AddedAt        string `json:"added_at"`

	// Resolved at runtime by the frontend (not stored).
	Resolved bool `json:"resolved"`
	Missing  bool `json:"missing"`
}

// pm is a helper function that returns the playlist manager.
func (a *App) pm() (*PlaylistManager, error) {
	if a.playlistManager == nil {
		return nil, fmt.Errorf("playlist manager not initialized")
	}
	return a.playlistManager, nil
}

// CreateLocalPlaylist creates a new local playlist and returns its summary.
func (a *App) CreateLocalPlaylist(name, description string) (*LocalPlaylistSummary, error) {
	pm, err := a.pm()
	if err != nil {
		return nil, err
	}
	return pm.CreateLocalPlaylist(name, description)
}

// GetLocalPlaylists returns all local playlists with aggregate counts.
func (a *App) GetLocalPlaylists() ([]LocalPlaylistSummary, error) {
	pm, err := a.pm()
	if err != nil {
		return nil, err
	}
	return pm.GetLocalPlaylists()
}

// GetLocalPlaylistItems returns all items in a local playlist, ordered by position.
func (a *App) GetLocalPlaylistItems(playlistID string) ([]LocalPlaylistItem, error) {
	pm, err := a.pm()
	if err != nil {
		return nil, err
	}
	return pm.GetLocalPlaylistItems(playlistID)
}

// UpdateLocalPlaylist updates a local playlist's metadata.
func (a *App) UpdateLocalPlaylist(id, name, description, artworkPath string) error {
	pm, err := a.pm()
	if err != nil {
		return err
	}
	return pm.UpdateLocalPlaylist(id, name, description, artworkPath)
}

// LinkLocalPlaylistToRemote updates the local playlist to store its linked remote_playlist_id.
func (a *App) LinkLocalPlaylistToRemote(id, remoteID string) error {
	pm, err := a.pm()
	if err != nil {
		return err
	}
	return pm.LinkLocalPlaylistToRemote(id, remoteID)
}

// DeleteLocalPlaylist removes a local playlist and all its items.
func (a *App) DeleteLocalPlaylist(id string) error {
	pm, err := a.pm()
	if err != nil {
		return err
	}
	return pm.DeleteLocalPlaylist(id)
}

// SetLocalPlaylistItems replaces all items in a local playlist.
func (a *App) SetLocalPlaylistItems(playlistID string, items []LocalPlaylistItem) error {
	pm, err := a.pm()
	if err != nil {
		return err
	}
	return pm.SetLocalPlaylistItems(playlistID, items)
}

// AddLocalPlaylistItem appends a single item to a local playlist.
func (a *App) AddLocalPlaylistItem(playlistID string, item LocalPlaylistItem) error {
	pm, err := a.pm()
	if err != nil {
		return err
	}
	return pm.AddLocalPlaylistItem(playlistID, item)
}

// UploadPlaylistToServer uploads a local playlist to the connected server.
// Returns the remote playlist ID.
func (a *App) UploadPlaylistToServer(playlistID string) (string, error) {
	pm, err := a.pm()
	if err != nil {
		return "", err
	}
	return pm.UploadPlaylistToServer(playlistID)
}

// PickPlaylistArtwork opens a native file dialog, resizes the selected image,
// stores it in the thumb cache, and updates the playlist's artwork_path in the DB.
func (a *App) PickPlaylistArtwork(playlistID string) (string, error) {
	pm, err := a.pm()
	if err != nil {
		return "", err
	}
	return pm.PickPlaylistArtwork(playlistID)
}

// RefreshPlaylistArtFromServer downloads the server's artwork for a playlist
// that has a remote_playlist_id, stores it locally, and updates the DB.
func (a *App) RefreshPlaylistArtFromServer(playlistID string) error {
	pm, err := a.pm()
	if err != nil {
		return err
	}
	return pm.RefreshPlaylistArtFromServer(playlistID)
}

// RefreshPlaylistArtByRemoteID finds the local playlist linked to the given
// server playlist ID and refreshes its artwork from the server.
func (a *App) RefreshPlaylistArtByRemoteID(remotePlaylistID string) error {
	pm, err := a.pm()
	if err != nil {
		return err
	}
	return pm.RefreshPlaylistArtByRemoteID(remotePlaylistID)
}
