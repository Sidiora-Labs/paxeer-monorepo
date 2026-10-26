package pricefeed_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
)

func TestAppModuleRegistersQueryServer(t *testing.T) {
	_, am, k, _, ctx := newModule(t, 1)
	registry := cdctypes.NewInterfaceRegistry()
	am.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	msgRouter := baseapp.NewMsgServiceRouter()
	msgRouter.SetInterfaceRegistry(registry)
	queryRouter := baseapp.NewGRPCQueryRouter()
	am.RegisterServices(module.NewConfigurator(cdc, msgRouter, queryRouter))

	for _, method := range []string{"Params", "Price", "Prices", "FeedKey", "ValidatorByFeedAccount", "Observation"} {
		require.NotNil(t, queryRouter.Route("/paxprotocol.paxchain.pricefeed.Query/"+method), method)
	}
	for _, typeURL := range []string{
		"/paxprotocol.paxchain.pricefeed.MsgSubmitPrices",
		"/paxprotocol.paxchain.pricefeed.MsgSetParams",
	} {
		require.NotNil(t, msgRouter.HandlerByTypeURL(typeURL), typeURL)
	}
	require.Equal(t, uint64(1), am.ConsensusVersion())

	params := types.DefaultParams(types.DefaultAuthority())
	params.StalenessWindowMs = 12000
	require.NoError(t, k.SetParams(ctx, params))
	request, err := cdc.Marshal(&types.QueryParamsRequest{})
	require.NoError(t, err)
	response, err := queryRouter.Route("/paxprotocol.paxchain.pricefeed.Query/Params")(ctx, abci.RequestQuery{
		Data: request, Height: ctx.BlockHeight(),
	})
	require.NoError(t, err)
	var result types.QueryParamsResponse
	require.NoError(t, cdc.Unmarshal(response.Value, &result))
	require.Equal(t, params.Authority, result.Authority)
	require.Equal(t, params.Markets, result.Markets)
	require.Equal(t, params.StalenessWindowMs, result.StalenessWindowMs)

	gateway := runtime.NewServeMux()
	am.RegisterGRPCGatewayRoutes(client.Context{}, gateway)
	for _, path := range []string{
		"params",
		"price/0",
		"prices",
		"feed_key/validator",
		"validator_by_feed_account/feeder",
		"observation/0/validator",
	} {
		t.Run("gateway/"+path, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/paxprotocol/paxchain/pricefeed/"+path, nil)
			gateway.ServeHTTP(response, request)
			require.Equal(t, http.StatusInternalServerError, response.Code)
			require.Contains(t, response.Body.String(), "no RPC client is defined in offline mode")
		})
	}
}
