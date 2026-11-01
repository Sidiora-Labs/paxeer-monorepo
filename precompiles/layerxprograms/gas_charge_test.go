package layerxprograms_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"sort"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func gasWriteFixture(t *testing.T, name string) (*harness, []byte) {
	t.Helper()
	pack := func(h *harness, args ...interface{}) (*harness, []byte) {
		t.Helper()
		input, err := lp.ProgramsABI.Pack(name, args...)
		require.NoError(t, err)
		return h, input
	}
	switch name {
	case "deploy":
		h := newHarness(t)
		authority, _, err := lp.PrincipalOf(h.stateDB.Ctx(), h.executor.TestEVMKeeper(), h.caller)
		require.NoError(t, err)
		seedDeployMetering(t, h, authority.Principal)
		wasm := deployCounterWasm()
		return pack(h, sha256.Sum256([]byte(t.Name())), uint16(1), uint8(0), [32]byte{}, sha256.Sum256(wasm), []byte{}, wasm)
	case "upgrade":
		h, program, oldHash, wasm := newUpgradeProgram(t)
		hash := sha256.Sum256(wasm)
		return pack(h, program, uint16(1), uint8(0), oldHash, hash, []byte{}, upgradeInterface(hash), wasm, big.NewInt(0))
	case "call":
		h, args, _ := programsCallFixture(t)
		return pack(h, args...)
	case "sandboxExecute":
		h, args := gasSandboxExecuteFixture(t)
		return pack(h, args...)
	case "retireStorage":
		h := newHarness(t)
		request, _, _ := retirementRequest(t, h)
		return pack(h, request)
	case "registryRead":
		return pack(newHarness(t), [32]byte{0x71}, []byte("manifest"))
	case "transfer":
		h, program, legs := newProgramsTransferHarness(t)
		return pack(h, program, legs)
	case "registerAccount":
		h, program, asset := registerAccountFixture(t)
		return pack(h, program, uint8(1), asset, []byte("gas-vault"))
	case "setExitRoute":
		f := newWindDownFixture(t)
		return pack(f.h, f.program, f.account, f.asset, f.destination, f.seed)
	case "deprecate":
		h, program := tombstoneProgram(t)
		return pack(h, program, program, uint64(1000))
	case "tombstone":
		h, program := tombstoneProgram(t)
		payload, err := lp.EncodeDeprecatePayload(program, program, 100)
		require.NoError(t, err)
		receipt, err := tombstoneSubmit(t, h, h.stateDB.Ctx(), "deprecate", 0x00090007, payload)
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
		return pack(h, program)
	case "exitValue", "exitValueBounded":
		f := newExitValueFixture(t, 7)
		args := []interface{}{f.program, f.account}
		if name == "exitValueBounded" {
			args = append(args, big.NewInt(7))
		}
		return pack(f.h, args...)
	case "sandboxFund", "sandboxActivate":
		h := newHarness(t)
		f := newSandboxFundFixture(t, h)
		input := f.input(t, f.request)
		if name == "sandboxFund" {
			return h, input
		}
		output, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
		require.NoError(t, err, "sandboxFund returned %x", output)
		funded, err := h.host.ReadSandbox(h.stateDB.Ctx(), f.lease, 0)
		require.NoError(t, err)
		require.Equal(t, byte(1), funded[f.stateOffset])
		return pack(h, f.lease, sha256.Sum256(funded), uint64(3))
	default:
		t.Fatalf("write method %s has no gas fixture", name)
		return nil, nil
	}
}

