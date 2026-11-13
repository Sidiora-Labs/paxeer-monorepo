package delivery

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go"
)

type orderedBrokerMessage struct{ message *nats.Msg }

func (m orderedBrokerMessage) Ack(ctx context.Context) error {
	return m.message.AckSync(nats.Context(ctx))
}

func (m orderedBrokerMessage) BrokerPosition() (BrokerPosition, error) {
	metadata, err := m.message.Metadata()
	if err != nil {
		return BrokerPosition{}, err
	}
	envelope, err := Decode(m.message.Data, Policy{Subjects: map[string]bool{m.message.Subject: true}, EventTypes: map[string]bool{accounting.MarketSubject: true}}, m.message.Subject)
	if err != nil {
		return BrokerPosition{}, err
	}
	return BrokerPosition{Stream: metadata.Stream, Sequence: metadata.Sequence.Stream, Subject: m.message.Subject, EventID: envelope.EventID, WireSHA256: Hash(m.message.Data)}, nil
}

func orderedBroker(t *testing.T) *nats.Conn {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	log, err := os.CreateTemp(t.TempDir(), "broker-*.log")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("nats-server", "-js", "-sd", t.TempDir(), "-a", "127.0.0.1", "-p", strconv.Itoa(port))
	command.Stdout, command.Stderr = log, log
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
		_ = log.Close()
	})
	address := fmt.Sprintf("nats://127.0.0.1:%d", port)
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		connection, err := nats.Connect(address, nats.NoReconnect(), nats.Timeout(100*time.Millisecond))
		if err == nil {
			t.Cleanup(connection.Close)
			return connection
		}
	}
	t.Fatal("owned broker did not become ready")
	return nil
}

