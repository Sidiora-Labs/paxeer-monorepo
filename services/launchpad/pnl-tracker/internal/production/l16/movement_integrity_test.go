package l16

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

func TestIndexedMovementsRequireCanonicalLogOrderAndContent(t *testing.T) {
	ctx := context.Background()
	s, pool := recordedRebuildService(t, ctx)
	origin := repair63RecordedRPC(t)
	for _, tc := range []struct {
		name   string
		mutate func([]any)
		live   bool
	}{
		{"removed", func(logs []any) { logs[0].(map[string]any)["removed"] = true }, true},
		{"emitter", func(logs []any) { logs[0].(map[string]any)["address"] = "0x" + strings.Repeat("e", 40) }, true},
		{"block binding", func(logs []any) { logs[0].(map[string]any)["blockHash"] = "0x" + strings.Repeat("e", 64) }, true},
		{"quantity binding", func(logs []any) { logs[0].(map[string]any)["data"] = "0x" + strings.Repeat("0", 63) + "1" }, true},
		{"duplicate", func(logs []any) { logs[1] = logs[0] }, true},
		{"order", func(logs []any) { logs[0], logs[1] = logs[1], logs[0] }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, hits := inventoryProviderVariant(t, origin, func(r inventoryRPCRequest) bool { return r.Method == "eth_getLogs" }, func(out map[string]any) {
				logs := out["result"].([]any)
				if len(logs) < 2 {
					t.Error("creation transfers absent")
					return
				}
				tc.mutate(logs)
			})
			a := *s.authority
			a.rpc = client
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer inventoryRollback(tx)
			if _, _, _, err = a.verifiedMovements(ctx, tx, realMarketToken, 33963716, 33963733); !errors.Is(err, delivery.ErrIntegrity) {
				t.Fatal(err)
			}
			if tc.live {
				if _, _, _, err = a.liveMovementPages(ctx, tx, realMarketToken, 33963716, 33963733); !errors.Is(err, delivery.ErrIntegrity) {
					t.Fatal(err)
				}
			}
			if hits.Load() == 0 {
				t.Fatal("canonical log read absent")
			}
		})
	}
	for _, bounds := range [][2]int64{{0, 1}, {10, 9}, {1, 10002}} {
		if _, err := s.authority.coverage(ctx, nil, bounds[0], bounds[1]); !errors.Is(err, ErrBaselineUnavailable) {
			t.Fatal(bounds, err)
		}
	}
}

