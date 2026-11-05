package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

// These tests exercise the real app adapters, PKI and wire codecs without
// supplying substitute keepers, databases, ticket authorities or transports.
// The task record explicitly distinguishes these checks from the external
// producer/store/restart proofs that this dependency-free gate cannot run.
func TestRepair_66_ContainerFrozenRoutes(t *testing.T) {
	ctx := context.Background()
	// pgx pools connect lazily. Construct the production stores for route
	// assembly, but exercise admission and validation before database access.
	pool, err := pgxpool.New(ctx, "postgres://unused:unused@127.0.0.1:1/unused?connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	redis := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"})
	defer redis.Close()
	r := &socialConnectedRuntime{pool: pool, chat: &fanout.DurableStore{Pool: pool, CursorKey: bytes.Repeat([]byte{0x66}, 32)}, safety: &safety.Store{DB: pool}, identity: sharedauth.MTLSIdentityAuthority{}}
	r.request = r.authenticateRequest
	if err = r.prepareFrozen(ctx, redis, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	if r.frozen.Chat != r.chat {
		t.Fatal("frozen adapter split the bound chat authority")
	}
	mux := chi.NewRouter()
	if err = r.registerFrozen(mux); err != nil {
		t.Fatal(err)
	}
	mounted := map[string]bool{}
	if err = chi.Walk(mux, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		mounted[method+" "+route] = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, route := range []frozenRoute{
		{"POST", "/v2/callouts"}, {"GET", "/v2/pools/{poolAddress}/callouts"},
		{"POST", "/v2/pools/{poolAddress}/comments"}, {"PATCH", "/v2/comments/{commentId}"}, {"DELETE", "/v2/comments/{commentId}"},
		{"PUT", "/v2/comments/{commentId}/likes/me"}, {"DELETE", "/v2/comments/{commentId}/likes/me"},
		{"POST", "/v2/reports"}, {"GET", "/v2/bounties"}, {"POST", "/v2/bounties"}, {"GET", "/v2/bounties/stats"},
		{"GET", "/v2/bounties/{bountyId}"}, {"GET", "/v2/bounties/{bountyId}/submissions"}, {"POST", "/v2/bounties/{bountyId}/submissions"}, {"POST", "/v2/bounties/{bountyId}/submissions/{submissionId}:accept"},
	} {
		if !mounted[route.method+" "+route.path] {
			t.Fatalf("unmounted frozen consumer: %+v", route)
		}
	}
	for _, path := range []string{"/v2/callouts", "/v2/pools/" + addr(1) + "/comments", "/v2/bounties", "/v2/bounties/" + uuid.NewString() + "/submissions"} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest("POST", path, strings.NewReader(`{}`)))
		if rr.Code != 401 {
			t.Fatalf("%s unauthenticated write: %d %s", path, rr.Code, rr.Body.String())
		}
		if err = v2.ValidateJSON("Error", rr.Body.Bytes()); err != nil {
			t.Fatal(err)
		}
	}
	// Exercise a real mTLS hop to the bound ingress and the nested frozen
	// router. The callout adapter must see the pool route parameter and reject
	// the page limit, rather than failing because its parameter was lost.
	pki := testpki.New(t)
	social := pki.Leaf("social", "social.private")
	gateway := pki.Leaf("gateway", "gateway.private")
	r.binding = config.SocialRuntimeBinding{GatewayDNS: "gateway.private", GatewayCertSHA256: gateway.SHA256}
	upstream := httptest.NewUnstartedServer(r.ingress((&socialComments{pool: pool}).reads(mux)))
	upstream.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{social.Pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.Roots()}
	upstream.StartTLS()
	defer upstream.Close()
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{gateway.Pair}, ServerName: "social.private"}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	for _, path := range []string{"/v2/pools/" + addr(1) + "/callouts?limit=0", "/v2/pools/" + addr(1) + "/comments?limit=0", "/v2/social/pool/" + addr(1) + "/comments?limit=0", "/v2/bounties?state=invalid"} {
		resp, e := client.Get(upstream.URL + path)
		if e != nil {
			t.Fatal(e)
		}
		raw, e := io.ReadAll(resp.Body)
		resp.Body.Close()
		status := 422
		if strings.Contains(path, "/v2/social/") {
			status = 400
		}
		if e != nil || resp.StatusCode != status {
			t.Fatalf("frozen ingress %s: %d %s %v", path, resp.StatusCode, raw, e)
		}
		if strings.Contains(path, "/v2/social/") {
			var value struct {
				Code      string `json:"code"`
				Retryable bool   `json:"retryable"`
			}
			if json.Unmarshal(raw, &value) != nil || value.Code != "INVALID_INPUT" || value.Retryable {
				t.Fatalf("interaction validation contract: %s", raw)
			}
		} else if err = v2.ValidateJSON("Error", raw); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(path, "callouts") && !strings.Contains(string(raw), "limit") {
			t.Fatalf("nested router lost the actual pool parameter: %s", raw)
		}
	}
}

