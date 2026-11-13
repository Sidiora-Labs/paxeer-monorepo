package control

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestActionApprovalPreservesProposalAndRevision(t *testing.T) {
	env := repairM1020EnvironmentFor(t, nil)
	ctx := context.Background()
	credential := env.credentials["infrastructure"]
	approval := env.credentials["operator2"]
	hash := strings.Repeat("a", 64)
	expires := time.Now().Add(time.Hour)
	for _, sample := range []struct {
		key, kind, digest string
		expires           time.Time
	}{
		{"bad key", controlReplay, hash, expires},
		{"invalid-kind", "unknown", hash, expires},
		{"invalid-digest", controlReplay, "short", expires},
		{"expired-action", controlReplay, hash, time.Now().Add(-time.Minute)},
		{"distant-action", controlReplay, hash, time.Now().Add(25 * time.Hour)},
	} {
		if _, err := env.store.Propose(ctx, credential, sample.key, sample.kind, "service", "review operation", sample.digest, sample.expires); !errors.Is(err, Invalid) {
			t.Fatalf("invalid proposal %s: %v", sample.key, err)
		}
	}
	action, err := env.store.Propose(ctx, credential, "approval-action", controlCapChange, "service", "review capability", hash, expires)
	if err != nil || action.State != controlProposed || action.Revision != 1 {
		t.Fatalf("proposal: %+v, %v", action, err)
	}
	repeated, err := env.store.Propose(ctx, credential, "approval-action", controlCapChange, "service", "review capability", hash, expires)
	if err != nil || repeated.ID != action.ID || repeated.Revision != action.Revision {
		t.Fatalf("stable proposal: %+v, %v", repeated, err)
	}
	if _, err := env.store.Propose(ctx, credential, "approval-action", controlCapChange, "service", "changed reason", hash, expires); !errors.Is(err, Conflict) {
		t.Fatalf("changed proposal: %v", err)
	}
	if _, err := env.store.Transition(ctx, approval, "approval-action", "other", "wrong scope", controlApproved, 1); !errors.Is(err, Denied) {
		t.Fatalf("other scope: %v", err)
	}
	if _, err := env.store.Transition(ctx, credential, "approval-action", "service", "self approval", controlApproved, 1); !errors.Is(err, Denied) {
		t.Fatalf("self approval: %v", err)
	}
	if _, err := env.store.Transition(ctx, approval, "approval-action", "service", "stale approval", controlApproved, 2); !errors.Is(err, Conflict) {
		t.Fatalf("stale approval: %v", err)
	}
	action, err = env.store.Transition(ctx, approval, "approval-action", "service", "independent approval", controlApproved, 1)
	if err != nil || action.State != controlApproved || action.ApprovedBy != env.actors["operator2"] || action.Revision != 2 {
		t.Fatalf("approval: %+v, %v", action, err)
	}
	if _, err := env.store.Transition(ctx, credential, "approval-action", "service", "execution unavailable", controlExecuting, 2); !errors.Is(err, Denied) {
		t.Fatalf("capability action execution: %v", err)
	}
	if _, err := env.store.Transition(ctx, credential, "approval-action", "service", "invalid transition", "pending", 2); !errors.Is(err, Invalid) {
		t.Fatalf("unknown transition: %v", err)
	}
	if _, err := env.store.Transition(ctx, credential, "approval-action", "service", "executor result required", controlSucceeded, 2); !errors.Is(err, Unavailable) {
		t.Fatalf("direct completion: %v", err)
	}
	action, err = env.store.Transition(ctx, credential, "approval-action", "service", "cancel approved change", controlCancelled, 2)
	if err != nil || action.State != controlCancelled || action.Revision != 3 {
		t.Fatalf("cancelled action: %+v, %v", action, err)
	}
	if _, err := env.store.Transition(ctx, credential, "approval-action", "service", "cancel again", controlCancelled, 3); !errors.Is(err, Denied) {
		t.Fatalf("repeated cancellation: %v", err)
	}
}

