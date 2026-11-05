package control

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func repair65Get(t *testing.T, e *phase2Environment, role, path string, out any) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, e.server.URL+"/v2/control/"+path, nil)
	must(t, err)
	if role != "" {
		req.Header.Set("Authorization", "Bearer "+e.credentials[role])
	}
	res, err := e.server.Client().Do(req)
	must(t, err)
	defer res.Body.Close()
	if out != nil {
		must(t, json.NewDecoder(res.Body).Decode(out))
	}
	return res.StatusCode
}

func TestRepair_65_PolicyWriteReadShape(t *testing.T) {
	e := phase2EnvironmentFor(t)
	ctx := context.Background()
	owner := e.actors["user"]
	for _, asset := range []Asset{usdl, pax} {
		p := Policy{ID: id(), Owner: owner, Mode: "disclosed_assistant", Disclosure: "Disclosed recommendation without execution", Asset: asset, Instruments: []string{USDLAddress, beneficiary}, Requests: 3, Spend: "1000000", QuoteAge: time.Minute, Expires: time.Now().Add(time.Hour)}
		status, raw := e.post(t, "user", "policies", PolicyRequest{Reason: "create public campaign", Policy: p})
		if status != http.StatusOK {
			t.Fatalf("POST policies: %d %s", status, raw)
		}
		var envelope struct {
			Data CampaignView `json:"data"`
		}
		if status = repair65Get(t, e, "user", "campaigns/"+p.ID, &envelope); status != http.StatusOK {
			t.Fatalf("GET campaign: %d", status)
		}
		v := envelope.Data
		if v.CampaignID != p.ID || v.AccountID != owner || !reflect.DeepEqual(v.AssetAllowlist, p.Instruments) || v.BudgetRaw != p.Spend || v.State != "draft" || v.ExpiresAt != p.Expires.UTC().Format("2006-01-02T15:04:05.000Z") {
			t.Fatalf("campaign failed round trip: %+v", v)
		}
		expectedAsset, err := wireAsset(asset)
		must(t, err)
		if !reflect.DeepEqual(v.BudgetAsset, expectedAsset) {
			t.Fatalf("asset encoding: %+v", v.BudgetAsset)
		}
		if status = repair65Get(t, e, "", "campaigns/"+p.ID, nil); status != http.StatusUnauthorized {
			t.Fatalf("anonymous GET: %d", status)
		}
		if status = repair65Get(t, e, "support", "campaigns/"+p.ID, nil); status != http.StatusForbidden {
			t.Fatalf("foreign GET: %d", status)
		}
		must(t, e.store.SetBudget(ctx, e.credentials["user"], owner, "explicit owner budget", Budget{Owner: owner, Asset: asset, Requests: 3, Spend: p.Spend, Expires: time.Now().Add(time.Hour)}, 0))
		must(t, e.store.PolicyState(ctx, e.credentials["user"], owner, p.ID, "enable recommendations", "enabled", 1))
		requestKey := "recommendation." + asset.Key()
		status, raw = e.post(t, "user", "recommendations", RecommendationRequest{PolicyID: p.ID, Key: requestKey, Instrument: beneficiary, Reason: "request disclosed recommendation", QuoteAt: time.Now().Add(-time.Second)})
		if status != http.StatusOK {
			t.Fatalf("POST recommendation: %d %s", status, raw)
		}
		var recommendation struct {
			Data RecommendationView `json:"data"`
		}
		if status = repair65Get(t, e, "user", "recommendations/"+requestKey, &recommendation); status != http.StatusOK {
			t.Fatalf("GET recommendation: %d", status)
		}
		if recommendation.Data.Campaign.CampaignID != p.ID || recommendation.Data.Campaign.AccountID != owner || !reflect.DeepEqual(recommendation.Data.Campaign.AssetAllowlist, p.Instruments) || recommendation.Data.Reservation.Instrument != beneficiary || recommendation.Data.Campaign.State != "blocked" {
			t.Fatalf("recommendation failed round trip: %+v", recommendation.Data)
		}
	}
}

