package store_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	v1 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v1"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

// The committed read transcripts are the source database peer, not the
// CanonicalInputReader, FrozenInputs, pgx codec or ranker being exercised.
// Query text and bindings must match; every unknown statement is rejected by
// the existing transactional in-memory PostgreSQL protocol fixture.
type repair931Read struct {
	SQL  string
	Args []string
	Rows [][]string
}

func TestRepairM_931_PublicationCategories(t *testing.T) {
	t.Run("rollback-and-completed-empty-recovery", TestRepair_32_HeavyRunRollbackAndEmptyCompletion)
	t.Run("immutable-heavy-membership", TestRepair_32_RejectIncompleteOrMismatchedHeavyRuns)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "publication.json")
	pool := repair32OpenPool(t, path)
	defer pool.Close()
	guard, err := queue.NewEpochGuard("recovery", 1, "fixture-loopback")
	if err != nil {
		t.Fatal(err)
	}
	publication, err := store.NewPublicationStore(pool, "ranking", guard)
	if err != nil {
		t.Fatal(err)
	}
	snapshots, err := ranker.ComputeRun(repair32PopulatedInput(t), false)
	if err != nil {
		t.Fatal(err)
	}
	repair931Membership(t, snapshots, ranker.HeavyCategoryNames())
	if err = publication.Complete(ctx, snapshots); err != nil {
		t.Fatal(err)
	}
	if err = publication.Complete(ctx, snapshots); err != nil {
		t.Fatal("idempotent completion", err)
	}
	writer := delivery.StreamWriter{Schema: "api_stream"}
	for range snapshots {
		worked, e := publication.CommitStreamOnce(ctx, writer, 3)
		if e != nil || !worked {
			t.Fatalf("real stream commit: %v %v", worked, e)
		}
	}
	if worked, e := publication.CommitStreamOnce(ctx, writer, 3); e != nil || worked {
		t.Fatalf("outbox not drained: %v %v", worked, e)
	}
	for _, snapshot := range snapshots {
		var sequence int64
		var raw []byte
		if err = pool.QueryRow(ctx, "SELECT sequence,frame FROM api_stream.stream_projection WHERE topic=$1", "market.rankings."+snapshot.Category()).Scan(&sequence, &raw); err != nil {
			t.Fatal(err)
		}
		if _, err = delivery.ParseCommittedStreamFrame(raw); err != nil {
			t.Fatal(err)
		}
		var frame v1.RankingEvent
		if json.Unmarshal(raw, &frame) != nil || sequence != 1 || frame.Topic != "market.rankings."+snapshot.Category() || frame.Payload.SnapshotId != snapshot.RunID() {
			t.Fatalf("wrong committed category identity: %s", raw)
		}
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM ranking.publication_outbox").Scan(&count); err != nil || count != len(snapshots) {
		t.Fatalf("intents=%d %v", count, err)
	}
}

func TestRepairM_931_ProductionUniverseCategories(t *testing.T) {
	// This committed fixture drives every producer category through real
	// persistence, the stream writer, frame parser and HTTP stored-page consumer,
	// stopping and reopening the pool/store while intents are pending.
	TestRepair_32_CoordinatorDurableStreamAndHTTPAcrossRestart(t)
}

func repair931Membership(t *testing.T, snapshots []ranker.Snapshot, want []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, snapshot := range snapshots {
		if seen[snapshot.Category()] {
			t.Fatalf("duplicate %s", snapshot.Category())
		}
		seen[snapshot.Category()] = true
	}
	expected := map[string]bool{}
	for _, category := range want {
		expected[category] = true
	}
	if !reflect.DeepEqual(seen, expected) {
		t.Fatalf("categories=%v want=%v", seen, expected)
	}
}

func repair931HistoricalSamples(t *testing.T, count int, step time.Duration) []ranker.HistoricalReplaySample {
	t.Helper()
	directory := t.TempDir()
	cutoff := time.Now().UTC().Truncate(time.Second).Add(-time.Duration(count-1) * step)
	var samples []ranker.HistoricalReplaySample
	for i := 0; i < count; i++ {
		in := repair32PopulatedInput(t)
		in.CutoffTimeUTC = cutoff.Add(time.Duration(i) * step).Unix()
		in.Provenance.Watermark.ObservedAt = time.Unix(in.CutoffTimeUTC, 0).UTC().Format("2006-01-02T15:04:05.000Z")
		in.Provenance.WindowStart, in.Provenance.WindowEnd = in.CutoffTimeUTC-86400, in.CutoffTimeUTC
		for j := range in.Candidates {
			c := &in.Candidates[j]
			c.SourceObservedAt, c.ModerationObservedAt = in.CutoffTimeUTC, in.CutoffTimeUTC
			c.Activity.WindowStart, c.Activity.WindowEnd = in.CutoffTimeUTC-86400, in.CutoffTimeUTC
		}
		heavy, err := ranker.ComputeRun(in, false)
		if err != nil {
			t.Fatal(err)
		}
		newest, err := ranker.ComputeRun(in, true)
		if err != nil {
			t.Fatal(err)
		}
		all := append(heavy, newest...)
		repair931Membership(t, all, ranker.Categories[:])
		for _, snapshot := range all {
			if len(snapshot.Entries()) == 0 {
				t.Fatalf("historical %s fixture must exercise scoring", snapshot.Category())
			}
			raw, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, fmt.Sprintf("%d-%s.json", i, snapshot.Category()))
			if err = os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(raw)
			samples = append(samples, ranker.HistoricalReplaySample{Cutoff: in.CutoffTimeUTC, Category: snapshot.Category(), Original: ranker.BoundFile{Path: path, SHA256: hex.EncodeToString(digest[:])}})
		}
	}
	return samples
}

