package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	"pneuma/internal/artwork"
	"pneuma/internal/config"
	"pneuma/internal/ingestion"
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
	lib             *library.Service
	q               *serverdb.Queries
	scanner         scanTrigger
	hub             eventPublisher
	queue           *ingestion.Queue
	uploadsDir      string
	tmpDir          string // temp staging area under uploadsDir
	trackArtworkDir string
	transcoder      *media.StreamTranscoder
}

// NewLibraryHandler creates a LibraryHandler.
func NewLibraryHandler(lib *library.Service, q *serverdb.Queries, sc scanTrigger, hub eventPublisher, queue *ingestion.Queue, uploadsDir, trackArtworkDir string, transcoder *media.StreamTranscoder) *LibraryHandler {
	tmpDir := filepath.Join(uploadsDir, "tmp")
	_ = os.MkdirAll(tmpDir, 0o755)
	if strings.TrimSpace(trackArtworkDir) == "" {
		trackArtworkDir = filepath.Join(uploadsDir, "track-artwork")
	}
	_ = os.MkdirAll(trackArtworkDir, 0o755)

	return &LibraryHandler{
		lib:             lib,
		q:               q,
		scanner:         sc,
		hub:             hub,
		queue:           queue,
		uploadsDir:      uploadsDir,
		tmpDir:          tmpDir,
		trackArtworkDir: trackArtworkDir,
		transcoder:      transcoder,
	}
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

	quality := media.ParseStreamQuality(c.QueryParam("quality"))

	info, err := os.Stat(track.Path)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if h.transcoder != nil {
		if cachedPath, ok := h.transcoder.ResolveCachedPath(track, track.Path, info, quality); ok {
			cachedFile, err := os.Open(cachedPath)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}
			defer cachedFile.Close()

			cachedInfo, err := cachedFile.Stat()
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}

			c.Response().Header().Set("Content-Type", media.MimeFromExt(".ogg"))
			c.Response().Header().Set("Cache-Control", "private, no-store")
			c.Response().Header().Set("X-Pneuma-Stream-Profile", string(media.NormalizeStreamQuality(quality)))
			normalizeRangeHeader(c.Request(), cachedInfo.Size())
			http.ServeContent(c.Response(), c.Request(), cachedInfo.Name(), cachedInfo.ModTime(), cachedFile)
			return nil
		}

		h.transcoder.QueueTranscode(track, track.Path, info, quality)
	}

	f, err := os.Open(track.Path)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(track.Path))

	c.Response().Header().Set("Content-Type", media.MimeFromExt(ext))
	c.Response().Header().Set("Cache-Control", "private, no-store")
	c.Response().Header().Set("X-Pneuma-Stream-Profile", string(media.StreamQualityOriginal))
	normalizeRangeHeader(c.Request(), info.Size())
	http.ServeContent(c.Response(), c.Request(), info.Name(), info.ModTime(), f)

	return nil
}

// normalizeRangeHeader removes the Range header if the requested
// start byte is beyond the end of the file.
func normalizeRangeHeader(req *http.Request, fileSize int64) {
	start, ok := parseRangeStart(req.Header.Get("Range"))
	if !ok {
		return
	}

	if start >= fileSize {
		req.Header.Del("Range")
	}
}

// parseRangeStart extracts the starting byte offset from a Range header value.
func parseRangeStart(value string) (int64, bool) {
	rangeValue := strings.TrimSpace(value)
	if rangeValue == "" || !strings.HasPrefix(rangeValue, "bytes=") {
		return 0, false
	}

	spec := strings.TrimSpace(strings.TrimPrefix(rangeValue, "bytes="))
	if spec == "" || strings.Contains(spec, ",") {
		return 0, false
	}

	sep := strings.Index(spec, "-")
	if sep <= 0 {
		return 0, false
	}

	start, err := strconv.ParseInt(strings.TrimSpace(spec[:sep]), 10, 64)
	if err != nil || start < 0 {
		return 0, false
	}

	return start, true
}

// ServeTrackArt returns embedded album art from the audio file.
func (h *LibraryHandler) ServeTrackArt(c echo.Context) error {
	track, err := h.lib.TrackByID(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	if track == nil {
		return c.NoContent(http.StatusNotFound)
	}

	info, err := os.Stat(track.Path)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}

	hash := trackArtCacheKey(track.Path, info)
	cachePath := h.trackArtPath(hash)
	if _, err := os.Stat(cachePath); err == nil {
		return h.serveTrackArtFromPath(c, hash, cachePath)
	}

	f, err := os.Open(track.Path)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}

	pic := m.Picture()
	if pic == nil || len(pic.Data) == 0 {
		return c.NoContent(http.StatusNotFound)
	}

	thumbData, err := artwork.ResizeToThumbnail(pic.Data, config.PlaylistMaxArtDim)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}

	if err := os.MkdirAll(h.trackArtworkDir, 0o755); err == nil {
		if writeErr := artwork.WriteThumbnail(h.trackArtworkDir, filepath.Base(cachePath), thumbData); writeErr == nil {
			return h.serveTrackArtFromPath(c, hash, cachePath)
		}
	}

	etag := trackArtETag(hash)
	if c.Request().Header.Get("If-None-Match") == etag {
		return c.NoContent(http.StatusNotModified)
	}
	c.Response().Header().Set("ETag", etag)
	c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	return c.Blob(http.StatusOK, "image/jpeg", thumbData)
}

