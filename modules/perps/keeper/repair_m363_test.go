package keeper_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestRepairM_363_ADLBlockAllowanceAndContinuation(t *testing.T) {
	TestEndblockADLSharedAllowanceAndContinuation(t)
}

func TestRepairM_363_LeverageTierPolicy(t *testing.T) {
	TestMsgSetMarketTierPolicyExecution(t)
	TestGovernedMaxLeverageRefusesHigherLeverage(t)
}

func TestRepairM_363_PauseAuthorityBeforeState(t *testing.T) {
	e := openPerpsGovernanceProof(t, t.TempDir())
	wrong := e.k.ModuleAddress().String()
	// Without an SDK context, reaching the unwrap itself would panic.
	response, err := keeper.NewMsgServerImpl(e.k).SetPaused(context.Background(), &types.MsgSetPaused{Authority: wrong, Paused: true})
	require.Nil(t, response)
	requireRefused(t, err, types.ErrUnauthorized, 35)
	for _, corrupt := range []bool{false, true} {
		e.ctx.KVStore(e.key).Set(types.ParamsKey, []byte("{invalid stored params"))
		if !corrupt {
			e.ctx.KVStore(e.key).Delete(types.ParamsKey)
		}
		e.ctx = e.ctx.WithEventManager(sdk.NewEventManager())
		before := storeContents(t, e.ctx.KVStore(e.key))
		result, err := e.deliver(t, &types.MsgSetPaused{Authority: wrong, Paused: true})
		require.Nil(t, result)
		requireRefused(t, err, types.ErrUnauthorized, 35)
		require.Equal(t, before, storeContents(t, e.ctx.KVStore(e.key)))
		require.Empty(t, e.ctx.EventManager().Events())
		if corrupt {
			result, err = e.deliver(t, &types.MsgSetPaused{Authority: e.k.Authority(), Paused: true})
			require.Nil(t, result)
			requireRefused(t, err, types.ErrInvalidParams, types.ErrInvalidParams.ABCICode())
			require.Equal(t, before, storeContents(t, e.ctx.KVStore(e.key)))
			require.Empty(t, e.ctx.EventManager().Events())
		}
	}
}

func TestRepairM_363_PauseGovernanceProof(t *testing.T) {
	TestProofPerpsGovernanceMalformedAndAuthorization(t)
	TestProofPerpsGovernanceProducerTransportConsumer(t)
}

func TestRepairM_363_ProducedOrderRefusalWireIdentity(t *testing.T) {
	env := newOrdersEnv(t)
	env.fund(t, traderA, 100)
	order := limitBuy(100, 59_000)
	order.MarketID = 999
	before := storeContents(t, env.ctx.KVStore(env.perpsKey))
	_, err := env.k.PlaceOrder(env.ctx, traderA, order)
	requireRefused(t, err, types.ErrUnknownMarket, 3)
	require.Equal(t, before, storeContents(t, env.ctx.KVStore(env.perpsKey)))

	order.MarketID = 2
	env.setAggregate(t, 2, 150, 150, markNowMs-16_000)
	_, err = env.k.PlaceOrder(env.ctx, traderA, order)
	requireRefused(t, err, types.ErrPriceStale, 5)
	require.Equal(t, before, storeContents(t, env.ctx.KVStore(env.perpsKey)))

	order = limitBuy(10_000, 59_000)
	_, err = env.k.PlaceOrder(env.ctx, traderA, order)
	requireRefused(t, err, types.ErrInsufficientMargin, 9)
	require.Equal(t, before, storeContents(t, env.ctx.KVStore(env.perpsKey)))
}

