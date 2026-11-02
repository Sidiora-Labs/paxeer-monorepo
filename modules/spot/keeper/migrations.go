package keeper

import (
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type Migrator struct{ keeper *Keeper }

func NewMigrator(keeper *Keeper) Migrator {
	return Migrator{keeper: keeper}
}

func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	cacheCtx, write := ctx.CacheContext()
	store := m.keeper.store(cacheCtx)
	for _, prefix := range [][]byte{types.ActiveLimitPrefix, types.ActiveStopLimitPrefix, types.ActiveStopLossPrefix} {
		ids, err := exportIDs(store, prefix)
		if err != nil {
			return err
		}
		count := uint64(len(ids))
		seen := make(map[uint64]bool, len(ids))
		for _, id := range ids {
			key := recordKey(prefix, orderIDBytes(id))
			position, err := activePosition(key, store.Get(key))
			if err != nil {
				return err
			}
			if id == 0 || position >= count {
				return fmt.Errorf("spot: invalid active index entry %x with position %d for length %d", key, position, count)
			}
			if seen[position] {
				return fmt.Errorf("spot: duplicate active index position %d in list %x", position, prefix)
			}
			seen[position] = true
			store.Set(types.ActivePositionKey(prefix, position), orderIDBytes(id))
		}
		store.Set(types.ActiveLengthKey(prefix), orderIDBytes(count))
	}
	write()
	return nil
}
