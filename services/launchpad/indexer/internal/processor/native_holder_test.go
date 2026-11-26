package processor

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/common"
)

func nativeHolderProducerFill(t *testing.T, ctx context.Context, tenant bool) (accounting.FillV1, receipt.Bundle) {
	t.Helper()
	raw, err := os.ReadFile("../receipt/testdata/producer_receipts.json")
	if err != nil {
		t.Fatal(err)
	}
	var input struct {
		Options                receipt.RPCOptions
		Markets                []receipt.DiscoveredMarket
		NativeSwap, TenantSwap common.Hash
		Responses              map[string]nativeAccountingRecorded
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	responses := map[string]nativeAccountingRecorded{}
	key := func(method string, params []json.RawMessage) string {
		if method == "eth_call" && len(params) == 2 {
			var call map[string]any
			if json.Unmarshal(params[0], &call) == nil && call["from"] == "0x0000000000000000000000000000000000000000" {
				delete(call, "from")
				params[0], _ = json.Marshal(call)
			}
		}
		return chainfixture.Key(method, params)
	}
	for encoded, response := range input.Responses {
		method, args, ok := strings.Cut(encoded, " ")
		var params []json.RawMessage
		if !ok || json.Unmarshal([]byte(args), &params) != nil {
			t.Fatal("invalid recorded RPC request")
		}
		responses[key(method, params)] = response
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage
			Method string
			Params []json.RawMessage
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		response, ok := responses[key(request.Method, request.Params)]
		if !ok {
			http.Error(w, "unrecorded producer read", http.StatusBadGateway)
			return
		}
		result := map[string]any{"jsonrpc": "2.0", "id": request.ID}
		if len(response.Error) > 0 {
			result["error"] = response.Error
		} else {
			result["result"] = response.Result
		}
		if err := json.NewEncoder(w).Encode(result); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client, closeClient, err := receipt.NewBoundedRPCClient(ctx, server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	defer closeClient()
	authority, err := receipt.NewRPCQualification(ctx, client, input.Options)
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.Rehydrate(ctx, input.Markets); err != nil {
		t.Fatal(err)
	}
	hash := input.NativeSwap
	if tenant {
		hash = input.TenantSwap
	}
	bundle, err := authority.Bundle(ctx, hash)
	if err != nil {
		t.Fatal(err)
	}
	batch, qualified, err := authority.FetchQualifiedBatch(ctx, bundle.Header.Number.Uint64(), bundle.Header.Number.Uint64())
	if err != nil {
		t.Fatal(err)
	}
	if err := receipt.ValidateBindings(ctx, batch, qualified); err != nil {
		t.Fatal(err)
	}
	for _, source := range qualified {
		transaction, _, _, err := source.SourceContext(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if transaction != strings.ToLower(hash.Hex()) {
			continue
		}
		fills, err := source.Fills(ctx)
		if err != nil || len(fills) != 1 {
			t.Fatalf("recorded native fill: %+v, %v", fills, err)
		}
		return fills[0], bundle
	}
	t.Fatal("recorded swap absent from qualified batch")
	return accounting.FillV1{}, receipt.Bundle{}
}

func TestNativeHolderAttributionInventorySameOwner(t *testing.T) {
	ctx := context.Background()
	h := newInventoryHarness(t, ctx)
	for name, tenant := range map[string]bool{"native": false, "tenant": true} {
		t.Run(name, func(t *testing.T) {
			fill, bundle := nativeHolderProducerFill(t, ctx, tenant)
			a := fill.Attribution
			if a.Quality != "qualified_calldata" || a.Payer == nil || a.Recipient == nil || *a.Payer != a.TransactionSender || *a.Recipient != a.TransactionSender {
				t.Fatalf("native same-owner attribution: %+v", a)
			}
			tx, err := h.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			canonical := store.CanonicalTx{Tx: tx}
			height := bundle.Header.Number.Int64()
			for _, log := range bundle.Receipt.Logs {
				if strconv.FormatUint(uint64(log.Index), 10) != fill.Position.LogIndex {
					continue
				}
				topics := make([]string, len(log.Topics))
				for i, topic := range log.Topics {
					topics[i] = strings.ToLower(topic.Hex())
				}
				rawTopics, err := json.Marshal(topics)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := tx.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,$2,$3,$4,$5,$6,$7,$8)`, fill.Position.TransactionHash, int64(log.Index), height, int64(log.TxIndex), fill.Position.BlockHash, fill.ActualEmitter, rawTopics, log.Data); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := canonical.InsertAccountingFill(ctx, fill); err != nil {
				t.Fatal(err)
			}
			batch := store.CanonicalBatch{ChainID: 125, StreamID: realStream, Owner: inventoryTestOwner, Fence: 1, PreviousHeight: height - 1, Blocks: []store.BlockCompleteness{{Height: height}}}
			_, _, err = qualifiedInventoryEnrichment(ctx, canonical, batch, fill, pnlGeneration, pnlInventoryVersion)
			var refusal *inventoryRefusal
			if !errors.As(err, &refusal) || !refusal.pending || refusal.code != "baseline-unissued" {
				t.Fatalf("native same-owner fill must reach baseline qualification: %v", err)
			}
			plans, err := planInventory(ctx, canonical, batch, pnlGeneration, pnlInventoryVersion, 1)
			if err != nil || len(plans) != 0 {
				t.Fatalf("native inventory waiting for baseline: %+v, %v", plans, err)
			}
			batch.PreviousHeight = height
			batch.Blocks[0].Height = height + 1
			plans, err = planInventory(ctx, canonical, batch, pnlGeneration, pnlInventoryVersion, 1)
			if err != nil || len(plans) != 0 {
				t.Fatalf("native inventory baseline deadline: %+v, %v", plans, err)
			}
			var requirement string
			if err := tx.QueryRow(ctx, `SELECT requirement FROM indexer.indexer_capability_degradation WHERE transaction_hash=$1`, fill.Position.TransactionHash).Scan(&requirement); err != nil || requirement != accounting.InventorySubject+":baseline-unissued-timeout" {
				t.Fatalf("native fill must be selected for inventory qualification: %s, %v", requirement, err)
			}
		})
	}
}
