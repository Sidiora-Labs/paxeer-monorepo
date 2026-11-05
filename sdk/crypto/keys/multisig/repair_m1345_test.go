package multisig_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	kmultisig "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/multisig"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	multisigtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types/multisig"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/ante"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
)

func repairM1345Codecs() (*codec.ProtoCodec, *codec.LegacyAmino) {
	registry := codectypes.NewInterfaceRegistry()
	sdk.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	amino := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(amino)
	cryptocodec.RegisterCrypto(amino)
	authtypes.RegisterLegacyAminoCodec(amino)
	banktypes.RegisterLegacyAminoCodec(amino)
	return codec.NewProtoCodec(registry), amino
}

func repairM1345Keys(n int) ([]cryptotypes.PrivKey, []cryptotypes.PubKey) {
	privs := make([]cryptotypes.PrivKey, n)
	pubs := make([]cryptotypes.PubKey, n)
	for i := range privs {
		privs[i] = secp256k1.GenPrivKeyFromSecret([]byte(fmt.Sprintf("repair-m1345-key-%d", i)))
		pubs[i] = privs[i].PubKey()
	}
	return privs, pubs
}

func repairM1345Signed(t *testing.T, privs []cryptotypes.PrivKey, msg []byte, sparse bool) *signing.MultiSignatureData {
	t.Helper()
	data := multisigtypes.NewMultisig(len(privs))
	// Insert backwards to exercise the SDK's ordering of signatures across bytes.
	for i := len(privs) - 1; i >= 0; i-- {
		if sparse && i != 0 && i != len(privs)-1 {
			continue
		}
		bz, err := privs[i].Sign(msg)
		require.NoError(t, err)
		multisigtypes.AddSignature(data, &signing.SingleSignatureData{
			SignMode: signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, Signature: bz,
		}, i)
	}
	return data
}

