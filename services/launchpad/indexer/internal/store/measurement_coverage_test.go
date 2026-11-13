package store

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestCanonicalMeasurementReadsCommittedRangeAndPlans(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, pool := canonicaltest.New(t)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("migration path unavailable")
	}
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../../migrations/60001.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	ddl, err := delivery.SchemaDDL("indexer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	st := New(pool)
	before, err := st.MeasureIngestStorage(ctx, "indexer")
	if err != nil || before.Database == "" || before.WALInsertLSN == "" || len(before.IndexDefinitions) == 0 {
		t.Fatalf("storage before commit: %+v %v", before, err)
	}
	if _, err := st.MeasureIngestStorage(ctx, "unsafe-name"); err == nil {
		t.Fatal("unbound delivery schema accepted")
	}
	fence, err := st.AcquireLease(ctx, 125, "measurement", "worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	hash := "0x" + strings.Repeat("1", 64)
	raw := RawRecord{TransactionHash: "0x" + strings.Repeat("2", 64), BlockHash: hash, Emitter: "0x" + strings.Repeat("3", 40), Height: 101, Topics: []string{}, Data: []byte{1}}
	batch := CanonicalBatch{ChainID: 125, StreamID: "measurement", Owner: "worker", BatchID: "batch-101", Fence: fence, PreviousHeight: 100, Blocks: []BlockCompleteness{{Height: 101, Hash: hash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}}, Records: []RawRecord{raw}, Outbox: []OutboxIntent{{TransactionHash: raw.TransactionHash, Kind: "Swap", Payload: []byte(`{"amount":1}`)}}}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'measurement',100,100)`); err != nil {
		t.Fatal(err)
	}
	if err := st.CommitCanonical(ctx, batch, nil); err != nil {
		t.Fatal(err)
	}
	after, err := st.MeasureIngestStorage(ctx, "indexer")
	if err != nil || after.Database != before.Database || after.WALInsertLSN == before.WALInsertLSN {
		t.Fatalf("storage after commit: %+v %v", after, err)
	}
	wal, err := WALBytesBetween(before.WALInsertLSN, after.WALInsertLSN)
	if err != nil || wal == 0 {
		t.Fatalf("WAL change: %d %v", wal, err)
	}
	digest, err := st.MeasureCanonicalRange(ctx, "measurement", 101, 101)
	if err != nil || digest.RawCount != 1 || digest.TypedCount != 1 || digest.BlockCount != 1 || len(digest.RawSHA256) != 64 || len(digest.TypedSHA256) != 64 {
		t.Fatalf("canonical digest: %+v %v", digest, err)
	}
	for _, bounds := range [][2]int64{{102, 101}, {-1, 101}, {101, 1000101}} {
		if _, err := st.MeasureCanonicalRange(ctx, "measurement", bounds[0], bounds[1]); err == nil {
			t.Fatalf("unbounded canonical range accepted: %v", bounds)
		}
	}
	if _, err := st.MeasureCanonicalRange(ctx, "measurement", 100, 102); err == nil {
		t.Fatal("incomplete canonical range accepted")
	}
	plans, err := st.MeasureIngestPlans(ctx, "measurement", raw, "0x"+strings.Repeat("4", 40))
	if err != nil || len(plans) != 5 {
		t.Fatalf("ingest plans: %d %v", len(plans), err)
	}
	for _, plan := range plans {
		if plan.Name == "" || len(plan.Plan) == 0 {
			t.Fatalf("empty query plan: %+v", plan)
		}
	}
}

func TestCanonicalPayloadAndWALBoundaries(t *testing.T) {
	for _, tc := range []struct {
		input, want string
	}{
		{`{"b":2,"a":1}`, `{"a":1,"b":2}`},
		{`[1,2,3]`, `[1,2,3]`},
	} {
		got, err := canonicalPayload([]byte(tc.input))
		if err != nil || string(got) != tc.want {
			t.Fatalf("canonical payload %q: %q %v", tc.input, got, err)
		}
	}
	if _, err := canonicalPayload([]byte(`{"a":`)); err == nil {
		t.Fatal("incomplete JSON accepted")
	}
	if delta, err := WALBytesBetween("0/FFFF", "0/10010"); err != nil || delta != 17 {
		t.Fatalf("WAL boundary: %d %v", delta, err)
	}
	for _, tc := range [][2]string{{"bad", "1/10"}, {"1/10", "bad"}, {"1/10", "0/FFFF"}} {
		if _, err := WALBytesBetween(tc[0], tc[1]); err == nil {
			t.Fatalf("invalid WAL interval accepted: %v", tc)
		}
	}
}
