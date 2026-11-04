package control

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
)

type repairM1020Environment struct {
	store       *Store
	authority   *LocalCredentialAuthority
	credentials map[string]string
	actors      map[string]string
	path        string
	server      *httptest.Server
}

func repairM1020EnvironmentFor(t *testing.T, verifier EvidenceVerifier) *repairM1020Environment {
	t.Helper()
	e := &repairM1020Environment{authority: NewLocalCredentialAuthority(), credentials: map[string]string{}, actors: map[string]string{}, path: filepath.Join(t.TempDir(), "objects.json")}
	for _, role := range []string{"infrastructure", "operator2", "finance", "support"} {
		actual := role
		if role == "operator2" {
			actual = "infrastructure"
		}
		e.actors[role] = id()
		e.credentials[role] = readySecret(t)
		must(t, e.authority.Grant(e.credentials[role], Principal{ID: e.actors[role], Role: actual, CredentialID: id(), Scopes: []string{"platform", "service"}}, time.Now().Add(time.Hour)))
	}
	var err error
	e.store, err = OpenLocalStore(e.path, e.authority.Reader(), verifier)
	must(t, err)
	e.mount(t)
	t.Cleanup(func() { e.server.Close(); must(t, e.store.CloseLocal()) })
	return e
}
func (e *repairM1020Environment) mount(t *testing.T) {
	t.Helper()
	service, err := NewHTTPService(e.store, nil)
	must(t, err)
	e.server = httptest.NewServer(service.Handler())
}
func (e *repairM1020Environment) reopen(t *testing.T) {
	t.Helper()
	e.server.Close()
	verifier := e.store.verifier
	provider := e.store.replay
	must(t, e.store.CloseLocal())
	var err error
	e.store, err = OpenLocalStore(e.path, e.authority.Reader(), verifier)
	must(t, err)
	e.store.replay = provider
	e.mount(t)
}
func (e *repairM1020Environment) post(t *testing.T, role, operation string, body any) (int, []byte) {
	t.Helper()
	raw, err := json.Marshal(body)
	must(t, err)
	req, err := http.NewRequest(http.MethodPost, e.server.URL+"/v2/control/"+operation, bytes.NewReader(raw))
	must(t, err)
	req.Header.Set("Authorization", "Bearer "+e.credentials[role])
	req.Header.Set("Content-Type", "application/json")
	response, err := e.server.Client().Do(req)
	must(t, err)
	defer response.Body.Close()
	raw, err = io.ReadAll(response.Body)
	must(t, err)
	return response.StatusCode, raw
}
func repairM1020Read[T any](t *testing.T, s *Store, kind, key string) T {
	t.Helper()
	var out T
	must(t, s.readObject(context.Background(), kind, key, &out))
	return out
}
func repairM1020Transport(server *httptest.Server) *http.Transport {
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	return &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
}

