package control

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReplayWorkflowRequiresBoundDurablePayload(t *testing.T) {
	env := repairM1020EnvironmentFor(t, nil)
	ctx := context.Background()
	credential := env.credentials["infrastructure"]
	proposal := ReplayProposal{Key: "durable-bound-replay", Scope: "service", Reason: "restore projection", Expires: time.Now().Add(time.Hour), Payload: ReplayPayload{ChainID: "125", FromHeight: "1", ToHeight: "2", Projection: "candles", DryRun: true}}
	revision := uint64(1)
	request := TransitionRequest{Key: proposal.Key, Scope: "service", Reason: "execute reviewed replay", Next: controlExecuting, Expected: &revision}
	if _, err := env.store.TransitionWorkflow(ctx, credential, request); !errors.Is(err, Missing) {
		t.Fatalf("missing durable action: %v", err)
	}
	action, err := env.store.ProposeReplay(ctx, credential, proposal)
	if err != nil || action.PayloadReference != proposal.Key {
		t.Fatalf("durable proposal: %+v, %v", action, err)
	}
	env.store.replay = repairM1020ReplayProvider(t, nil)
	base := make(map[string]json.RawMessage, len(env.store.local.objects))
	for key, body := range env.store.local.objects {
		base[key] = body
	}
	set := func(edit func(map[string]json.RawMessage)) {
		t.Helper()
		objects := make(map[string]json.RawMessage, len(base))
		for key, body := range base {
			objects[key] = body
		}
		edit(objects)
		if err := env.store.local.save(objects, nil); err != nil {
			t.Fatal(err)
		}
	}
	set(func(objects map[string]json.RawMessage) { delete(objects, "action_payload/"+proposal.Key) })
	if _, err := env.store.TransitionWorkflow(ctx, credential, request); !errors.Is(err, Unavailable) {
		t.Fatalf("missing replay payload: %v", err)
	}
	set(func(objects map[string]json.RawMessage) {
		changed := proposal.Payload
		changed.ToHeight = "3"
		wire, err := json.Marshal(changed)
		if err != nil {
			t.Fatal(err)
		}
		objects["action_payload/"+proposal.Key] = wire
	})
	if _, err := env.store.TransitionWorkflow(ctx, credential, request); !errors.Is(err, Conflict) {
		t.Fatalf("changed replay payload: %v", err)
	}
	set(func(objects map[string]json.RawMessage) {
		action.Kind = controlCapChange
		wire, err := json.Marshal(action)
		if err != nil {
			t.Fatal(err)
		}
		objects["action/"+proposal.Key] = wire
	})
	if _, err := env.store.TransitionWorkflow(ctx, credential, request); !errors.Is(err, Unavailable) {
		t.Fatalf("nonreplay action dispatched as replay: %v", err)
	}
}

func TestWorkerAndResponseBoundariesRejectMissingInputs(t *testing.T) {
	ctx := context.Background()
	if err := (ReconciliationWorker{}).Run(ctx); !errors.Is(err, Invalid) {
		t.Fatalf("unconfigured reconciliation: %v", err)
	}
	env := repairM1020EnvironmentFor(t, nil)
	worker := ReconciliationWorker{Store: env.store, Credential: env.credentials["finance"], Assets: []Asset{{}}, Interval: time.Second, Observe: func(string, error) {}}
	if err := worker.Run(ctx); !errors.Is(err, Invalid) {
		t.Fatalf("invalid reconciliation asset: %v", err)
	}
	if err := (IncidentWorker{}).Drain(ctx); !errors.Is(err, Invalid) {
		t.Fatalf("unconfigured incident drain: %v", err)
	}
	if err := (ReplayWorker{}).Process(ctx, "action", "service"); !errors.Is(err, Invalid) {
		t.Fatalf("unconfigured replay worker: %v", err)
	}
	if err := (ReplayWorker{}).Drain(ctx); !errors.Is(err, Invalid) {
		t.Fatalf("unconfigured replay drain: %v", err)
	}
	response := httptest.NewRecorder()
	respond(response, nil, errors.New("unclassified response"))
	var fault WorkflowError
	if err := json.Unmarshal(response.Body.Bytes(), &fault); err != nil || response.Code != http.StatusServiceUnavailable || fault.Code != "DEPENDENCY_UNAVAILABLE" {
		t.Fatalf("unclassified response mapping: %d %+v, %v", response.Code, fault, err)
	}
	if _, err := IdentityMigration("invalid schema"); !errors.Is(err, Invalid) {
		t.Fatalf("invalid identity schema: %v", err)
	}
	if _, err := NewRevocationProvider("", "", nil); !errors.Is(err, Invalid) {
		t.Fatalf("unbound revocation transport: %v", err)
	}
}
