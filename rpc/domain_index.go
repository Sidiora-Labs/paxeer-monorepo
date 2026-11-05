package evmrpc

import (
	"context"
	"fmt"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	chainskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/keeper"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type DomainIndex interface {
	DomainTxs(ctx sdk.Context, id uint32, height uint64) []common.Hash
	DomainOf(tx *ethtypes.Transaction) (uint32, bool)
}

type keeperDomainIndex struct{ evm *evmkeeper.Keeper }

func chainsKeeperForDomainIndex(ctx sdk.Context, k *evmkeeper.Keeper) (*chainskeeper.Keeper, error) {
	for _, key := range ctx.MultiStore().StoreKeys() {
		if key.Name() == chainstypes.StoreKey {
			chains, err := chainskeeper.NewKeeper(k.AccountKeeper().GetCodec(), key, k.BankKeeper(), k.GetBaseDenom)
			if err != nil {
				return nil, err
			}
			return chains, nil
		}
	}
	return nil, fmt.Errorf("EVM server chain registry store is not configured")
}

func (d keeperDomainIndex) DomainTxs(ctx sdk.Context, id uint32, height uint64) []common.Hash {
	chains, err := chainsKeeperForDomainIndex(ctx, d.evm)
	if err != nil {
		panic(err)
	}
	return chains.DomainTxs(ctx, id, height)
}

func (keeperDomainIndex) DomainOf(tx *ethtypes.Transaction) (uint32, bool) {
	if tx == nil {
		return 0, false
	}
	return chainstypes.DomainOfChainID(tx.ChainId())
}

func domainHashSet(index DomainIndex, sdkctx sdk.Context, scope DomainScope, height uint64) map[common.Hash]bool {
	hashes := index.DomainTxs(sdkctx, scope.ID, height)
	set := make(map[common.Hash]bool, len(hashes))
	for _, hash := range hashes {
		set[hash] = true
	}
	return set
}

func domainAllowsTransaction(ctx context.Context, index DomainIndex, tx *ethtypes.Transaction) bool {
	scope, scoped := DomainScopeFrom(ctx)
	if !scoped {
		return true
	}
	id, ok := index.DomainOf(tx)
	return ok && id == scope.ID
}
