package layerxpay

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"reflect"
	"testing"

	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type precompileServiceViewCase struct {
	name string
	id   [32]byte
	want interface{}
}

func precompileServiceID(seed byte) [32]byte {
	var id [32]byte
	for i := range id {
		id[i] = seed + byte(i)
	}
	return id
}

func precompileServiceWire(t *testing.T, size int, fields ...interface{}) []byte {
	t.Helper()
	var buffer bytes.Buffer
	buffer.WriteByte(1)
	for _, field := range fields {
		require.NoError(t, binary.Write(&buffer, binary.BigEndian, field))
	}
	require.Equal(t, size, buffer.Len())
	return buffer.Bytes()
}

func seedPrecompileServiceViews(t *testing.T, h *payPrecompileHarness) []precompileServiceViewCase {
	t.Helper()
	id := precompileServiceID
	u64 := func(value uint64) uint64 { return value }
	genesis := h.keeper.ExportGenesis(h.db.Ctx())
	genesis.Params = paytypes.Params{EscrowEnabled: true, BudgetEnabled: true, StreamEnabled: true, ServiceEnabled: true}
	put := func(prefix string, identifier [32]byte, value []byte) {
		t.Helper()
		key := make([]byte, 40)
		copy(key, prefix)
		copy(key[8:], identifier[:])
		genesis.KernelRecords = append(genesis.KernelRecords, kv.Pair{
			Key: paytypes.KernelKVKey(paytypes.ServiceModuleID, key), Value: value,
		})
	}
	priceBytes := [16]byte{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0xff}
	price := new(big.Int).SetBytes(priceBytes[:])
	put("offer::1", id(1), precompileServiceWire(t, 252,
		id(1), id(11), id(12), id(13), priceBytes, id(14), id(15),
		u64(12345), u64(23456), u64(34567), uint8(2), u64(45678), u64(101), true, false))
	contested := [][32]byte{id(18), id(19)}
	put("agree::1", id(2), precompileServiceWire(t, 311,
		id(2), id(1), id(12), id(16), id(14), id(17), u64(12345), u64(23456), u64(34567),
		uint8(2), uint8(7), u64(102), uint16(0x1234), true, u64(103), u64(45678), uint8(2), contested))
	put("commit:1", id(3), precompileServiceWire(t, 220,
		id(3), id(20), id(12), id(2), id(21), id(17), u64(12345), u64(23456), u64(104), true, uint16(0x2345)))
	var signature [64]byte
	for i := range signature {
		signature[i] = byte(i + 1)
	}
	execution := precompileServiceWire(t, 417,
		id(4), id(22), id(2), id(3), id(23), id(24), id(25), u64(12345), u64(23456), u64(34567),
		id(26), id(27), id(28), signature, u64(105))
	put("execut:1", id(4), execution)
	executionOutput := append([]byte("LXP:SERVICE:ATTEST:v1"), execution[1:345]...)
	executionOutput = append(executionOutput, signature[:]...)
	deliverables := []interface{}{
		[]interface{}{id(29), u64(0x1122334455667788), id(30)},
		[]interface{}{id(31), u64(0x8877665544332211), id(32)},
	}
	putDelivery := func(identifier, agreement, provider [32]byte, sequence uint64) {
		t.Helper()
		put("delivr:1", identifier, precompileServiceWire(t, 138,
			identifier, id(33), agreement, provider, uint8(2), sequence))
		for i, fields := range [][]interface{}{
			{id(29), u64(0x1122334455667788), id(30)},
			{id(31), u64(0x8877665544332211), id(32)},
		} {
			key := append(append([]byte("delitm:1"), identifier[:]...), byte(i))
			genesis.KernelRecords = append(genesis.KernelRecords, kv.Pair{
				Key:   paytypes.KernelKVKey(paytypes.ServiceModuleID, key),
				Value: precompileServiceWire(t, 73, fields...),
			})
		}
	}
	putDelivery(id(5), id(2), id(12), 106)
	putDelivery(id(90), id(2), id(91), 105)
	putDelivery(id(92), id(93), id(94), 999)
	putProgress := func(identifier, commitment [32]byte, progress uint32, sequence uint64) {
		t.Helper()
		put("progrs:1", identifier, precompileServiceWire(t, 245,
			identifier, id(34), commitment, id(2), id(12), id(35), id(36), progress, u64(12345), sequence))
	}
	putProgress(id(6), id(3), 8000, 107)
	putProgress(id(95), id(3), 4000, 999)
	putProgress(id(96), id(97), 10000, 1000)
	evidence := [][32]byte{id(38), id(39)}
	put("disput:1", id(7), precompileServiceWire(t, 249,
		id(7), id(37), id(2), id(16), u64(108), true, uint16(0x3456), uint32(6789), id(40), u64(109), uint8(2), evidence))
	require.NoError(t, h.keeper.InitGenesis(h.db.Ctx(), *genesis))
	return []precompileServiceViewCase{
		{"getOffer", id(1), []interface{}{id(1), id(11), id(12), id(13), price, id(14), id(15),
			u64(12345), u64(23456), u64(34567), uint8(2), u64(45678), u64(101), true, false}},
		{"getAgreement", id(2), []interface{}{id(2), id(1), id(12), id(16), id(14), id(17),
			u64(12345), u64(23456), u64(34567), uint8(2), uint8(7), u64(102), uint16(0x1234), contested,
			true, u64(103), u64(45678)}},
		{"getCommitment", id(3), []interface{}{id(3), id(20), id(12), id(2), id(21),
			u64(12345), u64(23456), id(17), u64(104), true, uint16(0x2345)}},
		{"getExecution", id(4), executionOutput},
		{"getDelivery", id(5), []interface{}{id(2), id(12), deliverables, u64(106)}},
		{"latestDelivery", id(2), []interface{}{id(2), id(12), deliverables, u64(106)}},
		{"getProgress", id(6), []interface{}{id(6), id(34), id(3), id(2), id(12), id(35), id(36),
			uint32(8000), u64(12345), u64(107)}},
		{"progressHighWater", id(3), uint32(8000)},
		{"getDispute", id(7), []interface{}{id(7), id(37), id(2), id(16), evidence,
			u64(108), true, uint16(0x3456), uint32(6789), id(40), u64(109)}},
	}
}

