package proxy

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
)

func TestConnectedProxyBoundsActualIngressAndEmptyResponse(t *testing.T) {
	rest, _, paths := connectedValidationHandlers(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"release-2"`)
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, tc := range []struct {
		name, code string
		prepare    func(*http.Request)
		status     int
	}{
		{"body read failure", "request_ingress_unavailable", func(r *http.Request) {
			reader, writer := io.Pipe()
			_ = writer.CloseWithError(errors.New("request body interrupted"))
			r.Body = reader
		}, http.StatusServiceUnavailable},
		{"ambiguous origin", "ambiguous_header", func(r *http.Request) {
			r.Header.Add("Origin", "https://kindle.test")
			r.Header.Add("Origin", "https://other.test")
		}, http.StatusBadRequest},
		{"empty upstream response", "", func(*http.Request) {}, http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/v2/streams", nil)
			tc.prepare(request)
			response := httptest.NewRecorder()
			rest.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("connected response: status=%d body=%s", response.Code, response.Body.String())
			}
			if tc.code != "" && !strings.Contains(response.Body.String(), tc.code) {
				t.Fatalf("connected response code %q absent: %s", tc.code, response.Body.String())
			}
			if tc.code == "" && (response.Body.Len() != 0 || response.Header().Get("ETag") != `"release-2"`) {
				t.Fatalf("empty upstream response: headers=%v body=%s", response.Header(), response.Body.String())
			}
		})
	}
	select {
	case path := <-paths:
		if path != "/v2/streams" {
			t.Fatalf("unexpected upstream route: %s", path)
		}
	default:
		t.Fatal("empty response did not reach upstream")
	}
}

func connectedValidationHandlers(t *testing.T, responses ...http.Handler) (*Connected, *ConnectedWS, <-chan string) {
	t.Helper()
	pki := testpki.New(t)
	serverLeaf := pki.Leaf("media-server", "localhost")
	clientLeaf := pki.Leaf("gateway-client", "localhost")
	key := testpki.Ed25519(t)
	paths := make(chan string, 4)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths <- r.URL.Path
		if len(responses) != 0 {
			responses[0].ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{serverLeaf.Pair}, ClientCAs: pki.Roots(), ClientAuth: tls.RequireAndVerifyClientCert}
	server.StartTLS()
	t.Cleanup(server.Close)
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, port, err := net.SplitHostPort(endpoint.Host)
	if err != nil {
		t.Fatal(err)
	}
	endpoint.Host = net.JoinHostPort("localhost", port)
	clientTLS := &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{clientLeaf.Pair}}
	identity, err := sharedauth.NewMTLSIdentityAuthority(endpoint.String(), serverLeaf.SHA256, &http.Client{Transport: &http.Transport{TLSClientConfig: clientTLS}}, sharedauth.ServiceRequestIssuer{Issuer: "gateway", Audience: "identity", Key: key.Private, Transport: sharedauth.AssertionTransport{AssertionHeader: sharedauth.IdentityAssertionHeader, SignatureHeader: sharedauth.IdentitySignatureHeader}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(identity.Client.CloseIdleConnections)
	rest, err := NewConnected(ConnectedDeps{Name: livestreamServiceName, URL: endpoint.String(), Routes: LivestreamConnectedRoutes(), Identity: identity, SigningKey: key.Private, TLS: clientTLS})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rest.client.CloseIdleConnections)
	socket, err := NewConnectedWS(ConnectedWSDeps{URL: endpoint.String(), Identity: identity, SigningKey: key.Private, TLS: clientTLS, Origins: []string{"https://kindle.test"}})
	if err != nil {
		t.Fatal(err)
	}
	return rest, socket, paths
}

func TestConnectedProxyPreservesBoundedUpstreamResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body, etag string
		upstream         int
		want             int
		code             string
	}{
		{"content", `{"state":"ready"}`, "", http.StatusOK, http.StatusOK, ""},
		{"not modified", "", `"revision-1"`, http.StatusNotModified, http.StatusNotModified, ""},
		{"redirect", "", "", http.StatusFound, http.StatusBadGateway, "upstream_redirect_denied"},
		{"service unavailable", `{"code":"unavailable"}`, "", http.StatusServiceUnavailable, http.StatusServiceUnavailable, ""},
		{"invalid JSON", "not-json", "", http.StatusOK, http.StatusBadGateway, "upstream_response_invalid"},
		{"private field", `{"privateKey":"owned"}`, "", http.StatusOK, http.StatusBadGateway, "upstream_response_restricted"},
		{"private list", `[{"authorization":"owned"}]`, "", http.StatusOK, http.StatusBadGateway, "upstream_response_restricted"},
		{"signed object URL", `{"url":"https://kindle.test/object?X-Amz-Signature=owned"}`, "", http.StatusOK, http.StatusBadGateway, "upstream_response_restricted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rest, _, paths := connectedValidationHandlers(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.etag != "" {
					w.Header().Set("ETag", tc.etag)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.upstream)
				_, _ = w.Write([]byte(tc.body))
			}))
			response := httptest.NewRecorder()
			rest.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/streams", nil))
			if response.Code != tc.want || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("bounded upstream response: status=%d body=%s", response.Code, response.Body.String())
			}
			if tc.code != "" && !strings.Contains(response.Body.String(), tc.code) {
				t.Fatalf("response code %q absent: %s", tc.code, response.Body.String())
			}
			if tc.want == http.StatusOK && response.Body.String() != tc.body {
				t.Fatalf("content response=%q, want %q", response.Body.String(), tc.body)
			}
			if tc.etag != "" && response.Header().Get("ETag") != tc.etag {
				t.Fatalf("conditional response etag=%q", response.Header().Get("ETag"))
			}
			select {
			case path := <-paths:
				if path != "/v2/streams" {
					t.Fatalf("upstream path=%q", path)
				}
			default:
				t.Fatal("upstream was not called")
			}
		})
	}
}

func TestConnectedProxyRejectsInvalidIngressBeforeUpstream(t *testing.T) {
	rest, socket, paths := connectedValidationHandlers(t)
	for _, tc := range []struct {
		name, path, method, code string
		body                     []byte
		headers                  http.Header
		status                   int
	}{
		{name: "unregistered route", path: "/v2/not-served", method: http.MethodGet, status: http.StatusNotFound},
		{name: "unsafe path", path: "/v2/streams/../private", method: http.MethodGet, status: http.StatusNotFound},
		{name: "oversize body", path: "/v2/streams", method: http.MethodGet, body: bytes.Repeat([]byte("a"), (1<<20)+1), status: http.StatusRequestEntityTooLarge, code: "request_too_large"},
		{name: "private ingest without session", path: "/v2/streams/stream-id/ingest-key", method: http.MethodPost, status: http.StatusUnauthorized, code: "identity_unavailable"},
		{name: "authenticated write without session", path: "/v2/streams", method: http.MethodPost, status: http.StatusForbidden, code: "identity_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(tc.body))
			request.Header = tc.headers.Clone()
			response := httptest.NewRecorder()
			rest.ServeHTTP(response, request)
			if response.Code != tc.status || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("ingress status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
			}
			if tc.code != "" {
				var payload map[string]string
				if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload["code"] != tc.code {
					t.Fatalf("ingress code=%v err=%v", payload, err)
				}
			}
			select {
			case path := <-paths:
				t.Fatalf("invalid ingress reached upstream: %s", path)
			default:
			}
		})
	}
	request := httptest.NewRequest(http.MethodGet, "/v2/streams", nil)
	response := httptest.NewRecorder()
	rest.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("admitted public route: status=%d body=%s", response.Code, response.Body.String())
	}
	select {
	case path := <-paths:
		if path != "/v2/streams" {
			t.Fatalf("forwarded path=%q", path)
		}
	default:
		t.Fatal("admitted public route did not reach upstream")
	}
	for _, tc := range []struct {
		name, path, origin, protocols string
		status                        int
		code                          string
	}{
		{"missing upgrade", "/v2/stream", "https://kindle.test", "", http.StatusBadRequest, "socket_request_invalid"},
		{"wrong origin", "/v2/stream", "https://other.test", "sidiora.v2,ticket." + strings.Repeat("a", 64), http.StatusForbidden, "origin_denied"},
		{"unexpected query", "/v2/stream?other=1", "https://kindle.test", "sidiora.v2,ticket." + strings.Repeat("a", 64), http.StatusUnauthorized, "socket_credentials_invalid"},
		{"duplicate protocol", "/v2/stream", "https://kindle.test", "sidiora.v2,sidiora.v2,ticket." + strings.Repeat("a", 64), http.StatusBadRequest, "socket_protocol_invalid"},
		{"duplicate ticket", "/v2/stream", "https://kindle.test", "sidiora.v2,ticket." + strings.Repeat("a", 64) + ",ticket." + strings.Repeat("b", 64), http.StatusUnauthorized, "socket_ticket_invalid"},
		{"invalid ticket", "/v2/stream", "https://kindle.test", "sidiora.v2,ticket.invalid", http.StatusUnauthorized, "socket_ticket_invalid"},
		{"duplicate query", "/v2/stream?topics=a&topics=b", "https://kindle.test", "sidiora.v2,ticket." + strings.Repeat("a", 64), http.StatusUnauthorized, "socket_credentials_invalid"},
		{"unknown protocol", "/v2/stream", "https://kindle.test", "unexpected,ticket." + strings.Repeat("a", 64), http.StatusBadRequest, "socket_protocol_invalid"},
		{"missing protocol", "/v2/stream", "https://kindle.test", "ticket." + strings.Repeat("a", 64), http.StatusUnauthorized, "socket_ticket_invalid"},
		{"missing ticket", "/v2/stream", "https://kindle.test", "sidiora.v2", http.StatusUnauthorized, "socket_ticket_invalid"},
		{"uppercase ticket", "/v2/stream", "https://kindle.test", "sidiora.v2,ticket." + strings.Repeat("A", 64), http.StatusUnauthorized, "socket_ticket_invalid"},
		{"upstream socket unavailable", "/v2/stream", "https://kindle.test", "sidiora.v2,ticket." + strings.Repeat("a", 64), http.StatusServiceUnavailable, "socket_upstream_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			request.Header.Set("Origin", tc.origin)
			if tc.name != "missing upgrade" {
				request.Header.Set("Connection", "Upgrade")
				request.Header.Set("Upgrade", "websocket")
				request.Header.Set("Sec-WebSocket-Version", "13")
				request.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
				request.Header.Set("Sec-WebSocket-Protocol", tc.protocols)
			}
			response := httptest.NewRecorder()
			socket.ServeHTTP(response, request)
			var payload map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || response.Code != tc.status || payload["code"] != tc.code || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("socket refusal: status=%d code=%v err=%v body=%s", response.Code, payload, err, response.Body.String())
			}
		})
	}
	select {
	case path := <-paths:
		if path != "/v2/stream" {
			t.Fatalf("unexpected socket upstream path: %s", path)
		}
	default:
		t.Fatal("bound socket ticket did not reach the upstream")
	}
	select {
	case path := <-paths:
		t.Fatalf("additional socket ingress reached upstream: %s", path)
	default:
	}
}

func TestConnectedSocketEnforcesCapacityAndSessionAuthority(t *testing.T) {
	_, socket, paths := connectedValidationHandlers(t)
	request := func(authorization string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/v2/stream?topics=room,account", nil)
		r.Header.Set("Origin", "https://kindle.test")
		r.Header.Set("Connection", "Upgrade")
		r.Header.Set("Upgrade", "websocket")
		r.Header.Set("Sec-WebSocket-Version", "13")
		r.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
		r.Header.Set("Sec-WebSocket-Protocol", "sidiora.v2,ticket."+strings.Repeat("a", 64))
		if authorization != "" {
			r.Header.Set("Authorization", authorization)
		}
		return r
	}
	for index := 0; index < cap(socket.slots); index++ {
		socket.slots <- struct{}{}
	}
	full := httptest.NewRecorder()
	socket.ServeHTTP(full, request(""))
	if full.Code != http.StatusServiceUnavailable || full.Header().Get("Retry-After") != "1" || !strings.Contains(full.Body.String(), "socket_capacity_unavailable") {
		t.Fatalf("socket capacity: status=%d headers=%v body=%s", full.Code, full.Header(), full.Body.String())
	}
	for index := 0; index < cap(socket.slots); index++ {
		<-socket.slots
	}
	session := httptest.NewRecorder()
	socket.ServeHTTP(session, request("Bearer invalid"))
	if session.Code != http.StatusServiceUnavailable || !strings.Contains(session.Body.String(), "socket_identity_unavailable") {
		t.Fatalf("socket session authority: status=%d body=%s", session.Code, session.Body.String())
	}
	select {
	case path := <-paths:
		if path != "/internal/v2/identity/request:authenticate" {
			t.Fatalf("unexpected authority path: %s", path)
		}
	default:
		t.Fatal("session authority was not consulted")
	}
	select {
	case path := <-paths:
		t.Fatalf("unauthorized socket reached stream upstream: %s", path)
	default:
	}
}
