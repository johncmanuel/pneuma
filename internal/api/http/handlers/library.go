package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"pneuma/internal/api/http/middleware"
	"pneuma/internal/library"
	"pneuma/internal/media"
	"pneuma/internal/models"
	"pneuma/internal/store/sqlite/dbconv"
	"pneuma/internal/store/sqlite/serverdb"
)

// scanTrigger is satisfied by *scanner.Scheduler.
type scanTrigger interface {
	ScanAll()
	ScanPath(path string)
}

// eventPublisher is satisfied by *ws.Hub.
type eventPublisher interface {
	Publish(eventType string, payload any)
	PublishToUser(userID string, eventType string, payload any)
}

// LibraryHandler serves library-related API routes.
type LibraryHandler struct {
	lib        *library.Service
	q          *serverdb.Queries
	scanner    scanTrigger
	hub        eventPublisher
	uploadsDir string
}

// NewLibraryHandler creates a LibraryHandler.
func NewLibraryHandler(lib *library.Service, q *serverdb.Queries, sc scanTrigger, hub eventPublisher, uploadsDir string) *LibraryHandler {
	return &LibraryHandler{lib: lib, q: q, scanner: sc, hub: hub, uploadsDir: uploadsDir}
}

// ListTracks returns tracks. Supports optional pagination via ?offset=&limit=
// query params. Without them, returns all tracks (backwards-compatible).
func (h *LibraryHandler) ListTracks(c echo.Context) error {
	ctx := c.Request().Context()

	// Bulk fetch by IDs: GET /api/library/tracks?ids=id1,id2,...
	if idsParam := c.QueryParam("ids"); idsParam != "" {
		ids := strings.Split(idsParam, ",")
		tracks, err := h.lib.TracksByIDs(ctx, ids)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, tracks)
	}

	// Fetch tracks by album: GET /api/library/tracks?album_name=X&album_artist=Y
	if albumName := c.QueryParam("album_name"); albumName != "" || c.QueryParam("album_artist") != "" {
		albumArtist := c.QueryParam("album_artist")
		tracks, err := h.lib.TracksByAlbum(ctx, albumName, albumArtist)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, tracks)
	}

	offsetStr := c.QueryParam("offset")
	limitStr := c.QueryParam("limit")
	if offsetStr != "" || limitStr != "" {
		offset, _ := strconv.Atoi(offsetStr)
		limit, _ := strconv.Atoi(limitStr)
		if limit <= 0 {
			limit = 50
		}
		if limit > 200 {
			limit = 200
		}
		if offset < 0 {
			offset = 0
		}
		tracks, err := h.lib.AllTracksPage(ctx, offset, limit)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		total, err := h.lib.CountTracks(ctx)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, map[string]any{
			"tracks": tracks,
			"total":  total,
			"offset": offset,
			"limit":  limit,
		})
	}

	tracks, err := h.lib.AllTracks(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, tracks)
}

// GetTrack returns a single track by ID.
func (h *LibraryHandler) GetTrack(c echo.Context) error {
	track, err := h.lib.TrackByID(c.Request().Context(), c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if track == nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	return c.JSON(http.StatusOK, track)
}

// StreamTrack serves the audio file.
func (h *LibraryHandler) StreamTrack(c echo.Context) error {
	track, err := h.lib.TrackByID(c.Request().Context(), c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if track == nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}

	f, err := os.Open(track.Path)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	c.Response().Header().Set("Content-Type", mimeFromExt(track.Path))
	http.ServeContent(c.Response(), c.Request(), info.Name(), info.ModTime(), f)
	return nil
}

// ServeTrackArt returns embedded album art from the audio file.
func (h *LibraryHandler) ServeTrackArt(c echo.Context) error {
	track, err := h.lib.TrackByID(c.Request().Context(), c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if track == nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}

	f, err := os.Open(track.Path)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
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

	sum := sha256.Sum256(pic.Data)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	if c.Request().Header.Get("If-None-Match") == etag {
		return c.NoContent(http.StatusNotModified)
	}

	c.Response().Header().Set("ETag", etag)
	c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	return c.Blob(http.StatusOK, ct, pic.Data)
}

// UpdateTrackMeta applies a partial metadata update (PATCH).
func (h *LibraryHandler) UpdateTrackMeta(c echo.Context) error {
	ctx := c.Request().Context()
	track, err := h.lib.TrackByID(ctx, c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if track == nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}

	var patch struct {
		Title       *string `json:"title"`
		AlbumName   *string `json:"album_name"`
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
	if patch.AlbumName != nil {
		track.AlbumName = *patch.AlbumName
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
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, track)
}

// ListAlbumGroups returns album groups derived from the tracks table using
// GROUP BY album_name, album_artist. This is more reliable than /albums because
// it does not require the albums table to be populated. Supports "?offset=&limit=&filter=" params.
func (h *LibraryHandler) ListAlbumGroups(c echo.Context) error {
	ctx := c.Request().Context()
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	filter := c.QueryParam("filter")
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	groups, err := h.lib.AllTrackAlbumGroupsPage(ctx, filter, offset, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	total, err := h.lib.CountTrackAlbumGroups(ctx, filter)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if groups == nil {
		groups = []*models.TrackAlbumGroup{}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"groups": groups,
		"total":  total,
		"offset": offset,
		"limit":  limit,
	})
}

// Search performs a text search across tracks.
func (h *LibraryHandler) Search(c echo.Context) error {
	q := c.QueryParam("q")
	if q == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing q param")
	}
	tracks, err := h.lib.Search(c.Request().Context(), q)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, tracks)
}

