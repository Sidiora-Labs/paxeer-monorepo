package chain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/readpolicy"
	"github.com/ethereum/go-ethereum"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/capabilities"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type snapshotRuntimeReader struct {
	header   *types.Header
	codes    map[common.Address][]byte
	storage  map[common.Address]map[common.Hash]common.Hash
	calls    map[string][]byte
	pool     common.Address
	mode     string
	endpoint string
}

func (b *snapshotRuntimeReader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     json.RawMessage
		Method string
		Params []json.RawMessage
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	var result any
	address := func(index int) common.Address {
		var raw string
		_ = json.Unmarshal(req.Params[index], &raw)
		return common.HexToAddress(raw)
	}
	switch req.Method {
	case "eth_chainId":
		result = "0x7d"
	case "eth_getBlockByNumber", "eth_getBlockByHash":
		result = reportedJSON(b.header)
	case "eth_getCode":
		result = hexutil.Bytes(b.codes[address(0)])
	case "eth_getStorageAt":
		var key string
		_ = json.Unmarshal(req.Params[1], &key)
		value := b.storage[address(0)][common.HexToHash(key)]
		result = hexutil.Bytes(value.Bytes())
	case "eth_getLogs":
		result = []any{}
	case "eth_getTransactionReceipt", "eth_getTransactionByHash":
		result = nil
	case "eth_call":
		var call struct {
			To    common.Address
			Data  hexutil.Bytes
			Input hexutil.Bytes
		}
		_ = json.Unmarshal(req.Params[0], &call)
		data := call.Input
		if len(data) == 0 {
			data = call.Data
		}
		word, ok := b.calls[common.Bytes2Hex(data)]
		if !ok {
			word = common.LeftPadBytes([]byte{1}, 32)
		}
		if call.To == b.pool && len(data) == 4 {
			switch b.mode {
			case "malformed amount":
				word = []byte{1}
			case "fee overflow":
				if common.Bytes2Hex(data) == common.Bytes2Hex(crypto.Keccak256([]byte("currentFeeBps()"))[:4]) {
					word = common.LeftPadBytes(big.NewInt(10001).Bytes(), 32)
				}
			}
		}
		result = hexutil.Bytes(word)
	default:
		http.Error(w, "unknown method", 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
}

func snapshotRuntimeFixture(t *testing.T, mode string) (*ReadAdapter, *snapshotRuntimeReader) {
	t.Helper()
	var artifacts struct {
		Contracts map[string]struct {
			Runtime    string
			Immutables map[string][]struct{ Start, Length int } `json:"immutableReferences"`
		}
	}
	raw, err := os.ReadFile("../config/deployment/testdata/evm-artifacts.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &artifacts); err != nil {
		t.Fatal(err)
	}
	runtime := func(name string, address common.Address) []byte {
		artifact := artifacts.Contracts[name]
		code := common.FromHex(artifact.Runtime)
		for _, refs := range artifact.Immutables {
			for _, ref := range refs {
				copy(code[ref.Start:ref.Start+ref.Length], common.LeftPadBytes(address.Bytes(), ref.Length))
			}
		}
		return code
	}
	manifest := deployment.Current()
	manifest.Status = "active"
	manifest.ActivationHeight = 1
	usdl := common.FromHex("0x6080604052600080fd")
	role := manifest.Roles["USDL"]
	role.RuntimeHash = crypto.Keccak256Hash(usdl).Hex()
	manifest.Roles["USDL"] = role
	b := &snapshotRuntimeReader{header: testHeader(100), codes: map[common.Address][]byte{}, storage: map[common.Address]map[common.Hash]common.Hash{}, calls: map[string][]byte{}, pool: common.HexToAddress("0x1111111111111111111111111111111111111111"), mode: mode}
	b.header.Time = uint64(time.Now().Unix())
	surface := common.FromHex("0x363d3d373360601b36523d3d601436013d3d6110175af13d600060003e6024573d6000fd5b3d6000f3")
	for name, role := range manifest.Roles {
		address, implementation := common.HexToAddress(role.Address), common.HexToAddress(role.ImplementationAddress)
		switch role.Kind {
		case "surface":
			b.codes[address] = surface
		case "erc1967":
			b.codes[address] = runtime("UUPSProxy", address)
			b.codes[implementation] = runtime(name, implementation)
			b.storage[address] = map[common.Hash]common.Hash{deployment.ImplementationSlot: common.BytesToHash(implementation.Bytes())}
		case "beacon":
			b.codes[address] = runtime("PoolBeacon", address)
			b.codes[implementation] = runtime("SidioraPool", implementation)
		case "direct":
			b.codes[address] = usdl
		}
	}
	for method, value := range map[string][]byte{
		"owner()":          common.LeftPadBytes(common.HexToAddress(manifest.Governance.ExpectedBeaconOwner).Bytes(), 32),
		"pendingOwner()":   make([]byte, 32),
		"implementation()": common.LeftPadBytes(common.HexToAddress(manifest.Roles["PoolBeacon"].ImplementationAddress).Bytes(), 32),
		"creationFee()":    common.LeftPadBytes(big.NewInt(1000000).Bytes(), 32),
		"usdlAddress()":    common.LeftPadBytes(common.HexToAddress(manifest.Roles["USDL"].Address).Bytes(), 32),
		"decimals()":       common.LeftPadBytes([]byte{6}, 32),
	} {
		b.calls[common.Bytes2Hex(crypto.Keccak256([]byte(method))[:4])] = value
	}
	b.codes[b.pool] = surface
	proxy := []byte{0x60, 0x01, 0x60, 0x00}
	switch mode {
	case "no code":
		delete(b.codes, b.pool)
	case "unidentified code":
		b.codes[b.pool] = []byte{0}
	case "proxy", "wrong beacon":
		b.codes[b.pool] = proxy
		b.storage[b.pool] = map[common.Hash]common.Hash{deployment.BeaconSlot: common.BytesToHash(common.HexToAddress(manifest.Roles["PoolBeacon"].Address).Bytes())}
		if mode == "wrong beacon" {
			b.storage[b.pool][deployment.BeaconSlot] = common.Hash{}
		}
	case "unregistered":
		registryABI := common.LeftPadBytes(b.pool.Bytes(), 32)
		selector := crypto.Keccak256([]byte("isRegisteredPool(address)"))[:4]
		b.calls[common.Bytes2Hex(append(selector, registryABI...))] = make([]byte, 32)
	}
	server := httptest.NewServer(b)
	b.endpoint = server.URL
	t.Cleanup(server.Close)
	hash, err := manifest.Hash()
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := NewReadAdapter(context.Background(), server.URL, "", ReadAdapterConfig{Manifest: manifest, ManifestHash: hash, CapabilityHash: capabilities.Frozen().SourceSHA256, InterfaceHash: strings.Repeat("a", 64), EndpointSetVersion: "e1", AnchorHeight: 100, AnchorHash: consensusHash(b.header), ConfigurationRevision: "snapshot", AttestationMaxAge: time.Minute, HistoricalConcurrency: 1, PoolProxyRuntimeHash: crypto.Keccak256Hash(proxy)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = adapter.Close() })
	return adapter, b
}

func TestAttestedSnapshotReadsBindPoolRuntimeAndAmounts(t *testing.T) {
	for _, mode := range []string{"surface", "proxy", "malformed amount", "fee overflow", "no code", "unidentified code", "wrong beacon", "unregistered"} {
		t.Run(mode, func(t *testing.T) {
			adapter, backend := snapshotRuntimeFixture(t, mode)
			pin, err := adapter.Pin(context.Background(), 100)
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := adapter.Snapshot(context.Background(), pin, backend.pool)
			if mode == "surface" || mode == "proxy" {
				if err != nil || snapshot.VirtualUSDLReserve != "1" || snapshot.RealUSDLBalance != "1" || snapshot.TokenReserve != "1" || snapshot.FeeBps != 1 || snapshot.BlockHash != pin.Hash || snapshot.ManifestHash != adapter.binding.ManifestHash {
					t.Fatalf("committed snapshot: %+v %v", snapshot, err)
				}
				var unavailable capabilities.Unavailable
				if err := adapter.ReleaseReady(); !errors.As(err, &unavailable) || unavailable.CapabilityID != "core_market" || unavailable.ReasonCode != "prerequisite_unavailable" {
					t.Fatalf("unavailable controller readiness: %v", err)
				}
				if _, err := adapter.HistoricalLogs(context.Background(), ethereum.FilterQuery{Addresses: []common.Address{backend.pool}, FromBlock: big.NewInt(100), ToBlock: big.NewInt(100)}, 1); err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatalf("invalid pool admitted: %+v", snapshot)
			}
		})
	}
}

func TestChainMeasurementRequiresNewDurableBlocks(t *testing.T) {
	ctx := context.Background()
	adapter, backend := snapshotRuntimeFixture(t, "surface")
	primary, err := NewChainMeasurementEndpoint(backend.endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer primary.Close()
	fallback, err := NewChainMeasurementEndpoint(backend.endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer fallback.Close()
	client, err := NewVerifiedClient(ctx, primary.URL, fallback.URL, VerifiedConfig{adapter.binding.Profile, 100, consensusHash(backend.header)})
	if err != nil {
		t.Fatal(err)
	}
	adapter.client.Close()
	adapter.client = client
	pool := newMeasurementDatabase(t)
	if _, err := pool.Exec(ctx, `CREATE SCHEMA indexer; CREATE TABLE indexer.indexer_ingest_watermarks_v1(chain_id bigint,stream_id text,raw_complete_height bigint,typed_complete_height bigint); INSERT INTO indexer.indexer_ingest_watermarks_v1 VALUES(125,'steady',100,100)`); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(binary)
	sha := hex.EncodeToString(digest[:])
	binding := readpolicy.MeasurementBinding{CandidateSHA256: sha, BuildSHA256: sha, InterfaceSHA256: adapter.binding.InterfaceHash, ManifestSHA256: adapter.binding.ManifestHash, TopologySHA256: sha, EnvironmentSHA256: sha, WorkloadSeed: "steady-source", ProfileVersion: "e1"}
	config := ChainMeasurementConfig{Binding: binding, OutputDirectory: filepath.Join(t.TempDir(), "capture"), StreamID: "steady", Duration: 10 * time.Second, Pool: backend.pool, LogAddresses: []common.Address{backend.pool}, MaxRows: 2000, MaxBytes: 2 << 20}
	run, err := RunChainMeasurement(ctx, adapter, pool, primary, fallback, config)
	if err == nil || run == nil || run.FailureCode != "MISSING_REAL_WORKLOAD" || run.HotKeyRequests != 1000 || run.HotKeyUpstreamGroups != 1 || run.EndpointLossRequests == 0 || run.IngestedBlocks != 0 {
		t.Fatalf("steady source measurement: %+v %v", run, err)
	}
	result, err := FinishChainMeasurement(run, ChainMeasurementEvidence{})
	if err == nil || result.LaneHarnessPassed || result.FailureCode != "MISSING_REAL_WORKLOAD" || result.RawRows <= 1000 {
		t.Fatalf("steady source capture: %+v %v", result, err)
	}
}
