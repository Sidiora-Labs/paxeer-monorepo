package quoter

import (
	"context"
	"strings"
	"testing"
	"time"

	perpskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/stretchr/testify/require"
)

func carryOverMarkets(t *testing.T, c *keeperChain) {
	t.Helper()
	for _, id := range []uint64{1, 2} {
		m, err := c.k.GetMarket(c.ctx, 0)
		require.NoError(t, err)
		m.ID = id
		m.Symbol, m.Name, m.OracleSymbol = map[uint64]string{1: "ETH", 2: "SOL"}[id], map[uint64]string{1: "Ethereum", 2: "Solana"}[id], map[uint64]string{1: "ETH", 2: "SOL"}[id]
		require.NoError(t, c.k.SetMarket(c.ctx, c.k.Authority(), m))
		c.ctx.KVStore(c.pfKey).Set(pricetypes.AggregateKey(id), c.ctx.KVStore(c.pfKey).Get(pricetypes.AggregateKey(0)))
		_, err = c.k.UpdatePrices(c.ctx)
		require.NoError(t, err)
		state, err := c.k.GetMarketState(c.ctx, id)
		require.NoError(t, err)
		state.MarkPrice = word(usd(t, "60000"))
		c.k.SetMarketState(c.ctx, state)
	}
}

func TestCarryOverKeepsLadderWithoutTransactions(t *testing.T) {
	c := newKeeperChain(t)
	first, _ := testEngine(t, c, nil)
	round(t, first, c)
	before := orders(t, c)
	c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(time.Second))
	second, logs := testEngine(t, c, nil)
	c.snapshots = nil
	round(t, second, c)
	require.Equal(t, before, orders(t, c))
	require.Equal(t, 4, c.places)
	require.Zero(t, c.cancels)
	require.Zero(t, second.Health().TransactionsLastRound)
	require.Equal(t, map[uint64]int{0: 1}, c.snapshots)
	require.Contains(t, logs.String(), `"msg":"quoter keep","market":0,"round":`)
	require.NotContains(t, logs.String(), `"msg":"quoter cancel"`)
	require.NotContains(t, logs.String(), `"msg":"quoter place"`)
	require.GreaterOrEqual(t, second.generation, before[0].Round)
	t.Log("second engine: 0 cancellations, 0 placements; replacement of four resting orders requires 8 transactions")
}

func TestCarryOverRenewsAfterPriceMove(t *testing.T) {
	c := newKeeperChain(t)
	first, _ := testEngine(t, c, nil)
	round(t, first, c)
	second, _ := testEngine(t, c, nil)
	round(t, second, c)
	prior := orders(t, c)
	c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(time.Second))
	c.price(t, "60100", c.ctx.BlockTime())
	round(t, second, c)
	require.Equal(t, 4, c.cancels)
	require.Equal(t, 8, c.places)
	for _, o := range orders(t, c) {
		require.Greater(t, o.Round, prior[0].Round)
	}
}

func TestCarryOverLifetimeStartsAtOriginalRound(t *testing.T) {
	c := newKeeperChain(t)
	first, _ := testEngine(t, c, nil)
	round(t, first, c)
	born := c.ctx.BlockTime()
	c.ctx = c.ctx.WithBlockTime(born.Add(20 * time.Second))
	c.price(t, "60000", c.ctx.BlockTime())
	second, _ := testEngine(t, c, nil)
	round(t, second, c)
	require.Zero(t, second.Health().TransactionsLastRound)
	require.Equal(t, born, second.state(0).born)
	c.ctx = c.ctx.WithBlockTime(born.Add(30 * time.Second))
	c.price(t, "60000", c.ctx.BlockTime())
	round(t, second, c)
	require.Equal(t, 8, second.Health().TransactionsLastRound)
}

func TestCarryOverRejectsExpiredAndFutureRounds(t *testing.T) {
	for _, kind := range []string{"expired", "future"} {
		t.Run(kind, func(t *testing.T) {
			c := newKeeperChain(t)
			first, _ := testEngine(t, c, nil)
			if kind == "future" {
				_, err := first.Round(context.Background(), c.ctx.BlockTime().Add(time.Second))
				require.NoError(t, err)
			} else {
				round(t, first, c)
				c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(30 * time.Second))
				c.price(t, "60000", c.ctx.BlockTime())
			}
			second, logs := testEngine(t, c, nil)
			round(t, second, c)
			require.NotContains(t, logs.String(), `"msg":"quoter keep"`)
			require.Equal(t, 8, second.Health().TransactionsLastRound)
			require.Len(t, orders(t, c), 4)
		})
	}
}

func TestCarryOverRejectsChangedSettings(t *testing.T) {
	for _, kind := range []string{"size", "spread"} {
		t.Run(kind, func(t *testing.T) {
			c := newKeeperChain(t)
			first, _ := testEngine(t, c, nil)
			round(t, first, c)
			c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(time.Second))
			second, logs := testEngine(t, c, func(q *config.QuoterConfig) {
				if kind == "size" {
					q.LevelUSD = usd(t, "30")
				} else {
					q.SpreadBPS = 20
				}
			})
			round(t, second, c)
			require.Equal(t, 8, second.Health().TransactionsLastRound)
			require.NotContains(t, logs.String(), `"msg":"quoter keep"`)
			require.Len(t, orders(t, c), 4)
		})
	}
}

