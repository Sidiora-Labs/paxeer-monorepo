// Proves: scheduler.PrecompileExecutor.schedule/cancel/reschedule/getJob/pending and NewMergePrecompile -> vm.EVM.Call/StaticCall with state.DBImpl in the application harness -> common.MethodGas, useSurface.receiptEvents and wallet.decodeModuleEvent.
//
// The five ABI methods, SchedulerAddress, LayerXAgentKeepers.LayerXAgentK and
// NewMergePrecompile cross real application keepers, escrow and EVM dispatch.
// A committed PaxDB store is closed and reopened before fresh keeper, precompile
// and EVM instances read the jobs. Repeated reads and subsequent mutations prove
// that neither the Go nor wallet consumers use a retained producer result.
// Node executes the consumer functions from their checked-in TypeScript source;
// no RPC, EVM, keeper, event encoder or event decoder is replaced by a double.
//
// Repairs scheduler/merge.go: NewMergePrecompile must refuse a typed-nil
// *node.PrecompileKeepers with ErrNoContext rather than dereference it.
// Repairs apps/wallet/src/surfaces/useSurface.ts: receiptEvents must refuse a
// malformed log envelope before address filtering or module event decoding.
//
// Outside scope, modules/layerxagent/keeper/scheduler.go: Keeper.runJob emits
// Cosmos EventJobExecuted/EventJobFailed at end block, not EVM receipt logs.
// The proof asserts that current behavior: wallet receiptEvents must not invent
// JobExecuted/JobFailed receipts, even though the merge ABI declares those events.
// The inventory's useSurface pending(address) reference is the gas station's
// pending lookup, not a scheduler reader; receiptEvents is the real wallet sink.
package scheduler_test

import (
	"bytes"
	"encoding/json"
	"math"
	"math/big"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	agentkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	agenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/scheduler"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	storev2 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type proofSchedulerKeepers struct {
	utils.Keepers
	agent *agentkeeper.Keeper
}

func (k proofSchedulerKeepers) LayerXAgentK() *agentkeeper.Keeper { return k.agent }

var _ scheduler.LayerXAgentKeepers = proofSchedulerKeepers{}

// proofSchedulerStore persists the application harness's real stores, including
// bank/wei balances, upgrade activation, EVM code and the scheduler indexes.
// Seeding happens before any producer call; reopening never copies job state.
type proofSchedulerStore struct {
	h      *schedulerHarness
	dir    string
	keys   []sdk.StoreKey
	agent  sdk.StoreKey
	stores sdk.CommitMultiStore
	ctx    sdk.Context
}

func newProofSchedulerStore(t *testing.T) *proofSchedulerStore {
	h := newSchedulerHarness(t)
	source := h.stateDB.Ctx()
	funded := new(big.Int).Set(h.balance(h.creator))
	s := &proofSchedulerStore{h: h, dir: t.TempDir(), keys: source.MultiStore().StoreKeys(), ctx: source}
	for _, key := range s.keys {
		if key.Name() == agentkeeper.StoreKey {
			s.agent = key
		}
	}
	require.NotNil(t, s.agent)
	s.open()
	for _, key := range s.keys {
		from, to := source.KVStore(key), s.stores.GetKVStore(key)
		it := from.Iterator(nil, nil)
		for ; it.Valid(); it.Next() {
			to.Set(bytes.Clone(it.Key()), bytes.Clone(it.Value()))
		}
		require.NoError(t, it.Close())
	}
	// PaxDB commitment.Store stages Set calls until Commit; publish the seeded
	// application state before opening the first EVM cache over that store.
	s.stores.Commit(true)
	s.bind()
	require.Equal(t, funded, h.balance(h.creator))
	t.Cleanup(func() { require.NoError(t, s.stores.Close()) })
	return s
}

