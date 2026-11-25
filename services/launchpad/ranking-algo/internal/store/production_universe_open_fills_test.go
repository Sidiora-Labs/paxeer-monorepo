package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductionUniverseOpenFillsContradictions(t *testing.T) {
	for _, tc := range []struct {
		name, statement string
	}{
		{"digest", `UPDATE stats.accounting_market_commits SET source_fact_digest=repeat('f',64)`},
		{"payload", `UPDATE stats.accounting_market_commits SET fill=jsonb_set(fill,'{usdlAmountRaw}','"2000000"')`},
		{"missing source", `DELETE FROM indexer.indexer_accounting_fills`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, reader, cutoff := openFillsUniverse(t, []string{poolA})
			insertFill(t, pool, 1, openFillsTrade(poolA, 101, cutoff.Unix()-60))
			if _, err := pool.Exec(context.Background(), tc.statement); err != nil {
				t.Fatal(err)
			}
			assertOpenFillsCoherenceRefusal(t, pool, reader, cutoff)
		})
	}
}

func TestProductionUniverseOpenFillsFrozenFence(t *testing.T) {
	pool, reader, cutoff := openFillsUniverse(t, []string{poolA})
	if _, err := pool.Exec(context.Background(), `INSERT INTO stats.accounting_pool_fences(pool,frozen) VALUES($1,true)`, poolA); err != nil {
		t.Fatal(err)
	}
	assertOpenFillsCoherenceRefusal(t, pool, reader, cutoff)
}

func assertOpenFillsCoherenceRefusal(t *testing.T, pool *pgxpool.Pool, reader *CanonicalInputReader, cutoff time.Time) {
	t.Helper()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer rollbackTransaction(context.Background(), tx)
	_, err = reader.ReadRankingUniverseTxDetailed(context.Background(), tx, uuid.NewString(), cutoff)
	step, class, ok := ranker.RefreshRefusalDetails(err)
	if !ok || step != "coherence" || class != "validation" {
		t.Fatalf("coherence refusal = %q/%q/%t: %v", step, class, ok, err)
	}
}

func openFillsTrade(address string, height, timestamp int64) universeFill {
	return universeFill{pool: address, side: "buy", wallet: walletOne, height: height, timestamp: timestamp, usdl: "1000000", token: "1" + strings.Repeat("0", 18), paid: "1000000", fee: "10000"}
}

func openFillsUniverse(t *testing.T, addresses []string) (*pgxpool.Pool, *CanonicalInputReader, time.Time) {
	t.Helper()
	pool := productionUniverseDatabase(t)
	cutoff := time.Now().UTC().Truncate(time.Second)
	ctx := context.Background()
	visible := make(map[string]bool, len(addresses))
	evidence := make(map[string]CanonicalPoolEvidence, len(addresses))
	for i, address := range addresses {
		if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$3,$3,1,100,$4,$5,$6,0,$7,'{}','{}')`, fmt.Sprintf("0x%040x", i+100000), address, walletOne, fmt.Sprintf("0x%064x", 100), fmt.Sprintf("0x%064x", i+10000), i, cutoff.Unix()-3600); err != nil {
			t.Fatal(err)
		}
		visible[address] = true
		evidence[address] = CanonicalPoolEvidence{Paused: boolPointer(false), PauseEvidence: strings.Repeat("a", 64)}
	}
	for _, statement := range []string{
		`INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,'` + universeStream + `','indexer-1',1,clock_timestamp()+interval '1 minute')`,
		`INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,'` + universeStream + `',200,200)`,
	} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete,completed_at) VALUES(125,$1,200,$2,0,true,true,$3)`, universeStream, fmt.Sprintf("0x%064x", 200), cutoff.Add(-2*time.Second)); err != nil {
		t.Fatal(err)
	}
	reader, err := NewCanonicalInputReader("indexer", "stats", universeStream, universeProjection, newFixtureVisibility(t, cutoff.Add(-time.Second), visible))
	if err != nil {
		t.Fatal(err)
	}
	reader.Evidence = newFixtureCanonicalEvidence(t, cutoff, 200, fmt.Sprintf("0x%064x", 200), evidence)
	return pool, reader, cutoff
}

