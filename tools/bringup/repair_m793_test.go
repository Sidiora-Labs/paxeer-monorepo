package bringup

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

func m793Fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile(filepath.Join("tests", "fixtures", "repair_m793", name))
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func m793JSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func m793Map(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if e := json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	return v
}
func m793Source(t *testing.T, path string, terms ...string) {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	for _, term := range terms {
		if !bytes.Contains(b, []byte(term)) {
			t.Fatalf("%s missing wiring %q", path, term)
		}
	}
}
func m793Reject(t *testing.T, err error, out *bytes.Buffer) {
	t.Helper()
	if err == nil {
		t.Fatal("invalid evidence accepted")
	}
	if out != nil && strings.Contains(out.String(), "pass ") {
		t.Fatalf("unproven success: %s", out)
	}
}

func TestRepairM_793_RegionProbeSelected(t *testing.T) {
	deployment := m793Fixture(t, "deployment.json")
	d, e := ReadRegionalDeployment(bytes.NewReader(deployment))
	if e != nil {
		t.Fatal(e)
	}
	var rows []RegionalResponse
	if e = json.Unmarshal(m793Fixture(t, "regional-responses.json"), &rows); e != nil {
		t.Fatal(e)
	}
	now := time.UnixMilli(1750000000001)
	var out bytes.Buffer
	if e = ValidateRegionalResponses(d, bytes.NewReader(m793JSON(t, rows)), now, &out); e != nil {
		t.Fatal(e)
	}
	for _, row := range rows {
		if !strings.Contains(out.String(), "instance="+row.Instance+" region="+row.Region) {
			t.Fatalf("missing selected evidence %s", out.String())
		}
	}
	for _, name := range []string{"missing", "duplicate", "deployment", "region", "same-region", "stale"} {
		t.Run(name, func(t *testing.T) {
			changed := append([]RegionalResponse(nil), rows...)
			clock := now
			switch name {
			case "missing":
				changed = changed[:1]
			case "duplicate":
				changed[1] = changed[0]
			case "deployment":
				changed[1].Deployment = "other"
			case "region":
				changed[1].Region = "undeclared"
			case "same-region":
				changed[1].Region = changed[0].Region
			case "stale":
				clock = now.Add(time.Minute)
			}
			var output bytes.Buffer
			m793Reject(t, ValidateRegionalResponses(d, bytes.NewReader(m793JSON(t, changed)), clock, &output), &output)
		})
	}
	var selected bytes.Buffer
	if e = RunInspector(context.Background(), []string{"regional-instances"}, bytes.NewReader(deployment), &selected); e != nil {
		t.Fatal(e)
	}
	if strings.Count(selected.String(), d.ID) != 2 {
		t.Fatal(selected.String())
	}
	m793Source(t, "check-live.sh", `--deployment-instance`, `railway_ssh "$app" "python3 -" "$instance"`, `RAILWAY_REPLICA_ID`, `RAILWAY_REPLICA_REGION`, `regional-readiness`)
}

func TestRepairM_793_HumanChildExit(t *testing.T) {
	transcript := string(m793Fixture(t, "human.txt"))
	var out bytes.Buffer
	if e := RunInspector(context.Background(), []string{"human-child", "0"}, strings.NewReader(transcript), &out); e != nil {
		t.Fatal(e)
	}
	for _, status := range []int{1, 2, 124, 127, 139} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var out bytes.Buffer
			if e := RunInspector(context.Background(), []string{"human-child", strconv.Itoa(status)}, strings.NewReader(transcript), &out); e == nil || !strings.Contains(out.String(), fmt.Sprintf("fail human-child exit=%d", status)) {
				t.Fatalf("exit lost: %v %s", e, &out)
			}
		})
	}
	for _, broken := range []string{"", strings.ReplaceAll(transcript, "pass preflight", "error preflight"), strings.ReplaceAll(transcript, "check-live: all checks passed", ""), strings.ReplaceAll(transcript, "pass human-evidence checks=live,preflight,plan", ""), transcript + "fail later http=500\n", transcript + "pass live duplicate=true\n"} {
		var out bytes.Buffer
		if ValidateHumanChild(strings.NewReader(broken), 0, &out) == nil {
			t.Fatalf("incomplete child accepted: %q", broken)
		}
	}
	m793Source(t, "check-live.sh", `child_status=$?`, `bringup_inspect human-child "$child_status" <<<"$child_output"`)
	m793Source(t, "../../scripts/wallet/check-live.sh", `pass human-evidence checks=live,preflight,plan`)
}

