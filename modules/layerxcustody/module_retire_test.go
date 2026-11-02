package layerxcustody_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody"
	"github.com/stretchr/testify/require"
)

func TestCustodyModuleRegistersNoTxCommand(t *testing.T) {
	basic := layerxcustody.AppModuleBasic{}
	require.Nil(t, basic.GetTxCmd())
	require.Nil(t, basic.GetQueryCmd())
}
