package layerxpay_test

import (
	"math"
	"math/big"
	"testing"

	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestPayServedChargeTracksStoreGas(t *testing.T) {
	f := newEscrowTestFixture(t)
	h := f.h
	a := h.precompile.GetABI()
	method := a.Methods["escrowOpen"]
	row := lp.MethodGas[method.Name]
	evmKeeper := h.keepers.EVMK()
	cosmosPerEVM := evmKeeper.GetCosmosGasLimitFromEVMGas(h.db.Ctx(), 1)
	require.Equal(t, uint64(1), cosmosPerEVM)
	const supplied uint64 = 2000000
	var previous uint64
	for _, id := range [][32]byte{{0x40}, {0x41}} {
		args := f.openArgs(t, id)
		input, err := a.Pack(method.Name, args...)
		require.NoError(t, err)
		rev := h.db.Snapshot()
		meter := sdk.NewGasMeter(math.MaxUint64, 1, 1)
		h.db.WithCtx(h.db.Ctx().WithGasMeter(meter).WithEventManager(sdk.NewEventManager()))
		output, err := h.precompile.GetExecutor().Execute(h.db.Ctx(), &method, h.caller, h.caller, args, big.NewInt(0), false, h.evm, nil)
		require.NoError(t, err, "%x", output)
		kv := meter.GasConsumed()
		require.Positive(t, kv)
		h.db.RevertToSnapshot(rev)

		want := row.Gas(input[4:]) + kv/cosmosPerEVM
		before := h.keeper.ExportGenesis(h.db.Ctx())
		logs := len(h.db.GetAllLogs())
		output, left, err := h.evm.Call(h.caller, h.precompile.Address(), input, want-1, uint256.NewInt(0))
		require.ErrorIs(t, err, vm.ErrOutOfGas)
		require.Nil(t, output)
		require.Zero(t, left)
		require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
		require.Len(t, h.db.GetAllLogs(), logs)

		output, left, err = h.evm.Call(h.caller, h.precompile.Address(), input, supplied, uint256.NewInt(0))
		require.NoError(t, err, "%x", output)
		require.Empty(t, output)
		spent := supplied - left
		require.Equal(t, want, spent)
		require.Equal(t, kv, evmKeeper.GetCosmosGasLimitFromEVMGas(h.db.Ctx(), spent-row.Gas(input[4:])))
		require.Greater(t, spent, h.precompile.RequiredGas(input))
		if previous != 0 {
			require.Greater(t, spent, previous)
		}
		previous = spent
	}
}
