package delivery

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
)

func TestWebhookPersistsReceiverOutcomesWithWorkBudget(t *testing.T) {
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
	secret := strings.Repeat("receiver-secret", 4)
	t.Setenv("WEBHOOK_OUTCOME_SECRET", secret)
	for _, tc := range []struct {
		name   string
		status int
		class  FailureClass
		state  string
	}{
		{"delivered", 204, "", "delivered"},
		{"forbidden", 403, Authorization, "quarantined"},
		{"invalid_request", 422, SchemaViolation, "quarantined"},
		{"redirect", 307, Transient, "pending"},
		{"missing_secret", 0, Authorization, "quarantined"},
		{"attempt_limit", 0, Transient, "quarantined"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var received atomic.Int32
			receiver := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received.Add(1)
				body, err := io.ReadAll(r.Body)
				if err != nil || verifyWebhookFixtureSignature(secret, r.Header.Get("X-Sidiora-Timestamp"), string(body), r.Header.Get("X-Sidiora-Signature"), time.Now(), time.Minute) != nil || r.Header.Get("Idempotency-Key") == "" {
					t.Error("receiver requires an authenticated delivery")
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if tc.status == http.StatusTemporaryRedirect {
					w.Header().Set("Location", "/redirected")
				}
				w.WriteHeader(tc.status)
			}))
			defer receiver.Close()
			roots := x509.NewCertPool()
			roots.AddCert(receiver.Certificate())
			transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
			defer transport.CloseIdleConnections()
			target := WebhookTarget{ID: tc.name, URL: receiver.URL, SecretReference: "WEBHOOK_OUTCOME_SECRET", Subjects: map[string]bool{fixtureSubject: true}}
			bridge, err := NewWebhookBridge(store, []WebhookTarget{target}, &http.Client{Transport: transport}, secrets.EnvProvider{}, verifyWebhookFixtureAdmin, "worker", 10*time.Second, time.Second, time.Second, 2)
			if err != nil {
				t.Fatal(err)
			}
			envelope := fixtureEnvelope("outcome_" + tc.name)
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer rollbackBounded(tx)
			if err := store.Enqueue(ctx, tx, fixtureSubject, envelope); err != nil {
				t.Fatal(err)
			}
			if err := bridge.StageTx(ctx, tx, "g1", "partition", 1, "position", "", fixtureSubject, envelope); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if tc.name == "missing_secret" {
				t.Setenv("WEBHOOK_OUTCOME_SECRET", "")
			}
			if tc.name == "attempt_limit" {
				if _, err := pool.Exec(ctx, "UPDATE "+store.table("delivery_webhooks")+" SET attempts=2 WHERE target_id=$1", target.ID); err != nil {
					t.Fatal(err)
				}
			}
			budget, err := NewWorkBudget(1, 1000, 10)
			if err != nil {
				t.Fatal(err)
			}
			err = bridge.RunOnceWithBudget(ctx, budget)
			wantError := tc.class != "" && tc.name != "attempt_limit"
			if (err != nil) != wantError || len(budget.slots) != 0 {
				t.Fatalf("delivery outcome and capacity: %v, slots=%d", err, len(budget.slots))
			}
			var state, class string
			var status, committed int
			if err := pool.QueryRow(ctx, "SELECT w.state,COALESCE(w.failure_class,''),w.response_status,c.committed_sequence FROM "+store.table("delivery_webhooks")+" w JOIN "+store.table("delivery_webhook_cursors")+" c USING(target_id,generation,partition) WHERE w.target_id=$1", target.ID).Scan(&state, &class, &status, &committed); err != nil {
				t.Fatal(err)
			}
			wantCommitted := 0
			if tc.state == "delivered" {
				wantCommitted = 1
			}
			if state != tc.state || class != string(tc.class) || status != tc.status || committed != wantCommitted {
				t.Fatalf("persisted outcome: %s %s %d %d", state, class, status, committed)
			}
			wantRequests := int32(1)
			if tc.status == 0 {
				wantRequests = 0
			}
			if received.Load() != wantRequests {
				t.Fatalf("receiver requests=%d, want %d", received.Load(), wantRequests)
			}
		})
	}
}

func TestWorkBudgetReleasesSlotWhenRateDeadlineCannotBeMet(t *testing.T) {
	budget, err := NewWorkBudget(1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	release, err := budget.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if release, err := budget.acquire(ctx); err == nil || release != nil || len(budget.slots) != 0 {
		t.Fatalf("rate admission: release=%v err=%v slots=%d", release != nil, err, len(budget.slots))
	}
	var absent *WorkBudget
	if _, err := absent.acquire(ctx); err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("missing budget: %v", err)
	}
}
