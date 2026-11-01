package keeper

import (
	"context"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

type msgServer struct {
	types.UnimplementedMsgServer
	keeper *Keeper
}

var _ types.MsgServer = (*msgServer)(nil)

func NewMsgServerImpl(k *Keeper) types.MsgServer { return &msgServer{keeper: k} }

func (s *msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if msg == nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap("missing message")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	cacheCtx, write := ctx.CacheContext()
	before, err := s.keeper.GetParams(cacheCtx)
	if err != nil {
		return nil, err
	}
	if err := s.keeper.UpdateParams(cacheCtx, msg.Authority, msg.Params); err != nil {
		return nil, err
	}
	if msg.Params.CollateralToken != before.CollateralToken {
		return nil, fmt.Errorf("%w: collateral token cannot change", types.ErrInvalidParams)
	}
	if msg.Params.MaxOpenOrdersPerAccount < before.MaxOpenOrdersPerAccount {
		iterator := sdk.KVStorePrefixIterator(s.keeper.store(cacheCtx), types.AccountPrefix)
		defer iterator.Close()
		var largestOpenOrderCount uint64
		for ; iterator.Valid(); iterator.Next() {
			var account types.Account
			if err := account.Unmarshal(iterator.Value()); err != nil {
				return nil, fmt.Errorf("%w: corrupt account: %v", types.ErrInvalidParams, err)
			}
			if account.OpenOrderCount > largestOpenOrderCount {
				largestOpenOrderCount = account.OpenOrderCount
			}
		}
		if msg.Params.MaxOpenOrdersPerAccount < largestOpenOrderCount {
			return nil, fmt.Errorf("%w: max open orders per account %d below live count %d",
				types.ErrInvalidParams, msg.Params.MaxOpenOrdersPerAccount, largestOpenOrderCount)
		}
	}
	cacheCtx.EventManager().EmitEvent(sdk.NewEvent("perps_params_updated",
		sdk.NewAttribute("authority", msg.Authority)))
	write()
	ctx.EventManager().EmitEvents(cacheCtx.EventManager().Events())
	return &types.MsgUpdateParamsResponse{}, nil
}