func trackArtETag(hash string) string {
	return `"` + hash + `"`
}

func trackArtCacheKey(path string, info os.FileInfo) string {
	basis := fmt.Sprintf("%s|%d|%d", path, info.Size(), info.ModTime().UnixNano())
	sum := sha256.Sum256([]byte(basis))
	return hex.EncodeToString(sum[:12])
}

func (h *LibraryHandler) trackArtPath(hash string) string {
	return filepath.Join(h.trackArtworkDir, "track-"+hash+".jpg")
}

func (h *LibraryHandler) serveTrackArtFromPath(c echo.Context, hash, path string) error {
	etag := trackArtETag(hash)
	if c.Request().Header.Get("If-None-Match") == etag {
		return c.NoContent(http.StatusNotModified)
	}

	c.Response().Header().Set("ETag", etag)
	c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	return c.File(path)
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
// The file is streamed to a temp file while being hashed (no full buffering).
// DB writes are delegated to the ingestion queue; the handler returns 202 Accepted.
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

	// create temp file for streaming hash
	tmp, err := os.CreateTemp(h.tmpDir, "upload-*"+ext)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	tmpPath := tmp.Name()

	hasher := sha256.New()
	if _, err = io.Copy(tmp, io.TeeReader(src, hasher)); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	tmp.Close()

	hash := hex.EncodeToString(hasher.Sum(nil))

	// handle dupes
	existing, err := h.lib.TrackByFingerprint(ctx, hash)
	if err != nil {
		os.Remove(tmpPath)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if existing != nil && existing.DeletedAt == nil {
		os.Remove(tmpPath)
		return c.JSON(http.StatusConflict, map[string]any{
			"error": "duplicate file",
			"track": existing,
		})
	}

	// if previously soft-deleted, restore it via the queue
	if existing != nil && existing.DeletedAt != nil {
		if err := h.lib.RestoreTrack(ctx, existing.ID); err != nil {
			os.Remove(tmpPath)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		// re-read tags from the temp file
		populateTrackFromTags(existing, tmpPath)
		existing.UpdatedAt = time.Now()

		finalPath := filepath.Join(h.uploadsDir, hash+ext)
		existing.Path = finalPath

		if err := h.queue.Enqueue(ingestion.Job{
			TmpPath:   tmpPath,
			FinalPath: finalPath,
			Track:     existing,
			UserID:    claims.UserID,
			Filename:  file.Filename,
		}); err != nil {
			os.Remove(tmpPath)
			return echo.NewHTTPError(http.StatusServiceUnavailable, "upload queue full")
		}
		return c.JSON(http.StatusAccepted, map[string]any{"status": "queued", "id": existing.ID})
	}

	// handle new uploads from temp file
	now := time.Now()
	info, _ := os.Stat(tmpPath)

	finalPath := filepath.Join(h.uploadsDir, hash+ext)

	t := &models.Track{
		ID:               uuid.NewString(),
		Path:             finalPath, // will be set to final by the queue worker
		Title:            strings.TrimSuffix(file.Filename, ext),
		AlbumArtist:      "__unorganized__",
		AlbumName:        "__unorganized__",
		Fingerprint:      hash,
		FileSizeBytes:    info.Size(),
		LastModified:     info.ModTime(),
		UploadedByUserID: claims.UserID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	populateTrackFromTags(t, tmpPath)

	if err := h.queue.Enqueue(ingestion.Job{
		TmpPath:   tmpPath,
		FinalPath: finalPath,
		Track:     t,
		UserID:    claims.UserID,
		Filename:  file.Filename,
	}); err != nil {
		os.Remove(tmpPath)
		return echo.NewHTTPError(http.StatusServiceUnavailable, "upload queue full")
	}

	return c.JSON(http.StatusAccepted, map[string]any{"status": "queued", "id": t.ID})
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

func populateTrackFromTags(t *models.Track, tmpPath string) {
	if f, openErr := os.Open(tmpPath); openErr == nil {
		defer f.Close()
		if m, tagErr := tag.ReadFrom(f); tagErr == nil {
			if m.Title() != "" {
				t.Title = m.Title()
			}
			t.AlbumArtist = m.AlbumArtist()
			if t.AlbumArtist == "" {
				t.AlbumArtist = m.Artist()
			}
			if t.AlbumArtist == "" {
				t.AlbumArtist = "__unorganized__"
			}
			t.AlbumName = m.Album()
			if t.AlbumName == "" {
				t.AlbumName = "__unorganized__"
			}
			t.Genre = m.Genre()
			t.Year = m.Year()
			t.TrackNumber, _ = m.Track()
			t.DiscNumber, _ = m.Disc()
		}
	}
}
