package l16

import (
	"context"
	"errors"
	"fmt"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func inventoryCancelableService(t *testing.T) (*Service, *internaltest.QueryCancellation) {
	t.Helper()
	source, pool := recordedRebuildService(t, context.Background())
	config := pool.Config()
	trace := new(internaltest.QueryCancellation)
	config.ConnConfig.Tracer = trace
	traced, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(traced.Close)
	authority := *source.authority
	authority.pool = traced
	service := *source
	service.authority = &authority
	return &service, trace
}

func TestInventoryDatabaseCancellationIsAtomic(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"baseline", "ordinary", "archive", "live", "publish", "worker", "enroll", "cancel", "interval", "measurement"} {
		t.Run(mode, func(t *testing.T) {
			captures := map[*Service]InventoryMeasurementCase{}
			principals := map[*Service]auth.IdentityPrincipal{}
			ranges := map[*Service][2]time.Time{}
			workers := map[*Service]*LiveTransferWorker{}
			invoke := func(t *testing.T, s *Service) error {
				t.Helper()
				switch mode {
				case "baseline":
					digest, err := s.authority.Capture(ctx, "0x"+strings.Repeat("e", 40), realMarketToken)
					if err == nil && len(digest) != 64 {
						t.Fatal(digest)
					}
					return err
				case "ordinary", "archive":
					snapshot, err := s.rebuild(ctx, realMarketCreator, realMarketToken, 33963733, mode == "archive")
					if err == nil && (snapshot.State.Known == "0" || snapshot.Height != 33963733) {
						t.Fatal(snapshot)
					}
					return err
				case "measurement":
					raw, err := captureInventoryMeasurement(ctx, workers[s], captures[s], s.release, 100000)
					if err == nil && len(raw) == 0 {
						t.Fatal("missing reconciliation capture")
					}
					return err
				case "interval":
					bounds := ranges[s]
					series, err := s.IntervalHistory(ctx, realMarketCreator, bounds[0], bounds[1], "1h", 24)
					if err == nil && (series.SnapshotID == "" || len(series.Entries) == 0) {
						t.Fatal(series)
					}
					return err
				case "enroll":
					return workers[s].EnqueueOwned(ctx, principals[s])
				case "cancel":
					return workers[s].CancelOwned(ctx, principals[s])
				case "worker":
					changed, err := workers[s].processWallet(ctx, realMarketCreator)
					if err == nil && !changed {
						t.Fatal("enrolled wallet was not processed")
					}
					return err
				case "live":
					tx, err := s.authority.pool.Begin(ctx)
					if err != nil {
						return err
					}
					defer inventoryRollback(tx)
					snapshot, err := s.liveRebuildTx(ctx, tx, realMarketCreator, realMarketToken, 33963733)
					if err == nil && (snapshot.State.Known == "0" || snapshot.Height != 33963733) {
						t.Fatal(snapshot)
					}
					return err
				default:
					summary, err := s.PublishRebuiltWallet(ctx, realMarketCreator, 33963733, l17.Repository{Pool: s.authority.pool})
					if err == nil && (summary.SnapshotID == "" || summary.Wallet != realMarketCreator) {
						t.Fatal(summary)
					}
					return err
				}
			}
			prepare := func(t *testing.T, s *Service) {
				if mode == "worker" || mode == "enroll" || mode == "cancel" || mode == "measurement" {
					inventoryCommittedHead(t, ctx, s.authority.pool)
					identity, principal := inventoryIdentityFixture(t, realMarketCreator)
					worker, err := NewLiveTransferWorker(s, l17.Repository{Pool: s.authority.pool}, identity, 1)
					if err != nil {
						t.Fatal(err)
					}
					if err = worker.EnqueueOwned(ctx, principal); err != nil {
						t.Fatal(err)
					}
					workers[s] = worker
					principals[s] = principal
					if mode == "measurement" {
						summary, err := worker.RebuildOwned(ctx, principal)
						if err != nil || summary.SnapshotID == "" {
							t.Fatal(summary, err)
						}
						snapshot, err := s.Active(ctx, realMarketCreator, realMarketToken)
						if err != nil {
							t.Fatal(err)
						}
						state := snapshot.State
						captures[s] = InventoryMeasurementCase{ID: "inventory-capture", Wallet: realMarketCreator, Token: realMarketToken, Height: snapshot.Height, BlockHash: snapshot.BlockHash, ExpectedState: &state}
					}
				}
				if mode == "interval" {
					snapshot := repair63LiveReplay(t, ctx, s, 33963749)
					summary, err := s.PublishRebuiltWallet(ctx, realMarketCreator, snapshot.Height, l17.Repository{Pool: s.authority.pool})
					if err != nil || summary.SnapshotID == "" {
						t.Fatal(summary, err)
					}
					ranges[s] = [2]time.Time{snapshot.ObservedThrough.Truncate(24 * time.Hour), snapshot.ObservedThrough.Truncate(time.Hour)}
				}
				if mode == "publish" {
					snapshot, err := s.Rebuild(ctx, realMarketCreator, realMarketToken, 33963733)
					if err != nil || snapshot.Height != 33963733 {
						t.Fatal(snapshot, err)
					}
				}
			}
			probe, trace := inventoryCancelableService(t)
			prepare(t, probe)
			trace.Start(0)
			err := invoke(t, probe)
			queries := trace.Stop()
			if err != nil || len(queries) == 0 {
				t.Fatal(queries, err)
			}
			candidate, cancellation := inventoryCancelableService(t)
			prepare(t, candidate)
			for i, query := range queries {
				// The live replay's caller owns rollback; cancellation of that cleanup
				// is outside the operation being verified.
				if strings.EqualFold(query, "rollback") {
					continue
				}
				t.Run(fmt.Sprintf("query_%03d", i+1), func(t *testing.T) {
					cancellation.Start(i + 1)
					err := invoke(t, candidate)
					observed := cancellation.Stop()
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("query %q: error=%v", query, err)
					}
					if len(observed) != i+1 || observed[i] != query {
						t.Fatalf("query sequence=%v want=%q", observed, query)
					}
					var count int
					wantSnapshots := 0
					if mode == "interval" || mode == "measurement" {
						wantSnapshots = 1
					}
					if err = candidate.authority.pool.QueryRow(ctx, `SELECT count(*) FROM pnl.portfolio_snapshots`).Scan(&count); err != nil || count != wantSnapshots {
						t.Fatal(count, err)
					}
				})
			}
		})
	}
}

func assertInventoryTxCancellations(t *testing.T, pool *pgxpool.Pool, operation func(pgx.Tx) error) {
	t.Helper()
	ctx := context.Background()
	config := pool.Config()
	trace := new(internaltest.QueryCancellation)
	config.ConnConfig.Tracer = trace
	traced, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer traced.Close()
	invoke := func(target int) ([]string, error) {
		tx, err := traced.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer inventoryRollback(tx)
		if _, err = tx.Exec(ctx, `SET LOCAL search_path TO pnl,pg_catalog`); err != nil {
			t.Fatal(err)
		}
		trace.Start(target)
		err = operation(tx)
		queries := trace.Stop()
		return queries, err
	}
	queries, err := invoke(0)
	if err != nil || len(queries) == 0 {
		t.Fatal(queries, err)
	}
	for i, query := range queries {
		t.Run(fmt.Sprintf("cancel_%03d", i+1), func(t *testing.T) {
			observed, err := invoke(i + 1)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("query %q: error=%v", query, err)
			}
			if len(observed) != i+1 || observed[i] != query {
				t.Fatal(observed, query)
			}
		})
	}
}