func TestRepair_65_PolicyRejectsMalformedBeforeCommit(t *testing.T) {
	e := phase2EnvironmentFor(t)
	ctx := context.Background()
	cases := []struct {
		name   string
		mutate func(*Policy)
	}{
		{"key policy ID", func(p *Policy) { p.ID = "bot1" }},
		{"invalid UUID version", func(p *Policy) { p.ID = "00000000-0000-0000-8000-000000000001" }},
		{"uppercase UUID", func(p *Policy) { p.ID = "AAAAAAAA-0000-4000-8000-000000000001" }},
		{"key account ID", func(p *Policy) { p.Owner = "owner" }},
		{"foreign account", func(p *Policy) { p.Owner = id() }},
		{"key instrument", func(p *Policy) { p.Instruments = []string{"pool1"} }},
		{"zero address", func(p *Policy) { p.Instruments = []string{"0x0000000000000000000000000000000000000000"} }},
		{"uppercase address", func(p *Policy) { p.Instruments = []string{strings.ToUpper(USDLAddress)} }},
		{"duplicate instrument", func(p *Policy) { p.Instruments = []string{USDLAddress, USDLAddress} }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			p := Policy{ID: id(), Owner: e.actors["user"], Mode: "disclosed_assistant", Disclosure: "Disclosed automation", Asset: usdl, Instruments: []string{USDLAddress}, Requests: 1, Spend: "1", QuoteAge: time.Minute, Expires: time.Now().Add(time.Hour)}
			test.mutate(&p)
			status, raw := e.post(t, "user", "policies", PolicyRequest{Reason: "validate before write", Policy: p})
			if status != http.StatusBadRequest {
				t.Fatalf("POST malformed policy: %d %s", status, raw)
			}
			var count int
			must(t, e.store.pool.QueryRow(ctx, "SELECT count(*) FROM "+e.store.schema+".objects WHERE kind='policy'").Scan(&count))
			if count != 0 {
				t.Fatal("invalid policy committed")
			}
		})
	}
	// Exercise the owning Store API with a non-UUID identity as well as HTTP's
	// independently resolved UUID identity.
	p := policy(id())
	is(t, e.store.CreatePolicy(ctx, e.credentials["user"], automationOwner, "wrong actor", p), Denied)
	p.Owner = "owner"
	is(t, e.store.CreatePolicy(ctx, e.credentials["user"], "owner", "malformed owner", p), Invalid)
}

func repair65RuntimeStore(t *testing.T, e *phase2Environment) *Store {
	t.Helper()
	ctx := context.Background()
	role := "repair65_runtime_" + strings.ReplaceAll(id(), "-", "")[:12]
	quotedRole := pgx.Identifier{role}.Sanitize()
	secret := readySecret(t)
	_, err := e.store.pool.Exec(ctx, "CREATE ROLE "+quotedRole+" LOGIN PASSWORD '"+secret+"' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS")
	must(t, err)
	// Read the production manifest instead of granting a test superuser or DELETE.
	root := os.Getenv("L47_SOURCE_ROOT")
	if root == "" {
		root, err = filepath.Abs("../../../..")
		must(t, err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "launchpad/spec/production-system/integration/migrations/grants.json"))
	must(t, err)
	var manifest struct {
		Roles []struct {
			RuntimeRole string `json:"runtime_role"`
			Grants      []struct {
				Schema              string `json:"schema"`
				Object              string `json:"object"`
				ObjectKind          string `json:"object_kind"`
				Privileges, Columns []string
			} `json:"grants"`
		} `json:"roles"`
	}
	must(t, json.Unmarshal(raw, &manifest))
	found := false
	for _, entry := range manifest.Roles {
		if entry.RuntimeRole != "control_runtime" {
			continue
		}
		found = true
		for _, grant := range entry.Grants {
			if grant.Schema != "pnl" || grant.ObjectKind != "table" {
				continue
			}
			privileges := strings.Join(grant.Privileges, ",")
			if len(grant.Columns) > 0 {
				columns := make([]string, len(grant.Columns))
				for n, c := range grant.Columns {
					columns[n] = pgx.Identifier{c}.Sanitize()
				}
				privileges += " (" + strings.Join(columns, ",") + ")"
			}
			_, err = e.store.pool.Exec(ctx, "GRANT "+privileges+" ON "+e.store.schema+"."+pgx.Identifier{grant.Object}.Sanitize()+" TO "+quotedRole)
			must(t, err)
		}
	}
	if !found {
		t.Fatal("control_runtime grants missing")
	}
	_, err = e.store.pool.Exec(ctx, "GRANT USAGE ON SCHEMA "+e.store.schema+" TO "+quotedRole+"; GRANT USAGE ON SEQUENCE "+e.store.schema+".audit_seq_seq TO "+quotedRole)
	must(t, err)
	t.Cleanup(func() {
		_, err := e.store.pool.Exec(context.Background(), "DROP OWNED BY "+quotedRole+"; DROP ROLE "+quotedRole)
		if err != nil {
			t.Error(err)
		}
	})
	pool := readyPool(t, ctx, role, secret)
	var canDelete, canUpdateID, canUpdateBody bool
	must(t, pool.QueryRow(ctx, "SELECT has_table_privilege(current_user,$1,'DELETE'),has_column_privilege(current_user,$1,'id','UPDATE'),has_column_privilege(current_user,$1,'body','UPDATE')", e.store.schema+".objects").Scan(&canDelete, &canUpdateID, &canUpdateBody))
	if canDelete || canUpdateID || !canUpdateBody {
		t.Fatalf("runtime object grants: DELETE=%v UPDATE(id)=%v UPDATE(body)=%v", canDelete, canUpdateID, canUpdateBody)
	}
	store, err := New(pool, strings.Trim(e.store.schema, "\""), e.identity, nil)
	must(t, err)
	return store
}

