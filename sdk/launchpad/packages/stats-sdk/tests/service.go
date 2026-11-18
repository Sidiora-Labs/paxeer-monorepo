package httpapi_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
)

func TestStatsSDKService(t *testing.T) {
	ctx := context.Background()
	st := store.New(internaltest.NewPostgres(t))
	rdb := internaltest.NewRedis(t)
	pool := "0x" + strings.Repeat("a", 40)
	otherPool := "0x" + strings.Repeat("b", 40)
	missingPool := "0x" + strings.Repeat("c", 40)
	creator := "0x" + strings.Repeat("Ab", 20)
	addresses := []string{pool, otherPool, missingPool}
	clearCache := func() {
		for _, address := range addresses {
			if err := rdb.Del(ctx, "stats:"+address).Err(); err != nil {
				t.Fatal(err)
			}
		}
	}
	clearCache()
	t.Cleanup(clearCache)
	rows := make(map[string]*store.PoolStatsRow)
	for i, address := range addresses[:2] {
		var owner *string
		if i == 1 {
			owner = &creator
		}
		if _, err := st.InsertInitialPoolStats(ctx, store.InitialPoolStats{
			PoolAddress: address, TokenAddress: creator, CreatorAddress: owner,
			Price: "9007199254740993123456789", MarketCap: "123456789012345678901234567890",
			High24h: "9007199254740993999999999", Low24h: "0",
			CreatedAt: 1700000000, UpdatedAt: 1700000123,
		}); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if _, err := st.Pool().Exec(ctx, `UPDATE stats.pool_stats SET risk_factors=NULL WHERE pool_address=$1`, address); err != nil {
				t.Fatal(err)
			}
		} else {
			if _, err := st.Pool().Exec(ctx, `UPDATE stats.pool_stats SET
				price_change_1m='-1', price_change_5m='2', price_change_15m='-3',
				price_change_1h='4', price_change_24h='-5', price_change_dollar_1m='-6',
				price_change_dollar_5m='7', price_change_dollar_15m='-8',
				price_change_dollar_1h='9', price_change_dollar_24h='-10',
				volume_24h='9007199254740993999999999', volume_1h='12', volume_5m='13',
				buy_count_24h=14, sell_count_24h=15, unique_traders_24h=16, holder_count=17,
				top10_concentration='1800', creator_holdings_pct='1900', risk_rating=20,
				risk_factors='["concentration"]' WHERE pool_address=$1`, address); err != nil {
				t.Fatal(err)
			}
		}
		row, err := st.GetPoolStats(ctx, address)
		if err != nil || row == nil {
			t.Fatalf("read pool stats: %v", err)
		}
		rows[address] = row
	}
	router := chi.NewRouter()
	httpapi.RegisterPoolStats(router, st, rdb)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.ContentLength != 0 || r.Header.Get("Accept") != "application/json" {
			t.Error("stats requests use GET with JSON accept and no body")
		}
		router.ServeHTTP(w, r)
	}))
	defer server.Close()
	ready, err := json.Marshal(map[string]any{
		"baseUrl": server.URL, "pool": pool, "otherPool": otherPool,
		"missingPool": missingPool, "rows": rows,
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("stats-service " + string(ready))
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		switch scanner.Text() {
		case "clear-cache":
			clearCache()
		case "database-offline":
			clearCache()
			st.Pool().Close()
		case "stop":
			return
		default:
			t.Fatal("unknown service command")
		}
		fmt.Println("stats-service " + scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}
