package projectionstats

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestCanonicalReadSnapshotBindsWatermarkAndCursor(t *testing.T) {
	ctx, pool, service, address := localReadFixture(t)
	filter := Filter{address, "", service.config.ProjectionVersion}
	position := fixture("buy", "7").Position
	id := uuid.New()
	cursor, err := service.encodeCompact(filter, id, position)
	if err != nil {
		t.Fatal(err)
	}
	watermark := CanonicalWatermark{ChainID: "125", SourceStream: service.config.SourceStream, Height: "10", BlockHash: "0x" + strings.Repeat("a", 64), ProjectionVersion: service.config.ProjectionVersion, ComputedAt: 200}
	write := func(w CanonicalWatermark) {
		t.Helper()
		raw, err := json.Marshal(w)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO stats_local.phase2_read_snapshots VALUES($1,$2,'',$3,$4,to_timestamp(1000)) ON CONFLICT(id) DO UPDATE SET watermark=excluded.watermark`, id, address, service.config.ProjectionVersion, raw); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.Trades(ctx, address, "", 1, cursor, 200); !errors.Is(err, ErrInvalid) {
		t.Fatalf("absent snapshot: %v", err)
	}
	write(watermark)
	page, err := service.Trades(ctx, address, "", 1, cursor, 200)
	if err != nil || len(page.Trades) != 0 || page.Watermark != watermark {
		t.Fatalf("snapshot page: %+v %v", page, err)
	}
	for _, tc := range []struct {
		name string
		edit func(*CanonicalWatermark)
	}{
		{"chain", func(w *CanonicalWatermark) { w.ChainID = "1" }},
		{"stream", func(w *CanonicalWatermark) { w.SourceStream = "other" }},
		{"projection", func(w *CanonicalWatermark) { w.ProjectionVersion = "other" }},
		{"block", func(w *CanonicalWatermark) { w.BlockHash = "invalid" }},
		{"height", func(w *CanonicalWatermark) { w.Height = "01" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := watermark
			tc.edit(&changed)
			write(changed)
			if _, err := service.Trades(ctx, address, "", 1, cursor, 200); !errors.Is(err, ErrIntegrity) {
				t.Fatalf("changed snapshot: %v", err)
			}
		})
	}
	write(watermark)
	if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_block_completeness SET fetch_complete=false`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Trades(ctx, address, "", 1, cursor, 200); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("snapshot source completeness: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE stats_local.phase2_read_snapshots SET expires_at=to_timestamp(100)`); err != nil {
		t.Fatal(err)
	}
	if n, err := service.PruneExpiredSnapshots(ctx, 10); err != nil || n != 1 {
		t.Fatalf("expired snapshot prune: %d %v", n, err)
	}
	for _, tc := range []struct {
		name, token string
		f           Filter
	}{
		{"length", strings.Repeat("a", 257), filter}, {"filter", cursor, Filter{address, "buy", filter.ProjectionVersion}},
		{"encoding", "%", filter}, {"short", base64.RawURLEncoding.EncodeToString([]byte("short")), filter},
		{"invalid filter", cursor, Filter{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := service.decodeCompact(tc.f, tc.token); !errors.Is(err, ErrInvalid) {
				t.Fatalf("compact cursor: %v", err)
			}
		})
	}
	bytes, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		t.Fatal(err)
	}
	bytes[30] ^= 1
	if _, _, err := service.decodeCompact(filter, base64.RawURLEncoding.EncodeToString(bytes)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("cursor authentication: %v", err)
	}
	if _, err := service.encodeCompact(Filter{}, id, position); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid compact filter: %v", err)
	}
	if _, err := service.LastWindow(ctx, "invalid", 60); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid last-window pool: %v", err)
	}
	pool.Close()
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"trades", func() error { _, e := service.Trades(ctx, address, "", 1, "", 200); return e }},
		{"window", func() error { _, e := service.Window(ctx, address, 60, 200); return e }},
		{"last window", func() error { _, e := service.LastWindow(ctx, address, 60); return e }},
		{"prune", func() error { _, e := service.PruneExpiredSnapshots(ctx, 10); return e }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil || !strings.Contains(err.Error(), "closed") {
				t.Fatalf("closed read service: %v", err)
			}
		})
	}
}

func TestCanonicalReadChecksCommittedTradePayload(t *testing.T) {
	for _, tc := range []struct{ name, query string }{
		{"trade JSON shape", `UPDATE stats.trade_facts SET payload=jsonb_set(payload,'{feeRaw}','1')`},
		{"trade content", `UPDATE stats.trade_facts SET payload=jsonb_set(payload,'{feeRaw}','"0"')`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPublicFixture(t)
			f.ingestReal()
			f.advance(33963761, f.real.Blocks["33963761"].Hash)
			service := f.readService()
			now := f.real.Blocks["33963749"].Timestamp + 60
			if _, err := f.pg.Exec(f.ctx, tc.query); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Trades(f.ctx, f.real.Pool, "", 10, "", now); !errors.Is(err, ErrIntegrity) {
				t.Fatalf("committed trade payload: %v", err)
			}
		})
	}
}

func TestProjectionRepositoryDatabaseErrors(t *testing.T) {
	ctx, pool, service, address := localReadFixture(t)
	repo := service.repository
	in := input(fixture("buy", "7"), 119)
	// Use a live transaction to assert the database's transaction state contract.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ApplyTx(ctx, tx, in); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("closed apply transaction: %v", err)
	}
	if _, err := repo.ApplyTx(ctx, nil, in); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing apply transaction: %v", err)
	}
	if _, err := repo.ApplyQualifiedTx(ctx, nil, QualifiedApplyInput{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing qualified transaction: %v", err)
	}
	if _, err := repo.ApplyQualifiedBatchTx(ctx, nil, nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing batch transaction: %v", err)
	}
	if _, err := repo.ApplyAuthorizedBatch(ctx, nil, nil, "c", "v"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing authority: %v", err)
	}
	if _, err := repo.ApplyAuthorized(ctx, nil, in.Trade, "c", "v"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing single authority: %v", err)
	}
	if _, err := NewRepository(nil, "stats"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing database: %v", err)
	}
	if err := repo.FreezePoolsTx(ctx, nil, []string{address}, ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing fence transaction: %v", err)
	}
	pool.Close()
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"apply", func() error { _, e := repo.Apply(ctx, in); return e }},
		{"legacy page", func() error { _, e := repo.Page(ctx, Filter{address, "", "projection-v1"}, 10, "100", ""); return e }},
		{"aggregate", func() error { _, e := repo.AggregateWindow(ctx, "projection-v1", "100", 200, 60); return e }},
		{"minute", func() error { _, _, e := repo.MinuteSnapshot(ctx, address, "projection-v1", 60, 18); return e }},
		{"accounting", func() error { _, _, e := repo.AccountingSnapshot(ctx, address, "projection-v1", 60); return e }},
		{"publisher", func() error {
			_, e := repo.PublishPending(ctx, 10, func(context.Context, Notification) error { return nil })
			return e
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil || !strings.Contains(err.Error(), "closed") {
				t.Fatalf("closed repository: %v", err)
			}
		})
	}
	if err := (ExpiryScheduler{Service: service, Windows: []int64{60}, Interval: time.Second}).Refresh(ctx, 200); err == nil {
		t.Fatal("closed expiry service refreshed")
	}
}
