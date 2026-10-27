package layerxprograms

import (
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

type EVMKeeper interface {
	GetLayerXDid(ctx sdk.Context, a common.Address) ([32]byte, bool)
}

func principalOf(ctx sdk.Context, evm EVMKeeper, caller common.Address) (lxphost.KernelAuthority, []byte, error) {
	didKey, bound := evm.GetLayerXDid(ctx, caller)
	if !bound {
		return lxphost.KernelAuthority{}, nil, ErrUnboundCaller
	}
	did := []byte(evmtypes.LayerXDid(didKey))
	actorID, err := lxphost.DIDID(did)
	if err != nil {
		return lxphost.KernelAuthority{}, nil, err
	}
	return lxphost.KernelAuthority{
		Bound:       true,
		Actor:       actorID,
		Principal:   actorID,
		VerifiedKey: didKey,
	}, did, nil
}
