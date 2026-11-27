package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/holdings"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	"github.com/go-chi/chi/v5"
)

func TestNumericOrderHolderPages(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	const schema = "numeric_holder_pages"
	ddl, err := holdings.SchemaSQL(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repository, err := holdings.NewRepository(db, schema)
	if err != nil {
		t.Fatal(err)
	}
	address := func(n int) string { return fmt.Sprintf("0x%040x", n) }
	pool, token := address(1), address(2)
	stats := store.New(db)
	if _, err = stats.InsertInitialPoolStats(ctx, store.InitialPoolStats{PoolAddress: pool, TokenAddress: token, Price: "1", MarketCap: "2299", High24h: "1", Low24h: "1", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err = repository.Discover(ctx, token, 6, "10", true); err != nil {
		t.Fatal(err)
	}
	balances := []string{"1000", "999", "100", "100", "90", "9", "1"}
	for i, balance := range balances {
		result, applyErr := repository.Apply(ctx, holdings.Transfer{ChainID: "125", Token: token, From: holdings.Zero, To: address(i + 10), Amount: balance, Height: "10", TransactionIndex: "0", LogIndex: fmt.Sprint(i), TransactionHash: fmt.Sprintf("0x%064x", 100), BlockHash: fmt.Sprintf("0x%064x", 200)})
		if applyErr != nil || result.Duplicate || result.Incident {
			t.Fatalf("holder mint %d: %+v %v", i, result, applyErr)
		}
	}
	router := chi.NewRouter()
	httpapi.RegisterPoolHolders(router, stats, schema)
	type holder struct {
		Address string `json:"address"`
		Balance string `json:"balance"`
		Rank    int    `json:"rank"`
	}
	type response struct {
		Wallets []holder `json:"walletMap"`
		Next    *string  `json:"nextCursor"`
	}
	request := func(query string) response {
		t.Helper()
		rec := serve(t, router, http.MethodGet, "/stats/"+pool+"/holders/distribution?"+query, nil, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("holder route: %d %s", rec.Code, rec.Body.String())
		}
		var body response
		decode(t, rec, &body)
		return body
	}
	all := request("limit=20")
	if len(all.Wallets) != len(balances) || all.Next != nil {
		t.Fatalf("all holders: %+v", all)
	}
	for i, item := range all.Wallets {
		if item.Address != address(i+10) || item.Balance != balances[i] || item.Rank != i+1 {
			t.Fatalf("holder %d: %+v", i, item)
		}
	}
	seen := map[string]bool{}
	cursor := ""
	count := 0
	for page := 0; page < len(balances); page++ {
		body := request("limit=2" + cursor)
		if len(body.Wallets) == 0 || len(body.Wallets) > 2 {
			t.Fatalf("page size: %+v", body)
		}
		for _, item := range body.Wallets {
			if count >= len(all.Wallets) || seen[item.Address] || item != all.Wallets[count] {
				t.Fatalf("holder page at %d: %+v", count, body)
			}
			seen[item.Address] = true
			count++
		}
		if body.Next == nil {
			break
		}
		cursor = "&cursor=" + *body.Next
	}
	if count != len(balances) || len(seen) != len(balances) {
		t.Fatalf("paged holders: %d, distinct %d, want %d", count, len(seen), len(balances))
	}
}
