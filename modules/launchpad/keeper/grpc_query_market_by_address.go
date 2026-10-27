package keeper

import (
	"context"
	"errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

func (q Querier) MarketByAddress(goCtx context.Context, req *types.QueryMarketByAddressRequest) (*types.QueryMarketByAddressResponse, error) {
	ctx, err := queryContext(goCtx, req)
	if err != nil {
		return nil, err
	}
	address, err := types.ParseHexAddress("address", req.Address)
	if err != nil {
		return nil, marketByAddressError(err)
	}
	market, found, err := q.poolMarket(ctx, address)
	if err != nil {
		return nil, marketByAddressError(err)
	}
	if !found {
		pool, registered := q.PoolByToken(ctx, address)
		if !registered {
			return nil, marketByAddressError(types.ErrUnknownMarket)
		}
		market, found, err = q.poolMarket(ctx, pool)
		if err != nil {
			return nil, marketByAddressError(err)
		}
		if !found {
			return nil, marketByAddressError(types.ErrUnknownMarket)
		}
	}
	return &types.QueryMarketByAddressResponse{Market: types.MarketInfoOf(market)}, nil
}

func marketByAddressError(err error) error {
	for _, root := range []*sdkerrors.Error{
		sdkerrors.ErrInvalidAddress,
		types.ErrZeroAddress,
		types.ErrInvalidParams,
		types.ErrInvalidConfiguration,
		types.ErrUnknownMarket,
	} {
		if errors.Is(err, root) {
			return sdkerrors.Wrap(root, err.Error())
		}
	}
	return err
}