func TestRepairM_1345_CardinalityAndShape(t *testing.T) {
	cdc, _ := repairM1345Codecs()
	privs, pubs := repairM1345Keys(3)
	msg := []byte("multisig shape validation")
	cases := []struct {
		name   string
		mutate func(**kmultisig.LegacyAminoPubKey, **signing.MultiSignatureData)
	}{
		{"missing-signature", func(_ **kmultisig.LegacyAminoPubKey, s **signing.MultiSignatureData) {
			(*s).Signatures = (*s).Signatures[:2]
		}},
		{"extra-signature", func(_ **kmultisig.LegacyAminoPubKey, s **signing.MultiSignatureData) {
			(*s).BitArray.SetIndex(2, false)
		}},
		{"zero-threshold", func(k **kmultisig.LegacyAminoPubKey, _ **signing.MultiSignatureData) { (*k).Threshold = 0 }},
		{"threshold-over-key-count", func(k **kmultisig.LegacyAminoPubKey, _ **signing.MultiSignatureData) { (*k).Threshold = 4 }},
		{"nil-key", func(k **kmultisig.LegacyAminoPubKey, _ **signing.MultiSignatureData) { *k = nil }},
		{"nil-signature", func(_ **kmultisig.LegacyAminoPubKey, s **signing.MultiSignatureData) { *s = nil }},
		{"nil-bits", func(_ **kmultisig.LegacyAminoPubKey, s **signing.MultiSignatureData) { (*s).BitArray = nil }},
		{"empty-bits", func(_ **kmultisig.LegacyAminoPubKey, s **signing.MultiSignatureData) { (*s).BitArray.Elems = nil }},
		{"invalid-extra-bits", func(_ **kmultisig.LegacyAminoPubKey, s **signing.MultiSignatureData) {
			(*s).BitArray.ExtraBitsStored = 9
		}},
		{"noncanonical-eight-bits", func(_ **kmultisig.LegacyAminoPubKey, s **signing.MultiSignatureData) {
			(*s).BitArray.ExtraBitsStored = 8
		}},
		{"wrong-bit-count", func(_ **kmultisig.LegacyAminoPubKey, s **signing.MultiSignatureData) {
			(*s).BitArray.ExtraBitsStored = 2
		}},
		{"extra-bit-byte", func(_ **kmultisig.LegacyAminoPubKey, s **signing.MultiSignatureData) {
			(*s).BitArray.Elems = append((*s).BitArray.Elems, 0)
		}},
		{"padding", func(_ **kmultisig.LegacyAminoPubKey, s **signing.MultiSignatureData) { (*s).BitArray.Elems[0] |= 1 }},
		{"below-threshold", func(_ **kmultisig.LegacyAminoPubKey, s **signing.MultiSignatureData) {
			(*s).BitArray.SetIndex(1, false)
			(*s).BitArray.SetIndex(2, false)
			(*s).Signatures = (*s).Signatures[:1]
		}},
		{"nil-child", func(_ **kmultisig.LegacyAminoPubKey, s **signing.MultiSignatureData) { (*s).Signatures[2] = nil }},
		{"typed-nil-single", func(_ **kmultisig.LegacyAminoPubKey, s **signing.MultiSignatureData) {
			(*s).Signatures[2] = (*signing.SingleSignatureData)(nil)
		}},
		{"typed-nil-multi", func(_ **kmultisig.LegacyAminoPubKey, s **signing.MultiSignatureData) {
			(*s).Signatures[2] = (*signing.MultiSignatureData)(nil)
		}},
		{"multi-for-single-key", func(_ **kmultisig.LegacyAminoPubKey, s **signing.MultiSignatureData) {
			(*s).Signatures[2] = multisigtypes.NewMultisig(1)
		}},
		{"nil-any", func(k **kmultisig.LegacyAminoPubKey, _ **signing.MultiSignatureData) { (*k).PubKeys[2] = nil }},
		{"unpacked-any", func(k **kmultisig.LegacyAminoPubKey, _ **signing.MultiSignatureData) {
			any := (*k).PubKeys[2]
			(*k).PubKeys[2] = &codectypes.Any{TypeUrl: any.TypeUrl, Value: any.Value}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Start with a key emitted by the production protobuf codec, with unpacked Any peers.
			key := kmultisig.NewLegacyAminoPubKey(2, pubs)
			wire, err := cdc.Marshal(key)
			require.NoError(t, err)
			key = new(kmultisig.LegacyAminoPubKey)
			require.NoError(t, cdc.Unmarshal(wire, key))
			data := repairM1345Signed(t, privs, msg, false)
			tc.mutate(&key, &data)
			calls := 0
			require.NotPanics(t, func() {
				require.Error(t, key.ValidateMultisignature(data))
				require.Error(t, key.VerifyMultisignature(func(signing.SignMode) ([]byte, error) { calls++; return msg, nil }, data))
			})
			require.Zero(t, calls, "all shape checks must precede signature traversal")
		})
	}
	key := kmultisig.NewLegacyAminoPubKey(2, pubs)
	data := repairM1345Signed(t, privs, msg, false)
	require.NoError(t, key.ValidateMultisignature(data))
	require.NoError(t, key.VerifyMultisignature(func(signing.SignMode) ([]byte, error) { return msg, nil }, data))
	require.Error(t, key.VerifyMultisignature(nil, data))
	want := errors.New("sign bytes failure")
	require.ErrorIs(t, key.VerifyMultisignature(func(signing.SignMode) ([]byte, error) { return nil, want }, data), want)
}

func TestRepairM_1345_NestedValidation(t *testing.T) {
	privs, pubs := repairM1345Keys(3)
	msg := []byte("nested validation before any signature traversal")
	for _, variant := range []string{"valid", "missing", "extra", "zero-threshold", "single-for-multikey"} {
		t.Run(variant, func(t *testing.T) {
			innerKey := kmultisig.NewLegacyAminoPubKey(2, pubs)
			innerData := repairM1345Signed(t, privs, msg, false)
			key := kmultisig.NewLegacyAminoPubKey(2, []cryptotypes.PubKey{pubs[0], innerKey})
			data := multisigtypes.NewMultisig(2)
			multisigtypes.AddSignature(data, innerData.Signatures[0], 0)
			multisigtypes.AddSignature(data, innerData, 1)
			switch variant {
			case "missing":
				innerData.Signatures = innerData.Signatures[:2]
			case "extra":
				innerData.BitArray.SetIndex(2, false)
			case "zero-threshold":
				innerKey.Threshold = 0
			case "single-for-multikey":
				data.Signatures[1] = innerData.Signatures[0]
			}
			calls := 0
			require.NotPanics(t, func() {
				err := key.VerifyMultisignature(func(signing.SignMode) ([]byte, error) { calls++; return msg, nil }, data)
				if variant == "valid" {
					require.NoError(t, err)
					require.Equal(t, 4, calls)
				} else {
					require.Error(t, err)
					require.Zero(t, calls)
				}
			})
		})
	}
}

func TestRepairM_1345_TransactionBoundaries(t *testing.T) {
	repairM1345Transactions(t)
}

