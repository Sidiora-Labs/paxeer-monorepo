package shadow

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
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/tracers"
	_ "github.com/ethereum/go-ethereum/eth/tracers/native"
)

// These are execution inputs, not expected trace JSON. The production EVM and
// native prestateTracer produce every result served by the loopback peer.
var repairM837Reader = common.HexToAddress("0x8370")
var repairM837Sender = common.HexToAddress("0x100")
var repairM837Created = crypto.CreateAddress(repairM837Sender, 0)
var repairM837ReadCode = common.FromHex("0x600154506000545000")             // SLOAD 1, SLOAD 0, STOP; no SSTORE.
var repairM837InitCode = common.FromHex("0x6009600255600060005360016000f3") // SSTORE(2,9), return STOP.

const repairM837Gas = uint64(500000)

type repairM837Execution struct {
	Full         []tracers.TxTraceResult
	Diff         []tracers.TxTraceResult
	State        *state.StateDB
	Transactions []common.Hash
	Hash         common.Hash
}

func repairM837Produce(t *testing.T, failMode int) *repairM837Execution {
	t.Helper()
	out := &repairM837Execution{Hash: common.HexToHash("0x837")}
	for _, diff := range []bool{false, true} {
		db := state.NewDatabaseForTesting()
		t.Cleanup(func() {
			if err := db.TrieDB().Close(); err != nil {
				t.Error(err)
			}
		})
		store, err := state.New(types.EmptyRootHash, db)
		if err != nil {
			t.Fatal(err)
		}
		store.SetCode(repairM837Reader, repairM837ReadCode)
		store.SetState(repairM837Reader, common.HexToHash("0x1"), common.HexToHash("0x7"))
		var entries []tracers.TxTraceResult
		for index := 0; index < 2; index++ {
			cfg := &runtime.Config{Origin: repairM837Sender, State: store, GasLimit: repairM837Gas, BlockNumber: big.NewInt(1)}
			runtime.SetDefaults(cfg)
			tracer, err := tracers.DefaultDirectory.New("prestateTracer", &tracers.Context{}, repair328JSON(t, map[string]bool{"diffMode": diff}), cfg.ChainConfig)
			if err != nil {
				t.Fatal(err)
			}
			cfg.EVMConfig = vm.Config{Tracer: tracer.Hooks}
			// Exercise native tracer termination deterministically, without a timing race.
			if index == 1 && ((!diff && failMode == 0) || (diff && failMode == 1)) {
				tracer.Stop(errors.New("execution timeout"))
			}
			var tx *types.Transaction
			if index == 0 {
				tx = types.NewTx(&types.LegacyTx{To: &repairM837Reader, Value: new(big.Int), Gas: repairM837Gas})
				_, _, err = runtime.Call(repairM837Reader, nil, cfg)
			} else {
				tx = types.NewTx(&types.LegacyTx{Data: repairM837InitCode, Value: new(big.Int), Gas: repairM837Gas})
				var code []byte
				var addr common.Address
				code, addr, _, err = runtime.Create(repairM837InitCode, cfg)
				if addr != repairM837Created || !bytes.Equal(code, []byte{0}) {
					t.Fatalf("real CREATE result: address=%s code=%x", addr, code)
				}
			}
			if err != nil {
				t.Fatalf("EVM transaction %d: %v", index, err)
			}
			raw, traceErr := tracer.GetResult()
			entry := tracers.TxTraceResult{TxHash: tx.Hash()}
			if traceErr != nil {
				entry.Error = traceErr.Error()
			} else {
				entry.Result = raw
			}
			entries = append(entries, entry)
			if !diff {
				out.Transactions = append(out.Transactions, tx.Hash())
			}
		}
		if diff {
			out.Diff = entries
			out.State = store
		} else {
			out.Full = entries
		}
	}
	return out
}

type repairM837Peer struct {
	URL   string
	mu    sync.Mutex
	calls []string
}

