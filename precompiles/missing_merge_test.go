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
	registered := []ecommon.Address{ecommon.HexToAddress("0x0907"), ecommon.HexToAddress("0x0901")}
	for _, address := range registered {
		require.NoError(t, RegisterMergeConstructor(address, constructor))
	}

	want := make([]MergeAddress, 0, len(MergeAddressTable)-len(registered))
	for _, entry := range MergeAddressTable {
		if entry.Address != registered[0] && entry.Address != registered[1] {
			want = append(want, entry)
		}
	}
	require.Len(t, want, len(MergeAddressTable)-len(registered))
	require.Equal(t, want, MissingMergeConstructors())
}