func TestOrderedBatchCommitsAndAcknowledgesBrokerMessages(t *testing.T) {
	ctx, pool, partitions, table := orderedPostgres(t)
	schema := os.Getenv("DELIVERY_TEST_SCHEMA") + "_ordered"
	subject := accounting.MarketSubject
	policy := Policy{Subjects: map[string]bool{subject: true}, EventTypes: map[string]bool{subject: true}}
	store, err := NewStore(pool, schema, policy)
	if err != nil {
		t.Fatal(err)
	}
	replayDDL, err := ReplayDDL(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, replayDDL); err != nil {
		t.Fatal(err)
	}
	f := accounting.FillV1{Version: accounting.FillVersion, Position: accounting.SourcePosition{ChainID: "125", Height: "100", TransactionIndex: "0", LogIndex: "7", TransactionHash: "0x" + strings.Repeat("a", 64), BlockHash: "0x" + strings.Repeat("b", 64)}, ActualEmitter: "0x" + strings.Repeat("1", 40), PoolAddress: "0x" + strings.Repeat("2", 40), Token: accounting.Asset{Address: "0x" + strings.Repeat("3", 40), Decimals: 18}, Settlement: accounting.Asset{Address: "0x" + strings.Repeat("4", 40), Decimals: 6}, Side: "buy", TokenAmountRaw: "332665999332665", USDLAmountRaw: "5000000000", PaidInputRaw: "5000000000", FeeRaw: "15000000", InputDerivationQuality: "receipt_incomplete", FeeDisposition: "distributed_usdl", Attribution: accounting.Attribution{TransactionSender: "0x" + strings.Repeat("5", 40), Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Execution: accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Provenance: accounting.Provenance{DeploymentID: "private-125", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch-1", PoolCodeEpoch: "epoch-1", ReceiptSHA256: strings.Repeat("c", 64), HeaderSHA256: strings.Repeat("d", 64)}, BlockTimestampSeconds: "1700000000"}
	f.SourceFillID = accounting.SourceFillIdentity(f.Position, f.Provenance.DeploymentID)
	digest, err := accounting.FillDigest(f)
	if err != nil {
		t.Fatal(err)
	}
	position := orderedAppend(t, ctx, pool, partitions, AppendInput{ProjectionVersion: "market-v1", PartitionKind: "market", PartitionKey: "125:" + f.PoolAddress, SourceEventID: f.SourceFillID, EffectIdentity: accounting.EffectIdentity(f.SourceFillID, "market", "market-v1", "", "market", "125:"+f.PoolAddress), PayloadDigest: digest, SourceOrder: f.Position})
	publication, err := accounting.NewMarketPublication(f, []accounting.PartitionPosition{position})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := accounting.CanonicalPublicationV1(publication)
	if err != nil {
		t.Fatal(err)
	}
	envelope := Envelope{EventID: publication.PublicationID, EventType: subject, SchemaVersion: 1, ChainID: 125, SourceOrder: SourceOrder{Height: f.Position.Height, TransactionIndex: f.Position.TransactionIndex, LogIndex: f.Position.LogIndex}, EmittedAt: time.Unix(1700000000, 0).UTC(), TraceID: publication.PublicationID, PayloadHash: Hash(payload), Payload: payload}
	wire, err := envelope.Encode(policy, subject)
	if err != nil {
		t.Fatal(err)
	}
	key := ConsumerKey{ConsumerID: "ordered_batch", ProjectionGeneration: "g1", PartitionKind: position.PartitionKind, PartitionKey: position.PartitionKey}
	lease := orderedBootstrap(t, ctx, pool, partitions, table, key, time.Minute)
	hooks := OrderedBatchHooks{
		Guard: func(ctx context.Context, tx pgx.Tx, e Envelope) (GuardedPosition, error) {
			return partitions.CheckAndLockTx(ctx, tx, lease, position)
		},
		Effect: func(ctx context.Context, tx pgx.Tx, effects []OrderedEffect) error {
			if len(effects) != 1 || effects[0].Envelope.EventID != publication.PublicationID || effects[0].Position.Position != position {
				return ErrIntegrity
			}
			_, err := tx.Exec(ctx, "INSERT INTO "+table+".ordered_effects(id) VALUES($1)", effects[0].Envelope.EventID)
			return err
		},
	}
	connection := orderedBroker(t)
	jetstream, err := connection.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jetstream.AddStream(&nats.StreamConfig{Name: "ORDERED_BATCH", Subjects: []string{subject}}); err != nil {
		t.Fatal(err)
	}
	subscription, err := jetstream.PullSubscribe(subject, "ordered_batch", nats.BindStream("ORDERED_BATCH"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = subscription.Unsubscribe() })
	for i := 0; i < 2; i++ {
		if _, err := jetstream.Publish(subject, wire); err != nil {
			t.Fatal(err)
		}
		messages, err := subscription.Fetch(1, nats.MaxWait(3*time.Second))
		if err != nil || len(messages) != 1 {
			t.Fatalf("broker delivery: %d, %v", len(messages), err)
		}
		message := orderedBrokerMessage{messages[0]}
		if err := store.ConsumeOrderedBatch(ctx, "ordered_batch", "g1", []OrderedDelivery{{Subject: subject, Wire: messages[0].Data, Delivery: message}}, hooks); err != nil {
			t.Fatal(err)
		}
	}
	poison := envelope
	poison.EventID = "invalid_publication"
	poison.TraceID = poison.EventID
	poison.Payload = []byte(`{}`)
	poison.PayloadHash = Hash(poison.Payload)
	if _, err := accounting.DecodePublicationV1(poison.Payload); err == nil {
		t.Fatal("invalid publication decoded as a market fact")
	}
	poisonWire, err := poison.Encode(policy, subject)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jetstream.Publish(subject, poisonWire); err != nil {
		t.Fatal(err)
	}
	poisonReceived := false
	for attempt := 0; attempt < 4; attempt++ {
		messages, err := subscription.Fetch(1, nats.MaxWait(3*time.Second))
		if err != nil || len(messages) != 1 {
			t.Fatalf("broker poison delivery: %d, %v", len(messages), err)
		}
		input := []OrderedDelivery{{Subject: subject, Wire: messages[0].Data, Delivery: orderedBrokerMessage{messages[0]}}}
		if bytes.Equal(messages[0].Data, wire) {
			if err := store.ConsumeOrderedBatch(ctx, "ordered_batch", "g1", input, hooks); err != nil {
				t.Fatalf("broker redelivery: %v", err)
			}
			continue
		}
		if !bytes.Equal(messages[0].Data, poisonWire) {
			t.Fatal("broker delivered a different publication")
		}
		poisonReceived = true
		if err := store.ConsumeOrderedBatch(ctx, "ordered_batch", "g1", input, hooks); err == nil {
			t.Fatal("invalid publication advanced the ordered batch")
		}
		break
	}
	if !poisonReceived {
		t.Fatal("invalid publication was not delivered")
	}
	var quarantineClass string
	if err := pool.QueryRow(ctx, "SELECT failure_class FROM "+table+".delivery_ordered_quarantine WHERE consumer_id=$1 AND envelope_hash=$2", "ordered_batch", Hash(poisonWire)).Scan(&quarantineClass); err != nil || quarantineClass != string(InnerSchema) {
		t.Fatalf("invalid publication quarantine: %s, %v", quarantineClass, err)
	}
	singleKey := ConsumerKey{ConsumerID: "ordered_single", ProjectionGeneration: "g1", PartitionKind: position.PartitionKind, PartitionKey: position.PartitionKey}
	singleLease := orderedBootstrap(t, ctx, pool, partitions, table, singleKey, time.Minute)
	singleHooks := ConsumeHooks{
		Guard: func(ctx context.Context, tx pgx.Tx, e Envelope) (GuardedPosition, error) {
			return partitions.CheckAndLockTx(ctx, tx, singleLease, position)
		},
		Effect: func(ctx context.Context, tx pgx.Tx, e Envelope, g GuardedPosition) error {
			if e.EventID != publication.PublicationID || g.Position != position {
				return ErrIntegrity
			}
			_, err := tx.Exec(ctx, "INSERT INTO "+table+".ordered_effects(id) VALUES($1)", "single_"+e.EventID)
			return err
		},
		Advance: func(ctx context.Context, tx pgx.Tx, e Envelope, g GuardedPosition) error {
			return partitions.AdvanceTx(ctx, tx, g)
		},
	}
	singleSubscription, err := jetstream.PullSubscribe(subject, "ordered_single", nats.BindStream("ORDERED_BATCH"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = singleSubscription.Unsubscribe() })
	for i := 0; i < 2; i++ {
		messages, err := singleSubscription.Fetch(1, nats.MaxWait(3*time.Second))
		if err != nil || len(messages) != 1 || !bytes.Equal(messages[0].Data, wire) {
			t.Fatalf("single ordered delivery: %d, %v", len(messages), err)
		}
		if err := store.ConsumeOrderedWithWatermark(ctx, "ordered_single", "g1", subject, messages[0].Data, orderedBrokerMessage{messages[0]}, singleHooks); err != nil {
			t.Fatal(err)
		}
	}
	var effects, inbox, next, watermark int64
	if err := pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM "+table+".ordered_effects WHERE id=$1),(SELECT count(*) FROM "+table+".delivery_inbox WHERE consumer_id='ordered_batch'),(SELECT next_sequence FROM "+table+".delivery_partition_offsets WHERE consumer_id='ordered_batch'),(SELECT broker_sequence FROM "+table+".delivery_consumer_watermarks WHERE consumer_id='ordered_batch')", publication.PublicationID).Scan(&effects, &inbox, &next, &watermark); err != nil || effects != 1 || inbox != 1 || next != 2 || watermark != 1 {
		t.Fatalf("ordered batch state: effects=%d inbox=%d next=%d watermark=%d err=%v", effects, inbox, next, watermark, err)
	}
	if err := pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM "+table+".ordered_effects WHERE id=$1),(SELECT count(*) FROM "+table+".delivery_inbox WHERE consumer_id='ordered_single'),(SELECT next_sequence FROM "+table+".delivery_partition_offsets WHERE consumer_id='ordered_single'),(SELECT broker_sequence FROM "+table+".delivery_consumer_watermarks WHERE consumer_id='ordered_single')", "single_"+publication.PublicationID).Scan(&effects, &inbox, &next, &watermark); err != nil || effects != 1 || inbox != 1 || next != 2 || watermark != 1 {
		t.Fatalf("single ordered state: effects=%d inbox=%d next=%d watermark=%d err=%v", effects, inbox, next, watermark, err)
	}
	if err := store.ConsumeOrderedBatch(ctx, "ordered_batch", "g1", nil, hooks); err == nil {
		t.Fatal("empty ordered batch was admitted")
	}
	changedIdentity := envelope
	changedIdentity.EventID = "changed-publication-identity"
	changedIdentity.TraceID = changedIdentity.EventID
	changedWire, err := changedIdentity.Encode(policy, subject)
	if err != nil {
		t.Fatal(err)
	}
	identitySubscription, err := jetstream.PullSubscribe(subject, "identity_probe", nats.BindStream("ORDERED_BATCH"), nats.DeliverNew())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = identitySubscription.Unsubscribe() })
	if _, err := jetstream.Publish(subject, changedWire); err != nil {
		t.Fatal(err)
	}
	changedMessages, err := identitySubscription.Fetch(1, nats.MaxWait(3*time.Second))
	if err != nil || len(changedMessages) != 1 || !bytes.Equal(changedMessages[0].Data, changedWire) {
		t.Fatalf("changed publication delivery: %d, %v", len(changedMessages), err)
	}
	if err := store.ConsumeOrderedWithWatermark(ctx, "ordered_single", "g1", subject, changedMessages[0].Data, orderedBrokerMessage{changedMessages[0]}, singleHooks); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed publication identity = %v", err)
	}
	var class string
	if err := pool.QueryRow(ctx, "SELECT failure_class FROM "+table+".delivery_ordered_quarantine WHERE consumer_id=$1 AND envelope_hash=$2", "ordered_single", Hash(changedWire)).Scan(&class); err != nil || class != string(PartitionBinding) {
		t.Fatalf("durable partition binding rejection = %q, %v", class, err)
	}
	if !errors.Is((&OrderedBatchFailure{EventID: envelope.EventID, Cause: ErrIntegrity}).Unwrap(), ErrIntegrity) {
		t.Fatal("ordered batch failure lost its cause")
	}
}