func gasSandboxExecuteFixture(t *testing.T) (*harness, []interface{}) {
	t.Helper()
	h, callArgs, _ := programsCallFixture(t)
	authority, _, err := lp.PrincipalOf(h.stateDB.Ctx(), h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	wasm := []byte{
		0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127,
		3, 3, 2, 0, 1, 5, 3, 1, 0, 1, 7, 41, 3,
		11, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'c', 'a', 'l', 'l', 0, 0,
		14, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'r', 'e', 's', 'e', 'r', 'v', 'e', 0, 1,
		6, 'm', 'e', 'm', 'o', 'r', 'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
	}
	program, hash := sha256.Sum256([]byte(t.Name())), sha256.Sum256(wasm)
	iface := upgradeInterface(hash)
	prefix := []byte("LayerX/program-interface/v3\x00")
	copy(iface, prefix)
	binary.BigEndian.PutUint16(iface[len(prefix)+32:], 3)
	conservationCall(t, h, "deploy", program, uint16(3), uint8(1), authority.Principal, hash, iface, wasm)
	callArgs[0], callArgs[1] = program, uint16(3)
	payload, err := lp.EncodeCallPayload(program, 3, callArgs[2].(string), callArgs[3].([]byte),
		callArgs[4].([]byte), callArgs[5].([]byte), callArgs[6].(uint32), callArgs[7].([7]uint64))
	require.NoError(t, err)
	args, fundFeeLimit := prepareSandboxExecute(t, h, callArgs, payload)
	fundFeeLimit()
	args[9] = big.NewInt(100000000)
	return h, args
}

func TestProgramsWriteMethodsChargeGasTable(t *testing.T) {
	names := make([]string, 0)
	for name, method := range lp.ProgramsABI.Methods {
		row, ok := lp.GasTable[name]
		require.True(t, ok, "missing gas row for %s", name)
		write := method.StateMutability != "view" && method.StateMutability != "pure"
		require.Equal(t, write, row.Write, "gas row classification for %s", name)
		if write {
			names = append(names, name)
		}
	}
	require.Len(t, names, 15)
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			h, input := gasWriteFixture(t, name)
			method, err := lp.ProgramsABI.MethodById(input[:4])
			require.NoError(t, err)
			require.Equal(t, name, method.Name)
			require.Equal(t, lp.Address, h.precompile.Address())
			row := lp.GasTable[name]
			required := row.Gas(input[4:])
			require.Positive(t, required)
			require.Equal(t, required, h.precompile.RequiredGas(input))
			const supplied uint64 = 20000000
			require.Less(t, required, supplied)
			var payer, asset, treasury [32]byte
			var payerBefore, treasuryBefore sdk.Int
			if name == "call" || name == "sandboxExecute" {
				fee, err := h.host.ReadFeeSchedule(h.stateDB.Ctx())
				require.NoError(t, err)
				require.GreaterOrEqual(t, len(fee), 97)
				asset = [32]byte(fee[65:97])
				_, payer, err = lxphost.AssetAccountID(h.publicKey, asset)
				require.NoError(t, err)
				if name == "sandboxExecute" {
					args, err := method.Inputs.Unpack(input[4:])
					require.NoError(t, err)
					payer = args[3].([32]byte)
				}
				treasury, err = codec.DeriveAccountID([]byte("system:fees"))
				require.NoError(t, err)
				payerBefore = conservationBalance(t, h, payer, asset)
				treasuryBefore = conservationBalance(t, h, treasury, asset)
			}
			output, remaining, err := h.evm.Call(h.caller, h.precompile.Address(), input, supplied, uint256.NewInt(0))
			if err != nil {
				reason, decodeErr := abi.UnpackRevert(output)
				require.NoError(t, err, "%s reverted: %s (decode: %v)", name, reason, decodeErr)
			}
			require.NoError(t, h.stateDB.GetPrecompileError())
			require.Equal(t, required, supplied-remaining, "%s EVM gas used", name)
			if name == "call" || name == "sandboxExecute" {
				values, err := method.Outputs.Unpack(output)
				require.NoError(t, err)
				require.Equal(t, int32(0), values[0])
				require.Equal(t, uint8(1), values[1])
				charged := payerBefore.Sub(conservationBalance(t, h, payer, asset))
				require.True(t, charged.IsPositive(), "kernel must charge the fee asset")
				require.Equal(t, charged, conservationBalance(t, h, treasury, asset).Sub(treasuryBefore))
			}
		})
	}
	t.Run("kernel fee price does not change EVM gas", func(t *testing.T) {
		var firstInput []byte
		var firstGas uint64
		var firstFee sdk.Int
		for _, price := range []uint64{1, 2} {
			h, input := gasWriteFixture(t, "call")
			store := h.stateDB.Ctx().KVStore(h.key)
			fee, err := h.host.ReadFeeSchedule(h.stateDB.Ctx())
			require.NoError(t, err)
			fee = bytes.Clone(fee)
			for i := 0; i < 7; i++ {
				binary.BigEndian.PutUint64(fee[9+8*i:17+8*i], price)
			}
			store.Set(append([]byte{1, 0, 9}, []byte("progfee/active/v1")...), fee)
			store.Set(append([]byte{1, 0, 9}, binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)...), fee)
			asset := [32]byte(fee[65:97])
			_, payer, err := lxphost.AssetAccountID(h.publicKey, asset)
			require.NoError(t, err)
			before := conservationBalance(t, h, payer, asset)
			output, remaining, err := h.evm.Call(h.caller, h.precompile.Address(), input, 20000000, uint256.NewInt(0))
			require.NoError(t, err, "call returned %x", output)
			values, err := lp.ProgramsABI.Unpack("call", output)
			require.NoError(t, err)
			require.Equal(t, int32(0), values[0])
			used := uint64(20000000) - remaining
			require.Equal(t, lp.GasTable["call"].Gas(input[4:]), used)
			charged := before.Sub(conservationBalance(t, h, payer, asset))
			require.True(t, charged.IsPositive())
			if price == 1 {
				firstInput, firstGas, firstFee = input, used, charged
			} else {
				require.Equal(t, firstInput, input)
				require.Equal(t, firstGas, used)
				require.Equal(t, firstFee.MulRaw(2), charged)
			}
		}
	})
}
