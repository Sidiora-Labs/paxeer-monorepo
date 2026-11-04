package sender

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/generator"
	"github.com/paxeer-network/pax-load/generator/scenarios"
	"github.com/paxeer-network/pax-load/stats"
	"github.com/paxeer-network/pax-load/types"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"golang.org/x/time/rate"
)

// Committed loopback peer: the production RPC codec transports signed bytes,
// canonical receipts and full Ethereum headers, with explicit node outcomes.
type RepairM821Node struct {
	mu          sync.Mutex
	submitted   []common.Hash
	receipts    map[common.Hash]*ethtypes.Receipt
	reject      bool
	receiptGate <-chan struct{}
	active      int
	maxActive   int
	feeds       chan chan *ethtypes.Header
}

func (n *RepairM821Node) SendRawTransaction(raw hexutil.Bytes) (common.Hash, error) {
	var tx ethtypes.Transaction
	if err := tx.UnmarshalBinary(raw); err != nil {
		return common.Hash{}, err
	}
	if _, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(big.NewInt(125)), &tx); err != nil {
		return common.Hash{}, err
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.reject {
		return common.Hash{}, &TransactionRPCError{Code: -32000, Message: "nonce too low"}
	}
	n.submitted = append(n.submitted, tx.Hash())
	return tx.Hash(), nil
}

func (n *RepairM821Node) GetTransactionReceipt(ctx context.Context, hash common.Hash) (*ethtypes.Receipt, error) {
	n.mu.Lock()
	n.active++
	n.maxActive = max(n.maxActive, n.active)
	receipt := n.receipts[hash]
	n.mu.Unlock()
	defer func() { n.mu.Lock(); n.active--; n.mu.Unlock() }()
	if n.receiptGate != nil {
		select {
		case <-n.receiptGate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return receipt, nil
}

func (n *RepairM821Node) NewHeads(ctx context.Context) (*gethrpc.Subscription, error) {
	notifier, ok := gethrpc.NotifierFromContext(ctx)
	if !ok {
		return nil, errors.New("notifications unavailable")
	}
	sub := notifier.CreateSubscription()
	feed := make(chan *ethtypes.Header)
	go func() {
		select {
		case n.feeds <- feed:
		case <-sub.Err():
			return
		}
		for {
			select {
			case header := <-feed:
				if err := notifier.Notify(sub.ID, header); err != nil {
					return
				}
			case <-sub.Err():
				return
			}
		}
	}()
	return sub, nil
}

func repairM821Node(t *testing.T) (*RepairM821Node, *httptest.Server, *httptest.Server) {
	t.Helper()
	node := &RepairM821Node{receipts: make(map[common.Hash]*ethtypes.Receipt), feeds: make(chan chan *ethtypes.Header)}
	codec := gethrpc.NewServer()
	require.NoError(t, codec.RegisterName("eth", node))
	httpServer := httptest.NewServer(codec)
	wsServer := httptest.NewServer(codec.WebsocketHandler([]string{"*"}))
	t.Cleanup(func() { codec.Stop(); httpServer.Close(); wsServer.Close() })
	return node, httpServer, wsServer
}

func repairM821Receipt(tx *types.LoadTx, status uint64) *ethtypes.Receipt {
	return &ethtypes.Receipt{Type: tx.EthTx.Type(), Status: status, CumulativeGasUsed: 21000, GasUsed: 21000,
		TxHash: tx.EthTx.Hash(), BlockHash: common.HexToHash("0x1234"), BlockNumber: big.NewInt(9), Logs: []*ethtypes.Log{}}
}

func repairM821RunWorker(t *testing.T, w *Worker) (context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("worker did not stop")
		}
	})
	return cancel, done
}