func TestRepairM_931_HistoricalReplayCategories(t *testing.T) {
	samples := repair931HistoricalSamples(t, 14, 6*time.Hour)
	cutoff := time.Unix(samples[0].Cutoff, 0)
	if err := store.RepairM931Replay(t, samples); err != nil {
		t.Fatalf("all canonical historical categories rejected: %v", err)
	}
	t.Run("legacy-six-category-subset", func(t *testing.T) {
		var subset []ranker.HistoricalReplaySample
		for _, sample := range samples {
			if sample.Category != "gainers" && sample.Category != "losers" {
				subset = append(subset, sample)
			}
		}
		if err := store.RepairM931Replay(t, subset); err == nil {
			t.Fatal("obsolete six-category evidence admitted")
		}
	})
	t.Run("missing-at-one-cutoff", func(t *testing.T) {
		if err := store.RepairM931Replay(t, samples[1:]); err == nil {
			t.Fatal("missing category admitted")
		}
	})
	t.Run("duplicate-category", func(t *testing.T) {
		duplicate := append([]ranker.HistoricalReplaySample(nil), samples...)
		duplicate[1] = duplicate[0]
		if err := store.RepairM931Replay(t, duplicate); err == nil {
			t.Fatal("duplicate admitted")
		}
	})
	t.Run("sampling-gap", func(t *testing.T) {
		var gap []ranker.HistoricalReplaySample
		for _, sample := range samples {
			if sample.Cutoff != cutoff.Add(36*time.Hour).Unix() {
				gap = append(gap, sample)
			}
		}
		// Thirteen complete cutoffs remain across 78 hours, so this reaches
		// the six-hour sampling-gap check rather than the minimum-size guard.
		if err := store.RepairM931Replay(t, gap); err == nil || !strings.Contains(err.Error(), "gap") {
			t.Fatalf("historical gap not rejected by gap gate: %v", err)
		}
	})
	t.Run("less-than-72-hours", func(t *testing.T) {
		short := repair931HistoricalSamples(t, 14, 5*time.Hour)
		if err := store.RepairM931Replay(t, short); err == nil || !strings.Contains(err.Error(), "span") {
			t.Fatalf("short span was not rejected by span gate: %v", err)
		}
	})
	t.Run("recomputed-output-differs", func(t *testing.T) {
		changed := append([]ranker.HistoricalReplaySample(nil), samples...)
		raw, err := os.ReadFile(changed[0].Original.Path)
		if err != nil {
			t.Fatal(err)
		}
		var artifact struct {
			Input        ranker.InputSnapshot `json:"input"`
			Category     string               `json:"category"`
			Entries      []ranker.Scored      `json:"entries"`
			Excluded     []ranker.Exclusion   `json:"excluded"`
			InputDigest  string               `json:"inputDigest"`
			OutputDigest string               `json:"outputDigest"`
		}
		if err = json.Unmarshal(raw, &artifact); err != nil {
			t.Fatal(err)
		}
		artifact.Entries[0].Score += 0.01
		entries, err := json.Marshal(artifact.Entries)
		if err != nil {
			t.Fatal(err)
		}
		output := sha256.Sum256(entries)
		artifact.OutputDigest = hex.EncodeToString(output[:])
		raw, err = json.Marshal(artifact)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(raw)
		path := filepath.Join(t.TempDir(), "changed.json")
		if err = os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		changed[0].Original = ranker.BoundFile{Path: path, SHA256: hex.EncodeToString(hash[:])}
		if _, err = ranker.ReadComparisonArtifact(raw, changed[0].Original.SHA256); err != nil {
			t.Fatalf("fixture must pass artifact integrity before replay: %v", err)
		}
		if err = store.RepairM931Replay(t, changed); err == nil || !strings.Contains(err.Error(), "replay differs") {
			t.Fatalf("changed computed output was not rejected by replay equality: %v", err)
		}
	})
	t.Run("digest-mismatch", func(t *testing.T) {
		changed := append([]ranker.HistoricalReplaySample(nil), samples...)
		changed[0].Original.SHA256 = strings.Repeat("0", 64)
		if err := store.RepairM931Replay(t, changed); err == nil {
			t.Fatal("unbound artifact admitted")
		}
	})
}

