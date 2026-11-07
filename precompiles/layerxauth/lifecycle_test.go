package layerxauth_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
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
		before := hostSnapshot(t, db.Ctx(), h.hostKey)
		reference, _ := db.Ctx().CacheContext()
		produced, err := h.keeper.RecordLifecycle(reference, actor, payload, key)
		require.NoError(t, err)
		require.Zero(t, produced.ResultCode)
		events, err := authkeeper.AuthEvents(produced.Receipt)
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, "IdentityStateUpdated", events[0].Name)
		require.Equal(t, actor.Authority.Actor, events[0].DIDID)
		require.Len(t, produced.Events, 1)
		require.Equal(t, uint16(0x7110), produced.Events[0].Code)
		require.Equal(t, produced.Events[0].Body, events[0].Body)
		require.Equal(t, before, hostSnapshot(t, db.Ctx(), h.hostKey))
		logCount := len(db.GetAllLogs())
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
		require.Equal(t, produced.GlobalSequence, sequence)
		logs := db.GetAllLogs()
		require.Len(t, logs, logCount+len(events))
		for i, event := range events {
			log := logs[logCount+i]
			require.Equal(t, h.precompile.Address(), log.Address)
			require.Equal(t, uint(logCount+i), log.Index)
			require.Equal(t, []common.Hash{a.Events[event.Name].ID, common.Hash(actor.Authority.Actor)}, log.Topics)
			expected, err := a.Events[event.Name].Inputs.NonIndexed().Pack(event.Body)
			require.NoError(t, err)
			require.Equal(t, expected, log.Data)
			decoded, err := a.Events[event.Name].Inputs.NonIndexed().Unpack(log.Data)
			require.NoError(t, err)
			require.Equal(t, []interface{}{event.Body}, decoded)
			require.Equal(t, event.Body, db.Ctx().KVStore(h.hostKey).Get(append([]byte{1, 0, 7}, actor.Authority.Actor[:]...)))
		}
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

	invalidRecovery := bytes.Clone(recovery)
	clear(invalidRecovery[36:68])
	for _, tc := range []struct {
		name    string
		payload []byte
		reason  string
		cause   error
	}{
		{"ordinal five", []byte{0x71, 5, 1, 3}, "LXP_NON_CANONICAL", types.ErrNonCanonical},
		{"short payload", []byte{0x71, 1, 0}, "LXP_NON_CANONICAL", types.ErrNonCanonical},
		{"wrong tag", []byte{0x70, 1, 0, 2}, "LXP_NON_CANONICAL", types.ErrNonCanonical},
		{"duplicate registration", registration, "LXP_SEQUENCE_REUSED", types.ErrSequenceReused},
		{"invalid recovery root", invalidRecovery, "LXP_AUTH_SCOPE", types.ErrAuthScope},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := hostSnapshot(t, db.Ctx(), h.hostKey)
			logs := append([]*ethtypes.Log(nil), db.GetAllLogs()...)
			input, err := a.Pack("recordLifecycle", tc.payload, [32]byte{0x53})
			require.NoError(t, err)
			snapshot := db.Snapshot()
			data, err := h.precompile.Run(evm, h.caller, h.precompile.Address(), input, nil, false, false, nil)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			assertLxpRevert(t, data, tc.reason)
			require.ErrorIs(t, db.GetPrecompileError(), tc.cause)
			db.RevertToSnapshot(snapshot)
			require.Equal(t, before, hostSnapshot(t, db.Ctx(), h.hostKey))
			require.Equal(t, logs, db.GetAllLogs())
		})
	}
}