func precompileServiceTupleValues(value interface{}) interface{} {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Struct {
		fields := make([]interface{}, v.NumField())
		for i := range fields {
			fields[i] = precompileServiceTupleValues(v.Field(i).Interface())
		}
		return fields
	}
	if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Struct {
		items := make([]interface{}, v.Len())
		for i := range items {
			items[i] = precompileServiceTupleValues(v.Index(i).Interface())
		}
		return items
	}
	return value
}

func assertPrecompileServiceViews(t *testing.T, h *payPrecompileHarness, cases []precompileServiceViewCase) {
	t.Helper()
	a := h.precompile.GetABI()
	executor := h.precompile.GetExecutor().(*Executor)
	before := h.keeper.ExportGenesis(h.db.Ctx())
	eventsBefore := h.db.Ctx().EventManager().Events()
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			method := a.Methods[test.name]
			require.True(t, method.IsConstant())
			input, err := a.Pack(test.name, test.id)
			require.NoError(t, err)
			row, ok := methodGas[test.name]
			require.True(t, ok)
			require.False(t, row.Write)
			require.Zero(t, row.Surcharge)
			gas := row.Gas(input[4:])
			require.Equal(t, uint64(1096), gas)
			require.Equal(t, gas, h.precompile.RequiredGas(input))
			require.Equal(t, gas, executor.RequiredGas(input[4:], &method))
			require.Equal(t, gas+12, RequiredGas(input))
			direct, handled, err := executor.serviceViewHandlers(h.db.Ctx(), &method, []interface{}{test.id})
			require.True(t, handled)
			require.NoError(t, err)
			decoded, err := method.Outputs.Unpack(direct)
			require.NoError(t, err)
			require.Len(t, decoded, 1)
			require.Equal(t, test.want, precompileServiceTupleValues(decoded[0]))
			for _, caller := range []common.Address{h.caller, common.HexToAddress("0xbad2288")} {
				output, left, err := h.evm.StaticCall(caller, h.precompile.Address(), input, gas)
				require.NoError(t, err)
				require.Zero(t, left)
				require.Equal(t, direct, output)
			}
			output, err := h.precompile.Run(h.evm, h.caller, h.caller, input, big.NewInt(0), false, false, nil)
			require.NoError(t, err)
			require.Equal(t, direct, output)
			_, left, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, gas-1)
			require.ErrorIs(t, err, vm.ErrOutOfGas)
			require.Zero(t, left)
			for _, unknown := range [][32]byte{{}, precompileServiceID(200)} {
				input, err := a.Pack(test.name, unknown)
				require.NoError(t, err)
				output, left, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, gas)
				require.Zero(t, left)
				if test.name == "progressHighWater" {
					require.NoError(t, err)
					values, err := method.Outputs.Unpack(output)
					require.NoError(t, err)
					require.Equal(t, []interface{}{uint32(0)}, values)
				} else {
					require.ErrorIs(t, err, vm.ErrExecutionReverted)
					reason, err := abi.UnpackRevert(output)
					require.NoError(t, err)
					require.Contains(t, reason, "LXP_UNKNOWN_FIELD:")
				}
			}
		})
	}
	require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	require.Equal(t, eventsBefore, h.db.Ctx().EventManager().Events())
	require.Empty(t, h.db.GetAllLogs())
}

func TestPrecompileServiceViews(t *testing.T) {
	h := newPayPrecompileHarness(t)
	cases := seedPrecompileServiceViews(t, h)
	require.Len(t, cases, 9)
	assertPrecompileServiceViews(t, h, cases)
}

func TestPrecompileServiceViewsUngatedWhenPaused(t *testing.T) {
	h := newPayPrecompileHarness(t)
	cases := seedPrecompileServiceViews(t, h)
	genesis := h.keeper.ExportGenesis(h.db.Ctx())
	genesis.Params = paytypes.Params{Paused: true}
	for _, module := range []uint16{paytypes.EscrowModuleID, paytypes.BudgetModuleID, paytypes.StreamModuleID, paytypes.ServiceModuleID} {
		key, err := paytypes.KernelModuleEnableKey(module)
		require.NoError(t, err)
		found := false
		for i := range genesis.KernelRecords {
			if bytes.Equal(genesis.KernelRecords[i].Key, key) {
				genesis.KernelRecords[i].Value = make([]byte, 32)
				found = true
			}
		}
		require.True(t, found)
	}
	require.NoError(t, h.keeper.InitGenesis(h.db.Ctx(), *genesis))
	require.Equal(t, paytypes.Params{Paused: true}, h.keeper.GetParams(h.db.Ctx()))
	for _, test := range cases {
		require.NoError(t, h.keeper.CheckWrite(h.db.Ctx(), test.name))
	}
	require.ErrorIs(t, h.keeper.CheckWrite(h.db.Ctx(), "serviceOfferPublish"), paytypes.ErrFamilyDisabled)
	assertPrecompileServiceViews(t, h, cases)
}
