package layerxprograms_test

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestProgramsPrecompileActiveAfterMergeHeight(t *testing.T) {
	h := newHarness(t)
	a := testkeeper.EVMTestApp
	ctx := h.stateDB.Ctx().WithBlockHeight(10)
	address := common.HexToAddress("0x000000000000000000000000000000000000101D")
	require.Equal(t, lp.Address, address)
	require.Equal(t, precompiles.MergeUpgrade, app.MergeUpgrade)
	keepers, ok := a.GetPrecompileKeepers().(lp.HostKeepers)
	require.True(t, ok, "node keepers must expose the LayerX host to the merge constructor")
	require.NotNil(t, a.LayerXHost)
	require.Same(t, a.LayerXHost, keepers.LayerXHost())
	prices := [7]uint64{11, 13, 17, 19, 23, 29, 31}
	asset := [32]byte{0xa5}
	record := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for _, price := range prices {
		record = binary.BigEndian.AppendUint64(record, price)
	}
	record = append(record, asset[:]...)
	for _, value := range [6]uint64{100, 1, 2, 9, 5, 50} {
		record = binary.BigEndian.AppendUint64(record, value)
	}
	for _, value := range []uint64{10, 10, 1} {
		record = binary.BigEndian.AppendUint64(record, value)
	}
	digest := [32]byte{0xd7}
	record = append(record, digest[:]...)
	record = append(record, make([]byte, 16)...)
	require.Len(t, record, 217)
	ctx.KVStore(a.GetKey(app.LayerXHostStoreKey)).Set(append([]byte{1, 0, 9}, []byte("progfee/active/v1")...), record)
	stored, err := keepers.LayerXHost().ReadFeeSchedule(ctx)
	require.NoError(t, err)
	require.Equal(t, record, stored)
	a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
	require.Equal(t, int64(10), a.UpgradeKeeper.GetDoneHeight(ctx, app.MergeUpgrade))
	input, err := lp.ProgramsABI.Pack("getCurrentFeeSchedule")
	require.NoError(t, err)
	for _, tracing := range []bool{false, true} {
		for _, height := range []int64{9, 10, 11} {
			t.Run(fmt.Sprintf("height=%d/tracing=%t", height, tracing), func(t *testing.T) {
				at := ctx.WithBlockHeight(height).WithIsTracing(tracing)
				served := a.EvmKeeper.CustomPrecompiles(at)
				versions := a.EvmKeeper.GetCustomPrecompilesVersions(at)
				if height < 10 {
					require.NotContains(t, served, address)
					require.NotContains(t, versions, address)
				} else {
					require.Contains(t, served, address)
					require.Equal(t, precompiles.MergeUpgrade, versions[address])
				}
				db := state.NewDBImpl(at, &a.EvmKeeper, false)
				require.Empty(t, db.GetCode(address), "the call must resolve through the node's precompile set")
				block, err := a.EvmKeeper.GetVMBlockContext(at, core.GasPool(math.MaxUint64))
				require.NoError(t, err)
				chain := evmtypes.DefaultChainConfig().EthereumConfig(a.EvmKeeper.ChainID(at))
				evm := vm.NewEVM(*block, db, chain, vm.Config{}, served)
				output, _, err := evm.StaticCall(h.caller, address, input, 2000000)
				require.NoError(t, err, "getCurrentFeeSchedule at height %d: output=%x, precompile error=%v", height, output, db.GetPrecompileError())
				if height < 10 {
					require.Empty(t, output, "an address with no precompile or bytecode returns no data before activation")
					return
				}
				values, err := lp.ProgramsABI.Methods["getCurrentFeeSchedule"].Outputs.Unpack(output)
				require.NoError(t, err)
				require.Len(t, values, 3)
				require.Equal(t, uint32(1), values[0])
				require.IsType(t, [7]uint64{}, values[1])
				require.IsType(t, [32]byte{}, values[2])
				require.Equal(t, prices, values[1])
				require.Equal(t, asset, values[2])
			})
		}
	}
}

func TestProgramsPrecompileListedInMergeSet(t *testing.T) {
	h := newHarness(t)
	a := testkeeper.EVMTestApp
	address := common.HexToAddress("0x000000000000000000000000000000000000101D")
	entries := 0
	for _, entry := range precompiles.MergeAddressTable {
		if entry.Address == address {
			entries++
			require.Equal(t, lp.PrecompileName, entry.Name)
		}
	}
	require.Equal(t, 1, entries, "0x101D must appear exactly once in the merge address set")
	for _, missing := range precompiles.MissingMergeConstructors() {
		require.NotEqual(t, address, missing.Address, "the node import must register the programs constructor")
	}
	keepers := a.GetPrecompileKeepers()
	wired, ok := keepers.(lp.HostKeepers)
	require.True(t, ok)
	require.NotNil(t, wired.LayerXHost())
	require.Same(t, a.LayerXHost, wired.LayerXHost())
	built, err := lp.NewMergePrecompile(keepers, precompiles.MergeUpgrade)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.Equal(t, address, built.Address())
	require.Equal(t, "layerxprograms", built.GetName())
	require.Equal(t, lp.ProgramsABI, built.GetABI())
	custom, err := precompiles.GetCustomPrecompilesE(app.LatestUpgrade, keepers)
	require.NoError(t, err)
	require.Contains(t, custom, address)
	require.Contains(t, custom[address], precompiles.MergeUpgrade)
	require.Equal(t, built, custom[address][precompiles.MergeUpgrade], "the registered constructor must build NewMergePrecompile with the real node keepers")
	ctx := h.stateDB.Ctx().WithBlockHeight(10)
	a.UpgradeKeeper.SetDone(ctx, precompiles.MergeUpgrade)
	served, ok := a.EvmKeeper.CustomPrecompiles(ctx)[address]
	require.True(t, ok, "the node must install the registered programs precompile")
	require.Equal(t, built, served)
}
