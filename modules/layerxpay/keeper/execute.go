package keeper

import (
	"errors"
	"fmt"
	"math"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

// Submit commits one bound kernel activity, discarding all writes on refusal.
func (k Keeper) Submit(ctx sdk.Context, caller common.Address, method string, payload []byte, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	if err := k.CheckWrite(ctx, method); err != nil {
		return lxphost.Receipt{}, err
	}
	authority, err := k.AuthorityFor(ctx, caller)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	mapping, ok := types.MethodActivity[method]
	if !ok {
		return lxphost.Receipt{}, fmt.Errorf("LXP_UNKNOWN_ACTIVITY: layerxpay method %q", method)
	}
	last, err := k.host.ReadLastSequence(ctx, authority.ActorDID)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	identitySequence, err := k.host.ReadIdentitySequence(ctx, authority.ActorDID)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	if last == math.MaxUint64 || identitySequence == math.MaxUint64 {
		return lxphost.Receipt{}, submitCodeError(method, lxphost.Code(-307))
	}
	window := k.host.TimestampWindowMs()
	blockTime := ctx.BlockTime().UnixMilli()
	if blockTime < 0 || uint64(blockTime) > math.MaxUint64-window {
		return lxphost.Receipt{}, fmt.Errorf("LXP_NON_CANONICAL: layerxpay timestamp window")
	}
	activity := lxphost.Activity{
		ProtocolVersion: 3,
		Type:            mapping.Type,
		ActorDID:        authority.ActorDID,
		AuthorityKey:    authority.AuthorityKey,
		AccountSequence: identitySequence,
		NotBefore:       uint64(blockTime),
		NotAfter:        uint64(blockTime) + window,
		IdempotencyKey:  idempotencyKey,
		Payload:         payload,
	}
	_, principal, err := lxphost.MainAccountID(authority.AuthorityKey)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	authority.Kernel.Principal = principal
	authority.Kernel.Bound = true
	// HostExecute persists refused receipts too; publish its writes only on success.
	branch, commit := ctx.CacheContext()
	receipt, err := k.host.HostExecute(lxphost.WithSequence(branch, last+1), activity, authority.Kernel)
	if err != nil {
		var code lxphost.Code
		if errors.As(err, &code) {
			return receipt, submitCodeError(method, code)
		}
		return receipt, err
	}
	if receipt.ResultCode != 0 {
		return receipt, submitCodeError(method, receipt.ResultCode)
	}
	commit()
	return receipt, nil
}

func submitCodeError(method string, code lxphost.Code) error {
	return fmt.Errorf("%s: layerxpay %s: %w", pcommon.LXPCodeName(int32(code)), method, code)
}