func repairM1345Transactions(t *testing.T) {
	t.Helper()
	cdc, amino := repairM1345Codecs()
	configs := []struct {
		name   string
		config client.TxConfig
	}{
		{"protobuf", authtx.NewTxConfig(cdc, []signing.SignMode{signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON})},
		{"legacy", legacytx.StdTxConfig{Cdc: amino}},
	}
	for _, n := range []int{7, 8, 9, 16} {
		for _, config := range configs {
			for _, sparse := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/sparse=%t", config.name, n, sparse), func(t *testing.T) {
					privs, pubs := repairM1345Keys(n)
					key := kmultisig.NewLegacyAminoPubKey(2, pubs)
					builder := config.config.NewTxBuilder()
					require.NoError(t, builder.SetMsgs(&banktypes.MsgSend{
						FromAddress: sdk.AccAddress(key.Address()).String(), ToAddress: sdk.AccAddress(pubs[0].Address()).String(),
						Amount: sdk.NewCoins(sdk.NewInt64Coin("stake", 1)),
					}))
					builder.SetGasLimit(200000)
					builder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("stake", 1)))
					builder.SetMemo("multisig boundary transaction")
					signer := authsigning.SignerData{ChainID: "repair-m1345", AccountNumber: 7}
					msg, err := config.config.SignModeHandler().GetSignBytes(signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, signer, builder.GetTx())
					require.NoError(t, err)
					for _, variant := range []string{"valid", "missing", "extra", "zero-threshold", "threshold-over-key-count"} {
						t.Run(variant, func(t *testing.T) {
							data := repairM1345Signed(t, privs, msg, sparse)
							candidate := kmultisig.NewLegacyAminoPubKey(2, pubs)
							switch variant {
							case "missing":
								data.Signatures = data.Signatures[:len(data.Signatures)-1]
							case "extra":
								if sparse {
									data.Signatures = append(data.Signatures, data.Signatures[0])
								} else {
									data.BitArray.SetIndex(n-1, false)
								}
							case "zero-threshold":
								candidate.Threshold = 0
							case "threshold-over-key-count":
								candidate.Threshold = uint32(n + 1)
							}
							require.NoError(t, builder.SetSignatures(signing.SignatureV2{PubKey: candidate, Data: data}))
							for _, jsonWire := range []bool{false, true} {
								encoder, decoder := config.config.TxEncoder(), config.config.TxDecoder()
								if jsonWire {
									encoder, decoder = config.config.TxJSONEncoder(), config.config.TxJSONDecoder()
								}
								wire, err := encoder(builder.GetTx())
								require.NoError(t, err)
								decoded, err := decoder(wire)
								require.NoError(t, err)
								tx, ok := decoded.(authsigning.Tx)
								require.True(t, ok)
								require.NotPanics(t, func() {
									sigs, err := tx.GetSignaturesV2()
									if variant == "missing" || variant == "extra" {
										require.Error(t, err, "transport must reject unequal selected and signature counts")
										return
									}
									require.NoError(t, err)
									require.Len(t, sigs, 1)
									decodedKey := sigs[0].PubKey.(*kmultisig.LegacyAminoPubKey)
									err = authsigning.VerifySignature(decodedKey, signer, sigs[0].Data, config.config.SignModeHandler(), tx)
									if variant != "valid" {
										require.Error(t, err)
										return
									}
									require.NoError(t, err)
									require.Equal(t, data, sigs[0].Data)
									require.NoError(t, tx.ValidateBasic())
									meter := sdk.NewInfiniteGasMeter(1, 1)
									require.NoError(t, ante.ConsumeMultisignatureVerificationGas(meter, sigs[0].Data.(*signing.MultiSignatureData), decodedKey, authtypes.DefaultParams(), 0))
									require.Equal(t, uint64(len(data.Signatures))*authtypes.DefaultParams().SigVerifyCostSecp256k1, meter.GasConsumed())
									repairM1345Ante(t, cdc, amino, tx, decodedKey, config.config.SignModeHandler(), signer)
									wrongSigner := signer
									wrongSigner.AccountNumber++
									require.Error(t, authsigning.VerifySignature(decodedKey, wrongSigner, sigs[0].Data, config.config.SignModeHandler(), tx))
								})
							}
						})
					}
				})
			}
		}
	}
}

