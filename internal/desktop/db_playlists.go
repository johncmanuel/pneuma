package desktop

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/image/draw"

	"pneuma/internal/models"
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

	thumbData, err := resizeToThumbnail(raw)
	if err != nil {
		return "", err
	}

	// Content-addressed filename derived from thumbnail bytes.
	sum := sha256.Sum256(thumbData)
	artHash := "pl-" + hex.EncodeToString(sum[:])[:24]
	fileName := artHash + ".jpg"

	if err := writeThumbnail(a.thumbDir, fileName, thumbData); err != nil {
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

	return fileName, nil
}

// resizeToThumbnail decodes raw image bytes, scales the image down to
// thumbMaxDim (preserving aspect ratio), and returns the result encoded as
// a JPEG. If the image is already within thumbMaxDim it is only re-encoded.
func resizeToThumbnail(raw []byte) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	b := src.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	dstW, dstH := srcW, srcH

	if srcW > thumbMaxDim || srcH > thumbMaxDim {
		if srcW >= srcH {
			dstW = thumbMaxDim
			dstH = srcH * thumbMaxDim / srcW
		} else {
			dstH = thumbMaxDim
			dstW = srcW * thumbMaxDim / srcH
		}
	}

	// Clamp to at least 1x1 to avoid zero-dimension images.
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}

	dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("encode thumbnail: %w", err)
	}

	return buf.Bytes(), nil
}

// writeThumbnail atomically writes data to dir/fileName using a temp file and later
// renames it so that a crash mid-write never leaves a corrupt file behind.
func writeThumbnail(dir, fileName string, data []byte) error {
	thumbPath := filepath.Join(dir, fileName)

	tmp, err := os.CreateTemp(dir, "tmp-pl-*.jpg")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write thumbnail: %w", err)
	}
	tmp.Close()

	if err := os.Rename(tmpName, thumbPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename thumbnail: %w", err)
	}

	return nil
}
