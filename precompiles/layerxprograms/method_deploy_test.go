package layerxprograms_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func deployCounterWasm() []byte {
	wasm := []byte{0, 0x61, 0x73, 0x6d, 1, 0, 0, 0}
	section := func(id byte, body []byte) {
		wasm = append(wasm, id, byte(len(body)))
		wasm = append(wasm, body...)
	}
	section(1, []byte{
		3, 0x60, 6, 0x7e, 0x7e, 0x7f, 0x7f, 0x7f, 0x7f, 1, 0x7f,
		0x60, 1, 0x7f, 1, 0x7f, 0x60, 2, 0x7f, 0x7f, 1, 0x7f,
	})
	section(2, []byte("\x01\x09layerx_v1\x0ctransfer_402\x00\x00"))
	section(3, []byte{2, 1, 2})
	section(5, []byte{1, 1, 1, 1})
	section(7, []byte("\x03\x0elayerx_reserve\x00\x01\x0blayerx_call\x00\x02\x06memory\x02\x00"))
	section(10, []byte{
		2, 4, 0, 0x41, 0, 0x0b,
		21, 0, 0x42, 0, 0x42, 1,
		0x41, 0x80, 1, 0x41, 32, 0x41, 0xa0, 1, 0x41, 32,
		0x10, 0, 0x1a, 0x41, 0, 0x0b,
	})
	data := []byte{1, 0, 0x41, 0x80, 1, 0x0b, 64}
	data = append(data, bytes.Repeat([]byte{9}, 32)...)
	data = append(data, bytes.Repeat([]byte{10}, 32)...)
	section(11, data)
	return wasm
}

func deployInterface(codeHash [32]byte) []byte {
	encoding := append([]byte("LayerX/program-interface/v1\x00"), codeHash[:]...)
	encoding = binary.BigEndian.AppendUint16(encoding, 1)
	encoding = binary.BigEndian.AppendUint16(encoding, 1)
	encoding = binary.BigEndian.AppendUint16(encoding, 11)
	encoding = append(encoding, []byte("layerx_call")...)
	encoding = append(encoding, 0, 0, 0, 0, 1, 0x20, 0, 0, 0, 64, 1, 0x20, 0, 0, 0, 64)
	encoding = append(encoding, 0, 1, 6)
	encoding = append(encoding, bytes.Repeat([]byte{9}, 32)...)
	encoding = append(encoding, bytes.Repeat([]byte{10}, 32)...)
	encoding = append(encoding, make([]byte, 15)...)
	encoding = append(encoding, 1, 0, 0, 0, 0)
	return encoding
}

func seedDeployMetering(t *testing.T, h *harness, principal [32]byte) {
	t.Helper()
	metering := binary.BigEndian.AppendUint32([]byte("LXMR1"), 1)
	for _, coefficient := range []uint64{1, 1, 1, 1, 1, 8, 8, 64, 8} {
		metering = binary.BigEndian.AppendUint64(metering, coefficient)
	}
	metering = binary.BigEndian.AppendUint64(metering, 1)
	metering = append(metering, 1)
	metering = append(metering, principal[:]...)
	store := h.stateDB.Ctx().KVStore(h.key)
	store.Set(append([]byte{1, 0, 9}, []byte("progmet/active/v1")...), metering)
	history := binary.BigEndian.AppendUint32([]byte("progmet/history/v1/"), 1)
	store.Set(append([]byte{1, 0, 9}, history...), metering)
}

