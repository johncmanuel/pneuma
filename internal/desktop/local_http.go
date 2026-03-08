package desktop

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"
	"golang.org/x/image/draw"
)

// handleLocalStream serves a local audio file for the <audio> element.
func (a *App) handleLocalStream(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(filepath.Ext(path))
	if !audioExts[ext] {
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

	// Set MIME type.
	switch ext {
	case ".mp3":
		w.Header().Set("Content-Type", "audio/mpeg")
	case ".flac":
		w.Header().Set("Content-Type", "audio/flac")
	case ".ogg":
		w.Header().Set("Content-Type", "audio/ogg")
	case ".opus":
		w.Header().Set("Content-Type", "audio/opus")
	case ".m4a", ".aac":
		w.Header().Set("Content-Type", "audio/mp4")
	case ".wav":
		w.Header().Set("Content-Type", "audio/wav")
	case ".aiff":
		w.Header().Set("Content-Type", "audio/aiff")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}

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
	if !audioExts[ext] {
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

	// Fast path: file identity → artwork hash already known.
	if v, ok := artworkHashCache.Load(fileKey); ok {
		artHash := v.(string)
		thumbPath := filepath.Join(a.thumbDir, artHash+".jpg")
		if _, err := os.Stat(thumbPath); err == nil {
			http.ServeFile(w, r, thumbPath)
			return
		}
		// Thumb was deleted from disk — fall through to regenerate.
	}

	// Open file and extract embedded art.
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

	if _, err := os.Stat(thumbPath); err == nil {
		// Another track already cached this exact artwork — serve it directly.
		http.ServeFile(w, r, thumbPath)
		return
	}

	// Decode, resize, and persist.
	src, _, err := image.Decode(bytes.NewReader(artData))
	if err != nil {
		http.Error(w, "failed to decode artwork", http.StatusInternalServerError)
		return
	}

	b := src.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	dstW, dstH := srcW, srcH
	if srcW > thumbMaxDim || srcH > thumbMaxDim {
		if srcW >= srcH {
			dstW = thumbMaxDim
			dstH = srcH * thumbMaxDim / srcW
		} else {
			dstH = thumbMaxDim
			dstW = srcW * thumbMaxDim / srcH
		}
	}
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}

	dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)

	// Write to a temp file then atomically rename to avoid serving partial files.
	tmp, err := os.CreateTemp(a.thumbDir, "tmp-*.jpg")
	if err != nil {
		http.Error(w, "cache write failed", http.StatusInternalServerError)
		return
	}
	tmpName := tmp.Name()

	if err := jpeg.Encode(tmp, dst, &jpeg.Options{Quality: 82}); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		http.Error(w, "encode failed", http.StatusInternalServerError)
		return
	}
	tmp.Close()

	if err := os.Rename(tmpName, thumbPath); err != nil {
		os.Remove(tmpName)
		http.Error(w, "cache write failed", http.StatusInternalServerError)
		return
	}

	http.ServeFile(w, r, thumbPath)
}