type m793Chain struct {
	raw                 []byte
	native              map[string]any
	key                 map[string]any
	header              map[string]any
	commit              map[string]any
	hash, block, height string
}

func m793RecordedChain(t *testing.T) m793Chain {
	t.Helper()
	root := m793Map(t, m793Fixture(t, "committed.json"))
	v := root["cases"].([]any)[0].(map[string]any)
	native := v["receiptJson"].(map[string]any)
	raw, e := hex.DecodeString(strings.TrimPrefix(v["rawTransaction"].(string), "0x"))
	if e != nil {
		t.Fatal(e)
	}
	return m793Chain{raw, native, v["pinnedKey"].(map[string]any), v["header"].(map[string]any), v["commit"].(map[string]any), native["txHash"].(string), native["blockHash"].(string), native["blockNumber"].(string)}
}

// The loopback inclusion peer uses the unmodified chain-125 execution
// transaction and native signature from the production RPC conformance
// recorder. Its block/envelope are explicit in-memory peer state, not a
// claim that the separate dev-chain Comet recording ran with chain ID 125.
func m793Chain125Peer(t *testing.T) m793Chain {
	t.Helper()
	root := m793Map(t, m793Fixture(t, "receipted.json"))
	var v map[string]any
	for _, item := range root["cases"].([]any) {
		candidate := item.(map[string]any)
		if candidate["id"] == "V1-layerxledger-pay-exact" {
			v = candidate
			break
		}
	}
	if v == nil {
		t.Fatal("recorded chain-125 payExact missing")
	}
	raw, e := hex.DecodeString(strings.TrimPrefix(v["rawTransaction"].(string), "0x"))
	if e != nil {
		t.Fatal(e)
	}
	tx, e := DecodeSignedTransaction(raw)
	if e != nil {
		t.Fatal(e)
	}
	native := v["receiptJson"].(map[string]any)
	height, e := quantity(native["height"].(string))
	if e != nil {
		t.Fatal(e)
	}
	block := types.NewBlockWithHeader(&types.Header{Number: new(big.Int).Set(height), TxHash: types.DeriveSha(types.Transactions{tx}, trie.NewStackTrie(nil))}).WithBody(types.Body{Transactions: []*types.Transaction{tx}})
	native["rung"] = "committed"
	native["blockNumber"] = "0x" + block.Number().Text(16)
	native["blockHash"] = block.Hash().Hex()
	return m793Chain{raw: raw, native: native, key: v["pinnedKey"].(map[string]any), hash: tx.Hash().Hex(), block: block.Hash().Hex(), height: native["blockNumber"].(string)}
}

func (c m793Chain) rpc(method string, params json.RawMessage) (any, bool) {
	switch method {
	case "eth_sendRawTransaction":
		var p []string
		if json.Unmarshal(params, &p) != nil || len(p) != 1 || p[0] != "0x"+hex.EncodeToString(c.raw) {
			return nil, false
		}
		return c.hash, true
	case "eth_getTransactionReceipt":
		return map[string]any{"transactionHash": c.hash, "blockHash": c.block, "blockNumber": c.height, "status": "0x1"}, true
	case "eth_getBlockByNumber":
		return map[string]any{"hash": c.block, "number": c.height, "transactions": []string{c.hash}}, true
	case "pax_sequencerReceipt":
		return c.native, true
	case "px_getReceiptKeys":
		return map[string]any{"keys": []any{c.key}}, true
	case "eth_chainId":
		return "0x7d", true
	case "eth_blockNumber":
		return "0xb", true
	case "status":
		return map[string]any{"node_info": map[string]any{"network": c.header["chain_id"]}, "sync_info": map[string]any{"latest_block_height": "11", "catching_up": false}}, true
	case "commit":
		h := map[string]any{}
		for k, v := range c.header {
			h[k] = v
		}
		h["height"] = "11"
		commit := map[string]any{}
		for k, v := range c.commit {
			commit[k] = v
		}
		commit["height"] = "11"
		return map[string]any{"canonical": true, "signed_header": map[string]any{"header": h, "commit": commit}}, true
	}
	return nil, false
}
func m793RPCServer(t *testing.T, c m793Chain, mutate func(string, any) any) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "invalid", 400)
			return
		}
		v, ok := c.rpc(req.Method, req.Params)
		if !ok {
			http.Error(w, "unexpected request", 400)
			return
		}
		if mutate != nil {
			v = mutate(req.Method, v)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": v})
	}))
	t.Cleanup(s.Close)
	return s
}