func TestRepairM_821_ParticipantRestartMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	collect := func() []metricdata.DataPoint[int64] {
		var data metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(t.Context(), &data))
		var points []metricdata.DataPoint[int64]
		for _, scope := range data.ScopeMetrics {
			for _, m := range scope.Metrics {
				if m.Name == "worker_queue_length" {
					points = append(points, m.Data.(metricdata.Gauge[int64]).DataPoints...)
				}
			}
		}
		return points
	}
	first := NewWorker(1, "125", "http://first.invalid", 4, 1, rate.NewLimiter(0.01, 1))
	first.SetDryRun(true)
	// Exhaust the initial burst so the queue remains observable.
	require.True(t, first.limiter.Allow())
	require.NoError(t, first.Send(t.Context(), repair324Tx(t, 0)))
	cancel, done := repairM821RunWorker(t, first)
	require.Eventually(t, func() bool { return len(collect()) == 1 }, time.Second, time.Millisecond)
	points := collect()
	require.Equal(t, int64(1), points[0].Value)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	require.Empty(t, collect())
	second := NewWorker(1, "125", "http://second.invalid", 4, 1, rate.NewLimiter(0.01, 1))
	second.SetDryRun(true)
	require.True(t, second.limiter.Allow())
	require.NoError(t, second.Send(t.Context(), repair324Tx(t, 1)))
	require.NoError(t, second.Send(t.Context(), repair324Tx(t, 2)))
	cancel2, done2 := repairM821RunWorker(t, second)
	require.Eventually(t, func() bool { return len(collect()) == 1 }, time.Second, time.Millisecond)
	points = collect()
	require.Equal(t, int64(2), points[0].Value)
	endpoint, ok := points[0].Attributes.Value("endpoint")
	require.True(t, ok)
	require.Equal(t, second.endpoint, endpoint.AsString())
	run, ok := points[0].Attributes.Value("run_id")
	require.True(t, ok)
	require.Equal(t, int64(second.runID), run.AsInt64())
	require.NotEqual(t, first.runID, second.runID)
	cancel2()
	require.ErrorIs(t, <-done2, context.Canceled)
	require.Empty(t, collect())
}

func TestRepairM_821_SubmissionAndReceipts(t *testing.T) {
	node, server, _ := repairM821Node(t)
	txs := []*types.LoadTx{repair324Tx(t, 0), repair324Tx(t, 1), repair324Tx(t, 2)}
	node.receipts[txs[0].EthTx.Hash()] = repairM821Receipt(txs[0], 1)
	node.receipts[txs[1].EthTx.Hash()] = repairM821Receipt(txs[1], 0)
	w := NewWorker(1, "125", server.URL, 2, 2, nil)
	w.SetTrackReceipts(true)
	w.receiptTimeout = 350 * time.Millisecond
	collector := stats.NewCollector()
	w.SetStatsCollector(collector, nil)
	cancel, done := repairM821RunWorker(t, w)
	for _, tx := range txs {
		require.NoError(t, w.Send(t.Context(), tx))
	}
	require.Eventually(t, func() bool { s := w.GetReceiptStats(); return s.Succeeded == 1 && s.Failed == 1 && s.TimedOut == 1 }, 2*time.Second, time.Millisecond)
	require.Equal(t, uint64(3), collector.GetStats().TotalTxs)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	require.Equal(t, ReceiptStats{Accepted: 3, Succeeded: 1, Failed: 1, TimedOut: 1}, w.GetReceiptStats())
	node.mu.Lock()
	require.ElementsMatch(t, []common.Hash{txs[0].EthTx.Hash(), txs[1].EthTx.Hash(), txs[2].EthTx.Hash()}, node.submitted)
	node.mu.Unlock()
}

