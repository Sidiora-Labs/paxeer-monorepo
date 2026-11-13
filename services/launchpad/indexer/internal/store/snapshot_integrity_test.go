package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

// Each savepoint inherits the enclosing repeatable-read snapshot.
type snapshotTransaction struct{ pgx.Tx }

func (s snapshotTransaction) BeginTx(ctx context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	return s.Tx.Begin(ctx)
}

func TestCanonicalSnapshotsRejectIncompleteAndChangedDurableFacts(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	ddl, err := delivery.SchemaDDL("indexer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	raw := lifecycleRaw()
	fence, err := st.AcquireLease(ctx, 125, "snapshot", "worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'snapshot',100,100)`); err != nil {
		t.Fatal(err)
	}
	batch := CanonicalBatch{ChainID: 125, StreamID: "snapshot", Owner: "worker", BatchID: "source", Fence: fence, PreviousHeight: 100, Blocks: []BlockCompleteness{{Height: 101, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}}, Records: []RawRecord{raw}, Outbox: []OutboxIntent{{TransactionHash: raw.TransactionHash, Kind: "Swap", Payload: json.RawMessage(`{"amount":"1"}`)}}}
	if err = st.CommitCanonical(ctx, batch, nil); err != nil {
		t.Fatal(err)
	}
	digest, err := RawDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	segment := ReplaySegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: batch.StreamID, FromHeight: 101, ToHeight: 101, Blocks: batch.Blocks, Records: batch.Records, Events: []ReplayEvent{{Raw: raw, RawDigest: digest, Kind: "Swap", Timestamp: 1700000000, Payload: batch.Outbox[0].Payload}}}
	wire, err := json.Marshal(segment)
	if err != nil {
		t.Fatal(err)
	}
	sha := delivery.Hash(wire)
	manifest := ArchiveManifest{FormatVersion: "raw-archive-v2", ChainID: 125, SourceStream: batch.StreamID, FromHeight: 101, ToHeight: 101, EventCount: 1, TypedCount: 1, ByteLength: int64(len(wire)), SHA256: sha, Bucket: "archive", ObjectKey: "indexer/raw/v2/snapshot/" + sha + ".json", Blocks: batch.Blocks}
	if err = st.CompleteArchive(ctx, manifest, segment); err != nil {
		t.Fatal(err)
	}
	request := ReplayRequest{SourceStream: "snapshot", Target: "charts", Generation: "g1", ProjectionRevision: "v1", FromHeight: 101, ToHeight: 101}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_replay_effects VALUES('charts','g1','v1','source',$1,101,'Swap',$2)`, digest, batch.Outbox[0].Payload); err != nil {
		t.Fatal(err)
	}
	reconcile := func(s *Store) error {
		ok, err := s.ReconcileCanonicalCommit(ctx, batch)
		if err == nil && !ok {
			return errors.New("canonical acknowledgement absent")
		}
		return err
	}
	measure := func(s *Store) error { _, err := s.MeasureCanonicalRange(ctx, "snapshot", 101, 101); return err }
	archive := func(s *Store) error { return s.CheckArchiveSource(ctx, segment) }
	complete := func(s *Store) error { return s.CompleteArchive(ctx, manifest, segment) }
	compare := func(s *Store) error { _, _, err := s.CompareReconstructionSegment(ctx, request, segment); return err }
	for _, call := range []func(*Store) error{reconcile, measure, archive, complete, compare} {
		if err = call(st); err != nil {
			t.Fatalf("original canonical facts %v", err)
		}
	}
	for _, tc := range []struct {
		name, sql string
		call      func(*Store) error
		want      error
	}{
		{"reconcile commit schema", `ALTER TABLE indexer.indexer_ingest_commits RENAME TO commits_unavailable`, reconcile, nil},
		{"reconcile completeness schema", `ALTER TABLE indexer.indexer_block_completeness RENAME TO blocks_unavailable`, reconcile, nil},
		{"reconcile missing block", `DELETE FROM indexer.indexer_block_completeness`, reconcile, ErrIntegrity},
		{"reconcile raw schema", `ALTER TABLE indexer.indexer_raw_logs RENAME TO raw_unavailable`, reconcile, nil},
		{"reconcile raw payload", `UPDATE indexer.indexer_raw_logs SET data=decode('02','hex')`, reconcile, ErrIntegrity},
		{"reconcile outbox schema", `ALTER TABLE indexer.indexer_ingest_outbox RENAME TO outbox_unavailable`, reconcile, nil},
		{"reconcile outbox payload", `UPDATE indexer.indexer_ingest_outbox SET payload='{}'`, reconcile, ErrIntegrity},
		{"reconcile watermark schema", `ALTER TABLE indexer.indexer_ingest_watermarks RENAME TO watermarks_unavailable`, reconcile, nil},
		{"reconcile watermark regression", `UPDATE indexer.indexer_ingest_watermarks SET typed_complete_height=100`, reconcile, ErrIntegrity},
		{"reconcile invalid batch", "", func(s *Store) error {
			invalid := batch
			invalid.Fence = 0
			_, err := s.ReconcileCanonicalCommit(ctx, invalid)
			return err
		}, nil},
		{"reconcile missing raw", `DELETE FROM indexer.indexer_ingest_outbox; DELETE FROM indexer.indexer_raw_logs`, reconcile, ErrIntegrity},
		{"reconcile null raw", `ALTER TABLE indexer.indexer_raw_logs ALTER COLUMN data DROP NOT NULL; UPDATE indexer.indexer_raw_logs SET data=NULL`, reconcile, nil},
		{"reconcile outbox scan", `ALTER TABLE indexer.indexer_ingest_outbox DROP CONSTRAINT indexer_ingest_outbox_pkey; ALTER TABLE indexer.indexer_ingest_outbox ALTER COLUMN event_kind DROP NOT NULL; UPDATE indexer.indexer_ingest_outbox SET event_kind=NULL`, reconcile, nil},
		{"measure block scan", `ALTER TABLE indexer.indexer_block_completeness ALTER COLUMN fetch_complete DROP NOT NULL; UPDATE indexer.indexer_block_completeness SET fetch_complete=NULL`, measure, nil},
		{"measure missing block", `DELETE FROM indexer.indexer_block_completeness`, measure, ErrWatermark},
		{"measure raw scan", `ALTER TABLE indexer.indexer_raw_logs ALTER COLUMN emitter DROP NOT NULL; UPDATE indexer.indexer_raw_logs SET emitter=NULL`, measure, nil},
		{"measure unsigned source index", `ALTER TABLE indexer.indexer_raw_logs DROP CONSTRAINT indexer_raw_logs_transaction_index_check; UPDATE indexer.indexer_raw_logs SET transaction_index=-1`, measure, nil},
		{"measure typed JSON", `ALTER TABLE indexer.indexer_ingest_outbox ALTER COLUMN payload DROP NOT NULL; UPDATE indexer.indexer_ingest_outbox SET payload=NULL`, measure, nil},
		{"measure completeness schema", `ALTER TABLE indexer.indexer_block_completeness RENAME TO blocks_unavailable`, measure, nil},
		{"measure decode incomplete", `UPDATE indexer.indexer_block_completeness SET required_decode_complete=false`, measure, ErrIntegrity},
		{"measure raw schema", `ALTER TABLE indexer.indexer_raw_logs RENAME TO raw_unavailable`, measure, nil},
		{"measure raw topics", `UPDATE indexer.indexer_raw_logs SET topics='{}'`, measure, nil},
		{"measure raw identity", `UPDATE indexer.indexer_raw_logs SET emitter='invalid'`, measure, nil},
		{"measure raw count", `UPDATE indexer.indexer_block_completeness SET raw_count=2`, measure, ErrIntegrity},
		{"measure outbox schema", `ALTER TABLE indexer.indexer_ingest_outbox RENAME TO outbox_unavailable`, measure, nil},
		{"measure projection schema", `ALTER TABLE indexer.indexer_accounting_fills RENAME TO fills_unavailable`, measure, nil},
		{"archive block mismatch", `UPDATE indexer.indexer_block_completeness SET required_decode_complete=false`, archive, ErrIntegrity},
		{"archive raw missing", `DELETE FROM indexer.indexer_ingest_outbox; DELETE FROM indexer.indexer_raw_logs`, archive, nil},
		{"archive raw mismatch", `UPDATE indexer.indexer_raw_logs SET data=decode('02','hex')`, archive, ErrIntegrity},
		{"archive completion schema", `ALTER TABLE indexer.indexer_archive_segments RENAME TO archives_unavailable`, complete, nil},
		{"archive completion conflict", `UPDATE indexer.indexer_archive_segments SET manifest='{}'`, complete, ErrIntegrity},
		{"compare raw schema", `ALTER TABLE indexer.indexer_raw_logs RENAME TO raw_unavailable`, compare, nil},
		{"compare raw topics", `UPDATE indexer.indexer_raw_logs SET topics='{}'`, compare, nil},
		{"compare raw conflict", `UPDATE indexer.indexer_raw_logs SET data=decode('02','hex')`, compare, ErrIntegrity},
		{"compare effect schema", `ALTER TABLE indexer.indexer_replay_effects RENAME TO effects_unavailable`, compare, nil},
		{"compare effect payload", `UPDATE indexer.indexer_replay_effects SET payload='{}'`, compare, ErrIntegrity},
		{"compare effect missing", `DELETE FROM indexer.indexer_replay_effects`, compare, nil},
		{"compare effect surplus", `INSERT INTO indexer.indexer_replay_effects SELECT target,generation,projection_revision,'extra',raw_digest,height,event_kind,payload FROM indexer.indexer_replay_effects`, compare, ErrIntegrity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			err = tc.call(NewFromDBTX(snapshotTransaction{tx}))
			if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("changed canonical facts admitted: %v", err)
			}
		})
	}
	for _, bounds := range [][2]string{{"zz/0", "0/1"}, {"0/zz", "0/1"}} {
		if _, err = WALBytesBetween(bounds[0], bounds[1]); err == nil {
			t.Fatal("invalid WAL quantity admitted")
		}
	}

}
