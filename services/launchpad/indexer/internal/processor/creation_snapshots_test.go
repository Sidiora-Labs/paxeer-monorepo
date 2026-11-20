package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/publisher"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5/pgxpool"
)

type creationSnapshotHarness struct {
	node      *creationSnapshotNode
	authority *receipt.RPCQualification
	created   *types.Receipt
	address   common.Address
	pool      *pgxpool.Pool
	proc      *Processor
	batch     store.CanonicalBatch
	receipts  []*receipt.QualifiedReceipt
	fact      store.CreationFact
}

func newCreationSnapshotHarness(t *testing.T, withSwap bool) creationSnapshotHarness {
	t.Helper()
	n, options := prepareCreationSnapshotFactory(t)
	authority, created, address := n.create(t, options, withSwap)
	_, pool := canonicaltest.New(t)
	applyProductionSchema(t, n.ctx, pool)
	if _, err := pool.Exec(n.ctx, `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=$1,typed_complete_height=$1`, created.BlockNumber.Int64()-1); err != nil {
		t.Fatal(err)
	}
	guard, err := queue.NewEpochGuard("recovery", 1, "railway-prod")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.NewLifecycleStore(pool, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{publisher.CanonicalCreationSubject: true, accounting.MarketSubject: true}, EventTypes: map[string]bool{publisher.CanonicalCreationEventType: true, accounting.MarketSubject: true}}
	outbox, err := delivery.NewStoreWithEpochGuard(pool, "indexer", policy, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	partitions, err := delivery.NewPartitionStore(pool, "indexer")
	if err != nil {
		t.Fatal(err)
	}
	durable, err := publisher.NewDurableStager(outbox, options.DeploymentVersion, map[string]string{publisher.CanonicalCreationEventType: publisher.CanonicalCreationSubject, accounting.MarketSubject: accounting.MarketSubject})
	if err != nil {
		t.Fatal(err)
	}
	head, err := source.NewEVM(n.ctx, source.EVMOptions{RPCURLs: []string{n.endpoint}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := head.Close(); err != nil {
			t.Error(err)
		}
	})
	p := New(Deps{Store: st, Decoder: decode.NewDecoder(), EventEmitterAddress: options.Roles["EventEmitter"].Address, ChainID: 125, QualifiedBatchSource: authority, Head: head, CanonicalStreamID: realStream, CanonicalOwner: "snapshot-indexer", AccountingPartitions: partitions, Durable: durable, AccountingProjectionVersion: "accounting-market-v1", LegacyProjection: true, Reserves: authority, PollInterval: 10 * time.Millisecond})
	fence, err := st.AcquireLease(n.ctx, 125, realStream, p.deps.CanonicalOwner, canonicalLeaseTTL)
	if err != nil {
		t.Fatal(err)
	}
	p.canonicalFence = fence
	p.canonicalRenewed = time.Now()
	batch, receipts, err := authority.FetchQualifiedBatch(n.ctx, created.BlockNumber.Uint64(), created.BlockNumber.Uint64())
	if err != nil {
		t.Fatal(err)
	}
	batch.StreamID, batch.Owner, batch.Fence = realStream, p.deps.CanonicalOwner, fence
	facts, err := qualifiedCreationFacts(n.ctx, receipts)
	if err != nil || len(facts) != 1 {
		t.Fatalf("qualified creation facts: %v %v", facts, err)
	}
	return creationSnapshotHarness{node: n, authority: authority, created: created, address: address, pool: pool, proc: p, batch: batch, receipts: receipts, fact: facts[0]}
}

func (h creationSnapshotHarness) commit(t *testing.T) {
	t.Helper()
	if err := h.proc.ProcessQualifiedCanonicalBatch(h.node.ctx, h.batch, h.receipts); err != nil {
		t.Fatal(err)
	}
}

func (h creationSnapshotHarness) assertCreation(t *testing.T, reader ReserveReader, at common.Hash) {
	t.Helper()
	want, err := reader.PoolReserves(h.node.ctx, h.address, at)
	if err != nil {
		t.Fatal(err)
	}
	var virtual, real, token, price, tx, id string
	var height, index, timestamp int64
	err = h.pool.QueryRow(h.node.ctx, `SELECT id,virtual_reserve,real_reserve,token_reserve,price,block_number,block_timestamp,tx_hash,log_index FROM indexer.pool_state_snapshots WHERE pool_id=$1 AND block_number=$2 AND log_index=$3`, poolIDFor(h.address), h.fact.Position.Height, h.fact.Position.LogIndex).Scan(&id, &virtual, &real, &token, &price, &height, &timestamp, &tx, &index)
	if err != nil {
		t.Fatal(err)
	}
	if id != fmt.Sprintf("%s-%s-%s", poolIDFor(h.address), h.fact.Position.Height, h.fact.Position.LogIndex) || virtual != want.VirtualUSDL.String() || real != want.RealUSDL.String() || token != want.Token.String() || price != want.Price.String() || tx != h.fact.Position.TransactionHash || strconv.FormatInt(height, 10) != h.fact.Position.Height || strconv.FormatInt(timestamp, 10) != h.fact.BlockTimestampSeconds || strconv.FormatInt(index, 10) != h.fact.Position.LogIndex {
		t.Fatalf("creation reserves or position differ: %s %s %s %s @ %d/%d", virtual, real, token, price, height, index)
	}
}

func (h creationSnapshotHarness) count(t *testing.T, table string) int {
	t.Helper()
	var count int
	if err := h.pool.QueryRow(h.node.ctx, `SELECT count(*) FROM indexer.`+table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestCreationSnapshotWithoutSwap(t *testing.T) {
	h := newCreationSnapshotHarness(t, false)
	h.commit(t)
	h.assertCreation(t, h.authority, h.created.BlockHash)
	if h.count(t, "pools") != 1 || h.count(t, "pool_state_snapshots") != 1 || h.count(t, "swaps") != 0 {
		t.Fatal("creation must publish a pool and exactly one state before its first swap")
	}
}

func TestCreationSnapshotWithSwapAndStateUpdateInSameBlock(t *testing.T) {
	h := newCreationSnapshotHarness(t, true)
	h.commit(t)
	h.assertCreation(t, h.authority, h.created.BlockHash)
	if h.count(t, "pool_state_snapshots") != 3 || h.count(t, "swaps") != 1 {
		t.Fatal("creation, swap and state update must retain their three snapshot positions")
	}
	var distinct int
	if err := h.pool.QueryRow(h.node.ctx, `SELECT count(DISTINCT log_index) FROM indexer.pool_state_snapshots`).Scan(&distinct); err != nil || distinct != 3 {
		t.Fatalf("snapshot log positions: %d %v", distinct, err)
	}
}

func TestCreationSnapshotSameBlockRetry(t *testing.T) {
	h := newCreationSnapshotHarness(t, false)
	h.commit(t)
	committed, err := h.proc.deps.Store.ReconcileCanonicalCommit(h.node.ctx, h.batch)
	if err != nil || !committed {
		t.Fatalf("canonical creation reconciliation: %v %v", committed, err)
	}
	if err = h.proc.ProcessBatch(h.node.ctx, h.batch.Blocks[0].Height, h.batch.Blocks[0].Height); err != nil {
		t.Fatal(err)
	}
	h.assertCreation(t, h.authority, h.created.BlockHash)
	if h.count(t, "pool_state_snapshots") != 1 || h.count(t, "pools") != 1 || h.count(t, "indexer_canonical_creations") != 1 {
		t.Fatal("repeating a complete block must keep one creation snapshot")
	}
}

func TestCreationSnapshotUnavailableReadHoldsBlock(t *testing.T) {
	h := newCreationSnapshotHarness(t, false)
	closedClient, err := ethclient.DialContext(h.node.ctx, strings.Replace(h.node.endpoint, "http://", "ws://", 1))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closedClient.Close)
	unavailable, err := receipt.NewPrivateFixtureRPCQualification(h.node.ctx, closedClient, h.node.options)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = unavailable.FetchQualifiedBatch(h.node.ctx, h.created.BlockNumber.Uint64(), h.created.BlockNumber.Uint64()); err != nil {
		t.Fatal(err)
	}
	h.proc.deps.Reserves = unavailable
	closedClient.Close()
	if err := h.proc.ProcessQualifiedCanonicalBatch(h.node.ctx, h.batch, h.receipts); err == nil || !strings.Contains(err.Error(), "pinned creation reserves") {
		t.Fatalf("creation reserve read must hold the block: %v", err)
	}
	for _, table := range []string{"pools", "pool_state_snapshots", "indexer_canonical_creations", "indexer_raw_logs", "delivery_outbox"} {
		if h.count(t, table) != 0 {
			t.Fatalf("%s advanced after a failed reserve read", table)
		}
	}
	watermark, err := h.proc.deps.Store.GetCanonicalWatermark(h.node.ctx, 125, realStream)
	if err != nil || watermark != h.created.BlockNumber.Int64()-1 {
		t.Fatalf("creation watermark: %d %v", watermark, err)
	}
	client, err := ethclient.DialContext(h.node.ctx, h.node.endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	authority, err := receipt.NewPrivateFixtureRPCQualification(h.node.ctx, client, h.node.options)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = authority.FetchQualifiedBatch(h.node.ctx, h.created.BlockNumber.Uint64(), h.created.BlockNumber.Uint64()); err != nil {
		t.Fatal(err)
	}
	h.proc.deps.Reserves = authority
	h.commit(t)
	h.assertCreation(t, authority, h.created.BlockHash)
}

func TestCreationSnapshotSharesCanonicalTransaction(t *testing.T) {
	h := newCreationSnapshotHarness(t, false)
	_, err := h.pool.Exec(h.node.ctx, `CREATE FUNCTION indexer.refuse_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'snapshot write refused'; END $$; CREATE TRIGGER refuse_snapshot BEFORE INSERT ON indexer.pool_state_snapshots FOR EACH ROW EXECUTE FUNCTION indexer.refuse_snapshot()`)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.proc.ProcessQualifiedCanonicalBatch(h.node.ctx, h.batch, h.receipts); err == nil || !strings.Contains(err.Error(), "snapshot write refused") {
		t.Fatalf("snapshot persistence must hold the block: %v", err)
	}
	for _, table := range []string{"pools", "pool_state_snapshots", "indexer_canonical_creations", "indexer_raw_logs", "delivery_outbox"} {
		if h.count(t, table) != 0 {
			t.Fatalf("%s escaped the canonical rollback", table)
		}
	}
	if _, err = h.pool.Exec(h.node.ctx, `DROP TRIGGER refuse_snapshot ON indexer.pool_state_snapshots`); err != nil {
		t.Fatal(err)
	}
	h.commit(t)
	h.assertCreation(t, h.authority, h.created.BlockHash)
}

func (h creationSnapshotHarness) restart(t *testing.T, reader ReserveReader, log io.Writer) {
	t.Helper()
	if h.proc.canonicalFence != 0 {
		if err := h.proc.deps.Store.ReleaseLease(h.node.ctx, 125, realStream, h.proc.deps.CanonicalOwner, h.proc.canonicalFence); err != nil {
			t.Fatal(err)
		}
		h.proc.canonicalFence = 0
	}
	deps := h.proc.deps
	deps.Reserves = reader
	deps.Logger = slog.New(slog.NewTextHandler(log, nil))
	p := New(deps)
	ctx, cancel := context.WithTimeout(h.node.ctx, 2*time.Second)
	defer cancel()
	if err := p.Run(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("restarted processor: %v", err)
	}
}

func (h creationSnapshotHarness) removeSnapshots(t *testing.T) {
	t.Helper()
	if _, err := h.pool.Exec(h.node.ctx, `DELETE FROM indexer.pool_state_snapshots`); err != nil {
		t.Fatal(err)
	}
}

func TestCreationSnapshotStartupBackfillsStoredPool(t *testing.T) {
	h := newCreationSnapshotHarness(t, false)
	h.commit(t)
	h.removeSnapshots(t)
	h.restart(t, h.authority, io.Discard)
	h.assertCreation(t, h.authority, h.created.BlockHash)
	if h.count(t, "pool_state_snapshots") != 1 {
		t.Fatal("startup must restore one creation snapshot")
	}
	h.restart(t, h.authority, io.Discard)
	if h.count(t, "pool_state_snapshots") != 1 {
		t.Fatal("second startup must preserve the existing snapshot")
	}
}

func (h *creationSnapshotHarness) advanceEmptyBlock(t *testing.T) common.Hash {
	t.Helper()
	if err := h.node.client.Client().CallContext(h.node.ctx, nil, "evm_mine"); err != nil {
		t.Fatal(err)
	}
	header, err := h.node.client.HeaderByNumber(h.node.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, receipts, err := h.authority.FetchQualifiedBatch(h.node.ctx, h.created.BlockNumber.Uint64()+1, header.Number.Uint64())
	if err != nil {
		t.Fatal(err)
	}
	b.StreamID, b.Owner, b.Fence = realStream, h.proc.deps.CanonicalOwner, h.proc.canonicalFence
	if err = h.proc.ProcessQualifiedCanonicalBatch(h.node.ctx, b, receipts); err != nil {
		t.Fatal(err)
	}
	return header.Hash()
}

func (h creationSnapshotHarness) retentionReader(t *testing.T, message string) (*receipt.RPCQualification, *atomic.Int64, *atomic.Int64) {
	t.Helper()
	endpoint, err := url.Parse(h.node.endpoint)
	if err != nil {
		t.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(endpoint)
	var enabled atomic.Bool
	var creation, current atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		var call struct {
			ID     json.RawMessage
			Method string
			Params []json.RawMessage
		}
		if err = json.Unmarshal(body, &call); err != nil {
			t.Error(err)
			return
		}
		if enabled.Load() && call.Method == "eth_call" && len(call.Params) == 2 {
			var selector struct{ BlockHash common.Hash }
			if err = json.Unmarshal(call.Params[1], &selector); err != nil {
				t.Error(err)
				return
			}
			if selector.BlockHash == h.created.BlockHash {
				creation.Add(1)
				w.Header().Set("Content-Type", "application/json")
				if err = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": call.ID, "error": map[string]any{"code": -32000, "message": message}}); err != nil {
					t.Error(err)
				}
				return
			}
			current.Add(1)
		}
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	client, err := ethclient.DialContext(h.node.ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	authority, err := receipt.NewPrivateFixtureRPCQualification(h.node.ctx, client, h.node.options)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = authority.FetchQualifiedBatch(h.node.ctx, h.created.BlockNumber.Uint64(), h.created.BlockNumber.Uint64()); err != nil {
		t.Fatal(err)
	}
	enabled.Store(true)
	return authority, &creation, &current
}

func TestCreationSnapshotStartupPrunedStateUsesCompleteBlock(t *testing.T) {
	h := newCreationSnapshotHarness(t, false)
	h.commit(t)
	latest := h.advanceEmptyBlock(t)
	h.removeSnapshots(t)
	reader, creation, current := h.retentionReader(t, "historical state pruned")
	h.restart(t, reader, io.Discard)
	h.assertCreation(t, h.authority, latest)
	if creation.Load() != 1 || current.Load() != 2 {
		t.Fatalf("pinned creation/current reserve calls: %d/%d", creation.Load(), current.Load())
	}
}

func TestCreationSnapshotStartupLeavesUnprovenPool(t *testing.T) {
	for _, tc := range []struct {
		name, message string
		withSwap      bool
	}{
		{"later pool logs", "historical state pruned", true},
		{"read refused", "execution reverted", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newCreationSnapshotHarness(t, tc.withSwap)
			h.commit(t)
			h.advanceEmptyBlock(t)
			h.removeSnapshots(t)
			reader, creation, current := h.retentionReader(t, tc.message)
			var log bytes.Buffer
			h.restart(t, reader, &log)
			if h.count(t, "pool_state_snapshots") != 0 || creation.Load() != 1 || current.Load() != 0 || strings.Count(log.String(), strings.ToLower(h.address.Hex())) != 1 {
				t.Fatalf("unproven pool must remain without a snapshot and be logged once: %d/%d %s", creation.Load(), current.Load(), log.String())
			}
			candidates, err := h.proc.deps.Store.CreationSnapshotCandidates(h.node.ctx, 125, creationSnapshotPoolLimit)
			if err != nil || len(candidates) != 1 || candidates[0].LaterLogs != tc.withSwap {
				t.Fatalf("pending snapshot candidate: %+v %v", candidates, err)
			}
		})
	}
}

func TestCreationSnapshotBoundedCandidatesContinueAfterDeferredPool(t *testing.T) {
	h := newCreationSnapshotHarness(t, false)
	h.commit(t)
	authority, created, _ := h.node.create(t, h.node.options, false)
	b, receipts, err := authority.FetchQualifiedBatch(h.node.ctx, h.created.BlockNumber.Uint64()+1, created.BlockNumber.Uint64())
	if err != nil {
		t.Fatal(err)
	}
	b.StreamID, b.Owner, b.Fence = realStream, h.proc.deps.CanonicalOwner, h.proc.canonicalFence
	h.proc.deps.Reserves = authority
	if err = h.proc.ProcessQualifiedCanonicalBatch(h.node.ctx, b, receipts); err != nil {
		t.Fatal(err)
	}
	h.removeSnapshots(t)
	first, err := h.proc.deps.Store.CreationSnapshotCandidates(h.node.ctx, 125, 1)
	if err != nil || len(first) != 1 {
		t.Fatalf("bounded candidates: %+v %v", first, err)
	}
	if err = h.proc.deps.Store.CommitCreationSnapshot(h.node.ctx, 125, realStream, h.proc.deps.CanonicalOwner, h.proc.canonicalFence, first[0], store.BlockCompleteness{}, false, nil); err != nil {
		t.Fatal(err)
	}
	next, err := h.proc.deps.Store.CreationSnapshotCandidates(h.node.ctx, 125, 1)
	if err != nil || len(next) != 1 || next[0].PoolAddress == first[0].PoolAddress {
		t.Fatalf("next pass must reach the next missing pool: %+v %v", next, err)
	}
	if _, err = h.proc.deps.Store.CreationSnapshotCandidates(h.node.ctx, 125, creationSnapshotPoolLimit+1); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("unbounded pool pass accepted: %v", err)
	}
}
