package l18

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func assertSettlementDatabaseCancellation(t *testing.T, authority *CanonicalAuthority, program Program, actor Actor, beneficiary, day, paid string) {
	t.Helper()
	ctx := context.Background()
	for _, action := range []string{"intent", "receipt", "investigation", "reverted"} {
		t.Run(action, func(t *testing.T) {
			rewardCancellationMatrix(t, func(t *testing.T, ledger *Ledger) func() error {
				if _, err := ledger.pool.Exec(ctx, `INSERT INTO `+ledger.schema+`.l18_awards(identity,program,version,day,beneficiary,asset,kind,eligible,payable,status,provenance) VALUES('cancellation-payout',$1,$2,$3,$4,$5,'cashback',1000000,1000000,'funded','canonical-payment')`, program.ID, program.Version, day, beneficiary, USDL); err != nil {
					t.Fatal(err)
				}
				service := &SettlementService{ledger: ledger, authority: authority}
				request := uuid.NewString()
				if action != "intent" {
					if err := service.RegisterIntent(ctx, actor, request, "cancellation-payout", paid, "beneficiary payout"); err != nil {
						t.Fatal(err)
					}
				}
				return func() error {
					if action == "intent" {
						return service.RegisterIntent(ctx, actor, request, "cancellation-payout", paid, "beneficiary payout")
					}
					if action == "receipt" {
						state, err := service.Reconcile(ctx, "cancellation-payout")
						if err == nil && state != statusIncludedSuccess {
							t.Fatal(state)
						}
						return err
					}
					state := statusPendingInvestigation
					if action == "reverted" {
						state = statusIncludedReverted
					}
					return service.markAttempt(ctx, "cancellation-payout", 1, paid, state)
				}
			})
		})
	}
}