func TestCarryOverRejectsMissingOrder(t *testing.T) {
	c := newKeeperChain(t)
	first, _ := testEngine(t, c, nil)
	round(t, first, c)
	require.NoError(t, c.Cancel(context.Background(), orders(t, c)[0]))
	c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(time.Second))
	second, logs := testEngine(t, c, nil)
	round(t, second, c)
	require.Equal(t, 7, second.Health().TransactionsLastRound)
	require.NotContains(t, logs.String(), `"msg":"quoter keep"`)
	require.Len(t, orders(t, c), 4)
}

func TestCarryOverRejectsPartialFill(t *testing.T) {
	c := newKeeperChain(t)
	first, _ := testEngine(t, c, nil)
	round(t, first, c)
	takerFill(t, c, true)
	c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(time.Second))
	second, logs := testEngine(t, c, nil)
	round(t, second, c)
	require.Equal(t, 8, second.Health().TransactionsLastRound)
	require.NotContains(t, logs.String(), `"msg":"quoter keep"`)
	require.Len(t, orders(t, c), 4)
}

func TestCarryOverRejectsOrderWithoutClientID(t *testing.T) {
	c := newKeeperChain(t)
	first, _ := testEngine(t, c, nil)
	round(t, first, c)
	_, err := c.k.PlaceOrder(c.ctx, c.account, perpskeeper.OrderParams{MarketID: 0, IsLong: true, OrderType: 0, TimeInForce: 0, SizeUsd: word(usd(t, "25")), Leverage: word(unit), LimitPrice: word(usd(t, "59000"))})
	require.NoError(t, err)
	c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(time.Second))
	second, logs := testEngine(t, c, func(q *config.QuoterConfig) { q.MaxTransactionsPerRound = 256 })
	round(t, second, c)
	require.Equal(t, 9, second.Health().TransactionsLastRound)
	require.NotContains(t, logs.String(), `"msg":"quoter keep"`)
	require.Len(t, orders(t, c), 4)
}

func TestCarryOverRejectsMixedRounds(t *testing.T) {
	c := newKeeperChain(t)
	first, _ := testEngine(t, c, nil)
	round(t, first, c)
	prior := orders(t, c)[0]
	require.NoError(t, c.Cancel(context.Background(), prior))
	prior.Round++
	prior.Cloid = ClientID(prior.Market, prior.Buy, prior.Level, prior.Round)
	require.NoError(t, c.Place(context.Background(), prior))
	c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(time.Second))
	second, logs := testEngine(t, c, nil)
	round(t, second, c)
	require.Equal(t, 8, second.Health().TransactionsLastRound)
	require.NotContains(t, logs.String(), `"msg":"quoter keep"`)
}

func TestCarryOverSignalAndStopPrecedence(t *testing.T) {
	for _, kind := range []string{"signal", "stop", "both", "configured stop", "default", "internal cancellation"} {
		t.Run(kind, func(t *testing.T) {
			c := newKeeperChain(t)
			e, logs := testEngine(t, c, func(q *config.QuoterConfig) { q.Handover = kind != "default" })
			round(t, e, c)
			before := orders(t, c)
			e.cfg.Stop = kind == "configured stop"
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind != "stop" && kind != "configured stop" {
				cancel()
			}
			stop := func() bool { return kind == "stop" || kind == "both" }
			c.snapshots = nil
			require.NoError(t, e.Run(ctx, stop, func() bool { return kind != "internal cancellation" }))
			require.True(t, e.Health().Stopped)
			if kind == "signal" {
				require.Equal(t, before, orders(t, c))
				require.Zero(t, c.cancels)
				require.Equal(t, 4, e.Health().OrdersLeft)
				require.Contains(t, logs.String(), `"handover":true`)
				require.Empty(t, c.snapshots)
			} else {
				require.Empty(t, orders(t, c))
				require.Equal(t, 4, c.cancels)
				require.Zero(t, e.Health().OrdersLeft)
			}
		})
	}
}

func TestCarryOverWalkResumesAtRefusedMarket(t *testing.T) {
	c := newKeeperChain(t)
	carryOverMarkets(t, c)
	e, _ := testEngine(t, c, func(q *config.QuoterConfig) { q.MaxTransactionsPerRound = 4 })
	round(t, e, c)
	require.Equal(t, uint64(1), *e.nextMarket)
	require.Equal(t, map[uint64]int{0: 6, 1: 1}, c.snapshots)
	require.Len(t, e.Health().Markets, 2)
	c.snapshots = nil
	round(t, e, c)
	require.Equal(t, uint64(2), *e.nextMarket)
	require.Equal(t, map[uint64]int{1: 6, 2: 1}, c.snapshots)
	require.Len(t, orders(t, c), 8)
}

