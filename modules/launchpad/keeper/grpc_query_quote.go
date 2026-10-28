package keeper

import (
	"context"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q Querier) Quote(goCtx context.Context, req *types.QueryQuoteRequest) (*types.QueryQuoteResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	for _, digit := range req.AmountIn {
		if digit < '0' || digit > '9' {
			return nil, status.Error(codes.InvalidArgument, "amount_in must be a positive base-10 integer")
		}
	}
	amount, ok := sdk.NewIntFromString(strings.TrimLeft(req.AmountIn, "0"))
	if !ok || !amount.IsPositive() {
		return nil, status.Error(codes.InvalidArgument, "amount_in must be a positive base-10 integer")
	}
	ctx, _ := sdk.UnwrapSDKContext(goCtx).CacheContext()
	var result SwapResult
	var err error
	if req.IsBuy {
		result, err = q.QuoteBuy(ctx, req.Denom, amount.BigInt())
	} else {
		result, err = q.QuoteSell(ctx, req.Denom, amount.BigInt())
	}
	if err != nil {
		return nil, err
	}
	return &types.QueryQuoteResponse{Quote: types.SwapQuote{
		AmountIn:    sdk.NewIntFromBigInt(result.AmountIn),
		AmountOut:   sdk.NewIntFromBigInt(result.AmountOut),
		FeeBps:      sdk.NewIntFromBigInt(result.FeeBps),
		FeeAmount:   sdk.NewIntFromBigInt(result.FeeAmount),
		ProtocolCut: sdk.NewIntFromBigInt(result.ProtocolCut),
		PoolCut:     sdk.NewIntFromBigInt(result.PoolCut),
	}}, nil
}
