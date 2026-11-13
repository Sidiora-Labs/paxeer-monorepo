package projectionstats

import (
	"errors"
	"math/big"
	"strings"
	"testing"
)

func TestHistoricalHoldingsRestoresDepartedWallet(t *testing.T) {
	f := newPublicFixture(t)
	fills := f.ingestReal()
	anchor := publicAnchor{Position: Position{ChainID: "125", Height: fills["sell"].Position.Height, TransactionIndex: fills["sell"].Position.TransactionIndex, LogIndex: fills["sell"].Position.LogIndex, TransactionHash: fills["sell"].Position.TransactionHash, BlockHash: fills["sell"].Position.BlockHash}}
	projector := f.projector()
	read := func() *anchorHoldings {
		t.Helper()
		tx, err := f.pg.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(f.ctx)
		state, err := projector.holdingsAtAnchorTx(f.ctx, tx, anchor, f.real.Token, f.real.Pool, f.real.Transactions[0].From)
		if err != nil || state == nil {
			t.Fatalf("historical ownership: %+v %v", state, err)
		}
		return state
	}
	before := read()
	owner := f.real.Transactions[0].From
	var balance string
	if err := f.pg.QueryRow(f.ctx, "SELECT balance::text FROM "+publicTestHolders+".balances WHERE token=$1 AND address=$2", f.real.Token, owner).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	amount, ok := new(big.Int).SetString(balance, 10)
	if !ok || amount.Sign() <= 0 {
		t.Fatal(balance)
	}
	later := realTransaction{Hash: "0x" + strings.Repeat("a", 64), From: owner, BlockNumber: 33963800, BlockHash: "0x" + strings.Repeat("b", 64)}
	recipient := "0x" + strings.Repeat("c", 40)
	log := realLog{Address: f.real.Token, LogIndex: 1, Topics: []string{transferTopic, "0x" + strings.Repeat("0", 24) + owner[2:], "0x" + strings.Repeat("0", 24) + recipient[2:]}, Data: encodeWords(amount)}
	f.rawLog(later, log)
	f.transfer(later, log)
	after := read()
	if after.count != before.count || after.supply.Cmp(before.supply) != 0 || after.creator.Cmp(before.creator) != 0 || after.poolBalance.Cmp(before.poolBalance) != 0 || after.top10.Cmp(before.top10) != 0 {
		t.Fatalf("post-anchor ownership changed history: before=%+v after=%+v", before, after)
	}
}

func TestHistoricalHoldingsRejectsChangedLedgerMetadata(t *testing.T) {
	f := newPublicFixture(t)
	fills := f.ingestReal()
	fill := fills["sell"]
	anchor := publicAnchor{Position: Position{ChainID: "125", Height: fill.Position.Height, TransactionIndex: fill.Position.TransactionIndex, LogIndex: fill.Position.LogIndex, TransactionHash: fill.Position.TransactionHash, BlockHash: fill.Position.BlockHash}}
	projector := f.projector()
	for _, tc := range []struct {
		name, sql string
		absent    bool
	}{
		{"unknown token", "", true},
		{"incomplete coverage", "UPDATE " + publicTestHolders + ".tokens SET coverage='incomplete' WHERE token=$1", true},
		{"malformed position", "UPDATE " + publicTestHolders + ".tokens SET last_event='[]' WHERE token=$1", false},
		{"invalid height", "UPDATE " + publicTestHolders + ".tokens SET last_event=jsonb_set(last_event,'{height}','\"invalid\"') WHERE token=$1", false},
		{"lagging projection", "UPDATE " + publicTestHolders + ".tokens SET last_event=NULL WHERE token=$1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := f.pg.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(f.ctx)
			if tc.sql != "" {
				if _, err := tx.Exec(f.ctx, tc.sql, f.real.Token); err != nil {
					t.Fatal(err)
				}
			}
			token := f.real.Token
			if tc.name == "unknown token" {
				token = "0x" + strings.Repeat("d", 40)
			}
			state, err := projector.holdingsAtAnchorTx(f.ctx, tx, anchor, token, f.real.Pool, f.real.Transactions[0].From)
			if tc.absent {
				if err != nil || state != nil {
					t.Fatalf("unproven ownership: %+v %v", state, err)
				}
			} else if !errors.Is(err, ErrIntegrity) {
				t.Fatalf("changed metadata accepted: %+v %v", state, err)
			}
		})
	}
}
