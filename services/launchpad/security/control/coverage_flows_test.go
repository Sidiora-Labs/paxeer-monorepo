package control

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReplayProposalBoundsAndStableIdentity(t *testing.T) {
	env := repairM1020EnvironmentFor(t, nil)
	ctx := context.Background()
	credential := env.credentials["infrastructure"]
	proposal := ReplayProposal{Key: "bounded-replay", Scope: "service", Reason: "restore projection", Expires: time.Now().Add(time.Hour), Payload: ReplayPayload{ChainID: "125", FromHeight: "1", ToHeight: "2", Projection: "candles", DryRun: true}}
	for _, sample := range []struct {
		name string
		edit func(*ReplayProposal)
	}{
		{"invalid start", func(q *ReplayProposal) { q.Payload.FromHeight = "01" }},
		{"invalid end", func(q *ReplayProposal) { q.Payload.ToHeight = "bad" }},
		{"reverse range", func(q *ReplayProposal) { q.Payload.ToHeight = "0" }},
		{"other chain", func(q *ReplayProposal) { q.Payload.ChainID = "1" }},
		{"invalid projection", func(q *ReplayProposal) { q.Payload.Projection = "bad projection" }},
		{"invalid key", func(q *ReplayProposal) { q.Key = "bad key" }},
		{"expired", func(q *ReplayProposal) { q.Expires = time.Now().Add(-time.Minute) }},
		{"distant expiry", func(q *ReplayProposal) { q.Expires = time.Now().Add(25 * time.Hour) }},
	} {
		t.Run(sample.name, func(t *testing.T) {
			candidate := proposal
			sample.edit(&candidate)
			if _, err := env.store.ProposeReplay(ctx, credential, candidate); !errors.Is(err, Invalid) {
				t.Fatalf("proposal %s: %v", sample.name, err)
			}
		})
	}
	first, err := env.store.ProposeReplay(ctx, credential, proposal)
	if err != nil || first.Kind != controlReplay || first.PayloadReference != proposal.Key {
		t.Fatalf("replay proposal: %+v, %v", first, err)
	}
	again, err := env.store.ProposeReplay(ctx, credential, proposal)
	if err != nil || again.ID != first.ID || again.PayloadHash != first.PayloadHash || again.Revision != first.Revision {
		t.Fatalf("repeated replay changed identity: %+v, %v", again, err)
	}
	changed := proposal
	changed.Payload.ToHeight = "3"
	if _, err := env.store.ProposeReplay(ctx, credential, changed); !errors.Is(err, Conflict) {
		t.Fatalf("changed replay: %v", err)
	}
	if _, err := env.store.TransitionWorkflow(ctx, credential, TransitionRequest{Key: proposal.Key, Scope: "service", Reason: "execute", Next: controlExecuting}); !errors.Is(err, Invalid) {
		t.Fatalf("missing revision: %v", err)
	}
	revision := first.Revision
	if _, err := env.store.TransitionWorkflow(ctx, credential, TransitionRequest{Key: proposal.Key, Scope: "service", Reason: "execute", Next: controlExecuting, Expected: &revision}); !errors.Is(err, Unavailable) {
		t.Fatalf("unbound replay provider: %v", err)
	}
}

