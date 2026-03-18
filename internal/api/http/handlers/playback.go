package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"pneuma/internal/api/http/middleware"
	"pneuma/internal/playback"
)

// claimsUserID extracts the user ID from the JWT claims attached to c, or ""
// if the request is unauthenticated.
func claimsUserID(c echo.Context) string {
	if claims := middleware.GetClaims(c); claims != nil {
		return claims.UserID
	}
	return ""
}

// PlaybackHandler handles /api/playback/* routes.
type PlaybackHandler struct {
	engine *playback.Engine
}

func NewPlaybackHandler(engine *playback.Engine) *PlaybackHandler {
	return &PlaybackHandler{engine: engine}
}

// GetState GET /api/playback
func (h *PlaybackHandler) GetState(c echo.Context) error {
	s, err := h.engine.GetState(claimsUserID(c))

	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	return c.JSON(http.StatusOK, s)
}

// Play POST /api/playback/play  body: {track_id, position_ms}
func (h *PlaybackHandler) Play(c echo.Context) error {
	var body struct {
		TrackID    string `json:"track_id"`
		PositionMS int64  `json:"position_ms"`
	}

	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return h.engine.Play(c.Request().Context(), claimsUserID(c), body.TrackID, body.PositionMS)
}

// Pause POST /api/playback/pause  body: {paused, position_ms?}
func (h *PlaybackHandler) Pause(c echo.Context) error {
	var body struct {
		Paused     bool  `json:"paused"`
		PositionMS int64 `json:"position_ms"`
	}

	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return h.engine.Pause(c.Request().Context(), claimsUserID(c), body.Paused, body.PositionMS)
}

// Seek POST /api/playback/seek  body: {position_ms}
func (h *PlaybackHandler) Seek(c echo.Context) error {
	var body struct {
		PositionMS int64 `json:"position_ms"`
	}

	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return h.engine.Seek(c.Request().Context(), claimsUserID(c), body.PositionMS)
}

// Next POST /api/playback/next
func (h *PlaybackHandler) Next(c echo.Context) error {
	nextID, queueIdx, err := h.engine.Next(c.Request().Context(), claimsUserID(c))

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"track_id": nextID, "queue_index": queueIdx})
}

// Prev POST /api/playback/prev
func (h *PlaybackHandler) Prev(c echo.Context) error {
	prevID, queueIdx, err := h.engine.Prev(c.Request().Context(), claimsUserID(c))

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"track_id": prevID, "queue_index": queueIdx})
}

// SetQueue POST /api/playback/queue  body: {track_ids, start_index}
func (h *PlaybackHandler) SetQueue(c echo.Context) error {
	var body struct {
		TrackIDs   []string `json:"track_ids"`
		StartIndex int      `json:"start_index"`
	}

	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return h.engine.SetQueue(c.Request().Context(), claimsUserID(c), body.TrackIDs, body.StartIndex)
}

// SetRepeat POST /api/playback/repeat  body: {mode}
func (h *PlaybackHandler) SetRepeat(c echo.Context) error {
	var body struct {
		Mode playback.RepeatMode `json:"mode"`
	}

	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return h.engine.SetRepeat(c.Request().Context(), claimsUserID(c), body.Mode)
}

// SetShuffle POST /api/playback/shuffle  body: {enabled}
func (h *PlaybackHandler) SetShuffle(c echo.Context) error {
	var body struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return h.engine.SetShuffle(c.Request().Context(), claimsUserID(c), body.Enabled)
}
