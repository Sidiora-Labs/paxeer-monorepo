package l16

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type checkpointRevisionFixture struct {
	ctx      context.Context
	service  *Service
	pool     *pgxpool.Pool
	consumer *AccountingConsumer
	frames   []delivery.Envelope
}

func newCheckpointRevisionFixture(t *testing.T, stored bool) checkpointRevisionFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	service, pool, frames := currentRecordedInventoryFrames(t, ctx)
	if len(frames) != 2 {
		t.Fatalf("fill count = %d, want 2", len(frames))
	}
	f := checkpointRevisionFixture{ctx: ctx, service: service, pool: pool, consumer: portfolioReplayConsumer(t, service, pool), frames: frames}
	if stored {
		var publication accounting.PublicationV1
		if err := json.Unmarshal(frames[0].Payload, &publication); err != nil {
			t.Fatal(err)
		}
		source := publication.Positions[0].SourceOrder
		repository := l17.Repository{Pool: pool}
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer inventoryRollback(tx)
		if _, err = tx.Exec(ctx, "SET LOCAL search_path TO pnl,pg_catalog"); err != nil {
			t.Fatal(err)
		}
		for revision := 1; revision <= 3; revision++ {
			watermark := l17.Watermark{ChainID: "125", Height: source.Height, BlockHash: source.BlockHash, ProjectionRevision: strconv.Itoa(revision), ObservedAt: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), Integrity: statusHealthy}
			summary, err := l17.Aggregate(realMarketCreator, "", watermark, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = repository.CommitSnapshotTx(ctx, tx, "watched-portfolio:"+strconv.Itoa(revision), summary, nil, time.Now().Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var inventory, snapshots int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM pnl.inventory_l16),(SELECT count(*) FROM pnl.portfolio_snapshots WHERE wallet=$1)`, realMarketCreator).Scan(&inventory, &snapshots); err != nil {
		t.Fatal(err)
	}
	wantSnapshots := 0
	if stored {
		wantSnapshots = 3
	}
	if inventory != 0 || snapshots != wantSnapshots {
		t.Fatalf("before fills: inventory=%d snapshots=%d, want 0 and %d", inventory, snapshots, wantSnapshots)
	}
	if stored {
		var revisions []string
		if err := pool.QueryRow(ctx, `SELECT array_agg(revision::text ORDER BY revision) FROM pnl.portfolio_snapshots WHERE wallet=$1`, realMarketCreator).Scan(&revisions); err != nil {
			t.Fatal(err)
		}
		if len(revisions) != 3 || revisions[0] != "1" || revisions[1] != "2" || revisions[2] != "3" {
			t.Fatal(revisions)
		}
	}
	return f
}

func (f checkpointRevisionFixture) consume(t *testing.T, index int) {
	t.Helper()
	consumeCurrentInventoryFrames(t, f.ctx, f.service, f.frames[index:index+1], f.consumer)
}

func (f checkpointRevisionFixture) assertRevision(t *testing.T, frameRevision, positionRevision string, snapshots int) delivery.StreamUpdate {
	t.Helper()
	var raw []byte
	if err := f.pool.QueryRow(f.ctx, `SELECT frame FROM api_stream.stream_projection WHERE topic=$1`, "private.portfolio."+realMarketCreator).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var frame delivery.StreamUpdate
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatal(err)
	}
	var update interfaces.PortfolioUpdate
	if err := json.Unmarshal(frame.Payload, &update); err != nil {
		t.Fatal(err)
	}
	if frame.Revision != frameRevision || string(update.SummaryRevision) != frameRevision || string(update.Summary.Watermark.ProjectionRevision) != frameRevision || !update.CompleteBatch || len(update.Changes) != 1 || update.Changes[0].Position == nil {
		t.Fatalf("frame=%+v update=%+v", frame, update)
	}
	change := update.Changes[0]
	if string(change.Revision) != positionRevision || string(change.Position.SourceWatermark.ProjectionRevision) != positionRevision {
		t.Fatal(change)
	}
	var watermark interfaces.Watermark
	if err := json.Unmarshal(frame.Watermark, &watermark); err != nil || string(watermark.ProjectionRevision) != frameRevision {
		t.Fatal(watermark, err)
	}
	row := readPortfolioReplayRow(t, f.ctx, f.pool, f.consumer.generation)
	assertPortfolioReplayState(t, update, row)
	if strconv.FormatInt(row.Revision, 10) != positionRevision {
		t.Fatal(row)
	}
	var revision, snapshotRevision, snapshotPositionRevision string
	var count int
	if err := f.pool.QueryRow(f.ctx, `SELECT revision::text,summary->'watermark'->>'projectionRevision',(SELECT payload->'sourceWatermark'->>'projectionRevision' FROM pnl.portfolio_snapshot_rows WHERE snapshot_id=s.id) FROM pnl.portfolio_snapshots s WHERE id=$1`, update.SnapshotId).Scan(&revision, &snapshotRevision, &snapshotPositionRevision); err != nil {
		t.Fatal(err)
	}
	if revision != frameRevision || snapshotRevision != frameRevision || snapshotPositionRevision != positionRevision {
		t.Fatal(revision, snapshotRevision, snapshotPositionRevision)
	}
	if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM pnl.portfolio_snapshots WHERE wallet=$1`, realMarketCreator).Scan(&count); err != nil || count != snapshots {
		t.Fatal(count, err)
	}
	return frame
}

