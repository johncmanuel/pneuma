package desktop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dhowden/tag"

	"pneuma/internal/artwork"
	"pneuma/internal/media"
)

// LocalStreamer serves audio files and resized artwork thumbnails to the frontend.
type LocalStreamer struct {
	server           *http.Server
	thumbDir         string
	artworkHashCache sync.Map
	port             int
}

// NewLocalStreamer creates a new LocalStreamer.
func NewLocalStreamer(thumbDir string) *LocalStreamer {
	return &LocalStreamer{
		thumbDir: thumbDir,
	}
}

// Start binds the streamer to a random port and starts the background HTTP server.
func (s *LocalStreamer) Start() (int, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/local/stream", s.handleLocalStream)
	mux.HandleFunc("/local/art", s.handleLocalArt)
	mux.HandleFunc("/local/playlist-art", s.handlePlaylistArt)

	s.server = &http.Server{Handler: mux}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}

	s.port = l.Addr().(*net.TCPAddr).Port

	go s.server.Serve(l)

	return s.port, nil
}

// Stop shuts down the streamer gracefully.
func (s *LocalStreamer) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// Port returns the port the streamer is listening on.
func (s *LocalStreamer) Port() int {
	return s.port
}

// ThumbDir returns the directory where artwork thumbnails are cached.
func (s *LocalStreamer) ThumbDir() string {
	return s.thumbDir
}

// ClearArtworkCache removes all cached thumbnail files and resets the hash cache.
func (s *LocalStreamer) ClearArtworkCache() error {
	entries, err := os.ReadDir(s.thumbDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, e := range entries {
		_ = os.Remove(filepath.Join(s.thumbDir, e.Name()))
	}

	s.artworkHashCache.Range(func(k, _ any) bool {
		s.artworkHashCache.Delete(k)
		return true
	})

	return nil
}

// setLocalCORSHeaders sets permissive CORS headers.
func setLocalCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range")
}

// handleLocalOptions handles preflight CORS requests.
func handleLocalOptions(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodOptions {
		return false
	}

	setLocalCORSHeaders(w)
	w.WriteHeader(http.StatusNoContent)
	return true
}

// handleLocalStream serves a local audio file for the <audio> element.
func (s *LocalStreamer) handleLocalStream(w http.ResponseWriter, r *http.Request) {
	if handleLocalOptions(w, r) {
		return
	}

	setLocalCORSHeaders(w)

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

func (s *LocalStreamer) handleLocalArt(w http.ResponseWriter, r *http.Request) {
	if handleLocalOptions(w, r) {
		return
	}

	setLocalCORSHeaders(w)

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
	if v, ok := s.artworkHashCache.Load(fileKey); ok {
		artHash := v.(string)
		thumbPath := filepath.Join(s.thumbDir, artHash+".jpg")
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

	sum := sha256.Sum256(artData)
	artHash := hex.EncodeToString(sum[:])[:24] // use first 24 chars for filename
	s.artworkHashCache.Store(fileKey, artHash)

	thumbPath := filepath.Join(s.thumbDir, artHash+".jpg")

	// serve artwork for that particular track if it is already cached
	if _, err := os.Stat(thumbPath); err == nil {
		http.ServeFile(w, r, thumbPath)
		return
	}

	// resize and persist the thumbnail
	thumbData, err := artwork.ResizeToThumbnail(artData, thumbMaxDim)
	if err != nil {
		http.Error(w, "failed to process artwork", http.StatusInternalServerError)
		return
	}

	if err := artwork.WriteThumbnail(s.thumbDir, artHash+".jpg", thumbData); err != nil {
		http.Error(w, "cache write failed", http.StatusInternalServerError)
		return
	}

	http.ServeFile(w, r, thumbPath)
}

// handlePlaylistArt serves a playlist's custom artwork thumbnail.
// The file query parameter is the basename stored in artwork_path (e.g. "pl-abc123.jpg").
func (s *LocalStreamer) handlePlaylistArt(w http.ResponseWriter, r *http.Request) {
	if handleLocalOptions(w, r) {
		return
	}

	setLocalCORSHeaders(w)

	file := r.URL.Query().Get("file")
	if file == "" {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}

	// only allow basenames and prevent path traversal.
	file = filepath.Base(file)
	artPath := filepath.Join(s.thumbDir, file)

	if _, err := os.Stat(artPath); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, artPath)
}
