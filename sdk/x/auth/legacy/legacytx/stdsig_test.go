package legacytx_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	multisigkeys "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/multisig"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/testdata"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx"
)

func TestStdSignatureMarshalYAML(t *testing.T) {
	_, pk, _ := testdata.KeyTestPubAddr()
	pkStr := pk.String()

	testCases := []struct {
		sig      legacytx.StdSignature
		expected string
	}{
		{
			legacytx.StdSignature{},
			"|\n  pubkey: \"\"\n  signature: \"\"\n",
		},
		{
			legacytx.StdSignature{PubKey: pk, Signature: []byte("dummySig")},
			fmt.Sprintf("|\n  pubkey: %s\n  signature: 64756D6D79536967\n", pkStr),
		},
		{
			legacytx.StdSignature{PubKey: pk, Signature: nil},
			fmt.Sprintf("|\n  pubkey: %s\n  signature: \"\"\n", pkStr),
		},
	}

	for i, tc := range testCases {
		bz, err := yaml.Marshal(tc.sig)
		require.NoError(t, err)
		require.Equal(t, tc.expected, string(bz), "test case #%d", i)
	}
}

type malformedAminoMultisigCase struct {
	name string
	data *signing.MultiSignatureData
}

func malformedAminoMultisigs(t *testing.T) (cryptotypes.PubKey, []malformedAminoMultisigCase) {
	t.Helper()
	priv, pk, _ := testdata.KeyTestPubAddr()
	_, otherPK, _ := testdata.KeyTestPubAddr()
	sig, err := priv.Sign([]byte("legacy multisig shape regression"))
	require.NoError(t, err)
	single := &signing.SingleSignatureData{SignMode: signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, Signature: sig}
	bits := func(size int, indices ...int) *cryptotypes.CompactBitArray {
		array := cryptotypes.NewCompactBitArray(size)
		for _, index := range indices {
			require.True(t, array.SetIndex(index, true))
		}
		return array
	}
	key := multisigkeys.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{pk, otherPK})
	return key, []malformedAminoMultisigCase{
		{"short_signatures", &signing.MultiSignatureData{BitArray: bits(2, 0, 1), Signatures: []signing.SignatureData{single}}},
		{"missing_signatures", &signing.MultiSignatureData{BitArray: bits(2, 1)}},
		{"extra_signatures", &signing.MultiSignatureData{BitArray: bits(2, 0), Signatures: []signing.SignatureData{single, single}}},
		{"signatures_without_set_bits", &signing.MultiSignatureData{BitArray: bits(2), Signatures: []signing.SignatureData{single}}},
		{"nil_bitarray", &signing.MultiSignatureData{Signatures: []signing.SignatureData{single}}},
		{"nil_bitarray_without_signatures", &signing.MultiSignatureData{}},
		{"oversized_bitarray", &signing.MultiSignatureData{BitArray: bits(3, 2), Signatures: []signing.SignatureData{single}}},
		{"oversized_unset_bitarray", &signing.MultiSignatureData{BitArray: bits(3, 0), Signatures: []signing.SignatureData{single}}},
		{"undersized_bitarray", &signing.MultiSignatureData{BitArray: bits(1, 0), Signatures: []signing.SignatureData{single}}},
		{"empty_bitarray", &signing.MultiSignatureData{BitArray: &cryptotypes.CompactBitArray{}}},
		{"missing_bitarray_storage", &signing.MultiSignatureData{BitArray: &cryptotypes.CompactBitArray{ExtraBitsStored: 10}, Signatures: []signing.SignatureData{single}}},
		{"invalid_extra_bits", &signing.MultiSignatureData{BitArray: &cryptotypes.CompactBitArray{ExtraBitsStored: 9, Elems: []byte{0x80}}, Signatures: []signing.SignatureData{single}}},
	}
}

func TestStdSignatureToSignatureV2MalformedMultisig(t *testing.T) {
	key, cases := malformedAminoMultisigs(t)
	cdc := codec.NewLegacyAmino()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdSig, err := legacytx.SignatureV2ToStdSignature(cdc, signing.SignatureV2{PubKey: key, Data: tc.data})
			require.NoError(t, err)
			require.NotPanics(t, func() {
				converted, err := legacytx.StdSignatureToSignatureV2(cdc, stdSig)
				require.Error(t, err)
				require.Equal(t, signing.SignatureV2{}, converted)
			})
		})
	}
}
