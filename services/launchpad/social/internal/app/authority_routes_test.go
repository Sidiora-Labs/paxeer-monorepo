package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/discovery"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/google/uuid"
)

func TestAuthorityRoutesAreSharedByContainerAndConnectedRuntimes(t *testing.T) {
	h := newPlatformHarness(t)
	alice := addr(0xa11ce)
	principal := h.wire.session("Bearer authority-routes", alice)
	h.wire.session("Bearer authority-creator", h.creator)

	if _, err := h.dbConn.Exec(context.Background(), `INSERT INTO chat.interaction_follows(follower,followee) VALUES($1,$2)`, alice, h.creator); err != nil {
		t.Fatal(err)
	}
	callout := data(h.expect(h.do(http.MethodPost, "/v2/callouts", "", map[string]any{
		"poolAddress":     h.pool,
		"body":            "authority route callout",
		"direction":       "neutral",
		"disclosure":      "organic",
		"clientCalloutId": uuid.NewString(),
	}, bearer("authority-routes")), http.StatusCreated, "CalloutCreateResponse"))["callout"].(map[string]any)
	comment := data(h.expect(h.do(http.MethodPost, "/v2/pools/"+h.pool+"/comments", "", map[string]any{
		"body":            "authority route comment",
		"clientCommentId": uuid.NewString(),
	}, bearer("authority-routes")), http.StatusCreated, "CommentCreateResponse"))["comment"].(map[string]any)
	calloutID := callout["calloutId"].(string)
	commentID := comment["commentId"].(string)
	publicationID := uuid.NewString()
	publication, err := json.Marshal(map[string]any{
		"TargetKind": "callout",
		"TargetID":   publicationID,
		"Actor":      principal.AccountID,
		"Revision":   1,
		"CreatedAt":  time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}

	peer, err := x509.ParseCertificate(h.pki.rankingClient.Pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	other := &socialConnectedRuntime{pool: h.app.pool, binding: configBinding(h.pki.rankingClient.SHA256)}
	request := func(handler http.HandlerFunc, method, path string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{peer}, VerifiedChains: [][]*x509.Certificate{{peer}}}
		rw := httptest.NewRecorder()
		handler(rw, req)
		return rw
	}

	type route struct {
		name, method, path string
		body               []byte
	}
	routes := []route{
		{"ranking-visibility", http.MethodGet, "/internal/v2/social/ranking-visibility", nil},
		{"content-visibility", http.MethodGet, "/internal/v2/social/content-visibility?targetType=token&targetId=" + h.pool, nil},
		{"graph", http.MethodGet, "/internal/v2/social/graph/" + principal.AccountID, nil},
		{"visibility", http.MethodGet, "/internal/v2/social/visibility/comment/" + commentID + "?accountId=" + principal.AccountID, nil},
		{"public", http.MethodGet, "/internal/v2/social/public/callout/" + calloutID, nil},
		{"callouts:review", http.MethodPost, "/internal/v2/social/callouts:review", []byte(`{"Actor":"` + principal.AccountID + `","Pool":"` + h.pool + `","Body":"route review"}`)},
		{"callout-authority", http.MethodPost, "/internal/v2/social/callout-authority", publication},
	}
	for _, tc := range routes {
		t.Run(tc.name, func(t *testing.T) {
			private := h.pki.mtlsClient(h.pki.rankingClient)
			response, err := private.Do(mustRequest(t, tc.method, h.server.URL+tc.path, tc.body))
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode == http.StatusNotFound {
				t.Fatalf("container profile route returned not found: %s", tc.path)
			}
			if response.StatusCode != http.StatusOK {
				t.Fatalf("container profile route %s = %d", tc.path, response.StatusCode)
			}
			connected := other.authorityResponse(tc.method, tc.path, tc.body, peer)
			if connected.Code == http.StatusNotFound || connected.Code != http.StatusOK {
				t.Fatalf("connected runtime route %s = %d %s", tc.path, connected.Code, connected.Body.String())
			}
		})
	}

	for _, tc := range routes {
		t.Run("outside-"+tc.name, func(t *testing.T) {
			response, err := h.client.Do(mustRequest(t, tc.method, h.server.URL+tc.path, tc.body))
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusForbidden {
				t.Fatalf("outside authority route %s = %d", tc.path, response.StatusCode)
			}
		})
	}

	graph := request(other.authorityHTTP, http.MethodGet, "/internal/v2/social/graph/"+principal.AccountID, nil)
	var graphValue discovery.GraphSnapshot
	if err := json.Unmarshal(graph.Body.Bytes(), &graphValue); err != nil || graphValue.AccountID != principal.AccountID || graphValue.Revision == 0 || graphValue.ObservedAt.IsZero() {
		t.Fatalf("graph response = %+v: %v", graphValue, err)
	}
	visibility := request(other.authorityHTTP, http.MethodGet, "/internal/v2/social/visibility/comment/"+commentID+"?accountId="+principal.AccountID, nil)
	var visibilityValue discovery.ModerationDecision
	if err := json.Unmarshal(visibility.Body.Bytes(), &visibilityValue); err != nil || visibilityValue.ResourceID != commentID || visibilityValue.Revision == 0 || visibilityValue.ObservedAt.IsZero() {
		t.Fatalf("visibility response = %+v: %v", visibilityValue, err)
	}
	public := request(other.authorityHTTP, http.MethodGet, "/internal/v2/social/public/callout/"+calloutID, nil)
	var publicValue discovery.PublicResource
	if err := json.Unmarshal(public.Body.Bytes(), &publicValue); err != nil || publicValue.Ref.Kind != "callout" || publicValue.Ref.ID != calloutID || !publicValue.Ref.Public || !publicValue.Ref.Authorized {
		t.Fatalf("public response = %+v: %v", publicValue, err)
	}
	if missing := request(other.authorityHTTP, http.MethodGet, "/internal/v2/social/public/callout/"+uuid.NewString(), nil); missing.Code != http.StatusNotFound {
		t.Fatalf("missing public resource = %d", missing.Code)
	}
	if content := request(other.authorityHTTP, http.MethodGet, "/internal/v2/social/content-visibility?targetType=token&targetId="+h.pool, nil); content.Code != http.StatusOK {
		t.Fatalf("content visibility = %d %s", content.Code, content.Body.String())
	} else {
		var value safety.Visibility
		if err := json.Unmarshal(content.Body.Bytes(), &value); err != nil || value.TargetID != h.pool {
			t.Fatalf("content visibility response = %+v: %v", value, err)
		}
	}
}

func configBinding(peerDigest string) config.SocialRuntimeBinding {
	return config.SocialRuntimeBinding{AuthorityPeers: map[string]string{"ranking.railway.internal": peerDigest}}
}

func (r *socialConnectedRuntime) authorityResponse(method, path string, body []byte, peer *x509.Certificate) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{peer}, VerifiedChains: [][]*x509.Certificate{{peer}}}
	rw := httptest.NewRecorder()
	r.authorityHTTP(rw, req)
	return rw
}

func mustRequest(t *testing.T, method, target string, body []byte) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, target, bytes.NewReader(body))
	if err != nil {
		t.Fatal(fmt.Errorf("authority request: %w", err))
	}
	if len(body) != 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}
