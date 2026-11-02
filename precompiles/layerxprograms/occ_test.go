package layerxprograms_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	rootmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/tasks"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/occ"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/utils/tracing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	ics23 "github.com/confio/ics23/go"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestProgramsCallsMatchSequentialUnderParallelExecution(t *testing.T) {
	h, args, _ := programsCallFixture(t)
	source := h.stateDB.Ctx()
	authority, did, err := lp.PrincipalOf(source, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	shared := args[0].([32]byte)
	record, err := h.host.ReadProgram(source, shared)
	require.NoError(t, err)
	iface, err := h.host.ReadProgramInterface(source, shared)
	require.NoError(t, err)
	wasm := bytes.Clone(source.KVStore(h.key).Get(append([]byte{5, 0, 9}, record.CodeHash[:]...)))
	require.NotEmpty(t, wasm)
	require.Equal(t, record.CodeHash, sha256.Sum256(wasm))
	programs := [8][32]byte{shared, {}, shared, {}, shared, {}, shared, {}}
	for i := 1; i < len(programs); i += 2 {
		programs[i] = [32]byte{byte(0x80 + i)}
		payload, err := lp.EncodeDeployPayload(programs[i], record.ABIVersion, record.Policy,
			record.Authority, record.CodeHash, iface.Encoding, wasm)
		require.NoError(t, err)
		sequence, err := h.host.ReadIdentitySequence(source, did)
		require.NoError(t, err)
		last, err := h.host.ReadLastSequence(source, did)
		require.NoError(t, err)
		receipt, err := lxphost.ExecuteActivity(lxphost.WithSequence(source, last+1), h.host,
			authority, 0x00090001, payload, sequence)
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
	}
	requests := make([]*sdk.DeliverTxEntry, len(programs))
	payloads := make([][]byte, len(programs))
	for i, program := range programs {
		callArgs := append([]interface{}(nil), args...)
		callArgs[0] = program
		input, err := lp.ProgramsABI.Pack("call", callArgs...)
		require.NoError(t, err)
		payloads[i], err = lp.EncodeCallPayload(program, args[1].(uint16), args[2].(string),
			args[3].([]byte), args[4].([]byte), args[5].([]byte), args[6].(uint32), args[7].([7]uint64))
		require.NoError(t, err)
		requests[i] = &sdk.DeliverTxEntry{AbsoluteIndex: i, Checksum: sha256.Sum256(input),
			Request: abci.RequestDeliverTxV2{Tx: input}}
	}
	method := lp.ProgramsABI.Methods["call"]
	type blockResult struct {
		responses []abci.ResponseDeliverTx
		receipts  [8]lxphost.Receipt
		logs      [8][]*ethtypes.Log
		hostHash  []byte
	}
	run := func(parallel bool) blockResult {
		sc := config.DefaultStateCommitConfig()
		sc.MemIAVLConfig.AsyncCommitBuffer = 0
		committed := rootmulti.NewStore(t.TempDir(), sc, config.StateStoreConfig{}, nil)
		t.Cleanup(func() { require.NoError(t, committed.Close()) })
		keys := append([]sdk.StoreKey(nil), source.MultiStore().StoreKeys()...)
		keys = append(keys, h.key)
		for _, key := range keys {
			kind := sdk.StoreTypeIAVL
			switch key.(type) {
			case *sdk.TransientStoreKey:
				kind = sdk.StoreTypeTransient
			case *sdk.MemoryStoreKey:
				kind = sdk.StoreTypeMemory
			}
			committed.MountStoreWithDB(key, kind, nil)
		}
		require.NoError(t, committed.LoadLatestVersion())
		seed := committed.CacheMultiStore()
		for _, key := range keys {
			iterator := source.KVStore(key).Iterator(nil, nil)
			for ; iterator.Valid(); iterator.Next() {
				seed.GetKVStore(key).Set(bytes.Clone(iterator.Key()), bytes.Clone(iterator.Value()))
			}
			require.NoError(t, iterator.Close())
		}
		seed.Write()
		committed.Commit(true)
		hostHash := func() []byte {
			key := append([]byte{1, 0, 9}, []byte("program\x00")...)
			key = append(key, shared[:]...)
			response := committed.Query(source.Context(), abci.RequestQuery{
				Path: "/" + h.key.Name() + "/key", Data: key, Prove: true,
			})
			require.Zero(t, response.Code, response.Log)
			require.NotEmpty(t, response.Value)
			require.NotNil(t, response.ProofOps)
			require.Len(t, response.ProofOps.Ops, 2)
			var proof ics23.CommitmentProof
			require.NoError(t, proof.Unmarshal(response.ProofOps.Ops[1].Data))
			existence := proof.GetExist()
			require.NotNil(t, existence)
			require.Equal(t, []byte(h.key.Name()), existence.Key)
			require.True(t, ics23.VerifyMembership(ics23.TendermintSpec, committed.LastCommitID().Hash,
				&proof, existence.Key, existence.Value))
			return bytes.Clone(existence.Value)
		}
		before := hostHash()
		require.Len(t, before, 32)
		blockStore := committed.CacheMultiStore()
		ctx := source.WithMultiStore(blockStore)
		require.Contains(t, ctx.MultiStore().StoreKeys(), h.key)
		require.Equal(t, h.dump(source), h.dump(ctx))
		var result blockResult
		var attempts [8]atomic.Int32
		var firstReads sync.WaitGroup
		firstReads.Add(len(programs))
		deliver := func(ctx sdk.Context, req abci.RequestDeliverTxV2, _ sdk.Tx, _ [32]byte) (response abci.ResponseDeliverTx) {
			defer func() {
				if recovered := recover(); recovered != nil {
					if _, ok := recovered.(occ.Abort); !ok {
						panic(recovered)
					}
					response = abci.ResponseDeliverTx{Code: 1, Log: "OCC abort"}
				}
			}()
			i := ctx.TxIndex()
			attempt := attempts[i].Add(1)
			ctx = ctx.WithEVMTxHash(common.Hash(sha256.Sum256(append([]byte{byte(i)}, req.Tx...)))).
				WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx)).WithEventManager(sdk.NewEventManager())
			if parallel && attempt == 1 {
				if _, err := h.host.ReadLastSequence(ctx, did); err != nil {
					panic(err)
				}
				firstReads.Done()
				firstReads.Wait()
			}
			failure := func(err error) abci.ResponseDeliverTx {
				return abci.ResponseDeliverTx{Code: 1, Log: err.Error()}
			}
			reference, _ := ctx.CacheContext()
			var feeLimit [16]byte
			args[8].(*big.Int).FillBytes(feeLimit[:])
			receipt, err := lp.Submit(reference, h.executor, lp.NewCallContext(h.caller, &method),
				0x00090003, payloads[i], feeLimit)
			if err != nil {
				return failure(err)
			}
			stateDB := state.NewDBImpl(ctx, h.executor.TestEVMKeeper(), false)
			block, err := h.executor.TestEVMKeeper().GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
			if err != nil {
				return failure(err)
			}
			chain := evmtypes.DefaultChainConfig().EthereumConfig(h.executor.TestEVMKeeper().ChainID(ctx))
			evm := vm.NewEVM(*block, stateDB, chain, vm.Config{}, map[common.Address]vm.PrecompiledContract{
				h.precompile.Address(): h.precompile,
			})
			const gas = uint64(20000000)
			output, remaining, err := evm.Call(h.caller, lp.Address, req.Tx, gas, uint256.NewInt(0))
			if err != nil {
				return failure(fmt.Errorf("call %d: %w (output %x)", i, err, output))
			}
			if receipt.ProgramOutcome == nil {
				return failure(fmt.Errorf("call %d has no program outcome", i))
			}
			outcome := receipt.ProgramOutcome
			expected, err := method.Outputs.Pack(outcome.Status, outcome.Kind, outcome.Response)
			if err != nil {
				return failure(err)
			}
			if !bytes.Equal(expected, output) || !reflect.DeepEqual(h.dump(reference), h.dump(stateDB.Ctx())) {
				return failure(fmt.Errorf("call %d differs from its kernel receipt", i))
			}
			if _, err := stateDB.Finalize(); err != nil {
				return failure(err)
			}
			result.receipts[i] = receipt
			result.logs[i] = stateDB.GetAllLogs()
			return abci.ResponseDeliverTx{Data: output, GasWanted: int64(gas), GasUsed: int64(gas - remaining)}
		}
		if parallel {
			scheduler := tasks.NewScheduler(len(programs), tracing.NewTracingInfo(nil, false), deliver)
			result.responses, err = scheduler.ProcessAll(ctx, requests)
			require.NoError(t, err)
			var total int32
			for i := range attempts {
				total += attempts[i].Load()
			}
			require.Greater(t, total, int32(len(programs)), "host store conflicts must cause OCC retries")
		} else {
			for i, request := range requests {
				result.responses = append(result.responses, deliver(ctx.WithTxIndex(i), request.Request, nil, request.Checksum))
			}
		}
		require.Len(t, result.responses, len(programs))
		for i, response := range result.responses {
			require.Zero(t, response.Code, "transaction %d: %s", i, response.Log)
			require.Positive(t, response.GasUsed)
			require.Zero(t, result.receipts[i].ResultCode)
			require.NotEmpty(t, result.receipts[i].Encoded)
			require.Zero(t, result.receipts[i].ProgramOutcome.Status)
			require.Equal(t, uint8(1), result.receipts[i].ProgramOutcome.Kind)
			require.Len(t, result.logs[i], 2)
			for j, name := range []string{"ProgramGuestEvent", "ProgramCallOutcome"} {
				require.Equal(t, lp.Address, result.logs[i][j].Address)
				require.Equal(t, []common.Hash{lp.ProgramsABI.Events[name].ID, common.Hash(programs[i])}, result.logs[i][j].Topics)
			}
			if i > 0 {
				require.Equal(t, result.receipts[i-1].GlobalSequence+1, result.receipts[i].GlobalSequence)
			}
		}
		blockStore.Write()
		committed.Commit(true)
		result.hostHash = hostHash()
		require.Len(t, result.hostHash, 32)
		require.NotEqual(t, before, result.hostHash)
		return result
	}
	sequential := run(false)
	parallel := run(true)
	require.Equal(t, sequential.responses, parallel.responses)
	require.Equal(t, sequential.receipts, parallel.receipts)
	require.Equal(t, sequential.logs, parallel.logs)
	require.Equal(t, sequential.hostHash, parallel.hostHash)
}