func TestRepairM_793_KernelCommittedReadiness(t *testing.T) {
	c := m793RecordedChain(t)
	server := m793RPCServer(t, c, nil)
	process := KernelProcess{Executable: "paxd", PID: 400, UID: 4020, Owner: 4020, Home: "/data/paxd", Mounted: true, Genesis: true, State: true, Running: true, StartTicks: 900, BootID: "boot-current"}
	var out bytes.Buffer
	if e := RunInspector(context.Background(), []string{"kernel-node", server.URL, server.URL}, bytes.NewReader(m793JSON(t, process)), &out); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out.String(), "committed-height=11") {
		t.Fatal(out.String())
	}
	for _, name := range []string{"retired", "mount", "owner", "state", "process", "restart"} {
		t.Run(name, func(t *testing.T) {
			p := process
			switch name {
			case "retired":
				p.Executable = "layerxd"
			case "mount":
				p.Mounted = false
			case "owner":
				p.Owner = 0
			case "state":
				p.State = false
			case "process":
				p.Running = false
			case "restart":
				p.StartTicks = 0
			}
			var out bytes.Buffer
			m793Reject(t, CheckKernelNode(context.Background(), server.Client(), server.URL, server.URL, p, &out), &out)
		})
	}
	for _, name := range []string{"catching", "head", "commit"} {
		t.Run(name, func(t *testing.T) {
			s := m793RPCServer(t, c, func(method string, v any) any {
				if name == "head" && method == "eth_blockNumber" {
					return "0x0"
				}
				if name == "catching" && method == "status" {
					v.(map[string]any)["sync_info"].(map[string]any)["catching_up"] = true
				}
				if name == "commit" && method == "commit" {
					v.(map[string]any)["canonical"] = false
				}
				return v
			})
			var out bytes.Buffer
			m793Reject(t, CheckKernelNode(context.Background(), s.Client(), s.URL, s.URL, process, &out), &out)
		})
	}
	source := string(m793FixtureSource(t, "check-live.sh"))
	start := strings.Index(source, "kernel_roster() {")
	end := strings.Index(source[start:], "\n}")
	roster := source[start : start+end]
	for _, retired := range []string{"layerxd", "guarantor-1", "guarantor-2", "receipt-authority", "relay-archive"} {
		if strings.Contains(roster, retired) {
			t.Fatalf("retired role %s", retired)
		}
	}
	m793Source(t, "check-live.sh", `printf 'paxd 4020`, `reason=unsupervised`, `no_new_privs`, `info=home.stat()`, `start_ticks`, `kernel-node "$node" "$comet"`)
}
func m793FixtureSource(t *testing.T, path string) []byte {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestRepairM_793_ArchiveCommittedReadback(t *testing.T) {
	recorded := m793RecordedChain(t)
	if _, err := DecodeSignedTransaction(recorded.raw); err == nil {
		t.Fatal("foreign dev-chain transaction accepted for chain 125")
	}
	if err := VerifyCommittedReceipt(m793JSON(t, recorded.native), m793JSON(t, map[string]any{"keys": []any{recorded.key}}), recorded.hash); err != nil {
		t.Fatal(err)
	}
	c := m793Chain125Peer(t)
	s := m793RPCServer(t, c, nil)
	path := filepath.Join(t.TempDir(), "signed.hex")
	if e := os.WriteFile(path, []byte("0x"+hex.EncodeToString(c.raw)), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var out bytes.Buffer
	if e := RunInspector(ctx, []string{"archive-delivery", s.URL, path, "-"}, strings.NewReader(""), &out); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out.String(), "transaction="+c.hash) || !strings.Contains(out.String(), "read-back=match") {
		t.Fatal(out.String())
	}
	for _, name := range []string{"rung", "canonical", "inclusion", "signature", "key-range"} {
		t.Run(name, func(t *testing.T) {
			copy := c
			copy.native = m793Map(t, m793JSON(t, c.native))
			copy.key = m793Map(t, m793JSON(t, c.key))
			if name == "rung" {
				copy.native["rung"] = "receipted"
			}
			if name == "signature" {
				raw, _ := hex.DecodeString(strings.TrimPrefix(copy.native["receipt"].(string), "0x"))
				raw[len(raw)-1] ^= 1
				copy.native["receipt"] = "0x" + hex.EncodeToString(raw)
			}
			if name == "key-range" {
				copy.key["firstHeight"] = 2000
				copy.key["lastHeight"] = 3000
			}
			s := m793RPCServer(t, copy, func(method string, v any) any {
				if method == "eth_getBlockByNumber" {
					if name == "canonical" {
						v.(map[string]any)["hash"] = "0x" + strings.Repeat("11", 32)
					}
					if name == "inclusion" {
						v.(map[string]any)["transactions"] = []string{}
					}
				}
				return v
			})
			if _, e := ReadCommittedTransaction(context.Background(), s.Client(), s.URL, c.hash); e == nil {
				t.Fatal("bad committed evidence accepted")
			}
		})
	}
	var output bytes.Buffer
	if CheckArchiveDelivery(context.Background(), s.Client(), s.URL, []byte("not signed"), &output) == nil {
		t.Fatal("invalid transaction submitted")
	}
	m793Source(t, "check-live.sh", `archive-delivery "$origin" "$transaction"`, `CHECK_LIVE_RELAY_ORIGIN`)
}

func TestRepairM_793_InteropChainTransfer(t *testing.T) {
	c := m793Chain125Peer(t)
	node := m793RPCServer(t, c, nil)
	decoded, e := DecodeSignedTransaction(c.raw)
	if e != nil {
		t.Fatal(e)
	}
	now := time.UnixMilli(int64(new(big.Int).SetBytes(decoded.Data()[156:164]).Uint64()) - 1000)
	tx, facts, e := InspectPayExact(c.raw, now)
	if e != nil {
		t.Fatal(e)
	}
	receiptBytes, _ := hex.DecodeString(strings.TrimPrefix(c.native["receipt"].(string), "0x"))
	receipt, e := codec.DecodeReceipt(receiptBytes)
	if e != nil {
		t.Fatal(e)
	}
	digest := receipt.Digest()
	for _, broken := range []string{"", "nonce", "offer", "payload", "receipt", "digest"} {
		t.Run("case-"+broken, func(t *testing.T) {
			seen := map[string]bool{}
			keys := map[string]bool{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if r.Method == "POST" {
					if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
						http.Error(w, "body", 400)
						return
					}
				}
				if r.URL.Path == "/gateway" {
					if r.Header.Get("Authorization") != "" {
						t.Error("facilitator credential leaked to public gateway")
					}
					method, _ := body["method"].(string)
					seen[method] = true
					var result any
					switch method {
					case "px_getAccount":
						nonce := fmt.Sprintf("0x%x", tx.Nonce())
						if broken == "nonce" {
							nonce = "0xffff"
						}
						result = map[string]any{"account": map[string]any{"evm_address": facts.Sender.Hex(), "bound": true}, "paxeer": map[string]any{"address": facts.Sender.Hex(), "balance": "0xffffffffffff", "nonce": nonce, "height": 10}}
					case "px_getBalances":
						result = map[string]any{"account": map[string]any{"evm_address": facts.Sender.Hex()}, "height": 10, "balances": []any{map[string]any{"denom": "uhpx", "amount": "10000000000000"}}}
					default:
						t.Errorf("retired/unexpected API %s", method)
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
					return
				}
				if r.Header.Get("Authorization") != "LayerX-Key fixture-custody" {
					t.Error("facilitator authorization lost")
				}
				seen[r.URL.Path] = true
				if r.Method == "POST" {
					key := r.Header.Get("Idempotency-Key")
					if key == "" || keys[key] {
						t.Error("operation key missing or reused")
					}
					keys[key] = true
				}
				var result any
				switch r.URL.Path {
				case "/v1/http/x402/supported":
					result = map[string]any{"supported": map[string]any{"kinds": []any{map[string]any{"scheme": "exact", "network": "eip155:125"}}}}
				case "/v1/http/x402/seller/offer":
					if broken == "offer" {
						body["accepts"].([]any)[0].(map[string]any)["amount"] = "999"
					}
					result = map[string]any{"status": 402, "payment_required": body, "payment_required_header": base64.StdEncoding.EncodeToString(m793JSON(t, body))}
				case "/v1/http/x402/buyer/build":
					scheme := body["scheme_payload"].(map[string]any)
					if scheme["layerxTransaction"] != "0x"+hex.EncodeToString(c.raw) || scheme["layerxIdempotencyKey"] != facts.Reference {
						t.Error("actual signed transfer lost")
					}
					if broken == "payload" {
						scheme["layerxIdempotencyKey"] = strings.Repeat("0", 64)
					}
					payment := map[string]any{"x402Version": 2, "accepted": body["payment_required"].(map[string]any)["accepts"].([]any)[0], "payload": scheme}
					result = map[string]any{"payment_payload": payment, "payment_header": base64.StdEncoding.EncodeToString(m793JSON(t, payment))}
				case "/v1/http/x402/settle":
					signed := c.native["receipt"].(string)
					dg := "0x" + hex.EncodeToString(digest[:])
					if broken == "receipt" {
						signed = "0x00"
					}
					if broken == "digest" {
						dg = "0x" + strings.Repeat("00", 32)
					}
					result = map[string]any{"success": true, "transaction": c.hash, "extensions": map[string]any{"layerx": map[string]any{"signedReceipt": signed, "receiptDigest": dg}}}
				default:
					http.Error(w, "unexpected", 400)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"result": result})
			}))
			defer server.Close()
			var out bytes.Buffer
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			e := CheckInteropPayment(ctx, server.Client(), server.URL, server.URL+"/gateway", node.URL, "fixture-custody", c.raw, now, &out)
			if broken != "" {
				m793Reject(t, e, &out)
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			for _, method := range []string{"px_getAccount", "px_getBalances", "/v1/http/x402/settle"} {
				if !seen[method] {
					t.Fatalf("missing real journey %s", method)
				}
			}
			if !strings.Contains(out.String(), "amount="+facts.Amount) || !strings.Contains(out.String(), "transaction="+c.hash) {
				t.Fatal(out.String())
			}
		})
	}
	if _, _, e = InspectPayExact(c.raw, time.UnixMilli(int64(facts.Expires)+1)); e == nil {
		t.Fatal("expired transfer accepted")
	}
	m793Source(t, "check-live.sh", `interop-payment "$origin" "$gateway" "$node" "$transaction" "$key"`)
}

