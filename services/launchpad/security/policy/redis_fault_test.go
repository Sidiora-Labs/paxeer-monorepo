package policy

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRealRedisAtomicExpiryCrashRecovery(t *testing.T) {
	const caseTimeout = 20 * time.Second
	directory := t.TempDir()
	socket := filepath.Join(directory, "redis.sock")
	ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
	defer cancel()
	log, err := os.Create(filepath.Join(directory, "redis.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	start := func() *exec.Cmd {
		t.Helper()
		command := exec.Command("redis-server", "--port", "0", "--unixsocket", socket, "--unixsocketperm", "700", "--dir", directory, "--appendonly", "yes", "--appendfsync", "always", "--save", "")
		command.Stdout = log
		command.Stderr = log
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		return command
	}
	client := func(timeout time.Duration) *redis.Client {
		return redis.NewClient(&redis.Options{
			Network: "unix", Addr: socket,
			DialTimeout: timeout, ReadTimeout: timeout, WriteTimeout: timeout, PoolTimeout: timeout,
			ContextTimeoutEnabled: true, MaxRetries: -1, DialerRetries: 1,
		})
	}
	command := start()
	live := true
	t.Cleanup(func() {
		if live {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})
	a, b := client(caseTimeout), client(caseTimeout)
	defer a.Close()
	defer b.Close()
	wait := func() {
		t.Helper()
		for {
			if a.Ping(ctx).Err() == nil {
				return
			}
			select {
			case <-ctx.Done():
				t.Fatal("real Redis startup", ctx.Err())
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
	wait()
	var admitted atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 61; i++ {
		wg.Add(1)
		selected := a
		if i%2 == 0 {
			selected = b
		}
		go func(r *redis.Client) {
			defer wg.Done()
			ok, _, err := Take(ctx, r, "l45-phase2", "account-write", "one-principal", 1)
			if err != nil {
				t.Error(err)
			}
			if ok {
				admitted.Add(1)
			}
		}(selected)
	}
	wg.Wait()
	if admitted.Load() != 10 {
		t.Fatal("replica quota multiplied", admitted.Load())
	}
	keys, err := a.Keys(ctx, "l45-phase2:budget:*").Result()
	if err != nil || len(keys) != 1 {
		t.Fatal(keys, err)
	}
	if err = command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()
	live = false
	// Outage checks keep a short timeout to assert fail-closed admission while Redis is stopped.
	outage := client(100 * time.Millisecond)
	defer outage.Close()
	if allowed, _, err := Take(ctx, outage, "l45-phase2", "account-write", "one-principal", 1); err == nil || allowed {
		t.Fatal("outage admitted", allowed, err)
	}
	for i := 0; i < 2000; i++ {
		if allowed, _, err := Take(ctx, outage, "l45-phase3", "account-write", "principal-"+strconv.Itoa(i), 1); err == nil || allowed {
			t.Fatal("mutation admitted during Redis loss", i, allowed, err)
		}
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	lease := EdgeLease{Origin: "origin-1", PolicyVersion: "v3", ID: "outage-1", Issued: now, Expires: now.Add(time.Minute), PerSecond: 100}
	wire, err := json.Marshal(lease)
	if err != nil {
		t.Fatal(err)
	}
	edge, err := NewEdgeAdmission(lease, ed25519.Sign(key, wire), pub, "origin-1", now)
	if err != nil {
		t.Fatal(err)
	}
	var cachedReads int
	handler, err := Admission(AdmissionOptions{Redis: outage, Namespace: "l45-phase3", Class: "public-read", Cost: 1, CachedPublicRead: true, Edge: edge}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { cachedReads++; w.WriteHeader(204) }))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		record := httptest.NewRecorder()
		handler.ServeHTTP(record, httptest.NewRequest("GET", "/cached", nil).WithContext(ctx))
		if record.Code != 204 {
			t.Fatal(record.Code)
		}
	}
	if cachedReads != 10 {
		t.Fatal("cached read lease unavailable", cachedReads)
	}
	uncached, err := Admission(AdmissionOptions{Redis: outage, Namespace: "l45-phase3", Class: "public-read", Cost: 1, Edge: edge}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("uncached outage read admitted") }))
	if err != nil {
		t.Fatal(err)
	}
	record := httptest.NewRecorder()
	uncached.ServeHTTP(record, httptest.NewRequest("GET", "/uncached", nil).WithContext(ctx))
	if record.Code != 503 {
		t.Fatal(record.Code)
	}
	command = start()
	live = true
	wait()
	ttl, err := a.PTTL(ctx, keys[0]).Result()
	if err != nil || ttl <= 0 || ttl > 120*time.Second {
		t.Fatal("counter expiration lost", ttl, err)
	}
}
