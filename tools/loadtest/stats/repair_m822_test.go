package stats_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/generator/scenarios"
	"github.com/paxeer-network/pax-load/sender"
	"github.com/paxeer-network/pax-load/stats"
	loadtypes "github.com/paxeer-network/pax-load/types"
	"github.com/stretchr/testify/require"
)

// This committed peer fixture serves signed transactions, linked headers and
// receipts through geth's real HTTP/WebSocket JSON-RPC codecs. Only peers of the
// repaired stats components are fixtures; the sender and collectors are real.
type RepairM822Node struct {
	mu             sync.Mutex
	blocks         []*ethtypes.Block
	head           uint64
	missing        map[uint64]bool
	receipts       map[common.Hash]*ethtypes.Receipt
	hiddenReceipts map[common.Hash]bool
	blockReads     map[common.Hash]int
	accepted       map[common.Hash][]byte
	reject         bool
	requests       int
	subscriptions  int
	notifiers      []repairM822Subscription
	codec          *rpc.Server
	server         *httptest.Server
}

type repairM822Subscription struct {
	notifier *rpc.Notifier
	id       rpc.ID
}

func repairM822Node(t *testing.T, blocks []*ethtypes.Block, head uint64) *RepairM822Node {
	t.Helper()
	node := &RepairM822Node{blocks: blocks, head: head, missing: make(map[uint64]bool), receipts: make(map[common.Hash]*ethtypes.Receipt), hiddenReceipts: make(map[common.Hash]bool), blockReads: make(map[common.Hash]int), accepted: make(map[common.Hash][]byte)}
	node.codec = rpc.NewServer()
	require.NoError(t, node.codec.RegisterName("eth", node))
	node.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		node.mu.Lock()
		codec := node.codec
		node.mu.Unlock()
		if strings.EqualFold(request.Header.Get("Upgrade"), "websocket") {
			codec.WebsocketHandler([]string{"*"}).ServeHTTP(w, request)
		} else {
			codec.ServeHTTP(w, request)
		}
	}))
	t.Cleanup(func() { node.codec.Stop(); node.server.Close() })
	return node
}

func (node *RepairM822Node) NewHeads(ctx context.Context) (*rpc.Subscription, error) {
	notifier, ok := rpc.NotifierFromContext(ctx)
	if !ok {
		return nil, errors.New("subscription transport unavailable")
	}
	subscription := notifier.CreateSubscription()
	node.mu.Lock()
	node.subscriptions++
	node.notifiers = append(node.notifiers, repairM822Subscription{notifier, subscription.ID})
	node.mu.Unlock()
	return subscription, nil
}

func (node *RepairM822Node) publish(t *testing.T, height uint64) {
	t.Helper()
	node.mu.Lock()
	node.head = height
	header := node.blocks[height-1].Header()
	subscribers := append([]repairM822Subscription(nil), node.notifiers...)
	node.mu.Unlock()
	for _, subscriber := range subscribers {
		_ = subscriber.notifier.Notify(subscriber.id, header)
	}
}

func (node *RepairM822Node) restart(t *testing.T, height uint64) {
	t.Helper()
	codec := rpc.NewServer()
	require.NoError(t, codec.RegisterName("eth", node))
	node.mu.Lock()
	old := node.codec
	node.codec = codec
	node.head = height
	node.notifiers = nil
	node.mu.Unlock()
	old.Stop()
}

func (node *RepairM822Node) GetBlockByNumber(number string, full bool) (json.RawMessage, error) {
	node.mu.Lock()
	defer node.mu.Unlock()
	height := node.head
	if number != "latest" {
		var err error
		height, err = hexutil.DecodeUint64(number)
		if err != nil {
			return nil, err
		}
	}
	if height == 0 || height > node.head || height > uint64(len(node.blocks)) || node.missing[height] {
		return json.RawMessage("null"), nil
	}
	return repairM822BlockJSON(node.blocks[height-1], full)
}

func (node *RepairM822Node) GetBlockByHash(hash common.Hash, full bool) (json.RawMessage, error) {
	node.mu.Lock()
	defer node.mu.Unlock()
	node.blockReads[hash]++
	for _, block := range node.blocks {
		if block.Hash() == hash && block.NumberU64() <= node.head {
			return repairM822BlockJSON(block, full)
		}
	}
	return json.RawMessage("null"), nil
}

