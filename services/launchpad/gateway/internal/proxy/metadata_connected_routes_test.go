package proxy

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
)

func metadataFrozenRouteExpectations() map[string]ConnectedRoute {
	routes := []ConnectedRoute{
		{Method: http.MethodGet, Template: "/v2/tokens/{tokenAddress}/metadata"},
		{Method: http.MethodPatch, Template: "/v2/tokens/{tokenAddress}/metadata", Authenticate: true},
		{Method: http.MethodGet, Template: "/v2/tokens/{tokenAddress}/media"},
		{Method: http.MethodGet, Template: "/v2/tokens/{tokenAddress}/assets/{assetId}", Authenticate: true},
		{Method: http.MethodPut, Template: "/v2/tokens/{tokenAddress}/assets/{assetId}", Authenticate: true, Upload: true},
		{Method: http.MethodGet, Template: "/v2/accounts/{wallet}/assets/{assetId}", Authenticate: true},
		{Method: http.MethodPut, Template: "/v2/accounts/{wallet}/assets/{assetId}", Authenticate: true, Upload: true},
	}
	want := make(map[string]ConnectedRoute, len(routes))
	for _, route := range routes {
		want[route.Method+" "+route.Template] = route
	}
	return want
}

func TestMetadataConnectedRoutesFrozenSurface(t *testing.T) {
	want := metadataFrozenRouteExpectations()
	seen := map[string]bool{}
	for _, route := range MetadataConnectedRoutes() {
		key := route.Method + " " + route.Template
		if seen[key] {
			t.Fatalf("duplicate metadata route: %s", key)
		}
		seen[key] = true
		if expected, ok := want[key]; ok && route != expected {
			t.Fatalf("metadata route: %+v, want %+v", route, expected)
		}
	}
	for template, methods := range FrozenConnectedSurface()[metadataServiceName] {
		for _, method := range strings.Fields(methods) {
			key := method + " " + template
			if _, ok := want[key]; !ok || !seen[key] {
				t.Fatalf("frozen metadata route missing: %s", key)
			}
			delete(want, key)
		}
	}
	if len(want) != 0 {
		t.Fatalf("metadata surface missing routes: %+v", want)
	}
	if !reflect.DeepEqual(MetadataConnectedRoutes(), MetadataConnectedRoutes()) {
		t.Fatal("metadata route order is not stable")
	}
	TestMetadataPublicationRoutesBelongToMetadataUpstream(t)
}

func TestMetadataConnectedRoutesPolicies(t *testing.T) {
	frozen := metadataFrozenRouteExpectations()
	for _, route := range MetadataConnectedRoutes() {
		for flags := 0; flags < 8; flags++ {
			t.Run(fmt.Sprintf("%s %s/%d", route.Method, route.Template, flags), func(t *testing.T) {
				candidate := route
				candidate.Authenticate, candidate.Optional, candidate.Upload = flags&1 != 0, flags&2 != 0, flags&4 != 0
				want := candidate == route
				if expected, ok := frozen[route.Method+" "+route.Template]; ok && !expected.Authenticate {
					want = !candidate.Upload && !(candidate.Authenticate && candidate.Optional)
				}
				if got := allowedConnectedRoute(metadataServiceName, candidate); got != want {
					t.Fatalf("metadata policy: %+v admitted=%t, want %t", candidate, got, want)
				}
			})
		}
	}
	for _, route := range append(RetiredConnectedRoutes(), ConnectedRoute{Method: http.MethodGet, Template: "/v2/tokens/{tokenAddress}/unknown"}) {
		if allowedConnectedRoute(metadataServiceName, route) {
			t.Fatalf("unknown or retired metadata route admitted: %+v", route)
		}
	}
}

func TestMetadataConnectedRoutesExampleDocument(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "deploy", "launchpad", "documents", "values", "gateway.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Runtime struct {
			Upstreams []struct {
				Name   string
				URL    string
				Routes []ConnectedRoute
			}
		}
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	pki := testpki.New(t)
	client := pki.Leaf("gateway-client", "localhost")
	server := pki.Leaf("metadata-server", "localhost")
	key := testpki.Ed25519(t)
	clientTLS := &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{client.Pair}}
	transport := &http.Transport{TLSClientConfig: clientTLS}
	t.Cleanup(transport.CloseIdleConnections)
	identity, err := sharedauth.NewMTLSIdentityAuthority("https://localhost", server.SHA256, &http.Client{Transport: transport}, sharedauth.ServiceRequestIssuer{Issuer: "gateway", Audience: "identity", Key: key.Private, Transport: sharedauth.AssertionTransport{AssertionHeader: sharedauth.IdentityAssertionHeader, SignatureHeader: sharedauth.IdentitySignatureHeader}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, upstream := range document.Runtime.Upstreams {
		if upstream.Name == metadataServiceName {
			found = true
			deps := ConnectedDeps{Name: upstream.Name, URL: upstream.URL, Routes: upstream.Routes, Identity: identity, SigningKey: key.Private, TLS: clientTLS}
			connected, err := NewConnected(deps)
			if err != nil {
				t.Fatalf("example metadata routes: %v", err)
			}
			t.Cleanup(connected.client.CloseIdleConnections)
			if !reflect.DeepEqual(connected.routes, upstream.Routes) {
				t.Fatalf("document route entries changed: %+v", connected.routes)
			}
			for _, route := range MetadataConnectedRoutes() {
				deps.Routes = []ConnectedRoute{route}
				accepted, err := NewConnected(deps)
				if err != nil {
					t.Fatalf("code metadata route: %+v: %v", route, err)
				}
				accepted.client.CloseIdleConnections()
				deps.Routes[0].Upload = !route.Upload
				if _, err := NewConnected(deps); err == nil {
					t.Fatalf("changed upload policy admitted: %+v", deps.Routes[0])
				}
			}
			deps.Routes = append(append([]ConnectedRoute(nil), upstream.Routes...), upstream.Routes[0])
			if _, err := NewConnected(deps); err == nil {
				t.Fatal("duplicate metadata route admitted")
			}
			deps.Routes = make([]ConnectedRoute, 257)
			if _, err := NewConnected(deps); err == nil || err.Error() != "private media channel unavailable" {
				t.Fatalf("metadata route bound: %v", err)
			}
		}
	}
	if !found {
		t.Fatal("example metadata upstream missing")
	}
}
