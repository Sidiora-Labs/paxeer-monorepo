package rpc_io_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func repair743Fixture(t *testing.T, name string) ioFixture {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	pairs, err := parseIOFile(string(content))
	if err != nil || len(pairs) == 0 {
		t.Fatalf("fixture %s: pairs=%d err=%v", name, len(pairs), err)
	}
	return ioFixture{Name: name, Pairs: pairs}
}

// The peer replays committed transcripts over HTTP. The parser, substitutions,
// transport, response validator and inventory consumer are the actual harness.
func repair743Transport(t *testing.T, requests, responses [][]byte) (*rpcClient, *atomic.Int32) {
	t.Helper()
	calls := new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index := int(calls.Add(1)) - 1
		if index >= len(responses) {
			t.Errorf("unexpected RPC request %d", index+1)
			http.Error(w, "unexpected request", 500)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			http.Error(w, "read", 500)
			return
		}
		actual, err := decodeRPC(body)
		if err != nil {
			t.Error(err)
			http.Error(w, "decode", 500)
			return
		}
		wanted, err := decodeRPC(requests[index])
		if err != nil {
			t.Error(err)
			http.Error(w, "fixture", 500)
			return
		}
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" || !reflect.DeepEqual(actual, wanted) {
			t.Errorf("transport request: got %s, want %s", body, requests[index])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(responses[index])
	}))
	t.Cleanup(server.Close)
	return &rpcClient{URL: server.URL, Client: server.Client()}, calls
}