func repairM1345Ante(t *testing.T, cdc *codec.ProtoCodec, amino *codec.LegacyAmino, tx sdk.Tx, key *kmultisig.LegacyAminoPubKey, handler authsigning.SignModeHandler, signer authsigning.SignerData) {
	t.Helper()
	storeKey := sdk.NewKVStoreKey("repair-m1345")
	transientKey := sdk.NewTransientStoreKey("repair-m1345-transient")
	ctx := testutil.DefaultContext(storeKey, transientKey).WithChainID(signer.ChainID).WithBlockHeight(1)
	subspace := paramtypes.NewSubspace(cdc, amino, storeKey, transientKey, "auth")
	keeper := authkeeper.NewAccountKeeper(cdc, storeKey, subspace, authtypes.ProtoBaseAccount, nil)
	account := authtypes.NewBaseAccount(sdk.AccAddress(key.Address()), key, signer.AccountNumber, signer.Sequence)
	keeper.SetAccount(ctx, account)
	keeper.SetParams(ctx, authtypes.DefaultParams())
	// The decorator reloads the account and key from the real store/codec.
	decorator := ante.NewSigVerificationDecorator(keeper, handler)
	anteHandler := sdk.ChainAnteDecorators(decorator)
	_, err := anteHandler(ctx, tx, false)
	require.NoError(t, err)
	_, err = anteHandler(ctx.WithChainID("wrong-chain"), tx, false)
	require.Error(t, err)
}

func TestRepairM_1345_MalformedTransactions(t *testing.T) {
	cdc, amino := repairM1345Codecs()
	privs, pubs := repairM1345Keys(3)
	for _, config := range []struct {
		name   string
		config client.TxConfig
	}{
		{"protobuf", authtx.NewTxConfig(cdc, []signing.SignMode{signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON})},
		{"legacy", legacytx.StdTxConfig{Cdc: amino}},
	} {
		for _, nested := range []bool{false, true} {
			for _, variant := range []string{"missing", "extra", "nil-bits", "empty-bits", "invalid-extra", "padding", "wrong-bit-count", "extra-byte", "zero-threshold"} {
				t.Run(fmt.Sprintf("%s/nested=%t/%s", config.name, nested, variant), func(t *testing.T) {
					key := kmultisig.NewLegacyAminoPubKey(2, pubs)
					data := repairM1345Signed(t, privs, []byte("malformed transport"), false)
					switch variant {
					case "missing":
						data.Signatures = data.Signatures[:2]
					case "extra":
						data.BitArray.SetIndex(2, false)
					case "nil-bits":
						data.BitArray = nil
					case "empty-bits":
						data.BitArray.Elems = nil
					case "invalid-extra":
						data.BitArray.ExtraBitsStored = 9
					case "padding":
						data.BitArray.Elems[0] |= 1
					case "wrong-bit-count":
						data.BitArray.ExtraBitsStored = 4
					case "extra-byte":
						data.BitArray.Elems = append(data.BitArray.Elems, 0)
					case "zero-threshold":
						key.Threshold = 0
					}
					if nested {
						key = kmultisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{key})
						inner := data
						data = multisigtypes.NewMultisig(1)
						multisigtypes.AddSignature(data, inner, 0)
					}
					builder := config.config.NewTxBuilder()
					require.NoError(t, builder.SetMsgs(&banktypes.MsgSend{
						FromAddress: sdk.AccAddress(pubs[0].Address()).String(), ToAddress: sdk.AccAddress(pubs[1].Address()).String(),
						Amount: sdk.NewCoins(sdk.NewInt64Coin("stake", 1)),
					}))
					require.NoError(t, builder.SetSignatures(signing.SignatureV2{PubKey: key, Data: data}))
					wire, err := config.config.TxEncoder()(builder.GetTx())
					require.NoError(t, err)
					require.NotPanics(t, func() {
						decoded, err := config.config.TxDecoder()(wire)
						require.NoError(t, err, "transaction envelope must reach signature consumers")
						tx := decoded.(authsigning.Tx)
						sigs, conversionErr := tx.GetSignaturesV2()
						if conversionErr != nil {
							require.ErrorContains(t, conversionErr, "multisig")
							return
						}
						require.Len(t, sigs, 1)
						calls := 0
						err = sigs[0].PubKey.(*kmultisig.LegacyAminoPubKey).VerifyMultisignature(func(signing.SignMode) ([]byte, error) {
							calls++
							return []byte("malformed transport"), nil
						}, sigs[0].Data.(*signing.MultiSignatureData))
						require.Error(t, err)
						require.Zero(t, calls)
					})
				})
			}
		}
	}
}