func TestRepairM_793_IndexerDurableCursors(t *testing.T) {
	original := m793Fixture(t, "indexer-state.json")
	db := m793Fixture(t, "indexer.sqlite")
	path := filepath.Join(t.TempDir(), "cursor.sqlite")
	if e := os.WriteFile(path, db, 0600); e != nil {
		t.Fatal(e)
	}
	// Close and reopen the committed database file, then run the same decoder
	// and consumer used by the live CLI against recovered bytes.
	for restart := 0; restart < 2; restart++ {
		f, e := os.Open(path)
		if e != nil {
			t.Fatal(e)
		}
		recovered, e := io.ReadAll(f)
		closeErr := f.Close()
		if e != nil || closeErr != nil {
			t.Fatal(e, closeErr)
		}
		rows, e := ReadSQLiteCursors(recovered)
		if e != nil {
			t.Fatal(e)
		}
		if rows["cursors"].Position != 21 || rows["backfill_cursors"].Position != 20 {
			t.Fatal(rows)
		}
		state := m793Map(t, original)
		state["database"] = base64.StdEncoding.EncodeToString(recovered)
		var out bytes.Buffer
		if e = RunInspector(context.Background(), []string{"indexer-state"}, bytes.NewReader(m793JSON(t, state)), &out); e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(out.String(), "database=consistent") {
			t.Fatal(out.String())
		}
	}
	for _, name := range []string{"done", "cutover", "live", "backfill_hash", "retained_start", "database"} {
		t.Run(name, func(t *testing.T) {
			v := m793Map(t, original)
			switch name {
			case "done":
				v[name] = false
			case "cutover":
				v[name] = 22
			case "live":
				v[name] = nil
			case "backfill_hash":
				v[name] = "wrong"
			case "retained_start":
				v[name] = 2
			case "database":
				v[name] = base64.StdEncoding.EncodeToString([]byte("corrupt"))
			}
			var out bytes.Buffer
			m793Reject(t, ValidateIndexerState(bytes.NewReader(m793JSON(t, v)), &out), &out)
		})
	}
	for _, bad := range [][]byte{nil, db[:100], append([]byte("notSQLite format"), db[16:]...)} {
		if _, e := ReadSQLiteCursors(bad); e == nil {
			t.Fatal("corrupt database accepted")
		}
	}
	m793Source(t, "check-live.sh", `for side in EVM COMET`, `CREATE TABLE evidence.cursors AS SELECT`, `main.backfill_cursors`, `px_getUnifiedHistory px_getHistory`, `indexer-state`)
	m793Source(t, "../../services/platform/hosted/indexer/env-init.sh", `SELECT position FROM backfill_cursors`, `backfill-done.new`, `"$position" = "$cutover"`, `"$(cat "$state_dir/backfill-done")" = "$cutover"`)
}

