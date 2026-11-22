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

func TestProjectedBackfillCommitsEffectsWithoutAdvancingLiveWatermark(t *testing.T) {
	for _, mode := range []string{"complete", "missing watermark", "watermark behind checkpoint", "incomplete block", "block insert refused", "canonical projection refused"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			st, pool := lifecycleStore(t)
			scope := authorizeStoreOperation(t, pool, "backfill")
			if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_replay_authorizations SET target='indexer' WHERE authorization_id=$1`, scope.AuthorizationID); err != nil {
				t.Fatal(err)
			}
			raw := lifecycleRaw()
			request := ReplayRequest{JobID: "projected-history", SourceStream: "canonical", Target: "indexer", Generation: scope.Generation, ProjectionRevision: scope.Revision, Mode: "backfill", Requester: scope.Operator, AuthorizationID: scope.AuthorizationID, FromHeight: 101, ToHeight: 101, CheckpointHeight: 103, CheckpointHash: raw.BlockHash}
			if err := st.AdmitReplay(ctx, request); err != nil {
				t.Fatal(err)
			}
			if err := st.BindReplayCheckpoint(ctx, request, BlockCompleteness{Height: 103, Hash: raw.BlockHash, FetchComplete: true, RequiredDecodeComplete: true}); err != nil {
				t.Fatal(err)
			}
			job, err := st.AcquireReplay(ctx, request.JobID, "backfill-worker", 30*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			digest, err := RawDigest(raw)
			if err != nil {
				t.Fatal(err)
			}
			segment := ReplaySegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: request.SourceStream, FromHeight: 101, ToHeight: 101, Blocks: []BlockCompleteness{{Height: 101, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}}, Records: []RawRecord{raw}, Events: []ReplayEvent{{Raw: raw, RawDigest: digest, Kind: "MarketCreated", Timestamp: 1700000000, Payload: []byte(`{"pool":"created"}`)}}}
			if mode != "missing watermark" {
				height := int64(103)
				if mode == "watermark behind checkpoint" {
					height = 102
				}
				if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'canonical',$1,$1)`, height); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "incomplete block" {
				if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness VALUES(125,'canonical',101,$1,1,false,false,clock_timestamp())`, raw.BlockHash); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "block insert refused" {
				if _, err := pool.Exec(ctx, `ALTER TABLE indexer.indexer_block_completeness ADD CONSTRAINT backfill_height CHECK(height<>101)`); err != nil {
					t.Fatal(err)
				}
			}
			canonical := func(ctx context.Context, tx CanonicalTx) error {
				row := sqlcdb.InsertPoolParams{PoolAddress: raw.Emitter, TokenAddress: raw.Emitter, Creator: raw.Emitter, Optical: raw.Emitter, PoolID: "0x" + strings.Repeat("4", 64), CreatedAt: 1700000000, CreatedBlock: 101, TxHash: raw.TransactionHash}
				if err := tx.Store.InsertPool(ctx, row); err != nil {
					return err
				}
				if mode == "canonical projection refused" {
					return ErrIntegrity
				}
				return nil
			}
			project := func(_ context.Context, _ pgx.Tx, event ReplayEvent) (string, error) {
				return event.RawDigest, nil
			}
			for _, change := range []string{"mode", "target", "projection"} {
				changed := job
				fn := canonical
				switch change {
				case "mode":
					changed.Request.Mode = "replay"
				case "target":
					changed.Request.Target = "charts"
				case "projection":
					fn = nil
				}
				if err := st.CommitProjectedBackfillSegment(ctx, changed, segment, project, fn); !errors.Is(err, ErrIntegrity) {
					t.Fatalf("unbound projected backfill %s: %v", change, err)
				}
			}
			err = st.CommitProjectedBackfillSegment(ctx, job, segment, project, canonical)
			if mode == "complete" {
				if err != nil {
					t.Fatal(err)
				}
			} else if mode == "missing watermark" {
				if !errors.Is(err, pgx.ErrNoRows) {
					t.Fatalf("missing live checkpoint: %v", err)
				}
			} else if mode == "watermark behind checkpoint" {
				if !errors.Is(err, ErrWatermark) {
					t.Fatalf("incomplete live checkpoint: %v", err)
				}
			} else if mode == "block insert refused" {
				if err == nil || !strings.Contains(err.Error(), "backfill_height") {
					t.Fatalf("block constraint: %v", err)
				}
			} else if !errors.Is(err, ErrIntegrity) {
				t.Fatalf("unbound projection: %v", err)
			}
			var status string
			var progress int64
			var rawCount, effectCount, poolCount int
			if err := pool.QueryRow(ctx, `SELECT status,progress_height,(SELECT count(*) FROM indexer.indexer_raw_logs),(SELECT count(*) FROM indexer.indexer_replay_effects),(SELECT count(*) FROM indexer.pools) FROM indexer.indexer_replay_jobs WHERE job_id=$1`, request.JobID).Scan(&status, &progress, &rawCount, &effectCount, &poolCount); err != nil {
				t.Fatal(err)
			}
			if mode == "complete" {
				if status != "completed" || progress != 101 || rawCount != 1 || effectCount != 1 || poolCount != 1 {
					t.Fatalf("committed backfill: %s/%d rows=%d/%d/%d", status, progress, rawCount, effectCount, poolCount)
				}
				var height int64
				var complete bool
				if err := pool.QueryRow(ctx, `SELECT typed_complete_height,(SELECT fetch_complete AND required_decode_complete FROM indexer.indexer_block_completeness WHERE stream_id='canonical' AND height=101) FROM indexer.indexer_ingest_watermarks WHERE stream_id='canonical'`).Scan(&height, &complete); err != nil || height != 103 || !complete {
					t.Fatalf("live watermark changed: %d/%v, %v", height, complete, err)
				}
				if err := st.CommitProjectedBackfillSegment(ctx, job, segment, project, canonical); !errors.Is(err, ErrFenced) {
					t.Fatalf("completed replay reused its lease: %v", err)
				}
			} else if status != "running" || progress != 100 || rawCount != 0 || effectCount != 0 || poolCount != 0 {
				t.Fatalf("failed projection advanced state: %s/%d rows=%d/%d/%d", status, progress, rawCount, effectCount, poolCount)
			}
		})
	}
}
