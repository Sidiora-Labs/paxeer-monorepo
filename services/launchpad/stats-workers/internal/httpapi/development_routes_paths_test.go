package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	"github.com/go-chi/chi/v5"
)

func TestDevelopmentReadRoutesUseMigratedStore(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	internaltest.EnsureMetadataSchema(t, pool)
	redis := internaltest.NewRedis(t)
	st := store.New(pool)
	router := chi.NewRouter()
	httpapi.RegisterPoolStats(router, st, redis)
	httpapi.RegisterPoolTransactions(router, st)
	httpapi.RegisterCrossTokenSwaps(router, st)
	httpapi.RegisterSearch(router, st, redis)
	_, randomAddress := newWallet(t)
	address := strings.ToLower(randomAddress)
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/stats/batch?pools=" + address, 200},
		{"/stats/" + address, 404},
		{"/stats/" + address + "/transactions", 200},
		{"/stats/cross-token-swaps/" + address, 200},
		{"/stats/cross-token-swaps/token/" + address, 200},
		{"/search?q=unknown", 200},
	} {
		response := serve(t, router, http.MethodGet, tc.path, nil, nil)
		if response.Code != tc.status || !json.Valid(response.Body.Bytes()) {
			t.Fatalf("%s: %d %s", tc.path, response.Code, response.Body.String())
		}
	}
	if _, err := st.InsertInitialPoolStats(t.Context(), store.InitialPoolStats{PoolAddress: address, TokenAddress: "0x" + strings.Repeat("2", 40), Price: "1000000", MarketCap: "2000000", High24h: "1000000", Low24h: "1000000", CreatedAt: 100, UpdatedAt: 100}); err != nil {
		t.Fatal(err)
	}
	response := serve(t, router, http.MethodGet, "/stats/"+address, nil, nil)
	var row store.PoolStatsRow
	decode(t, response, &row)
	if response.Code != 200 || row.PoolAddress != address {
		t.Fatalf("stored pool: %d %+v", response.Code, row)
	}
	cached := serve(t, router, http.MethodGet, "/stats/"+address, nil, nil)
	if cached.Code != 200 || cached.Body.String() != response.Body.String() {
		t.Fatalf("cached pool: %d %s", cached.Code, cached.Body.String())
	}
}
