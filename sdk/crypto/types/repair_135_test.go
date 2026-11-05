package types_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	multisigkeys "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/multisig"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	multisigtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types/multisig"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

func TestRepair_135_BitCountBounds(t *testing.T) {
	for name, array := range map[string]*cryptotypes.CompactBitArray{
		"nil":               nil,
		"constructor_empty": cryptotypes.NewCompactBitArray(0),
		"zero_value":        {},
	} {
		t.Run(name, func(t *testing.T) {
			for _, index := range []int{-1, 0, 1, 8, 16} {
				require.Zero(t, array.NumTrueBitsBefore(index), "index %d", index)
			}
		})
	}

	for size := 1; size <= 33; size++ {
		for _, pattern := range []string{"empty", "full", "alternating", "sparse"} {
			t.Run(fmt.Sprintf("%d/%s", size, pattern), func(t *testing.T) {
				array := cryptotypes.NewCompactBitArray(size)
				set := make([]bool, size)
				for i := range set {
					set[i] = pattern == "full" || pattern == "alternating" && i%2 == 0 ||
						pattern == "sparse" && (i%7 == 0 || i == size-1)
					require.True(t, array.SetIndex(i, set[i]))
				}
				if remaining := size % 8; remaining != 0 {
					array.Elems[len(array.Elems)-1] |= byte((1 << (8 - remaining)) - 1)
				}
				for index := -1; index <= size+8; index++ {
					want := 0
					for i, value := range set {
						if i < index && value {
							want++
						}
					}
					require.Equal(t, want, array.NumTrueBitsBefore(index), "index %d", index)
				}
			})
		}
	}
}

func TestRepair_135_MultisigTransactionVerification(t *testing.T) {
	registry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	config := authtx.NewTxConfig(codec.NewProtoCodec(registry), authtx.DefaultSignModes)
	handler := config.SignModeHandler()
	mode := signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON
	signer := authsigning.SignerData{ChainID: "repair-135", AccountNumber: 12, Sequence: 3}

	for _, size := range []int{7, 8, 9, 15, 16, 17} {
		for _, sparse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/sparse=%t", size, sparse), func(t *testing.T) {
				privateKeys := make([]cryptotypes.PrivKey, size)
				publicKeys := make([]cryptotypes.PubKey, size)
				var selected []int
				for i := range publicKeys {
					privateKeys[i] = secp256k1.GenPrivKey()
					publicKeys[i] = privateKeys[i].PubKey()
					if !sparse || i%3 != 1 {
						selected = append(selected, i)
					}
				}
				publicKey := multisigkeys.NewLegacyAminoPubKey(len(selected), publicKeys)
				builder := config.NewTxBuilder()
				message := banktypes.NewMsgSend(sdk.AccAddress(publicKey.Address()),
					sdk.AccAddress(publicKeys[0].Address()), sdk.NewCoins(sdk.NewInt64Coin("stake", 1)))
				require.NoError(t, message.ValidateBasic())
				require.NoError(t, builder.SetMsgs(message))
				builder.SetMemo("byte-aligned multisig")
				builder.SetGasLimit(200000)
				signBytes, err := handler.GetSignBytes(mode, signer, builder.GetTx())
				require.NoError(t, err)

				assembled := multisigtypes.NewMultisig(size)
				insufficient := multisigtypes.NewMultisig(size)
				for j := len(selected) - 1; j >= 0; j-- {
					index := selected[j]
					signature, err := privateKeys[index].Sign(signBytes)
					require.NoError(t, err)
					data := &signing.SingleSignatureData{SignMode: mode, Signature: signature}
					multisigtypes.AddSignature(assembled, data, index)
					if j != 0 {
						multisigtypes.AddSignature(insufficient, data, index)
					}
				}

				transport := func(data *signing.MultiSignatureData) authsigning.Tx {
					require.NoError(t, builder.SetSignatures(signing.SignatureV2{
						PubKey: publicKey, Data: data, Sequence: signer.Sequence,
					}))
					encoded, err := config.TxEncoder()(builder.GetTx())
					require.NoError(t, err)
					decoded, err := config.TxDecoder()(encoded)
					require.NoError(t, err)
					require.NoError(t, decoded.ValidateBasic())
					result, ok := decoded.(authsigning.Tx)
					require.True(t, ok)
					return result
				}

				decoded := transport(assembled)
				signatures, err := decoded.GetSignaturesV2()
				require.NoError(t, err)
				require.Len(t, signatures, 1)
				recovered, ok := signatures[0].Data.(*signing.MultiSignatureData)
				require.True(t, ok)
				require.Equal(t, size, recovered.BitArray.Count())
				require.Len(t, recovered.Signatures, len(selected))
				for i := 0; i < size; i++ {
					require.Equal(t, !sparse || i%3 != 1, recovered.BitArray.GetIndex(i), "signer %d", i)
				}
				require.NoError(t, authsigning.VerifySignature(signatures[0].PubKey, signer,
					signatures[0].Data, handler, decoded))
				wrongSigner := signer
				wrongSigner.AccountNumber++
				require.ErrorContains(t, authsigning.VerifySignature(signatures[0].PubKey, wrongSigner,
					signatures[0].Data, handler, decoded), "unable to verify signature")

				decoded = transport(insufficient)
				signatures, err = decoded.GetSignaturesV2()
				require.NoError(t, err)
				require.Len(t, signatures, 1)
				require.ErrorContains(t, authsigning.VerifySignature(signatures[0].PubKey, signer,
					signatures[0].Data, handler, decoded), "signature size is incorrect")
			})
		}
	}
}
