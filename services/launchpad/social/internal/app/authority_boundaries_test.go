package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
)

func TestSocialAuthorityRejectsUnverifiedAndMalformedRequests(t *testing.T) {
	h := newPlatformHarness(t)
	peer, err := x509.ParseCertificate(h.pki.rankingClient.Pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	runtime := &socialConnectedRuntime{pool: h.app.pool, binding: config.SocialRuntimeBinding{AuthorityPeers: map[string]string{"ranking.railway.internal": h.pki.rankingClient.SHA256}}}
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodGet, "/internal/v2/social/graph/invalid", "", 503},
		{http.MethodGet, "/internal/v2/social/visibility/comment/missing", "", 503},
		{http.MethodGet, "/internal/v2/social/visibility/unknown/id", "", 503},
		{http.MethodGet, "/internal/v2/social/visibility/too/many/parts", "", 503},
		{http.MethodGet, "/internal/v2/social/content-visibility?targetType=unknown&targetId=missing", "", 503},
		{http.MethodPost, "/internal/v2/social/callouts:review", "{", 503},
		{http.MethodPost, "/internal/v2/social/callouts:review", `{"Actor":"invalid","Pool":"invalid","Body":" "}`, 503},
		{http.MethodPost, "/internal/v2/social/callout-authority", `{}`, 503},
		{http.MethodPost, "/internal/v2/social/callout-authority", `{`, 503},
		{http.MethodGet, "/internal/v2/social/unknown", "", 404},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			request.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{peer}, VerifiedChains: [][]*x509.Certificate{{peer}}}
			response := httptest.NewRecorder()
			runtime.authorityHTTP(response, request)
			if response.Code != tc.status || response.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("authority request = %d %s", response.Code, response.Body.String())
			}
		})
	}
	unverified := httptest.NewRecorder()
	runtime.authorityHTTP(unverified, httptest.NewRequest(http.MethodGet, "/internal/v2/social/ranking-visibility", nil))
	if unverified.Code != http.StatusForbidden {
		t.Fatalf("unverified authority read = %d", unverified.Code)
	}
	if _, err := runtime.graphAuthority(context.Background(), "invalid"); err == nil {
		t.Fatal("invalid graph account accepted")
	}
	if _, err := runtime.visibilityAuthority(context.Background(), "unsupported", "id", ""); err != safety.ErrInvalid {
		t.Fatalf("unsupported visibility type = %v", err)
	}
	if _, err := runtime.reviewCallout(context.Background(), "invalid", "invalid", " "); err == nil {
		t.Fatal("blank callout accepted")
	}
}