func TestRepairM_793_WebhookFreshSource(t *testing.T) {
	for _, broken := range []string{"", "stale", "event", "subject", "ack", "pending"} {
		t.Run("case-"+broken, func(t *testing.T) {
			keys := map[string]bool{}
			sources := 0
			var receiver string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					key := r.Header.Get("Idempotency-Key")
					if len(key) < 64 || keys[key] {
						t.Error("operation correlation absent or reused")
					}
					keys[key] = true
				}
				if r.URL.Path == "/v1/intents/submit" {
					if r.Header.Get("Authorization") != "Bearer source-session" {
						t.Error("source not authorized")
					}
					var request map[string]any
					_ = json.NewDecoder(r.Body).Decode(&request)
					if request["signature"] != "recorded-authorized-intent" {
						t.Error("source body changed")
					}
					sources++
					_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"journey_id": "journey-current"}})
					return
				}
				if r.Header.Get("Authorization") != "Bearer hooks-session" {
					t.Error("webhook authorization lost")
				}
				switch r.URL.Path {
				case "/v1/webhooks/endpoints":
					if r.Method == "POST" {
						var request struct {
							URL string `json:"url"`
						}
						_ = json.NewDecoder(r.Body).Decode(&request)
						receiver = request.URL
						w.WriteHeader(201)
						_, _ = io.WriteString(w, `{"endpoint":"endpoint-probe"}`)
					} else {
						_ = json.NewEncoder(w).Encode([]any{map[string]any{"endpoint": "endpoint-probe", "url": receiver}})
					}
				case "/v1/webhooks/events":
					if sources == 0 {
						t.Error("events read before source submission")
					}
					b := m793Fixture(t, "events.json")
					if broken == "stale" {
						b = bytes.ReplaceAll(b, []byte("1750000001"), []byte("1749999999"))
					}
					_, _ = w.Write(b)
				case "/v1/webhooks/deliveries":
					b := m793Fixture(t, "deliveries.json")
					switch broken {
					case "event":
						b = bytes.ReplaceAll(b, []byte("event-current"), []byte("event-historical"))
					case "subject":
						b = bytes.ReplaceAll(b, []byte("journey-current"), []byte("other-journey"))
					case "ack":
						b = bytes.ReplaceAll(b, []byte("204"), []byte("503"))
					case "pending":
						b = bytes.ReplaceAll(b, []byte("delivered"), []byte("pending"))
					}
					_, _ = w.Write(b)
				default:
					http.Error(w, "unexpected path", 400)
				}
			}))
			defer server.Close()
			now := func() time.Time { return time.Unix(1750000002, 0) }
			// Fixtures occurred one second after operation start; the end clock is
			// advanced on the validation read to assert the bounded current window.
			for run := 0; run < 2; run++ {
				reads := 0
				clock := func() time.Time {
					reads++
					if reads == 1 {
						return time.Unix(1750000000, 0)
					}
					return now()
				}
				var out bytes.Buffer
				e := CheckDevelopersDelivery(context.Background(), server.Client(), server.URL, server.URL+"/receiver", server.URL+"/v1/intents/submit", "hooks-session", "source-session", json.RawMessage(`{"signature":"recorded-authorized-intent"}`), 1, clock, &out)
				if broken != "" {
					m793Reject(t, e, &out)
				} else if e != nil || !strings.Contains(out.String(), "event=event-current delivery=delivery-current") {
					t.Fatalf("fresh delivery lost: %v %s", e, &out)
				}
			}
			if sources != 2 || len(keys) != 4 {
				t.Fatalf("source correlation not unique: sources=%d keys=%d", sources, len(keys))
			}
		})
	}
	m793Source(t, "check-live.sh", `developers-delivery`, `CHECK_LIVE_DEVELOPERS_SOURCE_REQUEST`, `CHECK_LIVE_DEVELOPERS_SOURCE_TOKEN_FILE`)
}

