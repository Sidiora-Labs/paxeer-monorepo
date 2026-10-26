package keeper

import (
	"bytes"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
)

func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) error {
	if err := gs.Validate(); err != nil {
		return err
	}
	k.SetParams(ctx, gs.Params)
	store := ctx.KVStore(k.hostKey)
	for _, record := range gs.KernelRecords {
		store.Set(record.Key, append([]byte{}, record.Value...))
	}
	return nil
}

func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	gs := &types.GenesisState{Params: k.GetParams(ctx)}
	store := ctx.KVStore(k.hostKey)
	for _, prefix := range [][]byte{
		types.EscrowKVPrefix, types.BudgetKVPrefix, types.StreamKVPrefix,
		types.ServiceKVPrefix, types.GovernanceKVPrefix,
	} {
		iterator := sdk.KVStorePrefixIterator(store, prefix)
		for ; iterator.Valid(); iterator.Next() {
			gs.KernelRecords = append(gs.KernelRecords, kv.Pair{
				Key: bytes.Clone(iterator.Key()), Value: bytes.Clone(iterator.Value()),
			})
		}
		iterator.Close()
	}
	return gs
}
