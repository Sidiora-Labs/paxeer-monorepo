package legacytx_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	multisigkeys "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/multisig"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	multisigtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types/multisig"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/testdata"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
)

func TestRepairM_656_MalformedMultisigTransport(t *testing.T) {
	key, cases := malformedAminoMultisigs(t)
	cdc := testCodec()
	config := legacytx.StdTxConfig{Cdc: cdc}
	for _, tc := range cases {
		for _, nested := range []bool{false, true} {
			name := tc.name
			if nested {
				name += "/nested"
			}
			t.Run(name, func(t *testing.T) {
				pubKey, data := key, tc.data
				if nested {
					pubKey = multisigkeys.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{key})
					data = multisigtypes.NewMultisig(1)
					multisigtypes.AddSignature(data, tc.data, 0)
				}
				builder := config.NewTxBuilder()
				require.NoError(t, builder.SetMsgs(testdata.NewTestMsg()))
				require.NoError(t, builder.SetSignatures(signing.SignatureV2{PubKey: pubKey, Data: data}))
				stdTx := builder.GetTx().(legacytx.StdTx)
				var amino multisigtypes.AminoMultisignature
				require.NoError(t, cdc.Unmarshal(stdTx.Signatures[0].Signature, &amino), "fixture must be decodable Amino")
				for _, jsonTransport := range []bool{false, true} {
					encoder, decoder := config.TxEncoder(), config.TxDecoder()
					if jsonTransport {
						encoder, decoder = config.TxJSONEncoder(), config.TxJSONDecoder()
					}
					wire, err := encoder(builder.GetTx())
					require.NoError(t, err)
					decoded, err := decoder(wire)
					require.NoError(t, err)
					tx, ok := decoded.(authsigning.SigVerifiableTx)
					require.True(t, ok)
					require.NotPanics(t, func() {
						signatures, err := tx.GetSignaturesV2()
						require.ErrorContains(t, err, "multisig")
						require.Nil(t, signatures)
					})
				}
				wire, err := cdc.MarshalAsJSON(stdTx.Signatures)
				require.NoError(t, err)
				require.NotPanics(t, func() {
					signatures, err := config.UnmarshalSignatureJSON(wire)
					require.ErrorContains(t, err, "multisig")
					require.Nil(t, signatures)
				})
			})
		}
	}
}

func TestRepairM_656_ValidMultisigTransport(t *testing.T) {
	cdc := testCodec()
	config := legacytx.StdTxConfig{Cdc: cdc}
	priv0, pk0, _ := testdata.KeyTestPubAddr()
	priv1, pk1, _ := testdata.KeyTestPubAddr()
	priv2, pk2, _ := testdata.KeyTestPubAddr()
	keys := []cryptotypes.PubKey{pk0, pk1, pk2}
	privateKeys := []cryptotypes.PrivKey{priv0, priv1, priv2}
	signer := authsigning.SignerData{ChainID: "repair-m656", AccountNumber: 7, Sequence: 2}
	mode := signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON
	for _, nested := range []bool{false, true} {
		for _, sparse := range []bool{false, true} {
			name := "full"
			if sparse {
				name = "sparse"
			}
			if nested {
				name += "/nested"
			}
			t.Run(name, func(t *testing.T) {
				key := multisigkeys.NewLegacyAminoPubKey(2, keys)
				outerKeys := []cryptotypes.PubKey{pk0, key, pk2}
				pubKey := key
				if nested {
					pubKey = multisigkeys.NewLegacyAminoPubKey(1, outerKeys)
				}
				builder := config.NewTxBuilder()
				require.NoError(t, builder.SetMsgs(testdata.NewTestMsg()))
				builder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("stake", 1)))
				builder.SetGasLimit(100000)
				builder.SetMemo("multisig conversion")
				signBytes, err := config.SignModeHandler().GetSignBytes(mode, signer, builder.GetTx())
				require.NoError(t, err)
				data := multisigtypes.NewMultisig(len(keys))
				for i, priv := range privateKeys {
					if sparse && i == 0 {
						continue
					}
					sig, err := priv.Sign(signBytes)
					require.NoError(t, err)
					multisigtypes.AddSignature(data, &signing.SingleSignatureData{SignMode: mode, Signature: sig}, i)
				}
				if nested {
					inner := data
					data = multisigtypes.NewMultisig(len(outerKeys))
					multisigtypes.AddSignature(data, inner, 1)
				}
				require.NoError(t, builder.SetSignatures(signing.SignatureV2{PubKey: pubKey, Data: data}))
				wire, err := config.TxEncoder()(builder.GetTx())
				require.NoError(t, err)
				decoded, err := config.TxDecoder()(wire)
				require.NoError(t, err)
				tx, ok := decoded.(authsigning.Tx)
				require.True(t, ok)
				signatures, err := tx.GetSignaturesV2()
				require.NoError(t, err)
				require.Len(t, signatures, 1)
				require.Equal(t, data, signatures[0].Data)
				require.NoError(t, authsigning.VerifySignature(signatures[0].PubKey, signer, signatures[0].Data, config.SignModeHandler(), tx))
				wrongSigner := signer
				wrongSigner.AccountNumber++
				require.Error(t, authsigning.VerifySignature(signatures[0].PubKey, wrongSigner, signatures[0].Data, config.SignModeHandler(), tx))
			})
		}
	}
}
