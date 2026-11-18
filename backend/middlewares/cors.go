package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	allowedMethods  = "GET, POST, OPTIONS"
	allowedHeaders  = "Content-Type"
	preflightMaxAge = "600"
)

// CORS answers cross-origin requests for the allowed origins and rejects
// state-changing requests from any other origin.
//
//   - Allowed origins get Access-Control-Allow-Origin (never credentials).
//   - OPTIONS preflights are answered here with 204.
//   - GET requests are served regardless of origin.
//   - Any other method without an allowed Origin header gets 403.
//
// It must be registered on the engine (not a group) so it also runs for
// OPTIONS requests that have no matching route.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}

	return func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")
		_, isAllowed := allowed[origin]
		isAllowed = isAllowed && origin != ""

		header := ctx.Writer.Header()
		header.Add("Vary", "Origin")
		if isAllowed {
			header.Set("Access-Control-Allow-Origin", origin)
		}

		switch ctx.Request.Method {
		case http.MethodOptions:
			if isAllowed {
				header.Set("Access-Control-Allow-Methods", allowedMethods)
				header.Set("Access-Control-Allow-Headers", allowedHeaders)
				header.Set("Access-Control-Max-Age", preflightMaxAge)
			}
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		case http.MethodGet, http.MethodHead:
			ctx.Next()
			return
		}

		if !isAllowed {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "Invalid origin"})
			return
		}
		ctx.Next()
	}
}
