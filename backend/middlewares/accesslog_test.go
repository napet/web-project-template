package middlewares

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func logLines(t *testing.T, logClientHeaders bool, trustedProxies []string, requests ...*http.Request) []map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(trustedProxies))
	r.Use(AccessLog(logger, logClientHeaders, "/health"))
	r.GET("/thing", func(ctx *gin.Context) { ctx.Status(http.StatusTeapot) })
	r.GET("/health", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })
	for _, req := range requests {
		r.ServeHTTP(httptest.NewRecorder(), req)
	}

	var lines []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		lines = append(lines, entry)
	}
	return lines
}

func request(path, remoteAddr string, headers map[string]string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

func TestAccessLogShortensTheClient(t *testing.T) {
	lines := logLines(t, false, nil, request("/thing", "203.0.113.77:4000", map[string]string{"X-Forwarded-For": "198.51.100.9"}))
	require.Len(t, lines, 1)
	line := lines[0]
	require.Equal(t, "request", line["msg"])
	require.Equal(t, "GET", line["method"])
	require.Equal(t, "/thing", line["path"])
	require.EqualValues(t, http.StatusTeapot, line["status"])
	require.Equal(t, "203.0.113.0/24", line["client"])

	raw, _ := json.Marshal(line)
	require.NotContains(t, string(raw), "203.0.113.77", "no full address without diagnostics")
	require.NotContains(t, string(raw), "198.51.100.9", "no forwarding headers without diagnostics")
}

func TestAccessLogDiagnosticsShowHeaders(t *testing.T) {
	lines := logLines(t, true, []string{"203.0.113.0/24"}, request("/thing", "203.0.113.77:4000", map[string]string{
		"X-Forwarded-For":  "198.51.100.9, 203.0.113.5",
		"Cf-Connecting-Ip": "198.51.100.9",
	}))
	require.Len(t, lines, 1)
	line := lines[0]
	require.Equal(t, "198.51.100.9", line["client_ip"], "the trusted proxies are skipped")
	require.Equal(t, "198.51.100.0/24", line["client"])
	require.Equal(t, "203.0.113.77:4000", line["remote_addr"])
	require.Equal(t, "198.51.100.9, 203.0.113.5", line["header_X-Forwarded-For"])
	require.Equal(t, "198.51.100.9", line["header_Cf-Connecting-Ip"])
	require.NotContains(t, line, "header_True-Client-Ip", "absent headers are left out")
}

func TestAccessLogSkipsHealthChecksAtInfo(t *testing.T) {
	lines := logLines(t, false, nil,
		request("/health", "10.0.0.1:1", nil),
		request("/thing", "10.0.0.1:1", nil),
	)
	require.Len(t, lines, 1)
	require.Equal(t, "/thing", lines[0]["path"])
}

func TestShortenIP(t *testing.T) {
	cases := map[string]string{
		"203.0.113.77":          "203.0.113.0/24",
		"::ffff:203.0.113.77":   "203.0.113.0/24",
		"2001:db8:1234:5678::1": "2001:db8:1234::/48",
		"fe80::1%eth0":          "fe80::/48",
		"not-an-ip":             "invalid",
		"":                      "invalid",
	}
	for in, want := range cases {
		require.Equal(t, want, ShortenIP(in), in)
	}
}
