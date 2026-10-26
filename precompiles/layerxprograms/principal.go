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
	_, accountID, err := lxphost.MainAccountID(didKey)
	if err != nil {
		return lxphost.KernelAuthority{}, nil, err
	}
	return lxphost.KernelAuthority{
		Actor:       didKey,
		Principal:   accountID,
		VerifiedKey: didKey,
	}, []byte(evmtypes.LayerXDid(didKey)), nil
}
