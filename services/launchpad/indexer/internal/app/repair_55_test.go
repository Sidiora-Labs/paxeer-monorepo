package app

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/chainreader"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/runtimedocs"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// The inventory peer stores ordered creation identities on disk. Every page
// opens and closes the snapshot, so recovery cannot reuse a process cache.
type repair55DiskInventory struct {
	path                         string
	pages, maxPage, maxArguments int
}

func (s *repair55DiskInventory) Page(ctx context.Context, addresses []common.Address, through uint64, after string) ([]receipt.DiscoveredMarket, error) {
	if len(addresses) == 0 || len(addresses) > marketRecoveryPageSize {
		return nil, errors.New("unbounded inventory lookup")
	}
	file, err := os.Open(s.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	selected := make(map[common.Address]bool, len(addresses))
	for _, address := range addresses {
		selected[address] = true
	}
	markets := make([]receipt.DiscoveredMarket, 0, marketRecoveryPageSize)
	decoder := json.NewDecoder(file)
	for len(markets) < marketRecoveryPageSize {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		var market receipt.DiscoveredMarket
		if err = decoder.Decode(&market); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		if strings.ToLower(market.Token.Hex()) > after && market.Height <= through && (selected[market.Token] || selected[market.Pool]) {
			validated, err := persistedMarket(strings.ToLower(market.Token.Hex()), strings.ToLower(market.Pool.Hex()), int64(market.Height), int64(market.TransactionIndex))
			if err != nil {
				return nil, err
			}
			markets = append(markets, validated)
		}
	}
	s.pages++
	s.maxPage = max(s.maxPage, len(markets))
	s.maxArguments = max(s.maxArguments, len(addresses))
	return markets, nil
}

func repair55Inventory(t *testing.T) *repair55DiskInventory {
	t.Helper()
	s := &repair55DiskInventory{path: filepath.Join(t.TempDir(), "canonical.jsonl")}
	if err := os.WriteFile(s.path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	return s
}

func (s *repair55DiskInventory) insert(ctx context.Context, markets []receipt.DiscoveredMarket) error {
	file, err := os.Open(s.path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(file)
	all := append([]receipt.DiscoveredMarket{}, markets...)
	for {
		var market receipt.DiscoveredMarket
		if err = decoder.Decode(&market); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			file.Close()
			return err
		}
		all = append(all, market)
	}
	if err = file.Close(); err != nil {
		return err
	}
	sort.Slice(all, func(i, j int) bool { return bytes.Compare(all[i].Token[:], all[j].Token[:]) < 0 })
	pools := map[common.Address]bool{}
	var snapshot bytes.Buffer
	encoder := json.NewEncoder(&snapshot)
	for i, market := range all {
		if err = ctx.Err(); err != nil {
			return err
		}
		if pools[market.Pool] || i > 0 && all[i-1].Token == market.Token {
			return errors.New("duplicate durable market identity")
		}
		pools[market.Pool] = true
		if err = encoder.Encode(market); err != nil {
			return err
		}
	}
	if err = os.WriteFile(s.path+".next", snapshot.Bytes(), 0600); err != nil {
		return err
	}
	return os.Rename(s.path+".next", s.path)
}

// Supplement the committed producer transcript with exact historical eth_call
// responses recorded for this recovery path. Unknown calls still fail through
// chainfixture; the proxy never connects to a live endpoint.
func repair55Serve(t *testing.T, fixture *chainfixture.Fixture) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	transcript, err := os.Open(filepath.Join(filepath.Dir(file), "testdata/repair_55_rpc.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer transcript.Close()
	entries := map[string]json.RawMessage{}
	scanner := bufio.NewScanner(transcript)
	for scanner.Scan() {
		var entry struct {
			Key    string          `json:"k"`
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil || entry.Key == "" || len(entry.Result) == 0 {
			t.Fatalf("invalid supplemental recording: %s: %v", scanner.Text(), err)
		}
		if _, exists := entries[entry.Key]; exists {
			t.Fatalf("duplicate supplemental recording: %s", entry.Key)
		}
		entries[entry.Key] = entry.Result
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	upstream, err := url.Parse(fixture.Serve(t))
	if err != nil {
		t.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			http.Error(w, "unreadable recorded request", http.StatusBadRequest)
			return
		}
		r.Body.Close()
		var call struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(body, &call); err != nil {
			t.Error(err)
			http.Error(w, "invalid recorded request", http.StatusBadRequest)
			return
		}
		key := chainfixture.Key(call.Method, call.Params)
		if result, exists := entries[key]; exists {
			if fixture.Mutate != nil {
				result = fixture.Mutate(key, result)
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": call.ID, "result": result}); err != nil {
				t.Error(err)
			}
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func repair55Markets(count int) []receipt.DiscoveredMarket {
	markets := make([]receipt.DiscoveredMarket, count)
	for i := range markets {
		markets[i] = receipt.DiscoveredMarket{Token: common.HexToAddress(fmt.Sprintf("0x%040x", 2*i+100000)), Pool: common.HexToAddress(fmt.Sprintf("0x%040x", 2*i+100001)), Height: 33963716, TransactionIndex: uint(i % 7)}
	}
	return markets
}

func TestRepair_55_DurableInventoryCrossesLifetimeBounds(t *testing.T) {
	ctx := context.Background()
	inventory := repair55Inventory(t)
	// 4,200 markets imply 8,400 historical code epochs. Recovery admits only
	// the markets of each operation; neither old lifetime boundary applies.
	want := repair55Markets(4200)
	if err := inventory.insert(ctx, want); err != nil {
		t.Fatal(err)
	}
	for restart := 0; restart < 2; restart++ {
		inventory = &repair55DiskInventory{path: inventory.path}
		for start := 0; start < len(want); start += 128 {
			end := min(start+128, len(want))
			addresses := make([]common.Address, 0, 2*(end-start))
			for _, market := range want[start:end] {
				addresses = append(addresses, market.Token, market.Pool)
			}
			got, err := recoverBatchMarkets(ctx, inventory, addresses, 33963716)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want[start:end]) {
				t.Fatalf("restart %d lost identities at %d: got %d want %d", restart, start, len(got), end-start)
			}
		}
		if inventory.maxPage > 128 || inventory.maxArguments > 128 || inventory.pages < len(want)/128 {
			t.Fatalf("unbounded or absent paging: %+v", inventory)
		}
	}
	addresses := make([]common.Address, len(want))
	for i, m := range want {
		addresses[i] = m.Token
	}
	if _, err := recoverBatchMarkets(ctx, inventory, addresses, 33963716); !errors.Is(err, receipt.ErrQualification) {
		t.Fatalf("dense batch admission limit lost: %v", err)
	}
	if got, err := recoverBatchMarkets(ctx, inventory, addresses[:1], 33963715); err != nil || len(got) != 0 {
		t.Fatalf("future creation recovered: %v %+v", err, got)
	}
}

func repair55Options(t *testing.T) receipt.RPCOptions {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
	manifest, ok := deployment.Approved(32226215)
	if !ok {
		t.Fatal("approved fixture generation missing")
	}
	reviewed, err := os.ReadFile(filepath.Join(root, "sdk/launchpad/protocol/catalog/reviewed.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Use the real producer's recorded deployed bytecode. No compiler artifact
	// or live node is necessary to reconstruct the immutable-masked template.
	fixtureFile, err := os.Open(filepath.Join(filepath.Dir(file), "../internaltest/chainfixture/testdata/paxeer125-market.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer fixtureFile.Close()
	z, err := gzip.NewReader(fixtureFile)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	scanner := bufio.NewScanner(z)
	scanner.Buffer(make([]byte, 64<<10), 64<<20)
	var code []byte
	for scanner.Scan() {
		var record struct {
			Key    string          `json:"k"`
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(record.Key, `eth_getCode ["0x0f92fdd087c96dd3413ef98646b94d0d986192fa",`) {
			var encoded string
			if err := json.Unmarshal(record.Result, &encoded); err != nil {
				t.Fatal(err)
			}
			code = common.FromHex(encoded)
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(code) == 0 {
		t.Fatal("recorded deployed token code missing")
	}
	o, _, err := runtimedocs.ReceiptOptions(runtimedocs.Inputs{Manifest: manifest, ReviewedCatalog: reviewed, ERC20RuntimeCode: code, PoolRuntimeHash: common.HexToHash("0xa9b304d6d0aa562f283b45978b84ced8840a9331f8ac946742670b1b02a2872d"), Anchor: receipt.BlockAnchor{Height: 33963715, Hash: common.HexToHash("0x82477110c5fc791f6bef5a7d96636dc457e83daf59c8b802e264bada02f13395")}})
	if err != nil {
		t.Fatal(err)
	}
	return o
}

type repair55Runtime struct {
	source      *recoveringQualifiedSource
	client      *ethclient.Client
	closeClient func()
	authorities []*receipt.RPCQualification
	height      uint64
	hashes      map[uint64]common.Hash
}

func repair55Open(t *testing.T, ctx context.Context, url string, options receipt.RPCOptions, inventory marketInventory, height uint64, hash common.Hash) *repair55Runtime {
	t.Helper()
	client, closeClient, err := receipt.NewBoundedRPCClient(ctx, url, "")
	if err != nil {
		t.Fatal(err)
	}
	committed, err := runtimedocs.CommittedOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	committed.Endpoints[runtimedocs.PrimaryEndpointID] = url
	rpcSource, err := source.NewCommittedSource(ctx, committed)
	if err != nil {
		closeClient()
		t.Fatal(err)
	}
	r := &repair55Runtime{client: client, closeClient: closeClient, height: height, hashes: map[uint64]common.Hash{height: hash}}
	factory := func(c context.Context) (*receipt.RPCQualification, error) {
		a, err := receipt.NewRPCQualification(c, client, options)
		if err != nil {
			return nil, err
		}
		if err = a.ConfigureArchiveReads(1, -1); err != nil {
			return nil, err
		}
		r.authorities = append(r.authorities, a)
		return a, nil
	}
	r.source = &recoveringQualifiedSource{inventory: inventory, client: client, newAuthority: factory, history: &source.HistoricalSource{RPC: rpcSource}, durableHeight: func() uint64 { return r.height }, blockIdentity: options.BlockIdentity, priorHash: func(_ context.Context, h uint64) (common.Hash, error) {
		v, ok := r.hashes[h]
		if !ok {
			return common.Hash{}, errors.New("durable prior hash unavailable")
		}
		return v, nil
	}, reserves: &recoveringReserveReader{inventory: inventory, newAuthority: factory}}
	t.Cleanup(r.close)
	return r
}

func (r *repair55Runtime) close() {
	if r.source != nil {
		_ = r.source.history.Close()
		r.source = nil
	}
	if r.closeClient != nil {
		r.closeClient()
		r.closeClient = nil
	}
}

func TestRepair_55_RestartRecoversRealMarketBeyondLifetimeBounds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if os.Getenv(chainfixture.RecordEnv) != "" {
		t.Fatal("this gate must use recorded producer traffic without a live endpoint")
	}
	url := repair55Serve(t, chainfixture.New(t))
	options := repair55Options(t)
	inventory := repair55Inventory(t)
	r := repair55Open(t, ctx, url, options, inventory, options.Anchor.Height, options.Anchor.Hash)
	created, qualified, err := r.source.FetchQualifiedBatch(ctx, 33963716, 33963716)
	if err != nil {
		t.Fatal(err)
	}
	if len(qualified) != 1 || len(created.Blocks) != 1 {
		t.Fatal("real producer creation was not qualified")
	}
	logs, err := qualified[0].SourceLogs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txHash, _, _, err := qualified[0].SourceContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	deployments := make([]decode.Deployment, 0, len(options.Epochs)+2)
	for _, epoch := range options.Epochs {
		deployments = append(deployments, epoch.Deployment)
	}
	for _, epoch := range r.authorities[0].DiscoveredEpochs() {
		deployments = append(deployments, epoch.Deployment)
	}
	registry, err := decode.NewRegistry(options.CatalogJSON, deployments)
	if err != nil {
		t.Fatal(err)
	}
	var native *types.Log
	for i := range logs {
		envelope, err := registry.Decode(logs[i])
		if err != nil {
			t.Fatal(err)
		}
		if envelope.EventName == "MarketCreated" && envelope.EmitterRole == "SidioraFactory" {
			native = &logs[i]
		}
	}
	if native == nil {
		t.Fatal("native factory creation missing")
	}
	// Resolve the identity through the actual receipt codec, then persist the
	// output consumed by range recovery, rather than substituting an RPC answer.
	creationReceipt, err := r.client.TransactionReceipt(ctx, common.HexToHash(txHash))
	if err != nil {
		t.Fatal(err)
	}
	identity, err := chainreader.CreationFromReceipt(registry, creationReceipt, common.HexToHash(txHash))
	if err != nil {
		t.Fatal(err)
	}
	market := receipt.DiscoveredMarket{Token: identity.Token, Pool: identity.Pool, Height: native.BlockNumber, TransactionIndex: native.TxIndex}
	pagesBeforeReserve := inventory.pages
	if _, err = r.source.reserves.PoolReserves(ctx, market.Pool, common.HexToHash("0x50f4e42d7247aa1665d1aaebc33039c8fc8e493727a864085ba77910dda56f6b")); err != nil {
		t.Fatalf("new market reserves before canonical commit: %v", err)
	}
	if inventory.pages != pagesBeforeReserve {
		t.Fatal("uncommitted batch market required a durable creation")
	}
	if err = inventory.insert(ctx, []receipt.DiscoveredMarket{market}); err != nil {
		t.Fatal(err)
	}
	// Capture the producer's complete approval/sell interval before crossing
	// either lifetime bound. issueBatch deduplicates logs by transaction hash;
	// the approval is a distinct receipt and must not be discarded as a replay.
	beforeGrowth := *r.source
	beforeGrowth.historical = true
	beforeGrowth.reserves = nil
	originalTrades, originalReceipts, err := beforeGrowth.FetchQualifiedBatch(ctx, 33963734, 33963749)
	if err != nil {
		t.Fatal(err)
	}
	originalIdentities := repair55TradeIdentities(t, ctx, originalTrades, originalReceipts)
	if err = inventory.insert(ctx, repair55Markets(4200)); err != nil {
		t.Fatal(err)
	}
	creationHash := common.HexToHash(created.Blocks[0].Hash)
	r.close()
	// Discard every authority, decoder, source and inventory reader; only the
	// durable inventory and recorded chain survive this component restart.
	inventory = &repair55DiskInventory{path: inventory.path}
	r = repair55Open(t, ctx, url, options, inventory, 33963716, creationHash)
	buy, receipts, err := r.source.FetchQualifiedBatch(ctx, 33963717, 33963733)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 1 {
		t.Fatalf("recovered buy receipt count %d", len(receipts))
	}
	fills, err := receipts[0].Fills(ctx)
	if err != nil || len(fills) != 1 || fills[0].Side != "buy" {
		t.Fatalf("recovered real buy: %v %+v", err, fills)
	}
	if err = receipt.ValidateBindings(ctx, buy, receipts); err != nil {
		t.Fatal(err)
	}
	if len(r.authorities) != 1 || len(r.authorities[0].DiscoveredEpochs()) != 2 {
		t.Fatal("authority loaded lifetime inventory instead of the active market")
	}
	reserves, err := r.source.reserves.PoolReserves(ctx, market.Pool, common.HexToHash(buy.Blocks[len(buy.Blocks)-1].Hash))
	if err != nil || reserves.Price == nil || reserves.Price.Sign() <= 0 {
		t.Fatalf("active-batch reserves: %v %+v", err, reserves)
	}
	r.height = 33963733
	r.hashes[r.height] = common.HexToHash(buy.Blocks[len(buy.Blocks)-1].Hash)
	approval, approvals, err := r.source.FetchQualifiedBatch(ctx, 33963734, 33963740)
	if err != nil {
		t.Fatal(err)
	}
	if len(approvals) != 1 {
		t.Fatalf("recovered approval receipt count %d", len(approvals))
	}
	approvalHash, _, _, err := approvals[0].SourceContext(ctx)
	if err != nil || approvalHash != "0x107f2ccf01805e2b0bf9cac5240a5ba9066056618fee03514c6ffecd2d11c8a6" {
		t.Fatalf("recovered approval identity: %v %s", err, approvalHash)
	}
	approvalLogs, err := approvals[0].SourceLogs(ctx)
	if err != nil || len(approvalLogs) != 1 || approvalLogs[0].Address != market.Token || approvalLogs[0].BlockNumber != 33963740 || len(approvalLogs[0].Topics) != 3 || approvalLogs[0].Topics[0] != common.HexToHash("0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925") {
		t.Fatalf("recovered token approval: %v %+v", err, approvalLogs)
	}
	approvalFills, err := approvals[0].Fills(ctx)
	if err != nil || len(approvalFills) != 0 {
		t.Fatalf("approval produced economic fills: %v %+v", err, approvalFills)
	}
	if err = receipt.ValidateBindings(ctx, approval, approvals); err != nil {
		t.Fatal(err)
	}
	if len(r.authorities) != 2 || r.authorities[0] == r.authorities[1] || len(r.authorities[1].DiscoveredEpochs()) != 2 {
		t.Fatal("approval authority retained prior authority state")
	}
	r.height = 33963740
	r.hashes[r.height] = common.HexToHash(approval.Blocks[len(approval.Blocks)-1].Hash)
	sell, receipts, err := r.source.FetchQualifiedBatch(ctx, 33963741, 33963749)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 1 {
		t.Fatalf("recovered sell receipt count %d", len(receipts))
	}
	fills, err = receipts[0].Fills(ctx)
	if err != nil || len(fills) != 1 || fills[0].Side != "sell" {
		t.Fatalf("recovered real sell: %v %+v", err, fills)
	}
	if err = receipt.ValidateBindings(ctx, sell, receipts); err != nil {
		t.Fatal(err)
	}
	if len(r.authorities) != 3 || r.authorities[0] == r.authorities[2] || r.authorities[1] == r.authorities[2] || len(r.authorities[2].DiscoveredEpochs()) != 2 {
		t.Fatal("range authority retained prior authority state")
	}
	// A reserve consumer reopened independently of ingestion recovers the same
	// historical binding from the durable inventory.
	reader := &recoveringReserveReader{inventory: inventory, newAuthority: r.source.newAuthority}
	if _, err = reader.PoolReserves(ctx, market.Pool, common.HexToHash(sell.Blocks[len(sell.Blocks)-1].Hash)); err != nil {
		t.Fatal(err)
	}
	historical := *r.source
	historical.historical = true
	historical.reserves = nil
	replayedTrades, replayedReceipts, err := historical.FetchQualifiedBatch(ctx, 33963734, 33963749)
	if err != nil {
		t.Fatal(err)
	}
	replayedIdentities := repair55TradeIdentities(t, ctx, replayedTrades, replayedReceipts)
	if !reflect.DeepEqual(originalTrades, replayedTrades) || !reflect.DeepEqual(originalIdentities, replayedIdentities) {
		t.Fatal("receipt identities or canonical records changed across lifetime growth and restart")
	}
	old, oldReceipts, err := historical.FetchQualifiedBatch(ctx, 33963716, 33963716)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(old.Records, created.Records) || len(oldReceipts) != len(qualified) {
		t.Fatal("historical source identities changed after restart")
	}
	if inventory.maxPage > 128 || inventory.maxArguments > 128 {
		t.Fatal("recovery did not bound inventory work")
	}
}

func repair55TradeIdentities(t *testing.T, ctx context.Context, batch store.CanonicalBatch, receipts []*receipt.QualifiedReceipt) []string {
	t.Helper()
	if err := receipt.ValidateBindings(ctx, batch, receipts); err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 2 {
		t.Fatalf("producer approval/sell interval returned %d receipts, want exactly 2", len(receipts))
	}
	want := map[string]bool{
		"0x107f2ccf01805e2b0bf9cac5240a5ba9066056618fee03514c6ffecd2d11c8a6": true,
		"0xf5f9ed1136e71f03ffd44ae7f80ee192998b80fccd96ad3f75824fa9ef3c2845": true,
	}
	identities := make([]string, 0, len(receipts))
	sells := 0
	for _, q := range receipts {
		identity, _, _, err := q.SourceContext(ctx)
		if err != nil || !want[identity] {
			t.Fatalf("duplicate or unexpected producer receipt: %s: %v", identity, err)
		}
		delete(want, identity)
		identities = append(identities, identity)
		fills, err := q.Fills(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, fill := range fills {
			if fill.Side != "sell" || fill.Position.TransactionHash != identity {
				t.Fatalf("unexpected recovered fill: %+v", fill)
			}
			sells++
		}
	}
	if len(want) != 0 || sells != 1 {
		t.Fatalf("recovery changed producer count: missing %v, sell fills %d, want exactly 1", want, sells)
	}
	sort.Strings(identities)
	return identities
}

func TestRepair_55_ReceiptHintsRejectTampering(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if os.Getenv(chainfixture.RecordEnv) != "" {
		t.Fatal("offline gate required")
	}
	fixture := chainfixture.New(t)
	var changed atomic.Bool
	fixture.Mutate = func(key string, result json.RawMessage) json.RawMessage {
		if !strings.HasPrefix(key, "eth_getBlockReceipts ") || !strings.Contains(key, "0x81a18d822e327fbcbfe2be91b7f666bfa043016cb0e3f094c5bbabe91f9d192b") {
			return result
		}
		var receipts []*types.Receipt
		if json.Unmarshal(result, &receipts) != nil || len(receipts) == 0 || len(receipts[0].Logs) == 0 {
			t.Error("recorded receipt unavailable")
			return result
		}
		receipts[0].Logs[0].Address = common.HexToAddress("0xdeadbeef")
		altered, err := json.Marshal(receipts)
		if err != nil {
			t.Error(err)
			return result
		}
		changed.Store(true)
		return altered
	}
	url := repair55Serve(t, fixture)
	options := repair55Options(t)
	inventory := repair55Inventory(t)
	r := repair55Open(t, ctx, url, options, inventory, options.Anchor.Height, options.Anchor.Hash)
	if _, _, err := r.source.FetchQualifiedBatch(ctx, 33963716, 33963716); !errors.Is(err, receipt.ErrBinding) {
		t.Fatalf("altered receipt became a recovery hint: %v", err)
	}
	if !changed.Load() || inventory.pages != 0 {
		t.Fatal("unverified hints reached durable inventory")
	}
}

func TestRepair_55_PersistedIdentityValidation(t *testing.T) {
	valid := repair55Markets(1)[0]
	for _, test := range []struct {
		token, pool   string
		height, index int64
	}{
		{"bad", valid.Pool.Hex(), 1, 0},
		{valid.Token.Hex(), "bad", 1, 0},
		{valid.Token.Hex(), valid.Pool.Hex(), -1, 0},
		{valid.Token.Hex(), valid.Pool.Hex(), 1, -1},
		{valid.Token.Hex(), valid.Pool.Hex(), 1, math.MaxUint32 + 1},
		{common.Address{}.Hex(), valid.Pool.Hex(), 1, 0},
		{valid.Token.Hex(), valid.Token.Hex(), 1, 0},
	} {
		if _, err := persistedMarket(test.token, test.pool, test.height, test.index); err == nil {
			t.Fatalf("invalid durable identity admitted: %+v", test)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := recoverBatchMarkets(ctx, repair55Inventory(t), []common.Address{valid.Token}, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled recovery continued: %v", err)
	}
}
