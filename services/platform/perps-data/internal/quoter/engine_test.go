package quoter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	perpskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricetest "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/testutil"
	pricetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// keeperChain adapts the actual venue, book, margin and pricefeed keepers to
// the same interface as the RPC transport. Every write uses keeper validation.
type keeperChain struct {
	k               *perpskeeper.Keeper
	pf              pricekeeper.Keeper
	ctx             sdk.Context
	perpsKey, pfKey sdk.StoreKey
	account         common.Address
	places, cancels int
	snapshots       map[uint64]int
}

func usd(t *testing.T, raw string) *big.Int {
	t.Helper()
	n, err := config.ParseUSD(raw)
	require.NoError(t, err)
	return n
}
func word(n *big.Int) uint256.Int { var w uint256.Int; w.SetFromBig(n); return w }
func newKeeperChain(t *testing.T) *keeperChain {
	t.Helper()
	a := app.Setup(t, false, false, false)
	ctx := a.GetContextForDeliverTx([]byte{}).WithBlockHeight(10).WithBlockTime(time.Unix(1_700_000_010, 0).UTC())
	pfKey := sdk.NewKVStoreKey(pricetypes.StoreKey)
	ctx = pricetest.WithPriceFeedStore(ctx, pfKey)
	pf := pricekeeper.NewKeeper(pfKey, a.StakingKeeper, a.AccountKeeper, &a.EvmKeeper)
	perpsKey := sdk.NewKVStoreKey(perpstypes.StoreKey)
	ctx = pricetest.WithPriceFeedStore(ctx, perpsKey)
	k := perpskeeper.NewKeeper(perpsKey, &a.EvmKeeper, pf)
	c := &keeperChain{k: k, pf: pf, ctx: ctx, pfKey: pfKey, perpsKey: perpsKey, account: common.BytesToAddress([]byte{1})}
	m := perpstypes.Market{ID: 0, Symbol: "BTC", Name: "Bitcoin", OracleSymbol: "BTC", SzDecimals: 5, PxDecimals: 1, MaxLeverage: 1000, TierClass: perpstypes.TierClassA, ImpactNotionalUsd: sdk.NewIntWithDecimal(20000, 18), OiCapUsd: sdk.NewIntWithDecimal(500000000, 18), FundingCapPpmPerHour: 40000, Enabled: true}
	require.NoError(t, k.SetMarket(ctx, k.Authority(), m))
	c.price(t, "60000", ctx.BlockTime().Add(-time.Second))
	c.fund(t, c.account, 2000)
	return c
}
func (c *keeperChain) fund(t *testing.T, address common.Address, amount int64) {
	t.Helper()
	c.ctx.KVStore(c.perpsKey).Set(perpstypes.AccountKey(address), perpstypes.Account{Address: address, Collateral: *uint256.NewInt(uint64(amount) * 1_000_000), Created: true}.Marshal())
}
func (c *keeperChain) price(t *testing.T, amount string, at time.Time) {
	t.Helper()
	n := usd(t, amount)
	raw, err := json.Marshal(pricetypes.Aggregate{IndexPrice: sdk.NewIntFromBigInt(n), MarkPrice: sdk.NewIntFromBigInt(n), Basis: sdk.ZeroInt(), TimestampMs: uint64(at.UnixMilli()), Height: c.ctx.BlockHeight(), Sources: 1})
	require.NoError(t, err)
	c.ctx.KVStore(c.pfKey).Set(pricetypes.AggregateKey(0), raw)
	_, err = c.k.UpdatePrices(c.ctx)
	require.NoError(t, err)
	state, err := c.k.GetMarketState(c.ctx, 0)
	require.NoError(t, err)
	state.MarkPrice = word(n)
	c.k.SetMarketState(c.ctx, state)
}
func (c *keeperChain) ActiveMarkets(context.Context) ([]uint64, error) {
	var ids []uint64
	err := c.k.IterateMarkets(c.ctx, func(m perpstypes.Market) bool {
		if m.Enabled {
			ids = append(ids, m.ID)
		}
		return false
	})
	return ids, err
}
func (c *keeperChain) Orders(context.Context) ([]Order, error) {
	ids, err := c.k.AccountOpenOrders(c.ctx, c.account)
	if err != nil {
		return nil, err
	}
	out := []Order{}
	for _, id := range ids {
		o, err := c.k.GetOrder(c.ctx, id)
		if err != nil {
			return nil, err
		}
		left := new(big.Int).Sub(o.SizeBase.ToBig(), o.FilledBase.ToBig())
		_, _, level, round, _ := decodeClientID(o.Cloid)
		out = append(out, Order{ID: o.ID.ToBig(), Market: o.MarketID, Buy: o.IsLong, Level: level, Round: round, Cloid: o.Cloid, Price: o.LimitPrice.ToBig(), SizeUSD: o.SizeUsd.ToBig(), Base: left})
	}
	return out, nil
}
func (c *keeperChain) Snapshot(ctx context.Context, id uint64) (Snapshot, error) {
	if c.snapshots == nil {
		c.snapshots = map[uint64]int{}
	}
	c.snapshots[id]++
	s := Snapshot{PositionBase: new(big.Int)}
	m, err := c.k.GetMarket(c.ctx, id)
	if err != nil {
		return s, err
	}
	p, err := c.pf.GetPrice(c.ctx, id)
	if err != nil {
		return s, err
	}
	state, err := c.k.GetMarketState(c.ctx, id)
	if err != nil {
		return s, err
	}
	params, err := c.k.GetParams(c.ctx)
	if err != nil {
		return s, err
	}
	s.Market = Market{ID: id, Tick: new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(18-m.PxDecimals)), nil), Lot: new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(18-m.SzDecimals)), nil), MinOrderUSD: params.MinOrderValueUsd.BigInt(), Enabled: m.Enabled}
	s.Price = Price{Mark: state.MarkPrice.ToBig(), Time: time.UnixMilli(int64(p.TimestampMs)), Fresh: p.Fresh}
	orders, err := c.Orders(ctx)
	if err != nil {
		return s, err
	}
	for _, o := range orders {
		if o.Market == id {
			s.Orders = append(s.Orders, o)
		}
	}
	positions, err := c.k.AccountPositions(c.ctx, c.account)
	if err != nil {
		return s, err
	}
	for _, p := range positions {
		if p.MarketID == id {
			s.PositionBase = p.SizeBase.ToBig()
			if !p.IsLong {
				s.PositionBase.Neg(s.PositionBase)
			}
		}
	}
	summary, err := c.k.AccountMargin(c.ctx, c.account)
	if err != nil {
		return s, err
	}
	account, err := c.k.GetAccount(c.ctx, c.account)
	if err != nil {
		return s, err
	}
	free := new(big.Int).Sub(summary.Equity.BigInt(), summary.InitialMargin.BigInt())
	if free.Sign() < 0 {
		free.SetInt64(0)
	}
	idle := new(big.Int).Sub(new(big.Int).Mul(account.Collateral.ToBig(), big.NewInt(1_000_000_000_000)), summary.InitialMargin.BigInt())
	if idle.Sign() < 0 {
		idle.SetInt64(0)
	}
	withdrawable, err := c.k.Withdrawable(c.ctx, c.account)
	if err != nil {
		return s, err
	}
	s.FreeUSD = min(min(free, idle), new(big.Int).Mul(withdrawable.ToBig(), big.NewInt(1_000_000_000_000)))
	tier, err := c.k.FeeTier(c.ctx, c.account)
	if err != nil {
		return s, err
	}
	s.TakerFeePPM = tier.TakerFeePpm
	return s, nil
}
func (c *keeperChain) Place(_ context.Context, o Order) error {
	c.places++
	_, err := c.k.PlaceOrder(c.ctx, c.account, perpskeeper.OrderParams{MarketID: o.Market, IsLong: o.Buy, OrderType: 0, TimeInForce: 3, SizeUsd: word(o.SizeUSD), Leverage: word(unit), LimitPrice: word(o.Price), Cloid: o.Cloid})
	if errors.Is(err, perpstypes.ErrPaused) {
		return refused("PerpsPaused")
	}
	if err != nil {
		return refused("transaction refused")
	}
	return nil
}
func (c *keeperChain) Cancel(_ context.Context, o Order) error {
	c.cancels++
	var err error
	if o.Cloid != ([16]byte{}) {
		_, err = c.k.CancelByCloid(c.ctx, c.account, o.Market, o.Cloid)
	} else {
		_, err = c.k.CancelOrder(c.ctx, c.account, word(o.ID))
	}
	return err
}
func testEngine(t *testing.T, c *keeperChain, edit func(*config.QuoterConfig)) (*Engine, *bytes.Buffer) {
	t.Helper()
	q := config.DefaultQuoterConfig()
	q.KeyFile = "account-key"
	q.Mode = "live"
	if edit != nil {
		edit(&q)
	}
	var logs bytes.Buffer
	e, err := New(c, q, slog.New(slog.NewJSONHandler(&logs, nil)))
	require.NoError(t, err)
	return e, &logs
}
func round(t *testing.T, e *Engine, c *keeperChain) map[uint64][]Order {
	t.Helper()
	p, err := e.Round(context.Background(), c.ctx.BlockTime())
	require.NoError(t, err)
	return p
}
func orders(t *testing.T, c Chain) []Order {
	t.Helper()
	out, err := c.Orders(context.Background())
	require.NoError(t, err)
	return out
}

