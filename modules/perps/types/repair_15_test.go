package types_test

import (
	"encoding/json"
	"math/big"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// repair15Transport crosses the production ABCI encoder, protobuf wire and
// client decoder, retaining the producer's context and LXP reason as well.
func repair15Transport(t *testing.T, produced error, root *sdkerrors.Error, code uint32, name, detail string) {
	t.Helper()
	require.ErrorIs(t, produced, root)
	require.Contains(t, produced.Error(), detail)
	response := sdkerrors.ResponseDeliverTx(produced, 100, 75, false)
	wire, err := response.Marshal()
	require.NoError(t, err)
	var received abci.ResponseDeliverTx
	require.NoError(t, received.Unmarshal(wire))
	require.Equal(t, "perps", received.Codespace)
	require.Equal(t, code, received.Code)
	require.Contains(t, received.Log, name)
	require.Contains(t, received.Log, detail)
	require.Equal(t, int64(100), received.GasWanted)
	require.Equal(t, int64(75), received.GasUsed)
	decoded := sdkerrors.ABCIError(received.Codespace, received.Code, received.Log)
	require.ErrorIs(t, decoded, root)
	require.Contains(t, decoded.Error(), detail)
	proofPerpsTransport(t, produced, root)
}

func TestRepair_15_ABCITypeGenesis(t *testing.T) {
	cases := []struct {
		name, detail string
		mutate       func(*types.GenesisState)
	}{
		{"invalidGenesis", "next order id 0", func(gs *types.GenesisState) { gs.Counters.NextOrderID.Clear() }},
		{"params", "zero collateral token", func(gs *types.GenesisState) { gs.Params.CollateralToken = common.Address{} }},
		{"market", "market 0 max leverage", func(gs *types.GenesisState) { m := btcMarket(); m.MaxLeverage = 0; gs.Markets = []types.Market{m} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := types.DefaultGenesis()
			tc.mutate(gs)
			repair15Transport(t, types.ValidateGenesis(*gs), types.ErrInvalidGenesis, 39, "LXP_PERPS_INVALID_GENESIS", tc.detail)
		})
	}
}

func TestRepair_15_ABCIParams(t *testing.T) {
	p := types.DefaultParams()
	p.CollateralToken = common.Address{}
	repair15Transport(t, p.Validate(), types.ErrInvalidParams, 2, "LXP_PERPS_INVALID_PARAMS", "zero collateral token")
	m := btcMarket()
	m.MaxLeverage = 0
	repair15Transport(t, m.Validate(), types.ErrInvalidParams, 2, "LXP_PERPS_INVALID_PARAMS", "market 0 max leverage")
}

func TestRepair_15_ABCIMath(t *testing.T) {
	maxUnsigned := sdk.NewIntFromBigInt(new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)))
	maxSigned := sdk.NewIntFromBigInt(new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(1)))
	minSigned := sdk.NewIntFromBigInt(new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 255)))
	// NewIntFromBigInt retains its input pointer. Expanding it after construction
	// reaches the public conversion/operand guards without unsafe or a fake Int.
	backing := big.NewInt(1)
	oversized := sdk.NewIntFromBigInt(backing)
	backing.Lsh(backing, 256)
	for _, convert := range []struct {
		name string
		call func(sdk.Int) (sdk.Int, sdk.Int, error)
	}{
		{"UsdToUsdlDown", types.UsdToUsdlDown}, {"UsdToUsdlUp", types.UsdToUsdlUp},
	} {
		t.Run(convert.name, func(t *testing.T) {
			_, _, err := convert.call(oversized)
			repair15Transport(t, err, types.ErrOverflow, 38, "LXP_PERPS_OVERFLOW", "USD amount")
		})
	}
	for _, operation := range []struct {
		name   string
		call   func(sdk.Int, sdk.Int, sdk.Int) (sdk.Int, error)
		signed bool
	}{
		{"MulDivDown", types.MulDivDown, false}, {"MulDivUp", types.MulDivUp, false},
		{"MulDivDownSigned", types.MulDivDownSigned, true}, {"MulDivUpSigned", types.MulDivUpSigned, true},
	} {
		t.Run(operation.name, func(t *testing.T) {
			limit := maxUnsigned
			if operation.signed {
				limit = maxSigned
			}
			_, err := operation.call(limit, sdk.NewInt(2), sdk.OneInt())
			repair15Transport(t, err, types.ErrOverflow, 38, "LXP_PERPS_OVERFLOW", "result")
			for operand := 0; operand < 3; operand++ {
				values := []sdk.Int{sdk.OneInt(), sdk.OneInt(), sdk.OneInt()}
				values[operand] = oversized
				if operation.signed {
					values[operand] = maxUnsigned
				}
				_, err := operation.call(values[0], values[1], values[2])
				repair15Transport(t, err, types.ErrOverflow, 38, "LXP_PERPS_OVERFLOW", "operand")
			}
			if operation.signed {
				_, err := operation.call(minSigned, sdk.NewInt(2), sdk.OneInt())
				repair15Transport(t, err, types.ErrOverflow, 38, "LXP_PERPS_OVERFLOW", "result")
				belowMin := sdk.NewIntFromBigInt(new(big.Int).Sub(minSigned.BigInt(), big.NewInt(1)))
				_, err = operation.call(belowMin, sdk.OneInt(), sdk.OneInt())
				repair15Transport(t, err, types.ErrOverflow, 38, "LXP_PERPS_OVERFLOW", "operand")
			}
		})
	}
	// Exercise the one-unit overflow created specifically by ceiling rounding.
	for _, op := range []struct {
		call  func(sdk.Int, sdk.Int, sdk.Int) (sdk.Int, error)
		limit sdk.Int
	}{
		{types.MulDivUp, maxUnsigned}, {types.MulDivUpSigned, maxSigned},
	} {
		a := op.limit.SubRaw(1)
		b := op.limit.SubRaw(1)
		denominator := op.limit.SubRaw(2)
		_, err := op.call(a, b, denominator)
		repair15Transport(t, err, types.ErrOverflow, 38, "LXP_PERPS_OVERFLOW", "result")
	}
}

