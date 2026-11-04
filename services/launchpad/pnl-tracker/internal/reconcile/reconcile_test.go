package reconcile_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/pnlcache"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/constants"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/position"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/reconcile"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

const oneToken = "1000000000000000000"

// seed builds a store with the indexer schema + a pool, returning the store.
func seed(t *testing.T) *store.Store {
	t.Helper()
	st := store.New(internaltest.NewPostgres(t))
	internaltest.EnsureIndexerSchema(t, st.Pool())
	if _, err := st.Pool().Exec(context.Background(), `
		INSERT INTO indexer.pools (pool_address, token_address, creator, created_at)
		VALUES ('0xpool','0xtoken','0xc',1)`); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	return st
}

func seedSwap(t *testing.T, st *store.Store, id, sender string, isBuy bool, in, out string, block, logIndex int64) {
	t.Helper()
	if _, err := st.Pool().Exec(context.Background(), `
		INSERT INTO indexer.swaps (id, pool_id, pool_address, sender, is_buy, amount_in, amount_out, fee, price, block_number, block_timestamp, tx_hash, log_index)
		VALUES ($1::text,'pid','0xpool',$2,$3,$4,$5,'0','0',$6,$6,$1::varchar(66),$7)`,
		id, sender, isBuy, in, out, block, logIndex); err != nil {
		t.Fatalf("seed swap: %v", err)
	}
}

func TestReconcilerBackfillsAndAdvancesCursor(t *testing.T) {
	ctx := context.Background()
	st := seed(t)
	rdb := internaltest.NewRedis(t)
	r := reconcile.New(st, rdb, discardLogger(), 500)

	seedSwap(t, st, "0xa-0", "0xu", true, "1000000", oneToken, 100, 0)
	seedSwap(t, st, "0xb-0", "0xu", true, "3000000", oneToken, 101, 0)

	n, err := r.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if n != 2 {
		t.Fatalf("processed = %d, want 2", n)
	}

	pos, _ := st.GetPosition(ctx, "0xu", "0xpool")
	if pos == nil || pos.TotalUsdlSpent != "4000000" || pos.TokenAddress != "0xtoken" {
		t.Fatalf("folded position = %+v", pos)
	}

	cur, _ := st.GetCursor(ctx)
	if cur.LastBlock != 101 || cur.LastLogIndex != 0 {
		t.Fatalf("cursor = %+v, want (101,0)", cur)
	}

	// A second run with no new swaps is a no-op.
	n, err = r.RunOnce(ctx)
	if err != nil || n != 0 {
		t.Fatalf("idle run = %d err = %v", n, err)
	}
}

func TestReconcilerIdempotentWithConsumerOverlap(t *testing.T) {
	ctx := context.Background()
	st := seed(t)
	rdb := internaltest.NewRedis(t)
	r := reconcile.New(st, rdb, discardLogger(), 500)

	// The real realtime consumer folds the swap before backfill sees it.
	seedSwap(t, st, "0xdup-0", "0xu", true, "1000000", oneToken, 100, 0)
	if err := position.NewConsumer(st, rdb, discardLogger()).ProcessEvent(ctx, position.SwapEvent{
		Sender: "0xu", PoolAddress: "0xpool", IsBuy: true,
		AmountIn: "1000000", AmountOut: oneToken, Price: "0", Fee: "0",
		BlockNumber: 100, BlockTimestamp: 100, TxHash: "0xdup", LogIndex: 0,
	}); err != nil {
		t.Fatalf("pre-fold: %v", err)
	}

	if _, err := r.RunOnce(ctx); err != nil {
		t.Fatalf("run once: %v", err)
	}
	pos, _ := st.GetPosition(ctx, "0xu", "0xpool")
	if pos.TradeCount != 1 || pos.TotalUsdlSpent != "1000000" {
		t.Fatalf("reconciler double-counted an already-folded swap: %+v", pos)
	}
}

