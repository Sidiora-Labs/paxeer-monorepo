package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestMarketHintScanUsesRecordedIndividualReceiptFallback(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"individual receipts", "receipt unavailable", "bulk unavailable", "receipt count", "merged bloom"} {
		t.Run(mode, func(t *testing.T) {
			fixture := chainfixture.New(t)
			fixture.Mutate = func(key string, raw json.RawMessage) json.RawMessage {
				if mode == "receipt count" && strings.HasPrefix(key, "eth_getBlockReceipts ") {
					return json.RawMessage(`[]`)
				}
				if mode == "merged bloom" && strings.HasPrefix(key, "eth_getBlockByNumber ") {
					var block map[string]json.RawMessage
					if e := json.Unmarshal(raw, &block); e != nil {
						t.Error(e)
						return raw
					}
					block["logsBloom"], _ = json.Marshal("0x" + strings.Repeat("0", 512))
					changed, e := json.Marshal(block)
					if e != nil {
						t.Error(e)
						return raw
					}
					return changed
				}
				return raw
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, e := io.ReadAll(io.LimitReader(r.Body, 1<<20))
				if e != nil {
					t.Error(e)
					return
				}
				var call struct {
					ID     json.RawMessage
					Method string
				}
				if e = json.Unmarshal(body, &call); e != nil {
					t.Error(e)
					return
				}
				code := 0
				if call.Method == "eth_getBlockReceipts" {
					switch mode {
					case "individual receipts", "receipt unavailable":
						code = -32601
					case "bulk unavailable":
						code = -32000
					}
				}
				if mode == "receipt unavailable" && call.Method == "eth_getTransactionReceipt" {
					code = -32000
				}
				if code != 0 {
					if e = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": call.ID, "error": map[string]any{"code": code, "message": "receipt method unavailable"}}); e != nil {
						t.Error(e)
					}
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
				fixture.ServeHTTP(w, r)
			}))
			defer server.Close()
			client, e := ethclient.DialContext(ctx, server.URL)
			if e != nil {
				t.Fatal(e)
			}
			defer client.Close()
			source := recoveringQualifiedSource{client: client, blockIdentity: receipt.BlockIdentityReported}
			hints, blocks, e := source.marketAddresses(ctx, 33963716, 33963716)
			if mode == "individual receipts" {
				if e != nil || len(hints) == 0 || len(blocks) != 1 || blocks[33963716].TransactionCount != 1 {
					t.Fatalf("individual receipt hints %d %+v: %v", len(hints), blocks, e)
				}
			} else if e == nil {
				t.Fatal("unavailable or misbound receipts admitted market hints")
			}
		})
	}
}
