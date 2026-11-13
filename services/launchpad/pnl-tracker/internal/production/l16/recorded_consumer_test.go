package l16

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"fmt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func currentRecordedInventoryFrames(t *testing.T, ctx context.Context) (*Service, *pgxpool.Pool, []delivery.Envelope) {
	t.Helper()
	service, pool := recordedRebuildService(t, ctx)
	a := service.authority
	partition := "125:" + realMarketCreator + ":" + realMarketToken
	digest, err := a.Capture(ctx, realMarketCreator, realMarketToken)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	att, actual, err := a.readBaseline(ctx, tx, partition)
	if err != nil || actual != string(digest) {
		t.Fatal(att, actual, err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(ctx, `SELECT event_id,envelope FROM indexer.delivery_outbox WHERE subject=$1 ORDER BY created_at,event_id`, accounting.InventorySubject)
	if err != nil {
		t.Fatal(err)
	}
	type frame struct {
		id       string
		envelope delivery.Envelope
	}
	frames := []frame{}
	for rows.Next() {
		var id string
		var raw []byte
		if err = rows.Scan(&id, &raw); err != nil {
			t.Fatal(err)
		}
		var envelope delivery.Envelope
		if err = json.Unmarshal(raw, &envelope); err != nil {
			t.Fatal(err)
		}
		frames = append(frames, frame{id, envelope})
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2 {
		t.Fatal(len(frames))
	}
	previous := ""
	current := []delivery.Envelope{}
	for _, f := range frames {
		var old accounting.PublicationV1
		if err = json.Unmarshal(f.envelope.Payload, &old); err != nil {
			t.Fatal(err)
		}
		var raw []byte
		if err = pool.QueryRow(ctx, `SELECT payload FROM indexer.indexer_accounting_fills_v1 WHERE source_fill_id=$1`, old.SourceFillID).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		fill, e := accounting.DecodeFillV1(raw)
		if e != nil {
			t.Fatal(e)
		}
		source, e := accounting.FillDigest(fill)
		if e != nil {
			t.Fatal(e)
		}
		movementRows, e := pool.Query(ctx, `SELECT payload FROM indexer.indexer_token_transfers_v1 WHERE token=$1 AND height BETWEEN $2 AND $3::bigint AND (from_address=$4 OR to_address=$4) ORDER BY height,transaction_index,log_index`, realMarketToken, att.CreationHeight, fill.Position.Height, realMarketCreator)
		if e != nil {
			t.Fatal(e)
		}
		movements := []json.RawMessage{}
		for movementRows.Next() {
			var raw json.RawMessage
			if e = movementRows.Scan(&raw); e != nil {
				t.Fatal(e)
			}
			movements = append(movements, raw)
		}
		movementRows.Close()
		if e = movementRows.Err(); e != nil {
			t.Fatal(e)
		}
		movementRaw, e := json.Marshal(movements)
		if e != nil {
			t.Fatal(e)
		}
		enrichment := *old.Enrichment
		enrichment.SourceFactDigest = source
		enrichment.EvidenceDigest = accounting.Digest(accounting.HashTuple("fresh-inventory-evidence-v1", string(digest), att.MovementDigest, string(source), string(movementRaw)))
		position := old.Positions[0]
		position.PreviousEffectIdentity = previous
		position.EffectIdentity = accounting.EffectIdentity(fill.SourceFillID, "inventory", "inventory-v1", enrichment.EvidenceDigest, "inventory", partition)
		body, e := json.Marshal(enrichment)
		if e != nil {
			t.Fatal(e)
		}
		position.PayloadDigest = accounting.Digest(delivery.Hash(body))
		publication, e := accounting.NewEnrichmentPublication(enrichment, []accounting.PartitionPosition{position})
		if e != nil {
			t.Fatal(e)
		}
		payload, e := accounting.CanonicalPublicationV1(publication)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, `UPDATE indexer.indexer_ingest_outbox SET payload=$1 WHERE event_kind=$2 AND payload->>'sourceFillId'=$3`, payload, accounting.InventorySubject, fill.SourceFillID); e != nil {
			t.Fatal(e)
		}
		envelope := f.envelope
		envelope.EventID = publication.PublicationID
		envelope.Payload = payload
		envelope.PayloadHash = delivery.Hash(payload)

		current = append(current, envelope)
		previous = position.EffectIdentity
	}
	return service, pool, current
}

func TestRecordedAccountingConsumerCommitsInventory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	service, pool, current := currentRecordedInventoryFrames(t, ctx)
	a := service.authority
	partition := "125:" + realMarketCreator + ":" + realMarketToken
	var err error
	consumeCurrentInventoryFrames(t, ctx, service, current)

	var uses int
	var raw []byte
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM pnl.inventory_l16_baseline_uses WHERE partition=$1`, partition).Scan(&uses); err != nil || uses != 2 {
		t.Fatal(uses, err)
	}
	if err = pool.QueryRow(ctx, `SELECT state FROM pnl.inventory_l16 WHERE partition LIKE '%'||$1 ORDER BY revision DESC LIMIT 1`, ":"+partition).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	state, err := DecodeInventoryState(raw)
	if err != nil || state.Known == "0" || state.Unknown != "0" || state.IncompleteRealized {
		t.Fatal(state, err)
	}
	consumer, err := NewAccountingConsumerWithEpochGuard(pool, "pnl", "pnl", "g1", "pnl-1", "inventory-v1", a, a.epochGuard)
	if err != nil {
		t.Fatal(err)
	}
	if err = consumer.Run(ctx, nil); err == nil {
		t.Fatal("missing transport accepted")
	}
	if _, err = consumer.WithFullPortfolio(l17.Repository{Pool: pool}, service.outbox, "portfolio.snapshot.v2"); err == nil {
		t.Fatal("missing stream writer accepted")
	}

	full, err := NewAccountingConsumerWithRuntime(pool, "pnl", "pnl", "g1", "pnl-1", "inventory-v1", a, a.epochGuard, delivery.StreamWriter{Schema: "api_stream"})
	if err != nil {
		t.Fatal(err)
	}
	full, err = full.WithFullPortfolio(l17.Repository{Pool: pool}, service.outbox, "portfolio.snapshot.v2")
	if err != nil {
		t.Fatal(err)
	}
	projectionTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(projectionTx)
	if _, err = projectionTx.Exec(ctx, "SET LOCAL search_path TO pnl,pg_catalog"); err != nil {
		t.Fatal(err)
	}
	projectionRows, err := projectionTx.Query(ctx, `SELECT e.partition,s.revision,s.state,e.position,f.payload FROM inventory_l16 s JOIN inventory_l16_effects e ON e.projection_generation='g1' AND s.partition=$1||':'||e.partition AND e.leg_identity=s.last_identity JOIN indexer.indexer_accounting_fills_v1 f ON f.source_fill_id=convert_from(e.position,'UTF8')::jsonb->>'sourceEventId'`, Algorithm+":g1")
	if err != nil {
		t.Fatal(err)
	}
	portfolioRows := []portfolioRow{}
	for projectionRows.Next() {
		var row portfolioRow
		var stateRaw, positionRaw, fillRaw []byte
		if err = projectionRows.Scan(&row.LogicalPartition, &row.Revision, &stateRaw, &positionRaw, &fillRaw); err != nil {
			t.Fatal(err)
		}
		row.Inventory, err = DecodeInventoryState(stateRaw)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(positionRaw, &row.Position); err != nil {
			t.Fatal(err)
		}
		row.Fill, err = accounting.DecodeFillV1(fillRaw)
		if err != nil {
			t.Fatal(err)
		}
		portfolioRows = append(portfolioRows, row)
	}
	projectionRows.Close()
	if err = projectionRows.Err(); err != nil || len(portfolioRows) != 1 {
		t.Fatal(portfolioRows, err)
	}
	row := portfolioRows[0]
	leg, err := InventoryLeg(row.Fill, func() accounting.PublicationV1 {
		var p accounting.PublicationV1
		if err := json.Unmarshal(current[len(current)-1].Payload, &p); err != nil {
			t.Fatal(err)
		}
		return p
	}())
	if err != nil {
		t.Fatal(err)
	}
	guarded := delivery.GuardedPosition{Position: row.Position}
	assertPortfolioReplayBindings(t, projectionTx, full, guarded, leg, row)
	update, watermark, occurred, err := full.commitFullPortfolioTx(ctx, projectionTx, guarded, leg, portfolioRows)
	if err != nil || len(update.Changes) != 1 || !update.CompleteBatch || update.Summary.Wallet != realMarketCreator || string(update.SummaryRevision) != "2" || string(watermark.Height) != row.Fill.Position.Height || occurred == "" {
		t.Fatal(update, watermark, occurred, err)
	}
	assertConsumerSourceBindings(t, projectionTx, a, full, guarded, leg, row, update.SnapshotId)
	var history int
	if err = projectionTx.QueryRow(ctx, `SELECT count(*) FROM portfolio_history_rows WHERE snapshot_id=$1`, update.SnapshotId).Scan(&history); err != nil || history != 2 {
		t.Fatal(history, err)
	}
	for _, bad := range []portfolioRow{{}, func() portfolioRow { bad := row; bad.Revision = 0; return bad }(), func() portfolioRow { bad := row; bad.LogicalPartition = "bad"; return bad }()} {
		if _, _, _, err = full.commitFullPortfolioTx(ctx, projectionTx, guarded, leg, []portfolioRow{bad}); err == nil {
			t.Fatal("invalid portfolio row accepted")
		}
	}
	if _, _, _, err = full.commitFullPortfolioTx(ctx, projectionTx, guarded, leg, nil); err == nil {
		t.Fatal("empty portfolio accepted")
	}

	if err = projectionTx.QueryRow(ctx, `SELECT owner_epoch FROM inventory_l16_baseline_uses WHERE effect_identity=$1 ORDER BY owner_epoch DESC LIMIT 1`, row.Position.EffectIdentity).Scan(&guarded.Lease.Epoch); err != nil {
		t.Fatal(err)
	}
	consumer.streamWriter = &delivery.StreamWriter{}
	if err = consumer.appendPortfolioTx(ctx, projectionTx, guarded, leg, row.Fill); err == nil || !strings.Contains(err.Error(), "stream: source transaction required") {
		t.Fatal("unbound stream writer", err)
	}
	if err = projectionTx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM pnl.portfolio_snapshots WHERE wallet=$1`, realMarketCreator).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	t.Run("fresh baseline cancellation", func(t *testing.T) {
		assertInventoryTxCancellations(t, pool, func(tx pgx.Tx) error { return a.VerifyFreshSameOwnerTx(ctx, tx, "g1", row.Fill, row.Position) })
	})
	t.Run("full portfolio cancellation", func(t *testing.T) {
		assertInventoryTxCancellations(t, pool, func(tx pgx.Tx) error {
			result, _, _, err := full.commitFullPortfolioTx(ctx, tx, guarded, leg, portfolioRows)
			if err == nil && (len(result.Changes) != 1 || !result.CompleteBatch) {
				t.Fatal(result)
			}
			return err
		})
	})
	guarded.Lease.Key = delivery.ConsumerKey{ConsumerID: "pnl", ProjectionGeneration: "g1", PartitionKind: "inventory", PartitionKey: row.LogicalPartition}
	t.Run("owned replay cancellation", func(t *testing.T) {
		assertInventoryTxCancellations(t, pool, func(tx pgx.Tx) error {
			state, inserted, err := ApplyOwnedLegTx(ctx, tx, guarded, leg, "g1")
			if err == nil && (inserted || state != row.Inventory) {
				t.Fatal(state, inserted)
			}
			return err
		})
	})

	t.Run("stream portfolio cancellation", func(t *testing.T) {
		assertInventoryTxCancellations(t, pool, func(tx pgx.Tx) error { return full.appendPortfolioTx(ctx, tx, guarded, leg, row.Fill) })
	})
	t.Run("owned leg bindings", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer inventoryRollback(tx)
		if _, err = tx.Exec(ctx, "SET LOCAL search_path TO pnl,pg_catalog"); err != nil {
			t.Fatal(err)
		}
		for _, change := range []func(*delivery.GuardedPosition, *Leg){
			func(g *delivery.GuardedPosition, l *Leg) { g.Lease.Epoch = 0 },
			func(g *delivery.GuardedPosition, l *Leg) { l.Payer = zeroAddress },
			func(g *delivery.GuardedPosition, l *Leg) { l.Height++ },
			func(g *delivery.GuardedPosition, l *Leg) { g.Position.PreviousEffectIdentity = strings.Repeat("f", 64) },
			func(g *delivery.GuardedPosition, l *Leg) { l.Predecessor = "unbound" },
		} {
			bound, current := guarded, leg
			change(&bound, &current)
			if _, _, err = ApplyOwnedLegTx(ctx, tx, bound, current, "g1"); err == nil {
				t.Fatal("unbound owned leg accepted")
			}
		}
		if _, _, err = ApplyOwnedLegTx(ctx, tx, guarded, leg, ""); err == nil {
			t.Fatal("missing generation accepted")
		}
		if _, _, err = ApplyOwnedLegTx(ctx, nil, guarded, leg, "g1"); err == nil {
			t.Fatal("missing transaction accepted")
		}
	})
	canceled, done := context.WithCancel(ctx)
	done()
	if _, err = ReadSourceFillTx(canceled, nil, strings.Repeat("a", 64), accounting.Digest(strings.Repeat("b", 64))); err == nil {
		t.Fatal("missing transaction accepted")
	}
}

