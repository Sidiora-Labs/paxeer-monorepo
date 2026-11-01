package keeper_test

import (
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/stretchr/testify/require"
)

func TestReceiptKeyPinsMirrorStore(t *testing.T) {
	k, ctx, storeKey := newActivationKeeper(t)
	first, second := newReceiptSigner(0x52), newReceiptSigner(0x51)
	entries := keeper.ReceiptKeys{
		first.entry(uint64(math.MaxInt64)+1, math.MaxUint64),
		second.entry(5000, 6999),
	}
	require.NoError(t, k.SetReceiptKeys(ctx, entries))
	stored, err := k.ReceiptKeys(ctx)
	require.NoError(t, err)
	require.Len(t, stored, len(entries))

	pins, err := k.ReceiptKeyPins(ctx)
	require.NoError(t, err)
	require.Equal(t, []verify.SequencerAuthorization{
		{
			SequencerID:      first.keyID,
			PublicKey:        [32]byte(entries[0].PublicKey),
			FirstBatchNumber: uint64(math.MaxInt64) + 1,
			LastBatchNumber:  math.MaxUint64,
		},
		{
			SequencerID:      second.keyID,
			PublicKey:        [32]byte(entries[1].PublicKey),
			FirstBatchNumber: 5000,
			LastBatchNumber:  6999,
		},
	}, pins)
	for i, key := range stored {
		require.Equal(t, key.KeyId, pins[i].SequencerID)
		require.Equal(t, key.PublicKey, pins[i].PublicKey)
		require.Equal(t, key.FirstHeight, pins[i].FirstBatchNumber)
		require.Equal(t, key.LastHeight, pins[i].LastBatchNumber)
	}

	t.Run("corrupt store", func(t *testing.T) {
		ctx.KVStore(storeKey).Set(types.ReceiptKeysKey, []byte{1})
		pins, err := k.ReceiptKeyPins(ctx)
		require.ErrorIs(t, err, keeper.ErrCorruptStore)
		require.Nil(t, pins)
	})
}

func TestReceiptKeyPinsEmptyStore(t *testing.T) {
	k, ctx, storeKey := newActivationKeeper(t)
	require.Nil(t, ctx.KVStore(storeKey).Get(types.ReceiptKeysKey))
	pins, err := k.ReceiptKeyPins(ctx)
	require.NoError(t, err)
	require.Equal(t, []verify.SequencerAuthorization{}, pins)
}
