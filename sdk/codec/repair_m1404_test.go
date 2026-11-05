package codec_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/testdata"
)

func repairM1404UnpreparedAnimal(t *testing.T) *testdata.HasAnimal {
	t.Helper()
	packed, err := types.NewAnyWithValue(&testdata.Dog{Name: "rufus"})
	require.NoError(t, err)
	wire, err := proto.Marshal(&testdata.HasAnimal{Animal: packed})
	require.NoError(t, err)
	var received testdata.HasAnimal
	require.NoError(t, proto.Unmarshal(wire, &received))
	require.NotNil(t, received.Animal)
	require.Equal(t, packed.TypeUrl, received.Animal.TypeUrl)
	require.Equal(t, packed.Value, received.Animal.Value)
	require.Nil(t, received.Animal.GetCachedValue())
	envelope, err := types.NewAnyWithValue(received.Animal)
	require.NoError(t, err)
	require.Same(t, received.Animal, envelope.GetCachedValue())
	return &testdata.HasAnimal{Animal: envelope}
}

func TestRepairM_1404_UnpreparedAnyReturnsError(t *testing.T) {
	for _, adapter := range []bool{false, true} {
		t.Run(fmt.Sprintf("amino_adapter_%t", adapter), func(t *testing.T) {
			cdc := createTestCodec()
			wantErr := types.UnpackInterfaces(repairM1404UnpreparedAnimal(t), types.AminoJSONPacker{Cdc: cdc.Amino})
			require.Error(t, wantErr)
			marshal := cdc.MarshalJSONIndent
			if adapter {
				marshal = codec.NewAminoCodec(cdc).MarshalJSONIndent
			}
			var bz []byte
			var err error
			require.NotPanics(t, func() {
				bz, err = marshal(repairM1404UnpreparedAnimal(t), "\t", "    ")
			})
			require.Nil(t, bz)
			require.EqualError(t, err, wantErr.Error())
			require.IsType(t, wantErr, err)
		})
	}
}

func TestRepairM_1404_UnpreparedAnyHelperBoundaries(t *testing.T) {
	cdc := createTestCodec()
	_, wantErr := cdc.MarshalAsJSON(repairM1404UnpreparedAnimal(t))
	require.Error(t, wantErr)
	var bz []byte
	var err error
	require.NotPanics(t, func() {
		bz, err = codec.MarshalJSONIndent(cdc, repairM1404UnpreparedAnimal(t))
	})
	require.Nil(t, bz)
	require.EqualError(t, err, wantErr.Error())
	require.PanicsWithValue(t, "failed to marshal JSON: "+wantErr.Error(), func() {
		codec.MustMarshalJSONIndent(cdc, repairM1404UnpreparedAnimal(t))
	})
}

func TestRepairM_1404_PreparedAnyRoundTrip(t *testing.T) {
	cdc := createTestCodec()
	dog := &testdata.Dog{Name: "rufus \"犬\""}
	packed, err := types.NewAnyWithValue(dog)
	require.NoError(t, err)
	input := &testdata.HasAnimal{Animal: packed}
	compact, err := cdc.MarshalAsJSON(input)
	require.NoError(t, err)
	var want bytes.Buffer
	require.NoError(t, json.Indent(&want, compact, "\t", "    "))
	for _, marshal := range []func(interface{}, string, string) ([]byte, error){
		cdc.MarshalJSONIndent,
		codec.NewAminoCodec(cdc).MarshalJSONIndent,
	} {
		bz, err := marshal(input, "\t", "    ")
		require.NoError(t, err)
		require.Equal(t, want.Bytes(), bz)
		var received testdata.HasAnimal
		require.NoError(t, cdc.UnmarshalAsJSON(bz, &received))
		require.Equal(t, dog, received.Animal.GetCachedValue())
	}
}
