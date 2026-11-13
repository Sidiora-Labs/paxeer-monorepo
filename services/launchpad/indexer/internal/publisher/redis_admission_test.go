package publisher

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"
)

func TestRealtimeFanoutCountsOnlyEncodedAcceptedEvents(t *testing.T) {
	rdb, err := sharedredis.NewClient(internaltest.NewRedisURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer rdb.Close()
	p := New(Options{Redis: rdb, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	events := []WebhookEvent{{EventName: "Unknown"}, {EventName: "Swap", Args: map[string]any{"invalid": make(chan int)}}, {EventName: "Swap", TxHash: "encoded", Args: map[string]any{"amountIn": "1"}}}
	p.publishRedis(context.Background(), events)
	if snapshot := p.Snapshot(); snapshot.TotalRedisPublishes != 1 || snapshot.TotalRedisFailures != 0 {
		t.Fatalf("accepted realtime metrics %+v", snapshot)
	}
	if err = rdb.Close(); err != nil {
		t.Fatal(err)
	}
	if p.isDuplicate(context.Background(), events) {
		t.Fatal("unavailable realtime cache reported a duplicate")
	}
	p.publishRedis(context.Background(), events[2:])
	if snapshot := p.Snapshot(); snapshot.TotalRedisPublishes != 1 || snapshot.TotalRedisFailures != 1 {
		t.Fatalf("rejected realtime metrics %+v", snapshot)
	}
	p.wg.Add(1)
	started := time.Now()
	p.WaitForInflight(time.Millisecond)
	if elapsed := time.Since(started); elapsed < time.Millisecond || elapsed > time.Second {
		t.Fatalf("publication drain deadline %s", elapsed)
	}
	p.wg.Done()
	p.WaitForInflight(time.Second)
}
