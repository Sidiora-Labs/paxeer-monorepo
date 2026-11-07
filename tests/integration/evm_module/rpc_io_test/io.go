package rpc_io_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	reqPrefix       = ">>"
	respPrefix      = "<<"
	directivePrefix = "@"
)

const rpcCallTimeout = 30 * time.Second

type rpcClient struct {
	URL    string
	Client *http.Client

	once   sync.Once
	client *http.Client
}

type binding struct {
	Var  string
	Path string
}

// ioxPair is one request/response pair from a .io or .iox file
type ioxPair struct {
	Request       []byte
	Expected      []byte
	AfterBindings []binding
	RefPair       int // 1-based; 0 = no ref check

	// ExpectBodyContains: each substring must appear in the response body (UTF-8).
	ExpectBodyContains []string

	// ExpectResponseHeaders: each name must be present on the HTTP response (case-insensitive).
	ExpectResponseHeaders []string
}

// parseIOFile parses .io/.iox content. Markers are >>, <<, and @; optional ASCII whitespace after
// each marker is trimmed away with the rest of the payload (TrimSpace).
// Supports ">> request", "<< expected", "@ bind var = path",
// "<< @ ref_pair N", "@ expect_body_contains substring", "@ expect_response_header Header-Name".
func parseIOFile(content string) ([]ioxPair, error) {
	var pairs []ioxPair
	var curReq []byte
	pending := false
	requestLine := 0
	lineNumber := 0
	lastIdx := -1
	inBinding := false

	for line := range strings.SplitSeq(content, "\n") {
		lineNumber++
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if trimmed == "" {
			continue
		}

		if after, ok := strings.CutPrefix(trimmed, reqPrefix); ok {
			if pending {
				return nil, fmt.Errorf("line %d: request overwrites unmatched request from line %d", lineNumber, requestLine)
			}
			pending = true
			requestLine = lineNumber
			curReq = []byte(strings.TrimSpace(after))
			if len(curReq) == 0 {
				return nil, fmt.Errorf("line %d: empty request", lineNumber)
			}
			inBinding = false
			continue
		}

		if after, ok := strings.CutPrefix(trimmed, respPrefix); ok {
			rest := strings.TrimSpace(after)
			if !pending {
				return nil, fmt.Errorf("line %d: response without request", lineNumber)
			}
			pending = false
			if after, ok := strings.CutPrefix(rest, directivePrefix); ok {
				rest = strings.TrimSpace(after)
				var n int
				if _, err := fmt.Sscanf(rest, "ref_pair %d", &n); err == nil && n >= 1 {
					pairs = append(pairs, ioxPair{Request: curReq, RefPair: n})
					lastIdx = len(pairs) - 1
					inBinding = true
				} else {
					return nil, fmt.Errorf("invalid directive: expected ref_pair N (N>=1), got %q", trimmed)
				}
				curReq = nil
				continue
			}

			pairs = append(pairs, ioxPair{Request: curReq, Expected: []byte(rest)})
			lastIdx = len(pairs) - 1
			inBinding = true
			curReq = nil
			continue
		}

		if inBinding && lastIdx >= 0 && strings.HasPrefix(trimmed, directivePrefix) {
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, directivePrefix))
			if after, ok := strings.CutPrefix(rest, "bind "); ok {
				idx := strings.Index(after, "=")
				if idx <= 0 {
					return nil, fmt.Errorf("invalid bind directive: expected varName = path, got %q", trimmed)
				}
				varName := strings.TrimSpace(after[:idx])
				path := strings.TrimSpace(after[idx+1:])
				if varName == "" || path == "" {
					return nil, fmt.Errorf("invalid bind directive: var and path must be non-empty, got %q", trimmed)
				}
				pairs[lastIdx].AfterBindings = append(pairs[lastIdx].AfterBindings, binding{Var: varName, Path: path})
				continue
			}
			if after, ok := strings.CutPrefix(rest, "expect_body_contains "); ok {
				sub := strings.TrimSpace(after)
				if sub == "" {
					return nil, fmt.Errorf("expect_body_contains needs non-empty substring: %q", trimmed)
				}
				pairs[lastIdx].ExpectBodyContains = append(pairs[lastIdx].ExpectBodyContains, sub)
				continue
			}
			if after, ok := strings.CutPrefix(rest, "expect_response_header "); ok {
				name := strings.TrimSpace(after)
				if name == "" {
					return nil, fmt.Errorf("expect_response_header needs non-empty header name: %q", trimmed)
				}
				pairs[lastIdx].ExpectResponseHeaders = append(pairs[lastIdx].ExpectResponseHeaders, name)
				continue
			}
			return nil, fmt.Errorf("unknown directive: %q", trimmed)
		}
	}
	if pending {
		return nil, fmt.Errorf("EOF: unmatched request from line %d", requestLine)
	}
	return pairs, nil
}

