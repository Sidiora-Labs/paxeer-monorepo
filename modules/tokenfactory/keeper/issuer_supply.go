package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k Keeper) MintByIssuer(ctx sdk.Context, callerDid string, amount sdk.Coin, to string) error {
	issuerDid, err := k.IssuerDid(ctx, amount.Denom)
	if err != nil {
		return err
	}
	if issuerDid == "" || issuerDid != callerDid {
		return types.ErrIssuerMismatch
	}
	if err := k.mintTo(ctx, amount, to); err != nil {
		return err
	}
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.TypeMsgMint,
		sdk.NewAttribute(types.AttributeMintToAddress, to),
		sdk.NewAttribute(types.AttributeAmount, amount.String()),
	))
	return nil
}

func (k Keeper) BurnByIssuer(ctx sdk.Context, callerDid string, amount sdk.Coin, from string) error {
	issuerDid, err := k.IssuerDid(ctx, amount.Denom)
	if err != nil {
		return err
	}
	if issuerDid == "" || issuerDid != callerDid {
		return types.ErrIssuerMismatch
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
