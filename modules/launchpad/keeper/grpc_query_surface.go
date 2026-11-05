package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
)

func (q Querier) Surface(goCtx context.Context, req *types.QuerySurfaceRequest) (*types.QuerySurfaceResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	address, err := types.ParseHexAddress("address", req.Address)
	if err != nil {
		return nil, err
	}
	surface, found, err := q.GetSurface(ctx, address)
	if err != nil {
		return nil, err
	}
	if !found {
		return &types.QuerySurfaceResponse{Found: false}, nil
	}
	return &types.QuerySurfaceResponse{
		Found: true,
		Surface: &types.SurfaceInfo{
			Role:       surface.Role,
			Generation: uint32(surface.Generation),
			Market:     surface.Market,
		},
	}, nil
}
