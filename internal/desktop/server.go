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
	"time"
)

// RestoreSession attempts to restore a previous server session by validating
// the stored JWT via the server's refresh endpoint. On success a fresh token
// is stored and the background refresh loop is started. Returns an error if
// the token has expired or the server is unreachable.
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

	if a.stopRefresh != nil {
		a.stopRefresh() // cancel any running refresh goroutine before replacing
	}

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
	if a.stopRefresh != nil {
		a.stopRefresh() // cancel any running refresh goroutine before replacing
	}
	a.serverURL = serverURL
	a.token = result.Token
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
	// Stream the multipart body via io.Pipe — the file is never fully buffered
	// in memory, which is important for large audio files.
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
			a.doTokenRefresh()
		}
	}
}

// doTokenRefresh performs a single token refresh round-trip.
// Extracted from refreshLoop so that defer resp.Body.Close() is scoped to one
// call frame and fires on every exit path instead of accumulating across loop
// iterations.
func (a *App) doTokenRefresh() {
	a.mu.RLock()
	url := a.serverURL
	tok := a.token
	a.mu.RUnlock()
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

	a.mu.Lock()
	a.token = result.Token
	a.mu.Unlock()
}
