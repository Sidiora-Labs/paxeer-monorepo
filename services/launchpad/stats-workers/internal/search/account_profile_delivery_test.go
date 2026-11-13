package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

func TestAccountProfileDeliveryTracksPublicConsentAndDeletion(t *testing.T) {
	pool := integratedDatabase(t)
	ctx := context.Background()
	ddl := `CREATE SCHEMA users; CREATE SCHEMA profile_delivery;
CREATE TABLE users.account_state(account_id uuid PRIMARY KEY,wallet text NOT NULL,profile jsonb NOT NULL,revision bigint NOT NULL,updated_at timestamptz NOT NULL,deleted boolean NOT NULL);
CREATE TABLE users.identity_wallet_links(account_id uuid NOT NULL,wallet text NOT NULL,chain_id integer NOT NULL,state text NOT NULL,visibility text NOT NULL);
CREATE VIEW users.account_public_search_v2 AS SELECT a.wallet,a.profile->>'displayName' AS display_name,a.revision,a.updated_at,EXISTS(SELECT 1 FROM users.identity_wallet_links l WHERE l.account_id=a.account_id AND l.wallet=a.wallet AND l.chain_id=125 AND l.state='verified' AND l.visibility='public_optin') AND NOT a.deleted AS public,a.deleted FROM users.account_state a;
CREATE TABLE users.account_outbox(event_id uuid PRIMARY KEY,account_id uuid NOT NULL,kind text NOT NULL,payload jsonb NOT NULL,revision bigint NOT NULL);`
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	schema, err := delivery.SchemaDDL("profile_delivery")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		t.Fatal(err)
	}
	brokerURL := os.Getenv("L20_NATS_URL")
	if brokerURL == "" {
		t.Fatal("L20_NATS_URL required")
	}
	nc := internaltest.ConnectNATS(t, brokerURL)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	nonce := strings.ReplaceAll(uuid.NewString(), "-", "")
	stream, subject := "PROFILE_"+nonce, "profile_"+nonce+".events"
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: stream, Subjects: []string{subject}, Storage: jetstream.FileStorage, MaxBytes: 1048576}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := js.DeleteStream(cleanup, stream); err != nil {
			t.Error(err)
		}
	})
	config := natsdriver.ConsumerConfig{Stream: stream, Durable: "profile", Subject: subject, Batch: 1, MaxAckPending: 1, MaxWaiting: 1, MaxDeliver: 3, FetchWait: time.Second, AckWait: time.Second}
	native, err := config.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateOrUpdateConsumer(ctx, stream, native); err != nil {
		t.Fatal(err)
	}
	consumer, err := natsdriver.NewConsumer(ctx, nc, config)
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()
	publisher, err := natsdriver.NewPublisher(nc, stream)
	if err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{subject: true, "profile.invalidation": true}, EventTypes: map[string]bool{"profile.updated": true, "account.deletion.requested": true, "search_invalidation": true}}
	inbox, err := delivery.NewStore(pool, "profile_delivery", policy)
	if err != nil {
		t.Fatal(err)
	}
	projection := Projection{Delivery: inbox, ConsumerID: "profile", InboxGeneration: "g1", InvalidationSubject: "profile.invalidation", InvalidationEventType: "search_invalidation"}
	accountID, wallet := uuid.New(), fixtureAddress(804)
	if _, err := pool.Exec(ctx, `INSERT INTO users.account_state VALUES($1,$2,'{"displayName":"Alice"}',1,clock_timestamp(),false)`, accountID, wallet); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users.identity_wallet_links VALUES($1,$2,125,'verified','public_optin')`, accountID, wallet); err != nil {
		t.Fatal(err)
	}
	deliver := func(kind string, revision int64) {
		t.Helper()
		id := uuid.New()
		payload, err := json.Marshal(map[string]string{"accountId": accountID.String()})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO users.account_outbox VALUES($1,$2,$3,$4,$5)`, id, accountID, kind, payload, revision); err != nil {
			t.Fatal(err)
		}
		event := delivery.Envelope{EventID: id.String(), EventType: kind, SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: fmt.Sprint(revision), TransactionIndex: "0", LogIndex: "0"}, EmittedAt: time.Now().UTC(), TraceID: "profile", Payload: payload, PayloadHash: delivery.Hash(payload)}
		wire, err := event.Encode(policy, subject)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := publisher.Publish(ctx, subject, id.String(), wire); err != nil {
			t.Fatal(err)
		}
		messages, err := consumer.Fetch(ctx)
		if err != nil || len(messages) != 1 {
			t.Fatalf("profile delivery: %d %v", len(messages), err)
		}
		if err := projection.ConsumeAccountProfile(ctx, subject, messages[0].Wire, messages[0]); err != nil {
			t.Fatal(err)
		}
	}
	deliver("profile.updated", 1)
	var label string
	if err := pool.QueryRow(ctx, `SELECT display_name FROM stats_search.profile_documents_v2 WHERE wallet=$1`, wallet).Scan(&label); err != nil || label != "Alice" {
		t.Fatalf("public profile: %q %v", label, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users.identity_wallet_links SET visibility='private' WHERE wallet=$1`, wallet); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users.account_state SET revision=2,updated_at=clock_timestamp() WHERE account_id=$1`, accountID); err != nil {
		t.Fatal(err)
	}
	deliver("profile.updated", 2)
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM stats_search.profile_documents_v2 WHERE wallet=$1`, wallet).Scan(&count); err != nil || count != 0 {
		t.Fatalf("private profile row: %d %v", count, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users.account_state SET revision=3,deleted=true,updated_at=clock_timestamp() WHERE account_id=$1`, accountID); err != nil {
		t.Fatal(err)
	}
	deliver("account.deletion.requested", 3)
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM stats_search.profile_tombstones_v2 WHERE wallet=$1 AND revision=3`, wallet).Scan(&count); err != nil || count != 1 {
		t.Fatalf("deletion tombstone: %d %v", count, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM profile_delivery.delivery_outbox`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("privacy invalidations: %d %v", count, err)
	}
	service, err := NewService(Store{Pool: pool}, nil, 8)
	if err != nil {
		t.Fatal(err)
	}
	var current int64
	if err := pool.QueryRow(ctx, `SELECT revision FROM stats_search.active_v2 WHERE singleton`).Scan(&current); err != nil {
		t.Fatal(err)
	}
	for _, revision := range []int64{current, current + 1} {
		payload, err := json.Marshal(map[string]any{"version": 2, "revision": fmt.Sprint(revision)})
		if err != nil {
			t.Fatal(err)
		}
		id := uuid.NewString()
		event := delivery.Envelope{EventID: id, EventType: "search_invalidation", SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: fmt.Sprint(revision), TransactionIndex: "0", LogIndex: "0"}, EmittedAt: time.Now().UTC(), TraceID: "profile", Payload: payload, PayloadHash: delivery.Hash(payload)}
		wire, err := event.Encode(policy, subject)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := publisher.Publish(ctx, subject, id, wire); err != nil {
			t.Fatal(err)
		}
		messages, err := consumer.Fetch(ctx)
		if err != nil || len(messages) != 1 {
			t.Fatalf("invalidation delivery: %d %v", len(messages), err)
		}
		err = service.ConsumeInvalidation(ctx, subject, messages[0].Wire, messages[0], policy)
		if revision == current && err != nil {
			t.Fatalf("committed invalidation: %v", err)
		}
		if revision > current && !errors.Is(err, ErrUnavailable) {
			t.Fatalf("ahead invalidation: %v", err)
		}
	}
}
