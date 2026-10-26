package launchpad_test

import (
	"context"
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
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
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
	k := keeper.NewKeeper(sdk.NewKVStoreKey(types.StoreKey), nil, nil, nil)
	am := launchpad.NewAppModule(k)
	am.RegisterServices(module.NewConfigurator(codec.NewProtoCodec(registry), msgRouter, queryRouter))

	require.NotNil(t, msgRouter.Handler(&types.MsgUpdateParams{}))
	require.NotNil(t, msgRouter.Handler(&types.MsgTreasuryWithdraw{}))
	require.Equal(t, uint64(1), am.ConsensusVersion())
	methods := []string{
		"Params", "Market", "MarketByAddress", "Markets", "MarketsByCreator",
		"PriceHistory", "Quote", "Treasury", "Surface", "Retired", "FeeRight",
		"AirdropEpoch", "AirdropClaimed", "HasRole", "QuoteLedger", "FeeAccounting", "PoolMetadata",
	}
	require.Len(t, methods, 17)
	ctx := sdk.Context{}.WithContext(context.Background())
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			handler := queryRouter.Route("/paxprotocol.paxchain.launchpad.Query/" + method)
			require.NotNil(t, handler)
			_, err := handler(ctx, abci.RequestQuery{})
			require.Equal(t, codes.Unimplemented, status.Code(err))
			require.Equal(t, "method "+method+" not implemented", status.Convert(err).Message())
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
