package launchpad_test

import (
	"math/big"
	"testing"

	launchpadkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/launchpad"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestRepair_36_NativeValueLoss(t *testing.T) {
	s := newSurfaceHarness(t)
	s.stateDB.AddBalance(s.alice, uint256.NewInt(1_000_000_000_000_000_000), tracing.BalanceChangeUnspecified)
	s.stateDB.AddBalance(s.creator, uint256.NewInt(1_000_000_000_000_000_000), tracing.BalanceChangeUnspecified)
	require.NoError(t, s.stateDB.Error())
	check := func(t *testing.T, to common.Address, from common.Address, input []byte) {
		t.Helper()
		// First prove this selector succeeds when no native currency is sent.
		snapshot := s.stateDB.Snapshot()
		_, _, err := s.evm.Call(from, to, input, surfaceGas, uint256.NewInt(0))
		require.NoError(t, err)
		s.stateDB.RevertToSnapshot(snapshot)
		beforeState := s.exported()
		beforeLogs := len(s.stateDB.GetAllLogs())
		senderBalance := s.stateDB.GetBalance(from).ToBig()
		targetBalance := s.stateDB.GetBalance(to).ToBig()
		precompileBalance := s.stateDB.GetBalance(precompile).ToBig()
		s.frames = nil
		_, _, err = s.evm.Call(from, to, input, surfaceGas, uint256.NewInt(1))
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.NoError(t, s.stateDB.Error())
		require.Equal(t, senderBalance, s.stateDB.GetBalance(from).ToBig())
		require.Equal(t, targetBalance, s.stateDB.GetBalance(to).ToBig())
		require.Equal(t, precompileBalance, s.stateDB.GetBalance(precompile).ToBig())
		require.Equal(t, beforeState, s.exported())
		require.Len(t, s.stateDB.GetAllLogs(), beforeLogs)
		require.Len(t, s.frames, 1, "native value must be refused before forwarding")
	}
	for role, address := range launchpadkeeper.Generation2Surfaces {
		t.Run(role, func(t *testing.T) {
			require.Equal(t, launchpadkeeper.SurfaceRuntime, s.stateDB.GetCode(address))
			check(t, address, s.alice, s.encode(role, "DEFAULT_ADMIN_ROLE"))
		})
	}
	for i, pool := range s.pools {
		t.Run(big.NewInt(int64(i)).String(), func(t *testing.T) {
			require.Equal(t, launchpadkeeper.SurfaceRuntime, s.stateDB.GetCode(pool))
			check(t, pool, s.alice, s.encode(types.RolePool, "paused"))
			check(t, pool, s.creator, s.encode(types.RolePool, "pause"))
		})
	}
	role := repair36Grant(t, s, "repair 36 value role")
	t.Run("role write", func(t *testing.T) {
		check(t, surfaceAt(types.RoleRouter), s.alice, s.encode(types.RoleRouter, "renounceRole", role, s.alice))
	})
	t.Run("direct", func(t *testing.T) {
		check(t, precompile, s.alice, s.encode(types.RoleRouter, "usdlAddress"))
	})
}

func TestRepair_36_MulticallCumulativeGas(t *testing.T) {
	s := newSurfaceHarness(t)
	view := s.encode(types.RoleRouter, "usdlAddress")
	inner := s.encode(types.RoleRouter, "multicall", [][]byte{view, view})
	input := s.encode(types.RoleRouter, "multicall", [][]byte{view, inner})
	cost := launchpad.Gas(uint64(len(input)), launchpad.Writes("multicall")) +
		launchpad.Gas(uint64(len(inner)), launchpad.Writes("multicall")) +
		3*launchpad.Gas(uint64(len(view)), 0)
	ret, left, err := s.evm.Call(s.alice, precompile, input, cost, uint256.NewInt(0))
	require.NoError(t, err)
	require.Zero(t, left)
	results := s.decode(types.RoleRouter, "multicall", ret)[0].([][]byte)
	require.Len(t, results, 2)
	want := common.LeftPadBytes(s.usdl.Bytes(), 32)
	require.Equal(t, want, results[0])
	require.Equal(t, [][]byte{want, want}, s.decode(types.RoleRouter, "multicall", results[1])[0])
	_, left, err = s.evm.Call(s.alice, precompile, input, cost-1, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrOutOfGas)
	require.Zero(t, left)
}

