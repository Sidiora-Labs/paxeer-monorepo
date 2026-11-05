// Proves the registered node oracle through EVM dispatch, signed native intake,
// committed application recovery and ABI consumption.
package oracle_test

import (
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/oracle"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type proofOracleHarness struct {
	f          *oracleFixture
	keepers    *app.PrecompileKeepers
	node       *repairM1425Node
	p          precompiles.IPrecompile
	dispatches int
}

func newProofOracleHarness(t *testing.T) *proofOracleHarness {
	t.Helper()
	node := newRepairM1425Node(t)
	f := &oracleFixture{ctx: node.ctx, caller: [2]common.Address{node.signer.EvmAddress, node.other.EvmAddress}}
	h := &proofOracleHarness{f: f, keepers: node.a.GetPrecompileKeepers().(*app.PrecompileKeepers), node: node}
	// Use the production constructor registry, including its merge version alias.
	set, err := precompiles.GetCustomPrecompilesE(app.LatestUpgrade, h.keepers)
	require.NoError(t, err)
	address := common.HexToAddress(oracle.OracleAddress)
	require.Contains(t, set, address)
	var ok bool
	h.p, ok = set[address][precompiles.MergeUpgrade].(precompiles.IPrecompile)
	require.True(t, ok, "the registered oracle must expose the importing Go surface")
	require.Same(t, h.p, set[address][app.LatestUpgrade])
	require.Equal(t, "0x0000000000000000000000000000000000000903", h.p.Address().Hex())
	require.Equal(t, "oracle aggregator", h.p.GetName())
	require.Equal(t, oracle.PrecompileName, h.p.GetName())
	for name, want := range map[string]string{
		oracle.AggregateMethod: "81097a95", oracle.SubmitPriceMethod: "dc9c2875", oracle.GetValidatorPriceMethod: "99812615",
	} {
		require.Equal(t, want, hex.EncodeToString(h.p.GetABI().Methods[name].ID))
	}
	return h
}

func (h *proofOracleHarness) input(t *testing.T, name string, args ...interface{}) []byte {
	t.Helper()
	input, err := h.p.GetABI().Pack(name, args...)
	require.NoError(t, err)
	return input
}

