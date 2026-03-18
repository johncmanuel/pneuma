package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"pneuma/internal/api/http/middleware"
	"pneuma/internal/user"
)

// UserHandler handles /api/auth/* routes.
type UserHandler struct {
	users  *user.Service
	secret string
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(users *user.Service, jwtSecret string) *UserHandler {
	return &UserHandler{users: users, secret: jwtSecret}
}

// Register registers a new user with body {username, password}
func (h *UserHandler) Register(c echo.Context) error {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if body.Username == "" || body.Password == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "username and password required")
	}

	u, err := h.users.Register(c.Request().Context(), body.Username, body.Password)
	if err != nil {
		if err == user.ErrUserExists {
			return echo.NewHTTPError(http.StatusConflict, err.Error())
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	token, err := middleware.GenerateToken(
		h.secret, u.ID, u.Username, u.IsAdmin,
		u.CanUpload, u.CanEdit, u.CanDelete,
		middleware.AccessTokenTTL,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"user":  u,
		"token": token,
	})
}

// Login logs in a user with body {username, password}
func (h *UserHandler) Login(c echo.Context) error {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	u, err := h.users.Login(c.Request().Context(), body.Username, body.Password)
	if err != nil {
		if err == user.ErrWrongPassword {
			return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	token, err := middleware.GenerateToken(
		h.secret, u.ID, u.Username, u.IsAdmin,
		u.CanUpload, u.CanEdit, u.CanDelete,
		middleware.AccessTokenTTL,
	)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"user":  u,
		"token": token,
	})
}

// Refresh issues a new token from a valid existing one.
func (h *UserHandler) Refresh(c echo.Context) error {
	claims := middleware.GetClaims(c)
	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing token")
	}

	// Re-read user from DB to pick up any permission changes.
	u, err := h.users.GetByID(c.Request().Context(), claims.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if u == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "user not found")
	}

	token, err := middleware.GenerateToken(
		h.secret, u.ID, u.Username, u.IsAdmin,
		u.CanUpload, u.CanEdit, u.CanDelete,
		middleware.AccessTokenTTL,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"token": token,
	})
}

// ChangePassword changes a user's password. Only the authenticated user (changing their own) or an admin may call this.
func (h *UserHandler) ChangePassword(c echo.Context) error {
	claims := middleware.GetClaims(c)
	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing token")
	}

	var body struct {
		UserID      string `json:"user_id"`
		NewPassword string `json:"new_password"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if claims.UserID != body.UserID && !claims.IsAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "cannot change another user's password")
	}

	if err := h.users.ChangePassword(c.Request().Context(), body.UserID, body.NewPassword); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// StreamToken GET /api/auth/stream-token?track_id=...
// Issues a short-lived token for use with <audio> src URLs.
func (h *UserHandler) StreamToken(c echo.Context) error {
	claims := middleware.GetClaims(c)
	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing token")
	}

	token, err := middleware.GenerateToken(
		h.secret, claims.UserID, claims.Username, claims.IsAdmin,
		claims.CanUpload, claims.CanEdit, claims.CanDelete,
		middleware.StreamTokenTTL,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]string{
		"token": token,
	})
}
