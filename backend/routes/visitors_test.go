package routes

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"app/backend/limiter"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReadFreshCount(t *testing.T) {
	router := newRouter(t, Options{})
	rec := do(t, router, request{method: http.MethodGet, path: "/api/v1/visitors"})

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"count":0}`, rec.Body.String())
}

func TestReadWithoutOrigin(t *testing.T) {
	router := newRouter(t, Options{Store: &fakeStore{count: 3}})
	rec := do(t, router, request{method: http.MethodGet, path: "/api/v1/visitors"})

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"count":3}`, rec.Body.String())
}

func TestReadDatabaseUnavailable(t *testing.T) {
	router := newRouter(t, Options{Store: &fakeStore{err: errDBDown}})
	rec := do(t, router, request{method: http.MethodGet, path: "/api/v1/visitors"})

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "message")
	require.NotContains(t, rec.Body.String(), "count")
}

func TestSingleIncrement(t *testing.T) {
	store := &fakeStore{count: 41}
	router := newRouter(t, Options{Store: store})

	rec := post(t, router, "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"count":42}`, rec.Body.String())

	rec = do(t, router, request{method: http.MethodGet, path: "/api/v1/visitors"})
	require.JSONEq(t, `{"count":42}`, rec.Body.String())
}

func TestConcurrentIncrementsFromDistinctIPs(t *testing.T) {
	store := &fakeStore{}
	router := newRouter(t, Options{Store: store})

	const n = 50
	var wg sync.WaitGroup
	codes := make(chan int, n)
	for i := range n {
		wg.Go(func() {
			codes <- post(t, router, fmt.Sprintf("10.0.0.%d:1234", i+1)).Code
		})
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		require.Equal(t, http.StatusOK, code)
	}

	count, _ := store.Get(t.Context())
	require.Equal(t, int64(n), count)
}

func TestIncrementFromForeignOrigin(t *testing.T) {
	store := &fakeStore{}
	router := newRouter(t, Options{Store: store})

	rec := do(t, router, request{
		method: http.MethodPost, path: "/api/v1/visitors",
		headers: map[string]string{"Origin": "https://evil.example"},
	})

	require.Equal(t, http.StatusForbidden, rec.Code)
	count, _ := store.Get(t.Context())
	require.Equal(t, int64(0), count)
}

func TestPreflightForVisitors(t *testing.T) {
	router := newRouter(t, Options{})
	rec := do(t, router, request{
		method: http.MethodOptions, path: "/api/v1/visitors",
		headers: map[string]string{"Access-Control-Request-Method": "POST"},
	})

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, testOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
	require.NotContains(t, rec.Header().Get("Access-Control-Allow-Methods"), "DELETE")
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
}

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

func TestIncrementRateLimit(t *testing.T) {
	c := &clock{now: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)}
	store := &fakeStore{}
	router := newRouter(t, Options{
		Store:            store,
		IncrementLimiter: limiter.NewWindow(IncrementWindow, c.Now),
	})
	const ip = "203.0.113.7:1234"

	require.Equal(t, http.StatusOK, post(t, router, ip).Code)

	c.now = c.now.Add(time.Minute)
	rec := post(t, router, ip)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	require.NoError(t, err)
	require.Equal(t, 540, retryAfter)
	count, _ := store.Get(t.Context())
	require.Equal(t, int64(1), count, "a rejected increment must not change the count")

	c.now = c.now.Add(10 * time.Minute)
	require.Equal(t, http.StatusOK, post(t, router, ip).Code)
}

func TestFailedIncrementDoesNotConsumeWindow(t *testing.T) {
	store := &fakeStore{err: errDBDown}
	router := newRouter(t, Options{Store: store})
	const ip = "203.0.113.8:1234"

	require.Equal(t, http.StatusServiceUnavailable, post(t, router, ip).Code)

	store.setErr(nil)
	require.Equal(t, http.StatusOK, post(t, router, ip).Code)
}

