package keeper

import (
	"fmt"
	"math"
	"strconv"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func (k *Keeper) launchedChainOperator(ctx sdk.Context, chain uint64, caller common.Address) (common.Address, error) {
	record, found, err := k.GetLaunchedChain(ctx, chain)
	if err != nil {
		return common.Address{}, err
	}
	if !found || chain == 0 || chain > math.MaxUint32 || record.RegistryNumber != chain {
		return common.Address{}, types.ErrNotLaunchedChain
	}
	owner, err := k.launchedChainFeeRightOwner(ctx, record)
	if err != nil {
		return common.Address{}, err
	}
	if owner != caller {
		return common.Address{}, types.ErrNotFeeRightHolder
	}
	return common.BytesToAddress(types.ChainRevenueAccount(chain)), nil
}

// PauseLaunchedChain pauses a launched chain for its current fee right holder.
func (k *Keeper) PauseLaunchedChain(ctx sdk.Context, chain uint64, caller common.Address) error {
	operator, err := k.launchedChainOperator(ctx, chain, caller)
	if err != nil {
		return err
	}
	return k.chainRegistry.Pause(ctx, operator, uint32(chain))
}

// ResumeLaunchedChain resumes a holder-paused chain.
func (k *Keeper) ResumeLaunchedChain(ctx sdk.Context, chain uint64, caller common.Address) error {
	operator, err := k.launchedChainOperator(ctx, chain, caller)
	if err != nil {
		return err
	}
	return k.chainRegistry.Resume(ctx, operator, uint32(chain))
}

// RetireLaunchedChain returns the registry bond to the current fee right holder.
func (k *Keeper) RetireLaunchedChain(ctx sdk.Context, chain uint64, caller common.Address) error {
	operator, err := k.launchedChainOperator(ctx, chain, caller)
	if err != nil {
		return err
	}
	cache, write := ctx.CacheContext()
	revenue := types.ChainRevenueAccount(chain)
	denom := k.chainRegistry.BaseDenom(cache)
	before := k.bankKeeper.GetBalance(cache, revenue, denom).Amount
	if err := k.chainRegistry.Retire(cache, operator, uint32(chain)); err != nil {
		return err
	}
	returned := k.bankKeeper.GetBalance(cache, revenue, denom).Amount.Sub(before)
	if returned.IsNegative() {
		return fmt.Errorf("launchpad: chain %d retirement reduced the revenue balance", chain)
	}
	if returned.IsPositive() {
		holder := k.evmKeeper.GetPaxAddressOrDefault(cache, caller)
		if err := k.bankKeeper.SendCoins(cache, revenue, holder, sdk.NewCoins(sdk.NewCoin(denom, returned))); err != nil {
			return err
		}
	}
	cache.EventManager().EmitEvent(sdk.NewEvent(types.EventTypeLaunchedChainRetired,
		sdk.NewAttribute("chain", strconv.FormatUint(chain, 10)),
		sdk.NewAttribute(types.AttributeKeyHolder, caller.Hex()),
		sdk.NewAttribute("returned_bond", returned.String())))
	write()
	ctx.EventManager().EmitEvents(cache.EventManager().Events())
	return nil
}
