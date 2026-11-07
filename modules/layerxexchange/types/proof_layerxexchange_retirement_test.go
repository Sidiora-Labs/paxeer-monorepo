// Proves: upgradekeeper.Keeper.SetDone and types.ErrRetired -> types.UpgradeKeeper and sdkerrors.ResponseDeliverTx -> keeper.Keeper.Retired, MsgServer and types.ErrorCode.
//
// Repairs modules/layerxexchange/types/errors.go: ErrRetired's registered
// description must be "layerxexchange is retired from v6.12.0", as declared by
// the retirement contract. The proof uses real keepers, a committed store that
// is closed and reopened, and the SDK's ABCI protobuf encoder and decoder.
// The registry and interface themselves have no persistent state; repeated
// calls and a live done-record transition prove that consumers do not cache
// their results. Truncating a producer-emitted response proves decode refusal.
package types_test

import (
	"errors"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

var _ types.UpgradeKeeper = upgradekeeper.Keeper{}

// retirementDelivery crosses the SDK's actual server/client error boundary.
// Its input is the error returned by the producer, never a fabricated response.
func retirementDelivery(t *testing.T, producer error) (abci.ResponseDeliverTx, []byte) {
	t.Helper()
	response := sdkerrors.ResponseDeliverTx(producer, 0, 0, false)
	wire, err := response.Marshal()
	require.NoError(t, err)
	var observed abci.ResponseDeliverTx
	require.NoError(t, observed.Unmarshal(wire))
	require.Equal(t, response, observed)
	return observed, wire
}

func TestProofLayerxexchangeRetirement(t *testing.T) {
	const doneHeight int64 = 20
	dir := t.TempDir()
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())

	// Each open creates fresh keys, stores and keepers over the on-disk database.
	open := func() (*rootmulti.Store, sdk.Context, upgradekeeper.Keeper, *keeper.Keeper) {
		t.Helper()
		db, err := dbm.NewDB("retirement", dbm.GoLevelDBBackend, dir)
		require.NoError(t, err)
		ms := rootmulti.NewStore(db)
		upgradeKey := sdk.NewKVStoreKey(upgradetypes.StoreKey)
		exchangeKey := sdk.NewKVStoreKey(types.StoreKey)
		ms.MountStoreWithDB(upgradeKey, storetypes.StoreTypeIAVL, nil)
		ms.MountStoreWithDB(exchangeKey, storetypes.StoreTypeIAVL, nil)
		require.NoError(t, ms.LoadLatestVersion())
		ctx := sdk.NewContext(ms, tmproto.Header{Height: doneHeight}, false)
		upgrade := upgradekeeper.NewKeeper(nil, upgradeKey, cdc, dir, nil)
		exchange := keeper.NewKeeper(cdc, exchangeKey, nil, nil)
		var surface types.UpgradeKeeper = upgrade
		exchange.SetUpgradeKeeper(surface)
		return ms, ctx, upgrade, exchange
	}

	ms, ctx, upgrade, exchange := open()
	defer func() { require.NoError(t, ms.Close()) }()
	server := keeper.NewMsgServerImpl(exchange)
	params := types.DefaultParams()
	params.TimeInForce = true
	msg := &types.MsgUpdateParams{Authority: exchange.Authority(ctx), Params: params}

	for call := 0; call < 2; call++ {
		require.False(t, upgrade.IsUpgradeActiveAtHeight(ctx, types.RetiredFromUpgrade, doneHeight))
		retired, err := exchange.Retired(ctx)
		require.NoError(t, err)
		require.False(t, retired)
		response, err := server.UpdateParams(sdk.WrapSDKContext(ctx), msg)
		require.NoError(t, err)
		require.NotNil(t, response)
		observed, _ := retirementDelivery(t, err)
		require.Zero(t, observed.Code)
		require.Empty(t, observed.Codespace)
		require.Empty(t, observed.Log)
		require.Equal(t, params, exchange.GetParams(ctx))
	}

	// A pending retirement plan and another completed upgrade are insufficient.
	upgrade.SetDone(ctx.WithBlockHeight(doneHeight-1), "v6.11.0")
	require.NoError(t, upgrade.ScheduleUpgrade(ctx.WithBlockHeight(1), upgradetypes.Plan{
		Name: types.RetiredFromUpgrade, Height: doneHeight,
	}))
	for call := 0; call < 2; call++ {
		retired, err := exchange.Retired(ctx)
		require.NoError(t, err)
		require.False(t, retired)
	}

	wrongSigner := *msg
	wrongSigner.Authority = sdk.AccAddress([]byte("wrong-signer-address")).String()
	response, err := server.UpdateParams(sdk.WrapSDKContext(ctx), &wrongSigner)
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	require.NotNil(t, err)
	require.Nil(t, response)
	unauthorized, _ := retirementDelivery(t, err)
	require.ErrorIs(t, sdkerrors.ABCIError(unauthorized.Codespace, unauthorized.Code, unauthorized.Log), sdkerrors.ErrUnauthorized)
	require.Equal(t, params, exchange.GetParams(ctx))

	// Change the real producer at the same context/height: no cached false result.
	upgrade.SetDone(ctx, types.RetiredFromUpgrade)
	require.Equal(t, doneHeight, upgrade.GetDoneHeight(ctx, types.RetiredFromUpgrade))
	retired, err := exchange.Retired(ctx)
	require.NoError(t, err)
	require.True(t, retired)
	commit := ms.Commit(true)
	require.Equal(t, int64(1), commit.Version)
	require.NoError(t, ms.Close())

	ms, ctx, upgrade, exchange = open()
	require.Equal(t, commit, ms.LastCommitID())
	require.Equal(t, doneHeight, upgrade.GetDoneHeight(ctx, types.RetiredFromUpgrade))
	require.Equal(t, params, exchange.GetParams(ctx))
	server = keeper.NewMsgServerImpl(exchange)

	// Replay older heights even after observing retirement, then check the edge.
	for _, height := range []int64{doneHeight, doneHeight - 1, doneHeight + 1, 0, doneHeight} {
		for call := 0; call < 2; call++ {
			retired, err := exchange.Retired(ctx.WithBlockHeight(height))
			require.NoError(t, err)
			require.Equal(t, height >= doneHeight, retired, "height %d, call %d", height, call)
		}
	}

	refusedParams := params
	refusedParams.Paused = true
	for call := 0; call < 2; call++ {
		callCtx := ctx.WithEventManager(sdk.NewEventManager())
		response, err := server.UpdateParams(sdk.WrapSDKContext(callCtx), &types.MsgUpdateParams{
			Authority: exchange.Authority(callCtx), Params: refusedParams,
		})
		require.Nil(t, response)
		require.ErrorIs(t, err, types.ErrRetired)
		require.Equal(t, "v6.12.0 done at height 20: layerxexchange is retired from v6.12.0", err.Error())
		observed, wire := retirementDelivery(t, err)
		require.Equal(t, "layerxexchange", observed.Codespace)
		require.Equal(t, uint32(14), observed.Code)
		require.Equal(t, "v6.12.0 done at height 20: layerxexchange is retired from v6.12.0", observed.Log)
		consumerErr := sdkerrors.ABCIError(observed.Codespace, observed.Code, observed.Log)
		require.ErrorIs(t, consumerErr, types.ErrRetired)
		require.Equal(t, "LXP_EXCHANGE_RETIRED", types.ErrorCode(consumerErr))
		require.Equal(t, params, exchange.GetParams(callCtx), "retirement must refuse the write")
		require.Empty(t, callCtx.EventManager().Events())

		// Cut the codespace's last byte from the encoded producer response. The
		// protobuf decoder must reject it before error classification is possible.
		require.Greater(t, len(wire), 1)
		var malformed abci.ResponseDeliverTx
		require.Error(t, malformed.Unmarshal(wire[:len(wire)-1]))
	}

	exchange.SetUpgradeKeeper(nil)
	for call := 0; call < 2; call++ {
		retired, err := exchange.Retired(ctx)
		require.False(t, retired)
		require.ErrorIs(t, err, types.ErrKeeperMissing)
		response, err := server.UpdateParams(sdk.WrapSDKContext(ctx), msg)
		require.Nil(t, response)
		require.ErrorIs(t, err, types.ErrKeeperMissing)
		observed, _ := retirementDelivery(t, err)
		require.Equal(t, uint32(12), observed.Code)
		consumerErr := sdkerrors.ABCIError(observed.Codespace, observed.Code, observed.Log)
		require.Equal(t, "LXP_EXCHANGE_KEEPER_MISSING", types.ErrorCode(consumerErr))
		require.Equal(t, params, exchange.GetParams(ctx))
	}
}

