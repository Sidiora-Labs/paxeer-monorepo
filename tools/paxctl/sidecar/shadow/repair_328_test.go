package shadow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type repair328Node struct {
	URL          string         `json:"url"`
	Blocks       []common.Hash  `json:"blocks"`
	Transactions []common.Hash  `json:"transactions"`
	Contract     common.Address `json:"contract"`
	Created      common.Address `json:"created"`
}

// repair328Chain serves a committed peer fixture. TraceKeySource, JSON-RPC
// decoding, ethclient state reads and the Comparator are all production code.
func repair328Chain(t *testing.T, divergent bool) *repair328Node {
	t.Helper()
	return repair328ChainFixture(t, "testdata/repair_328_rpc.json", divergent)
}

func repair328ChainFixture(t *testing.T, path string, divergent bool) *repair328Node {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Blocks map[string]json.RawMessage `json:"blocks"`
		Full   json.RawMessage            `json:"full"`
		Diff   json.RawMessage            `json:"diff"`
		Errors json.RawMessage            `json:"errors"`
		State  map[common.Address]struct {
			Balance string                 `json:"balance"`
			Code    string                 `json:"code"`
			Nonce   string                 `json:"nonce"`
			Storage map[common.Hash]string `json:"storage"`
		} `json:"state"`
		Node repair328Node `json:"node"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	n := &fixture.Node
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request repair328Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, err.Error(), 400)
			return
		}
		reject := func(message string) {
			t.Error(message)
			http.Error(w, message, 400)
		}
		param := func(i int) string {
			var value string
			if i >= len(request.Params) {
				return ""
			}
			if err := json.Unmarshal(request.Params[i], &value); err != nil {
				t.Error(err)
			}
			return value
		}
		var result any
		switch request.Method {
		case "eth_getBlockByNumber":
			if len(request.Params) != 2 || string(request.Params[1]) != "false" {
				reject("block inventory must request transaction hashes")
				return
			}
			block, ok := fixture.Blocks[param(0)]
			if !ok {
				result = nil
			} else {
				result = block
			}
		case "debug_traceBlockByHash", "debug_traceBlockByNumber":
			if len(request.Params) != 2 {
				reject("invalid trace parameters")
				return
			}
			var config struct {
				Tracer       string `json:"tracer"`
				Timeout      string `json:"timeout"`
				TracerConfig struct {
					DiffMode       bool `json:"diffMode"`
					DisableCode    bool `json:"disableCode"`
					DisableStorage bool `json:"disableStorage"`
				} `json:"tracerConfig"`
			}
			if err := json.Unmarshal(request.Params[1], &config); err != nil {
				reject(err.Error())
				return
			}
			if config.Tracer != "prestateTracer" || config.TracerConfig.DisableCode || config.TracerConfig.DisableStorage {
				reject("trace request disabled a required coverage axis")
				return
			}
			block := param(0)
			if request.Method == "debug_traceBlockByHash" {
				if block == n.Blocks[0].Hex() {
					block = "0x1"
				} else if block == n.Blocks[1].Hex() {
					block = "0x2"
				} else {
					reject("trace was not pinned to the inventory block hash")
					return
				}
			}
			if block == "0x2" {
				result = json.RawMessage("[]")
			} else if block != "0x1" {
				reject("unexpected trace block")
				return
			} else if config.Timeout == "-1ns" {
				result = fixture.Errors
			} else if config.TracerConfig.DiffMode {
				result = fixture.Diff
			} else {
				result = fixture.Full
			}
		case "eth_getStorageAt", "eth_getCode", "eth_getBalance", "eth_getTransactionCount":
			heightIndex := 1
			if request.Method == "eth_getStorageAt" {
				heightIndex = 2
			}
			if len(request.Params) != heightIndex+1 || (param(heightIndex) != "0x1" && param(heightIndex) != "0x2") {
				reject("state read was not pinned to the compared height")
				return
			}
			addr := common.HexToAddress(param(0))
			account, ok := fixture.State[addr]
			if !ok {
				reject("unexpected state account")
				return
			}
			switch request.Method {
			case "eth_getStorageAt":
				slot := common.HexToHash(param(1))
				value, ok := account.Storage[slot]
				if !ok {
					reject("unexpected storage key")
					return
				}
				if divergent && addr == n.Contract && slot == common.HexToHash("0x1") {
					value = common.HexToHash("0x8").Hex()
				}
				result = value
			case "eth_getCode":
				result = account.Code
				if divergent && addr == n.Contract {
					result = account.Code + "00"
				}
			case "eth_getBalance":
				result = account.Balance
			case "eth_getTransactionCount":
				result = account.Nonce
			}
		default:
			reject("unexpected RPC method: " + request.Method)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	n.URL = server.URL
	return n
}

type repair328Request struct {
	Version string            `json:"jsonrpc"`
	ID      json.RawMessage   `json:"id"`
	Method  string            `json:"method"`
	Params  []json.RawMessage `json:"params"`
}

// Faults damage the peer request or response over real HTTP; the production
// source must reject incomplete coverage in either trace mode.
func repair328HTTP(t *testing.T, n *repair328Node, before func(*repair328Request), after func(string, bool, json.RawMessage) json.RawMessage) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request repair328Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, err.Error(), 500)
			return
		}
		if before != nil {
			before(&request)
		}
		body, err := json.Marshal(request)
		if err != nil {
			t.Error(err)
			http.Error(w, err.Error(), 500)
			return
		}
		upstream, err := http.NewRequestWithContext(r.Context(), http.MethodPost, n.URL, bytes.NewReader(body))
		if err != nil {
			t.Error(err)
			http.Error(w, err.Error(), 500)
			return
		}
		upstream.Header.Set("Content-Type", "application/json")
		responseFromNode, err := http.DefaultClient.Do(upstream)
		if err != nil {
			t.Error(err)
			http.Error(w, err.Error(), 500)
			return
		}
		defer responseFromNode.Body.Close()
		var response map[string]json.RawMessage
		if err := json.NewDecoder(responseFromNode.Body).Decode(&response); err != nil {
			t.Error(err)
			http.Error(w, err.Error(), 500)
			return
		}
		diff := false
		if strings.HasPrefix(request.Method, "debug_traceBlockBy") && len(request.Params) == 2 {
			var config struct {
				TracerConfig struct {
					DiffMode bool `json:"diffMode"`
				} `json:"tracerConfig"`
			}
			if err := json.Unmarshal(request.Params[1], &config); err != nil {
				t.Error(err)
			}
			diff = config.TracerConfig.DiffMode
		}
		if after != nil {
			response["result"] = after(request.Method, diff, response["result"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(responseFromNode.StatusCode)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func repair328Reader(t *testing.T, url string) *ethclient.Client {
	t.Helper()
	reader, err := ethclient.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reader.Close)
	return reader
}

func repair328Source(t *testing.T, url string) *TraceKeySource {
	t.Helper()
	source, err := NewTraceKeySource(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(source.Close)
	return source
}

func repair328Comparison(t *testing.T, ctx context.Context, shadow, canonical StateReader, source KeySource, height int64) *CompareResult {
	t.Helper()
	// The Comet peer supplies identical headers/receipts. Migration mode invokes
	// the real Layer 2 path even when the inexpensive layers agree.
	comet := rpcServer("AABB", "CCDD", nil)
	t.Cleanup(comet.Close)
	comparator := NewComparator(comet.URL, comet.URL, WithMigrationMode(), WithLayer2(shadow, canonical, source))
	result, err := comparator.CompareBlock(ctx, height)
	if err != nil {
		t.Fatal(err)
	}
	if result.Layer2 == nil {
		t.Fatal("comparison did not execute Layer 2")
	}
	return result
}

func repair328Indeterminate(t *testing.T, source *TraceKeySource, reader StateReader, height int64, want string) {
	t.Helper()
	comparison := repair328Comparison(t, context.Background(), reader, reader, source, height)
	result := comparison.Layer2
	if comparison.Match || comparison.DivergenceLayer == nil || *comparison.DivergenceLayer != 2 {
		t.Fatalf("trace failure must reject the block at Layer 2: %+v", comparison)
	}
	if !result.Indeterminate || !strings.Contains(result.Error, want) || result.AccountsChecked != 0 || result.KeysChecked != 0 || len(result.Divergences) != 0 {
		t.Fatalf("comparison should be indeterminate (%q), got %+v", want, result)
	}
	markdown := RenderMarkdown(&DivergenceReport{Comparison: CompareResult{Layer2: result}})
	if !strings.Contains(markdown, "**Indeterminate**") || !strings.Contains(markdown, result.Error) {
		t.Fatalf("consumer report lost trace failure: %s", markdown)
	}
}

func TestRepair_328_TraceErrorsDiscarded(t *testing.T) {
	n := repair328Chain(t, false)
	reader := repair328Reader(t, repair328HTTP(t, n, nil, nil))
	for _, failDiff := range []bool{false, true} {
		t.Run(fmt.Sprintf("diffMode=%t", failDiff), func(t *testing.T) {
			observed := make(chan []prestateTxTrace, 2)
			url := repair328HTTP(t, n, func(request *repair328Request) {
				if !strings.HasPrefix(request.Method, "debug_traceBlockBy") {
					return
				}
				var config map[string]any
				if err := json.Unmarshal(request.Params[1], &config); err != nil {
					t.Error(err)
					return
				}
				if config["tracerConfig"].(map[string]any)["diffMode"] == failDiff {
					config["timeout"] = "-1ns"
					request.Params[1], _ = json.Marshal(config)
				}
			}, func(method string, diff bool, raw json.RawMessage) json.RawMessage {
				if strings.HasPrefix(method, "debug_traceBlockBy") && diff == failDiff {
					var traces []prestateTxTrace
					if err := json.Unmarshal(raw, &traces); err != nil {
						t.Error(err)
					}
					observed <- traces
				}
				return raw
			})
			repair328Indeterminate(t, repair328Source(t, url), reader, 1, "execution timeout")
			select {
			case traces := <-observed:
				if len(traces) != 2 {
					t.Fatalf("producer trace count = %d", len(traces))
				}
				for i, trace := range traces {
					if trace.TxHash != n.Transactions[i] || trace.Error != "execution timeout" || trace.Result != nil {
						t.Fatalf("peer producer did not emit the expected per-transaction error: %+v", trace)
					}
				}
			default:
				t.Fatal("comparison did not reach the debug RPC peer")
			}
		})
	}
}

func TestRepair_328_PartialTraceFailure(t *testing.T) {
	n := repair328Chain(t, false)
	reader := repair328Reader(t, n.URL)
	for _, diffMode := range []bool{false, true} {
		for _, index := range []int{0, 1} {
			t.Run(fmt.Sprintf("diffMode=%t/transaction=%d", diffMode, index), func(t *testing.T) {
				url := repair328HTTP(t, n, nil, func(method string, diff bool, raw json.RawMessage) json.RawMessage {
					if method != "debug_traceBlockByHash" || diff != diffMode {
						return raw
					}
					var traces []map[string]json.RawMessage
					if err := json.Unmarshal(raw, &traces); err != nil {
						t.Error(err)
						return raw
					}
					traces[index]["error"] = json.RawMessage(`"execution timeout"`)
					return repair328JSON(t, traces)
				})
				repair328Indeterminate(t, repair328Source(t, url), reader, 1, "execution timeout")
			})
		}
	}
}

func TestRepair_328_TraceCoverage(t *testing.T) {
	n := repair328Chain(t, false)
	reader := repair328Reader(t, repair328HTTP(t, n, nil, nil))
	cases := []struct {
		name  string
		want  string
		alter func([]map[string]json.RawMessage) json.RawMessage
	}{
		{"missing_transaction", "trace coverage", func(traces []map[string]json.RawMessage) json.RawMessage { return repair328JSON(t, traces[:1]) }},
		{"extra_transaction", "trace coverage", func(traces []map[string]json.RawMessage) json.RawMessage {
			return repair328JSON(t, append(traces, traces[0]))
		}},
		{"duplicate_hash", "trace coverage", func(traces []map[string]json.RawMessage) json.RawMessage {
			traces[1]["txHash"] = traces[0]["txHash"]
			return repair328JSON(t, traces)
		}},
		{"missing_hash", "trace coverage", func(traces []map[string]json.RawMessage) json.RawMessage {
			delete(traces[0], "txHash")
			return repair328JSON(t, traces)
		}},
		{"wrong_hash", "trace coverage", func(traces []map[string]json.RawMessage) json.RawMessage {
			traces[0]["txHash"] = repair328JSON(t, n.Blocks[0])
			return repair328JSON(t, traces)
		}},
		{"missing_result", "missing or invalid prestate result", func(traces []map[string]json.RawMessage) json.RawMessage {
			delete(traces[0], "result")
			return repair328JSON(t, traces)
		}},
		{"null_result", "missing or invalid prestate result", func(traces []map[string]json.RawMessage) json.RawMessage {
			traces[0]["result"] = json.RawMessage("null")
			return repair328JSON(t, traces)
		}},
		{"empty_result", "missing or invalid prestate result", func(traces []map[string]json.RawMessage) json.RawMessage {
			traces[0]["result"] = json.RawMessage("{}")
			return repair328JSON(t, traces)
		}},
		{"array_result", "missing or invalid prestate result", func(traces []map[string]json.RawMessage) json.RawMessage {
			traces[0]["result"] = json.RawMessage("[]")
			return repair328JSON(t, traces)
		}},
		{"null_account", "missing account", func(traces []map[string]json.RawMessage) json.RawMessage {
			var result map[string]json.RawMessage
			if err := json.Unmarshal(traces[0]["result"], &result); err != nil {
				t.Error(err)
			}
			if pre, ok := result["pre"]; ok {
				var accounts map[string]json.RawMessage
				if err := json.Unmarshal(pre, &accounts); err != nil {
					t.Error(err)
				}
				accounts[n.Contract.Hex()] = json.RawMessage("null")
				result["pre"] = repair328JSON(t, accounts)
			} else {
				result[n.Contract.Hex()] = json.RawMessage("null")
			}
			traces[0]["result"] = repair328JSON(t, result)
			return repair328JSON(t, traces)
		}},
		{"null_entry", "trace coverage", func(traces []map[string]json.RawMessage) json.RawMessage {
			traces[0] = nil
			return repair328JSON(t, traces)
		}},
		{"null_list", "trace coverage", func([]map[string]json.RawMessage) json.RawMessage { return json.RawMessage("null") }},
		{"empty_list", "trace coverage", func([]map[string]json.RawMessage) json.RawMessage { return json.RawMessage("[]") }},
	}
	for _, diffMode := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("diffMode=%t/%s", diffMode, tc.name), func(t *testing.T) {
				url := repair328HTTP(t, n, nil, func(method string, diff bool, raw json.RawMessage) json.RawMessage {
					if !strings.HasPrefix(method, "debug_traceBlockBy") || diff != diffMode {
						return raw
					}
					var traces []map[string]json.RawMessage
					if err := json.Unmarshal(raw, &traces); err != nil {
						t.Error(err)
						return raw
					}
					return tc.alter(traces)
				})
				repair328Indeterminate(t, repair328Source(t, url), reader, 1, tc.want)
			})
		}
	}
	for _, field := range []string{"pre", "post"} {
		t.Run("missing_diff_"+field, func(t *testing.T) {
			url := repair328HTTP(t, n, nil, func(method string, diff bool, raw json.RawMessage) json.RawMessage {
				if !strings.HasPrefix(method, "debug_traceBlockBy") || !diff {
					return raw
				}
				var traces []map[string]json.RawMessage
				var result map[string]json.RawMessage
				if err := json.Unmarshal(raw, &traces); err != nil {
					t.Error(err)
					return raw
				}
				if err := json.Unmarshal(traces[0]["result"], &result); err != nil {
					t.Error(err)
					return raw
				}
				delete(result, field)
				traces[0]["result"] = repair328JSON(t, result)
				return repair328JSON(t, traces)
			})
			repair328Indeterminate(t, repair328Source(t, url), reader, 1, "missing or invalid prestate result")
		})
	}
	url := repair328HTTP(t, n, nil, nil)
	source := repair328Source(t, url)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	touched, err := source.TouchedAccounts(ctx, 2)
	if err != nil || len(touched) != 0 {
		t.Fatalf("empty block: touched=%+v error=%v", touched, err)
	}
	cleanComparison := repair328Comparison(t, ctx, reader, reader, source, 2)
	clean := cleanComparison.Layer2
	if !cleanComparison.Match || clean.Indeterminate || clean.AccountsChecked != 0 || clean.KeysChecked != 0 || len(clean.Divergences) != 0 {
		t.Fatalf("empty block should compare cleanly: %+v", clean)
	}
	repair328Indeterminate(t, source, reader, 3, "missing or invalid EVM block")
	repair328Indeterminate(t, source, reader, -1, "invalid trace height")
}

func repair328JSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Error(err)
	}
	return raw
}

func TestRepair_328_DiffTraceCoverage(t *testing.T) {
	canonical := repair328Chain(t, false)
	shadow := repair328Chain(t, true)
	canonicalURL := repair328HTTP(t, canonical, nil, nil)
	shadowURL := repair328HTTP(t, shadow, nil, nil)
	canonicalReader := repair328Reader(t, canonicalURL)
	shadowReader := repair328Reader(t, shadowURL)
	source := repair328Source(t, canonicalURL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Establish the producer behavior that made diff-only enumeration incomplete.
	var diff []prestateTxTrace
	if err := source.client.CallContext(ctx, &diff, "debug_traceBlockByNumber", "0x1", map[string]any{
		"tracer": "prestateTracer", "tracerConfig": map[string]any{"diffMode": true},
	}); err != nil {
		t.Fatal(err)
	}
	if len(diff) != 2 {
		t.Fatalf("producer returned %d traces", len(diff))
	}
	var readOnlyDiff, creationDiff prestateResult
	if err := json.Unmarshal(diff[0].Result, &readOnlyDiff); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(diff[1].Result, &creationDiff); err != nil {
		t.Fatal(err)
	}
	if readOnlyDiff.Pre[canonical.Contract] != nil || readOnlyDiff.Post[canonical.Contract] != nil {
		t.Fatal("diff fixture unexpectedly retained the unchanged read-only contract")
	}
	if creationDiff.Post[canonical.Created] == nil || creationDiff.Post[canonical.Created].Storage[common.HexToHash("0x2")] != common.HexToHash("0x9") {
		t.Fatal("diff fixture did not publish constructor storage")
	}
	touched, err := source.TouchedAccounts(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	byAddr := make(map[common.Address]TouchedAccount)
	for i, account := range touched {
		if !account.CheckCode || !account.CheckBalance || !account.CheckNonce {
			t.Fatalf("incomplete account axes: %+v", account)
		}
		if i > 0 && bytes.Compare(touched[i-1].Addr[:], account.Addr[:]) >= 0 {
			t.Fatal("account output is not sorted and unique")
		}
		for j := 1; j < len(account.Slots); j++ {
			if bytes.Compare(account.Slots[j-1][:], account.Slots[j][:]) >= 0 {
				t.Fatal("slot output is not sorted and unique")
			}
		}
		byAddr[account.Addr] = account
	}
	read := byAddr[canonical.Contract]
	if len(read.Slots) != 2 || read.Slots[0] != (common.Hash{}) || read.Slots[1] != common.HexToHash("0x1") {
		t.Fatalf("full prestate lost existing or zero read-only SLOAD: %+v", read)
	}
	created := byAddr[canonical.Created]
	if len(created.Slots) != 1 || created.Slots[0] != common.HexToHash("0x2") {
		t.Fatalf("diff poststate lost created storage: %+v", created)
	}
	comparison := repair328Comparison(t, ctx, shadowReader, canonicalReader, source, 1)
	result := comparison.Layer2
	if comparison.Match || comparison.DivergenceLayer == nil || *comparison.DivergenceLayer != 2 {
		t.Fatalf("read-only state divergence must reject the block at Layer 2: %+v", comparison)
	}
	if result.Indeterminate || result.AccountsChecked != 4 || result.KeysChecked != 15 || len(result.Divergences) != 2 {
		t.Fatalf("RPC state comparison lost touched axes: %+v", result)
	}
	found := make(map[string]StateDivergence)
	for _, divergence := range result.Divergences {
		found[divergence.Kind] = divergence
	}
	if found["storage"].Addr != canonical.Contract.Hex() || found["storage"].Slot != common.HexToHash("0x1").Hex() ||
		found["code"].Addr != canonical.Contract.Hex() {
		t.Fatalf("consumer missed read-only storage or unchanged contract code: %+v", result.Divergences)
	}
	cleanComparison := repair328Comparison(t, ctx, canonicalReader, canonicalReader, source, 1)
	clean := cleanComparison.Layer2
	if !cleanComparison.Match || clean.Indeterminate || clean.AccountsChecked != 4 || clean.KeysChecked != 15 || len(clean.Divergences) != 0 {
		t.Fatalf("identical RPC state did not compare cleanly: %+v", clean)
	}
}

// The native tracer's committed full/diff vectors are copied verbatim as
// results into this fixture. This independently covers the wire contract with
// producer data, including three unchanged SLOAD slots omitted by diff mode.
func TestRepair_328_RecordedTraceContract(t *testing.T) {
	canonical := repair328ChainFixture(t, "testdata/repair_328_recorded.json", false)
	shadow := repair328ChainFixture(t, "testdata/repair_328_recorded.json", true)
	source := repair328Source(t, canonical.URL)
	canonicalReader := repair328Reader(t, canonical.URL)
	shadowReader := repair328Reader(t, shadow.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	touched, err := source.TouchedAccounts(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(touched) != 4 {
		t.Fatalf("recorded account coverage = %d, want 4", len(touched))
	}
	var contract *TouchedAccount
	for i := range touched {
		if touched[i].Addr == canonical.Contract {
			contract = &touched[i]
		}
	}
	if contract == nil || !contract.CheckCode || !contract.CheckNonce || !contract.CheckBalance || len(contract.Slots) != 4 {
		t.Fatalf("recorded full prestate lost account axes or SLOAD keys: %+v", contract)
	}
	for i, slot := range contract.Slots {
		if slot != common.BigToHash(big.NewInt(int64(i))) {
			t.Fatalf("recorded SLOAD key %d = %s", i, slot)
		}
	}
	result := repair328Comparison(t, ctx, shadowReader, canonicalReader, source, 1)
	if result.Match || result.Layer2.Indeterminate || result.Layer2.AccountsChecked != 4 || result.Layer2.KeysChecked != 16 || len(result.Layer2.Divergences) != 2 {
		t.Fatalf("recorded trace comparison: %+v", result)
	}
	found := map[string]StateDivergence{}
	for _, divergence := range result.Layer2.Divergences {
		found[divergence.Kind] = divergence
	}
	if found["storage"].Slot != common.HexToHash("0x1").Hex() || found["storage"].Addr != canonical.Contract.Hex() || found["code"].Addr != canonical.Contract.Hex() {
		t.Fatalf("recorded trace lost unchanged storage/code: %+v", result.Layer2.Divergences)
	}
	clean := repair328Comparison(t, ctx, canonicalReader, canonicalReader, source, 1)
	if !clean.Match || clean.Layer2.Indeterminate || clean.Layer2.AccountsChecked != 4 || clean.Layer2.KeysChecked != 16 || len(clean.Layer2.Divergences) != 0 {
		t.Fatalf("recorded state must compare cleanly to itself: %+v", clean)
	}
}
