package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// The endpoint forwards pinned reads to the deployed pool and records the
// methods that cross the runtime transport.
func pauseBoundedClient(t *testing.T, c *pauseChain) func() int {
	t.Helper()
	target, err := url.Parse(c.endpoint)
	if err != nil {
		t.Fatal(err)
	}
	forward := httputil.NewSingleHostReverseProxy(target)
	var mu sync.Mutex
	var methods []string
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var call struct {
			Method string
			Params []json.RawMessage
		}
		if err = json.Unmarshal(raw, &call); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		methods = append(methods, call.Method)
		mu.Unlock()
		if call.Method == "eth_call" && (len(call.Params) != 2 || !bytes.Contains(call.Params[1], []byte(`"blockHash"`)) || !bytes.Contains(call.Params[0], []byte(common.Bytes2Hex(pauseSelector)))) {
			t.Error("pause base requires a hash-pinned paused() call")
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		r.ContentLength = int64(len(raw))
		forward.ServeHTTP(w, r)
	}))
	t.Cleanup(endpoint.Close)
	client, closeClient, err := receipt.NewBoundedRPCClientWithArchive(c.ctx, endpoint.URL, "", receipt.ArchiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeClient)
	c.observation.rpc = client
	return func() int {
		mu.Lock()
		defer mu.Unlock()
		for _, method := range methods {
			switch method {
			case "eth_call", "eth_getStorageAt", "eth_chainId", "eth_blockNumber", "eth_getCode", "eth_getBlockByHash", "eth_getBlockByNumber", "eth_getBlockReceipts", "eth_getTransactionReceipt", "anvil_nodeInfo":
			default:
				t.Fatalf("runtime method %s", method)
			}
			if method != "eth_call" {
				t.Fatalf("pause observer method %s", method)
			}
		}
		return len(methods)
	}
}

func pauseCreatedEpoch(t *testing.T, c *pauseChain, address common.Address, created *types.Receipt) receipt.RPCCodeEpoch {
	t.Helper()
	for _, event := range created.Logs {
		if event.Address == c.factoryAddress && event.Topics[0] == c.factoryABI.Events["MarketCreated"].ID && common.BytesToAddress(event.Topics[2].Bytes()) == address {
			return receipt.RPCCodeEpoch{Deployment: decode.Deployment{Address: address, Role: "SidioraPool", From: decode.Order{Block: event.BlockNumber, Transaction: event.TxIndex}}}
		}
	}
	t.Fatal("canonical factory creation required")
	return receipt.RPCCodeEpoch{}
}

func TestPauseObservationDiscoveredPools(t *testing.T) {
	c := newPauseChain(t)
	address, pool, created := c.create(t, "Discovered pause")
	c.retain(t, created)
	c.coverRaw()
	epochs := []receipt.RPCCodeEpoch{pauseCreatedEpoch(t, c, address, created)}
	reads := 0
	c.observation.discoveredEpochs = func() []receipt.RPCCodeEpoch {
		reads++
		return epochs
	}
	calls := pauseBoundedClient(t, c)
	c.refresh(t)
	base := c.observation.snapshot.Load().base
	server := httptest.NewServer(c.observation)
	defer server.Close()
	assertPauseState(t, readPauseResponse(t, server, created), address, false)
	if calls() != 1 || reads != 1 {
		t.Fatal("initial base must read the pool once and refresh its discoveries")
	}

	paused := c.transact(t, pool, "pause")
	c.retain(t, paused)
	c.refresh(t)
	assertPauseState(t, readPauseResponse(t, server, paused), address, true)
	if snapshot := c.observation.snapshot.Load(); snapshot.base != base || len(snapshot.events) != 1 || snapshot.events[0].Topics[0] != pauseTopic {
		t.Fatal("discovered pool pause must come from stored events after the base")
	}
	if calls() != 1 || reads != 2 {
		t.Fatal("covered refresh must reread discoveries and use retained logs")
	}

	later, laterPool, laterCreated := c.create(t, "New discovered pause")
	laterPaused := c.transact(t, laterPool, "pause")
	c.retain(t, laterPaused)
	epochs = append(epochs, pauseCreatedEpoch(t, c, later, laterCreated))
	c.refresh(t)
	assertPauseState(t, readPauseResponse(t, server, laterCreated), later, false)
	assertPauseState(t, readPauseResponse(t, server, laterPaused), later, true)
	assertPauseState(t, readPauseResponse(t, server, created), address, false)
	if c.observation.snapshot.Load().base != base || calls() != 1 || reads != 3 {
		t.Fatal("new discoveries must cover pools created after the observer started")
	}
}

