package json_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
)

type repair173Key string
type repair173InnerKey string
type repair173Map map[repair173Key]int64

func TestRepair_173_NamedStringMapRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input repair173Map
		want  repair173Map
	}{
		{
			name: "named keys and map",
			input: repair173Map{
				"":               0,
				"ordinary":       42,
				"quotes\"\\\n\t": -42,
				"日本語":            1<<63 - 1,
				"minimum":        -1 << 63,
			},
			want: repair173Map{
				"":               0,
				"ordinary":       42,
				"quotes\"\\\n\t": -42,
				"日本語":            1<<63 - 1,
				"minimum":        -1 << 63,
			},
		},
		{name: "empty", input: repair173Map{}, want: repair173Map{}},
		{name: "nil uses Amino empty object", input: nil, want: repair173Map{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire, err := json.Marshal(tc.input)
			require.NoError(t, err)
			got := repair173Map{"stale": 99}
			require.NoError(t, json.Unmarshal(wire, &got))
			require.Equal(t, tc.want, got)
		})
	}

	input := map[repair173Key]string{"key": "value"}
	wire, err := json.Marshal(input)
	require.NoError(t, err)
	var got map[repair173Key]string
	require.NoError(t, json.Unmarshal(wire, &got))
	require.Equal(t, input, got)
}

func TestRepair_173_NestedMapsRoundTrip(t *testing.T) {
	type innerMap map[repair173InnerKey]*int64
	type envelope struct {
		Maps map[repair173Key]innerMap `json:"maps"`
	}
	value := int64(1<<63 - 1)
	input := envelope{Maps: map[repair173Key]innerMap{
		"outer": {"value": &value, "null": nil},
		"empty": {},
	}}
	wire, err := json.Marshal(input)
	require.NoError(t, err)
	var got envelope
	require.NoError(t, json.Unmarshal(wire, &got))
	require.Equal(t, input, got)

	ordinaryOuter := map[string]repair173Map{"outer": {"inner": 7}}
	wire, err = json.Marshal(ordinaryOuter)
	require.NoError(t, err)
	var gotOrdinaryOuter map[string]repair173Map
	require.NoError(t, json.Unmarshal(wire, &gotOrdinaryOuter))
	require.Equal(t, ordinaryOuter, gotOrdinaryOuter)
}

func TestRepair_173_InvalidValues(t *testing.T) {
	for _, tc := range []struct {
		name   string
		input  any
		target any
	}{
		{
			name:   "invalid integer string",
			input:  map[repair173Key]string{"key": "invalid"},
			target: new(repair173Map),
		},
		{
			name:   "integer without Amino string encoding",
			input:  map[repair173Key]int32{"key": 42},
			target: new(repair173Map),
		},
		{
			name:   "overflow",
			input:  map[repair173Key]int32{"key": 128},
			target: new(map[repair173Key]int8),
		},
		{
			name:   "nested invalid value",
			input:  map[repair173Key]map[repair173InnerKey]string{"outer": {"inner": "invalid"}},
			target: new(map[repair173Key]map[repair173InnerKey]int64),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire, err := json.Marshal(tc.input)
			require.NoError(t, err)
			require.Error(t, json.Unmarshal(wire, tc.target))
		})
	}
}