func TestRepairM_931_PublicationHealthCategories(t *testing.T) {
	cutoff := time.Now().UTC().Truncate(time.Second)
	snapshots, err := ranker.ComputeRun(repair32Input(), false)
	if err != nil {
		t.Fatal(err)
	}
	newest, err := ranker.ComputeRun(repair32Input(), true)
	if err != nil {
		t.Fatal(err)
	}
	all := append(snapshots, newest...)
	for _, test := range []struct {
		name    string
		missing string
		ages    map[string]int64
		since   int64
		stale   bool
	}{
		{name: "all-eight-seven-heavy", since: 909, stale: true},
		{name: "omitted-last-alphabetic-heavy", missing: "volume"},
		{name: "omitted-unusual-heavy", missing: "unusual"},
		{name: "missing-new-still-complete-heavy", missing: "new", since: 909, stale: true},
		{name: "volume-fresh", ages: map[string]int64{"volume": 30}},
		{name: "unusual-fresh", ages: map[string]int64{"unusual": 90}},
		{name: "different-heavy-stale-ages", ages: map[string]int64{"volume": 500}, since: 409, stale: true},
		{name: "latest-just-became-stale", ages: map[string]int64{"volume": 91}, since: 0, stale: false},
		{name: "five-minute-boundary", ages: map[string]int64{"volume": 391}, since: 300, stale: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := repair32State{Snapshots: map[string]*repair32Stored{}, Heads: map[string]string{}, Frames: map[string]json.RawMessage{}}
			for _, snapshot := range all {
				category := snapshot.Category()
				if category == test.missing {
					continue
				}
				age := int64(1000)
				if value, exists := test.ages[category]; exists {
					age = value
				}
				key := snapshot.RunID() + "/" + category
				state.Heads[category] = key
				state.Snapshots[key] = &repair32Stored{Header: ranker.Header{Category: category, CutoffTime: cutoff.Unix() - age}}
			}
			raw, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "health.json")
			if err = os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			pool := repair32OpenPool(t, path)
			defer pool.Close()
			storage, err := store.NewSnapshotStore(pool, "ranking")
			if err != nil {
				t.Fatal(err)
			}
			health, err := storage.PublicationHealth(context.Background(), cutoff, 3)
			if err != nil {
				t.Fatal(err)
			}
			if len(health.CategoryAges) != len(state.Heads) {
				t.Fatalf("category ages truncated: %+v", health)
			}
			for category := range state.Heads {
				age := int64(1000)
				if value, exists := test.ages[category]; exists {
					age = value
				}
				if health.CategoryAges[category] != age {
					t.Fatalf("%s age=%d want=%d", category, health.CategoryAges[category], age)
				}
			}
			if health.AllHeavyStaleForFiveMinutes != test.stale {
				t.Fatalf("whole-heavy staleness: %+v", health)
			}
			expectSince := test.since > 0 || test.name == "latest-just-became-stale"
			if (health.AllHeavyStaleSince != nil) != expectSince {
				t.Fatalf("missing/fresh heavy falsely counted: %+v", health)
			}
			if expectSince && !health.AllHeavyStaleSince.Equal(cutoff.Add(-time.Duration(test.since)*time.Second)) {
				t.Fatalf("stale transition=%v", health.AllHeavyStaleSince)
			}
		})
	}
	t.Run("invalid-cutoff", func(t *testing.T) {
		for _, invalid := range []int64{0, cutoff.Unix() + 1, math.MaxInt64} {
			path := filepath.Join(t.TempDir(), "invalid.json")
			state := repair32State{Snapshots: map[string]*repair32Stored{"x": {Header: ranker.Header{Category: "volume", CutoffTime: invalid}}}, Heads: map[string]string{"volume": "x"}}
			raw, _ := json.Marshal(state)
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			pool := repair32OpenPool(t, path)
			storage, err := store.NewSnapshotStore(pool, "ranking")
			if err != nil {
				t.Fatal(err)
			}
			_, err = storage.PublicationHealth(context.Background(), cutoff, 3)
			pool.Close()
			if err == nil {
				t.Fatalf("invalid cutoff %d admitted", invalid)
			}
		}
	})
}