func consumeCurrentInventoryFrames(t *testing.T, ctx context.Context, service *Service, frames []delivery.Envelope, selected ...*AccountingConsumer) {
	t.Helper()
	nc, err := nats.Connect(realMarketEnv(t, "PNL_TEST_NATS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("PNL_CURRENT_%d", time.Now().UnixNano())
	if _, err = js.CreateStream(ctx, jetstream.StreamConfig{Name: name, Subjects: []string{accounting.InventorySubject}, Storage: jetstream.FileStorage}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := js.DeleteStream(cleanup, name); err != nil {
			t.Error(err)
		}
	}()
	cfg := natsdriver.ConsumerConfig{Stream: name, Durable: "pnl_inventory", Subject: accounting.InventorySubject, Batch: 16, MaxAckPending: 64, MaxWaiting: 4, MaxDeliver: 100, FetchWait: 250 * time.Millisecond, AckWait: 30 * time.Second}
	broker, err := cfg.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = js.CreateOrUpdateConsumer(ctx, name, broker); err != nil {
		t.Fatal(err)
	}
	publisher, err := natsdriver.NewPublisher(nc, name)
	if err != nil {
		t.Fatal(err)
	}
	for _, frame := range frames {
		raw, e := json.Marshal(frame)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = publisher.Publish(ctx, accounting.InventorySubject, frame.EventID, raw); e != nil {
			t.Fatal(e)
		}
	}
	transport, err := natsdriver.NewConsumer(ctx, nc, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	a := service.authority
	consumer, err := NewAccountingConsumerWithEpochGuard(a.pool, "pnl", a.consumerID, a.generation, a.owner, a.projectionVersion, a, a.epochGuard)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) > 0 {
		runCtx, stop := context.WithCancel(ctx)
		defer stop()
		result := make(chan error, 1)
		go func() { result <- selected[0].Run(runCtx, transport) }()
		for {
			var count int
			if err = service.authority.pool.QueryRow(ctx, `SELECT count(*) FROM pnl.portfolio_snapshots`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count == len(frames) {
				break
			}
			select {
			case err := <-result:
				t.Fatal(err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-time.After(10 * time.Millisecond):
			}
		}
		stop()
		if err = <-result; !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		return
	}
	count := 0
	for count < len(frames) {
		messages, e := transport.Fetch(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if len(messages) == 0 {
			t.Fatal("published inventory frames unavailable")
		}
		for _, message := range messages {
			if e = consumer.PrepareAndConsume(ctx, message.Subject, message.Wire, message); e != nil {
				t.Fatal(e)
			}
			count++
		}
	}
	if count != len(frames) {
		t.Fatal(count, len(frames))
	}
}

func TestAccountingRuntimePublishesCommittedPortfolioStream(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	service, pool, frames := currentRecordedInventoryFrames(t, ctx)
	a := service.authority
	consumer, err := NewAccountingConsumerWithRuntime(pool, "pnl", a.consumerID, a.generation, a.owner, a.projectionVersion, a, a.epochGuard, delivery.StreamWriter{Schema: "api_stream"})
	if err != nil {
		t.Fatal(err)
	}
	consumer, err = consumer.WithFullPortfolio(l17.Repository{Pool: pool}, service.outbox, "portfolio.snapshot.v2")
	if err != nil {
		t.Fatal(err)
	}
	consumeCurrentInventoryFrames(t, ctx, service, frames, consumer)
	var snapshots, history, streams int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM pnl.portfolio_snapshots),(SELECT count(*) FROM pnl.portfolio_history_snapshots),(SELECT count(*) FROM api_stream.stream_delivery_outbox)`).Scan(&snapshots, &history, &streams); err != nil || snapshots != 2 || history != 2 || streams != 2 {
		t.Fatal(snapshots, history, streams, err)
	}
	var raw []byte
	if err = pool.QueryRow(ctx, `SELECT summary FROM pnl.portfolio_snapshots ORDER BY revision DESC LIMIT 1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var summary l17.Summary
	if err = json.Unmarshal(raw, &summary); err != nil || summary.Wallet != realMarketCreator || summary.Watermark.ProjectionRevision != "2" || summary.PositionCount != 1 {
		t.Fatal(summary, err)
	}
}