func (node *RepairM822Node) GetTransactionReceipt(hash common.Hash) (*ethtypes.Receipt, error) {
	node.mu.Lock()
	defer node.mu.Unlock()
	if node.hiddenReceipts[hash] {
		return nil, nil
	}
	return node.receipts[hash], nil
}

func (node *RepairM822Node) SendRawTransaction(input hexutil.Bytes) (common.Hash, error) {
	node.mu.Lock()
	defer node.mu.Unlock()
	node.requests++
	var tx ethtypes.Transaction
	if err := tx.UnmarshalBinary(input); err != nil {
		return common.Hash{}, err
	}
	expected, exists := node.accepted[tx.Hash()]
	if !exists || string(expected) != string(input) {
		return common.Hash{}, fmt.Errorf("transaction is outside committed transcript")
	}
	if node.reject {
		return common.Hash{}, errors.New("node rejected transaction: invalid nonce")
	}
	return tx.Hash(), nil
}

func repairM822BlockJSON(block *ethtypes.Block, full bool) (json.RawMessage, error) {
	data, err := json.Marshal(block.Header())
	if err != nil || !full {
		return data, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	transactions := make([]json.RawMessage, 0, len(block.Transactions()))
	for _, tx := range block.Transactions() {
		encoded, err := json.Marshal(tx)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, encoded)
	}
	fields["transactions"], err = json.Marshal(transactions)
	if err != nil {
		return nil, err
	}
	fields["uncles"] = json.RawMessage("[]")
	return json.Marshal(fields)
}

func repairM822Blocks(count int) []*ethtypes.Block {
	blocks := make([]*ethtypes.Block, 0, count)
	timestamp := uint64(time.Now().Add(-4 * time.Hour).Unix())
	var parent common.Hash
	for height := 1; height <= count; height++ {
		interval, gas := uint64(2), uint64(1)
		if height == 2 {
			interval = 10
		}
		if height == 1 {
			gas = 999999
		}
		timestamp += interval
		header := &ethtypes.Header{Number: big.NewInt(int64(height)), ParentHash: parent, Difficulty: new(big.Int), Time: timestamp, GasLimit: 30000000, GasUsed: gas}
		block := ethtypes.NewBlock(header, &ethtypes.Body{}, nil, nil)
		blocks = append(blocks, block)
		parent = block.Hash()
	}
	return blocks
}

func repairM822RunCollector(t *testing.T, collector *stats.BlockCollector, endpoint string) (context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- collector.Run(ctx, endpoint) }()
	t.Cleanup(cancel)
	return cancel, done
}

func repairM822Height(t *testing.T, collector *stats.BlockCollector, height uint64) {
	t.Helper()
	require.Eventually(t, func() bool { return collector.GetBlockStats().MaxBlockNumber == height }, 5*time.Second, time.Millisecond)
}

func repairM822ReadReport(t *testing.T, collector *stats.Collector) (*stats.FinalStats, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "report.json")
	_, reportErr := stats.NewLogger(collector, time.Second, path, false).LogFinalStats()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var result stats.FinalStats
	require.NoError(t, json.Unmarshal(data, &result))
	return &result, reportErr
}

func repairM822BoundNames(violations []stats.Violation) []string {
	result := make([]string, 0, len(violations))
	for _, violation := range violations {
		result = append(result, violation.Bound)
	}
	return result
}