func (s *proofSchedulerStore) open() {
	s.h.t.Helper()
	cfg := config.DefaultStateCommitConfig()
	cfg.AsyncCommitBuffer = 0
	cfg.MemIAVLConfig.AsyncCommitBuffer = 0
	cfg.MemIAVLConfig.SnapshotInterval = 1
	cfg.MemIAVLConfig.SnapshotMinTimeInterval = 0
	s.stores = storev2.NewStore(s.dir, cfg, config.StateStoreConfig{}, nil)
	for _, key := range s.keys {
		kind := sdk.StoreTypeIAVL
		switch key.(type) {
		case *sdk.TransientStoreKey:
			kind = sdk.StoreTypeTransient
		case *sdk.MemoryStoreKey:
			kind = sdk.StoreTypeMemory
		}
		s.stores.MountStoreWithDB(key, kind, nil)
	}
	require.NoError(s.h.t, s.stores.LoadLatestVersion())
}

func (s *proofSchedulerStore) bind() {
	h, application := s.h, testkeeper.EVMTestApp
	ctx := s.ctx.WithMultiStore(s.stores.CacheMultiStore()).WithEventManager(sdk.NewEventManager())
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	h.keeper = agentkeeper.NewKeeper(s.agent, &application.EvmKeeper, schedulerPlan)
	keepers := proofSchedulerKeepers{app.NewPrecompileKeepers(application), &h.keeper}
	require.Same(h.t, &h.keeper, keepers.LayerXAgentK())
	built, err := scheduler.NewMergePrecompile(keepers, schedulerPlan)
	require.NoError(h.t, err)
	var ok bool
	h.precompile, ok = built.(*scheduler.SchedulerPrecompile)
	require.True(h.t, ok)
	require.Equal(h.t, common.HexToAddress(scheduler.SchedulerAddress), built.Address())
	require.Equal(h.t, scheduler.PrecompileName, built.GetName())
	h.stateDB = state.NewDBImpl(ctx, &application.EvmKeeper, false)
	block, err := application.EvmKeeper.GetVMBlockContext(h.stateDB.Ctx(), core.GasPool(math.MaxUint64))
	require.NoError(h.t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(application.EvmKeeper.ChainID(h.stateDB.Ctx()))
	h.evm = vm.NewEVM(*block, h.stateDB, chain, vm.Config{}, map[common.Address]vm.PrecompiledContract{built.Address(): built})
	s.ctx = ctx
}

func (s *proofSchedulerStore) reopen() {
	t := s.h.t
	t.Helper()
	_, err := s.h.stateDB.Finalize()
	require.NoError(t, err)
	s.ctx.MultiStore().(sdk.CacheMultiStore).Write()
	committed := s.stores.Commit(true)
	require.NotEmpty(t, committed.Hash)
	require.NoError(t, s.stores.Close())
	s.h.evm, s.h.stateDB, s.h.precompile = nil, nil, nil
	s.open()
	require.Equal(t, committed, s.stores.LastCommitID())
	s.bind()
}

// These are the wallet's real receipt filtering and generic ABI decoder, loaded
// without the React hook/module initialization. Node's type erasure leaves the
// checked-in function bodies intact. The SDK exchange module supplies its real
// abiEventTopic/Keccak implementation, using its declared npm dependency.
const proofSchedulerWalletConsumer = `
const fs = require('node:fs');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { stripTypeScriptTypes } = require('node:module');
(async () => {
  const root = process.argv[1];
  const sdkPath = 'services/agent/sdk/typescript/src/exchange.ts';
  const sdk = await import(pathToFileURL(path.join(root, sdkPath))).catch(error => {
    assert.fail('Cannot load the real wallet consumer dependency at ' + sdkPath +
      '; install its locked dependencies with npm ci --prefix services/agent/sdk/typescript: ' + error.message);
  });
  function sourceSlice(file, start, end) {
    const source = fs.readFileSync(path.join(root, file), 'utf8');
    const first = source.indexOf(start), last = source.indexOf(end, first);
    assert(first >= 0 && last > first, 'consumer source boundaries must exist');
    return stripTypeScriptTypes(source.slice(first, last)).replace(/^export /gm, '');
  }
  const moduleCode = sourceSlice('sdk/wallet/src/modules/index.ts', 'export type ModuleProvider', "export * from './exchange.js'");
  const surfaceCode = sourceSlice('apps/wallet/src/surfaces/useSurface.ts', 'function isRecord(', 'export function receiptStatus(');
  const { receiptEvents, decodeModuleEvent } = new Function('abiEventTopic', moduleCode + '\n' + surfaceCode + '\nreturn { receiptEvents, decodeModuleEvent };')(sdk.abiEventTopic);
  const input = JSON.parse(fs.readFileSync(0, 'utf8'));
  const spec = (name, inputs) => ({ name, inputs: inputs.map(([name, type, indexed = false]) => ({ name, type, indexed })) });
  const specs = [
    spec('JobScheduled', [['jobId','uint256',true],['creator','address',true],['target','address'],['executeAtBlock','uint64'],['gasLimit','uint64'],['deposit','uint256']]),
    spec('JobCancelled', [['jobId','uint256',true],['creator','address',true],['refund','uint256']]),
    spec('JobRescheduled', [['jobId','uint256',true],['newBlock','uint64']]),
    spec('JobExecuted', [['jobId','uint256',true],['creator','address',true],['gasUsed','uint64'],['refund','uint256']]),
    spec('JobFailed', [['jobId','uint256',true],['creator','address',true],['reason','string']]),
  ];
  let decodes = 0;
  const surface = { address: input.address, decodeEvent(log) { ++decodes; return decodeModuleEvent(specs, input.address, log); } };
  const normalise = value => JSON.parse(JSON.stringify(value, (_, v) => typeof v === 'bigint' ? v.toString() : v));
  for (let read = 0; read < 2; ++read) {
    const result = receiptEvents({ logs: input.logs }, surface).map(({event, fields}) => ({event, fields}));
    assert.deepEqual(normalise(result), input.expected);
    assert.equal(decodes, input.logs.length * (read + 1), 'every read must invoke the decoder again');
  }
  assert.throws(() => receiptEvents({}, surface), /no logs/);
  for (const log of input.logs) {
    const badData = { ...log, data: log.data.slice(0, -2) };
    assert.throws(() => receiptEvents({logs:[badData]}, surface), e => e.code === 'malformed_log');
    const badTopic = { ...log, topics: log.topics.slice(0, -1) };
    assert.throws(() => receiptEvents({logs:[badTopic]}, surface), e => e.code === 'malformed_log');
    const nonCanonical = { ...log, topics: log.topics.map((topic, i) => i === 1 ? topic.slice(0, -1) + 'g' : topic) };
    assert.throws(() => receiptEvents({logs:[nonCanonical]}, surface), /invalid log/);
    assert.throws(() => receiptEvents({logs:[{...log, address:null}]}, surface), /invalid log/);
  }
  process.stdout.write('wallet consumer asserted two reads and malformed-result refusals\n');
})().catch(error => { console.error(error); process.exitCode = 1; });
`

type proofSchedulerLog struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Data    string   `json:"data"`
}

