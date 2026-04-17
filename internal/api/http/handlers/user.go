package handlers

import (
	"net/http"
	"strings"
	"time"

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

	h.setSessionCookie(c, token)

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

	h.setSessionCookie(c, token)

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

	h.setSessionCookie(c, token)

	return c.JSON(http.StatusOK, map[string]any{
		"token": token,
	})
}

// Me returns the authenticated user's profile.
func (h *UserHandler) Me(c echo.Context) error {
	claims := middleware.GetClaims(c)
	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing token")
	}

	u, err := h.users.GetByID(c.Request().Context(), claims.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if u == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "user not found")
	}

	return c.NoContent(http.StatusOK)
}

// Logout clears the HttpOnly session cookie.
func (h *UserHandler) Logout(c echo.Context) error {
	h.clearSessionCookie(c)
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
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
		return c.NoContent(http.StatusForbidden)
	}

	if err := h.users.ChangePassword(c.Request().Context(), body.UserID, body.NewPassword); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// setSessionCookie sets the HttpOnly session cookie.
func (h *UserHandler) setSessionCookie(c echo.Context, token string) {
	maxAgeSeconds := int((middleware.AccessTokenTTL + time.Second - 1) / time.Second)
	c.SetCookie(&http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   requestIsHTTPS(c),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAgeSeconds,
		Expires:  time.Now().Add(middleware.AccessTokenTTL),
	})
}

// clearSessionCookie clears the HttpOnly session cookie.
func (h *UserHandler) clearSessionCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   requestIsHTTPS(c),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// requestIsHTTPS checks if the request is over HTTPS.
func requestIsHTTPS(c echo.Context) bool {
	if c.Request().TLS != nil {
		return true
	}

	// NOTE: X-Forwarded-Proto can be spoofed in situations where one isn't using a
	// reverse proxy in an HTTPS environment.
	forwarded := strings.TrimSpace(c.Request().Header.Get("X-Forwarded-Proto"))
	if forwarded == "" {
		return false
	}

	parts := strings.Split(forwarded, ",")
	return strings.EqualFold(strings.TrimSpace(parts[0]), "https")
}