func insertOpenFill(t *testing.T, pool *pgxpool.Pool, sequence int64, fill universeFill) {
	t.Helper()
	insertFill(t, pool, sequence, fill)
	if _, err := pool.Exec(context.Background(), `DELETE FROM stats.accounting_market_commits WHERE projection_version=$1 AND source_fill_id=$2`, universeProjection, fmt.Sprintf("0x%064x:0", sequence)); err != nil {
		t.Fatal(err)
	}
}

func freezeOpenFills(t *testing.T, pool *pgxpool.Pool, reader *CanonicalInputReader, cutoff time.Time) ranker.InputSnapshot {
	t.Helper()
	frozen, err := NewFrozenInputs(pool, reader)
	if err != nil {
		t.Fatal(err)
	}
	in, err := frozen.Freeze(context.Background(), uuid.NewString(), cutoff)
	if err != nil || !in.Complete {
		t.Fatalf("freeze open fills: complete=%t: %v", in.Complete, err)
	}
	return in
}

func TestProductionUniverseOpenFillsFresh(t *testing.T) {
	pool, reader, cutoff := openFillsUniverse(t, []string{poolA, poolB})
	insertFill(t, pool, 1, openFillsTrade(poolA, 101, cutoff.Unix()-600))
	insertFill(t, pool, 2, openFillsTrade(poolB, 102, cutoff.Unix()-600))
	baseline := freezeOpenFills(t, pool, reader, cutoff)
	insertOpenFill(t, pool, 3, openFillsTrade(poolA, 190, cutoff.Unix()-10))
	in := freezeOpenFills(t, pool, reader, cutoff)
	if len(baseline.Candidates) != 2 || len(in.Candidates) != 2 || baseline.OpenFillPools != 0 || in.OpenFillPools != 1 {
		t.Fatalf("candidate/open pool counts: %d/%d, %d/%d", len(baseline.Candidates), baseline.OpenFillPools, len(in.Candidates), in.OpenFillPools)
	}
	for i, candidate := range in.Candidates {
		want := baseline.Candidates[i]
		if want.SourceObservedAt != cutoff.Unix()-2 {
			t.Fatalf("committed observation = %d", want.SourceObservedAt)
		}
		if candidate.Pool.PoolAddress == poolA {
			want.SourceObservedAt = cutoff.Unix() - 11
		}
		if !reflect.DeepEqual(candidate, want) {
			t.Fatalf("candidate differs from committed figures: got %+v, want %+v", candidate, want)
		}
	}
	assertOpenFillsLists(t, in, false)
}

func TestProductionUniverseOpenFillsStaleAndRecovery(t *testing.T) {
	pool, reader, cutoff := openFillsUniverse(t, []string{poolA, poolB})
	insertFill(t, pool, 1, openFillsTrade(poolA, 101, cutoff.Unix()-600))
	insertFill(t, pool, 2, openFillsTrade(poolB, 102, cutoff.Unix()-600))
	insertOpenFill(t, pool, 3, openFillsTrade(poolA, 190, cutoff.Unix()-120))
	in := freezeOpenFills(t, pool, reader, cutoff)
	if len(in.Candidates) != 2 || in.OpenFillPools != 1 || in.Candidates[0].SourceObservedAt != cutoff.Unix()-121 || in.Candidates[1].SourceObservedAt != cutoff.Unix()-2 {
		t.Fatalf("stale pool observation: %+v", in)
	}
	assertOpenFillsLists(t, in, true)
	_, err := pool.Exec(context.Background(), `INSERT INTO stats.accounting_market_commits(publication_id,source_fill_id,source_fact_digest,projection_version,pool,bucket_start,position,fill,sequence,revision)
SELECT $1,source_fill_id,source_fact_digest,$2,pool_address,((payload->>'blockTimestampSeconds')::bigint/60)*60,'{}'::jsonb,payload,3,3
FROM indexer.indexer_accounting_fills WHERE source_fill_id=$3`, uuid.NewString(), universeProjection, fmt.Sprintf("0x%064x:0", 3))
	if err != nil {
		t.Fatal(err)
	}
	recovered := freezeOpenFills(t, pool, reader, cutoff)
	if len(recovered.Candidates) != 2 || recovered.OpenFillPools != 0 || recovered.Candidates[0].SourceObservedAt != cutoff.Unix()-2 || recovered.Candidates[0].Pool.Volume24h != "2.000000" {
		t.Fatalf("committed pool observation: %+v", recovered)
	}
	assertOpenFillsLists(t, recovered, false)
}

