// Package countcache keeps the latest counter value in memory so that reads
// do not reach the database on every request.
//
// This API is the only writer of the counter, so after the first read the
// cache is kept current by its own increments; the periodic refresh only
// picks up changes made outside the API (for example by hand in the database).
// It is correct for a single instance; see backend/README.md.
package countcache

import (
	"context"
	"sync"
	"time"
)

// Store reads and increments a counter.
type Store interface {
	Get(ctx context.Context) (int64, error)
	Increment(ctx context.Context) (int64, error)
}

// Cached wraps a Store and serves Get from memory.
type Cached struct {
	inner Store
	ttl   time.Duration
	now   func() time.Time

	mu      sync.Mutex
	value   int64
	fetched time.Time
	valid   bool
}

// New wraps inner, re-reading it at most once per ttl. A nil clock defaults to time.Now.
func New(inner Store, ttl time.Duration, now func() time.Time) *Cached {
	if now == nil {
		now = time.Now
	}
	return &Cached{inner: inner, ttl: ttl, now: now}
}

// Get returns the cached value, refreshing it from the store when it is older
// than the ttl. Concurrent callers share one refresh. If a refresh fails and a
// value was read before, the last value is returned instead of the error.
func (c *Cached) Get(ctx context.Context) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.valid && c.now().Sub(c.fetched) < c.ttl {
		return c.value, nil
	}
	value, err := c.inner.Get(ctx)
	if err != nil {
		if c.valid {
			return c.value, nil
		}
		return 0, err
	}
	c.store(value)
	return c.value, nil
}

// Increment increments the store and records the new value.
func (c *Cached) Increment(ctx context.Context) (int64, error) {
	value, err := c.inner.Increment(ctx)
	if err != nil {
		return 0, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store(value)
	return value, nil
}

// store records value. The counter only grows, so a result that arrives late
// from a concurrent call never moves the cached value backwards.
func (c *Cached) store(value int64) {
	if !c.valid || value > c.value {
		c.value = value
	}
	c.fetched = c.now()
	c.valid = true
}