func TestRepairM_822_BlockSubscriptionRecovery(t *testing.T) {
	node := repairM822Node(t, repairM822Blocks(8), 1)
	path := filepath.Join(t.TempDir(), "blocks.json")
	collector, err := stats.OpenBlockCollector("125", path)
	require.NoError(t, err)
	cancel, done := repairM822RunCollector(t, collector, node.server.URL)
	repairM822Height(t, collector, 1)
	node.publish(t, 2)
	repairM822Height(t, collector, 2)
	// A node restart loses heights 3 and 4 from the subscription. The same
	// collector must reconnect and obtain every committed header over RPC.
	node.restart(t, 5)
	repairM822Height(t, collector, 5)
	node.publish(t, 5)
	node.publish(t, 5)
	node.publish(t, 6)
	repairM822Height(t, collector, 6)
	require.Equal(t, 6, collector.GetBlockStats().GasSampleCount)
	require.Equal(t, 5, collector.GetBlockStats().SampleCount)
	require.False(t, collector.GetBlockStats().Incomplete)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	// Stop and reopen the actual consumer from its durable checkpoint, then
	// recover heights committed while the consumer was offline.
	collector = nil
	reopened, err := stats.OpenBlockCollector("125", path)
	require.NoError(t, err)
	require.Equal(t, uint64(6), reopened.GetBlockStats().MaxBlockNumber)
	node.restart(t, 8)
	cancelReopened, reopenedDone := repairM822RunCollector(t, reopened, node.server.URL)
	repairM822Height(t, reopened, 8)
	cancelReopened()
	require.ErrorIs(t, <-reopenedDone, context.Canceled)
	result := reopened.GetBlockStats()
	require.Equal(t, 8, result.GasSampleCount)
	require.Equal(t, 7, result.SampleCount)
	require.Equal(t, 10*time.Second, result.MaxBlockTime)
	require.False(t, result.Incomplete)
	node.mu.Lock()
	subscriptions := node.subscriptions
	node.mu.Unlock()
	require.GreaterOrEqual(t, subscriptions, 3)
	_, err = stats.OpenBlockCollector("other-chain", path)
	require.Error(t, err)
	require.NoError(t, os.WriteFile(path, []byte("{"), 0600))
	_, err = stats.OpenBlockCollector("125", path)
	require.Error(t, err)
}

func TestRepairM_822_UnrecoverableGap(t *testing.T) {
	node := repairM822Node(t, repairM822Blocks(3), 1)
	path := filepath.Join(t.TempDir(), "blocks.json")
	blocks, err := stats.OpenBlockCollector("125", path)
	require.NoError(t, err)
	_, done := repairM822RunCollector(t, blocks, node.server.URL)
	repairM822Height(t, blocks, 1)
	node.mu.Lock()
	node.missing[2] = true
	node.mu.Unlock()
	node.publish(t, 3)
	select {
	case err := <-done:
		require.ErrorContains(t, err, "height 2")
	case <-time.After(8 * time.Second):
		t.Fatal("unrecoverable gap did not exhaust bounded recovery")
	}
	collector := stats.NewCollector()
	collector.SetBlockCollector(blocks)
	report, err := repairM822ReadReport(t, collector)
	require.ErrorContains(t, err, "incomplete load test evidence")
	require.True(t, report.Incomplete)
	require.Equal(t, uint64(1), report.BlockStatistics.MaxBlockNumber)
	require.Contains(t, report.BlockStatistics.RecoveryError, "height 2")
	require.Contains(t, repairM822BoundNames((stats.Bounds{MaxP99Latency: time.Second}).Check(report)), "incomplete_run")
	reopened, err := stats.OpenBlockCollector("125", path)
	require.NoError(t, err)
	require.True(t, reopened.GetBlockStats().Incomplete)
	require.Equal(t, report.BlockStatistics.RecoveryError, reopened.GetBlockStats().RecoveryError)
}

func TestRepairM_822_BlockStatisticsSemantics(t *testing.T) {
	const count = 1106
	node := repairM822Node(t, repairM822Blocks(count), 1)
	blocks := stats.NewBlockCollector("125")
	cancel, done := repairM822RunCollector(t, blocks, node.server.URL)
	repairM822Height(t, blocks, 1)
	// A burst of delayed delivery must preserve the ordered chain intervals,
	// including the outlier and gas maximum beyond the old 1000-sample cap.
	node.publish(t, count-1)
	repairM822Height(t, blocks, count-1)
	full := blocks.GetBlockStats()
	require.Equal(t, count-2, full.SampleCount)
	require.Equal(t, count-1, full.GasSampleCount)
	require.Equal(t, 10*time.Second, full.MaxBlockTime)
	require.Equal(t, 2*time.Second, full.P50BlockTime)
	require.Equal(t, 2*time.Second, full.P99BlockTime)
	require.Equal(t, uint64(999999), full.MaxGasUsed)
	require.Equal(t, count-1, full.DeliverySampleCount)
	require.Greater(t, full.P50DeliveryLag, time.Hour)
	blocks.ResetWindowStats()
	node.publish(t, count)
	repairM822Height(t, blocks, count)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	require.Equal(t, 1, blocks.GetWindowBlockStats().SampleCount)
	require.Equal(t, 2*time.Second, blocks.GetWindowBlockTimePercentile(99))
	require.Equal(t, "reporting_window_committed_headers", blocks.GetWindowBlockStats().Scope)
	collector := stats.NewCollector()
	collector.SetBlockCollector(blocks)
	report, err := repairM822ReadReport(t, collector)
	require.NoError(t, err)
	require.Equal(t, count-1, report.BlockStatistics.SampleCount)
	require.Equal(t, "full_run_committed_headers", report.BlockStatistics.Scope)
	require.Equal(t, 10*time.Second, report.BlockStatistics.MaxBlockTime)
	require.Equal(t, report.BlockStatistics, report.GasStatistics)
	require.Contains(t, report.String(), "Delivery lag")
}

