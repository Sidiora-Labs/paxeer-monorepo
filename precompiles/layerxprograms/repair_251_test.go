package layerxprograms_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/big"
	"slices"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func repair251View(t *testing.T, h *harness, name string, args ...interface{}) []interface{} {
	t.Helper()
	input, err := lp.ProgramsABI.Pack(name, args...)
	require.NoError(t, err)
	before := h.dump(h.stateDB.Ctx())
	output, _, err := h.evm.StaticCall(h.caller, lp.Address, input, 20000000)
	reason, _ := abi.UnpackRevert(output)
	require.NoError(t, err, "%s: %s", name, reason)
	values, err := lp.ProgramsABI.Methods[name].Outputs.Unpack(output)
	require.NoError(t, err)
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	return values
}

func TestRepair_251_QuoteAsset(t *testing.T) {
	h := newHarness(t)
	budget := [7]uint64{19, 23, 29, 31, 37, 41, 43}
	activeKey := append([]byte{1, 0, 9}, []byte("progfee/active/v1")...)
	for iteration := 0; iteration < 2; iteration++ {
		if iteration == 1 {
			record, err := h.host.ReadFeeSchedule(h.stateDB.Ctx())
			require.NoError(t, err)
			record = bytes.Clone(record)
			binary.BigEndian.PutUint32(record[5:9], 2)
			for i := 0; i < 7; i++ {
				binary.BigEndian.PutUint64(record[9+i*8:17+i*8], uint64(i+2))
			}
			copy(record[65:97], bytes.Repeat([]byte{0x52}, 32))
			h.stateDB.Ctx().KVStore(h.key).Set(activeKey, record)
		}
		for reopen := 0; reopen < 2; reopen++ {
			if reopen == 1 {
				proofProgramsReopen(t, h)
			}
			record, err := h.host.ReadFeeSchedule(h.stateDB.Ctx())
			require.NoError(t, err)
			schedule := repair251View(t, h, "getCurrentFeeSchedule")
			quote := repair251View(t, h, "quoteCallFee", budget)
			require.Equal(t, schedule[0], quote[0])
			require.Equal(t, [32]byte(record[65:97]), schedule[2])
			require.Equal(t, schedule[2], quote[2], "feeAsset must select the active payment asset")
			require.NotEqual(t, sha256.Sum256(record), quote[2])
			ceiling := new(big.Int)
			for i, price := range schedule[1].([7]uint64) {
				ceiling.Add(ceiling, new(big.Int).Mul(new(big.Int).SetUint64(price), new(big.Int).SetUint64(budget[i])))
			}
			require.Equal(t, ceiling, quote[1])
		}
	}
	// The asset repair must not alter the existing uint128 arithmetic boundary.
	record, err := h.host.ReadFeeSchedule(h.stateDB.Ctx())
	require.NoError(t, err)
	for i := 0; i < 7; i++ {
		binary.BigEndian.PutUint64(record[9+i*8:17+i*8], math.MaxUint64)
	}
	binary.BigEndian.PutUint64(record[137:145], math.MaxUint64)
	h.stateDB.Ctx().KVStore(h.key).Set(activeKey, record)
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	quote := repair251View(t, h, "quoteCallFee", [7]uint64{math.MaxUint64, 2})
	require.Equal(t, maximum, quote[1])
	input, err := lp.ProgramsABI.Pack("quoteCallFee", [7]uint64{math.MaxUint64, 3})
	require.NoError(t, err)
	before := h.dump(h.stateDB.Ctx())
	output, _, err := h.evm.StaticCall(h.caller, lp.Address, input, 2000000)
	require.Error(t, err)
	reason, err := abi.UnpackRevert(output)
	require.NoError(t, err)
	require.Contains(t, reason, "LXP_OVERFLOW")
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
}