// The peer is an explicitly scoped owner-adapter transcript. It consumes actual
// requests emitted by ReplayProvider, retaining completion by the producer's
// identity. It is not a substitute for the indexer's qualified ReplayRunner.
func repairM1020ReplayProvider(t *testing.T, serve func(http.ResponseWriter, *http.Request)) *ReplayProvider {
	t.Helper()
	if serve == nil {
		serve = func(w http.ResponseWriter, r *http.Request) { http.Error(w, "owner unavailable", 503) }
	}
	server := httptest.NewTLSServer(http.HandlerFunc(serve))
	t.Cleanup(server.Close)
	provider, err := NewReplayProvider(server.URL, strings.Repeat("r", 32), repairM1020Transport(server))
	must(t, err)
	return provider
}
func TestRepairM_1020_ReplayExecutionRetryRestart(t *testing.T) {
	ctx := context.Background()
	e := repairM1020EnvironmentFor(t, nil)
	var mu sync.Mutex
	requests := []ReplayExecution{}
	completions := map[string]ReplayResult{}
	effects := 0
	e.store.replay = repairM1020ReplayProvider(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method != http.MethodPut || r.Header.Get("Authorization") != "Bearer "+strings.Repeat("r", 32) {
			http.Error(w, "denied", 403)
			return
		}
		var request ReplayExecution
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, "invalid", 400)
			return
		}
		requests = append(requests, request)
		if r.Header.Get("Idempotency-Key") != request.Job.CompletionIdentity || !strings.HasSuffix(r.URL.Path, request.Job.CompletionIdentity) || digest(request.Payload) != request.Job.PayloadHash {
			t.Error("unbound replay transport")
			http.Error(w, "conflict", 409)
			return
		}
		result, exists := completions[request.Job.CompletionIdentity]
		if !exists {
			effects++
			result = ReplayResult{request.Job.ActionID, request.JobID, request.Job.PayloadHash, request.Job.PayloadReference, request.Job.CompletionIdentity, "succeeded", "owner-checkpoint:2"}
			completions[request.Job.CompletionIdentity] = result
			http.Error(w, "acknowledgement interrupted after checkpoint", 503)
			return
		}
		must(t, json.NewEncoder(w).Encode(result))
	})
	proposal := ReplayProposal{Key: "approved-replay", Scope: "service", Reason: "restore projection", Expires: time.Now().Add(time.Hour), Payload: ReplayPayload{"125", "1", "2", "candles", true}}
	status, raw := e.post(t, "infrastructure", "replays", proposal)
	if status != 200 {
		t.Fatalf("proposal: %d %s", status, raw)
	}
	var envelope struct {
		Data Action `json:"data"`
	}
	must(t, json.Unmarshal(raw, &envelope))
	action := envelope.Data
	action, err := e.store.Transition(ctx, e.credentials["operator2"], proposal.Key, "service", "review exact replay", "approved", action.Revision)
	must(t, err)
	action, err = e.store.TransitionWorkflow(ctx, e.credentials["infrastructure"], TransitionRequest{proposal.Key, "service", "execute approved replay", "executing", &action.Revision})
	must(t, err)
	if action.PayloadReference != proposal.Key || action.CompletionIdentity != "control-replay:"+action.JobID {
		t.Fatal("ambiguous durable job", action)
	}
	worker := ReplayWorker{Store: e.store, Provider: e.store.replay, Credential: e.credentials["infrastructure"], Observe: func(string, error) {}}
	is(t, worker.Process(ctx, proposal.Key, "service"), Unavailable)
	if repairM1020Read[Action](t, e.store, "action", proposal.Key).State != "executing" {
		t.Fatal("lost acknowledgement became success")
	}
	schedule := repairM1020Read[workSchedule](t, e.store, "work_schedule", "job."+action.JobID)
	e.reopen(t)
	worker.Store = e.store
	is(t, worker.Process(ctx, proposal.Key, "service"), Unavailable)
	if got := repairM1020Read[workSchedule](t, e.store, "work_schedule", "job."+action.JobID); got != schedule {
		t.Fatal("restart lost retry schedule")
	}
	mu.Lock()
	if len(requests) != 1 || effects != 1 {
		t.Fatal(requests, effects)
	}
	mu.Unlock()
	// Respect the real persisted backoff. No test clock bypass or shortened lease.
	if delay := time.Until(schedule.Next); delay > 0 {
		time.Sleep(delay + 10*time.Millisecond)
	}
	must(t, worker.Drain(ctx))
	final := repairM1020Read[Action](t, e.store, "action", proposal.Key)
	if final.State != "succeeded" || final.JobID != action.JobID {
		t.Fatal(final)
	}
	result := repairM1020Read[ReplayResult](t, e.store, "executor_result", action.JobID)
	must(t, (DurableJobResults{e.store}).Verify(ctx, final, result.State, result.Reference))
	forged := final
	forged.CompletionIdentity = "another-job"
	is(t, (DurableJobResults{e.store}).Verify(ctx, forged, result.State, result.Reference), Denied)
	forged = final
	forged.PayloadReference = "another-payload"
	is(t, (DurableJobResults{e.store}).Verify(ctx, forged, result.State, result.Reference), Denied)
	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 || effects != 1 || requests[0].JobID != requests[1].JobID || requests[0].Payload != requests[1].Payload {
		t.Fatal("retry changed identity or payload", requests, effects)
	}
	e.reopen(t)
	must(t, workerDrainLocalReplay(e.store, worker.Provider, e.credentials["infrastructure"]))
}
func workerDrainLocalReplay(store *Store, provider *ReplayProvider, credential string) error {
	return (ReplayWorker{store, provider, credential, func(string, error) {}}).Drain(context.Background())
}

