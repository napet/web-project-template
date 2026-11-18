package main

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"app/backend/routes"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fixedStore struct{}

func (fixedStore) Get(context.Context) (int64, error)       { return 1, nil }
func (fixedStore) Increment(context.Context) (int64, error) { return 2, nil }

// startServer runs newServer on a free port with short timeouts and returns its base URL.
func startServer(t *testing.T, heartbeat time.Duration) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler, err := routes.NewRouter(routes.Options{
		AllowedOrigins: []string{"https://example.com"},
		Store:          fixedStore{},
		Heartbeat:      heartbeat,
	})
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := newServer("", handler, timeouts{readHeader: 100 * time.Millisecond, read: 200 * time.Millisecond, idle: 300 * time.Millisecond})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	return "http://" + listener.Addr().String()
}

func TestStreamOutlivesReadTimeout(t *testing.T) {
	base := startServer(t, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/visitors/stream", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// keep reading heartbeats for 5x the read timeout and 3x the idle timeout
	deadline := time.Now().Add(time.Second)
	scanner := bufio.NewScanner(resp.Body)
	pings := 0
	for time.Now().Before(deadline) {
		require.True(t, scanner.Scan(), "stream closed after %d pings: %v", pings, scanner.Err())
		if strings.HasPrefix(scanner.Text(), ": ping") {
			pings++
		}
	}
	require.Greater(t, pings, 10)
}

func TestIdleConnectionIsClosed(t *testing.T) {
	base := startServer(t, time.Hour)
	conn, err := net.Dial("tcp", strings.TrimPrefix(base, "http://"))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	// send nothing: the server must drop the connection after the header timeout
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, err = conn.Read(make([]byte, 1))
	require.Error(t, err)
	var netErr net.Error
	if errors.As(err, &netErr) {
		require.False(t, netErr.Timeout(), "the server should close the connection before our 2 s deadline")
	}
}
