package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"

	"pneuma/internal/artwork"
	"pneuma/internal/media"
)

// handleLocalStream serves a local audio file for the <audio> element.
func (a *App) handleLocalStream(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(filepath.Ext(path))
	if !media.IsSupportedAudio(ext) {
		http.Error(w, "not an audio file", http.StatusBadRequest)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		http.Error(w, "stat failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", media.MimeFromExt(ext))

	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// handleLocalArt serves a resized thumbnail of the embedded album art.
// Thumbnails are stored content-addressed (keyed by SHA-256 of the raw art
// bytes), so every track that shares the same cover image reuses one file on
// disk instead of creating duplicates.
func (a *App) handleLocalArt(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(filepath.Ext(path))
	if !media.IsSupportedAudio(ext) {
		http.Error(w, "not an audio file", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	// fileKey changes whenever the audio file is modified, invalidating the
	// cached artwork hash so we re-read the file on the next request.
	fileKey := thumbCacheKey(path, info)

	// if the artwork hash is already known, serve the thumbnail.
	if v, ok := artworkHashCache.Load(fileKey); ok {
		artHash := v.(string)
		thumbPath := filepath.Join(a.thumbDir, artHash+".jpg")
		if _, err := os.Stat(thumbPath); err == nil {
			http.ServeFile(w, r, thumbPath)
			return
		}
	}

	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil || m.Picture() == nil {
		http.Error(w, "no artwork", http.StatusNotFound)
		return
	}

	artData := m.Picture().Data

	// Content-addressed key: SHA-256 of the raw artwork bytes.
	// Tracks sharing identical embedded art resolve to the same cache file.
	sum := sha256.Sum256(artData)
	artHash := hex.EncodeToString(sum[:])[:24]
	artworkHashCache.Store(fileKey, artHash)

	thumbPath := filepath.Join(a.thumbDir, artHash+".jpg")

	// serve artwork for that particular track if it is already cached
	if _, err := os.Stat(thumbPath); err == nil {
		http.ServeFile(w, r, thumbPath)
		return
	}

	// then resize and persist the thumbnail.
	thumbData, err := artwork.ResizeToThumbnail(artData, thumbMaxDim)
	if err != nil {
		http.Error(w, "failed to process artwork", http.StatusInternalServerError)
		return
	}

	if err := artwork.WriteThumbnail(a.thumbDir, artHash+".jpg", thumbData); err != nil {
		http.Error(w, "cache write failed", http.StatusInternalServerError)
		return
	}

	http.ServeFile(w, r, thumbPath)
}

// handlePlaylistArt serves a playlist's custom artwork thumbnail.
// The file query parameter is the basename stored in artwork_path (e.g. "pl-abc123.jpg").
func (a *App) handlePlaylistArt(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Query().Get("file")
	if file == "" {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}

	// only allow basenames and prevent path traversal.
	file = filepath.Base(file)
	artPath := filepath.Join(a.thumbDir, file)

	if _, err := os.Stat(artPath); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, artPath)
}
