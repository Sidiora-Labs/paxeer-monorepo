package l16

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestReceiptSettlementValidatesSourceBoundTransferEvidence(t *testing.T) {
	ctx := context.Background()
	s, pool := recordedRebuildService(t, ctx)
	origin := repair63RecordedRPC(t)
	var raw []byte
	if err := pool.QueryRow(ctx, `SELECT payload FROM indexer.indexer_accounting_fills_v1 ORDER BY log_index LIMIT 1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	source, err := accounting.DecodeFillV1(raw)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := s.authority.rpc.TransactionReceipt(ctx, common.HexToHash(source.Position.TransactionHash))
	if err != nil {
		t.Fatal(err)
	}
	receiptRaw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	tokenIndex, quoteIndex := -1, -1
	for i, l := range receipt.Logs {
		if l.Address == common.HexToAddress(source.Token.Address) && len(l.Topics) == 3 && l.Topics[0] == transferTopic {
			tokenIndex = i
		}
		if l.Address == common.HexToAddress(source.Settlement.Address) && len(l.Topics) == 3 && l.Topics[0] == transferTopic {
			quoteIndex = i
		}
	}
	if tokenIndex < 0 || quoteIndex < 0 {
		t.Fatal("canonical settlement transfers absent")
	}
	for _, tc := range []struct {
		name   string
		change func(*types.Receipt)
		want   error
	}{
		{"nil log", func(r *types.Receipt) { r.Logs = append(r.Logs, nil) }, delivery.ErrIntegrity},
		{"malformed token transfer", func(r *types.Receipt) { r.Logs[tokenIndex].Data = []byte{1} }, delivery.ErrIntegrity},
		{"transfer transaction", func(r *types.Receipt) { r.Logs[tokenIndex].TxHash = common.HexToHash("0x" + strings.Repeat("e", 64)) }, delivery.ErrIntegrity},
		{"ambiguous token transfer", func(r *types.Receipt) { r.Logs = append(r.Logs, r.Logs[tokenIndex]) }, ErrCostAllocationUnavailable},
		{"missing token transfer", func(r *types.Receipt) {
			r.Logs[tokenIndex].Address = common.HexToAddress("0x" + strings.Repeat("e", 40))
		}, ErrCostAllocationUnavailable},
		{"zero payer", func(r *types.Receipt) { r.Logs[quoteIndex].Topics[1] = common.Hash{} }, ErrCostAllocationUnavailable},
		{"unallocated debit", func(r *types.Receipt) {
			l := *r.Logs[quoteIndex]
			l.Index = uint(0)
			l.Data = common.LeftPadBytes(big.NewInt(1).Bytes(), 32)
			r.Logs = append(r.Logs, &l)
		}, ErrCostAllocationUnavailable},
		{"gas price", func(r *types.Receipt) { r.EffectiveGasPrice = nil }, delivery.ErrIntegrity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var candidate types.Receipt
			if err := json.Unmarshal(receiptRaw, &candidate); err != nil {
				t.Fatal(err)
			}
			tc.change(&candidate)
			mutated, err := json.Marshal(&candidate)
			if err != nil {
				t.Fatal(err)
			}
			fill := source
			fill.Provenance.ReceiptSHA256 = delivery.Hash(mutated)
			fillRaw, err := json.Marshal(fill)
			if err != nil {
				t.Fatal(err)
			}
			digest, err := accounting.FillDigest(fill)
			if err != nil {
				t.Fatal(err)
			}
			var envelope any
			if err = json.Unmarshal(mutated, &envelope); err != nil {
				t.Fatal(err)
			}
			client, hits := inventoryProviderVariant(t, origin, func(r inventoryRPCRequest) bool { return r.Method == "eth_getTransactionReceipt" }, func(out map[string]any) { out["result"] = envelope })
			a := *s.authority
			a.rpc = client
			for _, live := range []bool{false, true} {
				t.Run(map[bool]string{false: "single fill", true: "receipt page"}[live], func(t *testing.T) {
					tx, err := pool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					defer inventoryRollback(tx)
					if _, err = tx.Exec(ctx, `UPDATE indexer.indexer_accounting_fills SET payload=$1,source_fact_digest=$2 WHERE source_fill_id=$3`, fillRaw, string(digest), fill.SourceFillID); err != nil {
						t.Fatal(err)
					}
					if live {
						_, err = a.liveReceiptSettlement(ctx, tx, fill)
					} else {
						_, err = a.receiptSettlement(ctx, tx, fill)
					}
					if !errors.Is(err, tc.want) {
						t.Fatal(err, tc.want)
					}
				})
			}
			if hits.Load() != 2 {
				t.Fatal(hits.Load())
			}
		})
	}
}