type proofSchedulerEvent struct {
	Event  string            `json:"event"`
	Fields map[string]string `json:"fields"`
}

func proofSchedulerWallet(t *testing.T, h *schedulerHarness, expected []proofSchedulerEvent) {
	t.Helper()
	logs := make([]proofSchedulerLog, 0)
	for _, entry := range h.stateDB.GetAllLogs() {
		topics := make([]string, len(entry.Topics))
		for i, topic := range entry.Topics {
			topics[i] = topic.Hex()
		}
		logs = append(logs, proofSchedulerLog{strings.ToLower(entry.Address.Hex()), topics, "0x" + common.Bytes2Hex(entry.Data)})
	}
	require.Len(t, logs, len(expected))
	payload, err := json.Marshal(struct {
		Address  string                `json:"address"`
		Logs     []proofSchedulerLog   `json:"logs"`
		Expected []proofSchedulerEvent `json:"expected"`
	}{strings.ToLower(h.precompile.Address().Hex()), logs, expected})
	require.NoError(t, err)
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	cmd := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", proofSchedulerWalletConsumer, root)
	cmd.Stdin = bytes.NewReader(payload)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	require.Contains(t, string(output), "wallet consumer asserted two reads")
}

func proofSchedulerRead(t *testing.T, h *schedulerHarness, name string, args ...interface{}) []interface{} {
	t.Helper()
	input := h.input(name, args...)
	before := len(h.stateDB.GetAllLogs())
	ret, remaining, err := h.evm.StaticCall(h.other, h.precompile.Address(), input, callGas)
	require.NoError(t, err)
	require.NoError(t, h.stateDB.Err())
	require.Equal(t, callGas-agentcommon.MethodGas(input[:4], len(input)), remaining)
	require.Len(t, h.stateDB.GetAllLogs(), before)
	a := h.precompile.GetABI()
	decoded, err := a.Methods[name].Outputs.Unpack(ret)
	require.NoError(t, err)
	_, err = a.Methods[name].Outputs.Unpack(ret[:31])
	require.Error(t, err, "the actual ABI consumer must refuse a truncated producer result")
	return decoded
}