func TestCarryOverWalkCompletesEveryMarket(t *testing.T) {
	c := newKeeperChain(t)
	carryOverMarkets(t, c)
	e, _ := testEngine(t, c, func(q *config.QuoterConfig) { q.MaxTransactionsPerRound = 3 })
	for i := 0; i < 4; i++ {
		round(t, e, c)
		require.Equal(t, 3, e.Health().TransactionsLastRound)
	}
	open := orders(t, c)
	require.Len(t, open, 12)
	for _, id := range []uint64{0, 1, 2} {
		count := 0
		for _, o := range open {
			if o.Market == id {
				count++
			}
		}
		require.Equal(t, 4, count)
	}
	require.Nil(t, e.nextMarket)
	c.snapshots = nil
	round(t, e, c)
	require.Equal(t, map[uint64]int{0: 1, 1: 1, 2: 1}, c.snapshots)
	require.Zero(t, e.Health().TransactionsLastRound)
}

func TestCarryOverCancelLogsCountSubmittedTransactions(t *testing.T) {
	c := newKeeperChain(t)
	carryOverMarkets(t, c)
	e, logs := testEngine(t, c, func(q *config.QuoterConfig) { q.MaxTransactionsPerRound = 256 })
	round(t, e, c)
	e.cfg.MaxTransactionsPerRound = 3
	c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(11 * time.Second))
	logs.Reset()
	c.snapshots = nil
	round(t, e, c)
	require.Equal(t, 3, c.cancels)
	require.Equal(t, c.cancels, strings.Count(logs.String(), `"msg":"quoter cancel"`))
	require.Equal(t, map[uint64]int{0: 2}, c.snapshots)
	require.Equal(t, uint64(0), *e.nextMarket)
	require.Len(t, e.Health().Markets[0].Orders, 1)
	require.Len(t, e.Health().Markets[1].Orders, 4)
	require.Len(t, e.Health().Markets[2].Orders, 4)
}

func TestCarryOverUnreadableMarketRespectsBound(t *testing.T) {
	c := newKeeperChain(t)
	carryOverMarkets(t, c)
	e, logs := testEngine(t, c, func(q *config.QuoterConfig) {
		q.Markets = []uint64{0, 1, 2}
		q.MaxTransactionsPerRound = 256
	})
	round(t, e, c)
	e.cfg.MaxTransactionsPerRound = 2
	c.ctx.KVStore(c.perpsKey).Delete(perpstypes.MarketKey(0))
	logs.Reset()
	c.snapshots = nil
	_, err := e.Round(context.Background(), c.ctx.BlockTime())
	require.Error(t, err)
	require.Equal(t, 2, c.cancels)
	require.Equal(t, 2, strings.Count(logs.String(), `"msg":"quoter cancel"`))
	require.Equal(t, map[uint64]int{0: 2}, c.snapshots)
	require.Equal(t, uint64(0), *e.nextMarket)
	require.Len(t, orders(t, c), 10)
}

func TestCarryOverKeepsPositionBoundedLadder(t *testing.T) {
	c := newKeeperChain(t)
	edit := func(q *config.QuoterConfig) { q.MaxPositionUSD = usd(t, "30") }
	first, _ := testEngine(t, c, edit)
	round(t, first, c)
	before := orders(t, c)
	require.Len(t, before, 2)
	second, logs := testEngine(t, c, edit)
	round(t, second, c)
	require.Equal(t, before, orders(t, c))
	require.Zero(t, second.Health().TransactionsLastRound)
	require.Contains(t, logs.String(), `"msg":"quoter keep"`)
}

func TestCarryOverRecoversAnchorAcrossTickRounding(t *testing.T) {
	c := newKeeperChain(t)
	c.price(t, "60000.035", c.ctx.BlockTime())
	first, _ := testEngine(t, c, nil)
	round(t, first, c)
	before := orders(t, c)
	c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(time.Second))
	c.price(t, "60000.04", c.ctx.BlockTime())
	second, logs := testEngine(t, c, nil)
	round(t, second, c)
	require.Zero(t, second.Health().TransactionsLastRound)
	require.Equal(t, before, orders(t, c))
	require.Contains(t, logs.String(), `"msg":"quoter keep"`)
}

func TestCarryOverAttemptsOnlyFirstSnapshot(t *testing.T) {
	c := newKeeperChain(t)
	first, _ := testEngine(t, c, nil)
	round(t, first, c)
	c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(time.Second))
	second, logs := testEngine(t, c, nil)
	second.state(0).health.RestUntil = c.ctx.BlockTime().Add(time.Second)
	second.cfg.Mode = "dry"
	round(t, second, c)
	require.True(t, second.state(0).started)
	require.Nil(t, second.state(0).anchor)
	c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(2 * time.Second))
	c.price(t, "60000", c.ctx.BlockTime())
	round(t, second, c)
	require.NotContains(t, logs.String(), `"msg":"quoter keep"`)
	require.Greater(t, second.state(0).round, orders(t, c)[0].Round)
	require.Equal(t, 4, c.places)
	require.Zero(t, c.cancels)
}
