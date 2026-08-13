package events

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDuplicateCache_StopStopsEviction(t *testing.T) {
	cache := NewDuplicateCache()
	cache.Start()

	var evictions int32

	cache.BeaconETHV1EventsHead.OnEviction(func(_ context.Context, reason ttlcache.EvictionReason, _ *ttlcache.Item[string, time.Time]) {
		if reason == ttlcache.EvictionReasonExpired {
			atomic.AddInt32(&evictions, 1)
		}
	})

	cache.BeaconETHV1EventsHead.Set("key-1", time.Now(), 20*time.Millisecond)

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&evictions) > 0
	}, 2*time.Second, 20*time.Millisecond, "eviction must happen while the cache is running")

	cache.Stop()

	atomic.StoreInt32(&evictions, 0)
	cache.BeaconETHV1EventsHead.Set("key-2", time.Now(), 20*time.Millisecond)

	// Give it a fair chance to (incorrectly) evict if Stop didn't actually stop the janitor.
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, int32(0), atomic.LoadInt32(&evictions),
		"no eviction should happen after Stop - the janitor goroutine must have exited")
}

func TestDuplicateCache_StopWithoutStartIsSafe(t *testing.T) {
	cache := NewDuplicateCache()

	assert.NotPanics(t, func() {
		cache.Stop()
	})
}

func TestDuplicateCache_StopIsIdempotent(t *testing.T) {
	cache := NewDuplicateCache()
	cache.Start()

	assert.NotPanics(t, func() {
		cache.Stop()
		cache.Stop()
	})
}
