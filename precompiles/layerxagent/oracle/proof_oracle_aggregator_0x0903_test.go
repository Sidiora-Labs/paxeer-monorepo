// Proves: oracle.Feed/Aggregate and lxphost.Host.Execute -> oracle.NewMergePrecompile, common.Precompile and vm.EVM.Call/StaticCall -> agentcommon.MethodGas and abi.ABI.Unpack in the importing application consumer.
//
// All six inventory surfaces are exercised: OracleAddress/PrecompileName,
// Aggregate, NewMergePrecompile, aggregate, submitPrice and getValidatorPrice.
// The application keepers, native host, signed replay, ABI and EVM dispatcher
// are real. The existing fixture supplies genesis market configuration only;
// every observed price and receipt is produced by the signed host execution.
// The anchor owns no store. We close and reopen the host and reconstruct the
// precompile before reading its committed result twice, then submit a new price
// and observe it twice. Gas tracing proves each call executes the transport.
// Truncated real return data is refused by the same ABI consumer; truncated
// committed producer state is refused by the native decoder through the EVM.
// Repairs merge.go NewMergePrecompile: the exported mutable MergeMaxConfidence
// previously let importing code change activation policy. Constructors now use
// the immutable rehearsal cap. oracle.go declares each ABI handler explicitly;
// common/gas.go keeps the oracle selectors and their read/write rates together.
package oracle_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	exchangekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/oracle"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// Embedding the concrete application bundle preserves the optional keeper
// interfaces needed by the production registry, including LayerXPayKeepers.
// Only the exchange store is supplied by the real keeper mounted by fixture.
type proofOracleKeepers struct {
	*app.PrecompileKeepers
	exchange *exchangekeeper.Keeper
}

func (k proofOracleKeepers) LayerXExchangeK() *exchangekeeper.Keeper { return k.exchange }

type proofOracleHarness struct {
	f          *oracleFixture
	keepers    proofOracleKeepers
	p          precompiles.IPrecompile
	dispatches int
}

