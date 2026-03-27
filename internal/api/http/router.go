package pneumahttp

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"

	"pneuma/internal/api/http/handlers"
	"pneuma/internal/api/http/middleware"
	apws "pneuma/internal/api/ws"
	"pneuma/internal/library"
	"pneuma/internal/playback"
	"pneuma/internal/playlist"
	"pneuma/internal/store/sqlite/serverdb"
	"pneuma/internal/user"
)

// Services groups all domain services the API depends on.
type Services struct {
	Library  *library.Service
	User     *user.Service
	Playback *playback.Engine
	Hub      *apws.Hub
	Queries  *serverdb.Queries
	Playlist *playlist.Service
	Scanner  interface {
		ScanAll()
		ScanPath(path string)
	} // *scanner.Scheduler
	JWTSecret   string
	UploadsDir  string
	ArtworkDir  string // directory for playlist artwork thumbnails
	UploadMaxMB int    // max upload body size in MB (0 = default 500 MB)
	WebUI       fs.FS  // embedded dashboard assets (nil = disabled)
	WebPlayerUI fs.FS  // embedded web player assets (nil = disabled)
}

// NewRouter builds and returns the configured Echo router.
func NewRouter(svc Services) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(echomw.LoggerWithConfig(echomw.LoggerConfig{
		Format: "[${time_rfc3339}] [HTTP]: ${method} ${uri}  ${status}  ${latency_human}  ${remote_ip}\n",
	}))
	e.Use(echomw.Recover())
	e.Use(echomw.CORS())
	e.Use(echomw.RequestID())

	secret := svc.JWTSecret
	authMW := middleware.RequireAuth(secret)
	adminMW := middleware.RequireAdmin(secret)

	lh := handlers.NewLibraryHandler(svc.Library, svc.Queries, svc.Scanner, svc.Hub, svc.UploadsDir)
	ph := handlers.NewPlaybackHandler(svc.Playback)
	uh := handlers.NewUserHandler(svc.User, secret)
	ah := handlers.NewAdminHandler(svc.User, svc.Queries)
	plh := handlers.NewPlaylistHandler(svc.Playlist, svc.Hub, svc.ArtworkDir)
	rh := handlers.NewRecentHandler(svc.Queries)

	svc.Hub.SetMessageHandler(playbackWSDispatch(svc.Playback))

	// WebSocket: validate JWT from ?token= query param for user identity.
	e.GET("/ws", func(c echo.Context) error {
		var userID string
		if tok := c.QueryParam("token"); tok != "" {
			if claims, err := middleware.ParseToken(secret, tok); err == nil {
				userID = claims.UserID
			}
		}
		svc.Hub.ServeWS(c.Response(), c.Request(), userID)
		return nil
	})

	// Auth (public)
	auth := e.Group("/api/auth")
	auth.POST("/register", uh.Register)
	auth.POST("/login", uh.Login)
	auth.POST("/password", uh.ChangePassword, authMW)
	auth.POST("/refresh", uh.Refresh, authMW)
	auth.GET("/stream-token", uh.StreamToken, authMW)

	// Admin (admin-only)
	admin := e.Group("/api/admin", adminMW)
	admin.GET("/users", ah.ListUsers)
	admin.PUT("/users/:id/permissions", ah.UpdatePermissions)
	admin.DELETE("/users/:id", ah.DeleteUser)
	admin.GET("/audit", ah.ListAudit)

	// Library (authenticated, some with permission guards)
	lib := e.Group("/api/library", authMW)
	lib.GET("/tracks", lh.ListTracks)
	lib.GET("/tracks/:id", lh.GetTrack)
	lib.GET("/tracks/:id/stream", lh.StreamTrack)
	lib.GET("/tracks/:id/art", lh.ServeTrackArt)
	lib.PATCH("/tracks/:id", lh.UpdateTrackMeta, middleware.RequirePerm(secret, "can_edit"))

	uploadMaxMB := svc.UploadMaxMB
	if uploadMaxMB <= 0 {
		uploadMaxMB = 500
	}

	uploadBodyLimit := echomw.BodyLimit(fmt.Sprintf("%dM", uploadMaxMB))

	lib.POST("/tracks/upload", lh.UploadTrack, middleware.RequirePerm(secret, "can_upload"), uploadBodyLimit)
	lib.DELETE("/tracks/:id", lh.DeleteTrack, middleware.RequirePerm(secret, "can_delete"))
	lib.GET("/albumgroups", lh.ListAlbumGroups)
	lib.GET("/search", lh.Search)
	lib.POST("/scan", lh.TriggerScan, adminMW)

	// Playlists (authenticated)
	pl := e.Group("/api/playlists", authMW)
	pl.GET("", plh.ListPlaylists)
	pl.POST("", plh.CreatePlaylist)
	pl.GET("/:id", plh.GetPlaylist)
	pl.PUT("/:id", plh.UpdatePlaylist)
	pl.DELETE("/:id", plh.DeletePlaylist)
	pl.GET("/:id/items", plh.GetPlaylistItems)
	pl.PUT("/:id/items", plh.SetPlaylistItems)
	pl.POST("/:id/items", plh.AddPlaylistItem)
	pl.POST("/:id/artwork", plh.UploadPlaylistArt)
	pl.GET("/:id/art", plh.ServePlaylistArt)

	// Stream (supports query-param token for <audio> elements)
	// This is an alternative stream endpoint that accepts ?token= for clients
	// that cannot set Authorization headers (e.g., <audio src="">).
	e.GET("/api/stream/tracks/:id", lh.StreamTrack, middleware.RequireAuth(secret))

	// Playback (authenticated)
	play := e.Group("/api/playback", authMW)
	play.GET("", ph.GetState)
	play.POST("/play", ph.Play)
	play.POST("/pause", ph.Pause)
	play.POST("/seek", ph.Seek)
	play.POST("/next", ph.Next)
	play.POST("/prev", ph.Prev)
	play.POST("/queue", ph.SetQueue)
	play.POST("/repeat", ph.SetRepeat)
	play.POST("/shuffle", ph.SetShuffle)

	// Recently played (authenticated)
	recent := e.Group("/api/recent", authMW)
	recent.GET("", rh.GetRecent)
	recent.POST("/albums", rh.RecordAlbum)
	recent.POST("/playlists", rh.RecordPlaylist)
	recent.DELETE("/playlists/:id", rh.DeleteRecentPlaylist)

	// redirect to player UI if visiting root
	e.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/player")
	})

	serveSPA := func(prefix string, ui fs.FS) {
		if ui == nil {
			return
		}

		fileServer := http.FileServer(http.FS(ui))
		spaHandler := echo.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Strip prefix so the file server sees paths relative to the FS root.
			// /player/assets/index.js -> /assets/index.js
			r2 := *r
			r2.URL = &url.URL{}
			*r2.URL = *r.URL
			p := strings.TrimPrefix(r.URL.Path, prefix)
			if p == "" {
				p = "/"
			}
			r2.URL.Path = p

			// fall back to index.html for SPA client-side routing.
			fsPath := strings.TrimPrefix(p, "/")
			if fsPath == "" {
				fsPath = "index.html"
			}
			if f, err := ui.Open(fsPath); err == nil {
				f.Close()
				fileServer.ServeHTTP(w, &r2)
				return
			}

			r2.URL.Path = "/index.html"
			fileServer.ServeHTTP(w, &r2)
		}))

		e.GET(prefix, spaHandler)
		e.GET(prefix+"/*", spaHandler)
	}

	serveSPA("/dashboard", svc.WebUI)
	serveSPA("/player", svc.WebPlayerUI)

	return e
}

