package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
)

type retirementUpgradeReader interface {
	types.UpgradeActivationReader
	GetUpgradePlan(sdk.Context) (upgradetypes.Plan, bool)
}

func (k *Keeper) ReleaseRetired(ctx sdk.Context) error {
	if k.Retired(ctx) {
		return sdkerrors.Wrap(types.ErrModuleRetired, "already released")
	}
	upgrade, ok := k.upgrade.(retirementUpgradeReader)
	if !ok {
		return sdkerrors.Wrap(types.ErrModuleRetired, "plan not running")
	}
	plan, found := upgrade.GetUpgradePlan(ctx)
	if !found || plan.Name != types.MergePlanUpgrade || plan.Height != ctx.BlockHeight() {
		return sdkerrors.Wrap(types.ErrModuleRetired, "plan not running")
	}
	_, err := atomically(ctx, func(ctx sdk.Context) (struct{}, error) {
		authority := k.Authority(ctx)
		recipient, err := sdk.AccAddressFromBech32(authority)
		if err != nil {
			return struct{}{}, err
		}
		var pending []types.Claim
		k.IterateClaims(ctx, func(claim types.Claim) bool {
			if claim.Status == types.ClaimStatus_CLAIM_STATUS_PENDING {
				pending = append(pending, claim)
			}
			return false
		})
		cancelled := make(map[string]uint64)
		for _, claim := range pending {
			if err := k.transition(ctx, claim, types.NullifierStatus_NULLIFIER_STATUS_CANCELLED); err != nil {
				return struct{}{}, err
			}
			claim.Status = types.ClaimStatus_CLAIM_STATUS_CANCELLED
			k.setClaim(ctx, claim)
			cancelled[claim.AssetId]++
		}
		k.syncExitCheckpoint(ctx)
		var releaseErr error
		k.IterateAssets(ctx, func(asset types.AssetMapping) bool {
			balance := k.bankKeeper.GetBalance(ctx, k.ModuleAddress(), asset.Denom)
			if balance.IsPositive() {
				if releaseErr = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName,
					recipient, sdk.NewCoins(balance)); releaseErr != nil {
					return true
				}
			}
			assetID, _ := types.ParseHash32(asset.AssetId)
			custodied, released, _ := k.totals(ctx, assetID)
			k.setTotals(ctx, assetID, sdk.ZeroInt(), released.Add(custodied), sdk.ZeroInt())
			releaseErr = ctx.EventManager().EmitTypedEvent(&types.EventRetiredRelease{
				AssetId: asset.AssetId, Recipient: authority, Amount: balance.Amount.String(),
				Denom: asset.Denom, CancelledClaims: cancelled[asset.AssetId],
			})
			return releaseErr != nil
		})
		return struct{}{}, releaseErr
	})
	return err
}
