package desktop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
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

// PlaylistManager handles all playlist business logic, coordinating between
// the LocalStore, LocalStreamer, and ServerClient. It is the single place
// responsible for CRUD operations, random generation, and remote syncing.
type PlaylistManager struct {
	ctx      context.Context
	store    *AppStore
	streamer *LocalStreamer
	client   *ServerClient
}

// NewPlaylistManager creates a new PlaylistManager.
func NewPlaylistManager(ctx context.Context, store *AppStore, streamer *LocalStreamer, client *ServerClient) *PlaylistManager {
	return &PlaylistManager{
		ctx:      ctx,
		store:    store,
		streamer: streamer,
		client:   client,
	}
}

// CreateLocalPlaylist creates a new local playlist and returns its summary.
func (pm *PlaylistManager) CreateLocalPlaylist(name, description string) (*LocalPlaylistSummary, error) {
	if pm.store == nil {
		return nil, fmt.Errorf("db not initialised")
	}

	now := dbconv.FormatTime(time.Now())
	id := uuid.NewString()

	if err := pm.store.queries().CreateLocalPlaylist(context.Background(), desktopdb.CreateLocalPlaylistParams{
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
func (pm *PlaylistManager) GetLocalPlaylists() ([]LocalPlaylistSummary, error) {
	if pm.store == nil {
		return nil, fmt.Errorf("db not initialised")
	}

	rows, err := pm.store.queries().ListLocalPlaylists(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list playlists: %w", err)
	}

	out := make([]LocalPlaylistSummary, len(rows))
	for i, r := range rows {
		out[i] = LocalPlaylistSummary{
			ID:               r.ID,
			Name:             r.Name,
			Description:      r.Description,
			ArtworkPath:      r.ArtworkPath,
			RemotePlaylistID: r.RemotePlaylistID,
			ItemCount:        int(r.ItemCount),
			TotalDurationMS:  r.TotalDurationMs,
			CreatedAt:        r.CreatedAt,
			UpdatedAt:        r.UpdatedAt,
		}
	}

	return out, nil
}

// GetLocalPlaylistItems returns all items in a local playlist, ordered by position.
func (pm *PlaylistManager) GetLocalPlaylistItems(playlistID string) ([]LocalPlaylistItem, error) {
	if pm.store == nil {
		return nil, fmt.Errorf("db not initialised")
	}

	rows, err := pm.store.queries().ListLocalPlaylistItems(context.Background(), playlistID)
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
func (pm *PlaylistManager) UpdateLocalPlaylist(id, name, description, artworkPath string) error {
	if pm.store == nil {
		return fmt.Errorf("db not initialised")
	}

	pl, err := pm.store.queries().GetLocalPlaylistByID(context.Background(), id)
	if err != nil {
		return fmt.Errorf("get local playlist: %w", err)
	}

	now := dbconv.FormatTime(time.Now())

	return pm.store.queries().UpdateLocalPlaylist(context.Background(), desktopdb.UpdateLocalPlaylistParams{
		Name:             name,
		Description:      description,
		ArtworkPath:      artworkPath,
		RemotePlaylistID: pl.RemotePlaylistID,
		UpdatedAt:        now,
		ID:               id,
	})
}

// LinkLocalPlaylistToRemote updates the local playlist to store its linked remote_playlist_id.
func (pm *PlaylistManager) LinkLocalPlaylistToRemote(id, remoteID string) error {
	if pm.store == nil {
		return fmt.Errorf("db not initialised")
	}

	pl, err := pm.store.queries().GetLocalPlaylistByID(context.Background(), id)
	if err != nil {
		return fmt.Errorf("get local playlist: %w", err)
	}

	now := dbconv.FormatTime(time.Now())

	return pm.store.queries().UpdateLocalPlaylist(context.Background(), desktopdb.UpdateLocalPlaylistParams{
		Name:             pl.Name,
		Description:      pl.Description,
		ArtworkPath:      pl.ArtworkPath,
		RemotePlaylistID: remoteID,
		UpdatedAt:        now,
		ID:               id,
	})
}

// DeleteLocalPlaylist removes a local playlist and all its items.
func (pm *PlaylistManager) DeleteLocalPlaylist(id string) error {
	if pm.store == nil {
		return fmt.Errorf("db not initialised")
	}
	return pm.store.queries().DeleteLocalPlaylist(context.Background(), id)
}

// SetLocalPlaylistItems replaces all items in a local playlist.
func (pm *PlaylistManager) SetLocalPlaylistItems(playlistID string, items []LocalPlaylistItem) error {
	if pm.store == nil {
		return fmt.Errorf("db not initialised")
	}

	ctx := context.Background()
	if err := pm.store.queries().DeleteLocalPlaylistItems(ctx, playlistID); err != nil {
		return fmt.Errorf("delete old items: %w", err)
	}

	for i, item := range items {
		addedAt := item.AddedAt
		if addedAt == "" {
			addedAt = dbconv.FormatTime(time.Now())
		}
		if err := pm.store.queries().InsertLocalPlaylistItem(ctx, desktopdb.InsertLocalPlaylistItemParams{
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
	return pm.store.queries().TouchLocalPlaylist(ctx, desktopdb.TouchLocalPlaylistParams{
		UpdatedAt: now,
		ID:        playlistID,
	})
}

// AddLocalPlaylistItem appends a single item to a local playlist.
func (pm *PlaylistManager) AddLocalPlaylistItem(playlistID string, item LocalPlaylistItem) error {
	if pm.store == nil {
		return fmt.Errorf("db not initialised")
	}

	ctx := context.Background()
	count, err := pm.store.queries().CountLocalPlaylistItems(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("count items: %w", err)
	}

	addedAt := dbconv.FormatTime(time.Now())
	if err := pm.store.queries().InsertLocalPlaylistItem(ctx, desktopdb.InsertLocalPlaylistItemParams{
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
	return pm.store.queries().TouchLocalPlaylist(ctx, desktopdb.TouchLocalPlaylistParams{
		UpdatedAt: now,
		ID:        playlistID,
	})
}

// UploadPlaylistToServer uploads a local playlist to the connected server.
// Only metadata references for local_ref items. Local file paths are not sent.
// Returns the remote playlist ID.
func (pm *PlaylistManager) UploadPlaylistToServer(playlistID string) (string, error) {
	if pm.client == nil {
		return "", fmt.Errorf("server client not initialized")
	}
	serverURL, token := pm.client.Credentials()
	if serverURL == "" || token == "" {
		return "", fmt.Errorf("not connected to server")
	}

	if pm.store == nil {
		return "", fmt.Errorf("db not initialised")
	}
	ctx := context.Background()

	lp, err := pm.store.queries().GetLocalPlaylistByID(ctx, playlistID)
	if err != nil {
		return "", fmt.Errorf("get local playlist: %w", err)
	}

	items, err := pm.store.queries().ListLocalPlaylistItems(ctx, playlistID)
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
		remoteID, err = pm.client.CreatePlaylist(serverURL, token, lp.Name, lp.Description, serverItems)
		if err != nil {
			return "", err
		}

		// Link the remote playlist ID locally so it can be retrieved later.
		now := dbconv.FormatTime(time.Now())
		_ = pm.store.queries().UpdateLocalPlaylist(ctx, desktopdb.UpdateLocalPlaylistParams{
			Name:             lp.Name,
			Description:      lp.Description,
			ArtworkPath:      lp.ArtworkPath,
			RemotePlaylistID: remoteID,
			UpdatedAt:        now,
			ID:               playlistID,
		})
	} else {
		err = pm.client.UpdatePlaylistItems(serverURL, token, remoteID, serverItems)
		if err != nil {
			return "", err
		}
	}

	return remoteID, nil
}

// PickPlaylistArtwork opens a native file dialog for selecting an image,
// resizes it to a thumbnail, stores it in the thumb cache, updates the
// playlist's artwork_path in the DB, and returns the stored filename.
func (pm *PlaylistManager) PickPlaylistArtwork(playlistID string) (string, error) {
	if pm.store == nil {
		return "", fmt.Errorf("db not initialised")
	}

	path, err := wailsrt.OpenFileDialog(pm.ctx, wailsrt.OpenDialogOptions{
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

	if pm.streamer == nil {
		return "", fmt.Errorf("local streamer not initialized")
	}

	if err := artwork.WriteThumbnail(pm.streamer.ThumbDir(), fileName, thumbData); err != nil {
		return "", err
	}

	now := dbconv.FormatTime(time.Now())
	if err := pm.store.queries().UpdateLocalPlaylistArtwork(context.Background(), desktopdb.UpdateLocalPlaylistArtworkParams{
		ArtworkPath: fileName,
		UpdatedAt:   now,
		ID:          playlistID,
	}); err != nil {
		return "", fmt.Errorf("update playlist artwork: %w", err)
	}

	go pm.uploadPlaylistArtToServer(playlistID, thumbData)

	return fileName, nil
}

// uploadPlaylistArtToServer uploads playlist artwork to the server.
// Called in a goroutine after local artwork is picked.
func (pm *PlaylistManager) uploadPlaylistArtToServer(playlistID string, jpgData []byte) {
	if pm.client == nil {
		return
	}
	serverURL, token := pm.client.Credentials()
	if serverURL == "" || token == "" {
		return
	}

	// skip any playlists that aren't synced to the server
	ctx := context.Background()
	lp, err := pm.store.queries().GetLocalPlaylistByID(ctx, playlistID)
	if err != nil || lp.RemotePlaylistID == "" {
		return
	}

	if err := pm.client.UploadPlaylistArt(serverURL, token, lp.RemotePlaylistID, jpgData); err != nil {
		slog.Warn("playlist art upload failed", "err", err)
	}
}

// RefreshPlaylistArtFromServer downloads the server's artwork for a playlist
// that has a remote_playlist_id, stores it locally, and updates the DB.
// Called when a playlist.updated WS event arrives from the server.
func (pm *PlaylistManager) RefreshPlaylistArtFromServer(playlistID string) error {
	if pm.client == nil {
		return fmt.Errorf("server client not initialized")
	}
	serverURL, token := pm.client.Credentials()
	if serverURL == "" || token == "" {
		return fmt.Errorf("not connected to server")
	}

	ctx := context.Background()
	lp, err := pm.store.queries().GetLocalPlaylistByID(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("get local playlist: %w", err)
	}

	// playlist isn't synced to the server, so there's no artwork to refresh
	if lp.RemotePlaylistID == "" {
		return nil
	}

	raw, err := pm.client.FetchPlaylistArt(serverURL, token, lp.RemotePlaylistID)
	if err != nil {
		return fmt.Errorf("fetch artwork: %w", err)
	}

	thumbData, err := artwork.ResizeToThumbnail(raw, thumbMaxDim)
	if err != nil {
		return fmt.Errorf("resize artwork: %w", err)
	}

	sum := sha256.Sum256(thumbData)
	hashPrefix := hex.EncodeToString(sum[:])[:24]

	fileName := "pl-" + hashPrefix + ".jpg"
	if pm.streamer == nil {
		return fmt.Errorf("local streamer not initialized")
	}

	if err := artwork.WriteThumbnail(pm.streamer.ThumbDir(), fileName, thumbData); err != nil {
		return fmt.Errorf("write artwork: %w", err)
	}

	now := dbconv.FormatTime(time.Now())
	if err := pm.store.queries().UpdateLocalPlaylistArtwork(ctx, desktopdb.UpdateLocalPlaylistArtworkParams{
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
func (pm *PlaylistManager) RefreshPlaylistArtByRemoteID(remotePlaylistID string) error {
	if remotePlaylistID == "" {
		return nil
	}

	ctx := context.Background()
	lp, err := pm.store.queries().GetLocalPlaylistByRemoteID(ctx, remotePlaylistID)
	if err != nil {
		return nil
	}

	return pm.RefreshPlaylistArtFromServer(lp.ID)
}

// ResolvePlaylistItems enriches local_ref items with their local file paths
// by fuzzy-matching on title, album, album_artist, and duration.
func (pm *PlaylistManager) ResolvePlaylistItems(playlistID string) ([]LocalPlaylistItem, error) {
	if pm.store == nil {
		return nil, fmt.Errorf("db not initialised")
	}

	ctx := context.Background()
	rows, err := pm.store.queries().ListLocalPlaylistItems(ctx, playlistID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}

	items := make([]LocalPlaylistItem, len(rows))
	for i, r := range rows {
		items[i] = LocalPlaylistItem{
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

	allTracks, err := pm.store.queries().ListAllLocalTracks(ctx)
	if err != nil {
		return items, nil
	}

	return resolvePlaylistItems(items, allTracks), nil
}

// GenerateRandomPlaylist creates a new playlist filled with randomly selected
// tracks targeting the given duration in minutes. Local tracks are always
// included. If useRemote is true and the app is connected to a server, remote
// tracks are added to the pool as well, producing a mix of both sources.
func (pm *PlaylistManager) GenerateRandomPlaylist(name, description string, durationMinutes int, useRemote bool) (*LocalPlaylistSummary, error) {
	if pm.store == nil {
		return nil, fmt.Errorf("db not initialised")
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("playlist name is required")
	}
	if durationMinutes <= 0 {
		return nil, fmt.Errorf("duration must be at least 1 minute")
	}

	targetMS := int64(durationMinutes) * 60 * 1000

	candidates, err := pm.randomPlaylistCandidates(useRemote)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no track candidates available")
	}

	deduped := dedupeRandomTracks(candidates)
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

	pl, err := pm.CreateLocalPlaylist(name, description)
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

	if err := pm.SetLocalPlaylistItems(pl.ID, items); err != nil {
		_ = pm.DeleteLocalPlaylist(pl.ID)
		return nil, fmt.Errorf("set items: %w", err)
	}

	return pl, nil
}

// resolvePlaylistItems matches local_ref items to actual local files
// by metadata (title + album + album_artist + duration tolerance).
func resolvePlaylistItems(items []LocalPlaylistItem, allTracks []desktopdb.LocalTrack) []LocalPlaylistItem {
	type trackRef struct {
		path       string
		durationMS int64
	}

	lookup := make(map[string][]trackRef, len(allTracks))
	for _, t := range allTracks {
		key := strings.ToLower(t.Title) + "|" + strings.ToLower(t.Album) + "|" + strings.ToLower(t.AlbumArtist)
		lookup[key] = append(lookup[key], trackRef{path: t.Path, durationMS: t.DurationMs})
	}

	const durationToleranceMS = 3000

	for i := range items {
		switch {
		case items[i].Source == string(models.SourceLocalRef) && items[i].LocalPath == "":
			key := strings.ToLower(items[i].RefTitle) + "|" + strings.ToLower(items[i].RefAlbum) + "|" + strings.ToLower(items[i].RefAlbumArtist)
			matched := false
			for _, c := range lookup[key] {
				durationDiff := items[i].RefDurationMS - c.durationMS
				if durationDiff < 0 {
					durationDiff = -durationDiff
				}
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
		case items[i].Source == string(models.SourceLocalRef) && items[i].LocalPath != "":
			items[i].Resolved = true
		case items[i].Source == string(models.SourceRemote):
			items[i].Resolved = true
		}
	}
	return items
}

// randomPlaylistCandidates loads the normalized candidates used by the random
// playlist picker. Remote tracks are optional and any remote fetch failure is
// treated as a best-effort miss so local playlists still work.
func (pm *PlaylistManager) randomPlaylistCandidates(useRemote bool) ([]randomTrack, error) {
	rows, err := pm.store.queries().ListAllLocalTracks(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list local tracks: %w", err)
	}

	localTracks := make([]randomTrack, 0, len(rows))
	for _, t := range rows {
		if t.DurationMs <= 0 {
			continue
		}
		localTracks = append(localTracks, randomTrack{
			source:      "local_ref",
			localPath:   t.Path,
			title:       t.Title,
			album:       t.Album,
			albumArtist: t.AlbumArtist,
			durationMS:  t.DurationMs,
		})
	}

	if !useRemote || pm.client == nil {
		return localTracks, nil
	}

	serverURL, token := pm.client.Credentials()
	if serverURL == "" || token == "" {
		return localTracks, nil
	}

	remoteTracks, err := pm.remoteRandomPlaylistCandidates(serverURL, token)
	if err != nil {
		return localTracks, nil
	}

	return append(localTracks, remoteTracks...), nil
}

// remoteRandomPlaylistCandidates normalizes remote tracks into the same shape as local tracks.
func (pm *PlaylistManager) remoteRandomPlaylistCandidates(serverURL, token string) ([]randomTrack, error) {
	const pageSize = 200

	tracks, err := pm.client.FetchAllRemoteTracks(serverURL, token, pageSize)
	if err != nil {
		return nil, err
	}

	candidates := make([]randomTrack, 0, len(tracks))
	for _, t := range tracks {
		if t.DurationMS <= 0 {
			continue
		}
		candidates = append(candidates, randomTrack{
			source:      "remote",
			trackID:     t.ID,
			title:       t.Title,
			album:       t.AlbumName,
			albumArtist: t.AlbumArtist,
			durationMS:  t.DurationMS,
		})
	}

	return candidates, nil
}
