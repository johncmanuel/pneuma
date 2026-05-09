package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"pneuma/internal/models"
)

// ServerClient manages the remote Pneuma server connection, authentication, and
// all outbound API requests. It owns its own mutex and state, decoupling the
// App composition root from any remote connectivity concerns.
type ServerClient struct {
	// mu protects serverURL, token, and stopRefresh while accessing them.
	mu sync.RWMutex
	// serverURL is the URL of the remote Pneuma server.
	serverURL string
	// token is the JWT used for authentication.
	token string
	// stopRefresh is a function that can be called to stop the background refresh loop.
	stopRefresh context.CancelFunc

	// parentCtx is used to derive refresh-loop contexts tied to the app lifetime.
	parentCtx context.Context
}

// NewServerClient creates a new ServerClient bound to the application context.
func NewServerClient(ctx context.Context) *ServerClient {
	return &ServerClient{parentCtx: ctx}
}

// Close cancels any running background refresh loop.
func (c *ServerClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopRefresh != nil {
		c.stopRefresh()
		c.stopRefresh = nil
	}
}

// Credentials returns the currently stored (serverURL, token) pair.
// The caller must treat these as a consistent snapshot.
func (c *ServerClient) Credentials() (string, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverURL, c.token
}

// IsConnected returns whether the client has an active session token.
func (c *ServerClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token != ""
}

// GetServerURL returns the current server URL (empty if not connected).
func (c *ServerClient) GetServerURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverURL
}

// GetToken returns the current JWT (empty if not connected).
func (c *ServerClient) GetToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

// setSession stores new credentials and starts the background refresh loop.
// Must be called with the mutex already locked (mu.Lock).
func (c *ServerClient) setSession(serverURL, token string) {
	if c.stopRefresh != nil {
		c.stopRefresh()
	}
	c.serverURL = serverURL
	c.token = token
	refreshCtx, cancel := context.WithCancel(c.parentCtx)
	c.stopRefresh = cancel
	go c.refreshLoop(refreshCtx)
}

// RestoreSession validates the stored JWT via the refresh endpoint and, on
// success, starts the background refresh loop.
func (c *ServerClient) RestoreSession(serverURL, token string) error {
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

	c.mu.Lock()
	c.setSession(serverURL, result.Token)
	c.mu.Unlock()

	return nil
}

// ConnectToServer authenticates against a remote Pneuma server and starts the
// background JWT refresh loop on success.
func (c *ServerClient) ConnectToServer(serverURL, username, password string) (*ConnectResult, error) {
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

	c.mu.Lock()
	c.setSession(serverURL, result.Token)
	c.mu.Unlock()

	return &result, nil
}

// Disconnect clears the server connection state and cancels the refresh loop.
func (c *ServerClient) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serverURL = ""
	c.token = ""
	if c.stopRefresh != nil {
		c.stopRefresh()
		c.stopRefresh = nil
	}
}

// UploadLocalFile uploads a local file to the server library via multipart streaming.
func (c *ServerClient) UploadLocalFile(filePath string) (string, error) {
	url, tok := c.Credentials()
	if url == "" || tok == "" {
		return "", fmt.Errorf("not connected to a server")
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}

	// Stream the multipart body via io.Pipe so the file is never fully buffered in memory.
	// this way, large audio files can be uploaded without consuming excessive memory.
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		part, err := mw.CreateFormFile("file", filepath.Base(filePath))
		if err != nil {
			f.Close()
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, f); err != nil {
			f.Close()
			pw.CloseWithError(err)
			return
		}
		f.Close()
		pw.CloseWithError(mw.Close())
	}()

	req, err := http.NewRequest("POST", url+"/api/library/tracks/upload", pr)
	if err != nil {
		pr.CloseWithError(err)
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
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

// CreatePlaylist creates a new playlist on the server and optionally sets its items.
// Returns the remote playlist ID.
func (c *ServerClient) CreatePlaylist(serverURL, token, name, description string, items []models.PlaylistItem) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"name":        name,
		"description": description,
	})

	req, err := http.NewRequest("POST", serverURL+"/api/playlists", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("server unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create playlist failed (%d): %s", resp.StatusCode, string(msg))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("invalid response: %w", err)
	}

	if len(items) > 0 {
		if err := c.UpdatePlaylistItems(serverURL, token, result.ID, items); err != nil {
			// playlist was created but setting items failed
			return result.ID, err
		}
	}

	return result.ID, nil
}

// UpdatePlaylistItems replaces all items of a server playlist.
func (c *ServerClient) UpdatePlaylistItems(serverURL, token, playlistID string, items []models.PlaylistItem) error {
	body, _ := json.Marshal(items)
	req, err := http.NewRequest("PUT", serverURL+"/api/playlists/"+playlistID+"/items", bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("server unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("set items failed (%d): %s", resp.StatusCode, string(msg))
	}
	return nil
}

// UploadPlaylistArt uploads the given JPEG data as a playlist's artwork on the server.
func (c *ServerClient) UploadPlaylistArt(serverURL, token, remotePlaylistID string, jpgData []byte) error {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	fw, err := w.CreateFormFile("file", "artwork.jpg")
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}
	if _, err := fw.Write(jpgData); err != nil {
		return fmt.Errorf("write data: %w", err)
	}
	w.Close()

	url := fmt.Sprintf("%s/api/playlists/%s/artwork", serverURL, remotePlaylistID)
	req, err := http.NewRequest("POST", url, &body)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// FetchPlaylistArt downloads the server's raw artwork bytes for a given remote playlist.
func (c *ServerClient) FetchPlaylistArt(serverURL, token, remotePlaylistID string) ([]byte, error) {
	url := fmt.Sprintf("%s/api/playlists/%s/art", serverURL, remotePlaylistID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download artwork: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// FetchAllRemoteTracks fetches all tracks from the server, paginating with pageSize.
func (c *ServerClient) FetchAllRemoteTracks(serverURL, token string, pageSize int) ([]models.Track, error) {
	var all []models.Track
	page := 1
	for {
		url := fmt.Sprintf("%s/api/library/tracks?page=%d&page_size=%d", serverURL, page, pageSize)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch tracks page %d: %w", page, err)
		}

		var pageResult struct {
			Tracks     []models.Track `json:"tracks"`
			TotalPages int            `json:"total_pages"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&pageResult); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode page %d: %w", page, err)
		}
		resp.Body.Close()

		all = append(all, pageResult.Tracks...)
		if page >= pageResult.TotalPages {
			break
		}
		page++
	}
	return all, nil
}

// refreshLoop periodically refreshes the JWT every 20 hours before it expires.
func (c *ServerClient) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.doTokenRefresh()
		}
	}
}

// doTokenRefresh performs a single token refresh round-trip.
// Extracted from refreshLoop so that defer resp.Body.Close() is scoped to one
// call frame and fires on every exit path instead of accumulating across loop
// iterations.
func (c *ServerClient) doTokenRefresh() {
	c.mu.RLock()
	url := c.serverURL
	tok := c.token
	c.mu.RUnlock()
	if url == "" || tok == "" {
		return
	}

	req, err := http.NewRequest("POST", url+"/api/auth/refresh", nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("token refresh failed", "err", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("token refresh returned", "status", resp.StatusCode)
		return
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	c.mu.Lock()
	c.token = result.Token
	c.mu.Unlock()
}
