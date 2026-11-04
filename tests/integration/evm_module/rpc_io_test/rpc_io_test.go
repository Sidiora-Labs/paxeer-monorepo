// Package rpc_io_test runs ethereum/execution-apis .io/.iox tests against a local Pax EVM RPC.
// Env: PAX_EVM_IO_RUN_INTEGRATION=1 to run (set by integration script/CI). If unset, tests skip.
// PAX_EVM_RPC_URL (default http://127.0.0.1:8545). Debug: PAX_EVM_IO_DEBUG_FILES="file1.iox,file2.io".
package rpc_io_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var evmRPCSpecResults struct{ passed, failed, skipped int }

type ioFixture struct {
	Name  string
	Pairs []ioxPair
}

type ioRunOptions struct{ SeedBlock, ReverterAddress, DeployTxHash string }

func caseIdentity(file string, pair int) string { return fmt.Sprintf("%s#pair-%d", file, pair+1) }

func compareCaseInventory(declared, executed []string) error {
	if len(declared) == 0 {
		return fmt.Errorf("declared case inventory is empty")
	}
	counts := make(map[string]int, len(declared))
	for _, id := range declared {
		if id == "" || counts[id] != 0 {
			return fmt.Errorf("empty or duplicate declared case %q", id)
		}
		counts[id] = 1
	}
	for _, id := range executed {
		if counts[id] != 1 {
			return fmt.Errorf("unexpected or duplicate executed case %q", id)
		}
		counts[id] = 0
	}
	var missing []string
	for _, id := range declared {
		if counts[id] != 0 {
			missing = append(missing, id)
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("missing executed cases: %v", missing)
	}
	return nil
}

// Load every declared fixture before transport, so a debug selection cannot hide missing files.
func loadIOFixtures(dir string, declared []string, selection string) ([]ioFixture, []string, error) {
	files, err := collectIOFiles(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("collect fixtures: %w", err)
	}
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("no .io/.iox fixtures in %s", dir)
	}
	var fixtures []ioFixture
	var discovered []string
	for _, rel := range files {
		content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, nil, err
		}
		pairs, err := parseIOFile(string(content))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", rel, err)
		}
		if len(pairs) == 0 {
			return nil, nil, fmt.Errorf("%s has no request/response pairs", rel)
		}
		fixtures = append(fixtures, ioFixture{Name: rel, Pairs: pairs})
		for i := range pairs {
			discovered = append(discovered, caseIdentity(rel, i))
		}
	}
	if err := compareCaseInventory(declared, discovered); err != nil {
		return nil, nil, err
	}
	if selection == "" {
		return fixtures, declared, nil
	}
	wanted := make(map[string]bool)
	for _, name := range strings.Split(selection, ",") {
		name = strings.TrimSpace(name)
		if name == "" || wanted[name] {
			return nil, nil, fmt.Errorf("empty or duplicate debug fixture %q", name)
		}
		wanted[name] = true
	}
	var selected []ioFixture
	var selectedIDs []string
	for _, fixture := range fixtures {
		if wanted[fixture.Name] {
			selected = append(selected, fixture)
			for i := range fixture.Pairs {
				selectedIDs = append(selectedIDs, caseIdentity(fixture.Name, i))
			}
			delete(wanted, fixture.Name)
		}
	}
	if len(wanted) != 0 {
		return nil, nil, fmt.Errorf("debug selection names unknown fixtures: %v", wanted)
	}
	return selected, selectedIDs, nil
}

func prepareIORequest(pair ioxPair, bindings map[string]any, options ioRunOptions) ([]byte, error) {
	if bytes.Contains(pair.Request, []byte("__REVERTER__")) && options.ReverterAddress == "" {
		return nil, fmt.Errorf("__REVERTER__ requires PAX_EVM_IO_REVERTER_ADDRESS")
	}
	for _, name := range requestPlaceholders(pair.Request) {
		if value, ok := bindings[name]; !ok || value == nil {
			return nil, fmt.Errorf("missing binding ${%s}", name)
		}
	}
	request := substituteSeedTag(substituteRequest(substituteReverterTag(pair.Request, options.ReverterAddress), bindings), options.SeedBlock)
	if bytes.Contains(request, []byte("${")) {
		return nil, fmt.Errorf("unresolved request binding")
	}
	return request, nil
}

