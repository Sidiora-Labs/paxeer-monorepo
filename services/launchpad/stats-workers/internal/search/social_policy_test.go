package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
)

type socialPolicyDelivery struct {
	pool       *pgxpool.Pool
	projection *Projection
	consumer   *natsdriver.Consumer
	broker     jetstream.Consumer
	publisher  *natsdriver.Publisher
	policy     delivery.Policy
	subject    string
}

func newSocialPolicyDelivery(t *testing.T) *socialPolicyDelivery {
	t.Helper()
	ctx := context.Background()
	pool := integratedDatabase(t)
	if _, err := pool.Exec(ctx, `CREATE SCHEMA chat; CREATE SCHEMA indexer; CREATE SCHEMA social_delivery;
CREATE TABLE indexer.indexer_canonical_creations_v1(pool text PRIMARY KEY,token text NOT NULL);
CREATE TABLE chat.safety_visibility(target_type text NOT NULL,target_id text NOT NULL,status text NOT NULL,revision bigint NOT NULL,PRIMARY KEY(target_type,target_id));`); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(coordinatortest.RepositoryRoot(t), "migrations/380003.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	ddl, err := delivery.SchemaDDL("social_delivery")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	nc := internaltest.ConnectNATS(t, os.Getenv("L20_NATS_URL"))
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	nonce := strings.ReplaceAll(uuid.NewString(), "-", "")
	stream, subject := "SOCIAL_SEARCH_"+nonce, "social_search_"+nonce+".policy"
	if _, err = js.CreateStream(ctx, jetstream.StreamConfig{Name: stream, Subjects: []string{subject}, Storage: jetstream.FileStorage, MaxBytes: 1048576}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bounded, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := js.DeleteStream(bounded, stream); err != nil {
			t.Error(err)
		}
	})
	config := natsdriver.ConsumerConfig{Stream: stream, Durable: "social_search", Subject: subject, Batch: 1, MaxAckPending: 1, MaxWaiting: 1, MaxDeliver: 3, FetchWait: time.Second, AckWait: time.Second}
	native, err := config.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	broker, err := js.CreateConsumer(ctx, stream, native)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := natsdriver.NewConsumer(ctx, nc, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(consumer.Close)
	publisher, err := natsdriver.NewPublisher(nc, stream)
	if err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{subject: true, "search.invalidation.v2": true}, EventTypes: map[string]bool{"social.policy.changed.v2": true, "profile.updated": true, "search.invalidation.v2": true}}
	store, err := delivery.NewStore(pool, "social_delivery", policy)
	if err != nil {
		t.Fatal(err)
	}
	return &socialPolicyDelivery{pool: pool, projection: &Projection{Delivery: store, ConsumerID: "social_search", InboxGeneration: "g1", InvalidationSubject: "search.invalidation.v2", InvalidationEventType: "search.invalidation.v2"}, consumer: consumer, broker: broker, publisher: publisher, policy: policy, subject: subject}
}

func (h *socialPolicyDelivery) publish(t *testing.T, eventType, kind, targetType, target string) *natsdriver.Message {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"kind": kind, "entityId": targetType + ":" + target, "revision": 1, "visibility": map[string]any{"targetType": targetType, "targetId": target, "status": "removed", "caseId": uuid.NewString(), "revision": 1}})
	if err != nil {
		t.Fatal(err)
	}
	id := "social_policy_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	event := delivery.Envelope{EventID: id, EventType: eventType, SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: "1", TransactionIndex: "0", LogIndex: "0"}, EmittedAt: time.Now().UTC(), TraceID: id, Payload: payload, PayloadHash: delivery.Hash(payload)}
	wire, err := event.Encode(h.policy, h.subject)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.publisher.Publish(context.Background(), h.subject, id, wire); err != nil {
		t.Fatal(err)
	}
	messages, err := h.consumer.Fetch(context.Background())
	if err != nil || len(messages) != 1 {
		t.Fatalf("policy delivery: %d %v", len(messages), err)
	}
	return messages[0]
}

func (h *socialPolicyDelivery) consume(t *testing.T, message *natsdriver.Message) {
	t.Helper()
	if err := h.projection.ConsumeSocialPolicy(context.Background(), h.subject, message.Wire, message); err != nil {
		t.Fatal(err)
	}
	info, err := h.broker.Info(context.Background())
	if err != nil || info.NumAckPending != 0 {
		t.Fatalf("policy acknowledgement: %+v %v", info, err)
	}
}

