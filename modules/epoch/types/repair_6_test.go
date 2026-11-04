package types_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/epoch"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/epoch/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/epoch/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

func repair6Keeper(t *testing.T) (*keeper.Keeper, sdk.Context, *codec.ProtoCodec, *sdk.KVStoreKey) {
	t.Helper()
	db := tmdb.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	stateStore := store.NewCommitMultiStore(db)
	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	memKey := storetypes.NewMemoryStoreKey(types.MemStoreKey)
	stateStore.MountStoreWithDB(storeKey, sdk.StoreTypeIAVL, db)
	stateStore.MountStoreWithDB(memKey, sdk.StoreTypeMemory, nil)
	require.NoError(t, stateStore.LoadLatestVersion())
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	params := paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), storeKey, memKey, types.ModuleName)
	k := keeper.NewKeeper(cdc, storeKey, memKey, params)
	ctx := sdk.NewContext(stateStore, tmproto.Header{
		Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Height: 1,
	}, false)
	return k, ctx, cdc, storeKey
}

func TestRepair_6_NonPositiveGenesisRejected(t *testing.T) {
	for _, duration := range []time.Duration{time.Duration(-1 << 63), -time.Minute, -time.Nanosecond, 0} {
		t.Run(duration.String(), func(t *testing.T) {
			k, ctx, cdc, storeKey := repair6Keeper(t)
			basic := epoch.NewAppModuleBasic(cdc)
			genesis := types.DefaultGenesis()
			genesis.Epoch.EpochDuration = duration
			raw := cdc.MustMarshalJSON(genesis)
			var decoded types.GenesisState
			require.NoError(t, cdc.UnmarshalAsJSON(raw, &decoded))
			require.Equal(t, duration, decoded.Epoch.EpochDuration)
			require.EqualError(t, decoded.Validate(), "epoch duration must be positive")
			require.EqualError(t, basic.ValidateGenesis(cdc, nil, raw), "epoch duration must be positive")
			stream := make(chan json.RawMessage, 1)
			stream <- raw
			close(stream)
			require.EqualError(t, basic.ValidateGenesisStream(cdc, nil, stream), "epoch duration must be positive")
			require.Nil(t, ctx.KVStore(storeKey).Get([]byte(keeper.EpochKey)))
			require.Empty(t, ctx.EventManager().Events())

			valid := types.DefaultGenesis()
			require.NoError(t, basic.ValidateGenesis(cdc, nil, cdc.MustMarshalJSON(valid)))
			epoch.InitGenesis(ctx, *k, *valid)
			before := k.GetEpoch(ctx)
			exported := epoch.ExportGenesis(ctx, *k)
			exported.Epoch.EpochDuration = duration
			require.EqualError(t, basic.ValidateGenesis(cdc, nil, cdc.MustMarshalJSON(exported)), "epoch duration must be positive")
			ctx = ctx.WithBlockTime(ctx.BlockTime().Add(time.Second)).WithBlockHeight(2)
			k.BeginBlock(ctx)
			require.Equal(t, before, k.GetEpoch(ctx))
			require.Empty(t, ctx.EventManager().Events())
		})
	}
}

func TestRepair_6_PositiveGenesisBoundedCatchup(t *testing.T) {
	for _, duration := range []time.Duration{time.Nanosecond, time.Minute} {
		t.Run(duration.String(), func(t *testing.T) {
			k, ctx, cdc, _ := repair6Keeper(t)
			genesis := types.DefaultGenesis()
			genesis.Epoch.EpochDuration = duration
			raw := cdc.MustMarshalJSON(genesis)
			require.NoError(t, epoch.NewAppModuleBasic(cdc).ValidateGenesis(cdc, nil, raw))
			var decoded types.GenesisState
			cdc.MustUnmarshalJSON(raw, &decoded)
			epoch.InitGenesis(ctx, *k, decoded)
			initial := k.GetEpoch(ctx)
			require.Equal(t, ctx.BlockTime(), initial.GenesisTime)
			require.Equal(t, ctx.BlockTime(), initial.CurrentEpochStartTime)
			require.Equal(t, duration, initial.EpochDuration)
			require.NoError(t, initial.Validate())

			ctx = ctx.WithBlockTime(initial.CurrentEpochStartTime.Add(duration)).WithBlockHeight(2)
			k.BeginBlock(ctx)
			require.Equal(t, initial, k.GetEpoch(ctx))
			require.Empty(t, ctx.EventManager().Events())

			ctx = ctx.WithBlockTime(initial.CurrentEpochStartTime.Add(5*duration + time.Nanosecond))
			for offset := uint64(1); offset <= 5; offset++ {
				ctx = ctx.WithBlockHeight(2 + int64(offset)).WithEventManager(sdk.NewEventManager())
				k.BeginBlock(ctx)
				current := k.GetEpoch(ctx)
				require.Equal(t, initial.CurrentEpoch+offset, current.CurrentEpoch)
				require.Equal(t, initial.CurrentEpochStartTime.Add(time.Duration(offset)*duration), current.CurrentEpochStartTime)
				require.Equal(t, ctx.BlockHeight(), current.CurrentEpochHeight)
				require.Equal(t, initial.GenesisTime, current.GenesisTime)
				require.Equal(t, duration, current.EpochDuration)
				require.NoError(t, current.Validate())
				require.Equal(t, sdk.Events{sdk.NewEvent(types.EventTypeNewEpoch,
					sdk.NewAttribute(types.AttributeEpochNumber, fmt.Sprint(current.CurrentEpoch)),
					sdk.NewAttribute(types.AttributeEpochTime, current.CurrentEpochStartTime.String()),
					sdk.NewAttribute(types.AttributeEpochHeight, fmt.Sprint(current.CurrentEpochHeight)),
				)}, ctx.EventManager().Events())
			}
			caughtUp := k.GetEpoch(ctx)
			ctx = ctx.WithBlockHeight(8).WithEventManager(sdk.NewEventManager())
			k.BeginBlock(ctx)
			require.Equal(t, caughtUp, k.GetEpoch(ctx))
			require.Empty(t, ctx.EventManager().Events())
			exported := epoch.ExportGenesis(ctx, *k)
			require.Equal(t, caughtUp, *exported.Epoch)
			require.NoError(t, epoch.NewAppModuleBasic(cdc).ValidateGenesis(cdc, nil, cdc.MustMarshalJSON(exported)))
		})
	}
}