func repair15FilledExport(t *testing.T) (*proofPerpsStore, *types.GenesisState) {
	t.Helper()
	e := newProofPerpsStore(t)
	feeder := e.registerFeeder(t)
	e.producePrice(t, feeder, 1, 1000)
	require.NoError(t, keeper.SeedV6120(e.ctx, *e.perps))
	_, err := e.perps.UpdatePrices(e.ctx)
	require.NoError(t, err)
	for _, address := range []common.Address{holderA, holderB} {
		_, err := e.perps.Deposit(e.ctx, address, word(1_000_000_000))
		require.NoError(t, err)
	}
	maker, err := e.perps.PlaceOrder(e.ctx, holderB, keeper.OrderParams{MarketID: 0, IsLong: false,
		OrderType: uint8(types.OrderTypeLimit), TimeInForce: uint8(types.TimeInForceGTC),
		LimitPrice: e18(1000), SizeUsd: e18(2000), Leverage: e18(10)})
	require.NoError(t, err)
	taker, err := e.perps.PlaceOrder(e.ctx, holderA, keeper.OrderParams{MarketID: 0, IsLong: true,
		OrderType: uint8(types.OrderTypeMarket), TimeInForce: uint8(types.TimeInForceIOC),
		SizeUsd: e18(1000), Leverage: e18(10)})
	require.NoError(t, err)
	filled, err := e.perps.GetOrder(e.ctx, taker.OrderID)
	require.NoError(t, err)
	require.Equal(t, types.OrderStatusFilled, filled.Status)
	resting, err := e.perps.GetOrder(e.ctx, maker.OrderID)
	require.NoError(t, err)
	require.Equal(t, types.OrderStatusResting, resting.Status)
	require.Equal(t, e18(1), resting.FilledBase)
	_, err = e.perps.PlaceOrder(e.ctx, holderA, keeper.OrderParams{MarketID: 0, IsLong: true,
		OrderType: uint8(types.OrderTypeStopMarket), TimeInForce: uint8(types.TimeInForceIOC),
		TriggerPrice: e18(1100), SizeUsd: e18(100), Leverage: e18(10)})
	require.NoError(t, err)
	gs := e.perps.ExportGenesis(e.ctx)
	require.Len(t, gs.Positions, 2)
	require.Len(t, gs.AccountOrders, 2)
	require.Len(t, gs.AccountMarketPositions, 2)
	require.NoError(t, keeper.ValidateGenesis(*gs))
	return e, gs
}

