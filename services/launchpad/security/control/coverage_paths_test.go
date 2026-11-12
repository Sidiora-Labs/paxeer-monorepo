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

func TestControlHTTPRoutesRejectUnqualifiedMutations(t *testing.T) {
	env := repairM1020EnvironmentFor(t, nil)
	operations := []string{"replays", "actions", "complete-action", "recover-incident", "bounty-submissions", "transition", controlCaps, "budget", "policies", "policy-state", "recommendations", "bounties", "bounty-state", controlInventory, "access", "incidents", "settlements", "unknown"}
	for _, operation := range operations {
		t.Run(operation, func(t *testing.T) {
			status, body := env.post(t, "infrastructure", operation, map[string]any{})
			if status < 400 || status > 599 {
				t.Fatalf("unqualified %s mutation: %d %s", operation, status, body)
			}
			var response WorkflowError
			if err := json.Unmarshal(body, &response); err != nil || response.Code == "" || response.RequestID == "" {
				t.Fatalf("unqualified %s response: %s, %v", operation, body, err)
			}
		})
	}
	for _, operation := range []string{"complete-action", "recover-incident", "transition", controlCaps, "budget", "policy-state", "bounty-state"} {
		status, body := env.post(t, "infrastructure", operation, map[string]any{"expectedRevision": 0})
		if status < 400 || status > 599 {
			t.Fatalf("unqualified revision-bearing %s mutation: %d %s", operation, status, body)
		}
	}
	response, err := env.server.Client().Get(env.server.URL + "/v2/control/settlement-capability")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var envelope struct {
		Data struct {
			Available bool     `json:"available"`
			Blockers  []string `json:"blockers"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil || response.StatusCode != http.StatusOK || envelope.Data.Available || len(envelope.Data.Blockers) != 1 || envelope.Data.Blockers[0] != "settlement_authority_unavailable" {
		t.Fatalf("settlement capability: %d %+v, %v", response.StatusCode, envelope, err)
	}
	for _, path := range []string{"campaigns/unknown", "recommendations/unknown"} {
		request := httptest.NewRequest(http.MethodGet, "/v2/control/"+path, nil)
		recorder := httptest.NewRecorder()
		service, err := NewHTTPService(env.store, nil)
		if err != nil {
			t.Fatal(err)
		}
		service.Handler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("uncredentialed %s: %d", path, recorder.Code)
		}
	}
	if service, err := NewHTTPService(nil, nil); !errors.Is(err, Invalid) || service != nil {
		t.Fatalf("service without store: %+v, %v", service, err)
	}
	service, err := NewHTTPService(env.store, nil)
	if err != nil {
		t.Fatal(err)
	}
	warning := httptest.NewRecorder()
	service.Handler().ServeHTTP(warning, httptest.NewRequest(http.MethodGet, "/v2/control/caps/bad%20capability", nil))
	if warning.Code != http.StatusOK || !strings.Contains(warning.Body.String(), "cap_unavailable") {
		t.Fatalf("invalid capability warning: %d %s", warning.Code, warning.Body.String())
	}
}

func TestControlPublicViewsAndErrorMapping(t *testing.T) {
	policy := policy(id())
	policy.Owner = id()
	policy.Instruments = []string{beneficiary}
	for _, state := range []string{controlDraft, "paused", controlStopped, controlRevoked, controlEnabled} {
		policy.State = state
		view, err := PublicCampaign(policy)
		if err != nil {
			t.Fatalf("campaign %s: %v", state, err)
		}
		want := state
		if state == controlEnabled {
			want = "blocked"
		}
		if view.State != want || view.BudgetAsset.Kind != "erc20" || view.AccountID != policy.Owner {
			t.Fatalf("campaign %s: %+v", state, view)
		}
	}
	policy.State = "unknown"
	if _, err := PublicCampaign(policy); !errors.Is(err, Invalid) {
		t.Fatalf("unknown campaign state: %v", err)
	}
	b := bounty(id(), usdl, "1000000")
	b.Scope = "service"
	for state, want := range map[string]string{controlDraft: controlDraft, controlCancelled: controlCancelled, controlExpired: controlExpired, controlFunded: "funding", controlActive: "open", controlSubmitted: "reviewing", controlAdjudicating: "reviewing", controlAwarded: "reviewing"} {
		b.State = state
		view, err := PublicBounty(b, beneficiary, nil)
		if err != nil || view.State != want {
			t.Fatalf("bounty %s: %+v, %v", state, view, err)
		}
	}
	b.State = "unknown"
	if _, err := PublicBounty(b, beneficiary, nil); !errors.Is(err, Invalid) {
		t.Fatalf("unknown bounty state: %v", err)
	}
	for _, sample := range []struct {
		err  error
		code int
		name string
	}{{Invalid, 400, "CURSOR_INVALID"}, {Denied, 403, "SCOPE_DENIED"}, {Missing, 404, "NOT_FOUND"}, {Conflict, 409, "IDEMPOTENCY_CONFLICT"}, {FundingRequired, 503, "DEPENDENCY_UNAVAILABLE"}, {BudgetExceeded, 503, "DEPENDENCY_UNAVAILABLE"}, {Unavailable, 503, "DEPENDENCY_UNAVAILABLE"}} {
		code, name := ErrorResponse(sample.err)
		if code != sample.code || name != sample.name {
			t.Fatalf("fault %v: %d %s", sample.err, code, name)
		}
	}
}

func TestControlWorkerLoopsObserveCancellation(t *testing.T) {
	env := repairM1020EnvironmentFor(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	observed := 0
	observe := func(string, error) { observed++ }
	for name, run := range map[string]func() error{
		"reconciliation": func() error {
			return (ReconciliationWorker{Store: env.store, Credential: env.credentials["infrastructure"], Assets: []Asset{usdl}, Interval: time.Second, Observe: observe}).Run(ctx)
		},
		"incident": func() error {
			return (IncidentWorker{Store: env.store, Credential: env.credentials["infrastructure"], Observe: observe}).Run(ctx)
		},
		"replay": func() error {
			return (ReplayWorker{Store: env.store, Provider: repairM1020ReplayProvider(t, nil), Credential: env.credentials["infrastructure"], Observe: observe}).Run(ctx)
		},
	} {
		if err := run(); !errors.Is(err, context.Canceled) {
			t.Fatalf("%s cancellation: %v", name, err)
		}
	}
	if observed == 0 {
		t.Fatal("worker did not observe its reconciliation attempt")
	}
}

func TestControlSubmissionAndAwardRequireBoundAuthority(t *testing.T) {
	env := repairM1020EnvironmentFor(t, nil)
	ctx := context.Background()
	credential := env.credentials["infrastructure"]
	owner := env.actors["infrastructure"]
	if err := env.store.SubmitBounty(ctx, credential, owner, "service", "submit", Submission{}); !errors.Is(err, Invalid) {
		t.Fatalf("empty submission: %v", err)
	}
	submission := Submission{ID: id(), BountyID: "bounty", Author: owner, Wallet: beneficiary, Body: "evidence", EvidenceURLs: []string{"https://example.invalid/evidence"}}
	if err := env.store.SubmitBounty(ctx, credential, owner, "service", "submit", submission); !errors.Is(err, Unavailable) {
		t.Fatalf("local submission without wallet authority: %v", err)
	}
	if err := (StoredAwardAuthority{}).VerifyAward(ctx, Bounty{}, submission.ID, submission.Wallet); !errors.Is(err, Unavailable) {
		t.Fatalf("award without store: %v", err)
	}
	identity, err := NewPostgresIdentity(nil, "control", "identity")
	if !errors.Is(err, Invalid) || identity != nil {
		t.Fatalf("identity without database: %+v, %v", identity, err)
	}
	if err := (&PostgresIdentity{}).Wallet(ctx, "invalid", beneficiary); !errors.Is(err, Denied) {
		t.Fatalf("wallet without valid actor: %v", err)
	}
	service, err := NewHTTPService(env.store, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/v2/control/caps/quote", nil)
	record := httptest.NewRecorder()
	service.Handler().ServeHTTP(record, request)
	if record.Code != http.StatusOK || !strings.Contains(record.Body.String(), "cap_unavailable") {
		t.Fatalf("unavailable cap warning: %d %s", record.Code, record.Body.String())
	}
}

func TestRuntimeCapabilityRoutesFollowFreshCapState(t *testing.T) {
	env := repairM1020EnvironmentFor(t, nil)
	runtime := &Runtime{Store: env.store}
	forwarded := 0
	handler, err := runtime.RequireCapability("quote", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		forwarded++
		w.WriteHeader(http.StatusNoContent)
	}))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/quote", nil)
	record := httptest.NewRecorder()
	handler.ServeHTTP(record, request)
	if record.Code != http.StatusServiceUnavailable || forwarded != 0 {
		t.Fatalf("missing cap forwarded: %d, %d", record.Code, forwarded)
	}
	cap := Cap{Capability: "quote", Version: 1, Limit: 0, Effective: time.Now().Add(-time.Minute), Expires: time.Now().Add(time.Hour), Revision: 1}
	for _, sample := range []struct {
		limit  uint64
		status int
	}{{0, http.StatusServiceUnavailable}, {1, http.StatusNoContent}} {
		cap.Limit = sample.limit
		wire, err := json.Marshal(cap)
		if err != nil {
			t.Fatal(err)
		}
		objects := map[string]json.RawMessage{controlCaps + "/quote": wire}
		if err := env.store.local.save(objects, nil); err != nil {
			t.Fatal(err)
		}
		record = httptest.NewRecorder()
		handler.ServeHTTP(record, request)
		if record.Code != sample.status {
			t.Fatalf("cap limit %d: %d", sample.limit, record.Code)
		}
	}
	if forwarded != 1 {
		t.Fatalf("enabled cap forwards %d requests", forwarded)
	}
	if _, err := runtime.RequireCapability("bad capability", http.NotFoundHandler()); !errors.Is(err, Invalid) {
		t.Fatalf("invalid capability: %v", err)
	}
}
