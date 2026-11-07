package position_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/constants"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/pnlcache"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/position"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// setup builds a consumer over real Postgres + Redis with indexer.pools seeded so
// the swap consumer can resolve the pool's token.
func setup(t *testing.T) (*position.Consumer, *store.Store, *goredis.Client) {
	t.Helper()
	st := store.New(internaltest.NewPostgres(t))
	rdb := internaltest.NewRedis(t)
	internaltest.EnsureIndexerSchema(t, st.Pool())
	if _, err := st.Pool().Exec(context.Background(), `
		INSERT INTO indexer.pools (pool_address, token_address, creator, created_at)
		VALUES ('0xpool','0xtoken','0xc',1)`); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	return position.NewConsumer(st, rdb, discardLogger()), st, rdb
}

func TestProcessEventFoldsAndResolvesToken(t *testing.T) {
	ctx := context.Background()
	c, st, rdb := setup(t)

	// Seed a stale positions cache to prove the consumer busts it on a new fold.
	if err := rdb.Set(ctx, pnlcache.KeyPositions("0xbuyer"), `{"stale":true}`, 0).Err(); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	buy := position.SwapEvent{
		PoolAddress: "0xPool", Sender: "0xBuyer", IsBuy: true,
		AmountIn: "1000000", AmountOut: "1000000000000000000", // buy: USDL in, tokens out
		Price: "1000000", Fee: "10", BlockNumber: 50, BlockTimestamp: 100, TxHash: "0xtx", LogIndex: 0,
	}
	if err := c.ProcessEvent(ctx, buy); err != nil {
		t.Fatalf("process buy: %v", err)
	}

	pos, err := st.GetPosition(ctx, "0xbuyer", "0xpool")
	if err != nil || pos == nil {
		t.Fatalf("position: %v %v", pos, err)
	}
	if pos.TokenAddress != "0xtoken" {
		t.Errorf("token resolved = %q, want 0xtoken", pos.TokenAddress)
	}
	if pos.TotalUsdlSpent != "1000000" || pos.CurrentHoldings != "1000000000000000000" {
		t.Errorf("position = %+v", pos)
	}
	// Cache was invalidated.
	if n, _ := rdb.Exists(ctx, pnlcache.KeyPositions("0xbuyer")).Result(); n != 0 {
		t.Error("positions cache should be invalidated after a fold")
	}
}

// TestProcessEventPublishesPnlUpdate proves a new fold signals pnl:update for the
// trader so the client refetches the (busted) portfolio ONCE instead of polling.
func TestProcessEventPublishesPnlUpdate(t *testing.T) {
	ctx := context.Background()
	c, _, rdb := setup(t)

	sub := rdb.Subscribe(ctx, constants.ChannelPnlUpdate)
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Close()

	if err := c.ProcessEvent(ctx, position.SwapEvent{
		PoolAddress: "0xPool", Sender: "0xBuyer", IsBuy: true,
		AmountIn: "1000000", AmountOut: "1000000000000000000",
		Price: "1000000", Fee: "10", BlockNumber: 50, BlockTimestamp: 100, TxHash: "0xtx", LogIndex: 0,
	}); err != nil {
		t.Fatalf("process buy: %v", err)
	}

	var payload []byte
	select {
	case msg := <-sub.Channel():
		payload = []byte(msg.Payload)
	case <-time.After(3 * time.Second):
		t.Fatal("expected pnl:update publish")
	}

	var got struct {
		UserAddress string `json:"userAddress"`
	}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal pnl:update: %v", err)
	}
	if got.UserAddress != "0xbuyer" {
		t.Errorf("pnl:update userAddress = %q, want lowercased 0xbuyer", got.UserAddress)
	}
}

func TestProcessEventIdempotentRedelivery(t *testing.T) {
	ctx := context.Background()
	c, st, _ := setup(t)

	ev := position.SwapEvent{
		PoolAddress: "0xpool", Sender: "0xb", IsBuy: true,
		AmountIn: "1000000", AmountOut: "1000000000000000000",
		Price: "1000000", Fee: "0", BlockNumber: 1, BlockTimestamp: 1, TxHash: "0xtx", LogIndex: 0,
	}
	if err := c.ProcessEvent(ctx, ev); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := c.ProcessEvent(ctx, ev); err != nil {
		t.Fatalf("redeliver: %v", err)
	}
	pos, _ := st.GetPosition(ctx, "0xb", "0xpool")
	if pos.TradeCount != 1 || pos.TotalUsdlSpent != "1000000" {
		t.Fatalf("redelivery double-counted: %+v", pos)
	}
}