func repair743Mutate(t *testing.T, body []byte, change func(map[string]any)) []byte {
	t.Helper()
	value, err := decodeRPC(body)
	if err != nil {
		t.Fatal(err)
	}
	change(value)
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestRepairM_743_UnpairedRequests(t *testing.T) {
	TestParseIOFile_UnpairedRequests(t)
	// The real inventory loader also refuses corruption before any transport.
	fixture := repair743Fixture(t, "eth_syncing/sync-status.iox")
	dir := t.TempDir()
	content := string(fixture.Pairs[0].Request)
	if err := os.WriteFile(filepath.Join(dir, "broken.io"), []byte(">> "+content+"\n<< "+string(fixture.Pairs[0].Expected)+"\n>> "+content), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err := loadIOFixtures(dir, []string{"broken.io#pair-1", "broken.io#pair-2"}, "")
	if err == nil || !strings.Contains(err.Error(), "unmatched request") {
		t.Fatalf("loader accepted incomplete transcript: %v", err)
	}
}

func TestRepairM_743_DeterministicValues(t *testing.T) {
	TestSpecOnly(t)
	fixture := repair743Fixture(t, "eth_chainId/get-chain-id.io")
	pair := fixture.Pairs[0]
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
		want   string
	}{
		{"matching", func(m map[string]any) {}, ""},
		{"wrong value", func(m map[string]any) { m["result"] = "0x2" }, "deterministic fixture"},
		{"wrong type", func(m map[string]any) { m["result"] = true }, "schema"},
		{"wrong version", func(m map[string]any) { m["jsonrpc"] = "1.0" }, "version"},
		{"wrong id", func(m map[string]any) { m["id"] = json.Number("2") }, "id"},
		{"string id", func(m map[string]any) { m["id"] = "1" }, "id"},
		{"missing id", func(m map[string]any) { delete(m, "id") }, "id"},
		{"both result and error", func(m map[string]any) { m["error"] = map[string]any{"code": -32602, "message": "bad params"} }, "exactly one"},
		{"neither result nor error", func(m map[string]any) { delete(m, "result") }, "exactly one"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := repair743Mutate(t, pair.Expected, tc.change)
			client, calls := repair743Transport(t, [][]byte{pair.Request}, [][]byte{response})
			executed, err := executeIOFixture(client, fixture, ioRunOptions{})
			if calls.Load() != 1 || len(executed) != 1 {
				t.Fatalf("case did not execute through HTTP: %v", executed)
			}
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}

func TestRepairM_743_EnvelopeIDs(t *testing.T) {
	fixture := repair743Fixture(t, "eth_chainId/get-chain-id.io")
	for _, id := range []any{json.Number("9007199254740993"), "request-id", nil} {
		pair := fixture.Pairs[0]
		pair.Request = repair743Mutate(t, pair.Request, func(m map[string]any) { m["id"] = id })
		pair.Expected = repair743Mutate(t, pair.Expected, func(m map[string]any) { m["id"] = id })
		client, _ := repair743Transport(t, [][]byte{pair.Request}, [][]byte{pair.Expected})
		if _, err := executeIOFixture(client, ioFixture{Name: fixture.Name, Pairs: []ioxPair{pair}}, ioRunOptions{}); err != nil {
			t.Fatal(err)
		}
		wrong := repair743Mutate(t, pair.Expected, func(m map[string]any) { m["id"] = json.Number("9007199254740992") })
		if err := validateRPCResponse(pair.Request, wrong, pair.Expected); err == nil {
			t.Fatal("distinct request ID accepted")
		}
	}
	for _, id := range []any{true, map[string]any{"id": 1}, json.Number("1.5")} {
		pair := fixture.Pairs[0]
		request := repair743Mutate(t, pair.Request, func(m map[string]any) { m["id"] = id })
		response := repair743Mutate(t, pair.Expected, func(m map[string]any) { m["id"] = id })
		if err := validateRPCResponse(request, response, response); err == nil {
			t.Fatalf("invalid matching id type accepted: %v", id)
		}
	}
}

func TestRepairM_743_MethodSchemas(t *testing.T) {
	TestSpecOnly_MethodSchemas(t)
	fee := repair743Fixture(t, "eth_feeHistory/fee-history.io")
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
		valid  bool
	}{
		{"recorded fee history", func(m map[string]any) {}, true},
		{"noncanonical quantity", func(m map[string]any) { m["oldestBlock"] = "0x01" }, false},
		{"missing fees", func(m map[string]any) { delete(m, "baseFeePerGas") }, false},
		{"fee length", func(m map[string]any) { m["baseFeePerGas"] = []any{"0x1"} }, false},
		{"ratio out of range", func(m map[string]any) { m["gasUsedRatio"] = []any{json.Number("1.01")} }, false},
		{"reward widths", func(m map[string]any) { m["reward"] = []any{[]any{"0x1"}} }, false},
		{"missing requested rewards", func(m map[string]any) { delete(m, "reward") }, false},
		{"wrong deterministic fee", func(m map[string]any) { m["baseFeePerGas"] = []any{"0x1", "0x2"} }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pair := fee.Pairs[0]
			response := repair743Mutate(t, pair.Expected, func(m map[string]any) { tc.change(m["result"].(map[string]any)) })
			client, _ := repair743Transport(t, [][]byte{pair.Request}, [][]byte{response})
			_, err := executeIOFixture(client, fee, ioRunOptions{})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v got %v", tc.valid, err)
			}
		})
	}
	// Empty result objects permit dynamic blocks only after the method schema passes.
	fixture := repair743Fixture(t, "cross_check/get-block-by-number-then-by-hash.iox")
	block := repair743Fixture(t, "eth_getBlockByNumber/get-genesis.io").Pairs[0].Expected
	hash, ok := getJSONPath(block, "result.hash")
	if !ok {
		t.Fatal("missing recorded block hash")
	}
	bound := substituteRequest(fixture.Pairs[1].Request, map[string]any{"blockHash": hash})
	client, calls := repair743Transport(t, [][]byte{fixture.Pairs[0].Request, bound}, [][]byte{block, block})
	executed, err := executeIOFixture(client, fixture, ioRunOptions{})
	if err != nil || calls.Load() != 2 {
		t.Fatalf("binding/reference flow: %v", err)
	}
	if err := compareCaseInventory([]string{caseIdentity(fixture.Name, 0), caseIdentity(fixture.Name, 1)}, executed); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(map[string]any){
		func(m map[string]any) { m["result"] = map[string]any{} },
		func(m map[string]any) { m["result"].(map[string]any)["gasUsed"] = "0x00" },
		func(m map[string]any) { m["result"].(map[string]any)["hash"] = "0xaa" },
	} {
		malformed := repair743Mutate(t, block, change)
		if err := validateRPCResponse(fixture.Pairs[0].Request, malformed, fixture.Pairs[0].Expected); err == nil {
			t.Fatal("malformed dynamic block accepted")
		}
	}
	changed := repair743Mutate(t, block, func(m map[string]any) { m["result"].(map[string]any)["number"] = "0x1" })
	client, _ = repair743Transport(t, [][]byte{fixture.Pairs[0].Request, bound}, [][]byte{block, changed})
	if _, err := executeIOFixture(client, fixture, ioRunOptions{}); err == nil || !strings.Contains(err.Error(), "ref_pair") {
		t.Fatalf("reference mismatch accepted: %v", err)
	}
}

