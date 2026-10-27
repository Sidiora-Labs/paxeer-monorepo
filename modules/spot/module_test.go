package spot_test

import (
	"encoding/json"
	"testing"
	"time"

	pricefeedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/testutil"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// invariantRecorder records the routes RegisterInvariants registers.
type invariantRecorder struct{ routes []string }

func (r *invariantRecorder) RegisterRoute(moduleName, route string, _ sdk.Invariant) {
	r.routes = append(r.routes, moduleName+"/"+route)
}

func TestModuleRegistersInvariants(t *testing.T) {
	am := spot.NewAppModule(keeper.NewKeeper(sdk.NewKVStoreKey(types.StoreKey), nil, nil, pricefeedkeeper.Keeper{}))

	invariants := &invariantRecorder{}
	am.RegisterInvariants(invariants)
	require.Equal(t, []string{
		"spot/" + keeper.SolvencyInvariantName,
		"spot/" + keeper.ShareBackingInvariantName,
		"spot/" + keeper.EscrowInvariantName,
		"spot/" + keeper.ActiveIndexInvariantName,
	}, invariants.routes)
}

func TestModuleBasics(t *testing.T) {
	basic := spot.NewAppModuleBasic()
	require.Equal(t, types.ModuleName, basic.Name())
	require.Equal(t, "spot", basic.Name())

	bz := basic.DefaultGenesis(nil)
	var gs types.GenesisState
	require.NoError(t, json.Unmarshal(bz, &gs))
	require.Equal(t, *types.DefaultGenesis(), gs)
	require.NoError(t, basic.ValidateGenesis(nil, nil, bz))

	require.Error(t, basic.ValidateGenesis(nil, nil, json.RawMessage(`{`)))
	invalid := types.DefaultGenesis()
	invalid.NextOrderID = 0
	require.ErrorIs(t, basic.ValidateGenesis(nil, nil, mustJSON(t, invalid)), types.ErrInvalidGenesis)
}

func TestGenesisRoundTrip(t *testing.T) {
	app := testkeeper.EVMTestApp
	ctx := app.GetContextForDeliverTx([]byte{}).WithBlockHeight(10)
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	pricefeed, ctx := pricefeedtestutil.NewKeeper(app, ctx)
	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx = pricefeedtestutil.WithPriceFeedStore(ctx, key)
	k := keeper.NewKeeper(key, app.BankKeeper, &app.EvmKeeper, pricefeed)
	am := spot.NewAppModule(k)

	input := am.DefaultGenesis(nil)
	require.Empty(t, am.InitGenesis(ctx, nil, input))
	exported := am.ExportGenesis(ctx, nil)
	require.JSONEq(t, string(input), string(exported))
	require.Equal(t, types.DefaultGenesis(), k.ExportGenesis(ctx))
	require.NoError(t, am.ValidateGenesis(nil, nil, exported))
}

func mustJSON(t *testing.T, gs *types.GenesisState) json.RawMessage {
	t.Helper()
	bz, err := json.Marshal(gs)
	require.NoError(t, err)
	return bz
}

func newEndBlockerKeeper(t *testing.T) (*keeper.Keeper, sdk.Context, sdk.StoreKey, sdk.StoreKey) {
	t.Helper()
	app := testkeeper.EVMTestApp
	ctx := app.GetContextForDeliverTx(nil).WithBlockHeight(10).WithBlockTime(time.Unix(1000, 0).UTC())
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx)).WithEventManager(sdk.NewEventManager())
	key := sdk.NewKVStoreKey(types.StoreKey)
	feedKey := sdk.NewKVStoreKey(pricefeedtypes.StoreKey)
	ctx = pricefeedtestutil.WithPriceFeedStore(ctx, feedKey)
	ctx = pricefeedtestutil.WithPriceFeedStore(ctx, key)
	feed := pricefeedkeeper.NewKeeper(feedKey, app.StakingKeeper, app.AccountKeeper, &app.EvmKeeper)
	require.NoError(t, feed.SetParams(ctx, pricefeedtypes.DefaultParams(pricefeedtypes.DefaultAuthority())))
	k := keeper.NewKeeper(key, app.BankKeeper, &app.EvmKeeper, feed)
	k.InitGenesis(ctx, *types.DefaultGenesis())
	return k, ctx.WithEventManager(sdk.NewEventManager()), key, feedKey
}

func TestEndBlockerEmitsRestingEvent(t *testing.T) {
	for _, tc := range []struct {
		name    string
		sampled bool
		want    string
	}{
		{name: "idle", want: "0"},
		{name: "price samples", sampled: true, want: "2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx, key, feedKey := newEndBlockerKeeper(t)
			params := types.DefaultParams()
			if tc.sampled {
				params.Tokens = types.OpeningTokens(common.HexToAddress("0xa1"), common.HexToAddress("0xb2"))
				for _, token := range params.Tokens {
					if token.IsStablecoin {
						continue
					}
					price := sdk.NewIntWithDecimal(1, 18)
					aggregate, err := json.Marshal(pricefeedtypes.Aggregate{
						IndexPrice: price, MarkPrice: price, Basis: sdk.ZeroInt(),
						TimestampMs: uint64(ctx.BlockTime().UnixMilli()), Height: ctx.BlockHeight(), Sources: 1,
					})
					require.NoError(t, err)
					ctx.KVStore(feedKey).Set(pricefeedtypes.AggregateKey(token.MarketID), aggregate)
				}
			}
			require.NoError(t, k.UpdateParams(ctx, k.Authority(), params))

			require.NoError(t, spot.EndBlocker(ctx, k))
			require.Equal(t, sdk.Events{sdk.NewEvent("spot_resting",
				sdk.NewAttribute("samples", tc.want),
				sdk.NewAttribute("expired", "0"),
				sdk.NewAttribute("activated", "0"),
				sdk.NewAttribute("filled", "0"),
			)}, ctx.EventManager().Events())
			if tc.sampled {
				for _, token := range params.Tokens[1:] {
					var ring types.TwapRing
					ringKey := append(append([]byte(nil), types.TwapRingPrefix...), token.Address.Bytes()...)
					require.NoError(t, json.Unmarshal(ctx.KVStore(key).Get(ringKey), &ring))
					require.Equal(t, uint32(1), ring.Count)
				}
			}
		})
	}
}

func TestEndBlockerEmitsFailureEvent(t *testing.T) {
	k, ctx, key, _ := newEndBlockerKeeper(t)
	cursorKey := append(append([]byte(nil), types.CursorPrefix...), byte(1))
	malformed := []byte{byte(types.FillIndexLimit)}
	ctx.KVStore(key).Set(cursorKey, malformed)

	err := spot.EndBlocker(ctx, k)
	require.EqualError(t, err, "spot: invalid fill cursor")
	require.Equal(t, sdk.Events{sdk.NewEvent("spot_resting_failed",
		sdk.NewAttribute("error", err.Error()),
	)}, ctx.EventManager().Events())
	require.Equal(t, malformed, ctx.KVStore(key).Get(cursorKey))
}
