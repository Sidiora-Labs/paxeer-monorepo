package keeper

import (
	"math"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
)

type capabilityBindings interface {
	GetEVMAddressByLayerXDid(ctx sdk.Context, key [32]byte) (common.Address, bool)
}

// The SDK bank send messages correspond to asset module 1, SEND ordinal 5.
// Kernel ledger messages remain owner-only and are never mirrored here.
func capabilityMessageURLs(scope lxphost.AuthorityScope) []string {
	if scope.ModuleMask&(1<<1) == 0 || scope.ActivityOrdinalMin > 5 || scope.ActivityOrdinalMax < 5 {
		return nil
	}
	return []string{sdk.MsgTypeURL(&banktypes.MsgSend{}), sdk.MsgTypeURL(&banktypes.MsgMultiSend{})}
}

// GrantCapability publishes a kernel grant and its SDK authz mirror atomically;
// receipt decoding or mirror failures discard both writes.
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
		// lxp_kernel_execute passes admission a false idempotency-exists flag;
		// lxp_admit_activity checks sequence before that flag. execute supplies a
		// fresh sequence, so the duplicate grant check reports sequence reused.
		// Preserve that kernel reason instead of synthesizing an idempotent replay.
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
	messageURLs := capabilityMessageURLs(scope)
	if len(messageURLs) == 0 {
		return types.ErrAuthScope.Wrap("capability scope has no supported executable SDK message")
	}
	authorization := &types.LayerXScopeAuthorization{
		ModuleMask: scope.ModuleMask,
		OrdinalMin: uint32(scope.ActivityOrdinalMin), OrdinalMax: uint32(scope.ActivityOrdinalMax),
		AssetId:        append([]byte(nil), scope.AssetID[:]...),
		MaxPerActivity: scope.MaximumPerActivity.String(), MaxTotal: scope.MaximumTotal.String(),
		RevocationEpoch: grant.GrantorRevocationSequence,
		GrantId:         append([]byte(nil), grant.GrantID[:]...),
		Kind:            uint32(grant.Kind),
		PeriodLength:    scope.PeriodLength, MaxPerPeriod: scope.MaximumPerPeriod.String(),
		PurposeHash: append([]byte(nil), scope.PurposeHash[:]...),
		NotBefore:   grant.NotBefore, NotAfter: grant.NotAfter,
		SpentTotal: scope.SpentTotal.String(),
	}
	// Disjoint message keys compose by union. An occupied key is an explicit
	// conflict, even for the same grant ID: replacing it could reset spending.
	// Cache the entire mirror so conflicts cannot leave a partial set of URLs.
	branch, write := ctx.CacheContext()
	granteeAddress, granterAddress := sdk.AccAddress(grantee.Bytes()), sdk.AccAddress(granter.Bytes())
	for _, messageURL := range messageURLs {
		authorization.MsgTypeUrl = messageURL
		if err := authorization.ValidateBasic(); err != nil {
			return err
		}
		if existing, _ := k.authzKeeper.GetCleanAuthorization(branch, granteeAddress, granterAddress, messageURL); existing != nil {
			return types.ErrAuthScope.Wrapf("capability message grant already exists: %s", messageURL)
		}
		if err := k.authzKeeper.SaveGrant(branch, granteeAddress, granterAddress, authorization,
			time.UnixMilli(int64(grant.NotAfter)).UTC()); err != nil {
			return err
		}
	}
	write()
	return nil
}
