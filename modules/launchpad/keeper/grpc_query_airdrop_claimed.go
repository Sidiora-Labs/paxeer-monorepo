package keeper

import (
	"context"
	"errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
)

func (q Querier) AirdropClaimed(goCtx context.Context, req *types.QueryAirdropClaimedRequest) (*types.QueryAirdropClaimedResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	pool, err := types.ParseHexAddress("pool", req.Pool)
	if err != nil {
		return nil, errors.Unwrap(err)
	}
	holder, err := types.ParseHexAddress("holder", req.Holder)
	if err != nil {
		return nil, errors.Unwrap(err)
	}
	return &types.QueryAirdropClaimedResponse{Claimed: q.HasClaimedPoolAirdrop(ctx, pool, holder, req.Epoch)}, nil
}