func TestRepairM_743_ErrorSchemas(t *testing.T) {
	fixture := repair743Fixture(t, "eth_call/call-revert-abi-error-pax.iox")
	pair := fixture.Pairs[0]
	options := ioRunOptions{ReverterAddress: "0x0000000000000000000000000000000000000001"}
	request, err := prepareIORequest(pair, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
		valid  bool
	}{
		{"declared revert", func(m map[string]any) {}, true},
		{"wrong code", func(m map[string]any) { m["code"] = json.Number("-32602") }, false},
		{"fractional code", func(m map[string]any) { m["code"] = json.Number("3.1") }, false},
		{"string code", func(m map[string]any) { m["code"] = "3" }, false},
		{"missing code", func(m map[string]any) { delete(m, "code") }, false},
		{"wrong message", func(m map[string]any) { m["message"] = "other error" }, false},
		{"missing message", func(m map[string]any) { delete(m, "message") }, false},
		{"empty message", func(m map[string]any) { m["message"] = "" }, false},
		{"wrong data", func(m map[string]any) { m["data"] = "0x00" }, false},
		{"malformed data", func(m map[string]any) { m["data"] = "0x1" }, false},
		{"arbitrary code", func(m map[string]any) { m["code"] = json.Number("42") }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := repair743Mutate(t, pair.Expected, func(m map[string]any) { tc.change(m["error"].(map[string]any)) })
			client, calls := repair743Transport(t, [][]byte{request}, [][]byte{response})
			_, err := executeIOFixture(client, fixture, options)
			if calls.Load() != 1 || (err == nil) != tc.valid {
				t.Fatalf("valid=%v calls=%d got %v", tc.valid, calls.Load(), err)
			}
		})
	}
	if err := validateRPCResponse(request, pair.Expected, []byte(`{"jsonrpc":"2.0","id":1,"error":{}}`)); err == nil {
		t.Fatal("undeclared error code accepted")
	}
	// Method-specific revert codes must not pass unrelated methods.
	chain := repair743Fixture(t, "eth_chainId/get-chain-id.io").Pairs[0]
	if err := validateRPCResponse(chain.Request, pair.Expected, nil); err == nil {
		t.Fatal("revert accepted for eth_chainId")
	}
}

