package l17

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/jackc/pgx/v5/pgxpool"
)

func portfolioCancellationMatrix(t *testing.T, prepare func(*testing.T, Repository) func() error) {
	t.Helper()
	ctx := context.Background()
	fixture := func(t *testing.T) (Repository, *internaltest.QueryCancellation) {
		source := phase2Repository(t)
		config := source.Pool.Config()
		trace := new(internaltest.QueryCancellation)
		config.ConnConfig.Tracer = trace
		pool, err := pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		return Repository{Pool: pool}, trace
	}
	counts := func(t *testing.T, r Repository) []int {
		out := make([]int, 5)
		for i, table := range []string{"portfolio_snapshots", "portfolio_history_snapshots", "portfolio_cards", "portfolio_export_jobs", "portfolio_outbox"} {
			if err := r.Pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&out[i]); err != nil {
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
			repository, cancellation := fixture(t)
			operation := prepare(t, repository)
			before := counts(t, repository)
			cancellation.Start(i + 1)
			err := operation()
			observed := cancellation.Stop()
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("query %q: error=%v", query, err)
			}
			if len(observed) != i+1 || observed[i] != query {
				t.Fatal(observed, query)
			}
			if after := counts(t, repository); !reflect.DeepEqual(before, after) {
				t.Fatal(before, after)
			}
		})
	}
}

func portfolioHistoryRecord(summary Summary) HistoryRecord {
	six := 6
	return HistoryRecord{EventID: "cancellation-deposit", Kind: "deposit", Asset: AssetRef{Kind: "erc20", ChainID: ptr("125"), Address: ptr(token), Decimals: &six}, AmountRaw: "1000000", BasisQuality: "unknown", Position: ChainPosition{ChainID: "125", Height: "99", BlockHash: summary.Watermark.BlockHash, TransactionHash: "0x" + strings.Repeat("b", 64), TransactionIndex: "0", LogIndex: "0"}}
}

