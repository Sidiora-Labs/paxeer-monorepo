package keeper_test

import (
	"bytes"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func storedVotesTestStore(t *testing.T, f fixture) sdk.KVStore {
	t.Helper()
	for _, key := range f.ctx.MultiStore().StoreKeys() {
		if key.Name() == types.StoreKey {
			return f.ctx.KVStore(key)
		}
	}
	t.Fatal("sequencer store is not mounted")
	return nil
}

func TestStoredVotesRoundTrip(t *testing.T) {
	f := newFixture(t)
	store := storedVotesTestStore(t, f)
	votes := []abci.VoteInfo{
		{Validator: abci.Validator{Address: bytes.Repeat([]byte{0x11}, 20), Power: 7}, SignedLastBlock: true},
		{Validator: abci.Validator{Address: bytes.Repeat([]byte{0x22}, 20), Power: 13}, SignedLastBlock: false},
	}
	require.NoError(t, f.keeper.StoreVotes(f.ctx, 9, votes))
	encoded, err := types.EncodeStoredVotes(votes)
	require.NoError(t, err)
	require.Equal(t, encoded, store.Get(types.StoredVotesKeyAt(9)))
	require.False(t, store.Has(types.StoredVotesKey))

	got, err := f.keeper.StoredVotes(f.ctx, 9)
	require.NoError(t, err)
	require.Equal(t, votes, got)
	missing, err := f.keeper.StoredVotes(f.ctx, 8)
	require.NoError(t, err)
	require.NotNil(t, missing)
	require.Empty(t, missing)

	require.NoError(t, f.keeper.StoreVotes(f.ctx, 9, votes[1:]))
	got, err = f.keeper.StoredVotes(f.ctx, 9)
	require.NoError(t, err)
	require.Equal(t, votes[1:], got)

	for _, empty := range [][]abci.VoteInfo{nil, {}} {
		require.NoError(t, f.keeper.StoreVotes(f.ctx, 10, empty))
		require.Equal(t, []byte{0, 0, 0, 0}, store.Get(types.StoredVotesKeyAt(10)))
		got, err = f.keeper.StoredVotes(f.ctx, 10)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Empty(t, got)
	}

	invalid := []abci.VoteInfo{{Validator: abci.Validator{Address: bytes.Repeat([]byte{0x33}, 256)}}}
	require.Error(t, f.keeper.StoreVotes(f.ctx, 11, invalid))
	require.False(t, store.Has(types.StoredVotesKeyAt(11)))
	got, err = f.keeper.StoredVotes(f.ctx, 9)
	require.NoError(t, err)
	require.Equal(t, votes[1:], got, "failed encoding must not prune existing votes")
	require.Error(t, f.keeper.StoreVotes(f.ctx, 9, invalid))
	got, err = f.keeper.StoredVotes(f.ctx, 9)
	require.NoError(t, err)
	require.Equal(t, votes[1:], got, "failed encoding must not replace existing votes")
}

func TestStoredVotesKeepsTwoHeights(t *testing.T) {
	f := newFixture(t)
	store := storedVotesTestStore(t, f)
	for height := int64(1); height <= 5; height++ {
		votes := []abci.VoteInfo{{
			Validator:       abci.Validator{Address: bytes.Repeat([]byte{byte(height)}, 20), Power: height},
			SignedLastBlock: height%2 == 0,
		}}
		require.NoError(t, f.keeper.StoreVotes(f.ctx, height, votes))
		for storedHeight := int64(1); storedHeight <= height; storedHeight++ {
			got, err := f.keeper.StoredVotes(f.ctx, storedHeight)
			require.NoError(t, err)
			if storedHeight < height-1 {
				require.NotNil(t, got)
				require.Empty(t, got, "height %d should be pruned at height %d", storedHeight, height)
				require.False(t, store.Has(types.StoredVotesKeyAt(storedHeight)))
				continue
			}
			require.Equal(t, []abci.VoteInfo{{
				Validator:       abci.Validator{Address: bytes.Repeat([]byte{byte(storedHeight)}, 20), Power: storedHeight},
				SignedLastBlock: storedHeight%2 == 0,
			}}, got)
		}
	}
}

func TestStoredVotesCorruptEntry(t *testing.T) {
	f := newFixture(t)
	store := storedVotesTestStore(t, f)
	for _, corrupt := range [][]byte{
		{0x00, 0x00, 0x00, 0x01},
		{},
		{0x00, 0x00, 0x00},
		{0x00, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2},
		{0x00, 0x00, 0x00, 0x00, 0xff},
	} {
		store.Set(types.StoredVotesKeyAt(7), corrupt)
		got, err := f.keeper.StoredVotes(f.ctx, 7)
		require.ErrorIs(t, err, keeper.ErrCorruptStore)
		require.Nil(t, got)
		require.Equal(t, corrupt, store.Get(types.StoredVotesKeyAt(7)))
	}
}
