package layerxprograms_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestProgramsRegistryReadEmitsEvent(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []byte
	}{
		{"manifest", []byte{0, 'm', 'a', 'n', 'i', 'f', 'e', 's', 't', 0xff}},
		{"empty body", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			ctx := h.stateDB.Ctx()
			programID := crypto.Keccak256Hash([]byte("registry program"))
			method := lp.ProgramsABI.Methods["registryRead"]
			input, err := lp.ProgramsABI.Pack("registryRead", [32]byte(programID), tc.body)
			require.NoError(t, err)
			payload, err := lp.EncodeRegistryReadPayload([32]byte(programID), tc.body)
			require.NoError(t, err)
			require.Equal(t, append(append([]byte(nil), programID[:]...), tc.body...), payload)
			authority, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
			require.NoError(t, err)
			var selector [4]byte
			copy(selector[:], method.ID)
			timestamp := uint64(ctx.BlockTime().UnixMilli())
			reference, _ := ctx.CacheContext()
			receipt, err := h.host.Execute(lxphost.WithSequence(reference, 1), lxphost.Activity{
				ProtocolVersion: 3,
				Type:            0x00090004,
				ActorDID:        did,
				AuthorityKey:    h.publicKey,
				AccountSequence: 0,
				NotBefore:       timestamp,
				NotAfter:        timestamp,
				IdempotencyKey:  lp.IdempotencyKey([32]byte(ctx.EVMTxHash()), 0, selector),
				FeeLimit:        [16]byte{},
				Payload:         payload,
			}, authority)
			require.NoError(t, err)
			require.Zero(t, receipt.ResultCode)
			require.Equal(t, []lxphost.ReceiptEvent{{Code: 4, Body: programID[:]}}, receipt.Events)

			out, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
			require.NoError(t, err)
			require.Empty(t, out)
			require.Equal(t, h.dump(reference), h.dump(h.stateDB.Ctx()))
			sequence, err := h.host.ReadIdentitySequence(h.stateDB.Ctx(), did)
			require.NoError(t, err)
			require.Equal(t, uint64(1), sequence)
			lastSequence, err := h.host.ReadLastSequence(h.stateDB.Ctx(), did)
			require.NoError(t, err)
			require.Equal(t, uint64(1), lastSequence)
			logs := h.stateDB.GetAllLogs()
			require.Len(t, logs, 1)
			require.Equal(t, common.HexToAddress("0x101D"), logs[0].Address)
			require.Equal(t, []common.Hash{crypto.Keccak256Hash([]byte("RegistryRead(bytes32)")), programID}, logs[0].Topics)
			require.Empty(t, logs[0].Data)
		})
	}

	t.Run("kernel refusal emits no event", func(t *testing.T) {
		h := newHarness(t)
		h.executor.SetTestHost(h.newHost(1))
		before := h.dump(h.stateDB.Ctx())
		input, err := lp.ProgramsABI.Pack("registryRead", [32]byte{0xa1}, []byte("manifest"))
		require.NoError(t, err)
		data, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		reason, err := abi.UnpackRevert(data)
		require.NoError(t, err)
		require.Equal(t, lp.KernelError(lxphost.Code(-601), "registryRead").Error(), reason)
		require.Empty(t, h.stateDB.GetAllLogs())
		require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	})
}

func TestProgramsRegistryReadZeroProgramReverts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []byte
	}{
		{"manifest", []byte("manifest")},
		{"empty body", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			ctx := h.stateDB.Ctx()
			before := h.dump(ctx)
			_, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
			require.NoError(t, err)
			input, err := lp.ProgramsABI.Pack("registryRead", [32]byte{}, tc.body)
			require.NoError(t, err)
			data, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			reason, err := abi.UnpackRevert(data)
			require.NoError(t, err)
			require.Equal(t, "LXP_NON_CANONICAL: program ID is zero", reason)
			require.EqualError(t, h.stateDB.GetPrecompileError(), reason)
			require.Empty(t, h.stateDB.GetAllLogs())
			require.Equal(t, before, h.dump(h.stateDB.Ctx()))
			sequence, err := h.host.ReadIdentitySequence(h.stateDB.Ctx(), did)
			require.NoError(t, err)
			require.Zero(t, sequence)
			lastSequence, err := h.host.ReadLastSequence(h.stateDB.Ctx(), did)
			require.NoError(t, err)
			require.Zero(t, lastSequence)
			branch, _ := h.stateDB.Ctx().CacheContext()
			require.Zero(t, lp.NextOrdinal(branch, [32]byte(ctx.EVMTxHash())))
		})
	}
}