func TestRepairM_822_ConflictingCommittedHash(t *testing.T) {
	node := repairM822Node(t, repairM822Blocks(3), 1)
	blocks := stats.NewBlockCollector("125")
	_, done := repairM822RunCollector(t, blocks, node.server.URL)
	repairM822Height(t, blocks, 1)
	node.publish(t, 2)
	repairM822Height(t, blocks, 2)
	node.mu.Lock()
	alternate := node.blocks[0].Header()
	alternate.Extra = []byte("conflicting committed block")
	node.blocks[0] = ethtypes.NewBlockWithHeader(alternate)
	node.mu.Unlock()
	node.publish(t, 1)
	select {
	case err := <-done:
		require.ErrorContains(t, err, "conflicting committed hash")
	case <-time.After(5 * time.Second):
		t.Fatal("conflicting committed hash was hidden")
	}
	require.True(t, blocks.GetBlockStats().Incomplete)
	require.Equal(t, 2, blocks.GetBlockStats().GasSampleCount)
}

func TestRepairM_822_LatencyBoundRequiresEvidence(t *testing.T) {
	bounds := stats.Bounds{MaxP99Latency: 20 * time.Millisecond}
	require.Equal(t, []string{"insufficient_samples:max_p99_latency"}, repairM822BoundNames(bounds.Check(&stats.FinalStats{})))
	require.Equal(t, []string{"insufficient_samples:max_p99_latency"}, repairM822BoundNames(bounds.Check(nil)))
	collector := stats.NewCollector("required-idle", "sampled")
	collector.RecordTransaction("transfer", "sampled", 30*time.Millisecond, true)
	report, err := repairM822ReadReport(t, collector)
	require.NoError(t, err)
	require.Equal(t, []string{"required-idle", "sampled"}, report.RequiredEndpoints)
	require.Zero(t, report.EndpointPerformance["required-idle"].SampleCount)
	require.Equal(t, []string{"insufficient_samples:required-idle", "max_p99_latency:sampled"}, repairM822BoundNames(bounds.Check(report)))
	// Required membership survives a missing endpoint row in a report.
	delete(report.EndpointPerformance, "required-idle")
	require.Equal(t, []string{"insufficient_samples:required-idle", "max_p99_latency:sampled"}, repairM822BoundNames(bounds.Check(report)))
	require.Equal(t, []stats.Violation{{Bound: "max_p99_latency:sampled", Limit: "20ms", Observed: "30ms"}}, bounds.Check(&stats.FinalStats{EndpointPerformance: map[string]stats.EndpointPerformance{"sampled": report.EndpointPerformance["sampled"]}}))
	require.Empty(t, (stats.Bounds{MaxP99Latency: 30 * time.Millisecond}).Check(&stats.FinalStats{EndpointPerformance: map[string]stats.EndpointPerformance{"sampled": report.EndpointPerformance["sampled"]}}))
	report.Incomplete = true
	require.Empty(t, (stats.Bounds{}).Check(report))
}

