package keeper

import (
	"strconv"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
)

func (k Keeper) SetDenomPausedByIssuer(ctx sdk.Context, denom, callerDid string, paused bool) error {
	issuerDid, err := k.IssuerDid(ctx, denom)
	if err != nil {
		return err
	}
	if issuerDid == "" || issuerDid != callerDid {
		return types.ErrIssuerMismatch
	}
	if err := k.setDenomPaused(ctx, denom, paused); err != nil {
		return err
	}
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.TypeMsgSetDenomPaused,
		sdk.NewAttribute(types.AttributeDenom, denom),
		sdk.NewAttribute(types.AttributePaused, strconv.FormatBool(paused)),
		sdk.NewAttribute(types.AttributeIssuerDid, issuerDid),
	))
	return nil
}
