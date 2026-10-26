package layerxprograms

import (
	"bytes"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func TestDecodeLegsTuple(t *testing.T) {
	maxAmount := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	want := []TransferLeg{
		{From: [32]byte{1, 2}, Asset: [32]byte{3, 4}, To: [32]byte{5, 6}, Amount: big.NewInt(0x10203)},
		{From: [32]byte{7, 8}, Asset: [32]byte{9, 10}, To: [32]byte{11, 12}, Amount: maxAmount},
	}
	for _, name := range []string{"transfer", "sandboxFund"} {
		t.Run(name, func(t *testing.T) {
			method := programsABI.Methods[name]
			args := []interface{}{[32]byte{13}, want}
			if name == "sandboxFund" {
				args = []interface{}{
					[32]byte{13}, [32]byte{14}, [32]byte{15}, [32]byte{16}, [32]byte{17},
					big.NewInt(18), uint64(19), uint32(20), [7]uint64{21}, []byte{22}, [32]byte{23}, want,
				}
			}
			packed, err := method.Inputs.Pack(args...)
			require.NoError(t, err)
			unpacked, err := method.Inputs.Unpack(packed)
			require.NoError(t, err)
			index := len(unpacked) - 1
			got, err := decodeLegs(unpacked[index])
			require.NoError(t, err)
			require.Equal(t, want, got)
			decoded, err := decodeArgs(&method, unpacked)
			require.NoError(t, err)
			require.Equal(t, want, decoded[index])
			sourceAmount := reflect.ValueOf(unpacked[index]).Index(0).FieldByName("Amount").Interface().(*big.Int)
			require.NotSame(t, sourceAmount, got[0].Amount)
			got[0].Amount.SetUint64(0)
			require.Equal(t, int64(0x10203), want[0].Amount.Int64())
			require.Equal(t, int64(0x10203), sourceAmount.Int64())
			for _, bad := range []*big.Int{new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(-1)} {
				reflect.ValueOf(unpacked[index]).Index(1).FieldByName("Amount").Set(reflect.ValueOf(bad))
				got, err := decodeLegs(unpacked[index])
				require.Nil(t, got)
				require.Error(t, err)
				require.True(t, strings.HasPrefix(err.Error(), "LXP_OVERFLOW:"))
				reason, unpackErr := abi.UnpackRevert(pcommon.LXPReason(err))
				require.NoError(t, unpackErr)
				require.Equal(t, "LXP_OVERFLOW: uint128 out of range; leg 1 amount", reason)
			}
			reflect.ValueOf(unpacked[index]).Index(1).FieldByName("Amount").Set(reflect.Zero(reflect.TypeOf((*big.Int)(nil))))
			_, err = decodeLegs(unpacked[index])
			require.Error(t, err)
			_, err = decodeArgs(&method, unpacked[:len(unpacked)-1])
			require.Error(t, err)
			reason, err := abi.UnpackRevert(pcommon.LXPReason(err))
			require.NoError(t, err)
			require.Equal(t, fmt.Sprintf("LXP_PRECOMPILE_ARG_COUNT: expected %d arguments but got %d", len(method.Inputs), len(unpacked)-1), reason)
			_, err = decodeArgs(&method, append(unpacked, nil))
			require.Error(t, err)
			reason, err = abi.UnpackRevert(pcommon.LXPReason(err))
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(reason, "LXP_PRECOMPILE_ARG_COUNT:"))
			args[len(args)-1] = []TransferLeg{}
			packed, err = method.Inputs.Pack(args...)
			require.NoError(t, err)
			unpacked, err = method.Inputs.Unpack(packed)
			require.NoError(t, err)
			got, err = decodeLegs(unpacked[index])
			require.NoError(t, err)
			require.Empty(t, got)
		})
	}
	for _, bad := range []interface{}{nil, [][32]byte{}, []interface{}{}, []struct{ Amount *big.Int }{{big.NewInt(1)}}} {
		_, err := decodeLegs(bad)
		require.Error(t, err)
	}
}

func TestDecodeBudgetAndFeeLimit(t *testing.T) {
	budget := [7]uint64{0, 1, 2, 3, 1 << 32, 1 << 63, ^uint64(0)}
	maxAmount := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	method := programsABI.Methods["call"]
	args := []interface{}{[32]byte{1}, uint16(2), "entry", []byte{3}, []byte{4}, []byte{5}, uint32(6), budget, maxAmount}
	packed, err := method.Inputs.Pack(args...)
	require.NoError(t, err)
	unpacked, err := method.Inputs.Unpack(packed)
	require.NoError(t, err)
	gotBudget, err := decodeBudget(unpacked[7])
	require.NoError(t, err)
	require.Equal(t, budget, gotBudget)
	gotFee, err := decodeU128(unpacked[8])
	require.NoError(t, err)
	require.Equal(t, bytes.Repeat([]byte{0xff}, 16), gotFee[:])
	decoded, err := decodeArgs(&method, unpacked)
	require.NoError(t, err)
	require.Equal(t, budget, decoded[7])
	require.Equal(t, gotFee, decoded[8])
	require.Equal(t, unpacked[:7], decoded[:7])
	for _, value := range []*big.Int{big.NewInt(0), big.NewInt(1), big.NewInt(0x10203), new(big.Int).Lsh(big.NewInt(1), 64), maxAmount} {
		encoded, err := decodeU128(value)
		require.NoError(t, err)
		require.Zero(t, value.Cmp(new(big.Int).SetBytes(encoded[:])))
	}
	encoded, err := decodeU128(big.NewInt(0x10203))
	require.NoError(t, err)
	require.Equal(t, [16]byte{13: 1, 14: 2, 15: 3}, encoded)
	for _, value := range []*big.Int{big.NewInt(-1), new(big.Int).Lsh(big.NewInt(1), 128), new(big.Int).Lsh(big.NewInt(1), 256)} {
		encoded, err := decodeU128(value)
		require.Error(t, err)
		require.Equal(t, [16]byte{}, encoded)
		reason, unpackErr := abi.UnpackRevert(pcommon.LXPReason(err))
		require.NoError(t, unpackErr)
		require.Equal(t, "LXP_OVERFLOW: uint128 out of range", reason)
		unpacked[8] = value
		_, err = decodeArgs(&method, unpacked)
		require.ErrorContains(t, err, "LXP_OVERFLOW:")
	}
	for _, bad := range []interface{}{nil, (*big.Int)(nil), big.Int{}, uint64(1), [16]byte{}} {
		_, err := decodeU128(bad)
		require.Error(t, err)
	}
	for _, bad := range []interface{}{nil, []uint64{1, 2, 3, 4, 5, 6, 7}, [6]uint64{}, [8]uint64{}, [7]int64{}} {
		_, err := decodeBudget(bad)
		require.Error(t, err)
	}
	for _, name := range []string{"call", "quoteCallFee"} {
		m := programsABI.Methods[name]
		for _, count := range []int{len(m.Inputs) - 1, len(m.Inputs) + 1} {
			_, err := decodeArgs(&m, make([]interface{}, count))
			require.Error(t, err)
			reason, unpackErr := abi.UnpackRevert(pcommon.LXPReason(err))
			require.NoError(t, unpackErr)
			require.True(t, strings.HasPrefix(reason, "LXP_PRECOMPILE_ARG_COUNT:"))
		}
	}
	_, err = decodeArgs(nil, nil)
	require.ErrorContains(t, err, "LXP_PRECOMPILE_UNKNOWN_METHOD:")
}