func repair65RecoveryApproval(t *testing.T, e *phase2Environment, incident, approval string) {
	t.Helper()
	payload := map[string]string{"incident": incident, "rotation": "rotation." + incident, "integrity": "integrity." + incident}
	a, err := e.store.Propose(context.Background(), e.credentials["infrastructure"], approval, "incident_recovery", "service", "review independent recovery evidence", digest(payload), time.Now().Add(time.Hour))
	must(t, err)
	_, err = e.store.Transition(context.Background(), e.credentials["operator2"], approval, "service", "approve exact recovery payload", "approved", a.Revision)
	must(t, err)
}

func repair65Recover(t *testing.T, e *phase2Environment, incident, approval string) error {
	t.Helper()
	revision := uint64(1)
	status, raw := e.post(t, "infrastructure", "recover-incident", IncidentRecoveryRequest{TransitionRequest: TransitionRequest{Scope: "service", Key: incident, Reason: "recover verified current quarantine", Expected: &revision}, ApprovalKey: approval, Rotation: "rotation." + incident, Integrity: "integrity." + incident})
	if status == http.StatusOK {
		return nil
	}
	var result WorkflowError
	must(t, json.Unmarshal(raw, &result))
	return Fault(result.Rule)
}

func repair65RecordRecoveryEvidence(t *testing.T, e *phase2Environment, incident, identity string) {
	t.Helper()
	// Independent adapter evidence lives in the real durable outbox consumed by
	// IncidentRecovery. Credentials are actually revoked by IncidentWorker first.
	worker := IncidentWorker{Store: e.store, Identity: e.identity, Credential: e.credentials["infrastructure"], Observe: func(_ string, err error) {
		if err != nil && err != Unavailable {
			t.Error(err)
		}
	}}
	must(t, worker.Drain(context.Background()))
	raw, err := json.Marshal(map[string]string{"state": "completed", "identity": identity, "rotation": "rotation." + incident, "integrity": "integrity." + incident})
	must(t, err)
	_, err = e.store.pool.Exec(context.Background(), "UPDATE "+e.store.schema+".objects SET body=$1 WHERE kind='incident_outbox' AND id=$2", raw, incident)
	must(t, err)
}

