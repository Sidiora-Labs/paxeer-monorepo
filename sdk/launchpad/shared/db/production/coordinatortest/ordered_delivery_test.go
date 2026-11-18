package coordinatortest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/brokerprovision"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func mountedBroker(t *testing.T) *nats.Conn {
	t.Helper()
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", "0"))
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	command := exec.Command("nats-server", "-js", "-sd", t.TempDir(), "-a", "127.0.0.1", "-p", strconv.Itoa(port))
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		_ = command.Process.Signal(os.Interrupt)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = command.Process.Kill()
			<-done
		}
	})
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		connection, err := nats.Connect("nats://"+address, nats.NoReconnect(), nats.Timeout(100*time.Millisecond))
		if err == nil {
			t.Cleanup(connection.Close)
			return connection
		}
	}
	t.Fatal("file-backed broker did not become ready")
	return nil
}

func assertOrderedBatchRecovery(t *testing.T, db Database, admin *pgxpool.Pool) {
	t.Helper()
	t.Run("ordered batches preserve broker progress across restart", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		connect := func(role string) *pgxpool.Pool {
			t.Helper()
			pool, err := pgxpool.New(ctx, db.Runtimes[role].DSN)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(pool.Close)
			return pool
		}
		producer := connect("indexer_runtime")
		producerPartitions, err := delivery.NewPartitionStore(producer, "indexer")
		if err != nil {
			t.Fatal(err)
		}
		policy := delivery.Policy{Subjects: map[string]bool{accounting.MarketSubject: true}, EventTypes: map[string]bool{accounting.MarketSubject: true}}
		guard, err := queue.NewEpochGuard("recovery", Epoch, Region)
		if err != nil {
			t.Fatal(err)
		}
		producerStore, err := delivery.NewStoreWithEpochGuard(producer, "indexer", policy, guard.Assert)
		if err != nil {
			t.Fatal(err)
		}
		manifest := deployment.Current()
		fill := accounting.FillV1{
			Version:       accounting.FillVersion,
			Position:      accounting.SourcePosition{ChainID: "125", Height: "100", TransactionIndex: "0", LogIndex: "7", TransactionHash: "0x" + strings.Repeat("a", 64), BlockHash: "0x" + strings.Repeat("b", 64)},
			ActualEmitter: strings.ToLower(manifest.Roles["EventEmitter"].Address),
			PoolAddress:   strings.ToLower(manifest.Roles["PoolRegistry"].Address),
			Token:         accounting.Asset{Address: strings.ToLower(manifest.Roles["SidioraNFT"].Address), Decimals: 18},
			Settlement:    accounting.Asset{Address: strings.ToLower(deployment.SettlementAddress), Decimals: 6},
			Side:          "buy", TokenAmountRaw: "332665999332665", USDLAmountRaw: "5000000000", PaidInputRaw: "5000000000", FeeRaw: "15000000",
			InputDerivationQuality: "receipt_incomplete", FeeDisposition: "distributed_usdl",
			Attribution:           accounting.Attribution{TransactionSender: strings.ToLower(manifest.Governance.ExpectedBeaconOwner), Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
			Execution:             accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
			Provenance:            accounting.Provenance{DeploymentID: manifest.DeploymentVersion, ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch-1", PoolCodeEpoch: "epoch-1", ReceiptSHA256: strings.Repeat("c", 64), HeaderSHA256: strings.Repeat("d", 64)},
			BlockTimestampSeconds: "1700000000",
		}
		positions := make([]accounting.PartitionPosition, 2)
		envelopes := make([]delivery.Envelope, 2)
		wires := make([][]byte, 2)
		for i := range positions {
			fill.Position.LogIndex = strconv.Itoa(7 + i)
			fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
			digest, err := accounting.FillDigest(fill)
			if err != nil {
				t.Fatal(err)
			}
			tx, err := producer.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			position, err := producerPartitions.AppendTx(ctx, tx, delivery.AppendInput{
				ProjectionVersion: "market-v1", PartitionKind: "market", PartitionKey: "125:" + fill.PoolAddress,
				SourceEventID: fill.SourceFillID, EffectIdentity: accounting.EffectIdentity(fill.SourceFillID, "market", "market-v1", "", "market", "125:"+fill.PoolAddress),
				PayloadDigest: digest, SourceOrder: fill.Position,
			})
			if err != nil {
				t.Fatal(err)
			}
			publication, err := accounting.NewMarketPublication(fill, []accounting.PartitionPosition{position})
			if err != nil {
				t.Fatal(err)
			}
			payload, err := accounting.CanonicalPublicationV1(publication)
			if err != nil {
				t.Fatal(err)
			}
			envelope := delivery.Envelope{EventID: publication.PublicationID, EventType: accounting.MarketSubject, SchemaVersion: 1, ChainID: 125,
				SourceOrder: delivery.SourceOrder{Height: fill.Position.Height, TransactionIndex: fill.Position.TransactionIndex, LogIndex: fill.Position.LogIndex},
				EmittedAt:   time.Unix(1700000000, 0).UTC(), TraceID: publication.PublicationID, PayloadHash: delivery.Hash(payload), Payload: payload}
			wire, err := envelope.Encode(policy, accounting.MarketSubject)
			if err != nil {
				t.Fatal(err)
			}
			if err := producerStore.Enqueue(ctx, tx, accounting.MarketSubject, envelope); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			positions[i], envelopes[i], wires[i] = position, envelope, wire
		}
		connection := mountedBroker(t)
		js, err := jetstream.New(connection)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(RepositoryRoot(t), "deploy/launchpad/documents/values/broker.example.json"))
		if err != nil {
			t.Fatal(err)
		}
		document, err := brokerprovision.Read(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		selected := brokerprovision.Document{Durables: []natsdriver.ConsumerConfig{}}
		for _, stream := range document.Streams {
			if stream.Name == "KINDLE_INDEXER_V1" {
				selected.Streams = append(selected.Streams, stream)
			}
		}
		for _, durable := range document.Durables {
			if durable.Durable == "stats_market" {
				selected.Durables = append(selected.Durables, durable)
			}
		}
		if len(selected.Streams) != 1 || len(selected.Durables) != 1 {
			t.Fatal("market stream and stats durable required")
		}
		if _, err := brokerprovision.Provision(ctx, js, selected, brokerprovision.Updates{}); err != nil {
			t.Fatal(err)
		}
		cfg := selected.Durables[0]
		publisher, err := natsdriver.NewPublisher(connection, cfg.Stream)
		if err != nil {
			t.Fatal(err)
		}
		var firstPublished time.Time
		claimed, err := producerStore.Claim(ctx, "indexer-instance", 2, time.Minute)
		if err != nil || len(claimed) != 2 {
			t.Fatalf("producer outbox claims: %d %v", len(claimed), err)
		}
		for _, i := range []int{1, 0} {
			var claimedEvent *delivery.Lease
			for j := range claimed {
				if claimed[j].EventID == envelopes[i].EventID {
					claimedEvent = &claimed[j]
				}
			}
			if claimedEvent == nil || claimedEvent.Subject != cfg.Subject || !bytes.Equal(claimedEvent.Wire, wires[i]) {
				t.Fatal("producer outbox did not preserve the publication")
			}
			ack, err := publisher.Publish(ctx, claimedEvent.Subject, claimedEvent.EventID, claimedEvent.Wire)
			if err != nil {
				t.Fatal(err)
			}
			if err := producerStore.MarkPublished(ctx, *claimedEvent, ack); err != nil {
				t.Fatal(err)
			}
			if i == 1 {
				firstPublished = time.Now()
			}
		}
		consumer, err := natsdriver.NewConsumer(ctx, connection, cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer consumer.Close()
		messages, err := consumer.Fetch(ctx)
		if err != nil || len(messages) != 2 || messages[0].Sequence != 1 || messages[1].Sequence != 2 || !bytes.Equal(messages[0].Wire, wires[1]) || !bytes.Equal(messages[1].Wire, wires[0]) {
			t.Fatalf("out-of-order broker publications: count=%d err=%v", len(messages), err)
		}
		pool := connect("stats_runtime")
		partitions, err := delivery.NewPartitionStore(pool, "stats")
		if err != nil {
			t.Fatal(err)
		}
		key := delivery.ConsumerKey{ConsumerID: cfg.Durable, ProjectionGeneration: "g1", PartitionKind: "market", PartitionKey: positions[0].PartitionKey}
		lease, err := partitions.Acquire(ctx, key, "stats-instance", 5*time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		origin, err := sourcebootstrap.FirstMarketPosition(ctx, pool, "indexer", "market-v1", fill.PoolAddress)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(context.Background())
		if err := partitions.BootstrapTx(ctx, tx, delivery.BootstrapInput{Lease: lease, NextSequence: "1", CheckpointDigest: accounting.Digest(delivery.Hash(origin))}, func(ctx context.Context, tx pgx.Tx, in delivery.BootstrapInput) error {
			wire, err := sourcebootstrap.FirstMarketPosition(ctx, tx, "indexer", "market-v1", fill.PoolAddress)
			if err != nil {
				return err
			}
			var position accounting.PartitionPosition
			if json.Unmarshal(wire, &position) != nil || position != positions[0] || string(in.CheckpointDigest) != delivery.Hash(wire) {
				return delivery.ErrIntegrity
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		store, err := delivery.NewStoreWithEpochGuard(pool, "stats", policy, guard.Assert)
		if err != nil {
			t.Fatal(err)
		}
		hooks := delivery.OrderedBatchHooks{
			Guard: func(ctx context.Context, tx pgx.Tx, envelope delivery.Envelope) (delivery.GuardedPosition, error) {
				publication, err := accounting.DecodePublicationV1(envelope.Payload)
				if err != nil || len(publication.Positions) != 1 {
					return delivery.GuardedPosition{}, delivery.ErrIntegrity
				}
				return partitions.CheckAndLockTx(ctx, tx, lease, publication.Positions[0])
			},
			Effect: func(ctx context.Context, tx pgx.Tx, effects []delivery.OrderedEffect) error {
				for _, effect := range effects {
					sequence, err := strconv.ParseInt(effect.Position.Position.Sequence, 10, 64)
					if err != nil {
						return err
					}
					if _, err := tx.Exec(ctx, `INSERT INTO stats.reaction_pools(pool,revision) VALUES($1,$2) ON CONFLICT(pool) DO UPDATE SET revision=EXCLUDED.revision`, fill.PoolAddress, sequence); err != nil {
						return err
					}
				}
				return nil
			},
		}
		input := func(m *natsdriver.Message) delivery.OrderedDelivery {
			return delivery.OrderedDelivery{Subject: m.Subject, Wire: m.Wire, Delivery: m}
		}
		state := func(count, next, watermark int64) {
			t.Helper()
			var inbox, commits, offset, broker, position, effects int64
			var event, hash, stream string
			if err := admin.QueryRow(ctx, `SELECT (SELECT count(*) FROM stats.delivery_inbox WHERE consumer_id=$1), (SELECT count(*) FROM stats.delivery_partition_commits WHERE consumer_id=$1), (SELECT next_sequence FROM stats.delivery_partition_offsets WHERE consumer_id=$1), coalesce((SELECT broker_sequence FROM stats.delivery_consumer_watermarks WHERE consumer_id=$1),0), coalesce((SELECT position_sequence FROM stats.delivery_consumer_watermarks WHERE consumer_id=$1),0), coalesce((SELECT revision FROM stats.reaction_pools WHERE pool=$2),0), coalesce((SELECT event_id FROM stats.delivery_consumer_watermarks WHERE consumer_id=$1),''), coalesce((SELECT envelope_hash FROM stats.delivery_consumer_watermarks WHERE consumer_id=$1),''), coalesce((SELECT broker_stream FROM stats.delivery_consumer_watermarks WHERE consumer_id=$1),'')`, key.ConsumerID, fill.PoolAddress).Scan(&inbox, &commits, &offset, &broker, &position, &effects, &event, &hash, &stream); err != nil {
				t.Fatal(err)
			}
			if inbox != count || commits != count || offset != next || broker != watermark || position != count || effects != count {
				t.Fatalf("ordered state: inbox=%d commits=%d next=%d broker=%d position=%d effects=%d", inbox, commits, offset, broker, position, effects)
			}
			if count > 0 && (event != envelopes[count-1].EventID || hash != delivery.Hash(wires[count-1]) || stream != cfg.Stream) {
				t.Fatal("watermark does not identify its committed publication")
			}
		}
		if err := store.ConsumeOrderedBatch(ctx, key.ConsumerID, key.ProjectionGeneration, []delivery.OrderedDelivery{input(messages[1]), input(messages[0])}, hooks); !errors.Is(err, delivery.ErrIntegrity) {
			t.Fatalf("regressing batch broker position: %v", err)
		}
		state(0, 1, 0)
		info, err := js.Consumer(ctx, cfg.Stream, cfg.Durable)
		if err != nil {
			t.Fatal(err)
		}
		pending, err := info.Info(ctx)
		if err != nil || pending.NumAckPending != 2 {
			t.Fatalf("rejected batch acknowledgement: %+v %v", pending, err)
		}
		if err := store.ConsumeOrderedBatch(ctx, key.ConsumerID, key.ProjectionGeneration, []delivery.OrderedDelivery{input(messages[1])}, hooks); err != nil {
			t.Fatal(err)
		}
		if err := store.ConsumeOrderedBatch(ctx, key.ConsumerID, key.ProjectionGeneration, []delivery.OrderedDelivery{input(messages[0])}, hooks); !errors.Is(err, delivery.ErrIntegrity) {
			t.Fatalf("persisted broker position regression: %v", err)
		}
		state(1, 2, 2)
		var quarantined int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM stats.delivery_ordered_quarantine WHERE consumer_id=$1 AND envelope_hash=$2 AND failure_class=$3`, key.ConsumerID, delivery.Hash(wires[1]), string(delivery.PartitionBinding)).Scan(&quarantined); err != nil || quarantined != 1 {
			t.Fatalf("regression quarantine: %d %v", quarantined, err)
		}
		consumer.Close()
		pool.Close()
		pool = connect("stats_runtime")
		partitions, err = delivery.NewPartitionStore(pool, "stats")
		if err != nil {
			t.Fatal(err)
		}
		store, err = delivery.NewStoreWithEpochGuard(pool, "stats", policy, guard.Assert)
		if err != nil {
			t.Fatal(err)
		}
		lease, err = partitions.Acquire(ctx, key, "stats-instance", 5*time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		state(1, 2, 2)
		// A renewed publication after the broker's deduplication window has a
		// new persistence position while retaining the source fact's identity.
		timer := time.NewTimer(time.Until(firstPublished.Add(selected.Streams[0].Limits.DuplicateWindow + time.Second)))
		defer timer.Stop()
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-timer.C:
		}
		ack, err := publisher.Publish(ctx, cfg.Subject, envelopes[1].EventID, wires[1])
		if err != nil || ack.Sequence != 3 {
			t.Fatalf("renewed publication: %+v %v", ack, err)
		}
		consumer, err = natsdriver.NewConsumer(ctx, connection, cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer consumer.Close()
		messages, err = consumer.Fetch(ctx)
		if err != nil || len(messages) != 2 {
			t.Fatalf("resumed broker delivery: %d %v", len(messages), err)
		}
		if messages[0].Sequence != 1 || messages[0].Deliveries < 2 || messages[1].Sequence != 3 || !bytes.Equal(messages[0].Wire, wires[1]) || !bytes.Equal(messages[1].Wire, wires[1]) {
			t.Fatal("durable restart did not retain the unacknowledged publication")
		}
		if err := store.ConsumeOrderedBatch(ctx, key.ConsumerID, key.ProjectionGeneration, []delivery.OrderedDelivery{input(messages[1])}, hooks); err != nil {
			t.Fatal(err)
		}
		state(2, 3, 3)
		if err := store.ConsumeOrderedBatch(ctx, key.ConsumerID, key.ProjectionGeneration, []delivery.OrderedDelivery{input(messages[0])}, hooks); err != nil {
			t.Fatal(err)
		}
		state(2, 3, 3)
		pending, err = info.Info(ctx)
		if err != nil || pending.NumAckPending != 0 || pending.AckFloor.Stream != 3 {
			t.Fatalf("recovered durable acknowledgement: %+v %v", pending, err)
		}
		if differences, err := brokerprovision.Check(ctx, js, selected); err != nil || len(differences) != 0 {
			t.Fatalf("market deployment drift after restart: %v %v", differences, err)
		}
	})
}
