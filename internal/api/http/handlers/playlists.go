package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"pneuma/internal/api/http/middleware"
	"pneuma/internal/artwork"
	"pneuma/internal/config"
	"pneuma/internal/models"
	"pneuma/internal/playlist"
)

// PlaylistHandler handles /api/playlists/* routes.
type PlaylistHandler struct {
	svc        *playlist.Service
	hub        eventPublisher
	artworkDir string
}

type playlistResponse struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description,omitempty"`
	ArtworkPath      string    `json:"artwork_path,omitempty"`
	RemotePlaylistID string    `json:"remote_playlist_id,omitempty"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
	ItemCount        int       `json:"item_count,omitempty"`
	DurationMS       int64     `json:"total_duration_ms,omitempty"`
	LegacyDurationMS int64     `json:"duration_ms,omitempty"`
	LegacyTotalDurMS int64     `json:"total_dur_ms,omitempty"`
	TrackCount       int       `json:"track_count,omitempty"`
}

type playlistItemResponse struct {
	Position       int    `json:"position"`
	Source         string `json:"source"`
	TrackID        string `json:"track_id,omitempty"`
	RefTitle       string `json:"ref_title,omitempty"`
	RefAlbum       string `json:"ref_album,omitempty"`
	RefAlbumArtist string `json:"ref_album_artist,omitempty"`
	RefDurationMS  int64  `json:"ref_duration_ms,omitempty"`
	AddedAt        string `json:"added_at,omitempty"`
	Missing        bool   `json:"missing"`
}

type playlistDeltaPayload struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description,omitempty"`
	ArtworkPath      string    `json:"artwork_path,omitempty"`
	RemotePlaylistID string    `json:"remote_playlist_id,omitempty"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
	ItemCount        int       `json:"item_count"`
	DurationMS       int64     `json:"total_duration_ms"`
	LegacyDurationMS int64     `json:"duration_ms"`
	LegacyTotalDurMS int64     `json:"total_dur_ms"`
	TrackCount       int       `json:"track_count"`
	ItemsChanged     bool      `json:"items_changed"`
	MetadataChanged  bool      `json:"metadata_changed"`
	Deleted          bool      `json:"deleted,omitempty"`
}

func toPlaylistResponse(pl *models.Playlist) *playlistResponse {
	if pl == nil {
		return nil
	}

	return &playlistResponse{
		ID:               pl.ID,
		Name:             pl.Name,
		Description:      pl.Description,
		ArtworkPath:      pl.ArtworkPath,
		RemotePlaylistID: pl.RemotePlaylistID,
		CreatedAt:        pl.CreatedAt,
		UpdatedAt:        pl.UpdatedAt,
		ItemCount:        pl.ItemCount,
		DurationMS:       pl.DurationMS,
		LegacyDurationMS: pl.DurationMS,
		LegacyTotalDurMS: pl.DurationMS,
		TrackCount:       pl.TrackCount,
	}
}

func toPlaylistResponses(playlists []*models.Playlist) []*playlistResponse {
	out := make([]*playlistResponse, 0, len(playlists))
	for _, playlist := range playlists {
		if playlist == nil {
			continue
		}
		out = append(out, toPlaylistResponse(playlist))
	}
	return out
}

func toPlaylistItemResponses(items []models.PlaylistItem) []playlistItemResponse {
	out := make([]playlistItemResponse, 0, len(items))
	for _, item := range items {
		out = append(out, playlistItemResponse{
			Position:       item.Position,
			Source:         string(item.Source),
			TrackID:        item.TrackID,
			RefTitle:       item.RefTitle,
			RefAlbum:       item.RefAlbum,
			RefAlbumArtist: item.RefAlbumArtist,
			RefDurationMS:  item.RefDurationMS,
			AddedAt:        item.AddedAt.Format(time.RFC3339),
			Missing:        item.Missing,
		})
	}
	return out
}

// toPlaylistDeltaPayload creates a playlistDeltaPayload from a
// Playlist model and change flags.
func toPlaylistDeltaPayload(pl *models.Playlist, itemsChanged, metadataChanged bool) *playlistDeltaPayload {
	if pl == nil {
		return nil
	}

	durationMS := pl.DurationMS

	return &playlistDeltaPayload{
		ID:               pl.ID,
		Name:             pl.Name,
		Description:      pl.Description,
		ArtworkPath:      pl.ArtworkPath,
		RemotePlaylistID: pl.RemotePlaylistID,
		CreatedAt:        pl.CreatedAt,
		UpdatedAt:        pl.UpdatedAt,
		ItemCount:        pl.ItemCount,
		DurationMS:       durationMS,
		LegacyDurationMS: durationMS,
		LegacyTotalDurMS: durationMS,
		TrackCount:       pl.ItemCount,
		ItemsChanged:     itemsChanged,
		MetadataChanged:  metadataChanged,
	}
}

func (h *PlaylistHandler) playlistDeltaPayload(ctx context.Context, playlistID string, itemsChanged, metadataChanged bool) *playlistDeltaPayload {
	pl, err := h.svc.GetByID(ctx, playlistID)
	if err != nil || pl == nil {
		return nil
	}

	count, durationMS, err := h.svc.PlaylistStats(ctx, playlistID)
	if err != nil {
		return toPlaylistDeltaPayload(pl, itemsChanged, metadataChanged)
	}

	pl.ItemCount = count
	pl.DurationMS = durationMS

	return toPlaylistDeltaPayload(pl, itemsChanged, metadataChanged)
}

// NewPlaylistHandler creates a PlaylistHandler.
func NewPlaylistHandler(svc *playlist.Service, hub eventPublisher, artworkDir string) *PlaylistHandler {
	return &PlaylistHandler{svc: svc, hub: hub, artworkDir: artworkDir}
}

// getPlaylistIfOwner retrieves a playlist by ID and verifies the authenticated user owns it.
func (h *PlaylistHandler) getPlaylistIfOwner(c echo.Context, id string) (*models.Playlist, error) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "missing token")
	}

	pl, err := h.svc.GetByID(c.Request().Context(), id)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	if pl.UserID != claims.UserID {
		return nil, echo.NewHTTPError(http.StatusForbidden, "access denied")
	}

	return pl, nil
}

// ListPlaylists lists all playlists for the authenticated user.
func (h *PlaylistHandler) ListPlaylists(c echo.Context) error {
	claims := middleware.GetClaims(c)

	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing token")
	}

	playlists, err := h.svc.ListByUser(c.Request().Context(), claims.UserID)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, toPlaylistResponses(playlists))
}

// GetPlaylist gets a playlist by ID
func (h *PlaylistHandler) GetPlaylist(c echo.Context) error {
	id := c.Param("id")

	pl, err := h.getPlaylistIfOwner(c, id)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, toPlaylistResponse(pl))
}

// CreatePlaylist creates a new playlist.
func (h *PlaylistHandler) CreatePlaylist(c echo.Context) error {
	claims := middleware.GetClaims(c)

	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing token")
	}

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}

	if body.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name is required")
	}

	pl, err := h.svc.Create(c.Request().Context(), claims.UserID, body.Name, body.Description)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	delta := h.playlistDeltaPayload(c.Request().Context(), pl.ID, false, true)
	if delta == nil {
		delta = toPlaylistDeltaPayload(pl, false, true)
	}

	h.hub.PublishToUser(claims.UserID, string(models.EventPlaylistCreated), delta)
	return c.JSON(http.StatusCreated, toPlaylistResponse(pl))
}

// UpdatePlaylist updates a playlist by ID.
func (h *PlaylistHandler) UpdatePlaylist(c echo.Context) error {
	id := c.Param("id")
	pl, err := h.getPlaylistIfOwner(c, id)
	if err != nil {
		return err
	}

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ArtworkPath string `json:"artwork_path"`
	}

	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}

	if err := h.svc.Update(c.Request().Context(), id, body.Name, body.Description, body.ArtworkPath); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	updated := h.playlistDeltaPayload(c.Request().Context(), id, false, true)
	if updated != nil {
		h.hub.PublishToUser(pl.UserID, string(models.EventPlaylistUpdated), updated)
	}

	return c.NoContent(http.StatusNoContent)
}

// DeletePlaylist deletes a playlist by ID.
func (h *PlaylistHandler) DeletePlaylist(c echo.Context) error {
	id := c.Param("id")
	pl, err := h.getPlaylistIfOwner(c, id)
	if err != nil {
		return err
	}

	if err := h.svc.Delete(c.Request().Context(), id); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	h.hub.PublishToUser(pl.UserID, string(models.EventPlaylistDeleted), &playlistDeltaPayload{ID: id, Deleted: true})
	return c.NoContent(http.StatusNoContent)
}

// GetPlaylistItems gets all items in a playlist by ID.
func (h *PlaylistHandler) GetPlaylistItems(c echo.Context) error {
	id := c.Param("id")
	if _, err := h.getPlaylistIfOwner(c, id); err != nil {
		return err
	}

	items, err := h.svc.GetItems(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if strings.EqualFold(strings.TrimSpace(c.QueryParam("view")), "full") {
		return c.JSON(http.StatusOK, items)
	}

	return c.JSON(http.StatusOK, toPlaylistItemResponses(items))
}

// SetPlaylistItems replaces all items in a playlist with the given items.
func (h *PlaylistHandler) SetPlaylistItems(c echo.Context) error {
	id := c.Param("id")
	pl, err := h.getPlaylistIfOwner(c, id)
	if err != nil {
		return err
	}

	var items []models.PlaylistItem

	if err := c.Bind(&items); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}

	if err := h.svc.SetItems(c.Request().Context(), id, items); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	updated := h.playlistDeltaPayload(c.Request().Context(), id, true, false)
	if updated != nil {
		h.hub.PublishToUser(pl.UserID, string(models.EventPlaylistUpdated), updated)
	}

	return c.NoContent(http.StatusNoContent)
}

// AddPlaylistItem adds an item to a playlist by ID.
func (h *PlaylistHandler) AddPlaylistItem(c echo.Context) error {
	id := c.Param("id")
	pl, err := h.getPlaylistIfOwner(c, id)
	if err != nil {
		return err
	}

	var item models.PlaylistItem

	if err := c.Bind(&item); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}

	if err := h.svc.AddItem(c.Request().Context(), id, item); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	updated := h.playlistDeltaPayload(c.Request().Context(), id, true, false)
	if updated != nil {
		h.hub.PublishToUser(pl.UserID, string(models.EventPlaylistUpdated), updated)
	}

	return c.NoContent(http.StatusNoContent)
}

// RemovePlaylistItem removes an item from a playlist by position and keeps order contiguous.
func (h *PlaylistHandler) RemovePlaylistItem(c echo.Context) error {
	id := c.Param("id")
	pl, err := h.getPlaylistIfOwner(c, id)
	if err != nil {
		return err
	}

	pos, err := strconv.Atoi(c.Param("position"))
	if err != nil || pos < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid position")
	}

	items, err := h.svc.GetItems(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if pos >= len(items) {
		return c.NoContent(http.StatusNoContent)
	}

	next := make([]models.PlaylistItem, 0, len(items)-1)
	for i, item := range items {
		if i == pos {
			continue
		}
		item.Position = len(next)
		next = append(next, item)
	}

	if err := h.svc.SetItems(c.Request().Context(), id, next); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	updated := h.playlistDeltaPayload(c.Request().Context(), id, true, false)
	if updated != nil {
		h.hub.PublishToUser(pl.UserID, string(models.EventPlaylistUpdated), updated)
	}

	return c.NoContent(http.StatusNoContent)
}

// AppendPlaylistItems appends one or more items to the end of a playlist.
func (h *PlaylistHandler) AppendPlaylistItems(c echo.Context) error {
	id := c.Param("id")
	pl, err := h.getPlaylistIfOwner(c, id)
	if err != nil {
		return err
	}

	var body struct {
		Items []models.PlaylistItem `json:"items"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}

	if len(body.Items) == 0 {
		return c.NoContent(http.StatusNoContent)
	}

	existing, err := h.svc.GetItems(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	next := make([]models.PlaylistItem, 0, len(existing)+len(body.Items))
	next = append(next, existing...)
	for _, item := range body.Items {
		item.Position = len(next)
		next = append(next, item)
	}

	if err := h.svc.SetItems(c.Request().Context(), id, next); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	updated := h.playlistDeltaPayload(c.Request().Context(), id, true, false)
	if updated != nil {
		h.hub.PublishToUser(pl.UserID, string(models.EventPlaylistUpdated), updated)
	}

	return c.NoContent(http.StatusNoContent)
}

