package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestLifecycleOperationsReturnDatabaseErrorsWithoutPartialWrites(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	ddl, err := delivery.SchemaDDL("indexer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	raw := lifecycleRaw()
	digest, err := RawDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	waiver := authorizeStoreOperation(t, pool, "waive")
	repair := authorizeStoreOperation(t, pool, "repair")
	replay := authorizeStoreOperation(t, pool, "replay")
	retention := authorizeStoreOperation(t, pool, "retention")
	request := ReplayRequest{JobID: "database-replay", SourceStream: "canonical", Target: replay.Target, Generation: replay.Generation, ProjectionRevision: replay.Revision, Mode: "replay", Requester: replay.Operator, AuthorizationID: replay.AuthorizationID, FromHeight: 101, ToHeight: 101, CheckpointHeight: 103, CheckpointHash: raw.BlockHash}
	poison := PoisonRecord{ChainID: 125, StreamID: "canonical", Raw: raw, ErrorCode: "arity", ABIRevision: "abi-v1", Requirement: "accounting", Required: true}
	if err := st.RetainPoison(ctx, poison); err != nil {
		t.Fatal(err)
	}
	if err := st.AdmitReplay(ctx, request); err != nil {
		t.Fatal(err)
	}
	fence, err := st.AcquireLease(ctx, 125, "canonical", "worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	candidate := CreationSnapshotCandidate{PoolAddress: raw.Emitter, PoolID: raw.Emitter, TransactionHash: raw.TransactionHash, Height: 101, Timestamp: 1700000000}
	snapshot := sqlcdb.InsertPoolStateSnapshotParams{PoolID: candidate.PoolID, BlockNumber: candidate.Height, BlockTimestamp: candidate.Timestamp, TxHash: candidate.TransactionHash}
	cases := []struct {
		name, table string
		call        func(context.Context) error
	}{
		{"retain raw", "indexer_raw_logs", func(c context.Context) error { return st.RetainPoison(c, poison) }},
		{"retain quarantine", "indexer_quarantine", func(c context.Context) error { return st.RetainPoison(c, poison) }},
		{"retain capability", "indexer_capability_degradation", func(c context.Context) error {
			optional := poison
			optional.StreamID = "optional"
			optional.Required = false
			return st.RetainPoison(c, optional)
		}},
		{"waiver insert", "indexer_quarantine_waivers", func(c context.Context) error {
			return st.WaivePoison(c, waiver, digest, poison.Requirement, "bounded input", time.Now().Add(time.Minute))
		}},
		{"repair update", "indexer_quarantine", func(c context.Context) error { return st.ResolvePoison(c, repair, digest, raw.BlockHash) }},
		{"replay admission", "indexer_replay_jobs", func(c context.Context) error { return st.AdmitReplay(c, request) }},
		{"replay lease", "indexer_replay_jobs", func(c context.Context) error {
			_, e := st.AcquireReplay(c, request.JobID, "worker", time.Second)
			return e
		}},
		{"replay authority", "indexer_replay_authorizations", func(c context.Context) error {
			_, e := st.AcquireReplay(c, request.JobID, "worker", time.Second)
			return e
		}},
		{"operation authority", "indexer_replay_authorizations", func(c context.Context) error { return st.CheckOperationAuthorization(c, replay) }},
		{"retention coverage", "indexer_archive_segments", func(c context.Context) error { _, e := st.RetentionEligible(c, retention); return e }},
		{"head retention", "indexer_committed_heads", func(c context.Context) error {
			return st.RetainCommittedHead(c, "canonical", 101, raw.BlockHash, "source")
		}},
		{"head quarantine", "indexer_committed_heads", func(c context.Context) error { return st.QuarantineCommittedHead(c, "canonical") }},
		{"ready lease", "indexer_ingest_leases", func(c context.Context) error { return st.IngestReady(c, 125, "canonical", "worker", fence) }},
		{"snapshot lease", "indexer_ingest_leases", func(c context.Context) error {
			return st.CommitCreationSnapshot(c, 125, "canonical", "worker", fence, candidate, BlockCompleteness{}, false, nil)
		}},
		{"snapshot lookup", "pool_state_snapshots", func(c context.Context) error {
			return st.CommitCreationSnapshot(c, 125, "canonical", "worker", fence, candidate, BlockCompleteness{}, false, &snapshot)
		}},
		{"snapshot progress", "backfill_jobs", func(c context.Context) error {
			return st.CommitCreationSnapshot(c, 125, "canonical", "worker", fence, candidate, BlockCompleteness{}, false, nil)
		}},
		{"measurement outbox", "delivery_outbox", func(c context.Context) error { _, e := st.MeasureIngestStorage(c, "indexer"); return e }},
		{"measurement indexes", "indexer_accounting_fills", func(c context.Context) error { _, e := st.MeasureIngestStorage(c, "indexer"); return e }},
		{"measurement blocks", "indexer_block_completeness", func(c context.Context) error { _, e := st.MeasureCanonicalRange(c, "canonical", 101, 101); return e }},
		{"measurement plan", "indexer_raw_logs", func(c context.Context) error {
			_, e := st.MeasureIngestPlans(c, "canonical", raw, raw.Emitter)
			return e
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, `ALTER TABLE indexer.`+tc.table+` RENAME TO unavailable_relation`); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := pool.Exec(ctx, `ALTER TABLE indexer.unavailable_relation RENAME TO `+tc.table); err != nil {
					t.Fatal(err)
				}
			}()
			err := tc.call(ctx)
			var databaseError *pgconn.PgError
			if !errors.As(err, &databaseError) || databaseError.Code != "42P01" {
				t.Fatalf("database read/write error: %v", err)
			}
		})
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	for _, tc := range cases {
		t.Run("canceled/"+tc.name, func(t *testing.T) {
			if err := tc.call(canceled); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled operation: %v", err)
			}
		})
	}
	var optional, jobs int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM indexer.indexer_capability_degradation),(SELECT count(*) FROM indexer.backfill_jobs)`).Scan(&optional, &jobs); err != nil || optional != 0 || jobs != 0 {
		t.Fatalf("failed writes retained state: %d/%d, %v", optional, jobs, err)
	}
}

func TestCanonicalTransactionsRequireEpochAuthority(t *testing.T) {
	ctx := context.Background()
	for _, db := range []*canonicalEpochDB{nil, {}} {
		if _, err := db.BeginTx(ctx, pgx.TxOptions{}); err == nil {
			t.Fatal("unbound epoch transaction admitted")
		}
	}
	for _, tx := range []*canonicalEpochTx{nil, {}} {
		if err := tx.Commit(ctx); err == nil {
			t.Fatal("unbound epoch commit admitted")
		}
	}
	if _, err := NewLifecycleStore(nil, nil); err == nil {
		t.Fatal("unbound lifecycle store admitted")
	}
	if _, err := (&Store{}).MeasureCanonicalRange(ctx, "canonical", 101, 101); err == nil || !strings.Contains(err.Error(), "snapshot transaction") {
		t.Fatalf("unbound measurement: %v", err)
	}
	if _, err := (&Store{}).MeasureIngestStorage(ctx, "bad schema"); err == nil {
		t.Fatal("invalid delivery schema admitted")
	}
	if _, err := canonicalPayload([]byte(`{`)); err == nil {
		t.Fatal("malformed projection payload admitted")
	}
	for _, lsn := range [][2]string{{"not-lsn", "0/0"}, {"x/0", "0/0"}, {"0/x", "0/0"}, {"0/0", "x/0"}} {
		if _, err := WALBytesBetween(lsn[0], lsn[1]); err == nil {
			t.Fatalf("invalid WAL coordinate %v", lsn)
		}
	}
}