// assertPairBodyDirectives checks optional @ expect_body_contains rules for one .io pair.
func assertPairBodyDirectives(t *testing.T, pair ioxPair, body []byte) {
	t.Helper()
	for _, sub := range pair.ExpectBodyContains {
		if !strings.Contains(string(body), sub) {
			t.Fatalf("expected response body to contain %q", sub)
		}
	}
}

func assertPairHeaderDirectives(t *testing.T, pair ioxPair, hdr http.Header) {
	t.Helper()
	for _, name := range pair.ExpectResponseHeaders {
		if hdr.Get(name) == "" {
			t.Fatalf("expected response header %q to be set (got %v)", name, hdr)
		}
	}
}

func (c *rpcClient) call(req []byte) ([]byte, int, http.Header, error) {
	resp, err := c.httpClient().Post(c.URL, "application/json", bytes.NewReader(req))
	if err != nil {
		return nil, 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	hcopy := resp.Header.Clone()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, resp.StatusCode, hcopy, err
	}
	return buf.Bytes(), resp.StatusCode, hcopy, nil
}

func (c *rpcClient) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	c.once.Do(func() {
		c.client = &http.Client{Timeout: rpcCallTimeout}
	})
	return c.client
}

func getJSONPath(body []byte, path string) (any, bool) {
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, false
	}
	current := root
	for p := range strings.SplitSeq(path, ".") {
		if p == "" {
			continue
		}
		if m, ok := current.(map[string]any); ok {
			next, ok := m[p]
			if !ok {
				return nil, false
			}
			current = next
			continue
		}
		if arr, ok := current.([]any); ok {
			var idx int
			if _, err := fmt.Sscanf(p, "%d", &idx); err != nil || idx < 0 || idx >= len(arr) {
				return nil, false
			}
			current = arr[idx]
			continue
		}
		return nil, false
	}
	return current, true
}

func requestPlaceholders(request []byte) []string {
	var out []string
	s := string(request)
	for {
		start := strings.Index(s, "${")
		if start < 0 {
			break
		}
		end := strings.Index(s[start+2:], "}")
		if end < 0 {
			break
		}
		out = append(out, s[start+2:start+2+end])
		s = s[start+2+end+1:]
	}
	return out
}

func substituteSeedTag(request []byte, seedBlock string) []byte {
	v := "latest"
	if seedBlock != "" {
		v = seedBlock
	}
	return []byte(strings.ReplaceAll(string(request), `"__SEED__"`, `"`+v+`"`))
}

func substituteReverterTag(request []byte, reverterAddr string) []byte {
	if reverterAddr == "" {
		return request
	}
	return []byte(strings.ReplaceAll(string(request), `"__REVERTER__"`, `"`+reverterAddr+`"`))
}

func substituteRequest(request []byte, bindings map[string]any) []byte {
	if len(bindings) == 0 {
		return request
	}
	s := string(request)
	for name, val := range bindings {
		placeholder := "${" + name + "}"
		if !strings.Contains(s, placeholder) {
			continue
		}
		var repl string
		switch v := val.(type) {
		case string:
			repl = v
		case float64:
			repl = fmt.Sprintf("%.0f", v)
		case nil:
			repl = "null"
		default:
			repl = fmt.Sprint(v)
		}
		s = strings.ReplaceAll(s, placeholder, repl)
	}
	return []byte(s)
}

