package precompiles_test

import (
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMergeServes0x101C(t *testing.T) {
	address := common.HexToAddress("0x000000000000000000000000000000000000101C")
	entry := precompiles.MergeAddress{Address: address, Name: "layerxauth"}
	require.Contains(t, precompiles.MergeAddressTable, entry)
	missingMergeConstructors0x101C(t, address)
	registerMergeConstructor0x101C(t, address)

	testApp := testkeeper.EVMTestApp
	ctx, _ := testApp.GetContextForDeliverTx([]byte{}).CacheContext()
	const planHeight = int64(100)
	testApp.UpgradeKeeper.SetDone(ctx.WithBlockHeight(planHeight), precompiles.MergeUpgrade)
	require.Equal(t, planHeight, testApp.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))

	for _, tracing := range []bool{false, true} {
		blockCtx := ctx.WithIsTracing(tracing)
		customPrecompiles0x101C(t, &testApp.EvmKeeper, blockCtx.WithBlockHeight(planHeight-1), entry, false)
		customPrecompiles0x101C(t, &testApp.EvmKeeper, blockCtx.WithBlockHeight(planHeight), entry, true)
		customPrecompiles0x101C(t, &testApp.EvmKeeper, blockCtx.WithBlockHeight(planHeight+1), entry, true)
		customPrecompiles0x101C(t, &testApp.EvmKeeper, blockCtx.WithBlockHeight(planHeight-1), entry, false)
	}
}

func missingMergeConstructors0x101C(t *testing.T, address common.Address) {
	t.Helper()
	for _, entry := range precompiles.MissingMergeConstructors() {
		require.NotEqual(t, address, entry.Address, "0x101C must have its real constructor registered by node imports")
	}
}

func registerMergeConstructor0x101C(t *testing.T, address common.Address) {
	t.Helper()
	require.EqualError(t,
		precompiles.RegisterMergeConstructor(address, nil),
		"nil merge constructor for "+address.Hex())
}

func customPrecompiles0x101C(t *testing.T, keeper *evmkeeper.Keeper, ctx sdk.Context, entry precompiles.MergeAddress, active bool) {
	t.Helper()
	served := keeper.CustomPrecompiles(ctx)
	if !active {
		require.NotContains(t, served, entry.Address, "height=%d tracing=%t", ctx.BlockHeight(), ctx.IsTracing())
		return
	}
	require.Contains(t, served, entry.Address, "height=%d tracing=%t", ctx.BlockHeight(), ctx.IsTracing())
	contract, ok := served[entry.Address].(precompiles.IPrecompile)
	require.True(t, ok, "0x101C must expose the precompile name and address")
	require.Equal(t, entry.Name, contract.GetName())
	require.Equal(t, entry.Address, contract.Address())
}