func assertOpenFillsLists(t *testing.T, in ranker.InputSnapshot, stale bool) {
	t.Helper()
	heavy, err := ranker.ComputeRun(in, false)
	if err != nil {
		t.Fatal(err)
	}
	newest, err := ranker.ComputeRun(in, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, snapshot := range append(heavy, newest...) {
		category := snapshot.Category()
		if stale {
			found := false
			for _, exclusion := range snapshot.Exclusions() {
				if exclusion.Address == poolA {
					found = true
					if exclusion.Reason != "source-stale" {
						t.Fatalf("%s open pool reason = %s", category, exclusion.Reason)
					}
				}
			}
			if !found {
				t.Fatalf("%s has no stale pool exclusion", category)
			}
		}
		if category != "trending" && category != "volume" && category != "new" {
			continue
		}
		want := 2
		if stale {
			want = 1
		}
		entries := snapshot.Entries()
		if len(entries) != want {
			t.Fatalf("%s entries = %+v, exclusions = %+v", category, entries, snapshot.Exclusions())
		}
		if stale && entries[0].Address != poolB {
			t.Fatalf("%s unaffected pool missing: %+v", category, entries)
		}
		if !stale && len(snapshot.Exclusions()) != 0 {
			t.Fatalf("%s fresh pools excluded: %+v", category, snapshot.Exclusions())
		}
	}
}

func TestProductionUniverseOpenFillsOldest(t *testing.T) {
	for _, tc := range []struct {
		name string
		ages []int64
		want int64
	}{
		{"oldest timestamp", []int64{10, 120, 30}, 121},
		{"cutoff completion cap", []int64{0, 0, 0}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, reader, cutoff := openFillsUniverse(t, []string{poolA})
			insertFill(t, pool, 1, openFillsTrade(poolA, 101, cutoff.Unix()-600))
			for i, age := range tc.ages {
				insertOpenFill(t, pool, int64(i+2), openFillsTrade(poolA, int64(190+(i+2)%3), cutoff.Unix()-age))
			}
			in := freezeOpenFills(t, pool, reader, cutoff)
			if len(in.Candidates) != 1 || in.OpenFillPools != 1 || in.Candidates[0].SourceObservedAt != cutoff.Unix()-tc.want || in.Candidates[0].Pool.Volume24h != "1.000000" {
				t.Fatalf("oldest open fill: %+v", in)
			}
		})
	}
}

func TestProductionUniverseOpenFillsCutoffAndProjection(t *testing.T) {
	pool, reader, cutoff := openFillsUniverse(t, []string{poolA, poolB})
	insertFill(t, pool, 1, openFillsTrade(poolA, 101, cutoff.Unix()-600))
	insertOpenFill(t, pool, 2, openFillsTrade(poolA, 201, cutoff.Unix()-120))
	insertFill(t, pool, 3, openFillsTrade(poolB, 190, cutoff.Unix()-10))
	if _, err := pool.Exec(context.Background(), `UPDATE stats.accounting_market_commits SET projection_version='other-version' WHERE source_fill_id=$1`, fmt.Sprintf("0x%064x:0", 3)); err != nil {
		t.Fatal(err)
	}
	in := freezeOpenFills(t, pool, reader, cutoff)
	if len(in.Candidates) != 2 || in.OpenFillPools != 1 || in.Candidates[0].SourceObservedAt != cutoff.Unix()-2 || in.Candidates[1].SourceObservedAt != cutoff.Unix()-11 || in.Candidates[1].Pool.Volume24h != "0.000000" {
		t.Fatalf("cutoff/projection isolation: %+v", in)
	}
}

