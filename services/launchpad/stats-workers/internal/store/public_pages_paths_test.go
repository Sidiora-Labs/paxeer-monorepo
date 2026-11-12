package store_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
)

func TestCrossTokenSwapPageFiltersAndPaginates(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	st := store.New(pool)
	for i, row := range []struct{ id, sender, in, out string }{
		{"first", "wallet-a", "token-a", "token-b"},
		{"second", "wallet-a", "token-b", "token-c"},
		{"third", "wallet-b", "token-a", "token-c"},
	} {
		_, err := pool.Exec(ctx, `INSERT INTO stats.cross_token_swaps(id,sender,token_in,token_out,pool_in,pool_out,amount_in,intermediate_usdl,amount_out,fee_in,fee_out,block_timestamp,tx_hash) VALUES($1,$2,$3,$4,'pool-a','pool-b','10','9','8','1','1',$5,$6)`, row.id, row.sender, row.in, row.out, 100+i, fmt.Sprintf("hash-%d", i))
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		subject string
		token   bool
		after   *store.HistoryPosition
		want    string
	}{
		{"wallet-a", false, nil, "second"},
		{"token-a", true, nil, "third"},
		{"token-a", true, &store.HistoryPosition{Timestamp: 102, ID: "third"}, "first"},
	} {
		rows, err := st.CrossTokenSwapPage(ctx, tc.subject, tc.token, 1, 0, tc.after)
		if err != nil || len(rows) != 1 || rows[0].ID != tc.want {
			t.Fatalf("swap page %+v: %+v %v", tc, rows, err)
		}
	}
	for _, invalid := range []struct{ limit, offset int }{{0, 0}, {502, 0}, {1, -1}, {1, 10001}} {
		if _, err := st.CrossTokenSwapPage(ctx, "wallet-a", false, invalid.limit, invalid.offset, nil); err == nil {
			t.Fatalf("invalid page accepted: %+v", invalid)
		}
	}
}

func TestHolderBalancePageAggregatesWallets(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	st := store.New(pool)
	const market = "pool-a"
	first := "0x00000000000000000000000000000000000000aa"
	second := "0x00000000000000000000000000000000000000bb"
	for _, row := range []struct{ wallet, balance string }{{first, "10"}, {"0x00000000000000000000000000000000000000AA", "5"}, {second, "7"}, {"bad-wallet", "100"}, {"0x00000000000000000000000000000000000000cc", "0"}} {
		insertHolder(t, ctx, pool, market, row.wallet, row.balance, "0")
	}
	page, err := st.ListHolderBalances(ctx, market, 1, 0)
	if err != nil || len(page) != 1 || page[0].Wallet != first || page[0].BalanceRaw != "15" {
		t.Fatalf("first holder page: %+v %v", page, err)
	}
	page, err = st.ListHolderBalances(ctx, market, 1, 1)
	if err != nil || len(page) != 1 || page[0].Wallet != second || page[0].BalanceRaw != "7" {
		t.Fatalf("second holder page: %+v %v", page, err)
	}
}
