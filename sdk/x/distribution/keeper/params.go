package keeper

import (
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
)

// GetParams returns the total set of distribution parameters.
func (k Keeper) GetParams(clientCtx sdk.Context) (params types.Params) {
	k.paramSpace.GetParamSet(clientCtx, &params)
	return params
}

// SetParams sets the distribution parameters to the param space.
func (k Keeper) SetParams(ctx sdk.Context, params types.Params) {
	if err := params.ValidateBasic(); err != nil {
		panic(err)
	}
	k.paramSpace.SetParamSet(ctx, &params)
}

// ParamChangeProposalHandler validates the final distribution parameter set
// before publishing any writes from the wrapped governance handler.
func (k Keeper) ParamChangeProposalHandler(handler govtypes.Handler) govtypes.Handler {
	return func(ctx sdk.Context, content govtypes.Content) error {
		p, ok := content.(*proposal.ParameterChangeProposal)
		if !ok {
			return handler(ctx, content)
		}
		for _, change := range p.Changes {
			if change.Subspace != types.ModuleName {
				continue
			}

			cacheCtx, write := ctx.CacheContext()
			if err := handler(cacheCtx, content); err != nil {
				return err
			}
			if err := k.GetParams(cacheCtx).ValidateBasic(); err != nil {
				return sdkerrors.Wrap(proposal.ErrSettingParameter, err.Error())
			}
			write()
			ctx.EventManager().EmitEvents(cacheCtx.EventManager().Events())
			return nil
		}
		return handler(ctx, content)
	}
}

// GetCommunityTax returns the current distribution community tax.
func (k Keeper) GetCommunityTax(ctx sdk.Context) (percent sdk.Dec) {
	k.paramSpace.Get(ctx, types.ParamStoreKeyCommunityTax, &percent)
	return percent
}

// GetBaseProposerReward returns the current distribution base proposer rate.
func (k Keeper) GetBaseProposerReward(ctx sdk.Context) (percent sdk.Dec) {
	k.paramSpace.Get(ctx, types.ParamStoreKeyBaseProposerReward, &percent)
	return percent
}

// GetBonusProposerReward returns the current distribution bonus proposer reward
// rate.
func (k Keeper) GetBonusProposerReward(ctx sdk.Context) (percent sdk.Dec) {
	k.paramSpace.Get(ctx, types.ParamStoreKeyBonusProposerReward, &percent)
	return percent
}

// GetWithdrawAddrEnabled returns the current distribution withdraw address
// enabled parameter.
func (k Keeper) GetWithdrawAddrEnabled(ctx sdk.Context) (enabled bool) {
	k.paramSpace.Get(ctx, types.ParamStoreKeyWithdrawAddrEnabled, &enabled)
	return enabled
}
