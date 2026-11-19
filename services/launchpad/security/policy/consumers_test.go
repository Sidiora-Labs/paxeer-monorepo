package policy

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/security/control"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type consumerLog struct {
	mu sync.Mutex
	bytes.Buffer
}

func (l *consumerLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Buffer.Write(p)
}

func (l *consumerLog) content() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Buffer.String()
}

type registeredSecurityDB struct {
	admin, identity, control *pgxpool.Pool
	dsn                      string
}

func (db registeredSecurityDB) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := db.admin.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func (db registeredSecurityDB) pool(t *testing.T, role string, max int32) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(db.dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.User = role
	cfg.MaxConns = max
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	return pool
}

// registeredSecurityDatabase installs the registered dependency closure and
// applies the manifest grants consumed by the security entries of the gateway.
func registeredSecurityDatabase(t *testing.T, root string) registeredSecurityDB {
	t.Helper()
	db := registeredSecurityDB{dsn: localSecurityPostgres(t)}
	var err error
	db.admin, err = pgxpool.New(context.Background(), db.dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.admin.Close)
	read := func(name string, out any) {
		t.Helper()
		wire, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(wire, out); err != nil {
			t.Fatal(err)
		}
	}
	var registry production.MigrationRegistry
	read("migrations/registry.json", &registry)
	catalog, err := registry.ValidateOrder()
	if err != nil {
		t.Fatal(err)
	}
	var grants struct {
		Roles []production.RoleManifest `json:"roles"`
	}
	read("migrations/grants.json", &grants)
	var roles []production.RoleManifest
	for _, m := range grants.Roles {
		if m.RuntimeRole != "identity_runtime" && m.RuntimeRole != "control_runtime" {
			continue
		}
		selected := m
		selected.Grants = nil
		for _, g := range m.Grants {
			if g.Schema == "pnl" || g.Schema == "security" || (g.Schema == "users" && g.Object == "delivery_quarantine") {
				selected.Grants = append(selected.Grants, g)
			}
		}
		if m.RuntimeRole == "identity_runtime" {
			selected.OwnerReadGrants = nil
			selected.SchemaCapabilities = nil
			for _, c := range m.SchemaCapabilities {
				if c.Service == "security/policy" {
					selected.SchemaCapabilities = append(selected.SchemaCapabilities, c)
				}
			}
		}
		roles = append(roles, selected)
	}
	if len(roles) != 2 {
		t.Fatal("gateway security roles required")
	}
	provision, err := production.ProvisionRolesSQL(roles)
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range provision {
		db.exec(t, sql)
	}
	needed := map[int64]bool{}
	var include func(int64)
	include = func(id int64) {
		if needed[id] {
			return
		}
		m, ok := catalog[id]
		if !ok {
			t.Fatalf("registered migration %d required", id)
		}
		needed[id] = true
		for _, predecessor := range m.PredecessorIDs {
			include(predecessor)
		}
	}
	include(470001)
	include(450001)
	for _, id := range registry.ApplicationOrder {
		if !needed[id] {
			continue
		}
		m := catalog[id]
		if id == 470001 {
			for _, role := range roles {
				owner := role
				owner.Grants = nil
				sql, err := owner.GrantSQL()
				if err != nil {
					t.Fatal(err)
				}
				for _, statement := range sql {
					db.exec(t, statement)
				}
			}
		}
		wire, err := os.ReadFile(filepath.Join(root, m.Path))
		if err != nil || production.Checksum(wire) != m.SHA256 {
			t.Fatalf("registered migration %d bytes: %v", id, err)
		}
		tx, err := db.admin.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		owner := map[string]string{"pnl": "pnl_owner", "users": "users_owner", "security": "security_owner"}[m.Schema]
		if owner == "" {
			t.Fatalf("migration owner required for %s", m.Schema)
		}
		if _, err = tx.Exec(context.Background(), "GRANT CREATE ON DATABASE postgres TO "+pgx.Identifier{owner}.Sanitize()); err == nil {
			_, err = tx.Exec(context.Background(), "SET LOCAL ROLE "+pgx.Identifier{owner}.Sanitize())
		}
		if err == nil {
			_, err = tx.Exec(context.Background(), string(wire))
		}
		if err == nil {
			_, err = tx.Exec(context.Background(), "RESET ROLE")
		}
		if err == nil {
			_, err = tx.Exec(context.Background(), "REVOKE CREATE ON DATABASE postgres FROM "+pgx.Identifier{owner}.Sanitize())
		}
		if err != nil {
			_ = tx.Rollback(context.Background())
			t.Fatalf("migration %d: %v", id, err)
		}
		if err := tx.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	for _, role := range roles {
		sql, err := role.GrantSQL()
		if err != nil {
			t.Fatal(err)
		}
		for _, statement := range sql {
			db.exec(t, statement)
		}
		db.exec(t, "ALTER ROLE "+pgx.Identifier{role.RuntimeRole}.Sanitize()+" LOGIN")
	}
	var schemaCreation bool
	if err := db.admin.QueryRow(context.Background(), `SELECT bool_or(has_database_privilege(role,current_database(),'CREATE')) FROM unnest(ARRAY['pnl_owner','users_owner','security_owner','control_runtime','identity_runtime']) AS role`).Scan(&schemaCreation); err != nil || schemaCreation {
		t.Fatal("runtime and schema owner database privileges", schemaCreation, err)
	}
	return db
}

func composedSecurityBinding(t *testing.T, root string) (string, string, int32, int32, string, string) {
	t.Helper()
	command := exec.Command("python3", "-c", `import base64,hashlib,json,sys
from pathlib import Path
sys.path.insert(0,sys.argv[1])
import gateway
docs=gateway.documents('gateway',Path(sys.argv[1])/'values')
print(json.dumps({k: {'document':base64.b64encode(v).decode(), 'sha256':hashlib.sha256(v).hexdigest()} for k,v in docs.items()}))`, filepath.Join(root, "deploy/launchpad/documents"))
	command.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	wire, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("gateway document composition: %v: %s", err, wire)
	}
	var documents map[string]struct {
		Document string
		SHA256   string
	}
	if err := json.Unmarshal(wire, &documents); err != nil {
		t.Fatal(err)
	}
	composed := documents["GATEWAY_RUNTIME_DOCUMENT"]
	runtime, err := base64.StdEncoding.Strict().DecodeString(composed.Document)
	sum := sha256.Sum256(runtime)
	if err != nil || hex.EncodeToString(sum[:]) != composed.SHA256 {
		t.Fatal("digest-bound runtime document required", err)
	}
	var binding struct {
		Control struct{ Schema, AuthoritySchema, IncidentCredentialReference, RevocationEndpoint, RevocationTokenReference string }
		Pools   struct {
			ControlCredentialReference                    string
			ControlMaxConnections, IdentityMaxConnections int32
		}
	}
	if err := json.Unmarshal(runtime, &binding); err != nil {
		t.Fatal(err)
	}
	if binding.Control.Schema != "pnl" || binding.Control.AuthoritySchema != "pnl" || binding.Pools.ControlCredentialReference != "GATEWAY_CONTROL_DATABASE_URL" || binding.Control.IncidentCredentialReference != "GATEWAY_INCIDENT_CREDENTIAL" || binding.Control.RevocationTokenReference != "GATEWAY_REVOCATION_TOKEN" {
		t.Fatal("gateway control authority bindings required")
	}
	return binding.Control.Schema, binding.Control.AuthoritySchema, binding.Pools.ControlMaxConnections, binding.Pools.IdentityMaxConnections, binding.Control.RevocationEndpoint, binding.Control.IncidentCredentialReference
}

