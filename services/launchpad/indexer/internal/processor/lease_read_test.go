package processor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/runtimedocs"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
)

// delayedRecordedSource retains the recorded RPC responses while allowing a
// read to span a lease heartbeat or a caller cancellation.
func delayedRecordedSource(t *testing.T, ctx context.Context, h lifecycleCoverageHarness) (<-chan struct{}, func()) {
	t.Helper()
	upstream, err := url.Parse(nativeAccountingRecordedRPC(t, 33963749))
	if err != nil {
		t.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	var delayed atomic.Bool
	var enteredOnce, releaseOnce sync.Once
	entered, release := make(chan struct{}), make(chan struct{})
	resume := func() { releaseOnce.Do(func() { close(release) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if delayed.Load() {
			enteredOnce.Do(func() { close(entered) })
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(resume)
	opts, err := runtimedocs.CommittedOptions(nativeAccountingReceiptOptions(t, nativeProjectionReceiptInputs(t)))
	if err != nil {
		t.Fatal(err)
	}
	opts.Endpoints[runtimedocs.PrimaryEndpointID] = server.URL
	rpc, err := source.NewCommittedSource(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := rpc.Close(); err != nil {
			t.Error(err)
		}
	})
	h.Replay.Source.(*source.HistoricalQualifiedSource).History.RPC = rpc
	delayed.Store(true)
	return entered, resume
}

func TestQualifiedReadsRetainLeaseAndHonorFencing(t *testing.T) {
	for _, replay := range []bool{false, true} {
		kind := "canonical"
		if replay {
			kind = "replay"
		}
		for _, outcome := range []string{"renewed", "fenced", "canceled"} {
			t.Run(kind+"/"+outcome, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				h := newLifecycleCoverageHarness(t, ctx)
				const height = int64(33963716)
				var request store.ReplayRequest
				var job store.ReplayJob
				var err error
				leaseQuery := `SELECT expires_at FROM indexer.indexer_ingest_leases WHERE chain_id=125 AND stream_id=$1`
				leaseID := realStream
				if replay {
					request = authorizeCoverageReplay(t, h, "replay", "read_lease", height, height)
					if err = h.Store.AdmitReplay(ctx, request); err != nil {
						t.Fatal(err)
					}
					job, err = h.Store.AcquireReplay(ctx, request.JobID, inventoryTestOwner, 30*time.Second)
					leaseQuery = `SELECT expires_at FROM indexer.indexer_replay_jobs WHERE job_id=$1`
					leaseID = request.JobID
				} else {
					h.Processor.canonicalFence, err = h.Store.AcquireLease(ctx, 125, realStream, inventoryTestOwner, 30*time.Second)
					h.Processor.readyFence.Store(h.Processor.canonicalFence)
					h.Processor.canonicalRenewed = time.Now()
				}
				if err != nil {
					t.Fatal(err)
				}
				var initialExpiry time.Time
				if err = h.Pool.QueryRow(ctx, leaseQuery, leaseID).Scan(&initialExpiry); err != nil {
					t.Fatal(err)
				}
				entered, resume := delayedRecordedSource(t, ctx, h)
				readCtx, stop := context.WithCancel(ctx)
				defer stop()
				type result struct {
					blocks int
					job    store.ReplayJob
					err    error
				}
				done := make(chan result, 1)
				go func() {
					if replay {
						read, renewed, readErr := h.Replay.fetchWithLease(readCtx, job, request, height, height)
						done <- result{len(read.Blocks), renewed, readErr}
						return
					}
					read, _, readErr := h.Processor.fetchQualifiedWithLease(readCtx, uint64(height), uint64(height))
					done <- result{blocks: len(read.Blocks), err: readErr}
				}()
				select {
				case <-entered:
				case got := <-done:
					t.Fatalf("read finished before delayed RPC: %+v", got)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				switch outcome {
				case "renewed":
					poll := time.NewTicker(10 * time.Millisecond)
					defer poll.Stop()
					for {
						var expiry time.Time
						if err = h.Pool.QueryRow(ctx, leaseQuery, leaseID).Scan(&expiry); err != nil {
							t.Fatal(err)
						}
						if expiry.After(initialExpiry) {
							break
						}
						select {
						case <-poll.C:
						case got := <-done:
							t.Fatalf("read finished without renewing lease: %+v", got)
						case <-ctx.Done():
							t.Fatal(ctx.Err())
						}
					}
					resume()
				case "fenced":
					statement := `UPDATE indexer.indexer_ingest_leases SET expires_at=clock_timestamp()-interval '1 second' WHERE stream_id=$1`
					if replay {
						statement = `UPDATE indexer.indexer_replay_jobs SET owner='replacement' WHERE job_id=$1`
					}
					if _, err = h.Pool.Exec(ctx, statement, leaseID); err != nil {
						t.Fatal(err)
					}
				case "canceled":
					stop()
				}
				select {
				case got := <-done:
					if outcome == "renewed" {
						if got.err != nil || got.blocks != 1 {
							t.Fatalf("renewed read: %+v", got)
						}
						if replay && (got.job.Fence != job.Fence || got.job.Owner != inventoryTestOwner || got.job.Status != "running") {
							t.Fatalf("renewal changed replay ownership: %+v", got.job)
						}
						if !replay && (h.Processor.canonicalFence == 0 || !h.Processor.canonicalRenewed.After(initialExpiry.Add(-30*time.Second))) {
							t.Fatal("canonical renewal did not retain its active fence and heartbeat")
						}
					} else {
						want := store.ErrFenced
						if outcome == "canceled" {
							want = context.Canceled
						}
						if !errors.Is(got.err, want) || got.blocks != 0 {
							t.Fatalf("rejected read: %+v, want %v without blocks", got, want)
						}
						if !replay && outcome == "fenced" && (h.Processor.canonicalFence != 0 || h.Processor.LeaseState() != "standby") {
							t.Fatal("fenced canonical reader retained its active lease")
						}
					}
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				var records int
				if err = h.Pool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_raw_logs`).Scan(&records); err != nil || records != 0 {
					t.Fatalf("source read changed canonical records: %d, %v", records, err)
				}
			})
		}
	}
}
