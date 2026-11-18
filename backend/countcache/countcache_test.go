package countcache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	mu    sync.Mutex
	value int64
	gets  int
	err   error
}

func (s *fakeStore) Get(context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gets++
	if s.err != nil {
		return 0, s.err
	}
	return s.value, nil
}

func (s *fakeStore) Increment(context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return 0, s.err
	}
	s.value++
	return s.value, nil
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

var errDown = errors.New("db down")

func TestRepeatedReadsHitTheStoreOnce(t *testing.T) {
	store := &fakeStore{value: 41}
	cache := New(store, time.Minute, (&clock{}).now)

	for range 100 {
		v, err := cache.Get(context.Background())
		require.NoError(t, err)
		require.Equal(t, int64(41), v)
	}
	require.Equal(t, 1, store.gets)
}

func TestConcurrentReadsShareOneRefresh(t *testing.T) {
	store := &fakeStore{value: 5}
	cache := New(store, time.Minute, nil)

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			v, err := cache.Get(context.Background())
			require.NoError(t, err)
			require.Equal(t, int64(5), v)
		})
	}
	wg.Wait()
	require.Equal(t, 1, store.gets)
}

func TestIncrementUpdatesTheCachedValue(t *testing.T) {
	store := &fakeStore{value: 1}
	cache := New(store, time.Minute, (&clock{}).now)

	_, _ = cache.Get(context.Background())
	v, err := cache.Increment(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(2), v)

	v, _ = cache.Get(context.Background())
	require.Equal(t, int64(2), v)
	require.Equal(t, 1, store.gets, "the increment result is reused, no extra read")
}

func TestRefreshAfterTTL(t *testing.T) {
	clk := &clock{}
	store := &fakeStore{value: 1}
	cache := New(store, time.Minute, clk.now)

	_, _ = cache.Get(context.Background())
	store.value = 10 // changed outside the API
	clk.advance(59 * time.Second)
	v, _ := cache.Get(context.Background())
	require.Equal(t, int64(1), v)

	clk.advance(2 * time.Second)
	v, _ = cache.Get(context.Background())
	require.Equal(t, int64(10), v)
	require.Equal(t, 2, store.gets)
}

func TestStaleValueServedWhenRefreshFails(t *testing.T) {
	clk := &clock{}
	store := &fakeStore{value: 7}
	cache := New(store, time.Minute, clk.now)
	_, _ = cache.Get(context.Background())

	store.err = errDown
	clk.advance(2 * time.Minute)
	v, err := cache.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(7), v)
}

func TestErrorWithoutAnyValue(t *testing.T) {
	cache := New(&fakeStore{err: errDown}, time.Minute, nil)
	_, err := cache.Get(context.Background())
	require.ErrorIs(t, err, errDown)
}

func TestFailedIncrementLeavesTheCacheAlone(t *testing.T) {
	store := &fakeStore{value: 3}
	cache := New(store, time.Minute, (&clock{}).now)
	_, _ = cache.Get(context.Background())

	store.err = errDown
	_, err := cache.Increment(context.Background())
	require.ErrorIs(t, err, errDown)
	store.err = nil
	v, _ := cache.Get(context.Background())
	require.Equal(t, int64(3), v)
}

func TestLateSmallerResultDoesNotMoveBackwards(t *testing.T) {
	cache := New(&fakeStore{}, time.Minute, (&clock{}).now)
	cache.mu.Lock()
	cache.store(9)
	cache.store(8) // e.g. an older concurrent increment finishing last
	cache.mu.Unlock()
	v, _ := cache.Get(context.Background())
	require.Equal(t, int64(9), v)
}