func TestProgramsDeployRegistersProgram(t *testing.T) {
	method := lp.ProgramsABI.Methods["deploy"]
	require.Len(t, method.Inputs, 7)
	require.Empty(t, method.Outputs)
	for _, input := range method.Inputs {
		require.NotEqual(t, "feeLimit", input.Name)
	}
	for _, policy := range []uint8{0, 1} {
		t.Run(map[uint8]string{0: "immutable", 1: "upgradeable with interface"}[policy], func(t *testing.T) {
			h := newHarness(t)
			ctx := h.stateDB.Ctx()
			authority, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
			require.NoError(t, err)
			seedDeployMetering(t, h, authority.Principal)
			programID := sha256.Sum256([]byte(t.Name()))
			wasm := deployCounterWasm()
			codeHash := sha256.Sum256(wasm)
			var upgradeAuthority [32]byte
			var iface []byte
			if policy == 1 {
				upgradeAuthority = authority.Principal
				iface = deployInterface(codeHash)
			}
			payload, err := lp.EncodeDeployPayload(programID, 1, policy, upgradeAuthority, codeHash, iface, wasm)
			require.NoError(t, err)
			reference, _ := ctx.CacheContext()
			accountSequence, err := h.host.ReadIdentitySequence(ctx, did)
			require.NoError(t, err)
			lastSequence, err := h.host.ReadLastSequence(ctx, did)
			require.NoError(t, err)
			var selector [4]byte
			copy(selector[:], method.ID)
			timestamp := uint64(ctx.BlockTime().UnixMilli())
			receipt, err := h.host.Execute(lxphost.WithSequence(reference, lastSequence+1), lxphost.Activity{
				ProtocolVersion: 3, Type: 0x00090001, ActorDID: did, AuthorityKey: h.publicKey,
				AccountSequence: accountSequence, NotBefore: timestamp, NotAfter: timestamp,
				IdempotencyKey: lp.IdempotencyKey([32]byte(ctx.EVMTxHash()), 0, selector),
				FeeLimit:       [16]byte{}, Payload: payload,
			}, authority)
			require.NoError(t, err)
			require.Zero(t, receipt.ResultCode)
			input, err := lp.ProgramsABI.Pack("deploy", programID, uint16(1), policy, upgradeAuthority, codeHash, iface, wasm)
			require.NoError(t, err)
			output, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
			require.NoError(t, err)
			require.Empty(t, output)
			require.Equal(t, h.dump(reference), h.dump(h.stateDB.Ctx()))
			record, err := h.host.ReadProgram(h.stateDB.Ctx(), programID)
			require.NoError(t, err)
			require.Equal(t, lxphost.ProgramRecord{
				Policy: policy, Authority: upgradeAuthority, CodeHash: codeHash,
				ABIVersion: 1, Version: 1, Active: true,
			}, record)
			if len(iface) != 0 {
				stored, err := h.host.ReadProgramInterface(h.stateDB.Ctx(), programID)
				require.NoError(t, err)
				require.Equal(t, lxphost.ProgramInterface{Version: 1, Digest: sha256.Sum256(iface), Encoding: iface}, stored)
			}
			logs := h.stateDB.GetAllLogs()
			require.Len(t, logs, 1)
			require.Equal(t, lp.Address, logs[0].Address)
			event := lp.ProgramsABI.Events["ProgramDeployed"]
			require.Equal(t, []common.Hash{event.ID, common.Hash(programID)}, logs[0].Topics)
			values, err := event.Inputs.NonIndexed().Unpack(logs[0].Data)
			require.NoError(t, err)
			require.Equal(t, []interface{}{codeHash}, values)
		})
	}
}

func TestProgramsDeployRefusesCodeHashMismatch(t *testing.T) {
	h := newHarness(t)
	ctx := h.stateDB.Ctx()
	authority, _, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	seedDeployMetering(t, h, authority.Principal)
	programID := sha256.Sum256([]byte(t.Name()))
	wasm := deployCounterWasm()
	codeHash := sha256.Sum256(wasm)
	codeHash[0] ^= 1
	before := h.dump(ctx)
	input, err := lp.ProgramsABI.Pack("deploy", programID, uint16(1), uint8(0), [32]byte{}, codeHash, []byte{}, wasm)
	require.NoError(t, err)
	output, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(output)
	require.NoError(t, err)
	require.Equal(t, lp.KernelError(lxphost.Code(-104), "deploy").Error(), reason)
	require.Contains(t, reason, "LXP_PAYLOAD_HASH_MISMATCH")
	require.ErrorIs(t, h.stateDB.GetPrecompileError(), lxphost.Code(-104))
	_, err = h.host.ReadProgram(h.stateDB.Ctx(), programID)
	require.ErrorIs(t, err, lxphost.Code(-7))
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	require.Empty(t, h.stateDB.GetAllLogs())
}