func repair931FrozenPool(t *testing.T, cutoff time.Time, evidence *store.HTTPCanonicalEvidence) (*pgxpool.Pool, *store.FrozenInputs) {
	t.Helper()
	const address = "0x00000000000000000000000000000000000000a1"
	visibilityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(store.RankingVisibility{Revision: strings.Repeat("e", 64), ObservedAt: cutoff, Complete: true, Pools: map[string]bool{address: true}})
	}))
	t.Cleanup(visibilityServer.Close)
	visibility, err := store.NewHTTPRankingVisibility(visibilityServer.URL, visibilityServer.Client())
	if err != nil {
		t.Fatal(err)
	}
	reader, err := store.NewCanonicalInputReader("indexer", "stats", "canonical", "accounting-v2", visibility)
	if err != nil {
		t.Fatal(err)
	}
	reader.Evidence = evidence
	universeSQL := store.RepairM931UniverseSQL(reader)
	for _, required := range []string{"round_trips AS", "SUM(2*LEAST(b,s))", "(c.fill->'position'->>'height')::numeric<=$2", "(c.fill->>'blockTimestampSeconds')::bigint<=$3", "WHERE ts>$3-86400"} {
		if !strings.Contains(universeSQL, required) {
			t.Fatalf("canonical query missing cutoff/heuristic separation %s", required)
		}
	}
	if strings.Contains(universeSQL, "selfv AS") {
		t.Fatal("round trips still labelled as self-trade evidence")
	}
	// Committed canonical read transcript: ordinary buy 500 USDL and partial
	// sell 130 USDL, at heights 160 and 170 in a complete height-200 history.
	reads := []repair931Read{
		{SQL: `SELECT w.raw_complete_height,w.typed_complete_height,l.expires_at>clock_timestamp() FROM "indexer"."indexer_ingest_watermarks_v1" w JOIN "indexer"."indexer_ingest_leases_v1" l USING(chain_id,stream_id) WHERE w.chain_id=125 AND w.stream_id=$1`, Args: []string{"canonical"}, Rows: [][]string{{"200", "200", "t"}}},
		{SQL: `SELECT block_hash,completed_at,fetch_complete AND required_decode_complete FROM "indexer"."indexer_block_completeness_v1" WHERE chain_id=125 AND stream_id=$1 AND height=$2`, Args: []string{"canonical", "200"}, Rows: [][]string{{fmt.Sprintf("0x%064x", 200), cutoff.Add(-time.Second).Format("2006-01-02 15:04:05Z07:00"), "t"}}},
		{SQL: `SELECT NOT EXISTS(SELECT 1 FROM "stats"."accounting_pool_fences_v2" WHERE frozen) AND NOT EXISTS(SELECT 1 FROM "indexer"."indexer_accounting_fills_v1" f WHERE (f.payload->'position'->>'height')::numeric<=$2 AND NOT EXISTS(SELECT 1 FROM "stats"."accounting_market_commits_v2" c WHERE c.projection_version=$1 AND c.source_fill_id=f.source_fill_id AND c.source_fact_digest=f.source_fact_digest AND c.fill=f.payload)) AND NOT EXISTS(SELECT 1 FROM "stats"."accounting_market_commits_v2" c WHERE c.projection_version=$1 AND (c.fill->'position'->>'height')::numeric<=$2 AND NOT EXISTS(SELECT 1 FROM "indexer"."indexer_accounting_fills_v1" f WHERE f.source_fill_id=c.source_fill_id AND f.source_fact_digest=c.source_fact_digest AND f.payload=c.fill))`, Args: []string{"accounting-v2", "200"}, Rows: [][]string{{"t"}}},
		{SQL: universeSQL, Args: []string{"accounting-v2", "200", strconv.FormatInt(cutoff.Unix(), 10)}, Rows: [][]string{{address, "100", "0", "0", strconv.FormatInt(cutoff.Unix()-90000, 10), "630000000", "630000000", "130000000", "1", "1", "1", "260000000", "1400000000000000000000", "100000000000000000000", "1485000000", "130000000", "2", "130000000", "100000000000000000000", "18", "1000000000", "1000000000000000000000", "18"}}},
	}
	pool := repair32OpenPool(t, filepath.Join(t.TempDir(), "canonical.json"), reads...)
	frozen, err := store.NewFrozenInputs(pool, reader)
	if err != nil {
		pool.Close()
		t.Fatal(err)
	}
	return pool, frozen
}