func TestRepairM_1020_ReplayUnavailableWithoutOwner(t *testing.T) {
	e := repairM1020EnvironmentFor(t, nil)
	ctx := context.Background()
	action, err := e.store.ProposeReplay(ctx, e.credentials["infrastructure"], ReplayProposal{"unwired", "service", "restore", time.Now().Add(time.Hour), ReplayPayload{"125", "1", "2", "candles", true}})
	must(t, err)
	action, err = e.store.Transition(ctx, e.credentials["operator2"], "unwired", "service", "review", "approved", action.Revision)
	must(t, err)
	status, raw := e.post(t, "infrastructure", "transition", TransitionRequest{"unwired", "service", "execute", "executing", &action.Revision})
	if status != 503 || !bytes.Contains(raw, []byte(Unavailable)) {
		t.Fatalf("%d %s", status, raw)
	}
	if got := repairM1020Read[Action](t, e.store, "action", "unwired"); got.State != "approved" || got.JobID != "" {
		t.Fatal(got)
	}
}

func TestRepairM_1020_IncidentDrainFairness(t *testing.T) {
	e := repairM1020EnvironmentFor(t, nil)
	ctx := context.Background()
	calls := map[string]int{}
	var mu sync.Mutex
	endpoint := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct{ Incident, Identity string }
		must(t, json.NewDecoder(r.Body).Decode(&input))
		mu.Lock()
		calls[input.Incident]++
		mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("v", 32) || r.Header.Get("Idempotency-Key") != input.Incident {
			http.Error(w, "denied", 403)
			return
		}
		if strings.HasPrefix(input.Incident, "b") {
			http.Error(w, "rotation unavailable", 503)
			return
		}
		must(t, json.NewEncoder(w).Encode(RevocationAcknowledgement{input.Incident, input.Identity, "rotation." + input.Incident, "integrity." + input.Incident, true, true}))
	}))
	defer endpoint.Close()
	provider, err := NewRevocationProvider(endpoint.URL, strings.Repeat("v", 32), repairM1020Transport(endpoint))
	must(t, err)
	worker := IncidentWorker{Store: e.store, Revocation: provider, Credential: e.credentials["infrastructure"], Observe: func(string, error) {}}
	detect := func(k string) string {
		actor := id()
		token := readySecret(t)
		must(t, e.authority.Grant(token, Principal{ID: actor, Role: "finance", CredentialID: id(), Scopes: []string{"service"}}, time.Now().Add(time.Hour)))
		must(t, e.store.DetectIncident(ctx, e.credentials["infrastructure"], "service", k, "detected compromise", actor, "retained-evidence"))
		return token
	}
	oldTokens := []string{}
	for n := 0; n < 110; n++ {
		oldTokens = append(oldTokens, detect(fmt.Sprintf("a%03d", n)))
	}
	must(t, worker.Drain(ctx))
	must(t, worker.Drain(ctx))
	for n, token := range oldTokens {
		_, err := e.store.identity.Resolve(ctx, token)
		is(t, err, Denied)
		if repairM1020Read[map[string]string](t, e.store, "incident_outbox", fmt.Sprintf("a%03d", n))["state"] != "completed" {
			t.Fatal("old work incomplete")
		}
	}
	for n := 0; n < 100; n++ {
		detect(fmt.Sprintf("b%03d", n))
	}
	lastToken := detect("c-later")
	must(t, worker.Drain(ctx))
	if repairM1020Read[map[string]string](t, e.store, "incident_outbox", "c-later")["state"] != "pending" {
		t.Fatal("batch ceiling was not exercised")
	}
	e.reopen(t)
	worker.Store = e.store
	must(t, worker.Drain(ctx))
	if repairM1020Read[map[string]string](t, e.store, "incident_outbox", "c-later")["state"] != "completed" {
		t.Fatal("old failures starved later identity")
	}
	_, err = e.store.identity.Resolve(ctx, lastToken)
	is(t, err, Denied)
	schedule := repairM1020Read[workSchedule](t, e.store, "work_schedule", "incident.b000")
	if schedule.Attempts != 1 || !schedule.Next.After(time.Now()) {
		t.Fatal("retry schedule not retained", schedule)
	}
	mu.Lock()
	defer mu.Unlock()
	for n := 0; n < 110; n++ {
		if calls[fmt.Sprintf("a%03d", n)] != 1 {
			t.Fatal("completed outbox called again")
		}
	}
	if calls["c-later"] != 1 || calls["b000"] != 1 {
		t.Fatal(calls)
	}
}

