package keeper

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

func (s *msgServer) MarketSymbol(ctx sdk.Context, marketID uint64) (string, bool, error) {
	symbol, enabled, err := s.keeper.pricefeed.MarketSymbol(ctx, marketID)
	if errors.Is(err, pricefeedtypes.ErrUnknownMarket) {
		return "", false, fmt.Errorf("%w: pricefeed market %d", types.ErrUnknownMarket, marketID)
	}
	return symbol, enabled, err
}

func (s *msgServer) SetMarket(goCtx context.Context, msg *types.MsgSetMarket) (*types.MsgSetMarketResponse, error) {
	if msg == nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap("missing message")
	}
	if msg.Authority != s.keeper.Authority() {
		return nil, fmt.Errorf("%w: %s", types.ErrUnauthorized, msg.Authority)
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	cacheCtx, write := ctx.CacheContext()
	market := msg.Market
	stored, err := s.keeper.GetMarket(cacheCtx, market.ID)
	switch {
	case errors.Is(err, types.ErrUnknownMarket):
	case err != nil:
		return nil, err
	case market.Symbol != stored.Symbol || market.OracleSymbol != stored.OracleSymbol ||
		market.SzDecimals != stored.SzDecimals || market.PxDecimals != stored.PxDecimals:
		return nil, fmt.Errorf("%w: market %d identity cannot change", types.ErrInvalidParams, market.ID)
	}
	symbol, _, err := s.MarketSymbol(cacheCtx, market.ID)
	if err != nil {
		return nil, err
	}
	if symbol != market.OracleSymbol {
		return nil, fmt.Errorf("%w: market %d oracle symbol %q differs from pricefeed symbol %q",
			types.ErrUnknownMarket, market.ID, market.OracleSymbol, symbol)
	}
	if err := s.keeper.SetMarket(cacheCtx, msg.Authority, market); err != nil {
		return nil, err
	}
	cacheCtx.EventManager().EmitEvent(sdk.NewEvent("perps_market_set",
		sdk.NewAttribute("market_id", strconv.FormatUint(market.ID, 10)),
		sdk.NewAttribute("enabled", strconv.FormatBool(market.Enabled))))
	write()
	ctx.EventManager().EmitEvents(cacheCtx.EventManager().Events())
	return &types.MsgSetMarketResponse{}, nil
}
