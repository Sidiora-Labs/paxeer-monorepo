package json_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/jsontypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
)

func TestRepairM_1091_ConcreteExample(t *testing.T) {
	type Record struct {
		Height int64  `json:"height"`
		Data   []byte `json:"data"`
	}
	input := Record{Height: 9007199254740993, Data: []byte{1, 2, 3}}
	wire, err := json.Marshal(input)
	require.NoError(t, err)
	require.Equal(t, `{"height":"9007199254740993","data":"AQID"}`, string(wire))
	var output Record
	require.NoError(t, json.Unmarshal(wire, &output))
	require.Equal(t, input, output)

	indented, err := json.MarshalIndent(input, "", "  ")
	require.NoError(t, err)
	require.Equal(t, "{\n  \"height\": \"9007199254740993\",\n  \"data\": \"AQID\"\n}", string(indented))
	var fromIndented Record
	require.NoError(t, json.Unmarshal(indented, &fromIndented))
	require.Equal(t, input, fromIndented)
	// Passing a concrete value through any does not request interface decoding.
	var dynamic any = input
	dynamicWire, err := json.Marshal(dynamic)
	require.NoError(t, err)
	require.Equal(t, wire, dynamicWire)
}

func TestRepairM_1091_EncodingRules(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input any
		wire  string
		want  any
	}{
		{"int", int(64), `"64"`, int(64)},
		{"uint", uint(64), `"64"`, uint(64)},
		{"int64", int64(-9223372036854775808), `"-9223372036854775808"`, int64(-9223372036854775808)},
		{"uint64", uint64(18446744073709551615), `"18446744073709551615"`, uint64(18446744073709551615)},
		{"int32", int32(32), `32`, int32(32)},
		{"uint32", uint32(32), `32`, uint32(32)},
		{"byte slice", []byte{1, 2, 3}, `"AQID"`, []byte{1, 2, 3}},
		{"byte array", [3]byte{1, 2, 3}, `"AQID"`, [3]byte{1, 2, 3}},
		{"integer slice", []int64{1, 2, 3}, `["1","2","3"]`, []int64{1, 2, 3}},
		{"nil slice", []int64(nil), `null`, []int64(nil)},
		{"empty slice", []int64{}, `[]`, []int64(nil)},
		{"nil map", map[string]int64(nil), `{}`, map[string]int64{}},
		{"empty map", map[string]int64{}, `{}`, map[string]int64{}},
		{"integer map", map[string]int64{"a": 1}, `{"a":"1"}`, map[string]int64{"a": 1}},
		{"UTC time", time.Date(2020, 6, 8, 16, 21, 28, 123, time.FixedZone("UTC+2", 2*60*60)), `"2020-06-08T14:21:28.000000123Z"`, time.Date(2020, 6, 8, 14, 21, 28, 123, time.UTC)},
		{"zero time", time.Time{}, `"0001-01-01T00:00:00Z"`, time.Time{}},
		{"nil pointer", (*time.Time)(nil), `null`, (*time.Time)(nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire, err := json.Marshal(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.wire, string(wire))
			output := reflect.New(reflect.TypeOf(tc.input))
			require.NoError(t, json.Unmarshal(wire, output.Interface()))
			require.Equal(t, tc.want, output.Elem().Interface())
		})
	}
}

func TestRepairM_1091_ConcreteOnly(t *testing.T) {
	input := struct {
		Wheels int64 `json:"wheels"`
	}{Wheels: 4}
	wire, err := json.Marshal(input)
	require.NoError(t, err)
	var output any
	require.Error(t, json.Unmarshal(wire, &output))
	output = input
	require.Error(t, json.Unmarshal(wire, &output))
	require.Error(t, json.Unmarshal(wire, input))

	for _, value := range []any{
		struct{ Value any }{Value: input},
		[]any{input},
		map[string]any{"value": input},
	} {
		_, err := json.Marshal(value)
		require.Error(t, err)
	}
	for _, tc := range []struct {
		producer    any
		destination any
	}{
		{struct{ Value int64 }{Value: 4}, new(struct{ Value any })},
		{[]int64{4}, new([]any)},
		{map[string]int64{"value": 4}, new(map[string]any)},
	} {
		wire, err := json.Marshal(tc.producer)
		require.NoError(t, err)
		require.Error(t, json.Unmarshal(wire, tc.destination))
	}
	null, err := json.Marshal(nil)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(null, &output))
	require.Nil(t, output)
}

type repairM1091Car struct {
	Wheels int64 `json:"wheels"`
}

func (*repairM1091Car) TypeTag() string { return "repair-m1091/car" }

func init() { jsontypes.MustRegister(&repairM1091Car{}) }

func TestRepairM_1091_TaggedAlternative(t *testing.T) {
	input := &repairM1091Car{Wheels: 4}
	wire, err := jsontypes.Marshal(input)
	require.NoError(t, err)
	require.Equal(t, `{"type":"repair-m1091/car","value":{"wheels":4}}`, string(wire))
	var output jsontypes.Tagged
	require.NoError(t, jsontypes.Unmarshal(wire, &output))
	require.IsType(t, input, output)
	require.Equal(t, input, output)

	// The separate registry does not add wrappers or polymorphism to libs/json.
	concreteWire, err := json.Marshal(input)
	require.NoError(t, err)
	require.Equal(t, `{"wheels":"4"}`, string(concreteWire))
	var concrete repairM1091Car
	require.NoError(t, json.Unmarshal(concreteWire, &concrete))
	require.Equal(t, *input, concrete)
	var unsupported jsontypes.Tagged
	require.Error(t, json.Unmarshal(wire, &unsupported))
}