func TestRepair_251_ValueFrozen(t *testing.T) {
	h, program, asset := registerAccountFixture(t)
	seed := []byte("repair-251-vault")
	conservationCall(t, h, "registerAccount", program, uint8(2), asset, seed)
	account, err := h.host.ReadProgramAccount(h.stateDB.Ctx(), program, asset, seed)
	require.NoError(t, err)
	accountKey := append([]byte{6}, account.AccountID[:]...)
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	for _, tc := range []struct {
		name    string
		balance *big.Int
		frozen  bool
	}{
		{"unfrozen_zero", new(big.Int), false},
		{"unfrozen_funded", big.NewInt(10), false},
		{"frozen_max_uint128", maximum, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Alter only balance/freeze fields in the real registration's stored account.
			row := bytes.Clone(h.stateDB.Ctx().KVStore(h.key).Get(accountKey))
			require.GreaterOrEqual(t, len(row), 40)
			balanceOffset := 4 + 32 + 4 + int(binary.BigEndian.Uint32(row[36:40])) + 1
			freezeOffset := balanceOffset + 16 + 4 + 32 + 1 + 8 + 8
			require.Greater(t, len(row), freezeOffset)
			tc.balance.FillBytes(row[balanceOffset : balanceOffset+16])
			row[freezeOffset] = 0
			if tc.frozen {
				row[freezeOffset] = 1
			}
			h.stateDB.Ctx().KVStore(h.key).Set(accountKey, row)
			for reopen := 0; reopen < 2; reopen++ {
				if reopen == 1 {
					proofProgramsReopen(t, h)
				}
				value, err := h.host.ReadValueAccount(h.stateDB.Ctx(), program, account.AccountID)
				require.NoError(t, err)
				require.Equal(t, tc.balance, value.Balance.BigInt())
				require.Equal(t, tc.frozen, value.Frozen)
				values := repair251View(t, h, "getValueAccount", program, account.AccountID)
				encodedBalance := tc.balance.FillBytes(make([]byte, 16))
				require.Equal(t, []interface{}{new(big.Int).SetBytes(encodedBalance), tc.frozen}, values)
			}
		})
	}
	missing := [32]byte{0xff}
	_, err = h.host.ReadValueAccount(h.stateDB.Ctx(), program, missing)
	require.ErrorIs(t, err, lxphost.Code(-7))
	require.Equal(t, []interface{}{new(big.Int).SetBytes(make([]byte, 16)), false}, repair251View(t, h, "getValueAccount", program, missing))
	proofProgramsReopen(t, h)
	require.Equal(t, []interface{}{new(big.Int).SetBytes(make([]byte, 16)), false}, repair251View(t, h, "getValueAccount", program, missing))
}

