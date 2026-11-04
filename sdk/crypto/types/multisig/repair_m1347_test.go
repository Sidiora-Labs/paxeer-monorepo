package multisig_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	multisigkeys "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/multisig"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types/multisig"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
)

var repairM1347Message = []byte("multisig-add-index production signing path")

func repairM1347Signatures(t *testing.T, size int) ([]cryptotypes.PubKey, []signing.SignatureData) {
	t.Helper()
	keys := make([]cryptotypes.PubKey, size)
	data := make([]signing.SignatureData, size)
	for i := range keys {
		keyBytes := make([]byte, secp256k1.PrivKeySize)
		keyBytes[len(keyBytes)-1] = byte(i + 1)
		privateKey := &secp256k1.PrivKey{Key: keyBytes}
		keys[i] = privateKey.PubKey()
		signature, err := privateKey.Sign(repairM1347Message)
		require.NoError(t, err)
		require.True(t, keys[i].VerifySignature(repairM1347Message, signature))
		data[i] = &signing.SingleSignatureData{
			SignMode: signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, Signature: signature,
		}
	}
	return keys, data
}

func repairM1347RejectUnchanged(t *testing.T, target *signing.MultiSignatureData, add func() error) {
	t.Helper()
	var before *signing.MultiSignatureData
	var backing []signing.SignatureData
	if target != nil {
		var bitArray *cryptotypes.CompactBitArray
		if target.BitArray != nil {
			copyArray := *target.BitArray
			copyArray.Elems = slices.Clone(target.BitArray.Elems)
			bitArray = &copyArray
		}
		before = &signing.MultiSignatureData{
			BitArray: bitArray, Signatures: slices.Clone(target.Signatures),
		}
		backing = slices.Clone(target.Signatures[:cap(target.Signatures)])
	}
	require.Error(t, add())
	require.Equal(t, before, target, "rejected addition must leave the target unchanged")
	if target != nil {
		require.Equal(t, backing, target.Signatures[:cap(target.Signatures)], "rejected append must not write into the backing array")
		for i := range before.Signatures {
			require.Same(t, before.Signatures[i], target.Signatures[i])
		}
	}
}

func TestRepairM_1347_MismatchedKeySets(t *testing.T) {
	keys, data := repairM1347Signatures(t, 18)
	for _, tc := range []struct{ size, keyCount, index int }{
		{1, 2, 1}, {7, 8, 7}, {8, 9, 8}, {8, 17, 16}, {16, 17, 16},
		{1, 9, 0}, {7, 9, 0}, {8, 9, 0}, {9, 8, 0}, {9, 10, 8}, {17, 9, 0}, {17, 16, 15},
	} {
		for _, populated := range []bool{false, true} {
			for _, api := range []string{"pubkey", "v2"} {
				t.Run(fmt.Sprintf("%d-bits/%d-keys/index-%d/populated-%t/%s", tc.size, tc.keyCount, tc.index, populated, api), func(t *testing.T) {
					target := multisig.NewMultisig(tc.size)
					if populated {
						require.NoError(t, multisig.AddSignature(target, data[0], 0))
					}
					repairM1347RejectUnchanged(t, target, func() error {
						if api == "pubkey" {
							return multisig.AddSignatureFromPubKey(target, data[tc.index], keys[tc.index], keys[:tc.keyCount])
						}
						return multisig.AddSignatureV2(target, signing.SignatureV2{PubKey: keys[tc.index], Data: data[tc.index]}, keys[:tc.keyCount])
					})
				})
			}
		}
	}
}

func TestRepairM_1347_InvalidIndices(t *testing.T) {
	_, data := repairM1347Signatures(t, 1)
	for _, size := range []int{1, 7, 8, 9, 16, 17} {
		for _, index := range []int{-1, size, size + 8, int(^uint(0) >> 1)} {
			t.Run(fmt.Sprintf("%d-bits/index-%d", size, index), func(t *testing.T) {
				target := multisig.NewMultisig(size)
				require.NoError(t, multisig.AddSignature(target, data[0], 0))
				repairM1347RejectUnchanged(t, target, func() error {
					return multisig.AddSignature(target, data[0], index)
				})
			})
		}
	}
}

