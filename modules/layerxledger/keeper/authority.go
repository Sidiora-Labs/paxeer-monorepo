package keeper

import (
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

type Actor struct {
	DIDKey    [32]byte
	DID       []byte
	Authority lxphost.KernelAuthority
}

func (k *Keeper) Actor(ctx sdk.Context, caller common.Address) (Actor, error) {
	key, bound := k.evmKeeper.GetLayerXDid(ctx, caller)
	if !bound {
		return Actor{}, types.ErrUnauthorizedDebit
	}
	did := []byte(evmtypes.LayerXDid(key))
	id, err := lxphost.DIDID(did)
	if err != nil {
		return Actor{}, err
	}
	return Actor{
		DIDKey: key,
		DID:    did,
		Authority: lxphost.KernelAuthority{
			Bound:       true,
			Actor:       id,
			Principal:   id,
			VerifiedKey: key,
		},
	}, nil
}
