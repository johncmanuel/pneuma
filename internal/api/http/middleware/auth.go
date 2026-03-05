package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// ContextKey is the echo context key used to store parsed JWT claims.
const ContextKey = "user_claims"

// Claims represents the JWT claims embedded in every access token.
type Claims struct {
	jwt.RegisteredClaims
	UserID    string `json:"uid"`
	IsAdmin   bool   `json:"adm"`
	CanUpload bool   `json:"can_upload"`
	CanEdit   bool   `json:"can_edit"`
	CanDelete bool   `json:"can_delete"`
}

// AccessTokenTTL is the lifetime of a regular access token.
const AccessTokenTTL = 24 * time.Hour

// RefreshTokenTTL is the lifetime of a refresh token.
const RefreshTokenTTL = 7 * 24 * time.Hour

// StreamTokenTTL is the lifetime of a short-lived stream token.
const StreamTokenTTL = 60 * time.Second

// GenerateToken creates a signed JWT for the given user.
func GenerateToken(secret, userID string, isAdmin, canUpload, canEdit, canDelete bool, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		UserID:    userID,
		IsAdmin:   isAdmin,
		CanUpload: canUpload,
		CanEdit:   canEdit,
		CanDelete: canDelete,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// RequireAuth is middleware that validates the JWT from the Authorization header
// and stores the parsed Claims in the echo context.
func RequireAuth(secret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, err := extractClaims(c, secret)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or missing token")
			}
			c.Set(ContextKey, claims)
			return next(c)
		}
	}
}

// RequireAdmin is middleware that requires an authenticated admin user.
func RequireAdmin(secret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, err := extractClaims(c, secret)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or missing token")
			}
			if !claims.IsAdmin {
				return echo.NewHTTPError(http.StatusForbidden, "admin access required")
			}
			c.Set(ContextKey, claims)
			return next(c)
		}
	}
}

// RequirePerm is middleware that requires the caller to have a specific
// permission flag set. Admins implicitly satisfy all permission checks.
func RequirePerm(secret string, perm string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, err := extractClaims(c, secret)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or missing token")
			}
			if !claims.IsAdmin && !hasPerm(claims, perm) {
				return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
			}
			c.Set(ContextKey, claims)
			return next(c)
		}
	}
}

// GetClaims retrieves the parsed Claims from the echo context.
// Returns nil if no claims are present (i.e., route was not authenticated).
func GetClaims(c echo.Context) *Claims {
	v := c.Get(ContextKey)
	if v == nil {
		return nil
	}
	claims, ok := v.(*Claims)
	if !ok {
		return nil
	}
	return claims
}

// ─── internal helpers ────────────────────────────────────────────────────────

func extractClaims(c echo.Context, secret string) (*Claims, error) {
	// First check Authorization header.
	auth := c.Request().Header.Get("Authorization")
	tokenStr := ""
	if strings.HasPrefix(auth, "Bearer ") {
		tokenStr = auth[7:]
	}
	// Fall back to query param (used for <audio> stream tokens).
	if tokenStr == "" {
		tokenStr = c.QueryParam("token")
	}
	if tokenStr == "" {
		return nil, echo.ErrUnauthorized
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, echo.ErrUnauthorized
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, echo.ErrUnauthorized
	}
	return claims, nil
}

func hasPerm(claims *Claims, perm string) bool {
	switch perm {
	case "can_upload":
		return claims.CanUpload
	case "can_edit":
		return claims.CanEdit
	case "can_delete":
		return claims.CanDelete
	default:
		return false
	}
}
