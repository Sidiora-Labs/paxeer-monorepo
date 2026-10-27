package layerxprograms

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestProgramsIdempotencyKeyDomainSeparated(t *testing.T) {
	var txHash [32]byte
	for i := range txHash {
		txHash[i] = byte(i)
	}
	selector := [4]byte{0x54, 0xbd, 0x2b, 0x48}
	const ordinal uint32 = 0x01020304
	preimage := append([]byte("layerxprograms/idempotency/v1"), txHash[:]...)
	preimage = append(preimage, 0x01, 0x02, 0x03, 0x04)
	preimage = append(preimage, selector[:]...)
	want := sha256.Sum256(preimage)
	got := IdempotencyKey(txHash, ordinal, selector)
	require.Equal(t, want, got)
	require.Equal(t, got, IdempotencyKey(txHash, ordinal, selector))
	require.NotEqual(t, [32]byte{}, got)
	require.NotEqual(t, sha256.Sum256(preimage[len("layerxprograms/idempotency/v1"):]), got)

	for i := range txHash {
		changed := txHash
		changed[i] ^= 1
		require.NotEqual(t, got, IdempotencyKey(changed, ordinal, selector))
	}
	for bit := uint32(0); bit < 32; bit++ {
		require.NotEqual(t, got, IdempotencyKey(txHash, ordinal^(1<<bit), selector))
	}
	for i := range selector {
		changed := selector
		changed[i] ^= 1
		require.NotEqual(t, got, IdempotencyKey(txHash, ordinal, changed))
	}
	zeroPreimage := append([]byte("layerxprograms/idempotency/v1"), make([]byte, 40)...)
	zeroInputs := IdempotencyKey([32]byte{}, 0, [4]byte{})
	require.Equal(t, sha256.Sum256(zeroPreimage), zeroInputs)
	require.NotEqual(t, [32]byte{}, zeroInputs)
}

func TestProgramsCallOrdinalPerTransaction(t *testing.T) {
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	persistentKey := sdk.NewKVStoreKey(evmtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(evmtypes.TransientStoreKey)
	otherKey := sdk.NewTransientStoreKey("other_transient")
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(persistentKey, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	ms.MountStoreWithDB(otherKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, ms.LoadLatestVersion())
	ctx := sdk.NewContext(ms, tmproto.Header{Height: 1}, false)
	ctx.KVStore(persistentKey).Set([]byte("sentinel"), []byte("committed"))
	ctx.TransientStore(otherKey).Set([]byte("sentinel"), []byte("unrelated"))

	txHash := [32]byte{1}
	selector := [4]byte{0x54, 0xbd, 0x2b, 0x48}
	first := nextOrdinal(ctx, txHash)
	require.Equal(t, uint32(0), first)
	nested, writeNested := ctx.CacheContext()
	second := nextOrdinal(nested, txHash)
	require.Equal(t, uint32(1), second)
	deeper, writeDeeper := nested.CacheContext()
	third := nextOrdinal(deeper, txHash)
	require.Equal(t, uint32(2), third)
	writeDeeper()
	writeNested()
	require.Equal(t, uint32(3), nextOrdinal(ctx, txHash))
	keys := map[[32]byte]bool{}
	for _, ordinal := range []uint32{first, second, third} {
		key := IdempotencyKey(txHash, ordinal, selector)
		require.False(t, keys[key])
		keys[key] = true
	}
	otherHash := txHash
	otherHash[31] = 1
	require.Equal(t, uint32(0), nextOrdinal(ctx, otherHash))
	require.Equal(t, uint32(4), nextOrdinal(ctx, txHash))
	require.Equal(t, uint32(1), nextOrdinal(ctx, otherHash))
	require.Equal(t, uint32(0), nextOrdinal(ctx, [32]byte{}))
	require.Equal(t, uint32(1), nextOrdinal(ctx, [32]byte{}))
	require.Equal(t, []byte("unrelated"), ctx.TransientStore(otherKey).Get([]byte("sentinel")))

	exhaustedHash := [32]byte{2}
	exhaustedKey := append([]byte(ordinalKeyPrefix), exhaustedHash[:]...)
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], math.MaxUint32-1)
	ctx.TransientStore(transientKey).Set(exhaustedKey, encoded[:])
	require.Equal(t, uint32(math.MaxUint32), nextOrdinal(ctx, exhaustedHash))
	require.PanicsWithValue(t, "layerxprograms: call ordinal exhausted", func() {
		nextOrdinal(ctx, exhaustedHash)
	})

	ms.Commit(true)
	transientIterator := ctx.TransientStore(transientKey).Iterator(nil, nil)
	require.False(t, transientIterator.Valid())
	require.NoError(t, transientIterator.Close())
	persistentIterator := ctx.KVStore(persistentKey).Iterator(nil, nil)
	require.True(t, persistentIterator.Valid())
	require.Equal(t, []byte("sentinel"), persistentIterator.Key())
	require.Equal(t, []byte("committed"), persistentIterator.Value())
	persistentIterator.Next()
	require.False(t, persistentIterator.Valid())
	require.NoError(t, persistentIterator.Close())
	require.Equal(t, uint32(0), nextOrdinal(ctx, txHash))

	reloaded := store.NewCommitMultiStore(db)
	reloaded.MountStoreWithDB(persistentKey, sdk.StoreTypeIAVL, db)
	reloaded.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, reloaded.LoadLatestVersion())
	reloadedCtx := sdk.NewContext(reloaded, tmproto.Header{Height: 2}, false)
	require.Equal(t, []byte("committed"), reloadedCtx.KVStore(persistentKey).Get([]byte("sentinel")))
	require.Equal(t, uint32(0), nextOrdinal(reloadedCtx, txHash))
	require.Equal(t, uint32(0), nextOrdinal(reloadedCtx, otherHash))
}
