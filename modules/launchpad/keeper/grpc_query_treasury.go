package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
)

func (q Querier) Treasury(goCtx context.Context, req *types.QueryTreasuryRequest) (*types.QueryTreasuryResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	token, err := types.ParseHexAddress("token", req.Token)
	if err != nil {
		return nil, err
	}
	balance, err := q.TreasuryGetBalance(ctx, token)
	if err != nil {
		return nil, err
	}
	held, err := q.TreasuryHeld(ctx, token)
	if err != nil {
		return nil, err
	}
	return &types.QueryTreasuryResponse{Balance: balance.String(), Held: held.String()}, nil
}
