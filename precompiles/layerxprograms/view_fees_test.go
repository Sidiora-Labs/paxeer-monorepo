package layerxprograms

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func feeViewHost(t *testing.T) (*lxphost.Host, sdk.Context, *sdk.KVStoreKey) {
	t.Helper()
	key := sdk.NewKVStoreKey("programs-fee-views")
	ctx := testutil.DefaultContext(key, sdk.NewTransientStoreKey("programs-fee-transient"))
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: key, NetworkID: 7, ParameterVersion: 1, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	return host, ctx, key
}

func feeViewCall(t *testing.T, ctx sdk.Context, host *lxphost.Host, name string, args ...interface{}) ([]byte, error) {
	t.Helper()
	method := programsABI.Methods[name]
	input, err := method.Inputs.Pack(args...)
	require.NoError(t, err)
	decoded, err := method.Inputs.Unpack(input)
	require.NoError(t, err)
	return (&PrecompileExecutor{host: host}).Execute(ctx, &method, common.Address{}, Address, decoded, big.NewInt(0), true, nil, nil)
}

func feeViewKey(name string) []byte {
	return append([]byte{1, 0, 9}, []byte(name)...)
}

func feeViewSnapshot(ctx sdk.Context, key sdk.StoreKey) map[string][]byte {
	values := map[string][]byte{}
	iterator := ctx.KVStore(key).Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		values[string(iterator.Key())] = bytes.Clone(iterator.Value())
	}
	return values
}

func feeViewRecord(version uint32, prices [7]uint64, asset [32]byte, demand [6]uint64) []byte {
	encoded := binary.BigEndian.AppendUint32([]byte("LXFR1"), version)
	for _, value := range prices {
		encoded = binary.BigEndian.AppendUint64(encoded, value)
	}
	encoded = append(encoded, asset[:]...)
	for _, value := range demand {
		encoded = binary.BigEndian.AppendUint64(encoded, value)
	}
	for _, value := range []uint64{0x102030405060708, 0x203040506070809, 0x30405060708090a} {
		encoded = binary.BigEndian.AppendUint64(encoded, value)
	}
	encoded = append(encoded, bytes.Repeat([]byte{0xd7}, 32)...)
	encoded = binary.BigEndian.AppendUint64(encoded, 0x405060708090a0b)
	return binary.BigEndian.AppendUint64(encoded, math.MaxUint64)
}

func feeViewPendingRecord(prices [7]uint64, asset [32]byte, demand [6]uint64) []byte {
	encoded := []byte("LXFP1")
	for _, value := range prices {
		encoded = binary.BigEndian.AppendUint64(encoded, value)
	}
	encoded = append(encoded, asset[:]...)
	for _, value := range demand {
		encoded = binary.BigEndian.AppendUint64(encoded, value)
	}
	for _, value := range []uint64{math.MaxUint64, 0x102030405060708, 0x203040506070809} {
		encoded = binary.BigEndian.AppendUint64(encoded, value)
	}
	return append(encoded, bytes.Repeat([]byte{0xe9}, 32)...)
}

