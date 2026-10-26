package keeper

import (
	"encoding/hex"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

type CallerAuthority struct {
	ActorDID     []byte
	AuthorityKey [32]byte
	Kernel       lxphost.KernelAuthority
}

func (k Keeper) AuthorityFor(ctx sdk.Context, caller common.Address) (CallerAuthority, error) {
	key, bound := k.evm.GetLayerXDid(ctx, caller)
	if !bound {
		return CallerAuthority{}, fmt.Errorf("%w: %s", types.ErrUnboundCaller, caller.Hex())
	}
	did := []byte("did:layerx:" + hex.EncodeToString(key[:]))
	actor, err := lxphost.DIDID(did)
	if err != nil {
		return CallerAuthority{}, fmt.Errorf("layerxpay: derive actor for %s: %w", caller.Hex(), err)
	}
	return CallerAuthority{
		ActorDID:     did,
		AuthorityKey: key,
		Kernel:       lxphost.KernelAuthority{Actor: actor, Principal: actor, VerifiedKey: key},
	}, nil
}
