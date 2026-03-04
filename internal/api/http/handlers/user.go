package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"pneuma/internal/user"
)

// UserHandler handles /api/auth/* routes.
type UserHandler struct {
	users *user.Service
}

func NewUserHandler(users *user.Service) *UserHandler {
	return &UserHandler{users: users}
}

// Register POST /api/auth/register  body: {username, password}
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
		return internalErr(err)
	}
	return c.JSON(http.StatusCreated, u)
}

// Login POST /api/auth/login  body: {username, password}
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
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, u)
}

// ChangePassword POST /api/auth/password  body: {user_id, new_password}
func (h *UserHandler) ChangePassword(c echo.Context) error {
	var body struct {
		UserID      string `json:"user_id"`
		NewPassword string `json:"new_password"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.users.ChangePassword(c.Request().Context(), body.UserID, body.NewPassword); err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
