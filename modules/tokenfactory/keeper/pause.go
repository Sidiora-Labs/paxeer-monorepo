package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/prefix"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
)

var denomPausedValue = []byte{1}

// IsDenomPaused reports whether transfers, mints and burns of the denom are paused
func (k Keeper) IsDenomPaused(ctx sdk.Context, denom string) bool {
	return ctx.KVStore(k.storeKey).Has(types.GetDenomPausedKey(denom))
}

// setDenomPaused stores the pause flag of the denom; setting the current value is refused
func (k Keeper) setDenomPaused(ctx sdk.Context, denom string, paused bool) error {
	if k.IsDenomPaused(ctx, denom) == paused {
		return types.ErrPauseUnchanged
	}
	store := ctx.KVStore(k.storeKey)
	key := types.GetDenomPausedKey(denom)
	if paused {
		store.Set(key, denomPausedValue)
	} else {
		store.Delete(key)
	}
	return nil
}

func (k Keeper) GetPausedDenoms(ctx sdk.Context) []string {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(types.DenomPausedPrefixKey+types.KeySeparator))
	iterator := store.Iterator(nil, nil)
	defer func() { _ = iterator.Close() }()
	var denoms []string
	for ; iterator.Valid(); iterator.Next() {
		denoms = append(denoms, string(iterator.Key()))
	}
	return denoms
}

// SendEnabledChecker reports whether bank transfers of the denom are allowed
func (k Keeper) SendEnabledChecker(ctx sdk.Context, denom string) bool {
	return !k.IsDenomPaused(ctx, denom)
}
