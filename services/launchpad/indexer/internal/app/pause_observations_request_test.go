package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/ethereum/go-ethereum/core/types"
)

func pauseRequest(t *testing.T, server *httptest.Server, block *types.Receipt, cutoff int64, hash string) (int, []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pauseURL(server.URL, block.BlockNumber.Uint64(), cutoff, hash), nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("pause evidence must not be cached")
	}
	return response.StatusCode, raw
}

func TestPauseObservationRequestExtendsSnapshot(t *testing.T) {
	c := newPauseChain(t)
	address, pool, created := c.create(t, "Request pause")
	c.retain(t, created)
	c.coverRaw(address)
	calls := pauseBoundedClient(t, c)
	c.refresh(t)
	previous := c.observation.snapshot.Load()
	paused := c.transact(t, pool, "pause")
	unpaused := c.transact(t, pool, "unpause")
	c.retain(t, unpaused)
	server := httptest.NewServer(c.observation)
	defer server.Close()
	cutoff := time.Now().Unix() + 1
	hash := strings.ToLower(paused.BlockHash.Hex())
	status, raw := pauseRequest(t, server, paused, cutoff, hash)
	if status != http.StatusOK {
		t.Fatalf("complete block response: %d %s", status, raw)
	}
	out, err := decodePauseConsumer(&http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(raw))}, paused.BlockNumber.Uint64(), cutoff, hash)
	if err != nil {
		t.Fatal(err)
	}
	assertPauseState(t, out, address, true)
	extended := c.observation.snapshot.Load()
	if extended == previous || extended.base != previous.base || extended.head.height != unpaused.BlockNumber.Int64() || len(extended.events) != 2 || calls() != 1 {
		t.Fatal("request must extend retained events to the stream head using the existing base")
	}
	periodic := &pauseObservations{pool: c.observation.pool, rpc: c.observation.rpc, stream: c.observation.stream, epochs: c.observation.epochs}
	periodic.snapshot.Store(previous)
	if err = periodic.refresh(c.ctx); err != nil {
		t.Fatal(err)
	}
	periodicServer := httptest.NewServer(periodic)
	defer periodicServer.Close()
	periodicStatus, periodicRaw := pauseRequest(t, periodicServer, paused, cutoff, hash)
	if periodicStatus != http.StatusOK || !bytes.Equal(raw, periodicRaw) {
		t.Fatalf("request and periodic evidence differ: %s %s", raw, periodicRaw)
	}
	age := *extended.base
	age.built = time.Now().Add(-2 * pauseBaseCycle)
	aged := *extended
	aged.base = &age
	c.observation.snapshot.Store(&aged)
	pausedAgain := c.transact(t, pool, "pause")
	c.retain(t, pausedAgain)
	assertPauseState(t, readPauseResponse(t, server, pausedAgain), address, true)
	if snapshot := c.observation.snapshot.Load(); snapshot.base != &age || snapshot.head.height != pausedAgain.BlockNumber.Int64() || calls() != 1 {
		t.Fatal("request extension must retain an aged base without contract reads")
	}
}

func TestPauseObservationRequestAtOrBelowSnapshotHead(t *testing.T) {
	c := newPauseChain(t)
	address, pool, created := c.create(t, "Covered request")
	c.retain(t, created)
	c.coverRaw(address)
	c.refresh(t)
	paused := c.transact(t, pool, "pause")
	c.retain(t, paused)
	c.refresh(t)
	previous := c.observation.snapshot.Load()
	unpaused := c.transact(t, pool, "unpause")
	c.retain(t, unpaused)
	server := httptest.NewServer(c.observation)
	defer server.Close()
	c.observation.mu.Lock()
	defer c.observation.mu.Unlock()
	assertPauseState(t, readPauseResponse(t, server, created), address, false)
	assertPauseState(t, readPauseResponse(t, server, paused), address, true)
	if c.observation.snapshot.Load() != previous {
		t.Fatal("covered requests must leave the snapshot unchanged")
	}
}

