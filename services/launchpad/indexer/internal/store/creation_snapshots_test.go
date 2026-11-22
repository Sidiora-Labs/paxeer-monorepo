package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/db/sqlcdb"
	"github.com/jackc/pgx/v5"
)

func TestCreationSnapshotCandidatesAndFencedPersistence(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	const stream, owner = "snapshot-storage", "snapshot-worker"
	fence, err := st.AcquireLease(ctx, 125, stream, owner, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	raw := lifecycleRaw()
	poolID := "0x" + strings.Repeat("4", 64)
	raw.Topics = []string{creationSnapshotTopics[0], poolID}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,$1,100,100)`, stream); err != nil {
		t.Fatal(err)
	}
	batch := CanonicalBatch{ChainID: 125, StreamID: stream, Owner: owner, Fence: fence, BatchID: "creation", PreviousHeight: 100, Blocks: []BlockCompleteness{{Height: 101, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}}, Records: []RawRecord{raw}}
	if err = st.CommitCanonical(ctx, batch, nil); err != nil {
		t.Fatal(err)
	}
	row := sqlcdb.InsertPoolParams{PoolAddress: raw.Emitter, TokenAddress: "0x" + strings.Repeat("5", 40), Creator: raw.Emitter, Optical: raw.Emitter, PoolID: poolID, CreatedAt: 1700000000, CreatedBlock: raw.Height, TxHash: raw.TransactionHash}
	if err = st.InsertPool(ctx, row); err != nil {
		t.Fatal(err)
	}
	candidates, err := st.CreationSnapshotCandidates(ctx, 125, 100)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("snapshot candidates: %+v, %v", candidates, err)
	}
	c := candidates[0]
	if c.PoolAddress != row.PoolAddress || c.PoolID != poolID || c.TransactionHash != raw.TransactionHash || c.BlockHash != raw.BlockHash || c.Height != raw.Height || c.TransactionIndex != int64(raw.TransactionIndex) || c.LogIndex != int64(raw.LogIndex) || c.Timestamp != row.CreatedAt || c.LaterLogs {
		t.Fatalf("candidate lost raw coordinates: %+v", c)
	}
	block, err := st.LastCompleteSnapshotBlock(ctx, 125, stream)
	if err != nil || block.Height != 101 || block.Hash != raw.BlockHash {
		t.Fatalf("complete snapshot block: %+v, %v", block, err)
	}
	if _, err := st.LastCompleteSnapshotBlock(ctx, 125, "absent"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("absent snapshot block: %v", err)
	}
	if err := st.CommitCreationSnapshot(ctx, 125, stream, owner, fence, c, block, false, nil); err != nil {
		t.Fatal(err)
	}
	assertJob := func(want string, height int64) {
		t.Helper()
		var status string
		var last int64
		if err := pool.QueryRow(ctx, `SELECT status,last_processed_block FROM indexer.backfill_jobs WHERE id=$1`, "pool-snapshot:"+c.PoolAddress).Scan(&status, &last); err != nil || status != want || last != height {
			t.Fatalf("snapshot job %s/%d: %v", status, last, err)
		}
	}
	assertJob("pending", 100)
	snapshot := sqlcdb.InsertPoolStateSnapshotParams{ID: "creation-snapshot", PoolID: poolID, VirtualReserve: "1000", RealReserve: "900", TokenReserve: "800", Price: "7", BlockNumber: c.Height, BlockTimestamp: c.Timestamp, TxHash: c.TransactionHash, LogIndex: int32(c.LogIndex)}
	for _, tc := range []struct {
		name string
		edit func(*sqlcdb.InsertPoolStateSnapshotParams)
	}{
		{"pool", func(p *sqlcdb.InsertPoolStateSnapshotParams) { p.PoolID = "other" }},
		{"height", func(p *sqlcdb.InsertPoolStateSnapshotParams) { p.BlockNumber++ }},
		{"index", func(p *sqlcdb.InsertPoolStateSnapshotParams) { p.LogIndex++ }},
		{"transaction", func(p *sqlcdb.InsertPoolStateSnapshotParams) { p.TxHash = "other" }},
		{"timestamp", func(p *sqlcdb.InsertPoolStateSnapshotParams) { p.BlockTimestamp++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := snapshot
			tc.edit(&changed)
			if err := st.CommitCreationSnapshot(ctx, 125, stream, owner, fence, c, block, false, &changed); !errors.Is(err, ErrIntegrity) {
				t.Fatalf("snapshot coordinate mismatch: %v", err)
			}
			assertJob("pending", 100)
		})
	}
	for _, tc := range []struct {
		name  string
		block BlockCompleteness
		later bool
	}{
		{"height", BlockCompleteness{Height: 102, Hash: block.Hash}, false},
		{"hash", BlockCompleteness{Height: 101, Hash: "0x" + strings.Repeat("6", 64)}, false},
		{"later logs", block, true},
	} {
		t.Run("fallback/"+tc.name, func(t *testing.T) {
			changed := c
			changed.LaterLogs = tc.later
			if err := st.CommitCreationSnapshot(ctx, 125, stream, owner, fence, changed, tc.block, true, &snapshot); !errors.Is(err, ErrIntegrity) {
				t.Fatalf("unbound snapshot fallback: %v", err)
			}
			assertJob("pending", 100)
		})
	}
	if err := st.CommitCreationSnapshot(ctx, 125, stream, owner, fence+1, c, block, false, &snapshot); !errors.Is(err, ErrFenced) {
		t.Fatalf("stale snapshot fence: %v", err)
	}
	if err := st.CommitCreationSnapshot(ctx, 125, stream, "other", fence, c, block, false, &snapshot); !errors.Is(err, ErrFenced) {
		t.Fatalf("unowned snapshot lease: %v", err)
	}
	for range 2 {
		if err := st.CommitCreationSnapshot(ctx, 125, stream, owner, fence, c, block, true, &snapshot); err != nil {
			t.Fatal(err)
		}
	}
	assertJob("completed", 101)
	var saved sqlcdb.InsertPoolStateSnapshotParams
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*),min(virtual_reserve),min(real_reserve),min(token_reserve),min(price) FROM indexer.pool_state_snapshots`).Scan(&count, &saved.VirtualReserve, &saved.RealReserve, &saved.TokenReserve, &saved.Price); err != nil || count != 1 || saved.VirtualReserve != snapshot.VirtualReserve || saved.RealReserve != snapshot.RealReserve || saved.TokenReserve != snapshot.TokenReserve || saved.Price != snapshot.Price {
		t.Fatalf("stored snapshot reserves: %+v count=%d, %v", saved, count, err)
	}
	if candidates, err := st.CreationSnapshotCandidates(ctx, 125, 100); err != nil || len(candidates) != 0 {
		t.Fatalf("completed pool remained eligible: %+v, %v", candidates, err)
	}
	for _, bounds := range [][2]int{{1, 1}, {125, 0}, {125, 101}} {
		if _, err := st.CreationSnapshotCandidates(ctx, int32(bounds[0]), bounds[1]); !errors.Is(err, ErrIntegrity) {
			t.Fatalf("candidate bounds %v: %v", bounds, err)
		}
	}
	if err := New(pool).CommitCreationSnapshot(ctx, 125, stream, owner, fence, c, block, false, &snapshot); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("unguarded snapshot write: %v", err)
	}
	pool.Close()
	if _, err := st.CreationSnapshotCandidates(ctx, 125, 1); err == nil {
		t.Fatal("closed database returned candidates")
	}
	if err := st.CommitCreationSnapshot(ctx, 125, stream, owner, fence, c, block, false, &snapshot); err == nil {
		t.Fatal("closed database persisted a snapshot")
	}
}
