package ibc_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	ibc "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/types"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
)

func TestCoreParamsSurviveExportAndImport(t *testing.T) {
	testCases := []struct {
		name   string
		params types.Params
	}{
		{name: "both disabled", params: types.NewParams(false, false)},
		{name: "inbound disabled", params: types.NewParams(false, true)},
		{name: "outbound disabled", params: types.NewParams(true, false)},
		{name: "both enabled", params: types.NewParams(true, true)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			coordinator := ibctesting.NewCoordinator(t, 2)
			chainA := coordinator.GetChain(ibctesting.GetChainID(1))
			chainB := coordinator.GetChain(ibctesting.GetChainID(2))
			keeperA := chainA.App.GetIBCKeeper()
			keeperB := chainB.App.GetIBCKeeper()
			keeperA.SetParams(chainA.GetContext(), tc.params)
			keeperB.SetParams(chainB.GetContext(), types.NewParams(!tc.params.InboundEnabled, !tc.params.OutboundEnabled))

			gs := ibc.ExportGenesis(chainA.GetContext(), *keeperA)
			require.NotNil(t, gs.Params)
			require.Equal(t, tc.params, *gs.Params)

			ibc.InitGenesis(chainB.GetContext(), *keeperB, true, gs)
			require.Equal(t, tc.params, keeperB.GetParams(chainB.GetContext()))
		})
	}
}

func TestInitGenesisWithoutParamsEnablesBoth(t *testing.T) {
	coordinator := ibctesting.NewCoordinator(t, 1)
	chain := coordinator.GetChain(ibctesting.GetChainID(1))
	keeper := chain.App.GetIBCKeeper()
	keeper.SetParams(chain.GetContext(), types.NewParams(false, false))
	gs := types.DefaultGenesisState()
	gs.Params = nil

	ibc.InitGenesis(chain.GetContext(), *keeper, true, gs)
	params := keeper.GetParams(chain.GetContext())
	require.Equal(t, types.DefaultParams(), params)
	require.True(t, params.InboundEnabled)
	require.True(t, params.OutboundEnabled)
}
