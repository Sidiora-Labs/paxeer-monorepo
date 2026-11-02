package precompiles_test

import (
	"fmt"
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMergeServes0x101D(t *testing.T) {
	entry := precompiles.MergeAddress{Address: common.HexToAddress("0x101D"), Name: "layerxprograms"}
	require.Contains(t, precompiles.MergeAddressTable, entry)
	MissingMergeConstructors(t, entry.Address)
	RegisterMergeConstructor(t, entry.Address)

	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx(nil).CacheContext()
	const planHeight = int64(100)
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))
	require.NotContains(t, a.UpgradeKeeper.GetModuleVersionMap(ctx), entry.Name)
	for _, tracing := range []bool{false, true} {
		CustomPrecompiles(t, &a.EvmKeeper, ctx.WithBlockHeight(planHeight-1).WithIsTracing(tracing), entry, false)
	}

	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(planHeight), precompiles.MergeUpgrade)
	require.Equal(t, planHeight, a.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))
	for _, tracing := range []bool{false, true} {
		for _, height := range []int64{planHeight - 1, planHeight, planHeight + 1, planHeight - 1} {
			t.Run(fmt.Sprintf("height=%d/tracing=%t", height, tracing), func(t *testing.T) {
				CustomPrecompiles(t, &a.EvmKeeper, ctx.WithBlockHeight(height).WithIsTracing(tracing), entry, height >= planHeight)
			})
		}
	}
}

func MissingMergeConstructors(t *testing.T, address common.Address) {
	t.Helper()
	for _, entry := range precompiles.MissingMergeConstructors() {
		require.NotEqual(t, address, entry.Address, "the node must register the layerxprograms constructor")
	}
}

func RegisterMergeConstructor(t *testing.T, address common.Address) {
	t.Helper()
	require.EqualError(t, precompiles.RegisterMergeConstructor(address, layerxprograms.NewMergePrecompile),
		"merge constructor for "+address.Hex()+" already registered")
}

func CustomPrecompiles(t *testing.T, k *evmkeeper.Keeper, ctx sdk.Context, entry precompiles.MergeAddress, active bool) {
	t.Helper()
	served := k.CustomPrecompiles(ctx)
	if !active {
		require.NotContains(t, served, entry.Address, "height=%d tracing=%t", ctx.BlockHeight(), ctx.IsTracing())
		return
	}
	require.Contains(t, served, entry.Address, "height=%d tracing=%t", ctx.BlockHeight(), ctx.IsTracing())
	contract, ok := served[entry.Address].(precompiles.IPrecompile)
	require.True(t, ok, "the served layerxprograms contract must report its name and address")
	require.Equal(t, entry.Name, contract.GetName())
	require.Equal(t, entry.Address, contract.Address())
}
