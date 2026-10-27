package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

func (q Querier) MarketsByCreator(goCtx context.Context, req *types.QueryMarketsByCreatorRequest) (*types.QueryMarketsByCreatorResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	creator, err := sdk.AccAddressFromBech32(req.Creator)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, err.Error())
	}
	if err := types.ValidatePage(req.Offset, req.Limit); err != nil {
		return nil, sdkerrors.Wrap(types.ErrInvalidRange, err.Error())
	}
	markets := q.GetMarketsByCreator(ctx, creator, req.Offset, req.Limit)
	response := &types.QueryMarketsByCreatorResponse{Markets: make([]types.MarketInfo, len(markets))}
	for index, market := range markets {
		response.Markets[index] = types.MarketInfoOf(market)
	}
	return response, nil
}
