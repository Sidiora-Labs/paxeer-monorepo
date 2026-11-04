package migrations_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/migrations"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/stretchr/testify/require"
)

func TestMigrateEip1559Params(t *testing.T) {
	dir := t.TempDir()
	k, ctx, ms, closeStore := repairM1243OpenStore(t, dir)
	stored := repairM1243GovernedParams(t)
	defaults := types.DefaultParams()
	require.NotEqual(t, defaults.MaxDynamicBaseFeeUpwardAdjustment, stored.MaxDynamicBaseFeeUpwardAdjustment)
	require.NotEqual(t, defaults.MaxDynamicBaseFeeDownwardAdjustment, stored.MaxDynamicBaseFeeDownwardAdjustment)
	require.NotEqual(t, defaults.TargetGasUsedPerBlock, stored.TargetGasUsedPerBlock)
	require.NotEqual(t, defaults.MinimumFeePerGas, stored.MinimumFeePerGas)
	k.SetParams(ctx, stored)
	require.Equal(t, stored, k.GetParams(ctx))
	before := ms.Commit(true)
	require.Positive(t, before.Version)
	closeStore()

	k, ctx, ms, closeStore = repairM1243OpenStore(t, dir)
	require.Equal(t, before, ms.LastCommitID())
	require.Equal(t, stored, k.GetParams(ctx))

	require.NoError(t, migrations.MigrateEip1559Params(ctx, k))
	expected := stored
	expected.MaxDynamicBaseFeeUpwardAdjustment = defaults.MaxDynamicBaseFeeUpwardAdjustment
	expected.MaxDynamicBaseFeeDownwardAdjustment = defaults.MaxDynamicBaseFeeDownwardAdjustment
	expected.TargetGasUsedPerBlock = defaults.TargetGasUsedPerBlock
	expected.MinimumFeePerGas = defaults.MinimumFeePerGas
	migrated := k.GetParams(ctx)
	require.Equal(t, defaults.MaxDynamicBaseFeeUpwardAdjustment, migrated.MaxDynamicBaseFeeUpwardAdjustment)
	require.Equal(t, defaults.MaxDynamicBaseFeeDownwardAdjustment, migrated.MaxDynamicBaseFeeDownwardAdjustment)
	require.Equal(t, defaults.TargetGasUsedPerBlock, migrated.TargetGasUsedPerBlock)
	require.Equal(t, defaults.MinimumFeePerGas, migrated.MinimumFeePerGas)
	require.Equal(t, stored.BaseFeePerGas, migrated.BaseFeePerGas)
	require.Equal(t, expected, migrated, "only the four EIP-1559 parameters may change")

	after := ms.Commit(true)
	require.Greater(t, after.Version, before.Version)
	closeStore()
	k, ctx, ms, _ = repairM1243OpenStore(t, dir)
	require.Equal(t, after, ms.LastCommitID())
	require.Equal(t, expected, k.GetParams(ctx), "migrated and preserved parameters must survive reopening")
}
