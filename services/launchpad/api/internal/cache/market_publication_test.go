package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func marketTestBroker(t *testing.T) (*nats.Conn, jetstream.JetStream) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("nats-server", "-js", "-p", strconv.Itoa(port), "-sd", t.TempDir())
	command.Stdout, command.Stderr = io.Discard, io.Discard
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
	address := fmt.Sprintf("nats://127.0.0.1:%d", port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		connection, err := nats.Connect(address, nats.NoReconnect(), nats.Timeout(100*time.Millisecond))
		if err == nil {
			t.Cleanup(connection.Close)
			js, err := jetstream.New(connection)
			if err != nil {
				t.Fatal(err)
			}
			return connection, js
		}
		select {
		case err := <-done:
			t.Fatalf("market broker stopped before readiness: %v", err)
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("market broker did not become ready")
	return nil, nil
}

func marketTestPublication(t *testing.T) (accounting.PublicationV1, accounting.SourcePosition) {
	t.Helper()
	position := accounting.SourcePosition{ChainID: "125", Height: "110", TransactionIndex: "0", LogIndex: "0", TransactionHash: "0x" + strings.Repeat("a", 64), BlockHash: "0x" + strings.Repeat("b", 64)}
	pool := repairM992Pool
	fill := accounting.FillV1{
		Version: accounting.FillVersion, Position: position, ActualEmitter: pool, PoolAddress: pool,
		Token:      accounting.Asset{Address: "0x" + strings.Repeat("3", 40), Decimals: 18},
		Settlement: accounting.Asset{Address: "0x" + strings.Repeat("4", 40), Decimals: 6},
		Side:       "buy", TokenAmountRaw: new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil).String(), USDLAmountRaw: "1000000", PaidInputRaw: "1000000", FeeRaw: "3000",
		InputDerivationQuality: "receipt_incomplete", FeeDisposition: "distributed_usdl",
		Attribution:           accounting.Attribution{TransactionSender: "0x" + strings.Repeat("5", 40), Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Execution:             accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Provenance:            accounting.Provenance{DeploymentID: "fixture125", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch1", PoolCodeEpoch: "epoch1", ReceiptSHA256: strings.Repeat("c", 64), HeaderSHA256: strings.Repeat("d", 64)},
		BlockTimestampSeconds: "1700000000",
	}
	fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	partition := accounting.PartitionPosition{ProjectionVersion: "v2", PartitionKind: "market", PartitionKey: "125:" + pool, Sequence: "1", SourceEventID: fill.SourceFillID, PayloadDigest: digest, SourceOrder: position}
	partition.EffectIdentity = accounting.EffectIdentity(fill.SourceFillID, "market", "v2", "", "market", partition.PartitionKey)
	publication, err := accounting.NewMarketPublication(fill, []accounting.PartitionPosition{partition})
	if err != nil {
		t.Fatal(err)
	}
	return publication, position
}

func TestMarketPublicationInvalidatesCachedRevision(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	client, err := sharedredis.NewBoundedClient(internaltest.NewRedisURL(t), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	config := repairM992Config(t, 0)
	config.Adapter.Budget.Pool = db
	config.Adapter.Redis = client
	config.Adapter.RedisTimeout = time.Second
	runtime := repairM992Runtime(t, config)
	publication, position := marketTestPublication(t)
	payload, err := accounting.CanonicalPublicationV1(publication)
	if err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{accounting.MarketSubject: true}, EventTypes: map[string]bool{accounting.MarketSubject: true}}
	envelope := delivery.Envelope{EventID: publication.PublicationID, EventType: accounting.MarketSubject, SchemaVersion: 1, ChainID: 125,
		SourceOrder: delivery.SourceOrder{Height: position.Height, TransactionIndex: position.TransactionIndex, LogIndex: position.LogIndex},
		EmittedAt:   time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), TraceID: "market-test", Payload: json.RawMessage(payload), PayloadHash: delivery.Hash(payload)}
	wire, err := envelope.Encode(policy, accounting.MarketSubject)
	if err != nil {
		t.Fatal(err)
	}
	connection, js := marketTestBroker(t)
	consumerConfig := config.MarketConsumer
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: consumerConfig.Stream, Subjects: []string{accounting.MarketSubject}, Storage: jetstream.FileStorage, Replicas: 1}); err != nil {
		t.Fatal(err)
	}
	approved, err := consumerConfig.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateConsumer(ctx, consumerConfig.Stream, approved); err != nil {
		t.Fatal(err)
	}
	consumer, err := natsdriver.NewConsumer(ctx, connection, consumerConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(consumer.Close)
	publisher, err := natsdriver.NewPublisher(connection, consumerConfig.Stream)
	if err != nil {
		t.Fatal(err)
	}
	if ack, err := publisher.Publish(ctx, accounting.MarketSubject, publication.PublicationID, wire); err != nil || ack.Sequence != 1 {
		t.Fatalf("market publication: %+v %v", ack, err)
	}
	messages, err := consumer.Fetch(ctx)
	if err != nil || len(messages) != 1 {
		t.Fatalf("market delivery: %d %v", len(messages), err)
	}
	if err := runtime.bindings.ConsumeMarket(ctx, messages[0]); err != nil {
		t.Fatal(err)
	}
	key := runtime.bindings.fenceKey(runtime.bindings.config, repairM992Pool)
	floor, found, err := sharedredis.GetPolicySnapshot(ctx, client, key)
	if err != nil || found || floor.Revision != "1" || string(floor.Body) != "null" || floor.Generation != "g1" {
		t.Fatalf("market invalidation floor: %+v found=%t err=%v", floor, found, err)
	}
	messages, err = consumer.Fetch(ctx)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) || len(messages) != 0 {
		t.Fatalf("acknowledged market delivery replayed: %d %v", len(messages), err)
	}
}