func TestRepairM_743_EnabledInventory(t *testing.T) {
	content, err := os.ReadFile("case_inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	var declared []string
	if err := json.Unmarshal(content, &declared); err != nil {
		t.Fatal(err)
	}
	fixtures, selected, err := loadIOFixtures("testdata", declared, "")
	if err != nil || len(fixtures) == 0 || !reflect.DeepEqual(selected, declared) {
		t.Fatalf("committed inventory: %v", err)
	}
	fixture := repair743Fixture(t, "eth_syncing/sync-status.iox")
	chosen, ids, err := loadIOFixtures("testdata", declared, fixture.Name)
	if err != nil || len(chosen) != 1 {
		t.Fatalf("declared debug subset: %v", err)
	}
	client, _ := repair743Transport(t, [][]byte{fixture.Pairs[0].Request}, [][]byte{fixture.Pairs[0].Expected})
	executed, err := executeIOFixture(client, chosen[0], ioRunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := compareCaseInventory(ids, executed); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name               string
		declared, executed []string
	}{
		{"same totals different identities", []string{"a#pair-1", "b#pair-1"}, []string{"a#pair-1", "c#pair-1"}},
		{"missing case", ids, nil},
		{"duplicate execution", ids, append(append([]string{}, executed...), executed...)},
		{"duplicate declaration", []string{"a", "a"}, []string{"a", "a"}},
		{"empty inventory", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := compareCaseInventory(tc.declared, tc.executed); err == nil {
				t.Fatal("invalid inventory accepted")
			}
		})
	}
	for _, selection := range []string{"missing.io", fixture.Name + ",missing.io", fixture.Name + "," + fixture.Name} {
		if _, _, err := loadIOFixtures("testdata", declared, selection); err == nil {
			t.Fatalf("unknown/duplicate selection accepted: %s", selection)
		}
	}
	// A missing declared fixture fails even when debugging an unrelated file.
	if _, _, err := loadIOFixtures("testdata", append(append([]string{}, declared...), "missing.io#pair-1"), fixture.Name); err == nil {
		t.Fatal("missing required fixture accepted")
	}
	for _, dir := range []string{filepath.Join(t.TempDir(), "missing"), t.TempDir()} {
		if _, _, err := loadIOFixtures(dir, ids, ""); err == nil {
			t.Fatalf("missing/empty directory accepted: %s", dir)
		}
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "empty.io"), []byte("// no pairs\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadIOFixtures(dir, []string{"empty.io#pair-1"}, ""); err == nil || !strings.Contains(err.Error(), "no request/response pairs") {
		t.Fatalf("empty fixture accepted: %v", err)
	}
}

func TestRepairM_743_EnabledSetup(t *testing.T) {
	reverter := repair743Fixture(t, "eth_call/call-revert-abi-error-pax.iox")
	code := repair743Fixture(t, "eth_getCode/get-code-from-deploy.iox")
	for _, fixture := range []ioFixture{reverter, {Name: code.Name, Pairs: code.Pairs[1:2]}, {Name: "empty.io"}} {
		client, calls := repair743Transport(t, nil, nil)
		executed, err := executeIOFixture(client, fixture, ioRunOptions{})
		if err == nil || len(executed) != 0 || calls.Load() != 0 {
			t.Fatalf("missing mandatory setup accepted: executed=%v calls=%d err=%v", executed, calls.Load(), err)
		}
	}
	// A successful schema check cannot conceal a missing producer binding path.
	fixture := repair743Fixture(t, "cross_check/get-block-by-number-then-by-hash.iox")
	fixture.Pairs[0].AfterBindings = append(fixture.Pairs[0].AfterBindings, binding{Var: "missing", Path: "result.missing"})
	block := repair743Fixture(t, "eth_getBlockByNumber/get-genesis.io").Pairs[0].Expected
	client, calls := repair743Transport(t, [][]byte{fixture.Pairs[0].Request}, [][]byte{block})
	executed, err := executeIOFixture(client, fixture, ioRunOptions{})
	if err == nil || !strings.Contains(err.Error(), "missing binding path") || calls.Load() != 1 {
		t.Fatalf("binding producer failure: %v", err)
	}
	if err := compareCaseInventory([]string{caseIdentity(fixture.Name, 0), caseIdentity(fixture.Name, 1)}, executed); err == nil {
		t.Fatal("unexecuted dependent case accepted")
	}
}

func TestRepairM_743_ParityGuide(t *testing.T) {
	content, err := os.ReadFile("RPC_IO_README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, phrase := range []string{"do not prove parity", "case_inventory.json", "deterministic", "method-aware", "kind-only"} {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("guide omits %q", phrase)
		}
	}
	// Equal totals with different failures are insufficient; identities and each
	// case's actual expectations must both be checked by the real harness.
	fixture := repair743Fixture(t, "eth_chainId/get-chain-id.io")
	wrong := repair743Mutate(t, fixture.Pairs[0].Expected, func(m map[string]any) { m["result"] = "0x2" })
	client, _ := repair743Transport(t, [][]byte{fixture.Pairs[0].Request}, [][]byte{wrong})
	executed, err := executeIOFixture(client, fixture, ioRunOptions{})
	if err == nil {
		t.Fatal("identity alone concealed a deterministic mismatch")
	}
	if err := compareCaseInventory([]string{caseIdentity(fixture.Name, 0)}, executed); err != nil {
		t.Fatal(fmt.Errorf("record actual execution identity: %w", err))
	}
}
