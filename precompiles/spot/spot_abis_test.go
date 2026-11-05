package spot

import (
	"testing"

	spotabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot/abi"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func TestPrecompileABIsWithoutKeepers(t *testing.T) {
	abis := PrecompileABIs()
	require.Len(t, abis, 5)

	expected := map[string]func() (abi.ABI, error){
		"spot_pecor":      spotabi.PECOR,
		"spot_orders":     spotabi.Orders,
		"spot_stoporders": spotabi.StopOrders,
		"spot_quoter":     spotabi.Quoter,
		"spot_vault":      spotabi.Vault,
	}
	for name, parse := range expected {
		t.Run(name, func(t *testing.T) {
			parsed, ok := abis[name]
			require.True(t, ok, "missing precompile %s", name)
			require.NotEmpty(t, parsed.Methods)
			want, err := parse()
			require.NoError(t, err)
			require.Equal(t, want, parsed)
		})
	}

	set, err := newPrecompileSet(nil)
	require.EqualError(t, err, "spot: the application keepers carry no spot keeper")
	require.Nil(t, set)
}
