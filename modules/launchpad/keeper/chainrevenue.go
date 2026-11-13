package keeper

import (
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

const chainRevenueEpochSeconds int64 = 3600

// OpenChainRevenueEpoch fixes the base fee balance against the preceding
// block's token checkpoints and places the fee coins in module custody.
func (k *Keeper) OpenChainRevenueEpoch(ctx sdk.Context, caller NestedCaller, chain uint64) (types.ChainRevenueEpoch, error) {
	launch, found, err := k.GetLaunchedChain(ctx, chain)
	if err != nil {
		return types.ChainRevenueEpoch{}, err
	}
	if !found {
		return types.ChainRevenueEpoch{}, types.ErrNotLaunchedChain
	}
	last := k.LastChainRevenueEpoch(ctx, chain)
	if last == math.MaxUint64 {
		return types.ChainRevenueEpoch{}, types.ErrOverflow
	}
	if last != 0 {
		previous, found, err := k.GetChainRevenueEpoch(ctx, chain, last)
		if err != nil {
			return types.ChainRevenueEpoch{}, err
		}
		if !found {
			return types.ChainRevenueEpoch{}, types.ErrInvalidGenesis.Wrapf("last revenue epoch %d/%d is absent", chain, last)
		}
		if ctx.BlockTime().Unix()-previous.OpenedAt < chainRevenueEpochSeconds {
			return types.ChainRevenueEpoch{}, types.ErrEpochTooEarly
		}
	}
	base, all, err := k.chainRevenueBalances(ctx, chain)
	if err != nil {
		return types.ChainRevenueEpoch{}, err
	}
	if !base.Amount.IsPositive() {
		return types.ChainRevenueEpoch{}, types.ErrNothingToDistribute
	}
	if ctx.BlockHeight() <= 0 {
		return types.ChainRevenueEpoch{}, types.ErrUnsupportedSnapshotToken
	}
	snapshot := uint64(ctx.BlockHeight() - 1)
	eligible, err := k.snapshotEligibleSupply(ctx, caller, k.ModuleEVMAddress(ctx),
		common.HexToAddress(launch.Token), common.HexToAddress(launch.Pool), snapshot)
	if err != nil {
		return types.ChainRevenueEpoch{}, err
	}
	if eligible.Sign() == 0 {
		return types.ChainRevenueEpoch{}, types.ErrNothingToDistribute
	}
	record := types.ChainRevenueEpoch{Chain: chain, Epoch: last + 1, Amount: base.Amount,
		SnapshotBlock: snapshot, EligibleSupply: sdk.NewIntFromBigInt(eligible),
		ClaimedTotal: sdk.ZeroInt(), OpenedAt: ctx.BlockTime().Unix()}
	cache, write := ctx.CacheContext()
	if err := k.moveChainRevenue(cache, chain, base, all); err != nil {
		return types.ChainRevenueEpoch{}, err
	}
	if err := k.SetChainRevenueEpoch(cache, record); err != nil {
		return types.ChainRevenueEpoch{}, err
	}
	write()
	ctx.EventManager().EmitEvent(sdk.NewEvent(types.EventTypeChainRevenueEpochOpened,
		sdk.NewAttribute("chain", strconv.FormatUint(chain, 10)),
		sdk.NewAttribute(types.AttributeKeyEpoch, strconv.FormatUint(record.Epoch, 10)),
		sdk.NewAttribute(types.AttributeKeyAmount, record.Amount.String()),
		sdk.NewAttribute("snapshot_block", strconv.FormatUint(snapshot, 10)),
		sdk.NewAttribute("eligible_supply", record.EligibleSupply.String())))
	return record, nil
}

// chainRevenueShare reads the amount due to one holder, including the
// remainder when every eligible token balance has been claimed.
func (k *Keeper) chainRevenueShare(ctx sdk.Context, caller NestedCaller, launch types.LaunchedChain,
	record types.ChainRevenueEpoch, holder common.Address) (sdk.Int, error) {
	if holder == (common.Address{}) || holder == common.HexToAddress(launch.Pool) ||
		holder == common.HexToAddress(types.DeadAddress) {
		return sdk.Int{}, types.ErrUnauthorized
	}
	if k.HasChainRevenueClaim(ctx, record.Chain, record.Epoch, holder) {
		return sdk.Int{}, types.ErrAlreadyClaimed
	}
	accumulator := k.ModuleEVMAddress(ctx)
	token := common.HexToAddress(launch.Token)
	balance, err := pastBalance(ctx, caller, accumulator, token, holder, record.SnapshotBlock)
	if err != nil {
		return sdk.Int{}, err
	}
	if balance.Sign() == 0 {
		return sdk.Int{}, types.ErrZeroAmount
	}
	// Claim records are the durable set of balances already paid from this
	// snapshot, including across genesis export and import.
	claimKey := types.ChainRevenueClaimKey(record.Chain, record.Epoch, common.Address{})
	prefix := claimKey[:len(claimKey)-common.AddressLength]
	claims := sdk.KVStorePrefixIterator(k.store(ctx), prefix)
	defer claims.Close()
	claimedSupply := new(big.Int)
	for ; claims.Valid(); claims.Next() {
		key := claims.Key()
		if len(key) != len(claimKey) {
			return sdk.Int{}, types.ErrInvalidGenesis.Wrap("invalid revenue claim key")
		}
		prior, err := pastBalance(ctx, caller, accumulator, token,
			common.BytesToAddress(key[len(prefix):]), record.SnapshotBlock)
		if err != nil {
			return sdk.Int{}, err
		}
		claimedSupply.Add(claimedSupply, prior)
	}
	claimedSupply.Add(claimedSupply, balance)
	if claimedSupply.Cmp(record.EligibleSupply.BigInt()) > 0 {
		return sdk.Int{}, types.ErrOverflow
	}
	product := new(big.Int).Mul(record.Amount.BigInt(), balance)
	if product.Cmp(types.MaxUint256()) > 0 {
		return sdk.Int{}, types.ErrOverflow
	}
	amount := sdk.NewIntFromBigInt(product.Quo(product, record.EligibleSupply.BigInt()))
	if claimedSupply.Cmp(record.EligibleSupply.BigInt()) == 0 {
		amount = record.Amount.Sub(record.ClaimedTotal)
	}
	if record.ClaimedTotal.Add(amount).GT(record.Amount) {
		return sdk.Int{}, types.ErrOverflow
	}
	return amount, nil
}

// ChainRevenueClaimable reads the PAX due to a holder in an open epoch.
func (k *Keeper) ChainRevenueClaimable(ctx sdk.Context, caller NestedCaller, chain, epoch uint64,
	holder common.Address) (sdk.Int, error) {
	record, found, err := k.GetChainRevenueEpoch(ctx, chain, epoch)
	if err != nil {
		return sdk.Int{}, err
	}
	if !found {
		return sdk.Int{}, types.ErrInvalidGenesis.Wrapf("chain revenue epoch %d/%d does not exist", chain, epoch)
	}
	launch, found, err := k.GetLaunchedChain(ctx, chain)
	if err != nil {
		return sdk.Int{}, err
	}
	if !found {
		return sdk.Int{}, types.ErrNotLaunchedChain
	}
	return k.chainRevenueShare(ctx, caller, launch, record, holder)
}

// ClaimChainRevenue pays one holder from module custody and records the claim.
func (k *Keeper) ClaimChainRevenue(ctx sdk.Context, caller NestedCaller, chain, epoch uint64,
	holder common.Address) (sdk.Context, sdk.Int, error) {
	amount, err := k.ChainRevenueClaimable(ctx, caller, chain, epoch, holder)
	if err != nil {
		return ctx, sdk.Int{}, err
	}
	record, _, err := k.GetChainRevenueEpoch(ctx, chain, epoch)
	if err != nil {
		return ctx, sdk.Int{}, err
	}
	base, _, err := k.chainRevenueBalances(ctx, chain)
	if err != nil {
		return ctx, sdk.Int{}, err
	}
	cache, write := ctx.CacheContext()
	if amount.IsPositive() {
		to := k.evmKeeper.GetPaxAddressOrDefault(cache, holder)
		if err := k.bankKeeper.SendCoins(cache, k.ModuleAddress(), to,
			sdk.NewCoins(sdk.NewCoin(base.Denom, amount))); err != nil {
			return ctx, sdk.Int{}, err
		}
	}
	record.ClaimedTotal = record.ClaimedTotal.Add(amount)
	if err := k.SetChainRevenueEpoch(cache, record); err != nil {
		return ctx, sdk.Int{}, err
	}
	if err := k.SetChainRevenueClaim(cache, chain, epoch, holder); err != nil {
		return ctx, sdk.Int{}, err
	}
	write()
	ctx.EventManager().EmitEvent(sdk.NewEvent(types.EventTypeChainRevenueClaimed,
		sdk.NewAttribute("chain", strconv.FormatUint(chain, 10)),
		sdk.NewAttribute(types.AttributeKeyEpoch, strconv.FormatUint(epoch, 10)),
		sdk.NewAttribute(types.AttributeKeyHolder, holder.Hex()),
		sdk.NewAttribute(types.AttributeKeyAmount, amount.String())))
	return ctx, amount, nil
}

// ClaimChainRevenueBatch claims every requested epoch for one holder atomically.
func (k *Keeper) ClaimChainRevenueBatch(ctx sdk.Context, caller NestedCaller, chain uint64,
	holder common.Address, epochs []uint64) (sdk.Context, []sdk.Int, error) {
	cache, write := ctx.CacheContext()
	amounts := make([]sdk.Int, 0, len(epochs))
	for _, epoch := range epochs {
		var amount sdk.Int
		var err error
		cache, amount, err = k.ClaimChainRevenue(cache, caller, chain, epoch, holder)
		if err != nil {
			return ctx, nil, fmt.Errorf("chain revenue epoch %d: %w", epoch, err)
		}
		amounts = append(amounts, amount)
	}
	write()
	ctx.EventManager().EmitEvents(cache.EventManager().Events())
	return ctx, amounts, nil
}
