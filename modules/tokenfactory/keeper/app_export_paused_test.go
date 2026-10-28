package keeper_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/apptesting"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	"github.com/stretchr/testify/require"
)

func ExportGenesis(t *testing.T, source *apptesting.KeeperTestHelper) servertypes.ExportedApp {
	t.Helper()
	_, err := source.App.Commit(context.Background())
	require.NoError(t, err)
	exported, err := source.App.ExportAppStateAndValidators(false, nil)
	require.NoError(t, err)
	require.NotEmpty(t, exported.AppState)
	return exported
}

func InitGenesis(t *testing.T, exported servertypes.ExportedApp, header tmproto.Header) *apptesting.KeeperTestHelper {
	t.Helper()
	restored := new(apptesting.KeeperTestHelper)
	restored.SetT(t)
	restored.App = app.Setup(t, true, false, false)
	t.Cleanup(func() { require.NoError(t, restored.App.Close()) })
	ctx := restored.App.NewUncachedContext(false, header)
	require.Empty(t, restored.App.TokenFactoryKeeper.GetPausedDenoms(ctx))
	denoms := restored.App.TokenFactoryKeeper.GetAllDenomsIterator(ctx)
	require.False(t, denoms.Valid())
	require.NoError(t, denoms.Close())
	_, err := restored.App.InitChain(context.Background(), &abci.RequestInitChain{
		Time:            header.Time,
		ChainId:         header.ChainID,
		ConsensusParams: exported.ConsensusParams,
		AppStateBytes:   exported.AppState,
		InitialHeight:   exported.Height,
	})
	require.NoError(t, err)
	header.Height = exported.Height
	restored.Ctx = restored.App.NewContext(false, header)
	return restored
}

func TestAppExportImportKeepsPausedDenoms(t *testing.T) {
	source := new(apptesting.KeeperTestHelper)
	source.SetT(t)
	source.Setup()
	source.SetupTokenFactory()
	t.Cleanup(func() { require.NoError(t, source.App.Close()) })
	creator := source.TestAccs[0].String()
	issuerDid := "did:layerx:" + strings.Repeat("ab", 32)
	k := source.App.TokenFactoryKeeper
	paused, err := k.CreateDenom(source.Ctx, creator, "export-paused")
	require.NoError(t, err)
	unpaused, err := k.CreateDenom(source.Ctx, creator, "export-unpaused")
	require.NoError(t, err)
	require.NoError(t, k.SetIssuerDid(source.Ctx, paused, issuerDid))
	require.NoError(t, k.SetDenomPausedByIssuer(source.Ctx, paused, issuerDid, true))
	require.True(t, k.IsDenomPaused(source.Ctx, paused))
	require.False(t, k.IsDenomPaused(source.Ctx, unpaused))
	storedDid, err := k.IssuerDid(source.Ctx, paused)
	require.NoError(t, err)
	require.Equal(t, issuerDid, storedDid)

	exported := ExportGenesis(t, source)
	var appState app.GenesisState
	require.NoError(t, json.Unmarshal(exported.AppState, &appState))
	require.Contains(t, appState, types.ModuleName)
	var tokenfactoryState types.GenesisState
	require.NoError(t, source.App.AppCodec().UnmarshalAsJSON(appState[types.ModuleName], &tokenfactoryState))
	require.Equal(t, []string{paused}, tokenfactoryState.PausedDenoms)
	expectedDenoms := []types.GenesisDenom{
		{Denom: paused, AuthorityMetadata: types.DenomAuthorityMetadata{Admin: creator, IssuerDid: issuerDid}},
		{Denom: unpaused, AuthorityMetadata: types.DenomAuthorityMetadata{Admin: creator}},
	}
	require.ElementsMatch(t, expectedDenoms, tokenfactoryState.FactoryDenoms)

	restored := InitGenesis(t, exported, source.Ctx.BlockHeader())
	require.NotSame(t, source.App, restored.App)
	require.True(t, restored.App.TokenFactoryKeeper.IsDenomPaused(restored.Ctx, paused))
	require.False(t, restored.App.TokenFactoryKeeper.IsDenomPaused(restored.Ctx, unpaused))
	require.Equal(t, []string{paused}, restored.App.TokenFactoryKeeper.GetPausedDenoms(restored.Ctx))
	storedDid, err = restored.App.TokenFactoryKeeper.IssuerDid(restored.Ctx, paused)
	require.NoError(t, err)
	require.Equal(t, issuerDid, storedDid)
	for _, expected := range expectedDenoms {
		metadata, err := restored.App.TokenFactoryKeeper.GetAuthorityMetadata(restored.Ctx, expected.Denom)
		require.NoError(t, err)
		require.Equal(t, expected.AuthorityMetadata, metadata)
	}
}
