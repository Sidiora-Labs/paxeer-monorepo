package ranker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
)

func TestCoordinatorPublishesCompleteEmptyUniverse(t *testing.T) {
	db := internaltest.NewRankingRuntimeDatabase(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	cutoff := time.Now().UTC().Truncate(time.Second)
	stream := "paxeer-125-canonical"
	blockHash := fmt.Sprintf("0x%064x", 200)
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,$1,'indexer-1',1,clock_timestamp()+interval '1 minute')`, []any{stream}},
		{`INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,$1,200,200)`, []any{stream}},
		{`INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete,completed_at) VALUES(125,$1,200,$2,0,true,true,$3)`, []any{stream, blockHash, cutoff.Add(-2 * time.Second)}},
	} {
		if _, err := pool.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	visibilityData := store.RankingVisibility{Complete: true, Revision: strings.Repeat("a", 64), ObservedAt: cutoff.Add(-time.Second), Pools: map[string]bool{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(visibilityData); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	visibility, err := store.NewHTTPRankingVisibility(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	reader, err := store.NewCanonicalInputReader("indexer", "stats", stream, "accounting-v2", visibility)
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.NewFrozenInputs(pool, reader)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := queue.NewEpochGuard("recovery", db.Epoch, db.Region)
	if err != nil {
		t.Fatal(err)
	}
	sink, err := store.NewPublicationStore(pool, "ranking", guard)
	if err != nil {
		t.Fatal(err)
	}
	coordinator, err := ranker.NewCoordinator(source, sink)
	if err != nil {
		t.Fatal(err)
	}
	runID := uuid.NewString()
	snapshots, err := coordinator.Refresh(ctx, runID, cutoff, false)
	if err != nil || len(snapshots) != len(ranker.HeavyCategoryNames()) {
		t.Fatalf("complete heavy run: %d snapshots, %v", len(snapshots), err)
	}
	for _, snapshot := range snapshots {
		if snapshot.RunID() != runID || len(snapshot.Entries()) != 0 || snapshot.OutputDigest() == "" {
			t.Fatalf("completed snapshot %s has incomplete identity", snapshot.Category())
		}
		header, err := sink.Current(ctx, snapshot.Category())
		if err != nil || header.RunID != runID || header.Total != 0 {
			t.Fatalf("committed %s header = %+v, %v", snapshot.Category(), header, err)
		}
	}
	newRunID := uuid.NewString()
	newOnly, err := coordinator.Refresh(ctx, newRunID, cutoff, true)
	if err != nil || len(newOnly) != 1 || newOnly[0].Category() != "new" || newOnly[0].RunID() != newRunID {
		t.Fatalf("new ranking run = %d snapshots, %v", len(newOnly), err)
	}
}