func TestReceiptLogRecoveryRejectsConflictingBlockAndReceipt(t *testing.T) {
	ctx := context.Background()
	s, _ := recordedRebuildService(t, ctx)
	origin := repair63RecordedRPC(t)
	for _, tc := range []struct {
		name, method string
		mutate       func(map[string]any)
		want         error
	}{
		{"block unavailable", "eth_getBlockByNumber", func(out map[string]any) {
			delete(out, "result")
			out["error"] = map[string]any{"code": -32000, "message": "block unavailable"}
		}, nil},
		{"receipt unavailable", "eth_getTransactionReceipt", func(out map[string]any) {
			delete(out, "result")
			out["error"] = map[string]any{"code": -32000, "message": "receipt unavailable"}
		}, nil},
		{"receipt block", "eth_getTransactionReceipt", func(out map[string]any) { out["result"].(map[string]any)["blockHash"] = "0x" + strings.Repeat("e", 64) }, delivery.ErrIntegrity},
		{"nil log", "eth_getTransactionReceipt", func(out map[string]any) { out["result"].(map[string]any)["logs"] = []any{nil} }, delivery.ErrIntegrity},
		{"log transaction", "eth_getTransactionReceipt", func(out map[string]any) {
			for _, log := range out["result"].(map[string]any)["logs"].([]any) {
				log.(map[string]any)["transactionHash"] = "0x" + strings.Repeat("e", 64)
			}
		}, delivery.ErrIntegrity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, hits := inventoryProviderVariant(t, origin, func(r inventoryRPCRequest) bool { return r.Method == "eth_getLogs" || r.Method == tc.method }, func(out map[string]any) {
				if _, ok := out["result"].([]any); ok {
					delete(out, "result")
					out["error"] = map[string]any{"code": -32000, "message": "use receipts"}
					return
				}
				tc.mutate(out)
			})
			a := *s.authority
			a.rpc = client
			logs, err := a.liveTransferLogs(ctx, realMarketToken, 33963716, 33963716)
			if err == nil || tc.want != nil && !errors.Is(err, tc.want) || len(logs) != 0 || hits.Load() < 2 {
				t.Fatal(logs, err, hits.Load())
			}
		})
	}
	// Recursive range recovery propagates the failing half of a block interval.
	for _, failLeft := range []bool{true, false} {
		client, _ := inventoryProviderVariant(t, origin, func(r inventoryRPCRequest) bool {
			return r.Method == "eth_getLogs" || r.Method == "eth_getBlockByNumber"
		}, func(out map[string]any) {
			if _, ok := out["result"].([]any); ok {
				delete(out, "result")
				out["error"] = map[string]any{"code": -32000, "message": "use receipts"}
				return
			}
			block := out["result"].(map[string]any)
			number := block["number"].(string)
			if (number == "0x2063ec4") == failLeft {
				delete(out, "result")
				out["error"] = map[string]any{"code": -32000, "message": "block unavailable"}
			}
		})
		a := *s.authority
		a.rpc = client
		if logs, err := a.liveTransferLogs(ctx, realMarketToken, 33963716, 33963717); err == nil || len(logs) != 0 {
			t.Fatal(logs, err)
		}
	}
}

func TestFactorySupplyRequiresCompleteOrderedMintAndSeed(t *testing.T) {
	ctx := context.Background()
	s, _ := recordedRebuildService(t, ctx)
	logs, err := s.authority.rpc.FilterLogs(ctx, ethereum.FilterQuery{FromBlock: big.NewInt(33963716), ToBlock: big.NewInt(33963716), Addresses: []common.Address{common.HexToAddress(realMarketToken)}, Topics: [][]common.Hash{{transferTopic}}})
	if err != nil {
		t.Fatal(err)
	}
	moves := make([]movement, len(logs))
	for i, log := range logs {
		moves[i], err = decodeTransfer(log)
		if err != nil {
			t.Fatal(err)
		}
	}
	supply, err := factoryCreationSupply(moves, realCreationTx, realMarketFactory, realMarketPool)
	if err != nil || supply.String() != "1000000000000000" {
		t.Fatal(supply, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*[]movement, *string, *string, *string)
	}{
		{"factory", func(_ *[]movement, _ *string, factory, _ *string) { *factory = "bad" }},
		{"transaction", func(_ *[]movement, tx, _, _ *string) { *tx = "bad" }},
		{"mint recipient", func(m *[]movement, _, _, _ *string) { (*m)[0].To = realMarketPool }},
		{"mint amount", func(m *[]movement, _, _, _ *string) { (*m)[0].Amount = "bad"; (*m)[1].Amount = "bad" }},
		{"zero supply", func(m *[]movement, _, _, _ *string) { (*m)[0].Amount = "0"; (*m)[1].Amount = "0" }},
		{"predecessor", func(m *[]movement, _, _, _ *string) {
			prior := (*m)[0]
			prior.TransactionHash = "0x" + strings.Repeat("e", 64)
			prior.LogIndex = 0
			*m = append(*m, prior)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := append([]movement(nil), moves...)
			tx, factory, pool := realCreationTx, realMarketFactory, realMarketPool
			tc.change(&candidate, &tx, &factory, &pool)
			if supply, err := factoryCreationSupply(candidate, tx, factory, pool); !errors.Is(err, ErrBaselineUnavailable) || supply != nil {
				t.Fatal(supply, err)
			}
		})
	}
}