func TestRepair_65_RecoveryWithProductionGrants(t *testing.T) {
	e := phase2EnvironmentFor(t)
	ctx := context.Background()
	actor := e.actors["finance"]
	original := e.credentials["finance"]
	runtimeStore := repair65RuntimeStore(t, e)
	service, err := NewHTTPService(runtimeStore, nil)
	must(t, err)
	e.server.Close()
	e.server = httptest.NewServer(service.Handler())
	t.Cleanup(e.server.Close)
	must(t, runtimeStore.DetectIncident(ctx, e.credentials["infrastructure"], "service", "incident65", "contain compromised service", actor, "retained-detection"))
	is(t, runtimeStore.Access(ctx, original, "platform", "blocked operation", "finance_liabilities", "", false), Denied)
	repair65RecoveryApproval(t, e, "incident65", "approval65")
	is(t, repair65Recover(t, e, "incident65", "approval65"), Unavailable)
	if load[Action](t, e.store, "action", "approval65").State != "approved" {
		t.Fatal("unverified recovery consumed approval")
	}
	repair65RecordRecoveryEvidence(t, e, "incident65", actor)
	// Rotate to a real fresh credential for the same service identity.
	replacement := readySecret(t)
	hash := sha256.Sum256([]byte(replacement))
	_, err = e.store.pool.Exec(ctx, "INSERT INTO "+e.store.schema+".authority_credentials(credential_id,actor_id,role,scopes,token_hash,expires_at) VALUES($1,$2,'finance',$3,$4,clock_timestamp()+interval '15 minutes')", id(), actor, []string{"platform", "service", actor}, hash[:])
	must(t, err)
	is(t, runtimeStore.Access(ctx, replacement, "platform", "still contained after rotation", "finance_liabilities", "", false), Denied)
	must(t, repair65Recover(t, e, "incident65", "approval65"))
	marker := load[map[string]string](t, e.store, "quarantine", actor)
	incident := load[Incident](t, e.store, "incident", "incident65")
	if marker["incident"] != "incident65" || marker["state"] != "recovered" || incident.State != "recovered" || incident.Revision != 2 || incident.DetectionReference != "retained-detection" || incident.RotationReference != "rotation.incident65" || incident.IntegrityReference != "integrity.incident65" || load[Action](t, e.store, "action", "approval65").State != "succeeded" {
		t.Fatalf("recovery lost history: %+v %+v", marker, incident)
	}
	must(t, runtimeStore.Access(ctx, replacement, "platform", "restored operation", "finance_liabilities", "", false))
	_, err = e.identity.Resolve(ctx, original)
	is(t, err, Denied)
	// Recovered identities can also approve again; approvalActive must use the
	// same recovered-marker semantics as authorization.
	infraActor := e.actors["operator2"]
	must(t, runtimeStore.DetectIncident(ctx, e.credentials["infrastructure"], "service", "approver65", "contain approver", infraActor, "retained-approver-detection"))
	// Recovery of this approver needs another independently active approver.
	thirdToken := readySecret(t)
	thirdHash := sha256.Sum256([]byte(thirdToken))
	_, err = e.store.pool.Exec(ctx, "INSERT INTO "+e.store.schema+".authority_credentials(credential_id,actor_id,role,scopes,token_hash,expires_at) VALUES($1,$2,'infrastructure',$3,$4,clock_timestamp()+interval '15 minutes')", id(), id(), []string{"service", "platform"}, thirdHash[:])
	must(t, err)
	oldApprover := e.credentials["operator2"]
	e.credentials["operator2"] = thirdToken
	repair65RecoveryApproval(t, e, "approver65", "approver-recovery65")
	repair65RecordRecoveryEvidence(t, e, "approver65", infraActor)
	rotatedApprover := readySecret(t)
	rotatedHash := sha256.Sum256([]byte(rotatedApprover))
	_, err = e.store.pool.Exec(ctx, "INSERT INTO "+e.store.schema+".authority_credentials(credential_id,actor_id,role,scopes,token_hash,expires_at) VALUES($1,$2,'infrastructure',$3,$4,clock_timestamp()+interval '15 minutes')", id(), infraActor, []string{"platform", "service", infraActor}, rotatedHash[:])
	must(t, err)
	must(t, repair65Recover(t, e, "approver65", "approver-recovery65"))
	e.credentials["operator2"] = rotatedApprover
	a, err := runtimeStore.Propose(ctx, e.credentials["infrastructure"], "recovered-approver-action", "bulk_export", "platform", "review export", digest(map[string]string{"dataset": "finance_liabilities", "scope": "platform"}), time.Now().Add(time.Hour))
	must(t, err)
	_, err = runtimeStore.Transition(ctx, rotatedApprover, "recovered-approver-action", "platform", "approve after verified recovery", "approved", a.Revision)
	must(t, err)
	must(t, runtimeStore.Access(ctx, replacement, "platform", "consume recovered approval", "finance_liabilities", "recovered-approver-action", true))
	_, err = e.identity.Resolve(ctx, oldApprover)
	is(t, err, Denied)
	// A new quarantine takes effect immediately, while both recovered incidents
	// and their verified references remain available as history.
	must(t, runtimeStore.DetectIncident(ctx, e.credentials["infrastructure"], "service", "newer65", "new compromise", actor, "newer-detection"))
	is(t, runtimeStore.Access(ctx, replacement, "platform", "new quarantine blocks operation", "finance_liabilities", "", false), Denied)
	if load[Incident](t, e.store, "incident", "incident65").State != "recovered" || load[map[string]string](t, e.store, "quarantine", actor)["incident"] != "newer65" {
		t.Fatal("new containment erased recovery history")
	}
}