func TestProductionUniverseOpenFillsThousandPools(t *testing.T) {
	addresses := make([]string, 1000)
	for i := range addresses {
		addresses[i] = fmt.Sprintf("0x%040x", i+1)
	}
	pool, reader, cutoff := openFillsUniverse(t, addresses)
	for i, address := range addresses {
		insertFill(t, pool, int64(i+1), openFillsTrade(address, 101, cutoff.Unix()-600))
		if i < 400 {
			insertOpenFill(t, pool, int64(i+1001), openFillsTrade(address, 190, cutoff.Unix()-10))
		}
	}
	started := time.Now()
	in := freezeOpenFills(t, pool, reader, cutoff)
	elapsed := time.Since(started)
	if elapsed >= 5*time.Second || len(in.Candidates) != 1000 || in.OpenFillPools != 400 {
		t.Fatalf("1000-pool freeze: candidates=%d open=%d elapsed=%s", len(in.Candidates), in.OpenFillPools, elapsed)
	}
	for i, candidate := range in.Candidates {
		want := cutoff.Unix() - 2
		if i < 400 {
			want = cutoff.Unix() - 11
		}
		if candidate.Pool.PoolAddress != addresses[i] || candidate.SourceObservedAt != want || candidate.Pool.Volume24h != "1.000000" {
			t.Fatalf("pool %d committed figures/observation: %+v", i, candidate)
		}
	}
	plans := make(map[string]json.RawMessage)
	var plansCoherence json.RawMessage
	if err := pool.QueryRow(context.Background(), "EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) "+reader.coherenceQuery(), universeProjection, int64(200)).Scan(&plansCoherence); err != nil {
		t.Fatal(err)
	}
	plans["coherence"] = plansCoherence
	universe, err := reader.ExplainUniverse(context.Background(), pool, 200, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	plans["universe"] = universe
	for name, plan := range plans {
		if !json.Valid(plan) {
			t.Fatalf("invalid %s query plan: %s", name, plan)
		}
		t.Logf("%s query plan: %s", name, plan)
	}
	if directory := os.Getenv("RANKING_QUERY_PLAN_DIR"); directory != "" {
		report, err := json.Marshal(struct {
			FreezeSeconds float64                    `json:"freezeSeconds"`
			Plans         map[string]json.RawMessage `json:"plans"`
		}{elapsed.Seconds(), plans})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "open-fills.json"), report, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func CanonicalPublicationDatabase(t *testing.T, cutoff time.Time) *pgxpool.Pool {
	t.Helper()
	pool := productionUniverseDatabase(t)
	ctx := context.Background()
	insertCreation(t, pool, poolA, 100, cutoff.Unix()-90000)
	e18 := strings.Repeat("0", 18)
	insertFill(t, pool, 1, universeFill{pool: poolA, side: "buy", wallet: walletOne, height: 101, timestamp: cutoff.Unix() - 90000, usdl: "1000000000", token: "1000" + e18, paid: "1000000000", fee: "10000000"})
	insertFill(t, pool, 2, universeFill{pool: poolA, side: "buy", wallet: walletTwo, height: 160, timestamp: cutoff.Unix() - 600, usdl: "500000000", token: "400" + e18, paid: "500000000", fee: "5000000"})
	insertFill(t, pool, 3, universeFill{pool: poolA, side: "sell", wallet: walletTwo, height: 170, txIndex: 1, timestamp: cutoff.Unix() - 120, usdl: "130000000", token: "100" + e18, paid: "100" + e18, fee: "0"})
	for _, statement := range []string{
		`INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,'canonical','indexer-1',1,clock_timestamp()+interval '1 minute')`,
		`INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,'canonical',200,200)`,
	} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete,completed_at) VALUES(125,'canonical',200,$1,0,true,true,$2)`, fmt.Sprintf("0x%064x", 200), cutoff.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	return pool
}