func repairM822Generated(t *testing.T, name string) *loadtypes.LoadTx {
	t.Helper()
	var cfg config.LoadConfig
	require.NoError(t, json.Unmarshal([]byte(`{"chainId":125}`), &cfg))
	// Contract scenarios attach through the production RPC client. Supply a
	// loopback peer even though generation signs locally without broadcasting.
	cfg.Endpoints = []string{repairM822Node(t, nil, 0).server.URL}
	scenario := scenarios.CreateScenario(config.Scenario{Name: name})
	require.NoError(t, scenario.Attach(&cfg, common.HexToAddress("0x822")))
	account, err := loadtypes.NewAccount()
	require.NoError(t, err)
	return scenario.Generate(&loadtypes.TxScenario{Name: name, Sender: account, Receiver: common.HexToAddress("0x1234")})
}

func repairM822Send(t *testing.T, collector *stats.Collector, endpoint string, transactions ...*loadtypes.LoadTx) {
	t.Helper()
	worker := sender.NewWorker(822, "125", endpoint, len(transactions)+1, 1, nil)
	worker.SetStatsCollector(collector, nil)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	before := collector.GetStats().TotalTxs
	go func() { done <- worker.Run(ctx) }()
	t.Cleanup(cancel)
	for _, tx := range transactions {
		require.NoError(t, worker.Send(t.Context(), tx))
	}
	require.Eventually(t, func() bool { return collector.GetStats().TotalTxs == before+uint64(len(transactions)) }, 5*time.Second, time.Millisecond)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
}

func TestRepairM_822_FailedAttemptThroughput(t *testing.T) {
	tx := repairM822Generated(t, scenarios.EVMTransfer)
	for _, failure := range []string{"node_rejection", "connection_failure"} {
		t.Run(failure, func(t *testing.T) {
			node := repairM822Node(t, repairM822Blocks(1), 1)
			endpoint := node.server.URL
			node.accepted[tx.EthTx.Hash()] = tx.Payload
			if failure == "node_rejection" {
				node.reject = true
			} else {
				node.server.Close()
			}
			collector := stats.NewCollector(endpoint)
			repairM822Send(t, collector, endpoint, tx, tx, tx)
			report, err := repairM822ReadReport(t, collector)
			require.NoError(t, err)
			require.Equal(t, stats.OutcomeCounts{Attempted: 3, Failed: 3}, report.Outcomes)
			require.Equal(t, report.Outcomes, report.EndpointPerformance[endpoint].Outcomes)
			require.Equal(t, uint64(3), report.LoadTestStatistics.TotalTxs)
			require.Equal(t, uint64(3), report.ScenarioDistribution[scenarios.EVMTransfer])
			require.Zero(t, report.OverallPerformance.TotalTransactions)
			require.Zero(t, report.OverallPerformance.AverageTPS)
			require.Zero(t, report.OverallTPS.Current)
			require.Zero(t, report.OverallTPS.Max)
			performance := report.EndpointPerformance[endpoint]
			require.Zero(t, performance.CurrentTPS)
			require.Zero(t, performance.MaxTPS)
			require.Zero(t, performance.WindowTxCount)
			require.Zero(t, performance.WindowLatencyCount)
			require.Zero(t, performance.WindowLatencySum)
			require.Zero(t, performance.CumulativeMaxLatency)
			require.Zero(t, performance.SampleCount)
			require.Equal(t, "node_accepted_submissions", report.ThroughputBasis)
			require.Equal(t, "unobserved", report.CommitEvidence)
			names := repairM822BoundNames((stats.Bounds{MinAverageTPS: 0.1, MinTotalTxs: 1, MaxP99Latency: time.Second}).Check(report))
			require.Equal(t, []string{"min_average_tps", "min_total_txs", "insufficient_samples:accepted_submissions", "insufficient_samples:" + endpoint}, names)
			node.mu.Lock()
			requests := node.requests
			node.mu.Unlock()
			if failure == "node_rejection" {
				require.Equal(t, 3, requests)
			} else {
				require.Zero(t, requests)
			}
		})
	}
}

