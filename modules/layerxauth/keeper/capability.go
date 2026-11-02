package keeper

import (
	"math"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

type capabilityBindings interface {
	GetEVMAddressByLayerXDid(ctx sdk.Context, key [32]byte) (common.Address, bool)
}

const capabilityAuthorizationTypeURL = "/paxprotocol.paxchain.layerxauth.LayerXScopeAuthorization"

func (k *Keeper) GrantCapability(ctx sdk.Context, actor Actor, grant []byte, idempotencyKey [32]byte) (Result, error) {
	if err := k.checkPause(ctx, "grantCapability"); err != nil {
		return Result{}, err
	}
	payload, err := types.EncodeCapabilityPayload(grant)
	if err != nil {
		return Result{}, err
	}
	cache, write := ctx.CacheContext()
	result, err := k.execute(cache, actor, payload, idempotencyKey)
	if err != nil {
		write()
		return result, err
	}
	events, err := AuthEvents(result.Receipt)
	if err != nil {
		return result, err
	}
	var grantID [32]byte
	found := false
	for _, event := range events {
		if event.Name == "CapabilityGranted" {
			if found {
				return result, types.ErrNonCanonical.Wrap("multiple capability grants in receipt")
			}
			grantID, found = event.GrantID, true
		}
	}
	if !found {
		return result, types.ErrNonCanonical.Wrap("missing capability grant in receipt")
	}
	issued, err := k.host.ReadGrant(cache, grantID)
	if err != nil {
		return result, authReadError(err)
	}
	if err := k.SaveGrant(cache, actor, issued); err != nil {
		return result, err
	}
	write()
	return result, nil
}

func (k *Keeper) SaveGrant(ctx sdk.Context, actor Actor, grant lxphost.AuthorityGrant) error {
	bindings, ok := k.evmKeeper.(capabilityBindings)
	if !ok || k.authzKeeper == nil {
		return types.ErrAuthScope.Wrap("capability mirroring keepers unavailable")
	}
	grantee, bound := bindings.GetEVMAddressByLayerXDid(ctx, grant.Key)
	if !bound {
		return types.ErrAuthScope.Wrap("capability key has no bound EVM address")
	}
	granter, bound := bindings.GetEVMAddressByLayerXDid(ctx, actor.DIDKey)
	if !bound || grant.Grantor != actor.Authority.Actor || grant.Grantee != actor.Authority.Actor {
		return types.ErrAuthScope.Wrap("capability grantor has no matching bound identity")
	}
	if grant.NotAfter > math.MaxInt64 {
		return types.ErrNonCanonical.Wrap("capability expiration exceeds supported timestamp")
	}
	scope := grant.Scope
	authorization := &types.LayerXScopeAuthorization{
		ModuleMask: scope.ModuleMask,
		OrdinalMin: uint32(scope.ActivityOrdinalMin), OrdinalMax: uint32(scope.ActivityOrdinalMax),
		AssetId:        append([]byte(nil), scope.AssetID[:]...),
		MaxPerActivity: scope.MaximumPerActivity.String(), MaxTotal: scope.MaximumTotal.String(),
		RevocationEpoch: grant.GrantorRevocationSequence,
		GrantId:         append([]byte(nil), grant.GrantID[:]...),
		MsgTypeUrl:      capabilityAuthorizationTypeURL, Kind: uint32(grant.Kind),
		PeriodLength: scope.PeriodLength, MaxPerPeriod: scope.MaximumPerPeriod.String(),
		PurposeHash: append([]byte(nil), scope.PurposeHash[:]...),
		NotBefore:   grant.NotBefore, NotAfter: grant.NotAfter,
		SpentTotal: scope.SpentTotal.String(),
	}
	if err := authorization.ValidateBasic(); err != nil {
		return err
	}
	return k.authzKeeper.SaveGrant(ctx, sdk.AccAddress(grantee.Bytes()), sdk.AccAddress(granter.Bytes()), authorization,
		time.UnixMilli(int64(grant.NotAfter)).UTC())
}