func TestIncidentContainmentAndAccessUseBoundApproval(t *testing.T) {
	env := repairM1020EnvironmentFor(t, nil)
	ctx := context.Background()
	credential := env.credentials["infrastructure"]
	identity := env.actors["operator2"]
	if err := env.store.DetectIncident(ctx, credential, "service", "bad key", "contain incident", identity, "recorded evidence"); !errors.Is(err, Invalid) {
		t.Fatalf("invalid incident key: %v", err)
	}
	if err := env.store.DetectIncident(ctx, credential, "service", "incident-bound", "contain incident", identity, "recorded evidence"); err != nil {
		t.Fatal(err)
	}
	if err := env.store.DetectIncident(ctx, credential, "service", "incident-bound", "repeat containment", identity, "recorded evidence"); !errors.Is(err, Conflict) {
		t.Fatalf("repeated incident: %v", err)
	}
	incident := repairM1020Read[Incident](t, env.store, controlIncident, "incident-bound")
	if incident.State != controlContained || incident.ServiceIdentity != identity || incident.Revision != 1 {
		t.Fatalf("contained incident: %+v", incident)
	}
	if err := env.store.RecoverIncident(ctx, credential, "service", incident.ID, "recover incident", "approval", "rotation", "integrity", 1, nil); !errors.Is(err, Unavailable) {
		t.Fatalf("unbound recovery: %v", err)
	}
	if err := env.store.Access(ctx, env.credentials["support"], "platform", "read public support data", "support_public", "", false); err != nil {
		t.Fatalf("support read: %v", err)
	}
	if err := env.store.Access(ctx, env.credentials["support"], "platform", "bulk support read", "support_public", "", true); !errors.Is(err, Denied) {
		t.Fatalf("bulk read without approval: %v", err)
	}
	if err := env.store.Access(ctx, env.credentials["support"], "platform", "unknown dataset", "custody_secret", "", false); !errors.Is(err, Denied) {
		t.Fatalf("unlisted dataset: %v", err)
	}
	observed := []error{}
	worker := IncidentWorker{Store: env.store, Credential: credential, Observe: func(_ string, err error) { observed = append(observed, err) }}
	if err := worker.Drain(ctx); err != nil || len(observed) != 2 || observed[0] != nil || !errors.Is(observed[1], Unavailable) {
		t.Fatalf("credential containment: %v, %+v", err, observed)
	}
	outbox := repairM1020Read[map[string]string](t, env.store, "incident_outbox", incident.ID)
	if outbox[controlState] != "credentials_revoked" || outbox["identity"] != identity {
		t.Fatalf("credential containment outbox: %+v", outbox)
	}
	if err := worker.Drain(ctx); err != nil {
		t.Fatalf("repeat containment: %v", err)
	}
}

func TestReplayProviderBindsCompletionAndResult(t *testing.T) {
	ctx := context.Background()
	jobID := id()
	payload := ReplayPayload{ChainID: "125", FromHeight: "1", ToHeight: "2", Projection: "candles", DryRun: true}
	job := ReplayJob{ActionID: id(), ActionKey: "replay-request", Kind: controlReplay, Scope: "service", State: "queued", PayloadHash: digest(payload), PayloadReference: "replay-request", CompletionIdentity: "control-replay:" + jobID}
	request := ReplayExecution{JobID: jobID, Job: job, Payload: payload}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.Header.Get("Idempotency-Key") != job.CompletionIdentity || !strings.HasSuffix(r.URL.Path, job.CompletionIdentity) {
			t.Errorf("unbound replay request: %s %s", r.Method, r.URL.Path)
		}
		var received ReplayExecution
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil || received.JobID != request.JobID || received.Job.PayloadHash != job.PayloadHash {
			t.Errorf("replay payload: %+v, %v", received, err)
		}
		if err := json.NewEncoder(w).Encode(ReplayResult{ActionID: job.ActionID, JobID: jobID, PayloadHash: job.PayloadHash, PayloadReference: job.PayloadReference, CompletionIdentity: job.CompletionIdentity, State: controlSucceeded, Reference: "owner-checkpoint:3"}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	provider, err := NewReplayProvider(server.URL, strings.Repeat("r", 32), repairM1020Transport(server))
	if err != nil {
		t.Fatal(err)
	}
	result, err := provider.Execute(ctx, request)
	if err != nil || result.State != controlSucceeded || result.Reference != "owner-checkpoint:3" {
		t.Fatalf("owner completion: %+v, %v", result, err)
	}
	invalid := request
	invalid.Job.CompletionIdentity = "other-job"
	if _, err := provider.Execute(ctx, invalid); !errors.Is(err, Invalid) {
		t.Fatalf("unbound completion identity: %v", err)
	}
	invalid = request
	invalid.Payload.ToHeight = "4"
	if _, err := provider.Execute(ctx, invalid); !errors.Is(err, Invalid) {
		t.Fatalf("changed replay payload: %v", err)
	}
}
