package json_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
)

type namedMapKey string
type namedMapInnerKey string
type namedInt64Map map[namedMapKey]int64

func TestNamedStringMapRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input namedInt64Map
		want  namedInt64Map
	}{
		{
			name: "named keys and map",
			input: namedInt64Map{
				"":               0,
				"ordinary":       42,
				"quotes\"\\\n\t": -42,
				"日本語":            1<<63 - 1,
				"minimum":        -1 << 63,
			},
			want: namedInt64Map{
				"":               0,
				"ordinary":       42,
				"quotes\"\\\n\t": -42,
				"日本語":            1<<63 - 1,
				"minimum":        -1 << 63,
			},
		},
		{name: "empty", input: namedInt64Map{}, want: namedInt64Map{}},
		{name: "nil uses Amino empty object", input: nil, want: namedInt64Map{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire, err := json.Marshal(tc.input)
			require.NoError(t, err)
			got := namedInt64Map{"stale": 99}
			require.NoError(t, json.Unmarshal(wire, &got))
			require.Equal(t, tc.want, got)
		})
	}

	input := map[namedMapKey]string{"key": "value"}
	wire, err := json.Marshal(input)
	require.NoError(t, err)
	var got map[namedMapKey]string
	require.NoError(t, json.Unmarshal(wire, &got))
	require.Equal(t, input, got)
}

func TestNestedNamedStringMapsRoundTrip(t *testing.T) {
	type innerMap map[namedMapInnerKey]*int64
	type envelope struct {
		Maps map[namedMapKey]innerMap `json:"maps"`
	}
	value := int64(1<<63 - 1)
	input := envelope{Maps: map[namedMapKey]innerMap{
		"outer": {"value": &value, "null": nil},
		"empty": {},
	}}
	wire, err := json.Marshal(input)
	require.NoError(t, err)
	var got envelope
	require.NoError(t, json.Unmarshal(wire, &got))
	require.Equal(t, input, got)

	ordinaryOuter := map[string]namedInt64Map{"outer": {"inner": 7}}
	wire, err = json.Marshal(ordinaryOuter)
	require.NoError(t, err)
	var gotOrdinaryOuter map[string]namedInt64Map
	require.NoError(t, json.Unmarshal(wire, &gotOrdinaryOuter))
	require.Equal(t, ordinaryOuter, gotOrdinaryOuter)
}

func TestNamedStringMapInvalidValues(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  any
		target any
	}{
		{
			name:   "invalid integer string",
			input:  map[namedMapKey]string{"key": "invalid"},
			target: new(namedInt64Map),
		},
		{
			name:   "integer without Amino string encoding",
			input:  map[namedMapKey]int32{"key": 42},
			target: new(namedInt64Map),
		},
		{
			name:   "overflow",
			input:  map[namedMapKey]int32{"key": 128},
			target: new(map[namedMapKey]int8),
		},
		{
			name:   "nested invalid value",
			input:  map[namedMapKey]map[namedMapInnerKey]string{"outer": {"inner": "invalid"}},
			target: new(map[namedMapKey]map[namedMapInnerKey]int64),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire, err := json.Marshal(tc.input)
			require.NoError(t, err)
			require.Error(t, json.Unmarshal(wire, tc.target))
		})
	}
}