func TestRepairM_822_AcceptedSubmissionEvidence(t *testing.T) {
	node := repairM822Node(t, repairM822Blocks(1), 1)
	tx := repairM822Generated(t, scenarios.EVMTransferNoop)
	node.accepted[tx.EthTx.Hash()] = tx.Payload
	collector := stats.NewCollector(node.server.URL)
	repairM822Send(t, collector, node.server.URL, tx)
	report, err := repairM822ReadReport(t, collector)
	require.NoError(t, err)
	require.Equal(t, stats.OutcomeCounts{Attempted: 1, Accepted: 1}, report.Outcomes)
	require.Equal(t, uint64(1), report.OverallPerformance.TotalTransactions)
	require.Equal(t, 1, report.OverallPerformance.SampleCount)
	require.Positive(t, report.OverallPerformance.AverageTPS)
	require.Equal(t, 0.1, report.OverallTPS.Current)
	require.Equal(t, "last_10000_accepted_submissions", report.EndpointPerformance[node.server.URL].LatencyScope)
	require.Empty(t, (stats.Bounds{MinTotalTxs: 1, MaxP99Latency: time.Second}).Check(report))
	// A successful RPC submission does not assert committed execution.
	require.Zero(t, report.Outcomes.Committed)
	require.Equal(t, "unobserved", report.CommitEvidence)
}

func repairM822Payment(t *testing.T, nonce uint64, value *big.Int) *ethtypes.Transaction {
	t.Helper()
	key, err := crypto.HexToECDSA(strings.Repeat("02", 32))
	require.NoError(t, err)
	tx, err := ethtypes.SignTx(ethtypes.NewTransaction(nonce, common.HexToAddress("0xfeed"), value, 21000, big.NewInt(1), nil), ethtypes.NewEIP155Signer(big.NewInt(125)), key)
	require.NoError(t, err)
	return tx
}

func repairM822CommittedBlock(transactions []*ethtypes.Transaction) (*ethtypes.Block, []*ethtypes.Receipt) {
	receipts := make([]*ethtypes.Receipt, len(transactions))
	for i, tx := range transactions {
		status := uint64(ethtypes.ReceiptStatusSuccessful)
		if i == 1 {
			status = ethtypes.ReceiptStatusFailed
		}
		receipts[i] = &ethtypes.Receipt{Type: tx.Type(), Status: status, CumulativeGasUsed: uint64(i+1) * 21000, GasUsed: 21000, Logs: []*ethtypes.Log{}}
	}
	header := &ethtypes.Header{Number: big.NewInt(1), Difficulty: new(big.Int), Time: uint64(time.Now().Unix()), GasLimit: 30000000, GasUsed: uint64(len(transactions)) * 21000}
	header.TxHash = repairM822ListRoot(ethtypes.Transactions(transactions))
	header.ReceiptHash = repairM822ListRoot(ethtypes.Receipts(receipts))
	header.UncleHash = ethtypes.EmptyUncleHash
	block := ethtypes.NewBlockWithHeader(header).WithBody(ethtypes.Body{Transactions: transactions})
	for i, tx := range transactions {
		receipts[i].TxHash = tx.Hash()
		receipts[i].BlockHash = block.Hash()
		receipts[i].BlockNumber = block.Number()
		receipts[i].TransactionIndex = uint(i)
	}
	return block, receipts
}

