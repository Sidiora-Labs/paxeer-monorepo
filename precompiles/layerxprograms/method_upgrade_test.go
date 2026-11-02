package layerxprograms_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func upgradeInterface(codeHash [32]byte) []byte {
	encoding := append([]byte("LayerX/program-interface/v1\x00"), codeHash[:]...)
	encoding = binary.BigEndian.AppendUint16(encoding, 1)
	encoding = binary.BigEndian.AppendUint16(encoding, 1)
	encoding = binary.BigEndian.AppendUint16(encoding, 11)
	encoding = append(encoding, []byte("layerx_call")...)
	return append(encoding, 0, 0, 0, 0, 1, 0x20, 0, 0, 0, 64, 1, 0x20, 0, 0, 0, 64, 0, 0, 0, 0, 0, 0)
}

func newUpgradeProgram(t *testing.T) (*harness, [32]byte, [32]byte, []byte) {
	t.Helper()
	h := newHarness(t)
	ctx := h.stateDB.Ctx()
	authority, _, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	metering := binary.BigEndian.AppendUint32([]byte("LXMR1"), 1)
	for _, coefficient := range []uint64{1, 1, 1, 1, 1, 8, 8, 64, 8} {
		metering = binary.BigEndian.AppendUint64(metering, coefficient)
	}
	metering = binary.BigEndian.AppendUint64(metering, 1)
	metering = append(metering, 1)
	metering = append(metering, authority.Principal[:]...)
	store := ctx.KVStore(h.key)
	store.Set(append([]byte{1, 0, 9}, []byte("progmet/active/v1")...), metering)
	store.Set(append([]byte{1, 0, 9}, binary.BigEndian.AppendUint32([]byte("progmet/history/v1/"), 1)...), metering)
	wasm := []byte{
		0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127,
		3, 3, 2, 0, 1, 5, 3, 1, 0, 1, 7, 41, 3,
		11, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'c', 'a', 'l', 'l', 0, 0,
		14, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'r', 'e', 's', 'e', 'r', 'v', 'e', 0, 1,
		6, 'm', 'e', 'm', 'o', 'r', 'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
	}
	program := sha256.Sum256([]byte(t.Name()))
	oldHash := sha256.Sum256(wasm)
	payload, err := lp.EncodeDeployPayload(program, 1, 1, authority.Principal, oldHash, upgradeInterface(oldHash), wasm)
	require.NoError(t, err)
	method := lp.ProgramsABI.Methods["deploy"]
	receipt, err := lp.Submit(ctx, h.executor, lp.NewCallContext(h.caller, &method), 0x00090001, payload, [16]byte{})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	record, err := h.host.ReadProgram(ctx, program)
	require.NoError(t, err)
	require.Equal(t, oldHash, record.CodeHash)
	require.Equal(t, uint32(1), record.Version)
	newWasm := append(wasm, 0, 2, 1, 'v')
	return h, program, oldHash, newWasm
}