func TestRepairM_821_RPCRefusal(t *testing.T) {
	node, server, _ := repairM821Node(t)
	tx := repair324Tx(t, 0)
	w := NewWorker(1, "125", server.URL, 1, 1, nil)
	w.SetTrackReceipts(true)
	node.reject = true
	var rpcError *TransactionRPCError
	require.ErrorAs(t, w.sendTransaction(t.Context(), server.Client(), tx), &rpcError)
	require.Equal(t, -32000, rpcError.Code)
	require.Equal(t, ReceiptStats{}, w.GetReceiptStats())
	for _, body := range []string{"{", `{"jsonrpc":"2.0","id":0,"result":"0x1234"}`, `{"jsonrpc":"2.0","id":0,"result":null}`} {
		bad := httptest.NewServer(http.HandlerFunc(func(out http.ResponseWriter, _ *http.Request) { _, _ = out.Write([]byte(body)) }))
		w.endpoint = bad.URL
		require.Error(t, w.sendTransaction(t.Context(), bad.Client(), tx))
		require.Empty(t, w.sentTxs)
		bad.Close()
	}
}

func TestRepairM_821_TrackingSaturationAndShutdown(t *testing.T) {
	node, server, _ := repairM821Node(t)
	gate := make(chan struct{})
	node.receiptGate = gate
	w := NewWorker(1, "125", server.URL, 1, 2, nil)
	w.SetTrackReceipts(true)
	cancel, done := repairM821RunWorker(t, w)
	txs := make([]*types.LoadTx, 8)
	for i := range txs {
		txs[i] = repair324Tx(t, uint64(i))
		node.mu.Lock()
		node.receipts[txs[i].EthTx.Hash()] = repairM821Receipt(txs[i], 1)
		node.mu.Unlock()
	}
	enqueued := make(chan error, 1)
	go func() {
		for _, tx := range txs {
			if err := w.Send(t.Context(), tx); err != nil {
				enqueued <- err
				return
			}
		}
		enqueued <- nil
	}()
	require.Eventually(t, func() bool {
		node.mu.Lock()
		defer node.mu.Unlock()
		return node.active == 2 && w.GetReceiptStats().Accepted >= 4
	}, time.Second, time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	require.LessOrEqual(t, w.GetReceiptStats().Accepted, uint64(5), "two consumers, one queued item and two blocked producers bound admission")
	close(gate)
	select {
	case err := <-enqueued:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("submission did not recover from backpressure")
	}
	require.Eventually(t, func() bool { return w.GetReceiptStats().Succeeded == 8 }, 2*time.Second, time.Millisecond)
	// Keep a final accepted transaction unresolved and cancel during receipt work.
	missing := repair324Tx(t, 99)
	require.NoError(t, w.Send(t.Context(), missing))
	require.Eventually(t, func() bool { return w.GetReceiptStats().Accepted == 9 }, time.Second, time.Millisecond)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	final := w.GetReceiptStats()
	require.Equal(t, ReceiptStats{Accepted: 9, Succeeded: 8, Unresolved: 1}, final)
	node.mu.Lock()
	require.Equal(t, 2, node.maxActive)
	node.mu.Unlock()
}

func TestRepairM_821_LowRateAndRecovery(t *testing.T) {
	node, server, _ := repairM821Node(t)
	limiter := rate.NewLimiter(2, 1)
	w := NewWorker(1, "125", server.URL, 4, 1, limiter)
	cancel, done := repairM821RunWorker(t, w)
	first, second := repair324Tx(t, 0), repair324Tx(t, 1)
	var cpuStart syscall.Rusage
	require.NoError(t, syscall.Getrusage(syscall.RUSAGE_SELF, &cpuStart))
	start := time.Now()
	require.NoError(t, w.Send(t.Context(), first))
	require.NoError(t, w.Send(t.Context(), second))
	require.Eventually(t, func() bool { node.mu.Lock(); defer node.mu.Unlock(); return len(node.submitted) == 1 }, 300*time.Millisecond, time.Millisecond)
	time.Sleep(150 * time.Millisecond)
	node.mu.Lock()
	require.Equal(t, []common.Hash{first.EthTx.Hash()}, node.submitted)
	node.mu.Unlock()
	require.Eventually(t, func() bool { node.mu.Lock(); defer node.mu.Unlock(); return len(node.submitted) == 2 }, time.Second, time.Millisecond)
	require.GreaterOrEqual(t, time.Since(start), 450*time.Millisecond)
	var cpuEnd syscall.Rusage
	require.NoError(t, syscall.Getrusage(syscall.RUSAGE_SELF, &cpuEnd))
	cpuUsed := time.Duration(cpuEnd.Utime.Nano() + cpuEnd.Stime.Nano() - cpuStart.Utime.Nano() - cpuStart.Stime.Nano())
	require.Less(t, cpuUsed, 200*time.Millisecond, "low-rate participant must sleep instead of busy-spinning")
	// The real recovery curve sets a 1 TPS interval; an exhausted limiter must
	// leave pending work queued and cancellation must wake its reservation.
	limiter.SetLimit(rate.Limit(NewRampCurveStep(20, 10, time.Second, time.Second).GetTPS(1500 * time.Millisecond)))
	require.NoError(t, w.Send(t.Context(), repair324Tx(t, 2)))
	time.Sleep(40 * time.Millisecond)
	cancelStart := time.Now()
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	require.Less(t, time.Since(cancelStart), 200*time.Millisecond)
	node.mu.Lock()
	require.Equal(t, []common.Hash{first.EthTx.Hash(), second.EthTx.Hash()}, node.submitted)
	node.mu.Unlock()
	require.Len(t, w.txChan, 1)
}

func repairM821StartCollector(t *testing.T, node *RepairM821Node, ws *httptest.Server, collector *stats.BlockCollector) (chan *ethtypes.Header, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- collector.Run(ctx, ws.URL) }()
	var feed chan *ethtypes.Header
	select {
	case feed = <-node.feeds:
	case err := <-done:
		t.Fatalf("collector subscription failed: %v", err)
	case <-time.After(time.Second):
		t.Fatal("subscription did not start")
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(time.Second):
				t.Error("collector did not stop")
			}
		})
	}
	t.Cleanup(stop)
	return feed, stop
}

