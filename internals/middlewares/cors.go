package middlewares

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// allowedOrigins returns the set of permitted origins from the ALLOWED_ORIGINS
// environment variable (comma-separated). If the variable is not set, an empty
// set is returned, which causes the middleware to reflect any non-empty origin
// (development fallback).
func allowedOrigins() map[string]struct{} {
	raw := os.Getenv("ALLOWED_ORIGINS")
	if raw == "" {
		return map[string]struct{}{}
	}
	set := make(map[string]struct{})
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			set[o] = struct{}{}
		}
	}
	return set
}

func CORSMiddleware() gin.HandlerFunc {
	// Parse the allowlist once at startup rather than on every request.
	origins := allowedOrigins()

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Determine whether to reflect the origin or deny it.
		allowOrigin := ""
		if origin != "" {
			if len(origins) == 0 {
				// No allowlist configured – reflect any origin (development mode).
				allowOrigin = origin
			} else if _, ok := origins[origin]; ok {
				allowOrigin = origin
			}
		}

		if allowOrigin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
			c.Writer.Header().Set("Vary", "Origin")
		}

		if c.Request.Method == "OPTIONS" {
			if allowOrigin != "" {
				c.AbortWithStatus(204)
			} else {
				c.AbortWithStatus(403)
			}
			return
		}

		c.Next()
	}
}
