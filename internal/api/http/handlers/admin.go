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

func NewAdminHandler(users *user.Service, q *serverdb.Queries) *AdminHandler {
	return &AdminHandler{users: users, q: q}
}

// ListUsers GET /api/admin/users
func (h *AdminHandler) ListUsers(c echo.Context) error {
	users, err := h.users.ListUsers(c.Request().Context())
	if err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, users)
}

// UpdatePermissions PUT /api/admin/users/:id/permissions
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
		return internalErr(err)
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

// DeleteUser DELETE /api/admin/users/:id
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
		return internalErr(err)
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

// ListAudit GET /api/admin/audit
func (h *AdminHandler) ListAudit(c echo.Context) error {
	ctx := c.Request().Context()
	rows, err := h.q.ListAuditEntries(ctx, 500)
	if err != nil {
		return internalErr(err)
	}
	entries := dbconv.AuditsToModels(rows)
	if entries == nil {
		entries = []models.AuditEntry{}
	}
	return c.JSON(http.StatusOK, entries)
}