func newProofOracleHarness(t *testing.T) *proofOracleHarness {
	t.Helper()
	f := fixture(t)
	application := testkeeper.EVMTestApp
	application.UpgradeKeeper.SetDone(f.ctx, precompiles.MergeUpgrade)
	h := &proofOracleHarness{f: f, keepers: proofOracleKeepers{application.GetPrecompileKeepers().(*app.PrecompileKeepers), f.exchange}}
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
	application := testkeeper.EVMTestApp
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
	t.Run("signed submission host reopen and committed reads", func(t *testing.T) {
		price := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 80), big.NewInt(57))
		ctx, activity := f.observation(t, 0, price)
		validatorHash := sha256.Sum256(f.public[0][:])
		validator := sdk.ValAddress(validatorHash[:20])
		h.refuse(t, ctx, f.caller[0], oracle.GetValidatorPriceMethod, true, agentcommon.ErrOracleNoFeeds, f.feed)
		h.refuse(t, ctx, f.caller[1], oracle.SubmitPriceMethod, false, agentcommon.ErrOracleNotSigner, f.feed, price, big.NewInt(5))
		deniedCtx, _ := f.observation(t, 1, price)
		h.refuse(t, deniedCtx, f.caller[1], oracle.SubmitPriceMethod, false, agentcommon.ErrOracleNotSigner, f.feed, price, big.NewInt(5))
		_, found, err := f.exchange.CommittedOraclePrice(ctx, validator, f.feed)
		require.NoError(t, err)
		require.False(t, found, "refused submissions must not produce an observation")
		h.refuse(t, ctx, f.caller[0], oracle.SubmitPriceMethod, true, agentcommon.ErrReadOnly, f.feed, price, big.NewInt(5))
		input := h.input(t, oracle.SubmitPriceMethod, f.feed, price, big.NewInt(5))
		require.Len(t, input, 100)
		require.Equal(t, uint64(33000), h.p.RequiredGas(input))
		output, err, cause := h.cross(t, ctx, f.caller[0], input, false)
		require.NoError(t, err)
		require.NoError(t, cause)
		require.Empty(t, output)
		_, err = h.p.GetABI().Unpack(oracle.SubmitPriceMethod, output)
		require.NoError(t, err)
		var recorded struct {
			Validator  sdk.ValAddress
			Confidence uint64
			Activity   lxphost.Activity
			Receipt    lxphost.Receipt
		}
		require.NoError(t, json.Unmarshal(ctx.KVStore(f.exchangeKey).Get(append([]byte("oracle/activity/"), activity.IdempotencyKey[:]...)), &recorded))
		require.Equal(t, validator, recorded.Validator)
		require.Equal(t, uint64(5), recorded.Confidence)
		require.Equal(t, activity, recorded.Activity)
		require.Zero(t, recorded.Receipt.ResultCode)
		require.NotEmpty(t, recorded.Receipt.Encoded)

		f.host.Close()
		f.host, err = lxphost.NewHost(lxphost.Config{StoreKey: f.hostKey, NetworkID: 7, ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
		require.NoError(t, err)
		t.Cleanup(f.host.Close)
		previous := h.p
		h.p, err = oracle.NewMergePrecompile(h.keepers, precompiles.MergeUpgrade)
		require.NoError(t, err)
		require.NotSame(t, previous, h.p)
		ctx = oracle.WithOracleReplay(ctx, f.host, f.hostKey, validator, lxphost.Activity{}, f.authority[0])
		committed, found, err := f.exchange.CommittedOraclePrice(ctx, validator, f.feed)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, price, committed.Price.BigInt())
		require.Equal(t, uint64(1), committed.ObservationSequence)
		require.NotEqual(t, [32]byte{}, committed.SourceSetDigest)
		want := []interface{}{price, big.NewInt(1), big.NewInt(ctx.BlockTime().Unix())}
		h.observe(t, ctx, oracle.GetValidatorPriceMethod, want, f.feed)
		require.Equal(t, uint64(1108), h.p.RequiredGas(h.input(t, oracle.GetValidatorPriceMethod, f.feed)))
		h.refuse(t, ctx, f.caller[0], oracle.GetValidatorPriceMethod, true, agentcommon.ErrOracleNoFeeds, [32]byte{0xff})

		// A new signed producer result must replace the first result after reopen.
		nextPrice := new(big.Int).Add(price, big.NewInt(7))
		next := proofOracleNextObservation(t, f, nextPrice)
		nextCtx := oracle.WithOracleReplay(lxphost.WithSequence(f.ctx, 4), f.host, f.hostKey, validator, next, f.authority[0])
		output, err, cause = h.cross(t, nextCtx, f.caller[0], h.input(t, oracle.SubmitPriceMethod, f.feed, nextPrice, big.NewInt(6)), false)
		require.NoError(t, err, "%s", output)
		require.NoError(t, cause)
		h.observe(t, nextCtx, oracle.GetValidatorPriceMethod, []interface{}{nextPrice, big.NewInt(1), big.NewInt(nextCtx.BlockTime().Unix())}, f.feed)

		// Refuse malformed real committed state at the native consumer decoder.
		key := append([]byte{1, 0, 6, 'o', 'r', 'a', 'c', 'l', 'e', ':'}, f.feed[:]...)
		produced := bytes.Clone(nextCtx.KVStore(f.hostKey).Get(key))
		require.NotEmpty(t, produced)
		nextCtx.KVStore(f.hostKey).Set(key, produced[:len(produced)-1])
		output, err, cause = h.cross(t, nextCtx, f.caller[0], h.input(t, oracle.GetValidatorPriceMethod, f.feed), true)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.ErrorIs(t, cause, lxphost.Code(-3))
		require.Contains(t, revertText(t, output), "LXP_ERR_NON_CANONICAL")
	})
}

func proofOracleNextObservation(t *testing.T, f *oracleFixture, price *big.Int) lxphost.Activity {
	t.Helper()
	payload := append([]byte(nil), f.feed[:]...)
	payload = binary.BigEndian.AppendUint64(payload, 2)
	amount := make([]byte, 16)
	price.FillBytes(amount)
	payload = append(payload, amount...)
	payload = binary.BigEndian.AppendUint64(payload, uint64(f.ctx.BlockTime().UnixMilli()))
	payload = binary.BigEndian.AppendUint64(payload, 1)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, append([]byte("LXP:ORACLE:OBSERVATION:v1"), payload...))
	require.NoError(t, err)
	transport := append([]byte{1}, payload...)
	transport = append(transport, ed25519.Sign(f.private[0], digest[:])...)
	sequence, err := f.host.ReadIdentitySequence(f.ctx, f.did[0])
	require.NoError(t, err)
	return f.sign(t, 0, 0x00060003, sequence, transport)
}