func TestProgramsUpgradeReplacesCodeHash(t *testing.T) {
	for _, clearInterface := range []bool{false, true} {
		name := "replace interface"
		if clearInterface {
			name = "clear interface"
		}
		t.Run(name, func(t *testing.T) {
			h, program, oldHash, wasm := newUpgradeProgram(t)
			newHash := sha256.Sum256(wasm)
			iface, flags := upgradeInterface(newHash), uint8(0)
			if clearInterface {
				iface, flags = nil, 2
			}
			ctx := h.stateDB.Ctx()
			before := h.dump(ctx)
			for _, limit := range []*big.Int{big.NewInt(9), new(big.Int).Lsh(big.NewInt(1), 64), new(big.Int).Lsh(big.NewInt(1), 127)} {
				input, err := lp.ProgramsABI.Pack("upgrade", program, uint16(1), flags, oldHash, newHash, []byte{}, iface, wasm, limit)
				require.NoError(t, err)
				data, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 4000000, uint256.NewInt(0))
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				reason, err := abi.UnpackRevert(data)
				require.NoError(t, err)
				require.Equal(t, lp.KernelError(lxphost.Code(-602), "upgrade").Error(), reason)
				require.Equal(t, before, h.dump(h.stateDB.Ctx()))
				require.Empty(t, h.stateDB.GetAllLogs())
			}
			input, err := lp.ProgramsABI.Pack("upgrade", program, uint16(1), flags, oldHash, newHash, []byte{}, iface, wasm, big.NewInt(0))
			require.NoError(t, err)
			data, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 4000000, uint256.NewInt(0))
			require.NoError(t, err)
			require.Empty(t, data)
			logs := h.stateDB.GetAllLogs()
			require.Len(t, logs, 1)
			event := lp.ProgramsABI.Events["ProgramUpgraded"]
			require.Equal(t, lp.Address, logs[0].Address)
			require.Equal(t, []common.Hash{event.ID, common.Hash(program)}, logs[0].Topics)
			values, err := event.Inputs.NonIndexed().Unpack(logs[0].Data)
			require.NoError(t, err)
			require.Equal(t, []interface{}{oldHash, newHash}, values)
			record, err := h.host.ReadProgram(h.stateDB.Ctx(), program)
			require.NoError(t, err)
			require.Equal(t, newHash, record.CodeHash)
			require.Equal(t, uint32(2), record.Version)
			storedInterface, err := h.host.ReadProgramInterface(h.stateDB.Ctx(), program)
			if clearInterface {
				require.ErrorIs(t, err, lxphost.Code(-7))
			} else {
				require.NoError(t, err)
				require.Equal(t, iface, storedInterface.Encoding)
				require.Equal(t, uint32(2), storedInterface.Version)
			}
			input, err = lp.ProgramsABI.Pack("getProgram", program)
			require.NoError(t, err)
			data, _, err = h.evm.StaticCall(h.caller, h.precompile.Address(), input, 4000000)
			require.NoError(t, err)
			values, err = lp.ProgramsABI.Unpack("getProgram", data)
			require.NoError(t, err)
			require.Equal(t, []interface{}{record.Policy, record.Authority, newHash, record.ABIVersion, uint32(2), true}, values)
		})
	}
}

func TestProgramsUpgradeRefusesStaleOldHash(t *testing.T) {
	h, program, oldHash, wasm := newUpgradeProgram(t)
	newHash := sha256.Sum256(wasm)
	staleHash := [32]byte(bytes.Repeat([]byte{0xff}, 32))
	before := h.dump(h.stateDB.Ctx())
	input, err := lp.ProgramsABI.Pack("upgrade", program, uint16(1), uint8(0), staleHash, newHash, []byte{}, upgradeInterface(newHash), wasm, big.NewInt(0))
	require.NoError(t, err)
	data, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 4000000, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(data)
	require.NoError(t, err)
	require.Equal(t, lp.KernelError(lxphost.Code(-213), "upgrade").Error(), reason)
	require.ErrorIs(t, h.stateDB.GetPrecompileError(), lxphost.Code(-213))
	require.Empty(t, h.stateDB.GetAllLogs())
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	record, err := h.host.ReadProgram(h.stateDB.Ctx(), program)
	require.NoError(t, err)
	require.Equal(t, oldHash, record.CodeHash)
	require.Equal(t, uint32(1), record.Version)
	storedInterface, err := h.host.ReadProgramInterface(h.stateDB.Ctx(), program)
	require.NoError(t, err)
	require.Equal(t, upgradeInterface(oldHash), storedInterface.Encoding)
	input, err = lp.ProgramsABI.Pack("getProgram", program)
	require.NoError(t, err)
	data, _, err = h.evm.StaticCall(h.caller, h.precompile.Address(), input, 4000000)
	require.NoError(t, err)
	values, err := lp.ProgramsABI.Unpack("getProgram", data)
	require.NoError(t, err)
	require.Equal(t, []interface{}{record.Policy, record.Authority, oldHash, record.ABIVersion, uint32(1), true}, values)
}