func repairM837Serve(t *testing.T, execution *repairM837Execution, store *state.StateDB) *repairM837Peer {
	t.Helper()
	peer := new(repairM837Peer)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req repair328Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			http.Error(w, err.Error(), 400)
			return
		}
		peer.mu.Lock()
		defer peer.mu.Unlock()
		peer.calls = append(peer.calls, req.Method)
		param := func(i int) string {
			var value string
			if i >= len(req.Params) {
				t.Errorf("missing parameter %d: %s", i, req.Method)
				return ""
			}
			if err := json.Unmarshal(req.Params[i], &value); err != nil {
				t.Error(err)
			}
			return value
		}
		reject := func(message string) { t.Error(message); http.Error(w, message, 400) }
		var result any
		switch req.Method {
		case "eth_getBlockByNumber":
			if len(req.Params) != 2 || param(0) != "0x1" || string(req.Params[1]) != "false" {
				reject("invalid block inventory request")
				return
			}
			result = map[string]any{"hash": execution.Hash, "number": "0x1", "transactions": execution.Transactions}
		case "debug_traceBlockByNumber", "debug_traceBlockByHash":
			if len(req.Params) != 2 {
				reject("invalid trace parameters")
				return
			}
			if (req.Method == "debug_traceBlockByNumber" && param(0) != "0x1") || (req.Method == "debug_traceBlockByHash" && param(0) != execution.Hash.Hex()) {
				reject("trace block does not match inventory")
				return
			}
			var config struct {
				Tracer       string `json:"tracer"`
				TracerConfig struct {
					DiffMode       bool `json:"diffMode"`
					DisableCode    bool `json:"disableCode"`
					DisableStorage bool `json:"disableStorage"`
				} `json:"tracerConfig"`
			}
			if err := json.Unmarshal(req.Params[1], &config); err != nil {
				reject(err.Error())
				return
			}
			if config.Tracer != "prestateTracer" || config.TracerConfig.DisableCode || config.TracerConfig.DisableStorage {
				reject("required trace coverage disabled")
				return
			}
			if config.TracerConfig.DiffMode {
				result = execution.Diff
			} else {
				result = execution.Full
			}
		case "eth_getStorageAt", "eth_getCode", "eth_getBalance", "eth_getTransactionCount":
			index := 1
			if req.Method == "eth_getStorageAt" {
				index = 2
			}
			if len(req.Params) != index+1 || param(index) != "0x1" {
				reject("state read must use compared height")
				return
			}
			addr := common.HexToAddress(param(0))
			switch req.Method {
			case "eth_getStorageAt":
				result = store.GetState(addr, common.HexToHash(param(1)))
			case "eth_getCode":
				result = hexutil.Encode(store.GetCode(addr))
			case "eth_getBalance":
				result = hexutil.EncodeBig(store.GetBalance(addr).ToBig())
			case "eth_getTransactionCount":
				result = hexutil.EncodeUint64(store.GetNonce(addr))
			}
		default:
			reject("unexpected RPC method: " + req.Method)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	peer.URL = server.URL
	return peer
}

func repairM837Trace(t *testing.T, source *TraceKeySource, diff bool) []prestateTxTrace {
	t.Helper()
	var entries []prestateTxTrace
	if err := source.client.CallContext(context.Background(), &entries, "debug_traceBlockByNumber", "0x1", map[string]any{"tracer": "prestateTracer", "tracerConfig": map[string]bool{"diffMode": diff}}); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("producer returned %d entries, want 2", len(entries))
	}
	return entries
}