func TestProofScheduler0x0905ProducerTransportConsumer(t *testing.T) {
	s := newProofSchedulerStore(t)
	h := s.h
	a := h.precompile.GetABI()
	for _, row := range []struct {
		name, signature, selector string
		base, perByte             uint64
		args                      []interface{}
	}{
		{"schedule", "schedule(address,bytes,uint64,uint64)", "849ba0a3", 25_000, 30, []interface{}{h.target, []byte{1}, uint64(schedulerHeight + 10), schedulerGas}},
		{"cancel", "cancel(uint256)", "40e58ee5", 20_000, 30, []interface{}{big.NewInt(1)}},
		{"reschedule", "reschedule(uint256,uint64)", "08e6f770", 20_000, 30, []interface{}{big.NewInt(1), uint64(schedulerHeight + 20)}},
		{"getJob", "getJob(uint256)", "bf22c457", 1_000, 3, []interface{}{big.NewInt(1)}},
		{"pending", "pending(address)", "5eebea20", 1_000, 3, []interface{}{h.creator}},
	} {
		input := h.input(row.name, row.args...)
		require.Equal(t, row.signature, a.Methods[row.name].Sig)
		require.Equal(t, common.FromHex(row.selector), crypto.Keccak256([]byte(row.signature))[:4])
		require.Equal(t, common.FromHex(row.selector), input[:4])
		gas := row.base + row.perByte*uint64(len(input))
		require.Equal(t, gas, agentcommon.MethodGas(input[:4], len(input)))
		require.Equal(t, gas, h.precompile.RequiredGas(input))
		require.Equal(t, uint64(math.MaxUint64), agentcommon.MethodGas(input[:4], -1))
	}
	for name, signature := range map[string]string{
		"JobScheduled":   "JobScheduled(uint256,address,address,uint64,uint64,uint256)",
		"JobCancelled":   "JobCancelled(uint256,address,uint256)",
		"JobRescheduled": "JobRescheduled(uint256,uint64)",
		"JobExecuted":    "JobExecuted(uint256,address,uint64,uint256)",
		"JobFailed":      "JobFailed(uint256,address,string)",
	} {
		event, found := a.Events[name]
		require.True(t, found)
		require.Equal(t, signature, event.Sig)
		require.Equal(t, crypto.Keccak256Hash([]byte(signature)), event.ID)
	}
	require.Empty(t, precompiles.MissingMergeConstructors())
	before := new(big.Int).Set(h.balance(h.creator))
	one := h.schedule()
	two := h.schedule()
	require.Equal(t, big.NewInt(1), one)
	require.Equal(t, big.NewInt(2), two)
	scheduled := func(id string) proofSchedulerEvent {
		return proofSchedulerEvent{"JobScheduled", map[string]string{
			"jobId": id, "creator": strings.ToLower(h.creator.Hex()), "target": strings.ToLower(h.target.Hex()),
			"executeAtBlock": "110", "gasLimit": "100000", "deposit": "200000",
		}}
	}
	proofSchedulerWallet(t, h, []proofSchedulerEvent{scheduled("1"), scheduled("2")})
	require.Zero(t, h.balance(h.precompile.Address()).Sign())
	require.Equal(t, big.NewInt(400_000), h.escrow())
	s.reopen()
	require.Equal(t, big.NewInt(400_000), h.escrow())
	for read := 0; read < 2; read++ {
		for _, id := range []*big.Int{one, two} {
			job := proofSchedulerRead(t, h, "getJob", id)
			require.Equal(t, []interface{}{id, h.creator, h.target, []byte{1}, uint64(110), schedulerGas, big.NewInt(200_000), true}, job)
		}
		require.Equal(t, []*big.Int{one, two}, proofSchedulerRead(t, h, "pending", h.creator)[0])
	}
	for _, name := range []string{"cancel", "reschedule"} {
		args := []interface{}{one}
		if name == "reschedule" {
			args = append(args, uint64(120))
		}
		ret, err := h.call(h.other, new(big.Int), name, args...)
		requireRevert(t, ret, err, string(agentcommon.ErrSchedulerUnauthorized))
		_, decodeErr := abi.UnpackRevert(ret[:35])
		require.Error(t, decodeErr)
		require.Empty(t, h.stateDB.GetAllLogs())
		require.Equal(t, big.NewInt(400_000), h.escrow())
	}
	h.write(new(big.Int), "reschedule", one, uint64(120))
	h.write(new(big.Int), "cancel", two)
	proofSchedulerWallet(t, h, []proofSchedulerEvent{
		{"JobRescheduled", map[string]string{"jobId": "1", "newBlock": "120"}},
		{"JobCancelled", map[string]string{"jobId": "2", "creator": strings.ToLower(h.creator.Hex()), "refund": "200000"}},
	})
	s.reopen()
	for read := 0; read < 2; read++ {
		require.Equal(t, uint64(120), proofSchedulerRead(t, h, "getJob", one)[4])
		require.Equal(t, []*big.Int{one}, proofSchedulerRead(t, h, "pending", h.creator)[0])
		absent := proofSchedulerRead(t, h, "getJob", two)
		// ABI uint256 words allocate their backing storage even for zero; retain
		// exact field equality with that representation rather than a nil-backed Int.
		zeroWord := make([]byte, 32)
		require.Equal(t, []interface{}{new(big.Int).SetBytes(zeroWord), common.Address{}, common.Address{}, []byte{}, uint64(0), uint64(0), new(big.Int).SetBytes(zeroWord), false}, absent)
	}
	ret, err := h.call(h.creator, new(big.Int), "cancel", two)
	requireRevert(t, ret, err, string(agentcommon.ErrSchedulerJobNotFound))
	input := h.input("reschedule", one, uint64(130))
	ret, _, err = h.evm.Call(h.creator, h.precompile.Address(), input[:len(input)-1], callGas, new(uint256.Int))
	requireRevert(t, ret, err, string(agentcommon.ErrABIDecode))
	require.Empty(t, h.stateDB.GetAllLogs())
	h.write(new(big.Int), "cancel", one)
	proofSchedulerWallet(t, h, []proofSchedulerEvent{{"JobCancelled", map[string]string{
		"jobId": "1", "creator": strings.ToLower(h.creator.Hex()), "refund": "200000",
	}}})
	s.reopen()
	require.Zero(t, h.escrow().Sign())
	require.Equal(t, before, h.balance(h.creator))
	for read := 0; read < 2; read++ {
		require.Empty(t, proofSchedulerRead(t, h, "pending", h.creator)[0])
	}
}