// UploadPlaylistArt handles multipart artwork upload for a playlist.
func (h *PlaylistHandler) UploadPlaylistArt(c echo.Context) error {
	id := c.Param("id")
	pl, err := h.getPlaylistIfOwner(c, id)
	if err != nil {
		return err
	}

	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "file field required")
	}

	if file.Size > config.PlaylistMaxArtSizeBytes {
		msg := fmt.Sprintf("file too large (max %d MB)", config.PlaylistMaxArtSizeBytes>>20)
		return echo.NewHTTPError(http.StatusBadRequest, msg)
	}

	src, err := file.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer src.Close()

	slog.Info("processing playlist artwork upload", "playlist_id", pl.ID, "file_name", file.Filename, "file_size", file.Size)

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, src); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	raw := buf.Bytes()

	thumbData, err := artwork.ResizeToThumbnail(raw, config.PlaylistMaxArtDim)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid image: "+err.Error())
	}

	if err := os.MkdirAll(h.artworkDir, 0o755); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	fileName := pl.ID + ".jpg"
	if err := artwork.WriteThumbnail(h.artworkDir, fileName, thumbData); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	ctx := c.Request().Context()
	if err := h.svc.Update(ctx, pl.ID, pl.Name, pl.Description, fileName); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	updated := h.playlistDeltaPayload(ctx, pl.ID, false, true)
	if updated != nil {
		h.hub.PublishToUser(pl.UserID, string(models.EventPlaylistUpdated), updated)
	}

	return c.JSON(http.StatusOK, map[string]string{"artwork_path": fileName})
}

