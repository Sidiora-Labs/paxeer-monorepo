package keeper_test

import (
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
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
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

var _ types.UpgradeKeeper = upgradekeeper.Keeper{}

func repairM574Delivery(t *testing.T, err error, code uint32) {
	t.Helper()
	produced := sdkerrors.ResponseDeliverTx(err, 0, 0, false)
	wire, marshalErr := produced.Marshal()
	require.NoError(t, marshalErr)
	var received abci.ResponseDeliverTx
	require.NoError(t, received.Unmarshal(wire))
	require.Equal(t, produced, received)
	require.Equal(t, types.ModuleName, received.Codespace)
	require.Equal(t, code, received.Code)
	classified := sdkerrors.ABCIError(received.Codespace, received.Code, received.Log)
	if code == 14 {
		require.ErrorIs(t, classified, types.ErrRetired)
		require.Equal(t, types.CodeRetired, types.ErrorCode(classified))
	} else {
		require.ErrorIs(t, classified, types.ErrKeeperMissing)
		require.Equal(t, types.CodeKeeperMissing, types.ErrorCode(classified))
	}
}

func TestRepairM_574_PersistedUpgradeReaderContract(t *testing.T) {
	const doneHeight int64 = 100
	dir := t.TempDir()
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	open := func() (*rootmulti.Store, sdk.Context, upgradekeeper.Keeper, *keeper.Keeper) {
		t.Helper()
		db, err := dbm.NewDB("repair-m574", dbm.GoLevelDBBackend, dir)
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
		var reader types.UpgradeKeeper = upgrade
		exchange.SetUpgradeKeeper(reader)
		return ms, ctx, upgrade, exchange
	}
	ms, ctx, upgrade, exchange := open()
	t.Cleanup(func() {
		if ms != nil {
			require.NoError(t, ms.Close())
		}
	})
	var reader types.UpgradeKeeper = upgrade
	require.Zero(t, reader.GetDoneHeight(ctx, types.RetiredFromUpgrade))
	upgrade.SetDone(ctx.WithBlockHeight(doneHeight-1), "v6.11.0")
	require.NoError(t, upgrade.ScheduleUpgrade(ctx.WithBlockHeight(1), upgradetypes.Plan{
		Name: types.RetiredFromUpgrade, Height: doneHeight,
	}))
	retired, err := exchange.Retired(ctx)
	require.NoError(t, err)
	require.False(t, retired, "scheduled retirement and another completed upgrade do not retire the exchange")
	params := types.DefaultParams()
	params.TimeInForce = true
	params.Markets = []types.Market{{
		MarketId: custodytypes.Hash32([32]byte{1}), MarginAssetId: custodytypes.Hash32([32]byte{2}), Enabled: true,
	}}
	server := keeper.NewMsgServerImpl(exchange)
	response, err := server.UpdateParams(sdk.WrapSDKContext(ctx), &types.MsgUpdateParams{
		Authority: exchange.Authority(ctx), Params: params,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, params, exchange.GetParams(ctx))
	upgrade.SetDone(ctx, types.RetiredFromUpgrade)
	require.Equal(t, doneHeight, reader.GetDoneHeight(ctx, types.RetiredFromUpgrade))
	retired, err = exchange.Retired(ctx)
	require.NoError(t, err)
	require.True(t, retired, "the same reader observes a newly completed upgrade")
	commit := ms.Commit(true)
	require.NoError(t, ms.Close())
	ms = nil
	ms, ctx, upgrade, exchange = open()
	require.Equal(t, commit, ms.LastCommitID())
	reader = upgrade
	require.Equal(t, doneHeight, reader.GetDoneHeight(ctx, types.RetiredFromUpgrade))
	require.Equal(t, params, exchange.GetParams(ctx))
	server = keeper.NewMsgServerImpl(exchange)
	before := exchange.ExportGenesis(ctx)
	owner := common.HexToAddress("0x1234")
	for _, height := range []int64{doneHeight, doneHeight - 1, doneHeight + 1, 0, doneHeight} {
		callCtx := ctx.WithBlockHeight(height).WithEventManager(sdk.NewEventManager())
		retired, err := exchange.Retired(callCtx)
		require.NoError(t, err)
		require.Equal(t, height >= doneHeight, retired, "replayed height %d", height)
		require.Equal(t, doneHeight, reader.GetDoneHeight(callCtx, types.RetiredFromUpgrade))
		if height < doneHeight {
			response, err := server.UpdateParams(sdk.WrapSDKContext(callCtx), &types.MsgUpdateParams{
				Authority: exchange.Authority(callCtx), Params: params,
			})
			require.NoError(t, err)
			require.NotNil(t, response)
			continue
		}
		intent, err := exchange.RecordCancelOrderIntent(callCtx, owner, [32]byte{3})
		require.Equal(t, types.Intent{}, intent)
		require.ErrorIs(t, err, types.ErrRetired)
		require.Equal(t, "v6.12.0 done at height 100: layerxexchange is retired from v6.12.0", err.Error())
		repairM574Delivery(t, err, 14)
		changed := params
		changed.Paused = true
		response, err := server.UpdateParams(sdk.WrapSDKContext(callCtx), &types.MsgUpdateParams{
			Authority: exchange.Authority(callCtx), Params: changed,
		})
		require.Nil(t, response)
		require.ErrorIs(t, err, types.ErrRetired)
		repairM574Delivery(t, err, 14)
		market := params.Markets[0]
		market.Enabled = false
		marketResponse, err := server.SetMarket(sdk.WrapSDKContext(callCtx), &types.MsgSetMarket{
			Authority: exchange.Authority(callCtx), Market: market,
		})
		require.Nil(t, marketResponse)
		require.ErrorIs(t, err, types.ErrRetired)
		repairM574Delivery(t, err, 14)
		require.Equal(t, before, exchange.ExportGenesis(callCtx))
		require.Zero(t, exchange.GetOwnerNonce(callCtx, owner))
		require.Empty(t, callCtx.EventManager().Events())
	}
	exchange.SetUpgradeKeeper(nil)
	intent, err := exchange.RecordCancelOrderIntent(ctx, owner, [32]byte{3})
	require.Equal(t, types.Intent{}, intent)
	require.ErrorIs(t, err, types.ErrKeeperMissing)
	repairM574Delivery(t, err, 12)
	require.Equal(t, before, exchange.ExportGenesis(ctx))
}

func TestRepairM_574_ApplicationUpgradeDependency(t *testing.T) {
	app := testkeeper.EVMTestApp
	var reader types.UpgradeKeeper = app.UpgradeKeeper
	var _ SetUpgradeKeeper = app.LayerXExchangeKeeper
	ctx, _ := app.GetContextForDeliverTx(nil).WithBlockHeight(2).CacheContext()
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	require.Equal(t, int64(1), reader.GetDoneHeight(ctx, types.RetiredFromUpgrade))
	require.True(t, reader.IsUpgradeActiveAtHeight(ctx, types.RetiredFromUpgrade, ctx.BlockHeight()))
	before := app.LayerXExchangeKeeper.ExportGenesis(ctx)
	intent, err := app.LayerXExchangeKeeper.RecordCancelOrderIntent(ctx, common.HexToAddress("0x1234"), [32]byte{3})
	require.Equal(t, types.Intent{}, intent)
	require.ErrorIs(t, err, types.ErrRetired)
	require.Equal(t, "v6.12.0 done at height 1: layerxexchange is retired from v6.12.0", err.Error())
	repairM574Delivery(t, err, 14)
	require.Equal(t, before, app.LayerXExchangeKeeper.ExportGenesis(ctx))
	require.Empty(t, ctx.EventManager().Events())
}
