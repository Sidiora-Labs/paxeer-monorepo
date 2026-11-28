package l16

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/consumerhealth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func recordedCatchUpConsumer(t *testing.T, ctx context.Context) (*AccountingConsumer, *pgxpool.Pool, []delivery.Envelope) {
	t.Helper()
	service, pool, frames := currentRecordedInventoryFrames(t, ctx)
	a := service.authority
	consumer, err := NewAccountingConsumerWithEpochGuard(pool, "pnl", a.consumerID, a.generation, a.owner, a.projectionVersion, a, a.epochGuard)
	if err != nil {
		t.Fatal(err)
	}
	for i := range frames {
		frames[i].TraceID = frames[i].EventID
	}
	return consumer, pool, frames
}

func catchUpWire(t *testing.T, frame delivery.Envelope) []byte {
	t.Helper()
	policy := delivery.Policy{Subjects: map[string]bool{accounting.InventorySubject: true}, EventTypes: map[string]bool{accounting.InventorySubject: true}}
	wire, err := frame.Encode(policy, accounting.InventorySubject)
	if err != nil {
		t.Fatal(err)
	}
	return wire
}

func catchUpPublication(t *testing.T, frame delivery.Envelope) accounting.PublicationV1 {
	t.Helper()
	publication, err := accounting.DecodePublicationV1(frame.Payload)
	if err != nil {
		t.Fatal(err)
	}
	return publication
}

