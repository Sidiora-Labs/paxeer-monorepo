package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	tokenfactorytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
)

func TestTokenfactoryAuthorityMetadataFactoryDenomOverREST(t *testing.T) {
	app, ctx, queryHelper := appQueryHelper(t)
	creator := sdk.AccAddress([]byte("tokenfactory-creator")).String()
	denom := CreateDenom(t, app.TokenFactoryKeeper, ctx, creator, "meta")

	tokenfactorytypes.RegisterQueryServer(queryHelper, app.TokenFactoryKeeper)
	server := serveGateway(t, func(ctx context.Context, mux *runtime.ServeMux) error {
		return tokenfactorytypes.RegisterQueryHandlerClient(ctx, mux, tokenfactorytypes.NewQueryClient(queryHelper))
	})

	endpoint := server.URL + "/pax-protocol/paxchain/tokenfactory/denom_authority_metadata?denom=" + url.QueryEscape(denom)
	metadata := DenomAuthorityMetadata(t, server.Client(), endpoint)
	if got := metadata.AuthorityMetadata.Admin; got != creator {
		t.Errorf("authority_metadata.admin = %q, want creator %q", got, creator)
	}

	pathEndpoint := server.URL + "/pax-protocol/paxchain/tokenfactory/denoms/" + denom + "/authority_metadata"
	response, err := server.Client().Get(pathEndpoint)
	if err != nil {
		t.Fatalf("GET factory denom path route: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read factory denom path response: %v", err)
	}
	if response.StatusCode != http.StatusNotFound {
		t.Errorf("GET %s: status = %d, want 404; body: %s", pathEndpoint, response.StatusCode, body)
	}
}

func CreateDenom(t *testing.T, tokenFactoryKeeper keeper.Keeper, ctx sdk.Context, creator, subdenom string) string {
	t.Helper()
	denom, err := tokenFactoryKeeper.CreateDenom(ctx, creator, subdenom)
	if err != nil {
		t.Fatalf("create factory denom: %v", err)
	}
	if want := "factory/" + creator + "/" + subdenom; denom != want {
		t.Fatalf("created denom = %q, want %q", denom, want)
	}
	return denom
}

func DenomAuthorityMetadata(t *testing.T, client *http.Client, endpoint string) tokenfactorytypes.QueryDenomAuthorityMetadataResponse {
	t.Helper()
	response, err := client.Get(endpoint)
	if err != nil {
		t.Fatalf("GET authority metadata query route: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read authority metadata query response: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want 200; body: %s", endpoint, response.StatusCode, body)
	}
	var metadata tokenfactorytypes.QueryDenomAuthorityMetadataResponse
	if err := json.Unmarshal(body, &metadata); err != nil {
		t.Fatalf("decode authority metadata query response: %v; body: %s", err, body)
	}
	return metadata
}
