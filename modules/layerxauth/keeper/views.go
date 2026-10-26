package keeper

import (
	"encoding/binary"
	"errors"
	"math"
	"time"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k *Keeper) GetGrant(ctx sdk.Context, grantID [32]byte) ([]byte, error) {
	grant, err := k.host.ReadGrant(ctx, grantID)
	if err != nil {
		return nil, authReadError(err)
	}
	version := byte(1)
	if grant.FeeBudget.Present {
		version = 2
	}
	if grant.AuthenticationOnly {
		version = 3
	}
	if grant.Kind == 7 || grant.Kind == 8 {
		version = 4
	}
	out := []byte{0, 1, 0x20, 1, version}
	out = authBytes(out, grant.Grantor[:])
	out = authBytes(out, grant.Grantee[:])
	out = append(out, grant.Kind)
	out = authBytes(out, grant.Key[:])
	out = append(out, encodeAuthScope(grant.Scope)...)
	out = binary.BigEndian.AppendUint64(out, grant.NotBefore)
	out = binary.BigEndian.AppendUint64(out, grant.NotAfter)
	out = binary.BigEndian.AppendUint64(out, grant.GrantorRevocationSequence)
	out = append(out, 0)
	if grant.Revoked {
		out[len(out)-1] = 1
	}
	out = binary.BigEndian.AppendUint64(out, grant.RevokedAtSequence)
	out = authBytes(out, grant.GrantorSignature[:])
	if fee := grant.FeeBudget; fee.Present {
		out = authBytes(out, fee.AssetID[:])
		out = authAmount(out, fee.MaximumPerActivity)
		out = authAmount(out, fee.MaximumTotal)
		out = authAmount(out, fee.SpentTotal)
		out = binary.BigEndian.AppendUint64(out, fee.PeriodLength)
		out = authAmount(out, fee.MaximumPerPeriod)
		out = authAmount(out, fee.SpentThisPeriod)
		out = binary.BigEndian.AppendUint64(out, fee.PeriodStart)
	}
	if grant.AuthenticationOnly {
		out = append(out, 1)
	}
	if version == 4 {
		out = appendAuthExtendedScope(out, grant.Scope)
	}
	return out, nil
}

func (k *Keeper) IsLive(ctx sdk.Context, grantID [32]byte, sequence, timestampMs uint64) (bool, string, error) {
	if timestampMs > math.MaxInt64 {
		return false, "", FromKernel(lxphost.Code(-3))
	}
	live, err := k.host.ReadIsLive(ctx.WithBlockTime(time.UnixMilli(int64(timestampMs))), grantID, sequence)
	if err != nil {
		reason, readErr := authRefusal(err)
		return false, reason, readErr
	}
	if live {
		return true, "", nil
	}
	grant, err := k.host.ReadGrant(ctx, grantID)
	if err != nil {
		return false, "", authReadError(err)
	}
	// ReadIsLive folds these kernel refusals into false. Preserve their order.
	code := int32(-203)
	switch {
	case timestampMs < grant.NotBefore:
		code = -304
	case timestampMs >= grant.NotAfter:
		code = -202
	case grant.Kind == 8 && ((grant.Scope.EarliestTimestamp != 0 && timestampMs < grant.Scope.EarliestTimestamp) ||
		(grant.Scope.EarliestSequence != 0 && sequence < grant.Scope.EarliestSequence)):
		code = -222
	}
	return false, pcommon.LXPCodeName(code), nil
}

func (k *Keeper) SessionKey(ctx sdk.Context, grantID [32]byte) ([]byte, error) {
	key, err := k.host.ReadSessionKey(ctx, grantID)
	return key, authReadError(err)
}

func (k *Keeper) CapabilityScope(ctx sdk.Context, grantID [32]byte) ([]byte, error) {
	scope, err := k.host.ReadCapabilityScope(ctx, grantID)
	if err != nil {
		return nil, authReadError(err)
	}
	out := encodeAuthScope(scope)
	grant, err := k.host.ReadGrant(ctx, grantID)
	if err != nil {
		return nil, authReadError(err)
	}
	if grant.Kind == 7 || grant.Kind == 8 {
		out = appendAuthExtendedScope(out, scope)
	}
	return out, nil
}

