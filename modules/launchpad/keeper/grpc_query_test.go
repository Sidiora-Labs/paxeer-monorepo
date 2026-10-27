package keeper_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQuerierRegistersOnGRPCRouter(t *testing.T) {
	app := testkeeper.EVMTestApp
	ctx, _ := app.GetContextForDeliverTx([]byte{}).CacheContext()
	querier := keeper.NewQuerier(app.LaunchpadKeeper)
	require.Same(t, app.LaunchpadKeeper, querier.Keeper)
	var server types.QueryServer = querier
	router := baseapp.NewQueryServerTestHelper(ctx, app.InterfaceRegistry())
	types.RegisterQueryServer(router, server)
	client := types.NewQueryClient(router)

	cases := []struct {
		name       string
		call       func(*testing.T, context.Context) error
		nilRequest func(*testing.T, context.Context) error
	}{
		{
			name: "Params",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.Params(goCtx, &types.QueryParamsRequest{})
				require.NoError(t, err)
				require.NotNil(t, response)
				require.Equal(t, querier.GetParams(ctx), response.Params)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.Params(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
		{
			name: "Market",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.Market(goCtx, &types.QueryMarketRequest{})
				require.Nil(t, response)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.Market(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
		{
			name: "MarketByAddress",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.MarketByAddress(goCtx, &types.QueryMarketByAddressRequest{})
				require.Nil(t, response)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.MarketByAddress(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
		{
			name: "Markets",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.Markets(goCtx, &types.QueryMarketsRequest{Limit: 1})
				require.NoError(t, err)
				require.NotNil(t, response)
				require.Empty(t, response.Markets)
				require.Equal(t, querier.GetMarketCount(ctx), response.Total)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.Markets(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
		{
			name: "MarketsByCreator",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.MarketsByCreator(goCtx, &types.QueryMarketsByCreatorRequest{})
				require.Nil(t, response)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.MarketsByCreator(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
		{
			name: "PriceHistory",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.PriceHistory(goCtx, &types.QueryPriceHistoryRequest{})
				require.Nil(t, response)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.PriceHistory(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
		{
			name: "Quote",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.Quote(goCtx, &types.QueryQuoteRequest{})
				require.Nil(t, response)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.Quote(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
		{
			name: "Treasury",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.Treasury(goCtx, &types.QueryTreasuryRequest{})
				require.Nil(t, response)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.Treasury(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
		{
			name: "Surface",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.Surface(goCtx, &types.QuerySurfaceRequest{})
				require.Nil(t, response)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.Surface(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
		{
			name: "Retired",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.Retired(goCtx, &types.QueryRetiredRequest{})
				require.Nil(t, response)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.Retired(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
		{
			name: "FeeRight",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.FeeRight(goCtx, &types.QueryFeeRightRequest{})
				require.Nil(t, response)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.FeeRight(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
		{
			name: "AirdropEpoch",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.AirdropEpoch(goCtx, &types.QueryAirdropEpochRequest{})
				require.Nil(t, response)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.AirdropEpoch(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
		{
			name: "AirdropClaimed",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.AirdropClaimed(goCtx, &types.QueryAirdropClaimedRequest{})
				require.Nil(t, response)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.AirdropClaimed(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
		{
			name: "HasRole",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.HasRole(goCtx, &types.QueryHasRoleRequest{})
				require.Nil(t, response)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.HasRole(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
		{
			name: "QuoteLedger",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.QuoteLedger(goCtx, &types.QueryQuoteLedgerRequest{})
				require.Nil(t, response)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.QuoteLedger(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
		{
			name: "FeeAccounting",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.FeeAccounting(goCtx, &types.QueryFeeAccountingRequest{})
				require.Nil(t, response)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.FeeAccounting(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
		{
			name: "PoolMetadata",
			call: func(t *testing.T, goCtx context.Context) error {
				response, err := client.PoolMetadata(goCtx, &types.QueryPoolMetadataRequest{})
				require.Nil(t, response)
				return err
			},
			nilRequest: func(t *testing.T, goCtx context.Context) error {
				response, err := server.PoolMetadata(goCtx, nil)
				require.Nil(t, response)
				return err
			},
		},
	}
	compressed, _ := (&types.QueryParamsRequest{}).Descriptor()
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	var file descriptor.FileDescriptorProto
	require.NoError(t, proto.Unmarshal(data, &file))
	var query *descriptor.ServiceDescriptorProto
	for _, service := range file.GetService() {
		if service.GetName() == "Query" {
			query = service
		}
	}
	require.NotNil(t, query)
	require.Equal(t, "paxprotocol.paxchain.launchpad", file.GetPackage())
	require.Len(t, cases, len(query.GetMethod()))
	for _, method := range query.GetMethod() {
		require.NotNil(t, router.Route("/"+file.GetPackage()+".Query/"+method.GetName()))
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, router.Route("/paxprotocol.paxchain.launchpad.Query/"+tc.name))
			err := tc.call(t, sdk.WrapSDKContext(ctx))
			switch tc.name {
			case "Params", "Markets":
				require.NoError(t, err)
			case "Market":
				require.Equal(t, codes.InvalidArgument, status.Code(err))
				require.Equal(t, "empty denom", status.Convert(err).Message())
			case "MarketByAddress":
				require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
				codespace, code, _ := sdkerrors.ABCIInfo(err, false)
				require.Equal(t, sdkerrors.ErrInvalidAddress.Codespace(), codespace)
				require.Equal(t, sdkerrors.ErrInvalidAddress.ABCICode(), code)
			default:
				require.Equal(t, codes.Unimplemented, status.Code(err))
				require.Equal(t, "method "+tc.name+" not implemented", status.Convert(err).Message())
			}

			err = tc.nilRequest(t, sdk.WrapSDKContext(ctx))
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			require.Equal(t, "empty request", status.Convert(err).Message())

			err = tc.nilRequest(t, context.Background())
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			require.Equal(t, "empty request", status.Convert(err).Message())
		})
	}
}