func TestRepairM_931_CanonicalPauseAndRecoveredEvidence(t *testing.T) {
	cutoff := time.Now().UTC().Truncate(time.Second).Add(-time.Second)
	const address = "0x00000000000000000000000000000000000000a1"
	paused, unpaused := true, false
	for _, test := range []struct {
		name     string
		evidence *bool
		include  bool
		eligible bool
	}{
		{name: "paused-before-cutoff", evidence: &paused, include: true},
		{name: "missing-pause", include: true},
		{name: "missing-pool"},
		{name: "recovered-unpaused", evidence: &unpaused, include: true, eligible: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("cutoffHeight") != "200" || r.URL.Query().Get("cutoffTime") != strconv.FormatInt(cutoff.Unix(), 10) || r.URL.Query().Get("blockHash") != fmt.Sprintf("0x%064x", 200) {
					t.Error("unbound evidence request")
					w.WriteHeader(400)
					return
				}
				pools := map[string]store.CanonicalPoolEvidence{}
				if test.include {
					pools[address] = store.CanonicalPoolEvidence{Paused: test.evidence, PauseEvidence: strings.Repeat("a", 64)}
				}
				_ = json.NewEncoder(w).Encode(store.CanonicalRankingEvidence{Revision: strings.Repeat("d", 64), CutoffHeight: 200, CutoffTime: cutoff.Unix(), BlockHash: fmt.Sprintf("0x%064x", 200), Pools: pools})
			}))
			defer server.Close()
			evidence, err := store.NewHTTPCanonicalEvidence(server.URL, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			// Close and reopen the actual pool/reader to prove recovered observations
			// are read again rather than inferred from an earlier unpaused default.
			for reopen := 0; reopen < 2; reopen++ {
				pool, frozen := repair931FrozenPool(t, cutoff, evidence)
				in, err := frozen.Freeze(context.Background(), uuid.NewString(), cutoff)
				pool.Close()
				if err != nil {
					t.Fatalf("canonical freeze: %v", err)
				}
				if !in.Complete || len(in.Candidates) != 1 {
					t.Fatalf("incomplete universe: %+v", in)
				}
				c := in.Candidates[0]
				if c.Paused == test.eligible {
					t.Fatalf("eligibility defaulted unknown to unpaused: %+v", c)
				}
				if (c.NullableMetrics.Paused == nil) != (test.evidence == nil) {
					t.Fatalf("pause observation lost: %+v", c.NullableMetrics)
				}
				if test.evidence != nil && *c.NullableMetrics.Paused != *test.evidence {
					t.Fatal("pause observation changed")
				}
				heavy, err := ranker.ComputeRun(in, false)
				if err != nil {
					t.Fatal(err)
				}
				newest, err := ranker.ComputeRun(in, true)
				if err != nil {
					t.Fatal(err)
				}
				for _, snapshot := range append(heavy, newest...) {
					if test.eligible && snapshot.Category() != "movers" && len(snapshot.Entries()) != 1 {
						t.Fatalf("recovered %s pool not eligible: %+v", snapshot.Category(), snapshot.Exclusions())
					}
					if !test.eligible && len(snapshot.Entries()) != 0 {
						t.Fatalf("unknown/paused pool reached %s", snapshot.Category())
					}
				}
			}
		})
	}
	t.Run("unwired-evidence-remains-unknown", func(t *testing.T) {
		pool, frozen := repair931FrozenPool(t, cutoff, nil)
		defer pool.Close()
		in, err := frozen.Freeze(context.Background(), uuid.NewString(), cutoff)
		if err != nil {
			t.Fatal(err)
		}
		if !in.Candidates[0].Paused || in.Candidates[0].NullableMetrics.Paused != nil {
			t.Fatal("missing canonical authority admitted pool")
		}
	})
}

