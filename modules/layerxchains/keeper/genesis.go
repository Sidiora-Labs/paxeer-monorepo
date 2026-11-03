package keeper

import (
	"encoding/binary"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) error {
	if err := gs.Validate(); err != nil {
		return err
	}
	if k.DomainCount(ctx) != 0 {
		return types.ErrInvalidGenesis.Wrap("registry already initialized")
	}
	cache, write := ctx.CacheContext()
	k.SetParams(cache, gs.Params)
	for _, d := range gs.Domains {
		k.setDomain(cache, d)
		index := make([]byte, 4)
		binary.BigEndian.PutUint32(index, d.Id)
		cache.KVStore(k.storeKey).Set(types.NameIndexKey(d.Name), index)
		k.SetNextBaseFee(cache, d.Id, gs.NextBaseFees[d.Id])
	}
	k.setNextID(cache, gs.NextID)
	write()
	return nil
}
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	gs := &types.GenesisState{Params: k.GetParams(ctx), Domains: []types.Domain{}, NextID: k.NextID(ctx), NextBaseFees: map[uint32]sdk.Dec{}}
	k.IterateDomains(ctx, func(d types.Domain) bool {
		gs.Domains = append(gs.Domains, d)
		gs.NextBaseFees[d.Id] = k.NextBaseFee(ctx, d.Id)
		return false
	})
	return gs
}
