package types_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/gogo/gateway"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type recordingBankQueryClient struct {
	banktypes.QueryClient
	supplyRequests   []*banktypes.QuerySupplyOfRequest
	metadataRequests []*banktypes.QueryDenomMetadataRequest
	supplyResponse   *banktypes.QuerySupplyOfResponse
	metadataResponse *banktypes.QueryDenomMetadataResponse
}

func (c *recordingBankQueryClient) SupplyOf(_ context.Context, req *banktypes.QuerySupplyOfRequest, _ ...grpc.CallOption) (*banktypes.QuerySupplyOfResponse, error) {
	c.supplyRequests = append(c.supplyRequests, req)
	return c.supplyResponse, nil
}

func (c *recordingBankQueryClient) DenomMetadata(_ context.Context, req *banktypes.QueryDenomMetadataRequest, _ ...grpc.CallOption) (*banktypes.QueryDenomMetadataResponse, error) {
	c.metadataRequests = append(c.metadataRequests, req)
	return c.metadataResponse, nil
}

func TestQueryGatewayServesSupplyOfByDenom(t *testing.T) {
	for _, tc := range []struct {
		name  string
		path  string
		denom string
	}{
		{"query", "/cosmos/bank/v1beta1/supply/by_denom?denom=" + url.QueryEscape("upax"), "upax"},
		{"query with slash", "/cosmos/bank/v1beta1/supply/by_denom?denom=" + url.QueryEscape("factory/pax1creator/token-1"), "factory/pax1creator/token-1"},
		{"existing path", "/cosmos/bank/v1beta1/supply/upax?denom=ignored", "upax"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := &banktypes.QuerySupplyOfResponse{Amount: sdk.NewInt64Coin(tc.denom, 123)}
			client := &recordingBankQueryClient{supplyResponse: want}
			marshaler := &gateway.JSONPb{OrigName: true, EmitDefaults: true}
			mux := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, marshaler))
			require.NoError(t, banktypes.RegisterQueryHandlerClient(context.Background(), mux, client))
			recorder := httptest.NewRecorder()

			mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, []*banktypes.QuerySupplyOfRequest{{Denom: tc.denom}}, client.supplyRequests)
			require.Empty(t, client.metadataRequests)
			var got banktypes.QuerySupplyOfResponse
			require.NoError(t, marshaler.Unmarshal(recorder.Body.Bytes(), &got))
			require.Equal(t, *want, got)
		})
	}
}

func TestQueryGatewayServesDenomMetadataByQueryString(t *testing.T) {
	for _, tc := range []struct {
		name  string
		path  string
		denom string
	}{
		{"query", "/cosmos/bank/v1beta1/denoms_metadata_by_query_string?denom=" + url.QueryEscape("upax"), "upax"},
		{"query with slash", "/cosmos/bank/v1beta1/denoms_metadata_by_query_string?denom=" + url.QueryEscape("ibc/ABCDEF1234"), "ibc/ABCDEF1234"},
		{"existing path", "/cosmos/bank/v1beta1/denoms_metadata/upax?denom=ignored", "upax"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := &banktypes.QueryDenomMetadataResponse{Metadata: banktypes.Metadata{
				Description: "Gateway query denomination",
				DenomUnits:  []*banktypes.DenomUnit{{Denom: tc.denom, Exponent: 0, Aliases: []string{}}},
				Base:        tc.denom,
				Display:     tc.denom,
				Name:        "Paxeer",
				Symbol:      "PAX",
			}}
			client := &recordingBankQueryClient{metadataResponse: want}
			marshaler := &gateway.JSONPb{OrigName: true, EmitDefaults: true}
			mux := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, marshaler))
			require.NoError(t, banktypes.RegisterQueryHandlerClient(context.Background(), mux, client))
			recorder := httptest.NewRecorder()

			mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, []*banktypes.QueryDenomMetadataRequest{{Denom: tc.denom}}, client.metadataRequests)
			require.Empty(t, client.supplyRequests)
			var got banktypes.QueryDenomMetadataResponse
			require.NoError(t, marshaler.Unmarshal(recorder.Body.Bytes(), &got))
			require.Equal(t, *want, got)
		})
	}
}
