package app

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/telemetry"
	"github.com/ethereum/go-ethereum/common"
)

func TestRuntimeCheckpointChecksTheSourceAndQuarantinesUnavailableHeads(t *testing.T) {
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
		body, e := os.ReadFile(filepath.Join("../../../../../migrations", id+".sql"))
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, string(body)); e != nil {
			t.Fatal(e)
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
	defer provider.Close()
	block, err := provider.BlockAt(ctx, 33963717)
	if err != nil {
		t.Fatal(err)
	}
	doc.InitialCheckpointHeight = int64(block.Height)
	doc.InitialCheckpointHash = strings.ToLower(block.Hash.Hex())
	const stream = "runtime-checkpoint"
	if err = seedCanonicalWatermark(ctx, pool, stream, doc); err != nil {
		t.Fatal(err)
	}
	if err = verifyCanonicalRuntimeCheckpoint(ctx, pool, provider, stream, doc.InitialCheckpointHeight, doc); err != nil {
		t.Fatal(err)
	}
	changed := doc
	changed.InitialCheckpointHash = common.HexToHash("0xbad").Hex()
	if err = verifyCanonicalRuntimeCheckpoint(ctx, pool, provider, stream, doc.InitialCheckpointHeight, changed); err == nil {
		t.Fatal("changed checkpoint admitted")
	}
	if err = verifyCanonicalRuntimeCheckpoint(ctx, pool, provider, stream, doc.InitialCheckpointHeight+1, doc); err == nil {
		t.Fatal("unpersisted checkpoint admitted")
	}
	for _, height := range []int64{-1, doc.InitialCheckpointHeight} {
		if err = verifyCanonicalRuntimeCheckpoint(ctx, pool, nil, stream, height, doc); err == nil {
			t.Fatal("missing checkpoint provider admitted")
		}
	}
	h := &canonicalRuntimeHead{source: provider, store: st, stream: stream, height: new(atomic.Uint64)}
	head, err := h.Head(ctx)
	if err != nil || head < doc.InitialCheckpointHeight || h.height.Load() != uint64(doc.InitialCheckpointHeight) {
		t.Fatalf("runtime head %d: %v", head, err)
	}
	if h.Name() != "durable-pinned-committed-rpc" || h.Close() != nil {
		t.Fatal("runtime head identity unavailable")
	}
	if err = provider.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = h.Head(ctx); err == nil {
		t.Fatal("unavailable head admitted")
	}
	var integrity string
	if err = pool.QueryRow(ctx, `SELECT integrity FROM indexer.indexer_committed_heads WHERE stream_id=$1 ORDER BY height DESC LIMIT 1`, stream).Scan(&integrity); err != nil || integrity != "quarantined" {
		t.Fatalf("unavailable head state %s: %v", integrity, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err = verifyCanonicalRuntimeCheckpoint(canceled, pool, provider, stream, doc.InitialCheckpointHeight, doc); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled checkpoint: %v", err)
	}
	if _, err = h.Head(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled durable head: %v", err)
	}
	for _, committed := range []*source.CommittedAnchor{nil, {Height: 1, Hash: common.HexToHash("0x1")}} {
		l := lifecycleRuntimeDocument{Committed: source.CommittedOptions{Anchor: committed}}
		e := bindRuntimeAnchor(&l, receipt.RPCOptions{})
		if committed == nil && e != nil || committed != nil && e == nil {
			t.Fatalf("genesis anchor binding: %v", e)
		}
		l.Committed.BlockIdentity = receipt.BlockIdentityReported
		if e = bindRuntimeAnchor(&l, receipt.RPCOptions{}); e == nil {
			t.Fatal("different block identity admitted")
		}
	}
	if err = attestDurableAnchor(ctx, nil, stream, receipt.BlockAnchor{}); err == nil {
		t.Fatal("missing anchor state admitted")
	}
	if err = seedCanonicalWatermark(ctx, pool, stream, lifecycleRuntimeDocument{}); err == nil {
		t.Fatal("unissued initial checkpoint admitted")
	}
	if _, err = pool.Exec(ctx, `ALTER TABLE indexer.indexer_block_completeness DROP CONSTRAINT IF EXISTS indexer_block_completeness_block_hash_check`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,1,'bad',0,true,true)`, stream); err != nil {
		t.Fatal(err)
	}
	if _, err = durableBlockHash(ctx, pool, stream, 1, doc); err == nil || !strings.Contains(err.Error(), "hash invalid") {
		t.Fatalf("invalid persisted hash: %v", err)
	}
}

func TestRuntimeReadMetricsClassifyCancellationAndInvalidRequests(t *testing.T) {
	metrics, err := telemetry.New("indexerd", "qualification", "zone-a")
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []int{0, http.StatusBadRequest, http.StatusUnprocessableEntity} {
		for _, canceled := range []bool{false, true} {
			ctx, cancel := context.WithCancel(context.Background())
			if canceled {
				cancel()
			}
			tracker := process.NewTracker()
			if err := tracker.SetReady(); err != nil {
				t.Fatal(err)
			}
			h := runtimeHandler(tracker, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if status != 0 {
					w.WriteHeader(status)
				}
			}), metrics)
			r := httptest.NewRecorder()
			h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/markets", nil).WithContext(ctx))
			cancel()
			want := status
			if want == 0 {
				want = http.StatusOK
			}
			if r.Code != want {
				t.Fatalf("read status %d, want %d", r.Code, want)
			}
		}
	}
	var rendered bytes.Buffer
	if err := metrics.WritePrometheus(&rendered); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(rendered.String(), "\n")
	found := false
	for _, line := range lines {
		if strings.HasPrefix(line, "sli_read_eligible_requests_total{") && strings.Contains(line, `route="other"`) {
			if !strings.HasSuffix(line, " 1") {
				t.Fatalf("eligible read metric %s", line)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("read eligibility metric absent")
	}
}