func TestProofLayerxexchangeRetirementErrorIdentity(t *testing.T) {
	require.Equal(t, "layerxexchange is retired from v6.12.0", types.ErrRetired.Error())
	for call := 0; call < 2; call++ {
		producer := sdkerrors.Wrap(types.ErrRetired, "legacy exchange message")
		observed, _ := retirementDelivery(t, producer)
		require.Equal(t, "layerxexchange", observed.Codespace)
		require.Equal(t, uint32(14), observed.Code)
		consumerErr := sdkerrors.ABCIError(observed.Codespace, observed.Code, observed.Log)
		require.True(t, errors.Is(consumerErr, types.ErrRetired))
		require.Equal(t, "LXP_EXCHANGE_RETIRED", types.ErrorCode(consumerErr))

		// Text alone cannot impersonate the registered SDK error identity.
		unregistered := errors.New(types.ErrRetired.Error())
		require.Empty(t, types.ErrorCode(unregistered))
		observed, _ = retirementDelivery(t, unregistered)
		consumerErr = sdkerrors.ABCIError(observed.Codespace, observed.Code, observed.Log)
		require.NotErrorIs(t, consumerErr, types.ErrRetired)
		require.Empty(t, types.ErrorCode(consumerErr))
		require.Empty(t, types.ErrorCode(nil))
	}
}
