package keeper

import (
	"context"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (q Querier) FeeAccounting(goCtx context.Context, req *types.QueryFeeAccountingRequest) (*types.QueryFeeAccountingResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	liabilities, err := q.TotalLiabilities(ctx)
	if err != nil {
		return nil, err
	}
	return &types.QueryFeeAccountingResponse{
		Accounting: types.FeeAccountingInfoOf(types.FeeAccounting{
			PendingProtocolFees: sdk.ZeroInt(),
			TotalLiabilities:    liabilities,
		}),
	}, nil
}
