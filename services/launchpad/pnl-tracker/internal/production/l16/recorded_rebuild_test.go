package l16

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5/pgxpool"
)

func recordedRebuildService(t *testing.T, ctx context.Context) (*Service, *pgxpool.Pool) {
	t.Helper()
	db, _ := repair63Database(t, ctx)
	pool, err := pgxpool.New(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	rpcURL := repair63RecordedRPC(t)
	primary, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(primary.Close)
	second, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(second.Close)
	guard, err := queue.NewEpochGuard("recovery", coordinatortest.Epoch, coordinatortest.Region)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewBaselineAuthorityWithEpochGuard(pool, primary, "pnl", realMarketStream, "pnl", "g1", "pnl-1", "inventory-v1", guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{RebuildSubject: true, "portfolio.snapshot.v2": true}, EventTypes: map[string]bool{RebuildSubject: true, "portfolio.snapshot.v2": true}}
	outbox, err := delivery.NewStoreWithEpochGuard(pool, "pnl", policy, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(authority, second, [2]string{"recorded-primary", "recorded-archive"}, strings.Repeat("c", 64), outbox)
	if err != nil {
		t.Fatal(err)
	}
	return service, pool
}

func TestRecordedInventoryOrdinaryReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	service, _ := recordedRebuildService(t, ctx)
	otherWallet := "0x" + strings.Repeat("e", 40)
	baseline, err := service.authority.Capture(ctx, otherWallet, realMarketToken)
	if err != nil || len(baseline) != 64 {
		t.Fatalf("zero balance baseline = %q, error = %v", baseline, err)
	}
	ordinary, err := service.Rebuild(ctx, realMarketCreator, realMarketToken, 33963733)
	if err != nil || ordinary.Height != 33963733 || ordinary.State.Known == "0" {
		t.Fatalf("ordinary inventory replay = %+v, error = %v", ordinary, err)
	}
	active, err := service.Active(ctx, realMarketCreator, realMarketToken)
	if err != nil || active.State != ordinary.State || active.Height != ordinary.Height {
		t.Fatalf("ordinary active inventory = %+v, error = %v", active, err)
	}
}

func TestRecordedInventoryRebuildAndPublication(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	service, pool := recordedRebuildService(t, ctx)
	first, err := service.RebuildArchive(ctx, realMarketCreator, realMarketToken, 33963733)
	if err != nil {
		t.Fatal(err)
	}
	if first.Height != 33963733 || first.State.Known == "0" {
		t.Fatalf("recorded buy inventory = %+v", first)
	}
	later := repair63LiveReplay(t, ctx, service, 33963749)
	if later.Height <= first.Height || later.State == first.State {
		t.Fatalf("recorded sale inventory = %+v, first = %+v", later, first)
	}
	active, err := service.Active(ctx, realMarketCreator, realMarketToken)
	if err != nil || active.Height != later.Height || active.State != later.State {
		t.Fatalf("active inventory = %+v, error = %v", active, err)
	}
	summary, err := service.PublishRebuiltWallet(ctx, realMarketCreator, later.Height, l17.Repository{Pool: pool})
	if err != nil || summary.SnapshotID == "" || summary.Wallet != realMarketCreator {
		t.Fatalf("published portfolio = %+v, error = %v", summary, err)
	}
	repeated, err := service.RebuildWallet(ctx, realMarketCreator, later.Height, l17.Repository{Pool: pool})
	if err != nil || repeated.SnapshotID != summary.SnapshotID {
		t.Fatalf("repeated portfolio = %+v, first = %+v, error = %v", repeated, summary, err)
	}
	if len(later.Checkpoints) == 0 {
		t.Fatal("recorded inventory lacks a day checkpoint")
	}
	day, err := time.Parse("2006-01-02", later.Checkpoints[0].Day)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := service.HistoricalDay(ctx, realMarketCreator, realMarketToken, day)
	if !errors.Is(err, ErrBaselineUnavailable) || checkpoint.Day != "" {
		t.Fatalf("incomplete historical day = %+v, error = %v", checkpoint, err)
	}
	end := later.ObservedThrough.Truncate(time.Hour)
	if !day.Before(end) {
		t.Fatalf("recorded market has no complete hour: %s to %s", day, end)
	}
	series, err := service.IntervalHistory(ctx, realMarketCreator, day, end, "1h", 24)
	if err != nil || series.SnapshotID != summary.SnapshotID || len(series.Entries) != int(end.Sub(day)/time.Hour) {
		t.Fatalf("hourly inventory = %+v, error = %v", series, err)
	}
}

func TestRecordedInventoryRejectsUnqualifiedRequests(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	service, pool := recordedRebuildService(t, ctx)
	for _, tc := range []struct {
		wallet, token string
		head          int64
	}{
		{"invalid", realMarketToken, 33963733}, {realMarketCreator, "invalid", 33963733}, {zeroAddress, realMarketToken, 33963733}, {realMarketCreator, realMarketToken, 0}, {realMarketCreator, realMarketToken, 1}, {realMarketCreator, realMarketToken, 33983733},
	} {
		if _, err := service.Rebuild(ctx, tc.wallet, tc.token, tc.head); !errors.Is(err, ErrBaselineUnavailable) {
			t.Fatal(tc, err)
		}
	}
	missing := "0x" + strings.Repeat("e", 40)
	if _, err := service.PublishRebuiltWallet(ctx, missing, 33963733, l17.Repository{Pool: pool}); !errors.Is(err, ErrBaselineUnavailable) {
		t.Fatal(err)
	}
	if _, err := service.PublishRebuiltWallet(ctx, realMarketCreator, 0, l17.Repository{Pool: pool}); !errors.Is(err, ErrBaselineUnavailable) {
		t.Fatal(err)
	}
	if _, err := service.RebuildWallet(ctx, zeroAddress, 1, l17.Repository{Pool: pool}); !errors.Is(err, ErrBaselineUnavailable) {
		t.Fatal(err)
	}
	if _, err := service.Active(ctx, "invalid", realMarketToken); !errors.Is(err, ErrBaselineUnavailable) {
		t.Fatal(err)
	}
	if _, err := service.HistoricalDay(ctx, "invalid", realMarketToken, time.Now()); !errors.Is(err, ErrBaselineUnavailable) {
		t.Fatal(err)
	}
	canceled, done := context.WithCancel(ctx)
	done()
	for name, call := range map[string]func() error{
		"rebuild": func() error {
			_, err := service.Rebuild(canceled, realMarketCreator, realMarketToken, 33963733)
			return err
		},
		"publish": func() error {
			_, err := service.PublishRebuiltWallet(canceled, realMarketCreator, 33963733, l17.Repository{Pool: pool})
			return err
		},
		"wallet": func() error {
			_, err := service.RebuildWallet(canceled, realMarketCreator, 33963733, l17.Repository{Pool: pool})
			return err
		},
		"capture": func() error {
			_, err := service.authority.Capture(canceled, realMarketCreator, realMarketToken)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
}