func registeredSecurityConsumers(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	schema, authoritySchema, controlMax, identityMax, revocationEndpoint, _ := composedSecurityBinding(t, root)
	db := registeredSecurityDatabase(t, root)
	db.identity = db.pool(t, "identity_runtime", identityMax)
	db.control = db.pool(t, "control_runtime", controlMax)
	logs := &consumerLog{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	ctx := context.Background()
	t.Run("private policy entries", func(t *testing.T) { registeredPolicyConsumers(t, db, logs) })
	t.Run("provider transport diagnostics", func(t *testing.T) { securityProviderTransport(t, logs) })
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		t.Fatal(err)
	}
	token := hex.EncodeToString(tokenBytes)
	hash := sha256.Sum256([]byte(token))
	actor := "b74814d9-319a-4b2c-bf58-1dc496ee0569"
	session := "28b36c60-ce90-473c-aae6-054c401ec7ee"
	db.exec(t, `INSERT INTO security.operator_sessions(session_id,wallet,actor,audience,issued_at,expires_at,mfa_expires_at,scopes) VALUES($1,$2,$3,'control',clock_timestamp()-interval '1 second',clock_timestamp()+interval '10 minutes',clock_timestamp()+interval '4 minutes',ARRAY['platform'])`, session, "0x52908400098527886E0F7030069857D2E4169EE7", actor)
	db.exec(t, `INSERT INTO pnl.authority_credentials(credential_id,actor_id,role,scopes,token_hash,session_id,expires_at) VALUES('da34c063-1a3c-49f2-bc8d-94a65607fb24',$1,'infrastructure',ARRAY['platform'],$2,$3,clock_timestamp()+interval '10 minutes')`, actor, hash[:], session)
	provider, err := control.NewRevocationProvider(revocationEndpoint, token, &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}})
	if err != nil {
		t.Fatal(err)
	}
	newRuntime := func() *control.Runtime {
		t.Helper()
		runtime, err := control.NewRuntime(control.RuntimeConfig{ControlPool: db.control, IdentityPool: db.identity, ControlSchema: schema, AuthoritySchema: authoritySchema, Revocation: provider, IncidentCredential: token, Observe: func(_ string, err error) {
			if err != nil {
				t.Error(err)
			}
		}})
		if err != nil {
			t.Fatal(err)
		}
		if err := runtime.Ready(ctx); err != nil {
			t.Fatal("runtime role readiness", err)
		}
		return runtime
	}
	runtime := newRuntime()
	server := httptest.NewServer(runtime.Handler())
	t.Cleanup(server.Close)
	request := func(t *testing.T, method, path string, body any, want int) []byte {
		t.Helper()
		wire, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(method, server.URL+path, bytes.NewReader(wire))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		res, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		response, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != want {
			t.Fatalf("%s %s: %d, want %d: %s", method, path, res.StatusCode, want, response)
		}
		return response
	}
	expires := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	proposal := control.ActionRequest{Key: "replay-window", Kind: "replay", Scope: "platform", Reason: "inspect a projection replay", PayloadHash: strings.Repeat("a", 64), Expires: expires}
	first := request(t, "POST", "/v2/control/actions", proposal, 200)
	var action struct{ Data control.Action }
	if err := json.Unmarshal(first, &action); err != nil || action.Data.State != "proposed" || action.Data.Revision != 1 {
		t.Fatal("durable action response", string(first), err)
	}
	zero := uint64(0)
	cap := control.CapRequest{Scope: "platform", Reason: "pause quote admission", Expected: &zero, Cap: control.Cap{Capability: "quote", Version: 1, Limit: 0, Effective: time.Now().Add(-time.Second), Expires: expires}}
	request(t, "POST", "/v2/control/caps", cap, 200)
	request(t, "POST", "/v2/control/caps", cap, 409)
	var capView struct{ Data control.CapWarning }
	if wire := request(t, "GET", "/v2/control/caps/quote", nil, 200); json.Unmarshal(wire, &capView) != nil || capView.Data.Allowed || capView.Data.Revision != "1" {
		t.Fatal("persisted cap response", string(wire))
	}
	settlement := request(t, "GET", "/v2/control/settlement-capability", nil, 200)
	if !bytes.Contains(settlement, []byte(`"available":false`)) || !bytes.Contains(settlement, []byte("settlement_authority_unavailable")) {
		t.Fatal("unconfigured settlement authority", string(settlement))
	}
	workerCtx, cancelWorkers := context.WithTimeout(ctx, 100*time.Millisecond)
	if err := runtime.Run(workerCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("worker cancellation", err)
	}
	cancelWorkers()
	server.Close()
	db.control.Close()
	db.identity.Close()
	db.control = db.pool(t, "control_runtime", controlMax)
	db.identity = db.pool(t, "identity_runtime", identityMax)
	runtime = newRuntime()
	server = httptest.NewServer(runtime.Handler())
	t.Cleanup(server.Close)
	var recovered struct{ Data control.Action }
	if wire := request(t, "POST", "/v2/control/actions", proposal, 200); json.Unmarshal(wire, &recovered) != nil || recovered.Data.ID != action.Data.ID || recovered.Data.Revision != action.Data.Revision {
		t.Fatal("restart changed durable proposal", string(wire))
	}
	if wire := request(t, "GET", "/v2/control/caps/quote", nil, 200); json.Unmarshal(wire, &capView) != nil || capView.Data.Revision != "1" || capView.Data.Allowed {
		t.Fatal("restart changed cap", string(wire))
	}
	if _, err := db.control.Exec(ctx, "UPDATE pnl.audit SET result='ok'"); err == nil {
		t.Fatal("runtime changed append-only audit")
	}
	if _, err := db.admin.Exec(ctx, "UPDATE pnl.audit SET result='ok'"); err == nil {
		t.Fatal("audit trigger permitted mutation")
	}
	t.Run("control storage diagnostics", func(t *testing.T) {
		db.exec(t, "REVOKE SELECT ON pnl.objects FROM control_runtime")
		t.Cleanup(func() { db.exec(t, "GRANT SELECT ON pnl.objects TO control_runtime") })
		request(t, "POST", "/v2/control/actions", proposal, 503)
		wire := request(t, "GET", "/v2/control/caps/quote", nil, 200)
		var view struct{ Data control.CapWarning }
		if err := json.Unmarshal(wire, &view); err != nil || view.Data.Allowed || !view.Data.Stale || view.Data.Reason != "cap_unavailable" {
			t.Fatal("storage outage capability response", string(wire), err)
		}
		if !strings.Contains(logs.content(), "control operation unavailable") || !strings.Contains(logs.content(), "control capability unavailable") || !strings.Contains(logs.content(), "permission denied for table objects") {
			t.Fatal("control storage diagnostics missing", logs.content())
		}
	})
	t.Run("wallet authority diagnostics", func(t *testing.T) {
		columns := "actor_id,account_id,wallet,chain_id,state"
		db.exec(t, "REVOKE SELECT ("+columns+") ON pnl.identity_control_wallets_v2 FROM identity_runtime")
		t.Cleanup(func() {
			db.exec(t, "GRANT SELECT ("+columns+") ON pnl.identity_control_wallets_v2 TO identity_runtime")
		})
		if err := runtime.Identity.Wallet(ctx, actor, "0x52908400098527886e0f7030069857d2e4169ee7"); !errors.Is(err, control.Unavailable) {
			t.Fatal("wallet authority availability", err)
		}
		if !strings.Contains(logs.content(), "control wallet authority unavailable") || !strings.Contains(logs.content(), "permission denied for view identity_control_wallets_v2") {
			t.Fatal("wallet authority diagnostics missing", logs.content())
		}
	})

	t.Run("identity dependency and recovery", func(t *testing.T) {
		columns := "credential_id,actor_id,role,scopes,token_hash,session_id,expires_at,revoked_at"
		db.exec(t, "REVOKE SELECT ("+columns+") ON pnl.authority_credentials_v2 FROM identity_runtime")
		t.Cleanup(func() { db.exec(t, "GRANT SELECT ("+columns+") ON pnl.authority_credentials_v2 TO identity_runtime") })
		if err := runtime.Ready(ctx); !errors.Is(err, control.Denied) {
			t.Fatal("readiness admitted missing authority grant", err)
		}
		for _, path := range []string{"/v2/control/campaigns/28b36c60-ce90-473c-aae6-054c401ec7ee", "/v2/control/recommendations/28b36c60-ce90-473c-aae6-054c401ec7ee"} {
			wire := request(t, "GET", path, nil, 503)
			var fault control.WorkflowError
			if err := json.Unmarshal(wire, &fault); err != nil || fault.Code != "DEPENDENCY_UNAVAILABLE" || !fault.Retryable {
				t.Fatal(string(wire), err)
			}
		}
		request(t, "POST", "/v2/control/actions", proposal, 503)
		if _, err := runtime.Store.Propose(ctx, token, proposal.Key, proposal.Kind, proposal.Scope, proposal.Reason, proposal.PayloadHash, expires); !errors.Is(err, control.Unavailable) {
			t.Fatal("store identity availability", err)
		}
		local, err := control.OpenLocalStore(filepath.Join(t.TempDir(), "objects.json"), runtime.Identity, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer local.CloseLocal()
		if _, err := local.Propose(ctx, token, proposal.Key, proposal.Kind, proposal.Scope, proposal.Reason, proposal.PayloadHash, expires); !errors.Is(err, control.Unavailable) {
			t.Fatal("local identity availability", err)
		}
		if !strings.Contains(logs.content(), "42501") || !strings.Contains(logs.content(), "control identity unavailable") || strings.Contains(logs.content(), token) {
			t.Fatal("identity dependency diagnostics missing or disclose credential", logs.content())
		}
	})
	if err := runtime.Ready(ctx); err != nil {
		t.Fatal("restored authority readiness", err)
	}
	request(t, "POST", "/v2/control/actions", proposal, 200)
	var decisions int
	if err := db.admin.QueryRow(ctx, "SELECT count(*) FROM pnl.audit WHERE operation='action' AND result='ok'").Scan(&decisions); err != nil || decisions != 3 {
		t.Fatal("unavailable identity mutated audit", decisions, err)
	}
	db.exec(t, "UPDATE security.operator_sessions SET revoked=true WHERE session_id=$1", session)
	request(t, "POST", "/v2/control/actions", proposal, 401)
	if _, err := runtime.Store.Propose(ctx, token, proposal.Key, proposal.Kind, proposal.Scope, proposal.Reason, proposal.PayloadHash, expires); !errors.Is(err, control.Denied) {
		t.Fatal("revoked operator session admitted", err)
	}
	db.identity.Close()
	if err := runtime.Ready(ctx); !errors.Is(err, control.Unavailable) || !strings.Contains(logs.content(), "control role readiness unavailable") {
		t.Fatal("closed identity pool readiness diagnostics", err, logs.content())
	}
}