func checkIOPair(request, body []byte, header http.Header, pair ioxPair, prior [][]byte) error {
	if len(pair.Expected) == 0 && pair.RefPair == 0 && bytes.HasPrefix(bytes.TrimSpace(body), []byte("[")) {
		var batch []json.RawMessage
		if err := json.Unmarshal(body, &batch); err != nil || len(batch) == 0 {
			return fmt.Errorf("invalid or empty batch response")
		}
		if len(pair.ExpectBodyContains) == 0 && len(pair.ExpectResponseHeaders) == 0 {
			return fmt.Errorf("batch response needs explicit directives")
		}
	} else {
		if err := validateRPCResponse(request, body, pair.Expected); err != nil {
			return err
		}
		if pair.RefPair != 0 {
			idx := pair.RefPair - 1
			if idx < 0 || idx >= len(prior) {
				return fmt.Errorf("ref_pair %d must name an earlier response", pair.RefPair)
			}
			for _, path := range []string{"result.number", "result.hash"} {
				got, ok := getJSONPath(body, path)
				reference, found := getJSONPath(prior[idx], path)
				if !ok || !found || !reflect.DeepEqual(got, reference) {
					return fmt.Errorf("ref_pair %d differs at %s", pair.RefPair, path)
				}
			}
		}
	}
	for _, sub := range pair.ExpectBodyContains {
		if !bytes.Contains(body, []byte(sub)) {
			return fmt.Errorf("response body missing %q", sub)
		}
	}
	for _, name := range pair.ExpectResponseHeaders {
		if header.Get(name) == "" {
			return fmt.Errorf("response header missing %q", name)
		}
	}
	return nil
}

func executeIOFixture(client *rpcClient, fixture ioFixture, options ioRunOptions) ([]string, error) {
	if len(fixture.Pairs) == 0 {
		return nil, fmt.Errorf("%s has no request/response pairs", fixture.Name)
	}
	bindings := make(map[string]any)
	if options.DeployTxHash != "" {
		bindings["deployTxHash"] = options.DeployTxHash
		bindings["txHash"] = options.DeployTxHash
	}
	var executed []string
	var responses [][]byte
	for i, pair := range fixture.Pairs {
		id := caseIdentity(fixture.Name, i)
		request, err := prepareIORequest(pair, bindings, options)
		if err != nil {
			return executed, fmt.Errorf("%s: %w", id, err)
		}
		body, status, header, err := client.call(request)
		executed = append(executed, id)
		if err != nil {
			return executed, fmt.Errorf("%s: call: %w", id, err)
		}
		if status != http.StatusOK {
			return executed, fmt.Errorf("%s: HTTP %d body %s", id, status, body)
		}
		if err := checkIOPair(request, body, header, pair, responses); err != nil {
			return executed, fmt.Errorf("%s: %w; response %s", id, err, body)
		}
		for _, b := range pair.AfterBindings {
			if value, ok := getJSONPath(body, b.Path); !ok || value == nil {
				return executed, fmt.Errorf("%s: missing binding path %s", id, b.Path)
			}
		}
		applyBindings(bindings, body, pair)
		responses = append(responses, body)
	}
	return executed, nil
}

