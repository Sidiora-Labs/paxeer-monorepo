package spot_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestNewServedSetWrapsEveryRail(t *testing.T) {
	f := newPECORFixture(t)
	keepers := spotKeepers{testkeeper.EVMTestApp.GetPrecompileKeepers(), f.keeper}
	set, err := spot.NewServedSet(keepers)
	require.NoError(t, err)
	addresses := []common.Address{
		common.HexToAddress("0x1021"),
		common.HexToAddress("0x1022"),
		common.HexToAddress("0x1023"),
		common.HexToAddress("0x1024"),
		common.HexToAddress("0x1025"),
	}
	require.Len(t, set, 5)
	for _, address := range addresses {
		require.Contains(t, set, address)
		contract := set[address]
		require.NotNil(t, contract, address.Hex())
		require.IsType(t, (*spot.Precompile)(nil), contract, address.Hex())
		require.Implements(t, (*vm.DynamicGasPrecompiledContract)(nil), contract, address.Hex())
		require.Equal(t, address, contract.Address())
	}
	for _, address := range []common.Address{common.HexToAddress("0x1026"), common.HexToAddress("0x1027")} {
		require.NotContains(t, set, address)
	}

	const latestUpgrade = "v7.0.0"
	versioned, err := spot.GetVersioned(latestUpgrade, keepers)
	require.NoError(t, err)
	require.Len(t, versioned, 5)
	for _, address := range addresses {
		require.Contains(t, versioned, address)
		versions := versioned[address]
		require.Len(t, versions, 2)
		require.Contains(t, versions, latestUpgrade)
		require.Contains(t, versions, "v6.12.0")
		require.IsType(t, (*spot.Precompile)(nil), versions[latestUpgrade])
		require.Same(t, versions[latestUpgrade], versions["v6.12.0"])
	}
}

func TestNewServedSetRefusesMissingSpotKeeper(t *testing.T) {
	keepers := testkeeper.EVMTestApp.GetPrecompileKeepers()
	set, err := spot.NewServedSet(spotKeepers{keepers, nil})
	require.EqualError(t, err, "spot: the application keepers carry no spot keeper")
	require.Nil(t, set)
}
