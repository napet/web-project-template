package limiter

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newClock() *fakeClock { return &fakeClock{now: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)} }

func TestWindowRefusesWithinPeriod(t *testing.T) {
	clock := newClock()
	w := NewWindow(10*time.Minute, clock.Now)

	ok, _ := w.Allow("1.1.1.1")
	require.True(t, ok)

	clock.Advance(time.Minute)
	ok, retry := w.Allow("1.1.1.1")
	require.False(t, ok)
	require.Equal(t, 9*time.Minute, retry)
}

func TestWindowAllowsAfterPeriod(t *testing.T) {
	clock := newClock()
	w := NewWindow(10*time.Minute, clock.Now)

	ok, _ := w.Allow("1.1.1.1")
	require.True(t, ok)
	clock.Advance(10*time.Minute + time.Second)
	ok, _ = w.Allow("1.1.1.1")
	require.True(t, ok)
}

func TestWindowKeysAreIndependent(t *testing.T) {
	w := NewWindow(10*time.Minute, newClock().Now)
	ok, _ := w.Allow("1.1.1.1")
	require.True(t, ok)
	ok, _ = w.Allow("2.2.2.2")
	require.True(t, ok)
}

func TestWindowRelease(t *testing.T) {
	w := NewWindow(10*time.Minute, newClock().Now)
	w.Allow("1.1.1.1")
	w.Release("1.1.1.1")
	ok, _ := w.Allow("1.1.1.1")
	require.True(t, ok)
}

func TestWindowSweepRemovesExpired(t *testing.T) {
	clock := newClock()
	w := NewWindow(10*time.Minute, clock.Now)
	w.Allow("old")
	clock.Advance(9 * time.Minute)
	w.Allow("new")
	clock.Advance(2 * time.Minute)

	require.Equal(t, 1, w.Sweep())
	require.Equal(t, 1, w.Len())
}

func TestConnCapPerKey(t *testing.T) {
	c := NewConnCap(5, 100)
	for range 5 {
		require.Equal(t, Acquired, c.Acquire("1.1.1.1"))
	}
	require.Equal(t, KeyFull, c.Acquire("1.1.1.1"))
	require.Equal(t, Acquired, c.Acquire("2.2.2.2"))

	c.Release("1.1.1.1")
	require.Equal(t, 4, c.Open("1.1.1.1"))
	require.Equal(t, Acquired, c.Acquire("1.1.1.1"))

	for range 5 {
		c.Release("1.1.1.1")
	}
	require.Equal(t, 0, c.Open("1.1.1.1"))
	require.Equal(t, 1, c.Total())
}

func TestConnCapTotal(t *testing.T) {
	c := NewConnCap(2, 3)
	require.Equal(t, Acquired, c.Acquire("a"))
	require.Equal(t, Acquired, c.Acquire("b"))
	require.Equal(t, Acquired, c.Acquire("c"))
	require.Equal(t, TotalFull, c.Acquire("d"), "a fresh key is refused once the total is reached")
	require.Equal(t, 0, c.Open("d"))

	c.Release("a")
	require.Equal(t, Acquired, c.Acquire("d"))
	require.Equal(t, 3, c.Total())
}

func TestConnCapReleaseOfUnknownKeyIsIgnored(t *testing.T) {
	c := NewConnCap(1, 1)
	c.Release("never-acquired")
	require.Equal(t, 0, c.Total())
	require.Equal(t, Acquired, c.Acquire("a"))
	require.Equal(t, TotalFull, c.Acquire("b"))
}

func TestClientKey(t *testing.T) {
	cases := map[string]string{
		"203.0.113.7":                      "203.0.113.7",
		"::ffff:203.0.113.7":               "203.0.113.7",
		"2001:db8:1:2:aaaa:bbbb:cccc:dddd": "2001:db8:1:2::/64",
		"2001:db8:1:2::1":                  "2001:db8:1:2::/64",
		"fe80::1%eth0":                     "fe80::/64",
		"not-an-ip":                        "not-an-ip",
	}
	for in, want := range cases {
		require.Equal(t, want, ClientKey(in), in)
	}
	require.Equal(t, ClientKey("2001:db8:1:2::1"), ClientKey("2001:db8:1:2:ffff::9"),
		"addresses in one /64 share a key")
	require.NotEqual(t, ClientKey("2001:db8:1:2::1"), ClientKey("2001:db8:1:3::1"))
}
