package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/security/control"
	"github.com/Sidiora-Technologies/KindleLaunch/security/control/enrolment"
	"github.com/Sidiora-Technologies/KindleLaunch/security/control/enrolment/enrolmenttest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	requesterActor = "11111111-1111-4111-8111-111111111111"
	approverActor  = "22222222-2222-4222-8222-222222222222"
)

type observedRequest struct {
	path, credential string
	body             []byte
}

type commandFixture struct {
	ctx         context.Context
	db          coordinatortest.Database
	admin       *pgxpool.Pool
	server      *httptest.Server
	environment []string
	mu          sync.Mutex
	requests    []observedRequest
}

func operatorCommandDatabase(t *testing.T) *commandFixture {
	t.Helper()
	ctx := context.Background()
	db := coordinatortest.Bootstrap(t, enrolmenttest.Start(t), "operator_runtime", "operator_registrar", "control_runtime", "identity_runtime")
	connect := func(dsn string) *pgxpool.Pool {
		t.Helper()
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			t.Fatal("connect operator database")
		}
		t.Cleanup(pool.Close)
		return pool
	}
	identity, err := control.NewPostgresIdentityV2(connect(db.Runtimes["identity_runtime"].DSN), "pnl", "pnl")
	if err != nil {
		t.Fatal(err)
	}
	store, err := control.New(connect(db.Runtimes["control_runtime"].DSN), "pnl", identity, nil)
	if err != nil {
		t.Fatal(err)
	}
	service, err := control.NewHTTPService(store, nil)
	if err != nil {
		t.Fatal(err)
	}
	f := &commandFixture{ctx: ctx, db: db, admin: connect(db.AdminDSN)}
	handler := service.Handler()
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error("read control request")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		f.mu.Lock()
		f.requests = append(f.requests, observedRequest{r.URL.Path, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), body})
		f.mu.Unlock()
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(f.server.Close)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	f.environment = []string{runtimeSetting + "=" + db.Runtimes["operator_runtime"].DSN, registrarSetting + "=" + db.Runtimes["operator_registrar"].DSN, factorSetting + "=" + hex.EncodeToString(key), gatewaySetting + "=" + f.server.URL}
	return f
}

func (f *commandFixture) observed() []observedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]observedRequest(nil), f.requests...)
}

func (f *commandFixture) run(t *testing.T, args []string, codes string) (int, string) {
	t.Helper()
	var output bytes.Buffer
	status := Run(args, f.environment, strings.NewReader(codes), &output)
	for _, request := range f.observed() {
		if request.credential != "" && strings.Contains(output.String(), request.credential) {
			t.Fatal("output contains session authority")
		}
	}
	for name, value := range settings(f.environment) {
		if name == factorSetting && strings.Contains(output.String(), value) {
			t.Fatal("output contains factor key")
		}
		if name == runtimeSetting || name == registrarSetting {
			u, err := url.Parse(value)
			if err != nil {
				t.Fatal("parse database address")
			}
			password, ok := u.User.Password()
			if !ok || password == "" {
				t.Fatal("database password required")
			}
			if strings.Contains(output.String(), password) || strings.Contains(output.String(), value) {
				t.Fatal("output contains database authority")
			}
		}
	}
	for _, code := range strings.Fields(codes) {
		if strings.Contains(output.String(), code) {
			t.Fatal("output contains one-time code")
		}
	}
	var active int
	if err := f.admin.QueryRow(f.ctx, "SELECT count(*) FROM security.operator_sessions WHERE NOT revoked").Scan(&active); err != nil || active != 0 {
		t.Fatal("operator session remained open")
	}
	if err := f.admin.QueryRow(f.ctx, "SELECT count(*) FROM pnl.authority_credentials WHERE session_id IS NOT NULL AND revoked_at IS NULL").Scan(&active); err != nil || active != 0 {
		t.Fatal("operator credential remained active")
	}
	return status, output.String()
}

