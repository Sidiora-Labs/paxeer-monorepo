package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/jackc/pgx/v5"
)

func canonicalSourceReadinessCase(t *testing.T) (*canonicalSourceReadiness, *atomic.Int64) {
	t.Helper()
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	ddl, err := delivery.PartitionSchemaDDL("indexer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `CREATE SCHEMA recovery`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"60001", "190001", "440000"} {
		body, err := os.ReadFile(filepath.Join("../../../../../migrations", id+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(body)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = pool.Exec(ctx, `INSERT INTO recovery.recovery_control VALUES(true,1,'primary',1,repeat('0',64))`); err != nil {
		t.Fatal(err)
	}
	guard, err := queue.NewEpochGuard("recovery", 1, "primary")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.NewLifecycleStore(pool, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	_, doc, _ := lifecycleDocuments(t)
	endpoint := status1695RPC(t, ctx)
	for id := range doc.Committed.Endpoints {
		doc.Committed.Endpoints[id] = endpoint
	}
	provider, err := source.NewCommittedSource(ctx, doc.Committed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = provider.Close() })
	block, err := provider.BlockAt(ctx, 33963717)
	if err != nil {
		t.Fatal(err)
	}
	doc.InitialCheckpointHeight = int64(block.Height)
	doc.InitialCheckpointHash = strings.ToLower(block.Hash.Hex())
	const stream = "canonical-readiness"
	if err = seedCanonicalWatermark(ctx, pool, stream, doc); err != nil {
		t.Fatal(err)
	}
	clock := new(atomic.Int64)
	clock.Store(time.Now().UnixNano())
	r := newCanonicalSourceReadiness(st, pool, provider, stream, doc, func() time.Time { return time.Unix(0, clock.Load()) }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(r.Close)
	return r, clock
}

func TestCanonicalSourceReadinessRetainsSuccessfulRefreshUntilStale(t *testing.T) {
	r, clock := canonicalSourceReadinessCase(t)
	ctx := context.Background()
	if err := r.Ready(ctx); !errors.Is(err, errCanonicalSourceUnrefreshed) {
		t.Fatalf("before first refresh: %v", err)
	}
	if err := r.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.Ready(ctx); err != nil {
		t.Fatalf("successful refresh: %v", err)
	}
	passedAt := r.lastSuccess
	clock.Add(int64(30 * time.Second))
	if err := r.provider.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.refresh(ctx); err == nil {
		t.Fatal("unavailable source refreshed successfully")
	}
	if !r.lastSuccess.Equal(passedAt) {
		t.Fatal("read error renewed successful refresh time")
	}
	if err := r.Ready(ctx); err != nil {
		t.Fatalf("recent success after read error: %v", err)
	}
	clock.Add(int64(30 * time.Second))
	if err := r.Ready(ctx); err != nil {
		t.Fatalf("success at maximum age: %v", err)
	}
	clock.Add(1)
	if err := r.Ready(ctx); !errors.Is(err, errCanonicalSourceStale) {
		t.Fatalf("expired success: %v", err)
	}
}

func TestCanonicalSourceReadinessInvalidatesAndRecovers(t *testing.T) {
	r, clock := canonicalSourceReadinessCase(t)
	ctx := context.Background()
	if err := r.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		missing bool
		want    error
		text    string
	}{
		{"difference", false, errCanonicalCheckpointMismatch, "indexer: persisted checkpoint differs from canonical source"},
		{"missing complete block", true, errCanonicalCheckpointIncomplete, "indexer: canonical checkpoint has no complete source block or issued initial checkpoint"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clock.Add(int64(time.Second))
			if _, err := r.pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,$2,$3,0,true,true)`, r.stream, r.doc.InitialCheckpointHeight, "0x"+strings.Repeat("b", 64)); err != nil {
				t.Fatal(err)
			}
			doc := r.doc
			if tc.missing {
				r.doc.InitialCheckpointHeight--
				if _, err := r.pool.Exec(ctx, `UPDATE indexer.indexer_block_completeness SET required_decode_complete=false WHERE stream_id=$1`, r.stream); err != nil {
					t.Fatal(err)
				}
			}
			if err := r.refresh(ctx); !errors.Is(err, tc.want) || err.Error() != tc.text {
				t.Fatalf("invalid checkpoint refresh: %v", err)
			}
			if err := r.Ready(ctx); !errors.Is(err, tc.want) || err.Error() != tc.text {
				t.Fatalf("invalid checkpoint readiness: %v", err)
			}
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			if err := r.refresh(canceled); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled refresh: %v", err)
			}
			if err := r.Ready(ctx); !errors.Is(err, tc.want) {
				t.Fatalf("read error cleared invalid checkpoint: %v", err)
			}
			r.doc = doc
			if _, err := r.pool.Exec(ctx, `DELETE FROM indexer.indexer_block_completeness WHERE stream_id=$1`, r.stream); err != nil {
				t.Fatal(err)
			}
			clock.Add(int64(time.Second))
			if err := r.refresh(ctx); err != nil {
				t.Fatal(err)
			}
			if err := r.Ready(ctx); err != nil {
				t.Fatalf("recovered checkpoint: %v", err)
			}
		})
	}
}

func TestCanonicalSourceReadinessMakesNoSourceOrStoreReads(t *testing.T) {
	r, clock := canonicalSourceReadinessCase(t)
	ctx := context.Background()
	if err := r.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.provider.Close(); err != nil {
		t.Fatal(err)
	}
	r.pool.Close()
	for range 5 {
		if err := r.Ready(ctx); err != nil {
			t.Fatalf("retained readiness with closed sources and store: %v", err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := r.Ready(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled readiness: %v", err)
	}
	clock.Add(int64(canonicalSourceMaxAge + time.Nanosecond))
	if err := r.Ready(ctx); !errors.Is(err, errCanonicalSourceStale) {
		t.Fatalf("aged readiness with closed sources and store: %v", err)
	}
}

func canonicalSourceReadinessLock(t *testing.T, r *canonicalSourceReadiness) pgx.Tx {
	t.Helper()
	tx, err := r.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), `LOCK TABLE indexer.indexer_ingest_watermarks IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return tx
}

func TestCanonicalSourceReadinessRefreshBound(t *testing.T) {
	r, clock := canonicalSourceReadinessCase(t)
	ctx := context.Background()
	if err := r.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	passedAt := r.lastSuccess
	canonicalSourceReadinessLock(t, r)
	started := time.Now()
	if err := r.refresh(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bounded refresh: %v", err)
	}
	if elapsed := time.Since(started); elapsed < canonicalSourceRefreshTimeout || elapsed > canonicalSourceRefreshTimeout+2*time.Second {
		t.Fatalf("refresh duration: %v", elapsed)
	}
	clock.Add(int64(canonicalSourceRefreshTimeout))
	if !r.lastSuccess.Equal(passedAt) {
		t.Fatal("bounded refresh renewed successful refresh time")
	}
	if err := r.Ready(ctx); err != nil {
		t.Fatalf("recent success after bounded refresh: %v", err)
	}
}

func TestCanonicalSourceReadinessPeriodicRefreshAndStop(t *testing.T) {
	r, clock := canonicalSourceReadinessCase(t)
	ctx := context.Background()
	r.Start(ctx)
	deadline := time.Now().Add(5 * time.Second)
	for r.Ready(ctx) != nil {
		if time.Now().After(deadline) {
			t.Fatal("initial background refresh did not complete")
		}
		time.Sleep(time.Millisecond)
	}
	r.mu.RLock()
	first := r.lastSuccess
	r.mu.RUnlock()
	clock.Add(int64(canonicalSourceRefreshInterval))
	deadline = time.Now().Add(canonicalSourceRefreshInterval + 5*time.Second)
	for {
		r.mu.RLock()
		refreshed := r.lastSuccess.After(first)
		r.mu.RUnlock()
		if refreshed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("periodic refresh did not complete")
		}
		time.Sleep(time.Millisecond)
	}
	lock := canonicalSourceReadinessLock(t, r)
	deadline = time.Now().Add(canonicalSourceRefreshInterval + 5*time.Second)
	for {
		var waiting int
		if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT raw_complete_height,typed_complete_height%'`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 1 {
			t.Fatalf("overlapping refreshes: %d", waiting)
		}
		if waiting == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("periodic refresh did not read the head")
		}
		time.Sleep(time.Millisecond)
	}
	started := time.Now()
	r.Close()
	if time.Since(started) > time.Second {
		t.Fatal("stop did not cancel in-flight refresh")
	}
	select {
	case <-r.done:
	default:
		t.Fatal("stop returned before refresh ended")
	}
	if err := lock.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	r.mu.RLock()
	last := r.lastSuccess
	r.mu.RUnlock()
	clock.Add(int64(canonicalSourceRefreshInterval))
	time.Sleep(canonicalSourceRefreshInterval + 10*time.Millisecond)
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.lastSuccess.Equal(last) {
		t.Fatal("refresh continued after stop")
	}
}

func TestCanonicalSourceReadinessCanceledRefreshPreservesCommittedHead(t *testing.T) {
	r, clock := canonicalSourceReadinessCase(t)
	ctx := context.Background()
	var endpoint string
	for _, value := range r.doc.Committed.Endpoints {
		endpoint = value
		break
	}
	target, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	forward := httputil.NewSingleHostReverseProxy(target)
	var blocked atomic.Bool
	entered := make(chan struct{}, 1)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if blocked.Load() {
			if _, err := io.Copy(io.Discard, req.Body); err != nil {
				t.Error(err)
				return
			}
			select {
			case entered <- struct{}{}:
			default:
			}
			<-req.Context().Done()
			return
		}
		forward.ServeHTTP(w, req)
	}))
	t.Cleanup(proxy.Close)
	options := r.doc.Committed
	options.Endpoints = make(map[string]string, len(options.Endpoints))
	for id := range r.doc.Committed.Endpoints {
		options.Endpoints[id] = proxy.URL
	}
	provider, err := source.NewCommittedSource(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = provider.Close() })
	if err := verifyCanonicalRuntimeCheckpoint(ctx, r.pool, provider, r.stream, r.doc.InitialCheckpointHeight, r.doc); err != nil {
		t.Fatal(err)
	}
	r.provider = provider
	head := &canonicalRuntimeHead{source: provider, store: r.store, stream: r.stream, height: new(atomic.Uint64)}
	if _, err := head.Head(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	passedAt := r.lastSuccess
	clock.Add(int64(time.Second))
	blocked.Store(true)
	r.Start(ctx)
	t.Cleanup(r.Close)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("refresh did not reach source transport")
	}
	r.Close()
	var integrity string
	if err := r.pool.QueryRow(ctx, `SELECT integrity FROM indexer.indexer_committed_heads WHERE stream_id=$1 ORDER BY height DESC LIMIT 1`, r.stream).Scan(&integrity); err != nil || integrity != "healthy" {
		t.Fatalf("committed head after canceled refresh: %s %v", integrity, err)
	}
	if !r.lastSuccess.Equal(passedAt) {
		t.Fatal("canceled source read renewed successful refresh time")
	}
	if err := r.Ready(ctx); err != nil {
		t.Fatalf("recent success after canceled source read: %v", err)
	}
	blocked.Store(false)
	if err := r.refresh(ctx); err != nil {
		t.Fatalf("refresh after source cancellation: %v", err)
	}
}

func TestCanonicalSourceReadinessRuntimeCases(t *testing.T) {
	t.Run("checkpoint admission", TestRuntimeCheckpointChecksTheSourceAndQuarantinesUnavailableHeads)
	t.Run("production application", TestIndexerStatus1695)
}
