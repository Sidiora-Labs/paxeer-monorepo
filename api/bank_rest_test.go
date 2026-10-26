package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	tokenfactorytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	node "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/gogo/gateway"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
)

func CreateDenom(t *testing.T, app *node.App, ctx sdk.Context) string {
	t.Helper()
	creator := sdk.AccAddress([]byte("bank-rest-creator-01")).String()
	denom, err := app.TokenFactoryKeeper.CreateDenom(ctx, creator, "rest")
	if err != nil {
		t.Fatalf("create factory denom: %v", err)
	}
	if want := "factory/" + creator + "/rest"; denom != want {
		t.Fatalf("created denom = %q, want %q", denom, want)
	}
	return denom
}

func MintCoins(t *testing.T, app *node.App, ctx sdk.Context, denom string) {
	t.Helper()
	coins := sdk.NewCoins(sdk.NewInt64Coin(denom, 1000))
	if err := app.BankKeeper.MintCoins(ctx, tokenfactorytypes.ModuleName, coins); err != nil {
		t.Fatalf("mint factory denom to tokenfactory module: %v", err)
	}
}

func RegisterQueryHandlerClient(t *testing.T, app *node.App, queryHelper *baseapp.QueryServiceTestHelper) *httptest.Server {
	t.Helper()
	banktypes.RegisterQueryServer(queryHelper, app.BankKeeper)
	return serveGateway(t, func(ctx context.Context, mux *runtime.ServeMux) error {
		// Match the production API marshaler, which supports SDK custom scalar types.
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &gateway.JSONPb{
			EmitDefaults: true,
			Indent:       "  ",
			OrigName:     true,
			AnyResolver:  app.InterfaceRegistry(),
		})(mux)
		return banktypes.RegisterQueryHandlerClient(ctx, mux, banktypes.NewQueryClient(queryHelper))
	})
}

func bankRESTGet(t *testing.T, server *httptest.Server, path string, wantStatus int) []byte {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+path, nil)
	if err != nil {
		t.Fatalf("create GET %s: %v", path, err)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read GET %s response: %v", path, err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("GET %s: status %d, want %d; body: %s", path, resp.StatusCode, wantStatus, body)
	}
	return body
}

func TestBankSupplyOfFactoryDenomOverREST(t *testing.T) {
	app, ctx, queryHelper := appQueryHelper(t)
	denom := CreateDenom(t, app, ctx)
	MintCoins(t, app, ctx, denom)
	server := RegisterQueryHandlerClient(t, app, queryHelper)

	body := bankRESTGet(t, server, "/cosmos/bank/v1beta1/supply/by_denom?denom="+url.QueryEscape(denom), http.StatusOK)
	var response struct {
		Amount struct {
			Denom  string `json:"denom"`
			Amount string `json:"amount"`
		} `json:"amount"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode supply response: %v; body: %s", err, body)
	}
	if response.Amount.Denom != denom {
		t.Errorf("amount.denom = %q, want %q", response.Amount.Denom, denom)
	}
	if response.Amount.Amount != "1000" {
		t.Errorf("amount.amount = %q, want %q", response.Amount.Amount, "1000")
	}
	bankRESTGet(t, server, "/cosmos/bank/v1beta1/supply/"+denom, http.StatusNotFound)
}

func TestBankDenomMetadataFactoryDenomOverREST(t *testing.T) {
	app, ctx, queryHelper := appQueryHelper(t)
	denom := CreateDenom(t, app, ctx)
	server := RegisterQueryHandlerClient(t, app, queryHelper)

	body := bankRESTGet(t, server, "/cosmos/bank/v1beta1/denoms_metadata_by_query_string?denom="+url.QueryEscape(denom), http.StatusOK)
	var response struct {
		Metadata struct {
			Base string `json:"base"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode metadata response: %v; body: %s", err, body)
	}
	if response.Metadata.Base != denom {
		t.Errorf("metadata.base = %q, want %q", response.Metadata.Base, denom)
	}
	bankRESTGet(t, server, "/cosmos/bank/v1beta1/denoms_metadata/"+denom, http.StatusNotFound)
}
