package l16

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestTransferCancellationPreservesBothInventoryPartitions(t *testing.T) {
	ctx := context.Background()
	_, pool := recordedRebuildService(t, ctx)
	buy := testLeg(1)
	unknown := testLeg(2)
	unknown.Kind = movementIncoming
	unknown.ActualUSDL = nil
	unknown.Predecessor, _ = buy.Identity.Key()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `SET LOCAL search_path TO pnl,pg_catalog`); err != nil {
		t.Fatal(err)
	}
	for _, leg := range []Leg{buy, unknown} {
		state, inserted, err := ApplyTx(ctx, tx, leg)
		if err != nil || !inserted || state.Known != "10" {
			t.Fatal(state, inserted, err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	outgoing := testLeg(3)
	outgoing.Kind = movementOutgoing
	outgoing.ActualUSDL = nil
	outgoing.Predecessor, _ = unknown.Identity.Key()
	outgoing.Recipient = "0x" + strings.Repeat("5", 40)
	base := TransferRequest{Source: outgoing, Destination: outgoing.Recipient, Carry: true, ProofReference: "same-owner-consent"}
	for _, carry := range []bool{true, false} {
		t.Run(map[bool]string{true: "carry", false: "external"}[carry], func(t *testing.T) {
			request := base
			request.Carry = carry
			assertInventoryTxCancellations(t, pool, func(tx pgx.Tx) error {
				source, destination, inserted, err := ApplyTransferTx(ctx, tx, request)
				if err == nil {
					wantKnown, wantUnknown, wantCost := "5", "5", "50"
					if !carry {
						wantKnown, wantUnknown, wantCost = "0", "10", "0"
					}
					if !inserted || source.Known != "5" || source.Unknown != "5" || source.Cost != "50" || destination.Known != wantKnown || destination.Unknown != wantUnknown || destination.Cost != wantCost {
						t.Fatal(source, destination, inserted)
					}
				}
				return err
			})
		})
	}
	for _, change := range []func(*TransferRequest){
		func(r *TransferRequest) { r.Source.Quantity = "bad" },
		func(r *TransferRequest) { r.Destination = r.Source.Wallet },
		func(r *TransferRequest) { r.ProofReference = "" },
	} {
		bad := base
		change(&bad)
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err = ApplyTransferTx(ctx, tx, bad); err == nil {
			t.Fatal("unbound transfer accepted")
		}
		if err = tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for _, state := range []string{`"not-an-inventory"`, `{"knownQuantityRaw":"bad"}`} {
		t.Run(state, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer inventoryRollback(tx)
			if _, err = tx.Exec(ctx, `SET LOCAL search_path TO pnl,pg_catalog`); err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec(ctx, `UPDATE inventory_l16 SET state=$1`, state); err != nil {
				t.Fatal(err)
			}
			if _, _, err = ApplyTx(ctx, tx, outgoing); err == nil {
				t.Fatal("invalid source inventory accepted")
			}
		})
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	if _, err = tx.Exec(ctx, `SET LOCAL search_path TO pnl,pg_catalog`); err != nil {
		t.Fatal(err)
	}
	if _, _, err = applyVersionedTx(ctx, tx, Leg{}, Apply, "v", "p"); err == nil {
		t.Fatal("invalid leg accepted")
	}
	if _, _, err = applyVersionedTx(ctx, tx, buy, nil, "v", "p"); err == nil {
		t.Fatal("missing fold accepted")
	}
	if _, _, err = ApplyTx(ctx, nil, buy); err == nil {
		t.Fatal("missing transaction accepted")
	}
	if _, _, err = ApplyTx(ctx, tx, outgoing); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = ApplyTransferTx(ctx, tx, base); err == nil || !strings.Contains(err.Error(), "outside atomic wrapper") {
		t.Fatal(err)
	}
}