func TestPauseObservationRequestConcurrentExtension(t *testing.T) {
	c := newPauseChain(t)
	address, pool, created := c.create(t, "Concurrent request")
	c.retain(t, created)
	c.coverRaw()
	epochs := []receipt.RPCCodeEpoch{pauseCreatedEpoch(t, c, address, created)}
	c.observation.discoveredEpochs = func() []receipt.RPCCodeEpoch { return epochs }
	calls := pauseBoundedClient(t, c)
	c.refresh(t)
	base := c.observation.snapshot.Load().base
	paused := c.transact(t, pool, "pause")
	c.retain(t, paused)
	entered := make(chan struct{})
	release := make(chan struct{})
	var extensions atomic.Int64
	c.observation.discoveredEpochs = func() []receipt.RPCCodeEpoch {
		if extensions.Add(1) == 1 {
			close(entered)
			<-release
		}
		return epochs
	}
	var requests atomic.Int64
	second := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 2 {
			close(second)
		}
		c.observation.ServeHTTP(w, r)
	}))
	defer server.Close()
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	type result struct {
		status int
		raw    []byte
		err    error
	}
	results := make(chan result, 2)
	cutoff := time.Now().Unix() + 1
	url := pauseURL(server.URL, paused.BlockNumber.Uint64(), cutoff, strings.ToLower(paused.BlockHash.Hex()))
	request := func() {
		ctx, cancel := context.WithTimeout(c.ctx, 2*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			results <- result{err: err}
			return
		}
		response, err := server.Client().Do(req)
		if err != nil {
			results <- result{err: err}
			return
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		results <- result{status: response.StatusCode, raw: raw, err: err}
	}
	go request()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("request did not enter the extension")
	}
	go request()
	select {
	case <-second:
	case <-time.After(time.Second):
		t.Fatal("second request did not reach the handler")
	}
	close(release)
	released = true
	var first []byte
	for range 2 {
		got := <-results
		if got.err != nil || got.status != http.StatusOK {
			t.Fatalf("concurrent evidence: status=%d error=%v body=%s", got.status, got.err, got.raw)
		}
		if first == nil {
			first = got.raw
		} else if !bytes.Equal(first, got.raw) {
			t.Fatal("concurrent requests must receive identical evidence")
		}
	}
	var out pauseConsumerEvidence
	if err := json.Unmarshal(first, &out); err != nil {
		t.Fatal(err)
	}
	assertPauseState(t, out, address, true)
	if extensions.Load() != 1 || calls() != 1 || c.observation.snapshot.Load().base != base {
		t.Fatal("concurrent requests must extend once using the same base")
	}
}

func TestPauseObservationRequestRefusals(t *testing.T) {
	c := newPauseChain(t)
	address, pool, created := c.create(t, "Request admission")
	c.retain(t, created)
	c.coverRaw(address)
	calls := pauseBoundedClient(t, c)
	server := httptest.NewServer(c.observation)
	defer server.Close()
	t.Run("NoBase", func(t *testing.T) {
		checkPauseUnavailable(t, server, created)
		if c.observation.snapshot.Load() != nil || calls() != 0 {
			t.Fatal("request must not construct a base")
		}
	})
	c.refresh(t)
	previous := c.observation.snapshot.Load()
	paused := c.transact(t, pool, "pause")
	c.retain(t, paused)
	t.Run("DifferentHash", func(t *testing.T) {
		status, raw := pauseRequest(t, server, paused, time.Now().Unix()+1, "0x"+strings.Repeat("b", 64))
		if status != http.StatusConflict || string(raw) != "cutoff is not a complete canonical block\n" || c.observation.snapshot.Load() != previous {
			t.Fatalf("noncanonical request response: %d %s", status, raw)
		}
	})
	t.Run("CoverageRequiresBase", func(t *testing.T) {
		c.coverRaw()
		defer c.coverRaw(address)
		checkPauseUnavailable(t, server, paused)
		if c.observation.snapshot.Load() != previous || calls() != 1 {
			t.Fatal("missing coverage must not extend or rebuild the base")
		}
	})
	t.Run("LockBound", func(t *testing.T) {
		c.observation.mu.Lock()
		defer c.observation.mu.Unlock()
		started := time.Now()
		status, raw := pauseRequest(t, server, paused, time.Now().Unix()+1, strings.ToLower(paused.BlockHash.Hex()))
		if elapsed := time.Since(started); elapsed >= 2*time.Second {
			t.Fatalf("lock-bound response took %s", elapsed)
		}
		if status != http.StatusServiceUnavailable || string(raw) != "pause observation unavailable\n" || c.observation.snapshot.Load() != previous || calls() != 1 {
			t.Fatalf("lock-bound response: %d %s", status, raw)
		}
	})
}
