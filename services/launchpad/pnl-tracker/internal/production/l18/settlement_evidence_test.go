package l18

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

func assertSettlementEvidenceBindings(t *testing.T, service *SettlementService, actor Actor, program Program, day, beneficiary, origin string, payment func(*testing.T) string) {
	t.Helper()
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		method string
		mutate func(map[string]any)
		want   error
		state  string
	}{
		{"transaction", "eth_getTransactionReceipt", func(out map[string]any) {
			out["result"].(map[string]any)["transactionHash"] = "0x" + strings.Repeat("a", 64)
		}, ErrUnavailable, statusPendingInvestigation},
		{"block-hash", "eth_getTransactionReceipt", func(out map[string]any) { out["result"].(map[string]any)["blockHash"] = "invalid" }, ErrUnavailable, statusPendingInvestigation},
		{"frontier", "eth_getTransactionReceipt", func(out map[string]any) { out["result"].(map[string]any)["blockNumber"] = "0xfffffff" }, ErrUnavailable, statusPendingInvestigation},
		{"status", "eth_getTransactionReceipt", func(out map[string]any) { out["result"].(map[string]any)["status"] = "0x2" }, ErrConflict, ""},
		{"canonical-block", "eth_getBlockByNumber", func(out map[string]any) { out["result"].(map[string]any)["hash"] = "0x" + strings.Repeat("a", 64) }, ErrConflict, ""},
		{"missing-transfer", "eth_getTransactionReceipt", func(out map[string]any) { out["result"].(map[string]any)["logs"] = []any{} }, ErrUnavailable, ""},
		{"amount", "eth_getTransactionReceipt", func(out map[string]any) {
			for _, v := range out["result"].(map[string]any)["logs"].([]any) {
				v.(map[string]any)["data"] = "0x" + strings.Repeat("0", 63) + "2"
			}
		}, ErrConflict, ""},
		{"deployment", "eth_getCode", func(out map[string]any) { out["result"] = "0x6000" }, ErrUnavailable, statusPendingInvestigation},
		{"header-read", "eth_getBlockByNumber", func(out map[string]any) {
			delete(out, "result")
			out["error"] = map[string]any{"code": -32000, "message": "block unavailable"}
		}, nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := "evidence-" + tc.name
			transaction := payment(t)
			if _, err := service.ledger.pool.Exec(ctx, `INSERT INTO pnl.l18_awards(identity,program,version,day,beneficiary,asset,kind,eligible,payable,status,provenance) VALUES($1,$2,$3,$4,$5,$6,'cashback',1,1,'funded',$1)`, id, program.ID, program.Version, day, beneficiary, USDL); err != nil {
				t.Fatal(err)
			}
			if err := service.RegisterIntent(ctx, actor, uuid.NewString(), id, transaction, "beneficiary payout"); err != nil {
				t.Fatal(err)
			}
			var receiptSeen atomic.Bool
			variant := rewardReceiptVariant(t, origin, func(method string, _ []json.RawMessage, out map[string]any) {
				if method == "eth_getTransactionReceipt" {
					receiptSeen.Store(true)
				}
				if method == tc.method && (method != "eth_getBlockByNumber" || receiptSeen.Load()) {
					tc.mutate(out)
				}
			})
			creator := *service.authority.creator
			creator.rpc = variant.rpc
			authority := *service.authority
			authority.creator = &creator
			candidate := &SettlementService{ledger: service.ledger, authority: &authority}
			state, err := candidate.Reconcile(ctx, id)
			if err == nil || tc.want != nil && !errors.Is(err, tc.want) || state != tc.state {
				t.Fatal(state, err, tc.want)
			}
			expectedStatus := "funded"
			expectedAttempt := "prepared"
			if tc.state == statusPendingInvestigation {
				expectedStatus = tc.state
				expectedAttempt = tc.state
			}
			var status, attempt string
			var entries int
			if err = service.ledger.pool.QueryRow(ctx, `SELECT a.status,p.state,(SELECT count(*) FROM pnl.l18_entries e WHERE e.identity=a.identity) FROM pnl.l18_awards a JOIN pnl.l18_payout_attempts p ON p.award_id=a.identity WHERE a.identity=$1`, id).Scan(&status, &attempt, &entries); err != nil || status != expectedStatus || attempt != expectedAttempt || entries != 0 {
				t.Fatal(status, attempt, entries, err)
			}
		})
	}

	for _, mode := range []string{"program", "checkpoint", "deployment"} {
		for _, operation := range []string{"intent", "receipt"} {
			t.Run(mode+"-"+operation, func(t *testing.T) {
				id := "availability-" + mode + "-" + operation
				transaction := payment(t)
				if _, err := service.ledger.pool.Exec(ctx, `INSERT INTO pnl.l18_awards(identity,program,version,day,beneficiary,asset,kind,eligible,payable,status,provenance) VALUES($1,$2,$3,$4,$5,$6,'cashback',1,1,'funded',$1)`, id, program.ID, program.Version, day, beneficiary, USDL); err != nil {
					t.Fatal(err)
				}
				if operation == "receipt" {
					if err := service.RegisterIntent(ctx, actor, uuid.NewString(), id, transaction, "beneficiary payout"); err != nil {
						t.Fatal(err)
					}
				}
				authority := *service.authority
				creator := *authority.creator
				authority.creator = &creator
				switch mode {
				case "program":
					authority.programs = map[string]Program{}
				case "checkpoint":
					creator.committed = func(context.Context) (Checkpoint, error) { return Checkpoint{}, ErrUnavailable }
				case "deployment":
					invalid := program
					invalid.AdapterCodeHash = "0x" + strings.Repeat("0", 64)
					authority.programs = map[string]Program{fingerprint([]string{program.ID, program.Version}): invalid}
				}
				candidate := &SettlementService{ledger: service.ledger, authority: &authority}
				var err error
				if operation == "intent" {
					err = candidate.RegisterIntent(ctx, actor, uuid.NewString(), id, transaction, "beneficiary payout")
				} else {
					_, err = candidate.Reconcile(ctx, id)
				}
				if !errors.Is(err, ErrUnavailable) {
					t.Fatal(err)
				}
				var entries int
				if err = service.ledger.pool.QueryRow(ctx, `SELECT count(*) FROM pnl.l18_entries WHERE identity=$1`, id).Scan(&entries); err != nil || entries != 0 {
					t.Fatal(entries, err)
				}
			})
		}
	}
	different := *service.authority
	if _, err := NewSettlementService(service.ledger, &different); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
