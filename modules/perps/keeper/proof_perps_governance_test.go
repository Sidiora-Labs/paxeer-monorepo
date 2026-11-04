// Proves: types.MsgUpdateParams/MsgSetMarket/MsgSetPaused -> codec.ProtoCodec and baseapp.MsgServiceRouter -> keeper.NewMsgServerImpl and the sdk.Result consumer.
// The consumer below is the first event consumer in the governance inventory.
// It decodes actual service results twice, without retaining decoded messages,
// and reads committed params and markets after closing and reopening LevelDB.
// Repairs msg_server.go: msgServer.UpdateParams must reject a non-governance
// authority before decoding stored params, including when those bytes are corrupt.
// pause.go: Keeper.SetPaused shares that authorization check before reading state.
package keeper_test

import (
	"strconv"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type perpsGovernanceProof struct {
	db     dbm.DB
	ms     storetypes.CommitMultiStore
	ctx    sdk.Context
	key    *sdk.KVStoreKey
	k      *keeper.Keeper
	feed   pricefeedkeeper.Keeper
	cdc    *codec.ProtoCodec
	router *baseapp.MsgServiceRouter
}

func openPerpsGovernanceProof(t *testing.T, dir string) *perpsGovernanceProof {
	t.Helper()
	db, err := dbm.NewDB("governance", dbm.GoLevelDBBackend, dir)
	require.NoError(t, err)
	e := &perpsGovernanceProof{db: db, key: sdk.NewKVStoreKey(types.StoreKey)}
	t.Cleanup(func() {
		if e.db != nil {
			require.NoError(t, e.db.Close())
		}
	})
	feedKey := sdk.NewKVStoreKey(pricefeedtypes.StoreKey)
	e.ms = store.NewCommitMultiStore(db)
	e.ms.MountStoreWithDB(e.key, sdk.StoreTypeIAVL, nil)
	e.ms.MountStoreWithDB(feedKey, sdk.StoreTypeIAVL, nil)
	require.NoError(t, e.ms.LoadLatestVersion())
	e.ctx = sdk.NewContext(e.ms, tmproto.Header{Height: e.ms.LastCommitID().Version + 1,
		Time: time.Unix(listingTime, 0).UTC()}, false)
	e.feed = pricefeedkeeper.NewKeeper(feedKey, nil, nil, nil)
	e.k = keeper.NewKeeper(e.key, nil, e.feed)
	registry := cdctypes.NewInterfaceRegistry()
	perps.NewAppModuleBasic().RegisterInterfaces(registry)
	e.cdc = codec.NewProtoCodec(registry)
	e.router = baseapp.NewMsgServiceRouter()
	e.router.SetInterfaceRegistry(registry)
	perps.NewAppModule(e.k).RegisterServices(module.NewConfigurator(e.cdc, e.router, nil))
	return e
}

func (e *perpsGovernanceProof) reopen(t *testing.T, dir string) *perpsGovernanceProof {
	t.Helper()
	committed := e.ms.Commit(true)
	require.NoError(t, e.db.Close())
	e.db = nil
	next := openPerpsGovernanceProof(t, dir)
	require.Equal(t, committed, next.ms.LastCommitID())
	return next
}

func (e *perpsGovernanceProof) deliver(t *testing.T, msg sdk.Msg) (*sdk.Result, error) {
	t.Helper()
	wire, err := e.cdc.MarshalInterface(msg)
	require.NoError(t, err)
	var decoded sdk.Msg
	require.NoError(t, e.cdc.UnmarshalInterface(wire, &decoded))
	require.Equal(t, msg, decoded)
	require.NotSame(t, msg, decoded)
	require.Equal(t, msg.GetSigners(), decoded.GetSigners())
	handler := e.router.Handler(decoded)
	require.NotNil(t, handler)
	return handler(e.ctx, decoded)
}

func consumePerpsGovernanceResult(t *testing.T, result *sdk.Result, response proto.Message, event sdk.Event) {
	t.Helper()
	wire, err := proto.Marshal(result)
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		var observed sdk.Result
		require.NoError(t, proto.Unmarshal(wire, &observed))
		require.Equal(t, sdk.Events{event}, observed.GetEvents())
		decoded := proto.Clone(response)
		decoded.Reset()
		require.NoError(t, proto.Unmarshal(observed.Data, decoded))
		require.Equal(t, response, decoded)
		// Damage the real producer's bytes at the consumer boundary.
		var malformed sdk.Result
		require.Error(t, proto.Unmarshal(append(append([]byte(nil), wire...), 0x80), &malformed))
		require.Error(t, proto.Unmarshal(append(append([]byte(nil), observed.Data...), 0x80), decoded))
	}
}