func TestPauseObservationWithoutBeaconCoverage(t *testing.T) {
	c := newPauseChain(t)
	address, pool, created := c.create(t, "Head pause base")
	c.retain(t, created)
	epoch := pauseCreatedEpoch(t, c, address, created)
	c.observation.discoveredEpochs = func() []receipt.RPCCodeEpoch { return []receipt.RPCCodeEpoch{epoch} }
	calls := pauseBoundedClient(t, c)
	c.refresh(t)
	base := c.observation.snapshot.Load().base
	paused := c.transact(t, pool, "pause")
	c.retain(t, paused)
	c.refresh(t)
	snapshot := c.observation.snapshot.Load()
	if snapshot.base == base || snapshot.base.block.height != paused.BlockNumber.Int64() || snapshot.base.block.hash != strings.ToLower(paused.BlockHash.Hex()) || len(snapshot.events) != 0 || calls() != 2 {
		t.Fatal("missing beacon coverage requires a fresh pinned base at the head without events")
	}
	server := httptest.NewServer(c.observation)
	defer server.Close()
	assertPauseState(t, readPauseResponse(t, server, paused), address, true)
	checkPauseUnavailable(t, server, created)
}

func TestPauseObservationDiscoveryCoverageBounds(t *testing.T) {
	address := common.HexToAddress("0x125")
	pool := pausePool{address: strings.ToLower(address.Hex()), height: 10, transaction: 2, log: 5}
	beacon := source.AddressEpoch{Role: "PoolBeacon", ActivationHeight: 1}
	observer := &pauseObservations{epochs: []source.AddressEpoch{beacon}}
	for _, test := range []struct {
		name    string
		role    string
		address common.Address
		from    decode.Order
		until   *decode.Order
		want    bool
	}{
		{"CreationTransaction", "SidioraPool", address, decode.Order{Block: 10, Transaction: 2}, nil, true},
		{"CreationLog", "SidioraPool", address, decode.Order{Block: 10, Transaction: 2, Log: 5}, nil, true},
		{"EarlierTransaction", "SidioraPool", address, decode.Order{Block: 10, Transaction: 1, Log: 99}, nil, true},
		{"EarlierBlock", "SidioraPool", address, decode.Order{Block: 9, Transaction: 99}, nil, true},
		{"LaterBlock", "SidioraPool", address, decode.Order{Block: 11}, nil, false},
		{"LaterTransaction", "SidioraPool", address, decode.Order{Block: 10, Transaction: 3}, nil, false},
		{"LaterLog", "SidioraPool", address, decode.Order{Block: 10, Transaction: 2, Log: 6}, nil, false},
		{"WrongRole", "SidioraERC20", address, decode.Order{Block: 10}, nil, false},
		{"WrongAddress", "SidioraPool", common.HexToAddress("0x126"), decode.Order{Block: 10}, nil, false},
		{"RetiredAtHead", "SidioraPool", address, decode.Order{Block: 10}, &decode.Order{Block: 20}, false},
		{"RetiredBeforeHead", "SidioraPool", address, decode.Order{Block: 10}, &decode.Order{Block: 19}, false},
		{"RetiredAfterHead", "SidioraPool", address, decode.Order{Block: 10}, &decode.Order{Block: 21}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			epoch := receipt.RPCCodeEpoch{Deployment: decode.Deployment{Address: test.address, Role: test.role, From: test.from, Until: test.until}}
			if got := observer.rawCoverage([]pausePool{pool}, []receipt.RPCCodeEpoch{epoch}, 9, 20); got != test.want {
				t.Fatalf("discovered coverage=%t want=%t", got, test.want)
			}
		})
	}
	if observer.rawCoverage([]pausePool{pool}, nil, 11, 20) {
		t.Fatal("missing discovered epoch cannot establish pool coverage")
	}
	observer.epochs = append(observer.epochs, source.AddressEpoch{Address: address.Hex(), Role: "SidioraPool", ActivationHeight: 1})
	if !observer.rawCoverage([]pausePool{pool}, nil, 11, 20) {
		t.Fatal("committed pool coverage must remain available")
	}
	observer.epochs = observer.epochs[1:]
	if observer.rawCoverage([]pausePool{pool}, nil, 11, 20) {
		t.Fatal("pool coverage cannot establish beacon coverage")
	}
}

