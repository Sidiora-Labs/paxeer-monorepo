package types_test

import (
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/multisig"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
)

func repairM1405Registries() map[string]func() types.InterfaceRegistry {
	return map[string]func() types.InterfaceRegistry{
		"current": types.NewInterfaceRegistry,
		"legacy":  types.NewLegacyInterfaceRegistry,
	}
}

func repairM1405Key() cryptotypes.PubKey {
	return ed25519.GenPrivKeyFromSecret([]byte("repair-m1405-public-key-fixture")).PubKey()
}

func repairM1405RegisterKeys(registry types.InterfaceRegistry) {
	registry.RegisterInterface("cosmos.crypto.PubKey", (*cryptotypes.PubKey)(nil),
		&ed25519.PubKey{}, &multisig.LegacyAminoPubKey{})
}

func TestRepairM_1405_CustomAliasEnvelope(t *testing.T) {
	for name, newRegistry := range repairM1405Registries() {
		t.Run(name, func(t *testing.T) {
			registry := newRegistry()
			const alias = "/repair.m1405/custom-ed25519"
			registry.(interface {
				RegisterCustomTypeURL(interface{}, string, proto.Message)
			}).RegisterCustomTypeURL((*cryptotypes.PubKey)(nil), alias, &ed25519.PubKey{})
			key := repairM1405Key()
			packed, err := types.NewAnyWithValue(key)
			require.NoError(t, err)
			packed.TypeUrl = alias
			// Unknown varints in the payload (field 100) and envelope (field 101).
			packed.Value = append(packed.Value, 0xa0, 0x06, 0x01)
			packed.XXX_unrecognized = []byte{0xa8, 0x06, 0x02}
			payload := append([]byte(nil), packed.Value...)
			unknown := append([]byte(nil), packed.XXX_unrecognized...)
			cdc := codec.NewProtoCodec(registry)
			wire, err := cdc.Marshal(packed)
			require.NoError(t, err)

			var received types.Any
			require.NoError(t, cdc.Unmarshal(wire, &received))
			require.Nil(t, received.GetCachedValue())
			var decoded cryptotypes.PubKey
			require.NoError(t, registry.UnpackAny(&received, &decoded))
			require.True(t, key.Equals(decoded))
			require.Same(t, decoded, received.GetCachedValue())
			require.Equal(t, alias, received.TypeUrl)
			require.Equal(t, payload, received.Value)
			require.Equal(t, unknown, received.XXX_unrecognized)

			var cached cryptotypes.PubKey
			require.NoError(t, registry.UnpackAny(&received, &cached))
			require.Same(t, decoded, cached)
			reencoded, err := cdc.Marshal(&received)
			require.NoError(t, err)
			require.Equal(t, wire, reencoded)

			var second types.Any
			require.NoError(t, cdc.Unmarshal(reencoded, &second))
			require.Nil(t, second.GetCachedValue())
			var decodedAgain cryptotypes.PubKey
			require.NoError(t, registry.UnpackAny(&second, &decodedAgain))
			require.True(t, key.Equals(decodedAgain))
			require.NotSame(t, decoded, decodedAgain)
			require.Equal(t, alias, second.TypeUrl)
			require.Equal(t, payload, second.Value)
			require.Equal(t, unknown, second.XXX_unrecognized)
			secondWire, err := cdc.Marshal(&second)
			require.NoError(t, err)
			require.Equal(t, wire, secondWire)
		})
	}
}

func repairM1405NestedKey(depth int) cryptotypes.PubKey {
	key := repairM1405Key()
	for i := 1; i < depth; i++ {
		key = multisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{key})
	}
	return key
}

func repairM1405DecodeKey(t *testing.T, registry types.InterfaceRegistry, key cryptotypes.PubKey, wantError string) {
	t.Helper()
	cdc := codec.NewProtoCodec(registry)
	wire, err := cdc.MarshalInterface(key)
	require.NoError(t, err)
	var received types.Any
	require.NoError(t, cdc.Unmarshal(wire, &received))
	require.Nil(t, received.GetCachedValue())
	var decoded cryptotypes.PubKey
	err = registry.UnpackAny(&received, &decoded)
	if wantError != "" {
		require.EqualError(t, err, wantError)
		require.Nil(t, decoded)
		require.Nil(t, received.GetCachedValue())
	} else {
		require.NoError(t, err)
		require.True(t, key.Equals(decoded))
		require.Same(t, decoded, received.GetCachedValue())
	}
	reencoded, err := cdc.Marshal(&received)
	require.NoError(t, err)
	require.Equal(t, wire, reencoded)
}

func TestRepairM_1405_RecursionLimit(t *testing.T) {
	for name, newRegistry := range repairM1405Registries() {
		t.Run(name, func(t *testing.T) {
			registry := newRegistry()
			repairM1405RegisterKeys(registry)
			atLimit := repairM1405NestedKey(types.MaxUnpackAnyRecursionDepth)
			repairM1405DecodeKey(t, registry, atLimit, "")
			repairM1405DecodeKey(t, registry,
				repairM1405NestedKey(types.MaxUnpackAnyRecursionDepth+1), "max depth exceeded")
			// Each top-level operation gets a fresh budget, including after refusal.
			repairM1405DecodeKey(t, registry, atLimit, "")
		})
	}
}

func repairM1405BranchedKey(calls int) cryptotypes.PubKey {
	// One root, two branch nodes, and the remaining calls split over their leaves.
	leafCount := calls - 3
	branches := make([]cryptotypes.PubKey, 2)
	for i, count := range []int{leafCount / 2, leafCount - leafCount/2} {
		leaves := make([]cryptotypes.PubKey, count)
		for j := range leaves {
			leaves[j] = repairM1405Key()
		}
		branches[i] = multisig.NewLegacyAminoPubKey(1, leaves)
	}
	return multisig.NewLegacyAminoPubKey(1, branches)
}

func TestRepairM_1405_SharedCallLimit(t *testing.T) {
	for name, newRegistry := range repairM1405Registries() {
		t.Run(name, func(t *testing.T) {
			registry := newRegistry()
			repairM1405RegisterKeys(registry)
			atLimit := repairM1405BranchedKey(types.MaxUnpackAnySubCalls)
			repairM1405DecodeKey(t, registry, atLimit, "")
			repairM1405DecodeKey(t, registry,
				repairM1405BranchedKey(types.MaxUnpackAnySubCalls+1), "call limit exceeded")
			repairM1405DecodeKey(t, registry, atLimit, "")
		})
	}
}