func TestRepair_66_TicketConsumerDelegates(t *testing.T) {
	// Exercise the real app -> shared authenticator rejection path without a
	// ticket service. A duplicate credential must be refused before redemption.
	req := httptest.NewRequest("GET", "https://social.private/v2/stream?topics=chat.pool."+addr(1), nil)
	req.Header.Set("Sec-WebSocket-Protocol", "sidiora.v2, ticket."+strings.Repeat("aa", 32)+", ticket."+strings.Repeat("bb", 32))
	runtime := &socialConnectedRuntime{}
	if _, err := runtime.authenticateTicket(req); !errors.Is(err, fanout.ErrInvalid) {
		t.Fatalf("ambiguous credential: %v", err)
	}
	// Guard the production wiring: this adapter must delegate all ticket-only,
	// optional-session comparison and continuous validation semantics to the
	// existing shared authenticator rather than reintroducing a second flow.
	file, err := parser.ParseFile(token.NewFileSet(), "app.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "authenticateTicket" {
			continue
		}
		if fn.Body == nil || len(fn.Body.List) != 1 {
			t.Fatal("ticket consumer duplicates shared authentication logic")
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "TicketAuthenticator" {
				found = true
			}
			return true
		})
	}
	if !found {
		t.Fatal("bound ticket consumer does not use TicketAuthenticator")
	}
}

