package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestCanonicalProjectionRollsBackUnavailableAndConflictingFacts(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	fence, err := st.AcquireLease(ctx, 125, "atomic", "worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'atomic',100,100)`); err != nil {
		t.Fatal(err)
	}
	raw := lifecycleRaw()
	raw.Topics = nil
	raw.Data = nil
	original := CanonicalBatch{ChainID: 125, StreamID: "atomic", Owner: "worker", BatchID: "source", Fence: fence, PreviousHeight: 100, Blocks: []BlockCompleteness{{Height: 101, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}}, Records: []RawRecord{raw}, Outbox: []OutboxIntent{{TransactionHash: raw.TransactionHash, Kind: "Swap", Payload: json.RawMessage(`{"amount":"1"}`)}}}
	for _, mode := range []string{"bound", "lease table", "watermark table", "raw table", "raw acknowledgement", "outbox table", "outbox acknowledgement", "outbox conflict", "block table", "commit table", "projection mismatch", "projection invalid", "watermark update", "commit update"} {
		t.Run(mode, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			s := NewFromDBTX(tx)
			b := original
			exec := func(sql string, args ...any) {
				if _, err := tx.Exec(ctx, sql, args...); err != nil {
					t.Fatal(err)
				}
			}
			noWrite := func(table, operation string) {
				exec(`CREATE FUNCTION pg_temp.reject_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$`)
				exec(`CREATE TRIGGER reject_write BEFORE ` + operation + ` ON indexer.` + table + ` FOR EACH ROW EXECUTE FUNCTION pg_temp.reject_write()`)
			}
			switch mode {
			case "lease table":
				exec(`ALTER TABLE indexer.indexer_ingest_leases RENAME TO leases_unavailable`)
			case "watermark table":
				exec(`ALTER TABLE indexer.indexer_ingest_watermarks RENAME TO watermarks_unavailable`)
			case "raw table":
				exec(`ALTER TABLE indexer.indexer_raw_logs RENAME TO raw_unavailable`)
			case "raw acknowledgement":
				noWrite("indexer_raw_logs", "INSERT")
			case "outbox table":
				exec(`ALTER TABLE indexer.indexer_ingest_outbox RENAME TO outbox_unavailable`)
			case "outbox acknowledgement":
				noWrite("indexer_ingest_outbox", "INSERT")
			case "outbox conflict":
				if err = stageImmutableRaw(ctx, tx, 125, raw); err != nil {
					t.Fatal(err)
				}
				exec(`INSERT INTO indexer.indexer_ingest_outbox(chain_id,transaction_hash,log_index,event_kind,payload) VALUES(125,$1,0,'Swap','{}')`, raw.TransactionHash)
			case "block table":
				exec(`ALTER TABLE indexer.indexer_block_completeness RENAME TO blocks_unavailable`)
			case "commit table":
				exec(`ALTER TABLE indexer.indexer_ingest_commits RENAME TO commits_unavailable`)
			case "projection mismatch":
				b.Outbox = []OutboxIntent{}
			case "watermark update":
				exec(`CREATE FUNCTION pg_temp.reject_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'watermark unavailable'; END $$`)
				exec(`CREATE TRIGGER reject_update BEFORE UPDATE ON indexer.indexer_ingest_watermarks FOR EACH ROW EXECUTE FUNCTION pg_temp.reject_update()`)
			case "commit update":
				exec(`CREATE FUNCTION pg_temp.reject_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'commit unavailable'; END $$`)
				exec(`CREATE TRIGGER reject_insert BEFORE INSERT ON indexer.indexer_ingest_commits FOR EACH ROW EXECUTE FUNCTION pg_temp.reject_insert()`)
			}
			err = s.CommitProjectedCanonical(ctx, b, func(_ context.Context, _ *Store) ([]OutboxIntent, error) {
				if mode == "projection invalid" {
					return []OutboxIntent{{}}, nil
				}
				return original.Outbox, nil
			})
			if mode == "bound" {
				if err != nil {
					t.Fatal(err)
				}
				height, e := s.GetCanonicalWatermark(ctx, 125, "atomic")
				if e != nil || height != 101 {
					t.Fatalf("projected watermark %d %v", height, e)
				}
			} else if err == nil {
				t.Fatal("unavailable or conflicting projection acknowledged")
			}
		})
	}
}

