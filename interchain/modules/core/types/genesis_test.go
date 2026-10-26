package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/types"
)

func TestDefaultGenesisStateCarriesDefaultCoreParams(t *testing.T) {
	genesis := types.DefaultGenesisState()

	require.NotNil(t, genesis.Params)
	require.Equal(t, types.DefaultParams(), *genesis.Params)
	require.True(t, genesis.Params.InboundEnabled)
	require.True(t, genesis.Params.OutboundEnabled)
	require.NoError(t, genesis.Validate())
}

func TestGenesisValidateAcceptsAbsentCoreParams(t *testing.T) {
	genesis := types.DefaultGenesisState()
	genesis.Params = nil

	require.NoError(t, genesis.Validate())
	require.Nil(t, genesis.Params)
}