func securityProviderTransport(t *testing.T, logs *consumerLog) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "https://" + listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	token := hex.EncodeToString(secret)
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	payout, err := control.NewPayoutProvider(endpoint, token, "settlement controller", control.USDLAddress, transport)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := payout.Lookup(context.Background(), "settlement.transport"); !errors.Is(err, control.Unavailable) {
		t.Fatal("payout transport availability", err)
	}
	revocation, err := control.NewRevocationProvider(endpoint, token, transport)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := revocation.Revoke(context.Background(), control.Incident{ID: "incident.transport", ServiceIdentity: "identity.transport"}); !errors.Is(err, control.Unavailable) {
		t.Fatal("revocation transport availability", err)
	}
	replay, err := control.NewReplayProvider(endpoint, token, transport)
	if err != nil {
		t.Fatal(err)
	}
	payload := control.ReplayPayload{ChainID: "125", FromHeight: "1", ToHeight: "2", Projection: "candles", DryRun: true}
	wire, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(wire)
	job := control.ReplayJob{PayloadHash: hex.EncodeToString(hash[:]), CompletionIdentity: "control-replay:transport"}
	if _, err := replay.Execute(context.Background(), control.ReplayExecution{JobID: "transport", Job: job, Payload: payload}); !errors.Is(err, control.Unavailable) {
		t.Fatal("replay transport availability", err)
	}
	for _, operation := range []string{"payout request", "identity revocation request", "projection replay request", "connection refused"} {
		if !strings.Contains(logs.content(), operation) {
			t.Fatal("provider dependency cause missing", operation, logs.content())
		}
	}
	if strings.Contains(logs.content(), token) {
		t.Fatal("provider credential disclosed")
	}
}