func applyBindings(bindings map[string]any, response []byte, pair ioxPair) {
	for _, b := range pair.AfterBindings {
		if v, ok := getJSONPath(response, b.Path); ok {
			bindings[b.Var] = v
		}
	}
}

func sameBlockResult(t *testing.T, actual, reference []byte) bool {
	t.Helper()
	numA, okA := getJSONPath(actual, "result.number")
	hashA, okB := getJSONPath(actual, "result.hash")
	numR, okR := getJSONPath(reference, "result.number")
	hashR, okS := getJSONPath(reference, "result.hash")
	if !okA || !okB || !okR || !okS {
		t.Log("missing result.number or result.hash in actual or reference")
		return false
	}
	if numA != numR || hashA != hashR {
		t.Logf("block mismatch: actual number=%v hash=%v, reference number=%v hash=%v", numA, hashA, numR, hashR)
		return false
	}
	return true
}

func specOnly(t *testing.T, request, actual, expected []byte) bool {
	t.Helper()
	if err := validateRPCResponse(request, actual, expected); err != nil {
		t.Log(err)
		return false
	}
	return true
}

// decodeRPC preserves integer precision for IDs, error codes and deterministic values.
func decodeRPC(body []byte) (map[string]any, error) {
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if value == nil || !json.Valid(body) {
		return nil, fmt.Errorf("expected one JSON object")
	}
	return value, nil
}

func validRPCID(value any) bool {
	switch v := value.(type) {
	case nil, string:
		return true
	case json.Number:
		return integerPattern.MatchString(string(v))
	default:
		return false
	}
}

func validateRPCResponse(request, actual, expected []byte) error {
	req, err := decodeRPC(request)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	method, ok := req["method"].(string)
	if !ok || method == "" {
		return fmt.Errorf("request has no method")
	}
	act, err := decodeRPC(actual)
	if err != nil {
		return fmt.Errorf("%s response: %w", method, err)
	}
	id, hasID := req["id"]
	responseID, hasResponseID := act["id"]
	if !hasID || !hasResponseID || !validRPCID(id) || !validRPCID(responseID) || !reflect.DeepEqual(id, responseID) || act["jsonrpc"] != "2.0" {
		return fmt.Errorf("%s: invalid JSON-RPC version or mismatched id", method)
	}
	_, result := act["result"]
	_, rpcError := act["error"]
	if result == rpcError {
		return fmt.Errorf("%s: require exactly one of result and error", method)
	}
	if rpcError {
		if err := validateRPCError(method, act["error"]); err != nil {
			return err
		}
	} else if err := validateMethodResult(method, req, act["result"]); err != nil {
		return fmt.Errorf("%s result: %w", method, err)
	}
	if len(expected) == 0 {
		return nil
	}
	exp, err := decodeRPC(expected)
	if err != nil {
		return fmt.Errorf("expected response: %w", err)
	}
	_, expectedID := exp["id"]
	if !expectedID || exp["jsonrpc"] != "2.0" || !reflect.DeepEqual(exp["id"], id) {
		return fmt.Errorf("invalid fixture version or id")
	}
	_, expResult := exp["result"]
	_, expError := exp["error"]
	if expResult == expError || expResult != result {
		return fmt.Errorf("%s: response kind mismatch", method)
	}
	if expError {
		e, ok := exp["error"].(map[string]any)
		if !ok {
			return fmt.Errorf("fixture error must be an object")
		}
		if _, ok := e["code"]; !ok {
			return fmt.Errorf("fixture error must declare an exact code")
		}
	}
	// Empty result objects request the method's full schema. Every populated field,
	// scalar, null, and array is still an exact expectation, including error data.
	field := "result"
	if rpcError {
		field = "error"
	}
	if !matchesExpectation(act[field], exp[field]) {
		return fmt.Errorf("%s: %s differs from deterministic fixture expectation", method, field)
	}
	return nil
}

