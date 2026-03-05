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

// PlaybackHandler handles /api/playback/* and /api/handoff routes.
type PlaybackHandler struct {
	engine  *playback.Engine
	handoff *playback.Handoff
}

func NewPlaybackHandler(engine *playback.Engine, handoff *playback.Handoff) *PlaybackHandler {
	return &PlaybackHandler{engine: engine, handoff: handoff}
}

// GetState GET /api/playback/:device_id
func (h *PlaybackHandler) GetState(c echo.Context) error {
	s, err := h.engine.GetState(c.Param("device_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	return c.JSON(http.StatusOK, s)
}

// Play POST /api/playback/:device_id/play  body: {track_id, position_ms}
func (h *PlaybackHandler) Play(c echo.Context) error {
	var body struct {
		TrackID    string `json:"track_id"`
		PositionMS int64  `json:"position_ms"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return h.engine.Play(c.Request().Context(), c.Param("device_id"), claimsUserID(c), body.TrackID, body.PositionMS)
}

// Pause POST /api/playback/:device_id/pause  body: {paused, position_ms?}
func (h *PlaybackHandler) Pause(c echo.Context) error {
	var body struct {
		Paused     bool  `json:"paused"`
		PositionMS int64 `json:"position_ms"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return h.engine.Pause(c.Request().Context(), c.Param("device_id"), claimsUserID(c), body.Paused, body.PositionMS)
}

// Seek POST /api/playback/:device_id/seek  body: {position_ms}
func (h *PlaybackHandler) Seek(c echo.Context) error {
	var body struct {
		PositionMS int64 `json:"position_ms"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return h.engine.Seek(c.Request().Context(), c.Param("device_id"), claimsUserID(c), body.PositionMS)
}

// Next POST /api/playback/:device_id/next
func (h *PlaybackHandler) Next(c echo.Context) error {
	nextID, queueIdx, err := h.engine.Next(c.Request().Context(), c.Param("device_id"), claimsUserID(c))
	if err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"track_id": nextID, "queue_index": queueIdx})
}

// Prev POST /api/playback/:device_id/prev
func (h *PlaybackHandler) Prev(c echo.Context) error {
	prevID, queueIdx, err := h.engine.Prev(c.Request().Context(), c.Param("device_id"), claimsUserID(c))
	if err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"track_id": prevID, "queue_index": queueIdx})
}

// SetQueue POST /api/playback/:device_id/queue  body: {track_ids, start_index}
func (h *PlaybackHandler) SetQueue(c echo.Context) error {
	var body struct {
		TrackIDs   []string `json:"track_ids"`
		StartIndex int      `json:"start_index"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return h.engine.SetQueue(c.Request().Context(), c.Param("device_id"), claimsUserID(c), body.TrackIDs, body.StartIndex)
}

// SetRepeat POST /api/playback/:device_id/repeat  body: {mode}
func (h *PlaybackHandler) SetRepeat(c echo.Context) error {
	var body struct {
		Mode playback.RepeatMode `json:"mode"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return h.engine.SetRepeat(c.Request().Context(), c.Param("device_id"), claimsUserID(c), body.Mode)
}

// SetShuffle POST /api/playback/:device_id/shuffle  body: {enabled}
func (h *PlaybackHandler) SetShuffle(c echo.Context) error {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return h.engine.SetShuffle(c.Request().Context(), c.Param("device_id"), claimsUserID(c), body.Enabled)
}

// Transfer POST /api/handoff  body: {user_id, source_device_id, target_device_id}
func (h *PlaybackHandler) Transfer(c echo.Context) error {
	var body struct {
		UserID         string `json:"user_id"`
		SourceDeviceID string `json:"source_device_id"`
		TargetDeviceID string `json:"target_device_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.handoff.Transfer(c.Request().Context(), body.UserID, body.SourceDeviceID, body.TargetDeviceID); err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "transferred"})
}

// Sessions GET /api/sessions/:user_id
func (h *PlaybackHandler) Sessions(c echo.Context) error {
	sessions, err := h.handoff.Sessions(c.Request().Context(), c.Param("user_id"))
	if err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, sessions)
}
