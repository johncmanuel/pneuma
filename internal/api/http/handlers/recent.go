package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"pneuma/internal/store/sqlite/dbconv"
	"pneuma/internal/store/sqlite/serverdb"
)

// RecentHandler handles /api/recent/* routes.
type RecentHandler struct {
	q *serverdb.Queries
}

func NewRecentHandler(q *serverdb.Queries) *RecentHandler {
	return &RecentHandler{q: q}
}

// GetRecent GET /api/recent
func (h *RecentHandler) GetRecent(c echo.Context) error {
	userID := claimsUserID(c)
	ctx := c.Request().Context()

	albumRows, err := h.q.ListRecentAlbumsByUser(ctx, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	playlistRows, err := h.q.ListRecentPlaylistsByUser(ctx, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	type albumJSON struct {
		AlbumName    string `json:"album_name"`
		AlbumArtist  string `json:"album_artist"`
		FirstTrackID string `json:"first_track_id"`
		PlayedAt     string `json:"played_at"`
	}

	type playlistJSON struct {
		PlaylistID  string `json:"playlist_id"`
		Name        string `json:"name"`
		ArtworkPath string `json:"artwork_path,omitempty"`
		PlayedAt    string `json:"played_at"`
	}

	albums := make([]albumJSON, len(albumRows))
	for i, r := range albumRows {
		albums[i] = albumJSON{
			AlbumName:    r.AlbumName,
			AlbumArtist:  r.AlbumArtist,
			FirstTrackID: r.FirstTrackID,
			PlayedAt:     r.PlayedAt,
		}
	}

	playlists := make([]playlistJSON, len(playlistRows))
	for i, r := range playlistRows {
		playlists[i] = playlistJSON{
			PlaylistID:  r.PlaylistID,
			Name:        r.Name,
			ArtworkPath: r.ArtworkPath,
			PlayedAt:    r.PlayedAt,
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"albums":    albums,
		"playlists": playlists,
	})
}

// RecordAlbum POST /api/recent/albums
func (h *RecentHandler) RecordAlbum(c echo.Context) error {
	userID := claimsUserID(c)

	var body struct {
		AlbumName    string `json:"album_name"`
		AlbumArtist  string `json:"album_artist"`
		FirstTrackID string `json:"first_track_id"`
	}

	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}

	if body.AlbumName == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "album_name is required")
	}

	if err := h.q.UpsertRecentAlbum(c.Request().Context(), serverdb.UpsertRecentAlbumParams{
		UserID:       userID,
		AlbumName:    body.AlbumName,
		AlbumArtist:  body.AlbumArtist,
		FirstTrackID: body.FirstTrackID,
		PlayedAt:     dbconv.FormatTime(time.Now()),
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// RecordPlaylist POST /api/recent/playlists
func (h *RecentHandler) RecordPlaylist(c echo.Context) error {
	userID := claimsUserID(c)

	var body struct {
		PlaylistID string `json:"playlist_id"`
	}

	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}

	if body.PlaylistID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "playlist_id is required")
	}

	if err := h.q.UpsertRecentPlaylist(c.Request().Context(), serverdb.UpsertRecentPlaylistParams{
		UserID:     userID,
		PlaylistID: body.PlaylistID,
		PlayedAt:   dbconv.FormatTime(time.Now()),
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// DeleteRecentPlaylist DELETE /api/recent/playlists/:id
func (h *RecentHandler) DeleteRecentPlaylist(c echo.Context) error {
	userID := claimsUserID(c)
	playlistID := c.Param("id")

	if err := h.q.DeleteRecentPlaylist(c.Request().Context(), serverdb.DeleteRecentPlaylistParams{
		UserID:     userID,
		PlaylistID: playlistID,
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}
