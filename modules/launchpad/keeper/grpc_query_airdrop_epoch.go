package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
)

func (q Querier) AirdropEpoch(goCtx context.Context, req *types.QueryAirdropEpochRequest) (*types.QueryAirdropEpochResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	pool, err := types.ParseHexAddress("pool", req.Pool)
	if err != nil {
		return nil, err
	}
	if req.Epoch == 0 {
		return nil, types.ErrInvalidAirdropEpoch
	}
	record, found, err := q.GetPoolAirdropEpoch(ctx, pool, req.Epoch)
	if err != nil {
		return nil, err
	}
	if !found {
		return &types.QueryAirdropEpochResponse{Found: false}, nil
	}
	info := types.AirdropEpochInfoOf(record)
	return &types.QueryAirdropEpochResponse{Found: true, AirdropEpoch: &info}, nil
}