func TestProofPerpsGovernanceProducerTransportConsumer(t *testing.T) {
	dir := t.TempDir()
	e := openPerpsGovernanceProof(t, dir)
	require.NoError(t, e.feed.SetParams(e.ctx, pricefeedtypes.DefaultParams(pricefeedtypes.DefaultAuthority())))
	market := classEMarket(0, "BTC")
	market.OiCapUsd = sdk.ZeroInt()
	params := types.DefaultParams()
	for round := 0; round < 2; round++ {
		params.SweepPositionsPerBlock = uint64(128 + round)
		result, err := e.deliver(t, &types.MsgUpdateParams{Authority: e.k.Authority(), Params: params})
		require.NoError(t, err)
		consumePerpsGovernanceResult(t, result, &types.MsgUpdateParamsResponse{},
			sdk.NewEvent("perps_params_updated", sdk.NewAttribute("authority", e.k.Authority())))

		market.Enabled = round == 0
		result, err = e.deliver(t, &types.MsgSetMarket{Authority: e.k.Authority(), Market: market})
		require.NoError(t, err)
		consumePerpsGovernanceResult(t, result, &types.MsgSetMarketResponse{}, sdk.NewEvent("perps_market_set",
			sdk.NewAttribute("market_id", strconv.FormatUint(market.ID, 10)),
			sdk.NewAttribute("enabled", strconv.FormatBool(market.Enabled))))
		market.ListedAt = uint64(listingTime)

		params.Paused = round == 0
		result, err = e.deliver(t, &types.MsgSetPaused{Authority: e.k.Authority(), Paused: params.Paused})
		require.NoError(t, err)
		consumePerpsGovernanceResult(t, result, &types.MsgSetPausedResponse{},
			sdk.NewEvent("perps_pause_set", sdk.NewAttribute("paused", strconv.FormatBool(params.Paused))))

		e = e.reopen(t, dir)
		for read := 0; read < 2; read++ {
			observedParams, err := e.k.GetParams(e.ctx)
			require.NoError(t, err)
			require.Equal(t, params, observedParams)
			observedMarket, err := e.k.GetMarket(e.ctx, market.ID)
			require.NoError(t, err)
			require.Equal(t, market, observedMarket)
			if params.Paused {
				require.ErrorIs(t, e.k.CheckPause(e.ctx, "openPosition"), types.ErrPaused)
			} else {
				require.NoError(t, e.k.CheckPause(e.ctx, "openPosition"))
			}
			require.NoError(t, e.k.CheckPause(e.ctx, "closePosition"))
		}
	}

	wrong := e.k.ModuleAddress().String()
	for _, msg := range []sdk.Msg{
		&types.MsgUpdateParams{Authority: wrong, Params: params},
		&types.MsgSetMarket{Authority: wrong, Market: market},
		&types.MsgSetPaused{Authority: wrong, Paused: true},
	} {
		before := storeContents(t, e.ctx.KVStore(e.key))
		for repeat := 0; repeat < 2; repeat++ {
			result, err := e.deliver(t, msg)
			require.ErrorIs(t, err, types.ErrUnauthorized)
			require.Nil(t, result)
			require.Equal(t, before, storeContents(t, e.ctx.KVStore(e.key)))
		}
	}
	unknown := classEMarket(999, "UNKNOWN")
	unknown.OiCapUsd = sdk.ZeroInt()
	before := storeContents(t, e.ctx.KVStore(e.key))
	result, err := e.deliver(t, &types.MsgSetMarket{Authority: e.k.Authority(), Market: unknown})
	require.ErrorIs(t, err, types.ErrUnknownMarket)
	require.Nil(t, result)
	require.Equal(t, before, storeContents(t, e.ctx.KVStore(e.key)))

	// Exercise the exported keeper entry point as well as its Msg adapter.
	require.NoError(t, e.k.SetPaused(e.ctx, e.k.Authority(), true))
	e = e.reopen(t, dir)
	for repeat := 0; repeat < 2; repeat++ {
		require.ErrorIs(t, e.k.CheckPause(e.ctx, "openPosition"), types.ErrPaused)
		require.ErrorIs(t, e.k.SetPaused(e.ctx, wrong, false), types.ErrUnauthorized)
		stored, err := e.k.GetParams(e.ctx)
		require.NoError(t, err)
		params.Paused = true
		require.Equal(t, params, stored)
	}
}

func TestProofPerpsGovernanceMalformedAndAuthorization(t *testing.T) {
	e := openPerpsGovernanceProof(t, t.TempDir())
	market := classEMarket(0, "BTC")
	market.OiCapUsd = sdk.ZeroInt()
	for _, msg := range []sdk.Msg{
		&types.MsgUpdateParams{Authority: e.k.Authority(), Params: types.DefaultParams()},
		&types.MsgSetMarket{Authority: e.k.Authority(), Market: market},
		&types.MsgSetPaused{Authority: e.k.Authority(), Paused: true},
	} {
		wire, err := e.cdc.MarshalInterface(msg)
		require.NoError(t, err)
		for repeat := 0; repeat < 2; repeat++ {
			var decoded sdk.Msg
			require.Error(t, e.cdc.UnmarshalInterface(append(append([]byte(nil), wire...), 0x80), &decoded))
			require.Nil(t, decoded)
		}
	}
	require.Nil(t, e.ctx.KVStore(e.key).Get(types.ParamsKey))
	wrong := e.k.ModuleAddress().String()
	corrupt := []byte("{invalid stored params")
	e.ctx.KVStore(e.key).Set(types.ParamsKey, corrupt)
	for repeat := 0; repeat < 2; repeat++ {
		result, err := e.deliver(t, &types.MsgUpdateParams{Authority: wrong, Params: types.DefaultParams()})
		require.ErrorIs(t, err, types.ErrUnauthorized)
		require.Nil(t, result)
		require.ErrorIs(t, e.k.SetPaused(e.ctx, wrong, true), types.ErrUnauthorized)
		_, err = e.k.GetParams(e.ctx)
		require.ErrorIs(t, err, types.ErrInvalidParams)
		require.ErrorIs(t, e.k.SetPaused(e.ctx, e.k.Authority(), true), types.ErrInvalidParams)
		result, err = e.deliver(t, &types.MsgUpdateParams{Authority: e.k.Authority(), Params: types.DefaultParams()})
		require.ErrorIs(t, err, types.ErrInvalidParams)
		require.Nil(t, result)
		result, err = e.deliver(t, &types.MsgSetPaused{Authority: wrong, Paused: true})
		requireRefused(t, err, types.ErrUnauthorized, types.ErrUnauthorized.ABCICode())
		require.Nil(t, result)
		require.Equal(t, corrupt, e.ctx.KVStore(e.key).Get(types.ParamsKey))
		require.Empty(t, e.ctx.EventManager().Events())
	}
}
