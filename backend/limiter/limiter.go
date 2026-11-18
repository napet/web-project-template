// Package limiter holds the in-memory, per-client limits used by the API.
// State lives only in process memory and is never persisted.
package limiter

import (
	"context"
	"sync"
	"time"
)

// Window allows one action per key per period ("once per window").
type Window struct {
	mu     sync.Mutex
	period time.Duration
	last   map[string]time.Time
	now    func() time.Time
}

// NewWindow creates a Window. A nil clock defaults to time.Now.
func NewWindow(period time.Duration, now func() time.Time) *Window {
	if now == nil {
		now = time.Now
	}
	return &Window{period: period, last: make(map[string]time.Time), now: now}
}

// Allow records an action for key if its window has elapsed. When the action
// is refused it returns false and the time left until the window reopens.
func (w *Window) Allow(key string) (bool, time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := w.now()
	if last, ok := w.last[key]; ok {
		if elapsed := now.Sub(last); elapsed < w.period {
			return false, w.period - elapsed
		}
	}
	w.last[key] = now
	return true, 0
}

// Release forgets the last action for key, e.g. when the action failed.
func (w *Window) Release(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.last, key)
}

// Sweep drops expired entries and returns how many were removed.
func (w *Window) Sweep() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := w.now()
	removed := 0
	for key, last := range w.last {
		if now.Sub(last) >= w.period {
			delete(w.last, key)
			removed++
		}
	}
	return removed
}

// Len returns the number of tracked keys.
func (w *Window) Len() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.last)
}

// RunSweeper calls Sweep every interval until ctx is done.
func (w *Window) RunSweeper(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.Sweep()
		}
	}
}

// ConnCap limits concurrent connections per key and in total.
type ConnCap struct {
	mu      sync.Mutex
	perKey  int
	total   int
	open    map[string]int
	openSum int
}

// AcquireResult says whether Acquire reserved a slot, and if not, which limit was hit.
type AcquireResult int

const (
	Acquired AcquireResult = iota
	// KeyFull means this key already holds its maximum number of connections.
	KeyFull
	// TotalFull means the process holds its maximum number of connections overall.
	TotalFull
)

// NewConnCap allows perKey connections per key and total connections overall.
func NewConnCap(perKey, total int) *ConnCap {
	return &ConnCap{perKey: perKey, total: total, open: make(map[string]int)}
}

// Acquire reserves a connection slot for key. Only an Acquired result must be
// paired with a Release.
func (c *ConnCap) Acquire(key string) AcquireResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.open[key] >= c.perKey {
		return KeyFull
	}
	if c.openSum >= c.total {
		return TotalFull
	}
	c.open[key]++
	c.openSum++
	return Acquired
}

// Release frees a slot previously reserved with Acquire.
func (c *ConnCap) Release(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n, ok := c.open[key]
	if !ok {
		return
	}
	c.openSum--
	if n <= 1 {
		delete(c.open, key)
		return
	}
	c.open[key] = n - 1
}

// Open returns the number of slots currently held for key.
func (c *ConnCap) Open(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.open[key]
}

// Total returns the number of slots currently held across all keys.
func (c *ConnCap) Total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.openSum
}
