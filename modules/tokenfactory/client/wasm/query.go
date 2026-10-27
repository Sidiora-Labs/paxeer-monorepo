package wasm

import (
	tokenfactorykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TokenFactoryWasmQueryHandler struct {
	tokenfactoryKeeper tokenfactorykeeper.Keeper
}

func NewTokenFactoryWasmQueryHandler(keeper *tokenfactorykeeper.Keeper) *TokenFactoryWasmQueryHandler {
	return &TokenFactoryWasmQueryHandler{
		tokenfactoryKeeper: *keeper,
	}
}

func (handler TokenFactoryWasmQueryHandler) GetDenomAuthorityMetadata(ctx sdk.Context, req *types.QueryDenomAuthorityMetadataRequest) (*types.QueryDenomAuthorityMetadataResponse, error) {
	c := sdk.WrapSDKContext(ctx)
	return handler.tokenfactoryKeeper.DenomAuthorityMetadata(c, req)
}

func (handler TokenFactoryWasmQueryHandler) GetDenomsFromCreator(ctx sdk.Context, req *types.QueryDenomsFromCreatorRequest) (*types.QueryDenomsFromCreatorResponse, error) {
	denoms := handler.tokenfactoryKeeper.GetAllDenomsFromCreator(ctx, req.Creator)
	return &types.QueryDenomsFromCreatorResponse{Denoms: denoms}, nil
}

func (handler TokenFactoryWasmQueryHandler) GetDenomPaused(ctx sdk.Context, req *types.QueryDenomPausedRequest) (*types.QueryDenomPausedResponse, error) {
	res, err := handler.tokenfactoryKeeper.DenomPaused(sdk.WrapSDKContext(ctx), req)
	if err != nil {
		return nil, err
	}
	if !handler.tokenfactoryKeeper.GetDenomPrefixStore(ctx, req.Denom).Has([]byte(types.DenomAuthorityMetadataKey)) {
		return nil, status.Errorf(codes.NotFound, "authority metadata for denom %s not found", req.Denom)
	}
	return res, nil
}
