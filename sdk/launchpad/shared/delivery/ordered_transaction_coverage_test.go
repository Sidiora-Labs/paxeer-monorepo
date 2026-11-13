package delivery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

func orderedPublicationFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, partitions *PartitionStore) (Envelope, accounting.PartitionPosition, []byte, Policy) {
	t.Helper()
	subject := accounting.MarketSubject
	policy := Policy{Subjects: map[string]bool{subject: true}, EventTypes: map[string]bool{subject: true}}
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
	return envelope, position, wire, policy
}

func TestOrderedTransactionsPreserveOffsetsOnRejectedWork(t *testing.T) {
	connection := orderedBroker(t)
	broker, err := connection.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broker.AddStream(&nats.StreamConfig{Name: "TRANSACTION_BOUNDARIES", Subjects: []string{accounting.MarketSubject}}); err != nil {
		t.Fatal(err)
	}
	denied := errors.New("projection admission denied")
	for _, consumerMode := range []string{"single", "batch", "watermark"} {
		for _, scenario := range []string{"guard", "serialization", "deadlock", "effect", "effect identity", "lease identity", "position identity", "duplicate status", "inbox unavailable", "offset unavailable", "advance", "commit identity", "epoch admission", "epoch commit", "cancel retry"} {
			t.Run(fmt.Sprintf("%s/%s", consumerMode, scenario), func(t *testing.T) {
				ctx, pool, partitions, table := orderedPostgres(t)
				envelope, position, wire, policy := orderedPublicationFixture(t, ctx, pool, partitions)
				schema := os.Getenv("DELIVERY_TEST_SCHEMA") + "_ordered"
				store, err := NewStore(pool, schema, policy)
				if err != nil {
					t.Fatal(err)
				}
				ddl, err := ReplayDDL(schema)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := pool.Exec(ctx, ddl); err != nil {
					t.Fatal(err)
				}
				key := ConsumerKey{ConsumerID: "transaction_consumer", ProjectionGeneration: "g1", PartitionKind: position.PartitionKind, PartitionKey: position.PartitionKey}
				lease := orderedBootstrap(t, ctx, pool, partitions, table, key, time.Minute)
				subscription, err := broker.PullSubscribe(accounting.MarketSubject, fmt.Sprintf("boundary_%d", time.Now().UnixNano()), nats.BindStream("TRANSACTION_BOUNDARIES"), nats.DeliverNew())
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = subscription.Unsubscribe() }()
				if _, err := broker.Publish(accounting.MarketSubject, wire); err != nil {
					t.Fatal(err)
				}
				messages, err := subscription.Fetch(1, nats.MaxWait(3*time.Second))
				if err != nil || len(messages) != 1 {
					t.Fatalf("publication delivery: %d %v", len(messages), err)
				}
				delivery := orderedBrokerMessage{messages[0]}
				workCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				calls, epochs := 0, 0
				if strings.HasPrefix(scenario, "epoch") {
					store, err = NewStoreWithEpochGuard(pool, schema, policy, func(context.Context, pgx.Tx) error {
						epochs++
						if scenario == "epoch admission" || epochs == 2 {
							return denied
						}
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
				}
				guard := func(ctx context.Context, tx pgx.Tx, e Envelope) (GuardedPosition, error) {
					calls++
					if e.EventID != envelope.EventID {
						return GuardedPosition{}, ErrIntegrity
					}
					switch scenario {
					case "guard":
						return GuardedPosition{}, denied
					case "serialization", "deadlock", "cancel retry":
						code := "40001"
						if scenario == "deadlock" {
							code = "40P01"
						}
						_, err := tx.Exec(ctx, "DO $$ BEGIN RAISE EXCEPTION 'transaction conflict' USING ERRCODE='"+code+"'; END $$")
						if scenario == "cancel retry" {
							cancel()
						}
						return GuardedPosition{}, err
					}
					g, err := partitions.CheckAndLockTx(ctx, tx, lease, position)
					if err != nil {
						return g, err
					}
					switch scenario {
					case "lease identity":
						g.Lease.Key.ConsumerID = "another_consumer"
					case "position identity":
						g.Position.PayloadDigest = accounting.Digest(Hash([]byte("another payload")))
					case "duplicate status":
						g.DuplicateCommitted = true
					case "inbox unavailable":
						_, err = tx.Exec(ctx, "DROP TABLE "+store.table("delivery_inbox"))
					case "offset unavailable":
						_, err = tx.Exec(ctx, "DROP TABLE "+store.table("delivery_partition_offsets"))
					}
					return g, err
				}
				effect := func(ctx context.Context, tx pgx.Tx, e Envelope, g GuardedPosition) error {
					switch scenario {
					case "effect":
						return &OrderedBatchFailure{EventID: e.EventID, Cause: denied}
					case "effect identity":
						return &OrderedBatchFailure{EventID: "another_event", Cause: denied}
					}
					_, err := tx.Exec(ctx, "INSERT INTO "+table+".ordered_effects(id) VALUES($1)", e.EventID)
					return err
				}
				advance := func(ctx context.Context, tx pgx.Tx, e Envelope, g GuardedPosition) error {
					if scenario == "advance" {
						return denied
					}
					if err := partitions.AdvanceTx(ctx, tx, g); err != nil {
						return err
					}
					if scenario == "commit identity" {
						_, err := tx.Exec(ctx, "DELETE FROM "+store.table("delivery_partition_commits"))
						return err
					}
					return nil
				}
				if consumerMode == "batch" {
					err = store.ConsumeOrderedBatch(workCtx, key.ConsumerID, key.ProjectionGeneration, []OrderedDelivery{{Subject: accounting.MarketSubject, Wire: wire, Delivery: delivery}}, OrderedBatchHooks{Guard: guard, Effect: func(ctx context.Context, tx pgx.Tx, values []OrderedEffect) error {
						if len(values) != 1 {
							return ErrIntegrity
						}
						if err := effect(ctx, tx, values[0].Envelope, values[0].Position); err != nil {
							return err
						}
						if scenario == "advance" {
							return denied
						}
						if scenario == "commit identity" {
							_, err := tx.Exec(ctx, "DELETE FROM "+store.table("delivery_partition_commits"))
							return err
						}
						return nil
					}})
				} else if consumerMode == "watermark" {
					err = store.ConsumeOrderedWithWatermark(workCtx, key.ConsumerID, key.ProjectionGeneration, accounting.MarketSubject, wire, delivery, ConsumeHooks{Guard: guard, Effect: effect, Advance: advance})
				} else {
					err = store.ConsumeOrdered(workCtx, key.ConsumerID, key.ProjectionGeneration, accounting.MarketSubject, wire, delivery, ConsumeHooks{Guard: guard, Effect: effect, Advance: advance})
				}
				if err == nil {
					t.Fatal("rejected projection committed")
				}
				if scenario == "serialization" || scenario == "deadlock" {
					var pgerr *pgconn.PgError
					want := "40001"
					if scenario == "deadlock" {
						want = "40P01"
					}
					if !errors.As(err, &pgerr) || pgerr.Code != want || calls != 3 {
						t.Fatalf("transaction retry: calls=%d error=%v", calls, err)
					}
				}
				if scenario == "effect" || scenario == "effect identity" {
					var failure *OrderedBatchFailure
					if !errors.Is(err, denied) || !errors.As(err, &failure) || !strings.Contains(failure.Error(), failure.EventID) {
						t.Fatalf("projection error identity: %v", err)
					}
				}
				if scenario == "cancel retry" && !errors.Is(err, context.Canceled) {
					t.Fatalf("retry cancellation: %v", err)
				}
				var inbox, effects, next int64
				if err := pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM "+store.table("delivery_inbox")+"),(SELECT count(*) FROM "+table+".ordered_effects),(SELECT next_sequence FROM "+store.table("delivery_partition_offsets")+" WHERE consumer_id=$1)", key.ConsumerID).Scan(&inbox, &effects, &next); err != nil || inbox != 0 || effects != 0 || next != 1 {
					t.Fatalf("rollback state: inbox=%d effects=%d next=%d error=%v", inbox, effects, next, err)
				}
				info, err := subscription.ConsumerInfo()
				if err != nil || info.NumAckPending != 1 {
					t.Fatalf("uncommitted broker acknowledgement: %+v %v", info, err)
				}
			})
		}
	}
}