// TriggerScan kicks off a full library rescan.
func (h *LibraryHandler) TriggerScan(c echo.Context) error {
	go h.scanner.ScanAll()
	return c.JSON(http.StatusAccepted, map[string]string{"status": "scan started"})
}

// UploadTrack uploads a track to the specified uploads directory.
// NOTE: this accepts a multipart audio file.
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

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !media.IsSupportedAudio(ext) {
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported audio format: "+ext)
	}

	src, err := file.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer src.Close()

	hasher := sha256.New()
	buf, err := io.ReadAll(src)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	hasher.Write(buf)
	hash := hex.EncodeToString(hasher.Sum(nil))

	existing, err := h.lib.TrackByFingerprint(ctx, hash)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if existing != nil && existing.DeletedAt == nil {
		return c.JSON(http.StatusConflict, map[string]any{
			"error": "duplicate file",
			"track": existing,
		})
	}

	if err := os.MkdirAll(h.uploadsDir, 0o755); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	destPath := filepath.Join(h.uploadsDir, hash+ext)
	if err := os.WriteFile(destPath, buf, 0o644); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// If previously soft-deleted, restore and re-enrich it.
	if existing != nil && existing.DeletedAt != nil {
		if err := h.lib.RestoreTrack(ctx, existing.ID); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		// Re-read tags in case the file has changed since the original upload.
		if m, tagErr := tag.ReadFrom(bytes.NewReader(buf)); tagErr == nil {
			if m.Title() != "" {
				existing.Title = m.Title()
			}
			existing.AlbumArtist = m.AlbumArtist()
			if existing.AlbumArtist == "" {
				existing.AlbumArtist = m.Artist()
			}
			existing.AlbumName = m.Album()
			existing.Genre = m.Genre()
			existing.Year = m.Year()
			existing.TrackNumber, _ = m.Track()
			existing.DiscNumber, _ = m.Disc()
			existing.UpdatedAt = time.Now()
			_ = h.lib.UpsertTrack(ctx, existing)
		}
		h.hub.Publish(string(models.EventTrackAdded), existing)
		return c.JSON(http.StatusOK, existing)
	}

	// Read embedded metadata from the uploaded file so the initial
	// record is fully populated
	now := time.Now()
	info, _ := os.Stat(destPath)

	title := strings.TrimSuffix(file.Filename, ext)
	var albumArtist, albumName, genre string
	var year, trackNumber, discNumber int
	if m, tagErr := tag.ReadFrom(bytes.NewReader(buf)); tagErr == nil {
		if m.Title() != "" {
			title = m.Title()
		}
		albumArtist = m.AlbumArtist()
		if albumArtist == "" {
			albumArtist = m.Artist()
		}
		albumName = m.Album()
		genre = m.Genre()
		year = m.Year()
		trackNumber, _ = m.Track()
		discNumber, _ = m.Disc()
	}

	t := &models.Track{
		ID:               uuid.NewString(),
		Path:             destPath,
		Title:            title,
		AlbumArtist:      albumArtist,
		AlbumName:        albumName,
		Genre:            genre,
		Year:             year,
		TrackNumber:      trackNumber,
		DiscNumber:       discNumber,
		Fingerprint:      hash,
		FileSizeBytes:    info.Size(),
		LastModified:     info.ModTime(),
		UploadedByUserID: claims.UserID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := h.lib.UpsertTrack(ctx, t); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	_ = h.q.InsertAuditEntry(ctx, serverdb.InsertAuditEntryParams{
		ID:         uuid.NewString(),
		UserID:     claims.UserID,
		Action:     "upload",
		TargetType: "track",
		TargetID:   t.ID,
		Detail:     dbconv.NullStr(file.Filename),
		CreatedAt:  dbconv.FormatTime(now),
	})

	h.hub.Publish(string(models.EventTrackAdded), t)

	// Enrich metadata asynchronously (duration, bitrate, tags) using the scanner.
	// The scanner will publish track.updated when done so all clients refresh.
	go h.scanner.ScanPath(destPath)

	return c.JSON(http.StatusCreated, t)
}

// DeleteTrack DELETE /api/library/tracks/:id.
// NOTE: this is a soft delete.
func (h *LibraryHandler) DeleteTrack(c echo.Context) error {
	claims := middleware.GetClaims(c)
	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing token")
	}
	ctx := c.Request().Context()

	track, err := h.lib.TrackByID(ctx, c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if track == nil || track.DeletedAt != nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}

	if err := h.lib.SoftDeleteTrack(ctx, track.ID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// If it was a user-uploaded file, remove from disk.
	if track.UploadedByUserID != "" {
		_ = os.Remove(track.Path)
	}

	_ = h.q.InsertAuditEntry(ctx, serverdb.InsertAuditEntryParams{
		ID:         uuid.NewString(),
		UserID:     claims.UserID,
		Action:     "delete",
		TargetType: "track",
		TargetID:   track.ID,
		Detail:     dbconv.NullStr(track.Title),
		CreatedAt:  dbconv.FormatTime(time.Now()),
	})

	h.hub.Publish(string(models.EventTrackRemoved), track)
	return c.NoContent(http.StatusNoContent)
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