func TestRepairM_1020_SettlementDrainFairness(t *testing.T) {
	node := &cometNode{header: &types.Header{Number: big.NewInt(33963716), Difficulty: big.NewInt(0), GasLimit: 100000000, Time: 1, Extra: []byte{}}, balance: big.NewInt(100000)}
	node.receiptBlock = node.hash()
	verifier, err := NewChainVerifier(node.serve(t), custody, fixtureManifest, fixtureAuthority{}, fixtureAward{})
	must(t, err)
	e := repairM1020EnvironmentFor(t, verifier)
	ctx := context.Background()
	must(t, e.store.Reconcile(ctx, e.credentials["finance"], "platform", "observed inventory", usdl))
	for n := 0; n < 101; n++ {
		k := fmt.Sprintf("q%03d", n)
		must(t, e.store.CreateBounty(ctx, e.credentials["finance"], "service", "create funded obligation", bounty(k, usdl, "80")))
		for revision, next := range []string{"funded", "active", "submitted", "adjudicating", "awarded"} {
			must(t, e.store.BountyState(ctx, e.credentials["finance"], "service", k, "review obligation", next, "reviewed-artifact-sha256", beneficiary, uint64(revision+1)))
		}
	}
	calls := map[string]int{}
	submitted := 0
	var mu sync.Mutex
	reference := "0xab00000000000000000000000000000000000000000000000000000000000001"
	endpoint := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k := r.Header.Get("Idempotency-Key")
		mu.Lock()
		defer mu.Unlock()
		calls[k]++
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("p", 32) {
			http.Error(w, "denied", 403)
			return
		}
		if k != "bounty.q100" {
			must(t, json.NewEncoder(w).Encode(PayoutResult{State: "unknown"}))
			return
		}
		if r.Method == http.MethodGet {
			w.WriteHeader(404)
			return
		}
		var request struct {
			Key         string `json:"idempotencyKey"`
			Asset       Asset  `json:"asset"`
			Destination string `json:"destination"`
			Amount      string `json:"amountRaw"`
			Authority   string `json:"authorityReference"`
		}
		must(t, json.NewDecoder(r.Body).Decode(&request))
		if request.Key != k || request.Asset != usdl || request.Amount != "80" || request.Destination != beneficiary || request.Authority != custody {
			t.Error("payout payload mismatch")
			http.Error(w, "invalid", 400)
			return
		}
		submitted++
		must(t, json.NewEncoder(w).Encode(PayoutResult{"confirmed", reference}))
	}))
	defer endpoint.Close()
	provider, err := NewPayoutProvider(endpoint.URL, strings.Repeat("p", 32), "recorded payout authority", custody, repairM1020Transport(endpoint))
	must(t, err)
	worker, err := NewSettlementWorker(e.store, provider)
	must(t, err)
	observed := map[string]error{}
	observe := func(k string, err error) { observed[k] = err }
	must(t, worker.Drain(ctx, e.credentials["finance"], observe))
	if len(observed) != 100 {
		t.Fatal("batch bound", len(observed))
	}
	for _, err := range observed {
		is(t, err, Unavailable)
	}
	e.reopen(t)
	worker, err = NewSettlementWorker(e.store, provider)
	must(t, err)
	must(t, worker.Drain(ctx, e.credentials["finance"], observe))
	obligation := repairM1020Read[Obligation](t, e.store, "settlement", "bounty.q100")
	if obligation.State != "settled" || obligation.SettlementReference != reference || observed["bounty.q100"] != nil {
		t.Fatal("later payout starved", obligation, observed["bounty.q100"])
	}
	inventory := repairM1020Read[Inventory](t, e.store, "inventory", usdl.Key())
	if inventory.Settled != "80" || inventory.Pending != "8000" || inventory.Reserved != "8000" {
		t.Fatal(inventory)
	}
	if repairM1020Read[Obligation](t, e.store, "settlement", "bounty.q000").State != "unknown" {
		t.Fatal("uncertainty hidden")
	}
	schedule := repairM1020Read[workSchedule](t, e.store, "work_schedule", "settlement.bounty.q000")
	if schedule.Attempts != 1 || !schedule.Next.After(time.Now()) {
		t.Fatal(schedule)
	}
	mu.Lock()
	defer mu.Unlock()
	if submitted != 1 || calls["bounty.q000"] != 1 || calls["bounty.q100"] != 2 {
		t.Fatal(submitted, calls)
	}
}

