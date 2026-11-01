package mint_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
)

func registerQueryRoutes() *mux.Router {
	router := mux.NewRouter()
	mint.AppModuleBasic{}.RegisterRESTRoutes(client.Context{}, router)
	return router
}

func TestRegisterRESTRoutesMountsNoLegacyQuery(t *testing.T) {
	router := registerQueryRoutes()
	for _, path := range []string{"/minting/parameters", "/minting/minter"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			var match mux.RouteMatch
			require.False(t, router.Match(request, &match), "legacy query route %s is still mounted", path)
			require.ErrorIs(t, match.MatchErr, mux.ErrNotFound)
			require.Nil(t, match.Route)
		})
	}
}

func TestNewProposalHandler(t *testing.T) {
	app := app.Setup(t, false, false, false)

	ctx := app.BaseApp.NewContext(false, tmproto.Header{})
	app.MintKeeper.SetParams(ctx, types.DefaultParams())
	app.MintKeeper.SetMinter(ctx, types.DefaultInitialMinter())

	handler := mint.NewProposalHandler(app.MintKeeper)

	newMinter := types.NewMinter(
		"2023-10-05",
		"2023-11-22",
		"uhpx",
		12345,
	)
	updateMinterProposal := &types.UpdateMinterProposal{
		Title:       "Test Title",
		Description: "Test Description",
		Minter:      &newMinter,
	}
	err := handler(ctx, updateMinterProposal)
	require.NoError(t, err)
	updatedMinter := app.MintKeeper.GetMinter(ctx)
	require.Equal(t, newMinter, updatedMinter)

	invalidMinter := types.NewMinter(
		"2023-11-22",
		"2023-10-05",
		"test",
		12345,
	)
	invalidProposal := &types.UpdateMinterProposal{
		Title:       "Invalid Minter",
		Description: "Invalid Minter",
		Minter:      &invalidMinter,
	}
	err = handler(ctx, invalidProposal)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "end date must be after start")
}
