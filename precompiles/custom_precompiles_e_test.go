package precompiles

import (
	"errors"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	ecommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestGetCustomPrecompilesEReturnsMergeError(t *testing.T) {
	mergeConstructorsMtx.Lock()
	previous := mergeConstructors
	mergeConstructors = make(map[ecommon.Address]MergeConstructor)
	mergeConstructorsMtx.Unlock()
	t.Cleanup(func() {
		mergeConstructorsMtx.Lock()
		defer mergeConstructorsMtx.Unlock()
		mergeConstructors = previous
	})

	address := ecommon.HexToAddress("0x0908")
	constructorErr := errors.New("eip712 constructor failed")
	calls := 0
	require.NoError(t, RegisterMergeConstructor(address, func(keepers utils.Keepers, planName string) (IPrecompile, error) {
		calls++
		require.Equal(t, MergeUpgrade, planName)
		return nil, constructorErr
	}))

	custom, err := GetCustomPrecompilesE(MergeUpgrade, nil)
	require.Nil(t, custom)
	require.ErrorIs(t, err, constructorErr)
	require.EqualError(t, err, "merge precompile eip712 at "+address.Hex()+": "+constructorErr.Error())
	require.Equal(t, 1, calls)

	func() {
		defer func() {
			panicErr, ok := recover().(error)
			require.True(t, ok, "GetCustomPrecompiles must panic with an error")
			require.ErrorIs(t, panicErr, constructorErr)
			require.EqualError(t, panicErr, err.Error())
		}()
		GetCustomPrecompiles(MergeUpgrade, nil)
	}()
	require.Equal(t, 2, calls)
}
