package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/holdings"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	"github.com/go-chi/chi/v5"
)

func TestCanonicalHolderRoutePagesCommittedTransferBalances(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	ddl, err := holdings.SchemaSQL("holder_route")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repository, err := holdings.NewRepository(db, "holder_route")
	if err != nil {
		t.Fatal(err)
	}
	address := func(n int) string { return fmt.Sprintf("0x%040x", n) }
	poolAddress, token, firstWallet, secondWallet := address(1), address(2), address(3), address(4)
	stats := store.New(db)
	if _, err := stats.InsertInitialPoolStats(ctx, store.InitialPoolStats{
		PoolAddress: poolAddress, TokenAddress: token, Price: "1", MarketCap: "100",
		High24h: "1", Low24h: "1", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Discover(ctx, token, 18, "9", true); err != nil {
		t.Fatal(err)
	}
	for i, transfer := range []struct{ from, to, amount string }{
		{holdings.Zero, firstWallet, "100"},
		{firstWallet, secondWallet, "40"},
	} {
		event := holdings.Transfer{
			ChainID: "125", Token: token, From: transfer.from, To: transfer.to,
			Amount: transfer.amount, Height: "10", TransactionIndex: "0", LogIndex: fmt.Sprint(i),
			TransactionHash: fmt.Sprintf("0x%064x", i+1), BlockHash: fmt.Sprintf("0x%064x", 100),
		}
		if _, err := repository.Apply(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	router := chi.NewRouter()
	httpapi.RegisterTransferHolders(router, stats, repository)
	type response struct {
		Data struct {
			Items []struct {
				Wallet     string `json:"wallet"`
				BalanceRaw string `json:"balanceRaw"`
			} `json:"items"`
			Coverage  string `json:"coverage"`
			SupplyRaw string `json:"supplyRaw"`
		} `json:"data"`
		Quality    string  `json:"quality"`
		Revision   *string `json:"revision"`
		NextCursor *string `json:"nextCursor"`
		Watermark  *struct {
			Height    string `json:"height"`
			Integrity string `json:"integrity"`
		} `json:"watermark"`
	}
	request := func(cursor string) response {
		t.Helper()
		path := "/v1/pools/" + poolAddress + "/holders?limit=1"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		rec := serve(t, router, http.MethodGet, path, nil, nil)
		if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Server-Timing"), "origin;dur=") {
			t.Fatalf("canonical holder response: status=%d timing=%q body=%s", rec.Code, rec.Header().Get("Server-Timing"), rec.Body.String())
		}
		var body response
		decode(t, rec, &body)
		if body.Data.Coverage != "complete" || body.Data.SupplyRaw != "100" || body.Quality != "complete" || body.Revision == nil || *body.Revision != "2" || body.Watermark == nil || body.Watermark.Height != "10" || body.Watermark.Integrity != "healthy" {
			t.Fatalf("canonical holder authority: %+v", body)
		}
		return body
	}
	first := request("")
	if len(first.Data.Items) != 1 || first.Data.Items[0].Wallet != firstWallet || first.Data.Items[0].BalanceRaw != "60" || first.NextCursor == nil {
		t.Fatalf("first holder page: %+v", first)
	}
	second := request(*first.NextCursor)
	if len(second.Data.Items) != 1 || second.Data.Items[0].Wallet != secondWallet || second.Data.Items[0].BalanceRaw != "40" || second.NextCursor != nil {
		t.Fatalf("second holder page: %+v", second)
	}
	legacy := chi.NewRouter()
	httpapi.RegisterPoolHolders(legacy, stats, "holder_route")
	httpapi.RegisterPoolAnalytics(legacy, stats, "holder_route")
	var distribution struct {
		Coverage string `json:"coverage"`
		Total    int    `json:"totalHolders"`
		Wallets  []struct {
			Address string `json:"address"`
			Rank    int    `json:"rank"`
		} `json:"walletMap"`
		Next *string `json:"nextCursor"`
	}
	rec := serve(t, legacy, http.MethodGet, "/stats/"+poolAddress+"/holders/distribution?limit=1", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("distribution: %d %s", rec.Code, rec.Body.String())
	}
	decode(t, rec, &distribution)
	if distribution.Coverage != "complete" || distribution.Total != 2 || len(distribution.Wallets) != 1 || distribution.Wallets[0].Address != firstWallet || distribution.Next == nil {
		t.Fatalf("distribution: %+v", distribution)
	}
	rec = serve(t, legacy, http.MethodGet, "/stats/"+poolAddress+"/holders/distribution?limit=1&cursor="+*distribution.Next, nil, nil)
	decode(t, rec, &distribution)
	if rec.Code != 200 || len(distribution.Wallets) != 1 || distribution.Wallets[0].Address != secondWallet || distribution.Wallets[0].Rank != 2 || distribution.Next != nil {
		t.Fatalf("distribution continuation: %d %+v", rec.Code, distribution)
	}
	var whales struct {
		Whales []struct {
			HolderAddress string `json:"holderAddress"`
			Pct           int64  `json:"pctOfSupply"`
		} `json:"whales"`
	}
	rec = serve(t, legacy, http.MethodGet, "/stats/"+poolAddress+"/whales", nil, nil)
	decode(t, rec, &whales)
	if rec.Code != 200 || len(whales.Whales) != 2 || whales.Whales[0].HolderAddress != firstWallet || whales.Whales[0].Pct != 6000 {
		t.Fatalf("canonical whales: %d %+v", rec.Code, whales)
	}
	for _, bad := range []string{"cursor=bad", "cursor=", "limit=1&offset=1&cursor=bad"} {
		rec = serve(t, legacy, http.MethodGet, "/stats/"+poolAddress+"/holders/distribution?"+bad, nil, nil)
		if rec.Code != 400 {
			t.Fatalf("%s: %d %s", bad, rec.Code, rec.Body.String())
		}
	}
	for _, tc := range []struct {
		path, code string
		status     int
	}{
		{"/v1/pools/0x12/holders", "FIELD_INVALID", 422},
		{"/v1/pools/" + poolAddress + "/holders?other=1", "FIELD_UNKNOWN", 422},
		{"/v1/pools/" + poolAddress + "/holders?limit=0", "FIELD_INVALID", 422},
		{"/v1/pools/" + poolAddress + "/holders?cursor=", "CURSOR_INVALID", 400},
		{"/v1/pools/" + poolAddress + "/holders?cursor=bad", "CURSOR_INVALID", 400},
		{"/v1/pools/" + address(999) + "/holders", "NOT_FOUND", 404},
	} {
		rec = serve(t, router, http.MethodGet, tc.path, nil, nil)
		var failure struct {
			Code string `json:"code"`
		}
		decode(t, rec, &failure)
		if rec.Code != tc.status || failure.Code != tc.code {
			t.Fatalf("%s: %d %s", tc.path, rec.Code, rec.Body.String())
		}
	}
	for _, tc := range []struct {
		name, setup, restore, cursor, quality, integrity string
		status                                           int
		code                                             string
	}{
		{name: "snapshot revision changed", setup: "UPDATE holder_route.tokens SET revision=revision+1", restore: "UPDATE holder_route.tokens SET revision=revision-1", cursor: *first.NextCursor, quality: "stale", integrity: "healthy", status: 200},
		{name: "quarantined holdings", setup: "UPDATE holder_route.tokens SET coverage='quarantined'", restore: "UPDATE holder_route.tokens SET coverage='complete'", quality: "quarantined", integrity: "quarantined", status: 200},
		{name: "incomplete holdings", setup: "UPDATE holder_route.tokens SET coverage='incomplete'", restore: "UPDATE holder_route.tokens SET coverage='complete'", quality: "partial", integrity: "healthy", status: 200},
		{name: "expired snapshot", setup: "UPDATE holder_route.snapshots SET expires_at=clock_timestamp()-interval '1 second'", restore: "UPDATE holder_route.snapshots SET expires_at=clock_timestamp()+interval '15 minutes'", cursor: *first.NextCursor, status: 409, code: "RESNAPSHOT_REQUIRED"},
		{name: "unknown holder token", setup: "UPDATE stats.pool_stats SET token_address='" + address(98) + "'", restore: "UPDATE stats.pool_stats SET token_address='" + token + "'", status: 503, code: "DEPENDENCY_UNAVAILABLE"},
		{name: "invalid holder token", setup: "UPDATE stats.pool_stats SET token_address='invalid'", restore: "UPDATE stats.pool_stats SET token_address='" + token + "'", status: 503, code: "DEPENDENCY_UNAVAILABLE"},
		{name: "materialization unavailable", setup: "ALTER TABLE holder_route.snapshots RENAME TO saved_snapshots", restore: "ALTER TABLE holder_route.saved_snapshots RENAME TO snapshots", status: 503, code: "DEPENDENCY_UNAVAILABLE"},
		{name: "snapshot storage unavailable", setup: "ALTER TABLE holder_route.snapshots RENAME TO saved_snapshots", restore: "ALTER TABLE holder_route.saved_snapshots RENAME TO snapshots", cursor: *first.NextCursor, status: 503, code: "DEPENDENCY_UNAVAILABLE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := db.Exec(ctx, tc.setup); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := db.Exec(ctx, tc.restore); err != nil {
					t.Error(err)
				}
			}()
			route := "/v1/pools/" + poolAddress + "/holders"
			if tc.cursor != "" {
				route += "?cursor=" + tc.cursor
			}
			result := serve(t, router, http.MethodGet, route, nil, nil)
			if result.Code != tc.status {
				t.Fatalf("holder availability: %d %s", result.Code, result.Body.String())
			}
			if tc.status != 200 {
				var failure struct {
					Code string `json:"code"`
				}
				decode(t, result, &failure)
				if failure.Code != tc.code {
					t.Fatalf("holder error: %s", result.Body.String())
				}
				return
			}
			var body response
			decode(t, result, &body)
			if body.Quality != tc.quality || body.Watermark == nil || body.Watermark.Integrity != tc.integrity || body.Data.SupplyRaw != "100" {
				t.Fatalf("holder coverage: %s", result.Body.String())
			}
			if tc.quality == "stale" && result.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("stale caching: %v", result.Header())
			}
		})
	}
	unavailable := chi.NewRouter()
	httpapi.RegisterTransferHolders(unavailable, nil, nil)
	rec = serve(t, unavailable, http.MethodGet, "/v1/pools/"+poolAddress+"/holders", nil, nil)
	if rec.Code != 503 || !strings.Contains(rec.Body.String(), "DEPENDENCY_UNAVAILABLE") {
		t.Fatalf("missing holder dependencies: %d %s", rec.Code, rec.Body.String())
	}
	db.Close()
	rec = serve(t, router, http.MethodGet, "/v1/pools/"+poolAddress+"/holders", nil, nil)
	if rec.Code != 503 || !strings.Contains(rec.Body.String(), "DEPENDENCY_UNAVAILABLE") {
		t.Fatalf("closed holder dependencies: %d %s", rec.Code, rec.Body.String())
	}

}
