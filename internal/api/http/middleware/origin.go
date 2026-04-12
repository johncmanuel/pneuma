package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
)

// RequireSameOriginForCookieAuth blocks unsafe cross-origin browser requests
// unless they present explicit bearer/query credentials. This protects cookie
// auth flows while keeping native/header-auth clients functional.
func RequireSameOriginForCookieAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Method == http.MethodOptions {
				return next(c)
			}

			if hasHeaderOrQueryCredential(c.Request()) {
				return next(c)
			}

			if IsCrossOrigin(c.Request()) {
				return echo.NewHTTPError(http.StatusForbidden, "cross-origin request blocked")
			}

			return next(c)
		}
	}
}

// IsCrossOrigin returns true when a request includes an Origin header that
// does not match the request's own origin.
func IsCrossOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return false
	}

	originURL, err := url.Parse(origin)
	if err != nil || originURL.Scheme == "" || originURL.Host == "" {
		return true
	}

	return !strings.EqualFold(originURL.Scheme, requestScheme(r)) ||
		!strings.EqualFold(originURL.Host, requestHost(r))
}

// requestScheme returns the scheme of the request, preferring X-Forwarded-Proto
// when available.
func requestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}

	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.ToLower(strings.TrimSpace(parts[0]))
	}

	return "http"
}

// requestHost returns the host of the request, preferring X-Forwarded-Host
// when available.
func requestHost(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		host := strings.TrimSpace(parts[0])
		if host != "" {
			return host
		}
	}

	return strings.TrimSpace(r.Host)
}

// hasHeaderOrQueryCredential returns true when a request includes a header or query
// parameter that could be used to authenticate the request.
func hasHeaderOrQueryCredential(r *http.Request) bool {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(auth, "Bearer ") && strings.TrimSpace(auth[7:]) != "" {
		return true
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	return token != ""
}
