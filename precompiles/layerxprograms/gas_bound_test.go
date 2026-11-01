package layerxprograms_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestProgramsCallOutOfGasDoesNotSubmit(t *testing.T) {
	h, args, _ := programsCallFixture(t)
	ctx := h.stateDB.Ctx()
	_, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	sequenceBefore, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	require.Positive(t, sequenceBefore)
	lastBefore, err := h.host.ReadLastSequence(ctx, did)
	require.NoError(t, err)

	mainName, mainID, err := lxphost.MainAccountID(h.publicKey)
	require.NoError(t, err)
	mainBefore, err := h.host.ReadAccount(ctx, mainName, mainID)
	require.NoError(t, err)
	require.Positive(t, mainBefore.Balance.Sign())
	fee, err := h.host.ReadFeeSchedule(ctx)
	require.NoError(t, err)
	asset := [32]byte(fee[65:97])
	_, payer, err := lxphost.AssetAccountID(h.publicKey, asset)
	require.NoError(t, err)
	balanceBefore, err := h.host.ReadBalance(ctx, payer, asset)
	require.NoError(t, err)
	require.Positive(t, balanceBefore.Sign())
	storeBefore := h.dump(ctx)

	input, err := lp.ProgramsABI.Pack("call", args...)
	require.NoError(t, err)
	row, ok := lp.GasTable["call"]
	require.True(t, ok)
	required := row.Gas(input[4:])
	require.Positive(t, required)
	require.Equal(t, required, h.precompile.RequiredGas(input))
	supplied := required - 1
	require.Less(t, supplied, required)
	output, remaining, err := h.evm.Call(h.caller, h.precompile.Address(), input, supplied, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrOutOfGas)
	require.Empty(t, output)
	require.Zero(t, remaining)
	require.NoError(t, h.stateDB.GetPrecompileError())
	require.Empty(t, h.stateDB.GetAllLogs())

	after := h.stateDB.Ctx()
	sequenceAfter, err := h.host.ReadIdentitySequence(after, did)
	require.NoError(t, err)
	require.Equal(t, sequenceBefore, sequenceAfter)
	lastAfter, err := h.host.ReadLastSequence(after, did)
	require.NoError(t, err)
	require.Equal(t, lastBefore, lastAfter)
	mainAfter, err := h.host.ReadAccount(after, mainName, mainID)
	require.NoError(t, err)
	require.Equal(t, mainBefore, mainAfter)
	balanceAfter, err := h.host.ReadBalance(after, payer, asset)
	require.NoError(t, err)
	require.Equal(t, balanceBefore, balanceAfter)
	require.Equal(t, storeBefore, h.dump(after))
}
