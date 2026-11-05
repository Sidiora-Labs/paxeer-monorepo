package keeper

import (
	"encoding/binary"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) error {
	return k.InitGenesisWithHistory(ctx, GenesisState{GenesisState: gs})
}

func (k Keeper) InitGenesisWithHistory(ctx sdk.Context, gs GenesisState) error {
	if err := gs.Validate(); err != nil {
		return err
	}
	if k.DomainCount(ctx) != 0 {
		return types.ErrInvalidGenesis.Wrap("registry already initialized")
	}
	for _, prefix := range [][]byte{types.DomainTxPrefix, types.DomainGasUsedPrefix} {
		it := sdk.KVStorePrefixIterator(ctx.KVStore(k.storeKey), prefix)
		hasHistory := it.Valid()
		it.Close()
		if hasHistory {
			return types.ErrInvalidGenesis.Wrap("domain history already initialized")
		}
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
	for _, history := range gs.DomainHistory {
		hashes := make([]byte, 0, len(history.TxHashes)*common.HashLength)
		for _, hash := range history.TxHashes {
			hashes = append(hashes, hash.Bytes()...)
		}
		used := make([]byte, 8)
		binary.BigEndian.PutUint64(used, history.GasUsed)
		cache.KVStore(k.storeKey).Set(types.HeightKey(types.DomainTxPrefix, history.DomainID, history.Height), hashes)
		cache.KVStore(k.storeKey).Set(types.HeightKey(types.DomainGasUsedPrefix, history.DomainID, history.Height), used)
	}
	k.setNextID(cache, gs.NextID)
	write()
	return nil
}
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	gs := k.ExportGenesisWithHistory(ctx)
	if len(gs.DomainHistory) != 0 {
		panic("layerxchains: domain history requires ExportGenesisWithHistory and InitGenesisWithHistory")
	}
	return &gs.GenesisState
}

func (k Keeper) exportRegistryGenesis(ctx sdk.Context) types.GenesisState {
	gs := &types.GenesisState{Params: k.GetParams(ctx), Domains: []types.Domain{}, NextID: k.NextID(ctx), NextBaseFees: map[uint32]sdk.Dec{}}
	k.IterateDomains(ctx, func(d types.Domain) bool {
		gs.Domains = append(gs.Domains, d)
		gs.NextBaseFees[d.Id] = k.NextBaseFee(ctx, d.Id)
		return false
	})
	return *gs
}
