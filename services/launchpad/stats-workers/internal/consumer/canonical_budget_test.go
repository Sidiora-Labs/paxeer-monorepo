package consumer_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/consumer"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCanonicalLegacyBridgeSingleConnectionRecovery(t *testing.T) {
	f := newLegacyBridgeFixture(t, 100)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cfg := f.pg.Config()
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	redis := internaltest.NewRedis(t)
	newBridge := func() *consumer.CanonicalLegacyBridge {
		st := store.New(pool)
		if err := st.SetAccountingProjection(legacyVersion); err != nil {
			t.Fatal(err)
		}
		market, err := consumer.NewMarketConsumer(st, discardLogger())
		if err != nil {
			t.Fatal(err)
		}
		holder := consumer.NewHolderTracker(st, redis, discardLogger(), time.Hour)
		t.Cleanup(holder.Close)
		return &consumer.CanonicalLegacyBridge{Pool: pool, ProjectionSchema: "stats", SourceSchema: "indexer", SourceStream: legacyStream, ProjectionVersion: legacyVersion, Market: market, Swap: consumer.NewSwapConsumer(st, redis, discardLogger()), Holder: holder, Batch: 100}
	}
	if n, err := newBridge().RunOnce(ctx); err != nil || n != 3 {
		t.Fatalf("single connection replay: %d %v", n, err)
	}
	stats, err := f.st.GetPoolStats(ctx, f.real.Pool)
	if err != nil || stats == nil || stats.Price != f.sellPrice {
		t.Fatalf("projected price: %+v %v", stats, err)
	}
	var balance string
	if err := pool.QueryRow(ctx, "SELECT balance::text FROM stats.pool_holders WHERE pool_address=$1 AND holder_address=$2", f.real.Pool, f.deployer).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	// Both fills return the real receipt's remaining token balance.
	if balance != "4803406952560" {
		t.Fatalf("holder balance: %s", balance)
	}
	if n, err := newBridge().RunOnce(ctx); err != nil || n != 0 {
		t.Fatalf("restart replay: %d %v", n, err)
	}
	rows, err := f.st.ListTransactions(ctx, f.real.Pool, 10, 0, "")
	if err != nil || len(rows) != 2 {
		t.Fatalf("committed transactions: %+v %v", rows, err)
	}
	var after string
	if err := pool.QueryRow(ctx, "SELECT balance::text FROM stats.pool_holders WHERE pool_address=$1 AND holder_address=$2", f.real.Pool, f.deployer).Scan(&after); err != nil || after != balance {
		t.Fatalf("replayed balance: %s %v", after, err)
	}
}
