package multisig_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	kmultisig "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/multisig"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
)

func TestRepair_136_ReplaceKeys(t *testing.T) {
	a := secp256k1.GenPrivKeyFromSecret([]byte("repair-136-a")).PubKey()
	b := secp256k1.GenPrivKeyFromSecret([]byte("repair-136-b")).PubKey()
	c := ed25519.GenPrivKeyFromSecret([]byte("repair-136-c")).PubKey()
	nested := kmultisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{b, c})
	repair136RoundTrip(t,
		kmultisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{a, a}),
		kmultisig.NewLegacyAminoPubKey(2, []cryptotypes.PubKey{b, b}),
		kmultisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{c, b}),
		kmultisig.NewLegacyAminoPubKey(2, []cryptotypes.PubKey{b, c}),
		kmultisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{nested, c}),
	)
}

func TestRepair_136_GrowKeys(t *testing.T) {
	a := secp256k1.GenPrivKeyFromSecret([]byte("repair-136-grow-a")).PubKey()
	b := ed25519.GenPrivKeyFromSecret([]byte("repair-136-grow-b")).PubKey()
	repair136RoundTrip(t,
		kmultisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{a}),
		kmultisig.NewLegacyAminoPubKey(2, []cryptotypes.PubKey{b, a}),
		kmultisig.NewLegacyAminoPubKey(3, []cryptotypes.PubKey{a, b, a}),
	)
}

func TestRepair_136_ShrinkKeys(t *testing.T) {
	a := secp256k1.GenPrivKeyFromSecret([]byte("repair-136-shrink-a")).PubKey()
	b := ed25519.GenPrivKeyFromSecret([]byte("repair-136-shrink-b")).PubKey()
	repair136RoundTrip(t,
		kmultisig.NewLegacyAminoPubKey(3, []cryptotypes.PubKey{a, b, a}),
		kmultisig.NewLegacyAminoPubKey(2, []cryptotypes.PubKey{b, a}),
		kmultisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{b}),
	)
}

func repair136RoundTrip(t *testing.T, values ...*kmultisig.LegacyAminoPubKey) {
	t.Helper()
	for _, transport := range []string{"callbacks", "codec"} {
		t.Run(transport, func(t *testing.T) {
			cdc := codec.NewLegacyAmino()
			cryptocodec.RegisterCrypto(cdc)
			registry := types.NewInterfaceRegistry()
			cryptocodec.RegisterInterfaces(registry)
			protoCdc := codec.NewProtoCodec(registry)
			destination := kmultisig.NewLegacyAminoPubKey(int(values[0].Threshold), values[0].GetPubKeys())
			for _, source := range values {
				// Leave binary compatibility data from the previous value in the destination.
				_, err := cdc.Marshal(destination)
				require.NoError(t, err)
				sourceJSON, err := cdc.MarshalAsJSON(source)
				require.NoError(t, err)
				if transport == "callbacks" {
					amino, err := source.MarshalAminoJSON()
					require.NoError(t, err)
					require.NoError(t, destination.UnmarshalAminoJSON(amino))
					for i, key := range source.GetPubKeys() {
						expectedJSON, err := cdc.MarshalAsJSON(key)
						require.NoError(t, err)
						actualJSON, err := destination.PubKeys[i].MarshalJSON()
						require.NoError(t, err)
						require.JSONEq(t, string(expectedJSON), string(actualJSON))
					}
				} else {
					require.NoError(t, cdc.UnmarshalAsJSON(sourceJSON, destination))
				}

				require.Equal(t, source.Threshold, destination.Threshold)
				require.Len(t, destination.PubKeys, len(source.PubKeys))
				require.True(t, source.Equals(destination))
				for i, key := range source.GetPubKeys() {
					require.Equal(t, source.PubKeys[i].TypeUrl, destination.PubKeys[i].TypeUrl)
					require.Equal(t, source.PubKeys[i].Value, destination.PubKeys[i].Value)
					require.True(t, key.Equals(destination.GetPubKeys()[i]))
				}

				// Prove both subsequent transports consume the replacement caches.
				actualJSON, err := cdc.MarshalAsJSON(destination)
				require.NoError(t, err)
				require.JSONEq(t, string(sourceJSON), string(actualJSON))
				require.Equal(t, source.Bytes(), destination.Bytes())
				require.Equal(t, source.Address(), destination.Address())
				binary, err := cdc.Marshal(destination)
				require.NoError(t, err)
				var binaryKey kmultisig.LegacyAminoPubKey
				require.NoError(t, cdc.Unmarshal(binary, &binaryKey))
				require.True(t, source.Equals(&binaryKey))
				protoJSON, err := protoCdc.MarshalInterfaceJSON(destination)
				require.NoError(t, err)
				var protoKey cryptotypes.PubKey
				require.NoError(t, protoCdc.UnmarshalInterfaceJSON(protoJSON, &protoKey))
				require.True(t, source.Equals(protoKey))
			}
		})
	}
}