func TestRepairM_793_SearchHostedAssetPayment(t *testing.T) {
	assets := m793Fixture(t, "assets.json")
	asset := m793Map(t, assets)["assets"].([]any)[0].(map[string]any)["asset_id"].(string)
	for _, broken := range []string{"", "price", "asset", "decimals", "paused", "backend", "receipt"} {
		t.Run("case-"+broken, func(t *testing.T) {
			reads, paid := 0, 0
			grant := strings.Repeat("ab", 32)
			payer := "0x" + strings.Repeat("12", 20)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/gateway" {
					var req struct {
						Method string `json:"method"`
					}
					_ = json.NewDecoder(r.Body).Decode(&req)
					if req.Method != "px_listAssets" {
						t.Errorf("retired asset API %s", req.Method)
					}
					reads++
					a := m793Map(t, assets)
					row := a["assets"].([]any)[0].(map[string]any)
					if broken == "decimals" {
						row["decimals"] = 6
					}
					if broken == "paused" {
						row["paused"] = true
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": a})
					return
				}
				if r.URL.Path != "/search" || r.URL.Query().Get("q") != "repair" || r.Header.Get("LAYERX-PAYER-ADDRESS") != payer {
					t.Error("current payer/route contract lost")
				}
				offer := map[string]any{"scheme": "metered", "network": "eip155:125", "amount": "74404761904762", "asset": asset, "payTo": strings.Repeat("34", 32), "extra": map[string]any{"layerx": map[string]any{"currency": "PAX"}}}
				if broken == "price" {
					offer["amount"] = "1"
				}
				if broken == "asset" {
					offer["asset"] = strings.Repeat("00", 32)
				}
				w.Header().Set("X-Search-Node", "selected-node")
				if r.Header.Get("PAYMENT-SIGNATURE") == "" {
					w.Header().Set("PAYMENT-REQUIRED", base64.StdEncoding.EncodeToString(m793JSON(t, map[string]any{"x402Version": 2, "accepts": []any{offer}})))
					w.WriteHeader(402)
					return
				}
				paid++
				raw, e := base64.StdEncoding.DecodeString(r.Header.Get("PAYMENT-SIGNATURE"))
				if e != nil {
					t.Error(e)
				}
				p := m793Map(t, raw)
				payload := p["payload"].(map[string]any)
				if len(payload) != 2 || payload["grantId"] != grant || len(payload["idempotencyKey"].(string)) != 64 || !equalJSON(m793JSON(t, p["accepted"]), m793JSON(t, offer)) {
					t.Error("metered standing grant payment payload mismatch")
				}
				if broken == "backend" {
					w.Header().Set("X-Search-Node", "other-node")
				}
				transaction := "0x" + strings.Repeat("55", 32)
				if broken == "receipt" {
					transaction = "legacy-digest"
				}
				w.Header().Set("PAYMENT-RESPONSE", base64.StdEncoding.EncodeToString(m793JSON(t, map[string]any{"success": true, "transaction": transaction})))
				_, _ = io.WriteString(w, `{"results":[]}`)
			}))
			defer server.Close()
			var out bytes.Buffer
			e := CheckSearchPayment(context.Background(), server.Client(), server.URL+"/search?q=repair", server.URL+"/gateway", payer, grant, "", &out)
			if broken != "" {
				m793Reject(t, e, &out)
			} else if e != nil || reads != 1 || paid != 1 || !strings.Contains(out.String(), "payment-response=success") {
				t.Fatalf("paid journey failed: %v reads=%d paid=%d %s", e, reads, paid, &out)
			}
		})
	}
	m793Source(t, "check-live.sh", `search-payment`, `CHECK_LIVE_SEARCH_PAYER`, `CHECK_LIVE_SEARCH_GRANT_ID`, `CHECK_LIVE_GATEWAY_RPC`)
}