func TestLadderTickLotAndMinimumValue(t *testing.T) {
	q := config.DefaultQuoterConfig()
	q.KeyFile = "account-key"
	q.LevelUSD = usd(t, "25")
	m := Market{ID: 7, Tick: usd(t, "0.25"), Lot: usd(t, "0.01"), MinOrderUSD: usd(t, "10"), Enabled: true}
	ladder := Ladder(m, usd(t, "101.01"), q, 1234)
	require.Len(t, ladder, 4)
	for _, o := range ladder {
		require.Zero(t, new(big.Int).Mod(o.Price, m.Tick).Sign())
		require.Zero(t, new(big.Int).Mod(o.Base, m.Lot).Sign())
		require.GreaterOrEqual(t, new(big.Int).Quo(new(big.Int).Mul(o.Base, o.Price), unit).Cmp(m.MinOrderUSD), 0)
		if o.Buy {
			require.Negative(t, o.Price.Cmp(usd(t, "101.01")))
		} else {
			require.Positive(t, o.Price.Cmp(usd(t, "101.01")))
		}
		id, buy, level, generation, ok := decodeClientID(o.Cloid)
		require.True(t, ok)
		require.Equal(t, uint64(7), id)
		require.Equal(t, o.Buy, buy)
		require.Equal(t, o.Level, level)
		require.Equal(t, uint64(1234), generation)
		require.True(t, noSelfCross(o, ladder))
	}
	require.Equal(t, usd(t, "100.75"), ladder[0].Price)
	require.Equal(t, usd(t, "101.25"), ladder[1].Price)
	q.LevelUSD = usd(t, "9.99")
	require.Empty(t, Ladder(m, usd(t, "101.01"), q, 1234))
	q.LevelUSD = usd(t, "10")
	m.Lot = usd(t, "0.1")
	require.Empty(t, Ladder(m, usd(t, "101.01"), q, 1234))
	// The actual venue accepts every generated request at its tick and lot.
	c := newKeeperChain(t)
	e, _ := testEngine(t, c, nil)
	round(t, e, c)
	require.Len(t, orders(t, c), 4)
}
func TestRenewalAfterMoveAndAge(t *testing.T) {
	for _, kind := range []string{"price", "age"} {
		t.Run(kind, func(t *testing.T) {
			c := newKeeperChain(t)
			e, _ := testEngine(t, c, nil)
			first := round(t, e, c)
			before := orders(t, c)
			round(t, e, c)
			require.Equal(t, 4, c.places)
			require.Zero(t, c.cancels)
			c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(time.Second))
			if kind == "price" {
				c.price(t, "60100", c.ctx.BlockTime())
			} else {
				c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(30 * time.Second))
				c.price(t, "60000", c.ctx.BlockTime())
			}
			second := round(t, e, c)
			require.Len(t, orders(t, c), 4)
			require.Equal(t, 8, c.places)
			require.Equal(t, 4, c.cancels)
			require.NotEqual(t, first[0][0].Cloid, second[0][0].Cloid)
			for _, old := range before {
				record, err := c.k.GetOrder(c.ctx, word(old.ID))
				require.NoError(t, err)
				require.Equal(t, perpstypes.OrderStatusCancelled, record.Status)
			}
		})
	}
}

