package precompiles_test

import (
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMergeServes0x101B(t *testing.T) {
	address := common.HexToAddress("0x101B")
	require.Contains(t, precompiles.MergeAddressTable, precompiles.MergeAddress{
		Address: address,
		Name:    "layerxpay",
	})
	MissingMergeConstructors(t, address)
	RegisterMergeConstructor(t, address)

	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx(nil).CacheContext()
	const mergeHeight int64 = 100
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(mergeHeight), precompiles.MergeUpgrade)
	require.Equal(t, mergeHeight, a.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))

	for _, mode := range []struct {
		name    string
		tracing bool
	}{
		{name: "execution"},
		{name: "tracing", tracing: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			for _, block := range []struct {
				name   string
				height int64
				served bool
			}{
				{name: "below plan height", height: mergeHeight - 1},
				{name: "at plan height", height: mergeHeight, served: true},
				{name: "after plan height", height: mergeHeight + 1, served: true},
			} {
				t.Run(block.name, func(t *testing.T) {
					at := ctx.WithBlockHeight(block.height).WithIsTracing(mode.tracing)
					CustomPrecompiles(t, &a.EvmKeeper, at, address, block.served)
				})
			}
		})
	}
}

func MissingMergeConstructors(t *testing.T, address common.Address) {
	t.Helper()
	for _, missing := range precompiles.MissingMergeConstructors() {
		require.NotEqual(t, address, missing.Address, "the node must register the layerxpay constructor")
	}
}

func RegisterMergeConstructor(t *testing.T, address common.Address) {
	t.Helper()
	require.ErrorContains(t,
		precompiles.RegisterMergeConstructor(address, layerxpay.NewMergePrecompile),
		"already registered",
		"the node must have registered the real layerxpay constructor before building its precompiles",
	)
}

func CustomPrecompiles(t *testing.T, k *evmkeeper.Keeper, ctx sdk.Context, address common.Address, wantServed bool) {
	t.Helper()
	served := k.CustomPrecompiles(ctx)
	if !wantServed {
		require.NotContains(t, served, address, "layerxpay must be withheld below the merge plan height")
		return
	}
	require.Contains(t, served, address, "layerxpay must be served from the merge plan height")
	contract, ok := served[address].(precompiles.IPrecompile)
	require.True(t, ok, "the served layerxpay contract must report its name and address")
	require.Equal(t, address, contract.Address())
	require.Equal(t, "layerxpay", contract.GetName())
}
