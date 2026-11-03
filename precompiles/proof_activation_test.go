// Proves: MergeAddressTable/NewMergePrecompile and upgrade.Keeper.SetDone -> GetCustomPrecompilesE[MergeUpgrade] and the upgrade KV store -> evmkeeper.Keeper.CustomPrecompiles, vm.EVM.StaticCall and ABI.Unpack.
//
// All eight agent producers are registered by the node's production imports;
// the test never supplies replacement constructors or an activation map. The
// application commits its upgrade marker and XWeb registrations, stops, and
// reopens before consumption. Fresh EVMs and freshly built constructor results
// are consumed twice; moving the real marker proves there is no activation cache.
// Malformed calldata and truncated producer output are refused by the ABI codecs.
// Repairs setup.go/InitializePrecompiles omitting merge entries from VM maps,
// and setup_test.go/MergeAddressTable omitting the chains address. Also makes
// teeattestor.go/NewMergePrecompile diagnose a nil XWeb keeper at wiring time.
// The anchor is stateless: only application producer state is persisted here.
package precompiles_test

import (
	"context"
	"encoding/json"
	"maps"
	"math"
	"math/big"
	"slices"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	xwebtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/testutil"
	xwebtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	agentoracle "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/oracle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/teeattestor"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// HexToAddress checks the inventory's full-width spelling before the lenient
// geth conversion, so a malformed configuration address cannot be truncated.
func HexToAddress(t *testing.T, text string) common.Address {
	t.Helper()
	require.Len(t, text, 42)
	require.True(t, common.IsHexAddress(text), text)
	return common.HexToAddress(text)
}

func proofActivationGlobals(t *testing.T) {
	t.Helper()
	initialized, info := precompiles.Initialized, precompiles.PrecompileNamesToInfo
	precompiles.Initialized = false
	precompiles.PrecompileNamesToInfo = map[string]precompiles.PrecompileInfo{}
	t.Cleanup(func() { precompiles.Initialized, precompiles.PrecompileNamesToInfo = initialized, info })
	for _, contracts := range []*vm.PrecompiledContracts{
		&vm.PrecompiledContractsHomestead, &vm.PrecompiledContractsByzantium,
		&vm.PrecompiledContractsIstanbul, &vm.PrecompiledContractsBerlin,
		&vm.PrecompiledContractsCancun, &vm.PrecompiledContractsBLS,
	} {
		before := *contracts
		*contracts = maps.Clone(before)
		t.Cleanup(func() { *contracts = before })
	}
	for _, addresses := range []*[]common.Address{
		&vm.PrecompiledAddressesHomestead, &vm.PrecompiledAddressesByzantium,
		&vm.PrecompiledAddressesIstanbul, &vm.PrecompiledAddressesBerlin, &vm.PrecompiledAddressesCancun,
	} {
		before := *addresses
		*addresses = slices.Clone(before)
		t.Cleanup(func() { *addresses = before })
	}
}

func proofActivationEVM(t *testing.T, a *app.App, ctx sdk.Context) (*vm.EVM, *state.DBImpl) {
	t.Helper()
	db := state.NewDBImpl(ctx, &a.EvmKeeper, false)
	block, err := a.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(a.EvmKeeper.ChainID(ctx))
	return vm.NewEVM(*block, db, chain, vm.Config{}, a.EvmKeeper.CustomPrecompiles(ctx)), db
}

func TestProofActivation(t *testing.T) {
	proofActivationGlobals(t)
	home, dir := t.TempDir(), t.TempDir()
	open := func() *app.App {
		database, err := dbm.NewGoLevelDB("application", dir)
		require.NoError(t, err, "node.New: activation producer database")
		return app.New(database, nil, true, map[int64]bool{}, home, 1, true, nil, app.MakeEncodingConfig(),
			wasm.EnableAllProposals, app.TestAppOpts{}, app.EmptyWasmOpts, nil)
	}
	at := time.Unix(1_800_000_000, 0).UTC()
	producer := open()
	genesis, err := json.Marshal(app.NewDefaultGenesisState(app.MakeEncodingConfig().Marshaler))
	require.NoError(t, err)
	_, err = producer.InitChain(context.Background(), &abci.RequestInitChain{
		Time: at, ConsensusParams: app.DefaultConsensusParams, ChainId: "pax-test", AppStateBytes: genesis,
	})
	require.NoError(t, err)
	_, err = producer.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: "pax-test", Height: 1, Time: at},
	})
	require.NoError(t, err)
	ctx := producer.GetContextForDeliverTx(nil).WithBlockHeight(1).WithBlockTime(at)
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	require.Equal(t, app.MergeUpgrade, precompiles.MergeUpgrade)
	require.Equal(t, pcommon.MergeUpgrade, precompiles.MergeUpgrade)
	producer.UpgradeKeeper.SetDone(ctx, precompiles.MergeUpgrade)
	keys := xwebtestutil.Attestors(5)
	for _, key := range keys {
		require.NoError(t, producer.XWebKeeper.RegisterAttestor(ctx, xwebtypes.MsgRegisterAttestor{
			Authority: xwebtypes.DefaultAuthority(), Attestor: key.Registration(),
		}))
	}
	wantRoots := producer.XWebKeeper.GetAttestorSet(ctx)
	require.Len(t, wantRoots.Attestors, 5)
	producer.SetDeliverStateToCommit()
	_, err = producer.Commit(context.Background())
	require.NoError(t, err)
	require.NoError(t, producer.Close())
	producer = nil

	consumer := open()
	t.Cleanup(func() { require.NoError(t, consumer.Close()) })
	require.EqualValues(t, 1, consumer.LastBlockHeight())
	ctx, err = consumer.CreateQueryContext(0, false)
	require.NoError(t, err)
	ctx = ctx.WithBlockHeight(1).WithBlockTime(at)
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	require.EqualValues(t, 1, consumer.UpgradeKeeper.GetDoneHeight(ctx, precompiles.MergeUpgrade))
	require.Equal(t, wantRoots, consumer.XWebKeeper.GetAttestorSet(ctx))
	require.Empty(t, precompiles.MissingMergeConstructors(), "node production imports must host every merge producer")

	order := []struct {
		OffsetBps int16
		Size      *big.Int
	}{}
	domain, structHash := [32]byte{1}, [32]byte{2}
	digest := crypto.Keccak256Hash([]byte{0x19, 0x01}, domain[:], structHash[:])
	rows := []struct {
		address, name, method string
		args, want            []interface{}
	}{
		{"0x0000000000000000000000000000000000000901", "orob", "resolveOffset", []interface{}{big.NewInt(10000), int16(100)}, []interface{}{big.NewInt(10100)}},
		{"0x0000000000000000000000000000000000000902", "clearing", "computeClearing", []interface{}{big.NewInt(1000), order, order}, []interface{}{int16(0), big.NewInt(1000), big.NewInt(0)}},
		{"0x0000000000000000000000000000000000000903", "oracle aggregator", "aggregate", []interface{}{[]agentoracle.Feed{{Price: big.NewInt(123), Confidence: big.NewInt(5), Timestamp: big.NewInt(at.Unix())}}}, []interface{}{big.NewInt(123), big.NewInt(5)}},
		{"0x0000000000000000000000000000000000000904", "pofq", "scoreFill", []interface{}{big.NewInt(1000), big.NewInt(1000)}, []interface{}{new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)}},
		{"0x0000000000000000000000000000000000000905", "scheduler", "pending", []interface{}{common.Address{}}, []interface{}{[]*big.Int{}}},
		{"0x0000000000000000000000000000000000000906", "streams", "accrued", []interface{}{big.NewInt(1)}, []interface{}{big.NewInt(0)}},
		{"0x0000000000000000000000000000000000000907", "tee attestor", "rootCount", []interface{}{uint8(0)}, []interface{}{big.NewInt(5)}},
		{"0x0000000000000000000000000000000000000908", "eip712", "hashTypedData", []interface{}{domain, structHash}, []interface{}{[32]byte(digest)}},
	}
	var previous map[common.Address]precompiles.IPrecompile
	for round := 0; round < 2; round++ {
		built, err := precompiles.GetCustomPrecompilesE(app.LatestUpgrade, consumer.GetPrecompileKeepers())
		require.NoError(t, err)
		consumer.EvmKeeper.SetCustomPrecompiles(built, app.LatestUpgrade)
		current := map[common.Address]precompiles.IPrecompile{}
		// Clear metadata so a second observation cannot use a previous registry entry.
		precompiles.Initialized = false
		precompiles.PrecompileNamesToInfo = map[string]precompiles.PrecompileInfo{}
		require.NoError(t, precompiles.InitializePrecompiles(false, consumer.GetPrecompileKeepers()))
		for _, row := range rows {
			t.Run(row.name+big.NewInt(int64(round)).String(), func(t *testing.T) {
				address := HexToAddress(t, row.address)
				contract, ok := built[address][precompiles.MergeUpgrade].(precompiles.IPrecompile)
				require.True(t, ok, "missing real producer: precompiles/layerxagent/%s", row.name)
				require.Equal(t, address, contract.Address())
				require.Equal(t, row.name, contract.GetName())
				require.Same(t, contract, built[address][app.LatestUpgrade])
				if round > 0 {
					require.NotSame(t, previous[address], contract)
				}
				current[address] = contract
				info, err := precompiles.GetPrecompileInfoE(row.name)
				require.NoError(t, err)
				require.Equal(t, address, info.Address)
				for _, table := range []vm.PrecompiledContracts{
					vm.PrecompiledContractsHomestead, vm.PrecompiledContractsByzantium,
					vm.PrecompiledContractsIstanbul, vm.PrecompiledContractsBerlin,
					vm.PrecompiledContractsCancun, vm.PrecompiledContractsBLS,
				} {
					named, ok := table[address].(precompiles.IPrecompile)
					require.True(t, ok, "InitializePrecompiles must publish %s to the VM", row.name)
					require.Equal(t, contract.GetName(), named.GetName())
				}
				input, err := info.ABI.Pack(row.method, row.args...)
				require.NoError(t, err)
				method := info.ABI.Methods[row.method]
				for _, tracing := range []bool{false, true} {
					active := ctx.WithIsTracing(tracing)
					require.Same(t, contract, consumer.EvmKeeper.CustomPrecompiles(active)[address])
					evm, _ := proofActivationEVM(t, consumer, active)
					output, _, err := evm.StaticCall(common.Address{}, address, input, 2_000_000)
					require.NoError(t, err)
					observed, err := method.Outputs.Unpack(output)
					require.NoError(t, err)
					wantOutput, err := method.Outputs.Pack(row.want...)
					require.NoError(t, err)
					require.Equal(t, wantOutput, output, "exact canonical ABI result")
					require.Len(t, observed, len(row.want))
					for i, expected := range row.want {
						if integer, ok := expected.(*big.Int); ok {
							actual, ok := observed[i].(*big.Int)
							require.True(t, ok)
							require.Zero(t, integer.Cmp(actual), "integer output %d", i)
						} else {
							require.Equal(t, expected, observed[i])
						}
					}
					require.NotEmpty(t, output)
					_, err = method.Outputs.Unpack(output[:len(output)-1])
					require.Error(t, err, "consumer must refuse a malformed producer result")
					evm, db := proofActivationEVM(t, consumer, active)
					output, _, err = evm.StaticCall(common.Address{}, address, input[:4], 2_000_000)
					require.ErrorIs(t, err, vm.ErrExecutionReverted)
					_, err = abi.UnpackRevert(output)
					require.NoError(t, err)
					require.ErrorIs(t, db.GetPrecompileError(), agentcommon.ErrABIDecode)
					// Querying a historical height must refuse the same registered producer.
					require.NotContains(t, consumer.EvmKeeper.CustomPrecompiles(active.WithBlockHeight(0)), address)
				}
			})
		}
		previous = current
	}
	// Even with the same keeper and context, changing the producer's real KV
	// marker immediately changes both ordinary execution and tracing selection.
	changed, _ := ctx.CacheContext()
	consumer.UpgradeKeeper.SetDone(changed.WithBlockHeight(2), precompiles.MergeUpgrade)
	for _, tracing := range []bool{false, true} {
		for _, row := range rows {
			address := HexToAddress(t, row.address)
			require.NotContains(t, consumer.EvmKeeper.CustomPrecompiles(changed.WithIsTracing(tracing)), address)
			require.Contains(t, consumer.EvmKeeper.CustomPrecompiles(changed.WithBlockHeight(2).WithIsTracing(tracing)), address)
		}
	}
	// Real incomplete keeper bundles fail before they can publish a TEE producer.
	unavailable, err := teeattestor.NewMergePrecompile(&app.PrecompileKeepers{EVMKeeper: &consumer.EvmKeeper}, precompiles.MergeUpgrade)
	require.Nil(t, unavailable)
	require.ErrorIs(t, err, agentcommon.ErrNoContext)
	require.EqualError(t, err, "LXP_PRECOMPILE_NO_CONTEXT: xweb keeper is unavailable")
	_, err = precompiles.GetPrecompileInfoE("unknown activation producer")
	require.Error(t, err)
}