// ServePlaylistArt serves a playlist's artwork image.
func (h *PlaylistHandler) ServePlaylistArt(c echo.Context) error {
	id := c.Param("id")

	pl, err := h.svc.GetByID(c.Request().Context(), id)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}

	if pl.ArtworkPath == "" {
		return c.NoContent(http.StatusNotFound)
	}

	// Path traversal protection
	cleanName := filepath.Base(pl.ArtworkPath)
	artPath := filepath.Join(h.artworkDir, cleanName)

	if _, err := os.Stat(artPath); os.IsNotExist(err) {
		return c.NoContent(http.StatusNotFound)
	}

	return c.File(artPath)
}

// GenerateRandom creates a new playlist filled with randomly selected tracks.
func (h *PlaylistHandler) GenerateRandom(c echo.Context) error {
	claims := middleware.GetClaims(c)
	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing token")
	}

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Duration    int    `json:"duration"`
	}

	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}

	if body.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name is required")
	}
	if body.Duration < 1 {
		return echo.NewHTTPError(http.StatusBadRequest, "duration must be at least 1 minute")
	}

	pl, err := h.svc.GenerateRandom(c.Request().Context(), claims.UserID, body.Name, body.Description, body.Duration)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	delta := h.playlistDeltaPayload(c.Request().Context(), pl.ID, true, true)
	if delta == nil {
		delta = toPlaylistDeltaPayload(pl, true, true)
	}

	h.hub.PublishToUser(claims.UserID, string(models.EventPlaylistCreated), delta)
	return c.JSON(http.StatusCreated, toPlaylistResponse(pl))
}
