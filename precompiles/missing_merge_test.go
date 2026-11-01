package precompiles

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	ecommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMissingMergeConstructorsInTableOrder(t *testing.T) {
	mergeConstructorsMtx.Lock()
	previous := mergeConstructors
	mergeConstructors = make(map[ecommon.Address]MergeConstructor)
	mergeConstructorsMtx.Unlock()
	t.Cleanup(func() {
		mergeConstructorsMtx.Lock()
		defer mergeConstructorsMtx.Unlock()
		mergeConstructors = previous
	})

	require.Equal(t, MergeAddressTable, MissingMergeConstructors())
	constructor := func(keepers utils.Keepers, _ string) (IPrecompile, error) {
		return bank.NewPrecompile(keepers)
	}
	require.NoError(t, RegisterMergeConstructor(ecommon.HexToAddress("0x0907"), constructor))
	require.NoError(t, RegisterMergeConstructor(ecommon.HexToAddress("0x0901"), constructor))

	want := []MergeAddress{
		{ecommon.HexToAddress("0x101A"), "layerxledger"},
		{ecommon.HexToAddress("0x101B"), "layerxpay"},
		{ecommon.HexToAddress("0x101C"), "layerxauth"},
		{ecommon.HexToAddress("0x101D"), "layerxprograms"},
		{ecommon.HexToAddress("0x0902"), "clearing"},
		{ecommon.HexToAddress("0x0903"), "oracle aggregator"},
		{ecommon.HexToAddress("0x0904"), "pofq"},
		{ecommon.HexToAddress("0x0905"), "scheduler"},
		{ecommon.HexToAddress("0x0906"), "streams"},
		{ecommon.HexToAddress("0x0908"), "eip712"},
	}
	require.Equal(t, want, MissingMergeConstructors())
}