func TestProcessSellLegSplit(t *testing.T) {
	ctx := context.Background()
	c, st, _ := setup(t)

	// A sell: amountIn = tokens, amountOut = USDL received.
	sell := position.SwapEvent{
		PoolAddress: "0xpool", Sender: "0xs", IsBuy: false,
		AmountIn: "1000000000000000000", AmountOut: "6000000",
		Price: "6000000", Fee: "0", BlockNumber: 2, BlockTimestamp: 2, TxHash: "0xtx", LogIndex: 0,
	}
	// An unbacked sell must fail atomically, without recording the trade or
	// inventing a position. The same event can be retried after its buy arrives.
	if err := c.ProcessEvent(ctx, sell); err == nil {
		t.Fatal("sell without inventory accepted")
	}
	var trades, positions int
	if err := st.Pool().QueryRow(ctx, `SELECT
		(SELECT count(*) FROM pnl.user_trades WHERE user_address='0xs'),
		(SELECT count(*) FROM pnl.user_positions WHERE user_address='0xs')`).Scan(&trades, &positions); err != nil {
		t.Fatal(err)
	}
	if trades != 0 || positions != 0 {
		t.Fatalf("unbacked sell persisted: trades=%d positions=%d", trades, positions)
	}
	if err := c.ProcessEvent(ctx, position.SwapEvent{
		PoolAddress: "0xpool", Sender: "0xs", IsBuy: true,
		AmountIn: "1000000", AmountOut: "1000000000000000000",
		Price: "1000000", Fee: "0", BlockNumber: 1, BlockTimestamp: 1, TxHash: "0xbuy", LogIndex: 0,
	}); err != nil {
		t.Fatalf("process prior buy: %v", err)
	}
	if err := c.ProcessEvent(ctx, sell); err != nil {
		t.Fatalf("process sell: %v", err)
	}
	pos, err := st.GetPosition(ctx, "0xs", "0xpool")
	if err != nil || pos == nil {
		t.Fatalf("position: %v %v", pos, err)
	}
	// Six USDL proceeds less one USDL cost: five USDL realized profit.
	if pos.RealizedPnlUsdl != "5000000" || pos.TotalTokensSold != "1000000000000000000" {
		t.Fatalf("sell fold = %+v", pos)
	}
	if pos.TotalUsdlSpent != "1000000" || pos.TotalUsdlReceived != "6000000" || pos.CurrentHoldings != "0" || pos.TradeCount != 2 {
		t.Fatalf("sell legs or prior inventory lost: %+v", pos)
	}
}

func TestProcessIndexerWebhookAfterStoreReopen(t *testing.T) {
	repairM1015Restart(t)
}

