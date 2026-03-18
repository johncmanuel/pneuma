package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"pneuma/internal/api/http/middleware"
	"pneuma/internal/models"
	"pneuma/internal/store/sqlite/dbconv"
	"pneuma/internal/store/sqlite/serverdb"
	"pneuma/internal/user"
)

// AdminHandler handles /api/admin/* routes.
type AdminHandler struct {
	users *user.Service
	q     *serverdb.Queries
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(users *user.Service, q *serverdb.Queries) *AdminHandler {
	return &AdminHandler{users: users, q: q}
}

// ListUsers lists all users.
func (h *AdminHandler) ListUsers(c echo.Context) error {
	users, err := h.users.ListUsers(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, users)
}

// UpdatePermissions updates a user's permissions.
func (h *AdminHandler) UpdatePermissions(c echo.Context) error {
	claims := middleware.GetClaims(c)
	ctx := c.Request().Context()
	targetID := c.Param("id")

	var body struct {
		CanUpload bool `json:"can_upload"`
		CanEdit   bool `json:"can_edit"`
		CanDelete bool `json:"can_delete"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := h.users.UpdatePermissions(ctx, targetID, body.CanUpload, body.CanEdit, body.CanDelete); err != nil {
		if err == user.ErrNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	_ = h.q.InsertAuditEntry(ctx, serverdb.InsertAuditEntryParams{
		ID:         uuid.NewString(),
		UserID:     claims.UserID,
		Action:     "update_permissions",
		TargetType: "user",
		TargetID:   targetID,
		CreatedAt:  dbconv.FormatTime(time.Now()),
	})

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteUser deletes a user.
func (h *AdminHandler) DeleteUser(c echo.Context) error {
	claims := middleware.GetClaims(c)
	ctx := c.Request().Context()
	targetID := c.Param("id")

	if err := h.users.DeleteUser(ctx, claims.UserID, targetID); err != nil {
		if err == user.ErrNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		if err == user.ErrSelfDelete {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	_ = h.q.InsertAuditEntry(ctx, serverdb.InsertAuditEntryParams{
		ID:         uuid.NewString(),
		UserID:     claims.UserID,
		Action:     "delete_user",
		TargetType: "user",
		TargetID:   targetID,
		CreatedAt:  dbconv.FormatTime(time.Now()),
	})

	return c.NoContent(http.StatusNoContent)
}

// ListAudit shows the last 500 audit entries.
func (h *AdminHandler) ListAudit(c echo.Context) error {
	ctx := c.Request().Context()
	rows, err := h.q.ListAuditEntries(ctx, 500)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	entries := dbconv.AuditsToModels(rows)
	if entries == nil {
		entries = []models.AuditEntry{}
	}
	return c.JSON(http.StatusOK, entries)
}
