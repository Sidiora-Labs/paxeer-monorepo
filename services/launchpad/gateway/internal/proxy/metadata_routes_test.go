package proxy

import (
	"net/http"
	"testing"
)

// The token-create flow's /v3 publication calls and the web metadata
// client's legacy reads are admitted on the metadata upstream only, with
// exactly the identity policy media/metadata serves them under.
func TestMetadataPublicationRoutesBelongToMetadataUpstream(t *testing.T) {
	want := map[string]bool{
		"GET /v3/metadata-publication-authority":                            true,
		"POST /v3/tokens/{tokenAddress}/metadata-publications":              true,
		"GET /v3/tokens/{tokenAddress}/metadata-publications/{operationId}": true,
		"GET /metadata/batch":          false,
		"GET /metadata/{tokenAddress}": false,
		"GET /logo/{file}":             false,
		"GET /banner/{file}":           false,
	}
	for _, route := range metadataFrozenRouteExpectations() {
		want[route.Method+" "+route.Template] = route.Authenticate
	}
	for _, route := range MetadataConnectedRoutes() {
		key := route.Method + " " + route.Template
		authenticate, ok := want[key]
		frozen := metadataFrozenRouteExpectations()[key]
		if !ok || route.Authenticate != authenticate || route.Optional || route.Upload != frozen.Upload {
			t.Fatal("unexpected metadata route policy", route)
		}
		delete(want, key)
		if !allowedConnectedRoute("metadata", route) {
			t.Fatal("metadata route denied", route)
		}
		for _, other := range []string{"user", "social", "livestream"} {
			if allowedConnectedRoute(other, route) {
				t.Fatal("metadata route admitted on", other, route)
			}
		}
		flipped := route
		flipped.Authenticate = !route.Authenticate
		publicFrozen := frozen.Template != "" && !frozen.Authenticate
		if allowedConnectedRoute("metadata", flipped) != publicFrozen {
			t.Fatal("metadata route identity policy differs", flipped)
		}
		optional := route
		optional.Optional, optional.Authenticate = true, false
		if allowedConnectedRoute("metadata", optional) != publicFrozen {
			t.Fatal("metadata route optional policy differs", optional)
		}
		upload := route
		upload.Upload = !route.Upload
		if allowedConnectedRoute("metadata", upload) {
			t.Fatal("metadata route admitted with a different upload policy", upload)
		}
	}
	if len(want) != 0 {
		t.Fatal("metadata routes missing", want)
	}
	// Only the methods metadata registers: no other verb on these paths.
	for _, route := range []ConnectedRoute{
		{Method: http.MethodPost, Template: "/v3/metadata-publication-authority", Authenticate: true},
		{Method: http.MethodPut, Template: "/v3/tokens/{tokenAddress}/metadata-publications", Authenticate: true},
		{Method: http.MethodGet, Template: "/v3/tokens/{tokenAddress}/metadata-publications", Authenticate: true},
		{Method: http.MethodDelete, Template: "/v3/tokens/{tokenAddress}/metadata-publications/{operationId}", Authenticate: true},
		{Method: http.MethodPost, Template: "/metadata/batch", Authenticate: true},
		{Method: http.MethodPut, Template: "/logo/{file}", Authenticate: true, Upload: true},
		{Method: http.MethodGet, Template: "/v3/tokens/{tokenAddress}/metadata-publications/*", Authenticate: true},
	} {
		if allowedConnectedRoute("metadata", route) {
			t.Fatal("unregistered metadata route admitted", route)
		}
	}
	token := "0xce20da50f92d21d8bee497910c782e9aa25b6263"
	operation := "8a5f44d2-8ee0-42c5-b365-cffdd2e91723"
	if !connectedMatch(metadataPublications, "/v3/tokens/"+token+"/metadata-publications") || !connectedMatch(metadataPublication, "/v3/tokens/"+token+"/metadata-publications/"+operation) || connectedMatch(metadataPublication, "/v3/tokens/"+token+"/metadata-publications") || connectedMatch(metadataPublications, "/v3/tokens/"+token+"/metadata-publications/"+operation) {
		t.Fatal("publication templates confused")
	}
	if !connectedMatch(metadataLogo, "/logo/"+token+".png") || connectedMatch(metadataLogo, "/logo/a/b.png") {
		t.Fatal("logo template mismatch")
	}
	if MetadataPublicationBodyLimit != 36864 {
		t.Fatal("publication body limit drifted from media/metadata PublicationBodyLimit", MetadataPublicationBodyLimit)
	}
}
