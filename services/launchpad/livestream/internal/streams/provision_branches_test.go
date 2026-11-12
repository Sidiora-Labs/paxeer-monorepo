package streams

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/livepeer"
)

func TestProvisionWorkerStopsRevokedAndUnallocatedBroadcasts(t *testing.T) {
	f := repair57New(t)
	f.transit = repair57Vault(t)
	ctx := context.Background()
	first, _ := f.create("provision-policy-revoked")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/stream" {
			t.Errorf("unexpected provider request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	worker := &ProvisionWorker{
		Store: f.store, Provider: livepeer.New("fixture-key", server.URL, nil),
		Credentials:  &EncryptedCredentials{Store: f.store, Transit: f.transit, IngestEndpoint: "rtmps://rtmp.livepeer.studio/live"},
		Restrictions: &RestrictionReader{Pool: f.database},
	}
	if _, err := f.database.Exec(ctx, "INSERT INTO chat.safety_bans(wallet,scope) VALUES($1,'global')", f.wallet); err != nil {
		t.Fatal(err)
	}
	if err := worker.Step(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	failed, err := f.store.Get(ctx, first.ID)
	if err != nil || failed.State != stateFailed || failed.ProviderID != "" {
		t.Fatalf("revoked broadcast = %+v, %v", failed, err)
	}
	if _, err := f.database.Exec(ctx, "DELETE FROM chat.safety_bans WHERE wallet=$1", f.wallet); err != nil {
		t.Fatal(err)
	}
	second, _ := f.create("provision-ended-before-allocation")
	ending, err := f.store.Transition(ctx, second.ID, f.wallet, endAction, second.Revision, ActionDecision{Wallet: f.wallet, OperationID: second.ID, Action: endAction, Revision: second.Revision, Allowed: true, ExpiresAt: time.Now().Add(time.Minute)})
	if err != nil || ending.State != stateEnding {
		t.Fatalf("termination intent = %+v, %v", ending, err)
	}
	if err := worker.Step(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	ended, err := f.store.Get(ctx, second.ID)
	if err != nil || ended.State != stateEnded || ended.ProviderID != "" {
		t.Fatalf("unallocated broadcast = %+v, %v", ended, err)
	}
}

func TestProvisionWorkerRetainsCurrentProviderAndEndsRestrictedAllocation(t *testing.T) {
	f := repair57New(t)
	f.transit = repair57Vault(t)
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/stream" {
			t.Errorf("provider discovery = %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	worker := &ProvisionWorker{Store: f.store, Provider: livepeer.New("fixture-key", server.URL, nil), Credentials: &EncryptedCredentials{Store: f.store, Transit: f.transit}, Restrictions: &RestrictionReader{Pool: f.database}}
	bound, _ := f.create("provision-existing-provider")
	bound, err := f.store.BindProvider(ctx, bound.ID, "existing_provider")
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.Step(ctx, bound.ID); err != nil {
		t.Fatalf("already adopted provider = %v", err)
	}
	var reason string
	if err := f.database.QueryRow(ctx, "SELECT last_reason FROM livestream.stream_worker_attempts WHERE operation_id=$1", bound.ID).Scan(&reason); err != nil || reason != idleOutcome {
		t.Fatalf("idle provider attempt = %q, %v", reason, err)
	}
	if err := worker.Step(ctx, "invalid"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid worker operation = %v", err)
	}
	if err := f.store.releaseWorker(ctx, bound.ID, "invalid", "invalid reason"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid release reason = %v", err)
	}
	if err := f.store.reserveProviderCreate(ctx, bound.ID, "11111111-1111-4111-8111-111111111111", bound.Revision); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unleased provider create = %v", err)
	}
	if err := f.store.finishUnallocated(ctx, bound.ID, "11111111-1111-4111-8111-111111111111"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unleased completion = %v", err)
	}
	if err := worker.compensateRevoked(ctx, bound); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing correlated provider = %v", err)
	}
	ending, _ := f.create("provision-restricted-after-request")
	ending, err = f.store.BeginProvision(ctx, ending.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.database.Exec(ctx, "INSERT INTO chat.safety_bans(wallet,scope) VALUES($1,'global')", f.wallet); err != nil {
		t.Fatal(err)
	}
	if err := worker.Step(ctx, ending.ID); err != nil {
		t.Fatalf("restricted provisioning completion = %v", err)
	}
	current, err := f.store.Get(ctx, ending.ID)
	if err != nil || current.State != stateEnded || current.ProviderID != "" {
		t.Fatalf("restricted unallocated stream = %+v, %v", current, err)
	}
}

func TestSuspensionRevokesProviderAfterDurableDecision(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	o, _ := f.create("suspend-provider-revocation")
	o, err := f.store.BindProvider(ctx, o.ID, "suspended_provider")
	if err != nil {
		t.Fatal(err)
	}
	var deletes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/stream/suspended_provider" {
			t.Errorf("unexpected revocation request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		deletes.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	decision := SuspensionDecision{OperationID: o.ID, Revision: o.Revision, Actor: "policy_reconciler", Reason: "active_restriction", Allowed: true, ExpiresAt: time.Now().Add(time.Minute)}
	suspended, err := f.store.SuspendAndRevoke(ctx, o.ID, decision, livepeer.New("fixture-key", server.URL, nil))
	if err != nil || suspended.State != stateSuspended || deletes.Load() != 1 {
		t.Fatalf("provider revocation = %+v, deletes=%d, %v", suspended, deletes.Load(), err)
	}
	other, _ := f.create("suspend-unavailable-provider")
	decision.OperationID, decision.Revision = other.ID, other.Revision
	suspended, err = f.store.SuspendAndRevoke(ctx, other.ID, decision, nil)
	if !errors.Is(err, ErrUnavailable) || suspended.State != stateSuspended {
		t.Fatalf("unallocated revocation = %+v, %v", suspended, err)
	}
}

func TestProvisionWorkerAdoptsCorrelatedProviderAfterRevocation(t *testing.T) {
	f := repair57New(t)
	f.transit = repair57Vault(t)
	ctx := context.Background()
	o, _ := f.create("revoked-provider-recovery")
	o, err := f.store.Suspend(ctx, o.ID, SuspensionDecision{OperationID: o.ID, Revision: o.Revision, Actor: "policy_reconciler", Reason: "active_restriction", Allowed: true, ExpiresAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/stream" {
			t.Errorf("unexpected provider request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"revoked_provider","playbackId":"playback_1","streamKey":"revoked_key","name":"KindleLaunch:` + o.ID + `"}]`))
	}))
	defer server.Close()
	worker := &ProvisionWorker{Store: f.store, Provider: livepeer.New("fixture-key", server.URL, nil), Credentials: &EncryptedCredentials{Store: f.store, Transit: f.transit, IngestEndpoint: "rtmps://rtmp.livepeer.studio/live"}, Restrictions: &RestrictionReader{Pool: f.database}}
	if err := worker.compensateRevoked(ctx, o); err != nil {
		t.Fatal(err)
	}
	bound, err := f.store.Get(ctx, o.ID)
	if err != nil || bound.State != stateSuspended || bound.ProviderID != "revoked_provider" {
		t.Fatalf("bound suspended provider = %+v, %v", bound, err)
	}
	var stored string
	if err := f.database.QueryRow(ctx, "SELECT ciphertext FROM livestream.stream_credentials WHERE operation_id=$1", o.ID).Scan(&stored); err != nil || stored == "revoked_key" || stored == "" {
		t.Fatalf("bound encrypted key = %q, %v", stored, err)
	}
}

func TestProvisionWorkerSuspendsProviderWhenPolicyChangesDuringAllocation(t *testing.T) {
	f := repair57New(t)
	f.transit = repair57Vault(t)
	ctx := context.Background()
	o, _ := f.create("provision-policy-change")
	var created atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /stream":
			if created.Load() {
				_, _ = w.Write([]byte(`[{"id":"changing_provider","playbackId":"playback_1","streamKey":"changing_key","name":"KindleLaunch:` + o.ID + `"}]`))
			} else {
				_, _ = w.Write([]byte(`[]`))
			}
		case "POST /stream":
			created.Store(true)
			_, _ = w.Write([]byte(`{"id":"changing_provider","playbackId":"playback_1","streamKey":"changing_key","name":"KindleLaunch:` + o.ID + `"}`))
		case "PATCH /stream/changing_provider":
			if _, err := f.database.Exec(ctx, "INSERT INTO chat.safety_bans(wallet,scope) VALUES($1,'global')", f.wallet); err != nil {
				t.Error(err)
			}
			w.WriteHeader(http.StatusNoContent)
		case "GET /stream/changing_provider":
			_, _ = w.Write([]byte(`{"id":"changing_provider","playbackId":"playback_1","streamKey":"changing_key","playbackPolicy":{"type":"webhook","webhookId":"playback_hook","webhookContext":{"operationId":"` + o.ID + `"},"refreshInterval":1}}`))
		default:
			t.Errorf("unexpected provider request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	provider := livepeer.New("fixture-key", server.URL, nil, livepeer.WithWebhookPlayback("playback_hook"))
	worker := &ProvisionWorker{Store: f.store, Provider: provider, Credentials: &EncryptedCredentials{Store: f.store, Transit: f.transit, IngestEndpoint: "rtmps://rtmp.livepeer.studio/live"}, Restrictions: &RestrictionReader{Pool: f.database}}
	if err := worker.Step(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	bound, err := f.store.Get(ctx, o.ID)
	if err != nil || bound.State != stateSuspended || bound.ProviderID != "changing_provider" || !created.Load() {
		t.Fatalf("post-allocation policy state = %+v, created=%t, %v", bound, created.Load(), err)
	}
	var cipher string
	if err := f.database.QueryRow(ctx, "SELECT ciphertext FROM livestream.stream_credentials WHERE operation_id=$1", o.ID).Scan(&cipher); err != nil || cipher == "" || cipher == "changing_key" {
		t.Fatalf("suspended provider key = %q, %v", cipher, err)
	}
}
