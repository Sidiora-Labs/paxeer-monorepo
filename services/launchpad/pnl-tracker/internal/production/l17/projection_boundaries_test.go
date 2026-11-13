package l17

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func assertProjectionBindings(t *testing.T, r Repository, store *delivery.Store, subject string, event delivery.Envelope, update interfaces.PortfolioUpdate, summary Summary, positions []Position) {
	t.Helper()
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		change func(*interfaces.PortfolioUpdate)
	}{
		{"partial batch", func(u *interfaces.PortfolioUpdate) { u.CompleteBatch = false }},
		{"snapshot binding", func(u *interfaces.PortfolioUpdate) { u.SnapshotId = uuid.NewString() }},
		{"revision binding", func(u *interfaces.PortfolioUpdate) { u.SummaryRevision = "2" }},
		{"position count", func(u *interfaces.PortfolioUpdate) { u.Summary.PositionCount = 0 }},
		{"derived totals", func(u *interfaces.PortfolioUpdate) { u.Summary.KnownRealizedSubtotalUsdlRaw = "1" }},
		{"delete operation", func(u *interfaces.PortfolioUpdate) { u.Changes[0].Operation = "delete"; u.Changes[0].Position = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(update)
			if err != nil {
				t.Fatal(err)
			}
			var bad interfaces.PortfolioUpdate
			if err = json.Unmarshal(raw, &bad); err != nil {
				t.Fatal(err)
			}
			tc.change(&bad)
			tx, err := r.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer rollbackPortfolio(tx)
			if _, err = r.CommitPortfolioUpdateTx(ctx, tx, event.EventID, bad, time.Now().Add(time.Hour)); err == nil {
				t.Fatal("unbound projection accepted")
			}
		})
	}
	for _, raw := range []string{`{`, `{"a":1,"a":2}`, `{}`, `{"unexpected":true}`} {
		tx, err := r.Pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		altered := event
		altered.Payload = []byte(raw)
		if _, err = r.CommitProjectionDeliveryTx(ctx, tx, store, subject, altered, update, time.Now().Add(time.Hour)); err == nil {
			t.Fatal("unbound projection envelope accepted")
		}
		if _, err = r.CommitFullProjectionDeliveryTx(ctx, tx, store, subject, event.EventID, summary, positions, time.Now().Add(time.Hour), altered); err == nil {
			t.Fatal("unbound notice accepted")
		}
		if err = tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.CommitProjectionDeliveryTx(ctx, nil, store, subject, event, update, time.Now().Add(time.Hour)); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := r.CommitFullProjectionDeliveryTx(ctx, nil, store, subject, event.EventID, summary, positions, time.Now().Add(time.Hour), event); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	cfg := r.Pool.Config()
	trace := new(internaltest.QueryCancellation)
	cfg.ConnConfig.Tracer = trace
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, full := range []bool{false, true} {
		t.Run(fmt.Sprintf("full_%v", full), func(t *testing.T) {
			invoke := func(target int) ([]string, error) {
				tx, err := pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer rollbackPortfolio(tx)
				trace.Start(target)
				var committed Summary
				if full {
					notice := Notice{Wallet: summary.Wallet, SnapshotID: summary.SnapshotID, Revision: summary.Watermark.ProjectionRevision, Algorithm: Algorithm}
					raw, err := json.Marshal(notice)
					if err != nil {
						t.Fatal(err)
					}
					bound := event
					bound.Payload = raw
					bound.PayloadHash = delivery.Hash(raw)
					committed, err = r.CommitFullProjectionDeliveryTx(ctx, tx, store, subject, event.EventID, summary, positions, time.Now().Add(time.Hour), bound)
					queries := trace.Stop()
					if err == nil && committed.SnapshotID != summary.SnapshotID {
						t.Fatal(committed)
					}
					return queries, err
				}
				committed, err = r.CommitProjectionDeliveryTx(ctx, tx, store, subject, event, update, time.Now().Add(time.Hour))
				queries := trace.Stop()
				if err == nil && committed.SnapshotID != summary.SnapshotID {
					t.Fatal(committed)
				}
				return queries, err
			}
			queries, err := invoke(0)
			if err != nil || len(queries) == 0 {
				t.Fatal(queries, err)
			}
			for i, query := range queries {
				t.Run(fmt.Sprintf("query_%03d", i+1), func(t *testing.T) {
					observed, err := invoke(i + 1)
					if !errors.Is(err, context.Canceled) || len(observed) != i+1 || observed[i] != query {
						t.Fatal(observed, query, err)
					}
				})
			}
		})
	}
}
