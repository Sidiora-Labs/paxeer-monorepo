package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
)

type m880EthPeer struct {
	mu      sync.Mutex
	txs     map[common.Hash]*ethtypes.Transaction
	queries []common.Hash
	sent    []common.Hash
}

func (p *m880EthPeer) Version() string { return "125" }

func (p *m880EthPeer) GetTransactionCount(context.Context, common.Address, string) hexutil.Uint64 {
	return 0
}

func (p *m880EthPeer) GetTransactionReceipt(_ context.Context, hash common.Hash) (*ethtypes.Receipt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.queries = append(p.queries, hash)
	tx, ok := p.txs[hash]
	if !ok {
		return nil, fmt.Errorf("receipt requested for a transaction the producer did not emit: %s", hash)
	}
	return &ethtypes.Receipt{
		Type: tx.Type(), Status: ethtypes.ReceiptStatusSuccessful,
		TxHash: hash, BlockHash: common.HexToHash("0x880"), BlockNumber: big.NewInt(88),
		CumulativeGasUsed: tx.Gas(), GasUsed: tx.Gas(), Logs: []*ethtypes.Log{},
	}, nil
}

func (p *m880EthPeer) SendRawTransaction(_ context.Context, raw hexutil.Bytes) (common.Hash, error) {
	var tx ethtypes.Transaction
	if err := tx.UnmarshalBinary(raw); err != nil {
		return common.Hash{}, err
	}
	if _, err := ethtypes.Sender(ethtypes.NewLondonSigner(big.NewInt(125)), &tx); err != nil {
		return common.Hash{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.txs[tx.Hash()] = &tx
	p.sent = append(p.sent, tx.Hash())
	return tx.Hash(), nil
}

func m880Peer(t *testing.T) (*m880EthPeer, *ethclient.Client, string) {
	t.Helper()
	peer := &m880EthPeer{txs: make(map[common.Hash]*ethtypes.Transaction)}
	server := rpc.NewServer()
	if err := server.RegisterName("eth", peer); err != nil {
		t.Fatal(err)
	}
	if err := server.RegisterName("net", peer); err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	t.Cleanup(server.Stop)
	client, err := ethclient.Dial(httpServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return peer, client, httpServer.URL
}

func m880EvmClient(client *ethclient.Client) *EvmTxClient {
	return NewEvmTxClient(secp256k1.GenPrivKey(), big.NewInt(125), big.NewInt(1), []*ethclient.Client{client}, nil, false)
}

func m880Store(t *testing.T, capacity int) *evmTxHashStore {
	t.Helper()
	previous := EvmTxHashes
	EvmTxHashes = newEvmTxHashStore(capacity)
	t.Cleanup(func() { EvmTxHashes = previous })
	return EvmTxHashes
}

func m880CheckReceiptQueries(t *testing.T, peer *m880EthPeer, endpoint string, want []common.Hash) {
	t.Helper()
	peer.mu.Lock()
	peer.queries = nil
	peer.mu.Unlock()
	if err := runEvmQueries(Config{
		EvmRpcEndpoints: endpoint, EVMAddresses: &EVMAddresses{},
		PostTxEvmQueries: PostTxEvmQueries{Receipt: len(want)},
	}); err != nil {
		t.Fatal(err)
	}
	peer.mu.Lock()
	defer peer.mu.Unlock()
	counts := make(map[common.Hash]int)
	for _, hash := range peer.queries {
		counts[hash]++
	}
	if len(peer.queries) != len(want) {
		t.Fatalf("receipt transport got %d requests, want %d", len(peer.queries), len(want))
	}
	for _, hash := range want {
		if counts[hash] != 1 {
			t.Fatalf("receipt consumer queried %s %d times, want once", hash, counts[hash])
		}
	}
}

func TestRepairM_880_F042ConcurrentProducersAndReceiptQueries(t *testing.T) {
	const workers, perWorker = 8, 64
	store := m880Store(t, workers*perWorker)
	peer, ethClient, endpoint := m880Peer(t)
	client := &LoadTestClient{LoadTestConfig: Config{MessageTypes: []string{EVM}}}
	for i := 0; i < workers; i++ {
		client.EvmTxClients = append(client.EvmTxClients, m880EvmClient(ethClient))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue := make(chan SignedTx, workers*perWorker)
	counts := map[string]*int64{EVM: new(int64)}
	var running sync.WaitGroup
	readerDone := make(chan struct{})
	readerReady := make(chan struct{})
	readerStopped := make(chan struct{})
	readerErrors := make(chan error, 1)
	go func() {
		defer close(readerStopped)
		close(readerReady)
		for {
			select {
			case <-readerDone:
				return
			default:
			}
			snapshot := store.Snapshot()
			seen := make(map[common.Hash]bool)
			for _, hash := range snapshot {
				if hash == (common.Hash{}) || seen[hash] || len(snapshot) > workers*perWorker {
					select {
					case readerErrors <- fmt.Errorf("incoherent concurrent snapshot: %v", snapshot):
					default:
					}
					return
				}
				seen[hash] = true
			}
		}
	}()
	<-readerReady
	t.Cleanup(func() { close(readerDone); <-readerStopped })
	producerStopped := make(chan struct{})
	for i := 0; i < workers; i++ {
		running.Add(1)
		go func(index int) {
			defer running.Done()
			// Produce exactly perWorker real signatures under the limiter.
			client.BuildTxs(ctx, queue, index, rate.NewLimiter(0, perWorker), counts)
		}(i)
	}
	go func() { running.Wait(); close(producerStopped) }()
	produced := make(map[common.Hash]*ethtypes.Transaction)
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	for i := 0; i < workers*perWorker; i++ {
		select {
		case tx := <-queue:
			if tx.MsgType != EVM || tx.EvmTx == nil {
				t.Fatalf("producer emitted invalid queue entry: %+v", tx)
			}
			if _, err := ethtypes.Sender(ethtypes.NewLondonSigner(big.NewInt(125)), tx.EvmTx); err != nil {
				t.Fatal(err)
			}
			produced[tx.EvmTx.Hash()] = tx.EvmTx
			sender, err := ethtypes.Sender(ethtypes.NewLondonSigner(big.NewInt(125)), tx.EvmTx)
			if err != nil {
				t.Fatal(err)
			}
			owner := -1
			for index, evm := range client.EvmTxClients {
				if evm.accountAddress == sender {
					owner = index
					break
				}
			}
			if owner < 0 {
				t.Fatal("no owner for produced transaction")
			}
			toSend := make(chan SignedTx, 1)
			toSend <- tx
			close(toSend)
			if err := client.SendTxs(ctx, toSend, owner, map[string]*int64{EVM: new(int64)}, semaphore.NewWeighted(1)); err != nil {
				t.Fatal(err)
			}
		case <-timeout.C:
			t.Fatal("real producers did not fill their quotas")
		}
	}
	// Query the accepted submissions while the snapshot reader remains active.
	peer.mu.Lock()
	peer.txs = produced
	peer.mu.Unlock()
	snapshot := store.Snapshot()
	if len(snapshot) != workers*perWorker || len(produced) != workers*perWorker {
		t.Fatalf("lost hashes: store=%d produced=%d want=%d", len(snapshot), len(produced), workers*perWorker)
	}
	for _, hash := range snapshot {
		if _, ok := produced[hash]; !ok {
			t.Fatalf("store retained a hash not emitted by BuildTxs: %s", hash)
		}
	}
	m880CheckReceiptQueries(t, peer, endpoint, snapshot)
	client.EvmTxClients[0].EnsureTxSuccess(snapshot[0])
	select {
	case err := <-readerErrors:
		t.Fatal(err)
	default:
	}
	// The deferred signal stops every worker; wait before restoring the shared store.
	t.Cleanup(func() {
		select {
		case <-producerStopped:
		case <-time.After(5 * time.Second):
			t.Error("producers did not stop")
		}
		if count := atomic.LoadInt64(counts[EVM]); count != workers*perWorker {
			t.Errorf("production counter = %d, want %d", count, workers*perWorker)
		}
	})
}

func TestRepairM_880_F042BoundedIndependentSnapshot(t *testing.T) {
	const capacity = 4
	store := m880Store(t, capacity)
	peer, ethClient, endpoint := m880Peer(t)
	client := m880EvmClient(ethClient)
	var emitted []common.Hash
	var retainedSnapshot []common.Hash
	for i := 0; i < 5*capacity; i++ {
		tx, err := client.GetTxForMsgType(EVM)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.SendEvmTx(tx, func() { store.Add(tx.Hash()) }); err != nil {
			t.Fatal(err)
		}
		emitted = append(emitted, tx.Hash())
		first := len(emitted) - capacity
		if first < 0 {
			first = 0
		}
		if snapshot := store.Snapshot(); !reflect.DeepEqual(snapshot, emitted[first:]) {
			t.Fatalf("ring order at write %d = %v, want %v", i, snapshot, emitted[first:])
		}
		if len(store.hashes) > capacity || cap(store.hashes) > capacity {
			t.Fatalf("hash memory exceeded fixed capacity: len=%d cap=%d", len(store.hashes), cap(store.hashes))
		}
		if i == capacity-1 {
			retainedSnapshot = store.Snapshot()
		}
	}
	if !reflect.DeepEqual(retainedSnapshot, emitted[:capacity]) {
		t.Fatal("later writes changed an earlier snapshot")
	}
	snapshot := store.Snapshot()
	snapshot[0] = common.Hash{}
	if store.Snapshot()[0] == (common.Hash{}) {
		t.Fatal("snapshot mutation changed the retained store")
	}
	m880CheckReceiptQueries(t, peer, endpoint, emitted[len(emitted)-capacity:])
	client.EnsureTxSuccess(emitted[len(emitted)-1])

	t.Run("empty receipt source", func(t *testing.T) {
		m880Store(t, capacity)
		peer.mu.Lock()
		before := len(peer.queries)
		peer.mu.Unlock()
		if err := runEvmQueries(Config{EvmRpcEndpoints: endpoint, EVMAddresses: &EVMAddresses{}, PostTxEvmQueries: PostTxEvmQueries{Receipt: 3}}); err == nil || !strings.Contains(err.Error(), "receipt") {
			t.Fatalf("missing receipt observations: %v", err)
		}
		peer.mu.Lock()
		defer peer.mu.Unlock()
		if len(peer.queries) != before {
			t.Fatal("empty hash store issued receipt requests")
		}
	})
}

func m880Metrics(t *testing.T) *sdkmetric.ManualReader {
	t.Helper()
	previous := loadtestMetrics
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("repair-m880")
	loadtestMetrics.produceCount = must(meter.Int64Counter("produce"))
	loadtestMetrics.consumeCount = must(meter.Int64Counter("consume"))
	t.Cleanup(func() {
		loadtestMetrics = previous
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return reader
}

func m880Consumed(t *testing.T, reader *sdkmetric.ManualReader) int64 {
	t.Helper()
	var result metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &result); err != nil {
		t.Fatal(err)
	}
	var count int64
	for _, scope := range result.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != "consume" {
				continue
			}
			sum, ok := metric.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("unexpected consume metric data: %T", metric.Data)
			}
			for _, point := range sum.DataPoints {
				if value, ok := point.Attributes.Value("msg_type"); !ok || value.AsString() != EVM {
					t.Fatalf("phantom consumed message type: %v", point.Attributes)
				}
				count += point.Value
			}
		}
	}
	return count
}

func m880SendClosedQueue(t *testing.T, client *LoadTestClient, queue chan SignedTx, counts map[string]*int64) {
	t.Helper()
	close(queue)
	sem := semaphore.NewWeighted(1)
	result := make(chan any, 1)
	go func() {
		defer func() { result <- recover() }()
		client.SendTxs(context.Background(), queue, 0, counts, sem)
	}()
	select {
	case failure := <-result:
		if failure != nil {
			t.Fatalf("closed queue panicked: %v", failure)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("closed queue consumer did not exit")
	}
	if !sem.TryAcquire(1) {
		t.Fatal("consumer leaked semaphore permit")
	}
	sem.Release(1)
}

func TestRepairM_880_F026ClosedQueueShutdown(t *testing.T) {
	reader := m880Metrics(t)
	counts := map[string]*int64{EVM: new(int64)}
	m880SendClosedQueue(t, &LoadTestClient{}, make(chan SignedTx), counts)
	if *counts[EVM] != 0 || len(counts) != 1 || m880Consumed(t, reader) != 0 {
		t.Fatal("empty closed queue changed sent counters or emitted a consumed metric")
	}

	peer, ethClient, _ := m880Peer(t)
	client := &LoadTestClient{EvmTxClients: []*EvmTxClient{m880EvmClient(ethClient)}}
	queue := make(chan SignedTx, 3)
	var want []common.Hash
	for i := 0; i < cap(queue); i++ {
		tx, err := client.generateSignedEvmTx(0, EVM)
		if err != nil {
			t.Fatal(err)
		}
		queue <- SignedTx{EvmTx: tx, MsgType: EVM}
		want = append(want, tx.Hash())
	}
	previous := producedCountPerMsgType
	producedCountPerMsgType = map[string]*int64{EVM: new(int64)}
	t.Cleanup(func() { producedCountPerMsgType = previous })
	m880SendClosedQueue(t, client, queue, counts)
	if *counts[EVM] != 3 || *producedCountPerMsgType[EVM] != 0 || m880Consumed(t, reader) != 3 {
		t.Fatalf("closed buffered queue must send exactly three transactions without changing production count: sent=%d produced=%d", *counts[EVM], *producedCountPerMsgType[EVM])
	}
	peer.mu.Lock()
	defer peer.mu.Unlock()
	if !reflect.DeepEqual(peer.sent, want) {
		t.Fatalf("real EVM transport sent %v, want %v", peer.sent, want)
	}
}

func m880Config() Config {
	return Config{
		MessageTypes:  []string{WasmMintNft},
		PriceDistr:    NumericDistribution{Min: sdk.NewDec(2), Max: sdk.NewDec(4), NumDistinct: 4},
		QuantityDistr: NumericDistribution{Min: sdk.NewDec(3), Max: sdk.NewDec(7), NumDistinct: 4},
	}
}

func m880ExpectConfigFailure(t *testing.T, key string, admit func()) {
	t.Helper()
	defer func() {
		failure := recover()
		err, ok := failure.(error)
		if !ok || !strings.Contains(err.Error(), key) {
			t.Fatalf("expected configuration error naming %s, got %v", key, failure)
		}
	}()
	admit()
	t.Fatal("admitted invalid numeric distribution")
}

func TestRepairM_880_F027ConfigurationAdmission(t *testing.T) {
	for _, key := range []string{"price_distribution", "quantity_distribution"} {
		for _, defect := range []string{"zero", "negative", "missing min", "missing max", "reversed"} {
			t.Run(key+"/"+defect, func(t *testing.T) {
				config := m880Config()
				dist := &config.PriceDistr
				if key == "quantity_distribution" {
					dist = &config.QuantityDistr
				}
				suffix := ".number_of_distinct_values"
				switch defect {
				case "zero":
					dist.NumDistinct = 0
				case "negative":
					dist.NumDistinct = -1
				case "missing min":
					dist.Min = sdk.Dec{}
					suffix = ".min"
				case "missing max":
					dist.Max = sdk.Dec{}
					suffix = ".max"
				case "reversed":
					dist.Max = dist.Min.Sub(sdk.OneDec())
					suffix = ".max"
				}
				if err := config.Validate(); err == nil || !strings.Contains(err.Error(), key+suffix) {
					t.Fatalf("configuration validation did not report %s%s: %v", key, suffix, err)
				}
				m880ExpectConfigFailure(t, key+suffix, func() { run(&config) })
				if config.EVMAddresses != nil {
					t.Fatal("invalid distribution reached startup before rejection")
				}
				// Drive the JSON admission path as well, preserving missing fields.
				raw, err := json.Marshal(m880Config())
				if err != nil {
					t.Fatal(err)
				}
				var document map[string]json.RawMessage
				if err := json.Unmarshal(raw, &document); err != nil {
					t.Fatal(err)
				}
				var numeric map[string]json.RawMessage
				if err := json.Unmarshal(document[key], &numeric); err != nil {
					t.Fatal(err)
				}
				switch defect {
				case "zero":
					numeric["number_of_distinct_values"] = json.RawMessage("0")
				case "negative":
					numeric["number_of_distinct_values"] = json.RawMessage("-1")
				case "missing min":
					delete(numeric, "min")
				case "missing max":
					delete(numeric, "max")
				case "reversed":
					numeric["max"] = json.RawMessage(`"-1"`)
				}
				document[key], err = json.Marshal(numeric)
				if err != nil {
					t.Fatal(err)
				}
				raw, err = json.Marshal(document)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(t.TempDir(), "config.json")
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				m880ExpectConfigFailure(t, key+suffix, func() { ReadConfig(path) })
			})
		}
	}
	for _, messageType := range []string{Bank, EVM, ERC20, ERC721, Staking, WasmInstantiate} {
		if err := (Config{MessageTypes: []string{messageType}}).Validate(); err != nil {
			t.Fatalf("inactive distributions blocked %s: %v", messageType, err)
		}
	}
}

func TestRepairM_880_F027ValidDistributionSampling(t *testing.T) {
	config := m880Config()
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	config = ReadConfig(path)
	for _, dist := range []NumericDistribution{config.PriceDistr, config.QuantityDistr,
		{Min: sdk.NewDec(5), Max: sdk.NewDec(5), NumDistinct: 1},
		{Min: sdk.NewDec(-4), Max: sdk.NewDec(-2), NumDistinct: 2},
	} {
		if err := dist.Validate("sample"); err != nil {
			t.Fatal(err)
		}
		valid, invalid := make(map[string]bool), make(map[string]bool)
		for i := int64(0); i < dist.NumDistinct; i++ {
			step := dist.Max.Sub(dist.Min).QuoInt64(dist.NumDistinct).Mul(sdk.NewDec(i))
			valid[dist.Min.Add(step).String()] = true
			invalid[dist.Min.Add(step).String()] = true
			invalid[dist.Max.Add(step).String()] = true
		}
		for i := 0; i < 256; i++ {
			if sample := dist.Sample(); !valid[sample.String()] {
				t.Fatalf("valid sampler changed its lattice: %s", sample)
			}
			if sample := dist.InvalidSample(); !invalid[sample.String()] {
				t.Fatalf("invalid sampler changed its lattice: %s", sample)
			}
		}
	}
	// The admitted distributions feed the real message consumer and its coin encoding.
	config.PriceDistr = NumericDistribution{Min: sdk.NewDec(2), Max: sdk.NewDec(2), NumDistinct: 1}
	config.QuantityDistr = NumericDistribution{Min: sdk.NewDec(3), Max: sdk.NewDec(3), NumDistinct: 1}
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
	client := &LoadTestClient{LoadTestConfig: config}
	msgs, _, _, _, _ := client.generateMessage(secp256k1.GenPrivKey(), WasmMintNft)
	if len(msgs) != 1 {
		t.Fatalf("mint produced %d messages, want one", len(msgs))
	}
	msg, ok := msgs[0].(*wasmtypes.MsgExecuteContract)
	if !ok || !msg.Funds.IsEqual(sdk.NewCoins(sdk.NewInt64Coin("uhpx", 6))) {
		t.Fatalf("admitted price/quantity did not become the expected mint funds: %v", msgs[0])
	}
}
