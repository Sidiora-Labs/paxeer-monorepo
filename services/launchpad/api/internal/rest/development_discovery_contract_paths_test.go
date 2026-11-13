package rest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/discovery"
	"github.com/go-chi/chi/v5"
)

func TestDevelopmentDiscoveryBoundsAnonymousRequests(t *testing.T) {
	d := devDiscovery{}
	for _, check := range []struct {
		name, path, code string
		status           int
		handler          http.HandlerFunc
	}{
		{"feed query", "/v2/discovery/feed?unexpected=1", literalQUERYINVALID, 400, d.feed},
		{"feed limit", "/v2/discovery/feed?limit=00", literalFIELDINVALID, 422, d.feed},
		{"feed order", "/v2/discovery/feed?order=unknown", literalFIELDINVALID, 422, d.feed},
		{"feed cursor", "/v2/discovery/feed?cursor=invalid", literalCURSORINVALID, 400, d.feed},
		{"detail query", "/v2/discovery/feed/item?unknown=1", literalQUERYINVALID, 400, d.feedDetail},
		{"detail id", "/v2/discovery/feed/item", literalFIELDINVALID, 422, d.feedDetail},
		{"leaderboard query", "/v2/discovery/leaderboards?unknown=1", literalQUERYINVALID, 400, d.leaderboards},
		{"leaderboard range", "/v2/discovery/leaderboards?start=2026-10-10T00:00:00Z&end=2026-10-09T00:00:00Z", literalFIELDINVALID, 422, d.leaderboards},
		{"callout query", "/v2/discovery/callouts?unknown=1", literalQUERYINVALID, 400, d.poolCallouts},
		{"callout pool", "/v2/discovery/callouts", literalFIELDINVALID, 422, d.poolCallouts},
	} {
		t.Run(check.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, check.path, nil)
			response := httptest.NewRecorder()
			check.handler(response, request)
			if response.Code != check.status || !strings.Contains(response.Body.String(), `"code":"`+check.code+`"`) {
				t.Fatalf("%s: %d %s", check.path, response.Code, response.Body.String())
			}
		})
	}
	for _, token := range []string{"", "!", strings.Repeat("x", 2050)} {
		if key, err := decodeDiscoveryKey(token, 2048); err == nil || key != nil {
			t.Fatalf("invalid discovery cursor accepted: %q %+v", token, key)
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/v2/discovery/feed?unknown=%ZZ", nil)
	if _, ok := strictQuery(request, "order"); ok {
		t.Fatal("malformed discovery query accepted")
	}
}

func TestDevelopmentDiscoveryRespectsSourceVisibility(t *testing.T) {
	d := devDiscovery{}
	removed, err := d.resolve(context.Background(), discovery.SourceRef{Tombstone: true})
	if err != nil || removed.State != "removed" {
		t.Fatalf("removed source: %+v %v", removed, err)
	}
	private, err := d.resolve(context.Background(), discovery.SourceRef{Public: false, Authorized: true})
	if !errors.Is(err, discovery.ErrForbidden) || private.State != literalUnavailable {
		t.Fatalf("private source: %+v %v", private, err)
	}
	response := httptest.NewRecorder()
	writeDiscoveryDocument(response, httptest.NewRequest(http.MethodGet, "/v2/discovery/feed", nil), "ConnectedDiscoveryFeedPage", map[string]any{"entries": "invalid"})
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), literalINTEGRITYQUARANTINED) {
		t.Fatalf("invalid discovery document: %d %s", response.Code, response.Body.String())
	}
	request := httptest.NewRequest(http.MethodGet, "/v2/discovery/feed/item", nil)
	route := chi.NewRouteContext()
	route.URLParams.Add("id", strings.Repeat("x", 161))
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, route))
	response = httptest.NewRecorder()
	d.feedDetail(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("oversized source id: %d %s", response.Code, response.Body.String())
	}
}