func matchesExpectation(actual, expected any) bool {
	if fields, ok := expected.(map[string]any); ok {
		object, ok := actual.(map[string]any)
		if !ok {
			return false
		}
		for name, value := range fields {
			got, exists := object[name]
			if !exists || !matchesExpectation(got, value) {
				return false
			}
		}
		return true
	}
	if elements, ok := expected.([]any); ok {
		got, ok := actual.([]any)
		if !ok || len(got) != len(elements) {
			return false
		}
		for i, value := range elements {
			if !matchesExpectation(got[i], value) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(actual, expected)
}

func validateRPCError(method string, value any) error {
	e, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: error must be an object", method)
	}
	number, ok := e["code"].(json.Number)
	if !ok {
		return fmt.Errorf("%s: error code must be an integer", method)
	}
	code, err := strconv.ParseInt(string(number), 10, 32)
	if err != nil {
		return fmt.Errorf("%s: invalid error code", method)
	}
	message, ok := e["message"].(string)
	if !ok || message == "" {
		return fmt.Errorf("%s: error message must be nonempty", method)
	}
	valid := code == -32700 || (code >= -32603 && code <= -32600) || (code >= -32099 && code <= -32000)
	if code == 3 && (method == "eth_call" || method == "eth_estimateGas" || method == "eth_createAccessList") {
		valid = true
	}
	if !valid {
		return fmt.Errorf("%s: undeclared JSON-RPC error code %d", method, code)
	}
	if code == 3 {
		if data, exists := e["data"]; exists && !dataValue(data) {
			return fmt.Errorf("%s: revert data must be hex bytes", method)
		}
	}
	return nil
}

type valueRule func(any) bool

var integerPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
var quantityPattern = regexp.MustCompile(`^0x(0|[1-9a-f][0-9a-f]*)$`)
var dataPattern = regexp.MustCompile(`^0x([0-9a-fA-F]{2})*$`)
var decimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)