func TestEVMRPCSpec(t *testing.T) {
	if os.Getenv("PAX_EVM_IO_RUN_INTEGRATION") != "1" {
		t.Skip("EVM RPC integration tests skipped (set PAX_EVM_IO_RUN_INTEGRATION=1 to run)")
	}
	abs, err := ioTestsDir()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile("case_inventory.json")
	if err != nil {
		t.Fatalf("read declared case inventory: %v", err)
	}
	var declared []string
	if err := json.Unmarshal(content, &declared); err != nil {
		t.Fatal(err)
	}
	fixtures, selected, err := loadIOFixtures(abs, declared, os.Getenv("PAX_EVM_IO_DEBUG_FILES"))
	if err != nil {
		t.Fatal(err)
	}
	client := &rpcClient{URL: rpcURL()}
	if !nodeReachable(client) {
		t.Fatalf("EVM RPC node not reachable at %s", client.URL)
	}
	options := ioRunOptions{SeedBlock: os.Getenv("PAX_EVM_IO_SEED_BLOCK"), ReverterAddress: os.Getenv("PAX_EVM_IO_REVERTER_ADDRESS"), DeployTxHash: os.Getenv("PAX_EVM_IO_DEPLOY_TX_HASH")}
	var executed []string
	evmRPCSpecResults = struct{ passed, failed, skipped int }{}
	for _, fixture := range fixtures {
		if t.Run(fixture.Name, func(t *testing.T) {
			cases, err := executeIOFixture(client, fixture, options)
			executed = append(executed, cases...)
			for _, id := range cases {
				t.Logf("executed %s", id)
			}
			if err != nil {
				t.Fatal(err)
			}
		}) {
			evmRPCSpecResults.passed++
		} else {
			evmRPCSpecResults.failed++
		}
	}
	if err := compareCaseInventory(selected, executed); err != nil {
		t.Fatal(err)
	}
}

func TestEVMRPCSpecSummary(t *testing.T) {
	p, f, s := evmRPCSpecResults.passed, evmRPCSpecResults.failed, evmRPCSpecResults.skipped
	if p+f+s == 0 {
		return
	}
	rate := 0.0
	if p+f > 0 {
		rate = 100 * float64(p) / float64(p+f)
	}
	t.Logf("")
	t.Logf("========== Pax EVM RPC .io/.iox test report ==========")
	t.Logf("  Total:  %d", p+f+s)
	t.Logf("  Passed: %d", p)
	t.Logf("  Failed: %d", f)
	t.Logf("  Skipped: %d", s)
	t.Logf("  Pass rate: %.1f%%", rate)
	t.Logf("=======================================================")
}

func rpcURL() string {
	if u := os.Getenv("PAX_EVM_RPC_URL"); u != "" {
		return u
	}
	return "http://127.0.0.1:8545"
}

func nodeReachable(c *rpcClient) bool {
	body, status, _, err := c.call([]byte(`{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`))
	if err != nil || status != http.StatusOK {
		return false
	}
	var m map[string]any
	return json.Unmarshal(body, &m) == nil && (m["result"] != nil || m["error"] != nil)
}

func logActualResponse(t *testing.T, body []byte) {
	t.Helper()
	var m map[string]json.RawMessage
	if json.Unmarshal(body, &m) != nil {
		return
	}
	if e, ok := m["error"]; ok {
		var errObj struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(e, &errObj)
		t.Logf("actual response: error code=%d message=%q", errObj.Code, errObj.Message)
		return
	}
	if r, ok := m["result"]; ok {
		s := string(r)
		if len(s) > 120 {
			s = s[:120] + "..."
		}
		t.Logf("actual response: result=%s", s)
	}
}

func logDebugResponse(t *testing.T, body []byte, pairIdx int) {
	t.Helper()
	var m map[string]json.RawMessage
	if json.Unmarshal(body, &m) != nil {
		return
	}
	if e, ok := m["error"]; ok {
		t.Logf("[DEBUG] pair %d: response error: %s", pairIdx, e)
		return
	}
	r, ok := m["result"]
	if !ok {
		return
	}
	var res any
	if json.Unmarshal(r, &res) != nil {
		return
	}
	resM, ok := res.(map[string]any)
	if !ok {
		return
	}
	if txs, ok := resM["transactions"]; ok {
		n := -1
		if arr, ok := txs.([]any); ok {
			n = len(arr)
		}
		t.Logf("[DEBUG] pair %d: result.transactions len=%d", pairIdx, n)
	}
}