func TestRepair_65_RecoveryPreservesNewerQuarantine(t *testing.T) {
	e := phase2EnvironmentFor(t)
	ctx := context.Background()
	runtimeStore := repair65RuntimeStore(t, e)
	actor := e.actors["finance"]
	must(t, runtimeStore.DetectIncident(ctx, e.credentials["infrastructure"], "service", "older65", "first containment", actor, "first-detection"))
	repair65RecoveryApproval(t, e, "older65", "older-approval65")
	repair65RecordRecoveryEvidence(t, e, "older65", actor)
	must(t, runtimeStore.DetectIncident(ctx, e.credentials["infrastructure"], "service", "latest65", "second containment", actor, "second-detection"))
	is(t, runtimeStore.RecoverIncident(ctx, e.credentials["infrastructure"], "service", "older65", "stale recovery", "older-approval65", "rotation.older65", "integrity.older65", 1, IncidentRecovery{runtimeStore}), Conflict)
	if load[Incident](t, e.store, "incident", "older65").State != "contained" || load[Action](t, e.store, "action", "older-approval65").State != "approved" {
		t.Fatal("stale recovery changed history or consumed approval")
	}
	marker := load[map[string]string](t, e.store, "quarantine", actor)
	if marker["incident"] != "latest65" || marker["state"] != "contained" {
		t.Fatalf("stale recovery cleared newer quarantine: %+v", marker)
	}
}

// This local provider accepts the real PayoutProvider HTTP request, persists its
// key in PostgreSQL and executes a real Anvil transfer. Its responses are derived
// from the transaction receipt; it does not manufacture settlement evidence.
type repair65PayoutEndpoint struct {
	store                   *Store
	chain                   *ethclient.Client
	obligation              Obligation
	token, boundary         string
	sent, release, accepted chan struct{}
	puts                    atomic.Int32
}

