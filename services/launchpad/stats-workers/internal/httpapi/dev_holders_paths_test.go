package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	"github.com/go-chi/chi/v5"
)

func TestDevelopmentHolderPagesAndValidation(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	st := store.New(db)
	pool, token := fmt.Sprintf("0x%040x", 31), fmt.Sprintf("0x%040x", 32)
	if _, err := st.InsertInitialPoolStats(ctx, store.InitialPoolStats{PoolAddress: pool, TokenAddress: token, Price: "1", MarketCap: "1", High24h: "1", Low24h: "1", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		if _, err := db.Exec(ctx, `INSERT INTO stats.pool_holders(pool_address,holder_address,balance,pct_of_supply,last_updated) VALUES($1,$2,$3,'0',1)`, pool, fmt.Sprintf("0x%040x", i), fmt.Sprint(10*i)); err != nil {
			t.Fatal(err)
		}
	}
	router := chi.NewRouter()
	httpapi.RegisterDevPoolHolders(router, st)
	path := "/v1/pools/" + pool + "/holders"
	var first struct {
		Data struct {
			Items []struct {
				Wallet  string `json:"wallet"`
				Balance string `json:"balanceRaw"`
			} `json:"items"`
			Coverage string `json:"coverage"`
		} `json:"data"`
		Quality string  `json:"quality"`
		Next    *string `json:"nextCursor"`
	}
	rec := serve(t, router, http.MethodGet, path+"?limit=2", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("first page: %d %s", rec.Code, rec.Body.String())
	}
	decode(t, rec, &first)
	if len(first.Data.Items) != 2 || first.Data.Items[0].Balance != "30" || first.Data.Items[1].Balance != "20" || first.Data.Coverage != "partial" || first.Quality != "partial" || first.Next == nil {
		t.Fatalf("first page: %+v", first)
	}
	var second struct {
		Data struct {
			Items []struct {
				Balance string `json:"balanceRaw"`
			} `json:"items"`
		} `json:"data"`
		Next *string `json:"nextCursor"`
	}
	rec = serve(t, router, http.MethodGet, path+"?limit=2&cursor="+url.QueryEscape(*first.Next), nil, nil)
	if rec.Code != 200 {
		t.Fatalf("second page: %d %s", rec.Code, rec.Body.String())
	}
	decode(t, rec, &second)
	if len(second.Data.Items) != 1 || second.Data.Items[0].Balance != "10" || second.Next != nil {
		t.Fatalf("second page: %+v", second)
	}
	for _, tc := range []struct {
		path, code string
		status     int
	}{
		{path + "?limit=0", "FIELD_INVALID", 422},
		{path + "?cursor=bad", "CURSOR_INVALID", 400},
		{path + "?unknown=1", "FIELD_UNKNOWN", 422},
		{"/v1/pools/0x12/holders", "FIELD_INVALID", 422},
		{"/v1/pools/" + fmt.Sprintf("0x%040x", 99) + "/holders", "NOT_FOUND", 404},
	} {
		rec = serve(t, router, http.MethodGet, tc.path, nil, nil)
		var body struct {
			Code string `json:"code"`
		}
		decode(t, rec, &body)
		if rec.Code != tc.status || body.Code != tc.code {
			t.Fatalf("%s: %d %s", tc.path, rec.Code, rec.Body.String())
		}
	}
	nilRouter := chi.NewRouter()
	httpapi.RegisterDevPoolHolders(nilRouter, nil)
	rec = serve(t, nilRouter, http.MethodGet, path, nil, nil)
	var unavailable struct {
		Code string `json:"code"`
	}
	decode(t, rec, &unavailable)
	if rec.Code != 503 || unavailable.Code != "DEPENDENCY_UNAVAILABLE" {
		t.Fatalf("dependency: %d %s", rec.Code, rec.Body.String())
	}
}