func TestRepairM_837_NativeProducerReadOnlyAndCreation(t *testing.T) {
	execution := repairM837Produce(t, -1)
	canonical := repairM837Serve(t, execution, execution.State)
	source := repair328Source(t, canonical.URL)
	full := repairM837Trace(t, source, false)
	diff := repairM837Trace(t, source, true)
	for i := range full {
		if full[i].TxHash != execution.Transactions[i] || diff[i].TxHash != execution.Transactions[i] || full[i].Error != "" || diff[i].Error != "" {
			t.Fatalf("trace identity/error: full=%+v diff=%+v", full[i], diff[i])
		}
	}
	var read map[common.Address]struct {
		Code    hexutil.Bytes               `json:"code"`
		Storage map[common.Hash]common.Hash `json:"storage"`
	}
	if err := json.Unmarshal(full[0].Result, &read); err != nil {
		t.Fatal(err)
	}
	account, ok := read[repairM837Reader]
	if !ok || !bytes.Equal(account.Code, repairM837ReadCode) || len(account.Storage) != 2 || account.Storage[common.Hash{}] != (common.Hash{}) || account.Storage[common.HexToHash("0x1")] != common.HexToHash("0x7") {
		t.Fatalf("native producer lost read-only code/zero/nonzero SLOAD: %+v", read)
	}
	var readDiff, createDiff prestateResult
	if err := json.Unmarshal(diff[0].Result, &readDiff); err != nil {
		t.Fatal(err)
	}
	if readDiff.Pre[repairM837Reader] != nil || readDiff.Post[repairM837Reader] != nil {
		t.Fatalf("native diff must omit unchanged read-only contract: %+v", readDiff)
	}
	if err := json.Unmarshal(diff[1].Result, &createDiff); err != nil {
		t.Fatal(err)
	}
	created := createDiff.Post[repairM837Created]
	if created == nil || created.Storage[common.HexToHash("0x2")] != common.HexToHash("0x9") {
		t.Fatalf("native CREATE missing poststate storage: %+v", createDiff)
	}
	if !bytes.Equal(execution.State.GetCode(repairM837Created), []byte{0}) || execution.State.GetNonce(repairM837Created) != 1 {
		t.Fatal("CREATE did not install runtime code and nonce")
	}
	touched, err := source.TouchedAccounts(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(touched) != 4 {
		t.Fatalf("touched %d accounts, want 4: %+v", len(touched), touched)
	}
	byAddr := make(map[common.Address]TouchedAccount)
	for _, account := range touched {
		if !account.CheckBalance || !account.CheckCode || !account.CheckNonce {
			t.Fatalf("lost touched account axes: %+v", account)
		}
		byAddr[account.Addr] = account
	}
	if !reflect.DeepEqual(byAddr[repairM837Reader].Slots, []common.Hash{{}, common.HexToHash("0x1")}) || !reflect.DeepEqual(byAddr[repairM837Created].Slots, []common.Hash{common.HexToHash("0x2")}) {
		t.Fatalf("lost read-only or created slots: %+v", touched)
	}
	reader := repair328Reader(t, canonical.URL)
	clean := repair328Comparison(t, context.Background(), reader, reader, source, 1)
	if !clean.Match || clean.Layer2.Indeterminate || clean.Layer2.AccountsChecked != 4 || clean.Layer2.KeysChecked != 15 || len(clean.Layer2.Divergences) != 0 {
		t.Fatalf("clean native trace comparison: %+v", clean)
	}
	for _, tc := range []struct {
		name, kind string
		addr       common.Address
		alter      func(*state.StateDB)
	}{
		{"read_only_storage", "storage", repairM837Reader, func(s *state.StateDB) { s.SetState(repairM837Reader, common.HexToHash("0x1"), common.HexToHash("0x8")) }},
		{"unchanged_code", "code", repairM837Reader, func(s *state.StateDB) {
			s.SetCode(repairM837Reader, append(append([]byte{}, repairM837ReadCode...), 0))
		}},
		{"created_code", "code", repairM837Created, func(s *state.StateDB) { s.SetCode(repairM837Created, []byte{0x60, 0}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			divergent, ok := execution.State.Copy().(*state.StateDB)
			if !ok {
				t.Fatal("production StateDB.Copy did not retain its concrete store type")
			}
			tc.alter(divergent)
			shadow := repairM837Serve(t, execution, divergent)
			result := repair328Comparison(t, context.Background(), repair328Reader(t, shadow.URL), reader, source, 1)
			if result.Match || result.DivergenceLayer == nil || *result.DivergenceLayer != 2 || result.Layer2.Indeterminate || result.Layer2.AccountsChecked != 4 || result.Layer2.KeysChecked != 15 || len(result.Layer2.Divergences) != 1 {
				t.Fatalf("native trace consumer missed %s: %+v", tc.name, result)
			}
			divergence := result.Layer2.Divergences[0]
			if divergence.Kind != tc.kind || divergence.Addr != tc.addr.Hex() || divergence.Shadow == divergence.Canonical {
				t.Fatalf("wrong state divergence: %+v", divergence)
			}
			if tc.kind == "storage" && divergence.Slot != common.HexToHash("0x1").Hex() {
				t.Fatalf("wrong read-only SLOAD slot: %+v", divergence)
			}
		})
	}
}

func TestRepairM_837_NativeTraceErrorFailsClosed(t *testing.T) {
	for mode := 0; mode < 2; mode++ {
		t.Run(fmt.Sprintf("diffMode=%t", mode == 1), func(t *testing.T) {
			execution := repairM837Produce(t, mode)
			peer := repairM837Serve(t, execution, execution.State)
			source := repair328Source(t, peer.URL)
			entries := repairM837Trace(t, source, mode == 1)
			if entries[0].Error != "" || len(entries[0].Result) == 0 || entries[1].TxHash != execution.Transactions[1] || entries[1].Error != "execution timeout" || len(entries[1].Result) != 0 {
				t.Fatalf("native partial trace failure: %+v", entries)
			}
			touched, err := source.TouchedAccounts(context.Background(), 1)
			if err == nil || touched != nil || !strings.Contains(err.Error(), execution.Transactions[1].Hex()) || !strings.Contains(err.Error(), "execution timeout") {
				t.Fatalf("partial trace escaped fail-closed source: touched=%+v err=%v", touched, err)
			}
			reader := repair328Reader(t, peer.URL)
			repair328Indeterminate(t, source, reader, 1, "execution timeout")
			peer.mu.Lock()
			defer peer.mu.Unlock()
			hashTraces := 0
			for _, method := range peer.calls {
				if method == "debug_traceBlockByHash" {
					hashTraces++
				}
				if strings.HasPrefix(method, "eth_get") && method != "eth_getBlockByNumber" {
					t.Fatalf("partial trace triggered state comparison: %s", method)
				}
			}
			want := 2
			if mode == 1 {
				want = 4
			}
			if hashTraces != want {
				t.Fatalf("hash-pinned trace calls=%d, want %d", hashTraces, want)
			}
		})
	}
}

func TestRepairM_837_RecordedNativeTraceConsumer(t *testing.T) {
	// This committed native-tracer transcript retains original producer results.
	// It is explicitly not labelled a capture of the paxd block RPC wrapper.
	canonical := repair328ChainFixture(t, "testdata/repair_328_recorded.json", false)
	shadow := repair328ChainFixture(t, "testdata/repair_328_recorded.json", true)
	source := repair328Source(t, canonical.URL)
	result := repair328Comparison(t, context.Background(), repair328Reader(t, shadow.URL), repair328Reader(t, canonical.URL), source, 1)
	if result.Match || result.Layer2.Indeterminate || result.Layer2.AccountsChecked != 4 || result.Layer2.KeysChecked != 16 || len(result.Layer2.Divergences) != 2 {
		t.Fatalf("recorded native producer comparison: %+v", result)
	}
	found := map[string]StateDivergence{}
	for _, divergence := range result.Layer2.Divergences {
		found[divergence.Kind] = divergence
	}
	if found["storage"].Addr != canonical.Contract.Hex() || found["storage"].Slot != common.HexToHash("0x1").Hex() || found["code"].Addr != canonical.Contract.Hex() {
		t.Fatalf("recorded native producer lost unchanged state: %+v", result.Layer2.Divergences)
	}
}

// Each exchange is an unmodified request/response captured from a source-built
// paxd. Replaying peers never generate trace JSON or execute a subprocess.
type repairM837Capture struct {
	Provenance struct {
		SourceRevision string `json:"source_revision"`
		Producer       string `json:"producer"`
		GenesisSource  string `json:"genesis_source"`
		GenesisSHA256  string `json:"genesis_sha256"`
		BinarySHA256   string `json:"binary_sha256"`
		ChainID        int    `json:"chain_id"`
	} `json:"provenance"`
	Reader   common.Address `json:"reader"`
	Created  common.Address `json:"created"`
	ReadTx   common.Hash    `json:"read_tx"`
	CreateTx common.Hash    `json:"create_tx"`
	Receipts []struct {
		BlockNumber     hexutil.Uint64 `json:"blockNumber"`
		TransactionHash common.Hash    `json:"transactionHash"`
		Status          string         `json:"status"`
	} `json:"receipts"`
	Transcript []struct {
		Request  repair328Request `json:"request"`
		Response struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Result  json.RawMessage `json:"result"`
		} `json:"response"`
	} `json:"transcript"`
}

func repairM837LoadCapture(t *testing.T) *repairM837Capture {
	t.Helper()
	raw, err := os.ReadFile("testdata/repair_m837_paxd.json")
	if err != nil {
		t.Fatal(err)
	}
	var capture repairM837Capture
	if err := json.Unmarshal(raw, &capture); err != nil {
		t.Fatal(err)
	}
	p := capture.Provenance
	if p.Producer != "paxd ./daemon/paxd" || p.GenesisSource != "services/platform/hosted/paxeer/init-chain.sh" || p.ChainID != 125 || len(p.SourceRevision) != 40 || len(p.GenesisSHA256) != 64 || len(p.BinarySHA256) != 64 || len(capture.Receipts) != 2 || len(capture.Transcript) == 0 {
		t.Fatalf("missing node capture provenance: %+v", capture)
	}
	for i, hash := range []common.Hash{capture.ReadTx, capture.CreateTx} {
		if hash == (common.Hash{}) || capture.Receipts[i].TransactionHash != hash || capture.Receipts[i].Status != "0x1" || capture.Receipts[i].BlockNumber == 0 {
			t.Fatalf("capture transaction %d has no successful receipt", i)
		}
	}
	return &capture
}

func repairM837Params(t *testing.T, raw []json.RawMessage) string {
	t.Helper()
	var params []any
	if err := json.Unmarshal(repair328JSON(t, raw), &params); err != nil {
		t.Fatal(err)
	}
	return string(repair328JSON(t, params))
}

func repairM837Replay(t *testing.T, capture *repairM837Capture, failMode int, alter string) *repairM837Peer {
	t.Helper()
	peer := new(repairM837Peer)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req repair328Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			http.Error(w, err.Error(), 400)
			return
		}
		peer.mu.Lock()
		peer.calls = append(peer.calls, req.Method)
		peer.mu.Unlock()
		params := req.Params
		if strings.HasPrefix(req.Method, "debug_traceBlock") && failMode >= 0 {
			var cfg map[string]any
			if len(params) != 2 || json.Unmarshal(params[1], &cfg) != nil {
				t.Error("invalid trace config")
				http.Error(w, "config", 400)
				return
			}
			tracerCfg, ok := cfg["tracerConfig"].(map[string]any)
			if !ok {
				t.Error("missing tracer config")
				http.Error(w, "config", 400)
				return
			}
			if tracerCfg["diffMode"] == (failMode == 1) {
				// Route the normal consumer request to a separately recorded node
				// response obtained with an expired tracer timeout.
				cfg["timeout"] = "-1ns"
				params = append([]json.RawMessage(nil), params...)
				params[1] = repair328JSON(t, cfg)
			}
		}
		method := req.Method
		// Error responses were captured by number; map the hash from the
		// recorded block inventory without changing the trace payload.
		if method == "debug_traceBlockByHash" && len(params) == 2 && strings.Contains(string(params[1]), "timeout") {
			for _, row := range capture.Transcript {
				if row.Request.Method != "eth_getBlockByNumber" {
					continue
				}
				var block struct {
					Hash common.Hash `json:"hash"`
				}
				if err := json.Unmarshal(row.Response.Result, &block); err != nil {
					t.Error(err)
					return
				}
				if string(params[0]) == string(repair328JSON(t, block.Hash)) {
					params[0] = row.Request.Params[0]
					method = "debug_traceBlockByNumber"
					break
				}
			}
		}
		for _, row := range capture.Transcript {
			if row.Request.Method != method || repairM837Params(t, row.Request.Params) != repairM837Params(t, params) {
				continue
			}
			response := row.Response
			response.ID = req.ID
			if len(req.Params) > 0 && string(req.Params[0]) == string(repair328JSON(t, capture.Reader)) {
				if alter == "storage" && method == "eth_getStorageAt" && string(req.Params[1]) == string(repair328JSON(t, common.HexToHash("0x1"))) {
					response.Result = repair328JSON(t, common.HexToHash("0x8"))
				}
				if alter == "code" && method == "eth_getCode" {
					response.Result = repair328JSON(t, hexutil.Encode(append(append([]byte{}, repairM837ReadCode...), 0)))
				}
			}
			if alter == "created_code" && method == "eth_getCode" && string(req.Params[0]) == string(repair328JSON(t, capture.Created)) {
				response.Result = repair328JSON(t, "0x6000")
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Error(err)
			}
			return
		}
		t.Errorf("RPC exchange absent from node capture: %s %s", method, repairM837Params(t, params))
		http.Error(w, "unrecorded exchange", 400)
	}))
	t.Cleanup(server.Close)
	peer.URL = server.URL
	return peer
}