func (p *repair65PayoutEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+p.token || r.Header.Get("Idempotency-Key") != p.obligation.IdempotencyKey || r.URL.Path != "/v1/settlements/"+p.obligation.IdempotencyKey {
		http.Error(w, "unauthorized payout request", http.StatusForbidden)
		return
	}
	result, err := p.handle(r)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "repair65 payout endpoint:", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (p *repair65PayoutEndpoint) handle(r *http.Request) (PayoutResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var result PayoutResult
	if r.Method == http.MethodGet {
		var raw []byte
		if err := p.store.pool.QueryRow(ctx, "SELECT body FROM "+p.store.schema+".objects WHERE kind='provider_settlement' AND id=$1", p.obligation.IdempotencyKey).Scan(&raw); err != nil {
			return result, err
		}
		err := json.Unmarshal(raw, &result)
		return result, err
	}
	if r.Method != http.MethodPut {
		return result, Invalid
	}
	p.puts.Add(1)
	var request struct {
		Key         string `json:"idempotencyKey"`
		Asset       Asset  `json:"asset"`
		Destination string `json:"destination"`
		Amount      string `json:"amountRaw"`
		Authority   string `json:"authorityReference"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return result, err
	}
	o := p.obligation
	if request.Key != o.IdempotencyKey || request.Asset != o.Asset || request.Destination != o.Destination || request.Amount != o.Amount || request.Authority != o.AuthorityReference {
		return result, Denied
	}
	if p.boundary == "send" && p.puts.Load() == 1 {
		close(p.sent)
		<-p.release
	}
	tx, err := p.store.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "provider:"+request.Key); err != nil {
		return result, err
	}
	var raw []byte
	err = tx.QueryRow(ctx, "SELECT body FROM "+p.store.schema+".objects WHERE kind='provider_settlement' AND id=$1", request.Key).Scan(&raw)
	if err == nil {
		err = json.Unmarshal(raw, &result)
		return result, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	value, err := amount(request.Amount)
	if err != nil {
		return result, err
	}
	var hash common.Hash
	if err = p.chain.Client().CallContext(ctx, &hash, "eth_sendTransaction", map[string]string{"from": request.Authority, "to": request.Destination, "value": "0x" + value.Text(16)}); err != nil {
		return result, err
	}
	receipt, err := p.chain.TransactionReceipt(ctx, hash)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for errors.Is(err, ethereum.NotFound) {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-ticker.C:
			receipt, err = p.chain.TransactionReceipt(ctx, hash)
		}
	}
	if err != nil {
		return result, err
	}
	if receipt.Status != 1 {
		return result, Denied
	}
	result = PayoutResult{State: "confirmed", Reference: hash.Hex()}
	raw, err = json.Marshal(result)
	if err != nil {
		return result, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO "+p.store.schema+".objects(kind,id,body) VALUES('provider_settlement',$1,$2)", request.Key, raw); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	close(p.accepted)
	return result, nil
}

func repair65Transport(cert *x509.Certificate) *http.Transport {
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	return &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}, DisableKeepAlives: true}
}

// TestMain dispatches only the dedicated worker subprocess here, so killing the
// worker leaves the parent's real PostgreSQL, TLS provider and chain running.
func repair65RunWorker() error {
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(os.Getenv("L47_REPAIR65_DSN"))
	if err != nil {
		return err
	}
	config.ConnConfig.RuntimeParams["application_name"] = os.Getenv("L47_REPAIR65_APP")
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return err
	}
	defer pool.Close()
	schema := os.Getenv("L47_REPAIR65_SCHEMA")
	identity, err := NewPostgresIdentity(pool, schema, schema)
	if err != nil {
		return err
	}
	client, err := ethclient.DialContext(ctx, os.Getenv("L47_ANVIL_URL"))
	if err != nil {
		return err
	}
	defer client.Close()
	verifier, err := NewChainVerifier(client, custody, fixtureManifest, fixtureAuthority{}, fixtureAward{})
	if err != nil {
		return err
	}
	store, err := New(pool, schema, identity, verifier)
	if err != nil {
		return err
	}
	certificate, err := os.ReadFile(os.Getenv("L47_REPAIR65_CA"))
	if err != nil {
		return err
	}
	block, _ := pem.Decode(certificate)
	if block == nil {
		return fmt.Errorf("missing provider certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	transport := repair65Transport(cert)
	if os.Getenv("L47_REPAIR65_BOUNDARY") == "intent" {
		var dials atomic.Int32
		transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			if dials.Add(1) == 2 {
				if err := os.WriteFile(os.Getenv("L47_REPAIR65_MARKER"), []byte("intent committed before submission"), 0600); err != nil {
					return nil, err
				}
				<-ctx.Done()
				return nil, ctx.Err()
			}
			return (&net.Dialer{}).DialContext(ctx, network, address)
		}
	}
	provider, err := NewPayoutProvider(os.Getenv("L47_REPAIR65_URL"), os.Getenv("L47_REPAIR65_TOKEN"), "isolated real payout endpoint", custody, transport)
	if err != nil {
		return err
	}
	worker, err := NewSettlementWorker(store, provider)
	if err != nil {
		return err
	}
	_, err = worker.Process(ctx, os.Getenv("L47_REPAIR65_CREDENTIAL"), "service", os.Getenv("L47_REPAIR65_KEY"), "process durable payout in subprocess")
	return fmt.Errorf("worker returned before process-loss boundary: %v", err)
}

func repair65Eventually(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("real worker did not reach the required process-loss boundary")
}

func repair65Reopen(t *testing.T, schema string) *Store {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), fixtureDSN)
	must(t, err)
	t.Cleanup(pool.Close)
	identity, err := NewPostgresIdentity(pool, strings.Trim(schema, "\""), strings.Trim(schema, "\""))
	must(t, err)
	store, err := New(pool, strings.Trim(schema, "\""), identity, chain(t, fixtureAward{}))
	must(t, err)
	return store
}

func TestRepair_65_RoutingRestart(t *testing.T) {
	for _, boundary := range []string{"intent", "send", "response"} {
		t.Run(boundary, func(t *testing.T) {
			ctx := context.Background()
			e := phase2EnvironmentFor(t)
			verifier := chain(t, fixtureAward{})
			setBalance(t, verifier, "100000000000000000000")
			e.store.verifier = verifier
			must(t, e.store.Reconcile(ctx, e.credentials["finance"], "platform", "confirmed funding inventory", pax))
			bountyID := id()
			must(t, e.store.CreateBounty(ctx, e.credentials["finance"], "service", "create funded payout", bounty(bountyID, pax, "1000000")))
			for n, next := range []string{"funded", "active", "submitted", "adjudicating", "awarded"} {
				must(t, e.store.BountyState(ctx, e.credentials["finance"], "service", bountyID, "accept payout obligation", next, "reviewed-artifact-sha256", beneficiary, uint64(n+1)))
			}
			key := "bounty." + bountyID
			original := load[Obligation](t, e.store, "settlement", key)
			nonceBefore, err := verifier.client.NonceAt(ctx, common.HexToAddress(custody), nil)
			must(t, err)
			endpoint := &repair65PayoutEndpoint{store: e.store, chain: verifier.client, obligation: original, token: readySecret(t), boundary: boundary, sent: make(chan struct{}), release: make(chan struct{}), accepted: make(chan struct{})}
			server := httptest.NewTLSServer(endpoint)
			t.Cleanup(server.Close)
			temp := t.TempDir()
			ca := filepath.Join(temp, "provider.pem")
			must(t, os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600))
			marker := filepath.Join(temp, "intent")
			app := "repair65_" + strings.ReplaceAll(id(), "-", "")
			// A real PostgreSQL lock pauses the final response write, after provider
			// acceptance, without inserting any synthetic payout_attempt state.
			var lock *pgxpool.Conn
			if boundary == "response" {
				lock, err = e.store.pool.Acquire(ctx)
				must(t, err)
				_, err = lock.Exec(ctx, "SELECT pg_advisory_lock(226565)")
				must(t, err)
				_, err = e.store.pool.Exec(ctx, "CREATE FUNCTION "+e.store.schema+".pause_response65() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.kind='payout_attempt' AND NEW.body->>'Claim'='' AND NEW.body->>'Reference'<>'' THEN PERFORM pg_advisory_xact_lock(226565); END IF; RETURN NEW; END $$; CREATE TRIGGER pause_response65 BEFORE UPDATE ON "+e.store.schema+".objects FOR EACH ROW EXECUTE FUNCTION "+e.store.schema+".pause_response65()")
				must(t, err)
			}
			output, err := os.Create(filepath.Join(temp, "worker.log"))
			must(t, err)
			defer output.Close()
			process := exec.Command(os.Args[0])
			process.Env = append(os.Environ(),
				"L47_REPAIR65_WORKER=1", "L47_REPAIR65_DSN="+fixtureDSN,
				"L47_REPAIR65_SCHEMA="+strings.Trim(e.store.schema, "\""),
				"L47_REPAIR65_APP="+app, "L47_REPAIR65_CA="+ca,
				"L47_REPAIR65_URL="+server.URL, "L47_REPAIR65_TOKEN="+endpoint.token,
				"L47_REPAIR65_CREDENTIAL="+e.credentials["finance"], "L47_REPAIR65_KEY="+key,
				"L47_REPAIR65_BOUNDARY="+boundary, "L47_REPAIR65_MARKER="+marker)
			process.Stdout = output
			process.Stderr = output
			must(t, process.Start())
			t.Cleanup(func() {
				if process.ProcessState == nil {
					_ = process.Process.Kill()
					_ = process.Wait()
				}
				if lock != nil {
					_, _ = lock.Exec(context.Background(), "SELECT pg_advisory_unlock(226565)")
					lock.Release()
				}
				select {
				case <-endpoint.release:
				default:
					close(endpoint.release)
				}
				if t.Failed() {
					raw, _ := os.ReadFile(output.Name())
					t.Logf("worker evidence: %s", raw)
				}
			})
			switch boundary {
			case "intent":
				repair65Eventually(t, func() bool { _, err := os.Stat(marker); return err == nil })
			case "send":
				repair65Eventually(t, func() bool {
					select {
					case <-endpoint.sent:
						return true
					default:
						return false
					}
				})
			case "response":
				repair65Eventually(t, func() bool {
					var waiting bool
					err := e.store.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event='advisory')", app).Scan(&waiting)
					return err == nil && waiting
				})
			}
			must(t, process.Process.Kill())
			err = process.Wait()
			var exited *exec.ExitError
			if !errors.As(err, &exited) || exited.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
				t.Fatalf("worker was not stopped by process loss: %v", err)
			}
			if lock != nil {
				_, err = lock.Exec(ctx, "SELECT pg_advisory_unlock(226565)")
				must(t, err)
				lock.Release()
				lock = nil
				_, err = e.store.pool.Exec(ctx, "DROP TRIGGER pause_response65 ON "+e.store.schema+".objects; DROP FUNCTION "+e.store.schema+".pause_response65()")
				must(t, err)
			}
			if boundary == "send" {
				close(endpoint.release)
			}
			if boundary != "intent" {
				repair65Eventually(t, func() bool {
					select {
					case <-endpoint.accepted:
						return true
					default:
						return false
					}
				})
			}
			attempt := load[payoutAttempt](t, e.store, "payout_attempt", key)
			if !attempt.Submitted || attempt.Accepted || attempt.Claim == "" || attempt.State != "unknown" {
				t.Fatalf("durable intent confused with acceptance: %+v", attempt)
			}
			if got := load[Obligation](t, e.store, "settlement", key); got.IdempotencyKey != original.IdempotencyKey || got.State == "settled" {
				t.Fatalf("crash lost obligation: %+v", got)
			}
			// The old process owns the lease until its original durable expiry. Do not
			// edit or backdate that state to manufacture restart eligibility.
			wait := time.Until(attempt.Until) + 10*time.Millisecond
			if wait > 0 {
				time.Sleep(wait)
			}
			reopened := repair65Reopen(t, e.store.schema)
			provider, err := NewPayoutProvider(server.URL, endpoint.token, "isolated real payout endpoint", custody, repair65Transport(server.Certificate()))
			must(t, err)
			// Provider acceptance alone cannot release inventory: the independently
			// configured chain reader is actually disconnected for this first recovery.
			reopened.verifier.(*ChainVerifier).client.Close()
			worker, err := NewSettlementWorker(reopened, provider)
			must(t, err)
			_, err = worker.Process(ctx, e.credentials["finance"], "service", key, "recover with unavailable independent receipt reader")
			must(t, err)
			attempt = load[payoutAttempt](t, e.store, "payout_attempt", key)
			if !attempt.Accepted || attempt.Reference == "" {
				t.Fatalf("provider acceptance was not durable: %+v", attempt)
			}
			if got := load[Obligation](t, e.store, "settlement", key); got.State != "unknown" || got.IdempotencyKey != key {
				t.Fatalf("provider response bypassed independent verification: %+v", got)
			}
			inventory := load[Inventory](t, e.store, "inventory", "pax")
			if inventory.Reserved != "1000000" || inventory.Pending != "1000000" || inventory.Settled != "0" {
				t.Fatalf("unverified payout released inventory: %+v", inventory)
			}
			reopened.pool.Close()
			recovered := repair65Reopen(t, e.store.schema)
			worker, err = NewSettlementWorker(recovered, provider)
			must(t, err)
			settled, err := worker.Process(ctx, e.credentials["finance"], "service", key, "recover and independently verify receipt")
			must(t, err)
			if settled.State != "settled" || settled.SettlementReference != attempt.Reference || settled.IdempotencyKey != key {
				t.Fatalf("restart failed to settle original obligation: %+v", settled)
			}
			for n := 0; n < 3; n++ {
				again, err := worker.Process(ctx, e.credentials["finance"], "service", key, "repeat recovered payout")
				must(t, err)
				if again.SettlementReference != settled.SettlementReference {
					t.Fatal("retry changed receipt")
				}
			}
			nonceAfter, err := verifier.client.NonceAt(ctx, common.HexToAddress(custody), nil)
			must(t, err)
			if nonceAfter != nonceBefore+1 || endpoint.puts.Load() != 1 {
				t.Fatalf("duplicate or missing payout: nonce %d -> %d, PUTs %d", nonceBefore, nonceAfter, endpoint.puts.Load())
			}
			var receipts, providerKeys int
			must(t, e.store.pool.QueryRow(ctx, "SELECT count(*) FILTER (WHERE kind='receipt'),count(*) FILTER (WHERE kind='provider_settlement') FROM "+e.store.schema+".objects").Scan(&receipts, &providerKeys))
			if receipts != 1 || providerKeys != 1 {
				t.Fatalf("durable receipt/key count: %d/%d", receipts, providerKeys)
			}
			inventory = load[Inventory](t, e.store, "inventory", "pax")
			if inventory.Reserved != "0" || inventory.Pending != "0" || inventory.Settled != "1000000" {
				t.Fatalf("verified payout accounting: %+v", inventory)
			}
		})
	}
}

func TestRepair_65_RecoveryMarkerFailsClosed(t *testing.T) {
	e := phase2EnvironmentFor(t)
	ctx := context.Background()
	actor := e.actors["finance"]
	runtimeStore := repair65RuntimeStore(t, e)
	must(t, runtimeStore.DetectIncident(ctx, e.credentials["infrastructure"], "service", "unverified65", "contain identity", actor, "retained-detection"))
	for _, incident := range []string{"unverified65", "missing65"} {
		raw, err := json.Marshal(map[string]string{"incident": incident, "state": "recovered"})
		must(t, err)
		_, err = e.store.pool.Exec(ctx, "UPDATE "+e.store.schema+".objects SET body=$1 WHERE kind='quarantine' AND id=$2", raw, actor)
		must(t, err)
		is(t, runtimeStore.Access(ctx, e.credentials["finance"], "platform", "marker alone cannot authorize", "finance_liabilities", "", false), Denied)
	}
	if load[Incident](t, e.store, "incident", "unverified65").State != "contained" {
		t.Fatal("unverified marker changed incident history")
	}
}