func TestRepairM_931_CanonicalVolumeEvidence(t *testing.T) {
	store.TestProductionUniverseActivityEvidence(t)
	cutoff := time.Now().UTC().Truncate(time.Second).Add(-time.Second)
	const address = "0x00000000000000000000000000000000000000a1"
	unpaused := false
	for _, test := range []struct{ name, volume, want string }{
		{name: "normal-buy-partial-sell", want: "630.000000"},
		{name: "independent-canonical-wash-proof", volume: "260000000", want: "370.000000"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				evidence := store.CanonicalPoolEvidence{Paused: &unpaused, PauseEvidence: strings.Repeat("a", 64)}
				if test.volume != "" {
					evidence.SelfVolume = &store.CanonicalSelfVolume{Raw: test.volume, WindowStart: cutoff.Unix() - 86400, WindowEnd: cutoff.Unix(), Evidence: strings.Repeat("b", 64)}
				}
				_ = json.NewEncoder(w).Encode(store.CanonicalRankingEvidence{Revision: strings.Repeat("d", 64), CutoffHeight: 200, CutoffTime: cutoff.Unix(), BlockHash: fmt.Sprintf("0x%064x", 200), Pools: map[string]store.CanonicalPoolEvidence{address: evidence}})
			}))
			defer server.Close()
			evidence, err := store.NewHTTPCanonicalEvidence(server.URL, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			pool, frozen := repair931FrozenPool(t, cutoff, evidence)
			defer pool.Close()
			in, err := frozen.Freeze(context.Background(), uuid.NewString(), cutoff)
			if err != nil {
				t.Fatal(err)
			}
			c := in.Candidates[0]
			if c.Activity.RawVolume != "630.000000" || c.Pool.Volume24h != "630.000000" || c.Activity.AdjustedVolume != test.want {
				t.Fatalf("raw facts changed or incorrect discount: %+v", c.Activity)
			}
			if contains931(c.Activity.Flags, "evidenced-self-volume") != (test.volume != "") || !contains931(c.Activity.Flags, "round-trip-activity") {
				t.Fatalf("proof and heuristic conflated: %+v", c.Activity)
			}
			if contains931(c.NullableMetrics.Unavailable, "canonical-self-volume-unavailable") != (test.volume == "") {
				t.Fatal("unknown self evidence not exposed")
			}
			heavy, err := ranker.ComputeRun(in, false)
			if err != nil {
				t.Fatal(err)
			}
			repair931Membership(t, heavy, ranker.HeavyCategoryNames())
			// The normal wallet must remain eligible all the way into production
			// scoring; independently proven wash discounts the score, not raw facts.
			for _, snapshot := range heavy {
				if snapshot.Category() == "volume" && (len(snapshot.Entries()) != 1 || snapshot.Entries()[0].Score <= 0) {
					t.Fatal("valid activity never reached volume ranking")
				}
			}
		})
	}
}
func contains931(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestRepairM_931_RejectUnboundCanonicalEvidence(t *testing.T) {
	cutoff := time.Now().UTC().Truncate(time.Second).Add(-time.Second)
	const address = "0x00000000000000000000000000000000000000a1"
	unpaused := false
	for _, test := range []struct {
		name   string
		mutate func(*store.CanonicalRankingEvidence)
		status int
	}{
		{name: "other-height", mutate: func(e *store.CanonicalRankingEvidence) { e.CutoffHeight++ }},
		{name: "after-cutoff", mutate: func(e *store.CanonicalRankingEvidence) { e.CutoffTime++ }},
		{name: "before-cutoff", mutate: func(e *store.CanonicalRankingEvidence) { e.CutoffTime-- }},
		{name: "other-fork", mutate: func(e *store.CanonicalRankingEvidence) { e.BlockHash = "0x" + strings.Repeat("a", 64) }},
		{name: "missing-revision", mutate: func(e *store.CanonicalRankingEvidence) { e.Revision = "" }},
		{name: "missing-observations", mutate: func(e *store.CanonicalRankingEvidence) { e.Pools = nil }},
		{name: "pause-without-proof", mutate: func(e *store.CanonicalRankingEvidence) {
			p := e.Pools[address]
			p.PauseEvidence = ""
			e.Pools[address] = p
		}},
		{name: "self-volume-without-proof", mutate: func(e *store.CanonicalRankingEvidence) { e.Pools[address].SelfVolume.Evidence = "" }},
		{name: "self-volume-wrong-window", mutate: func(e *store.CanonicalRankingEvidence) { e.Pools[address].SelfVolume.WindowStart++ }},
		{name: "self-volume-after-cutoff", mutate: func(e *store.CanonicalRankingEvidence) { e.Pools[address].SelfVolume.WindowEnd++ }},
		{name: "negative-volume", mutate: func(e *store.CanonicalRankingEvidence) { e.Pools[address].SelfVolume.Raw = "-1" }},
		{name: "noncanonical-volume", mutate: func(e *store.CanonicalRankingEvidence) { e.Pools[address].SelfVolume.Raw = "01" }},
		{name: "volume-exceeds-raw", mutate: func(e *store.CanonicalRankingEvidence) { e.Pools[address].SelfVolume.Raw = "630000001" }},
		{name: "source-unavailable", status: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.status != 0 {
					w.WriteHeader(test.status)
					return
				}
				response := store.CanonicalRankingEvidence{Revision: strings.Repeat("d", 64), CutoffHeight: 200, CutoffTime: cutoff.Unix(), BlockHash: fmt.Sprintf("0x%064x", 200), Pools: map[string]store.CanonicalPoolEvidence{address: {Paused: &unpaused, PauseEvidence: strings.Repeat("a", 64), SelfVolume: &store.CanonicalSelfVolume{Raw: "260000000", WindowStart: cutoff.Unix() - 86400, WindowEnd: cutoff.Unix(), Evidence: strings.Repeat("b", 64)}}}}
				test.mutate(&response)
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			evidence, err := store.NewHTTPCanonicalEvidence(server.URL, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			pool, frozen := repair931FrozenPool(t, cutoff, evidence)
			defer pool.Close()
			if in, err := frozen.Freeze(context.Background(), uuid.NewString(), cutoff); err == nil {
				t.Fatalf("unqualified evidence froze a complete run: %+v", in)
			}
		})
	}
}

func TestRepairM_931_PauseEvidenceRecovery(t *testing.T) {
	cutoff := time.Now().UTC().Truncate(time.Second).Add(-time.Second)
	const address = "0x00000000000000000000000000000000000000a1"
	// Each stage is immutable until the corresponding request and freeze finish.
	// Reopen both the evidence HTTP client and database reader between stages.
	for stage := 0; stage < 3; stage++ {
		func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				pools := map[string]store.CanonicalPoolEvidence{}
				if stage > 0 {
					paused := stage == 1
					pools[address] = store.CanonicalPoolEvidence{Paused: &paused, PauseEvidence: strings.Repeat("a", 64)}
				}
				_ = json.NewEncoder(w).Encode(store.CanonicalRankingEvidence{Revision: strings.Repeat("d", 64), CutoffHeight: 200, CutoffTime: cutoff.Unix(), BlockHash: fmt.Sprintf("0x%064x", 200), Pools: pools})
			}))
			defer server.Close()
			evidence, err := store.NewHTTPCanonicalEvidence(server.URL, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			pool, frozen := repair931FrozenPool(t, cutoff, evidence)
			in, err := frozen.Freeze(context.Background(), uuid.NewString(), cutoff)
			pool.Close()
			if err != nil {
				t.Fatal(err)
			}
			c := in.Candidates[0]
			if (c.NullableMetrics.Paused == nil) != (stage == 0) || c.Paused != (stage < 2) {
				t.Fatalf("stage %d recovered wrong pause state: %+v", stage, c.NullableMetrics)
			}
			newest, err := ranker.ComputeRun(in, true)
			if err != nil {
				t.Fatal(err)
			}
			if (len(newest[0].Entries()) == 1) != (stage == 2) {
				t.Fatalf("stage %d discovery eligibility differs", stage)
			}
		}()
	}
}

