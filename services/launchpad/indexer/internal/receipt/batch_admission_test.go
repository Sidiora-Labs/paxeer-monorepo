package receipt

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/ethereum/go-ethereum/common"
)

func TestIssuedReceiptsRequireCompleteUniqueBatchBindings(t *testing.T) {
	ctx := context.Background()
	input := repair53InputsFromProducer(t)
	a, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	if err := a.Rehydrate(ctx, input.Markets); err != nil {
		t.Fatal(err)
	}
	a = a.current()
	b, err := a.Bundle(ctx, input.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	batch, receipts, err := a.FetchQualifiedBatch(ctx, b.Header.Number.Uint64(), b.Header.Number.Uint64())
	if err != nil || len(receipts) != 1 {
		t.Fatalf("recorded batch %d: %v", len(receipts), err)
	}
	q := receipts[0]
	if err = q.ValidateBinding(ctx, batch); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"chain", "other transaction", "missing selected", "duplicate block", "incomplete block", "missing block", "missing receipt", "duplicate receipt", "unbounded receipts", "unbounded records", "raw height without block"} {
		t.Run(mode, func(t *testing.T) {
			changed, e := clone(batch)
			if e != nil {
				t.Fatal(e)
			}
			qualified := receipts
			switch mode {
			case "chain":
				changed.ChainID = 1
			case "other transaction":
				for i := range changed.Records {
					changed.Records[i].TransactionHash = common.HexToHash("0xbad").Hex()
				}
			case "missing selected":
				changed.Records = changed.Records[:len(changed.Records)-1]
			case "duplicate block":
				changed.Blocks = append(changed.Blocks, changed.Blocks[0])
			case "incomplete block":
				changed.Blocks[0].FetchComplete = false
			case "missing block":
				changed.Blocks = nil
			case "missing receipt":
				qualified = nil
			case "duplicate receipt":
				qualified = append(append([]*QualifiedReceipt(nil), receipts...), q)
			case "unbounded receipts":
				qualified = make([]*QualifiedReceipt, 4097)
			case "unbounded records":
				changed.Records = make([]store.RawRecord, 65537)
			case "raw height without block":
				qualified = nil
				changed.Blocks = nil
			}
			if e = ValidateBindings(ctx, changed, qualified); !errors.Is(e, ErrBinding) {
				t.Fatalf("incomplete batch admitted: %v", e)
			}
			if mode == "chain" || mode == "other transaction" || mode == "missing selected" || mode == "duplicate block" || mode == "incomplete block" || mode == "missing block" {
				if e = q.ValidateBinding(ctx, changed); !errors.Is(e, ErrBinding) {
					t.Fatalf("incomplete receipt binding: %v", e)
				}
			}
		})
	}
	if _, err = q.SourceProvenance(ctx, common.HexToAddress("0xbad")); !errors.Is(err, ErrQualification) {
		t.Fatalf("unrelated provenance: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = q.QualifiedEvents(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled envelopes: %v", err)
	}
	if _, err = q.SourceLogs(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled source logs: %v", err)
	}
	if _, err = q.SourceProvenance(canceled, common.HexToAddress(batch.Records[0].Emitter)); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled provenance: %v", err)
	}
	if _, err = q.SignedChainID(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled domain: %v", err)
	}
	if _, err = Qualify(canceled, a.Registry(), a, b); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled qualification: %v", err)
	}
	changed, err := clone(b)
	if err != nil {
		t.Fatal(err)
	}
	changed.Receipt.Logs[0].Removed = true
	if _, err = Qualify(ctx, a.Registry(), a, changed); !errors.Is(err, ErrQualification) {
		t.Fatalf("removed source input: %v", err)
	}
	changed, err = clone(b)
	if err != nil {
		t.Fatal(err)
	}
	changed.Header.Extra = make([]byte, 1<<20)
	if _, err = Qualify(ctx, a.Registry(), a, changed); !errors.Is(err, ErrQualification) {
		t.Fatalf("unbounded source input: %v", err)
	}
	changed, err = clone(b)
	if err != nil {
		t.Fatal(err)
	}
	changed.Receipt.GasUsed++
	if _, err = Qualify(ctx, a.Registry(), a, changed); !errors.Is(err, ErrQualification) || !strings.Contains(err.Error(), "authority") {
		t.Fatalf("altered source input: %v", err)
	}
}
