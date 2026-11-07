package keeper

import (
	"encoding/binary"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

type DomainHistory struct {
	DomainID uint32        `json:"domain_id"`
	Height   uint64        `json:"height"`
	TxHashes []common.Hash `json:"tx_hashes"`
	GasUsed  uint64        `json:"gas_used"`
}

type GenesisState struct {
	types.GenesisState
	DomainHistory []DomainHistory `json:"domain_history,omitempty"`
}

func (gs GenesisState) Validate() error {
	if err := gs.GenesisState.Validate(); err != nil {
		return err
	}
	ids := make(map[uint32]bool, len(gs.Domains))
	for _, d := range gs.Domains {
		ids[d.Id] = true
	}
	seen := make(map[[2]uint64]bool, len(gs.DomainHistory))
	for _, history := range gs.DomainHistory {
		key := [2]uint64{uint64(history.DomainID), history.Height}
		if !ids[history.DomainID] || seen[key] || len(history.TxHashes) == 0 {
			return types.ErrInvalidGenesis.Wrap("unknown domain, duplicate height or empty transaction history")
		}
		seen[key] = true
	}
	return nil
}

func (k Keeper) ExportGenesisWithHistory(ctx sdk.Context) *GenesisState {
	gs := &GenesisState{GenesisState: k.exportRegistryGenesis(ctx)}
	store := ctx.KVStore(k.storeKey)
	it := sdk.KVStorePrefixIterator(store, types.DomainTxPrefix)
	defer it.Close()
	for ; it.Valid(); it.Next() {
		key, hashes := it.Key(), it.Value()
		if len(key) != len(types.DomainTxPrefix)+12 || len(hashes) == 0 || len(hashes)%common.HashLength != 0 {
			panic(types.ErrInvalidGenesis.Wrap("invalid stored domain transaction history"))
		}
		id := binary.BigEndian.Uint32(key[len(types.DomainTxPrefix):])
		height := binary.BigEndian.Uint64(key[len(types.DomainTxPrefix)+4:])
		gas := store.Get(types.HeightKey(types.DomainGasUsedPrefix, id, height))
		if len(gas) != 8 {
			panic(types.ErrInvalidGenesis.Wrap("missing or invalid stored domain gas history"))
		}
		gs.DomainHistory = append(gs.DomainHistory, DomainHistory{
			DomainID: id, Height: height, TxHashes: k.DomainTxs(ctx, id, height), GasUsed: binary.BigEndian.Uint64(gas),
		})
	}
	gasIt := sdk.KVStorePrefixIterator(store, types.DomainGasUsedPrefix)
	defer gasIt.Close()
	for ; gasIt.Valid(); gasIt.Next() {
		key := gasIt.Key()
		if len(key) != len(types.DomainGasUsedPrefix)+12 || len(gasIt.Value()) != 8 {
			panic(types.ErrInvalidGenesis.Wrap("invalid stored domain gas history"))
		}
		id := binary.BigEndian.Uint32(key[len(types.DomainGasUsedPrefix):])
		height := binary.BigEndian.Uint64(key[len(types.DomainGasUsedPrefix)+4:])
		if !store.Has(types.HeightKey(types.DomainTxPrefix, id, height)) {
			panic(types.ErrInvalidGenesis.Wrap("orphaned stored domain gas history"))
		}
	}
	if err := gs.Validate(); err != nil {
		panic(err)
	}
	return gs
}