func (h *proofOracleHarness) cross(t *testing.T, ctx sdk.Context, caller common.Address, input []byte, static bool) ([]byte, error, error) {
	t.Helper()
	application := h.node.a
	db := state.NewDBImpl(ctx, &application.EvmKeeper, false)
	block, err := application.EvmKeeper.GetVMBlockContext(db.Ctx(), core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(application.EvmKeeper.ChainID(ctx))
	charged := uint64(0)
	hooks := &tracing.Hooks{OnGasChange: func(before, after uint64, reason tracing.GasChangeReason) {
		if reason == tracing.GasChangeCallPrecompiledContract {
			h.dispatches++
			charged = before - after
		}
	}}
	evm := vm.NewEVM(*block, db, chain, vm.Config{Tracer: hooks}, map[common.Address]vm.PrecompiledContract{h.p.Address(): h.p})
	before := h.dispatches
	const supplied = uint64(2_000_000)
	var output []byte
	var left uint64
	if static {
		output, left, err = evm.StaticCall(caller, h.p.Address(), input, supplied)
	} else {
		output, left, err = evm.Call(caller, h.p.Address(), input, supplied, uint256.NewInt(0))
	}
	require.Equal(t, before+1, h.dispatches, "the real dispatcher must execute every observation")
	require.Equal(t, h.p.RequiredGas(input), charged)
	require.Equal(t, agentcommon.MethodGas(input[:4], len(input)), charged)
	require.Equal(t, charged, supplied-left, "charge the full producer calldata, including the selector")
	require.NoError(t, db.Err())
	require.Empty(t, db.GetAllLogs())
	cause := db.GetPrecompileError()
	if err == nil && !static {
		_, finalizeErr := db.Finalize()
		require.NoError(t, finalizeErr)
	}
	return output, err, cause
}

func (h *proofOracleHarness) observe(t *testing.T, ctx sdk.Context, name string, want []interface{}, args ...interface{}) []byte {
	t.Helper()
	var output []byte
	for call := 0; call < 2; call++ {
		var err, cause error
		output, err, cause = h.cross(t, ctx, common.Address{19: 99}, h.input(t, name, args...), true)
		require.NoError(t, err)
		require.NoError(t, cause)
		// This is the actual consumer for the inventory's Go-only surface.
		values, decodeErr := h.p.GetABI().Unpack(name, output)
		require.NoError(t, decodeErr)
		require.Equal(t, want, values)
	}
	// Damage bytes emitted by the real producer at the consumer decode boundary.
	require.NotEmpty(t, output)
	_, err := h.p.GetABI().Unpack(name, output[:len(output)-1])
	require.Error(t, err)
	return output
}

func (h *proofOracleHarness) refuse(t *testing.T, ctx sdk.Context, caller common.Address, name string, static bool, code agentcommon.ErrorCode, args ...interface{}) {
	t.Helper()
	output, err, cause := h.cross(t, ctx, caller, h.input(t, name, args...), static)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.ErrorIs(t, cause, code)
	require.Contains(t, revertText(t, output), code.Error()+": ")
}

func TestProofOracleAggregator0x0903(t *testing.T) {
	h := newProofOracleHarness(t)
	f := h.f
	t.Run("constructor policy cannot be changed through exported big integer", func(t *testing.T) {
		original := oracle.MergeMaxConfidence
		// Replacing the exported pointer must not replace constructor policy.
		oracle.MergeMaxConfidence = big.NewInt(1)
		defer func() { oracle.MergeMaxConfidence = original }()
		p, err := oracle.NewMergePrecompile(h.keepers, precompiles.MergeUpgrade)
		require.NoError(t, err)
		registered := h.p
		h.p = p
		defer func() { h.p = registered }()
		h.observe(t, f.ctx, oracle.AggregateMethod, []interface{}{big.NewInt(44), big.NewInt(2)}, []oracle.Feed{feed(44, 2)})
		over := new(big.Int).Add(big.NewInt(1_000_000_000_000_000_000), big.NewInt(1))
		h.refuse(t, f.ctx, f.caller[0], oracle.AggregateMethod, true, agentcommon.ErrOracleConfidence,
			[]oracle.Feed{{Price: big.NewInt(44), Confidence: over, Timestamp: big.NewInt(1)}})
	})
	t.Run("exported aggregate and wire consumer", func(t *testing.T) {
		for _, row := range []struct {
			name              string
			feeds             []oracle.Feed
			price, confidence int64
		}{
			{"single", []oracle.Feed{feed(44, 7)}, 44, 7},
			{"weighted", []oracle.Feed{feed(100, 1), feed(30, 6), feed(10, 2)}, 30, 1},
			{"zero confidence", []oracle.Feed{feed(10, 0), feed(70, 9)}, 70, 9},
		} {
			t.Run(row.name, func(t *testing.T) {
				before, err := json.Marshal(row.feeds)
				require.NoError(t, err)
				price, confidence, err := oracle.Aggregate(row.feeds, oracle.MergeMaxFeeds)
				require.NoError(t, err)
				require.Equal(t, big.NewInt(row.price), price)
				require.Equal(t, big.NewInt(row.confidence), confidence)
				h.observe(t, f.ctx, oracle.AggregateMethod, []interface{}{price, confidence}, row.feeds)
				price.SetInt64(999)
				confidence.SetInt64(999)
				after, err := json.Marshal(row.feeds)
				require.NoError(t, err)
				require.Equal(t, before, after, "neither Go nor EVM calls may mutate the producer's feeds")
			})
		}
		feeds := make([]oracle.Feed, oracle.MergeMaxFeeds)
		for i := range feeds {
			feeds[i] = feed(55, 1_000_000_000_000_000_000)
		}
		input := h.input(t, oracle.AggregateMethod, feeds)
		require.Len(t, input, 4+64+32*96)
		require.Equal(t, uint64(10420), h.p.RequiredGas(input))
		h.observe(t, f.ctx, oracle.AggregateMethod, []interface{}{big.NewInt(55), big.NewInt(1_000_000_000_000_000_000)}, feeds)
		h.refuse(t, f.ctx, f.caller[0], oracle.AggregateMethod, true, agentcommon.ErrBatchTooLarge, append(feeds, feed(55, 1)))
		h.refuse(t, f.ctx, f.caller[0], oracle.AggregateMethod, true, agentcommon.ErrOracleNoFeeds, []oracle.Feed{})
		h.refuse(t, f.ctx, f.caller[0], oracle.AggregateMethod, true, agentcommon.ErrOraclePrice, []oracle.Feed{feed(-1, 1)})
		malformed := h.input(t, oracle.AggregateMethod, []oracle.Feed{feed(1, 1)})
		output, err, cause := h.cross(t, f.ctx, f.caller[0], malformed[:len(malformed)-1], true)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.ErrorIs(t, cause, agentcommon.ErrABIDecode)
		require.Contains(t, revertText(t, output), agentcommon.ErrABIDecode.Error())
	})
	t.Run("signed submission application reopen and committed reads", repairM1425RecoveryProof)
}
