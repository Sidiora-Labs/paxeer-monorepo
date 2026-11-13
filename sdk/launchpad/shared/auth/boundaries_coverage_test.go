package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestAuthorizationDatabaseBoundaries(t *testing.T) {
	pool, _ := phase2AuthDatabase(t)
	ctx := context.Background()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	request, verify := signedOperation(t)
	ledger := Ledger{Pool: pool, Environment: "test", AuditKey: bytes.Repeat([]byte("a"), 32)}
	event := AuditEvent{RequestID: uuid.NewString(), Actor: request.Actor, Action: request.Action, Result: valueDenied, PolicyVersion: "test", RequestDigest: strings.Repeat("a", 64)}
	principal := Principal{Wallet: request.Actor, SessionID: uuid.NewString()}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	calls := 0
	mutation := func(context.Context, pgx.Tx, []byte) ([]byte, error) { calls++; return []byte(`{}`), nil }
	for name, call := range map[string]func() error{
		"snapshot absent":              func() error { _, err := ReadAuthStateSnapshot(ctx, nil); return err },
		"snapshot cancelled":           func() error { _, err := ReadAuthStateSnapshot(cancelled, pool); return err },
		"operation cancelled":          func() error { _, _, err := ledger.ExecuteValidated(cancelled, request, verify, mutation); return err },
		"operation closed transaction": func() error { _, _, err := ledger.ExecuteValidatedTx(ctx, tx, request, verify, mutation); return err },
		"audit closed transaction":     func() error { return AppendAudit(ctx, tx, ledger.AuditKey, event) },
		"audit cancelled":              func() error { return ledger.RecordDenial(cancelled, event) },
		"account revoked transaction":  func() error { return RevokeIdentityAccountTx(ctx, tx, uuid.NewString(), "logout") },
		"reauth closed transaction": func() error {
			return ConsumeIdentityReauthTx(ctx, tx, uuid.NewString(), uuid.NewString(), "wallet.unlink", strings.Repeat("a", 64))
		},
		"identity cancelled": func() error {
			_, err := (PostgresIdentityResolver{Pool: pool}).Resolve(cancelled, principal)
			return err
		},
		"guarded identity cancelled": func() error {
			_, err := (PostgresIdentityResolver{Pool: pool, GuardFunction: "users.identity_guard_v2"}).Resolve(cancelled, principal)
			return err
		},
		"identity transaction": func() error {
			_, err := (PostgresIdentityResolver{Pool: pool}).ResolveTx(ctx, tx, principal)
			return err
		},
		"identity guarded transaction": func() error {
			_, err := (PostgresIdentityResolver{Pool: pool, GuardFunction: "users.identity_guard_v2"}).ResolveTx(ctx, tx, principal)
			return err
		},
		"committed database absent": func() error { _, _, err := (PostgresCommittedBlockProvider{}).CommittedBlock(ctx); return err },
		"committed database cancelled": func() error {
			_, _, err := (PostgresCommittedBlockProvider{Pool: pool}).CommittedBlock(cancelled)
			return err
		},
		"audit stage cancelled":   func() error { _, err := (AuditBridge{Pool: pool}).Stage(cancelled, 1); return err },
		"audit stage transaction": func() error { _, err := (AuditBridge{Pool: pool}).stageOne(cancelled, 1); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("unavailable authorization dependency admitted work")
			}
		})
	}
	if calls != 0 {
		t.Fatalf("unavailable authorization dependency reached mutation: %d", calls)
	}
	for _, digest := range []string{strings.Repeat("z", 64), strings.Repeat("A", 64)} {
		changed := event
		changed.RequestDigest = digest
		if err := AppendAudit(ctx, tx, ledger.AuditKey, changed); err == nil || err.Error() != "invalid audit digest" {
			t.Fatalf("audit digest admission: %v", err)
		}
	}
	event.Actor = "invalid"
	if err := AppendAudit(ctx, tx, ledger.AuditKey, event); err == nil {
		t.Fatal("invalid audit actor admitted")
	}
	for _, change := range []func(*OperationRequest){
		func(r *OperationRequest) { r.PolicyVersion = "" }, func(r *OperationRequest) { r.Actor = "invalid" }, func(r *OperationRequest) { r.Body = []byte(`{"a":1,"a":2}`) }, func(r *OperationRequest) { r.Envelope.ExpiresAt = 0 },
	} {
		candidate := request
		change(&candidate)
		if _, _, err := ledger.ExecuteValidatedTx(ctx, tx, candidate, verify, mutation); err == nil {
			t.Fatal("invalid operation reached ledger")
		}
	}
	if _, _, err := (Ledger{}).ExecuteValidated(ctx, request, verify, mutation); err == nil {
		t.Fatal("ledger without owning database admitted operation")
	}
	for _, value := range []error{ErrScopeDenied, ErrReplayMismatch, &VerificationError{Category: valueUpstream, Retryable: true}, errors.New("proof rejected")} {
		status, code, retryable := PublicCode(value)
		switch value {
		case ErrScopeDenied:
			if status != 403 || code != "SCOPE_DENIED" || retryable {
				t.Fatalf("scope response: %d %s %t", status, code, retryable)
			}
		case ErrReplayMismatch:
			if status != 409 || code != "IDEMPOTENCY_CONFLICT" || retryable {
				t.Fatalf("replay response: %d %s %t", status, code, retryable)
			}
		default:
			if status != 401 && status != 503 {
				t.Fatalf("verification response: %d", status)
			}
		}
	}
}

