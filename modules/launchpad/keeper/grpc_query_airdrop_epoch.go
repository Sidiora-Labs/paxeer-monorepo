package keeper

import (
	"context"
	"errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

type airdropEpochQueryError struct {
	err error
}

func (e airdropEpochQueryError) Error() string { return e.err.Error() }

func (e airdropEpochQueryError) Unwrap() error { return e.err }

func (e airdropEpochQueryError) Cause() error {
	var root *sdkerrors.Error
	if errors.As(e.err, &root) {
		return root
	}
	return e.err
}

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
		return nil, airdropEpochQueryError{err: err}
	}
	if !found {
		return &types.QueryAirdropEpochResponse{Found: false}, nil
	}
	info := types.AirdropEpochInfoOf(record)
	return &types.QueryAirdropEpochResponse{Found: true, AirdropEpoch: &info}, nil
}