// The guest is executable Wasm: it publishes 64 bytes and transfers seven units
// through the actual host import. No return, receipt or log is constructed here.
func repair251Responder(t *testing.T) (*harness, []interface{}, []byte, []byte) {
	t.Helper()
	h, args, _ := programsCallFixture(t)
	// Install the production callWithWebRequests dispatch, also used when no
	// x-web request keeper is attached to an ordinary Programs call.
	lp.ConfigureXWeb(h.executor, nil)
	authority, _, err := lp.PrincipalOf(h.stateDB.Ctx(), h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	fee, err := h.host.ReadFeeSchedule(h.stateDB.Ctx())
	require.NoError(t, err)
	asset := [32]byte(fee[65:97])
	destination, err := codec.DeriveAccountID([]byte("system:fees"))
	require.NoError(t, err)
	guest := make([]byte, 64)
	for i := range guest {
		guest[i] = byte(i)
	}
	uleb := func(out []byte, value uint32) []byte {
		for value >= 128 {
			out = append(out, byte(value)|128)
			value >>= 7
		}
		return append(out, byte(value))
	}
	section := func(id byte, body []byte) []byte { return append(uleb([]byte{id}, uint32(len(body))), body...) }
	label := func(value string) []byte { return append([]byte{byte(len(value))}, value...) }
	constant := func(out []byte, opcode byte, value uint32) []byte {
		out = append(out, opcode)
		for value >= 64 {
			out = append(out, byte(value)|128)
			value >>= 7
		}
		return append(out, byte(value))
	}
	wasm := []byte{0, 97, 115, 109, 1, 0, 0, 0}
	wasm = append(wasm, section(1, []byte{4,
		0x60, 3, 0x7f, 0x7f, 0x7f, 1, 0x7f,
		0x60, 6, 0x7e, 0x7e, 0x7f, 0x7f, 0x7f, 0x7f, 1, 0x7f,
		0x60, 1, 0x7f, 1, 0x7f, 0x60, 2, 0x7f, 0x7f, 1, 0x7f})...)
	imports := append([]byte{2}, label("layerx_v2")...)
	imports = append(imports, label("response_write")...)
	imports = append(imports, 0, 0)
	imports = append(imports, label("layerx_v1")...)
	imports = append(imports, label("transfer_402")...)
	wasm = append(wasm, section(2, append(imports, 0, 1))...)
	wasm = append(wasm, section(3, []byte{2, 2, 3})...)
	wasm = append(wasm, section(5, []byte{1, 1, 1, 1})...)
	exports := append([]byte{3}, label("layerx_reserve")...)
	exports = append(exports, 0, 2)
	exports = append(exports, label("layerx_call")...)
	exports = append(exports, 0, 3)
	exports = append(exports, label("memory")...)
	wasm = append(wasm, section(7, append(exports, 2, 0))...)
	reserve := []byte{0, 0x41, 0x80, 2, 0x0b}
	body := constant([]byte{0}, 0x42, 0)
	body = constant(body, 0x42, 7)
	body = constant(body, 0x41, 64)
	body = constant(body, 0x41, 32)
	body = constant(body, 0x41, 96)
	body = constant(body, 0x41, 32)
	body = append(body, 0x10, 1, 0x04, 0x40, 0x00, 0x0b)
	body = constant(body, 0x41, 0)
	body = constant(body, 0x41, 0)
	body = constant(body, 0x41, uint32(len(guest)))
	body = append(body, 0x10, 0, 0x1a, 0x41, 0, 0x0b)
	code := append(uleb([]byte{2}, uint32(len(reserve))), reserve...)
	code = append(uleb(code, uint32(len(body))), body...)
	wasm = append(wasm, section(10, code)...)
	data := append(bytes.Clone(guest), asset[:]...)
	data = append(data, destination[:]...)
	wasm = append(wasm, section(11, append(uleb([]byte{1, 0, 0x41, 0, 0x0b}, uint32(len(data))), data...))...)
	program, codeHash := sha256.Sum256([]byte(t.Name())), sha256.Sum256(wasm)
	iface := upgradeInterface(codeHash)
	prefix := []byte("LayerX/program-interface/v3\x00")
	copy(iface, prefix)
	binary.BigEndian.PutUint16(iface[len(prefix)+32:], 3)
	// One declared transfer requirement, followed by the unchanged empty lists.
	iface = iface[:len(iface)-6]
	iface = append(iface, 0, 1, 6)
	iface = append(iface, asset[:]...)
	iface = append(iface, destination[:]...)
	iface = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(iface, 0), 7)
	iface = append(iface, 0, 0, 0, 0)
	conservationCall(t, h, "deploy", program, uint16(3), uint8(1), authority.Principal, codeHash, iface, wasm)
	args[0], args[1], args[4] = program, uint16(3), append(append([]byte{0, 1, 5}, asset[:]...), destination[:]...)
	args[4] = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(args[4].([]byte), 0), 7)
	payload, err := lp.EncodeCallPayload(program, 3, args[2].(string), args[3].([]byte), args[4].([]byte), args[5].([]byte), 64, args[7].([7]uint64))
	require.NoError(t, err)
	h.stateDB.ResetForTracer()
	return h, args, payload, guest
}

func repair251Receipt(t *testing.T, h *harness, args []interface{}, payload []byte) (lxphost.Receipt, sdk.Context) {
	t.Helper()
	reference, _ := h.stateDB.Ctx().CacheContext()
	method := lp.ProgramsABI.Methods["call"]
	var limit [16]byte
	args[8].(*big.Int).FillBytes(limit[:])
	receipt, err := lp.Submit(reference, h.executor, lp.NewCallContext(h.caller, &method), 0x00090003, payload, limit)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	require.NotNil(t, receipt.ProgramOutcome)
	require.Equal(t, uint8(1), receipt.ProgramOutcome.Kind)
	require.Len(t, receipt.Transfers, 1)
	return receipt, reference
}

func repair251TransferRoot(t *testing.T, receipt lxphost.Receipt) [32]byte {
	t.Helper()
	// Read the independent producer's terminal event, never the returned frame.
	for _, event := range receipt.Events {
		if event.Code == 7 {
			require.Len(t, event.Body, 255)
			require.Equal(t, "LXMO\x02\x02", string(event.Body[:6]))
			return [32]byte(event.Body[127:159])
		}
	}
	t.Fatal("host did not produce terminal evidence")
	return [32]byte{}
}

