package control

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSettlementWorkerRetainsProviderStateByObligationKey(t *testing.T) {
	verifier := chain(t, nil)
	env := repairM1020EnvironmentFor(t, verifier)
	ctx := context.Background()
	key := "bounty.coverage-settlement"
	obligation := Obligation{ID: id(), Scope: "service", BountyID: "coverage-settlement", IdempotencyKey: key, Amount: "1000", Destination: beneficiary, AuthorityReference: custody, State: controlPending, Asset: pax, AssetKey: pax.Key(), Created: time.Now(), Revision: 1}
	wire, err := json.Marshal(obligation)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.store.local.save(map[string]json.RawMessage{controlSettlement + "/" + key: wire}, nil); err != nil {
		t.Fatal(err)
	}
	var lookups, submissions atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Idempotency-Key") != key || r.Header.Get("Authorization") != "Bearer "+strings.Repeat("p", 32) {
			t.Errorf("settlement request: %s %s", r.Method, r.URL.Path)
		}
		switch r.Method {
		case http.MethodGet:
			lookups.Add(1)
			http.NotFound(w, r)
		case http.MethodPut:
			submissions.Add(1)
			var request struct {
				Key         string `json:"idempotencyKey"`
				Destination string `json:"destination"`
				Amount      string `json:"amountRaw"`
				Authority   string `json:"authorityReference"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.Key != key || request.Destination != beneficiary || request.Amount != obligation.Amount || request.Authority != custody {
				t.Errorf("settlement payload: %+v", request)
			}
			if err := json.NewEncoder(w).Encode(PayoutResult{State: controlPending}); err != nil {
				t.Error(err)
			}
		default:
			t.Errorf("settlement method: %s", r.Method)
		}
	}))
	defer server.Close()
	provider, err := NewPayoutProvider(server.URL, strings.Repeat("p", 32), "settlement controller", custody, repairM1020Transport(server))
	if err != nil {
		t.Fatal(err)
	}
	worker, err := NewSettlementWorker(env.store, provider)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewHTTPService(env.store, worker)
	if err != nil {
		t.Fatal(err)
	}
	capability := httptest.NewRecorder()
	service.Handler().ServeHTTP(capability, httptest.NewRequest(http.MethodGet, "/v2/control/settlement-capability", nil))
	var available struct {
		Data struct {
			Available          bool   `json:"available"`
			Controller         string `json:"controller"`
			AuthorityReference string `json:"authorityReference"`
		} `json:"data"`
	}
	if err := json.Unmarshal(capability.Body.Bytes(), &available); err != nil || capability.Code != http.StatusOK || !available.Data.Available || available.Data.Controller != provider.Controller || available.Data.AuthorityReference != custody {
		t.Fatalf("settlement capability: %d %+v, %v", capability.Code, available, err)
	}
	if _, err := worker.Process(ctx, env.credentials["finance"], "other", key, "different scope"); !errors.Is(err, Denied) {
		t.Fatalf("other obligation scope: %v", err)
	}
	if _, err := worker.Process(ctx, env.credentials["finance"], "service", "missing-obligation", "unknown obligation"); !errors.Is(err, Missing) {
		t.Fatalf("missing obligation: %v", err)
	}
	result, err := worker.Process(ctx, env.credentials["finance"], "service", key, "submit retained obligation")
	if err != nil || result.State != controlPending || result.Revision != 2 || lookups.Load() != 1 || submissions.Load() != 1 {
		t.Fatalf("retained payout state: %+v, %v, lookup=%d submit=%d", result, err, lookups.Load(), submissions.Load())
	}
	attempt := repairM1020Read[payoutAttempt](t, env.store, "payout_attempt", key)
	if !attempt.Accepted || !attempt.Submitted || attempt.State != controlPending || attempt.Claim != "" {
		t.Fatalf("retained payout attempt: %+v", attempt)
	}
	if err := worker.Drain(ctx, "", func(string, error) {}); !errors.Is(err, Invalid) {
		t.Fatalf("unbound settlement credential: %v", err)
	}
	if err := worker.Run(ctx, "", func(string, error) {}); !errors.Is(err, Invalid) {
		t.Fatalf("worker without credential: %v", err)
	}
	if err := env.store.ObserveSettlement(ctx, env.credentials["finance"], "service", key, "unconfirmed chain reference", "0x"+strings.Repeat("1", 64)); err != nil {
		t.Fatalf("unconfirmed settlement retained: %v", err)
	}
	current := repairM1020Read[Obligation](t, env.store, controlSettlement, key)
	if current.State != controlUnknown || current.Revision != 3 || current.SettlementReference != "" {
		t.Fatalf("unknown chain proof: %+v", current)
	}
	requestBody, err := json.Marshal(SettlementRequest{Scope: "service", Key: key, Reason: "retry pending obligation"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v2/control/settlements", strings.NewReader(string(requestBody)))
	request.Header.Set("Authorization", "Bearer "+env.credentials["finance"])
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	service.Handler().ServeHTTP(response, request)
	var envelope struct {
		Data Obligation `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || response.Code != http.StatusOK || envelope.Data.IdempotencyKey != key || envelope.Data.State != controlPending {
		t.Fatalf("settlement route: %d %+v, %v", response.Code, envelope, err)
	}
}
