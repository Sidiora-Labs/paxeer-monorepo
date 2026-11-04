package evmrpc

import (
	"context"
	"encoding/json"
	"math/big"
	"sync"
	"testing"
	"time"

	evmstate "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	traceLogger "github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/stretchr/testify/require"
)

func TestShouldUseProfiledBlockTrace(t *testing.T) {
	t.Parallel()

	defaultTracer := ""
	callTracer := "callTracer"

	tests := []struct {
		name     string
		enabled  bool
		config   *tracers.TraceConfig
		expected bool
	}{
		{
			name:     "disabled by default with nil config",
			enabled:  false,
			config:   nil,
			expected: false,
		},
		{
			name:    "disabled by default with default tracer",
			enabled: false,
			config: &tracers.TraceConfig{
				Tracer: &defaultTracer,
			},
			expected: false,
		},
		{
			name:     "enabled with nil config",
			enabled:  true,
			config:   nil,
			expected: true,
		},
		{
			name:    "enabled with default tracer",
			enabled: true,
			config: &tracers.TraceConfig{
				Tracer: &defaultTracer,
			},
			expected: true,
		},
		{
			name:    "explicit tracer keeps legacy path",
			enabled: true,
			config: &tracers.TraceConfig{
				Tracer: &callTracer,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			api := &DebugAPI{profiledBlockTrace: tt.enabled}
			require.Equal(t, tt.expected, api.shouldUseProfiledBlockTrace(tt.config))
		})
	}
}

// AssertProfiledDefaultExecution is shared with the repair gate's real committed
// app fixture. It exercises public routing and decodes execution, rather than
// allowing the predicate-only table to stand as proof of a supported tracer.
func AssertProfiledDefaultExecution(t *testing.T, api *DebugAPI, height rpc.BlockNumber, blockHash common.Hash, hashes []common.Hash) {
	t.Helper()
	empty, call, invalid := "", "callTracer", "not valid tracer source !"
	configs := []*tracers.TraceConfig{nil, {}, {Tracer: &empty}}
	var expected json.RawMessage
	for _, config := range configs {
		require.True(t, api.shouldUseProfiledBlockTrace(config))
		byNumber, err := api.TraceBlockByNumber(context.Background(), height, config)
		require.NoError(t, err)
		raw, err := json.Marshal(byNumber)
		require.NoError(t, err)
		if expected == nil {
			expected = raw
		} else {
			require.JSONEq(t, string(expected), string(raw))
		}
		byHash, err := api.TraceBlockByHash(context.Background(), blockHash, config)
		require.NoError(t, err)
		hashRaw, err := json.Marshal(byHash)
		require.NoError(t, err)
		require.JSONEq(t, string(raw), string(hashRaw))
		var entries []struct {
			TxHash common.Hash
			Result json.RawMessage
			Error  string
		}
		require.NoError(t, json.Unmarshal(raw, &entries))
		require.Len(t, entries, len(hashes))
		for i, entry := range entries {
			require.Equal(t, hashes[i], entry.TxHash)
			require.Empty(t, entry.Error)
			assertProfiledStructResult(t, entry.Result)
			profiled, err := api.TraceTransactionProfile(context.Background(), entry.TxHash, config)
			require.NoError(t, err)
			response := profiled.(TraceTransactionProfileResponse)
			txRaw, err := json.Marshal(response.Trace)
			require.NoError(t, err)
			require.JSONEq(t, string(entry.Result), string(txRaw))
		}
	}
	require.False(t, api.shouldUseProfiledBlockTrace(&tracers.TraceConfig{Tracer: &call}))
	named, err := api.TraceBlockByNumber(context.Background(), height, &tracers.TraceConfig{Tracer: &call})
	require.NoError(t, err)
	raw, err := json.Marshal(named)
	require.NoError(t, err)
	var calls []struct {
		Result struct{ Type string }
		Error  string
	}
	require.NoError(t, json.Unmarshal(raw, &calls))
	require.Len(t, calls, len(hashes))
	for _, entry := range calls {
		require.Empty(t, entry.Error)
		require.Equal(t, "CALL", entry.Result.Type)
	}
	require.False(t, api.shouldUseProfiledBlockTrace(&tracers.TraceConfig{Tracer: &invalid}))
	invalidBlock, err := api.TraceBlockByNumber(context.Background(), height, &tracers.TraceConfig{Tracer: &invalid})
	require.NoError(t, err)
	raw, err = json.Marshal(invalidBlock)
	require.NoError(t, err)
	var failures []struct {
		TxHash common.Hash
		Result json.RawMessage
		Error  string
	}
	require.NoError(t, json.Unmarshal(raw, &failures))
	require.Len(t, failures, len(hashes))
	for i, entry := range failures {
		require.Equal(t, hashes[i], entry.TxHash)
		require.Empty(t, entry.Result)
		require.NotEmpty(t, entry.Error)
	}
	_, err = api.TraceTransactionProfile(context.Background(), hashes[0], &tracers.TraceConfig{Tracer: &invalid})
	require.Error(t, err)
}

