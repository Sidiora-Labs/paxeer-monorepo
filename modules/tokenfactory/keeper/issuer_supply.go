package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k Keeper) MintByIssuer(ctx sdk.Context, callerDid string, amount sdk.Coin, to string) error {
	if _, err := k.requireIssuer(ctx, amount.Denom, callerDid); err != nil {
		return err
	}
	// mintTo mints before decoding the recipient; reject bad addresses before
	// allowing that helper to change supply through this exported keeper API.
	if _, err := sdk.AccAddressFromBech32(to); err != nil {
		return err
	}
	cacheCtx, commit := ctx.CacheContext()
	if err := k.mintTo(cacheCtx, amount, to); err != nil {
		return err
	}
	cacheCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.TypeMsgMint,
		sdk.NewAttribute(types.AttributeMintToAddress, to),
		sdk.NewAttribute(types.AttributeAmount, amount.String()),
	))
	commit()
	ctx.EventManager().EmitEvents(cacheCtx.EventManager().Events())
	return nil
}

func (k Keeper) BurnByIssuer(ctx sdk.Context, callerDid string, amount sdk.Coin, from string) error {
	if _, err := k.requireIssuer(ctx, amount.Denom, callerDid); err != nil {
		return err
	}
	if err := k.burnFrom(ctx, amount, from); err != nil {
		return err
	}
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.TypeMsgBurn,
		sdk.NewAttribute(types.AttributeBurnFromAddress, from),
		sdk.NewAttribute(types.AttributeAmount, amount.String()),
	))
	return nil
}
