package xweb_test

import (
	"testing"

	xwebkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestOriginTwoMethodsBeforeTheMergeRevertModuleDisabled(t *testing.T) {
	h := newHarness(t)
	observation := types.Observation{Origin: types.OriginProgram, NetworkID: kernelNetworkID,
		ProgramID: types.Hash32{0x71}, RequestID: 17, Kind: types.KindSearch,
		PayloadHash: types.Keccak(payload), ContentDigest: types.Keccak(response),
		FullLength: fullLength, Response: response}
	raw := signProgramDispatchObservation(t, h, observation, 2)
	for _, tc := range []struct {
		method     string
		args       []interface{}
		base       uint64
		signatures int
		readOnly   bool
	}{
		{xweb.FulfilProgramMethod, []interface{}{raw}, xweb.WriteBaseGas, 2, false},
		{xweb.GetProgramResultMethod, []interface{}{[32]byte(observation.ProgramID), observation.RequestID}, xweb.ViewBaseGas, 0, true},
	} {
		t.Run(tc.method, func(t *testing.T) {
			input := h.input(tc.method, tc.args...)
			wantGas := cost(input, tc.base, tc.signatures)
			for _, height := range []int64{startHeight, 99} {
				h.stateDB.WithCtx(h.ctx().WithBlockHeight(height))
				require.False(t, h.keeper.ProgramOriginActive(h.ctx()))
				require.Equal(t, wantGas, h.precompile.RequiredGas(input))
				call := func(gas uint64) ([]byte, uint64, error) {
					if tc.readOnly {
						return h.evm.StaticCall(stranger, precompile, input, gas)
					}
					return h.evm.Call(stranger, precompile, input, gas, uint256.NewInt(0))
				}
				_, left, err := call(wantGas - 1)
				require.ErrorIs(t, err, vm.ErrOutOfGas)
				require.Zero(t, left)
				ret, left, err := call(supplied)
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				reason, err := abi.UnpackRevert(ret)
				require.NoError(t, err)
				require.Equal(t, "LXP_MODULE_DISABLED", reason)
				require.Equal(t, supplied-wantGas, left)
				require.Zero(t, h.keeper.ProgramPendingCount(h.ctx()))
				require.Zero(t, h.keeper.ProgramCommittedCount(h.ctx()))
			}
		})
	}
}

func TestGetKernelNetworkIdBeforeTheMergeRevertsModuleDisabled(t *testing.T) {
	h := newHarness(t)
	input := h.input(xweb.GetKernelNetworkIdMethod)
	wantGas := cost(input, xweb.ViewBaseGas, 0)
	assertDisabled := func(height int64) {
		t.Helper()
		h.stateDB.WithCtx(h.ctx().WithBlockHeight(height))
		require.False(t, h.keeper.ProgramOriginActive(h.ctx()))
		require.Equal(t, wantGas, h.precompile.RequiredGas(input))
		ret, left, err := h.evm.StaticCall(stranger, precompile, input, supplied)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		reason, err := abi.UnpackRevert(ret)
		require.NoError(t, err)
		require.Equal(t, "LXP_MODULE_DISABLED", reason)
		require.Equal(t, supplied-wantGas, left)
	}
	assertDisabled(startHeight)
	assertDisabled(99)
	const planHeight = int64(100)
	testkeeper.EVMTestApp.UpgradeKeeper.SetDone(h.ctx().WithBlockHeight(planHeight), xwebkeeper.ProgramOriginPlan)
	assertDisabled(planHeight - 1)
	for _, height := range []int64{planHeight, planHeight + 1} {
		h.stateDB.WithCtx(h.ctx().WithBlockHeight(height))
		require.True(t, h.keeper.ProgramOriginActive(h.ctx()))
		require.Equal(t, wantGas, h.precompile.RequiredGas(input))
		require.Equal(t, []interface{}{h.keeper.GetParams(h.ctx()).KernelNetworkId}, h.view(xweb.GetKernelNetworkIdMethod))
	}
}