// playbackWSDispatch returns a ws.InboundHandler that routes inbound WS
// messages to the playback engine.
func playbackWSDispatch(engine *playback.Engine) apws.InboundHandler {
	log := slog.Default().With("component", "ws-dispatch")
	return func(userID string, msg apws.InboundMessage) {
		ctx := context.Background()
		switch msg.Type {
		case "playback.play":
			var p struct {
				TrackID    string `json:"track_id"`
				PositionMS int64  `json:"position_ms"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.Play(ctx, userID, p.TrackID, p.PositionMS)
			}
		case "playback.pause":
			var p struct {
				Paused     bool  `json:"paused"`
				PositionMS int64 `json:"position_ms"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.Pause(ctx, userID, p.Paused, p.PositionMS)
			}
		case "playback.seek":
			var p struct {
				PositionMS int64 `json:"position_ms"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.Seek(ctx, userID, p.PositionMS)
			}
		case "playback.next":
			engine.Next(ctx, userID)
		case "playback.prev":
			engine.Prev(ctx, userID)
		case "playback.queue":
			var p struct {
				TrackIDs   []string `json:"track_ids"`
				StartIndex int      `json:"start_index"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.SetQueue(ctx, userID, p.TrackIDs, p.StartIndex)
			}
		case "playback.repeat":
			var p struct {
				Mode playback.RepeatMode `json:"mode"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.SetRepeat(ctx, userID, p.Mode)
			}
		case "playback.shuffle":
			var p struct {
				Enabled bool `json:"enabled"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.SetShuffle(ctx, userID, p.Enabled)
			}
		default:
			log.Debug("unknown ws message type", "type", msg.Type)
		}
	}
}