func (f *commandFixture) enrol(t *testing.T, actor, role string) []byte {
	t.Helper()
	status, output := f.run(t, []string{"enrol", actor, "operator-wallet", role, "platform"}, "")
	if status != 0 {
		t.Fatal("enrol command failed:", output)
	}
	if strings.Count(output, "\n") != 1 {
		t.Fatal("enrol must print one provisioning address")
	}
	address, err := url.Parse(strings.TrimSpace(output))
	if err != nil {
		t.Fatal("invalid provisioning address")
	}
	query := address.Query()
	if address.Scheme != "otpauth" || address.Host != "totp" || address.Path != "/launchpad:"+actor || query.Get("algorithm") != "SHA1" || query.Get("digits") != "6" || query.Get("period") != "30" || query.Get("issuer") != "launchpad" {
		t.Fatal("provisioning parameters differ")
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(query.Get("secret"))
	if err != nil || len(secret) != 20 {
		t.Fatal("provisioning factor is not base32")
	}
	return secret
}

func (f *commandFixture) code(t *testing.T, secret []byte) string {
	t.Helper()
	var now time.Time
	if err := f.admin.QueryRow(f.ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		t.Fatal(err)
	}
	code, err := enrolment.Code(secret, now)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func (f *commandFixture) warning(t *testing.T, name string) control.CapWarning {
	t.Helper()
	g, err := gateway(f.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer g.client.CloseIdleConnections()
	warning, _, err := g.cap(f.ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	return warning
}

func changeArgs(verb, actor, approver, limit, hours string) []string {
	args := []string{verb, actor}
	if verb == "raise" {
		args = append(args, approver)
	}
	return append(args, "pool_creation", limit, hours, "adjust capability quota")
}

func TestOperatorCommandRaise(t *testing.T) {
	f := operatorCommandDatabase(t)
	requester := f.enrol(t, requesterActor, "infrastructure")
	approver := f.enrol(t, approverActor, "infrastructure")
	status, output := f.run(t, changeArgs("raise", requesterActor, approverActor, "17", "24"), f.code(t, requester)+"\n"+f.code(t, approver)+"\n")
	if status != 0 || !strings.Contains(output, "pool_creation allowed=true stale=false limit=17") {
		t.Fatal("raise did not admit capability:", output)
	}
	warning := f.warning(t, "pool_creation")
	if !warning.Allowed || warning.Stale || warning.Limit != 17 || warning.Revision != "1" || warning.Expires.Sub(warning.Effective) != 24*time.Hour {
		t.Fatal("raised capability does not bind limit and duration")
	}
	var cap control.Cap
	var raw []byte
	if err := f.admin.QueryRow(f.ctx, "SELECT body FROM pnl.objects WHERE kind='caps' AND id='pool_creation'").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(raw, &cap) != nil || cap.Effective.After(time.Now()) {
		t.Fatal("capability effective time differs")
	}
	var proposal control.Action
	if err := f.admin.QueryRow(f.ctx, "SELECT body FROM pnl.objects WHERE kind='action'").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(raw, &proposal) != nil || proposal.State != "succeeded" || proposal.RequestedBy != requesterActor || proposal.ApprovedBy != approverActor || proposal.PayloadHash != control.CapDigest(cap) || !proposal.Expires.Equal(cap.Expires) {
		t.Fatal("raise did not consume the bound dual approval")
	}
	cap.Revision = 0
	canonical, err := json.Marshal(cap)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(canonical)
	if proposal.PayloadHash != hex.EncodeToString(digest[:]) {
		t.Fatal("capability digest differs from canonical payload")
	}
	for _, actor := range []string{requesterActor, approverActor} {
		var count int
		if err := f.admin.QueryRow(f.ctx, "SELECT count(*) FROM pnl.audit WHERE actor=$1 AND operation='action' AND resource='platform' AND result='ok'", actor).Scan(&count); err != nil || count != 1 {
			t.Fatal("dual approval actor missing from audit")
		}
	}
	var capAudits int
	if err := f.admin.QueryRow(f.ctx, "SELECT count(*) FROM pnl.audit WHERE actor=$1 AND operation='caps' AND result='ok'", requesterActor).Scan(&capAudits); err != nil || capAudits != 1 {
		t.Fatal("capability audit missing")
	}
	requests := f.observed()
	var posts []string
	for _, request := range requests {
		if request.credential != "" {
			posts = append(posts, request.path)
		}
	}
	if !reflect.DeepEqual(posts, []string{"/v2/control/actions", "/v2/control/transition", "/v2/control/caps"}) {
		t.Fatal("raise operation order differs")
	}
	var capRequest control.CapRequest
	var proposalRequest control.ActionRequest
	for _, request := range requests {
		switch request.path {
		case "/v2/control/actions":
			if json.Unmarshal(request.body, &proposalRequest) != nil {
				t.Fatal("decode proposal")
			}
		case "/v2/control/caps":
			if json.Unmarshal(request.body, &capRequest) != nil {
				t.Fatal("decode capability")
			}
		}
	}
	if capRequest.ApprovalKey == "" || capRequest.ApprovalKey != proposalRequest.Key || capRequest.Expected == nil || *capRequest.Expected != 0 || capRequest.Cap.Revision != 0 || !capRequest.Cap.Effective.Equal(warning.Effective) {
		t.Fatal("raise does not bind proposal and expected revision")
	}
}

func TestOperatorCommandEqualActors(t *testing.T) {
	f := operatorCommandDatabase(t)
	before := len(f.observed())
	status, output := f.run(t, changeArgs("raise", requesterActor, strings.ToUpper(requesterActor), "1", "1"), "")
	if status == 0 || output != "invalid command arguments\n" || len(f.observed()) != before {
		t.Fatal("equal actors must be rejected before requests")
	}
}

func TestOperatorCommandWrongApproverCode(t *testing.T) {
	f := operatorCommandDatabase(t)
	requester := f.enrol(t, requesterActor, "infrastructure")
	f.enrol(t, approverActor, "infrastructure")
	before := f.warning(t, "pool_creation")
	wrong := "not-a-code"
	status, output := f.run(t, changeArgs("raise", requesterActor, approverActor, "9", "1"), f.code(t, requester)+"\n"+wrong+"\n")
	if status == 0 || !strings.Contains(output, "invalid one-time code") {
		t.Fatal("invalid approver code admitted")
	}
	after := f.warning(t, "pool_creation")
	before.ObservedAt, after.ObservedAt = "", ""
	if !reflect.DeepEqual(before, after) {
		t.Fatal("invalid approver code changed capability")
	}
	// Authentication also refuses an incorrect six-digit code.
	f2 := operatorCommandDatabase(t)
	r2 := f2.enrol(t, requesterActor, "infrastructure")
	a2 := f2.enrol(t, approverActor, "infrastructure")
	var now time.Time
	if err := f2.admin.QueryRow(f2.ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		t.Fatal(err)
	}
	current, err := enrolment.Code(a2, now)
	if err != nil {
		t.Fatal(err)
	}
	previous, err := enrolment.Code(a2, now.Add(-30*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []string{"000000", "111111", "222222"} {
		if candidate != current && candidate != previous {
			wrong = candidate
			break
		}
	}
	status, output = f2.run(t, changeArgs("raise", requesterActor, approverActor, "9", "1"), f2.code(t, r2)+"\n"+wrong+"\n")
	if status == 0 || output != "operator authentication denied\n" {
		t.Fatal("wrong approver factor admitted")
	}
	if warning := f2.warning(t, "pool_creation"); warning.Allowed || warning.Revision != "0" {
		t.Fatal("wrong approver factor changed capability")
	}
}

func TestOperatorCommandRouteRefusal(t *testing.T) {
	f := operatorCommandDatabase(t)
	requester := f.enrol(t, requesterActor, "infrastructure")
	approver := f.enrol(t, approverActor, "support")
	status, output := f.run(t, changeArgs("raise", requesterActor, approverActor, "1", "1"), f.code(t, requester)+"\n"+f.code(t, approver)+"\n")
	if status == 0 || !strings.Contains(output, "status=403 code=SCOPE_DENIED") {
		t.Fatal("route refusal must include status and code:", output)
	}
	if warning := f.warning(t, "pool_creation"); warning.Allowed || warning.Revision != "0" {
		t.Fatal("refused approval changed capability")
	}
}

func TestOperatorCommandReduceAndShow(t *testing.T) {
	f := operatorCommandDatabase(t)
	secret := f.enrol(t, requesterActor, "infrastructure")
	status, output := f.run(t, changeArgs("reduce", requesterActor, "", "0", "1"), f.code(t, secret)+"\n")
	if status != 0 || !strings.Contains(output, "pool_creation allowed=false stale=false limit=0") {
		t.Fatal("single operator reduction refused:", output)
	}
	var posts int
	for _, request := range f.observed() {
		if request.credential == "" {
			continue
		}
		posts++
		var q control.CapRequest
		if request.path != "/v2/control/caps" || json.Unmarshal(request.body, &q) != nil || q.ApprovalKey != "" || q.Expected == nil || *q.Expected != 0 {
			t.Fatal("reduction requested approval")
		}
	}
	if posts != 1 {
		t.Fatal("reduction must use one mutation")
	}
	status, output = f.run(t, []string{"show"}, "")
	if status != 0 || strings.Count(output, "\n") != 4 {
		t.Fatal("show did not print four capabilities")
	}
	for _, name := range capabilities {
		if !strings.Contains(output, name+" allowed=") || !strings.Contains(output, "limit=") || !strings.Contains(output, "expires=") || !strings.Contains(output, "revision=") {
			t.Fatal("show is missing capability fields")
		}
	}
	if !strings.Contains(output, "pool_creation allowed=false stale=false limit=0") || !strings.Contains(output, "revision=1") {
		t.Fatal("show differs from reduced admission")
	}
}

func TestOperatorCommandRevoke(t *testing.T) {
	f := operatorCommandDatabase(t)
	secret := f.enrol(t, requesterActor, "infrastructure")
	values := settings(f.environment)
	pool, err := pgxpool.New(f.ctx, values[runtimeSetting])
	if err != nil {
		t.Fatal("connect operator runtime")
	}
	defer pool.Close()
	key, err := hex.DecodeString(values[factorSetting])
	if err != nil {
		t.Fatal("decode factor key")
	}
	store, err := enrolment.New(pool, key)
	if err != nil {
		t.Fatal(err)
	}
	credential, _, err := store.Open(f.ctx, requesterActor, f.code(t, secret))
	if err != nil {
		t.Fatal(err)
	}
	f.environment = []string{registrarSetting + "=" + values[registrarSetting]}
	status, output := f.run(t, []string{"revoke", requesterActor}, "")
	if status != 0 || output != "" || strings.Contains(output, credential) {
		t.Fatal("revoke did not close operator authority")
	}
	var revoked bool
	if err := f.admin.QueryRow(f.ctx, "SELECT revoked_at IS NOT NULL FROM security.operator_enrolments WHERE actor=$1", requesterActor).Scan(&revoked); err != nil || !revoked {
		t.Fatal("operator enrolment remains active")
	}
}

func TestOperatorCommandRaiseExistingRevision(t *testing.T) {
	f := operatorCommandDatabase(t)
	actor := "33333333-3333-4333-8333-333333333333"
	first := f.enrol(t, actor, "infrastructure")
	if status, output := f.run(t, changeArgs("reduce", actor, "", "0", "1"), f.code(t, first)+"\n"); status != 0 {
		t.Fatal("disable capability:", output)
	}
	requester := f.enrol(t, requesterActor, "infrastructure")
	approver := f.enrol(t, approverActor, "infrastructure")
	status, output := f.run(t, changeArgs("raise", requesterActor, approverActor, "12", "2"), f.code(t, requester)+"\n"+f.code(t, approver)+"\n")
	if status != 0 || !strings.Contains(output, "allowed=true stale=false limit=12") || !strings.Contains(output, "revision=2") {
		t.Fatal("raise did not use the present revision:", output)
	}
	var last control.CapRequest
	for _, request := range f.observed() {
		if request.path == "/v2/control/caps" && json.Unmarshal(request.body, &last) != nil {
			t.Fatal("decode cap request")
		}
	}
	if last.Expected == nil || *last.Expected != 1 || last.ApprovalKey == "" {
		t.Fatal("raise did not bind existing revision")
	}
}

func TestOperatorCommandReduceIncreaseRefused(t *testing.T) {
	f := operatorCommandDatabase(t)
	requester := f.enrol(t, requesterActor, "infrastructure")
	status, output := f.run(t, changeArgs("reduce", requesterActor, "", "1", "1"), f.code(t, requester)+"\n")
	if status == 0 || !strings.Contains(output, "status=403 code=SCOPE_DENIED") {
		t.Fatal("single operator increased capability")
	}
	warning := f.warning(t, "pool_creation")
	if warning.Allowed || warning.Revision != "0" {
		t.Fatal("refused reduction changed capability")
	}
}