func TestPortfolioRepositoryCancellationIsAtomic(t *testing.T) {
	ctx := context.Background()
	for _, action := range []string{"snapshot", "snapshot replay", "page", "latest", "history commit", "history replay", "history page", "history series", "metadata", "share", "card create", "card render", "card worker", "card owner", "card revoke", "card public", "card purge", "card pending", "export create", "export worker", "export open", "export cleanup", "export status", "export cancel", "export history", "export positions", "pending", "ack"} {
		t.Run(action, func(t *testing.T) {
			portfolioCancellationMatrix(t, func(t *testing.T, r Repository) func() error {
				summary := phase2Commit(t, r, "cancellation-source", "1", 2)
				record := portfolioHistoryRecord(summary)
				now := time.Now().UTC()
				if action != "history commit" {
					if err := r.CommitHistory(ctx, wallet, summary.SnapshotID, "cancellation-history", []HistoryRecord{record}); err != nil {
						t.Fatal(err)
					}
				}
				var card StoredCard
				var job ExportJob
				var err error
				if strings.HasPrefix(action, "card ") && action != "card create" {
					card, err = r.CreateCard(ctx, wallet, wallet, summary.SnapshotID, "cancellation-card", false, now)
					if err != nil {
						t.Fatal(err)
					}
					if action == "card public" {
						if err = r.RenderCard(ctx, card.ID); err != nil {
							t.Fatal(err)
						}
					}
					if action == "card purge" {
						if err = r.RevokeCard(ctx, card.ID, wallet, wallet, now); err != nil {
							t.Fatal(err)
						}
					}
				}
				if action == "export status" || action == "export cancel" || action == "export worker" || action == "export open" || action == "export cleanup" {
					job, err = r.CreateExportJob(ctx, wallet, wallet, summary.SnapshotID, "jsonl", "cancellation-export")
					if err != nil {
						t.Fatal(err)
					}
					if action == "export cleanup" {
						if err = r.CancelExport(ctx, job.ID, wallet, wallet); err != nil {
							t.Fatal(err)
						}
					}
				}
				directory := t.TempDir()
				if err = os.Chmod(directory, 0700); err != nil {
					t.Fatal(err)
				}
				if action == "export open" {
					if err = (ExportWorker{Repository: r, Owner: "cancellation-export-worker", Directory: directory}).RunOnce(ctx); err != nil {
						t.Fatal(err)
					}
				}
				return func() error {
					switch action {
					case "snapshot":
						positions := phase2Positions(2, "2")
						result, err := r.CommitSnapshot(ctx, "cancellation-second", Summary{Wallet: wallet, Watermark: positions[0].SourceWatermark}, positions, now.Add(time.Hour))
						if err == nil && (result.SnapshotID == "" || result.Watermark.ProjectionRevision != "2") {
							t.Fatal(result)
						}
						return err
					case "snapshot replay":
						result, err := r.CommitSnapshot(ctx, "cancellation-source", Summary{Wallet: wallet, Watermark: phase2Positions(2, "1")[0].SourceWatermark}, phase2Positions(2, "1"), now.Add(time.Hour))
						if err == nil && result.SnapshotID != summary.SnapshotID {
							t.Fatal(result)
						}
						return err
					case "page":
						page, err := r.Page(ctx, wallet, summary.SnapshotID, -1, 2, now)
						if err == nil && (len(page.Positions) != 2 || page.Summary.SnapshotID != summary.SnapshotID) {
							t.Fatal(page)
						}
						return err
					case "latest":
						id, revision, _, err := r.Latest(ctx, wallet, now)
						if err == nil && (id != summary.SnapshotID || revision != "1") {
							t.Fatal(id, revision)
						}
						return err
					case "history commit", "history replay":
						return r.CommitHistory(ctx, wallet, summary.SnapshotID, "cancellation-history", []HistoryRecord{record})
					case "history page":
						page, err := r.HistoryPage(ctx, wallet, wallet, summary.SnapshotID, -1, 10, now)
						if err == nil && len(page.Records) != 1 {
							t.Fatal(page)
						}
						return err
					case "history series":
						observed, err := time.Parse(time.RFC3339Nano, summary.Watermark.ObservedAt)
						if err != nil {
							t.Fatal(err)
						}
						entries, last, err := r.HistorySeries(ctx, wallet, wallet, observed.Add(-time.Hour), observed.Add(time.Hour), "1h", 10)
						if err == nil && (len(entries) == 0 || last == nil) {
							t.Fatal(entries, last)
						}
						return err
					case "metadata":
						tx, err := r.Pool.Begin(ctx)
						if err != nil {
							return err
						}
						defer rollbackPortfolio(tx)
						return r.CommitHistoryMetadataTx(ctx, tx, wallet, summary.SnapshotID, SnapshotHistoryMetadata{SourceEvent: "cancellation-metadata", Start: now.Add(-time.Hour), End: now, Quote: fixture().MarkPrice.QuoteAsset})
					case "share":
						share, err := r.Share(ctx, wallet, wallet, summary.SnapshotID, true, now)
						if err == nil && (share.Wallet == nil || *share.Wallet != wallet) {
							t.Fatal(share)
						}
						return err
					case "card create":
						card, err := r.CreateCard(ctx, wallet, wallet, summary.SnapshotID, "cancellation-card", false, now)
						if err == nil && card.State != "pending" {
							t.Fatal(card)
						}
						return err
					case "card render":
						return r.RenderCard(ctx, card.ID)
					case "card worker":
						return (CardWorker{Repository: r, Owner: "cancellation-card-worker", FontDir: filepath.Join("..", "..", "ogrender", "fonts")}).RunOnce(ctx)
					case "card owner":
						result, err := r.OwnerCardV2(ctx, card.ID, wallet, wallet, now)
						if err == nil && result.ID != card.ID {
							t.Fatal(result)
						}
						return err
					case "card revoke":
						return r.RevokeCard(ctx, card.ID, wallet, wallet, now)
					case "card purge":
						return r.AckCardPurge(ctx, card.ID, now)
					case "card public":
						result, err := r.PublicCard(ctx, card.ID)
						if err == nil && len(result.Image) == 0 {
							t.Fatal(result)
						}
						return err
					case "card pending":
						effects, err := r.PendingCardEffects(ctx, 10)
						if err == nil && len(effects) != 1 {
							t.Fatal(effects)
						}
						return err
					case "export create":
						result, err := r.CreateExportJob(ctx, wallet, wallet, summary.SnapshotID, "jsonl", "cancellation-export")
						if err == nil && result.State != "queued" {
							t.Fatal(result)
						}
						return err
					case "export open":
						file, manifest, err := r.OpenExport(ctx, directory, job.ID, wallet, wallet)
						if err == nil {
							if manifest.Rows != 1 {
								t.Fatal(manifest)
							}
							return file.Close()
						}
						return err
					case "export worker":
						return (ExportWorker{Repository: r, Owner: "cancellation-export-worker", Directory: directory}).RunOnce(ctx)
					case "export cleanup":
						return (ExportWorker{Repository: r, Owner: "cancellation-export-worker", Directory: directory}).CleanupOnce(ctx)
					case "export status":
						result, err := r.ExportJob(ctx, job.ID, wallet, wallet)
						if err == nil && result.ID != job.ID {
							t.Fatal(result)
						}
						return err
					case "export cancel":
						return r.CancelExport(ctx, job.ID, wallet, wallet)
					case "export history":
						manifest, err := r.ExportHistory(ctx, directory, wallet, wallet, summary.SnapshotID, "jsonl", now)
						if err == nil && manifest.Rows != 1 {
							t.Fatal(manifest)
						}
						return err
					case "export positions":
						manifest, err := r.ExportJSON(ctx, directory, wallet, wallet, summary.SnapshotID, now)
						if err == nil && manifest.Rows != 2 {
							t.Fatal(manifest)
						}
						return err
					case "pending":
						events, err := r.Pending(ctx, 10)
						if err == nil && len(events) != 1 {
							t.Fatal(events)
						}
						return err
					default:
						return r.Ack(ctx, summary.SnapshotID)
					}
				}
			})
		})
	}
}