func TestPauseObservationWithoutPoolCoverage(t *testing.T) {
	c := newPauseChain(t)
	address, pool, created := c.create(t, "Uncovered pause")
	c.retain(t, created)
	c.coverRaw()
	calls := pauseBoundedClient(t, c)
	c.refresh(t)
	paused := c.transact(t, pool, "pause")
	c.retain(t, paused)
	c.refresh(t)
	snapshot := c.observation.snapshot.Load()
	if snapshot.base.block.height != paused.BlockNumber.Int64() || len(snapshot.events) != 0 || calls() != 2 {
		t.Fatal("missing pool epoch requires a head base without events")
	}
	server := httptest.NewServer(c.observation)
	defer server.Close()
	assertPauseState(t, readPauseResponse(t, server, paused), address, true)
	checkPauseUnavailable(t, server, created)
}

func TestPauseObservationQualifiedDiscoveryReader(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	endpoint := marketRecoveryServe(t, chainfixture.New(t))
	options := marketRecoveryOptions(t)
	runtime := marketRecoveryOpen(t, ctx, endpoint, options, marketRecoveryInventory(t), options.Anchor.Height, options.Anchor.Hash)
	observer := &pauseObservations{discoveredEpochs: runtime.source.reserves.DiscoveredEpochs}
	if len(observer.discoveredEpochs()) != 0 {
		t.Fatal("source without an active authority must have no discoveries")
	}
	batch, qualified, err := runtime.source.FetchQualifiedBatch(ctx, 33963716, 33963716)
	if err != nil {
		t.Fatal(err)
	}
	if len(qualified) != 1 || len(batch.Blocks) != 1 {
		t.Fatal("canonical creation must be qualified")
	}
	var pool pausePool
	for _, raw := range batch.Records {
		if raw.Emitter == strings.ToLower(options.Manifest.Roles["SidioraFactory"].Address) && len(raw.Topics) == 4 && raw.Topics[0] == strings.ToLower(crypto.Keccak256Hash([]byte("MarketCreated(address,address,address,uint256,address)")).Hex()) {
			pool = pausePool{address: strings.ToLower(common.HexToAddress(raw.Topics[2]).Hex()), height: raw.Height, transaction: int64(raw.TransactionIndex), log: int64(raw.LogIndex)}
		}
	}
	if pool.address == "" {
		t.Fatal("qualified creation must name its pool")
	}
	for _, epoch := range options.Epochs {
		if strings.ToLower(epoch.Deployment.Address.Hex()) == pool.address {
			t.Fatal("created pool must be absent from configured epochs")
		}
	}
	observer.epochs = []source.AddressEpoch{{Address: options.Manifest.Roles["PoolBeacon"].Address, Role: "PoolBeacon", ActivationHeight: options.Manifest.ActivationHeight}}
	if !observer.rawCoverage([]pausePool{pool}, observer.discoveredEpochs(), pool.height, pool.height) {
		t.Fatal("active qualified source discovery must cover the canonical creation")
	}
}