func TestRepair_66_MessageOverlayCodec(t *testing.T) {
	original := fanout.DurableMessage{Type: "pool_message", ID: uuid.NewString(), ConversationID: "pool:" + addr(1), PoolAddress: addr(1), Sender: addr(2), Content: "retained original", ReplyTo: uuid.NewString(), Sequence: 3, Revision: 4, ClientID: "client-66", CreatedAt: 100}
	visibility := safety.Visibility{TargetType: "message", TargetID: original.ID, CaseID: uuid.NewString(), Revision: 1, Status: "removed"}
	removed, err := projectSafetyMessage(original, visibility, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(removed)
	if err != nil {
		t.Fatal(err)
	}
	var consumed fanout.DurableMessage
	if err = json.Unmarshal(raw, &consumed); err != nil {
		t.Fatal(err)
	}
	if !consumed.Deleted || consumed.Content != "" || consumed.ReplyTo != "" || consumed.Revision != 5 || consumed.Sequence != original.Sequence {
		t.Fatalf("removal codec: %+v", consumed)
	}
	if original.Content != "retained original" || original.Deleted {
		t.Fatal("moderation destroyed the source")
	}
	restored := visibility
	restored.Revision = 2
	restored.Status = "restored"
	restored = safety.ApplyVisibility(visibility, restored)
	visible, err := projectSafetyMessage(original, restored, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	if visible.Deleted || visible.Content != original.Content || visible.ReplyTo != original.ReplyTo || visible.Revision <= consumed.Revision {
		t.Fatalf("restoration: %+v", visible)
	}
	persisted := original
	persisted.Revision = visible.Revision
	afterConsumption, e := projectSafetyMessage(persisted, restored, false, restored.Revision)
	if e != nil || afterConsumption != visible {
		t.Fatalf("consumer advanced the same visibility revision twice: %+v %v", afterConsumption, e)
	}
	for _, state := range []struct{ deleted, erased bool }{{true, false}, {false, true}, {true, true}} {
		source := original
		source.Deleted = state.deleted
		got, e := projectSafetyMessage(source, restored, state.erased, 0)
		if e != nil || !got.Deleted || got.Content != "" || got.ReplyTo != "" {
			t.Fatalf("restore resurrected deletion/erasure: %+v %v", got, e)
		}
	}
	stale := safety.ApplyVisibility(restored, visibility)
	if stale != restored {
		t.Fatal("older removal replaced a restored authoritative overlay")
	}
	overflow := original
	overflow.Revision = math.MaxInt64
	if _, err = projectSafetyMessage(overflow, restored, false, 0); !errors.Is(err, safety.ErrInvalid) {
		t.Fatalf("revision overflow: %v", err)
	}
	mismatched := restored
	mismatched.TargetID = uuid.NewString()
	if _, err = projectSafetyMessage(original, mismatched, false, 0); !errors.Is(err, safety.ErrInvalid) {
		t.Fatalf("overlay binding: %v", err)
	}
}

func TestRepair_66_CommentOverlayCodecAndCache(t *testing.T) {
	parent := uuid.NewString()
	source := interactions.CommentView{ID: uuid.NewString(), Pool: addr(1), Author: addr(2), Parent: &parent, Body: "appealed original", Likes: "2", Revision: "7"}
	visibility := safety.Visibility{TargetType: "comment", TargetID: source.ID, CaseID: uuid.NewString(), Revision: 1, Status: "removed"}
	event := projectSafetyComment(source, visibility, false, false, 8)
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := interactions.DecodePublicEvent(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !removed.Removed || removed.Body != "" || removed.Parent != "" {
		t.Fatalf("private original leaked into removal replay: %+v", removed)
	}
	visibility.Revision = 2
	visibility.Status = "restored"
	event = projectSafetyComment(source, visibility, false, false, 9)
	raw, err = json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := interactions.DecodePublicEvent(raw)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Removed || restored.Body != source.Body || restored.Parent != parent || restored.Revision <= removed.Revision {
		t.Fatalf("restoration replay: %+v", restored)
	}
	for _, state := range []struct{ deleted, erased bool }{{true, false}, {false, true}, {true, true}} {
		event = projectSafetyComment(source, visibility, state.deleted, state.erased, 10)
		raw, err = json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		got, e := interactions.DecodePublicEvent(raw)
		if e != nil || !got.Removed || got.Body != "" || got.Parent != "" {
			t.Fatalf("comment resurrection: %+v %v", got, e)
		}
	}
	first := commentSnapshotTag(source.Pool, "", 50, "1", []interactions.CommentView{source})
	source.CountAsOf = "new observation"
	if got := commentSnapshotTag(source.Pool, "", 50, "1", []interactions.CommentView{source}); got != first {
		t.Fatal("observation time changed content identity")
	}
	if got := commentSnapshotTag(source.Pool, "", 50, "2", []interactions.CommentView{source}); got == first {
		t.Fatal("restoration reused an older comment ETag")
	}
	if got := commentSnapshotTag(source.Pool, "", 50, "1", nil); got == first {
		t.Fatal("removal reused a visible comment ETag")
	}
	for _, path := range []string{"/v2/pools/" + source.Pool + "/comments", "/v2/social/pool/" + source.Pool + "/comments"} {
		pool, _, ok := commentListPath(path)
		if !ok || pool != source.Pool {
			t.Fatalf("comment read bypasses common overlay: %s", path)
		}
		rr := httptest.NewRecorder()
		(&socialComments{}).reads(chi.NewRouter()).ServeHTTP(rr, httptest.NewRequest("GET", path, nil))
		if rr.Code != 503 {
			t.Fatalf("unavailable overlay did not fail closed: %d %s", rr.Code, rr.Body.String())
		}
		if strings.Contains(path, "/v2/social/") {
			var value struct {
				Code      string `json:"code"`
				Retryable bool   `json:"retryable"`
			}
			if json.Unmarshal(rr.Body.Bytes(), &value) != nil || value.Code != "DEPENDENCY_UNAVAILABLE" || !value.Retryable {
				t.Fatalf("interaction authority contract: %s", rr.Body.String())
			}
		} else if err = v2.ValidateJSON("Error", rr.Body.Bytes()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRepair_66_FrozenProducerCodec(t *testing.T) {
	request := v2.CalloutCreateBody{PoolAddress: addr(1), Body: "new callout", Direction: "bullish", Disclosure: "organic", ClientCalloutId: uuid.NewString()}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var consumer v2.CalloutCreateBody
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v2/callouts", bytes.NewReader(raw))
	if !frozenDecode(rr, req, "CalloutCreateBody", &consumer) || consumer != request {
		t.Fatalf("frozen producer/consumer codec: %+v %s", consumer, rr.Body.String())
	}
	for _, raw := range [][]byte{[]byte(`{"poolAddress":"bad"}`), append(append([]byte(nil), raw...), []byte(` {}`)...), []byte(strings.Repeat(" ", 64<<10+1))} {
		rr = httptest.NewRecorder()
		req = httptest.NewRequest("POST", "/v2/callouts", bytes.NewReader(raw))
		if frozenDecode(rr, req, "CalloutCreateBody", &consumer) || rr.Code != 422 {
			t.Fatalf("malformed producer payload accepted: %d", rr.Code)
		}
	}
}

func TestRepair_66_FrozenReportContract(t *testing.T) {
	for state, want := range map[string]string{"open": "open", "triaged": "reviewing", "actioned": "resolved", "appealed": "reviewing", "dismissed": "dismissed"} {
		report := safety.Case{ID: uuid.NewString(), TargetType: "comment", TargetID: uuid.NewString(), State: state, Revision: 1}
		out, err := frozenReport(report, addr(2), "specific report explanation")
		if err != nil || out.State != want || out.Reason != "other" || out.ReportId != report.ID {
			t.Fatalf("safety to frozen report mapping: %+v %v", out, err)
		}
		raw, err := json.Marshal(out)
		if err != nil {
			t.Fatal(err)
		}
		if err = v2.ValidateJSON("Report", raw); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := frozenReport(safety.Case{State: "unknown"}, addr(2), "spam"); !errors.Is(err, interactions.ErrDependency) {
		t.Fatalf("unknown moderation state accepted: %v", err)
	}
}
