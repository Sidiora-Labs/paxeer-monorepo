package evmrpc

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	ipRateLimitIdleTimeout     = 10 * time.Minute
	ipRateLimitCleanupInterval = time.Minute
)

type ipRateLimitEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ipRateLimiter struct {
	mu       sync.Mutex
	entries  map[string]*ipRateLimitEntry
	rps      rate.Limit
	burst    int
	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

func newIPRateLimiter(rps float64, burst int) *ipRateLimiter {
	if rps == 0 {
		return nil
	}
	l := &ipRateLimiter{
		entries: make(map[string]*ipRateLimitEntry),
		rps:     rate.Limit(rps),
		burst:   burst,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
	ticker := time.NewTicker(ipRateLimitCleanupInterval)
	go func() {
		defer close(l.doneCh)
		defer ticker.Stop()
		for {
			select {
			case now := <-ticker.C:
				l.evictIdle(now)
			case <-l.stopCh:
				return
			}
		}
	}()
	return l
}

func (l *ipRateLimiter) allow(r *http.Request) bool {
	if l == nil {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	entry := l.entries[host]
	if entry == nil {
		entry = &ipRateLimitEntry{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.entries[host] = entry
	}
	entry.lastSeen = now
	return entry.limiter.AllowN(now, 1)
}

func (l *ipRateLimiter) evictIdle(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for host, entry := range l.entries {
		if now.Sub(entry.lastSeen) >= ipRateLimitIdleTimeout {
			delete(l.entries, host)
		}
	}
}

func (l *ipRateLimiter) stop() {
	if l == nil {
		return
	}
	l.stopOnce.Do(func() { close(l.stopCh) })
	<-l.doneCh
}