func TestProgramsViewCurrentFeeSchedule(t *testing.T) {
	host, ctx, key := feeViewHost(t)
	prices := [7]uint64{math.MaxUint64, 1 << 63, 0x102030405060708, 13, 19, 23, 29}
	asset := [32]byte{0: 0xa5, 17: 0xb6, 31: 0xc7}
	demand := [6]uint64{1001, 7, 2, 9, 5, math.MaxUint64}
	active := feeViewRecord(0xabcdef01, prices, asset, demand)
	require.Len(t, active, 217)
	historicalPrices := [7]uint64{31, 37, 41, 43, 47, 53, 59}
	historicalAsset := [32]byte{0: 0x76, 31: 0x98}
	historyKey := binary.BigEndian.AppendUint32(feeViewKey("progfee/history/v1/"), 0x12345678)
	ctx.KVStore(key).Set(feeViewKey("progfee/active/v1"), active)
	ctx.KVStore(key).Set(historyKey, feeViewRecord(0x12345678, historicalPrices, historicalAsset, demand))
	before := feeViewSnapshot(ctx, key)

	t.Run("current record and ABI", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			out, err := feeViewCall(t, ctx, host, "getCurrentFeeSchedule")
			require.NoError(t, err)
			require.Len(t, out, 9*32)
			fields, err := programsABI.Methods["getCurrentFeeSchedule"].Outputs.Unpack(out)
			require.NoError(t, err)
			require.Equal(t, []interface{}{uint32(0xabcdef01), prices, asset}, fields)
		}
		record, err := decodeFeeScheduleRecord(active)
		require.NoError(t, err)
		require.Equal(t, demand, record.Demand)
		require.Equal(t, uint64(0x102030405060708), record.ActivationBatch)
		require.Equal(t, uint64(0x203040506070809), record.LastOccupancyBatch)
		require.Equal(t, uint64(0x30405060708090a), record.GovernanceSequence)
		require.Equal(t, bytes.Repeat([]byte{0xd7}, 32), record.GovernanceReceiptDigest[:])
		require.Equal(t, uint64(0x405060708090a0b), binary.BigEndian.Uint64(record.ObservedOccupancy[:8]))
		require.Equal(t, uint64(math.MaxUint64), binary.BigEndian.Uint64(record.ObservedOccupancy[8:]))
	})

	t.Run("versioned history through host reader", func(t *testing.T) {
		reader, ok := any(host).(feeScheduleHistoryReader)
		require.True(t, ok, "wiring.J52.8 must expose Host.ReadFeeScheduleAt(ctx, version) to read retained history")
		encoded, err := reader.ReadFeeScheduleAt(ctx, 0x12345678)
		require.NoError(t, err)
		require.Equal(t, feeViewRecord(0x12345678, historicalPrices, historicalAsset, demand), encoded)
		out, err := feeViewCall(t, ctx, host, "getFeeSchedule", uint32(0x12345678))
		require.NoError(t, err)
		require.Len(t, out, 8*32)
		fields, err := programsABI.Methods["getFeeSchedule"].Outputs.Unpack(out)
		require.NoError(t, err)
		require.Equal(t, []interface{}{historicalPrices, historicalAsset}, fields)
		for _, version := range []uint32{0x12345679, 0xabcdef01} {
			out, err = feeViewCall(t, ctx, host, "getFeeSchedule", version)
			require.ErrorIs(t, err, lxphost.Code(-7))
			require.Nil(t, out)
			reason, unpackErr := abi.UnpackRevert(pcommon.LXPReason(err))
			require.NoError(t, unpackErr)
			require.Contains(t, reason, "LXP_NOT_FOUND")
		}
		out, err = feeViewCall(t, ctx, host, "getFeeSchedule", uint32(0))
		require.ErrorIs(t, err, lxphost.Code(-3))
		require.Nil(t, out)

		branch, _ := ctx.CacheContext()
		branch.KVStore(key).Set(historyKey, active)
		require.PanicsWithValue(t, lxphost.Code(-1002), func() {
			_, _ = feeViewCall(t, branch, host, "getFeeSchedule", uint32(0x12345678))
		})
	})

	t.Run("metering history reader", func(t *testing.T) {
		branch, _ := ctx.CacheContext()
		encoded := []byte("LXMR1")
		encoded = binary.BigEndian.AppendUint32(encoded, 0x12345678)
		for _, coefficient := range [9]uint64{1, 1, 1, 1, 1, 8, 8, 64, 8} {
			encoded = binary.BigEndian.AppendUint64(encoded, coefficient)
		}
		encoded = binary.BigEndian.AppendUint64(encoded, 2)
		encoded = append(encoded, 2)
		encoded = append(encoded, bytes.Repeat([]byte{0xaa}, 32)...)
		require.Len(t, encoded, 122)
		branch.KVStore(key).Set(binary.BigEndian.AppendUint32(feeViewKey("progmet/history/v1/"), 0x12345678), encoded)
		row, err := host.ReadMeteringScheduleAt(branch, 0x12345678)
		require.NoError(t, err)
		require.Equal(t, encoded, row)
		row, err = host.ReadMeteringScheduleAt(branch, 0x12345679)
		require.ErrorIs(t, err, lxphost.Code(-7))
		require.Nil(t, row)
	})

	t.Run("reject malformed records", func(t *testing.T) {
		_, err := decodeFeeScheduleRecord(nil)
		require.Equal(t, lxphost.Code(-3), err)
		for _, encoded := range [][]byte{active[:216], append(bytes.Clone(active), 0), append([]byte("LXFR2"), active[5:]...)} {
			branch, _ := ctx.CacheContext()
			branch.KVStore(key).Set(feeViewKey("progfee/active/v1"), encoded)
			out, err := feeViewCall(t, branch, host, "getCurrentFeeSchedule")
			require.Error(t, err)
			require.Nil(t, out)
		}
		for _, bounds := range [][2]int{{5, 9}, {9, 17}, {65, 97}, {97, 105}, {145, 153}, {161, 169}, {169, 201}} {
			invalid := bytes.Clone(active)
			clear(invalid[bounds[0]:bounds[1]])
			_, err := decodeFeeScheduleRecord(invalid)
			require.Equal(t, lxphost.Code(-1002), err)
		}
	})

	t.Run("validate view arguments", func(t *testing.T) {
		for _, name := range []string{"getCurrentFeeSchedule", "getFeeSchedule", "getPendingFeeSchedule"} {
			method := programsABI.Methods[name]
			for _, args := range [][]interface{}{{uint32(1), uint32(2)}, {}} {
				if len(args) == len(method.Inputs) {
					continue
				}
				out, err := (&PrecompileExecutor{host: host}).Execute(ctx, &method, common.Address{}, Address, args, big.NewInt(0), true, nil, nil)
				require.Error(t, err)
				require.Nil(t, out)
			}
		}
		method := programsABI.Methods["getFeeSchedule"]
		out, err := (&PrecompileExecutor{host: host}).Execute(ctx, &method, common.Address{}, Address, []interface{}{uint64(1)}, big.NewInt(0), true, nil, nil)
		require.ErrorContains(t, err, "LXP_NON_CANONICAL")
		require.Nil(t, out)
	})

	for _, name := range []string{"getCurrentFeeSchedule", "getFeeSchedule"} {
		t.Run(name+" dispatch", func(t *testing.T) {
			method := programsABI.Methods[name]
			var args []interface{}
			if name == "getFeeSchedule" {
				args = []interface{}{uint32(0x12345678)}
			}
			out, err := (&PrecompileExecutor{host: host}).Execute(ctx, &method, common.Address{}, Address, args, big.NewInt(0), true, nil, nil)
			require.NoError(t, err, "the real executor must serve fee views under STATICCALL")
			require.NotEmpty(t, out)
		})
	}
	require.Equal(t, before, feeViewSnapshot(ctx, key))
}

