package keeper

import (
	"context"
	"errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

type feeRightQueryError struct {
	err error
}

func (e feeRightQueryError) Error() string { return e.err.Error() }

func (e feeRightQueryError) Unwrap() error { return e.err }

func (e feeRightQueryError) Cause() error {
	for _, root := range []*sdkerrors.Error{
		sdkerrors.ErrInvalidAddress,
		types.ErrZeroAddress,
		types.ErrOverflow,
	} {
		if errors.Is(e.err, root) {
			return root
		}
	}
	return e.err
}

func (q Querier) FeeRight(goCtx context.Context, req *types.QueryFeeRightRequest) (*types.QueryFeeRightResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	collection, err := types.ParseHexAddress("collection", req.Collection)
	if err != nil {
		return nil, feeRightQueryError{err: err}
	}
	right, found, err := q.GetFeeRight(ctx, collection, req.Id)
	if err != nil {
		return nil, feeRightQueryError{err: err}
	}
	if !found {
		return &types.QueryFeeRightResponse{}, nil
	}
	info := types.FeeRightInfoOf(right)
	return &types.QueryFeeRightResponse{Found: true, FeeRight: &info}, nil
}