func (k *Keeper) CheckActivity(ctx sdk.Context, grantID [32]byte, activityType uint32, assetID [32]byte, amount sdk.Int) (string, error) {
	grant, err := k.host.ReadGrant(ctx, grantID)
	if err != nil {
		return authRefusal(err)
	}
	sequence, err := k.host.ReadLastSequence(ctx, []byte(evmtypes.LayerXDid(grant.Key)))
	if err != nil {
		return authRefusal(err)
	}
	timestamp := ctx.BlockTime().UnixMilli()
	if timestamp < 0 {
		return "", FromKernel(lxphost.Code(-3))
	}
	live, reason, err := k.IsLive(ctx, grantID, sequence, uint64(timestamp))
	if err != nil || !live {
		return reason, err
	}
	scope, err := k.host.ReadCapabilityScope(ctx, grantID)
	if err != nil {
		return authRefusal(err)
	}
	module, ordinal := activityType>>16, uint16(activityType)
	refusal := pcommon.LXPCodeName(-204)
	if grant.AuthenticationOnly || module >= 64 || scope.ModuleMask&(uint64(1)<<module) == 0 ||
		ordinal < scope.ActivityOrdinalMin || ordinal > scope.ActivityOrdinalMax || assetID != scope.AssetID ||
		amount.IsNil() || amount.IsNegative() || amount.BigInt().BitLen() > 128 {
		return refusal, nil
	}
	if amount.GT(scope.MaximumPerActivity) {
		return refusal, nil
	}
	total := scope.SpentTotal.Add(amount)
	if total.BigInt().BitLen() > 128 || (!scope.MaximumTotal.IsZero() && total.GT(scope.MaximumTotal)) {
		return refusal, nil
	}
	if scope.PeriodLength != 0 {
		if uint64(timestamp) < scope.PeriodStart {
			return pcommon.LXPCodeName(-304), nil
		}
		spent := scope.SpentThisPeriod
		if (uint64(timestamp)-scope.PeriodStart)/scope.PeriodLength != 0 {
			spent = sdk.ZeroInt()
		}
		period := spent.Add(amount)
		if period.BigInt().BitLen() > 128 || period.GT(scope.MaximumPerPeriod) {
			return refusal, nil
		}
	}
	return "", nil
}

func (k *Keeper) ResultByIdempotency(ctx sdk.Context, didKey, idempotencyKey [32]byte) ([]byte, error) {
	receipt, err := k.host.ReadIdempotentReceipt(ctx, []byte(evmtypes.LayerXDid(didKey)), idempotencyKey)
	if err == lxphost.Code(-7) {
		return nil, nil
	}
	if err != nil {
		return nil, authReadError(err)
	}
	return receipt.Encoded, nil
}

func (k *Keeper) RevocationSequence(ctx sdk.Context, didKey [32]byte) (uint64, error) {
	identity, err := k.host.ReadIdentityState(ctx, []byte(evmtypes.LayerXDid(didKey)))
	if err != nil {
		return 0, authReadError(err)
	}
	return identity.RevocationSequence, nil
}

func authReadError(err error) error {
	var code lxphost.Code
	if errors.As(err, &code) {
		return FromKernel(code)
	}
	return err
}

func authRefusal(err error) (string, error) {
	var code lxphost.Code
	if errors.As(err, &code) && code < 0 && code > -1000 {
		return pcommon.LXPCodeName(int32(code)), nil
	}
	return "", authReadError(err)
}

func authBytes(out, value []byte) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(value)))
	return append(out, value...)
}

func authAmount(out []byte, amount sdk.Int) []byte {
	var value [16]byte
	amount.BigInt().FillBytes(value[:])
	return append(out, value[:]...)
}

func encodeAuthScope(scope lxphost.AuthorityScope) []byte {
	out := binary.BigEndian.AppendUint64(nil, scope.ModuleMask)
	out = binary.BigEndian.AppendUint16(out, scope.ActivityOrdinalMin)
	out = binary.BigEndian.AppendUint16(out, scope.ActivityOrdinalMax)
	out = authBytes(out, scope.AssetID[:])
	out = authAmount(out, scope.MaximumPerActivity)
	out = authAmount(out, scope.MaximumTotal)
	out = authAmount(out, scope.SpentTotal)
	out = binary.BigEndian.AppendUint64(out, scope.PeriodLength)
	out = authAmount(out, scope.MaximumPerPeriod)
	out = authAmount(out, scope.SpentThisPeriod)
	out = binary.BigEndian.AppendUint64(out, scope.PeriodStart)
	return authBytes(out, scope.PurposeHash[:])
}

func appendAuthExtendedScope(out []byte, scope lxphost.AuthorityScope) []byte {
	out = append(out, scope.SignerThreshold, scope.SignerCount)
	var signers, approvals []byte
	for i := uint8(0); i < scope.SignerCount; i++ {
		signers = append(signers, scope.Signers[i][:]...)
	}
	out = authBytes(out, signers)
	out = append(out, scope.ApprovalCount)
	for i := uint8(0); i < scope.ApprovalCount; i++ {
		approvals = append(approvals, scope.Approvals[i][:]...)
	}
	out = authBytes(out, approvals)
	out = binary.BigEndian.AppendUint64(out, scope.EarliestSequence)
	return binary.BigEndian.AppendUint64(out, scope.EarliestTimestamp)
}