func TestRoundsBoundTransactionsAndResumeReconciliation(t *testing.T) {
	c := newKeeperChain(t)
	e, _ := testEngine(t, c, func(q *config.QuoterConfig) { q.MaxTransactionsPerRound = 3 })
	step := func(writes, resting int) {
		t.Helper()
		before := c.places + c.cancels
		round(t, e, c)
		require.Equal(t, writes, c.places+c.cancels-before)
		require.Equal(t, writes, e.Health().TransactionsLastRound)
		require.LessOrEqual(t, writes, e.cfg.MaxTransactionsPerRound)
		require.Len(t, orders(t, c), resting)
		require.Len(t, e.Health().Markets[0].Orders, resting)
	}
	step(3, 3)
	first := orders(t, c)
	step(1, 4)
	step(0, 4)
	c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(time.Second))
	c.price(t, "60100", c.ctx.BlockTime())
	step(3, 1)
	// Finish cancellation before placing the first replacement levels.
	require.Equal(t, 4, c.places)
	step(3, 2)
	step(2, 4)
	for _, o := range orders(t, c) {
		require.NotEqual(t, first[0].Round, o.Round)
	}
	// Stale prices drain the remaining orders over bounded rounds.
	c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(11 * time.Second))
	step(3, 1)
	step(1, 0)
	step(0, 0)
	c.price(t, "60100", c.ctx.BlockTime())
	step(3, 3)
	step(1, 4)
	result, err := e.Stop(context.Background())
	require.NoError(t, err)
	require.Zero(t, result.OrdersLeft)
	require.Empty(t, orders(t, c))
}
func takerFill(t *testing.T, c *keeperChain, buy bool) uint256.Int {
	t.Helper()
	other := common.BytesToAddress([]byte{2})
	c.fund(t, other, 2000)
	size := usd(t, "20")
	result, err := c.k.PlaceOrder(c.ctx, other, perpskeeper.OrderParams{MarketID: 0, IsLong: buy, OrderType: 4, TimeInForce: 1, SizeUsd: word(size), Leverage: word(unit)})
	require.NoError(t, err)
	require.NotEmpty(t, result.Fills)
	var total uint256.Int
	for _, f := range result.Fills {
		total.Add(&total, &f.SizeBase)
	}
	return total
}
func TestPositionCapWithdrawsGrowingSide(t *testing.T) {
	for _, buy := range []bool{true, false} {
		t.Run(map[bool]string{true: "short", false: "long"}[buy], func(t *testing.T) {
			c := newKeeperChain(t)
			e, _ := testEngine(t, c, nil)
			round(t, e, c)
			takerFill(t, c, buy)
			e.cfg.MaxPositionUSD = usd(t, "10")
			c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(time.Second))
			round(t, e, c)
			remaining := orders(t, c)
			require.NotEmpty(t, remaining)
			snap, err := c.Snapshot(context.Background(), 0)
			require.NoError(t, err)
			exposure := new(big.Int).Abs(new(big.Int).Quo(new(big.Int).Mul(snap.PositionBase, snap.Price.Mark), unit))
			require.Positive(t, exposure.Cmp(e.cfg.MaxPositionUSD))
			for _, o := range remaining {
				require.Equal(t, buy, o.Buy, "only the side reducing the position may remain")
			}
		})
	}
}
func TestStalePriceWithdrawsAllAndFreshPriceResumes(t *testing.T) {
	c := newKeeperChain(t)
	e, _ := testEngine(t, c, nil)
	round(t, e, c)
	c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(11 * time.Second))
	round(t, e, c)
	require.Empty(t, orders(t, c))
	require.Equal(t, 4, c.cancels)
	c.price(t, "60000", c.ctx.BlockTime())
	round(t, e, c)
	require.Len(t, orders(t, c), 4)
}
func TestDryModeSendsNothing(t *testing.T) {
	c := newKeeperChain(t)
	live, _ := testEngine(t, c, nil)
	round(t, live, c)
	before := orders(t, c)
	placed, cancelled := c.places, c.cancels
	dry, logs := testEngine(t, c, func(q *config.QuoterConfig) { q.Mode = "dry" })
	c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(time.Second))
	plans := round(t, dry, c)
	require.Len(t, plans[0], 4)
	require.Equal(t, placed, c.places)
	require.Equal(t, cancelled, c.cancels)
	require.Equal(t, before, orders(t, c))
	require.Contains(t, logs.String(), "quoter keep")
	require.NotContains(t, logs.String(), "quoter cancel")
	require.NotContains(t, logs.String(), "quoter place")
	result, err := dry.Stop(context.Background())
	require.ErrorIs(t, err, ErrOrdersRemain)
	require.Equal(t, 4, result.OrdersLeft)
	require.Equal(t, before, orders(t, c))
}
func TestStopClearsEveryOwnOrderIncludingUnselectedMarket(t *testing.T) {
	c := newKeeperChain(t)
	e, _ := testEngine(t, c, nil)
	round(t, e, c)
	// A separate order with no client id exercises cancelOrder during shutdown.
	_, err := c.k.PlaceOrder(c.ctx, c.account, perpskeeper.OrderParams{MarketID: 0, IsLong: true, OrderType: 0, TimeInForce: 0, SizeUsd: word(usd(t, "25")), Leverage: word(unit), LimitPrice: word(usd(t, "59000"))})
	require.NoError(t, err)
	e.cfg.Markets = []uint64{99}
	result, err := e.Stop(context.Background())
	require.NoError(t, err)
	require.Zero(t, result.OrdersLeft)
	require.Empty(t, orders(t, c))
	require.True(t, e.Health().Stopped)
	_, err = e.Round(context.Background(), c.ctx.BlockTime())
	require.Error(t, err)
}
func TestStopSignalAndVariableLeaveNoOrders(t *testing.T) {
	for _, kind := range []string{"signal", "variable"} {
		t.Run(kind, func(t *testing.T) {
			c := newKeeperChain(t)
			e, _ := testEngine(t, c, nil)
			round(t, e, c)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "signal" {
				cancel()
			}
			require.NoError(t, e.Run(ctx, func() bool { return kind == "variable" }))
			require.Empty(t, orders(t, c))
			require.Zero(t, e.Health().OrdersLeft)
		})
	}
}
func TestRefusalsAreCountedAndMarketRests(t *testing.T) {
	c := newKeeperChain(t)
	e, logs := testEngine(t, c, nil)
	params, err := c.k.GetParams(c.ctx)
	require.NoError(t, err)
	params.Paused = true
	require.NoError(t, c.k.UpdateParams(c.ctx, c.k.Authority(), params))
	var first [16]byte
	for i := 0; i < 3; i++ {
		plans, err := e.Round(context.Background(), c.ctx.BlockTime())
		require.Error(t, err)
		if i == 0 {
			first = plans[0][0].Cloid
		} else {
			require.Equal(t, first, plans[0][0].Cloid)
		}
		c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(time.Second))
	}
	h := e.Health().Markets[0]
	require.Equal(t, uint64(3), h.Refusals)
	require.Equal(t, 3, h.Consecutive)
	require.Equal(t, "PerpsPaused", h.LastRefusal)
	require.True(t, h.RestUntil.After(c.ctx.BlockTime()))
	require.Contains(t, logs.String(), "PerpsPaused")
	params.Paused = false
	require.NoError(t, c.k.UpdateParams(c.ctx, c.k.Authority(), params))
	c.price(t, "60000", c.ctx.BlockTime())
	round(t, e, c)
	require.Equal(t, 3, c.places)
	require.Empty(t, orders(t, c))
	c.ctx = c.ctx.WithBlockTime(h.RestUntil.Add(time.Second))
	c.price(t, "60000", c.ctx.BlockTime())
	round(t, e, c)
	require.Len(t, orders(t, c), 4)
	require.Zero(t, e.Health().Markets[0].Consecutive)
}
func TestNoOrderBeyondFreeCollateral(t *testing.T) {
	c := newKeeperChain(t)
	c.fund(t, c.account, 26)
	e, _ := testEngine(t, c, nil)
	before, err := c.Snapshot(context.Background(), 0)
	require.NoError(t, err)
	round(t, e, c)
	open := orders(t, c)
	require.Len(t, open, 1)
	reserved := new(big.Int)
	for _, o := range open {
		reserved.Add(reserved, reserveUSD(o, before.Price.Mark, before.TakerFeePPM))
	}
	require.LessOrEqual(t, reserved.Cmp(before.FreeUSD), 0)
	require.Zero(t, e.Health().Markets[0].Refusals)
	account, err := c.k.GetAccount(c.ctx, c.account)
	require.NoError(t, err)
	require.Equal(t, uint64(26_000_000), account.Collateral.Uint64())
	dry, _ := testEngine(t, c, func(q *config.QuoterConfig) { q.Mode = "dry" })
	round(t, dry, c)
	require.Equal(t, 1, c.places)
}
func TestInventoryIncludesAllRestingExposure(t *testing.T) {
	c := newKeeperChain(t)
	e, _ := testEngine(t, c, func(q *config.QuoterConfig) { q.MaxPositionUSD = usd(t, "30") })
	round(t, e, c)
	snap, err := c.Snapshot(context.Background(), 0)
	require.NoError(t, err)
	require.Len(t, snap.Orders, 2)
	for _, buy := range []bool{true, false} {
		base := new(big.Int)
		for _, o := range snap.Orders {
			if o.Buy == buy {
				base.Add(base, o.Base)
			}
		}
		value := ceilDiv(new(big.Int).Mul(base, snap.Price.Mark), unit)
		require.LessOrEqual(t, value.Cmp(e.cfg.MaxPositionUSD), 0)
	}
}
func TestHealthAndErrorsRedactCredentials(t *testing.T) {
	c := newKeeperChain(t)
	e, _ := testEngine(t, c, nil)
	round(t, e, c)
	require.Equal(t, "chain request failed", reason(errors.New("https://user:secret@rpc.example/key")))
	require.Equal(t, "chain request failed", reason(&ChainError{Reason: "secret"}))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	e.Handler().ServeHTTP(response, req)
	require.Equal(t, http.StatusOK, response.Code)
	var health Health
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &health))
	require.Equal(t, "live", health.Mode)
	require.False(t, health.LastRound.IsZero())
	require.Len(t, health.Markets[0].Orders, 4)
	require.Equal(t, "0", health.Markets[0].Position)
	for _, s := range []string{"http", "secret", "KEY", "account-key"} {
		require.False(t, strings.Contains(response.Body.String(), s))
	}
	// Health copies cannot mutate the engine's observable book.
	copy := e.Health()
	copy.Markets[0].Orders = slices.Clone(copy.Markets[0].Orders)
	copy.Markets[0].Orders[0].Price = "changed"
	require.NotEqual(t, "changed", e.Health().Markets[0].Orders[0].Price)
}