func repair251AssertResponse(t *testing.T, output []byte, receipt lxphost.Receipt, guest []byte) {
	t.Helper()
	root := repair251TransferRoot(t, receipt)
	require.NotEqual(t, [32]byte{}, root)
	decoded, err := lp.DecodeProgramExecutionResult(output, 64, root)
	require.NoError(t, err)
	require.Zero(t, decoded.TerminalResult)
	require.Equal(t, uint8(1), decoded.TerminalKind)
	require.Equal(t, guest, decoded.GuestResponse)
	require.Greater(t, len(decoded.TerminalFrame), 64)
	require.Equal(t, receipt.ProgramOutcome.Response, decoded.TerminalFrame)
	require.Len(t, decoded.AppliedLegs, 115)
	require.True(t, bytes.HasPrefix(decoded.Execution, []byte("LXP/program-execution-with-transfer-authority/v2\x00")))
	leg := decoded.AppliedLegs
	require.Equal(t, receipt.Transfers[0].From[:], leg[1:33])
	require.Equal(t, receipt.Transfers[0].To[:], leg[33:65])
	require.Equal(t, receipt.Transfers[0].Asset[:], leg[65:97])
	require.Equal(t, receipt.Transfers[0].Amount.BigInt(), new(big.Int).SetBytes(leg[97:113]))
	_, err = lp.DecodeProgramExecutionResult(output, 63, root)
	require.Error(t, err, "guest capacity must still be enforced")
	_, err = lp.DecodeProgramExecutionResult(output, 1_048_577, root)
	require.Error(t, err)
	wrongRoot := root
	wrongRoot[0] ^= 1
	_, err = lp.DecodeProgramExecutionResult(output, 64, wrongRoot)
	require.Error(t, err, "transfer evidence must be independently bound")
	for _, index := range []int{0, 32, 63, 95, 127, len(output) - 1} {
		bad := bytes.Clone(output)
		bad[index] ^= 0x80
		_, err = lp.DecodeProgramExecutionResult(bad, 64, root)
		require.Error(t, err, "mutation at %d", index)
	}
	for _, bad := range [][]byte{output[:len(output)-1], append(bytes.Clone(output), make([]byte, 32)...)} {
		_, err = lp.DecodeProgramExecutionResult(bad, 64, root)
		require.Error(t, err)
	}
	// Mutate only real producer frames; canonical ABI packing cannot hide malformed inner framing.
	for _, mutate := range []func([]byte) []byte{
		func(frame []byte) []byte { return append(frame, 0) },
		func(frame []byte) []byte {
			frame[len("LXP/programs/terminal-applied-legs/v1\x00")] = 0xff
			return frame
		},
		func(frame []byte) []byte { frame[len(frame)-1] ^= 1; return frame },
		func(frame []byte) []byte {
			at := len("LXP/programs/terminal-applied-legs/v1\x00") + 4
			// The first inner word is the authority domain, so mutate its spelling.
			frame[at] ^= 1
			return frame
		},
		func(frame []byte) []byte {
			at := len("LXP/programs/terminal-applied-legs/v1\x00")
			executionLength := int(binary.BigEndian.Uint32(frame[at : at+4]))
			// Mismatch the authority wrapper's root without changing applied legs.
			frame[at+4+executionLength-1] ^= 1
			return frame
		},
	} {
		frame := mutate(bytes.Clone(decoded.TerminalFrame))
		bad, err := lp.ProgramsABI.Methods["call"].Outputs.Pack(int32(0), uint8(1), frame)
		require.NoError(t, err)
		_, err = lp.DecodeProgramExecutionResult(bad, 64, root)
		require.Error(t, err)
	}
	// Decoder ownership must not allow a consumer to mutate the transport bytes.
	before := bytes.Clone(output)
	decoded.TerminalFrame[0] ^= 1
	decoded.Execution[0] ^= 1
	decoded.AppliedLegs[0] ^= 1
	decoded.GuestResponse[0] ^= 1
	require.Equal(t, before, output)
}