func TestReconcilerBatchKeysetDrainsLargeBlock(t *testing.T) {
	ctx := context.Background()
	st := seed(t)
	rdb := internaltest.NewRedis(t)
	// Batch of 1 forces multiple runs to drain a single 3-swap block.
	r := reconcile.New(st, rdb, discardLogger(), 1)

	seedSwap(t, st, "0xa-0", "0xu", true, "1000000", oneToken, 100, 0)
	seedSwap(t, st, "0xa-1", "0xu", true, "1000000", oneToken, 100, 1)
	seedSwap(t, st, "0xa-2", "0xu", true, "1000000", oneToken, 100, 2)

	total := 0
	for i := 0; i < 5; i++ {
		n, err := r.RunOnce(ctx)
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		total += n
		if n == 0 {
			break
		}
	}
	if total != 3 {
		t.Fatalf("drained %d swaps, want 3", total)
	}
	pos, _ := st.GetPosition(ctx, "0xu", "0xpool")
	if pos.TradeCount != 3 || pos.TotalUsdlSpent != "3000000" {
		t.Fatalf("position after drain = %+v", pos)
	}
}

func TestReconcilerPoisonPrefixReopensWithRefresh(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	st := seed(t)
	rdb := internaltest.NewRedis(t)
	txHash := fmt.Sprintf("0xrestart%x", time.Now().UnixNano())
	seedSwap(t, st, txHash+"-0", "0xu", true, "1000000", oneToken, 100, 0)
	seedSwap(t, st, txHash+"-1", "0xu", true, "poison", oneToken, 100, 1)
	for _, key := range []string{pnlcache.KeyPositions("0xu"), pnlcache.KeyPortfolio("0xu")} {
		if err := rdb.Set(ctx, key, "stale", 0).Err(); err != nil {
			t.Fatal(err)
		}
	}
	sub := rdb.Subscribe(ctx, constants.ChannelPnlUpdate)
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	worker := reconcile.New(st, rdb, discardLogger(), 500)
	if n, err := worker.RunOnce(ctx); n != 1 || err == nil {
		t.Fatalf("poison prefix=%d err=%v", n, err)
	}
	message, err := sub.ReceiveMessage(ctx)
	if err != nil || message.Payload != `{"userAddress":"0xu"}` {
		t.Fatalf("prefix signal=%+v err=%v", message, err)
	}
	for _, key := range []string{pnlcache.KeyPositions("0xu"), pnlcache.KeyPortfolio("0xu")} {
		if n, err := rdb.Exists(ctx, key).Result(); err != nil || n != 0 {
			t.Fatalf("stale cache %s exists=%d err=%v", key, n, err)
		}
	}
	config := st.Pool().Config()
	st.Pool().Close()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st = store.New(pool)
	if err := position.NewConsumer(st, rdb, discardLogger()).ProcessEvent(ctx, position.SwapEvent{
		PoolAddress: "0xpool", Sender: "0xu", IsBuy: true, AmountIn: "1000000", AmountOut: oneToken,
		Price: "0", Fee: "0", BlockNumber: 100, BlockTimestamp: 100, TxHash: txHash, LogIndex: 0,
	}); err != nil {
		t.Fatal(err)
	}
	worker = reconcile.New(st, rdb, discardLogger(), 500)
	if n, err := worker.RunOnce(ctx); n != 0 || err == nil {
		t.Fatalf("reopened poison=%d err=%v", n, err)
	}
	cursor, err := st.GetCursor(ctx)
	if err != nil || cursor.LastBlock != 100 || cursor.LastLogIndex != 0 {
		t.Fatalf("recovered cursor=%+v err=%v", cursor, err)
	}
	position, err := st.GetPosition(ctx, "0xu", "0xpool")
	if err != nil || position == nil || position.TradeCount != 1 || position.TotalUsdlSpent != "1000000" || position.CurrentHoldings != oneToken {
		t.Fatalf("recovered position=%+v err=%v", position, err)
	}
	trades, err := st.ListTrades(ctx, "0xu", store.TradeFilter{})
	if err != nil || len(trades) != 1 {
		t.Fatalf("recovered trades=%+v err=%v", trades, err)
	}
}
