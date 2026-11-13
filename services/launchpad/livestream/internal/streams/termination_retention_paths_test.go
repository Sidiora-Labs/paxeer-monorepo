package streams

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/livepeer"
)

func TestTerminationReconcilesProviderAcknowledgmentAndAbsence(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/stream/ending_provider/terminate" {
			t.Errorf("termination request = %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	provider := livepeer.New("fixture-key", server.URL, nil)
	o, _ := f.create("termination-provider-confirmation")
	if err := f.store.ReconcileTermination(ctx, o.ID, provider); !errors.Is(err, ErrConflict) {
		t.Fatalf("termination without intent = %v", err)
	}
	o, err := f.store.BindProvider(ctx, o.ID, "ending_provider")
	if err != nil {
		t.Fatal(err)
	}
	o, err = f.store.Transition(ctx, o.ID, f.wallet, endAction, o.Revision, ActionDecision{Wallet: f.wallet, OperationID: o.ID, Action: endAction, Revision: o.Revision, Allowed: true, ExpiresAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.ReconcileTermination(ctx, o.ID, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("termination without provider = %v", err)
	}
	if err := f.store.ReconcileTermination(ctx, o.ID, provider); err != nil {
		t.Fatalf("provider absence confirms termination = %v", err)
	}
	current, err := f.store.Get(ctx, o.ID)
	if err != nil || current.State != stateEnding || current.RetryAt == nil {
		t.Fatalf("termination remains pending until observation = %+v, %v", current, err)
	}
}

func TestTerminalRetentionExpiresIdempotencyWithoutChangingState(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	o, _ := f.create("terminal-retention")
	o, err := f.store.Transition(ctx, o.ID, f.wallet, endAction, o.Revision, ActionDecision{Wallet: f.wallet, OperationID: o.ID, Action: endAction, Revision: o.Revision, Allowed: true, ExpiresAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.database.Exec(ctx, "UPDATE livestream.stream_operations SET state='ENDED',terminal_at=clock_timestamp()-interval '25 hours' WHERE id=$1", o.ID); err != nil {
		t.Fatal(err)
	}
	count, err := f.store.ExpireTerminalKeys(ctx)
	if err != nil || count != 1 {
		t.Fatalf("terminal retention = %d, %v", count, err)
	}
	var state, key string
	if err := f.database.QueryRow(ctx, "SELECT state,idem_key FROM livestream.stream_operations WHERE id=$1", o.ID).Scan(&state, &key); err != nil || state != stateEnded || key != "expired_"+o.ID {
		t.Fatalf("retained stream = %s %s, %v", state, key, err)
	}
	count, err = f.store.ExpireTerminalKeys(ctx)
	if err != nil || count != 0 {
		t.Fatalf("repeat retention = %d, %v", count, err)
	}
}

func TestReconciliationConfirmsTerminatedProviderStreams(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "DELETE /stream/terminated_provider/terminate":
			w.WriteHeader(http.StatusNoContent)
		case "GET /stream/terminated_provider":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"terminated_provider","playbackId":"playback_1","isActive":false}`))
		default:
			t.Errorf("provider termination route = %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	o, _ := f.create("reconciled-termination")
	o, err := f.store.BindProvider(ctx, o.ID, "terminated_provider")
	if err != nil {
		t.Fatal(err)
	}
	o, err = f.store.Transition(ctx, o.ID, f.wallet, endAction, o.Revision, ActionDecision{Wallet: f.wallet, OperationID: o.ID, Action: endAction, Revision: o.Revision, Allowed: true, ExpiresAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	worker := &ReconcileWorker{Store: f.store, Provider: livepeer.New("fixture-key", server.URL, nil), Restrictions: &RestrictionReader{Pool: f.database}}
	if err := worker.Step(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	current, err := f.store.Get(ctx, o.ID)
	if err != nil || current.State != stateEnded || current.ProviderID != "terminated_provider" {
		t.Fatalf("provider termination observation = %+v, %v", current, err)
	}
}

func TestSuspensionRetainsRetryAfterProviderRefusal(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	o, _ := f.create("suspension-retry")
	o, err := f.store.BindProvider(ctx, o.ID, "retry_provider")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/stream/retry_provider" {
			t.Errorf("revocation route = %s %s", r.Method, r.URL)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	decision := SuspensionDecision{OperationID: o.ID, Revision: o.Revision, Actor: "policy_reconciler", Reason: "active_restriction", Allowed: true, ExpiresAt: time.Now().Add(time.Minute)}
	suspended, err := f.store.SuspendAndRevoke(ctx, o.ID, decision, livepeer.New("fixture-key", server.URL, nil))
	if !errors.Is(err, ErrUnavailable) || suspended.State != stateSuspended {
		t.Fatalf("provider revocation retry = %+v, %v", suspended, err)
	}
	current, err := f.store.Get(ctx, o.ID)
	if err != nil || current.RetryAt == nil || current.State != stateSuspended {
		t.Fatalf("retained suspension = %+v, %v", current, err)
	}
	if err := f.store.TerminationFailed(ctx, "11111111-1111-4111-8111-111111111111"); !errors.Is(err, ErrConflict) {
		t.Fatalf("unknown termination retry = %v", err)
	}
}
