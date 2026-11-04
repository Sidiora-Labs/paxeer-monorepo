package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepairM_1346_NullToPreallocated(t *testing.T) {
	original := NewCompactBitArray(9)
	for i := 0; i < original.Count(); i++ {
		require.True(t, original.SetIndex(i, true))
	}
	for _, direct := range []bool{false, true} {
		t.Run(fmt.Sprintf("direct=%t", direct), func(t *testing.T) {
			array := original.Copy()
			if direct {
				require.NoError(t, array.UnmarshalJSON([]byte("null")))
			} else {
				require.NoError(t, json.Unmarshal([]byte("null"), array))
			}
			require.Zero(t, array.Count())
			require.Zero(t, array.ExtraBitsStored)
			require.Nil(t, array.Elems)
			require.False(t, array.GetIndex(0))
			require.False(t, array.SetIndex(0, true))
			require.Zero(t, array.NumTrueBitsBefore(0))

			wire, err := json.Marshal(array)
			require.NoError(t, err)
			require.Equal(t, `""`, string(wire))
			var recovered *CompactBitArray
			require.NoError(t, json.Unmarshal(wire, &recovered))
			require.NotNil(t, recovered)
			require.True(t, array.Equal(recovered))

			wire, err = json.Marshal(original)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(wire, recovered))
			require.True(t, original.Equal(recovered))
			require.Equal(t, 9, recovered.NumTrueBitsBefore(recovered.Count()))
		})
	}
}

func TestRepairM_1346_EmptyJSON(t *testing.T) {
	for name, array := range map[string]*CompactBitArray{
		"nil":               nil,
		"constructor_empty": NewCompactBitArray(0),
		"zero_value":        {},
		"empty_slice":       {Elems: []byte{}},
	} {
		t.Run(name, func(t *testing.T) {
			want := `""`
			if array == nil {
				want = "null"
			}
			directWire, err := array.MarshalJSON()
			require.NoError(t, err)
			require.Equal(t, want, string(directWire))
			wire, err := json.Marshal(array)
			require.NoError(t, err)
			require.Equal(t, directWire, wire)

			allocated := NewCompactBitArray(16)
			require.True(t, allocated.SetIndex(15, true))
			require.NoError(t, json.Unmarshal(wire, allocated))
			require.Zero(t, allocated.Count())
			require.Nil(t, allocated.Elems)
			require.Zero(t, allocated.ExtraBitsStored)
			require.Zero(t, allocated.NumTrueBitsBefore(allocated.Count()))
			canonical, err := json.Marshal(allocated)
			require.NoError(t, err)
			require.Equal(t, `""`, string(canonical))
			require.NoError(t, json.Unmarshal(canonical, allocated))
			require.Zero(t, allocated.Count())

			pointer := NewCompactBitArray(8)
			require.NoError(t, json.Unmarshal(wire, &pointer))
			if array == nil {
				require.Nil(t, pointer)
			} else {
				require.NotNil(t, pointer)
				require.True(t, array.Equal(pointer))
			}
		})
	}
	t.Run("existing_round_trips", TestJSONMarshalUnmarshal)
}

func TestRepairM_1346_MalformedJSON(t *testing.T) {
	for _, wire := range []string{
		``, `"`, `"x`, `"X"`, `"-"`, `" x"`, `"x "`, `"x\n_"`,
		`"\u0078"`, `0`, `true`, `[]`, `{}`, `"x" "_"`,
	} {
		t.Run(wire, func(t *testing.T) {
			array := NewCompactBitArray(9)
			require.True(t, array.SetIndex(0, true))
			require.True(t, array.SetIndex(8, true))
			before := array.Copy()
			require.Error(t, array.UnmarshalJSON([]byte(wire)))
			require.True(t, before.Equal(array), "direct decoding mutated receiver")
			require.Error(t, json.Unmarshal([]byte(wire), array))
			require.True(t, before.Equal(array), "JSON decoding mutated receiver")
		})
	}
}

func TestRepairM_1346_TestGeneration(t *testing.T) {
	for size := 0; size <= 257; size++ {
		array, source := randCompactBitArray(t, size)
		var binarySource strings.Builder
		for _, value := range source {
			fmt.Fprintf(&binarySource, "%08b", value)
		}
		want := strings.NewReplacer("0", "_", "1", "x").Replace(binarySource.String())[:size]
		require.Equal(t, size, array.Count())
		wire, err := json.Marshal(array)
		require.NoError(t, err)
		if size == 0 {
			require.Equal(t, "null", string(wire))
		} else {
			require.Equal(t, `"`+want+`"`, string(wire), "size %d", size)
		}
		var recovered *CompactBitArray
		require.NoError(t, json.Unmarshal(wire, &recovered))
		require.Equal(t, size, recovered.Count())
		for index := 0; index <= size; index++ {
			count := strings.Count(want[:index], "x")
			require.Equal(t, count, array.NumTrueBitsBefore(index), "source size %d index %d", size, index)
			require.Equal(t, count, recovered.NumTrueBitsBefore(index), "decoded size %d index %d", size, index)
			if index < size {
				require.Equal(t, want[index] == 'x', array.GetIndex(index), "source size %d index %d", size, index)
				require.Equal(t, want[index] == 'x', recovered.GetIndex(index), "decoded size %d index %d", size, index)
			}
		}
	}
	t.Run("aligned_and_empty_counts", TestCompactBitArrayNumOfTrueBitsBefore)
	t.Run("set_index_results", TestCompactBitArrayGetSetIndex)
}