func assertProfiledStructResult(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var result struct {
		Gas         uint64
		Failed      bool
		ReturnValue string
		StructLogs  []struct{ Op string }
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	require.GreaterOrEqual(t, result.Gas, uint64(21000))
	require.False(t, result.Failed)
	require.NotEmpty(t, result.StructLogs)
	var ops []string
	for _, step := range result.StructLogs {
		ops = append(ops, step.Op)
	}
	require.Contains(t, ops, "LOG1")
	require.Equal(t, "STOP", ops[len(ops)-1])
}

// AssertProfiledParentCancellation observes real struct-logger opcode hooks to
// ensure cancellation happens inside the running EVM. All hooks/results/Stop
// delegate to the production logger; there is no simulated execution or tracer.
func AssertProfiledParentCancellation(t *testing.T, api *DebugAPI) {
	t.Helper()
	for _, cause := range []string{"parent_cancel", "deadline"} {
		t.Run(cause, func(t *testing.T) {
			started := make(chan struct{})
			var once sync.Once
			name := "repairM1292ObservedStructLogger"
			tracers.DefaultDirectory.Register(name, func(_ *tracers.Context, _ json.RawMessage, _ *params.ChainConfig) (*tracers.Tracer, error) {
				actual := traceLogger.NewStructLogger(&traceLogger.Config{Limit: 4, DisableStack: true, DisableStorage: true})
				hooks := actual.Hooks()
				onOpcode := hooks.OnOpcode
				hooks.OnOpcode = func(pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, returnData []byte, depth int, err error) {
					onOpcode(pc, op, gas, cost, scope, returnData, depth, err)
					once.Do(func() { close(started) })
				}
				return &tracers.Tracer{Hooks: hooks, GetResult: actual.GetResult, Stop: actual.Stop}, nil
			}, false)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			// Use the same admission path as a live debug request, with one slot.
			traceCtx, release, err := api.prepareTraceContext(ctx)
			require.NoError(t, err)
			sdkCtx, _ := api.ctxProvider(LatestCtxHeight).CacheContext()
			to := common.HexToAddress("0x1292cafe")
			api.keeper.SetCode(sdkCtx, to, common.FromHex("0x5b600056")) // JUMPDEST PUSH1 0 JUMP
			api.keeper.SetNonce(sdkCtx, to, 1)
			state := evmstate.NewDBImpl(sdkCtx, api.keeper, true)
			from := common.HexToAddress("0x1292beef")
			tx := ethtypes.NewTx(&ethtypes.LegacyTx{To: &to, Gas: 1_000_000_000_000, GasPrice: big.NewInt(0), Value: big.NewInt(0)})
			msg := &core.Message{From: from, To: &to, GasLimit: tx.Gas(), GasPrice: big.NewInt(0), GasFeeCap: big.NewInt(0), GasTipCap: big.NewInt(0), Value: big.NewInt(0)}
			blockCtx, err := api.keeper.GetVMBlockContext(sdkCtx, api.keeper.GetGasPool())
			require.NoError(t, err)
			timeout := "5s"
			if cause == "deadline" {
				timeout = "100ms"
			}
			result := make(chan error, 1)
			go func() {
				_, err := api.profiledTraceTx(traceCtx, tx, msg, &tracers.Context{TxHash: tx.Hash(), BlockNumber: blockCtx.BlockNumber}, *blockCtx, state, &tracers.TraceConfig{Tracer: &name, Timeout: &timeout}, nil, false, nil)
				release()
				result <- err
			}()
			select {
			case <-started:
			case err := <-result:
				t.Fatalf("trace ended before executing EVM loop: %v", err)
			case <-time.After(3 * time.Second):
				t.Fatal("EVM did not start")
			}
			_, err = api.acquireTraceSemaphore(context.Background())
			require.ErrorIs(t, err, errTraceConcurrencyLimit)
			begin := time.Now()
			if cause == "parent_cancel" {
				cancel()
			}
			select {
			case err := <-result:
				if cause == "parent_cancel" {
					require.ErrorIs(t, err, context.Canceled)
				} else {
					require.EqualError(t, err, "execution timeout")
				}
			case <-time.After(time.Second):
				cancel()
				t.Fatal("canceled live EVM trace did not stop promptly")
			}
			require.Less(t, time.Since(begin), time.Second)
			// Joining the watcher and releasing admission must leave the next request usable.
			next, done, err := api.prepareTraceContext(context.Background())
			require.NoError(t, err)
			require.NoError(t, next.Err())
			done()
			require.Empty(t, api.traceCallSemaphore)
		})
	}
}

// AssertLegacyHTTPFilterCount observes the real registered producer's state so
// the transport repair can prove that suppressed notifications did not execute.
func AssertLegacyHTTPFilterCount(t *testing.T, server *HTTPServer, want int) {
	t.Helper()
	found := false
	for _, api := range server.apis {
		if service, ok := api.Service.(*PaxFilterAPI); ok {
			found = true
			service.f.filtersMu.RLock()
			count := len(service.f.filters)
			service.f.filtersMu.RUnlock()
			require.Equal(t, want, count)
		}
	}
	require.True(t, found, "production pax filter service must be registered")
}
