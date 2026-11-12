package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/google/uuid"
)

func TestSocialAuthorityReadsCommittedGraphAndVisibility(t *testing.T) {
	h := newPlatformHarness(t)
	alice, bob := addr(0xa11ce), addr(0xb0b)
	account := h.wire.session("Bearer authority-alice", alice).AccountID
	bobAccount := h.wire.session("Bearer authority-bob", bob).AccountID
	h.wire.session("Bearer authority-creator", h.creator)
	if _, err := h.dbConn.Exec(context.Background(), `INSERT INTO chat.interaction_follows(follower,followee) VALUES($1,$2)`, alice, bob); err != nil {
		t.Fatal(err)
	}
	var storedEdges int
	var linkedAccount *string
	if err := h.dbConn.QueryRow(context.Background(), `SELECT count(*) FROM chat.interaction_follows WHERE follower=$1 AND followee=$2`, alice, bob).Scan(&storedEdges); err != nil || storedEdges != 1 {
		t.Fatalf("authoritative follow edges = %d: %v", storedEdges, err)
	}
	if err := h.dbConn.QueryRow(context.Background(), `SELECT chat.social_account_for_wallet($1)::text`, bob).Scan(&linkedAccount); err != nil || linkedAccount == nil {
		t.Fatalf("authoritative follow = %d, account %v: %v", storedEdges, linkedAccount, err)
	}
	runtime := &socialConnectedRuntime{pool: h.app.pool}
	graph, err := runtime.graphAuthority(context.Background(), account)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	var observed struct {
		Revision         uint64
		FollowedActorIDs []string
	}
	if err := json.Unmarshal(raw, &observed); err != nil || observed.Revision < 1 || len(observed.FollowedActorIDs) != 1 {
		t.Fatalf("committed graph = %+v: %v", observed, err)
	}
	if _, err := runtime.graphAuthority(context.Background(), "not-an-account"); err == nil {
		t.Fatal("invalid account accepted")
	}
	visible, err := runtime.rankingVisibility(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(visible)
	if err != nil {
		t.Fatal(err)
	}
	var ranking struct {
		Complete bool            `json:"complete"`
		Pools    map[string]bool `json:"pools"`
	}
	if err := json.Unmarshal(raw, &ranking); err != nil || !ranking.Complete || !ranking.Pools[h.pool] {
		t.Fatalf("ranking visibility = %+v: %v", ranking, err)
	}
	decision, err := runtime.reviewCallout(context.Background(), account, h.pool, "visible callout")
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(decision)
	if err != nil {
		t.Fatal(err)
	}
	var reviewed struct {
		ResourceID string
		Visible    bool
	}
	if err := json.Unmarshal(raw, &reviewed); err != nil || reviewed.ResourceID != h.pool || !reviewed.Visible {
		t.Fatalf("callout decision = %+v: %v", reviewed, err)
	}
	if _, err := runtime.reviewCallout(context.Background(), account, h.pool, " "); err == nil {
		t.Fatal("blank callout accepted")
	}
	content, err := runtime.contentVisibility(context.Background(), "token", h.pool)
	if err != nil || content.Status != "allowed" || content.TargetID != h.pool {
		t.Fatalf("token visibility = %+v: %v", content, err)
	}
	if _, err := runtime.contentVisibility(context.Background(), "message", uuid.NewString()); err != safety.ErrInvalid {
		t.Fatalf("unsupported content type = %v", err)
	}
	comment := data(h.expect(h.do(http.MethodPost, "/v2/pools/"+h.pool+"/comments", "", map[string]any{"body": "authority comment", "clientCommentId": uuid.NewString()}, bearer("authority-alice")), http.StatusCreated, "CommentCreateResponse"))["comment"].(map[string]any)
	visibility, err := runtime.visibilityAuthority(context.Background(), "comment", comment["commentId"].(string), account)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(visibility)
	if err != nil {
		t.Fatal(err)
	}
	var shown struct {
		ResourceID string
		Visible    bool
	}
	if err := json.Unmarshal(raw, &shown); err != nil || shown.ResourceID != comment["commentId"] || !shown.Visible {
		t.Fatalf("comment visibility = %+v: %v", shown, err)
	}
	if _, err := runtime.visibilityAuthority(context.Background(), "unknown", "id", ""); err != safety.ErrInvalid {
		t.Fatalf("unsupported visibility type = %v", err)
	}
	for _, target := range []struct{ kind, id string }{{"pool", h.pool}, {"token", h.token}} {
		value, err := runtime.visibilityAuthority(context.Background(), target.kind, target.id, account)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var visible struct{ Visible bool }
		if err := json.Unmarshal(raw, &visible); err != nil || !visible.Visible {
			t.Fatalf("canonical %s visibility = %+v: %v", target.kind, visible, err)
		}
	}
	commentID := comment["commentId"].(string)
	if _, err := h.dbConn.Exec(context.Background(), `INSERT INTO chat.interaction_blocks(owner,target) VALUES($1,$2)`, alice, bob); err != nil {
		t.Fatal(err)
	}
	blockedGraph, err := runtime.graphAuthority(context.Background(), account)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(blockedGraph)
	if err != nil {
		t.Fatal(err)
	}
	var blocked struct{ BlockedActorIDs []string }
	if err := json.Unmarshal(raw, &blocked); err != nil || len(blocked.BlockedActorIDs) != 1 || blocked.BlockedActorIDs[0] != bobAccount {
		t.Fatalf("committed block graph = %+v: %v", blocked, err)
	}
	blockedDecision, err := runtime.visibilityAuthority(context.Background(), "comment", commentID, bobAccount)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(blockedDecision)
	if err != nil {
		t.Fatal(err)
	}
	var hidden struct{ Visible bool }
	if err := json.Unmarshal(raw, &hidden); err != nil || hidden.Visible {
		t.Fatalf("blocked viewer visibility = %+v: %v", hidden, err)
	}
	denied := httptest.NewRecorder()
	runtime.authorityHTTP(denied, httptest.NewRequest(http.MethodGet, "/internal/v2/social/ranking-visibility", nil))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("authority without peer = %d", denied.Code)
	}
	peer, err := x509.ParseCertificate(h.pki.rankingClient.Pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	runtime.binding = config.SocialRuntimeBinding{AuthorityPeers: map[string]string{"ranking.railway.internal": h.pki.rankingClient.SHA256}}
	bound := func(method, path string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{peer}, VerifiedChains: [][]*x509.Certificate{{peer}}}
		rw := httptest.NewRecorder()
		runtime.authorityHTTP(rw, req)
		return rw
	}
	for _, path := range []string{
		"/internal/v2/social/graph/" + account,
		"/internal/v2/social/ranking-visibility",
		"/internal/v2/social/visibility/comment/" + comment["commentId"].(string) + "?accountId=" + account,
		"/internal/v2/social/content-visibility?targetType=token&targetId=" + h.pool,
	} {
		response := bound(http.MethodGet, path, nil)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("authority read %s = %d %s", path, response.Code, response.Body.String())
		}
	}
	callout := bound(http.MethodPost, "/internal/v2/social/callouts:review", []byte(`{"Actor":"`+account+`","Pool":"`+h.pool+`","Body":"visible callout"}`))
	if callout.Code != http.StatusOK || !bytes.Contains(callout.Body.Bytes(), []byte(`"Visible":true`)) {
		t.Fatalf("callout authority = %d %s", callout.Code, callout.Body.String())
	}
	calloutID := uuid.NewString()
	publication, err := json.Marshal(map[string]any{"TargetKind": "callout", "TargetID": calloutID, "Actor": account, "Revision": 1, "CreatedAt": time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	committed := bound(http.MethodPost, "/internal/v2/social/callout-authority", publication)
	if committed.Code != http.StatusOK || !bytes.Contains(committed.Body.Bytes(), []byte(`"committed":true`)) {
		t.Fatalf("callout publication = %d %s", committed.Code, committed.Body.String())
	}
	calloutVisibility := bound(http.MethodGet, "/internal/v2/social/visibility/callout/"+calloutID, nil)
	if calloutVisibility.Code != http.StatusOK || !bytes.Contains(calloutVisibility.Body.Bytes(), []byte(`"Visible":true`)) {
		t.Fatalf("callout visibility = %d %s", calloutVisibility.Code, calloutVisibility.Body.String())
	}
	missing := bound(http.MethodGet, "/internal/v2/social/unknown", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown authority path = %d", missing.Code)
	}
}
