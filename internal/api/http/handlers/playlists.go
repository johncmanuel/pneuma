package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"pneuma/internal/api/http/middleware"
	"pneuma/internal/models"
	"pneuma/internal/playlist"
)

// PlaylistHandler handles /api/playlists/* routes.
type PlaylistHandler struct {
	svc *playlist.Service
	hub eventPublisher
}

// NewPlaylistHandler creates a PlaylistHandler.
func NewPlaylistHandler(svc *playlist.Service, hub eventPublisher) *PlaylistHandler {
	return &PlaylistHandler{svc: svc, hub: hub}
}

// ListPlaylists GET /api/playlists
func (h *PlaylistHandler) ListPlaylists(c echo.Context) error {
	claims := middleware.GetClaims(c)

	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing token")
	}

	playlists, err := h.svc.ListByUser(c.Request().Context(), claims.UserID)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, playlists)
}

// GetPlaylist GET /api/playlists/:id
func (h *PlaylistHandler) GetPlaylist(c echo.Context) error {
	id := c.Param("id")
	pl, err := h.svc.GetByID(c.Request().Context(), id)

	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	return c.JSON(http.StatusOK, pl)
}

// CreatePlaylist POST /api/playlists
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

	h.hub.Publish(string(models.EventPlaylistCreated), pl)
	return c.JSON(http.StatusCreated, pl)
}

// UpdatePlaylist PUT /api/playlists/:id
func (h *PlaylistHandler) UpdatePlaylist(c echo.Context) error {
	id := c.Param("id")

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

	h.hub.Publish(string(models.EventPlaylistUpdated), map[string]string{"id": id})
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// DeletePlaylist DELETE /api/playlists/:id
func (h *PlaylistHandler) DeletePlaylist(c echo.Context) error {
	id := c.Param("id")

	if err := h.svc.Delete(c.Request().Context(), id); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	h.hub.Publish(string(models.EventPlaylistDeleted), map[string]string{"id": id})
	return c.NoContent(http.StatusNoContent)
}

// GetPlaylistItems GET /api/playlists/:id/items
func (h *PlaylistHandler) GetPlaylistItems(c echo.Context) error {
	id := c.Param("id")

	items, err := h.svc.GetItems(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, items)
}

// SetPlaylistItems PUT /api/playlists/:id/items
func (h *PlaylistHandler) SetPlaylistItems(c echo.Context) error {
	id := c.Param("id")
	var items []models.PlaylistItem

	if err := c.Bind(&items); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}

	if err := h.svc.SetItems(c.Request().Context(), id, items); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	h.hub.Publish(string(models.EventPlaylistUpdated), map[string]string{"id": id})
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// AddPlaylistItem POST /api/playlists/:id/items
func (h *PlaylistHandler) AddPlaylistItem(c echo.Context) error {
	id := c.Param("id")
	var item models.PlaylistItem

	if err := c.Bind(&item); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}

	if err := h.svc.AddItem(c.Request().Context(), id, item); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	h.hub.Publish(string(models.EventPlaylistUpdated), map[string]string{"id": id})
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