func TestRepair_251_CallResponse(t *testing.T) {
	h, args, payload, guest := repair251Responder(t)
	proofProgramsReopen(t, h)
	receipt, reference := repair251Receipt(t, h, args, payload)
	input, err := lp.ProgramsABI.Pack("call", args...)
	require.NoError(t, err)
	output, _, err := h.evm.Call(h.caller, lp.Address, input, 20000000, uint256.NewInt(0))
	reason, _ := abi.UnpackRevert(output)
	require.NoError(t, err, "%s", reason)
	repair251AssertResponse(t, output, receipt, guest)
	require.Equal(t, h.dump(reference), h.dump(h.stateDB.Ctx()))
	logs := h.stateDB.GetAllLogs()
	require.Len(t, logs, 1)
	require.Equal(t, lp.Address, logs[0].Address)
	require.Equal(t, []common.Hash{lp.ProgramsABI.Events["ProgramCallOutcome"].ID, common.Hash(args[0].([32]byte))}, logs[0].Topics)
	values, err := lp.ProgramsABI.Events["ProgramCallOutcome"].Inputs.NonIndexed().Unpack(logs[0].Data)
	require.NoError(t, err)
	require.Equal(t, repair251TransferRoot(t, receipt), values[9])
	proofProgramsReopen(t, h)
	repair251AssertResponse(t, output, receipt, guest)
	t.Run("legacy_execution", func(t *testing.T) {
		legacy, legacyArgs, legacyPayload := programsCallFixture(t)
		lp.ConfigureXWeb(legacy.executor, nil)
		proofProgramsReopen(t, legacy)
		legacyInput, err := lp.ProgramsABI.Pack("call", legacyArgs...)
		require.NoError(t, err)
		legacyOutput, terminal := call(t, legacy, legacyInput, legacyPayload, legacyArgs[8].(*big.Int))
		decoded, err := lp.DecodeProgramExecutionResult(legacyOutput, 64, [32]byte{})
		require.NoError(t, err)
		require.Equal(t, terminal, decoded.TerminalFrame)
		require.Empty(t, decoded.AppliedLegs)
		require.Empty(t, decoded.GuestResponse)
	})
}

func repair251Stores(ctx sdk.Context) map[string]map[string][]byte {
	stores := make(map[string]map[string][]byte)
	for _, key := range ctx.MultiStore().StoreKeys() {
		switch key := key.(type) {
		case *sdk.KVStoreKey:
			stores[key.Name()] = simulationStoreDump(ctx.KVStore(key))
		case *sdk.TransientStoreKey:
			stores[key.Name()] = simulationStoreDump(ctx.TransientStore(key))
		}
	}
	return stores
}

func TestRepair_251_SimulationResponse(t *testing.T) {
	h, args, payload, guest := repair251Responder(t)
	proofProgramsReopen(t, h)
	receipt, reference := repair251Receipt(t, h, args, payload)
	before := repair251Stores(h.stateDB.Ctx())
	events := slices.Clone(h.stateDB.Ctx().EventManager().Events())
	input, err := lp.ProgramsABI.Pack("simulateCall", args...)
	require.NoError(t, err)
	var simulated []byte
	for repeat := 0; repeat < 2; repeat++ {
		if repeat == 1 {
			proofProgramsReopen(t, h)
		}
		output, _, err := h.evm.StaticCall(h.caller, lp.Address, input, 20000000)
		reason, _ := abi.UnpackRevert(output)
		require.NoError(t, err, "%s", reason)
		repair251AssertResponse(t, output, receipt, guest)
		if repeat == 0 {
			simulated = bytes.Clone(output)
		} else {
			require.Equal(t, simulated, output)
		}
		require.Equal(t, before, repair251Stores(h.stateDB.Ctx()))
		require.Equal(t, events, h.stateDB.Ctx().EventManager().Events())
		require.Empty(t, h.stateDB.GetAllLogs())
	}
	callInput, err := lp.ProgramsABI.Pack("call", args...)
	require.NoError(t, err)
	called, _, err := h.evm.Call(h.caller, lp.Address, callInput, 20000000, uint256.NewInt(0))
	require.NoError(t, err)
	require.Equal(t, simulated, called)
	require.Equal(t, h.dump(reference), h.dump(h.stateDB.Ctx()))
}