func TestRepairM_1020_AuthenticatedRecoveryRestart(t *testing.T) {
	e := repairM1020EnvironmentFor(t, nil)
	ctx := context.Background()
	original := e.credentials["finance"]
	status, raw := e.post(t, "infrastructure", "incidents", IncidentRequest{"service", "recovery", "contain compromise", e.actors["finance"], "retained detection"})
	if status != 200 {
		t.Fatalf("detection %d %s", status, raw)
	}
	is(t, e.store.Access(ctx, original, "platform", "contained", "finance_liabilities", "", false), Denied)
	payload := map[string]string{"incident": "recovery", "rotation": "rotation.recovery", "integrity": "integrity.recovery"}
	action, err := e.store.Propose(ctx, e.credentials["infrastructure"], "recovery-approval", "incident_recovery", "service", "review evidence", digest(payload), time.Now().Add(time.Hour))
	must(t, err)
	_, err = e.store.Transition(ctx, e.credentials["operator2"], "recovery-approval", "service", "independent approval", "approved", action.Revision)
	must(t, err)
	revision := uint64(1)
	request := IncidentRecoveryRequest{TransitionRequest: TransitionRequest{Key: "recovery", Scope: "service", Reason: "verified recovery", Expected: &revision}, ApprovalKey: "recovery-approval", Rotation: payload["rotation"], Integrity: payload["integrity"]}
	status, _ = e.post(t, "infrastructure", "recover-incident", request)
	if status != 503 {
		t.Fatal("unverified recovery admitted", status)
	}
	endpoint := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct{ Incident, Identity string }
		must(t, json.NewDecoder(r.Body).Decode(&input))
		must(t, json.NewEncoder(w).Encode(RevocationAcknowledgement{input.Incident, input.Identity, payload["rotation"], payload["integrity"], true, true}))
	}))
	defer endpoint.Close()
	provider, err := NewRevocationProvider(endpoint.URL, strings.Repeat("v", 32), repairM1020Transport(endpoint))
	must(t, err)
	must(t, (IncidentWorker{Store: e.store, Revocation: provider, Credential: e.credentials["infrastructure"], Observe: func(string, error) {}}).Drain(ctx))
	_, err = e.store.identity.Resolve(ctx, original)
	is(t, err, Denied)
	replacement := readySecret(t)
	must(t, e.authority.Grant(replacement, Principal{ID: e.actors["finance"], Role: "finance", CredentialID: id(), Scopes: []string{"platform", "service"}}, time.Now().Add(time.Hour)))
	is(t, e.store.Access(ctx, replacement, "platform", "rotated but contained", "finance_liabilities", "", false), Denied)
	e.reopen(t)
	status, raw = e.post(t, "infrastructure", "recover-incident", request)
	if status != 200 {
		t.Fatalf("recovery %d %s", status, raw)
	}
	e.reopen(t)
	must(t, e.store.Access(ctx, replacement, "platform", "restored", "finance_liabilities", "", false))
	is(t, e.store.Access(ctx, e.credentials["support"], "platform", "unrelated denial", "finance_liabilities", "", false), Denied)
	_, err = e.store.Propose(ctx, replacement, "finance-no-infra", "replay", "service", "unrelated mutation", strings.Repeat("a", 64), time.Now().Add(time.Hour))
	is(t, err, Denied)
	if repairM1020Read[Incident](t, e.store, "incident", "recovery").State != "recovered" || repairM1020Read[Action](t, e.store, "action", "recovery-approval").State != "succeeded" {
		t.Fatal("restart lost verified recovery")
	}
	_, err = e.store.identity.Resolve(ctx, original)
	is(t, err, Denied)
	if _, ok := any(e.store.identity).(interface {
		Grant(string, Principal, time.Time) error
	}); ok {
		t.Fatal("runtime reader exposes setup writer")
	}
}

