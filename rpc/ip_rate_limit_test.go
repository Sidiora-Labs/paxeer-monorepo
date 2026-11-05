package evmrpc

import (
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIPRateLimiterRefusesAboveBurst(t *testing.T) {
	t.Run("burst", func(t *testing.T) {
		l := newIPRateLimiter(0.001, 3)
		t.Cleanup(l.stop)
		r := &http.Request{RemoteAddr: "192.0.2.1:1234"}
		for i := 0; i < 3; i++ {
			require.True(t, l.allow(r), "request %d within burst", i+1)
		}
		require.False(t, l.allow(r), "request beyond burst")
		require.False(t, l.allow(r), "refused requests must not reset the budget")
	})

	t.Run("refill", func(t *testing.T) {
		l := newIPRateLimiter(2, 1)
		t.Cleanup(l.stop)
		r := &http.Request{RemoteAddr: "192.0.2.1:1234"}
		require.True(t, l.allow(r))
		require.False(t, l.allow(r))
		require.Eventually(t, func() bool { return l.allow(r) }, 2*time.Second, 10*time.Millisecond)
	})

	t.Run("concurrent requests share the burst", func(t *testing.T) {
		l := newIPRateLimiter(0.001, 5)
		t.Cleanup(l.stop)
		r := &http.Request{RemoteAddr: "192.0.2.1:1234"}
		var admitted atomic.Int32
		var workers sync.WaitGroup
		for i := 0; i < 32; i++ {
			workers.Add(1)
			go func() {
				defer workers.Done()
				if l.allow(r) {
					admitted.Add(1)
				}
			}()
		}
		workers.Wait()
		require.Equal(t, int32(5), admitted.Load())
	})

	t.Run("idle eviction and stop", func(t *testing.T) {
		l := newIPRateLimiter(0.001, 1)
		t.Cleanup(l.stop)
		require.True(t, l.allow(&http.Request{RemoteAddr: "192.0.2.1:1234"}))
		require.True(t, l.allow(&http.Request{RemoteAddr: "192.0.2.2:1234"}))
		l.mu.Lock()
		now := time.Now()
		l.entries["192.0.2.1"].lastSeen = now.Add(-10 * time.Minute)
		l.entries["192.0.2.2"].lastSeen = now.Add(-9 * time.Minute)
		l.mu.Unlock()
		l.evictIdle(now)
		l.mu.Lock()
		_, idleExists := l.entries["192.0.2.1"]
		_, activeExists := l.entries["192.0.2.2"]
		l.mu.Unlock()
		require.False(t, idleExists)
		require.True(t, activeExists)
		require.True(t, l.allow(&http.Request{RemoteAddr: "192.0.2.1:5678"}))
		require.False(t, l.allow(&http.Request{RemoteAddr: "192.0.2.2:5678"}))
		l.stop()
		select {
		case <-l.doneCh:
		default:
			t.Fatal("stop returned before cleanup exited")
		}
		l.stop()
	})
}

func TestIPRateLimiterKeysByRemoteIP(t *testing.T) {
	for _, addresses := range [][3]string{
		{"192.0.2.1:1234", "192.0.2.1:5678", "192.0.2.2:1234"},
		{"[2001:db8::1]:1234", "[2001:db8::1]:5678", "[2001:db8::2]:1234"},
	} {
		t.Run(addresses[0], func(t *testing.T) {
			l := newIPRateLimiter(0.001, 1)
			t.Cleanup(l.stop)
			require.True(t, l.allow(&http.Request{RemoteAddr: addresses[0]}))
			require.False(t, l.allow(&http.Request{RemoteAddr: addresses[1]}), "changing ports must not reset the budget")
			require.True(t, l.allow(&http.Request{RemoteAddr: addresses[2]}), "another host has its own budget")
			require.False(t, l.allow(&http.Request{
				RemoteAddr: addresses[0],
				Header: http.Header{
					"X-Forwarded-For": {"198.51.100.1"},
					"X-Real-Ip":       {"198.51.100.2"},
				},
			}), "forwarded headers must not change the key")
		})
	}

	t.Run("disabled", func(t *testing.T) {
		l := newIPRateLimiter(0, 0)
		require.Nil(t, l)
		for i := 0; i < 10; i++ {
			require.True(t, l.allow(&http.Request{RemoteAddr: "192.0.2.1:1234"}))
		}
		l.stop()
	})

	t.Run("malformed remote address", func(t *testing.T) {
		l := newIPRateLimiter(0.001, 1)
		t.Cleanup(l.stop)
		for _, address := range []string{"", "192.0.2.1", ":1234"} {
			require.False(t, l.allow(&http.Request{RemoteAddr: address}))
		}
	})
}
