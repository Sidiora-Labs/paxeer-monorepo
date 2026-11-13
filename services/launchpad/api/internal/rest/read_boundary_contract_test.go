package rest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/go-chi/chi/v5"
)

func TestChainHandlersValidateRequestsAndMissingDependencies(t *testing.T) {
	for _, st := range []*store.Store{nil, store.New(nil, nil)} {
		router := chi.NewRouter()
		router.Get("/v2/chains", chainsList(st))
		router.Get("/v2/chains/{chain}", chainsDetail(st))
		router.Get("/v2/chains/{chain}/revenue", chainsRevenue(st))
		for _, tc := range []struct {
			path   string
			status int
			code   string
		}{
			{"/v2/chains?holder=invalid", 422, "FIELD_UNKNOWN"},
			{"/v2/chains?limit=1&limit=2", 422, "FIELD_UNKNOWN"},
			{"/v2/chains?limit=101", 422, "FIELD_INVALID"},
			{"/v2/chains", 503, "DEPENDENCY_UNAVAILABLE"},
			{"/v2/chains/7?unknown=1", 400, "QUERY_INVALID"},
			{"/v2/chains/7", 503, "DEPENDENCY_UNAVAILABLE"},
			{"/v2/chains/0/revenue", 422, "FIELD_INVALID"},
			{"/v2/chains/7/revenue?unknown=1", 422, "FIELD_UNKNOWN"},
			{"/v2/chains/7/revenue?holder=invalid", 422, "FIELD_INVALID"},
			{"/v2/chains/7/revenue", 503, "DEPENDENCY_UNAVAILABLE"},
		} {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
			var body APIError
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != tc.status || body.Code != tc.code {
				t.Fatalf("%s: status=%d body=%s err=%v", tc.path, response.Code, response.Body.String(), err)
			}
		}
	}
}

func TestReadEnvelopesRejectUnencodableAndInvalidDocuments(t *testing.T) {
	for _, data := range []any{make(chan int), strings.Repeat("x", store.PublicJSONMaxBytes)} {
		request := httptest.NewRequest(http.MethodGet, "/v2/chains", nil)
		for _, write := range []func(http.ResponseWriter){
			func(w http.ResponseWriter) { writeChainEnvelope(w, request, data, nil) },
			func(w http.ResponseWriter) {
				writeComputedEnvelope(w, request, "LeaderboardsResponse", data, store.ChainWatermark{}, "1", "complete", nil)
			},
			func(w http.ResponseWriter) { writeHSDEnvelope(w, request, "TradesResponse", data, nil) },
			func(w http.ResponseWriter) {
				writePagedProjectionEnvelope(w, request, "StatsResponse", data, &store.PublicProjection{}, nil)
			},
		} {
			response := httptest.NewRecorder()
			write(response)
			if response.Code != 503 || !strings.Contains(response.Body.String(), `"code":"INTEGRITY_QUARANTINED"`) {
				t.Fatalf("invalid document: status=%d body=%s", response.Code, response.Body.String())
			}
		}
	}
	for _, payload := range []json.RawMessage{json.RawMessage(`{`), json.RawMessage(`{"items":[{}]}`)} {
		response := httptest.NewRecorder()
		writeRankingsProjection(response, httptest.NewRequest(http.MethodGet, "/v2/rankings/volume", nil), store.PublicProjection{Payload: payload}, 1)
		if response.Code != 503 || !strings.Contains(response.Body.String(), "INTEGRITY_QUARANTINED") {
			t.Fatalf("invalid ranking: status=%d body=%s", response.Code, response.Body.String())
		}
	}
}

func TestReadPaginationDecodersRejectInvalidClaims(t *testing.T) {
	encode := func(raw string) string { return base64.RawURLEncoding.EncodeToString([]byte(raw)) }
	for _, token := range []string{"", strings.Repeat("a", 257), "!", encode(`{`), encode(`{"snapshotId":"s","offset":-1,"windowEnd":1}`), encode(`{"snapshotId":"s","offset":0,"windowEnd":0}`), encode(`{"snapshotId":"s","offset":0,"windowEnd":1,"unknown":true}`)} {
		if _, err := decodeLeaderboardCursor(token); err == nil {
			t.Fatalf("leaderboard cursor accepted: %q", token)
		}
	}
	pool := "0x" + strings.Repeat("1", 40)
	for _, token := range []string{"", "!", strings.Repeat("a", 769), encode(`{`), encode(`{"pool":"` + pool + `","balance":"01","wallet":"` + pool + `"}`), encode(`{"pool":"` + pool + `","balance":"1","wallet":"invalid"}`)} {
		if _, err := decodeHoldersCursor(token, pool); err == nil {
			t.Fatalf("holder cursor accepted: %q", token)
		}
	}
	if _, err := decodePoolListCursor(encode(`{"snapshotId":"s","offset":-1}`)); err == nil {
		t.Fatal("negative pool offset accepted")
	}
}

func TestPublicQueryAndErrorEnvelopeBounds(t *testing.T) {
	q := url.Values{}
	for i := 0; i < 33; i++ {
		q.Set(strconv.Itoa(i), "1")
	}
	if err := QueryParameters(q); err == nil {
		t.Fatal("unbounded query accepted")
	}
	addresses := make([]string, 101)
	for i := range addresses {
		addresses[i] = "0x" + strings.Repeat("1", 36) + strconv.FormatInt(int64(4096+i), 16)
	}
	if _, err := PoolAddresses(strings.Join(addresses, ",")); err == nil {
		t.Fatal("unbounded distinct pool selection accepted")
	}
	for _, tc := range []struct {
		path, code, expected string
		status               int
	}{
		{"/v1/pools", "unknown", "DATA_UNAVAILABLE", 503},
		{"/v2/pools", "CURSOR_EXPIRED", "CURSOR_INVALID", 400},
		{"/v2/pools", "CURSOR_SCOPE_MISMATCH", "CURSOR_INVALID", 400},
	} {
		request := httptest.NewRequest(http.MethodGet, tc.path, nil)
		request.Header.Set("Authorization", "Bearer session")
		response := httptest.NewRecorder()
		WriteAPIError(response, request, 400, tc.code)
		if response.Code != tc.status || !strings.Contains(response.Body.String(), `"code":"`+tc.expected+`"`) || response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("%s: status=%d body=%s headers=%v", tc.code, response.Code, response.Body.String(), response.Header())
		}
	}
	if err := RegisterProductionReads(chi.NewRouter(), ProductionReadOptions{}); err == nil {
		t.Fatal("unconfigured production reads admitted")
	}
	if err := RegisterV2SearchProxy(chi.NewRouter(), "https://user:pass@example.test"); err == nil {
		t.Fatal("credential-bearing origin admitted")
	}
	if _, err := newHoldersUpstream("https://example.test/path"); err == nil {
		t.Fatal("path-bearing origin admitted")
	}
	if upstream, err := newHoldersUpstream(""); err != nil || upstream.target != nil {
		t.Fatalf("unconfigured holders origin: %+v %v", upstream, err)
	}
	if _, err := newDevelopmentChain(context.Background(), ":invalid"); err == nil {
		t.Fatal("malformed chain origin admitted")
	}
}