func quantityValue(v any) bool { s, ok := v.(string); return ok && quantityPattern.MatchString(s) }
func dataValue(v any) bool     { s, ok := v.(string); return ok && dataPattern.MatchString(s) }
func sizedData(size int) valueRule {
	return func(v any) bool { s, ok := v.(string); return ok && len(s) == 2+size*2 && dataValue(v) }
}
func stringValue(v any) bool { _, ok := v.(string); return ok }
func boolValue(v any) bool   { _, ok := v.(bool); return ok }
func arrayOf(rule valueRule) valueRule {
	return func(v any) bool {
		a, ok := v.([]any)
		if !ok {
			return false
		}
		for _, item := range a {
			if !rule(item) {
				return false
			}
		}
		return true
	}
}
func objectFields(v any, fields map[string]valueRule) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	for key, rule := range fields {
		value, exists := m[key]
		if !exists || !rule(value) {
			return false
		}
	}
	return true
}
func nullable(rule valueRule) valueRule { return func(v any) bool { return v == nil || rule(v) } }
func logValue(v any) bool {
	return objectFields(v, map[string]valueRule{"address": sizedData(20), "topics": arrayOf(sizedData(32)), "data": dataValue,
		"blockNumber": nullable(quantityValue), "blockHash": nullable(sizedData(32)), "transactionHash": nullable(sizedData(32)),
		"transactionIndex": nullable(quantityValue), "logIndex": nullable(quantityValue), "removed": boolValue})
}
func transactionValue(v any) bool {
	return objectFields(v, map[string]valueRule{"hash": sizedData(32), "nonce": quantityValue, "from": sizedData(20), "to": nullable(sizedData(20)),
		"gas": quantityValue, "value": quantityValue, "input": dataValue, "v": quantityValue, "r": quantityValue, "s": quantityValue,
		"blockHash": nullable(sizedData(32)), "blockNumber": nullable(quantityValue), "transactionIndex": nullable(quantityValue)})
}
func blockValue(v any, full bool) bool {
	txRule := sizedData(32)
	if full {
		txRule = transactionValue
	}
	return objectFields(v, map[string]valueRule{"number": nullable(quantityValue), "hash": nullable(sizedData(32)), "parentHash": sizedData(32),
		"nonce": nullable(sizedData(8)), "sha3Uncles": sizedData(32), "logsBloom": nullable(sizedData(256)), "transactionsRoot": sizedData(32),
		"stateRoot": sizedData(32), "receiptsRoot": sizedData(32), "miner": sizedData(20), "difficulty": quantityValue, "extraData": dataValue,
		"size": quantityValue, "gasLimit": quantityValue, "gasUsed": quantityValue, "timestamp": quantityValue,
		"transactions": arrayOf(txRule), "uncles": arrayOf(sizedData(32))})
}
func receiptValue(v any) bool {
	return objectFields(v, map[string]valueRule{"transactionHash": sizedData(32), "transactionIndex": quantityValue, "blockHash": sizedData(32),
		"blockNumber": quantityValue, "from": sizedData(20), "to": nullable(sizedData(20)), "cumulativeGasUsed": quantityValue,
		"gasUsed": quantityValue, "contractAddress": nullable(sizedData(20)), "logs": arrayOf(logValue), "logsBloom": sizedData(256),
		"status": func(v any) bool { return v == "0x0" || v == "0x1" }})
}
func feeHistoryValue(v any, req map[string]any) bool {
	ratio := func(v any) bool {
		n, ok := v.(json.Number)
		if !ok {
			return false
		}
		f, err := n.Float64()
		return err == nil && f >= 0 && f <= 1
	}
	if !objectFields(v, map[string]valueRule{"oldestBlock": quantityValue, "baseFeePerGas": arrayOf(quantityValue), "gasUsedRatio": arrayOf(ratio)}) {
		return false
	}
	m := v.(map[string]any)
	count := len(m["gasUsedRatio"].([]any))
	if len(m["baseFeePerGas"].([]any)) != count+1 {
		return false
	}
	params, _ := req["params"].([]any)
	percentileCount := 0
	if len(params) > 2 {
		percentiles, ok := params[2].([]any)
		if !ok {
			return false
		}
		percentileCount = len(percentiles)
	}
	if reward, exists := m["reward"]; exists {
		if !arrayOf(arrayOf(quantityValue))(reward) {
			return false
		}
		rows := reward.([]any)
		if len(rows) != count {
			return false
		}
		for _, row := range rows {
			if len(row.([]any)) != percentileCount {
				return false
			}
		}
	} else if percentileCount > 0 {
		return false
	}
	if blob, exists := m["baseFeePerBlobGas"]; exists {
		if !arrayOf(quantityValue)(blob) || len(blob.([]any)) != count+1 {
			return false
		}
	}
	if blob, exists := m["blobGasUsedRatio"]; exists {
		if !arrayOf(ratio)(blob) || len(blob.([]any)) != count {
			return false
		}
	}
	return true
}
func traceValue(v any) bool {
	if objectFields(v, map[string]valueRule{"type": stringValue, "from": sizedData(20), "gas": quantityValue, "gasUsed": quantityValue, "input": dataValue}) {
		return true
	}
	return objectFields(v, map[string]valueRule{"gas": func(v any) bool {
		n, ok := v.(json.Number)
		if !ok {
			return false
		}
		i, e := n.Int64()
		return e == nil && i >= 0
	}, "failed": boolValue, "returnValue": stringValue, "structLogs": arrayOf(func(v any) bool {
		return objectFields(v, map[string]valueRule{"op": stringValue, "pc": func(v any) bool { _, ok := v.(json.Number); return ok }})
	})})
}
func validateMethodResult(method string, req map[string]any, result any) error {
	full := false
	if params, ok := req["params"].([]any); ok && len(params) > 1 {
		full, _ = params[1].(bool)
	}
	var rule valueRule
	switch method {
	case "eth_blockNumber", "eth_chainId", "eth_gasPrice", "eth_maxPriorityFeePerGas", "eth_blobBaseFee", "eth_getBalance", "eth_getTransactionCount", "eth_estimateGas", "eth_estimateGasAfterCalls", "eth_getBlockTransactionCountByHash", "eth_getBlockTransactionCountByNumber", "eth_newFilter", "eth_newBlockFilter", "eth_newPendingTransactionFilter", "pax_newFilter", "pax_newBlockFilter":
		rule = quantityValue
	case "eth_call", "eth_getCode", "eth_sign", "eth_signTransaction":
		rule = dataValue
	case "eth_getStorageAt", "eth_sendRawTransaction", "eth_sendTransaction":
		rule = sizedData(32)
	case "eth_coinbase", "pax_getEVMAddress":
		rule = sizedData(20)
	case "eth_accounts":
		rule = arrayOf(sizedData(20))
	case "net_version":
		rule = func(v any) bool { s, ok := v.(string); return ok && decimalPattern.MatchString(s) }
	case "web3_clientVersion", "echo_echo", "pax_getCosmosTx", "pax_getEvmTx", "pax_getPaxAddress", "eth_getTransactionErrorByHash":
		rule = stringValue
	case "eth_syncing":
		rule = func(v any) bool {
			return v == false || objectFields(v, map[string]valueRule{"startingBlock": quantityValue, "currentBlock": quantityValue, "highestBlock": quantityValue})
		}
	case "eth_uninstallFilter", "pax_uninstallFilter":
		rule = boolValue
	case "eth_getBlockByNumber", "eth_getBlockByHash", "pax_getBlockByNumber", "pax_getBlockByHashExcludeTraceFail", "pax_getBlockByNumberExcludeTraceFail":
		rule = nullable(func(v any) bool { return blockValue(v, full) })
	case "eth_getTransactionByHash", "eth_getTransactionByBlockHashAndIndex", "eth_getTransactionByBlockNumberAndIndex":
		rule = nullable(transactionValue)
	case "eth_getTransactionReceipt", "pax_getTransactionReceiptExcludeTraceFail":
		rule = nullable(receiptValue)
	case "eth_getBlockReceipts":
		rule = nullable(arrayOf(receiptValue))
	case "eth_getLogs", "eth_getFilterLogs", "pax_getLogs", "pax_getFilterLogs":
		rule = arrayOf(logValue)
	case "eth_getFilterChanges", "pax_getFilterChanges":
		rule = func(v any) bool { return arrayOf(logValue)(v) || arrayOf(sizedData(32))(v) }
	case "eth_feeHistory":
		rule = func(v any) bool { return feeHistoryValue(v, req) }
	case "eth_createAccessList":
		rule = func(v any) bool {
			return objectFields(v, map[string]valueRule{"gasUsed": quantityValue, "accessList": arrayOf(func(v any) bool {
				return objectFields(v, map[string]valueRule{"address": sizedData(20), "storageKeys": arrayOf(sizedData(32))})
			})})
		}
	case "eth_getProof":
		rule = func(v any) bool {
			return objectFields(v, map[string]valueRule{"address": sizedData(20), "balance": quantityValue, "nonce": quantityValue, "codeHash": sizedData(32), "storageHash": sizedData(32), "accountProof": arrayOf(dataValue), "storageProof": arrayOf(func(v any) bool {
				return objectFields(v, map[string]valueRule{"key": quantityValue, "value": quantityValue, "proof": arrayOf(dataValue)})
			})})
		}
	case "debug_traceCall", "debug_traceTransaction":
		rule = traceValue
	case "debug_traceBlockByNumber", "debug_traceBlockByHash", "pax_traceBlockByNumberExcludeTraceFail", "pax_traceBlockByHashExcludeTraceFail":
		rule = arrayOf(func(v any) bool { return objectFields(v, map[string]valueRule{"result": traceValue}) })
	case "debug_traceStateAccess":
		rule = func(v any) bool {
			return objectFields(v, map[string]valueRule{"accessList": arrayOf(func(v any) bool {
				return objectFields(v, map[string]valueRule{"address": sizedData(20), "storageKeys": arrayOf(sizedData(32))})
			})})
		}
	case "eth_getVMError":
		rule = nullable(stringValue)
	case "txpool_content":
		rule = func(v any) bool {
			object := func(v any) bool { _, ok := v.(map[string]any); return ok }
			return objectFields(v, map[string]valueRule{"pending": object, "queued": object})
		}
	default:
		return fmt.Errorf("no success schema declared for %s", method)
	}
	if !rule(result) {
		return fmt.Errorf("invalid method response schema")
	}
	return nil
}

func ioTestsDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, "testdata"), nil
}

func collectIOFiles(dir string) ([]string, error) {
	var out []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".io" && ext != ".iox" {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}
