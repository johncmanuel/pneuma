package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dhowden/tag"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ffprobePath is resolved once at init; empty means unavailable.
var ffprobePath string

// fpcalcPath is resolved once at init; empty means unavailable.
var fpcalcPath string

// durationCache avoids re-running ffprobe for files that haven't changed.
// Key: "path|size|mtime_unix"  Value: duration in milliseconds.
var durationCache sync.Map

// fingerprintCache avoids re-hashing unchanged files.
// Key: "path|size|mtime_unix"  Value: [2]string{sha256Hex, acousticFP}
var fingerprintCache sync.Map

func init() {
	if p, err := exec.LookPath("ffprobe"); err == nil {
		ffprobePath = p
	}
	if p, err := exec.LookPath("fpcalc"); err == nil {
		fpcalcPath = p
	}
}

// audioExts is the set of file extensions the local file dialog will accept.
var audioExts = map[string]bool{
	".mp3": true, ".flac": true, ".ogg": true, ".opus": true,
	".m4a": true, ".aac": true, ".wav": true, ".aiff": true,
	".wma": true, ".alac": true, ".ape": true, ".wv": true,
}

// App is the Wails application struct. It acts as a thin client —
// local file playback is always available; server connectivity is optional.
type App struct {
	ctx context.Context

	// Local stream server (serves local audio files to the <audio> element).
	localPort int
	localSrv  *http.Server

	// Optional server connection state (guarded by mu).
	mu          sync.RWMutex
	serverURL   string
	token       string
	stopRefresh context.CancelFunc
}

// NewApp creates a new App.
func NewApp() *App {
	return &App{}
}

// startup is called by Wails when the application starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Start a local-only HTTP server on a random port for streaming local files.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		slog.Error("local stream listener failed", "err", err)
		return
	}
	a.localPort = listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/local/stream", a.handleLocalStream)
	mux.HandleFunc("/local/art", a.handleLocalArt)
	a.localSrv = &http.Server{Handler: mux}
	go func() {
		if err := a.localSrv.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Error("local stream server error", "err", err)
		}
	}()

	slog.Info("pneuma desktop started", "local_stream_port", a.localPort)
}

// shutdown is called by Wails when the application is closing.
func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	if a.stopRefresh != nil {
		a.stopRefresh()
	}
	a.mu.Unlock()
	if a.localSrv != nil {
		a.localSrv.Close() //nolint:errcheck
	}
}

// ─── Wails-bound methods (callable from Svelte) ──────────────────────────────

// GetLocalPort returns the local stream server port.
func (a *App) GetLocalPort() int {
	return a.localPort
}

// OpenLocalFiles opens a native file dialog for selecting audio files.
func (a *App) OpenLocalFiles() ([]string, error) {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Open Audio Files",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Audio Files", Pattern: "*.mp3;*.flac;*.ogg;*.opus;*.m4a;*.aac;*.wav;*.aiff"},
		},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	return []string{path}, nil
}

// OpenLocalFolder opens a native directory dialog and returns all audio files found.
func (a *App) OpenLocalFolder() ([]string, error) {
	dir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Open Music Folder",
	})
	if err != nil {
		return nil, err
	}
	if dir == "" {
		return nil, nil
	}

	var files []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if audioExts[ext] {
			files = append(files, path)
		}
		return nil
	})
	return files, nil
}

// ─── Local Library Scanning ──────────────────────────────────────────────────

// LocalTrack holds metadata read from a local audio file via embedded tags.
type LocalTrack struct {
	Path                string `json:"path"`
	Title               string `json:"title"`
	Artist              string `json:"artist"`
	Album               string `json:"album"`
	AlbumArtist         string `json:"album_artist"`
	Genre               string `json:"genre"`
	Year                int    `json:"year"`
	TrackNumber         int    `json:"track_number"`
	DiscNumber          int    `json:"disc_number"`
	DurationMs          int64  `json:"duration_ms"` // 0 if unavailable from tags
	HasArtwork          bool   `json:"has_artwork"`
	Fingerprint         string `json:"fingerprint"`          // SHA-256 content hash ("sha256:<hex>")
	AcousticFingerprint string `json:"acoustic_fingerprint"` // Chromaprint fpcalc output (empty if unavailable)
}

