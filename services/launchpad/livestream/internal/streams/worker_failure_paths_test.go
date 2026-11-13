package streams

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/livepeer"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProvisionWorkerRecordsProviderFailureAtTheFailedStage(t *testing.T) {
	for _, stage := range []string{"lookup", "create", "policy", "vault"} {
		t.Run(stage, func(t *testing.T) {
			f := repair57New(t)
			f.transit = repair57Vault(t)
			ctx := context.Background()
			o, _ := f.create("provider-" + stage)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + r.URL.Path {
				case "GET /stream":
					if stage == "lookup" {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					if stage == "create" {
						_, _ = w.Write([]byte(`[]`))
						return
					}
					_, _ = fmt.Fprintf(w, `[{"id":"provider_1","name":%q,"streamKey":"secret_1","playbackId":"playback_1"}]`, livepeer.CorrelationName(o.ID))
				case "POST /stream":
					if stage != "create" {
						t.Errorf("unexpected provider create during %s", stage)
					}
					w.WriteHeader(http.StatusServiceUnavailable)
				case "PATCH /stream/provider_1":
					if stage == "policy" {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					w.WriteHeader(http.StatusNoContent)
				case "GET /stream/provider_1":
					_, _ = fmt.Fprintf(w, `{"id":"provider_1","streamKey":"secret_1","playbackId":"playback_1","playbackPolicy":{"type":"webhook","webhookId":"playback_hook","webhookContext":{"operationId":%q},"refreshInterval":1}}`, o.ID)
				default:
					t.Errorf("unexpected provider route %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			transit := f.transit
			if stage == "vault" {
				changed := *transit
				changed.Token = "wrong-token"
				transit = &changed
			}
			worker := &ProvisionWorker{Store: f.store, Provider: livepeer.New("fixture-key", server.URL, nil, livepeer.WithWebhookPlayback("playback_hook")), Credentials: &EncryptedCredentials{Store: f.store, Transit: transit}, Restrictions: &RestrictionReader{Pool: f.database}}
			if err := worker.Step(ctx, o.ID); !errors.Is(err, ErrUnavailable) || calls.Load() == 0 {
				t.Fatalf("%s provisioning = %v, calls=%d", stage, err, calls.Load())
			}
			var reason string
			if err := f.database.QueryRow(ctx, "SELECT last_reason FROM livestream.stream_worker_attempts WHERE operation_id=$1", o.ID).Scan(&reason); err != nil {
				t.Fatal(err)
			}
			want := map[string]string{"lookup": "provider_unavailable", "create": "provider_create_ambiguous", "policy": "playback_policy_unconfirmed", "vault": "credential_encryption_unavailable"}[stage]
			if reason != want {
				t.Fatalf("%s release reason = %q, want %q", stage, reason, want)
			}
		})
	}
}

func TestReconcileWorkerReportsDisagreementAndProviderOutage(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	o, _ := f.create("provider-disagreement")
	o, err := f.store.BindProvider(ctx, o.ID, "disagreement_provider")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stream/disagreement_provider" {
			t.Errorf("unexpected provider path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"disagreement_provider","isActive":true,"playbackId":"playback_1"}`))
	}))
	defer server.Close()
	var alerts atomic.Int32
	worker := &ReconcileWorker{Store: f.store, Provider: livepeer.New("fixture-key", server.URL, nil), Restrictions: &RestrictionReader{Pool: f.database}, Disagreement: func(reason string, duration time.Duration) {
		if reason != "provider_state_disagreement" || duration < 120*time.Second {
			t.Errorf("disagreement alert = %s %s", reason, duration)
		}
		alerts.Add(1)
	}}
	if _, err := f.database.Exec(ctx, "INSERT INTO livestream.stream_worker_attempts(operation_id,owner,lease_until,attempt,disagreement_at) VALUES($1,'','-infinity',1,clock_timestamp()-interval '121 seconds')", o.ID); err != nil {
		t.Fatal(err)
	}
	if err := worker.Step(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	live, err := f.store.Get(ctx, o.ID)
	if err != nil || live.State != stateLive || alerts.Load() != 1 {
		t.Fatalf("reconciled disagreement = %+v, alerts=%d, %v", live, alerts.Load(), err)
	}
	if _, err := f.database.Exec(ctx, "UPDATE livestream.stream_worker_attempts SET retry_at='-infinity',lease_until='-infinity',disagreement_at=clock_timestamp() WHERE operation_id=$1", o.ID); err != nil {
		t.Fatal(err)
	}
	if err := worker.Step(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	var cleared bool
	if err := f.database.QueryRow(ctx, "SELECT disagreement_at IS NULL FROM livestream.stream_worker_attempts WHERE operation_id=$1", o.ID).Scan(&cleared); err != nil || !cleared {
		t.Fatalf("resolved provider agreement = %t, %v", cleared, err)
	}
	if _, err := f.database.Exec(ctx, "UPDATE livestream.stream_worker_attempts SET retry_at='-infinity',lease_until='-infinity' WHERE operation_id=$1", o.ID); err != nil {
		t.Fatal(err)
	}
	server.Close()
	if err := worker.Step(ctx, o.ID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("provider outage = %v", err)
	}
	var reason string
	if err := f.database.QueryRow(ctx, "SELECT last_reason FROM livestream.stream_worker_attempts WHERE operation_id=$1", o.ID).Scan(&reason); err != nil || !strings.Contains(reason, "reconcile_unavailable") {
		t.Fatalf("provider outage release = %q, %v", reason, err)
	}
}

func TestReconcileWorkerRetainsRestrictedStreamUntilProviderRevokesIt(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	o, _ := f.create("restricted-provider-retry")
	o, err := f.store.BindProvider(ctx, o.ID, "restricted_provider")
	if err != nil {
		t.Fatal(err)
	}
	var deletes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/stream/restricted_provider" {
			t.Errorf("unexpected provider request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		deletes.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	if _, err := f.database.Exec(ctx, "INSERT INTO chat.safety_bans(wallet,scope) VALUES($1,'global')", f.wallet); err != nil {
		t.Fatal(err)
	}
	worker := &ReconcileWorker{Store: f.store, Provider: livepeer.New("fixture-key", server.URL, nil), Restrictions: &RestrictionReader{Pool: f.database}}
	if err := worker.Step(ctx, o.ID); !errors.Is(err, ErrUnavailable) || deletes.Load() == 0 {
		t.Fatalf("restricted provider revocation = %v, deletes=%d", err, deletes.Load())
	}
	suspended, err := f.store.Get(ctx, o.ID)
	if err != nil || suspended.State != stateSuspended || suspended.RetryAt == nil {
		t.Fatalf("retained stream = %+v, %v", suspended, err)
	}
	var reason string
	if err := f.database.QueryRow(ctx, "SELECT last_reason FROM livestream.stream_operations WHERE id=$1", o.ID).Scan(&reason); err != nil || reason != "reconcile_unavailable" {
		t.Fatalf("revocation retry reason = %q, %v", reason, err)
	}
	other, _ := f.create("unavailable-policy-reader")
	other, err = f.store.BindProvider(ctx, other.ID, "other_provider")
	if err != nil {
		t.Fatal(err)
	}
	closed, err := pgxpool.New(ctx, f.dsn)
	if err != nil {
		t.Fatal(err)
	}
	closed.Close()
	worker.Restrictions = &RestrictionReader{Pool: closed}
	if err := worker.Step(ctx, other.ID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unavailable policy read = %v", err)
	}
}