func TestRepairM_363_OpenQuoteExecution(t *testing.T) {
	for _, scenario := range []string{"unaligned_mark", "own_best_bid", "partial_fill", "fee_tier", "multiple_fills"} {
		t.Run(scenario, func(t *testing.T) {
			e := newRepair14Env(t, 5_000_000_000)
			repair14Fresh(e)
			isLong := scenario != "own_best_bid"
			var ownID uint256.Int
			if !isLong {
				e.cancelAsk()
				e.mustCall(repair14TraderA, "deposit", e.usdl, big.NewInt(4_000_000_000))
				ownID = e.rest(e.ctx(), repair14TraderA, true, 60_100)
				e.rest(e.ctx(), repair14BidderC, true, 60_000)
			}
			if scenario == "unaligned_mark" {
				e.setMark(new(big.Int).Add(repair14E18(60_000), big.NewInt(1)))
			}
			if scenario == "partial_fill" || scenario == "multiple_fills" {
				e.cancelAsk()
				base := repair14Word(t, "5000000000000000")
				if scenario == "multiple_fills" {
					base = repair14Word(t, "10000000000000000")
				}
				notional := new(big.Int).Mul(base.ToBig(), repair14E18(60_000))
				notional.Quo(notional, repair14E18(1))
				_, err := e.k.PlaceOrder(e.ctx(), repair14MakerB, keeper.OrderParams{MarketID: 0, IsLong: false,
					OrderType: uint8(types.OrderTypeLimit), TimeInForce: uint8(types.TimeInForceGTC),
					SizeUsd: *uint256.MustFromBig(notional), LimitPrice: *uint256.MustFromBig(repair14E18(60_000)),
					Leverage: *uint256.MustFromBig(repair14E18(20))})
				require.NoError(t, err)
				if scenario == "multiple_fills" {
					e.rest(e.ctx(), repair14BidderC, false, 60_100)
					e.mustCall(repair14TraderA, "deposit", e.usdl, big.NewInt(1_000_000))
				}
			}
			state, err := e.k.GetMarketState(e.ctx(), 0)
			require.NoError(t, err)
			if scenario != "unaligned_mark" {
				state.MarkPrice = *uint256.MustFromBig(repair14E18(60_000))
			}
			state.IndexPrice = *uint256.MustFromBig(repair14E18(60_000))
			state.HourlyRate = *uint256.NewInt(12_500_000_000_000)
			e.k.SetMarketState(e.ctx(), state)
			params, err := e.k.GetParams(e.ctx())
			require.NoError(t, err)
			if scenario == "fee_tier" {
				volume := params.FeeTiers[1].MinVolumeUsd.Add(sdk.NewIntWithDecimal(1, 18))
				require.NoError(t, e.k.RecordVolume(e.ctx(), repair14TraderA, repair14KeeperK, *uint256.MustFromBig(volume.BigInt())))
			}
			tier, err := e.k.FeeTier(e.ctx(), repair14TraderA)
			require.NoError(t, err)
			if scenario == "fee_tier" {
				require.Equal(t, uint8(1), tier.Tier)
				require.Equal(t, params.FeeTiers[1].TakerFeePpm, tier.TakerFeePpm)
			} else {
				require.Equal(t, uint8(0), tier.Tier)
				require.Equal(t, uint64(450), tier.TakerFeePpm)
			}

			before := storeContents(t, e.ctx().KVStore(e.perpsKey))
			wallet, module := e.usdlBalance(repair14TraderA), e.usdlBalance(e.k.ModuleEVMAddress())
			logsBefore := len(e.logs())
			collateralBefore := new(big.Int)
			if account, err := e.k.GetAccount(e.ctx(), repair14TraderA); err == nil {
				collateralBefore.Set(account.Collateral.ToBig())
			} else {
				require.ErrorIs(t, err, types.ErrNoAccount)
			}
			produced, err := e.k.QuoteOpen(e.ctx(), repair14TraderA, 0, e.usdl, *uint256.NewInt(100_000_000),
				*uint256.MustFromBig(repair14E18(10)), isLong)
			require.NoError(t, err)
			quote := e.quoteOpen(isLong)
			for _, field := range []struct {
				name      string
				want, got *big.Int
			}{
				{"entry", produced.EntryPrice, quote.EntryPrice},
				{"size", produced.SizeUsd, quote.SizeUsd},
				{"collateral", produced.CollateralUsd, quote.CollateralUsd},
				{"leverage", produced.Leverage, quote.Leverage},
				{"fee tokens", produced.TradingFee, quote.TradingFee},
				{"fee USD", produced.TradingFeeUsd, quote.TradingFeeUsd},
				{"impact", produced.PriceImpact, quote.PriceImpact},
				{"liquidation", produced.LiquidationPrice, quote.LiquidationPrice},
				{"funding", produced.EstimatedFunding24h, quote.EstimatedFunding24h},
				{"maintenance", produced.MaintenanceMarginBps, quote.MaintenanceMarginBps},
			} {
				require.NotNil(t, field.want, field.name)
				require.NotNil(t, field.got, field.name)
				require.Equal(t, field.want.String(), field.got.String(), field.name)
			}
			require.Equal(t, before, storeContents(t, e.ctx().KVStore(e.perpsKey)))
			require.Equal(t, wallet, e.usdlBalance(repair14TraderA))
			require.Equal(t, module, e.usdlBalance(e.k.ModuleEVMAddress()))
			require.Len(t, e.logs(), logsBefore)
			ret := e.mustCall(repair14TraderA, "openPosition", big.NewInt(0), e.usdl, big.NewInt(100_000_000), repair14E18(10), isLong)
			out, err := e.perpsABI.Methods["openPosition"].Outputs.Unpack(ret)
			require.NoError(t, err)
			position, ok := e.position(repair14TraderA)
			require.True(t, ok)
			require.Equal(t, out[0], position.ID.ToBig())
			require.Equal(t, isLong, position.IsLong)
			require.Equal(t, quote.EntryPrice, position.EntryPrice.ToBig())
			require.Equal(t, quote.Leverage, position.Leverage.ToBig())
			require.Equal(t, repair14E18(100), quote.CollateralUsd)
			require.Equal(t, new(big.Int).Sub(wallet, big.NewInt(100_000_000)), e.usdlBalance(repair14TraderA))
			require.Equal(t, new(big.Int).Add(module, big.NewInt(100_000_000)), e.usdlBalance(e.k.ModuleEVMAddress()))

			_, fills := e.perpsLogs("Fill")
			notional, fee, base := new(big.Int), new(big.Int), new(big.Int)
			fillCount := 0
			for _, fill := range fills {
				if fill["taker"] != repair14TraderA {
					continue
				}
				require.NotEqual(t, repair14TraderA, fill["maker"])
				size, price := fill["sizeUsd"].(*big.Int), fill["price"].(*big.Int)
				notional.Add(notional, size)
				fee.Add(fee, fill["takerFeeUsd"].(*big.Int))
				base.Add(base, new(big.Int).Quo(new(big.Int).Mul(size, repair14E18(1)), price))
				expectedFee := new(big.Int).Quo(new(big.Int).Mul(size, new(big.Int).SetUint64(tier.TakerFeePpm)), big.NewInt(1_000_000))
				require.Equal(t, expectedFee, fill["takerFeeUsd"])
				fillCount++
			}
			wantFills := 1
			if scenario == "multiple_fills" {
				wantFills = 2
			}
			require.Equal(t, wantFills, fillCount)
			require.Equal(t, quote.SizeUsd, notional)
			require.Equal(t, quote.TradingFeeUsd, fee)
			require.Equal(t, position.SizeBase.ToBig(), base)
			if scenario == "partial_fill" {
				require.Equal(t, repair14Word(t, "5000000000000000"), position.SizeBase)
				require.Equal(t, repair14E18(300), quote.SizeUsd)
			}
			if scenario == "unaligned_mark" {
				require.Equal(t, repair14Word(t, "16500000000000000"), position.SizeBase)
			}
			if !isLong {
				own, err := e.k.GetOrder(e.ctx(), ownID)
				require.NoError(t, err)
				require.Equal(t, types.OrderStatusCancelled, own.Status)
				require.True(t, own.FilledBase.IsZero())
			}
			feeTokens := new(big.Int)
			feeLogs, collected := e.perpsLogs("FeeCollected")
			for i, event := range collected {
				if feeLogs[i].Topics[2] == common.BytesToHash(repair14TraderA.Bytes()) {
					feeTokens.Add(feeTokens, event["feeTokens"].(*big.Int))
				}
			}
			require.Equal(t, quote.TradingFee, feeTokens)
			expectedCollateral := new(big.Int).Add(collateralBefore, big.NewInt(100_000_000))
			expectedCollateral.Sub(expectedCollateral, feeTokens)
			if scenario == "multiple_fills" {
				expectedCollateral.Add(expectedCollateral, big.NewInt(1_000_000))
			}
			collateralAfter := e.collateral(repair14TraderA)
			require.Equal(t, expectedCollateral, collateralAfter.ToBig())
			liquidation, err := e.k.LiquidationPrice(e.ctx(), position.ID)
			require.NoError(t, err)
			require.Equal(t, quote.LiquidationPrice, liquidation.ToBig())
			state, err = e.k.GetMarketState(e.ctx(), 0)
			require.NoError(t, err)
			impact := new(big.Int).Abs(new(big.Int).Sub(quote.EntryPrice, state.MarkPrice.ToBig()))
			impact.Mul(impact, big.NewInt(10_000)).Quo(impact, state.MarkPrice.ToBig())
			require.Equal(t, impact.String(), quote.PriceImpact.String())
			require.Equal(t, big.NewInt(5), quote.MaintenanceMarginBps)
			funding := new(big.Int).Mul(position.SizeBase.ToBig(), state.IndexPrice.ToBig())
			funding.Mul(funding, state.HourlyRate.ToBig()).Mul(funding, big.NewInt(24))
			funding.Quo(funding, new(big.Int).Mul(repair14E18(1), repair14E18(1)))
			if isLong {
				funding.Neg(funding)
			}
			require.Equal(t, funding.String(), quote.EstimatedFunding24h.String())
		})
	}
}
