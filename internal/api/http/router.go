package pneumahttp

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"pneuma/internal/api/http/handlers"
	apws "pneuma/internal/api/ws"
	"pneuma/internal/library"
	"pneuma/internal/offline"
	"pneuma/internal/playback"
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
	Scanner  interface{ ScanAll() } // *scanner.Scheduler
}

// NewRouter builds and returns the configured Echo router.
func NewRouter(svc Services) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(middleware.RequestID())

	lh := handlers.NewLibraryHandler(svc.Library, svc.Scanner)
	ph := handlers.NewPlaybackHandler(svc.Playback, svc.Handoff)
	uh := handlers.NewUserHandler(svc.User)

	// WebSocket
	e.GET("/ws", func(c echo.Context) error {
		svc.Hub.ServeWS(c.Response(), c.Request())
		return nil
	})

	// Auth
	auth := e.Group("/api/auth")
	auth.POST("/register", uh.Register)
	auth.POST("/login", uh.Login)
	auth.POST("/password", uh.ChangePassword)

	// Library
	lib := e.Group("/api/library")
	lib.GET("/tracks", lh.ListTracks)
	lib.GET("/tracks/:id", lh.GetTrack)
	lib.GET("/tracks/:id/stream", lh.StreamTrack)
	lib.GET("/tracks/:id/art", lh.ServeTrackArt)
	lib.PATCH("/tracks/:id", lh.UpdateTrackMeta)
	lib.GET("/albums", lh.ListAlbums)
	lib.GET("/search", lh.Search)
	lib.POST("/scan", lh.TriggerScan)

	// Playback
	play := e.Group("/api/playback")
	play.GET("/:device_id", ph.GetState)
	play.POST("/:device_id/play", ph.Play)
	play.POST("/:device_id/pause", ph.Pause)
	play.POST("/:device_id/seek", ph.Seek)
	play.POST("/:device_id/next", ph.Next)
	play.POST("/:device_id/prev", ph.Prev)
	play.POST("/:device_id/queue", ph.SetQueue)
	play.POST("/:device_id/repeat", ph.SetRepeat)
	play.POST("/:device_id/shuffle", ph.SetShuffle)

	e.POST("/api/handoff", ph.Transfer)
	e.GET("/api/sessions/:user_id", ph.Sessions)

	// Offline
	off := e.Group("/api/offline")
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

	return e
}
