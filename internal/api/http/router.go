package pneumahttp

import (
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"

	"pneuma/internal/api/http/handlers"
	"pneuma/internal/api/http/middleware"
	apws "pneuma/internal/api/ws"
	"pneuma/internal/library"
	"pneuma/internal/offline"
	"pneuma/internal/playback"
	"pneuma/internal/store/sqlite"
	"pneuma/internal/user"
)

// Services groups all domain services the API depends on.
type Services struct {
	Library  *library.Service
	User     *user.Service
	Playback *playback.Engine
	Handoff  *playback.Handoff
	Offline  *offline.Packager
	Hub      *apws.Hub
	Store    *sqlite.Store
	Scanner  interface {
		ScanAll()
		ScanPath(path string)
	} // *scanner.Scheduler
	Fingerprinter interface {
		Available() bool
		FingerprintString(ctx context.Context, path string) (string, error)
	} // *chromaprint.Service; nil disables acoustic dedup on upload
	JWTSecret  string
	UploadsDir string
	WebUI      fs.FS // embedded web UI assets (nil = disabled)
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

	lh := handlers.NewLibraryHandler(svc.Library, svc.Store, svc.Scanner, svc.Hub, svc.UploadsDir, svc.Fingerprinter)
	ph := handlers.NewPlaybackHandler(svc.Playback, svc.Handoff)
	uh := handlers.NewUserHandler(svc.User, secret)
	ah := handlers.NewAdminHandler(svc.User, svc.Store)

	// Wire inbound WebSocket messages to the playback engine.
	svc.Hub.SetMessageHandler(playbackWSDispatch(svc.Playback))

	// WebSocket — validate JWT from ?token= query param for user identity.
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

	// ── Auth (public) ─────────────────────────────────────────────────────────
	auth := e.Group("/api/auth")
	auth.POST("/register", uh.Register)
	auth.POST("/login", uh.Login)
	auth.POST("/password", uh.ChangePassword, authMW)
	auth.POST("/refresh", uh.Refresh, authMW)
	auth.GET("/stream-token", uh.StreamToken, authMW)

	// ── Admin (admin-only) ────────────────────────────────────────────────────
	admin := e.Group("/api/admin", adminMW)
	admin.GET("/users", ah.ListUsers)
	admin.PUT("/users/:id/permissions", ah.UpdatePermissions)
	admin.DELETE("/users/:id", ah.DeleteUser)
	admin.GET("/audit", ah.ListAudit)

	// ── Library (authenticated, some with permission guards) ──────────────────
	lib := e.Group("/api/library", authMW)
	lib.GET("/tracks", lh.ListTracks)
	lib.GET("/tracks/:id", lh.GetTrack)
	lib.GET("/tracks/:id/stream", lh.StreamTrack)
	lib.GET("/tracks/:id/art", lh.ServeTrackArt)
	lib.PATCH("/tracks/:id", lh.UpdateTrackMeta, middleware.RequirePerm(secret, "can_edit"))
	lib.POST("/tracks/upload", lh.UploadTrack, middleware.RequirePerm(secret, "can_upload"))
	lib.DELETE("/tracks/:id", lh.DeleteTrack, middleware.RequirePerm(secret, "can_delete"))
	lib.GET("/albums", lh.ListAlbums)
	lib.GET("/albumgroups", lh.ListAlbumGroups)
	lib.GET("/search", lh.Search)
	lib.POST("/scan", lh.TriggerScan, adminMW)

	// ── Stream (supports query-param token for <audio> elements) ──────────────
	// This is an alternative stream endpoint that accepts ?token= for clients
	// that cannot set Authorization headers (e.g., <audio src="">).
	e.GET("/api/stream/tracks/:id", lh.StreamTrack, middleware.RequireAuth(secret))

	// ── Playback (authenticated) ──────────────────────────────────────────────
	play := e.Group("/api/playback", authMW)
	play.GET("/:device_id", ph.GetState)
	play.POST("/:device_id/play", ph.Play)
	play.POST("/:device_id/pause", ph.Pause)
	play.POST("/:device_id/seek", ph.Seek)
	play.POST("/:device_id/next", ph.Next)
	play.POST("/:device_id/prev", ph.Prev)
	play.POST("/:device_id/queue", ph.SetQueue)
	play.POST("/:device_id/repeat", ph.SetRepeat)
	play.POST("/:device_id/shuffle", ph.SetShuffle)

	e.POST("/api/handoff", ph.Transfer, authMW)
	e.GET("/api/sessions/:user_id", ph.Sessions, authMW)

	// ── Offline (authenticated) ───────────────────────────────────────────────
	off := e.Group("/api/offline", authMW)
	off.GET("/:user_id", func(c echo.Context) error {
		packs, err := svc.Offline.ListPacks(c.Request().Context(), c.Param("user_id"))
		if err != nil {
			return echo.ErrInternalServerError
		}
		return c.JSON(http.StatusOK, packs)
	})
	off.POST("/:user_id/tracks/:track_id", func(c echo.Context) error {
		t, err := svc.Library.TrackByID(c.Request().Context(), c.Param("track_id"))
		if err != nil || t == nil {
			return echo.NewHTTPError(http.StatusNotFound, "track not found")
		}
		go svc.Offline.Download(c.Request().Context(), t, c.Param("user_id"))
		return c.JSON(http.StatusAccepted, map[string]string{"status": "queued"})
	})
	off.DELETE("/:user_id/tracks/:track_id", func(c echo.Context) error {
		if err := svc.Offline.Remove(c.Request().Context(), c.Param("user_id"), c.Param("track_id")); err != nil {
			return echo.ErrInternalServerError
		}
		return c.NoContent(http.StatusNoContent)
	})

	// ── Web UI (SPA fallback) ────────────────────────────────────────────────
	if svc.WebUI != nil {
		// Serve static assets directly; anything that doesn't match a file
		// falls back to index.html (SPA routing).
		fileServer := http.FileServer(http.FS(svc.WebUI))
		spaHandler := echo.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try to serve the file as-is first.
			path := r.URL.Path
			if path == "/" {
				path = "index.html"
			} else if len(path) > 0 && path[0] == '/' {
				path = path[1:]
			}
			if f, err := svc.WebUI.Open(path); err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
			// Fallback to index.html for SPA client-side routing.
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
		}))
		e.GET("/", spaHandler)
		e.GET("/*", spaHandler)
	}

	return e
}

