package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestRetiredAfterV6120Done(t *testing.T) {
	f := newFixture(t)
	key := sdk.NewKVStoreKey("layerxexchange-retire-upgrade")
	ctx := withStore(t, f.ctx, key)
	upgrade := upgradekeeper.NewKeeper(nil, key, testkeeper.EVMTestApp.AppCodec(), t.TempDir(), nil)
	f.keeper.SetUpgradeKeeper(upgrade)
	const doneHeight int64 = 100

	for _, height := range []int64{1, doneHeight - 1, doneHeight, doneHeight + 1} {
		retired, err := f.keeper.Retired(ctx.WithBlockHeight(height))
		require.NoError(t, err)
		require.False(t, retired, "no done record at height %d", height)
	}

	upgrade.SetDone(ctx.WithBlockHeight(doneHeight-1), "v6.11.0")
	require.NoError(t, upgrade.ScheduleUpgrade(ctx.WithBlockHeight(1), upgradetypes.Plan{
		Name: types.RetiredFromUpgrade, Height: doneHeight,
	}))
	retired, err := f.keeper.Retired(ctx.WithBlockHeight(doneHeight))
	require.NoError(t, err)
	require.False(t, retired, "a pending retirement plan and another completed upgrade do not retire the exchange")

	upgrade.SetDone(ctx.WithBlockHeight(doneHeight), types.RetiredFromUpgrade)
	require.Equal(t, doneHeight, upgrade.GetDoneHeight(ctx, types.RetiredFromUpgrade))
	for _, tc := range []struct {
		height  int64
		retired bool
	}{
		{0, false},
		{1, false},
		{doneHeight - 1, false},
		{doneHeight, true},
		{doneHeight + 1, true},
		{doneHeight + 100, true},
	} {
		retired, err := f.keeper.Retired(ctx.WithBlockHeight(tc.height))
		require.NoError(t, err)
		require.Equal(t, tc.retired, retired, "height %d", tc.height)
	}
}

func TestRetiredFailsClosedWithoutUpgradeKeeper(t *testing.T) {
	f := newFixture(t)
	retired, err := f.keeper.Retired(f.ctx)
	require.False(t, retired)
	require.ErrorIs(t, err, types.ErrKeeperMissing)
	codespace, code, _ := sdkerrors.ABCIInfo(err, false)
	require.Equal(t, types.ModuleName, codespace)
	require.Equal(t, uint32(12), code)
	require.Equal(t, types.CodeKeeperMissing, types.ErrorCode(err))
}
