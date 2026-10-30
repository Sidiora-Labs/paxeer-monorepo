package proxy

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
)

func TestLivestreamWebhookForwardsProviderSignature(t *testing.T) {
	pki := testpki.New(t)
	server := pki.Leaf("livestream-server", "livestream.test")
	client := pki.Leaf("gateway-client", "gateway.test")
	key := testpki.Ed25519(t)
	type upstreamRequest struct {
		method, path string
		body         []byte
		signatures   []string
		err          error
	}
	received := make(chan upstreamRequest, 16)
	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		received <- upstreamRequest{r.Method, r.URL.Path, body, r.Header.Values("Livepeer-Signature"), err}
		w.WriteHeader(http.StatusNoContent)
	}))
	upstream.TLS = &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{server.Pair},
		ClientCAs:    pki.Roots(),
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	upstream.StartTLS()
	t.Cleanup(upstream.Close)
	clientTLS := &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{client.Pair}}
	transport := &http.Transport{TLSClientConfig: clientTLS}
	t.Cleanup(transport.CloseIdleConnections)
	identity, err := sharedauth.NewMTLSIdentityAuthority(upstream.URL, server.SHA256, &http.Client{Transport: transport}, sharedauth.ServiceRequestIssuer{
		Issuer: "gateway", Audience: "identity", Key: key.Private,
		Transport: sharedauth.AssertionTransport{AssertionHeader: sharedauth.IdentityAssertionHeader, SignatureHeader: sharedauth.IdentitySignatureHeader},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(identity.Client.CloseIdleConnections)
	livestream, err := NewConnected(ConnectedDeps{Name: "livestream", URL: upstream.URL, Routes: LivestreamConnectedRoutes(), Identity: identity, SigningKey: key.Private, TLS: clientTLS})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(livestream.client.CloseIdleConnections)
	user, err := NewConnected(ConnectedDeps{Name: "user", URL: upstream.URL, Routes: UserConnectedRoutes(), Identity: identity, SigningKey: key.Private, TLS: clientTLS})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(user.client.CloseIdleConnections)
	body := []byte(" {\n\t\"timestamp\": 1700000000, \"event\": \"stream.started\", \"text\": \"\\u0061\"\n}\r\n")
	const signature = "t=1700000000,v1=0123456789abcdef,v1=fedcba9876543210"
	for _, tc := range []struct {
		name, method, path string
		proxy              *Connected
		signatures         []string
		wantSignature      string
		wantStatus         int
	}{
		{"single signature", http.MethodPost, "/v2/provider-webhooks/livepeer", livestream, []string{signature}, signature, http.StatusNoContent},
		{"duplicate signatures", http.MethodPost, "/v2/provider-webhooks/livepeer", livestream, []string{signature, "t=1700000001,v1=abcdef"}, "", http.StatusBadRequest},
		{"repeated identical signature", http.MethodPost, "/v2/provider-webhooks/livepeer", livestream, []string{signature, signature}, "", http.StatusBadRequest},
		{"no signature", http.MethodPost, "/v2/provider-webhooks/livepeer", livestream, nil, "", http.StatusNoContent},
		{"other livestream route", http.MethodGet, "/v2/streams/live", livestream, []string{signature}, "", http.StatusNoContent},
		{"other upstream", http.MethodGet, "/v2/profiles:batch", user, []string{signature}, "", http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			for _, value := range tc.signatures {
				req.Header.Add("Livepeer-Signature", value)
			}
			rec := httptest.NewRecorder()
			tc.proxy.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusBadRequest {
				var response map[string]string
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil || response["code"] != "ambiguous_header" {
					t.Fatalf("expected ambiguous_header: %s (decode error: %v)", rec.Body.String(), err)
				}
			} else {
				select {
				case got := <-received:
					if got.err != nil || got.method != tc.method || got.path != tc.path || !bytes.Equal(got.body, body) {
						t.Fatalf("upstream request = %+v, want %s %s with exact body %q", got, tc.method, tc.path, body)
					}
					if tc.wantSignature != "" {
						if len(got.signatures) != 1 || got.signatures[0] != tc.wantSignature {
							t.Fatalf("signatures = %q, want [%q]", got.signatures, tc.wantSignature)
						}
					} else if len(got.signatures) != 0 {
						t.Fatalf("unexpected forwarded signature: %q", got.signatures)
					}
				default:
					t.Fatal("request did not reach upstream")
				}
			}
			select {
			case got := <-received:
				t.Fatalf("unexpected upstream request: %+v", got)
			default:
			}
		})
	}
}

func TestSocialConnectedRoutesForwardFrozenVoiceTemplates(t *testing.T) {
	want := []ConnectedRoute{
		{Method: http.MethodPost, Template: "/v3/voice-rooms", Authenticate: true},
		{Method: http.MethodPost, Template: "/v3/voice-rooms/{roomId}/moderation-actions", Authenticate: true},
		{Method: http.MethodPost, Template: "/v3/voice-rooms/{roomId}/tickets", Authenticate: true},
		{Method: http.MethodGet, Template: "/v2/voice-rooms/{roomId}", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/voice-rooms/{roomId}:end", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/voice-rooms/{roomId}:leave", Authenticate: true},
	}
	routes := SocialConnectedRoutes()
	for _, expected := range want {
		t.Run(expected.Method+" "+expected.Template, func(t *testing.T) {
			found := 0
			for _, route := range routes {
				if route.Method != expected.Method || route.Template != expected.Template {
					continue
				}
				found++
				if route != expected {
					t.Errorf("voice route = %#v, want %#v", route, expected)
				}
				if !allowedConnectedRoute("social", route) {
					t.Errorf("social upstream rejected voice route %#v", route)
				}
			}
			if found != 1 {
				t.Errorf("found %d voice routes, want exactly one", found)
			}
		})
	}
}

func TestLivestreamConnectedRoutesForwardFrozenStreamTemplates(t *testing.T) {
	want := []ConnectedRoute{
		{Method: http.MethodGet, Template: "/v2/streams"},
		{Method: http.MethodPost, Template: "/v2/streams/{streamId}:start", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/streams/{streamId}:end", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/streams/{streamId}/viewers:heartbeat"},
		{Method: http.MethodPost, Template: "/v2/provider-webhooks/livepeer"},
	}
	routes := LivestreamConnectedRoutes()
	for _, expected := range want {
		t.Run(expected.Method+" "+expected.Template, func(t *testing.T) {
			found := 0
			for _, route := range routes {
				if route.Method != expected.Method || route.Template != expected.Template {
					continue
				}
				found++
				if route != expected {
					t.Errorf("stream route = %#v, want %#v", route, expected)
				}
				if !allowedConnectedRoute("livestream", route) {
					t.Errorf("livestream upstream rejected stream route %#v", route)
				}
			}
			if found != 1 {
				t.Errorf("found %d stream routes, want exactly one", found)
			}
		})
	}
}
