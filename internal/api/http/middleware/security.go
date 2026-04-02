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

			// Mitigate XSS attacks; require Trusted Types for DOM sinks.
			h.Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self'; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data: blob:; "+
					"media-src 'self' blob:; "+
					"connect-src 'self' ws: wss:; "+
					"font-src 'self'; "+
					"frame-ancestors 'none'; "+
					"trusted-types default; "+
					"require-trusted-types-for 'script'",
			)

			// Enforce HTTPS for the site and all subdomains; opt into the preload list.
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

			// Isolate the browsing context to prevent cross-origin attacks.
			h.Set("Cross-Origin-Opener-Policy", "same-origin")

			// Prevent the page from being embedded in iframes (clickjacking mitigation).
			h.Set("X-Frame-Options", "DENY")

			return next(c)
		}
	}
}