// playbackWSDispatch returns a ws.InboundHandler that routes inbound WS
// messages to the playback engine.  The REST endpoints remain available as
// a fallback (e.g. for the desktop Wails app).
func playbackWSDispatch(engine *playback.Engine) apws.InboundHandler {
	log := slog.Default().With("component", "ws-dispatch")
	return func(userID string, msg apws.InboundMessage) {
		ctx := context.Background()
		switch msg.Type {
		case "playback.play":
			var p struct {
				DeviceID   string `json:"device_id"`
				TrackID    string `json:"track_id"`
				PositionMS int64  `json:"position_ms"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.Play(ctx, p.DeviceID, userID, p.TrackID, p.PositionMS)
			}
		case "playback.pause":
			var p struct {
				DeviceID   string `json:"device_id"`
				Paused     bool   `json:"paused"`
				PositionMS int64  `json:"position_ms"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.Pause(ctx, p.DeviceID, userID, p.Paused, p.PositionMS)
			}
		case "playback.seek":
			var p struct {
				DeviceID   string `json:"device_id"`
				PositionMS int64  `json:"position_ms"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.Seek(ctx, p.DeviceID, userID, p.PositionMS)
			}
		case "playback.next":
			var p struct {
				DeviceID string `json:"device_id"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.Next(ctx, p.DeviceID, userID)
			}
		case "playback.prev":
			var p struct {
				DeviceID string `json:"device_id"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.Prev(ctx, p.DeviceID, userID)
			}
		case "playback.queue":
			var p struct {
				DeviceID   string   `json:"device_id"`
				TrackIDs   []string `json:"track_ids"`
				StartIndex int      `json:"start_index"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.SetQueue(ctx, p.DeviceID, userID, p.TrackIDs, p.StartIndex)
			}
		case "playback.repeat":
			var p struct {
				DeviceID string              `json:"device_id"`
				Mode     playback.RepeatMode `json:"mode"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.SetRepeat(ctx, p.DeviceID, userID, p.Mode)
			}
		case "playback.shuffle":
			var p struct {
				DeviceID string `json:"device_id"`
				Enabled  bool   `json:"enabled"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.SetShuffle(ctx, p.DeviceID, userID, p.Enabled)
			}
		default:
			log.Debug("unknown ws message type", "type", msg.Type)
		}
	}
}