func TestOwnerBudgetPolicyAndRecommendationTransitions(t *testing.T) {
	env := phase2EnvironmentFor(t)
	ctx := context.Background()
	owner := env.actors["user"]
	credential := env.credentials["user"]
	budget := Budget{Owner: owner, Asset: usdl, Requests: 2, Spend: "1000000", Expires: time.Now().Add(time.Hour)}
	if err := env.store.SetBudget(ctx, credential, owner, "set recommendation budget", budget, 0); err != nil {
		t.Fatal(err)
	}
	if err := env.store.SetBudget(ctx, credential, owner, "repeat budget revision", budget, 0); !errors.Is(err, Conflict) {
		t.Fatalf("stale budget revision: %v", err)
	}
	other := budget
	other.Owner = env.actors["finance"]
	if err := env.store.SetBudget(ctx, credential, owner, "different budget owner", other, 0); !errors.Is(err, Invalid) {
		t.Fatalf("different budget owner: %v", err)
	}
	invalidBudget := budget
	invalidBudget.Expires = time.Now().Add(25 * time.Hour)
	if err := env.store.SetBudget(ctx, credential, owner, "distant budget expiry", invalidBudget, 1); !errors.Is(err, Invalid) {
		t.Fatalf("distant budget expiry: %v", err)
	}
	policy := Policy{ID: id(), Owner: owner, Mode: controlDisclosedAssistant, Disclosure: "Recommendation only", Asset: usdl, Instruments: []string{USDLAddress}, Requests: 2, Spend: "1000000", QuoteAge: time.Minute, Expires: time.Now().Add(time.Hour)}
	duplicateInstrument := policy
	duplicateInstrument.Instruments = []string{USDLAddress, USDLAddress}
	if err := env.store.CreatePolicy(ctx, credential, owner, "duplicate instrument", duplicateInstrument); !errors.Is(err, Invalid) {
		t.Fatalf("duplicate instrument: %v", err)
	}
	if err := env.store.CreatePolicy(ctx, credential, owner, "create recommendation policy", policy); err != nil {
		t.Fatal(err)
	}
	if err := env.store.CreatePolicy(ctx, credential, owner, "duplicate recommendation policy", policy); !errors.Is(err, Conflict) {
		t.Fatalf("duplicate policy: %v", err)
	}
	if err := env.store.PolicyState(ctx, credential, owner, policy.ID, "invalid state", "settling", 1); !errors.Is(err, Invalid) {
		t.Fatalf("unsupported state: %v", err)
	}
	if err := env.store.PolicyState(ctx, credential, owner, policy.ID, "enable recommendation", controlEnabled, 1); err != nil {
		t.Fatal(err)
	}
	if err := env.store.PolicyState(ctx, credential, owner, policy.ID, "stale policy revision", "paused", 1); !errors.Is(err, Conflict) {
		t.Fatalf("stale policy: %v", err)
	}
	if _, err := env.store.ReserveRecommendation(ctx, credential, owner, policy.ID, "future-quote", USDLAddress, "future quote", time.Now().Add(time.Minute)); !errors.Is(err, Denied) {
		t.Fatalf("future quote: %v", err)
	}
	if _, err := env.store.ReserveRecommendation(ctx, credential, owner, policy.ID, "wrong-asset", beneficiary, "unlisted instrument", time.Now()); !errors.Is(err, Denied) {
		t.Fatalf("unlisted instrument: %v", err)
	}
	reservation, err := env.store.ReserveRecommendation(ctx, credential, owner, policy.ID, "accepted-quote", USDLAddress, "current quote", time.Now())
	if err != nil || reservation.State != "suggestion" || reservation.Owner != owner {
		t.Fatalf("current quote: %+v, %v", reservation, err)
	}
	view, err := env.store.Recommendation(ctx, credential, owner, "inspect recommendation", "accepted-quote")
	if err != nil || view.Reservation.ID != reservation.ID || view.Origin != "automated_recommendation" || view.OrganicRankingEligible {
		t.Fatalf("disclosed recommendation: %+v, %v", view, err)
	}
	for _, path := range []string{"campaigns/" + policy.ID, "recommendations/accepted-quote"} {
		request, err := http.NewRequest(http.MethodGet, env.server.URL+"/v2/control/"+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+credential)
		response, err := env.server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		decodeErr := json.NewDecoder(response.Body).Decode(&envelope)
		response.Body.Close()
		if response.StatusCode != http.StatusOK || decodeErr != nil || len(envelope.Data) == 0 {
			t.Fatalf("owner view %s: %d, %s, %v", path, response.StatusCode, envelope.Data, decodeErr)
		}
	}
	again, err := env.store.ReserveRecommendation(ctx, credential, owner, policy.ID, "accepted-quote", USDLAddress, "current quote retry", time.Now())
	if err != nil || again.ID != reservation.ID || again.Owner != reservation.Owner || again.PolicyID != reservation.PolicyID || !again.Created.Equal(reservation.Created) {
		t.Fatalf("stable recommendation: %+v, %v", again, err)
	}
	if _, err := env.store.ReserveRecommendation(ctx, credential, owner, policy.ID, "accepted-quote", beneficiary, "changed instrument", time.Now()); !errors.Is(err, Conflict) {
		t.Fatalf("changed reservation: %v", err)
	}
	if _, err := env.store.ReserveRecommendation(ctx, credential, owner, policy.ID, "stale-quote", USDLAddress, "stale quote", time.Now().Add(-2*time.Minute)); !errors.Is(err, Denied) {
		t.Fatalf("stale quote: %v", err)
	}
	if _, err := env.store.ReserveRecommendation(ctx, credential, owner, policy.ID, "accepted-second", USDLAddress, "second current quote", time.Now()); err != nil {
		t.Fatalf("second recommendation: %v", err)
	}
	if _, err := env.store.ReserveRecommendation(ctx, credential, owner, policy.ID, "exhausted-quote", USDLAddress, "budget ceiling", time.Now()); !errors.Is(err, BudgetExceeded) {
		t.Fatalf("exhausted recommendation budget: %v", err)
	}
	if err := env.store.PolicyState(ctx, credential, owner, policy.ID, "pause recommendation", "paused", 4); err != nil {
		t.Fatal(err)
	}
	if _, err := env.store.ReserveRecommendation(ctx, credential, owner, policy.ID, "paused-quote", USDLAddress, "paused recommendation", time.Now()); !errors.Is(err, Denied) {
		t.Fatalf("paused recommendation: %v", err)
	}
	if err := env.store.PolicyState(ctx, credential, owner, policy.ID, "stop recommendation", controlStopped, 5); err != nil {
		t.Fatal(err)
	}
	if err := env.store.PolicyState(ctx, credential, owner, policy.ID, "restart stopped policy", controlEnabled, 6); !errors.Is(err, Denied) {
		t.Fatalf("stopped policy restarted: %v", err)
	}
	withoutBudget := policy
	withoutBudget.ID = id()
	withoutBudget.Asset = pax
	if err := env.store.CreatePolicy(ctx, credential, owner, "create native recommendation", withoutBudget); err != nil {
		t.Fatal(err)
	}
	if err := env.store.PolicyState(ctx, credential, owner, withoutBudget.ID, "enable native recommendation", controlEnabled, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := env.store.ReserveRecommendation(ctx, credential, owner, withoutBudget.ID, "native-quote", USDLAddress, "missing native budget", time.Now()); !errors.Is(err, Unavailable) {
		t.Fatalf("missing budget: %v", err)
	}
}

func TestBountyDraftStatesAndFundingAuthority(t *testing.T) {
	env := repairM1020EnvironmentFor(t, nil)
	ctx := context.Background()
	credential := env.credentials["finance"]
	b := bounty("coverage-bounty", usdl, "1000")
	invalid := b
	invalid.Amount = "0"
	if err := env.store.CreateBounty(ctx, credential, "service", "zero amount", invalid); !errors.Is(err, Invalid) {
		t.Fatalf("zero bounty: %v", err)
	}
	if err := env.store.CreateBounty(ctx, credential, "service", "create bounty draft", b); err != nil {
		t.Fatal(err)
	}
	if err := env.store.CreateBounty(ctx, credential, "service", "duplicate bounty", b); !errors.Is(err, Conflict) {
		t.Fatalf("duplicate bounty: %v", err)
	}
	for _, sample := range []struct {
		name, scope, next string
		revision          uint64
		want              error
	}{
		{"invalid key", "service", controlFunded, 1, Invalid},
		{"different scope", "other", controlFunded, 1, Denied},
		{"stale revision", "service", controlFunded, 2, Conflict},
		{"funding absent", "service", controlFunded, 1, FundingRequired},
		{"active without funding", "service", controlActive, 1, FundingRequired},
		{"submitted without activation", "service", controlSubmitted, 1, Denied},
		{"unknown state", "service", "unknown", 1, Invalid},
	} {
		key := b.ID
		if sample.name == "invalid key" {
			key = "bad key"
		}
		if err := env.store.BountyState(ctx, credential, sample.scope, key, "review bounty state", sample.next, "evidence", beneficiary, sample.revision); !errors.Is(err, sample.want) {
			t.Fatalf("%s: %v", sample.name, err)
		}
	}
	if err := env.store.BountyState(ctx, credential, "service", b.ID, "cancel draft", controlCancelled, "", "", 1); err != nil {
		t.Fatal(err)
	}
	if err := env.store.BountyState(ctx, credential, "service", b.ID, "cancel again", controlCancelled, "", "", 2); !errors.Is(err, Denied) {
		t.Fatalf("cancelled bounty: %v", err)
	}
	if err := env.store.ObserveSettlement(ctx, credential, "service", "missing-obligation", "observe absent settlement", "reference"); !errors.Is(err, Unavailable) {
		t.Fatalf("settlement without verifier: %v", err)
	}
}

func TestProviderRequestsBindAuthorityAndResponse(t *testing.T) {
	ctx := context.Background()
	credential := strings.Repeat("p", 32)
	incident := Incident{ID: "revoke-credential", ServiceIdentity: "service-credential"}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+credential || r.Header.Get("Idempotency-Key") == "" {
			t.Errorf("unbound provider request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "denied", http.StatusForbidden)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/identity-revocations/") {
			if r.Method != http.MethodPut || r.Header.Get("Idempotency-Key") != incident.ID {
				t.Errorf("revocation route: %s %s", r.Method, r.URL.Path)
			}
			if err := json.NewEncoder(w).Encode(RevocationAcknowledgement{Incident: incident.ID, Identity: incident.ServiceIdentity, Rotation: "rotation-1", Integrity: "integrity-1", SecretsRevoked: true, CachesInvalidated: true}); err != nil {
				t.Error(err)
			}
			return
		}
		if r.Method == http.MethodGet {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPut {
			t.Errorf("payout method: %s", r.Method)
		}
		var request struct {
			Key         string `json:"idempotencyKey"`
			Destination string `json:"destination"`
			Amount      string `json:"amountRaw"`
			Authority   string `json:"authorityReference"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Key != r.Header.Get("Idempotency-Key") || request.Destination != beneficiary || request.Amount != "1000" || request.Authority != custody {
			t.Errorf("payout request: %+v", request)
		}
		if err := json.NewEncoder(w).Encode(PayoutResult{State: controlPending}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	transport := repairM1020Transport(server)
	revocation, err := NewRevocationProvider(server.URL, credential, transport)
	if err != nil {
		t.Fatal(err)
	}
	ack, err := revocation.Revoke(ctx, incident)
	if err != nil || ack.Incident != incident.ID || !ack.SecretsRevoked || !ack.CachesInvalidated {
		t.Fatalf("revocation acknowledgement: %+v, %v", ack, err)
	}
	if _, err := revocation.Revoke(ctx, Incident{ID: "bad key"}); !errors.Is(err, Invalid) {
		t.Fatalf("invalid revocation identity: %v", err)
	}
	payout, err := NewPayoutProvider(server.URL, credential, "settlement authority", custody, transport)
	if err != nil {
		t.Fatal(err)
	}
	obligation := Obligation{IdempotencyKey: "bounty.coverage", Asset: usdl, Destination: beneficiary, Amount: "1000", AuthorityReference: custody}
	result, err := payout.Submit(ctx, obligation)
	if err != nil || result.State != controlPending {
		t.Fatalf("pending payout: %+v, %v", result, err)
	}
	result, err = payout.Lookup(ctx, obligation.IdempotencyKey)
	if err != nil || result.State != controlAbsent {
		t.Fatalf("absent payout: %+v, %v", result, err)
	}
	for _, sample := range []struct {
		name string
		edit func(*Obligation)
		want error
	}{
		{"different authority", func(o *Obligation) { o.AuthorityReference = beneficiary }, Denied},
		{"invalid asset", func(o *Obligation) { o.Asset = Asset{} }, Denied},
		{"invalid destination", func(o *Obligation) { o.Destination = "invalid" }, Denied},
		{"invalid amount", func(o *Obligation) { o.Amount = "01" }, Invalid},
	} {
		candidate := obligation
		sample.edit(&candidate)
		if _, err := payout.Submit(ctx, candidate); !errors.Is(err, sample.want) {
			t.Fatalf("%s: %v", sample.name, err)
		}
	}
	if _, err := payout.Lookup(ctx, "bad key"); !errors.Is(err, Invalid) {
		t.Fatalf("invalid payout key: %v", err)
	}
	if _, err := NewPayoutProvider(server.URL, credential, "settlement authority", custody, &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS10}}); !errors.Is(err, Invalid) {
		t.Fatalf("weak payout transport: %v", err)
	}
}
