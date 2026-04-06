package desktop

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"

	"pneuma/internal/artwork"
	"pneuma/internal/models"
	"pneuma/internal/playlist"
	"pneuma/internal/store/sqlite/dbconv"
	"pneuma/internal/store/sqlite/desktopdb"
)

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

// CreateLocalPlaylist creates a new local playlist and returns its summary.
func (a *App) CreateLocalPlaylist(name, description string) (*LocalPlaylistSummary, error) {
	if a.dq == nil {
		return nil, fmt.Errorf("db not initialised")
	}

	now := dbconv.FormatTime(time.Now())
	id := uuid.NewString()

	if err := a.dq.CreateLocalPlaylist(context.Background(), desktopdb.CreateLocalPlaylistParams{
		ID:               id,
		Name:             name,
		Description:      description,
		ArtworkPath:      "",
		RemotePlaylistID: "",
		CreatedAt:        now,
		UpdatedAt:        now,
	}); err != nil {
		return nil, fmt.Errorf("create local playlist: %w", err)
	}

	return &LocalPlaylistSummary{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// GetLocalPlaylists returns all local playlists with aggregate counts.
func (a *App) GetLocalPlaylists() ([]LocalPlaylistSummary, error) {
	if a.dq == nil {
		return nil, fmt.Errorf("db not initialised")
	}

	rows, err := a.dq.ListLocalPlaylists(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list playlists: %w", err)
	}

	out := make([]LocalPlaylistSummary, len(rows))
	for i, r := range rows {
		var dur int64
		switch v := r.TotalDurationMs.(type) {
		case int64:
			dur = v
		case float64:
			dur = int64(v)
		}
		out[i] = LocalPlaylistSummary{
			ID:               r.ID,
			Name:             r.Name,
			Description:      r.Description,
			ArtworkPath:      r.ArtworkPath,
			RemotePlaylistID: r.RemotePlaylistID,
			ItemCount:        int(r.ItemCount),
			TotalDurationMS:  dur,
			CreatedAt:        r.CreatedAt,
			UpdatedAt:        r.UpdatedAt,
		}
	}

	return out, nil
}

// GetLocalPlaylistItems returns all items in a local playlist, ordered by position.
func (a *App) GetLocalPlaylistItems(playlistID string) ([]LocalPlaylistItem, error) {
	if a.dq == nil {
		return nil, fmt.Errorf("db not initialised")
	}

	rows, err := a.dq.ListLocalPlaylistItems(context.Background(), playlistID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}

	out := make([]LocalPlaylistItem, len(rows))
	for i, r := range rows {
		out[i] = LocalPlaylistItem{
			Position:       int(r.Position),
			Source:         r.Source,
			TrackID:        r.TrackID,
			LocalPath:      r.LocalPath,
			RefTitle:       r.RefTitle,
			RefAlbum:       r.RefAlbum,
			RefAlbumArtist: r.RefAlbumArtist,
			RefDurationMS:  r.RefDurationMs,
			AddedAt:        r.AddedAt,
		}
	}
	return out, nil
}

// UpdateLocalPlaylist updates a local playlist's metadata.
func (a *App) UpdateLocalPlaylist(id, name, description, artworkPath string) error {
	if a.dq == nil {
		return fmt.Errorf("db not initialised")
	}

	now := dbconv.FormatTime(time.Now())

	return a.dq.UpdateLocalPlaylist(context.Background(), desktopdb.UpdateLocalPlaylistParams{
		Name:             name,
		Description:      description,
		ArtworkPath:      artworkPath,
		RemotePlaylistID: "", // preserve existing; caller should use LinkLocalPlaylist
		UpdatedAt:        now,
		ID:               id,
	})
}

// DeleteLocalPlaylist removes a local playlist and all its items.
func (a *App) DeleteLocalPlaylist(id string) error {
	if a.dq == nil {
		return fmt.Errorf("db not initialised")
	}
	return a.dq.DeleteLocalPlaylist(context.Background(), id)
}

// SetLocalPlaylistItems replaces all items in a local playlist.
func (a *App) SetLocalPlaylistItems(playlistID string, items []LocalPlaylistItem) error {
	if a.dq == nil {
		return fmt.Errorf("db not initialised")
	}

	ctx := context.Background()
	if err := a.dq.DeleteLocalPlaylistItems(ctx, playlistID); err != nil {
		return fmt.Errorf("delete old items: %w", err)
	}

	for i, item := range items {
		addedAt := item.AddedAt
		if addedAt == "" {
			addedAt = dbconv.FormatTime(time.Now())
		}
		if err := a.dq.InsertLocalPlaylistItem(ctx, desktopdb.InsertLocalPlaylistItemParams{
			PlaylistID:     playlistID,
			Position:       int64(i),
			Source:         item.Source,
			TrackID:        item.TrackID,
			LocalPath:      item.LocalPath,
			RefTitle:       item.RefTitle,
			RefAlbum:       item.RefAlbum,
			RefAlbumArtist: item.RefAlbumArtist,
			RefDurationMs:  item.RefDurationMS,
			AddedAt:        addedAt,
		}); err != nil {
			return fmt.Errorf("insert item %d: %w", i, err)
		}
	}

	now := dbconv.FormatTime(time.Now())
	return a.dq.TouchLocalPlaylist(ctx, desktopdb.TouchLocalPlaylistParams{
		UpdatedAt: now,
		ID:        playlistID,
	})
}

// AddLocalPlaylistItem appends a single item to a local playlist.
func (a *App) AddLocalPlaylistItem(playlistID string, item LocalPlaylistItem) error {
	if a.dq == nil {
		return fmt.Errorf("db not initialised")
	}

	ctx := context.Background()
	count, err := a.dq.CountLocalPlaylistItems(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("count items: %w", err)
	}

	addedAt := dbconv.FormatTime(time.Now())
	if err := a.dq.InsertLocalPlaylistItem(ctx, desktopdb.InsertLocalPlaylistItemParams{
		PlaylistID:     playlistID,
		Position:       count,
		Source:         item.Source,
		TrackID:        item.TrackID,
		LocalPath:      item.LocalPath,
		RefTitle:       item.RefTitle,
		RefAlbum:       item.RefAlbum,
		RefAlbumArtist: item.RefAlbumArtist,
		RefDurationMs:  item.RefDurationMS,
		AddedAt:        addedAt,
	}); err != nil {
		return fmt.Errorf("insert item: %w", err)
	}

	now := dbconv.FormatTime(time.Now())
	return a.dq.TouchLocalPlaylist(ctx, desktopdb.TouchLocalPlaylistParams{
		UpdatedAt: now,
		ID:        playlistID,
	})
}

// UploadPlaylistToServer uploads a local playlist to the connected server.
// Only metadata references for local_ref items. Local file paths are not sent.
// Returns the remote playlist ID.
func (a *App) UploadPlaylistToServer(playlistID string) (string, error) {
	a.mu.RLock()
	serverURL := a.serverURL
	token := a.token
	a.mu.RUnlock()

	if serverURL == "" || token == "" {
		return "", fmt.Errorf("not connected to server")
	}

	if a.dq == nil {
		return "", fmt.Errorf("db not initialised")
	}
	ctx := context.Background()

	lp, err := a.dq.GetLocalPlaylistByID(ctx, playlistID)
	if err != nil {
		return "", fmt.Errorf("get local playlist: %w", err)
	}

	items, err := a.dq.ListLocalPlaylistItems(ctx, playlistID)
	if err != nil {
		return "", fmt.Errorf("list local items: %w", err)
	}

	serverItems := make([]models.PlaylistItem, 0, len(items))
	for _, it := range items {
		pi := models.PlaylistItem{
			Position:       int(it.Position),
			Source:         models.ItemSource(it.Source),
			TrackID:        it.TrackID,
			RefTitle:       it.RefTitle,
			RefAlbum:       it.RefAlbum,
			RefAlbumArtist: it.RefAlbumArtist,
			RefDurationMS:  it.RefDurationMs,
			AddedAt:        dbconv.ParseTime(it.AddedAt),
		}
		serverItems = append(serverItems, pi)
	}

	remoteID := lp.RemotePlaylistID

	// Create a new remote playlist if remoteID is empty, otherwise
	// update the existing remote playlist.
	if remoteID == "" {
		remoteID, err = a.createServerPlaylist(serverURL, token, lp.Name, lp.Description, serverItems)
		if err != nil {
			return "", err
		}

		// Link the remote playlist ID locally so it can be retrieved later.
		now := dbconv.FormatTime(time.Now())
		_ = a.dq.UpdateLocalPlaylist(ctx, desktopdb.UpdateLocalPlaylistParams{
			Name:             lp.Name,
			Description:      lp.Description,
			ArtworkPath:      lp.ArtworkPath,
			RemotePlaylistID: remoteID,
			UpdatedAt:        now,
			ID:               playlistID,
		})
	} else {
		err = a.updateServerPlaylistItems(serverURL, token, remoteID, serverItems)
		if err != nil {
			return "", err
		}
	}

	return remoteID, nil
}

// ResolvePlaylistItems attempts to match local_ref items to actual local files
// by metadata (title + album + album_artist + duration tolerance). That way,
// the desktop app can display the correct file paths for local_ref items.
func (a *App) ResolvePlaylistItems(playlistID string) ([]LocalPlaylistItem, error) {
	items, err := a.GetLocalPlaylistItems(playlistID)
	if err != nil {
		return nil, err
	}

	if a.dq == nil {
		return items, nil
	}

	allTracks, err := a.dq.ListAllLocalTracks(context.Background())
	if err != nil {
		return items, nil // should be non-fatal, so just return unresolved items
	}

	// Build lookup map keyed by normalized "title|album|album_artist".
	// Values represent the local file path and duration of the track.
	type trackRef struct {
		path       string
		durationMS int64
	}

	lookup := make(map[string][]trackRef, len(allTracks))
	for _, t := range allTracks {
		key := strings.ToLower(t.Title) + "|" + strings.ToLower(t.Album) + "|" + strings.ToLower(t.AlbumArtist)
		lookup[key] = append(lookup[key], trackRef{path: t.Path, durationMS: t.DurationMs})
	}

	// duration tolerance is needed since some tracks may have slightly different durations
	// and it's not always possible to match them exactly, so don't want to mark them as missing
	const durationToleranceMS = 3000

	// Attempt to match local_ref items to actual local files.
	// Any local_ref items that already have a path or remote items
	// (tracks streamed from the server) are automatically considered resolved
	for i := range items {
		// attempt to match local_ref items to actual local files
		if items[i].Source == string(models.SourceLocalRef) && items[i].LocalPath == "" {
			key := strings.ToLower(items[i].RefTitle) + "|" + strings.ToLower(items[i].RefAlbum) + "|" + strings.ToLower(items[i].RefAlbumArtist)
			candidates := lookup[key]
			matched := false

			for _, c := range candidates {
				durationDiff := int64(math.Abs(float64(c.durationMS - items[i].RefDurationMS)))
				if items[i].RefDurationMS == 0 || durationDiff <= durationToleranceMS {
					items[i].LocalPath = c.path
					items[i].Resolved = true
					items[i].Missing = false
					matched = true
					break
				}
			}

			if !matched {
				items[i].Missing = true
			}
		} else if items[i].Source == string(models.SourceLocalRef) && items[i].LocalPath != "" {
			items[i].Resolved = true
		} else if items[i].Source == string(models.SourceRemote) {
			items[i].Resolved = true // remote tracks are always "resolved" from desktop perspective
		}
	}
	return items, nil
}

// PickPlaylistArtwork opens a native file dialog for selecting an image,
// resizes it to a thumbnail, stores it in the thumb cache, updates the
// playlist's artwork_path in the DB, and returns the stored filename.
func (a *App) PickPlaylistArtwork(playlistID string) (string, error) {
	if a.dq == nil {
		return "", fmt.Errorf("db not initialised")
	}

	path, err := wailsrt.OpenFileDialog(a.ctx, wailsrt.OpenDialogOptions{
		Title: "Choose Playlist Artwork",
		Filters: []wailsrt.FileFilter{
			{DisplayName: "Images", Pattern: "*.png;*.jpg;*.jpeg;*.webp;*.bmp"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("file dialog: %w", err)
	}

	// user cancelled
	if path == "" {
		return "", nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	thumbData, err := artwork.ResizeToThumbnail(raw, thumbMaxDim)
	if err != nil {
		return "", err
	}

	// Content-addressed filename derived from thumbnail bytes.
	sum := sha256.Sum256(thumbData)
	artHash := "pl-" + hex.EncodeToString(sum[:])[:24]
	fileName := artHash + ".jpg"

	if err := artwork.WriteThumbnail(a.thumbDir, fileName, thumbData); err != nil {
		return "", err
	}

	now := dbconv.FormatTime(time.Now())
	if err := a.dq.UpdateLocalPlaylistArtwork(context.Background(), desktopdb.UpdateLocalPlaylistArtworkParams{
		ArtworkPath: fileName,
		UpdatedAt:   now,
		ID:          playlistID,
	}); err != nil {
		return "", fmt.Errorf("update playlist artwork: %w", err)
	}

	go a.uploadPlaylistArtToServer(playlistID, thumbData)

	return fileName, nil
}

// uploadPlaylistArtToServer uploads playlist artwork to the server.
// Called in a goroutine after local artwork is picked.
func (a *App) uploadPlaylistArtToServer(playlistID string, jpgData []byte) {
	a.mu.RLock()
	serverURL := a.serverURL
	token := a.token
	a.mu.RUnlock()

	if serverURL == "" || token == "" {
		return
	}

	// skip any playlists that aren't synced to the server
	ctx := context.Background()
	lp, err := a.dq.GetLocalPlaylistByID(ctx, playlistID)
	if err != nil || lp.RemotePlaylistID == "" {
		return
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	fw, err := w.CreateFormFile("file", "artwork.jpg")
	if err != nil {
		slog.Warn("playlist art upload: create form file", "err", err)
		return
	}
	if _, err := fw.Write(jpgData); err != nil {
		slog.Warn("playlist art upload: write data", "err", err)
		return
	}
	w.Close()

	url := fmt.Sprintf("%s/api/playlists/%s/artwork", serverURL, lp.RemotePlaylistID)
	req, err := http.NewRequest("POST", url, &body)
	if err != nil {
		slog.Warn("playlist art upload: new request", "err", err)
		return
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("playlist art upload: request failed", "err", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		slog.Warn("playlist art upload: server error", "status", resp.StatusCode, "body", string(body))
	}
}

// RefreshPlaylistArtFromServer downloads the server's artwork for a playlist
// that has a remote_playlist_id, stores it locally, and updates the DB.
// Called when a playlist.updated WS event arrives from the server.
func (a *App) RefreshPlaylistArtFromServer(playlistID string) error {
	a.mu.RLock()
	serverURL := a.serverURL
	token := a.token
	a.mu.RUnlock()

	if serverURL == "" || token == "" {
		return fmt.Errorf("not connected to server")
	}

	ctx := context.Background()
	lp, err := a.dq.GetLocalPlaylistByID(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("get local playlist: %w", err)
	}

	// playlist isn't synced to the server, so there's no artwork to refresh
	if lp.RemotePlaylistID == "" {
		return nil
	}

	url := fmt.Sprintf("%s/api/playlists/%s/art", serverURL, lp.RemotePlaylistID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download artwork: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read artwork: %w", err)
	}

	thumbData, err := artwork.ResizeToThumbnail(raw, thumbMaxDim)
	if err != nil {
		return fmt.Errorf("resize artwork: %w", err)
	}

	sum := sha256.Sum256(thumbData)

	// 24 characters is good enough
	hashPrefix := hex.EncodeToString(sum[:])[:24]

	fileName := "pl-" + hashPrefix + ".jpg"
	if err := artwork.WriteThumbnail(a.thumbDir, fileName, thumbData); err != nil {
		return fmt.Errorf("write artwork: %w", err)
	}

	now := dbconv.FormatTime(time.Now())
	if err := a.dq.UpdateLocalPlaylistArtwork(ctx, desktopdb.UpdateLocalPlaylistArtworkParams{
		ArtworkPath: fileName,
		UpdatedAt:   now,
		ID:          playlistID,
	}); err != nil {
		return fmt.Errorf("update artwork: %w", err)
	}

	return nil
}

// RefreshPlaylistArtByRemoteID finds the local playlist linked to the given
// server playlist ID and refreshes its artwork from the server.
// Called by the WS handler when playlist.updated arrives with a server playlist ID.
func (a *App) RefreshPlaylistArtByRemoteID(remotePlaylistID string) error {
	if remotePlaylistID == "" {
		return nil
	}

	ctx := context.Background()
	lp, err := a.dq.GetLocalPlaylistByRemoteID(ctx, remotePlaylistID)
	if err != nil {
		return nil
	}

	return a.RefreshPlaylistArtFromServer(lp.ID)
}

// randomTrack holds the minimum info needed for random playlist generation.
type randomTrack struct {
	source      string // "local_ref" or "remote"
	trackID     string
	localPath   string
	title       string
	album       string
	albumArtist string
	durationMS  int64
}

// GenerateRandomPlaylist creates a new playlist filled with randomly selected
// tracks targeting the given duration in minutes. Local tracks are always
// included. If useRemote is true and the app is connected to a server, remote
// tracks are added to the pool as well, producing a mix of both sources.
func (a *App) GenerateRandomPlaylist(name, description string, durationMinutes int, useRemote bool) (*LocalPlaylistSummary, error) {
	if a.dq == nil {
		return nil, fmt.Errorf("db not initialised")
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("playlist name is required")
	}
	if durationMinutes <= 0 {
		return nil, fmt.Errorf("duration must be at least 1 minute")
	}

	targetMS := int64(durationMinutes) * 60 * 1000

	var candidates []randomTrack

	// TODO: improve efficiency by limiting number of local tracks loaded from DB
	// and remote tracks from server. this could be a bottleneck if the user has a lot of
	// local songs, i feel.

	rows, err := a.dq.ListAllLocalTracks(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list local tracks: %w", err)
	}
	for _, t := range rows {
		if t.DurationMs > 0 {
			candidates = append(candidates, randomTrack{
				source:      "local_ref",
				localPath:   t.Path,
				title:       t.Title,
				album:       t.Album,
				albumArtist: t.AlbumArtist,
				durationMS:  t.DurationMs,
			})
		}
	}

	if useRemote {
		a.mu.RLock()
		serverURL := a.serverURL
		token := a.token
		a.mu.RUnlock()

		const pageSize = 200

		if serverURL != "" && token != "" {
			tracks, err := a.fetchAllRemoteTracks(serverURL, token, pageSize)
			if err == nil {
				for _, t := range tracks {
					if t.DurationMS > 0 {
						candidates = append(candidates, randomTrack{
							source:      "remote",
							trackID:     t.ID,
							title:       t.Title,
							album:       t.AlbumName,
							albumArtist: t.AlbumArtist,
							durationMS:  t.DurationMS,
						})
					}
				}
			}
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no track candidates available")
	}

	// Deduplicate by title+album+album_artist
	deduped := make([]randomTrack, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, t := range candidates {
		key := strings.ToLower(t.title) + "|" + strings.ToLower(t.album) + "|" + strings.ToLower(t.albumArtist)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, t)
	}

	if len(deduped) == 0 {
		return nil, fmt.Errorf("no track candidates available after deduplication")
	}

	durations := make([]int64, len(deduped))
	for i, t := range deduped {
		durations[i] = t.durationMS
	}
	selected := playlist.SelectRandomByDuration(durations, targetMS)

	if len(selected) == 0 {
		return nil, fmt.Errorf("no selected tracks available")
	}

	pl, err := a.CreateLocalPlaylist(name, description)
	if err != nil {
		return nil, err
	}

	items := make([]LocalPlaylistItem, len(selected))
	for i, idx := range selected {
		t := deduped[idx]
		items[i] = LocalPlaylistItem{
			Position:       i,
			Source:         t.source,
			TrackID:        t.trackID,
			LocalPath:      t.localPath,
			RefTitle:       t.title,
			RefAlbum:       t.album,
			RefAlbumArtist: t.albumArtist,
			RefDurationMS:  t.durationMS,
		}
	}

	if err := a.SetLocalPlaylistItems(pl.ID, items); err != nil {
		_ = a.DeleteLocalPlaylist(pl.ID)
		return nil, fmt.Errorf("set items: %w", err)
	}

	return pl, nil
}

// fetchAllRemoteTracks retrieves all tracks from the connected server via
// paginated requests.
func (a *App) fetchAllRemoteTracks(serverURL, token string, pageSize int) ([]models.Track, error) {
	offset := 0
	var all []models.Track

	for {
		url := fmt.Sprintf("%s/api/library/tracks?offset=%d&limit=%d", serverURL, offset, pageSize)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("server unreachable: %w", err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch tracks failed (%d): %s", resp.StatusCode, string(body))
		}

		var page struct {
			Tracks []models.Track `json:"tracks"`
			Total  int            `json:"total"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		all = append(all, page.Tracks...)

		if offset+len(page.Tracks) >= page.Total {
			break
		}
		offset += pageSize
	}

	return all, nil
}