// ScanLocalFolder recursively scans a directory for audio files and reads
// their embedded tags. Returns a list of LocalTrack entries.
func (a *App) ScanLocalFolder(dir string) ([]LocalTrack, error) {
	var tracks []LocalTrack
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !audioExts[ext] {
			return nil
		}

		lt := LocalTrack{Path: path, Title: filepath.Base(path)}

		f, err := os.Open(path)
		if err != nil {
			tracks = append(tracks, lt)
			return nil
		}
		defer f.Close()

		m, err := tag.ReadFrom(f)
		if err != nil {
			tracks = append(tracks, lt)
			return nil
		}

		if m.Title() != "" {
			lt.Title = m.Title()
		}
		lt.Artist = m.Artist()
		lt.Album = m.Album()
		lt.AlbumArtist = m.AlbumArtist()
		lt.Genre = m.Genre()
		lt.Year = m.Year()
		tn, _ := m.Track()
		lt.TrackNumber = tn
		dn, _ := m.Disc()
		lt.DiscNumber = dn
		lt.HasArtwork = m.Picture() != nil

		// Read duration via ffprobe (best-effort), then pure-Go fallback.
		probeLocalDuration(path, info, &lt)
		if lt.DurationMs == 0 {
			parseDurationFallbackLocal(path, info, &lt)
		}

		// Compute content hash + acoustic fingerprint for duplicate detection.
		computeLocalFingerprints(path, info, &lt)

		tracks = append(tracks, lt)
		return nil
	})
	return tracks, err
}

// durationCacheKey builds a cache key from the file's path, size, and mtime.
func durationCacheKey(path string, fi os.FileInfo) string {
	return path + "|" + strconv.FormatInt(fi.Size(), 10) + "|" + strconv.FormatInt(fi.ModTime().Unix(), 10)
}

// probeLocalDuration shells out to ffprobe to read duration for a local track.
// Results are cached by path+size+mtime so subsequent scans skip the exec.
func probeLocalDuration(path string, fi os.FileInfo, lt *LocalTrack) {
	key := durationCacheKey(path, fi)
	if v, ok := durationCache.Load(key); ok {
		lt.DurationMs = v.(int64)
		return
	}
	if ffprobePath == "" {
		return
	}
	cmd := exec.Command(ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return
	}
	var result struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return
	}
	if dur, err := strconv.ParseFloat(result.Format.Duration, 64); err == nil {
		lt.DurationMs = int64(dur * 1000)
		durationCache.Store(key, lt.DurationMs)
	}
}

// parseDurationFallbackLocal reads duration using pure Go for supported formats.
func parseDurationFallbackLocal(path string, fi os.FileInfo, lt *LocalTrack) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".flac":
		parseFLACDurationLocal(path, fi, lt)
	}
}

// parseFLACDurationLocal reads the FLAC STREAMINFO block to compute duration.
func parseFLACDurationLocal(path string, fi os.FileInfo, lt *LocalTrack) {
	// Check cache first.
	key := durationCacheKey(path, fi)
	if v, ok := durationCache.Load(key); ok {
		lt.DurationMs = v.(int64)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return
	}
	if string(magic) != "fLaC" {
		return
	}

	for {
		hdr := make([]byte, 4)
		if _, err := io.ReadFull(f, hdr); err != nil {
			return
		}
		isLast := hdr[0]&0x80 != 0
		blockType := hdr[0] & 0x7F
		blockLen := int(binary.BigEndian.Uint32([]byte{0, hdr[1], hdr[2], hdr[3]}))

		if blockType == 0 && blockLen >= 18 {
			data := make([]byte, blockLen)
			if _, err := io.ReadFull(f, data); err != nil {
				return
			}
			v := uint64(data[10])<<56 | uint64(data[11])<<48 |
				uint64(data[12])<<40 | uint64(data[13])<<32 |
				uint64(data[14])<<24 | uint64(data[15])<<16 |
				uint64(data[16])<<8 | uint64(data[17])
			sampleRate := int64(v >> 44)
			totalSamples := int64(v & 0x0000000FFFFFFFFF)
			if sampleRate > 0 && totalSamples > 0 {
				lt.DurationMs = totalSamples * 1000 / sampleRate
				durationCache.Store(key, lt.DurationMs)
			}
			return
		}

		if _, err := f.Seek(int64(blockLen), io.SeekCurrent); err != nil {
			return
		}
		if isLast {
			break
		}
	}
}

// ChooseLocalFolder opens a directory picker and returns only the chosen path
// (does NOT scan). The frontend stores the list and calls ScanLocalFolder separately.
func (a *App) ChooseLocalFolder() (string, error) {
	dir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Add Local Music Folder",
	})
	if err != nil {
		return "", err
	}
	return dir, nil
}

