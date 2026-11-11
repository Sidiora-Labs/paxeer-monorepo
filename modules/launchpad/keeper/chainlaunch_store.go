package keeper

import (
	"encoding/json"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

// GetLaunchedChain reads the registry binding of a launched chain.
func (k *Keeper) GetLaunchedChain(ctx sdk.Context, number uint64) (types.LaunchedChain, bool, error) {
	bz := k.store(ctx).Get(types.LaunchedChainKey(number))
	if bz == nil {
		return types.LaunchedChain{}, false, nil
	}
	var record types.LaunchedChain
	if err := json.Unmarshal(bz, &record); err != nil {
		return types.LaunchedChain{}, false, fmt.Errorf("%w: launched chain %d: %v", types.ErrInvalidGenesis, number, err)
	}
	return record, true, nil
}

// SetLaunchedChain stores the registry binding and both pool indexes.
func (k *Keeper) SetLaunchedChain(ctx sdk.Context, record types.LaunchedChain) error {
	if err := record.Validate(); err != nil {
		return err
	}
	if old, found, err := k.GetLaunchedChain(ctx, record.RegistryNumber); err != nil {
		return err
	} else if found && old.Pool != record.Pool {
		return types.ErrInvalidGenesis.Wrapf("registry number %d already binds pool %s", record.RegistryNumber, old.Pool)
	}
	pool := common.HexToAddress(record.Pool)
	if number, found := k.RegistryNumberByPool(ctx, pool); found && number != record.RegistryNumber {
		return types.ErrInvalidGenesis.Wrapf("pool %s already binds registry number %d", record.Pool, number)
	}
	bz, err := json.Marshal(record)
	if err != nil {
		return err
	}
	store := k.store(ctx)
	store.Set(types.LaunchedChainKey(record.RegistryNumber), bz)
	k.setUint64(ctx, types.ChainPoolKey(pool), record.RegistryNumber)
	k.SetHomeChainByPool(ctx, pool, record.ChainID)
	return nil
}

// RegistryNumberByPool resolves the chain registry entry of a pool.
func (k *Keeper) RegistryNumberByPool(ctx sdk.Context, pool common.Address) (uint64, bool) {
	key := types.ChainPoolKey(pool)
	if !k.store(ctx).Has(key) {
		return 0, false
	}
	return k.getUint64(ctx, key), true
}

// HomeChainByPool returns zero for a root-network pool or an absent record.
func (k *Keeper) HomeChainByPool(ctx sdk.Context, pool common.Address) uint64 {
	return k.getUint64(ctx, types.HomeChainKey(pool))
}

// SetHomeChainByPool records the chain where a pool's trades take place.
func (k *Keeper) SetHomeChainByPool(ctx sdk.Context, pool common.Address, chain uint64) {
	key := types.HomeChainKey(pool)
	if chain == 0 {
		k.store(ctx).Delete(key)
		return
	}
	k.setUint64(ctx, key, chain)
}

// GetChainRevenueEpoch reads a chain's fixed revenue epoch.
func (k *Keeper) GetChainRevenueEpoch(ctx sdk.Context, chain, epoch uint64) (types.ChainRevenueEpoch, bool, error) {
	bz := k.store(ctx).Get(types.ChainRevenueEpochKey(chain, epoch))
	if bz == nil {
		return types.ChainRevenueEpoch{}, false, nil
	}
	var record types.ChainRevenueEpoch
	if err := json.Unmarshal(bz, &record); err != nil {
		return types.ChainRevenueEpoch{}, false, fmt.Errorf("%w: chain revenue epoch %d/%d: %v", types.ErrInvalidGenesis, chain, epoch, err)
	}
	return record, true, nil
}

// SetChainRevenueEpoch stores an epoch and advances the chain's last epoch.
func (k *Keeper) SetChainRevenueEpoch(ctx sdk.Context, record types.ChainRevenueEpoch) error {
	if err := record.Validate(); err != nil {
		return err
	}
	if _, found, err := k.GetLaunchedChain(ctx, record.Chain); err != nil {
		return err
	} else if !found {
		return types.ErrNotLaunchedChain.Wrapf("registry number %d", record.Chain)
	}
	bz, err := json.Marshal(record)
	if err != nil {
		return err
	}
	k.store(ctx).Set(types.ChainRevenueEpochKey(record.Chain, record.Epoch), bz)
	if record.Epoch > k.LastChainRevenueEpoch(ctx, record.Chain) {
		k.setUint64(ctx, types.ChainRevenueLastKey(record.Chain), record.Epoch)
	}
	return nil
}

// LastChainRevenueEpoch is zero before the first epoch opens.
func (k *Keeper) LastChainRevenueEpoch(ctx sdk.Context, chain uint64) uint64 {
	return k.getUint64(ctx, types.ChainRevenueLastKey(chain))
}

// HasChainRevenueClaim reports whether a holder has claimed an epoch.
func (k *Keeper) HasChainRevenueClaim(ctx sdk.Context, chain, epoch uint64, holder common.Address) bool {
	return k.store(ctx).Has(types.ChainRevenueClaimKey(chain, epoch, holder))
}

// SetChainRevenueClaim marks a holder's claim of an existing epoch.
func (k *Keeper) SetChainRevenueClaim(ctx sdk.Context, chain, epoch uint64, holder common.Address) error {
	if holder == (common.Address{}) {
		return types.ErrZeroAddress
	}
	if _, found, err := k.GetChainRevenueEpoch(ctx, chain, epoch); err != nil {
		return err
	} else if !found {
		return types.ErrInvalidGenesis.Wrapf("chain revenue epoch %d/%d does not exist", chain, epoch)
	}
	if k.HasChainRevenueClaim(ctx, chain, epoch, holder) {
		return types.ErrAlreadyClaimed
	}
	k.store(ctx).Set(types.ChainRevenueClaimKey(chain, epoch, holder), []byte{1})
	return nil
}
