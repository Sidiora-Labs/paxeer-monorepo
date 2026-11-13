package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/search"
	"github.com/go-chi/chi/v5"
)

func TestExpandedSearchServesCanonicalGeneration(t *testing.T) {
	path := newRepair69Path(t)
	pool := path.create(55, repair69Address(56))
	path.change(search.Change{Version: 2, Kind: "visibility", Revision: 1, PoolAddress: pool, Public: true})
	service, err := search.NewService(search.Store{Pool: path.pool}, nil, 16)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	httpapi.RegisterIntegratedSearch(router, service, nil)
	query := "/v2/search/expanded?q=canonical&type=pool&limit=5"
	rec := serve(t, router, http.MethodGet, query, nil, nil)
	var page struct {
		Data struct {
			Items      []search.IntegratedHit `json:"items"`
			Generation string                 `json:"generation"`
		} `json:"data"`
		Revision string `json:"revision"`
	}
	decode(t, rec, &page)
	if rec.Code != 200 || len(page.Data.Items) != 1 || page.Data.Generation == "" || page.Revision == "" || rec.Header().Get("ETag") == "" {
		t.Fatalf("expanded search: %d %s", rec.Code, rec.Body.String())
	}
	for _, tc := range []struct {
		name, route, authorization, code string
		status                           int
	}{
		{"unsupported kind", "/v2/search/expanded?q=canonical&type=unknown", "", "QUERY_INVALID", 400},
		{"invalid query", "/v2/search/expanded?q=a&type=pool", "", "QUERY_INVALID", 400},
		{"identity dependency", query, "Bearer value", "DEPENDENCY_UNAVAILABLE", 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			headers := map[string]string{}
			if tc.authorization != "" {
				headers["Authorization"] = tc.authorization
			}
			result := serve(t, router, http.MethodGet, tc.route, nil, headers)
			var body struct {
				Code string `json:"code"`
			}
			decode(t, result, &body)
			if result.Code != tc.status || body.Code != tc.code {
				t.Fatalf("%d %s", result.Code, result.Body.String())
			}
		})
	}
	nilRouter := chi.NewRouter()
	httpapi.RegisterIntegratedSearch(nilRouter, nil, nil)
	rec = serve(t, nilRouter, http.MethodGet, query, nil, nil)
	if rec.Code != 503 || !strings.Contains(rec.Body.String(), "DEPENDENCY_UNAVAILABLE") {
		t.Fatalf("nil service: %d %s", rec.Code, rec.Body.String())
	}
	invalid := url.Values{"q": {"canonical"}, "type": {"pool"}, "cursor": {"bad"}}
	rec = serve(t, router, http.MethodGet, "/v2/search/expanded?"+invalid.Encode(), nil, nil)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "QUERY_INVALID") {
		t.Fatalf("invalid cursor: %d %s", rec.Code, rec.Body.String())
	}
}

func TestExpandedSearchValidatesPeerCursorAndAdmission(t *testing.T) {
	path := newRepair69Path(t)
	for _, n := range []int{61, 63} {
		pool := path.create(n, repair69Address(n+1))
		path.change(search.Change{Version: 2, Kind: "visibility", Revision: 1, PoolAddress: pool, Public: true})
	}
	service, err := search.NewService(search.Store{Pool: path.pool}, nil, 16)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	httpapi.RegisterIntegratedSearch(router, service, nil)
	route := "/v2/search/expanded?q=canonical&type=pool&limit=1"
	for _, tc := range []struct{ name, remote, query string }{
		{"peer host", "invalid-peer", "q=canonical&type=pool"},
		{"peer address", "hostname:1234", "q=canonical&type=pool"},
		{"query encoding", "127.0.0.3:1234", "q=%"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, route, nil)
			request.RemoteAddr, request.URL.RawQuery = tc.remote, tc.query
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != 400 || !strings.Contains(response.Body.String(), "QUERY_INVALID") {
				t.Fatalf("request admission: %d %s", response.Code, response.Body.String())
			}
		})
	}
	response := serve(t, router, http.MethodGet, route, nil, nil)
	var page struct {
		Data search.IntegratedPage `json:"data"`
	}
	decode(t, response, &page)
	if response.Code != 200 || page.Data.Next == nil || len(page.Data.Items) != 1 {
		t.Fatalf("expanded first page: %d %s", response.Code, response.Body.String())
	}
	page.Data.Next.Generation = "different-generation"
	raw, err := json.Marshal(page.Data.Next)
	if err != nil {
		t.Fatal(err)
	}
	response = serve(t, router, http.MethodGet, route+"&cursor="+url.QueryEscape(string(raw)), nil, nil)
	if response.Code != 400 || !strings.Contains(response.Body.String(), "CURSOR_INVALID") {
		t.Fatalf("stale expanded cursor: %d %s", response.Code, response.Body.String())
	}
	for i := 0; i < 120; i++ {
		if err := service.Admit("127.0.0.4"); err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(http.MethodGet, route, nil)
	request.RemoteAddr = "127.0.0.4:1234"
	limited := httptest.NewRecorder()
	router.ServeHTTP(limited, request)
	if limited.Code != 429 || limited.Header().Get("Retry-After") != "60" || !strings.Contains(limited.Body.String(), "RATE_LIMITED") {
		t.Fatalf("expanded rate limit: %d %s", limited.Code, limited.Body.String())
	}
	path.pool.Close()
	response = serve(t, router, http.MethodGet, route, nil, nil)
	if response.Code != 503 || !strings.Contains(response.Body.String(), "DEPENDENCY_UNAVAILABLE") {
		t.Fatalf("expanded dependency: %d %s", response.Code, response.Body.String())
	}
}
