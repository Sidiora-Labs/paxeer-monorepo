package bringup

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

type searchExactOptions struct {
	challengeScheme string
	duplicate       bool
	serveReceipt    bool
	paidStatus      int
	keyMode         os.FileMode
	payerAddress    string
}

func searchExactJourney(t *testing.T, options searchExactOptions) (error, string, string) {
	t.Helper()
	keyText := strings.Repeat("0", 63) + "1"
	keyFile := filepath.Join(t.TempDir(), "payer.key")
	if err := os.WriteFile(keyFile, []byte(keyText+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if options.keyMode != 0 {
		if err := os.Chmod(keyFile, options.keyMode); err != nil {
			t.Fatal(err)
		}
	}
	key, err := gethcrypto.HexToECDSA(keyText)
	if err != nil {
		t.Fatal(err)
	}
	payer := gethcrypto.PubkeyToAddress(key.PublicKey)
	if options.payerAddress == "" {
		options.payerAddress = payer.Hex()
	}
	assets := liveEvidenceMap(t, liveEvidenceFixture(t, "assets.json"))
	asset := assets["assets"].([]any)[0].(map[string]any)["asset_id"].(string)
	offer := map[string]any{
		"scheme": "exact", "network": "eip155:125", "amount": "74404761904762",
		"asset": asset, "payTo": strings.Repeat("34", 32), "maxTimeoutSeconds": 2,
		"extra": map[string]any{"layerx": map[string]any{"currency": "PAX"}},
	}
	if options.challengeScheme != "" {
		offer["scheme"] = options.challengeScheme
	}
	var sentHash string
	var payload map[string]any
	chain := liveEvidenceChain125Peer(t)
	chain.native["rung"] = "receipted"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			w.Header().Set("X-Search-Node", "selected-node")
			if r.Header.Get("PAYMENT-SIGNATURE") == "" {
				body, _ := json.Marshal(map[string]any{"x402Version": 2, "accepts": func() []any {
					items := []any{offer}
					if options.duplicate {
						items = append(items, offer)
					}
					return items
				}()})
				w.Header().Set("PAYMENT-REQUIRED", base64.StdEncoding.EncodeToString(body))
				w.WriteHeader(http.StatusPaymentRequired)
				return
			}
			raw, decodeErr := base64.StdEncoding.DecodeString(r.Header.Get("PAYMENT-SIGNATURE"))
			if decodeErr == nil {
				var payment map[string]any
				if json.Unmarshal(raw, &payment) == nil {
					payload, _ = payment["payload"].(map[string]any)
				}
			}
			w.Header().Set("PAYMENT-RESPONSE", base64.StdEncoding.EncodeToString([]byte(`{"success":true,"transaction":"`+sentHash+`"}`)))
			if options.paidStatus != 0 {
				w.WriteHeader(options.paidStatus)
				return
			}
			_, _ = io.WriteString(w, `{"results":[]}`)
			return
		}
		if r.URL.Path == "/gateway" {
			var request struct {
				Method string `json:"method"`
			}
			if json.NewDecoder(r.Body).Decode(&request) != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if request.Method == "px_listAssets" {
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": assets})
				return
			}
			if request.Method == "pax_sequencerReceipt" && options.serveReceipt && sentHash != "" {
				chain.native["txHash"] = sentHash
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": chain.native})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": nil})
			return
		}
		if r.URL.Path == "/node" {
			var request struct {
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if json.NewDecoder(r.Body).Decode(&request) != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			result := any("0x7d")
			switch request.Method {
			case "eth_getTransactionCount":
				result = "0x7"
			case "eth_gasPrice":
				result = "0x1"
			case "eth_estimateGas":
				result = "0x33450"
			case "eth_sendRawTransaction":
				var params []string
				_ = json.Unmarshal(request.Params, &params)
				raw, decodeErr := hex.DecodeString(strings.TrimPrefix(params[0], "0x"))
				if decodeErr != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				var tx types.Transaction
				if tx.UnmarshalBinary(raw) != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				data := tx.Data()
				if tx.To() == nil || !strings.EqualFold(tx.To().Hex(), searchLedgerPrecompile) || len(data) != 196 || hex.EncodeToString(data[:4]) != hex.EncodeToString(gethcrypto.Keccak256([]byte("payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)"))[:4]) {
					t.Errorf("payExact transaction shape mismatch")
				}
				if hex.EncodeToString(data[68:100]) != asset || hex.EncodeToString(data[36:68]) != strings.Repeat("34", 32) || new(big.Int).SetBytes(data[116:132]).String() != offer["amount"].(string) {
					t.Errorf("payExact calldata does not carry the offer")
				}
				sentHash = tx.Hash().Hex()
				result = sentHash
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	var output bytes.Buffer
	err = CheckSearchPayment(context.Background(), server.Client(), server.URL+"/search?q=repair", server.URL+"/gateway", server.URL+"/node", keyFile, options.payerAddress, "", &output)
	if err == nil && payload == nil {
		t.Errorf("paid request did not carry an exact payload")
	}
	return err, output.String(), sentHash
}

func TestSearchExactFullPath(t *testing.T) {
	err, output, hash := searchExactJourney(t, searchExactOptions{serveReceipt: true})
	if err != nil || hash == "" || !strings.Contains(output, "payment-response=success") || !strings.Contains(output, "transaction="+hash) {
		t.Fatalf("exact payment failed: %v %s", err, output)
	}
}

func TestSearchExactCalldataMatchesOffer(t *testing.T) {
	err, _, hash := searchExactJourney(t, searchExactOptions{serveReceipt: true})
	if err != nil || hash == "" {
		t.Fatalf("exact calldata path failed: %v", err)
	}
}

func TestSearchExactMeteredChallengeRefused(t *testing.T) {
	err, _, _ := searchExactJourney(t, searchExactOptions{challengeScheme: "metered"})
	if err == nil {
		t.Fatal("metered challenge accepted")
	}
}

func TestSearchExactDuplicateOffersRefused(t *testing.T) {
	err, _, _ := searchExactJourney(t, searchExactOptions{duplicate: true})
	if err == nil {
		t.Fatal("ambiguous exact offers accepted")
	}
}

func TestSearchExactReceiptBound(t *testing.T) {
	err, _, _ := searchExactJourney(t, searchExactOptions{serveReceipt: false})
	if err == nil || !strings.Contains(err.Error(), "receipt deadline") {
		t.Fatalf("unserved receipt accepted: %v", err)
	}
}

func TestSearchExactPaidAnswerStatus(t *testing.T) {
	err, _, _ := searchExactJourney(t, searchExactOptions{serveReceipt: true, paidStatus: http.StatusBadGateway})
	if err == nil {
		t.Fatal("non-200 paid answer accepted")
	}
}

func TestSearchExactKeyPermissions(t *testing.T) {
	err, _, _ := searchExactJourney(t, searchExactOptions{keyMode: 0o640})
	if err == nil {
		t.Fatal("group-readable payer key accepted")
	}
}

func TestSearchExactPayerAddress(t *testing.T) {
	err, _, _ := searchExactJourney(t, searchExactOptions{payerAddress: "0x" + strings.Repeat("22", 20)})
	if err == nil {
		t.Fatal("mismatched payer address accepted")
	}
}
