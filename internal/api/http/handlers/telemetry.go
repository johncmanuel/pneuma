package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"pneuma/internal/api/http/middleware"
	"pneuma/internal/models"
	"pneuma/internal/store/sqlite/dbconv"
	"pneuma/internal/store/sqlite/serverdb"
)

// TelemetryHandler handles /api/telemetry/* and /api/admin/telemetry/* routes.
type TelemetryHandler struct {
	q *serverdb.Queries
}

// NewTelemetryHandler creates a new TelemetryHandler.
func NewTelemetryHandler(q *serverdb.Queries) *TelemetryHandler {
	return &TelemetryHandler{q: q}
}

// SubmitStreamTelemetry ingests a single stream-quality measurement from a client.
func (h *TelemetryHandler) SubmitStreamTelemetry(c echo.Context) error {
	claims := middleware.GetClaims(c)
	ctx := c.Request().Context()

	var body struct {
		TrackID           string `json:"track_id"`
		StreamQuality     string `json:"stream_quality"`
		LatencyMs         int64  `json:"latency_ms"`
		StutterCount      int64  `json:"stutter_count"`
		StutterDurationMs int64  `json:"stutter_duration_ms"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if body.TrackID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "track_id is required")
	}

	if body.StreamQuality == "" {
		body.StreamQuality = "original"
	}

	userID := ""
	if claims != nil {
		userID = claims.UserID
	}

	deviceID := c.Request().Header.Get("X-Device-ID")

	if err := h.q.InsertStreamTelemetry(ctx, serverdb.InsertStreamTelemetryParams{
		ID:                uuid.NewString(),
		TrackID:           body.TrackID,
		UserID:            userID,
		DeviceID:          deviceID,
		StreamQuality:     body.StreamQuality,
		LatencyMs:         body.LatencyMs,
		StutterCount:      body.StutterCount,
		StutterDurationMs: body.StutterDurationMs,
		CreatedAt:         dbconv.FormatTime(time.Now()),
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

// GetStreamTelemetryStats returns aggregated stream telemetry statistics.
func (h *TelemetryHandler) GetStreamTelemetryStats(c echo.Context) error {
	ctx := c.Request().Context()

	rows, err := h.q.GetStreamTelemetryStats(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	stats := dbconv.TelemetryStatsToModels(rows)
	if stats == nil {
		stats = []models.StreamTelemetryStat{}
	}

	return c.JSON(http.StatusOK, stats)
}

// ClearStreamTelemetry deletes all stream telemetry data.
func (h *TelemetryHandler) ClearStreamTelemetry(c echo.Context) error {
	ctx := c.Request().Context()

	if err := h.q.ClearStreamTelemetry(ctx); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "cleared"})
}
