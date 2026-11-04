package ed25519_test

import (
	"bytes"
	stded25519 "crypto/ed25519"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
)

func TestRepairM_1344_RejectInconsistentImports(t *testing.T) {
	cdc := codec.NewLegacyAmino()
	source := ed25519.GenPrivKeyFromSecret([]byte("repair m1344 source"))
	other := ed25519.GenPrivKeyFromSecret([]byte("repair m1344 other"))
	encodings := []struct {
		name      string
		marshal   func(any) ([]byte, error)
		unmarshal func([]byte, any) error
	}{
		{"binary", cdc.Marshal, cdc.Unmarshal},
		{"json", cdc.MarshalAsJSON, cdc.UnmarshalAsJSON},
	}
	malformed := make(map[string][]byte)
	for i := ed25519.SeedSize; i < ed25519.PrivKeySize; i++ {
		key := append([]byte(nil), source.Bytes()...)
		key[i] ^= 1
		malformed[fmt.Sprintf("suffix_byte_%d", i-ed25519.SeedSize)] = key
	}
	zeroSuffix := append([]byte(nil), source.Bytes()...)
	clear(zeroSuffix[ed25519.SeedSize:])
	malformed["zero_suffix"] = zeroSuffix
	otherSuffix := append([]byte(nil), source.Bytes()...)
	copy(otherSuffix[ed25519.SeedSize:], other.PubKey().Bytes())
	malformed["other_public_key"] = otherSuffix
	changedSeed := append([]byte(nil), source.Bytes()...)
	changedSeed[0] ^= 1
	malformed["changed_seed"] = changedSeed

	for _, encoding := range encodings {
		for name, key := range malformed {
			t.Run(encoding.name+"/"+name, func(t *testing.T) {
				wire, err := encoding.marshal(&ed25519.PrivKey{Key: key})
				require.NoError(t, err)
				for _, initialized := range []bool{false, true} {
					t.Run(fmt.Sprintf("initialized_%t", initialized), func(t *testing.T) {
						var imported ed25519.PrivKey
						if initialized {
							imported.Key = append([]byte(nil), other.Bytes()...)
						}
						before := append(stded25519.PrivateKey(nil), imported.Key...)
						require.ErrorContains(t, encoding.unmarshal(wire, &imported), "invalid privkey public suffix")
						require.Equal(t, before, imported.Key, "rejected import must preserve the receiver")
					})
				}
			})
		}
	}
}

func TestRepairM_1344_ImportSignVerify(t *testing.T) {
	// RFC 8032 section 7.1, test vector 1: seed followed by its public key.
	fixture, err := hex.DecodeString("9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60" +
		"d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a")
	require.NoError(t, err)
	keys := []struct {
		name string
		key  *ed25519.PrivKey
	}{
		{"rfc8032", &ed25519.PrivKey{Key: fixture}},
		{"sdk_generated", ed25519.GenPrivKeyFromSecret([]byte("repair m1344 valid import"))},
		{"zero_seed", &ed25519.PrivKey{Key: stded25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))}},
	}
	cdc := codec.NewLegacyAmino()
	encodings := []struct {
		name      string
		marshal   func(any) ([]byte, error)
		unmarshal func([]byte, any) error
	}{
		{"binary", cdc.Marshal, cdc.Unmarshal},
		{"json", cdc.MarshalAsJSON, cdc.UnmarshalAsJSON},
	}
	for _, key := range keys {
		for _, encoding := range encodings {
			t.Run(key.name+"/"+encoding.name, func(t *testing.T) {
				wire, err := encoding.marshal(key.key)
				require.NoError(t, err)
				var imported ed25519.PrivKey
				require.NoError(t, encoding.unmarshal(wire, &imported))
				require.Equal(t, key.key.Bytes(), imported.Bytes())
				require.True(t, key.key.PubKey().Equals(imported.PubKey()))
				roundTrip, err := encoding.marshal(&imported)
				require.NoError(t, err)
				require.Equal(t, wire, roundTrip)

				for _, msg := range [][]byte{nil, []byte("repair m1344 signing message")} {
					sig, err := imported.Sign(msg)
					require.NoError(t, err)
					require.Len(t, sig, ed25519.SignatureSize)
					require.Equal(t, stded25519.Sign(stded25519.PrivateKey(key.key.Bytes()), msg), sig)
					require.True(t, imported.PubKey().VerifySignature(msg, sig))
					require.True(t, stded25519.Verify(stded25519.PublicKey(key.key.PubKey().Bytes()), msg, sig))
					require.False(t, imported.PubKey().VerifySignature(append([]byte{1}, msg...), sig))
				}
			})
		}
	}
}

func TestRepairM_1344_ImportOwnsValidatedBytes(t *testing.T) {
	source := ed25519.GenPrivKeyFromSecret([]byte("repair m1344 validated bytes"))
	for _, json := range []bool{false, true} {
		t.Run(fmt.Sprintf("json_%t", json), func(t *testing.T) {
			var imported ed25519.PrivKey
			marshal := source.MarshalAmino
			unmarshal := imported.UnmarshalAmino
			if json {
				marshal = source.MarshalAminoJSON
				unmarshal = imported.UnmarshalAminoJSON
			}
			key, err := marshal()
			require.NoError(t, err)
			input := append([]byte(nil), key...)
			require.NoError(t, unmarshal(input))
			require.Equal(t, key, imported.Bytes())
			input[0] ^= 1
			input[ed25519.SeedSize] ^= 1
			require.Equal(t, key, imported.Bytes(), "input mutation must not invalidate an accepted key")
			msg := []byte("validated import stays consistent")
			sig, err := imported.Sign(msg)
			require.NoError(t, err)
			require.True(t, source.PubKey().VerifySignature(msg, sig))
		})
	}
}

func TestRepairM_1344_RejectInvalidLengths(t *testing.T) {
	for _, size := range []int{0, 31, 32, 63, 65, 128} {
		for _, json := range []bool{false, true} {
			t.Run(fmt.Sprintf("size_%d/json_%t", size, json), func(t *testing.T) {
				imported := ed25519.GenPrivKeyFromSecret([]byte("repair m1344 length rejection"))
				before := append([]byte(nil), imported.Bytes()...)
				unmarshal := imported.UnmarshalAmino
				if json {
					unmarshal = imported.UnmarshalAminoJSON
				}
				require.ErrorContains(t, unmarshal(bytes.Repeat([]byte{1}, size)), "invalid privkey size")
				require.Equal(t, before, imported.Bytes())
			})
		}
	}
}
