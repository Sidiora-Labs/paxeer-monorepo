package types_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/types"
)

func TestCoreParamsProtoKeepsTheJSONNames(t *testing.T) {
	params := types.DefaultParams()
	require.Equal(t, types.NewParams(true, true), params)
	require.NoError(t, params.Validate())
	require.Equal(t, "ibc.core.types.v1.Params", proto.MessageName(&params))

	for field, name := range map[string]string{
		"InboundEnabled":  "inbound_enabled",
		"OutboundEnabled": "outbound_enabled",
	} {
		info, ok := reflect.TypeOf(params).FieldByName(field)
		require.True(t, ok)
		require.Equal(t, name, strings.Split(info.Tag.Get("json"), ",")[0])
		require.Equal(t, name, info.Tag.Get("yaml"))
	}
	encoded, err := json.Marshal(params)
	require.NoError(t, err)
	require.JSONEq(t, `{"inbound_enabled":true,"outbound_enabled":true}`, string(encoded))

	pairs := params.ParamSetPairs()
	require.Len(t, pairs, 2)
	require.Equal(t, []byte("InboundEnabled"), types.KeyInboundEnabled)
	require.Equal(t, []byte("OutboundEnabled"), types.KeyOutboundEnabled)
	require.Equal(t, types.KeyInboundEnabled, pairs[0].Key)
	require.Equal(t, types.KeyOutboundEnabled, pairs[1].Key)
	require.Same(t, &params.InboundEnabled, pairs[0].Value)
	require.Same(t, &params.OutboundEnabled, pairs[1].Value)
	for _, pair := range pairs {
		require.NoError(t, pair.ValidatorFn(true))
		require.NoError(t, pair.ValidatorFn(false))
		require.Error(t, pair.ValidatorFn("true"))
		*pair.Value.(*bool) = false
	}
	require.Equal(t, types.NewParams(false, false), params)
	require.NotPanics(t, func() { types.ParamKeyTable() })

	for _, flags := range []types.Params{
		types.NewParams(false, false), types.NewParams(false, true),
		types.NewParams(true, false), types.NewParams(true, true),
	} {
		wire, err := proto.Marshal(&flags)
		require.NoError(t, err)
		var decoded types.Params
		require.NoError(t, proto.Unmarshal(wire, &decoded))
		require.Equal(t, flags, decoded)
		encoded, err := json.Marshal(flags)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(encoded, &decoded))
		require.Equal(t, flags, decoded)
		marshaler := jsonpb.Marshaler{OrigName: true, EmitDefaults: true}
		protoJSON, err := marshaler.MarshalToString(&flags)
		require.NoError(t, err)
		decoded = types.Params{}
		require.NoError(t, jsonpb.UnmarshalString(protoJSON, &decoded))
		require.Equal(t, flags, decoded)
	}
}

func TestGenesisStateMarshalsCoreParams(t *testing.T) {
	legacy := &types.GenesisState{}
	legacy.ClientGenesis.NextClientSequence = 7
	legacy.ConnectionGenesis.NextConnectionSequence = 11
	legacy.ChannelGenesis.NextChannelSequence = 13
	legacyWire, err := proto.Marshal(legacy)
	require.NoError(t, err)
	var legacyDecoded types.GenesisState
	require.NoError(t, proto.Unmarshal(legacyWire, &legacyDecoded))
	require.Nil(t, legacyDecoded.Params)
	require.Equal(t, legacy, &legacyDecoded)
	require.NoError(t, proto.Unmarshal(nil, &types.GenesisState{}))
	var absent types.GenesisState
	require.NoError(t, json.Unmarshal([]byte(`{}`), &absent))
	require.Nil(t, absent.Params)
	require.NoError(t, jsonpb.UnmarshalString(`{}`, &absent))
	require.Nil(t, absent.Params)

	for _, tc := range []struct {
		name    string
		params  types.Params
		payload []byte
	}{
		{"disabled", types.NewParams(false, false), nil},
		{"inbound", types.NewParams(true, false), []byte{0x08, 0x01}},
		{"outbound", types.NewParams(false, true), []byte{0x10, 0x01}},
		{"enabled", types.NewParams(true, true), []byte{0x08, 0x01, 0x10, 0x01}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			genesis := *legacy
			genesis.Params = &tc.params
			wire, err := proto.Marshal(&genesis)
			require.NoError(t, err)
			// Field 4 is a length-delimited Params message, including when both flags are false.
			expected := append(append([]byte{}, legacyWire...), 0x22, byte(len(tc.payload)))
			expected = append(expected, tc.payload...)
			require.Equal(t, expected, wire)
			var decoded types.GenesisState
			require.NoError(t, proto.Unmarshal(wire, &decoded))
			require.NotNil(t, decoded.Params)
			require.Equal(t, genesis, decoded)

			encoded, err := json.Marshal(genesis)
			require.NoError(t, err)
			decoded = types.GenesisState{}
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			require.Equal(t, genesis, decoded)
			marshaler := jsonpb.Marshaler{OrigName: true}
			protoJSON, err := marshaler.MarshalToString(&genesis)
			require.NoError(t, err)
			decoded = types.GenesisState{}
			require.NoError(t, jsonpb.UnmarshalString(protoJSON, &decoded))
			require.Equal(t, genesis, decoded)
		})
	}
}
