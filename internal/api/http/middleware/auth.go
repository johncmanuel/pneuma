package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

const (
	// ContextKey is the echo context key used to store parsed JWT claims.
	ContextKey = "user_claims"

	// SessionCookieName is the HttpOnly cookie that stores the access token.
	SessionCookieName = "pneuma_session"
)

// Claims represents the JWT claims embedded in every access token.
type Claims struct {
	jwt.RegisteredClaims
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	IsAdmin   bool   `json:"is_admin"`
	CanUpload bool   `json:"can_upload"`
	CanEdit   bool   `json:"can_edit"`
	CanDelete bool   `json:"can_delete"`
}

// AccessTokenTTL is the lifetime of a regular access token.
const AccessTokenTTL = 24 * time.Hour

// RefreshTokenTTL is the lifetime of a refresh token.
const RefreshTokenTTL = 7 * 24 * time.Hour

// GenerateToken creates a signed JWT for the given user.
func GenerateToken(secret, userID, username string, isAdmin, canUpload, canEdit, canDelete bool, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		UserID:    userID,
		Username:  username,
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

// RequireAuthWithQuery validates JWT from Authorization header, session cookie,
// or token query parameter. This is intended only for endpoints that cannot
// attach Authorization headers, such as the desktop client's (via <audio>) and WebSocket URL auth.
func RequireAuthWithQuery(secret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, err := ParseRequestClaimsWithQuery(c.Request(), secret)
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

// ParseToken validates a raw JWT string and returns the embedded Claims.
// Useful outside of Echo handlers (e.g. WebSocket upgrade).
func ParseToken(secret, tokenStr string) (*Claims, error) {
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

// ParseRequestClaims extracts and validates auth claims from an HTTP request.
// Token sources are checked in this order:
// 1. Authorization: Bearer <token>
// 2. HttpOnly session cookie
func ParseRequestClaims(r *http.Request, secret string) (*Claims, error) {
	tokenStr := tokenFromRequest(r)
	if tokenStr == "" {
		return nil, echo.ErrUnauthorized
	}
	return ParseToken(secret, tokenStr)
}

// ParseRequestClaimsWithQuery validates claims from header/cookie or token query.
func ParseRequestClaimsWithQuery(r *http.Request, secret string) (*Claims, error) {
	tokenStr := tokenFromRequestWithQuery(r)
	if tokenStr == "" {
		return nil, echo.ErrUnauthorized
	}
	return ParseToken(secret, tokenStr)
}

// extractClaims is a helper function to extract claims from an echo context.
func extractClaims(c echo.Context, secret string) (*Claims, error) {
	return ParseRequestClaims(c.Request(), secret)
}

// tokenFromRequest extracts the token from the Authorization header or session cookie.
func tokenFromRequest(r *http.Request) string {
	return tokenFromHeaderOrCookie(r)
}

// tokenFromRequestWithQuery extracts the token from the Authorization header, session cookie, or token query parameter.
func tokenFromRequestWithQuery(r *http.Request) string {
	token := tokenFromHeaderOrCookie(r)
	if token != "" {
		return token
	}

	return strings.TrimSpace(r.URL.Query().Get("token"))
}

// tokenFromHeaderOrCookie extracts the token from the Authorization header or session cookie.
func tokenFromHeaderOrCookie(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimSpace(auth[7:])
		if token != "" {
			return token
		}
	}

	if cookie, err := r.Cookie(SessionCookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	return ""
}

// hasPerm checks if the given claims have the specified permission.
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
