package routes

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const testOrigin = "https://example.com"

func init() {
	gin.SetMode(gin.TestMode)
}

// fakeStore is an in-memory CounterStore; set err to simulate a DB outage.
type fakeStore struct {
	mu    sync.Mutex
	count int64
	err   error
}

func (s *fakeStore) Get(context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return 0, s.err
	}
	return s.count, nil
}

func (s *fakeStore) Increment(context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return 0, s.err
	}
	s.count++
	return s.count, nil
}

func (s *fakeStore) setErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

var errDBDown = errors.New("db down")

func newRouter(t *testing.T, opts Options) *gin.Engine {
	t.Helper()
	if opts.AllowedOrigins == nil {
		opts.AllowedOrigins = []string{testOrigin}
	}
	if opts.Store == nil {
		opts.Store = &fakeStore{}
	}
	router, err := NewRouter(opts)
	require.NoError(t, err)
	return router
}

type request struct {
	method     string
	path       string
	remoteAddr string
	headers    map[string]string
}

func do(t *testing.T, router http.Handler, r request) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(r.method, r.path, nil)
	if r.remoteAddr != "" {
		req.RemoteAddr = r.remoteAddr
	}
	if r.method != http.MethodGet {
		req.Header.Set("Origin", testOrigin)
	}
	for k, v := range r.headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func post(t *testing.T, router http.Handler, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, router, request{method: http.MethodPost, path: "/api/v1/visitors", remoteAddr: remoteAddr})
}

// sseStream reads raw lines of an open event stream into a channel.
type sseStream struct {
	resp  *http.Response
	lines chan string
}

func openStream(t *testing.T, server *httptest.Server) (*sseStream, *http.Response) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/visitors/stream", nil)
	require.NoError(t, err)
	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })

	s := &sseStream{resp: resp, lines: make(chan string, 64)}
	if resp.StatusCode != http.StatusOK {
		close(s.lines)
		return s, resp
	}
	go func() {
		defer close(s.lines)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			s.lines <- scanner.Text()
		}
	}()
	return s, resp
}

// nextEvent returns the next event's name and data, skipping comments and retry hints.
func (s *sseStream) nextEvent(t *testing.T) (string, string) {
	t.Helper()
	var name, data string
	timeout := time.After(3 * time.Second)
	for {
		select {
		case line, ok := <-s.lines:
			require.True(t, ok, "stream closed before an event arrived")
			switch {
			case line == "":
				if name != "" || data != "" {
					return name, data
				}
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				data = strings.TrimPrefix(line, "data: ")
			}
		case <-timeout:
			t.Fatal("timed out waiting for an event")
		}
	}
}

// waitForLine waits until a line matching pred arrives.
func (s *sseStream) waitForLine(t *testing.T, pred func(string) bool) string {
	t.Helper()
	timeout := time.After(3 * time.Second)
	for {
		select {
		case line, ok := <-s.lines:
			require.True(t, ok, "stream closed")
			if pred(line) {
				return line
			}
		case <-timeout:
			t.Fatal("timed out waiting for line")
		}
	}
}