func repairM821Header(t *testing.T, feed chan *ethtypes.Header, collector *stats.BlockCollector, height uint64) {
	t.Helper()
	header := &ethtypes.Header{Number: new(big.Int).SetUint64(height), Difficulty: big.NewInt(0), Time: uint64(time.Now().Unix()), GasLimit: 30000000, GasUsed: 21000, BaseFee: big.NewInt(1), Extra: []byte{}}
	select {
	case feed <- header:
	case <-time.After(time.Second):
		t.Fatal("header transport stalled")
	}
	require.Eventually(t, func() bool { return collector.GetBlockStats().MaxBlockNumber == height }, time.Second, time.Millisecond)
}

func TestRepairM_821_RamperLiveProgressSLOAndRestart(t *testing.T) {
	node, _, ws := repairM821Node(t)
	for run := range 2 {
		collector := stats.NewBlockCollector("125")
		require.Zero(t, collector.GetBlockStats().SampleCount)
		feed, stopCollector := repairM821StartCollector(t, node, ws, collector)
		limiter := rate.NewLimiter(100, 1)
		ramper := NewRamper(NewRampCurveStep(50, 25, 10*time.Second, time.Second), collector, limiter)
		ramper.noProgressTimeout = 3 * time.Second
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- ramper.Run(ctx) }()
		// No distribution, then one interval: neither may advance the ramp.
		time.Sleep(150 * time.Millisecond)
		require.Equal(t, rate.Limit(1), limiter.Limit())
		repairM821Header(t, feed, collector, uint64(run*10+1))
		time.Sleep(120 * time.Millisecond)
		repairM821Header(t, feed, collector, uint64(run*10+2))
		time.Sleep(120 * time.Millisecond)
		require.Equal(t, rate.Limit(1), limiter.Limit())
		repairM821Header(t, feed, collector, uint64(run*10+3))
		require.Eventually(t, func() bool { return limiter.Limit() == 50 }, time.Second, time.Millisecond)
		require.Equal(t, 2, collector.GetWindowBlockStats().SampleCount)
		time.Sleep(1100 * time.Millisecond)
		repairM821Header(t, feed, collector, uint64(run*10+4))
		select {
		case err := <-done:
			require.ErrorIs(t, err, ErrRampTestFailedSLO)
		case <-time.After(time.Second):
			t.Fatal("SLO breach did not stop ramp")
		}
		cancel()
		require.Equal(t, rate.Limit(1), limiter.Limit())
		stopCollector()
	}
}