func catchUpBrokerDelivery(t *testing.T, ctx context.Context, frame delivery.Envelope) (*natsdriver.Message, jetstream.Consumer) {
	t.Helper()
	nc, err := nats.Connect(realMarketEnv(t, "PNL_TEST_NATS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("PNL_CATCHUP_%d", time.Now().UnixNano())
	if _, err = js.CreateStream(ctx, jetstream.StreamConfig{Name: name, Subjects: []string{accounting.InventorySubject}, Storage: jetstream.FileStorage}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := js.DeleteStream(cleanup, name); err != nil {
			t.Error(err)
		}
	})
	cfg := natsdriver.ConsumerConfig{Stream: name, Durable: "inventory", Subject: accounting.InventorySubject, Batch: 1, MaxAckPending: 8, MaxWaiting: 1, MaxDeliver: 100, FetchWait: time.Second, AckWait: 30 * time.Second}
	brokerCfg, err := cfg.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	broker, err := retryInventoryStreamRequest(ctx, func(requestCtx context.Context) (jetstream.Consumer, error) {
		return js.CreateOrUpdateConsumer(requestCtx, name, brokerCfg)
	})
	if err != nil {
		t.Fatalf("create broker consumer: %v", err)
	}
	publisher, err := natsdriver.NewPublisher(nc, name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = publisher.Publish(ctx, accounting.InventorySubject, frame.EventID, catchUpWire(t, frame)); err != nil {
		t.Fatal(err)
	}
	transport, err := natsdriver.NewConsumer(ctx, nc, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(transport.Close)
	messages, err := transport.Fetch(ctx)
	if err != nil || len(messages) != 1 {
		t.Fatal(len(messages), err)
	}
	return messages[0], broker
}

func assertCatchUpAcknowledged(t *testing.T, ctx context.Context, broker jetstream.Consumer) {
	t.Helper()
	info, err := broker.Info(ctx)
	if err != nil || info.NumAckPending != 0 || info.AckFloor.Consumer != 1 {
		t.Fatal(info, err)
	}
}

func assertCatchUpCommits(t *testing.T, ctx context.Context, consumer *AccountingConsumer, frames []delivery.Envelope) {
	t.Helper()
	rows, err := consumer.pool.Query(ctx, `SELECT position FROM pnl.delivery_partition_commits WHERE consumer_id=$1 AND projection_generation=$2 ORDER BY xmin::text::bigint,partition_key,sequence`, consumer.consumerID, consumer.generation)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []accounting.PartitionPosition
	for rows.Next() {
		var raw []byte
		var position accounting.PartitionPosition
		if err = rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, &position); err != nil {
			t.Fatal(err)
		}
		got = append(got, position)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := make([]accounting.PartitionPosition, 0, len(frames))
	for _, frame := range frames {
		want = append(want, catchUpPublication(t, frame).Positions[0])
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("committed order = %+v, want %+v", got, want)
	}
}

func assertCatchUpInventory(t *testing.T, ctx context.Context, consumer *AccountingConsumer, frames []delivery.Envelope) {
	t.Helper()
	expected := Empty()
	for _, frame := range frames {
		publication := catchUpPublication(t, frame)
		tx, err := consumer.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		fill, err := ReadSourceFillTx(ctx, tx, publication.SourceFillID, publication.SourceFactDigest)
		inventoryRollback(tx)
		if err != nil {
			t.Fatal(err)
		}
		leg, err := InventoryLeg(fill, publication)
		if err != nil {
			t.Fatal(err)
		}
		expected, err = Apply(expected, leg)
		if err != nil {
			t.Fatal(err)
		}
	}
	var raw []byte
	var revision int
	partition := catchUpPublication(t, frames[0]).Positions[0].PartitionKey
	if err := consumer.pool.QueryRow(ctx, `SELECT state,revision FROM pnl.inventory_l16 WHERE partition=$1`, Algorithm+":"+consumer.generation+":"+partition).Scan(&raw, &revision); err != nil {
		t.Fatal(err)
	}
	state, err := DecodeInventoryState(raw)
	if err != nil || state != expected || revision != len(frames) {
		t.Fatal(state, expected, revision, err)
	}
}

func TestInventoryCatchUpAppliesAMissedPredecessorOnADelivery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	consumer, pool, frames := recordedCatchUpConsumer(t, ctx)
	message, broker := catchUpBrokerDelivery(t, ctx, frames[1])
	if err := consumer.PrepareAndConsume(ctx, message.Subject, message.Wire, message); err != nil {
		t.Fatal(err)
	}
	assertCatchUpCommits(t, ctx, consumer, frames)
	assertCatchUpInventory(t, ctx, consumer, frames)
	assertCatchUpAcknowledged(t, ctx, broker)
	var actual string
	if err := pool.QueryRow(ctx, `SELECT envelope_hash FROM pnl.delivery_inbox WHERE consumer_id=$1 AND projection_generation=$2 AND event_id=$3`, consumer.consumerID, consumer.generation, frames[0].EventID).Scan(&actual); err != nil || actual != delivery.Hash(catchUpWire(t, frames[0])) {
		t.Fatal(actual, err)
	}
}

func catchUpTwoPartitions(t *testing.T) (*AccountingConsumer, *pgxpool.Pool, []delivery.Envelope) {
	t.Helper()
	fixture := newLocalCanonicalFixture(t, "inventory_catchup")
	ctx, a := fixture.ctx, fixture.authority
	for _, wallet := range []string{fixture.chain.Wallet, fixture.chain.SecondWallet} {
		if _, err := a.Capture(ctx, wallet, fixture.chain.Token); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(fixture.control, "trade"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		_, err := os.Stat(filepath.Join(fixture.control, "traded.json"))
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
	admin := nodeWindowAdmin(t, fixture)
	rows, err := admin.Query(ctx, `SELECT envelope FROM indexer.delivery_outbox WHERE subject=$1 ORDER BY (convert_from(envelope,'UTF8')::jsonb->'source_order'->>'height')::numeric,(convert_from(envelope,'UTF8')::jsonb->'source_order'->>'transaction_index')::numeric,(convert_from(envelope,'UTF8')::jsonb->'source_order'->>'log_index')::numeric`, accounting.InventorySubject)
	if err != nil {
		t.Fatal(err)
	}
	var frames []delivery.Envelope
	for rows.Next() {
		var raw []byte
		var frame delivery.Envelope
		if err = rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, &frame); err != nil {
			t.Fatal(err)
		}
		frames = append(frames, frame)
	}
	rows.Close()
	if err = rows.Err(); err != nil || len(frames) != 3 {
		t.Fatal(len(frames), err)
	}
	consumer, err := NewAccountingConsumerWithEpochGuard(a.pool, "pnl", a.consumerID, a.generation, a.owner, a.projectionVersion, a, a.epochGuard)
	if err != nil {
		t.Fatal(err)
	}
	first := catchUpPublication(t, frames[0]).Positions[0]
	second := catchUpPublication(t, frames[1]).Positions[0]
	third := catchUpPublication(t, frames[2]).Positions[0]
	if first.PartitionKey == second.PartitionKey || first.PartitionKey != third.PartitionKey || first.Sequence != "1" || second.Sequence != "1" || third.Sequence != "2" {
		t.Fatal(first, second, third)
	}
	if err = consumer.PreparePartition(ctx, first.PartitionKey); err != nil {
		t.Fatal(err)
	}
	var offsets, owed int
	if err = a.pool.QueryRow(ctx, `SELECT count(*),COALESCE(sum(next_sequence),0)::int FROM pnl.delivery_partition_offsets WHERE consumer_id=$1 AND projection_generation=$2`, a.consumerID, a.generation).Scan(&offsets, &owed); err != nil || offsets != 1 || owed != 1 {
		t.Fatal(offsets, owed, err)
	}
	return consumer, admin, frames
}

func TestInventoryCatchUpPassAppliesOwedRecordsWithoutADelivery(t *testing.T) {
	consumer, _, frames := catchUpTwoPartitions(t)
	ctx := context.Background()
	now := frames[len(frames)-1].EmittedAt.Add(time.Hour)
	consumer.Progress = consumerhealth.New("inventory_consumer", time.Minute, nil, func() time.Time { return now })
	consumer.Progress.RecordDelivery(ctx, delivery.ErrGap, "gap", catchUpWire(t, frames[0]))
	now = now.Add(2 * time.Minute)
	if err := consumer.Progress.Check(ctx); !errors.Is(err, consumerhealth.ErrNoProgress) {
		t.Fatal(err)
	}
	pass := NewInventoryCatchUp(consumer)
	pass.Now = func() time.Time { return now }
	if applied, err := pass.RunOnce(ctx); err != nil || applied != 3 {
		t.Fatal(applied, err)
	}
	assertCatchUpCommits(t, ctx, consumer, frames)
	assertCatchUpInventory(t, ctx, consumer, []delivery.Envelope{frames[0], frames[2]})
	assertCatchUpInventory(t, ctx, consumer, frames[1:2])
	for _, frame := range frames {
		var hash string
		if err := consumer.pool.QueryRow(ctx, `SELECT envelope_hash FROM pnl.delivery_inbox WHERE consumer_id=$1 AND projection_generation=$2 AND event_id=$3`, consumer.consumerID, consumer.generation, frame.EventID).Scan(&hash); err != nil || hash != delivery.Hash(catchUpWire(t, frame)) {
			t.Fatal(hash, err)
		}
	}
	if err := consumer.Progress.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if state := consumer.Progress.Snapshot(); state.LastSuccess.IsZero() || state.ConsecutiveFailures != 0 || state.RetryPartitions != 0 {
		t.Fatal(state)
	}
	before := consumer.Progress.Snapshot()
	if applied, err := pass.RunOnce(ctx); err != nil || applied != 0 || consumer.Progress.Snapshot() != before {
		t.Fatal(applied, err, consumer.Progress.Snapshot(), before)
	}
}

func TestInventoryCatchUpLeavesTheGapWhenARecordIsMissing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	consumer, pool, frames := recordedCatchUpConsumer(t, ctx)
	if _, err := pool.Exec(ctx, `DELETE FROM indexer.indexer_ingest_outbox WHERE event_kind=$1 AND payload->>'publicationId'=$2`, accounting.InventorySubject, frames[0].EventID); err != nil {
		t.Fatal(err)
	}
	message, broker := catchUpBrokerDelivery(t, ctx, frames[1])
	if err := consumer.PrepareAndConsume(ctx, message.Subject, message.Wire, message); !errors.Is(err, delivery.ErrGap) {
		t.Fatal(err)
	}
	var commits, effects, inbox int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM pnl.delivery_partition_commits),(SELECT count(*) FROM pnl.inventory_l16_effects),(SELECT count(*) FROM pnl.delivery_inbox)`).Scan(&commits, &effects, &inbox); err != nil || commits != 0 || effects != 0 || inbox != 0 {
		t.Fatal(commits, effects, inbox, err)
	}
	info, err := broker.Info(ctx)
	if err != nil || info.NumAckPending != 1 || info.AckFloor.Consumer != 0 {
		t.Fatal(info, err)
	}
}

func catchUpRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool) []byte {
	t.Helper()
	var raw []byte
	if err := pool.QueryRow(ctx, `SELECT jsonb_build_array((SELECT jsonb_agg(to_jsonb(o) ORDER BY partition_key) FROM pnl.delivery_partition_offsets o),(SELECT jsonb_agg(to_jsonb(c) ORDER BY partition_key,sequence) FROM pnl.delivery_partition_commits c),(SELECT jsonb_agg(to_jsonb(i) ORDER BY event_id) FROM pnl.delivery_inbox i),(SELECT jsonb_agg(to_jsonb(s) ORDER BY partition) FROM pnl.inventory_l16 s),(SELECT jsonb_agg(to_jsonb(e) ORDER BY partition,effect_identity) FROM pnl.inventory_l16_effects e),(SELECT jsonb_agg(to_jsonb(b) ORDER BY partition,effect_identity,owner_epoch) FROM pnl.inventory_l16_baseline_uses b))`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestInventoryCatchUpLaterDeliveryOfAnAppliedRecordIsADuplicate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	consumer, pool, frames := recordedCatchUpConsumer(t, ctx)
	pass := NewInventoryCatchUp(consumer)
	pass.Limit = 1
	if applied, err := pass.RunOnce(ctx); err != nil || applied != 1 {
		t.Fatal(applied, err)
	}
	assertCatchUpCommits(t, ctx, consumer, frames[:1])
	before := catchUpRows(t, ctx, pool)
	a := consumer.authority.(*BaselineAuthority)
	restarted, err := NewAccountingConsumerWithEpochGuard(pool, "pnl", a.consumerID, a.generation, a.owner, a.projectionVersion, a, a.epochGuard)
	if err != nil {
		t.Fatal(err)
	}
	message, broker := catchUpBrokerDelivery(t, ctx, frames[0])
	if err = restarted.PrepareAndConsume(ctx, message.Subject, message.Wire, message); err != nil {
		t.Fatal(err)
	}
	assertCatchUpAcknowledged(t, ctx, broker)
	if after := catchUpRows(t, ctx, pool); !bytes.Equal(before, after) {
		t.Fatalf("duplicate changed inventory rows: before=%s after=%s", before, after)
	}
}

func TestInventoryCatchUpPassKeepsTheAgeAndTheNumber(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	consumer, _, frames := recordedCatchUpConsumer(t, ctx)
	now := frames[0].EmittedAt.Add(30 * time.Second)
	consumer.Progress = consumerhealth.New("inventory_consumer", time.Minute, nil, func() time.Time { return now })
	pass := NewInventoryCatchUp(consumer)
	if pass.Period != 30*time.Second || pass.MinAge != 120*time.Second || pass.Limit != 16 {
		t.Fatal(pass)
	}
	pass.Now = func() time.Time { return now }
	pass.Limit = 1
	before := consumer.Progress.Snapshot()
	if applied, err := pass.RunOnce(ctx); err != nil || applied != 0 || consumer.Progress.Snapshot() != before {
		t.Fatal(applied, err, consumer.Progress.Snapshot())
	}
	var commits int
	if err := consumer.pool.QueryRow(ctx, `SELECT count(*) FROM pnl.delivery_partition_commits`).Scan(&commits); err != nil || commits != 0 {
		t.Fatal(commits, err)
	}
	now = frames[0].EmittedAt.Add(pass.MinAge)
	if applied, err := pass.RunOnce(ctx); err != nil || applied != 0 {
		t.Fatal(applied, err)
	}
	now = frames[len(frames)-1].EmittedAt.Add(time.Hour)
	for i := 1; i <= len(frames); i++ {
		if applied, err := pass.RunOnce(ctx); err != nil || applied != 1 {
			t.Fatal(applied, err)
		}
		assertCatchUpCommits(t, ctx, consumer, frames[:i])
		var next int64
		if err := consumer.pool.QueryRow(ctx, `SELECT next_sequence FROM pnl.delivery_partition_offsets WHERE consumer_id=$1 AND projection_generation=$2 AND partition_key=$3`, consumer.consumerID, consumer.generation, catchUpPublication(t, frames[0]).Positions[0].PartitionKey).Scan(&next); err != nil || next != int64(i+1) {
			t.Fatal(next, err)
		}
	}
	assertCatchUpInventory(t, ctx, consumer, frames)
}