func TestAuthorizationJSONAndStreamBoundaries(t *testing.T) {
	for _, wire := range []string{"", `{`, `{"nested":{"key":1,"key":2}}`, `{"array":[`, `[[[[`, `{"x":1} {"x":2}`, strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66), strings.Repeat("x", (1<<20)+1)} {
		if _, err := BodyDigest([]byte(wire)); err == nil {
			t.Fatalf("invalid body admitted: %.40s", wire)
		}
	}
	for _, wire := range []string{`{"array":[1,{"nested":true}],"null":null}`, `"text"`, `[0,1,2]`} {
		digest, err := BodyDigest([]byte(wire))
		want := sha256.Sum256([]byte(wire))
		if err != nil || digest != hex.EncodeToString(want[:]) {
			t.Fatalf("body byte identity: %s %v", digest, err)
		}
	}
	p := StreamPrincipal{AccountID: uuid.NewString(), SessionID: uuid.NewString(), Wallet: strings.ToLower(goldenAddr), AuthEpoch: 1}
	base := StreamTicketBinding{SchemaMajor: 2, Endpoint: "/v2/stream", Origin: "https://app.example", Topics: []string{"chat.pool." + p.Wallet}}
	base.FilterDigest = StreamFilterDigest(base.Topics)
	if err := ValidateChatStreamBinding(p, base); err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(*StreamTicketBinding){
		func(b *StreamTicketBinding) { b.SchemaMajor = 1 }, func(b *StreamTicketBinding) { b.Origin = "invalid" }, func(b *StreamTicketBinding) { b.Topics = []string{"chat.pool.invalid"} }, func(b *StreamTicketBinding) { b.Topics = []string{"chat.pool"} }, func(b *StreamTicketBinding) { b.Topics = []string{"market.pool." + p.Wallet} }, func(b *StreamTicketBinding) { b.Topics = []string{base.Topics[0], base.Topics[0]} },
	} {
		changed := base
		edit(&changed)
		changed.FilterDigest = StreamFilterDigest(changed.Topics)
		if err := ValidateChatStreamBinding(p, changed); !errors.Is(err, ErrScopeDenied) {
			t.Fatalf("chat stream admission: %v", err)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	handler := (OperationAuthorizer{}).Handler("profile.update", ServiceAssertionVerifier{}, PostgresIdentityResolver{}, nil, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var body struct {
		RequestID string `json:"requestId"`
		Code      string `json:"code"`
		Retryable bool   `json:"retryable"`
	}
	if response.Code != 503 || json.Unmarshal(response.Body.Bytes(), &body) != nil || !validIdentityID(body.RequestID) || body.Code != "DEPENDENCY_UNAVAILABLE" || !body.Retryable {
		t.Fatalf("unconfigured authorization response: %d %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("authorization response can be cached")
	}
	extended, routes := AccountOwnerExtendedPoliciesV2()
	if len(extended.Actions) <= 0 || len(routes) <= 0 || len(extended.Actions) != len(routes) {
		t.Fatalf("extended owner routes: %d %d", len(extended.Actions), len(routes))
	}
	if _, err := (&AuthVerificationMeasurements{}).Snapshot(); err == nil {
		t.Fatal("empty verification observer admitted")
	}
	var absent *AuthVerificationMeasurements
	if _, err := absent.Snapshot(); err == nil {
		t.Fatal("absent verification observer admitted")
	}
	if nearestRank(nil, 95) != 0 {
		t.Fatal("empty latency rank differs")
	}
	if _, err := (SignedLoadHarness{}).Run(context.Background(), SignedLoadPlan{}, nil); err == nil {
		t.Fatal("measurement without raw writer admitted")
	}
	if _, err := (SignedLoadHarness{}).Run(context.Background(), SignedLoadPlan{}, &bytes.Buffer{}); err == nil {
		t.Fatal("measurement without bounded plan admitted")
	}
	now := time.Now()
	if _, err := MeasureIdentityRevocation(context.Background(), MTLSIdentityAuthority{}, p, now.Add(time.Minute)); err == nil {
		t.Fatal("future revocation admitted")
	}
}