func TestRepairM_821_RamperStalledEvidence(t *testing.T) {
	for _, afterProgress := range []bool{false, true} {
		t.Run(fmt.Sprint(afterProgress), func(t *testing.T) {
			node, _, ws := repairM821Node(t)
			collector := stats.NewBlockCollector("125")
			feed, _ := repairM821StartCollector(t, node, ws, collector)
			limiter := rate.NewLimiter(100, 1)
			ramper := NewRamper(NewRampCurveStep(50, 25, time.Second, time.Second), collector, limiter)
			ramper.noProgressTimeout = 600 * time.Millisecond
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- ramper.Run(ctx) }()
			time.Sleep(150 * time.Millisecond)
			require.Equal(t, rate.Limit(1), limiter.Limit())
			if afterProgress {
				for height := uint64(1); height <= 3; height++ {
					repairM821Header(t, feed, collector, height)
					time.Sleep(110 * time.Millisecond)
				}
				require.Eventually(t, func() bool { return limiter.Limit() == 50 }, 200*time.Millisecond, time.Millisecond)
			}
			select {
			case err := <-done:
				require.ErrorIs(t, err, ErrRampTestFailedSLO)
			case <-time.After(1500 * time.Millisecond):
				t.Fatal("stalled chain kept ramping")
			}
			require.Equal(t, rate.Limit(1), limiter.Limit())
		})
	}
}

func TestRepairM_821_WriterBlockLimit(t *testing.T) {
	for _, blocks := range []uint64{0, 1, 3} {
		t.Run(fmt.Sprint(blocks), func(t *testing.T) {
			dir := t.TempDir()
			writer := NewTxsWriter(21000, dir, 4, blocks)
			for i := uint64(0); i < blocks; i++ {
				require.NoError(t, writer.Send(t.Context(), repair324Tx(t, i)))
			}
			require.ErrorIs(t, writer.Send(t.Context(), repair324Tx(t, 99)), ErrWriterComplete)
			require.NoError(t, writer.Flush())
			require.NoError(t, writer.Flush())
			files, err := filepath.Glob(filepath.Join(dir, "*_txs.json"))
			require.NoError(t, err)
			require.Len(t, files, int(blocks))
			for i := uint64(0); i < blocks; i++ {
				repair324ReadBatch(t, writer.batchPath(4+i), repair324Tx(t, i))
			}
			reopened := NewTxsWriter(21000, dir, 4, blocks)
			require.ErrorIs(t, reopened.Send(t.Context(), repair324Tx(t, 100)), ErrWriterComplete)
			require.Empty(t, reopened.txBuffer)
			require.Equal(t, blocks, reopened.blocksGenerated)
		})
	}
	dir := t.TempDir()
	writer := NewTxsWriter(20000, dir, 1, 1)
	require.ErrorContains(t, writer.Send(t.Context(), repair324Tx(t, 0)), "exceeds block gas limit")
	require.Empty(t, writer.txBuffer)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestRepairM_821_WriterDispatcherTail(t *testing.T) {
	cfg := &config.LoadConfig{ChainID: 125}
	accounts := types.NewAccountPool(&types.AccountConfig{Accounts: types.GenerateAccounts(2)})
	scenario := scenarios.CreateScenario(config.Scenario{Name: scenarios.EVMTransfer, Weight: 1})
	scenario.Deploy(cfg, accounts.NextAccount())
	for _, count := range []int{1, 4} {
		dir := t.TempDir()
		writer := NewTxsWriter(42000, dir, 1, 1)
		dispatcher := NewDispatcher(generator.NewScenarioGenerator(accounts, scenario), writer)
		require.NoError(t, dispatcher.RunBatch(t.Context(), count))
		require.Equal(t, uint64(min(count, 2)), dispatcher.GetStats().TotalSent)
		files, err := filepath.Glob(filepath.Join(dir, "*_txs.json"))
		require.NoError(t, err)
		require.Len(t, files, 1)
		data, err := os.ReadFile(files[0])
		require.NoError(t, err)
		var batch TxWriteData
		require.NoError(t, json.Unmarshal(data, &batch))
		require.Len(t, batch.TxPayloads, min(count, 2))
		for _, payload := range batch.TxPayloads {
			var tx ethtypes.Transaction
			require.NoError(t, tx.UnmarshalBinary(payload))
			_, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(big.NewInt(125)), &tx)
			require.NoError(t, err)
		}
		require.Empty(t, writer.txBuffer)
	}
}