func TestClientIPBehindTrustedProxy(t *testing.T) {
	const proxy = "192.0.2.1:5000"
	forwarded := func(router http.Handler, client string) int {
		return do(t, router, request{
			method: http.MethodPost, path: "/api/v1/visitors", remoteAddr: proxy,
			headers: map[string]string{"X-Forwarded-For": client},
		}).Code
	}

	trusting := newRouter(t, Options{TrustedProxies: []string{"192.0.2.1"}})
	require.Equal(t, http.StatusOK, forwarded(trusting, "198.51.100.1"))
	require.Equal(t, http.StatusOK, forwarded(trusting, "198.51.100.2"), "each forwarded client is limited independently")
	require.Equal(t, http.StatusTooManyRequests, forwarded(trusting, "198.51.100.1"))

	untrusting := newRouter(t, Options{})
	require.Equal(t, http.StatusOK, forwarded(untrusting, "198.51.100.1"))
	require.Equal(t, http.StatusTooManyRequests, forwarded(untrusting, "198.51.100.2"),
		"without trusted proxies X-Forwarded-For is ignored and the proxy address is limited")
}

func TestStreamSendsInitialValue(t *testing.T) {
	server := httptest.NewServer(newRouter(t, Options{Store: &fakeStore{count: 7}}))
	t.Cleanup(server.Close)

	stream, resp := openStream(t, server)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	stream.waitForLine(t, func(l string) bool { return l == "retry: 5000" })

	name, data := stream.nextEvent(t)
	require.Equal(t, "count", name)
	require.Equal(t, "7", data)
}

func TestStreamBroadcastsIncrements(t *testing.T) {
	router := newRouter(t, Options{Store: &fakeStore{count: 1}})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	a, _ := openStream(t, server)
	b, _ := openStream(t, server)
	_, initialA := a.nextEvent(t)
	_, initialB := b.nextEvent(t)
	require.Equal(t, "1", initialA)
	require.Equal(t, "1", initialB)

	require.Equal(t, http.StatusOK, post(t, router, "").Code)

	nameA, dataA := a.nextEvent(t)
	nameB, dataB := b.nextEvent(t)
	require.Equal(t, []string{"count", "2"}, []string{nameA, dataA})
	require.Equal(t, []string{"count", "2"}, []string{nameB, dataB})
}

func TestStreamHeartbeat(t *testing.T) {
	server := httptest.NewServer(newRouter(t, Options{Heartbeat: 20 * time.Millisecond}))
	t.Cleanup(server.Close)

	stream, _ := openStream(t, server)
	stream.nextEvent(t)
	isPing := func(l string) bool { return strings.HasPrefix(l, ": ping") }
	stream.waitForLine(t, isPing)
	stream.waitForLine(t, isPing)
}

func TestStreamUnavailableDatabase(t *testing.T) {
	server := httptest.NewServer(newRouter(t, Options{Store: &fakeStore{err: errDBDown}}))
	t.Cleanup(server.Close)

	_, resp := openStream(t, server)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestStreamCapPerIP(t *testing.T) {
	streams := limiter.NewConnCap(MaxStreamsPerIP, MaxStreams)
	server := httptest.NewServer(newRouter(t, Options{StreamCap: streams}))
	t.Cleanup(server.Close)

	open := make([]*sseStream, 0, MaxStreamsPerIP)
	for range MaxStreamsPerIP {
		s, resp := openStream(t, server)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		s.nextEvent(t)
		open = append(open, s)
	}

	_, resp := openStream(t, server)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	// existing streams stay open
	require.Equal(t, MaxStreamsPerIP, streams.Open("127.0.0.1"))
	for _, s := range open {
		select {
		case _, ok := <-s.lines:
			require.True(t, ok, "existing stream was closed")
		default:
		}
	}
}

func TestStreamTotalCap(t *testing.T) {
	// a total of 2 with a generous per-client cap: the third stream is refused for capacity
	streams := limiter.NewConnCap(MaxStreamsPerIP, 2)
	server := httptest.NewServer(newRouter(t, Options{StreamCap: streams}))
	t.Cleanup(server.Close)

	for range 2 {
		s, resp := openStream(t, server)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		s.nextEvent(t)
	}

	_, resp := openStream(t, server)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Equal(t, "30", resp.Header.Get("Retry-After"))
	require.Equal(t, 2, streams.Total(), "a refused stream holds no slot")
}

func TestIncrementLimitsIPv6ByBlock(t *testing.T) {
	router := newRouter(t, Options{})
	require.Equal(t, http.StatusOK, post(t, router, "[2001:db8:1:2::1]:5000").Code)
	require.Equal(t, http.StatusTooManyRequests, post(t, router, "[2001:db8:1:2::ffff]:5000").Code,
		"another address in the same /64 is the same client")
	require.Equal(t, http.StatusOK, post(t, router, "[2001:db8:1:3::1]:5000").Code,
		"a different /64 is a different client")
}