func TestProofScheduler0x0905MergeRefusesMissingKeepers(t *testing.T) {
	var typedNil *app.PrecompileKeepers
	for _, keepers := range []utils.Keepers{nil, typedNil, &app.PrecompileKeepers{}} {
		p, err := scheduler.NewMergePrecompile(keepers, schedulerPlan)
		require.Nil(t, p)
		require.ErrorIs(t, err, agentcommon.ErrNoContext)
	}
}

func TestProofScheduler0x0905EndBlockOutcomeBoundary(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "executed", true: "failed"}[failure], func(t *testing.T) {
			s := newProofSchedulerStore(t)
			h := s.h
			code := []byte{byte(vm.PUSH1), 0x2a, byte(vm.PUSH1), 0, byte(vm.SSTORE), byte(vm.STOP)}
			if failure {
				code = append(code[:len(code)-1], byte(vm.PUSH1), 0, byte(vm.PUSH1), 0, byte(vm.REVERT))
			}
			testkeeper.EVMTestApp.EvmKeeper.SetCode(h.stateDB.Ctx(), h.target, code)
			id := h.write(big.NewInt(200_000), "schedule", h.target, []byte{}, uint64(110), schedulerGas)[0].(*big.Int)
			s.reopen()
			end := h.stateDB.Ctx().WithBlockHeight(110).WithEventManager(sdk.NewEventManager())
			used, err := h.keeper.RunDue(end)
			require.NoError(t, err)
			require.Greater(t, used, uint64(21_000))
			require.Less(t, used, schedulerGas)
			outcomes := 0
			for _, event := range end.EventManager().Events() {
				if !strings.Contains(event.Type, "EventJobExecuted") && !strings.Contains(event.Type, "EventJobFailed") {
					continue
				}
				outcomes++
				decoded, err := sdk.ParseTypedEvent(abci.Event(event))
				require.NoError(t, err)
				if failure {
					require.Equal(t, &agenttypes.EventJobFailed{JobId: id.Uint64(), Creator: h.creator.Hex(), Reason: "execution reverted"}, decoded)
				} else {
					require.Equal(t, &agenttypes.EventJobExecuted{JobId: id.Uint64(), Creator: h.creator.Hex(), GasUsed: used, Refund: new(big.Int).Sub(big.NewInt(200_000), new(big.Int).SetUint64(used)).String()}, decoded)
				}
			}
			require.Equal(t, 1, outcomes)
			expected := common.Hash{}
			if !failure {
				expected = common.BigToHash(big.NewInt(42))
			}
			require.Equal(t, expected, testkeeper.EVMTestApp.EvmKeeper.GetState(end, h.target, common.Hash{}))
			_, found := h.keeper.GetJob(end, id.Uint64())
			require.False(t, found)
			require.Empty(t, h.keeper.Pending(end, h.creator))
			require.Zero(t, h.escrow().Sign())
			require.Empty(t, h.stateDB.GetAllLogs())
			proofSchedulerWallet(t, h, []proofSchedulerEvent{})
			// Flush this real end-block cached context before the next reopen.
			s.reopen()
			for read := 0; read < 2; read++ {
				require.Empty(t, proofSchedulerRead(t, h, "pending", h.creator)[0])
				require.Equal(t, false, proofSchedulerRead(t, h, "getJob", id)[7])
			}
			require.Equal(t, expected, testkeeper.EVMTestApp.EvmKeeper.GetState(h.stateDB.Ctx(), h.target, common.Hash{}))
		})
	}
}