func TestReadWaveRoundSnapshotsFollowMarketTransactions(t *testing.T) {
	for _, mode := range []string{"live", "dry", "refused"} {
		t.Run(mode, func(t *testing.T) {
			c := newKeeperChain(t)
			market, err := c.k.GetMarket(c.ctx, 0)
			require.NoError(t, err)
			market.ID, market.Symbol, market.Name, market.OracleSymbol = 1, "ETH", "Ethereum", "ETH"
			require.NoError(t, c.k.SetMarket(c.ctx, c.k.Authority(), market))
			c.ctx.KVStore(c.pfKey).Set(pricetypes.AggregateKey(1), c.ctx.KVStore(c.pfKey).Get(pricetypes.AggregateKey(0)))
			_, err = c.k.UpdatePrices(c.ctx)
			require.NoError(t, err)
			state, err := c.k.GetMarketState(c.ctx, 1)
			require.NoError(t, err)
			state.MarkPrice = word(usd(t, "60000"))
			c.k.SetMarketState(c.ctx, state)
			e, logs := testEngine(t, c, func(q *config.QuoterConfig) {
				q.Markets = []uint64{0, 1}
				q.MaxTransactionsPerRound = 1
				if mode == "dry" {
					q.Mode = "dry"
				}
			})
			if mode == "refused" {
				params, err := c.k.GetParams(c.ctx)
				require.NoError(t, err)
				params.Paused = true
				require.NoError(t, c.k.UpdateParams(c.ctx, c.k.Authority(), params))
			}
			_, err = e.Round(context.Background(), c.ctx.BlockTime())
			if mode == "refused" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if mode == "dry" {
				require.Equal(t, map[uint64]int{0: 1, 1: 1}, c.snapshots)
				require.Zero(t, e.Health().TransactionsLastRound)
			} else {
				if mode == "refused" {
					require.Equal(t, map[uint64]int{0: 3, 1: 1}, c.snapshots)
				} else {
					require.Equal(t, map[uint64]int{0: 3}, c.snapshots)
				}
				require.Equal(t, 1, e.Health().TransactionsLastRound)
			}
			if mode == "live" {
				require.Len(t, e.Health().Markets, 1)
			} else {
				require.Len(t, e.Health().Markets, 2)
				require.Empty(t, e.Health().Markets[1].Orders)
			}
			actual := orders(t, c)
			require.Len(t, e.Health().Markets[0].Orders, len(actual))
			var last map[string]any
			for _, line := range bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte("\n")) {
				var entry map[string]any
				require.NoError(t, json.Unmarshal(line, &entry))
				if entry["msg"] == "quoter round" {
					last = entry
				}
			}
			require.NotNil(t, last)
			seconds, ok := last["seconds"].(float64)
			require.True(t, ok)
			require.GreaterOrEqual(t, seconds, float64(0))
		})
	}
	t.Run("unchanged book", func(t *testing.T) {
		c := newKeeperChain(t)
		e, _ := testEngine(t, c, nil)
		round(t, e, c)
		require.Equal(t, map[uint64]int{0: 6}, c.snapshots)
		c.snapshots = nil
		round(t, e, c)
		require.Equal(t, map[uint64]int{0: 1}, c.snapshots)
		require.Zero(t, e.Health().TransactionsLastRound)
		require.Len(t, e.Health().Markets[0].Orders, 4)
		c.snapshots = nil
		c.ctx = c.ctx.WithBlockTime(c.ctx.BlockTime().Add(11 * time.Second))
		round(t, e, c)
		require.Equal(t, map[uint64]int{0: 2}, c.snapshots)
		require.Equal(t, 4, e.Health().TransactionsLastRound)
		require.Empty(t, e.Health().Markets[0].Orders)
	})
}
