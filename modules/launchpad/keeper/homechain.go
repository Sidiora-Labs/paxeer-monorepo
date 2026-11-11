package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func (k *Keeper) checkHomeChain(ctx sdk.Context, pool common.Address) error {
	home := k.HomeChainByPool(ctx, pool)
	if home != 0 && home != chainstypes.ChainIDFrom(ctx) {
		return types.ErrWrongChain
	}
	return nil
}
