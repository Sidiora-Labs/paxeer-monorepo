package precompiles_test

import (
	"fmt"
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMergeServes0x101A(t *testing.T) {
	address := common.HexToAddress("0x101A")
	var ledger precompiles.MergeAddress
	for _, entry := range precompiles.MergeAddressTable {
		if entry.Address == address {
			ledger = entry
		}
	}
	require.Equal(t, precompiles.MergeAddress{Address: address, Name: "layerxledger"}, ledger)
	MissingMergeConstructors(t, address)
	RegisterMergeConstructor(t, address)

	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(20)
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))
	require.Contains(t, a.UpgradeKeeper.GetModuleVersionMap(ctx), ledgertypes.ModuleName)
	a.UpgradeKeeper.SetDone(ctx, precompiles.MergeUpgrade)
	require.Equal(t, ctx.BlockHeight(), a.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))
	CustomPrecompiles(t, &a.EvmKeeper, ctx, ledger)
}

func MissingMergeConstructors(t *testing.T, address common.Address) {
	t.Helper()
	for _, entry := range precompiles.MissingMergeConstructors() {
		require.NotEqual(t, address, entry.Address, "node must register the ledger merge constructor")
	}
}

func RegisterMergeConstructor(t *testing.T, address common.Address) {
	t.Helper()
	require.EqualError(t, precompiles.RegisterMergeConstructor(address, layerxledger.NewMergePrecompile),
		"merge constructor for "+address.Hex()+" already registered")
}

func CustomPrecompiles(t *testing.T, k *evmkeeper.Keeper, ctx sdk.Context, ledger precompiles.MergeAddress) {
	t.Helper()
	planHeight := ctx.BlockHeight()
	for _, tracing := range []bool{false, true} {
		for _, height := range []int64{1, planHeight - 1, planHeight, planHeight + 1} {
			t.Run(fmt.Sprintf("height=%d/tracing=%t", height, tracing), func(t *testing.T) {
				at := ctx.WithBlockHeight(height).WithIsTracing(tracing)
				served := k.CustomPrecompiles(at)
				if height < planHeight {
					require.NotContains(t, served, ledger.Address)
					return
				}
				require.Contains(t, served, ledger.Address)
				contract, ok := served[ledger.Address].(precompiles.IPrecompile)
				require.True(t, ok, "served ledger must expose its precompile identity")
				require.Equal(t, ledger.Address, contract.Address())
				require.Equal(t, "layerxledger", contract.GetName())
			})
		}
	}
}
