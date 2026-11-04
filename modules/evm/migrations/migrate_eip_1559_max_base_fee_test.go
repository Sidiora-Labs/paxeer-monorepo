package migrations_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/migrations"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/stretchr/testify/require"
)

func TestMigrateEip1559MaxBaseFee(t *testing.T) {
	dir := t.TempDir()
	k, ctx, ms, closeStore := repairM1243OpenStore(t, dir)
	stored := repairM1243GovernedParams(t)
	require.NotEqual(t, types.DefaultParams().MaximumFeePerGas, stored.MaximumFeePerGas)
	k.SetParams(ctx, stored)
	require.Equal(t, stored, k.GetParams(ctx))
	before := ms.Commit(true)
	require.Positive(t, before.Version)
	closeStore()

	k, ctx, ms, closeStore = repairM1243OpenStore(t, dir)
	require.Equal(t, before, ms.LastCommitID())
	require.Equal(t, stored, k.GetParams(ctx))

	require.NoError(t, migrations.MigrateEip1559MaxFeePerGas(ctx, k))
	expected := stored
	expected.MaximumFeePerGas = types.DefaultParams().MaximumFeePerGas
	migrated := k.GetParams(ctx)
	require.Equal(t, types.DefaultParams().MaximumFeePerGas, migrated.MaximumFeePerGas)
	require.Equal(t, expected, migrated, "only the maximum fee may change")

	after := ms.Commit(true)
	require.Greater(t, after.Version, before.Version)
	closeStore()
	k, ctx, ms, _ = repairM1243OpenStore(t, dir)
	require.Equal(t, after, ms.LastCommitID())
	require.Equal(t, expected, k.GetParams(ctx), "migrated and preserved parameters must survive reopening")
}