func TestProgramsViewPendingFeeSchedule(t *testing.T) {
	host, ctx, key := feeViewHost(t)
	prices := [7]uint64{math.MaxUint64, 61, 67, 71, 73, 79, 83}
	asset := [32]byte{0: 0x81, 19: 0x92, 31: 0xa3}
	demand := [6]uint64{0x102030405060708, 13, 3, 11, 17, math.MaxUint64}
	pending := feeViewPendingRecord(prices, asset, demand)
	require.Len(t, pending, 197)

	t.Run("missing pending reverts with LXP_NOT_FOUND", func(t *testing.T) {
		out, err := feeViewCall(t, ctx, host, "getPendingFeeSchedule")
		require.Nil(t, out)
		require.ErrorIs(t, err, lxphost.Code(-7))
		reason, unpackErr := abi.UnpackRevert(pcommon.LXPReason(err))
		require.NoError(t, unpackErr)
		require.Equal(t, "LXP_NOT_FOUND: no pending fee schedule: LXP_ERR_UNKNOWN_FIELD", reason)
	})
	ctx.KVStore(key).Set(feeViewKey("progfee/pending/v1"), pending)
	before := feeViewSnapshot(ctx, key)
	t.Run("pending record and ABI", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			out, err := feeViewCall(t, ctx, host, "getPendingFeeSchedule")
			require.NoError(t, err)
			require.Len(t, out, 15*32)
			fields, err := programsABI.Methods["getPendingFeeSchedule"].Outputs.Unpack(out)
			require.NoError(t, err)
			require.Equal(t, []interface{}{prices, asset, demand, uint64(math.MaxUint64)}, fields)
		}
		record, err := decodePendingFeeScheduleRecord(pending)
		require.NoError(t, err)
		require.Equal(t, uint64(0x102030405060708), record.StagedBatch)
		require.Equal(t, uint64(0x203040506070809), record.GovernanceSequence)
		require.Equal(t, bytes.Repeat([]byte{0xe9}, 32), record.GovernanceReceiptDigest[:])
	})
	t.Run("reject malformed pending records", func(t *testing.T) {
		for _, encoded := range [][]byte{nil, pending[:196], append(bytes.Clone(pending), 0), append([]byte("LXFP2"), pending[5:]...)} {
			_, err := decodePendingFeeScheduleRecord(encoded)
			require.Equal(t, lxphost.Code(-3), err)
		}
		for _, bounds := range [][2]int{{5, 13}, {61, 93}, {93, 101}, {141, 149}, {149, 157}, {157, 165}, {165, 197}} {
			invalid := bytes.Clone(pending)
			clear(invalid[bounds[0]:bounds[1]])
			_, err := decodePendingFeeScheduleRecord(invalid)
			require.Equal(t, lxphost.Code(-1002), err)
		}
	})
	t.Run("real executor dispatch", func(t *testing.T) {
		method := programsABI.Methods["getPendingFeeSchedule"]
		out, err := (&PrecompileExecutor{host: host}).Execute(ctx, &method, common.Address{}, Address, nil, big.NewInt(0), true, nil, nil)
		require.NoError(t, err, "the real executor must serve pending fees under STATICCALL")
		require.NotEmpty(t, out)
	})
	require.Equal(t, before, feeViewSnapshot(ctx, key))
}