func repair15Clone(t *testing.T, gs *types.GenesisState) types.GenesisState {
	t.Helper()
	wire, err := json.Marshal(gs)
	require.NoError(t, err)
	var received types.GenesisState
	require.NoError(t, json.Unmarshal(wire, &received))
	return received
}

func TestRepair_15_GenesisOwnershipAndCounts(t *testing.T) {
	source, exported := repair15FilledExport(t)
	target := newProofPerpsStore(t)
	cases := []struct {
		name, detail string
		mutate       func(*types.GenesisState)
	}{
		{"missing_all_orders", "without an account order entry", func(gs *types.GenesisState) { gs.AccountOrders = nil }},
		{"missing_resting_order", "without an account order entry", func(gs *types.GenesisState) {
			for i, entry := range gs.AccountOrders {
				for _, order := range gs.Orders {
					if entry.OrderID == order.ID && order.Status == types.OrderStatusResting {
						gs.AccountOrders = append(gs.AccountOrders[:i], gs.AccountOrders[i+1:]...)
						return
					}
				}
			}
		}},
		{"missing_untriggered_order", "without an account order entry", func(gs *types.GenesisState) {
			for i, entry := range gs.AccountOrders {
				for _, order := range gs.Orders {
					if entry.OrderID == order.ID && order.Status == types.OrderStatusUntriggered {
						gs.AccountOrders = append(gs.AccountOrders[:i], gs.AccountOrders[i+1:]...)
						return
					}
				}
			}
		}},
		{"missing_all_positions", "without an account market entry", func(gs *types.GenesisState) { gs.AccountMarketPositions = nil }},
		{"missing_one_position", "without an account market entry", func(gs *types.GenesisState) { gs.AccountMarketPositions = gs.AccountMarketPositions[:1] }},
		{"open_order_count_low", "open order count", func(gs *types.GenesisState) {
			for i := range gs.Accounts {
				if gs.Accounts[i].Address == holderA {
					gs.Accounts[i].OpenOrderCount = 0
				}
			}
		}},
		{"open_order_count_high", "open order count", func(gs *types.GenesisState) { gs.Accounts[0].OpenOrderCount++ }},
		{"position_count_low", "position count", func(gs *types.GenesisState) {
			for i := range gs.Accounts {
				if gs.Accounts[i].Address == holderA {
					gs.Accounts[i].PositionCount = 0
				}
			}
		}},
		{"position_count_high", "position count", func(gs *types.GenesisState) { gs.Accounts[0].PositionCount++ }},
		{"long_oi", "open interest disagrees", func(gs *types.GenesisState) {
			gs.MarketStates[0].LongOpenInterest.AddUint64(&gs.MarketStates[0].LongOpenInterest, 1)
		}},
		{"short_oi", "open interest disagrees", func(gs *types.GenesisState) { gs.MarketStates[0].ShortOpenInterest.Clear() }},
		{"oi_without_positions", "open interest disagrees", func(gs *types.GenesisState) { gs.MarketStates[1].LongOpenInterest.SetUint64(1) }},
		{"missing_market_state", "positions without a market state", func(gs *types.GenesisState) { gs.MarketStates = gs.MarketStates[1:] }},
		{"oi_sum_overflow", "overflows 256 bits", func(gs *types.GenesisState) {
			gs.Positions[0].IsLong, gs.Positions[1].IsLong = true, true
			gs.Positions[0].SizeBase.SetAllOne()
			gs.Positions[1].SizeBase.SetUint64(1)
		}},
		{"unbalanced_sides", "long and short open interest differ", func(gs *types.GenesisState) {
			gs.Positions[0].IsLong, gs.Positions[1].IsLong = true, true
			gs.MarketStates[0].LongOpenInterest = e18(2)
			gs.MarketStates[0].ShortOpenInterest.Clear()
		}},
	}
	for _, status := range []types.OrderStatus{types.OrderStatusFilled, types.OrderStatusCancelled, types.OrderStatusExpired} {
		cases = append(cases, struct {
			name, detail string
			mutate       func(*types.GenesisState)
		}{
			name: "indexed_closed_order_" + big.NewInt(int64(status)).String(), detail: "account order entry of closed order",
			mutate: func(gs *types.GenesisState) {
				for i := range gs.Orders {
					if gs.Orders[i].Status == types.OrderStatusFilled {
						gs.Orders[i].Status = status
						gs.AccountOrders = append(gs.AccountOrders, types.AccountOrderEntry{Account: gs.Orders[i].Account, OrderID: gs.Orders[i].ID})
						return
					}
				}
			},
		})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			corrupted := repair15Clone(t, exported)
			tc.mutate(&corrupted)
			transported := repair15Clone(t, &corrupted)
			err := keeper.ValidateGenesis(transported)
			repair15Transport(t, err, types.ErrInvalidGenesis, 39, "LXP_PERPS_INVALID_GENESIS", tc.detail)
			require.PanicsWithError(t, err.Error(), func() { target.perps.InitGenesis(target.ctx, transported) })
			iter := target.ctx.KVStore(target.keys[types.StoreKey]).Iterator(nil, nil)
			require.False(t, iter.Valid(), "invalid recovery wrote state")
			require.NoError(t, iter.Close())
		})
	}
	require.Equal(t, exported, source.perps.ExportGenesis(source.ctx))
}

