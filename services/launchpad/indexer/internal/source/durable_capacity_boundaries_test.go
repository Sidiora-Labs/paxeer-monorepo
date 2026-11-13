package source

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
)

func TestCapacityDurableEmptyBlocksRequireObservedCommit(t *testing.T) {
	ctx := context.Background()
	node := newCapacityChain(t)
	node.logs = nil
	live := repair997Open(t, fopts(node.server.URL))
	_, pool := canonicaltest.New(t)
	st := store.New(pool)
	fence, err := st.AcquireLease(ctx, 125, "empty-capacity", "capacity", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'empty-capacity',100,100)`); err != nil {
		t.Fatal(err)
	}
	workload := &canonicalCapacityWorkload{source: live, store: st, fence: fence, stream: "empty-capacity"}
	observed := time.Now().Add(-time.Second)
	if err = workload.ProcessBatch(ctx, 101, 101); err != nil {
		t.Fatal(err)
	}
	expected, err := live.FetchCommitted(ctx, FetchOptions{FromBlock: 101, ToBlock: 101}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name         string
		observations map[uint64]time.Time
		sql, restore string
		want         string
	}{
		{name: "complete", observations: map[uint64]time.Time{101: observed}},
		{name: "missing observation", want: "capacity-empty-observation"},
		{name: "future observation", observations: map[uint64]time.Time{101: time.Now().Add(time.Hour)}, want: "capacity-empty-commit"},
		{name: "missing completeness", sql: `UPDATE indexer.indexer_block_completeness SET height=102 WHERE stream_id='empty-capacity'`, restore: `UPDATE indexer.indexer_block_completeness SET height=101 WHERE stream_id='empty-capacity'`, want: "capacity-block-coverage"},
		{name: "unavailable raw", sql: `ALTER TABLE indexer.indexer_raw_logs RENAME TO capacity_raw_unavailable`, restore: `ALTER TABLE indexer.capacity_raw_unavailable RENAME TO indexer_raw_logs`, want: "does not exist"},
		{name: "unavailable blocks", sql: `ALTER TABLE indexer.indexer_block_completeness RENAME TO capacity_blocks_unavailable`, restore: `ALTER TABLE indexer.capacity_blocks_unavailable RENAME TO indexer_block_completeness`, want: "capacity-block-coverage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.sql != "" {
				if _, err := pool.Exec(ctx, tc.sql); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if _, err := pool.Exec(ctx, tc.restore); err != nil {
						t.Error(err)
					}
				}()
			}
			capture, err := newCapacityCapture(filepath.Join(t.TempDir(), "capture.jsonl"), 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			latest, count, err := capacityDurable(ctx, pool, "empty-capacity", 101, 101, expected, tc.observations, 0, map[int64]uint64{}, capture)
			if tc.want == "" {
				if err != nil || latest.Before(observed) || count != 0 {
					t.Fatalf("empty commit %v %d %v", latest, count, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid empty commit %v", err)
			}
			if _, _, err := capture.finish(); err != nil {
				t.Fatal(err)
			}
		})
	}
	pool.Close()
	capture, err := newCapacityCapture(filepath.Join(t.TempDir(), "closed.jsonl"), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = capacityDurable(ctx, pool, "empty-capacity", 101, 101, expected, nil, 0, map[int64]uint64{}, capture); err == nil {
		t.Fatal("closed durable pool accepted")
	}
	if _, _, err = capture.finish(); err != nil {
		t.Fatal(err)
	}
}
