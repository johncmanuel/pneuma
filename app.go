package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
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
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/image/draw"
)

// ffprobePath is resolved once at init; empty means unavailable.
var ffprobePath string

// durationCache avoids re-running ffprobe for files that haven't changed.
// Key: "path|size|mtime_unix"  Value: duration in milliseconds.
var durationCache sync.Map

// artworkHashCache maps "path|size|mtime_unix" → sha256 hex of the raw artwork
// bytes. Lets us resolve the content-addressed thumbnail path without re-reading
// the audio file on every subsequent request, and ensures all tracks sharing
// the same embedded image share a single cached thumbnail on disk.
var artworkHashCache sync.Map

func init() {
	if p, err := exec.LookPath("ffprobe"); err == nil {
		ffprobePath = p
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

	// App-local SQLite database for persisting desktop client state.
	appDB *sql.DB

	// Local stream server (serves local audio files to the <audio> element).
	localPort int
	localSrv  *http.Server

	// Directory for cached artwork thumbnails.
	thumbDir string

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

	// Open the app-local SQLite KV store for persisting desktop client state.
	if db, err := openAppDB(); err != nil {
		slog.Warn("app db open failed — local state will not be persisted", "err", err)
	} else {
		a.appDB = db
	}

	// Initialise thumbnail cache directory.
	if cacheDir, err := os.UserCacheDir(); err == nil {
		a.thumbDir = filepath.Join(cacheDir, "pneuma", "thumbs")
	} else {
		a.thumbDir = filepath.Join(os.TempDir(), "pneuma-thumbs")
		slog.Warn("UserCacheDir unavailable, using temp dir for thumbnails", "dir", a.thumbDir)
	}
	if err := os.MkdirAll(a.thumbDir, 0o755); err != nil {
		slog.Error("failed to create thumbnail cache dir", "dir", a.thumbDir, "err", err)
	}

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
	a.closeAppDB()
}

// onSecondInstanceLaunch is called by Wails when a second instance of the app is launched.
func (a *App) onSecondInstanceLaunch(secondInstanceData options.SecondInstanceData) {
	secondInstanceArgs := secondInstanceData.Args

	slog.Info("second instance launched", "args", secondInstanceArgs)
	slog.Info("opened from directorry:", secondInstanceData.WorkingDirectory)

	runtime.WindowUnminimise(a.ctx)
	runtime.Show(a.ctx)

	go runtime.EventsEmit(a.ctx, "launchArgs", secondInstanceArgs)
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
	Path        string `json:"path"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	AlbumArtist string `json:"album_artist"`
	Genre       string `json:"genre"`
	Year        int    `json:"year"`
	TrackNumber int    `json:"track_number"`
	DiscNumber  int    `json:"disc_number"`
	DurationMs  int64  `json:"duration_ms"` // 0 if unavailable from tags
	HasArtwork  bool   `json:"has_artwork"`
}

// LocalAlbumGroup represents a group of tracks sharing the same album+artist,
// computed via SQL GROUP BY. The frontend uses this for the album grid without
// needing the full track list.
type LocalAlbumGroup struct {
	Key            string `json:"key"`              // "albumName|||albumArtist"
	Name           string `json:"name"`             // album name
	Artist         string `json:"artist"`           // album artist
	TrackCount     int    `json:"track_count"`      // number of tracks in the album
	FirstTrackPath string `json:"first_track_path"` // for artwork resolution
}

// ScanLocalFolderStream recursively scans a directory for audio files,
// reading embedded tags and persisting each track to the local SQLite DB.
// Instead of returning the full list, progress is streamed to the frontend
// via Wails events so the UI can show "42 / 200 songs scanned":
//
//	"local:scan:start"   → { folder string, total int }
//	"local:track:scanned" → { folder string, done int, total int, track LocalTrack }
//	"local:scan:done"    → { folder string, count int }
//
// The method itself returns nil on success or an error.
func (a *App) ScanLocalFolderStream(dir string) error {
	// ── Pass 1: count audio files so we know the total ──
	var total int
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if audioExts[strings.ToLower(filepath.Ext(path))] {
			total++
		}
		return nil
	})

	wailsruntime.EventsEmit(a.ctx, "local:scan:start", map[string]any{
		"folder": dir,
		"total":  total,
	})

	// ── Pass 2: read metadata, upsert to DB, emit per-file progress ──
	done := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !audioExts[ext] {
			return nil
		}

		lt := LocalTrack{Path: path, Title: filepath.Base(path)}

		f, err := os.Open(path)
		if err != nil {
			done++
			_ = a.upsertLocalTrack(lt, dir)
			wailsruntime.EventsEmit(a.ctx, "local:track:scanned", map[string]any{
				"folder": dir, "done": done, "total": total, "track": lt,
			})
			return nil
		}

		m, tagErr := tag.ReadFrom(f)
		f.Close()

		if tagErr == nil {
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
		}

		probeLocalDuration(path, info, &lt)
		if lt.DurationMs == 0 {
			parseDurationFallbackLocal(path, info, &lt)
		}

		_ = a.upsertLocalTrack(lt, dir)

		done++
		wailsruntime.EventsEmit(a.ctx, "local:track:scanned", map[string]any{
			"folder": dir, "done": done, "total": total, "track": lt,
		})
		return nil
	})

	wailsruntime.EventsEmit(a.ctx, "local:scan:done", map[string]any{
		"folder": dir,
		"count":  done,
	})
	return err
}

// GetLocalTracks returns all cached tracks for the given folders from the
// local SQLite DB.  This is an indexed query — much faster than re-scanning
// the file system or deserialising a JSON blob.
func (a *App) GetLocalTracks(folders []string) ([]LocalTrack, error) {
	return a.getLocalTracks(folders)
}

// GetLocalTracksPage returns a paginated slice of cached tracks for the given
// folders. Offset/limit pagination keeps IPC payloads small.
func (a *App) GetLocalTracksPage(folders []string, offset, limit int) ([]LocalTrack, int, error) {
	return a.getLocalTracksPage(folders, offset, limit)
}

// SearchLocalTracks performs a case-insensitive search across title, artist,
// album, and path columns of local_tracks, returning at most 50 results.
func (a *App) SearchLocalTracks(folders []string, query string) ([]LocalTrack, error) {
	return a.searchLocalTracks(folders, query)
}

// GetLocalTracksByPaths returns tracks for the given exact paths.
// Used by the queue to resolve track IDs without loading the full library.
func (a *App) GetLocalTracksByPaths(paths []string) ([]LocalTrack, error) {
	return a.getLocalTracksByPaths(paths)
}

// LocalAlbumGroupsResult holds paginated album groups plus the total count.
type LocalAlbumGroupsResult struct {
	Albums []LocalAlbumGroup `json:"albums"`
	Total  int               `json:"total"`
}

// GetLocalAlbumGroups returns paginated album groups computed via SQL GROUP BY.
// filter is an optional case-insensitive substring match on album name or artist.
func (a *App) GetLocalAlbumGroups(folders []string, filter string, offset, limit int) (*LocalAlbumGroupsResult, error) {
	return a.getLocalAlbumGroups(folders, filter, offset, limit)
}

// GetLocalAlbumTracks returns the tracks for a specific album by its group key
// ("albumName|||albumArtist"), ordered by disc/track number.
func (a *App) GetLocalAlbumTracks(folders []string, albumName, albumArtist string) ([]LocalTrack, error) {
	return a.getLocalAlbumTracks(folders, albumName, albumArtist)
}

// ClearLocalFolder removes all cached tracks for a folder from the local DB.
func (a *App) ClearLocalFolder(folder string) error {
	return a.deleteLocalTracksByFolder(folder)
}

// FindLocalDuplicates returns groups of local tracks that share the same
// title, album, and album_artist (metadata-only duplicate detection via SQL).
func (a *App) FindLocalDuplicates(folders []string) ([]LocalDuplicateGroup, error) {
	return a.findLocalDuplicates(folders)
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

// ─── Server Connection ───────────────────────────────────────────────────────

// ConnectResult is returned on a successful server login.
type ConnectResult struct {
	User  json.RawMessage `json:"user"`
	Token string          `json:"token"`
}

// RestoreSession attempts to restore a previous server session by validating
// the stored JWT via the server's refresh endpoint. On success a fresh token
// is stored and the background refresh loop is started. Returns an error if
// the token has expired or the server is unreachable — the caller should then
// clear the stored session and prompt the user to log in again.
func (a *App) RestoreSession(serverURL, token string) error {
	serverURL = strings.TrimRight(serverURL, "/")

	req, err := http.NewRequest("POST", serverURL+"/api/auth/refresh", nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("server unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session expired (%d): %s", resp.StatusCode, string(msg))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("invalid server response: %w", err)
	}

	a.mu.Lock()
	a.serverURL = serverURL
	a.token = result.Token
	refreshCtx, cancel := context.WithCancel(a.ctx)
	a.stopRefresh = cancel
	a.mu.Unlock()

	go a.refreshLoop(refreshCtx)
	return nil
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

// ClearArtworkCache removes all cached thumbnail files from the thumbs
// directory and resets the in-memory artwork hash cache. The cache is
// rebuilt on demand when artwork is next requested.
func (a *App) ClearArtworkCache() error {
	entries, err := os.ReadDir(a.thumbDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		_ = os.Remove(filepath.Join(a.thumbDir, e.Name()))
	}
	// Purge the in-memory hash map so subsequent requests regenerate thumbnails.
	artworkHashCache.Range(func(k, _ any) bool {
		artworkHashCache.Delete(k)
		return true
	})
	return nil
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

// thumbMaxDim is the maximum width/height for cached artwork thumbnails.
const thumbMaxDim = 400

// thumbCacheKey returns a short hex key derived from the file path, size, and
// mtime. Used only as an index into artworkHashCache — not as the disk filename.
func thumbCacheKey(path string, info os.FileInfo) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%d|%d", path, info.Size(), info.ModTime().UnixNano())
	return hex.EncodeToString(h.Sum(nil))[:16]
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