func TestRepair_15_GenesisRecoveryMarginWithdrawal(t *testing.T) {
	source, exported := repair15FilledExport(t)
	before, err := source.perps.AccountMargin(source.ctx, holderA)
	require.NoError(t, err)
	require.Equal(t, uint64(1), before.Positions)
	require.True(t, before.InitialMargin.Equal(sdk.NewIntWithDecimal(100, 18)))
	allowed, err := source.perps.Withdrawable(source.ctx, holderA)
	require.NoError(t, err)
	require.True(t, allowed.Lt(uint256.NewInt(1_000_000_000)))
	source.restart(t)
	afterRestart, err := source.perps.AccountMargin(source.ctx, holderA)
	require.NoError(t, err)
	require.Equal(t, before, afterRestart)
	target := newProofPerpsStore(t)
	feeder := target.registerFeeder(t)
	target.producePrice(t, feeder, 1, 1000)
	received := repair15Clone(t, exported)
	require.NoError(t, keeper.ValidateGenesis(received))
	target.perps.InitGenesis(target.ctx, received)
	target.restart(t)
	restored, err := target.perps.AccountMargin(target.ctx, holderA)
	require.NoError(t, err)
	require.Equal(t, before, restored)
	restoredAllowed, err := target.perps.Withdrawable(target.ctx, holderA)
	require.NoError(t, err)
	require.Equal(t, allowed, restoredAllowed)
	account, err := target.perps.GetAccount(target.ctx, holderA)
	require.NoError(t, err)
	var tooMuch uint256.Int
	tooMuch.AddUint64(&allowed, 1)
	_, err = target.perps.Withdraw(target.ctx, holderA, tooMuch)
	require.ErrorIs(t, err, types.ErrInsufficientMargin)
	unchanged, err := target.perps.GetAccount(target.ctx, holderA)
	require.NoError(t, err)
	require.Equal(t, account.Collateral, unchanged.Collateral)
	_, err = target.perps.Withdraw(target.ctx, holderA, allowed)
	require.NoError(t, err)
	remaining, err := target.perps.Withdrawable(target.ctx, holderA)
	require.NoError(t, err)
	require.True(t, remaining.IsZero())
	margin, err := target.perps.AccountMargin(target.ctx, holderA)
	require.NoError(t, err)
	require.Equal(t, uint64(1), margin.Positions)
	require.True(t, margin.Equity.Equal(sdk.NewIntWithDecimal(100, 18)))
	require.NoError(t, keeper.ValidateGenesis(*target.perps.ExportGenesis(target.ctx)))
}