func TestRepairM_822_UserLatencyUnrelatedTransactions(t *testing.T) {
	tracker := stats.NewUserLatencyTracker(time.Millisecond)
	workload := []*loadtypes.LoadTx{
		repairM822Generated(t, scenarios.EVMTransfer),
		repairM822Generated(t, scenarios.EVMTransferNoop),
		repairM822Generated(t, scenarios.ERC20),
	}
	ordinary := []*ethtypes.Transaction{
		repairM822Payment(t, 0, big.NewInt(1)),
		repairM822Payment(t, 1, new(big.Int).Lsh(big.NewInt(1), 200)),
		repairM822Payment(t, 2, big.NewInt(time.Now().Add(-time.Minute).Unix())),
	}
	transactions := make([]*ethtypes.Transaction, 0, len(workload)+len(ordinary))
	for _, load := range workload {
		transactions = append(transactions, load.EthTx)
	}
	transactions = append(transactions, ordinary...)
	block, receipts := repairM822CommittedBlock(transactions)
	node := repairM822Node(t, []*ethtypes.Block{block}, 1)
	for i, load := range workload {
		node.accepted[load.EthTx.Hash()] = load.Payload
		node.receipts[load.EthTx.Hash()] = receipts[i]
	}
	collector := stats.NewCollector(node.server.URL)
	collector.SetUserLatencyTracker(tracker)
	for _, load := range workload {
		require.NoError(t, tracker.TrackTransaction(load.EthTx.Hash(), node.server.URL, time.Now()))
	}
	missing := repairM822Generated(t, scenarios.EVMTransferNoop)
	require.NoError(t, tracker.TrackTransaction(missing.EthTx.Hash(), node.server.URL, time.Now()))
	// Drive the real sender with the generated workload before receipt polling.
	repairM822Send(t, collector, node.server.URL, workload...)
	client, err := ethclient.DialContext(t.Context(), node.server.URL)
	require.NoError(t, err)
	defer client.Close()
	// One committed receipt is initially unavailable; repeated polling must
	// recover it using the cached block without counting the block twice.
	node.hiddenReceipts[workload[2].EthTx.Hash()] = true
	require.NoError(t, tracker.Poll(t.Context(), client))
	first := tracker.GetStats()
	require.Equal(t, 2, first.SampleCount)
	require.Equal(t, uint64(2), first.MissingReceipts)
	require.Equal(t, 1, first.BlocksConsumed)
	node.mu.Lock()
	node.hiddenReceipts[workload[2].EthTx.Hash()] = false
	node.mu.Unlock()
	require.NoError(t, tracker.Poll(t.Context(), client))
	require.NoError(t, tracker.Poll(t.Context(), client))
	user := tracker.GetStats()
	require.Equal(t, 3, user.SampleCount)
	require.Equal(t, uint64(1), user.MissingReceipts)
	require.Equal(t, 1, user.BlocksConsumed)
	endpoint := user.Endpoints[node.server.URL]
	require.Equal(t, uint64(3), endpoint.Committed)
	require.Equal(t, uint64(1), endpoint.FailedExecutions)
	require.Positive(t, endpoint.LatencyP50)
	require.Positive(t, endpoint.LatencyP99)
	node.mu.Lock()
	reads := node.blockReads[block.Hash()]
	node.mu.Unlock()
	require.Equal(t, 1, reads)
	report, err := repairM822ReadReport(t, collector)
	require.ErrorContains(t, err, "missing committed receipts: 1")
	require.True(t, report.Incomplete)
	require.Equal(t, uint64(3), report.Outcomes.Accepted)
	require.Equal(t, uint64(3), report.Outcomes.Committed)
	require.Equal(t, uint64(1), report.Outcomes.FailedExecutions)
	require.Equal(t, uint64(1), report.Outcomes.MissingReceipts)
	require.Equal(t, 3, report.UserLatency.SampleCount)
	require.Contains(t, report.UserLatency.Scope, "send_to_committed_receipt_observation")
	// Neither a huge ordinary payment nor a plausible Unix timestamp becomes
	// a sample; only the three registered signed hashes are represented.
	require.Equal(t, 3, report.UserLatency.Endpoints[node.server.URL].SampleCount)
}

func TestRepairM_822_UserLatencyMissingAndInvalidEvidence(t *testing.T) {
	tracker := stats.NewUserLatencyTracker(time.Millisecond)
	tx := repairM822Generated(t, scenarios.EVMTransfer)
	endpoint := "workload-endpoint"
	sentAt := time.Now()
	require.Error(t, tracker.TrackTransaction(common.Hash{}, endpoint, sentAt))
	require.Error(t, tracker.TrackTransaction(tx.EthTx.Hash(), endpoint, time.Time{}))
	require.Error(t, tracker.TrackTransaction(tx.EthTx.Hash(), endpoint, sentAt.Add(time.Hour)))
	require.NoError(t, tracker.TrackTransaction(tx.EthTx.Hash(), endpoint, sentAt))
	require.NoError(t, tracker.TrackTransaction(tx.EthTx.Hash(), endpoint, sentAt))
	require.Error(t, tracker.TrackTransaction(tx.EthTx.Hash(), "other", sentAt))
	block, receipts := repairM822CommittedBlock([]*ethtypes.Transaction{tx.EthTx})
	node := repairM822Node(t, []*ethtypes.Block{block}, 1)
	client, err := ethclient.DialContext(t.Context(), node.server.URL)
	require.NoError(t, err)
	defer client.Close()
	require.NoError(t, tracker.Poll(t.Context(), client))
	require.Zero(t, tracker.GetStats().SampleCount)
	require.Equal(t, uint64(1), tracker.GetStats().MissingReceipts)
	bad := *receipts[0]
	bad.TxHash = common.HexToHash("0xbad")
	node.receipts[tx.EthTx.Hash()] = &bad
	require.ErrorContains(t, tracker.Poll(t.Context(), client), "invalid committed receipt")
	require.Zero(t, tracker.GetStats().SampleCount)
	require.Contains(t, tracker.GetStats().LastError, "invalid committed receipt")
	collector := stats.NewCollector(endpoint)
	collector.SetUserLatencyTracker(tracker)
	report, err := repairM822ReadReport(t, collector)
	require.Error(t, err)
	require.Contains(t, report.EvidenceErrors, "insufficient user latency samples")
	node.mu.Lock()
	node.receipts[tx.EthTx.Hash()] = receipts[0]
	node.mu.Unlock()
	require.NoError(t, tracker.Poll(t.Context(), client))
	require.Empty(t, tracker.GetStats().LastError)
	require.Equal(t, 1, tracker.GetStats().SampleCount)
	require.Zero(t, tracker.GetStats().MissingReceipts)
}