func socialSearchState(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var state string
	err := pool.QueryRow(context.Background(), `SELECT jsonb_build_array(
(SELECT jsonb_agg(to_jsonb(a) || jsonb_build_object('xmin',xmin::text) ORDER BY pool_address) FROM stats_search.authority_v2 a),
(SELECT jsonb_agg(to_jsonb(a) || jsonb_build_object('xmin',xmin::text)) FROM stats_search.active_v2 a),
(SELECT count(*) FROM social_delivery.delivery_outbox))::text`).Scan(&state)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestConsumeSocialPolicyVisibilityAndRecovery(t *testing.T) {
	h := newSocialPolicyDelivery(t)
	ctx := context.Background()
	d := seedIndexed(t, h.pool, 401)
	if _, err := h.pool.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations_v1 VALUES($1,$2);`, d.PoolAddress, d.TokenAddress); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, `INSERT INTO chat.safety_visibility VALUES('pool',$1,'removed',2)`, d.PoolAddress); err != nil {
		t.Fatal(err)
	}
	assertCreationSearch(t, h.pool, d, true)
	message := h.publish(t, "social.policy.changed.v2", "visibility_changed", "pool", strings.ToUpper(d.PoolAddress))
	h.consume(t, message)
	assertCreationAuthority(t, h.pool, d.PoolAddress, 3, false, false)
	assertCreationSearch(t, h.pool, d, false)
	before := socialSearchState(t, h.pool)
	store, err := delivery.NewStore(h.pool, "social_delivery", h.policy)
	if err != nil {
		t.Fatal(err)
	}
	h.projection = &Projection{Delivery: store, ConsumerID: "social_search", InboxGeneration: "g1", InvalidationSubject: "search.invalidation.v2", InvalidationEventType: "search.invalidation.v2"}
	// A new durable delivers the same persisted event after the projection restarts.
	config := natsdriver.ConsumerConfig{Stream: h.consumerStream(t), Durable: "social_replay", Subject: h.subject, Batch: 1, MaxAckPending: 1, MaxWaiting: 1, MaxDeliver: 3, FetchWait: time.Second, AckWait: time.Second}
	nc := internaltest.ConnectNATS(t, os.Getenv("L20_NATS_URL"))
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	native, err := config.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = js.CreateConsumer(ctx, config.Stream, native); err != nil {
		t.Fatal(err)
	}
	replay, err := natsdriver.NewConsumer(ctx, nc, config)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Close()
	messages, err := replay.Fetch(ctx)
	if err != nil || len(messages) != 1 || string(messages[0].Wire) != string(message.Wire) {
		t.Fatalf("policy replay: %d %v", len(messages), err)
	}
	h.consume(t, messages[0])
	if after := socialSearchState(t, h.pool); after != before {
		t.Fatalf("duplicate policy changed search: %s != %s", after, before)
	}
	if _, err = h.pool.Exec(ctx, `UPDATE chat.safety_visibility SET status='restored',revision=5 WHERE target_type='pool' AND target_id=$1`, d.PoolAddress); err != nil {
		t.Fatal(err)
	}
	h.consume(t, h.publish(t, "social.policy.changed.v2", "visibility_changed", "pool", d.PoolAddress))
	assertCreationAuthority(t, h.pool, d.PoolAddress, 6, true, false)
	assertCreationSearch(t, h.pool, d, true)
	var count int
	if err = h.pool.QueryRow(ctx, `SELECT count(*) FROM social_delivery.delivery_outbox`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("visibility invalidations: %d %v", count, err)
	}
	before = socialSearchState(t, h.pool)
	h.consume(t, h.publish(t, "social.policy.changed.v2", "visibility_changed", "pool", d.PoolAddress))
	if socialSearchState(t, h.pool) != before {
		t.Fatal("unchanged visibility wrote search state")
	}
}

func (h *socialPolicyDelivery) consumerStream(t *testing.T) string {
	t.Helper()
	info, err := h.broker.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return info.Stream
}

func TestConsumeSocialPolicyTokenAndNoops(t *testing.T) {
	h := newSocialPolicyDelivery(t)
	ctx := context.Background()
	d := seedIndexed(t, h.pool, 402)
	if _, err := h.pool.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations_v1 VALUES($1,$2)`, d.PoolAddress, d.TokenAddress); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, `INSERT INTO chat.safety_visibility VALUES('token',$1,'removed',2)`, d.TokenAddress); err != nil {
		t.Fatal(err)
	}
	h.consume(t, h.publish(t, "social.policy.changed.v2", "visibility_changed", "token", strings.ToUpper(d.TokenAddress)))
	assertCreationAuthority(t, h.pool, d.PoolAddress, 3, false, false)
	assertCreationSearch(t, h.pool, d, false)
	before := socialSearchState(t, h.pool)
	for _, tc := range []struct{ kind, targetType, target string }{
		{"ban_changed", "pool", d.PoolAddress},
		{"visibility_changed", "comment", d.PoolAddress},
		{"visibility_changed", "pool", fixtureAddress(909)},
		{"visibility_changed", "token", fixtureAddress(910)},
	} {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			h.consume(t, h.publish(t, "social.policy.changed.v2", tc.kind, tc.targetType, tc.target))
			if socialSearchState(t, h.pool) != before {
				t.Fatal("irrelevant policy changed search state")
			}
		})
	}
	message := h.publish(t, "profile.updated", "visibility_changed", "pool", d.PoolAddress)
	if err := h.projection.ConsumeSocialPolicy(ctx, h.subject, message.Wire, message); !errors.Is(err, ErrInvalid) {
		t.Fatalf("wrong event type: %v", err)
	}
	info, err := h.broker.Info(ctx)
	if err != nil || info.NumAckPending != 1 {
		t.Fatalf("refused event acknowledgement: %+v %v", info, err)
	}
	var count int
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM social_delivery.delivery_inbox`).Scan(&count); err != nil || count != 5 {
		t.Fatalf("policy inbox: %d %v", count, err)
	}
	if socialSearchState(t, h.pool) != before {
		t.Fatal("refused event changed search state")
	}
}

func TestConsumeSocialPolicyRuntimeRole(t *testing.T) {
	ctx := context.Background()
	database := coordinatortest.Bootstrap(t, os.Getenv("COORDINATOR_TEST_POSTGRES_DSN"), "stats_search_runtime", "stats_runtime")
	admin, err := pgxpool.New(ctx, database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	pool, err := pgxpool.New(ctx, database.Runtimes["stats_search_runtime"].DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	address, token := fixtureAddress(501), fixtureAddress(502)
	if _, err = admin.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$2,$2,$2,1,1,'block','transaction',0,0,1,'{}','{}')`, token, address); err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct{ kind, address string }{{"pool", address}, {"token", token}} {
		rows, err := pool.Query(ctx, socialPoolVisibilitySQL, target.kind, strings.ToUpper(target.address))
		if err != nil {
			t.Fatal(err)
		}
		var found string
		var public bool
		var revision int64
		if !rows.Next() {
			t.Fatalf("runtime visibility row missing: %v", rows.Err())
		}
		if err = rows.Scan(&found, &public, &revision); err != nil || found != address || !public || revision != 0 {
			t.Fatalf("runtime visibility: %s %t %d %v", found, public, revision, err)
		}
		if rows.Next() || rows.Err() != nil {
			t.Fatal("unexpected runtime visibility rows", rows.Err())
		}
		rows.Close()
	}
	writer, err := pgxpool.New(ctx, database.Runtimes["stats_runtime"].DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	tx, err := writer.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	changed, err := applyChangeTx(ctx, tx, Change{Version: 2, Kind: "visibility", Revision: 1, PoolAddress: address, Public: true})
	if err != nil || !changed {
		t.Fatalf("runtime visibility authority write: %t %v", changed, err)
	}
}

func TestConsumeSocialPolicyStatementInventory(t *testing.T) {
	raw, err := os.ReadFile("../analytics/runtime_statements.json")
	if err != nil {
		t.Fatal(err)
	}
	var inventory struct {
		Statements []struct {
			Function    string            `json:"function"`
			SQL         string            `json:"sql"`
			RuntimeRole string            `json:"runtime_role"`
			Sources     map[string]string `json:"sources"`
		} `json:"statements"`
	}
	if err := json.Unmarshal(raw, &inventory); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, statement := range inventory.Statements {
		if statement.Function == "Projection.ConsumeSocialPolicy" && statement.SQL == socialPoolVisibilitySQL {
			if statement.RuntimeRole != "stats_search_runtime" || statement.Sources["socialPoolVisibilityView"] != socialPoolVisibilityView || statement.Sources["socialPoolVisibilitySQL"] != socialPoolVisibilitySQL {
				t.Fatal("social visibility statement sources differ from the projection")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("social visibility statement is missing from the inventory")
	}
}
