package pneumahttp

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

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

	// RateLimitingEnabled toggles the application-layer rate limiter.
	// Set to false if using a reverse proxy with its own rate limiting.
	RateLimitingEnabled bool
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
	e.Use(middleware.SecurityHeaders())

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
		deviceID := c.QueryParam("device_id")
		svc.Hub.ServeWS(c.Response(), c.Request(), userID, deviceID)
		return nil
	})

	// Auth (public)
	auth := e.Group("/api/auth")

	// disable rate limiting by default
	noop := func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	registerRL := noop
	loginRL := noop
	passwordRL := noop

	// think these are good rates, though self-hosters need to rely more on reverse proxy rate limiting
	// than application layer rate limiting. having this added in will be useful for servers not using
	// reverse proxies
	if svc.RateLimitingEnabled {
		registerRL = newRateLimiter(10.0/3600.0, 10, 2*time.Hour) // 10 per hour
		loginRL = newRateLimiter(30.0/60.0, 15, 5*time.Minute)    // 30 per minute
		passwordRL = newRateLimiter(20.0/60.0, 10, 5*time.Minute) // 20 per minute
	}

	auth.POST("/register", uh.Register, registerRL)
	auth.POST("/login", uh.Login, loginRL)

	auth.POST("/password", uh.ChangePassword, authMW, passwordRL)
	auth.POST("/refresh", uh.Refresh, authMW)
	auth.GET("/stream-token", uh.StreamToken, authMW)

	// Admin
	admin := e.Group("/api/admin", adminMW)
	admin.GET("/users", ah.ListUsers)
	admin.PUT("/users/:id/permissions", ah.UpdatePermissions)
	admin.DELETE("/users/:id", ah.DeleteUser)
	admin.GET("/audit", ah.ListAudit)

	// Library
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

	// Playlists
	pl := e.Group("/api/playlists", authMW)
	pl.GET("", plh.ListPlaylists)
	pl.POST("", plh.CreatePlaylist)
	pl.POST("/generate", plh.GenerateRandom)
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

	// Playback
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

	// Recently played
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
	return func(userID, deviceID string, msg apws.InboundMessage) {
		ctx := context.Background()
		switch msg.Type {
		case "playback.play":
			var p struct {
				TrackID    string `json:"track_id"`
				PositionMS int64  `json:"position_ms"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.Play(ctx, userID, deviceID, p.TrackID, p.PositionMS)
			}
		case "playback.pause":
			var p struct {
				Paused     bool  `json:"paused"`
				PositionMS int64 `json:"position_ms"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.Pause(ctx, userID, deviceID, p.Paused, p.PositionMS)
			}
		case "playback.seek":
			var p struct {
				PositionMS int64 `json:"position_ms"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.Seek(ctx, userID, deviceID, p.PositionMS)
			}
		case "playback.next":
			engine.Next(ctx, userID, deviceID)
		case "playback.prev":
			engine.Prev(ctx, userID, deviceID)
		case "playback.queue":
			var p struct {
				TrackIDs   []string `json:"track_ids"`
				StartIndex int      `json:"start_index"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.SetQueue(ctx, userID, deviceID, p.TrackIDs, p.StartIndex)
			}
		case "playback.repeat":
			var p struct {
				Mode playback.RepeatMode `json:"mode"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.SetRepeat(ctx, userID, deviceID, p.Mode)
			}
		case "playback.shuffle":
			var p struct {
				Enabled bool `json:"enabled"`
			}
			if json.Unmarshal(msg.Payload, &p) == nil {
				engine.SetShuffle(ctx, userID, deviceID, p.Enabled)
			}
		default:
			log.Debug("unknown ws message type", "type", msg.Type)
		}
	}
}

// newRateLimiter returns an IP-based rate limiter Echo middleware.
// r is the sustained rate (requests/second), burst is the max burst size,
// and expiresIn controls how long an idle IP is kept in memory.
func newRateLimiter(r rate.Limit, burst int, expiresIn time.Duration) echo.MiddlewareFunc {
	retryAfterSecs := strconv.Itoa(int(math.Ceil(float64(burst) / float64(r))))
	return echomw.RateLimiterWithConfig(echomw.RateLimiterConfig{
		Skipper: echomw.DefaultSkipper,
		Store: echomw.NewRateLimiterMemoryStoreWithConfig(
			echomw.RateLimiterMemoryStoreConfig{
				Rate:      r,
				Burst:     burst,
				ExpiresIn: expiresIn,
			},
		),
		IdentifierExtractor: func(ctx echo.Context) (string, error) {
			id := ctx.RealIP()
			return id, nil
		},
		ErrorHandler: func(ctx echo.Context, err error) error {
			slog.Error("Rate limiter store error", "ip", ctx.RealIP(), "error", err)
			return ctx.JSON(http.StatusInternalServerError, map[string]string{
				"message": "An internal error occurred. Please try again later.",
			})
		},
		DenyHandler: func(ctx echo.Context, identifier string, err error) error {
			slog.Warn("Rate limit exceeded", "ip", ctx.RealIP(), "identifier", identifier, "error", err)
			ctx.Response().Header().Set("Retry-After", retryAfterSecs)
			return ctx.JSON(http.StatusTooManyRequests, map[string]string{
				"message": "Too many requests. Please try again later.",
			})
		},
	})
}
