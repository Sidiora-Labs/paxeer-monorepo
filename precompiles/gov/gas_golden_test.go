package gov_test

import (
	_ "embed"
	"strings"
	"testing"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/gov"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

//go:embed abi.json
var govGasABIJSON string

func TestGovGasGolden(t *testing.T) {
	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx(nil).CacheContext()
	const mergeHeight int64 = 100
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(1), "v6.4.0")
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(mergeHeight), pcommon.MergeUpgrade)
	require.Equal(t, mergeHeight, a.UpgradeKeeper.GetDoneHeight(ctx, pcommon.MergeUpgrade))
	below := ctx.WithBlockHeight(mergeHeight - 1)
	at := ctx.WithBlockHeight(mergeHeight)
	require.False(t, pcommon.MergePlanActive(below, &a.EvmKeeper))
	require.True(t, pcommon.MergePlanActive(at, &a.EvmKeeper))

	constructed, err := gov.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	govABI, err := abi.JSON(strings.NewReader(govGasABIJSON))
	require.NoError(t, err)
	require.Equal(t, govABI, constructed.GetABI())

	belowContract, ok := a.EvmKeeper.CustomPrecompiles(below)[constructed.Address()].(*pcommon.Precompile)
	require.True(t, ok, "gov must be served below the merge plan height")
	atContract, ok := a.EvmKeeper.CustomPrecompiles(at)[constructed.Address()].(*pcommon.Precompile)
	require.True(t, ok, "gov must be served at the merge plan height")
	require.Equal(t, govABI, belowContract.GetABI())
	require.Equal(t, govABI, atContract.GetABI())

	golden := map[string]struct {
		args []interface{}
		gas  uint64
	}{
		"deposit": {args: []interface{}{uint64(1)}, gas: 30_000},
		"submitProposal": {
			args: []interface{}{`{"type":"Text","title":"Gas golden proposal","description":"Fixed gas golden proposal","is_expedited":false}`},
			gas:  50_000,
		},
		"vote": {args: []interface{}{uint64(1), int32(1)}, gas: 30_000},
		"voteWeighted": {
			args: []interface{}{uint64(1), []struct {
				Option int32  `json:"option"`
				Weight string `json:"weight"`
			}{
				{Option: 1, Weight: "0.7"},
				{Option: 2, Weight: "0.3"},
			}},
			gas: 30_000,
		},
	}
	require.Len(t, golden, len(govABI.Methods), "every ABI method must have a gas golden row")
	for name := range govABI.Methods {
		require.Contains(t, golden, name, "ABI method is missing its gas golden row")
	}
	for name, row := range golden {
		t.Run(name, func(t *testing.T) {
			require.Contains(t, govABI.Methods, name)
			input, err := govABI.Pack(name, row.args...)
			require.NoError(t, err)
			belowGas := belowContract.RequiredGas(input)
			atGas := atContract.RequiredGas(input)
			t.Logf("%s RequiredGas: height %d = %d, height %d = %d, golden = %d", name, mergeHeight-1, belowGas, mergeHeight, atGas, row.gas)
			require.Equal(t, row.gas, belowGas, "gas below the merge plan height must match the frozen table")
			require.Equal(t, row.gas, atGas, "gas at the merge plan height must match the frozen table")
			require.Equal(t, belowGas, atGas, "gas must remain unchanged across the merge plan height")
			require.Equal(t, row.gas, constructed.RequiredGas(input), "the gov constructor must use the frozen gas table")
		})
	}
}
