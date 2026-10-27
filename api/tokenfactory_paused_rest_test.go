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
	"github.com/gogo/gateway"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
)

type tokenfactoryPausedRESTFixture struct {
	keeper  keeper.Keeper
	ctx     sdk.Context
	creator string
}

func (f tokenfactoryPausedRESTFixture) CreateDenom(t *testing.T, subdenom string) string {
	t.Helper()
	denom, err := f.keeper.CreateDenom(f.ctx, f.creator, subdenom)
	if err != nil {
		t.Fatalf("create factory denom %q: %v", subdenom, err)
	}
	if want := "factory/" + f.creator + "/" + subdenom; denom != want {
		t.Fatalf("created denom = %q, want %q", denom, want)
	}
	return denom
}

func DenomPaused(t *testing.T, client *http.Client, endpoint string) bool {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatalf("create denom paused request: %v", err)
	}
	response, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET denom paused query route: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read denom paused query response: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want 200; body: %s", endpoint, response.StatusCode, body)
	}
	var payload struct {
		Paused *bool `json:"paused"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode denom paused query response: %v; body: %s", err, body)
	}
	if payload.Paused == nil {
		t.Fatalf("denom paused query response has no boolean paused field; body: %s", body)
	}
	return *payload.Paused
}

func TestTokenfactoryDenomPausedFactoryDenomOverREST(t *testing.T) {
	app, ctx, queryHelper := appQueryHelper(t)
	creator := sdk.AccAddress([]byte("paused-rest-creator1")).String()
	fixture := tokenfactoryPausedRESTFixture{keeper: app.TokenFactoryKeeper, ctx: ctx, creator: creator}
	pausedDenom := fixture.CreateDenom(t, "halt")
	unpausedDenom := fixture.CreateDenom(t, "active")
	msgServer := keeper.NewMsgServerImpl(app.TokenFactoryKeeper)
	if _, err := msgServer.SetDenomPaused(sdk.WrapSDKContext(ctx), tokenfactorytypes.NewMsgSetDenomPaused(creator, pausedDenom, true)); err != nil {
		t.Fatalf("pause factory denom: %v", err)
	}

	tokenfactorytypes.RegisterQueryServer(queryHelper, app.TokenFactoryKeeper)
	server := serveGateway(t, func(ctx context.Context, mux *runtime.ServeMux) error {
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &gateway.JSONPb{
			EmitDefaults: true,
			OrigName:     true,
			AnyResolver:  app.InterfaceRegistry(),
		})(mux)
		return tokenfactorytypes.RegisterQueryHandlerClient(ctx, mux, tokenfactorytypes.NewQueryClient(queryHelper))
	})

	for _, tc := range []struct {
		name   string
		denom  string
		paused bool
	}{
		{name: "paused", denom: pausedDenom, paused: true},
		{name: "unpaused", denom: unpausedDenom, paused: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := server.URL + "/pax-protocol/paxchain/tokenfactory/denom_paused?denom=" + url.QueryEscape(tc.denom)
			if got := DenomPaused(t, server.Client(), endpoint); got != tc.paused {
				t.Errorf("denom %q: paused = %t, want %t", tc.denom, got, tc.paused)
			}
		})
	}

	pathEndpoint := server.URL + "/pax-protocol/paxchain/tokenfactory/denoms/" + pausedDenom + "/paused"
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, pathEndpoint, nil)
	if err != nil {
		t.Fatalf("create factory denom path request: %v", err)
	}
	response, err := server.Client().Do(req)
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
