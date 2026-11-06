package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	keeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// Recording command: hf-done (the Rust compiled_key_rotation_binds_exact_payload_and_idempotency_key
// test emits compiled_key_rotation_hex and compares its compiler output with this constant).
const repair2231496RotationHex = "71020004f10a99b0747f5f086b7300447f26f27e3ee30d10d34828e00c377b8484fdaf287ea0e3bd52e207c9d3b0eba65c0704e66fca2d8e165a175218b174fc4160e413000001a3185c53e8000001a3185c57d00000000000000064"

func repair2231496Fixture(t *testing.T) (*keeper.Harness, keeper.Actor) {
	t.Helper()
	f := newHarness(t)
	*f.Context() = f.Context().WithChainID("hyperpax_125-1")
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{74}, ed25519.SeedSize))
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	caller := common.Address{0x26, 0x49, 0x6}
	evm := &testkeeper.EVMTestApp.EvmKeeper
	message, err := evmtypes.LayerXBindMessage(evm.ChainID(*f.Context()), caller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = evm.BindLayerX(*f.Context(), caller, public, signature)
	require.NoError(t, err)
	actor, err := f.Keeper().Actor(*f.Context(), caller)
	require.NoError(t, err)
	require.True(t, actor.Authority.Bound)
	require.Equal(t, public, actor.Authority.VerifiedKey)
	registration := append([]byte{0x71, 1, 0, 2}, actor.Authority.Actor[:]...)
	registration = append(registration, public[:]...)
	result, err := f.Keeper().RecordLifecycle(*f.Context(), actor, registration, [32]byte{0x48})
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), result.ResultCode)
	require.NotEmpty(t, result.Encoded)
	return f, actor
}

func TestRepair_22_31_496_CompiledRotationReachesKernel(t *testing.T) {
	f, actor := repair2231496Fixture(t)
	payload, err := hex.DecodeString(repair2231496RotationHex)
	require.NoError(t, err)
	require.Len(t, payload, 92)
	require.Equal(t, []byte{0x71, 2, 0, 4}, payload[:4])
	require.Equal(t, actor.Authority.Actor[:], payload[4:36])
	before, err := f.Keeper().TestHost().ReadIdentityState(*f.Context(), actor.DID)
	require.NoError(t, err)
	require.False(t, before.HasPendingKey)
	sequence, err := lxphost.NextSequence(*f.Context(), f.HostKey())
	require.NoError(t, err)

	result, err := f.Keeper().RecordLifecycle(*f.Context(), actor, payload, [32]byte{0x49})
	require.NotEmpty(t, result.Encoded, "the compiler payload must reach the real kernel")
	require.NoError(t, err, "governance/2 compiler payload returned kernel code %d", result.ResultCode)
	require.Equal(t, lxphost.Code(0), result.ResultCode)
	require.Equal(t, sequence, result.GlobalSequence)
	require.Equal(t, uint64(f.Context().BlockTime().UnixMilli()), result.Timestamp)
	require.NotEmpty(t, result.Events)

	after, err := f.Keeper().TestHost().ReadIdentityState(*f.Context(), actor.DID)
	require.NoError(t, err)
	pendingPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{75}, ed25519.SeedSize))
	var pending [32]byte
	copy(pending[:], pendingPrivate.Public().(ed25519.PublicKey))
	require.NotEqual(t, before.PrimaryKey, pending)
	require.Equal(t, pending[:], payload[36:68])
	require.True(t, after.HasPendingKey)
	require.Equal(t, pending, after.PendingKey)
	require.Equal(t, before.PrimaryKey, after.PrimaryKey)
	require.Equal(t, uint64(1_800_000_001_000), after.RotationEffectiveAt)
	require.Equal(t, uint64(1_800_000_002_000), after.RotationLapseAt)
	require.Equal(t, uint64(100), after.RotationEffectiveSequence)
	require.Equal(t, before.NextSequence+1, after.NextSequence)
	revocation, err := f.Keeper().RevocationSequence(*f.Context(), actor.DIDKey)
	require.NoError(t, err)
	require.Equal(t, after.RevocationSequence, revocation)
}

func TestRepair_22_31_496_RotationWrongLengthIsNonCanonical(t *testing.T) {
	f, actor := repair2231496Fixture(t)
	payload, err := hex.DecodeString(repair2231496RotationHex)
	require.NoError(t, err)
	before := authViewStores(*f.Context(), f.HostKey(), f.StoreKey())
	for _, length := range []int{4, 68, 70, 86, 91, 93, 1024, 1025} {
		malformed := make([]byte, length)
		copy(malformed, payload)
		result, err := f.Keeper().RecordLifecycle(*f.Context(), actor, malformed, [32]byte{0x49})
		require.ErrorIs(t, err, types.ErrNonCanonical, "rotation payload length %d", length)
		require.Empty(t, result.Encoded, "wrong length must be refused before the kernel")
		require.Equal(t, before, authViewStores(*f.Context(), f.HostKey(), f.StoreKey()))
	}
	for _, mutation := range [][2]byte{{0, 0x70}, {1, 4}, {2, 1}, {2, 2}, {3, 3}, {3, 5}} {
		malformed := bytes.Clone(payload)
		malformed[mutation[0]] = mutation[1]
		result, err := f.Keeper().RecordLifecycle(*f.Context(), actor, malformed, [32]byte{0x49})
		require.ErrorIs(t, err, types.ErrNonCanonical)
		require.Empty(t, result.Encoded)
		require.Equal(t, before, authViewStores(*f.Context(), f.HostKey(), f.StoreKey()))
	}
}
