package middleware

import (
	"github.com/labstack/echo/v4"
)

// SecurityHeaders returns middleware that sets recommended HTTP security headers
// on every response to mitigate common web vulnerabilities.
func SecurityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			h := c.Response().Header()

			// require Trusted Types for DOM sinks.
			h.Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self'; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data: blob:; "+
					"media-src 'self' blob:; "+
					"connect-src 'self' ws: wss:; "+
					"font-src 'self'; "+
					"frame-ancestors 'none'; "+
					"trusted-types default svelte-trusted-html; "+
					"require-trusted-types-for 'script'",
			)

			// Prevent MIME type sniffing to reduce the risk of XSS attacks.
			h.Set("X-Content-Type-Options", "nosniff")

			// Isolate the browsing context to prevent cross-origin attacks.
			// COOP is only honored by browsers on secure (HTTPS) origins, so skip it on
			// plain HTTP to avoid a spurious console warning (e.g. when running behind a
			// reverse proxy like Tailscale that terminates TLS externally).
			if c.Request().TLS != nil || c.Request().Header.Get("X-Forwarded-Proto") == "https" {
				h.Set("Cross-Origin-Opener-Policy", "same-origin")
			}

			// Control the Referer header to protect user privacy and prevent information leakage.
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Prevent the page from being embedded in iframes (clickjacking mitigation).
			h.Set("X-Frame-Options", "DENY")

			return next(c)
		}
	}
}
