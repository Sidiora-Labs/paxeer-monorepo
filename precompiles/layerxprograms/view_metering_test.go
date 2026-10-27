package layerxprograms_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func meteringViewRecord(version uint32, coefficients [9]uint64, activation uint64, authorityKind byte) []byte {
	record := binary.BigEndian.AppendUint32([]byte("LXMR1"), version)
	for _, coefficient := range coefficients {
		record = binary.BigEndian.AppendUint64(record, coefficient)
	}
	record = binary.BigEndian.AppendUint64(record, activation)
	record = append(record, authorityKind)
	return append(record, bytes.Repeat([]byte{0xa5}, 32)...)
}

func seedMeteringViews(t *testing.T, h *harness) ([9]uint64, [9]uint64) {
	t.Helper()
	genesis := [9]uint64{1, 1, 1, 1, 1, 8, 8, 64, 8}
	updated := [9]uint64{2, 17, 256, 65537, 1 << 32, 1 << 40, 1 << 48, 1 << 63, math.MaxUint64}
	store := h.stateDB.Ctx().KVStore(h.key)
	for _, schedule := range []struct {
		version    uint32
		words      [9]uint64
		activation uint64
		kind       byte
	}{{1, genesis, 1, 1}, {2, updated, 50, 2}} {
		record := meteringViewRecord(schedule.version, schedule.words, schedule.activation, schedule.kind)
		require.Len(t, record, 122)
		key := binary.BigEndian.AppendUint32([]byte("\x01\x00\x09progmet/history/v1/"), schedule.version)
		store.Set(key, record)
		if schedule.version == 2 {
			store.Set([]byte("\x01\x00\x09progmet/active/v1"), record)
		}
	}
	return genesis, updated
}

func TestProgramsViewMeteringSchedules(t *testing.T) {
	h := newHarness(t)
	genesis, updated := seedMeteringViews(t, h)
	before := h.dump(h.stateDB.Ctx())
	raw, err := h.host.ReadMeteringSchedule(h.stateDB.Ctx())
	require.NoError(t, err)
	require.Equal(t, meteringViewRecord(2, updated, 50, 2), raw)
	raw, err = h.host.ReadMeteringScheduleAt(h.stateDB.Ctx(), 1)
	require.NoError(t, err)
	require.Equal(t, meteringViewRecord(1, genesis, 1, 1), raw)

	for _, tc := range []struct {
		name string
		args []interface{}
		want []interface{}
	}{
		{"getCurrentMeteringSchedule", nil, []interface{}{uint32(2), updated}},
		{"getMeteringSchedule", []interface{}{uint32(1)}, []interface{}{genesis, uint8(1)}},
		{"getMeteringSchedule", []interface{}{uint32(2)}, []interface{}{updated, uint8(2)}},
	} {
		t.Run(fmt.Sprintf("%s/%v", tc.name, tc.args), func(t *testing.T) {
			input, err := lp.ProgramsABI.Pack(tc.name, tc.args...)
			require.NoError(t, err)
			for _, caller := range []common.Address{h.caller, common.HexToAddress("0x13277001")} {
				data, _, err := h.evm.StaticCall(caller, h.precompile.Address(), input, 2000000)
				require.NoError(t, err)
				require.Len(t, data, 10*32)
				values, err := lp.ProgramsABI.Methods[tc.name].Outputs.Unpack(data)
				require.NoError(t, err)
				require.Equal(t, tc.want, values)
				data, _, err = h.evm.Call(caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
				require.NoError(t, err)
				require.Len(t, data, 10*32)
				values, err = lp.ProgramsABI.Methods[tc.name].Outputs.Unpack(data)
				require.NoError(t, err)
				require.Equal(t, tc.want, values)
			}
		})
	}
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
}

func TestProgramsViewMeteringUnknownVersion(t *testing.T) {
	h := newHarness(t)
	seedMeteringViews(t, h)
	before := h.dump(h.stateDB.Ctx())
	for _, version := range []uint32{0, 3, math.MaxUint32} {
		input, err := lp.ProgramsABI.Pack("getMeteringSchedule", version)
		require.NoError(t, err)
		data, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, 2000000)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		reason, err := abi.UnpackRevert(data)
		require.NoError(t, err)
		require.Contains(t, reason, "LXP_NOT_FOUND:")
		require.ErrorIs(t, h.stateDB.GetPrecompileError(), lxphost.Code(-7))
	}
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
}
