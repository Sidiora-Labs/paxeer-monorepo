package app

import (
	"context"
	"math/big"
	"testing"
	"time"

	evmcfg "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/benchmark"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestBenchmarkHelperMethods(t *testing.T) {
	app := &App{}

	// Test helper methods with nil manager (should not panic)
	app.RecordBenchmarkCommitTime(100 * time.Millisecond)
	app.StartBenchmarkBlockProcessing()
	app.EndBenchmarkBlockProcessing()

	// BenchmarkLogger should return nil when manager is nil
	require.Nil(t, app.BenchmarkLogger())

	// Create a benchmark manager with logger
	benchLogger := benchmark.NewLogger()
	app.benchmarkManager = &benchmark.Manager{
		Logger: benchLogger,
	}

	// Now helper methods should work (just verify they don't panic)
	app.RecordBenchmarkCommitTime(100 * time.Millisecond)
	app.StartBenchmarkBlockProcessing()
	app.EndBenchmarkBlockProcessing()

	// BenchmarkLogger should return the logger
	require.NotNil(t, app.BenchmarkLogger())
	require.Equal(t, benchLogger, app.BenchmarkLogger())
}

func TestInitBenchmark_PanicsOnLiveChainID(t *testing.T) {

	chainID := "hyperpax_125-1"
	liveEVMChainID := int64(125)
	require.Equal(t, liveEVMChainID, evmcfg.GetEVMChainID(chainID).Int64())
	require.True(t, evmcfg.IsLiveEVMChainID(liveEVMChainID))

	// Create a minimal App struct
	app := &App{
		encodingConfig: MakeEncodingConfig(),
	}

	ctx := context.Background()

	// Test that InitBenchmark panics with live chain ID
	require.PanicsWithValue(t, "benchmark not allowed on live chains", func() {
		app.InitBenchmark(ctx, chainID, liveEVMChainID)
	}, "InitBenchmark should panic on live chain ID")

	// Verify nothing was initialized
	require.Nil(t, app.benchmarkManager, "benchmarkManager should not be initialized on panic")
}

func TestInitBenchmark_AllLiveChainIDs(t *testing.T) {

	require.NotEmpty(t, evmcfg.EVMChainIDMapping)
	for evmChainID, chainID := range evmcfg.EVMChainIDMapping {
		t.Run(chainID, func(t *testing.T) {
			app := &App{
				encodingConfig: MakeEncodingConfig(),
			}
			ctx := context.Background()

			require.PanicsWithValue(t, "benchmark not allowed on live chains", func() {
				app.InitBenchmark(ctx, chainID, evmChainID)
			}, "InitBenchmark should panic on live chain ID: %s", chainID)
			require.Nil(t, app.benchmarkManager)
		})
	}
}

func TestInitBenchmark_Success(t *testing.T) {

	chainID := "test-chain"
	evmChainID := int64(12345) // Non-live chain ID

	// Create a minimal App struct with required fields
	app := &App{
		encodingConfig: MakeEncodingConfig(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test InitBenchmark with non-live chain ID
	app.InitBenchmark(ctx, chainID, evmChainID)

	// Verify benchmarkManager is set
	require.NotNil(t, app.benchmarkManager, "benchmarkManager should be initialized")
	require.NotNil(t, app.benchmarkManager.Logger, "benchmarkManager.Logger should be set")
	require.NotNil(t, app.benchmarkManager.Generator, "benchmarkManager.Generator should be set")

	// Verify we can get the proposal channel
	require.NotNil(t, app.benchmarkManager.ProposalChannel(), "proposal channel should be available")

	for range benchmark.WarmupBlocks {
		app.RecordBenchmarkCommitTime(time.Millisecond)
	}
	select {
	case proposal, ok := <-app.benchmarkManager.ProposalChannel():
		require.True(t, ok, "proposal channel closed before producing transactions")
		require.NotEmpty(t, proposal)
		for _, wire := range proposal {
			tx, err := app.GetTxConfig().TxDecoder()(wire)
			require.NoError(t, err)
			require.Len(t, tx.GetMsgs(), 1)
			msg, ok := tx.GetMsgs()[0].(*evmtypes.MsgEVMTransaction)
			require.True(t, ok)
			require.NoError(t, msg.ValidateBasic())
			eth, data := msg.AsTransaction()
			require.NotNil(t, eth)
			require.NotNil(t, data)
			require.Equal(t, big.NewInt(evmChainID), eth.ChainId())
			require.NotNil(t, eth.To())
			require.Empty(t, eth.Data())
			require.Positive(t, eth.Gas())
			_, err = types.Sender(types.LatestSignerForChainID(eth.ChainId()), eth)
			require.NoError(t, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("benchmark did not produce a proposal")
	}
	cancel()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-app.benchmarkManager.ProposalChannel():
			if !ok {
				return
			}
		case <-timer.C:
			t.Fatal("benchmark generator did not stop after cancellation")
		}
	}
}
