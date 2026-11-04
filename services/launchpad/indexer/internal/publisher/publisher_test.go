package publisher_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/constants"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/publisher"
)

const secret = "test-secret"

type received struct {
	events    []map[string]any
	sigValid  bool
	hadSigHdr bool
}

// recorder is an httptest webhook receiver that verifies the HMAC signature and
// captures the decoded batch. statusFn lets a test script per-attempt responses.
func recorder(t *testing.T, hits *[]received, mu *sync.Mutex, statusFn func(n int) int) *httptest.Server {
	t.Helper()
	var n atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ts := r.Header.Get("X-Sidiora-Timestamp")
		sig := r.Header.Get("X-Sidiora-Signature")
		rec := received{hadSigHdr: sig != ""}
		rec.sigValid = auth.VerifyWebhook(secret, ts, string(body), sig, time.Now(), 0) == nil
		var payload struct {
			Events []map[string]any `json:"events"`
		}
		_ = json.Unmarshal(body, &payload)
		rec.events = payload.Events
		mu.Lock()
		*hits = append(*hits, rec)
		mu.Unlock()

		code := http.StatusOK
		if statusFn != nil {
			code = statusFn(int(n.Add(1)))
		}
		w.WriteHeader(code)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func sampleEvents() []publisher.WebhookEvent {
	return []publisher.WebhookEvent{
		{EventName: "Swap", BlockNumber: 10, BlockTimestamp: 1700000000, TxHash: "0xabc", LogIndex: 0,
			Args: map[string]any{"amountIn": "100"}, TraceID: "0xabc:0"},
		{EventName: "MarketCreated", BlockNumber: 11, BlockTimestamp: 1700000001, TxHash: "0xdef", LogIndex: 1,
			Args: map[string]any{"pool": "0xpool"}},
	}
}

func TestBuildTargets(t *testing.T) {
	t.Parallel()
	for _, secrets := range [][]string{nil, {"s1"}, {"s1", "s2"}} {
		targets, err := publisher.BuildTargets([]string{"https://a", "https://b"}, "def", secrets)
		if !errors.Is(err, publisher.ErrLegacyWebhookRetired) || targets != nil {
			t.Fatalf("configured compatibility targets = %v, %v; want retirement error and no targets", targets, err)
		}
	}
	if _, err := publisher.BuildTargets([]string{"https://a"}, "", nil); !errors.Is(err, publisher.ErrLegacyWebhookRetired) {
		t.Fatalf("unsigned compatibility configuration must also be refused: %v", err)
	}
	targets, err := publisher.BuildTargets(nil, "", nil)
	if err != nil || len(targets) != 0 {
		t.Fatalf("realtime-only configuration = %v, %v", targets, err)
	}
}

func TestPublishBatchDeliversSigned(t *testing.T) {
	t.Parallel()
	var hits []received
	var mu sync.Mutex
	srv := recorder(t, &hits, &mu, nil)
	assertLegacyWebhookRefused(t, publisher.Options{
		Targets: []publisher.Target{{URL: srv.URL, Secret: secret}},
	})
	mu.Lock()
	defer mu.Unlock()
	if len(hits) != 0 {
		t.Fatalf("retired signed webhook delivery reached receiver %d times", len(hits))
	}
}

func TestPublishBatchMultipleTargets(t *testing.T) {
	t.Parallel()
	var h1, h2 []received
	var mu sync.Mutex
	a := recorder(t, &h1, &mu, nil)
	b := recorder(t, &h2, &mu, nil)
	assertLegacyWebhookRefused(t, publisher.Options{Targets: []publisher.Target{
		{URL: a.URL, Secret: secret}, {URL: b.URL, Secret: secret},
	}})
	mu.Lock()
	defer mu.Unlock()
	if len(h1) != 0 || len(h2) != 0 {
		t.Fatalf("retired fan-out reached receivers a=%d b=%d", len(h1), len(h2))
	}
}

func TestPublishBatchNoOpPaths(t *testing.T) {
	t.Parallel()
	var hits []received
	var mu sync.Mutex
	srv := recorder(t, &hits, &mu, nil)
	assertLegacyWebhookRefused(t, publisher.Options{Targets: []publisher.Target{{URL: srv.URL, Secret: secret}}})
	p := publisher.New(publisher.Options{})

	p.PublishBatch(context.Background(), nil)            // empty events
	p.SetBackfillMode(true)                              // suppressed
	p.PublishBatch(context.Background(), sampleEvents()) // dropped (backfill)
	p.WaitForInflight(time.Second)
	mu.Lock()
	n := len(hits)
	mu.Unlock()
	if n != 0 {
		t.Fatalf("expected 0 deliveries (empty + backfill), got %d", n)
	}

	// No targets at all is also a no-op.
	pEmpty := publisher.New(publisher.Options{})
	pEmpty.PublishBatch(context.Background(), sampleEvents())
	if pEmpty.Snapshot().TotalEventsPublished != 0 {
		t.Error("no-target publisher should not count events")
	}
}

func TestDeadLetterOnPermanentFailure(t *testing.T) {
	t.Parallel()
	var hits []received
	var mu sync.Mutex
	srv := recorder(t, &hits, &mu, func(int) int { return http.StatusInternalServerError })
	assertLegacyWebhookRefused(t, publisher.Options{
		Targets: []publisher.Target{{URL: srv.URL, Secret: secret}}, Attempts: 1,
	})
	p := publisher.New(publisher.Options{})
	p.PublishBatch(context.Background(), sampleEvents())
	p.Disconnect()
	if p.DeadLetterCount() != 0 || len(p.DeadLetterQueue()) != 0 || len(p.DrainDeadLetterQueue()) != 0 {
		t.Fatal("retired delivery must not accept work into a volatile dead-letter queue")
	}
	if m := p.Snapshot(); m.TotalFailures != 0 || m.TotalDeadLettered != 0 || m.TotalDeliveries != 0 {
		t.Fatalf("retired delivery metrics = %+v", m)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(hits) != 0 {
		t.Fatalf("retired target received %d requests", len(hits))
	}
}

func TestRetryThenSucceed(t *testing.T) {
	t.Parallel()
	var hits []received
	var mu sync.Mutex
	srv := recorder(t, &hits, &mu, func(n int) int {
		if n == 1 {
			return http.StatusBadGateway
		}
		return http.StatusOK
	})
	assertLegacyWebhookRefused(t, publisher.Options{
		Targets: []publisher.Target{{URL: srv.URL, Secret: secret}}, Attempts: 2,
	})
	mu.Lock()
	defer mu.Unlock()
	if len(hits) != 0 {
		t.Fatalf("retired target received %d retry requests", len(hits))
	}
}

func TestDedupViaRedis(t *testing.T) {
	url := internaltest.NewRedisURL(t)
	rdb, err := sharedredis.NewClient(url)
	if err != nil {
		t.Fatalf("redis client: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })

	var hits []received
	var mu sync.Mutex
	srv := recorder(t, &hits, &mu, nil)
	assertLegacyWebhookRefused(t, publisher.Options{
		Targets: []publisher.Target{{URL: srv.URL, Secret: secret}}, Redis: rdb,
	})
	p := publisher.New(publisher.Options{Redis: rdb})

	ev := sampleEvents()
	p.PublishBatch(context.Background(), ev) // first: realtime fan-out + fingerprint reserved
	p.WaitForInflight(5 * time.Second)
	p.PublishBatch(context.Background(), ev) // identical batch: deduped, not delivered
	p.WaitForInflight(5 * time.Second)

	mu.Lock()
	n := len(hits)
	mu.Unlock()
	if n != 0 {
		t.Fatalf("retired compatibility deliveries = %d, want 0", n)
	}
	if m := p.Snapshot(); m.TotalDeduplicated == 0 {
		t.Errorf("TotalDeduplicated = 0, want >0")
	}
}

// TestPublishBatchFansOutToRedis exercises the realtime dual-delivery path: with
// NO webhook targets (a Redis-only deployment), every event must still be
// PUBLISHed to its indexer:<event> channel as the per-event envelope.
func TestPublishBatchFansOutToRedis(t *testing.T) {
	url := internaltest.NewRedisURL(t)
	rdb, err := sharedredis.NewClient(url)
	if err != nil {
		t.Fatalf("redis client: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })

	ctx := context.Background()
	sub := rdb.Subscribe(ctx, constants.ChannelSwap, constants.ChannelMarketCreated)
	if _, err := sub.Receive(ctx); err != nil { // wait for SUBSCRIBE to register
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Close()
	ch := sub.Channel()

	// No webhook targets — only the Redis path can deliver.
	p := publisher.New(publisher.Options{Redis: rdb})
	p.PublishBatch(ctx, sampleEvents())
	p.WaitForInflight(5 * time.Second)

	got := map[string]map[string]any{}
	timeout := time.After(5 * time.Second)
	for len(got) < 2 {
		select {
		case msg := <-ch:
			var ev map[string]any
			if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
				t.Fatalf("unmarshal redis event: %v", err)
			}
			got[msg.Channel] = ev
		case <-timeout:
			t.Fatalf("timed out; received %d of 2 channel messages", len(got))
		}
	}

	if ev := got[constants.ChannelSwap]; ev["eventName"] != "Swap" || ev["txHash"] != "0xabc" {
		t.Errorf("swap envelope wrong: %+v", ev)
	}
	if ev := got[constants.ChannelMarketCreated]; ev["eventName"] != "MarketCreated" {
		t.Errorf("market_created envelope wrong: %+v", ev)
	}
	if m := p.Snapshot(); m.TotalRedisPublishes != 2 {
		t.Errorf("TotalRedisPublishes = %d, want 2", m.TotalRedisPublishes)
	}
}

func assertLegacyWebhookRefused(t *testing.T, opts publisher.Options) {
	t.Helper()
	var refusal any
	func() {
		defer func() { refusal = recover() }()
		publisher.New(opts)
	}()
	if refusal != publisher.ErrLegacyWebhookRetired {
		t.Fatalf("New with compatibility targets panicked with %v, want ErrLegacyWebhookRetired", refusal)
	}
}
