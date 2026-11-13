package proxy

import (
	"bytes"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
)

func TestConnectedMetadataLogoPreservesApprovedImageBytes(t *testing.T) {
	pki := testpki.New(t)
	serverLeaf := pki.Leaf("metadata-server", "localhost")
	clientLeaf := pki.Leaf("gateway-client", "localhost")
	key := testpki.Ed25519(t)
	valid := append([]byte("\x89PNG\r\n\x1a\n"), []byte("owned-image")...)
	for _, tc := range []struct {
		name, contentType string
		body              []byte
		status            int
	}{
		{"approved", "image/png", valid, http.StatusOK},
		{"wrong content type", "application/octet-stream", valid, http.StatusBadGateway},
		{"short image", "image/png", []byte("short"), http.StatusBadGateway},
		{"wrong image signature", "image/png", []byte("not-png-image"), http.StatusBadGateway},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/logo/owned.png" || r.Method != http.MethodGet {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", tc.contentType)
				_, _ = w.Write(tc.body)
			}))
			server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{serverLeaf.Pair}, ClientCAs: pki.Roots(), ClientAuth: tls.RequireAndVerifyClientCert}
			server.StartTLS()
			defer server.Close()
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
			transport := &http.Transport{TLSClientConfig: clientTLS}
			defer transport.CloseIdleConnections()
			identity, err := sharedauth.NewMTLSIdentityAuthority(endpoint.String(), serverLeaf.SHA256, &http.Client{Transport: transport}, sharedauth.ServiceRequestIssuer{Issuer: "gateway", Audience: "identity", Key: key.Private, Transport: sharedauth.AssertionTransport{AssertionHeader: sharedauth.IdentityAssertionHeader, SignatureHeader: sharedauth.IdentitySignatureHeader}})
			if err != nil {
				t.Fatal(err)
			}
			defer identity.Client.CloseIdleConnections()
			connected, err := NewConnected(ConnectedDeps{Name: metadataServiceName, URL: endpoint.String(), Routes: MetadataConnectedRoutes(), Identity: identity, SigningKey: key.Private, TLS: clientTLS})
			if err != nil {
				t.Fatal(err)
			}
			defer connected.client.CloseIdleConnections()
			response := httptest.NewRecorder()
			connected.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/logo/owned.png", nil))
			if response.Code != tc.status {
				t.Fatalf("logo status=%d body=%s", response.Code, response.Body.String())
			}
			if tc.status == http.StatusOK {
				if !bytes.Equal(response.Body.Bytes(), valid) || response.Header().Get("Content-Type") != "image/png" || response.Header().Get("Content-Security-Policy") == "" {
					t.Fatalf("approved logo: headers=%v body=%q", response.Header(), response.Body.Bytes())
				}
			} else if !strings.Contains(response.Body.String(), "upstream_response_invalid") {
				t.Fatalf("invalid logo refusal: %s", response.Body.String())
			}
		})
	}
}
