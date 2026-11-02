package precompiles_test

import (
	"testing"

	gigaprecompiles "github.com/Sidiora-Labs/Paxeer-X-Network/engine/executor/precompiles"
	gigautils "github.com/Sidiora-Labs/Paxeer-X-Network/engine/executor/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestSelfDestructAbortError(t *testing.T) {
	abortErr, ok := gigaprecompiles.ErrSelfDestructUnsupported.(vm.AbortError)
	require.True(t, ok, "ErrSelfDestructUnsupported must implement vm.AbortError")
	require.True(t, abortErr.IsAbortError())
	require.NotEmpty(t, gigaprecompiles.ErrSelfDestructUnsupported.Error())

	// ShouldExecutionAbort must recognize it so the giga executor falls back to v2.
	require.True(t, gigautils.ShouldExecutionAbort(gigaprecompiles.ErrSelfDestructUnsupported))
}

func TestLateFailFastPrecompiles(t *testing.T) {
	late := gigaprecompiles.LateFailFastPrecompileAddresses
	require.Len(t, late, 6)
	for _, addr := range gigaprecompiles.FailFastPrecompileAddresses {
		require.Same(t, gigaprecompiles.FailFastSingleton, gigaprecompiles.AllCustomPrecompilesFailFast[addr])
		require.Same(t, gigaprecompiles.FailFastSingleton, gigaprecompiles.AllCustomPrecompilesFailFastLate[addr])
		require.NotContains(t, late, addr)
	}
	for _, addr := range late {
		require.Same(t, gigaprecompiles.FailFastSingleton, gigaprecompiles.AllCustomPrecompilesFailFastLate[addr])
		require.NotContains(t, gigaprecompiles.AllCustomPrecompilesFailFast, addr)
	}
	require.Len(t, gigaprecompiles.AllCustomPrecompilesFailFastLate, len(gigaprecompiles.FailFastPrecompileAddresses)+len(late))
	for _, hex := range []string{
		"0x0000000000000000000000000000000000001015",
		"0x0000000000000000000000000000000000001016",
		"0x0000000000000000000000000000000000001017",
		"0x0000000000000000000000000000000000001018",
		"0x0000000000000000000000000000000000001019",
		"0x000000000000000000000000000000000000101B",
	} {
		require.Contains(t, late, common.HexToAddress(hex))
	}
	_, err := gigaprecompiles.FailFastSingleton.Run(nil, common.Address{}, common.Address{}, nil, nil, false, false, nil)
	require.ErrorIs(t, err, gigaprecompiles.ErrInvalidPrecompileCall)
	require.True(t, gigautils.ShouldExecutionAbort(err))
}

func TestMergeFailFastPrecompiles(t *testing.T) {
	expectedMerge := []common.Address{
		common.HexToAddress("0x101A"),
		common.HexToAddress("0x101C"),
		common.HexToAddress("0x101D"),
		common.HexToAddress("0x0901"),
		common.HexToAddress("0x0902"),
		common.HexToAddress("0x0903"),
		common.HexToAddress("0x0904"),
		common.HexToAddress("0x0905"),
		common.HexToAddress("0x0906"),
		common.HexToAddress("0x0907"),
		common.HexToAddress("0x0908"),
	}
	require.ElementsMatch(t, expectedMerge, gigaprecompiles.MergeFailFastPrecompileAddresses)
	require.Contains(t, gigaprecompiles.LateFailFastPrecompileAddresses, common.HexToAddress("0x101B"))
	require.NotContains(t, gigaprecompiles.MergeFailFastPrecompileAddresses, common.HexToAddress("0x101B"))

	expected := map[common.Address]struct{}{}
	for _, addresses := range [][]common.Address{
		gigaprecompiles.FailFastPrecompileAddresses,
		gigaprecompiles.LateFailFastPrecompileAddresses,
		expectedMerge,
	} {
		for _, addr := range addresses {
			expected[addr] = struct{}{}
			require.Same(t, gigaprecompiles.FailFastSingleton, gigaprecompiles.AllCustomPrecompilesFailFastMerge[addr], "address %s", addr.Hex())
		}
	}
	require.Len(t, gigaprecompiles.AllCustomPrecompilesFailFastMerge, len(expected))

	for _, addr := range gigaprecompiles.FailFastPrecompileAddresses {
		require.Same(t, gigaprecompiles.FailFastSingleton, gigaprecompiles.AllCustomPrecompilesFailFast[addr])
		require.Same(t, gigaprecompiles.FailFastSingleton, gigaprecompiles.AllCustomPrecompilesFailFastLate[addr])
	}
	for _, addr := range gigaprecompiles.LateFailFastPrecompileAddresses {
		require.Same(t, gigaprecompiles.FailFastSingleton, gigaprecompiles.AllCustomPrecompilesFailFastLate[addr])
		require.NotContains(t, gigaprecompiles.AllCustomPrecompilesFailFast, addr)
	}
	for _, addr := range expectedMerge {
		require.NotContains(t, gigaprecompiles.AllCustomPrecompilesFailFast, addr)
		require.NotContains(t, gigaprecompiles.AllCustomPrecompilesFailFastLate, addr)
	}
	require.Len(t, gigaprecompiles.AllCustomPrecompilesFailFast, len(gigaprecompiles.FailFastPrecompileAddresses))
	require.Len(t, gigaprecompiles.LateFailFastPrecompileAddresses, 6)
	require.Len(t, gigaprecompiles.AllCustomPrecompilesFailFastLate, len(gigaprecompiles.FailFastPrecompileAddresses)+len(gigaprecompiles.LateFailFastPrecompileAddresses))
}