func repairM1015Restart(t *testing.T) {
	t.Helper()
	h := repairM1015NewHarness(t)
	buy := repairM1015Fixture(t, "buy.json")
	sell := repairM1015Fixture(t, "sell.json")
	h.seedCache(t)
	h.deliver(t, buy)
	h.assertCache(t, "")
	h.notification(t, 1)

	assertBuy := func() *store.PositionRow {
		t.Helper()
		pos, err := h.store.GetPosition(h.ctx, repairM1015User, repairM1015Pool)
		if err != nil || pos == nil {
			t.Fatalf("indexed buy position: %+v, %v", pos, err)
		}
		if pos.UserAddress != repairM1015User || pos.PoolAddress != repairM1015Pool || pos.TokenAddress != repairM1015Token ||
			pos.TotalUsdlSpent != "100000000" || pos.TotalTokensBought != "9606813905120" ||
			pos.CurrentHoldings != "9606813905120" || pos.AvgCostBasis != "10409278350515" ||
			pos.TotalUsdlReceived != "0" || pos.TotalTokensSold != "0" || pos.RealizedPnlUsdl != "0" ||
			pos.TradeCount != 1 || pos.FirstBuyTs == nil || *pos.FirstBuyTs != 1791875553 || pos.LastTradeTs != 1791875553 {
			t.Fatalf("buy units, token resolution or aggregates: %+v", pos)
		}
		trades, err := h.store.ListTrades(h.ctx, repairM1015User, store.TradeFilter{})
		if err != nil || len(trades) != 1 {
			t.Fatalf("one trade per emitted buy: %+v, %v", trades, err)
		}
		if trade := trades[0]; trade.ID != "0x3d28a4d088c87b4eca7507e74b47bdc01ae30ff4b1e478dc404a1caa9921a503-8" ||
			!trade.IsBuy || trade.UsdlAmount != "100000000" || trade.TokenAmount != "9606813905120" ||
			trade.TokenAddress != repairM1015Token || trade.UserAddress != repairM1015User || trade.PoolAddress != repairM1015Pool ||
			trade.Price != "10194940899999" || trade.Fee != "3000000" || trade.BlockNumber != 33963733 ||
			trade.BlockTimestamp != 1791875553 || trade.TxHash != "0x3d28a4d088c87b4eca7507e74b47bdc01ae30ff4b1e478dc404a1caa9921a503" {
			t.Fatalf("persisted producer buy: %+v", trade)
		}
		return pos
	}
	beforeBuy := assertBuy()
	oldPool, oldStore := h.pool, h.store
	h.reopen(t)
	if h.pool == oldPool || h.store == oldStore {
		t.Fatal("restart reused the consumer's pool or store")
	}
	if got := assertBuy(); !reflect.DeepEqual(got, beforeBuy) {
		t.Fatalf("buy state lost on reopen: got %+v, want %+v", got, beforeBuy)
	}
	h.seedCache(t)
	h.deliver(t, buy)
	if got := assertBuy(); !reflect.DeepEqual(got, beforeBuy) {
		t.Fatalf("buy redelivery changed recovered state: %+v", got)
	}
	h.assertCache(t, "stale")
	if n := h.redisPeer.publishCount(); n != 1 {
		t.Fatalf("buy redelivery published %d updates, want one original update", n)
	}

	h.deliver(t, sell)
	h.assertCache(t, "")
	h.notification(t, 2)
	assertSell := func() *store.PositionRow {
		t.Helper()
		pos, err := h.store.GetPosition(h.ctx, repairM1015User, repairM1015Pool)
		if err != nil || pos == nil {
			t.Fatalf("indexed sell position: %+v, %v", pos, err)
		}
		if pos.TotalUsdlSpent != "100000000" || pos.TotalTokensBought != "9606813905120" ||
			pos.TotalUsdlReceived != "47278912" || pos.TotalTokensSold != "4803406952560" ||
			pos.CurrentHoldings != "4803406952560" || pos.RealizedPnlUsdl != "-2721088" ||
			pos.AvgCostBasis != "10409278350515" || pos.TradeCount != 2 || pos.TokenAddress != repairM1015Token ||
			pos.FirstBuyTs == nil || *pos.FirstBuyTs != 1791875553 || pos.LastTradeTs != 1791875556 {
			t.Fatalf("sell units or remaining basis: %+v", pos)
		}
		trades, err := h.store.ListTrades(h.ctx, repairM1015User, store.TradeFilter{})
		if err != nil || len(trades) != 2 {
			t.Fatalf("one trade per emitted swap: %+v, %v", trades, err)
		}
		counts := map[bool]int{}
		for _, trade := range trades {
			counts[trade.IsBuy]++
			if !trade.IsBuy && (trade.ID != "0xf5f9ed1136e71f03ffd44ae7f80ee192998b80fccd96ad3f75824fa9ef3c2845-2" ||
				trade.UsdlAmount != "47278912" || trade.TokenAmount != "4803406952560" ||
				trade.Fee != "144102208576" || trade.Price != "10098226981692" ||
				trade.BlockNumber != 33963749 || trade.BlockTimestamp != 1791875556 ||
				trade.TxHash != "0xf5f9ed1136e71f03ffd44ae7f80ee192998b80fccd96ad3f75824fa9ef3c2845" ||
				trade.UserAddress != repairM1015User || trade.PoolAddress != repairM1015Pool || trade.TokenAddress != repairM1015Token) {
				t.Fatalf("persisted producer sell: %+v", trade)
			}
		}
		if counts[true] != 1 || counts[false] != 1 {
			t.Fatalf("wrong buy/sell counts: %v", counts)
		}
		return pos
	}
	beforeSell := assertSell()
	h.reopen(t)
	if got := assertSell(); !reflect.DeepEqual(got, beforeSell) {
		t.Fatalf("sell state lost on reopen: got %+v, want %+v", got, beforeSell)
	}
	h.seedCache(t)
	h.deliver(t, sell)
	h.deliver(t, buy)
	if got := assertSell(); !reflect.DeepEqual(got, beforeSell) {
		t.Fatalf("redelivery changed recovered sell state: %+v", got)
	}
	h.assertCache(t, "stale")
	if n := h.redisPeer.publishCount(); n != 2 {
		t.Fatalf("redelivery published %d updates, want two original updates", n)
	}
}
