package common_test

import (
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestUpgradeReaderOfTypedNil(t *testing.T) {
	k := new(evmkeeper.Keeper)
	require.Nil(t, k.UpgradeKeeper())

	for _, tc := range []struct {
		name   string
		keeper any
	}{
		{name: "nil keeper", keeper: nil},
		{name: "without upgrade accessor", keeper: struct{}{}},
		{name: "nil upgrade keeper", keeper: k},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := common.UpgradeReaderOf(tc.keeper)
			require.True(t, reader == nil, "reader must be a nil interface")
			require.False(t, common.RetiredAt(sdk.Context{}, reader, common.MergeUpgrade, 200))
		})
	}
}

func TestUpgradeReaderOfUpgradeKeeper(t *testing.T) {
	k := &testkeeper.EVMTestApp.EvmKeeper
	reader := common.UpgradeReaderOf(k)
	require.Same(t, &testkeeper.EVMTestApp.UpgradeKeeper, reader)
	require.Same(t, k.UpgradeKeeper(), reader)

	ctx, _ := testkeeper.EVMTestApp.GetContextForDeliverTx(nil).CacheContext()
	const plan = "test-upgrade-reader"
	const height = int64(200)
	require.False(t, common.RetiredAt(ctx, reader, plan, height))
	k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(height), plan)
	require.False(t, common.RetiredAt(ctx, reader, plan, height-1))
	require.True(t, common.RetiredAt(ctx, reader, plan, height))
	require.True(t, common.RetiredAt(ctx, reader, plan, height+1))
}
