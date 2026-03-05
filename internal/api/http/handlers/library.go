package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"pneuma/internal/api/http/middleware"
	"pneuma/internal/library"
	"pneuma/internal/models"
	"pneuma/internal/store/sqlite"
)

// scanTrigger is satisfied by *scanner.Scheduler.
type scanTrigger interface {
	ScanAll()
}

// eventPublisher is satisfied by *ws.Hub.
type eventPublisher interface {
	Publish(eventType string, payload any)
}

// LibraryHandler serves library-related API routes.
type LibraryHandler struct {
	lib        *library.Service
	store      *sqlite.Store
	scanner    scanTrigger
	hub        eventPublisher
	uploadsDir string
}

// NewLibraryHandler creates a LibraryHandler.
func NewLibraryHandler(lib *library.Service, store *sqlite.Store, sc scanTrigger, hub eventPublisher, uploadsDir string) *LibraryHandler {
	return &LibraryHandler{lib: lib, store: store, scanner: sc, hub: hub, uploadsDir: uploadsDir}
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

// UploadTrack POST /api/library/tracks/upload — accepts a multipart audio file.
func (h *LibraryHandler) UploadTrack(c echo.Context) error {
	claims := middleware.GetClaims(c)
	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing token")
	}
	ctx := c.Request().Context()

	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "file field required")
	}

	// Validate extension.
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !isAudioExt(ext) {
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported audio format: "+ext)
	}

	src, err := file.Open()
	if err != nil {
		return internalErr(err)
	}
	defer src.Close()

	// Hash the file contents for dedup.
	hasher := sha256.New()
	buf, err := io.ReadAll(src)
	if err != nil {
		return internalErr(err)
	}
	hasher.Write(buf)
	hash := hex.EncodeToString(hasher.Sum(nil))

	// Check if a track with this fingerprint already exists.
	existing, err := h.lib.TrackByFingerprint(ctx, hash)
	if err != nil {
		return internalErr(err)
	}
	if existing != nil && existing.DeletedAt == nil {
		return c.JSON(http.StatusConflict, map[string]any{
			"error": "duplicate file",
			"track": existing,
		})
	}

	// Write file to uploads dir.
	if err := os.MkdirAll(h.uploadsDir, 0o755); err != nil {
		return internalErr(err)
	}
	destPath := filepath.Join(h.uploadsDir, hash+ext)
	if err := os.WriteFile(destPath, buf, 0o644); err != nil {
		return internalErr(err)
	}

	// If previously soft-deleted, restore it.
	if existing != nil && existing.DeletedAt != nil {
		if err := h.lib.RestoreTrack(ctx, existing.ID); err != nil {
			return internalErr(err)
		}
		h.hub.Publish(string(models.EventTrackAdded), existing)
		return c.JSON(http.StatusOK, existing)
	}

	// Build the track record from the filename metadata for now.
	// The scanner will pick it up and enrich it later, but we create
	// a basic record immediately so the upload response is useful.
	now := time.Now()
	info, _ := os.Stat(destPath)
	t := &models.Track{
		ID:               uuid.NewString(),
		Path:             destPath,
		Title:            strings.TrimSuffix(file.Filename, ext),
		Fingerprint:      hash,
		FileSizeBytes:    info.Size(),
		LastModified:     info.ModTime(),
		UploadedByUserID: claims.UserID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := h.lib.UpsertTrack(ctx, t); err != nil {
		return internalErr(err)
	}

	// Audit log.
	_ = h.store.InsertAuditEntry(ctx, &models.AuditEntry{
		ID:         uuid.NewString(),
		UserID:     claims.UserID,
		Action:     "upload",
		TargetType: "track",
		TargetID:   t.ID,
		Detail:     file.Filename,
		CreatedAt:  now,
	})

	h.hub.Publish(string(models.EventTrackAdded), t)
	return c.JSON(http.StatusCreated, t)
}

// DeleteTrack DELETE /api/library/tracks/:id — soft-deletes a track.
func (h *LibraryHandler) DeleteTrack(c echo.Context) error {
	claims := middleware.GetClaims(c)
	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing token")
	}
	ctx := c.Request().Context()

	track, err := h.lib.TrackByID(ctx, c.Param("id"))
	if err != nil {
		return internalErr(err)
	}
	if track == nil || track.DeletedAt != nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}

	if err := h.lib.SoftDeleteTrack(ctx, track.ID); err != nil {
		return internalErr(err)
	}

	// If it was a user-uploaded file, remove from disk.
	if track.UploadedByUserID != "" {
		_ = os.Remove(track.Path)
	}

	// Audit log.
	_ = h.store.InsertAuditEntry(ctx, &models.AuditEntry{
		ID:         uuid.NewString(),
		UserID:     claims.UserID,
		Action:     "delete",
		TargetType: "track",
		TargetID:   track.ID,
		Detail:     track.Title,
		CreatedAt:  time.Now(),
	})

	h.hub.Publish(string(models.EventTrackRemoved), track)
	return c.NoContent(http.StatusNoContent)
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

var allowedAudioExts = map[string]bool{
	".mp3": true, ".flac": true, ".ogg": true, ".opus": true,
	".m4a": true, ".aac": true, ".wav": true, ".aiff": true,
	".wma": true, ".alac": true, ".ape": true, ".wv": true,
}

func isAudioExt(ext string) bool {
	return allowedAudioExts[ext]
}