func TestRepairM_1347_InvalidTargets(t *testing.T) {
	keys, data := repairM1347Signatures(t, 9)
	for name, corrupt := range map[string]func(*signing.MultiSignatureData) *signing.MultiSignatureData{
		"nil-target": func(_ *signing.MultiSignatureData) *signing.MultiSignatureData { return nil },
		"nil-array":  func(m *signing.MultiSignatureData) *signing.MultiSignatureData { m.BitArray = nil; return m },
		"empty-array": func(m *signing.MultiSignatureData) *signing.MultiSignatureData {
			m.BitArray = &cryptotypes.CompactBitArray{}
			return m
		},
		"empty-with-extra": func(m *signing.MultiSignatureData) *signing.MultiSignatureData {
			m.BitArray = &cryptotypes.CompactBitArray{ExtraBitsStored: 1}
			return m
		},
		"extra-eight": func(m *signing.MultiSignatureData) *signing.MultiSignatureData {
			m.BitArray.ExtraBitsStored = 8
			return m
		},
		"extra-overflow": func(m *signing.MultiSignatureData) *signing.MultiSignatureData {
			m.BitArray.ExtraBitsStored = ^uint32(0)
			return m
		},
		"nonzero-padding": func(m *signing.MultiSignatureData) *signing.MultiSignatureData { m.BitArray.Elems[1] |= 1; return m },
		"missing-signature": func(m *signing.MultiSignatureData) *signing.MultiSignatureData {
			m.Signatures = m.Signatures[:0]
			return m
		},
		"surplus-signature": func(m *signing.MultiSignatureData) *signing.MultiSignatureData {
			m.Signatures = append(m.Signatures, data[1])
			return m
		},
	} {
		for _, api := range []string{"index", "pubkey", "v2"} {
			t.Run(name+"/"+api, func(t *testing.T) {
				target := multisig.NewMultisig(9)
				require.NoError(t, multisig.AddSignature(target, data[0], 0))
				target = corrupt(target)
				repairM1347RejectUnchanged(t, target, func() error {
					switch api {
					case "index":
						return multisig.AddSignature(target, data[0], 0)
					case "pubkey":
						return multisig.AddSignatureFromPubKey(target, data[0], keys[0], keys)
					default:
						return multisig.AddSignatureV2(target, signing.SignatureV2{PubKey: keys[0], Data: data[0]}, keys)
					}
				})
			})
		}
	}
	for _, api := range []string{"index", "pubkey", "v2"} {
		t.Run("nil-signature/"+api, func(t *testing.T) {
			target := multisig.NewMultisig(9)
			repairM1347RejectUnchanged(t, target, func() error {
				switch api {
				case "index":
					return multisig.AddSignature(target, nil, 0)
				case "pubkey":
					return multisig.AddSignatureFromPubKey(target, nil, keys[0], keys)
				default:
					return multisig.AddSignatureV2(target, signing.SignatureV2{PubKey: keys[0]}, keys)
				}
			})
		})
	}
	t.Run("unknown-key", func(t *testing.T) {
		target := multisig.NewMultisig(8)
		repairM1347RejectUnchanged(t, target, func() error {
			return multisig.AddSignatureFromPubKey(target, data[8], keys[8], keys[:8])
		})
	})
}

func TestRepairM_1347_SignEncodeVerify(t *testing.T) {
	keys, data := repairM1347Signatures(t, 18)
	for _, size := range []int{1, 7, 8, 9, 16, 17} {
		t.Run(fmt.Sprintf("%d-keys", size), func(t *testing.T) {
			publicKey := multisigkeys.NewLegacyAminoPubKey(size, keys[:size])
			target := multisig.NewMultisig(size)
			order := []int{size - 1, 0, size / 2}
			for i := size - 1; i >= 0; i-- {
				order = append(order, i)
			}
			for j, index := range order {
				switch j % 3 {
				case 0:
					require.NoError(t, multisig.AddSignature(target, data[index], index))
				case 1:
					require.NoError(t, multisig.AddSignatureFromPubKey(target, data[index], keys[index], keys[:size]))
				case 2:
					require.NoError(t, multisig.AddSignatureV2(target, signing.SignatureV2{PubKey: keys[index], Data: data[index]}, keys[:size]))
				}
			}
			require.Len(t, target.Signatures, size)
			for i := 0; i < size; i++ {
				require.True(t, target.BitArray.GetIndex(i))
				require.Same(t, data[i], target.Signatures[i], "signatures must follow public key order")
			}
			for api, index := range []int{0, size / 2, size - 1} {
				replacement := *data[index].(*signing.SingleSignatureData)
				replacement.Signature = slices.Clone(replacement.Signature)
				switch api {
				case 0:
					require.NoError(t, multisig.AddSignature(target, &replacement, index))
				case 1:
					require.NoError(t, multisig.AddSignatureFromPubKey(target, &replacement, keys[index], keys[:size]))
				case 2:
					require.NoError(t, multisig.AddSignatureV2(target, signing.SignatureV2{PubKey: keys[index], Data: &replacement}, keys[:size]))
				}
				require.Len(t, target.Signatures, size)
				require.Same(t, &replacement, target.Signatures[index], "replacement must store the new signature")
			}
			repairM1347RejectUnchanged(t, target, func() error {
				return multisig.AddSignatureV2(target, signing.SignatureV2{PubKey: keys[0], Data: data[0]}, keys[:size+1])
			})
			wire, err := signing.SignatureDataToProto(target).Marshal()
			require.NoError(t, err)
			var descriptor signing.SignatureDescriptor_Data
			require.NoError(t, descriptor.Unmarshal(wire))
			recovered, ok := signing.SignatureDataFromProto(&descriptor).(*signing.MultiSignatureData)
			require.True(t, ok)
			require.Equal(t, target, recovered)
			require.NoError(t, publicKey.VerifyMultisignature(func(mode signing.SignMode) ([]byte, error) {
				require.Equal(t, signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, mode)
				return repairM1347Message, nil
			}, recovered))
			require.ErrorContains(t, publicKey.VerifyMultisignature(func(signing.SignMode) ([]byte, error) {
				return []byte("different signed message"), nil
			}, recovered), "unable to verify signature")
		})
	}
}
