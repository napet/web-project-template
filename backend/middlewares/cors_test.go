package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const site = "https://example.com"

func newCORSRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS([]string{site}))
	r.GET("/thing", func(ctx *gin.Context) { ctx.String(http.StatusOK, "read") })
	r.POST("/thing", func(ctx *gin.Context) { ctx.String(http.StatusOK, "written") })
	return r
}

func do(r *gin.Engine, method, origin string, extra map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/thing", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestPreflightFromAllowedOrigin(t *testing.T) {
	rec := do(newCORSRouter(), http.MethodOptions, site, map[string]string{"Access-Control-Request-Method": "POST"})

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, site, rec.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "POST")
	require.NotContains(t, rec.Header().Get("Access-Control-Allow-Methods"), "DELETE")
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
}

func TestPreflightFromForeignOrigin(t *testing.T) {
	rec := do(newCORSRouter(), http.MethodOptions, "https://evil.example", map[string]string{"Access-Control-Request-Method": "POST"})

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Methods"))
}

func TestPostFromAllowedOrigin(t *testing.T) {
	rec := do(newCORSRouter(), http.MethodPost, site, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, site, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestPostFromForeignOriginIsRejected(t *testing.T) {
	rec := do(newCORSRouter(), http.MethodPost, "https://evil.example", nil)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.NotEqual(t, "written", rec.Body.String())
}

func TestPostWithoutOriginIsRejected(t *testing.T) {
	rec := do(newCORSRouter(), http.MethodPost, "", nil)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestGetIsServedRegardlessOfOrigin(t *testing.T) {
	r := newCORSRouter()

	rec := do(r, http.MethodGet, "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "read", rec.Body.String())

	rec = do(r, http.MethodGet, "https://evil.example", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}
