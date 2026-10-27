package launchpad_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRegisterServicesServesQuery(t *testing.T) {
	registry := cdctypes.NewInterfaceRegistry()
	basic := launchpad.NewAppModuleBasic()
	basic.RegisterInterfaces(registry)
	msgRouter := baseapp.NewMsgServiceRouter()
	msgRouter.SetInterfaceRegistry(registry)
	queryRouter := baseapp.NewGRPCQueryRouter()
	queryRouter.SetInterfaceRegistry(registry)
	key := sdk.NewKVStoreKey(types.StoreKey)
	k := keeper.NewKeeper(key, nil, nil, nil)
	am := launchpad.NewAppModule(k)
	cdc := codec.NewProtoCodec(registry)
	am.RegisterServices(module.NewConfigurator(cdc, msgRouter, queryRouter))

	require.NotNil(t, msgRouter.Handler(&types.MsgUpdateParams{}))
	require.NotNil(t, msgRouter.Handler(&types.MsgTreasuryWithdraw{}))
	require.Equal(t, uint64(1), am.ConsensusVersion())
	methods := []string{
		"Params", "Market", "MarketByAddress", "Markets", "MarketsByCreator",
		"PriceHistory", "Quote", "Treasury", "Surface", "Retired", "FeeRight",
		"AirdropEpoch", "AirdropClaimed", "HasRole", "QuoteLedger", "FeeAccounting", "PoolMetadata",
	}
	require.Len(t, methods, 17)
	ctx := testutil.DefaultContext(key, sdk.NewTransientStoreKey("transient_launchpad"))
	params := types.DefaultParams()
	params.CreationFee = sdk.NewInt(7654321)
	params.QuoteToken = "0x0000000000000000000000000000000000001017"
	require.NoError(t, k.UpdateParams(ctx, k.Authority(), params))
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			handler := queryRouter.Route("/paxprotocol.paxchain.launchpad.Query/" + method)
			require.NotNil(t, handler)
			response, err := handler(ctx, abci.RequestQuery{})
			switch method {
			case "Params":
				require.NoError(t, err)
				var result types.QueryParamsResponse
				require.NoError(t, cdc.Unmarshal(response.Value, &result))
				require.Equal(t, params, result.Params)
			case "Market":
				require.Equal(t, codes.InvalidArgument, status.Code(err))
				require.Equal(t, "empty denom", status.Convert(err).Message())
			case "Surface":
				require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
				address, err := types.ParseHexAddress("address", "0x0000000000000000000000000000000000001234")
				require.NoError(t, err)
				surface := keeper.Surface{Role: types.RolePool, Generation: types.Generation1, Market: "surface-market"}
				require.NoError(t, k.SetSurface(ctx, address, surface))
				data, err := cdc.Marshal(&types.QuerySurfaceRequest{Address: address.Hex()})
				require.NoError(t, err)
				response, err = handler(ctx, abci.RequestQuery{Data: data})
				require.NoError(t, err)
				var result types.QuerySurfaceResponse
				require.NoError(t, cdc.Unmarshal(response.Value, &result))
				require.Equal(t, types.QuerySurfaceResponse{
					Found: true,
					Surface: &types.SurfaceInfo{
						Role: surface.Role, Generation: uint32(surface.Generation), Market: surface.Market,
					},
				}, result)
			case "Retired":
				require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
				address, err := types.ParseHexAddress("address", "0x0000000000000000000000000000000000001235")
				require.NoError(t, err)
				retired := types.RetiredAddress{
					Address: address.Hex(), Role: types.RoleRouter,
					Replacement: keeper.Generation2Surfaces[types.RoleRouter].Hex(),
				}
				require.NoError(t, k.RecordRetired(ctx, retired))
				data, err := cdc.Marshal(&types.QueryRetiredRequest{Address: address.Hex()})
				require.NoError(t, err)
				response, err = handler(ctx, abci.RequestQuery{Data: data})
				require.NoError(t, err)
				var result types.QueryRetiredResponse
				require.NoError(t, cdc.Unmarshal(response.Value, &result))
				info := types.RetiredInfoOf(retired)
				require.Equal(t, types.QueryRetiredResponse{Found: true, Retired: &info}, result)
			case "FeeRight":
				require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
				collection, err := types.ParseHexAddress("collection", "0x0000000000000000000000000000000000001236")
				require.NoError(t, err)
				owner, err := types.ParseHexAddress("owner", "0x0000000000000000000000000000000000001237")
				require.NoError(t, err)
				pool, err := types.ParseHexAddress("pool", "0x0000000000000000000000000000000000001238")
				require.NoError(t, err)
				id, err := k.MintFeeRight(ctx, collection, owner, pool, types.FeeStrategyClaim)
				require.NoError(t, err)
				right, found, err := k.GetFeeRight(ctx, collection, id)
				require.NoError(t, err)
				require.True(t, found)
				data, err := cdc.Marshal(&types.QueryFeeRightRequest{Collection: collection.Hex(), Id: id})
				require.NoError(t, err)
				response, err = handler(ctx, abci.RequestQuery{Data: data})
				require.NoError(t, err)
				var result types.QueryFeeRightResponse
				require.NoError(t, cdc.Unmarshal(response.Value, &result))
				info := types.FeeRightInfoOf(right)
				require.Equal(t, types.QueryFeeRightResponse{Found: true, FeeRight: &info}, result)
			case "Treasury":
				require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
				data, err := cdc.Marshal(&types.QueryTreasuryRequest{Token: params.QuoteToken})
				require.NoError(t, err)
				response, err = handler(ctx, abci.RequestQuery{Data: data})
				require.NoError(t, err)
				var result types.QueryTreasuryResponse
				require.NoError(t, cdc.Unmarshal(response.Value, &result))
				require.Equal(t, sdk.ZeroInt().String(), result.Balance)
				require.Equal(t, sdk.ZeroInt().String(), result.Held)
			case "Quote":
				require.Equal(t, codes.InvalidArgument, status.Code(err))
				require.Equal(t, "amount_in must be a positive base-10 integer", status.Convert(err).Message())
				data, err := cdc.Marshal(&types.QueryQuoteRequest{Denom: "unknown-market", IsBuy: true, AmountIn: "1000000"})
				require.NoError(t, err)
				_, err = handler(ctx, abci.RequestQuery{Data: data})
				require.ErrorIs(t, err, types.ErrUnknownMarket)
				var launchpadErr *sdkerrors.Error
				require.ErrorAs(t, err, &launchpadErr)
				codespace, code, _ := sdkerrors.ABCIInfo(launchpadErr, false)
				require.Equal(t, types.ErrUnknownMarket.Codespace(), codespace)
				require.Equal(t, types.ErrUnknownMarket.ABCICode(), code)
			case "PriceHistory":
				require.ErrorIs(t, err, types.ErrUnknownMarket)
				require.Equal(t, ": "+types.ErrUnknownMarket.Error(), err.Error())
				codespace, code, _ := sdkerrors.ABCIInfo(err, false)
				require.Equal(t, types.ErrUnknownMarket.Codespace(), codespace)
				require.Equal(t, types.ErrUnknownMarket.ABCICode(), code)
			case "MarketByAddress":
				require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
				codespace, code, _ := sdkerrors.ABCIInfo(err, false)
				require.Equal(t, sdkerrors.ErrInvalidAddress.Codespace(), codespace)
				require.Equal(t, sdkerrors.ErrInvalidAddress.ABCICode(), code)
			case "Markets":
				require.ErrorIs(t, err, types.ErrInvalidRange)
				codespace, code, _ := sdkerrors.ABCIInfo(err, false)
				require.Equal(t, types.ErrInvalidRange.Codespace(), codespace)
				require.Equal(t, types.ErrInvalidRange.ABCICode(), code)
				data, err := cdc.Marshal(&types.QueryMarketsRequest{Limit: 1})
				require.NoError(t, err)
				response, err = handler(ctx, abci.RequestQuery{Data: data})
				require.NoError(t, err)
				var result types.QueryMarketsResponse
				require.NoError(t, cdc.Unmarshal(response.Value, &result))
				require.Empty(t, result.Markets)
				require.Equal(t, k.GetMarketCount(ctx), result.Total)
			case "MarketsByCreator":
				require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
				codespace, code, _ := sdkerrors.ABCIInfo(err, false)
				require.Equal(t, sdkerrors.ErrInvalidAddress.Codespace(), codespace)
				require.Equal(t, sdkerrors.ErrInvalidAddress.ABCICode(), code)
				creator := sdk.AccAddress([]byte("launchpad creator 01"))
				data, err := cdc.Marshal(&types.QueryMarketsByCreatorRequest{Creator: creator.String(), Limit: 1})
				require.NoError(t, err)
				response, err = handler(ctx, abci.RequestQuery{Data: data})
				require.NoError(t, err)
				var result types.QueryMarketsByCreatorResponse
				require.NoError(t, cdc.Unmarshal(response.Value, &result))
				require.Empty(t, result.Markets)
			default:
				require.Equal(t, codes.Unimplemented, status.Code(err))
				require.Equal(t, "method "+method+" not implemented", status.Convert(err).Message())
			}
		})
	}
}

func TestRegisterGatewayRoutesMountsLaunchpad(t *testing.T) {
	mux := runtime.NewServeMux()
	launchpad.NewAppModuleBasic().RegisterGRPCGatewayRoutes(client.Context{}, mux)
	paths := []string{
		"params", "market?denom=token", "market_by_address/0x1017", "markets",
		"markets_by_creator/creator", "price_history?denom=token", "quote?denom=token",
		"treasury/token", "surface/0x1017", "retired/0x1012", "fee_right/collection/1",
		"airdrop_epoch/pool/1", "airdrop_claimed/pool/holder/1", "has_role/contract/role/account",
		"quote_ledger/QUOTE_LEDGER_KIND_HELD", "fee_accounting", "pool_metadata/pool",
	}
	require.Len(t, paths, 17)
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/pax-protocol/paxchain/launchpad/"+path, nil)
			mux.ServeHTTP(response, request)
			require.Equal(t, http.StatusInternalServerError, response.Code)
			var body struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			require.Equal(t, int(codes.Unknown), body.Code)
			require.Equal(t, "no RPC client is defined in offline mode", body.Message)
		})
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/pax-protocol/paxchain/launchpad/unknown", nil))
	require.Equal(t, http.StatusNotFound, response.Code)
}