func TestCheckpointRevisionFirstFillAfterStoredSnapshots(t *testing.T) {
	f := newCheckpointRevisionFixture(t, true)
	f.consume(t, 0)
	f.assertRevision(t, "4", "1", 4)
}

func TestCheckpointRevisionSecondFill(t *testing.T) {
	f := newCheckpointRevisionFixture(t, true)
	f.consume(t, 0)
	f.assertRevision(t, "4", "1", 4)
	f.consume(t, 1)
	f.assertRevision(t, "5", "2", 5)
}

func TestCheckpointRevisionWithoutStoredSnapshots(t *testing.T) {
	f := newCheckpointRevisionFixture(t, false)
	f.consume(t, 0)
	f.assertRevision(t, "1", "1", 1)
	f.consume(t, 1)
	f.assertRevision(t, "2", "2", 2)
}

func (f checkpointRevisionFixture) assertRejected(t *testing.T, frame delivery.StreamUpdate, revision any, summaryRevision, proofSequence, message string) {
	t.Helper()
	var update interfaces.PortfolioUpdate
	if err := json.Unmarshal(frame.Payload, &update); err != nil {
		t.Fatal(err)
	}
	update.SummaryRevision = interfaces.PositiveUInt256(summaryRevision)
	payload, err := json.Marshal(update)
	if err != nil {
		t.Fatal(err)
	}
	frame.Payload = payload
	row := readPortfolioReplayRow(t, f.ctx, f.pool, f.consumer.generation)
	proof := delivery.StreamSourceProof{Kind: "portfolio_commit", Wallet: realMarketCreator, Token: realMarketToken, ProjectionVersion: f.consumer.generation, Sequence: proofSequence, SourceID: row.Position.EffectIdentity}
	if err := f.pool.QueryRow(f.ctx, `SELECT leg_identity,mapping_digest,attestation_digest,owner_epoch::text FROM pnl.inventory_l16_stream_proof_v1 WHERE projection_generation=$1 AND partition=$2 AND effect_identity=$3`, f.consumer.generation, row.LogicalPartition, row.Position.EffectIdentity).Scan(&proof.SourceFillID, &proof.SourceFactDigest, &proof.SourceDigest, &proof.OwnerEpoch); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err = json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	object["revision"] = revision
	raw, err = json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	proofRaw, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	_, err = tx.Exec(f.ctx, `SELECT api_stream.append_portfolio($1::jsonb,$2::jsonb)`, raw, proofRaw)
	var postgres *pgconn.PgError
	if !errors.As(err, &postgres) || postgres.Code != "P0001" || postgres.Message != message {
		t.Fatalf("append refusal = %v, want %s", err, message)
	}
}

func TestCheckpointRevisionBelowTheSum(t *testing.T) {
	f := newCheckpointRevisionFixture(t, false)
	f.consume(t, 0)
	f.consume(t, 1)
	frame := f.assertRevision(t, "2", "2", 2)
	f.assertRejected(t, frame, "1", "1", "1", "portfolio checkpoint revision mismatch")
}

func TestCheckpointRevisionSummaryDiffers(t *testing.T) {
	f := newCheckpointRevisionFixture(t, true)
	f.consume(t, 0)
	frame := f.assertRevision(t, "4", "1", 4)
	f.assertRejected(t, frame, "4", "3", "4", "portfolio checkpoint revision mismatch")
}

func TestCheckpointRevisionDecimalAndProof(t *testing.T) {
	f := newCheckpointRevisionFixture(t, true)
	f.consume(t, 0)
	frame := f.assertRevision(t, "4", "1", 4)
	for _, revision := range []any{nil, "", "0", "04", "+4", "-4", "4.0", "4e0", " 4"} {
		raw, err := json.Marshal(revision)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(string(raw), func(t *testing.T) {
			f.assertRejected(t, frame, revision, "4", "4", "portfolio checkpoint revision mismatch")
		})
	}
	f.assertRejected(t, frame, "4", "4", "3", "portfolio checkpoint revision mismatch")
}
