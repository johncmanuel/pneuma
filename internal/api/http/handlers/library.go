package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/labstack/echo/v4"

	"pneuma/internal/library"
)

// scanTrigger is satisfied by *scanner.Scheduler.
type scanTrigger interface {
	ScanAll()
}

// LibraryHandler serves library-related API routes.
type LibraryHandler struct {
	lib     *library.Service
	scanner scanTrigger
}

// NewLibraryHandler creates a LibraryHandler.
func NewLibraryHandler(lib *library.Service, sc scanTrigger) *LibraryHandler {
	return &LibraryHandler{lib: lib, scanner: sc}
}

// ListTracks returns all tracks.
func (h *LibraryHandler) ListTracks(c echo.Context) error {
	tracks, err := h.lib.AllTracks(c.Request().Context())
	if err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, tracks)
}

// GetTrack returns a single track by ID.
func (h *LibraryHandler) GetTrack(c echo.Context) error {
	track, err := h.lib.TrackByID(c.Request().Context(), c.Param("id"))
	if err != nil {
		return internalErr(err)
	}
	if track == nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	return c.JSON(http.StatusOK, track)
}

// StreamTrack serves the audio file with Range header support.
func (h *LibraryHandler) StreamTrack(c echo.Context) error {
	track, err := h.lib.TrackByID(c.Request().Context(), c.Param("id"))
	if err != nil {
		return internalErr(err)
	}
	if track == nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}

	f, err := os.Open(track.Path)
	if err != nil {
		return internalErr(err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return internalErr(err)
	}

	c.Response().Header().Set("Content-Type", mimeFromExt(track.Path))
	http.ServeContent(c.Response(), c.Request(), info.Name(), info.ModTime(), f)
	return nil
}

// ServeTrackArt returns embedded album art from the audio file.
func (h *LibraryHandler) ServeTrackArt(c echo.Context) error {
	track, err := h.lib.TrackByID(c.Request().Context(), c.Param("id"))
	if err != nil {
		return internalErr(err)
	}
	if track == nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}

	f, err := os.Open(track.Path)
	if err != nil {
		return internalErr(err)
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "no tags")
	}

	pic := m.Picture()
	if pic == nil || len(pic.Data) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "no embedded art")
	}

	ct := pic.MIMEType
	if ct == "" {
		ct = "image/jpeg"
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=604800")
	return c.Blob(http.StatusOK, ct, pic.Data)
}

// UpdateTrackMeta applies a partial metadata update (PATCH).
func (h *LibraryHandler) UpdateTrackMeta(c echo.Context) error {
	ctx := c.Request().Context()
	track, err := h.lib.TrackByID(ctx, c.Param("id"))
	if err != nil {
		return internalErr(err)
	}
	if track == nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}

	var patch struct {
		Title       *string `json:"title"`
		Artist      *string `json:"artist"`
		Album       *string `json:"album"`
		AlbumArtist *string `json:"album_artist"`
		Genre       *string `json:"genre"`
		Year        *int    `json:"year"`
		TrackNumber *int    `json:"track_number"`
		DiscNumber  *int    `json:"disc_number"`
	}
	if err := c.Bind(&patch); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if patch.Title != nil {
		track.Title = *patch.Title
	}
	if patch.AlbumArtist != nil {
		track.AlbumArtist = *patch.AlbumArtist
	}
	if patch.Genre != nil {
		track.Genre = *patch.Genre
	}
	if patch.Year != nil {
		track.Year = *patch.Year
	}
	if patch.TrackNumber != nil {
		track.TrackNumber = *patch.TrackNumber
	}
	if patch.DiscNumber != nil {
		track.DiscNumber = *patch.DiscNumber
	}
	track.UpdatedAt = time.Now()

	if err := h.lib.UpsertTrack(ctx, track); err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, track)
}

// ListAlbums returns all albums.
func (h *LibraryHandler) ListAlbums(c echo.Context) error {
	albums, err := h.lib.AllAlbums(c.Request().Context())
	if err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, albums)
}

// Search performs a text search across tracks.
func (h *LibraryHandler) Search(c echo.Context) error {
	q := c.QueryParam("q")
	if q == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing q param")
	}
	tracks, err := h.lib.Search(c.Request().Context(), q)
	if err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, tracks)
}

// TriggerScan kicks off a full library rescan.
func (h *LibraryHandler) TriggerScan(c echo.Context) error {
	go h.scanner.ScanAll()
	return c.JSON(http.StatusAccepted, map[string]string{"status": "scan started"})
}

// ─── shared helpers (also used by handlers/playback.go) ──────────────────────

func internalErr(err error) *echo.HTTPError {
	return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
}

func mimeFromExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		return "audio/mpeg"
	case ".flac":
		return "audio/flac"
	case ".ogg":
		return "audio/ogg"
	case ".opus":
		return "audio/opus"
	case ".m4a", ".aac":
		return "audio/mp4"
	case ".wav":
		return "audio/wav"
	case ".aiff":
		return "audio/aiff"
	default:
		return "application/octet-stream"
	}
}
