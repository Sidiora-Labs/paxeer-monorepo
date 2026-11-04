package bringup

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
)

// The peer replays the committed native RPC recording, including the original
// transaction, receipt signature, key range and block identity. Only negative
// cases alter evidence. The inspector and its codecs run in this process.
func m1388Peer(t *testing.T, variant string) (*httptest.Server, m793Chain, string, *[]string) {
	t.Helper()
	c := m793RecordedChain(t)
	var tx types.Transaction
	if err := tx.UnmarshalBinary(c.raw); err != nil {
		t.Fatal(err)
	}
	if tx.Hash().Hex() != c.hash {
		t.Fatal("recorded transaction and committed receipt disagree")
	}
	chain := tx.ChainId().String()
	var calls []string
	chainReads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if r.URL.Path != "/" || json.NewDecoder(r.Body).Decode(&request) != nil {
			http.Error(w, "invalid root RPC request", 400)
			return
		}
		calls = append(calls, request.Method)
		answer, ok := c.rpc(request.Method, request.Params)
		if !ok {
			t.Errorf("unexpected RPC method or submission bytes: %s", request.Method)
			http.Error(w, "unexpected RPC", 400)
			return
		}
		if request.Method == "eth_chainId" {
			chainReads++
			answer = "0x" + tx.ChainId().Text(16)
			if variant == "wrong-chain" || (variant == "chain-changed" && chainReads > 1) {
				answer = "0x1"
			}
		}
		switch request.Method {
		case "eth_getTransactionReceipt":
			var params []string
			if json.Unmarshal(request.Params, &params) != nil || len(params) != 1 || params[0] != c.hash {
				t.Error("receipt read lost the submitted transaction hash")
			}
			if variant == "reverted" {
				answer.(map[string]any)["status"] = "0x0"
			}
		case "eth_getBlockByNumber":
			if variant == "not-in-block" {
				answer.(map[string]any)["transactions"] = []string{}
			}
		case "pax_sequencerReceipt":
			if variant == "uncommitted" {
				answer.(map[string]any)["rung"] = "receipted"
			}
			if variant == "signature" {
				encoded := answer.(map[string]any)["receipt"].(string)
				raw, err := hex.DecodeString(strings.TrimPrefix(encoded, "0x"))
				if err != nil {
					t.Error(err)
				}
				raw[len(raw)-1] ^= 1
				answer.(map[string]any)["receipt"] = "0x" + hex.EncodeToString(raw)
			}
		case "px_getReceiptKeys":
			if variant == "key-range" {
				answer.(map[string]any)["keys"].([]any)[0].(map[string]any)["firstHeight"] = 1000000
			}
		case "eth_blockNumber":
			if variant == "head-behind" {
				answer = "0x0"
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": answer})
	}))
	t.Cleanup(server.Close)
	return server, c, chain, &calls
}

func TestRepairM_1388_NativeSubmissionAndCommittedReceipt(t *testing.T) {
	server, c, chain, calls := m1388Peer(t, "")
	params, err := json.Marshal([]string{"0x" + hex.EncodeToString(c.raw)})
	if err != nil {
		t.Fatal(err)
	}
	var submitted bytes.Buffer
	if err := RunInspector(context.Background(), []string{"rpc", server.URL, "eth_sendRawTransaction", string(params)}, nil, &submitted); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(submitted.String()) != c.hash {
		t.Fatal("native submission acknowledgement changed transaction identity")
	}
	for attempt := 0; attempt < 2; attempt++ {
		var out bytes.Buffer
		if err := RunInspector(context.Background(), []string{"committed-transaction", server.URL, c.hash, "-", chain}, bytes.NewReader(c.raw), &out); err != nil {
			t.Fatal(err)
		}
		var result map[string]string
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || result["hash"] != c.hash || result["block"] != c.block || result["height"] != c.height {
			t.Fatalf("committed evidence changed: %s, %v", out.Bytes(), err)
		}
	}
	wantRead := "eth_chainId,eth_getTransactionReceipt,eth_getBlockByNumber,pax_sequencerReceipt,px_getReceiptKeys,eth_blockNumber,eth_chainId"
	if got := strings.Join(*calls, ","); got != "eth_sendRawTransaction,"+wantRead+","+wantRead {
		t.Fatalf("submission or committed read-back path differs: %s", got)
	}
}

