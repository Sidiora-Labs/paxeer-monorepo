package types

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
)

// validateGenesisProtobufStream sends real protobuf codec output through the
// typed channel consumed by the module's production validation boundary.
func validateGenesisProtobufStream(t *testing.T, states ...GenesisState) error {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	decoded := make([]GenesisState, len(states))
	for i := range states {
		encoded, err := cdc.Marshal(&states[i])
		require.NoError(t, err)
		require.NoError(t, cdc.Unmarshal(encoded, &decoded[i]))
		require.Equal(t, states[i].Params, decoded[i].Params)
	}

	stream := make(chan GenesisState)
	producerDone := make(chan struct{})
	go func() {
		defer close(producerDone)
		defer close(stream)
		for _, state := range decoded {
			stream <- state
		}
	}()
	err := ValidateGenesisStream(stream)
	<-producerDone
	return err
}

func validateGenesisJSONStream(t *testing.T, states ...GenesisState) error {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	decoded := make([]GenesisState, len(states))
	for i := range states {
		encoded, err := cdc.MarshalAsJSON(&states[i])
		require.NoError(t, err)
		require.NoError(t, cdc.UnmarshalAsJSON(encoded, &decoded[i]))
		require.Equal(t, states[i].Params, decoded[i].Params)
	}
	return validateGenesisProtobufStream(t, decoded...)
}

func TestGenesisStreamEmptyGenesisStream(t *testing.T) {
	err := validateGenesisProtobufStream(t)
	require.ErrorIs(t, err, ErrEmpty)
	require.EqualError(t, err, "missing params: empty")
	require.EqualError(t, validateGenesisJSONStream(t), "missing params: empty")
}

func TestGenesisStreamInvalidOnlyStreams(t *testing.T) {
	address := sdk.AccAddress(make([]byte, SDKAddrLen)).String()
	cases := map[string]Params{
		"zero params":                  {},
		"empty instantiate permission": {CodeUploadAccess: AllowEverybody},
		"unknown instantiate permission": {
			CodeUploadAccess: AllowEverybody, InstantiateDefaultPermission: AccessType(1111),
		},
		"empty upload permission": {InstantiateDefaultPermission: AccessTypeEverybody},
		"unknown upload permission": {
			CodeUploadAccess:             AccessConfig{Permission: AccessType(1111)},
			InstantiateDefaultPermission: AccessTypeEverybody,
		},
		"invalid upload address": {
			CodeUploadAccess:             AccessConfig{Permission: AccessTypeOnlyAddress, Address: "invalid"},
			InstantiateDefaultPermission: AccessTypeEverybody,
		},
		"everybody with address": {
			CodeUploadAccess:             AccessConfig{Permission: AccessTypeEverybody, Address: address},
			InstantiateDefaultPermission: AccessTypeEverybody,
		},
		"nobody with address": {
			CodeUploadAccess:             AccessConfig{Permission: AccessTypeNobody, Address: address},
			InstantiateDefaultPermission: AccessTypeNobody,
		},
	}
	for name, params := range cases {
		t.Run(name, func(t *testing.T) {
			paramErr := params.ValidateBasic()
			require.Error(t, paramErr)
			want := sdkerrors.Wrap(paramErr, "params")
			for _, states := range [][]GenesisState{
				{{Params: params}},
				{{}, {Params: params}},
			} {
				err := validateGenesisProtobufStream(t, states...)
				require.EqualError(t, err, want.Error())
				// Unknown enum values are intentionally normalized to Unspecified
				// by AccessType's JSON marshaler; binary protobuf preserves them.
				if params.InstantiateDefaultPermission != AccessType(1111) && params.CodeUploadAccess.Permission != AccessType(1111) {
					require.EqualError(t, validateGenesisJSONStream(t, states...), want.Error())
				}
			}
		})
	}
}

func TestGenesisStreamValidStreams(t *testing.T) {
	address := sdk.AccAddress(make([]byte, SDKAddrLen))
	for _, params := range []Params{
		DefaultParams(),
		{CodeUploadAccess: AllowNobody, InstantiateDefaultPermission: AccessTypeNobody},
		{CodeUploadAccess: AccessTypeOnlyAddress.With(address), InstantiateDefaultPermission: AccessTypeOnlyAddress},
	} {
		require.NoError(t, params.ValidateBasic())
		valid := GenesisState{Params: params}
		data := GenesisState{Sequences: []Sequence{{IDKey: []byte("sequence"), Value: 3}}}
		for _, states := range [][]GenesisState{
			{valid},
			{valid, data, {}},
			{data, valid, {}},
			{data, {}, valid},
		} {
			require.NoError(t, validateGenesisProtobufStream(t, states...))
			require.NoError(t, validateGenesisJSONStream(t, states...))
		}
	}
}

func TestGenesisStreamChunkChecksPreserved(t *testing.T) {
	cases := map[string]struct {
		state GenesisState
		want  error
	}{
		"code": {
			state: GenesisState{Codes: []Code{{}}},
			want:  sdkerrors.Wrapf((Code{}).ValidateBasic(), "code: %d", 0),
		},
		"contract": {
			state: GenesisState{Contracts: []Contract{{ContractAddress: "invalid"}}},
			want:  sdkerrors.Wrapf((Contract{ContractAddress: "invalid"}).ValidateBasic(), "contract: %d", 0),
		},
		"sequence": {
			state: GenesisState{Sequences: []Sequence{{}}},
			want:  sdkerrors.Wrapf((Sequence{}).ValidateBasic(), "sequence: %d", 0),
		},
		"genesis message": {
			state: GenesisState{GenMsgs: []GenesisState_GenMsgs{{}}},
			want:  sdkerrors.Wrapf((GenesisState_GenMsgs{}).ValidateBasic(), "gen message: %d", 0),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.Error(t, tc.want)
			valid := GenesisState{Params: DefaultParams()}
			withParams := tc.state
			withParams.Params = DefaultParams()
			for _, states := range [][]GenesisState{
				{valid, tc.state, valid},
				{withParams, valid, {}},
			} {
				require.EqualError(t, validateGenesisProtobufStream(t, states...), tc.want.Error())
				require.EqualError(t, validateGenesisJSONStream(t, states...), tc.want.Error())
			}
		})
	}
}
