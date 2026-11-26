package projectionstats

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/holdings"
)

func TestNumericOrderTopHolders(t *testing.T) {
	f := newPublicFixture(t)
	if err := f.holdings.Discover(f.ctx, f.real.Token, 6, "10", true); err != nil {
		t.Fatal(err)
	}
	creator := fmt.Sprintf("0x%040x", 1)
	for i := 0; i < 12; i++ {
		amount := fmt.Sprint(i)
		if i == 0 {
			amount = "1000"
		}
		result, err := f.holdings.Apply(f.ctx, holdings.Transfer{ChainID: "125", Token: f.real.Token, From: holdings.Zero, To: fmt.Sprintf("0x%040x", i+1), Amount: amount, Height: "10", TransactionIndex: "0", LogIndex: fmt.Sprint(i), TransactionHash: fmt.Sprintf("0x%064x", 100), BlockHash: fmt.Sprintf("0x%064x", 200)})
		if err != nil || result.Incident || result.Duplicate {
			t.Fatalf("holder %d: %+v %v", i, result, err)
		}
	}
	tx, err := f.pg.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	anchor := publicAnchor{Position: Position{ChainID: "125", Height: "10", TransactionIndex: "0", LogIndex: "11"}}
	state, err := f.projector().holdingsAtAnchorTx(f.ctx, tx, anchor, f.real.Token, f.real.Pool, creator)
	if err != nil || state == nil {
		t.Fatalf("top holders: %+v %v", state, err)
	}
	if state.count != 12 || state.supply.String() != "1066" || state.top10.String() != "1063" || state.creator.String() != "1000" || state.poolBalance.Sign() != 0 {
		t.Fatalf("top holders: count=%d supply=%s top10=%s creator=%s pool=%s", state.count, state.supply, state.top10, state.creator, state.poolBalance)
	}
}

func TestNumericOrderNewestLog(t *testing.T) {
	f := newPublicFixture(t)
	transaction := realTransaction{Hash: fmt.Sprintf("0x%064x", 100), BlockHash: fmt.Sprintf("0x%064x", 200), BlockNumber: 10, TransactionIndex: 0}
	for _, index := range []int64{9, 10} {
		data := encodeWords(big.NewInt(1), big.NewInt(index), big.NewInt(100), big.NewInt(index*100), big.NewInt(100), big.NewInt(10))
		f.rawLog(transaction, realLog{Address: f.real.EventEmitter, LogIndex: index, Topics: []string{poolStateUpdatedTopic, poolTopic(f.real.Pool)}, Data: data})
	}
	anchor := publicAnchor{Position: Position{ChainID: "125", Height: "10", TransactionIndex: "0", LogIndex: "10"}, timestamp: 100}
	projector := f.projector()
	for i := 0; i < 2; i++ {
		tx, err := f.pg.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		events, err := projector.syncEventsTx(f.ctx, tx, anchor, []string{f.real.EventEmitter}, f.real.Pool, 101)
		if err != nil {
			_ = tx.Rollback(f.ctx)
			t.Fatal(err)
		}
		if len(events) != 1 || events[0].logIndex.Cmp(big.NewInt(10)) != 0 || events[0].price.String() != "1000" || events[0].realReserve.String() != "10" {
			_ = tx.Rollback(f.ctx)
			t.Fatalf("newest reserve log: %+v", events)
		}
		if err = tx.Commit(f.ctx); err != nil {
			t.Fatal(err)
		}
	}
}
