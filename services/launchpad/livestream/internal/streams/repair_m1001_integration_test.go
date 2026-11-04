package streams

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Supplementary PostgreSQL qualification; the bounded TestRepairM_1001_ gate
// exercises the codec and resolver without requiring a database process.
func TestPostgresRepairM1001DecimalsAndAlerts(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	if _, err := s.pool.Exec(ctx, s.ConnectedSchemaSQL()+s.BudgetSchemaSQL()); err != nil {
		t.Fatal(err)
	}
	request, decision := intent("decimal_usage")
	o, err := s.Create(ctx, request, decision, true)
	if err != nil {
		t.Fatal(err)
	}
	policy := repairM1001Budget()
	s.Budget = &policy
	s.Budget.VendorUserID = ""
	for i, cost := range []string{"5.125", "3.075", "2.05"} {
		record := repairM1001Usage("0.25")
		record.ProviderEvent = fmt.Sprintf("usage_%d", i)
		record.OperationID = o.ID
		record.CostUnits = cost
		if err = s.RecordUsage(ctx, record); err != nil {
			t.Fatal("actual numeric write", err)
		}
		record.CostUnits = cost + "0"
		if err = s.RecordUsage(ctx, record); err != nil {
			t.Fatal("exact decimal redelivery", err)
		}
		if err = s.EmitBudgetAlerts(ctx); err != nil {
			t.Fatal("actual alert generation", err)
		}
		var count int
		if err = s.pool.QueryRow(ctx, "SELECT count(*) FROM "+s.schema+".stream_budget_alerts").Scan(&count); err != nil || count != i+1 {
			t.Fatalf("alerts=%d want=%d %v", count, i+1, err)
		}
	}
	rows, err := s.pool.Query(ctx, "SELECT threshold,spent_units::text,budget_units::text FROM "+s.schema+".stream_budget_alerts ORDER BY threshold")
	if err != nil {
		t.Fatal(err)
	}
	expected := map[int]string{50: "5.125", 80: "8.2", 100: "10.25"}
	for rows.Next() {
		var threshold int
		var spent, budget string
		if err = rows.Scan(&threshold, &spent, &budget); err != nil {
			t.Fatal(err)
		}
		actual, ok := new(big.Rat).SetString(spent)
		want, _ := new(big.Rat).SetString(expected[threshold])
		limit, valid := new(big.Rat).SetString(budget)
		expectedLimit, _ := new(big.Rat).SetString("10.25")
		if !ok || !valid || want == nil || actual.Cmp(want) != 0 || limit.Cmp(expectedLimit) != 0 {
			t.Fatalf("stored alert lost exact units: %d %s %s", threshold, spent, budget)
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"1/2", "1e3", "1e-999999999", "0." + strings.Repeat("1", 19)} {
		record := repairM1001Usage("0.25")
		record.OperationID = o.ID
		record.CostUnits = invalid
		if err = s.RecordUsage(ctx, record); !errors.Is(err, ErrInvalid) {
			t.Fatalf("database ingestion admitted %q: %v", invalid, err)
		}
		s.Budget.MonthlyUnits = invalid
		if err = s.EmitBudgetAlerts(ctx); !errors.Is(err, ErrInvalid) {
			t.Fatalf("alert policy admitted %q: %v", invalid, err)
		}
	}
	var count int
	if err = s.pool.QueryRow(ctx, "SELECT count(*) FROM "+s.schema+".stream_usage").Scan(&count); err != nil || count != 3 {
		t.Fatalf("invalid or duplicate usage persisted: %d %v", count, err)
	}
}

func TestPostgresRepairM1001AmbiguityRestartAndAdmission(t *testing.T) {
	base := fixture(t)
	ctx := context.Background()
	if _, err := base.pool.Exec(ctx, base.ConnectedSchemaSQL()); err != nil {
		t.Fatal(err)
	}
	config := base.pool.Config().Copy()
	open := func() *LifecycleStore {
		p, err := pgxpool.NewWithConfig(ctx, config.Copy())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(p.Close)
		s, err := NewLifecycleStore(p, base.schema, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	s := open()
	request, decision := intent("crash_before_send")
	o, err := s.Create(ctx, request, decision, true)
	if err != nil {
		t.Fatal(err)
	}
	o, err = s.BeginProvision(ctx, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	if claimed, err := s.claimWorker(ctx, o.ID, owner); err != nil || !claimed {
		t.Fatal("claim", claimed, err)
	}
	if err = s.reserveProviderCreate(ctx, o.ID, owner, o.Revision); err != nil {
		t.Fatal(err)
	}
	// Stop the component after reserving the send, with no HTTP create performed.
	s.pool.Close()
	s = open()
	if _, err = s.pool.Exec(ctx, "UPDATE "+s.schema+".stream_worker_attempts SET lease_until='-infinity' WHERE operation_id=$1", o.ID); err != nil {
		t.Fatal(err)
	}
	state, err := s.ProvisionRecovery(ctx, o.ID)
	if err != nil || !state.CreateSent || len(state.Actions()) != 3 {
		t.Fatal("recovered ambiguity", state, err)
	}
	var creates int
	provider, _ := repairM1001Provider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			creates++
			http.Error(w, "provider rejected", 429)
			return
		}
		_, _ = io.WriteString(w, `[]`)
	})
	worker := ProvisionWorker{Store: s, Provider: provider}
	o, err = worker.ResolveAmbiguity(ctx, repairM1001Decision(state, ProvisionResume))
	if err != nil || o.Revision != 3 {
		t.Fatal(o, err)
	}
	s.pool.Close()
	s = open()
	worker.Store = s
	state, err = s.ProvisionRecovery(ctx, o.ID)
	if err != nil || state.CreateSent || state.LastReason != "provider_nonallocation_resume" {
		t.Fatal("resume lost on reopen", state, err)
	}
	owner, err = randomID()
	if err != nil {
		t.Fatal(err)
	}
	if claimed, err := s.claimWorker(ctx, o.ID, owner); err != nil || !claimed {
		t.Fatal("resumed claim", claimed, err)
	}
	if err = s.reserveProviderCreate(ctx, o.ID, owner, o.Revision); err != nil {
		t.Fatal("confirmed resume did not allow one create", err)
	}
	if err = s.reserveProviderCreate(ctx, o.ID, owner, o.Revision); !errors.Is(err, ErrUnavailable) {
		t.Fatal("blind repeat allowed", err)
	}
	if _, err = provider.CreateCorrelated(ctx, o.ID); err == nil {
		t.Fatal("rejection accepted")
	}
	if err = s.releaseWorker(ctx, o.ID, owner, "provider_create_ambiguous"); err != nil {
		t.Fatal(err)
	}
	state, err = s.ProvisionRecovery(ctx, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	o, err = worker.ResolveAmbiguity(ctx, repairM1001Decision(state, ProvisionRelease))
	if err != nil || o.State != "FAILED" || creates != 1 {
		t.Fatalf("release: %+v creates=%d %v", o, creates, err)
	}
	request.IdempotencyKey = "after_release"
	if _, err = s.Create(ctx, request, decision, true); err != nil {
		t.Fatal("confirmed release still held admission", err)
	}
	var receipts int
	if err = s.pool.QueryRow(ctx, "SELECT count(*) FROM "+s.schema+".stream_audit WHERE operation_id=$1 AND reason LIKE 'provider_ambiguity_resolution:%'", o.ID).Scan(&receipts); err != nil || receipts != 2 {
		t.Fatalf("durable resolution audit missing: %d %v", receipts, err)
	}
}