func TestRepairM_1388_RejectUnboundOrUncommittedEvidence(t *testing.T) {
	for _, variant := range []string{"wrong-chain", "chain-changed", "reverted", "not-in-block", "uncommitted", "signature", "key-range", "head-behind", "wrong-signed-chain", "wrong-hash", "malformed-transaction", "missing-transaction", "oversized-transaction", "tenant-rpc", "noncanonical-chain"} {
		t.Run(variant, func(t *testing.T) {
			server, c, chain, calls := m1388Peer(t, variant)
			raw, hash, endpoint := c.raw, c.hash, server.URL
			switch variant {
			case "wrong-signed-chain":
				chain = "1"
			case "wrong-hash":
				hash = "0x" + strings.Repeat("f", 64)
			case "malformed-transaction":
				raw = []byte{2, 0xff}
			case "missing-transaction":
				raw = nil
			case "oversized-transaction":
				raw = bytes.Repeat([]byte{1}, 1048577)
			case "tenant-rpc":
				endpoint += "/chain/7"
			case "noncanonical-chain":
				chain = "0" + chain
			}
			var out bytes.Buffer
			err := RunInspector(context.Background(), []string{"committed-transaction", endpoint, hash, "-", chain}, bytes.NewReader(raw), &out)
			if err == nil || out.Len() != 0 {
				t.Fatalf("invalid committed evidence reported success: %v %s", err, out.Bytes())
			}
			if variant == "wrong-chain" && strings.Join(*calls, ",") != "eth_chainId" {
				t.Fatalf("read evidence before checking chain identity: %v", *calls)
			}
		})
	}
}

func TestRepairM_1388_ScriptUsesNativeChainEvidence(t *testing.T) {
	raw, err := os.ReadFile("value-loop.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(raw)
	for _, retired := range []string{"layerxd.lni.sock", "layerxctl", "protocol 3", "receipt-authority", "authorized-batches", "as_lni", "latest_sealed_batch", `echo "batch `} {
		if strings.Contains(script, retired) {
			t.Fatalf("reachable retired submission dependency: %s", retired)
		}
	}
	for _, required := range []string{
		`layerx-node-probe handshake --rpc "$rpc" --trust-anchor "$trust_anchor" --network-id "$network_id"`,
		`jq -er '.latest_committed_height'`,
		`layerx-node-probe balance --rpc "$rpc" --trust-anchor "$trust_anchor" --network-id "$network_id"`,
		`--account "agent:$1:main" --asset "$asset"`,
		`layerx-node-probe "$send_command" --rpc "$rpc" --trust-anchor "$trust_anchor" --network-id "$network_id"`,
		`send_command=write-send`, `send_command=write-asset-send`,
		`send_journal reconcile`, `eth_sendRawTransaction`, `eth_getTransactionByHash`,
		`committed-transaction "$rpc" "0x$activity" "$SSL_CERT_FILE" "$network_id" <"$state/send.bin"`,
		`printf 'layerx-asset:%s:%s' "$network_id" "$symbol"`,
		`"$committed_height" -ge "$((inclusion_height))"`,
		`echo "block hash=$block committed_height=$committed_height"`,
	} {
		if !strings.Contains(script, required) {
			t.Error(fmt.Sprintf("native value-loop wiring missing %q", required))
		}
	}
	// The declared gate invokes the inspector as a Go function, never a
	// subprocess, installed toolchain, or environment-selected executable.
	var out bytes.Buffer
	if err := RunInspector(context.Background(), []string{"committed-transaction", "http://127.0.0.1", "0x" + strings.Repeat("0", 64), "-", "125"}, io.LimitReader(strings.NewReader(""), 1), &out); err == nil || out.Len() != 0 {
		t.Fatal("missing retained signed bytes did not refuse before transport")
	}
}
