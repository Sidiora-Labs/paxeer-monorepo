package sequencer_test

import (
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func receiptPinsKeeper(t *testing.T) (seqkeeper.Keeper, sdk.Context) {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ms := store.NewCommitMultiStore(db)
	key := sdk.NewKVStoreKey(seqtypes.StoreKey)
	ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, ms.LoadLatestVersion())
	return seqkeeper.NewKeeper(key), sdk.NewContext(ms, tmproto.Header{Height: 4}, false)
}

func TestReceiptKeyFollowsRotatedPins(t *testing.T) {
	keeper, ctx := receiptPinsKeeper(t)
	public := publicOf(testKeySeed)
	original := seqkeeper.ReceiptKeyEntry{
		KeyId: testKeyID[:], PublicKey: public[:], FirstHeight: 1, LastHeight: 100,
	}
	require.NoError(t, keeper.SetReceiptKeys(ctx, seqkeeper.ReceiptKeys{original}))
	pins, err := keeper.ReceiptKeyPins(ctx)
	require.NoError(t, err)
	key, err := loadKey(t, testKeySeed, pins, 4)
	require.NoError(t, err)
	id, err := key.KeyID(5)
	require.NoError(t, err)
	require.Equal(t, testKeyID, id)

	nextID := [32]byte(label("refreshed receipt key id"))
	foreignID := [32]byte(label("foreign receipt key id"))
	foreignPublic := publicOf(label("foreign receipt key seed"))
	original.LastHeight = 4
	next := seqkeeper.ReceiptKeyEntry{
		KeyId: nextID[:], PublicKey: public[:], FirstHeight: 5, LastHeight: 9,
	}
	rotated := seqkeeper.ReceiptKeys{
		{KeyId: foreignID[:], PublicKey: foreignPublic[:], FirstHeight: 5, LastHeight: 100},
		original, next,
	}
	require.NoError(t, seqkeeper.ValidateRotation(seqkeeper.ReceiptKeys{{
		KeyId: testKeyID[:], PublicKey: public[:], FirstHeight: 1, LastHeight: 100,
	}}, rotated, 4))
	require.NoError(t, keeper.SetReceiptKeys(ctx, rotated))
	pins, err = keeper.ReceiptKeyPins(ctx)
	require.NoError(t, err)
	require.NoError(t, key.SetPins(pins))
	for _, height := range []int64{1, 4, 5, 9} {
		id, err = key.KeyID(height)
		require.NoError(t, err)
		want := testKeyID
		if height >= 5 {
			want = nextID
		}
		require.Equal(t, want, id, "height %d", height)
	}
	for _, height := range []int64{-1, 0, 10, 100} {
		_, err = key.KeyID(height)
		require.ErrorIs(t, err, sequencer.ErrReceiptKeyNotPinned)
	}
	pins[2].SequencerID = foreignID
	id, err = key.KeyID(5)
	require.NoError(t, err)
	require.Equal(t, nextID, id)
	require.Equal(t, public, key.PublicKey())

	require.NoError(t, keeper.SetReceiptKeys(ctx, seqkeeper.ReceiptKeys{next}))
	pins, err = keeper.ReceiptKeyPins(ctx)
	require.NoError(t, err)
	require.NoError(t, key.SetPins(pins))
	_, err = key.KeyID(4)
	require.ErrorIs(t, err, sequencer.ErrReceiptKeyNotPinned)
	id, err = key.KeyID(5)
	require.NoError(t, err)
	require.Equal(t, nextID, id)
}

func TestReceiptKeyRefusesForeignPins(t *testing.T) {
	keeper, ctx := receiptPinsKeeper(t)
	public := publicOf(testKeySeed)
	require.NoError(t, keeper.SetReceiptKeys(ctx, seqkeeper.ReceiptKeys{{
		KeyId: testKeyID[:], PublicKey: public[:], FirstHeight: 1, LastHeight: 100,
	}}))
	pins, err := keeper.ReceiptKeyPins(ctx)
	require.NoError(t, err)
	key, err := loadKey(t, testKeySeed, pins, 4)
	require.NoError(t, err)
	foreignID := [32]byte(label("foreign receipt key id"))
	foreignPublic := publicOf(label("foreign receipt key seed"))
	require.NoError(t, keeper.SetReceiptKeys(ctx, seqkeeper.ReceiptKeys{{
		KeyId: foreignID[:], PublicKey: foreignPublic[:], FirstHeight: 1, LastHeight: 100,
	}}))
	foreign, err := keeper.ReceiptKeyPins(ctx)
	require.NoError(t, err)
	validReplacement := pins[0]
	validReplacement.SequencerID = [32]byte(label("replacement receipt key id"))
	zeroID := pins[0]
	zeroID.SequencerID = [32]byte{}
	inverted := pins[0]
	inverted.FirstBatchNumber = inverted.LastBatchNumber + 1
	invalidForeign := foreign[0]
	invalidForeign.SequencerID = [32]byte{}
	for name, rejected := range map[string][]verify.SequencerAuthorization{
		"nil":                   nil,
		"empty":                 {},
		"foreign":               foreign,
		"zero id":               {validReplacement, zeroID},
		"inverted range":        {validReplacement, inverted},
		"invalid foreign entry": {validReplacement, invalidForeign},
	} {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, key.SetPins(rejected), sequencer.ErrReceiptKeyPin)
			id, err := key.KeyID(4)
			require.NoError(t, err)
			require.Equal(t, testKeyID, id)
			require.Equal(t, public, key.PublicKey())
		})
	}
}
