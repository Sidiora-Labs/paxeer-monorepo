package pointerview_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/cw1155"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/cw20"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/cw721"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/native"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pointerview"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestPointerViewGasGolden(t *testing.T) {
	const mergeHeight int64 = 100
	a := testkeeper.EVMTestApp
	k := &a.EvmKeeper
	ctx, _ := a.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(mergeHeight - 1).WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(1), "v6.4.0")
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(mergeHeight), pcommon.MergeUpgrade)
	require.Equal(t, mergeHeight, a.UpgradeKeeper.GetDoneHeight(ctx, pcommon.MergeUpgrade))

	p, err := pointerview.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	cwAddress := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes()).String()
	pointer := common.HexToAddress("0x2345678901234567890123456789012345678901")
	require.NoError(t, k.SetERC20NativePointer(ctx, "uhpx", pointer))
	require.NoError(t, k.SetERC20CW20Pointer(ctx, cwAddress, pointer))
	require.NoError(t, k.SetERC721CW721Pointer(ctx, cwAddress, pointer))
	require.NoError(t, k.SetERC1155CW1155Pointer(ctx, cwAddress, pointer))

	golden := map[string]struct {
		argument string
		gas      uint64
		version  uint16
	}{
		"getNativePointer": {argument: "uhpx", gas: 2000, version: native.CurrentVersion},
		"getCW20Pointer":   {argument: cwAddress, gas: 2000, version: cw20.CurrentVersion(ctx)},
		"getCW721Pointer":  {argument: cwAddress, gas: 2000, version: cw721.CurrentVersion},
		"getCW1155Pointer": {argument: cwAddress, gas: 2000, version: cw1155.CurrentVersion},
	}
	require.Len(t, golden, len(p.ABI.Methods), "every ABI method must have a frozen gas value")
	for name := range p.ABI.Methods {
		_, ok := golden[name]
		require.True(t, ok, "missing gas golden for %s", name)
	}

	for name, row := range golden {
		t.Run(name, func(t *testing.T) {
			input, err := p.ABI.Pack(name, row.argument)
			require.NoError(t, err)
			var gas [2]uint64
			for i, height := range []int64{mergeHeight - 1, mergeHeight} {
				t.Run(fmt.Sprintf("height_%d", height), func(t *testing.T) {
					at := ctx.WithBlockHeight(height)
					require.Equal(t, height >= mergeHeight, pcommon.MergePlanActive(at, k))
					served, ok := k.CustomPrecompiles(at)[p.Address()]
					require.True(t, ok, "pointerview must be served at height %d", height)
					gas[i] = p.RequiredGas(input)
					require.Equal(t, row.gas, gas[i], "constructor gas must match the frozen table")
					require.Equal(t, row.gas, served.RequiredGas(input), "served gas must match the frozen table")

					evm := &vm.EVM{StateDB: state.NewDBImpl(at, k, false)}
					ret, err := p.Run(evm, common.Address{}, common.Address{}, input, nil, true, false, nil)
					require.NoError(t, err)
					outputs, err := p.ABI.Methods[name].Outputs.Unpack(ret)
					require.NoError(t, err)
					require.Equal(t, []interface{}{pointer, row.version, true}, outputs)
				})
			}
			require.Equal(t, gas[0], gas[1], "merge activation must preserve RequiredGas")
		})
	}
}
