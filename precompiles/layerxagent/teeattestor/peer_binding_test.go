package teeattestor_test

import (
	"encoding/hex"
	"testing"

	xwebtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/testutil"
	xwebtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/teeattestor"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func peerEnvelope(h *harness, family uint8, peerKey, modelDigest, runtimeDigest [32]byte) []byte {
	h.t.Helper()
	envelope := h.quote(family, blockOpen-60, false, 7, h.keys[:3]...)[:96]
	report := crypto.Keccak256Hash(peerKey[:], modelDigest[:], runtimeDigest[:])
	copy(envelope[39:71], report[:])
	digest := xwebtypes.Hash32(crypto.Keccak256Hash(envelope[:95]))
	for _, signature := range xwebtestutil.Sign(digest, h.keys[:3]...) {
		envelope = append(envelope, signature...)
	}
	return envelope
}

func readPeerBinding(h *harness, peerKey [32]byte) []interface{} {
	h.t.Helper()
	ret, _, err := h.evm.StaticCall(caller, h.precompile.Address(), h.input("peerBinding", peerKey), suppliedGas)
	require.NoError(h.t, err)
	a := h.precompile.GetABI()
	fields, err := a.Methods["peerBinding"].Outputs.Unpack(ret)
	require.NoError(h.t, err)
	require.Len(h.t, fields, 4)
	return fields
}

func TestPeerBindingMatchingEnvelopeAndStoredFields(t *testing.T) {
	h := newHarness(t, 5)
	peerKey, modelDigest, runtimeDigest := [32]byte{0x11}, [32]byte{0x22}, [32]byte{0x33}
	a := h.precompile.GetABI()
	require.Equal(t, "c7c179e5", hex.EncodeToString(a.Methods["bindPeer"].ID))
	require.Equal(t, "c3c05f2e", hex.EncodeToString(a.Methods["peerBinding"].ID))
	require.False(t, a.Methods["bindPeer"].IsConstant())
	require.True(t, a.Methods["peerBinding"].IsConstant())
	require.Equal(t, []interface{}{uint8(0), [32]byte{}, [32]byte{}, uint64(0)}, readPeerBinding(h, peerKey))
	for family := uint8(0); family < 4; family++ {
		peerKey[31] = family
		envelope := peerEnvelope(h, family, peerKey, modelDigest, runtimeDigest)
		verified := attestation(t, h.view("verify", family, envelope))
		require.Equal(t, [32]byte(crypto.Keccak256Hash(peerKey[:], modelDigest[:], runtimeDigest[:])), verified.ReportData)
		input := h.input("bindPeer", family, envelope, peerKey, modelDigest, runtimeDigest)
		require.Equal(t, uint64(106000+30*len(input)), h.precompile.RequiredGas(input))
		ret, left, err := h.evm.Call(caller, h.precompile.Address(), input, suppliedGas, uint256.NewInt(0))
		require.NoError(t, err)
		require.Equal(t, suppliedGas-h.precompile.RequiredGas(input), left)
		output, err := a.Methods["bindPeer"].Outputs.Unpack(ret)
		require.NoError(t, err)
		require.Equal(t, []interface{}{true}, output)
		require.Equal(t, []interface{}{family, modelDigest, runtimeDigest, uint64(activationHeight)}, readPeerBinding(h, peerKey))
		logs := h.stateDB.GetAllLogs()
		require.Len(t, logs, 1)
		event := a.Events[teeattestor.PeerAttestationBound]
		require.Equal(t, crypto.Keccak256Hash([]byte("PeerAttestationBound(bytes32,uint8,bytes32,bytes32)")), event.ID)
		require.Equal(t, h.precompile.Address(), logs[0].Address)
		require.Equal(t, []ethcommon.Hash{event.ID, peerKey}, logs[0].Topics)
		values, err := event.Inputs.NonIndexed().Unpack(logs[0].Data)
		require.NoError(t, err)
		require.Equal(t, []interface{}{family, modelDigest, runtimeDigest}, values)

		// Recreate both the executor and EVM state reader over the stored chain
		// context, proving the binding is not held in the precompile instance.
		ctx := h.stateDB.Ctx()
		app := testkeeper.EVMTestApp
		h.precompile, err = teeattestor.NewPrecompile(h.keeper, &app.EvmKeeper, &app.UpgradeKeeper, upgradePlan)
		require.NoError(t, err)
		h.resetEVM(ctx)
		require.Equal(t, []interface{}{family, modelDigest, runtimeDigest, uint64(activationHeight)}, readPeerBinding(h, peerKey))
	}
	for family := uint8(0); family < 4; family++ {
		peerKey[31] = family
		require.Equal(t, []interface{}{family, modelDigest, runtimeDigest, uint64(activationHeight)}, readPeerBinding(h, peerKey))
	}
}

