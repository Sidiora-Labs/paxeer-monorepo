package layerxauth_test

import (
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestPrecompileRecordLifecycleRegistersDid(t *testing.T) {
	h := newHarness(t)
	host, err := lxphost.NewHost(lxphost.Config{
		StoreKey: h.hostKey, NetworkID: 7, ParameterVersion: 1,
		TimestampWindowMs: 60000, GasLimit: 1000000,
	})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	actor, err := h.keeper.Actor(h.ctx, h.caller)
	require.NoError(t, err)
	evm, db := h.evm()
	require.Equal(t, common.HexToAddress(layerxauth.AuthAddress), h.precompile.Address())
	a := h.precompile.GetABI()
	row, found := layerxauth.GasTable["recordLifecycle"]
	require.True(t, found)
	require.True(t, row.Write)
	require.Equal(t, uint64(36680), row.Surcharge)
	call := func(payload []byte, key [32]byte) uint64 {
		t.Helper()
		input, err := a.Pack("recordLifecycle", payload, key)
		require.NoError(t, err)
		require.Equal(t, row.Gas(input[4:]), h.precompile.RequiredGas(input))
		data, err := h.precompile.Run(evm, h.caller, h.precompile.Address(), input, nil, false, false, nil)
		require.NoError(t, err, "recordLifecycle returned %x", data)
		values, err := a.Methods["recordLifecycle"].Outputs.Unpack(data)
		require.NoError(t, err)
		require.Len(t, values, 1)
		sequence := values[0].(uint64)
		require.NotZero(t, sequence)
		return sequence
	}
	before, err := host.ReadIdentitySequence(db.Ctx(), actor.DID)
	require.NoError(t, err)
	require.Zero(t, before)
	registration := append([]byte{0x71, 1, 0, 2}, actor.Authority.Actor[:]...)
	registration = append(registration, actor.DIDKey[:]...)
	require.Len(t, registration, 68)
	registered := call(registration, [32]byte{0x51})
	next, err := host.ReadIdentitySequence(db.Ctx(), actor.DID)
	require.NoError(t, err)
	require.Equal(t, uint64(1), next)
	identity, err := host.ReadIdentityState(db.Ctx(), actor.DID)
	require.NoError(t, err)
	require.Equal(t, actor.Authority.Actor, identity.DIDID)
	require.Equal(t, actor.DIDKey, identity.PrimaryKey)
	require.Equal(t, registered, identity.RevocationSequence)

	root := [32]byte{0x61}
	recovery := append([]byte{0x71, 3, 0, 3}, actor.Authority.Actor[:]...)
	recovery = append(recovery, root[:]...)
	recovery = binary.BigEndian.AppendUint16(recovery, 2)
	require.Len(t, recovery, 70)
	recovered := call(recovery, [32]byte{0x52})
	require.Greater(t, recovered, registered)
	identity, err = host.ReadIdentityState(db.Ctx(), actor.DID)
	require.NoError(t, err)
	require.Equal(t, root, identity.RecoveryRoot)
	require.Equal(t, uint16(2), identity.RecoveryThreshold)
	require.Equal(t, uint64(2), identity.NextSequence)

	for _, tc := range []struct {
		name    string
		payload []byte
	}{
		{"ordinal five", []byte{0x71, 5, 1, 3}},
		{"short payload", []byte{0x71, 1, 0}},
		{"wrong tag", []byte{0x70, 1, 0, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := hostSnapshot(t, db.Ctx(), h.hostKey)
			input, err := a.Pack("recordLifecycle", tc.payload, [32]byte{0x53})
			require.NoError(t, err)
			snapshot := db.Snapshot()
			data, err := h.precompile.Run(evm, h.caller, h.precompile.Address(), input, nil, false, false, nil)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			assertLxpRevert(t, data, "LXP_NON_CANONICAL")
			require.ErrorIs(t, db.GetPrecompileError(), types.ErrNonCanonical)
			db.RevertToSnapshot(snapshot)
			require.Equal(t, before, hostSnapshot(t, db.Ctx(), h.hostKey))
		})
	}
}
