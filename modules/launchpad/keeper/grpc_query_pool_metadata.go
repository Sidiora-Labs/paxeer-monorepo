package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
)

func (q Querier) PoolMetadata(goCtx context.Context, req *types.QueryPoolMetadataRequest) (*types.QueryPoolMetadataResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	pool, err := types.ParseHexAddress("pool", req.Pool)
	if err != nil {
		return nil, err
	}
	metadata, found, err := q.GetPoolMetadata(ctx, pool)
	if err != nil {
		return nil, err
	}
	response := &types.QueryPoolMetadataResponse{Found: found}
	if found {
		response.Metadata = &types.PoolMetadataInfo{
			Creator:      metadata.Creator.Hex(),
			Token:        metadata.Token.Hex(),
			Optical:      metadata.Optical.Hex(),
			NFTID:        metadata.NFTID,
			CreatedAt:    metadata.CreatedAt,
			CreatedBlock: metadata.CreatedBlock,
		}
	}
	return response, nil
}
