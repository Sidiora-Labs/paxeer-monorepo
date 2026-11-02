package keeper

import (
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k Keeper) StoreVotes(ctx sdk.Context, height int64, votes []abci.VoteInfo) error {
	bz, err := types.EncodeStoredVotes(votes)
	if err != nil {
		return err
	}
	store := k.store(ctx)
	store.Set(types.StoredVotesKeyAt(height), bz)
	if height > 2 {
		store.Delete(types.StoredVotesKeyAt(height - 2))
	}
	return nil
}

func (k Keeper) StoredVotes(ctx sdk.Context, height int64) ([]abci.VoteInfo, error) {
	bz := k.store(ctx).Get(types.StoredVotesKeyAt(height))
	if bz == nil {
		return []abci.VoteInfo{}, nil
	}
	votes, err := types.DecodeStoredVotes(bz)
	if err != nil {
		return nil, ErrCorruptStore.Wrapf("stored votes at height %d: %v", height, err)
	}
	return votes, nil
}
