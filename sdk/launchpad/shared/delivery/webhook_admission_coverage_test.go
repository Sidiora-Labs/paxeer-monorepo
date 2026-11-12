package delivery

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
	"github.com/jackc/pgx/v5"
)

func TestWebhookStagingBindsOriginalBytesAndPartitionOrder(t *testing.T) {
	ctx, pool, _, _ := orderedPostgres(t)
	schema := os.Getenv("DELIVERY_TEST_SCHEMA") + "_ordered"
	ddl, err := WebhookDDL(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(pool, schema, fixturePolicy())
	if err != nil {
		t.Fatal(err)
	}
	target := WebhookTarget{ID: "receiver", URL: "https://receiver.example/events", SecretReference: "WEBHOOK_SIGNING_KEY", Subjects: map[string]bool{fixtureSubject: true}}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}}
	bridge, err := NewWebhookBridge(store, []WebhookTarget{target}, client, secrets.EnvProvider{}, verifyWebhookFixtureAdmin, "owner", time.Minute, time.Second, time.Second, 3)
	if err != nil {
		t.Fatal(err)
	}
	envelope := fixtureEnvelope("stage_original")
	if err := store.Produce(ctx, []Pending{{fixtureSubject, envelope}}, func(ctx context.Context, tx pgx.Tx) error { _, err := tx.Exec(ctx, "SELECT 1"); return err }); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"first", "duplicate", "position conflict", "gap", "changed bytes", "missing original", "invalid envelope", "invalid bounds"} {
		t.Run(mode, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer rollbackBounded(tx)
			candidate := envelope
			sequence := int64(1)
			identity := "position_1"
			previous := ""
			var want error
			switch mode {
			case "position conflict":
				identity = "other_position"
				want = ErrIntegrity
			case "gap":
				sequence = 3
				previous = "position_2"
				want = ErrGap
			case "changed bytes":
				candidate.TraceID = "other_trace"
				want = ErrIntegrity
			case "missing original":
				candidate.EventID = "missing_event"
				want = pgx.ErrNoRows
			case "invalid envelope":
				candidate.SchemaVersion = 2
				want = ErrSchema
			case "invalid bounds":
				sequence = 0
				want = ErrIntegrity
			}
			err = bridge.StageTx(ctx, tx, "g1", "partition", sequence, identity, previous, fixtureSubject, candidate)
			if want != nil {
				if !errors.Is(err, want) {
					t.Fatalf("staging %s: %v", mode, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
	var staged, tail int
	if err := pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM "+store.table("delivery_webhooks")+"),(SELECT enqueued_sequence FROM "+store.table("delivery_webhook_cursors")+")").Scan(&staged, &tail); err != nil || staged != 1 || tail != 1 {
		t.Fatalf("immutable staging: %d %d %v", staged, tail, err)
	}
	for name, edit := range map[string]func(*WebhookTarget){"http": func(c *WebhookTarget) { c.URL = "http://receiver.example" }, "query": func(c *WebhookTarget) { c.URL += "?secret=unbound" }, "subject": func(c *WebhookTarget) { c.Subjects = map[string]bool{"unknown": true} }} {
		t.Run(name, func(t *testing.T) {
			candidate := target
			edit(&candidate)
			if _, err := NewWebhookBridge(store, []WebhookTarget{candidate}, client, secrets.EnvProvider{}, verifyWebhookFixtureAdmin, "owner", time.Minute, time.Second, time.Second, 3); err == nil {
				t.Fatal("unreviewed webhook target admitted")
			}
		})
	}
	if _, err := NewWebhookBridge(store, []WebhookTarget{target, target}, client, secrets.EnvProvider{}, verifyWebhookFixtureAdmin, "owner", time.Minute, time.Second, time.Second, 3); !errors.Is(err, ErrIntegrity) {
		t.Fatal(err)
	}
	if _, err := NewWebhookBridge(store, []WebhookTarget{target}, &http.Client{}, secrets.EnvProvider{}, verifyWebhookFixtureAdmin, "owner", time.Minute, time.Second, time.Second, 3); err == nil {
		t.Fatal("unbound TLS transport admitted")
	}
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	admin := context.WithValue(ctx, webhookAdminContextKey{}, webhookAdminCredential{stamp, signWebhookBody(strings.Repeat("admin-secret", 4), stamp, "webhook.inspect")})
	cancelled, cancel := context.WithCancel(admin)
	cancel()
	if _, err := bridge.Inspect(cancelled, "receiver", 1); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := bridge.Stats(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := bridge.RunOnce(cancelled); err == nil {
		t.Fatal("cancelled webhook work admitted")
	}
	if _, err := bridge.Inspect(admin, "unknown", 1); !errors.Is(err, ErrSchema) {
		t.Fatal(err)
	}
	if err := bridge.Retry(ctx, "receiver", "g1", "partition", 1); err == nil {
		t.Fatal("unauthorized webhook retry admitted")
	}
	nextEnvelope := fixtureEnvelope("stage_next")
	if err := store.Produce(ctx, []Pending{{fixtureSubject, nextEnvelope}}, func(ctx context.Context, tx pgx.Tx) error { _, err := tx.Exec(ctx, "SELECT 1"); return err }); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"cursor unavailable", "cursor fields", "delivery unavailable", "delivery insert", "cursor update"} {
		t.Run("catalog/"+mode, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer rollbackBounded(tx)
			sequence, identity, predecessor, candidate := int64(1), "position_1", "", envelope
			var sql string
			switch mode {
			case "cursor unavailable":
				sql = "ALTER TABLE " + store.table("delivery_webhook_cursors") + " RENAME COLUMN target_id TO unbound_target"
			case "cursor fields":
				sql = "ALTER TABLE " + store.table("delivery_webhook_cursors") + " ALTER COLUMN enqueued_event DROP NOT NULL; UPDATE " + store.table("delivery_webhook_cursors") + " SET enqueued_event=NULL"
			case "delivery unavailable":
				sql = "ALTER TABLE " + store.table("delivery_webhooks") + " RENAME COLUMN idempotency_key TO unbound_key"
			case "delivery insert":
				sql = "ALTER TABLE " + store.table("delivery_webhooks") + " ADD CHECK(sequence<=1)"
			case "cursor update":
				sql = "ALTER TABLE " + store.table("delivery_webhook_cursors") + " ADD CHECK(enqueued_sequence<=1)"
			}
			if mode == "delivery insert" || mode == "cursor update" {
				sequence, identity, predecessor, candidate = 2, "position_2", "position_1", nextEnvelope
			}
			if _, err := tx.Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
			if err := bridge.StageTx(ctx, tx, "g1", "partition", sequence, identity, predecessor, fixtureSubject, candidate); err == nil {
				t.Fatal("inconsistent webhook catalog admitted")
			}
		})
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("delivery_webhooks")+" RENAME COLUMN attempts TO unavailable_attempts"); err != nil {
		t.Fatal(err)
	}
	if err := bridge.RunOnce(ctx); err == nil {
		t.Fatal("unavailable webhook claim admitted")
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("delivery_webhooks")+" RENAME COLUMN unavailable_attempts TO attempts"); err != nil {
		t.Fatal(err)
	}
	claim, err := bridge.claim(ctx, "receiver")
	if err != nil || claim == nil {
		t.Fatalf("committed webhook lease: %+v %v", claim, err)
	}
	foreign := *claim
	foreign.fence++
	if err := bridge.finish(ctx, &foreign, 200, ""); !errors.Is(err, ErrLease) {
		t.Fatalf("unbound webhook completion: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("delivery_webhook_cursors")+" SET committed_sequence=1,committed_event='position_1'"); err != nil {
		t.Fatal(err)
	}
	if err := bridge.finish(ctx, claim, 200, ""); !errors.Is(err, ErrGap) {
		t.Fatalf("out-of-order webhook completion: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+store.table("delivery_webhook_cursors")+" SET committed_sequence=0,committed_event=''"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("delivery_webhooks")+" RENAME COLUMN response_status TO unavailable_status"); err != nil {
		t.Fatal(err)
	}
	if err := bridge.finish(ctx, claim, 200, ""); err == nil {
		t.Fatal("unavailable webhook completion catalog admitted")
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("delivery_webhooks")+" RENAME COLUMN unavailable_status TO response_status"); err != nil {
		t.Fatal(err)
	}
	if err := bridge.finish(ctx, claim, 403, Authorization); err != nil {
		t.Fatal(err)
	}
	retryAdmin := context.WithValue(ctx, webhookAdminContextKey{}, webhookAdminCredential{stamp, signWebhookBody(strings.Repeat("admin-secret", 4), stamp, "webhook.retry")})
	if err := bridge.Retry(retryAdmin, "unknown", "g1", "partition", 1); !errors.Is(err, ErrSchema) {
		t.Fatalf("unknown retry target: %v", err)
	}
	if err := bridge.Retry(retryAdmin, "receiver", "g1", "partition", 2); !errors.Is(err, ErrLease) {
		t.Fatalf("unrecorded retry: %v", err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("delivery_webhooks")+" RENAME COLUMN allowed_attempts TO unavailable_budget"); err != nil {
		t.Fatal(err)
	}
	if err := bridge.Retry(retryAdmin, "receiver", "g1", "partition", 1); err == nil {
		t.Fatal("unavailable retry catalog admitted")
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+store.table("delivery_webhooks")+" RENAME COLUMN unavailable_budget TO allowed_attempts"); err != nil {
		t.Fatal(err)
	}
	if err := bridge.Retry(retryAdmin, "receiver", "g1", "partition", 1); err != nil {
		t.Fatal(err)
	}
	var retryCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+store.table("delivery_webhook_retry_audit")).Scan(&retryCount); err != nil || retryCount != 1 {
		t.Fatalf("authorized retry audit: %d %v", retryCount, err)
	}

}