func TestRepairM_1020_DocumentedBoundaries(t *testing.T) {
	raw, err := os.ReadFile("README.md")
	must(t, err)
	for _, required := range []string{"python3 services/launchpad/security/control/check.py", "NewRuntime", "SettlementWorker.Process", "RevocationProvider", "unknown", "production", "work_schedule", "ReplayProvider"} {
		if !bytes.Contains(raw, []byte(required)) {
			t.Fatalf("missing lifecycle documentation: %s", required)
		}
	}
	record, err := os.ReadFile("repair_m1020.kvx")
	must(t, err)
	for _, finding := range []string{"control-readme-behavior", "replay-executor-missing", "registry-contract-pages-missing", "incident-drain-fairness", "settlement-drain-fairness", "workflow-production-role-proof"} {
		if !bytes.Contains(record, []byte("[finding."+finding+"]")) {
			t.Fatal("missing finding closure", finding)
		}
	}
	// Every local lifecycle reference resolves; foreign registry edits are an
	// explicit domain-17 wiring request, rather than fabricated contract pages.
	for _, path := range []string{"providers.go", "workers.go", "replay.go", "workflows.go", "provider-contract.md"} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRepairM_1020_RegistryContractPages(t *testing.T) {
	// The audit describes the older missing pages. They are present on this
	// task's base; qualify their current owning locations without rewriting rows
	// outside scope or claiming they were missing in this unchanged checkout.
	for _, path := range []string{"contracts/services.md", "contracts/clients.md", "contracts/operations.md", "contracts/launchpad.md", "contracts/explorer-frontend-config.md", "contracts/launchpad/README.md", "contracts/launchpad/operations/README.md"} {
		raw, err := os.ReadFile(filepath.Join("../../../..", path))
		must(t, err)
		if len(bytes.TrimSpace(raw)) == 0 {
			t.Fatal("empty owning contract", path)
		}
	}
	raw, err := os.ReadFile("../../../../spec/layerx-merge/interfaces.kvx")
	must(t, err)
	rows := 0
	for _, block := range strings.Split(string(raw), "\n[interface.") {
		if !strings.Contains(block, "services/launchpad/security/control/") {
			continue
		}
		if strings.Contains(block, `contract = "contracts/services.md#launchpad-security"`) {
			rows++
		}
	}
	if rows == 0 {
		t.Fatal("control registry identity lost")
	}
	page, err := os.ReadFile("../../../../contracts/services.md")
	must(t, err)
	if !bytes.Contains(page, []byte("## Launchpad security")) {
		t.Fatal("registered owner anchor missing")
	}
	lifecycle, err := os.ReadFile("provider-contract.md")
	must(t, err)
	for _, required := range []string{"Idempotency-Key", "authority_unavailable", "executor_result", "PayloadReference", "CompletionIdentity", "generation", "checkpoint", "Recovery"} {
		if !bytes.Contains(lifecycle, []byte(required)) {
			t.Fatal("provider lifecycle incomplete", required)
		}
	}
}

func TestRepairM_1020_ReplayRevokedApprovalCannotComplete(t *testing.T) {
	e := repairM1020EnvironmentFor(t, nil)
	ctx := context.Background()
	e.store.replay = repairM1020ReplayProvider(t, func(w http.ResponseWriter, r *http.Request) {
		var request ReplayExecution
		must(t, json.NewDecoder(r.Body).Decode(&request))
		must(t, e.authority.Reader().RevokeIdentity(ctx, e.actors["operator2"]))
		must(t, json.NewEncoder(w).Encode(ReplayResult{request.Job.ActionID, request.JobID, request.Job.PayloadHash, request.Job.PayloadReference, request.Job.CompletionIdentity, "succeeded", "retained-owner-checkpoint"}))
	})
	action, err := e.store.ProposeReplay(ctx, e.credentials["infrastructure"], ReplayProposal{"revoked-approval", "service", "restore", time.Now().Add(time.Hour), ReplayPayload{"125", "1", "2", "candles", true}})
	must(t, err)
	action, err = e.store.Transition(ctx, e.credentials["operator2"], "revoked-approval", "service", "approve", "approved", action.Revision)
	must(t, err)
	action, err = e.store.TransitionWorkflow(ctx, e.credentials["infrastructure"], TransitionRequest{"revoked-approval", "service", "execute", "executing", &action.Revision})
	must(t, err)
	worker := ReplayWorker{e.store, e.store.replay, e.credentials["infrastructure"], func(string, error) {}}
	is(t, worker.Process(ctx, "revoked-approval", "service"), Denied)
	if repairM1020Read[Action](t, e.store, "action", "revoked-approval").State != "executing" {
		t.Fatal("revoked approval completed action")
	}
	result := repairM1020Read[ReplayResult](t, e.store, "executor_result", action.JobID)
	if result.Reference != "retained-owner-checkpoint" {
		t.Fatal(result)
	}
	e.reopen(t)
	worker.Store = e.store
	is(t, worker.Process(ctx, "revoked-approval", "service"), Denied)
}

func TestRepairM_1020_LocalStoreRollbackAndExclusiveReopen(t *testing.T) {
	e := repairM1020EnvironmentFor(t, nil)
	ctx := context.Background()
	other, err := OpenLocalStore(e.path, e.authority.Reader(), nil)
	if err == nil {
		other.CloseLocal()
		t.Fatal("simultaneous store opened")
	}
	is(t, e.store.run(ctx, e.credentials["infrastructure"], "action", "service", "rejected mutation", func(tx *transaction) error {
		if err := tx.put("action", "rolled-back", Action{ID: id(), State: "proposed"}); err != nil {
			return err
		}
		return Conflict
	}), Conflict)
	var out Action
	is(t, e.store.readObject(ctx, "action", "rolled-back", &out), Missing)
	e.reopen(t)
	is(t, e.store.readObject(ctx, "action", "rolled-back", &out), Missing)
	snapshot, err := os.ReadFile(e.path)
	must(t, err)
	var state localSnapshot
	must(t, json.Unmarshal(snapshot, &state))
	if len(state.Audits) != 1 || state.Audits[0].Result != string(Conflict) {
		t.Fatal("denial audit missing", state.Audits)
	}
	if bytes.Contains(snapshot, []byte(e.credentials["infrastructure"])) {
		t.Fatal("credential persisted in domain state")
	}
}
