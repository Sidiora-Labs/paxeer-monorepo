package perps_test

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	perpskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps"
	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestGovernedFundingDetailsView(t *testing.T) {
	for _, tc := range []struct {
		name    string
		bid     string
		ask     string
		premium string
		rate    string
	}{
		{"positive premium", "60060", "60120", "0.001", "0.00001"},
		{"negative premium", "59880", "59940", "-0.001", "-0.00001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := testkeeper.EVMTestApp
			base, _ := app.NewContext(false, tmtypes.Header{}).WithBlockHeight(11).
				WithBlockTime(time.Unix(viewsNow, 0)).WithEventManager(sdk.NewEventManager()).CacheContext()
			key := sdk.NewKVStoreKey(perpstypes.StoreKey)
			feedKey := sdk.NewKVStoreKey(pricefeedtypes.StoreKey)
			ctx := viewsWithStore(viewsWithStore(base, key), feedKey)
			feed := pricefeedkeeper.NewKeeper(feedKey, app.StakingKeeper, app.AccountKeeper, &app.EvmKeeper)
			require.NoError(t, feed.SetParams(ctx, pricefeedtypes.DefaultParams(pricefeedtypes.DefaultAuthority())))
			k := perpskeeper.NewKeeper(key, &app.EvmKeeper, feed)
			gs := perpstypes.DefaultGenesis()
			gs.Markets = []perpstypes.Market{{
				ID: 0, Symbol: "BTC", Name: "Bitcoin", OracleSymbol: "BTC", SzDecimals: 5, PxDecimals: 1,
				MaxLeverage: 1000, TierClass: perpstypes.TierClassA,
				ImpactNotionalUsd: sdk.NewIntWithDecimal(20_000, 18), OiCapUsd: sdk.NewIntWithDecimal(500_000_000, 18),
				FundingCapPpmPerHour: 40_000, Enabled: true,
			}}
			viewsState(t, gs, perpstypes.MarketState{MarketID: 0, Fresh: true}, "60000")
			k.InitGenesis(ctx, *gs)
			market, err := k.GetMarket(ctx, 0)
			require.NoError(t, err)
			params, err := k.GetParams(ctx)
			require.NoError(t, err)
			require.Greater(t, params.FundingCapPpmPerHour, uint64(10))
			require.Greater(t, market.FundingCapPpmPerHour, uint64(10))
			params.FundingCapPpmPerHour = 10
			response, err := perpskeeper.NewMsgServerImpl(k).UpdateParams(sdk.WrapSDKContext(ctx), &perpstypes.MsgUpdateParams{
				Authority: k.Authority(), Params: params,
			})
			require.NoError(t, err)
			require.NotNil(t, response)
			stored, err := k.GetParams(ctx)
			require.NoError(t, err)
			require.Equal(t, params, stored)

			for i, price := range []string{tc.bid, tc.ask} {
				require.NoError(t, k.BookInsert(ctx, perpstypes.Order{
					ID: viewsID(uint64(i + 1)), Account: viewsA, MarketID: 0, IsLong: i == 0,
					Type: perpstypes.OrderTypeLimit, TimeInForce: perpstypes.TimeInForceGTC,
					Status: perpstypes.OrderStatusResting, LimitPrice: viewsWord(t, price),
					SizeBase: viewsWord(t, "1"), Leverage: viewsWord(t, "1"), Sequence: uint64(i + 1),
				}))
			}
			setPrice := func() {
				bz, err := json.Marshal(pricefeedtypes.Aggregate{
					IndexPrice: sdk.NewIntWithDecimal(60_000, 18), MarkPrice: sdk.NewIntWithDecimal(60_000, 18),
					Basis: sdk.ZeroInt(), TimestampMs: uint64(ctx.BlockTime().UnixMilli()), Height: ctx.BlockHeight(), Sources: 1,
				})
				require.NoError(t, err)
				ctx.KVStore(feedKey).Set(pricefeedtypes.AggregateKey(0), bz)
			}
			endBlock := func() []perpstypes.EventRecord {
				setPrice()
				records := k.EndBlock(ctx)
				for _, event := range ctx.EventManager().Events() {
					require.NotEqual(t, "perps_endblock_error", event.Type, "%v", event)
				}
				return records
			}
			endBlock()
			sampled, err := k.GetMarketState(ctx, 0)
			require.NoError(t, err)
			require.Equal(t, uint64(1), sampled.PremiumCount)
			require.Equal(t, viewsWord(t, tc.premium), sampled.PremiumSum)
			require.Equal(t, uint64(viewsNow), sampled.LastSampleTime)
			wantRate := viewsFixed(t, tc.rate)
			require.Positive(t, new(big.Int).Abs(viewsFixed(t, tc.premium)).Cmp(new(big.Int).Abs(wantRate)))

			precompile, err := perps.NewPrecompile(viewsKeepers{Keepers: app.GetPrecompileKeepers(), perps: k})
			require.NoError(t, err)
			parsed, err := perpsabi.ABI()
			require.NoError(t, err)
			_, caller := testkeeper.MockAddressPair()
			view := func() *viewsEnv {
				stateDB := state.NewDBImpl(ctx.WithEventManager(sdk.NewEventManager()), &app.EvmKeeper, true)
				return &viewsEnv{t: t, ctx: ctx, keeper: k, abi: parsed, precompile: precompile, caller: caller,
					evm: &vm.EVM{StateDB: stateDB, TxContext: vm.TxContext{Origin: caller}}}
			}
			view().expect("getCurrentFundingRate", []interface{}{viewsInt(0)}, new(big.Int).Quo(wantRate, big.NewInt(3600)))

			nextHour := int64(viewsNow/3600+1) * 3600
			ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1).WithBlockTime(time.Unix(nextHour, 0)).
				WithEventManager(sdk.NewEventManager())
			var settled bool
			for _, record := range endBlock() {
				if record.Name == perpstypes.EventFundingSettled {
					require.Equal(t, []any{viewsInt(0), wantRate, viewsInt(0), viewsInt(0)}, record.Args)
					settled = true
				}
			}
			require.True(t, settled, "EndBlock must settle funding through the keeper hourly-rate calculation")
			funding, err := k.GetMarketState(ctx, 0)
			require.NoError(t, err)
			require.Equal(t, viewsWord(t, tc.rate), funding.HourlyRate)
			require.Equal(t, uint64(nextHour/3600), funding.LastIndexHour)
			require.Zero(t, funding.PremiumCount)
			require.True(t, funding.PremiumSum.IsZero())
			wantIndex := new(big.Int).Mul(big.NewInt(60_000), wantRate)
			view().expect("getFundingDetails", []interface{}{viewsInt(0)},
				wantIndex, viewsInt(nextHour), wantRate, viewsInt(0), viewsInt(0))
		})
	}
}
