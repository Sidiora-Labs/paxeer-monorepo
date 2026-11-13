package l18

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func rewardCancellationMatrix(t *testing.T, prepare func(*testing.T, *Ledger) func() error) {
	t.Helper()
	ctx := context.Background()
	fixture := func(t *testing.T) (*Ledger, *internaltest.QueryCancellation) {
		original := storage(t)
		config := original.pool.Config()
		trace := new(internaltest.QueryCancellation)
		config.ConnConfig.Tracer = trace
		pool, err := pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		l := *original
		l.pool = pool
		return &l, trace
	}
	counts := func(t *testing.T, l *Ledger) []int {
		tables := []string{"l18_inbox", "l18_outbox", "l18_funding", "l18_awards", "l18_entries", "l18_bindings", "l18_referral_requests", "l18_referral_codes", "l18_referral_consents", "l18_analytics", "l18_audit", "l18_corrections", "l18_epochs", "l18_epoch_claims", "l18_claims"}
		out := make([]int, len(tables))
		for i, table := range tables {
			if err := l.pool.QueryRow(ctx, "SELECT count(*) FROM "+l.schema+"."+table).Scan(&out[i]); err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	probe, trace := fixture(t)
	operation := prepare(t, probe)
	trace.Start(0)
	err := operation()
	queries := trace.Stop()
	if err != nil || len(queries) == 0 {
		t.Fatal(queries, err)
	}
	for i, query := range queries {
		if strings.EqualFold(query, "rollback") {
			continue
		}
		t.Run(fmt.Sprintf("query_%03d", i+1), func(t *testing.T) {
			ledger, cancellation := fixture(t)
			operation := prepare(t, ledger)
			before := counts(t, ledger)
			cancellation.Start(i + 1)
			err := operation()
			observed := cancellation.Stop()
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("query %q: error=%v", query, err)
			}
			if len(observed) != i+1 || observed[i] != strings.ReplaceAll(query, probe.schema, ledger.schema) {
				t.Fatal(observed, query)
			}
			if after := counts(t, ledger); !reflect.DeepEqual(before, after) {
				t.Fatal(before, after)
			}
		})
	}
}

func TestRewardDatabaseCancellationIsAtomic(t *testing.T) {
	ctx := context.Background()
	actions := []string{"fund", "fund replay", "accrue", "accrue replay", "referral award", "hold", "review", "correction propose", "correction approve", "liability", "liabilities", "code", "existing code", "code replay", "consent", "click", "bind", "bind replay", "bind issued", "existing binding", "binding", "dashboard", "analytics"}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			rewardCancellationMatrix(t, func(t *testing.T, l *Ledger) func() error {
				now := time.Unix(1791590400, 0).UTC()
				attribution, err := NewAttribution(l, []byte(strings.Repeat("q", 32)), func() time.Time { return now }, "campaign", "v1")
				if err != nil {
					t.Fatal(err)
				}
				request := uuid.NewString()
				reviewer := Actor{Wallet: actorA, Roles: map[string]bool{"reward-review": true, "reward-correction-propose": true}}
				approver := Actor{Wallet: actorC, Roles: map[string]bool{"reward-correction-approve": true}}
				candidate := award("cancellation-award", actorB)
				if action != "fund" {
					if err = l.Fund(ctx, funding()); err != nil {
						t.Fatal(err)
					}
				}
				if action != "fund" && action != "fund replay" && action != "accrue" && action != "referral award" {
					result, err := l.Accrue(ctx, candidate)
					if err != nil || result.PayableRaw != "1000000" {
						t.Fatal(result, err)
					}
				}
				if action == "correction approve" {
					if err = l.Correction(ctx, reviewer, request, candidate.Identity, "accounting review", false); err != nil {
						t.Fatal(err)
					}
				}
				token, err := attribution.IssueClick(actorA, "cancellation-click")
				if err != nil {
					t.Fatal(err)
				}
				if action == "bind replay" || action == "binding" || action == "dashboard" || action == "existing binding" {
					if _, err = attribution.Bind(ctx, request, actorB, true, []string{token}); err != nil {
						t.Fatal(err)
					}
				}
				var code ReferralCode
				if action == "existing code" || action == "code replay" || action == "click" || action == "bind issued" {
					codeRequest := uuid.NewString()
					if action == "code replay" {
						codeRequest = request
					}
					raw, err := attribution.IssueCode(ctx, actorA, codeRequest)
					if err != nil {
						t.Fatal(err)
					}
					if err = json.Unmarshal(raw, &code); err != nil {
						t.Fatal(err)
					}
				}
				if action == "bind issued" {
					raw, err := attribution.ClickCode(ctx, actorB, uuid.NewString(), code.Code, true)
					if err != nil {
						t.Fatal(err)
					}
					var click ReferralClick
					if err = json.Unmarshal(raw, &click); err != nil {
						t.Fatal(err)
					}
					token = click.Token
				}
				if action == "analytics" {
					if _, err = l.pool.Exec(ctx, `INSERT INTO pnl.referrals(short_code,sharer_address,card_id,created_at) VALUES('cancel-code',$1,'portfolio-card',$2)`, actorA, now.Unix()); err != nil {
						t.Fatal(err)
					}
				}
				return func() error {
					switch action {
					case "fund", "fund replay":
						return l.Fund(ctx, funding())
					case "accrue", "accrue replay", "referral award":
						if action == "referral award" {
							candidate.Kind = "referral"
							candidate.Referred = actorC
							candidate.Identity, err = ReferralAwardIdentity("campaign", "v1", actorC)
							if err != nil {
								t.Fatal(err)
							}
						}
						result, err := l.Accrue(ctx, candidate)
						if err == nil && result.PayableRaw != "1000000" {
							t.Fatal(result)
						}
						return err
					case "hold":
						return l.Hold(ctx, request, candidate.Identity, "beneficiary review")
					case "review":
						return l.ReviewHold(ctx, reviewer, request, candidate.Identity, "beneficiary review")
					case "correction propose":
						return l.Correction(ctx, reviewer, request, candidate.Identity, "accounting review", false)
					case "correction approve":
						return l.Correction(ctx, approver, request, candidate.Identity, "approve accounting", true)
					case "liability":
						result, err := l.Liability(ctx, candidate.Identity)
						if err == nil && result.PayableRaw != "1000000" {
							t.Fatal(result)
						}
						return err
					case "liabilities":
						result, err := l.Liabilities(ctx, Actor{Wallet: actorB}, "", 10)
						if err == nil && len(result) != 1 {
							t.Fatal(result)
						}
						return err
					case "code", "existing code", "code replay":
						raw, err := attribution.IssueCode(ctx, actorA, request)
						if err == nil {
							var result ReferralCode
							if err = json.Unmarshal(raw, &result); err != nil || result.Code == "" {
								t.Fatal(result, err)
							}
						}
						return err
					case "consent":
						raw, err := attribution.SetConsent(ctx, actorB, request, true)
						if err == nil {
							var result ReferralConsent
							if err = json.Unmarshal(raw, &result); err != nil || !result.Consent {
								t.Fatal(result, err)
							}
						}
						return err
					case "click":
						raw, err := attribution.ClickCode(ctx, actorB, request, code.Code, true)
						if err == nil {
							var result ReferralClick
							if err = json.Unmarshal(raw, &result); err != nil || result.Token == "" {
								t.Fatal(result, err)
							}
						}
						return err
					case "bind", "bind replay":
						result, err := attribution.Bind(ctx, request, actorB, true, []string{token})
						if err == nil && result.Referrer != actorA {
							t.Fatal(result)
						}
						return err
					case "bind issued", "existing binding":
						result, err := attribution.BindIssued(ctx, actorB, uuid.NewString(), true, []string{token})
						if err == nil && result.Referrer != actorA {
							t.Fatal(result)
						}
						return err
					case "binding":
						result, err := attribution.Binding(ctx, actorB)
						if err == nil && result.Referrer != actorA {
							t.Fatal(result)
						}
						return err
					case "dashboard":
						result, err := attribution.Dashboard(ctx, actorB)
						if err == nil && (result.Binding == nil || !result.Consent.Consent) {
							t.Fatal(result)
						}
						return err
					default:
						return l.RecordAnalytics(ctx, Actor{Wallet: actorB}, request, "campaign", "cancel-code", "view", now)
					}
				}
			})
		})
	}
}