func registeredPolicyConsumers(t *testing.T, db registeredSecurityDB, logs *consumerLog) {
	ctx := context.Background()
	db.exec(t, `INSERT INTO security.operator_sessions(session_id,wallet,actor,audience,issued_at,expires_at,mfa_expires_at,scopes) VALUES('private-operator','0x52908400098527886E0F7030069857D2E4169EE7','operator-a','indexer',clock_timestamp()-interval '1 second',clock_timestamp()+interval '10 minutes',clock_timestamp()+interval '4 minutes',ARRAY['inspect','replay'])`)
	socket := localSecurityRedis(t)
	redisClient := redis.NewClient(&redis.Options{Network: "unix", Addr: socket, MaxRetries: -1})
	t.Cleanup(func() { _ = redisClient.Close() })
	var captureBytes consumerLog
	capture := &MiddlewareCapture{Writer: &captureBytes, Key: []byte(strings.Repeat("k", 32)), RunID: "runtime", ReplicaID: "private", CandidateSHA256: strings.Repeat("a", 64), PolicyVersion: "v1"}
	var served atomic.Int32
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { served.Add(1); w.WriteHeader(204) })
	server, client, key := realPrivateOperatorServer(t, db.identity, "security", func(v WorkloadVerifier) http.Handler {
		mux := http.NewServeMux()
		mux.Handle("/authorize", MeasuredAuthorize(v, PostgresJournal{Pool: db.identity, Schema: "security"}, "indexer", "replay", "v1", next, capture))
		mux.Handle("/boundary", PrivateBoundary(v, next))
		admission, err := MeasuredAdmission(AdmissionOptions{Redis: redisClient, Namespace: "registered", Class: classAccountWrite, Cost: 1, Verifier: v}, next, capture)
		if err != nil {
			t.Fatal(err)
		}
		mux.Handle("/admission", admission)
		letters := DeadLetters{Pool: db.identity, DeliverySchema: "users", SecuritySchema: "security", Audience: "indexer", PolicyVersion: "v1", Verifier: v, Journal: PostgresJournal{Pool: db.identity, Schema: "security"}}
		mux.Handle("/status/", letters.Handler())
		mux.Handle("/operator/redrive", Authorize(v, letters.Journal, "indexer", "replay", "v1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			job, err := letters.RequestRedrive(r.Context(), r, "identity", "g1", strings.Repeat("a", 64), "retry identity projection")
			if err != nil {
				t.Error(err)
				http.Error(w, "redrive unavailable", 503)
				return
			}
			w.WriteHeader(202)
			_, _ = io.WriteString(w, job)
		})))
		return mux
	})
	send := func(path string, want int) []byte {
		t.Helper()
		r, err := http.NewRequest("GET", server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		signRealOperatorRequest(t, r, key, "private-operator")
		res, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != want {
			t.Fatalf("%s: %d, want %d: %s", path, res.StatusCode, want, body)
		}
		return body
	}
	paths := []string{"/authorize", "/boundary", "/admission"}
	for _, path := range paths {
		send(path, 204)
	}
	db.exec(t, "REVOKE SELECT ON security.operator_sessions FROM identity_runtime")
	for _, path := range paths {
		send(path, 503)
	}
	if served.Load() != 3 {
		t.Fatal("unavailable identity reached consumer", served.Load())
	}
	for _, line := range strings.Split(strings.TrimSpace(captureBytes.content()), "\n") {
		var record MiddlewareRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if !record.Allowed && !record.BackendUnavailable {
			t.Fatal("dependency failure recorded as denial", record)
		}
	}
	db.exec(t, "GRANT SELECT ON security.operator_sessions TO identity_runtime")
	for _, path := range paths {
		send(path, 204)
	}
	db.exec(t, "REVOKE INSERT ON security.authorization_decisions FROM identity_runtime")
	send("/authorize", 503)
	db.exec(t, "GRANT INSERT ON security.authorization_decisions TO identity_runtime")
	send("/authorize", 204)
	if err := redisClient.Close(); err != nil {
		t.Fatal(err)
	}
	send("/admission", 503)
	if !strings.Contains(logs.content(), "authorization journal") || !strings.Contains(logs.content(), "request budget") || !strings.Contains(logs.content(), "42501") {
		t.Fatal("dependency diagnostics missing", logs.content())
	}
	file, err := os.CreateTemp(t.TempDir(), "capture-*.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	capture.Writer = file
	send("/authorize", 503)
	if served.Load() != 7 || !strings.Contains(logs.content(), "middleware capture") || !strings.Contains(logs.content(), "file already closed") {
		t.Fatal("capture outage reached consumer or lost diagnostics", served.Load(), logs.content())
	}
	for _, query := range []string{"limit=0", "limit=101", "after=invalid"} {
		send("/status/dead-letter?"+query, 400)
	}
	db.exec(t, `INSERT INTO users.delivery_quarantine(consumer_id,projection_generation,envelope_hash,subject,envelope,failure_class) VALUES('identity','g1',$1,'identity.account.v1',$2,'handler-error')`, strings.Repeat("a", 64), []byte("private payload"))
	body := send("/status/dead-letter?limit=1", 200)
	var page struct {
		Entries    []DeadLetterMetadata
		NextCursor string
	}
	if err := json.Unmarshal(body, &page); err != nil || len(page.Entries) != 1 || page.NextCursor == "" || bytes.Contains(body, []byte("private payload")) {
		t.Fatal("registered quarantine metadata", string(body), err)
	}
	if body := send("/status/dead-letter?limit=1&after="+page.NextCursor, 200); !bytes.Contains(body, []byte(`"entries":[]`)) {
		t.Fatal("cursor continuation", string(body))
	}
	job := send("/operator/redrive", 202)
	if replay := send("/operator/redrive", 202); !bytes.Equal(job, replay) || !digestPattern.Match(job) {
		t.Fatal("redrive retry identity", string(job), string(replay))
	}
	var redrives, audits int
	if err := db.identity.QueryRow(ctx, "SELECT count(*) FROM security.deadletter_redrives WHERE state='pending'").Scan(&redrives); err != nil || redrives != 1 {
		t.Fatal("durable redrive cardinality", redrives, err)
	}
	if err := db.identity.QueryRow(ctx, "SELECT count(*) FROM security.audit_outbox").Scan(&audits); err != nil || audits != 2 {
		t.Fatal("durable redrive audit", audits, err)
	}
	db.exec(t, "REVOKE SELECT ON users.delivery_quarantine FROM identity_runtime")
	send("/status/dead-letter", 503)
	db.exec(t, "GRANT SELECT ON users.delivery_quarantine TO identity_runtime")
	send("/status/dead-letter", 200)
	if !strings.Contains(logs.content(), "dead-letter inspection") {
		t.Fatal("quarantine diagnostics missing")
	}
	if _, err := db.identity.Exec(ctx, "UPDATE security.authorization_decisions SET allowed=true"); err == nil {
		t.Fatal("runtime changed authorization journal")
	}
}
