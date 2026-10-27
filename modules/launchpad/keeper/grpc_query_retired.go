package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
)

func (q Querier) Retired(goCtx context.Context, req *types.QueryRetiredRequest) (*types.QueryRetiredResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	address, err := types.ParseHexAddress("address", req.Address)
	if err != nil {
		return nil, err
	}
	retired, found, err := q.GetRetired(ctx, address)
	if err != nil {
		return nil, err
	}
	if !found {
		return &types.QueryRetiredResponse{Found: false}, nil
	}
	info := types.RetiredInfoOf(retired)
	return &types.QueryRetiredResponse{Found: true, Retired: &info}, nil
}