func repair36Grant(t *testing.T, s *surfaceHarness, label string) [32]byte {
	t.Helper()
	role := [32]byte(crypto.Keccak256Hash([]byte(label)))
	granted, err := s.keeper.GovernanceGrant(s.ctx(), s.keeper.Authority(), surfaceAt(types.RoleRouter), role, s.alice)
	require.NoError(t, err)
	require.True(t, granted)
	return role
}

func TestRepair_36_MulticallWriteGasRollback(t *testing.T) {
	s := newSurfaceHarness(t)
	roleA := repair36Grant(t, s, "repair 36 first role")
	roleB := repair36Grant(t, s, "repair 36 second role")
	a := s.encode(types.RoleRouter, "renounceRole", roleA, s.alice)
	b := s.encode(types.RoleRouter, "renounceRole", roleB, s.alice)
	input := s.encode(types.RoleRouter, "multicall", [][]byte{a, b})
	cost := launchpad.Gas(uint64(len(input)), launchpad.Writes("multicall")) +
		launchpad.Gas(uint64(len(a)), launchpad.Writes("renounceRole")) +
		launchpad.Gas(uint64(len(b)), launchpad.Writes("renounceRole"))
	before := s.exported()
	logs := len(s.stateDB.GetAllLogs())
	events := len(s.ctx().EventManager().Events())
	_, left, err := s.evm.Call(s.alice, precompile, input, cost-1, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrOutOfGas)
	require.Zero(t, left)
	require.Equal(t, before, s.exported(), "the first entry's keeper write must roll back")
	require.Len(t, s.stateDB.GetAllLogs(), logs, "the first entry's RoleRevoked log must roll back")
	require.Len(t, s.ctx().EventManager().Events(), events)
	require.True(t, s.keeper.HasRole(s.ctx(), surfaceAt(types.RoleRouter), roleA, s.alice))
	require.True(t, s.keeper.HasRole(s.ctx(), surfaceAt(types.RoleRouter), roleB, s.alice))
	ret, left, err := s.evm.Call(s.alice, precompile, input, cost, uint256.NewInt(0))
	require.NoError(t, err)
	require.Zero(t, left)
	require.Equal(t, [][]byte{{}, {}}, s.decode(types.RoleRouter, "multicall", ret)[0])
	require.False(t, s.keeper.HasRole(s.ctx(), surfaceAt(types.RoleRouter), roleA, s.alice))
	require.False(t, s.keeper.HasRole(s.ctx(), surfaceAt(types.RoleRouter), roleB, s.alice))
	require.Len(t, s.stateDB.GetAllLogs(), logs+2)
}

func TestRepair_36_MulticallDepthRollback(t *testing.T) {
	s := newSurfaceHarness(t)
	role := repair36Grant(t, s, "repair 36 depth role")
	revoke := s.encode(types.RoleRouter, "renounceRole", role, s.alice)
	batch := s.method(types.RoleRouter, "multicall")
	pack := func(entries ...[]byte) []byte {
		args, err := batch.Inputs.Pack(entries)
		require.NoError(t, err)
		return append(append([]byte(nil), batch.ID...), args...)
	}
	// One root and limit-1 nested batches put the last entry at the limit.
	nested := s.encode(types.RoleRouter, "usdlAddress")
	for i := uint64(0); i < params.CallCreateDepth-1; i++ {
		nested = pack(nested)
	}
	allowed := pack(nested)
	_, _, err := s.evm.Call(s.alice, precompile, allowed, 4_000_000_000, uint256.NewInt(0))
	require.NoError(t, err, "batching must still work through the permitted depth")
	require.Zero(t, s.evm.GetDepth())
	before := s.exported()
	logs := len(s.stateDB.GetAllLogs())
	events := len(s.ctx().EventManager().Events())
	input := pack(revoke, allowed)
	ret, left, err := s.evm.Call(s.alice, precompile, input, 4_000_000_000, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, pcommon.RevertReason(vm.ErrDepth), ret)
	require.Positive(t, left, "the refusal is a depth bound, not exhausted gas")
	require.Equal(t, before, s.exported())
	require.True(t, s.keeper.HasRole(s.ctx(), surfaceAt(types.RoleRouter), role, s.alice))
	require.Len(t, s.stateDB.GetAllLogs(), logs)
	require.Len(t, s.ctx().EventManager().Events(), events)
	require.Zero(t, s.evm.GetDepth(), "synthetic frames must be released after failure")
	_, err = s.call(s.alice, surfaceAt(types.RoleRouter), s.encode(types.RoleRouter, "usdlAddress"))
	require.NoError(t, err, "depth refusal must leave the EVM usable")
}