// Fixture generation uses the canonical RLP-indexed hexary Patricia tree. This
// keeps a full node/database dependency out of the stats gate while producing
// genuine transaction and receipt roots for the signed fixture contents.
type repairM822TrieEntry struct {
	path  []byte
	value []byte
}

func repairM822ListRoot(list ethtypes.DerivableList) common.Hash {
	if list.Len() == 0 {
		return ethtypes.EmptyRootHash
	}
	entries := make([]repairM822TrieEntry, 0, list.Len())
	for index := 0; index < list.Len(); index++ {
		key := rlp.AppendUint64(nil, uint64(index))
		path := make([]byte, 0, 2*len(key))
		for _, b := range key {
			path = append(path, b>>4, b&15)
		}
		var value bytes.Buffer
		list.EncodeIndex(index, &value)
		entries = append(entries, repairM822TrieEntry{path, append([]byte(nil), value.Bytes()...)})
	}
	return crypto.Keccak256Hash(repairM822TrieNode(entries, 0))
}

func repairM822RLP(value any) []byte {
	encoded, err := rlp.EncodeToBytes(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func repairM822Compact(path []byte, leaf bool) []byte {
	flag := byte(0)
	if leaf {
		flag = 2
	}
	nibbles := []byte{flag, 0}
	if len(path)%2 != 0 {
		nibbles = []byte{flag + 1}
	}
	nibbles = append(nibbles, path...)
	result := make([]byte, len(nibbles)/2)
	for i := range result {
		result[i] = nibbles[2*i]<<4 | nibbles[2*i+1]
	}
	return result
}

func repairM822TrieReference(encoded []byte) any {
	if len(encoded) < 32 {
		return rlp.RawValue(encoded)
	}
	return crypto.Keccak256(encoded)
}

func repairM822TrieNode(entries []repairM822TrieEntry, depth int) []byte {
	if len(entries) == 1 {
		return repairM822RLP([]any{repairM822Compact(entries[0].path[depth:], true), entries[0].value})
	}
	commonEnd := depth
	for commonEnd < len(entries[0].path) {
		shared := true
		for _, entry := range entries[1:] {
			if len(entry.path) <= commonEnd || entry.path[commonEnd] != entries[0].path[commonEnd] {
				shared = false
				break
			}
		}
		if !shared {
			break
		}
		commonEnd++
	}
	if commonEnd > depth {
		child := repairM822TrieNode(entries, commonEnd)
		return repairM822RLP([]any{repairM822Compact(entries[0].path[depth:commonEnd], false), repairM822TrieReference(child)})
	}
	branches := make([]any, 17)
	for i := range branches {
		branches[i] = []byte{}
	}
	for nibble := byte(0); nibble < 16; nibble++ {
		var children []repairM822TrieEntry
		for _, entry := range entries {
			if len(entry.path) == depth {
				branches[16] = entry.value
			} else if entry.path[depth] == nibble {
				children = append(children, entry)
			}
		}
		if len(children) > 0 {
			branches[nibble] = repairM822TrieReference(repairM822TrieNode(children, depth+1))
		}
	}
	return repairM822RLP(branches)
}
