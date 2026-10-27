package keeper

import (
	"bytes"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
)

func (k *Keeper) RevokeGrant(ctx sdk.Context, actor Actor, grantID [32]byte, reason uint8, sequence uint64, idempotencyKey [32]byte) (Result, error) {
	if err := k.checkPause(ctx, "revokeGrant"); err != nil {
		return Result{}, err
	}
	if !actor.Authority.Bound {
		return Result{}, types.ErrAuthScope
	}
	grant, err := k.host.ReadGrant(ctx, grantID)
	if err != nil {
		return Result{}, authReadError(err)
	}
	if grant.Grantor != actor.Authority.Actor {
		return Result{}, types.ErrUnauthorizedDebit
	}
	payload := types.EncodeRevokePayload(grantID, reason, sequence)
	if err := types.ValidateRevokePayload(payload); err != nil {
		return Result{}, err
	}
	branch, write := ctx.CacheContext()
	result, err := k.execute(branch, actor, payload, idempotencyKey)
	if err != nil {
		return result, err
	}
	events, err := AuthEvents(result.Receipt)
	if err != nil {
		return result, err
	}
	for _, event := range events {
		if event.Name == "GrantRevoked" && event.GrantID == grantID {
			if err := k.DeleteGrant(branch, event.GrantID); err != nil {
				return result, err
			}
			write()
			ctx.EventManager().EmitEvents(branch.EventManager().Events())
			return result, nil
		}
	}
	return result, types.ErrNonCanonical.Wrap("revoke receipt has no matching GrantRevoked event")
}

func (k *Keeper) DeleteGrant(ctx sdk.Context, grantID [32]byte) error {
	grants, ok := k.authzKeeper.(interface {
		IterateGrants(sdk.Context, func(sdk.AccAddress, sdk.AccAddress, authz.Grant) bool)
	})
	if !ok {
		return fmt.Errorf("layerxauth: authz keeper does not support grant iteration")
	}
	type matchingGrant struct {
		granter, grantee sdk.AccAddress
		msgType          string
	}
	var matches []matchingGrant
	grants.IterateGrants(ctx, func(granter, grantee sdk.AccAddress, grant authz.Grant) bool {
		authorization, ok := grant.GetAuthorization().(*types.LayerXScopeAuthorization)
		if ok && bytes.Equal(authorization.GrantId, grantID[:]) {
			matches = append(matches, matchingGrant{
				granter: bytes.Clone(granter), grantee: bytes.Clone(grantee), msgType: authorization.MsgTypeURL(),
			})
		}
		return false
	})
	for _, match := range matches {
		if err := k.authzKeeper.DeleteGrant(ctx, match.grantee, match.granter, match.msgType); err != nil {
			return err
		}
	}
	return nil
}
