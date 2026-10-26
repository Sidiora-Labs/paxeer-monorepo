package clearing_test

import (
	"fmt"
	"math/big"
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/clearing"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestClearingMergeConstructorRegistered(t *testing.T) {
	for _, keepers := range []utils.Keepers{nil, &utils.EmptyKeepers{}} {
		p, err := clearing.NewMergePrecompile(keepers, precompiles.MergeUpgrade)
		require.ErrorIs(t, err, agentcommon.ErrNoContext)
		require.Nil(t, p)
	}

	testApp := app.Setup(t, false, true, false)
	keepers := testApp.GetPrecompileKeepers()
	address := ethcommon.HexToAddress("0x0000000000000000000000000000000000000902")
	set := precompiles.GetCustomPrecompiles(app.LatestUpgrade, keepers)
	versions, found := set[address]
	require.True(t, found)
	served, ok := versions[precompiles.MergeUpgrade].(*clearing.ClearingPrecompile)
	require.True(t, ok)
	require.Same(t, served, versions[app.LatestUpgrade])
	require.Equal(t, address, served.Address())
	require.Equal(t, "clearing", served.GetName())
	require.EqualError(t, precompiles.RegisterMergeConstructor(address, clearing.NewMergePrecompile),
		"merge constructor for "+address.Hex()+" already registered")

	k := &testApp.EvmKeeper
	k.SetCustomPrecompileActivation([]evmkeeper.CustomPrecompileActivation{{
		Upgrade: precompiles.MergeUpgrade,
		Modules: map[ethcommon.Address][]string{address: {"layerxagent"}},
	}})
	k.SetCustomPrecompiles(set, app.LatestUpgrade)
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(20)
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	k.UpgradeKeeper().SetDone(ctx, precompiles.MergeUpgrade)
	require.NotContains(t, k.CustomPrecompiles(ctx.WithBlockHeight(19)), address)
	require.Same(t, served, k.CustomPrecompiles(ctx)[address])

	a := served.GetABI()
	input, err := a.Pack(clearing.ComputeClearingMethod, big.NewInt(1000), []mergeClearingOrder{}, []mergeClearingOrder{})
	require.NoError(t, err)
	require.Equal(t, []byte{0x83, 0x97, 0xe9, 0xf8}, input[:4])
	for _, height := range []int64{19, 20} {
		db := state.NewDBImpl(ctx.WithBlockHeight(height), k, false)
		result, err := served.Run(&vm.EVM{StateDB: db}, ethcommon.Address{}, address, input, nil, true, false, nil)
		if height == 19 {
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.ErrorIs(t, db.GetPrecompileError(), agentcommon.ErrNotActive)
			reason, decodeErr := abi.UnpackRevert(result)
			require.NoError(t, decodeErr)
			require.Equal(t, "LXP_AGENT_NOT_ACTIVE: "+precompiles.MergeUpgrade+" not active at height 19", reason)
		} else {
			require.NoError(t, err)
			values, decodeErr := a.Methods[clearing.ComputeClearingMethod].Outputs.Unpack(result)
			require.NoError(t, decodeErr)
			require.Len(t, values, 3)
			require.Equal(t, int16(0), values[0])
			require.Zero(t, big.NewInt(1000).Cmp(values[1].(*big.Int)))
			require.Zero(t, big.NewInt(0).Cmp(values[2].(*big.Int)))
		}
	}
}

type mergeClearingOrder struct {
	OffsetBps int16
	Size      *big.Int
}

func TestClearingMergeOrderCap(t *testing.T) {
	require.Equal(t, 256, clearing.MergeMaxOrders)
	testApp := app.Setup(t, false, true, false)
	const planName = "clearing-merge-cap"
	p, err := clearing.NewMergePrecompile(testApp.GetPrecompileKeepers(), planName)
	require.NoError(t, err)
	a := p.GetABI()
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(100)
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	testApp.UpgradeKeeper.SetDone(ctx, planName)
	for _, total := range []int{0, 1, clearing.MergeMaxOrders - 1, clearing.MergeMaxOrders, clearing.MergeMaxOrders + 1} {
		for _, buyCount := range []int{0, total / 2, total} {
			t.Run(fmt.Sprintf("total_%d_buys_%d", total, buyCount), func(t *testing.T) {
				buys := make([]mergeClearingOrder, buyCount)
				sells := make([]mergeClearingOrder, total-buyCount)
				for _, orders := range [][]mergeClearingOrder{buys, sells} {
					for i := range orders {
						orders[i] = mergeClearingOrder{Size: big.NewInt(1)}
					}
				}
				input, err := a.Pack(clearing.ComputeClearingMethod, big.NewInt(1000), buys, sells)
				require.NoError(t, err)
				require.Equal(t, []byte{0x83, 0x97, 0xe9, 0xf8}, input[:4])
				db := state.NewDBImpl(ctx, &testApp.EvmKeeper, false)
				result, err := p.Run(&vm.EVM{StateDB: db}, ethcommon.Address{}, p.Address(), input, nil, true, false, nil)
				if total > clearing.MergeMaxOrders {
					require.ErrorIs(t, err, vm.ErrExecutionReverted)
					require.ErrorIs(t, db.GetPrecompileError(), agentcommon.ErrBatchTooLarge)
					reason, decodeErr := abi.UnpackRevert(result)
					require.NoError(t, decodeErr)
					require.Equal(t, "LXP_AGENT_BATCH_TOO_LARGE: batch over the cap (cap set at activation rehearsal)", reason)
					return
				}
				require.NoError(t, err)
				values, err := a.Methods[clearing.ComputeClearingMethod].Outputs.Unpack(result)
				require.NoError(t, err)
				volume := min(len(buys), len(sells))
				require.Len(t, values, 3)
				require.Equal(t, int16(0), values[0])
				require.Zero(t, big.NewInt(1000).Cmp(values[1].(*big.Int)))
				require.Zero(t, big.NewInt(int64(volume)).Cmp(values[2].(*big.Int)))
			})
		}
	}
}
