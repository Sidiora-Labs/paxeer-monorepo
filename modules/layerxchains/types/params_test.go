package types_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestDefaultParamsAreBankBaseUnits(t *testing.T) {
	params := types.DefaultParams()
	require.Equal(t, sdk.NewInt(100_000_000), params.MinBond)
	require.Equal(t, sdk.NewInt(10_000_000), params.CreationFee)
	require.NoError(t, params.Validate())
	require.Equal(t, params, types.DefaultGenesis().Params)
}
