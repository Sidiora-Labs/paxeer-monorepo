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
	poolCount, err := q.feeAccountingCount(ctx, feeAccountingPoolCountKey)
	if err != nil {
		return nil, err
	}
	epochCount, err := q.feeAccountingCount(ctx, feeAccountingEpochCountKey)
	if err != nil {
		return nil, err
	}
	claimCount, err := q.feeAccountingCount(ctx, feeAccountingClaimCountKey)
	if err != nil {
		return nil, err
	}
	return &types.QueryFeeAccountingResponse{
		Accounting: types.FeeAccountingInfo{
			PendingProtocolFees: sdk.ZeroInt(),
			TotalLiabilities:    liabilities,
			PoolCount:           poolCount,
			EpochCount:          epochCount,
			ClaimCount:          claimCount,
		},
	}, nil
}
