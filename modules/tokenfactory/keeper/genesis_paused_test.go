package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/stretchr/testify/require"
)

func TestGenesisPausedRoundTrip(t *testing.T) {
	source := new(KeeperTestSuite)
	source.SetT(t)
	source.SetupTest()
	creator := source.TestAccs[0].String()
	paused := "factory/" + creator + "/paused"
	unpaused := "factory/" + creator + "/unpaused"
	adminless := "factory/" + creator + "/adminless"
	genesis := types.GenesisState{
		Params: types.DefaultParams(),
		FactoryDenoms: []types.GenesisDenom{
			{Denom: paused, AuthorityMetadata: types.DenomAuthorityMetadata{Admin: creator}},
			{Denom: unpaused, AuthorityMetadata: types.DenomAuthorityMetadata{Admin: creator}},
			{Denom: adminless, AuthorityMetadata: types.DenomAuthorityMetadata{}},
		},
		PausedDenoms: []string{paused, adminless},
	}

	require.NotPanics(t, func() {
		source.App.TokenFactoryKeeper.InitGenesis(source.Ctx, genesis)
	})
	require.True(t, source.App.TokenFactoryKeeper.IsDenomPaused(source.Ctx, paused))
	require.True(t, source.App.TokenFactoryKeeper.IsDenomPaused(source.Ctx, adminless))
	require.False(t, source.App.TokenFactoryKeeper.IsDenomPaused(source.Ctx, unpaused))
	exported := source.App.TokenFactoryKeeper.ExportGenesis(source.Ctx)
	require.NotNil(t, exported)
	require.Equal(t, []string{adminless, paused}, exported.GetPausedDenoms())
	require.ElementsMatch(t, genesis.FactoryDenoms, exported.GetFactoryDenoms())
	require.Equal(t, genesis.Params, exported.Params)

	restored := new(KeeperTestSuite)
	restored.SetT(t)
	restored.SetupTest()
	require.Empty(t, restored.App.TokenFactoryKeeper.GetPausedDenoms(restored.Ctx))
	require.Empty(t, restored.App.TokenFactoryKeeper.ExportGenesis(restored.Ctx).GetFactoryDenoms())
	require.NotPanics(t, func() {
		restored.App.TokenFactoryKeeper.InitGenesis(restored.Ctx, *exported)
	})
	require.True(t, restored.App.TokenFactoryKeeper.IsDenomPaused(restored.Ctx, paused))
	require.True(t, restored.App.TokenFactoryKeeper.IsDenomPaused(restored.Ctx, adminless))
	require.False(t, restored.App.TokenFactoryKeeper.IsDenomPaused(restored.Ctx, unpaused))
	require.Equal(t, exported, restored.App.TokenFactoryKeeper.ExportGenesis(restored.Ctx))
}

func TestInitGenesisPausedUnknownDenomPanics(t *testing.T) {
	s := new(KeeperTestSuite)
	s.SetT(t)
	s.SetupTest()
	creator := s.TestAccs[0].String()
	known := "factory/" + creator + "/known"
	unknown := "factory/" + creator + "/unknown"
	genesis := types.GenesisState{
		Params: types.DefaultParams(),
		FactoryDenoms: []types.GenesisDenom{
			{Denom: known, AuthorityMetadata: types.DenomAuthorityMetadata{Admin: creator}},
		},
		PausedDenoms: []string{unknown},
	}

	require.PanicsWithValue(t, types.ErrDenomDoesNotExist, func() {
		s.App.TokenFactoryKeeper.InitGenesis(s.Ctx, genesis)
	})
	require.False(t, s.App.TokenFactoryKeeper.IsDenomPaused(s.Ctx, unknown))
}