// The broker peer speaks the committed NATS publish/ack transcript over a
// loopback socket. The actual nats client, JetStream publisher, outbox and
// acknowledgement checks run unchanged. Stored producer frames survive reopen.
type repair931BrokerMessage struct {
	ID, Topic string
	Wire      []byte
	Sequence  uint64
}
type repair931Broker struct {
	t        *testing.T
	listener net.Listener
	path     string
	badAck   bool
	mu       sync.Mutex
	messages []repair931BrokerMessage
}

func repair931OpenBroker(t *testing.T, path string, badAck bool) *repair931Broker {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	broker := &repair931Broker{t: t, listener: listener, path: path, badAck: badAck}
	if raw, err := os.ReadFile(path); err == nil {
		if err = json.Unmarshal(raw, &broker.messages); err != nil {
			t.Fatal(err)
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go broker.serve(conn)
		}
	}()
	return broker
}
func (b *repair931Broker) serve(conn net.Conn) {
	defer conn.Close()
	if _, err := fmt.Fprintf(conn, "INFO {\"server_id\":\"repair-m931\",\"server_name\":\"repair-m931\",\"version\":\"2.10.0\",\"proto\":1,\"headers\":true,\"max_payload\":1048576}\r\n"); err != nil {
		return
	}
	reader := bufio.NewReader(conn)
	subscriptions := map[string]string{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			b.t.Error("empty NATS command")
			return
		}
		switch fields[0] {
		case "CONNECT", "PONG", "UNSUB":
		case "PING":
			if _, err = io.WriteString(conn, "PONG\r\n"); err != nil {
				return
			}
		case "SUB":
			if len(fields) != 3 {
				b.t.Errorf("unexpected subscription: %s", line)
				return
			}
			subscriptions[fields[2]] = fields[1]
		case "HPUB":
			if len(fields) != 5 {
				b.t.Errorf("unexpected publish: %s", line)
				return
			}
			headerSize, e1 := strconv.Atoi(fields[3])
			size, e2 := strconv.Atoi(fields[4])
			if e1 != nil || e2 != nil || size < headerSize || size > 1<<20 {
				b.t.Error("unbounded publication")
				return
			}
			wire := make([]byte, size+2)
			if _, err = io.ReadFull(reader, wire); err != nil {
				return
			}
			if !bytes.Equal(wire[size:], []byte("\r\n")) {
				b.t.Error("invalid NATS framing")
				return
			}
			headers := textproto.NewReader(bufio.NewReader(bytes.NewReader(wire[:headerSize])))
			if version, err := headers.ReadLine(); err != nil || version != "NATS/1.0" {
				b.t.Error("invalid NATS headers")
				return
			}
			header, err := headers.ReadMIMEHeader()
			if err != nil {
				b.t.Error(err)
				return
			}
			id := header.Get("Nats-Msg-Id")
			if id == "" || header.Get("Nats-Expected-Stream") != "RANKING" {
				b.t.Error("producer identity/stream not bound")
				return
			}
			payload := wire[headerSize:size]
			var event v1.RankingEvent
			if v1.ValidateJSON("RankingEvent", payload) != nil || json.Unmarshal(payload, &event) != nil || event.EventId != id || event.Topic != fields[1] {
				b.t.Errorf("invalid actual producer frame: %s", payload)
				return
			}
			b.mu.Lock()
			sequence := uint64(0)
			duplicate := false
			for _, message := range b.messages {
				if message.ID == id {
					sequence = message.Sequence
					duplicate = true
					if !bytes.Equal(message.Wire, payload) || message.Topic != fields[1] {
						b.t.Error("retry changed immutable producer frame")
					}
					break
				}
			}
			if !duplicate {
				sequence = uint64(len(b.messages) + 1)
				b.messages = append(b.messages, repair931BrokerMessage{ID: id, Topic: fields[1], Wire: append([]byte(nil), payload...), Sequence: sequence})
				raw, err := json.Marshal(b.messages)
				if err != nil {
					b.t.Error(err)
					b.mu.Unlock()
					return
				}
				if err = os.WriteFile(b.path, raw, 0600); err != nil {
					b.t.Error(err)
					b.mu.Unlock()
					return
				}
			}
			b.mu.Unlock()
			stream := "RANKING"
			if b.badAck {
				stream = "OTHER"
			}
			ack, _ := json.Marshal(map[string]any{"stream": stream, "seq": sequence, "duplicate": duplicate})
			sid := ""
			for id, subject := range subscriptions {
				if subject == fields[2] || strings.HasSuffix(subject, ".*") && strings.HasPrefix(fields[2], strings.TrimSuffix(subject, "*")) {
					sid = id
					break
				}
			}
			if sid == "" {
				b.t.Error("publish reply has no live subscription")
				return
			}
			if _, err = fmt.Fprintf(conn, "MSG %s %s %d\r\n%s\r\n", fields[2], sid, len(ack), ack); err != nil {
				return
			}
		default:
			b.t.Errorf("unsupported NATS transcript command %s", fields[0])
			return
		}
	}
}
func repair931Publisher(t *testing.T, broker *repair931Broker) (*nats.Conn, *natsdriver.Publisher) {
	t.Helper()
	conn, err := nats.Connect("nats://"+broker.listener.Addr().String(), nats.NoReconnect(), nats.Timeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := natsdriver.NewPublisher(conn, "RANKING")
	if err != nil {
		conn.Close()
		t.Fatal(err)
	}
	return conn, publisher
}

func TestRepairM_931_PublicationAcknowledgementsAndExhaustion(t *testing.T) {
	ctx := context.Background()
	snapshots, err := ranker.ComputeRun(repair32Input(), false)
	if err != nil {
		t.Fatal(err)
	}
	repair931Membership(t, snapshots, ranker.HeavyCategoryNames())
	guard, err := queue.NewEpochGuard("recovery", 1, "fixture-loopback")
	if err != nil {
		t.Fatal(err)
	}
	for _, exhaust := range []bool{false, true} {
		t.Run(fmt.Sprintf("exhaustion=%v", exhaust), func(t *testing.T) {
			directory := t.TempDir()
			databasePath, brokerPath := filepath.Join(directory, "publication.json"), filepath.Join(directory, "broker.json")
			pool := repair32OpenPool(t, databasePath)
			publication, err := store.NewPublicationStore(pool, "ranking", guard)
			if err != nil {
				t.Fatal(err)
			}
			if err = publication.Complete(ctx, snapshots); err != nil {
				t.Fatal(err)
			}
			broker := repair931OpenBroker(t, brokerPath, true)
			conn, publisher := repair931Publisher(t, broker)
			attempts := 1
			if exhaust {
				attempts = 2 * len(snapshots)
			}
			for i := 0; i < attempts; i++ {
				if worked, err := publication.DispatchOnce(ctx, publisher, "RANKING", 2); err == nil || worked {
					t.Fatalf("foreign-stream persistence ack admitted: %v %v", worked, err)
				}
			}
			conn.Close()
			_ = broker.listener.Close()
			pool.Close()
			// Stop and reopen both peers and the production publisher/store. The
			// accepted broker message and the unacknowledged intent must keep identity.
			pool = repair32OpenPool(t, databasePath)
			defer pool.Close()
			publication, err = store.NewPublicationStore(pool, "ranking", guard)
			if err != nil {
				t.Fatal(err)
			}
			broker = repair931OpenBroker(t, brokerPath, false)
			defer broker.listener.Close()
			conn, publisher = repair931Publisher(t, broker)
			defer conn.Close()
			if exhaust {
				if worked, err := publication.DispatchOnce(ctx, publisher, "RANKING", 2); err != nil || worked {
					t.Fatalf("exhausted intent retried: %v %v", worked, err)
				}
				health, err := publication.PublicationHealth(ctx, time.Now(), 2)
				if err != nil || health.Exhausted != int64(len(snapshots)) || health.Pending != int64(len(snapshots)) {
					t.Fatalf("exhaustion lost on reopen: %+v %v", health, err)
				}
			} else {
				for range snapshots {
					if worked, err := publication.DispatchOnce(ctx, publisher, "RANKING", 2); err != nil || !worked {
						t.Fatalf("ack recovery failed: %v %v", worked, err)
					}
				}
				if worked, err := publication.DispatchOnce(ctx, publisher, "RANKING", 2); err != nil || worked {
					t.Fatalf("acknowledged intent replayed: %v %v", worked, err)
				}
				pool.Close()
				var state repair32State
				raw, err := os.ReadFile(databasePath)
				if err != nil || json.Unmarshal(raw, &state) != nil {
					t.Fatal("read durable ack", err)
				}
				for _, intent := range state.Outbox {
					if !intent.Published || intent.BrokerStream != "RANKING" || intent.BrokerSequence < 1 {
						t.Fatalf("persistence acknowledgement not durable: %+v", intent)
					}
				}
			}
			broker.mu.Lock()
			messages := append([]repair931BrokerMessage(nil), broker.messages...)
			broker.mu.Unlock()
			if len(messages) != len(snapshots) {
				t.Fatalf("broker message count=%d want=%d", len(messages), len(snapshots))
			}
			seen := map[string]bool{}
			for _, message := range messages {
				seen[strings.TrimPrefix(message.Topic, "market.rankings.")] = true
			}
			for _, snapshot := range snapshots {
				if !seen[snapshot.Category()] {
					t.Fatalf("broker omitted %s", snapshot.Category())
				}
			}
		})
	}
}