func TestRepairM_837_RecordedPaxdReadOnlyAndCreation(t *testing.T) {
	capture := repairM837LoadCapture(t)
	canonical := repairM837Replay(t, capture, -1, "")
	source := repair328Source(t, canonical.URL)
	reader := repair328Reader(t, canonical.URL)
	for i, hash := range []common.Hash{capture.ReadTx, capture.CreateTx} {
		height := int64(capture.Receipts[i].BlockNumber)
		full, diff := []prestateTxTrace{}, []prestateTxTrace{}
		for _, mode := range []bool{false, true} {
			entries := &full
			if mode {
				entries = &diff
			}
			if err := source.client.CallContext(context.Background(), entries, "debug_traceBlockByNumber", hexutil.EncodeUint64(uint64(height)), map[string]any{"tracer": "prestateTracer", "tracerConfig": map[string]bool{"diffMode": mode, "disableCode": false, "disableStorage": false}}); err != nil {
				t.Fatal(err)
			}
		}
		index := -1
		for j, entry := range full {
			if entry.TxHash == hash {
				index = j
			}
		}
		if index < 0 || len(full) != len(diff) || diff[index].TxHash != hash || full[index].Error != "" || diff[index].Error != "" {
			t.Fatalf("capture lost transaction coverage: full=%+v diff=%+v", full, diff)
		}
		var pre map[common.Address]struct {
			Code    hexutil.Bytes               `json:"code"`
			Storage map[common.Hash]common.Hash `json:"storage"`
		}
		if err := json.Unmarshal(full[index].Result, &pre); err != nil {
			t.Fatal(err)
		}
		var changed struct {
			Pre  map[common.Address]json.RawMessage `json:"pre"`
			Post map[common.Address]struct {
				Code    hexutil.Bytes               `json:"code"`
				Storage map[common.Hash]common.Hash `json:"storage"`
			} `json:"post"`
		}
		if err := json.Unmarshal(diff[index].Result, &changed); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			account, ok := pre[capture.Reader]
			if !ok || !bytes.Equal(account.Code, repairM837ReadCode) || len(account.Storage) != 2 || account.Storage[common.Hash{}] != (common.Hash{}) || account.Storage[common.HexToHash("0x1")] != common.HexToHash("0x7") {
				t.Fatalf("paxd full trace lost read-only state: %+v", pre)
			}
			if changed.Pre[capture.Reader] != nil {
				t.Fatalf("paxd diff retained unchanged reader: %+v", changed)
			}
			if _, ok := changed.Post[capture.Reader]; ok {
				t.Fatal("paxd diff retained read-only poststate")
			}
		} else {
			account, ok := changed.Post[capture.Created]
			if !ok || !bytes.Equal(account.Code, []byte{0}) || account.Storage[common.HexToHash("0x2")] != common.HexToHash("0x9") {
				t.Fatalf("paxd CREATE lost code/storage: %+v", changed)
			}
		}
		touched, err := source.TouchedAccounts(context.Background(), height)
		if err != nil {
			t.Fatal(err)
		}
		target := capture.Reader
		wantSlots := []common.Hash{{}, common.HexToHash("0x1")}
		if i == 1 {
			target = capture.Created
			wantSlots = []common.Hash{common.HexToHash("0x2")}
		}
		found := false
		for _, account := range touched {
			if !account.CheckBalance || !account.CheckNonce || !account.CheckCode {
				t.Fatalf("missing account axes: %+v", account)
			}
			if account.Addr == target {
				found = true
				if !reflect.DeepEqual(account.Slots, wantSlots) {
					t.Fatalf("paxd slots: %+v", account)
				}
			}
		}
		if !found {
			t.Fatalf("paxd trace consumer omitted %s", target)
		}
		clean := repair328Comparison(t, context.Background(), reader, reader, source, height)
		if len(touched) != 4 || !clean.Match || clean.Layer2.Indeterminate || clean.Layer2.AccountsChecked != 4 || clean.Layer2.KeysChecked != 15 || len(clean.Layer2.Divergences) != 0 {
			t.Fatalf("captured node clean comparison: %+v touched=%+v", clean, touched)
		}
		axes := []string{"storage", "code"}
		if i == 1 {
			axes = []string{"created_code"}
		}
		for _, axis := range axes {
			t.Run(fmt.Sprintf("height=%d/%s", height, axis), func(t *testing.T) {
				shadow := repairM837Replay(t, capture, -1, axis)
				result := repair328Comparison(t, context.Background(), repair328Reader(t, shadow.URL), reader, source, height)
				if result.Match || result.DivergenceLayer == nil || *result.DivergenceLayer != 2 || result.Layer2.Indeterminate || len(result.Layer2.Divergences) != 1 || result.Layer2.AccountsChecked != clean.Layer2.AccountsChecked || result.Layer2.KeysChecked != clean.Layer2.KeysChecked {
					t.Fatalf("paxd capture consumer missed %s: %+v", axis, result)
				}
				d := result.Layer2.Divergences[0]
				kind := axis
				if axis == "created_code" {
					kind = "code"
				}
				if d.Kind != kind || d.Addr != target.Hex() || d.Shadow == d.Canonical {
					t.Fatalf("wrong captured state divergence: %+v", d)
				}
				if kind == "storage" && d.Slot != common.HexToHash("0x1").Hex() {
					t.Fatalf("wrong read-only slot: %+v", d)
				}
			})
		}
	}
}

func TestRepairM_837_RecordedPaxdTraceErrorFailsClosed(t *testing.T) {
	capture := repairM837LoadCapture(t)
	for mode := 0; mode < 2; mode++ {
		t.Run(fmt.Sprintf("diffMode=%t", mode == 1), func(t *testing.T) {
			peer := repairM837Replay(t, capture, mode, "")
			source := repair328Source(t, peer.URL)
			height := int64(capture.Receipts[0].BlockNumber)
			touched, err := source.TouchedAccounts(context.Background(), height)
			if err == nil || touched != nil || !strings.Contains(err.Error(), "execution timeout") || !strings.Contains(err.Error(), capture.ReadTx.Hex()) {
				t.Fatalf("captured paxd trace error escaped: touched=%+v err=%v", touched, err)
			}
			repair328Indeterminate(t, source, repair328Reader(t, peer.URL), height, "execution timeout")
			peer.mu.Lock()
			defer peer.mu.Unlock()
			for _, method := range peer.calls {
				if strings.HasPrefix(method, "eth_get") && method != "eth_getBlockByNumber" {
					t.Fatalf("failed node trace triggered state read: %s", method)
				}
			}
		})
	}
}