// computeLocalFingerprints computes the SHA-256 content hash and (if available)
// the Chromaprint acoustic fingerprint for a local track.  Results are cached
// by path+size+mtime so only new / modified files pay the I/O cost.
func computeLocalFingerprints(path string, fi os.FileInfo, lt *LocalTrack) {
	key := durationCacheKey(path, fi)

	// Check cache first.
	if v, ok := fingerprintCache.Load(key); ok {
		pair := v.([2]string)
		lt.Fingerprint = pair[0]
		lt.AcousticFingerprint = pair[1]
		return
	}

	// SHA-256 content hash.
	if f, err := os.Open(path); err == nil {
		h := sha256.New()
		if _, err := io.Copy(h, f); err == nil {
			lt.Fingerprint = "sha256:" + hex.EncodeToString(h.Sum(nil))
		}
		f.Close()
	}

	// Chromaprint acoustic fingerprint (best-effort).
	if fpcalcPath != "" {
		cmd := exec.Command(fpcalcPath, "-plain", path)
		if out, err := cmd.Output(); err == nil {
			lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
			if len(lines) >= 2 && lines[1] != "" {
				lt.AcousticFingerprint = lines[1]
			}
		}
	}

	fingerprintCache.Store(key, [2]string{lt.Fingerprint, lt.AcousticFingerprint})
}

// ─── Server Connection ───────────────────────────────────────────────────────

// ConnectResult is returned on a successful server login.
type ConnectResult struct {
	User  json.RawMessage `json:"user"`
	Token string          `json:"token"`
}

// ConnectToServer authenticates against a remote Pneuma server.
func (a *App) ConnectToServer(serverURL, username, password string) (*ConnectResult, error) {
	serverURL = strings.TrimRight(serverURL, "/")

	body, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	resp, err := http.Post(serverURL+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("server unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("login failed (%d): %s", resp.StatusCode, string(msg))
	}

	var result ConnectResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid server response: %w", err)
	}

	a.mu.Lock()
	a.serverURL = serverURL
	a.token = result.Token
	// Start background token refresh.
	refreshCtx, cancel := context.WithCancel(a.ctx)
	a.stopRefresh = cancel
	a.mu.Unlock()

	go a.refreshLoop(refreshCtx)

	return &result, nil
}

// DisconnectFromServer clears the server connection state.
func (a *App) DisconnectFromServer() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.serverURL = ""
	a.token = ""
	if a.stopRefresh != nil {
		a.stopRefresh()
		a.stopRefresh = nil
	}
}

// IsConnected returns whether the app is connected to a server.
func (a *App) IsConnected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.token != ""
}

// GetServerURL returns the current server URL (empty if not connected).
func (a *App) GetServerURL() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.serverURL
}

// GetToken returns the current JWT (empty if not connected).
func (a *App) GetToken() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.token
}

// UploadLocalFile uploads a local file to the server library.
func (a *App) UploadLocalFile(filePath string) (string, error) {
	a.mu.RLock()
	url := a.serverURL
	tok := a.token
	a.mu.RUnlock()

	if url == "" || tok == "" {
		return "", fmt.Errorf("not connected to a server")
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", err
	}
	writer.Close()

	req, err := http.NewRequest("POST", url+"/api/library/tracks/upload", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upload failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return string(respBody), nil
}

// Notify sends a desktop OS notification (logging fallback).
func (a *App) Notify(title, message string) {
	wailsruntime.LogInfo(a.ctx, fmt.Sprintf("[notify] %s: %s", title, message))
}

// ─── Internal ────────────────────────────────────────────────────────────────

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

// handleLocalArt extracts embedded album art from a local audio file.
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

	// Stat for ETag before opening the file.
	info, err := os.Stat(path)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	etag := fmt.Sprintf(`"%x-%x"`, info.ModTime().UnixNano(), info.Size())
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
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

	pic := m.Picture()
	ct := pic.MIMEType
	if ct == "" {
		ct = "image/jpeg"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Write(pic.Data) //nolint:errcheck
}

// refreshLoop periodically refreshes the JWT before it expires.
func (a *App) refreshLoop(ctx context.Context) {
	// Refresh every 20 hours (token TTL is 24h).
	ticker := time.NewTicker(20 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.mu.RLock()
			url := a.serverURL
			tok := a.token
			a.mu.RUnlock()
			if url == "" || tok == "" {
				return
			}

			req, err := http.NewRequest("POST", url+"/api/auth/refresh", nil)
			if err != nil {
				continue
			}
			req.Header.Set("Authorization", "Bearer "+tok)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				slog.Warn("token refresh failed", "err", err)
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				slog.Warn("token refresh returned", "status", resp.StatusCode)
				continue
			}

			var result struct {
				Token string `json:"token"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				continue
			}

			a.mu.Lock()
			a.token = result.Token
			a.mu.Unlock()
		}
	}
}