func TestPeerBindingRefusesUnboundReportData(t *testing.T) {
	for _, field := range []string{"peerKey", "modelDigest", "runtimeDigest"} {
		t.Run(field, func(t *testing.T) {
			h := newHarness(t, 5)
			peerKey, modelDigest, runtimeDigest := [32]byte{0x11}, [32]byte{0x22}, [32]byte{0x33}
			envelope := peerEnvelope(h, 0, peerKey, modelDigest, runtimeDigest)
			switch field {
			case "peerKey":
				peerKey[31]++
			case "modelDigest":
				modelDigest[31]++
			case "runtimeDigest":
				runtimeDigest[31]++
			}
			ret, _, err := h.evm.Call(caller, h.precompile.Address(), h.input("bindPeer", uint8(0), envelope, peerKey, modelDigest, runtimeDigest), suppliedGas, uint256.NewInt(0))
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.ErrorIs(t, h.stateDB.GetPrecompileError(), agentcommon.ErrTEEReportDataMismatch)
			reason, err := abi.UnpackRevert(ret)
			require.NoError(t, err)
			require.Equal(t, "LXP_AGENT_TEE_REPORT_DATA_MISMATCH: report data mismatch", reason)
			require.Empty(t, h.stateDB.GetAllLogs())
			require.Equal(t, []interface{}{uint8(0), [32]byte{}, [32]byte{}, uint64(0)}, readPeerBinding(h, peerKey))
		})
	}
}

func TestPeerBindingRefusesUnknownFamilyAndInvalidEnvelope(t *testing.T) {
	for _, tc := range []struct {
		name     string
		family   uint8
		mutation func([]byte) []byte
		code     agentcommon.ErrorCode
	}{
		{"unknown", 4, func(b []byte) []byte { return b }, agentcommon.ErrTEEUnknownFamily},
		{"hardwareQuote", 0, func(b []byte) []byte { b[0] = 0; return b }, agentcommon.ErrTEEInvalidQuote},
		{"tamperedSignature", 0, func(b []byte) []byte { b[96+64] = 0; return b }, agentcommon.ErrTEEInvalidQuote},
		{"belowThreshold", 0, func(b []byte) []byte { b[95] = 2; return b[:96+2*65] }, agentcommon.ErrTEEUntrustedChain},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, 5)
			peerKey, modelDigest, runtimeDigest := [32]byte{0x11}, [32]byte{0x22}, [32]byte{0x33}
			envelope := tc.mutation(peerEnvelope(h, tc.family, peerKey, modelDigest, runtimeDigest))
			_, _, err := h.evm.Call(caller, h.precompile.Address(), h.input("bindPeer", tc.family, envelope, peerKey, modelDigest, runtimeDigest), suppliedGas, uint256.NewInt(0))
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.ErrorIs(t, h.stateDB.GetPrecompileError(), tc.code)
			require.Empty(t, h.stateDB.GetAllLogs())
			require.Equal(t, []interface{}{uint8(0), [32]byte{}, [32]byte{}, uint64(0)}, readPeerBinding(h, peerKey))
		})
	}
}

func TestPeerBindingWriteGuardsAndRollback(t *testing.T) {
	h := newHarness(t, 5)
	peerKey, modelDigest, runtimeDigest := [32]byte{0x11}, [32]byte{0x22}, [32]byte{0x33}
	envelope := peerEnvelope(h, 0, peerKey, modelDigest, runtimeDigest)
	input := h.input("bindPeer", uint8(0), envelope, peerKey, modelDigest, runtimeDigest)
	_, _, err := h.evm.StaticCall(caller, h.precompile.Address(), input, suppliedGas)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.ErrorIs(t, h.stateDB.GetPrecompileError(), agentcommon.ErrReadOnly)
	_, err = h.precompile.Run(h.evm, caller, h.precompile.Address(), input, nil, false, true, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.ErrorIs(t, h.stateDB.GetPrecompileError(), agentcommon.ErrDelegatecall)
	_, err = h.precompile.Run(h.evm, caller, h.precompile.Address(), input, uint256.NewInt(1).ToBig(), false, false, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.ErrorIs(t, h.stateDB.GetPrecompileError(), agentcommon.ErrNonPayable)
	require.Empty(t, h.stateDB.GetAllLogs())
	require.Equal(t, []interface{}{uint8(0), [32]byte{}, [32]byte{}, uint64(0)}, readPeerBinding(h, peerKey))

	snapshot := h.stateDB.Snapshot()
	_, _, err = h.evm.Call(caller, h.precompile.Address(), input, suppliedGas, uint256.NewInt(0))
	require.NoError(t, err)
	require.Equal(t, []interface{}{uint8(0), modelDigest, runtimeDigest, uint64(activationHeight)}, readPeerBinding(h, peerKey))
	h.stateDB.RevertToSnapshot(snapshot)
	require.Empty(t, h.stateDB.GetAllLogs())
	require.Equal(t, []interface{}{uint8(0), [32]byte{}, [32]byte{}, uint64(0)}, readPeerBinding(h, peerKey))

	h.resetEVM(h.stateDB.Ctx().WithBlockHeight(activationHeight - 1))
	_, _, err = h.evm.Call(caller, h.precompile.Address(), input, suppliedGas, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.ErrorIs(t, h.stateDB.GetPrecompileError(), agentcommon.ErrNotActive)
	require.Empty(t, h.stateDB.GetAllLogs())
}
