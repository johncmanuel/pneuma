package desktop

import (
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"pneuma/internal/models"
)

// GetLocalPort returns the local stream server port.
func (a *App) GetLocalPort() int {
	if a.streamer != nil {
		return a.streamer.Port()
	}
	return 0
}

// Notify sends a desktop OS notification (logging fallback).
func (a *App) Notify(title, message string) {
	runtime.LogInfo(a.ctx, fmt.Sprintf("[notify] %s: %s", title, message))
}

// ClearArtworkCache removes all cached thumbnail files from the thumbs
// directory and resets the in-memory artwork hash cache. The cache is
// rebuilt on demand when artwork is next requested.
func (a *App) ClearArtworkCache() error {
	if a.streamer != nil {
		return a.streamer.ClearArtworkCache()
	}
	return nil
}

// RestoreSession attempts to restore a previous server session.
func (a *App) RestoreSession(serverURL, token string) error {
	if a.client == nil {
		return fmt.Errorf("server client not initialized")
	}
	return a.client.RestoreSession(serverURL, token)
}

// ConnectToServer authenticates against a remote Pneuma server.
func (a *App) ConnectToServer(serverURL, username, password string) (*ConnectResult, error) {
	if a.client == nil {
		return nil, fmt.Errorf("server client not initialized")
	}
	return a.client.ConnectToServer(serverURL, username, password)
}

// DisconnectFromServer clears the server connection state.
func (a *App) DisconnectFromServer() {
	if a.client != nil {
		a.client.Disconnect()
	}
}

// IsConnected returns whether the app is connected to a server.
func (a *App) IsConnected() bool {
	if a.client == nil {
		return false
	}
	return a.client.IsConnected()
}

// GetServerURL returns the current server URL (empty if not connected).
func (a *App) GetServerURL() string {
	if a.client == nil {
		return ""
	}
	return a.client.GetServerURL()
}

// GetToken returns the current JWT (empty if not connected).
func (a *App) GetToken() string {
	if a.client == nil {
		return ""
	}
	return a.client.GetToken()
}

// UploadLocalFile uploads a local file to the server library.
func (a *App) UploadLocalFile(filePath string) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("server client not initialized")
	}
	return a.client.UploadLocalFile(filePath)
}

// CreateServerPlaylist creates a playlist on the server and returns its remote ID.
// This is a thin orchestration wrapper; the HTTP logic lives in ServerClient.
func (a *App) CreateServerPlaylist(serverURL, token, name, description string, items []models.PlaylistItem) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("server client not initialized")
	}
	return a.client.CreatePlaylist(serverURL, token, name, description, items)
}
