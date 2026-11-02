package keeper

import (
	"sync"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

type ChainsBinding interface {
	RecordDomainTx(ctx sdk.Context, id uint32, height int64, hash common.Hash, gasUsed uint64)
	ActiveDomains(ctx sdk.Context) []uint32
	DomainGasUsed(ctx sdk.Context, id uint32, height int64) uint64
	AdjustDomainBaseFee(ctx sdk.Context, id uint32, gasUsed uint64, minimum, maximum, upward, downward sdk.Dec) sdk.Dec
	NextBaseFee(ctx sdk.Context, id uint32) sdk.Dec
	Operator(ctx sdk.Context, id uint32) (common.Address, uint16)
}

var chainsBindings sync.Map

type domainExecutionKey struct{}

type domainExecution struct {
	id      uint32
	baseFee sdk.Dec
}

func (k *Keeper) SetChainsBinding(binding ChainsBinding) {
	if binding == nil {
		chainsBindings.Delete(k.storeKey)
		return
	}
	chainsBindings.Store(k.storeKey, binding)
}

func (k Keeper) chainsBinding() ChainsBinding {
	binding, ok := chainsBindings.Load(k.storeKey)
	if !ok {
		return nil
	}
	return binding.(ChainsBinding)
}