func TestRepairM_821_DispatcherCompletionAndWriteFailure(t *testing.T) {
	cfg := &config.LoadConfig{ChainID: 125}
	accounts := types.NewAccountPool(&types.AccountConfig{Accounts: types.GenerateAccounts(2)})
	scenario := scenarios.CreateScenario(config.Scenario{Name: scenarios.EVMTransfer, Weight: 1})
	scenario.Deploy(cfg, accounts.NextAccount())
	dir := t.TempDir()
	writer := NewTxsWriter(42000, dir, 1, 1)
	dispatcher := NewDispatcher(generator.NewScenarioGenerator(accounts, scenario), writer)
	require.NoError(t, dispatcher.Run(t.Context()))
	require.Equal(t, uint64(2), dispatcher.GetStats().TotalSent)
	files, err := filepath.Glob(filepath.Join(dir, "*_txs.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(blocked, []byte("obstruction"), 0600))
	broken := NewDispatcher(generator.NewScenarioGenerator(accounts, scenario), NewTxsWriter(42000, blocked, 1, 1))
	var pathErr *os.PathError
	require.ErrorAs(t, broken.RunBatch(t.Context(), 3), &pathErr)
	require.Zero(t, broken.GetStats().TotalSent)
}

func TestRepairM_821_WriterDurability(t *testing.T) {
	// Preserve the prerequisite's real-filesystem failure and restart assertions.
	TestRepair_324_WriterErrorDataLoss(t)
	TestRepair_324_WriterRestartRecovery(t)
	TestRepair_324_WriterDuplicateHeight(t)
	TestRepair_324_WriterRejectsCorruptRecovery(t)
	dir := t.TempDir()
	writer := NewTxsWriter(42000, dir, 1, 1)
	tx := repair324Tx(t, 0)
	require.NoError(t, writer.Send(t.Context(), tx))
	require.NoError(t, writer.stageBatch(repair324Batch(t, tx)))
	require.NoFileExists(t, writer.batchPath(1))
	pendingInfo, err := os.Stat(writer.pendingPath(1))
	require.NoError(t, err)
	pending, err := os.ReadFile(writer.pendingPath(1))
	require.NoError(t, err)
	require.Equal(t, repair324Batch(t, tx), pending)
	reopened := NewTxsWriter(42000, dir, 1, 1)
	require.NoError(t, reopened.resume())
	require.NoFileExists(t, reopened.pendingPath(1))
	publishedInfo, err := os.Stat(reopened.batchPath(1))
	require.NoError(t, err)
	require.True(t, os.SameFile(pendingInfo, publishedInfo), "publication links the complete synced inode atomically")
	repair324ReadBatch(t, reopened.batchPath(1), tx)
	files, err := filepath.Glob(filepath.Join(dir, "*_txs.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.ErrorIs(t, reopened.Send(t.Context(), repair324Tx(t, 1)), ErrWriterComplete)
}