func TestReplaySegmentsRollBackChangedDurableBindings(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	raw := lifecycleRaw()
	digest, err := RawDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	request := ReplayRequest{JobID: "atomic-replay", SourceStream: "replay-atomic", Target: "charts", Generation: "g1", ProjectionRevision: "v1", Mode: "backfill", Requester: "operator", AuthorizationID: "atomic-approval", FromHeight: 101, ToHeight: 101, CheckpointHeight: 101, CheckpointHash: raw.BlockHash}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_replay_authorizations VALUES($1,$2,$3,$4,$5,$6,$7,101,101,1,clock_timestamp()+interval '1 hour',NULL)`, request.AuthorizationID, request.Requester, strings.Repeat("a", 64), request.Mode, request.Target, request.Generation, request.ProjectionRevision); err != nil {
		t.Fatal(err)
	}
	if err = st.AdmitReplay(ctx, request); err != nil {
		t.Fatal(err)
	}
	block := BlockCompleteness{Height: 101, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}
	if err = st.BindReplayCheckpoint(ctx, request, block); err != nil {
		t.Fatal(err)
	}
	job, err := st.AcquireReplay(ctx, request.JobID, "worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	segment := ReplaySegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: request.SourceStream, FromHeight: 101, ToHeight: 101, Blocks: []BlockCompleteness{block}, Records: []RawRecord{raw}, Events: []ReplayEvent{{RawDigest: digest, Raw: raw, Kind: "Swap", Timestamp: 1700000000, Payload: json.RawMessage(`{"amount":"1"}`)}}}
	for _, mode := range []string{"bound", "job table", "job fence", "authorization table", "poison table", "checkpoint table", "checkpoint conflict", "canonical table", "canonical conflict", "source table", "source acknowledgement", "source conflict", "raw table", "effect table", "effect acknowledgement", "effect conflict", "batch table", "job update", "projector failure"} {
		t.Run(mode, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			s := NewFromDBTX(tx)
			exec := func(sql string, args ...any) {
				if _, err := tx.Exec(ctx, sql, args...); err != nil {
					t.Fatal(err)
				}
			}
			noWrite := func(table string) {
				exec(`CREATE FUNCTION pg_temp.reject_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$`)
				exec(`CREATE TRIGGER reject_write BEFORE INSERT ON indexer.` + table + ` FOR EACH ROW EXECUTE FUNCTION pg_temp.reject_write()`)
			}
			switch mode {
			case "job table":
				exec(`ALTER TABLE indexer.indexer_replay_jobs RENAME TO jobs_unavailable`)
			case "job fence":
				exec(`UPDATE indexer.indexer_replay_jobs SET fence=fence+1`)
			case "authorization table":
				exec(`ALTER TABLE indexer.indexer_replay_authorizations RENAME TO authorizations_unavailable`)
			case "poison table":
				exec(`ALTER TABLE indexer.indexer_quarantine RENAME TO poison_unavailable`)
			case "checkpoint table":
				exec(`ALTER TABLE indexer.indexer_replay_checkpoints RENAME TO checkpoints_unavailable`)
			case "checkpoint conflict":
				exec(`UPDATE indexer.indexer_replay_checkpoints SET block_hash=$1`, "0x"+strings.Repeat("9", 64))
			case "canonical table":
				exec(`ALTER TABLE indexer.indexer_block_completeness RENAME TO blocks_unavailable`)
			case "canonical conflict":
				exec(`INSERT INTO indexer.indexer_block_completeness VALUES(125,$1,101,$2,1,true,true)`, request.SourceStream, "0x"+strings.Repeat("9", 64))
			case "source table":
				exec(`ALTER TABLE indexer.indexer_replay_source_blocks RENAME TO source_unavailable`)
			case "source acknowledgement":
				noWrite("indexer_replay_source_blocks")
			case "source conflict":
				exec(`INSERT INTO indexer.indexer_replay_source_blocks VALUES($1,101,$2,2)`, request.SourceStream, raw.BlockHash)
			case "raw table":
				exec(`ALTER TABLE indexer.indexer_raw_logs RENAME TO raw_unavailable`)
			case "effect table":
				exec(`ALTER TABLE indexer.indexer_replay_effects RENAME TO effects_unavailable`)
			case "effect acknowledgement":
				noWrite("indexer_replay_effects")
			case "effect conflict":
				exec(`INSERT INTO indexer.indexer_replay_effects VALUES('charts','g1','v1','source',$1,101,'Swap','{}')`, digest)
			case "batch table":
				exec(`ALTER TABLE indexer.indexer_replay_batches RENAME TO batches_unavailable`)
			case "job update":
				exec(`CREATE FUNCTION pg_temp.reject_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'job unavailable'; END $$`)
				exec(`CREATE TRIGGER reject_update BEFORE UPDATE ON indexer.indexer_replay_jobs FOR EACH ROW EXECUTE FUNCTION pg_temp.reject_update()`)
			}
			err = s.CommitReplaySegment(ctx, job, segment, func(ctx context.Context, tx pgx.Tx, event ReplayEvent) (string, error) {
				if mode == "projector failure" {
					return "", errors.New("projection unavailable")
				}
				_, err := tx.Exec(ctx, `INSERT INTO indexer.indexer_generation_raw VALUES('charts','g1','v1',101,0,0,$1,$2,$3)`, raw.TransactionHash, digest, event.Payload)
				return "source", err
			})
			if mode == "bound" {
				if err != nil {
					t.Fatal(err)
				}
				var status string
				if err = tx.QueryRow(ctx, `SELECT status FROM indexer.indexer_replay_jobs WHERE job_id=$1`, request.JobID).Scan(&status); err != nil || status != "completed" {
					t.Fatalf("replayed state %s %v", status, err)
				}
			} else if err == nil {
				t.Fatal("changed replay binding acknowledged")
			}
		})
	}
	var progress int64
	if err = pool.QueryRow(ctx, `SELECT progress_height FROM indexer.indexer_replay_jobs WHERE job_id=$1`, request.JobID).Scan(&progress); err != nil || progress != 100 {
		t.Fatalf("rolled back replay progress %d %v", progress, err)
	}
}
