package handlers

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"pneuma/internal/api/http/middleware"
	"pneuma/internal/models"
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

type playbackTrackItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	AlbumArtist string `json:"album_artist"`
	AlbumName   string `json:"album_name"`
	DurationMS  int64  `json:"duration_ms"`
}

func compactPlaybackTrack(track *models.Track) *playbackTrackItem {
	if track == nil {
		return nil
	}

	return &playbackTrackItem{
		ID:          track.ID,
		Title:       track.Title,
		AlbumArtist: track.AlbumArtist,
		AlbumName:   track.AlbumName,
		DurationMS:  track.DurationMS,
	}
}

func NewPlaybackHandler(engine *playback.Engine) *PlaybackHandler {
	return &PlaybackHandler{engine: engine}
}

// GetState GET /api/playback
func (h *PlaybackHandler) GetState(c echo.Context) error {
	deviceID := c.Request().Header.Get("X-Device-ID")
	s, err := h.engine.GetState(claimsUserID(c), deviceID)

	if err != nil {
		if errors.Is(err, playback.ErrNoActiveSession) {
			return c.JSON(http.StatusOK, map[string]any{
				"playing":     false,
				"track_id":    "",
				"position_ms": 0,
				"queue":       []string{},
				"queue_index": 0,
				"repeat":      playback.RepeatOff,
				"shuffle":     false,
			})
		}
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"playing":     s.Playing,
		"track_id":    s.TrackID,
		"track":       compactPlaybackTrack(s.Track),
		"position_ms": s.PositionMS,
		"queue":       s.Queue,
		"queue_index": s.QueueIndex,
		"repeat":      s.Repeat,
		"shuffle":     s.Shuffle,
	})
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

	deviceID := c.Request().Header.Get("X-Device-ID")
	return h.engine.Play(c.Request().Context(), claimsUserID(c), deviceID, body.TrackID, body.PositionMS)
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

	deviceID := c.Request().Header.Get("X-Device-ID")
	return h.engine.Pause(c.Request().Context(), claimsUserID(c), deviceID, body.Paused, body.PositionMS)
}

// Seek POST /api/playback/seek  body: {position_ms}
func (h *PlaybackHandler) Seek(c echo.Context) error {
	var body struct {
		PositionMS int64 `json:"position_ms"`
	}

	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	deviceID := c.Request().Header.Get("X-Device-ID")
	return h.engine.Seek(c.Request().Context(), claimsUserID(c), deviceID, body.PositionMS)
}

// Next POST /api/playback/next
func (h *PlaybackHandler) Next(c echo.Context) error {
	deviceID := c.Request().Header.Get("X-Device-ID")
	nextID, queueIdx, err := h.engine.Next(c.Request().Context(), claimsUserID(c), deviceID)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"track_id": nextID, "queue_index": queueIdx})
}

// Prev POST /api/playback/prev
func (h *PlaybackHandler) Prev(c echo.Context) error {
	deviceID := c.Request().Header.Get("X-Device-ID")
	prevID, queueIdx, err := h.engine.Prev(c.Request().Context(), claimsUserID(c), deviceID)

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

	deviceID := c.Request().Header.Get("X-Device-ID")
	return h.engine.SetQueue(c.Request().Context(), claimsUserID(c), deviceID, body.TrackIDs, body.StartIndex)
}

// SetRepeat POST /api/playback/repeat  body: {mode}
func (h *PlaybackHandler) SetRepeat(c echo.Context) error {
	var body struct {
		Mode playback.RepeatMode `json:"mode"`
	}

	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	deviceID := c.Request().Header.Get("X-Device-ID")
	return h.engine.SetRepeat(c.Request().Context(), claimsUserID(c), deviceID, body.Mode)
}

// SetShuffle POST /api/playback/shuffle  body: {enabled}
func (h *PlaybackHandler) SetShuffle(c echo.Context) error {
	var body struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	deviceID := c.Request().Header.Get("X-Device-ID")
	return h.engine.SetShuffle(c.Request().Context(), claimsUserID(c), deviceID, body.Enabled)
}
