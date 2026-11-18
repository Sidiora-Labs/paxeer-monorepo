package rest

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestDevelopmentPoolHomeChainDocument(t *testing.T) {
	for _, home := range []string{"0", "7", "4294967295"} {
		t.Run(home, func(t *testing.T) {
			row := marketDocumentPool()
			row.HomeChain = home
			pool, err := developmentPoolDocument(row, marketDocumentPosition())
			if err != nil {
				t.Fatal(err)
			}
			wm := marketDocumentWatermark(t)
			legacy, err := json.Marshal(store.PoolStatsRow{PoolAddress: row.Pool, Volume24h: "0", UpdatedAt: row.CreatedAt})
			if err != nil {
				t.Fatal(err)
			}
			stats, err := developmentPoolStats(row, legacy, marketDocumentPosition(), wm)
			if err != nil {
				t.Fatal(err)
			}
			body := emitMarketDocument(t, "PoolDetailResponse", map[string]any{
				"pool": pool, "token": tokenDocument(store.PoolRegistryRow{Token: row.Token}), "stats": stats,
			}, &wm, nil)
			var response struct {
				Data struct {
					Pool struct {
						HomeChain string `json:"homeChain"`
					} `json:"pool"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &response); err != nil {
				t.Fatal(err)
			}
			if response.Data.Pool.HomeChain != home {
				t.Fatalf("pool home chain = %q, want %q", response.Data.Pool.HomeChain, home)
			}
		})
	}
}

func TestDevelopmentPoolHomeChainDocumentRejectsUnchecked(t *testing.T) {
	for _, home := range []string{"", "-1", "+1", "01", " 1", "1 ", "1.0", "invalid", "4294967296"} {
		t.Run(home, func(t *testing.T) {
			row := marketDocumentPool()
			row.HomeChain = home
			if document, err := developmentPoolDocument(row, marketDocumentPosition()); !errors.Is(err, store.ErrDataIntegrity) || document != nil {
				t.Fatalf("unchecked home chain %q: document=%v error=%v", home, document, err)
			}
		})
	}
}

func TestDevelopmentPoolHomeChainRead(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	internaltest.ApplyIndexerTables(t, db, "cursors", "swaps")
	row := marketDocumentPool()
	if _, err := db.Exec(ctx, `INSERT INTO indexer.pools(pool_address,token_address,creator,optical,pool_id,created_at,created_block,tx_hash)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, row.Pool, row.Token, row.Creator, row.Optical, row.PoolID, row.CreatedAt, row.CreatedHeight, row.CreationTransaction); err != nil {
		t.Fatal(err)
	}
	st := store.New(db, nil)
	for _, value := range []int64{0, 7, 4294967295} {
		home := strconv.FormatInt(value, 10)
		t.Run(home, func(t *testing.T) {
			if tag, err := db.Exec(ctx, `UPDATE indexer.pools SET home_chain=$2 WHERE pool_address=$1`, row.Pool, value); err != nil || tag.RowsAffected() != 1 {
				t.Fatalf("set home chain: %v, %v", tag, err)
			}
			pools, err := st.DevelopmentPools(ctx, []string{row.Pool})
			if err != nil || len(pools) != 1 || pools[row.Pool].HomeChain != home {
				t.Fatalf("indexed home chain: pools=%+v error=%v, want %q", pools, err, home)
			}
		})
	}
	internaltest.ApplyReadMigrations(t, db, "60000", "60003")
	pools, err := st.DevelopmentPools(ctx, []string{row.Pool})
	if err != nil || len(pools) != 1 || pools[row.Pool].HomeChain != "4294967295" {
		t.Fatalf("home chain after migration: pools=%+v error=%v", pools, err)
	}
	for _, value := range []int64{-1, 4294967296} {
		home := strconv.FormatInt(value, 10)
		t.Run(home, func(t *testing.T) {
			if tag, err := db.Exec(ctx, `UPDATE indexer.pools SET home_chain=$2 WHERE pool_address=$1`, row.Pool, value); err != nil || tag.RowsAffected() != 1 {
				t.Fatalf("set home chain: %v, %v", tag, err)
			}
			if pools, err := st.DevelopmentPools(ctx, []string{row.Pool}); !errors.Is(err, store.ErrDataIntegrity) || pools != nil {
				t.Fatalf("unchecked indexed home chain %q: pools=%+v error=%v", home, pools, err)
			}
		})
	}
}
