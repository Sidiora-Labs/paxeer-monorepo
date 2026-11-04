package store_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
)

func insertPoolStatsFull(t *testing.T, ctx context.Context, pool *pgxpool.Pool, addr, vol24, vol1h, mcap string, buy24, sell24 int, createdAt int64) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO stats.pool_stats
			(pool_address, token_address, volume_24h, volume_1h, market_cap, buy_count_24h, sell_count_24h, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$8)`,
		addr, addr+"-tok", vol24, vol1h, mcap, buy24, sell24, createdAt); err != nil {
		t.Fatalf("seed pool_stats: %v", err)
	}
}

func insertTxFull(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id, addr string, isBuy bool, amountIn, amountOut, fee string, ts int64) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO stats.pool_transactions
			(id, pool_address, sender, is_buy, amount_in, amount_out, price, fee, block_timestamp, tx_hash)
		VALUES ($1::text,$2,'0xs',$3,$4,$5,'1000',$6,$7,$1::varchar(66))`,
		id, addr, isBuy, amountIn, amountOut, fee, ts); err != nil {
		t.Fatalf("seed tx: %v", err)
	}
}

func TestPlatformMetrics(t *testing.T) {
	repairM1041FeeContract(t)
}

func TestPruneBucketTraders(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	st := store.New(pool)

	insertTrader(t, ctx, pool, "0xp", 100, "0xold1")
	insertTrader(t, ctx, pool, "0xp", 200, "0xold2")
	insertTrader(t, ctx, pool, "0xp", 1000, "0xnew1")
	insertTrader(t, ctx, pool, "0xp", 2000, "0xnew2")

	removed, err := st.PruneBucketTraders(ctx, 500)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	var remaining int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM stats.bucket_traders`).Scan(&remaining)
	if remaining != 2 {
		t.Fatalf("remaining = %d, want 2", remaining)
	}
}

func TestPressureStats(t *testing.T) {
	repairM1041PressureContract(t)
}

func TestSearch(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	internaltest.EnsureMetadataSchema(t, pool)
	st := store.New(pool)

	seedToken := func(token, p, name, symbol, vol string) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO metadata.token_metadata (token_address, pool_address, name, symbol, description, created_by, created_at)
			VALUES ($1,$2,$3,$4,'desc','0xc',100)`, token, p, name, symbol); err != nil {
			t.Fatalf("seed token: %v", err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO stats.pool_stats (pool_address, token_address, volume_24h, created_at, updated_at)
			VALUES ($1,$2,$3,100,100)`, p, token, vol); err != nil {
			t.Fatalf("seed pool_stats: %v", err)
		}
	}
	seedToken("0xtoken1", "0xpool1", "DogeCoin", "DOGE", "500")
	seedToken("0xtoken2", "0xpool2", "CatCoin", "CAT", "900")

	t.Run("by name", func(t *testing.T) {
		res, err := st.Search(ctx, false, "%doge%", 20)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(res) != 1 || res[0].TokenAddress != "0xtoken1" {
			t.Fatalf("by name = %+v", res)
		}
		if res[0].Name == nil || *res[0].Name != "DogeCoin" || res[0].Volume24h != "500" {
			t.Fatalf("fields = %+v", res[0])
		}
	})

	t.Run("ordered by volume_24h desc", func(t *testing.T) {
		res, err := st.Search(ctx, false, "%coin%", 20)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(res) != 2 || res[0].TokenAddress != "0xtoken2" || res[1].TokenAddress != "0xtoken1" {
			t.Fatalf("order = %+v (want CatCoin/900 first)", res)
		}
	})

	t.Run("by address", func(t *testing.T) {
		res, err := st.Search(ctx, true, "%0xtoken1%", 20)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(res) != 1 || res[0].TokenAddress != "0xtoken1" {
			t.Fatalf("by address = %+v", res)
		}
	})

	t.Run("limit respected", func(t *testing.T) {
		res, err := st.Search(ctx, false, "%coin%", 1)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("limit = %d, want 1", len(res))
		}
	})

	t.Run("no match -> empty non-nil", func(t *testing.T) {
		res, err := st.Search(ctx, false, "%zzz%", 20)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if res == nil || len(res) != 0 {
			t.Fatalf("want empty non-nil, got %+v", res)
		}
	})
}
