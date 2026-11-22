package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestDevelopmentEnvelopeRequiresBoundedSourcePosition(t *testing.T) {
	raw := `{"eventName":"Swap","blockNumber":1,"transactionIndex":0,"logIndex":0,"args":{"blockTimestamp":1704067260}}`
	swap, err := DecodeDevelopmentSwap([]byte(raw))
	if err != nil || swap.BlockTimestamp != 1704067260 || swap.Position == nil || swap.Position.TransactionIndex != 0 {
		t.Fatalf("authenticated argument timestamp: %+v, %v", swap, err)
	}
	for _, tc := range []struct{ raw, want string }{
		{raw + ` {}`, "trailing swap envelope"},
		{strings.Replace(raw, `"transactionIndex":0`, `"transactionIndex":4294967296`, 1), "source transaction index out of range"},
	} {
		if _, err := DecodeDevelopmentSwap([]byte(tc.raw)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("unbounded source envelope: %v", err)
		}
	}
	if got := asInt64(json.Number("9223372036854775808")); got != 0 {
		t.Fatalf("overflowing timestamp: %d", got)
	}
}

func TestCompletenessRequiresAuthorityAndHonorsCancellation(t *testing.T) {
	ctx := context.Background()
	if c, err := NewAccountingConsumer(AccountingConfig{}); c != nil || err == nil {
		t.Fatalf("incomplete accounting authority: %+v, %v", c, err)
	}
	for _, c := range []*AccountingConsumer{nil, {}} {
		if err := c.RefreshCompleteness(ctx, accounting.PublicationV1{}); !errors.Is(err, ErrAccountingAuthority) {
			t.Fatalf("unbound completeness refresh: %v", err)
		}
		if err := c.RunCompleteness(ctx, nil); !errors.Is(err, ErrAccountingAuthority) {
			t.Fatalf("unbound completeness loop: %v", err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	c := &AccountingConsumer{}
	if err := c.runCompleteness(canceled, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled completeness loop: %v", err)
	}
}

func TestPostgresCompletenessRejectsMissingAppliedAuthority(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	ddl, err := store.ExactSchemaDDL("candles")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	exact, err := store.NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	config := SourceVerifiedAccountingConfig{Pool: pool, Exact: exact}
	persistence := postgresCompletenessStore{config: config}
	if _, err := persistence.Publications(ctx, "", 1); err == nil {
		t.Fatal("missing publication authority accepted")
	}
	fill := accountingFixture()
	publication := accounting.PublicationV1{Fill: &fill}
	tx, err := persistence.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Applied(ctx, publication); err == nil {
		t.Fatal("missing pool fence accepted")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO candles.chart_pool_fences VALUES('125',$1,1,false)`, fill.PoolAddress); err != nil {
		t.Fatal(err)
	}
	tx, err = persistence.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Applied(ctx, publication); !errors.Is(err, ErrAccountingAuthority) {
		t.Fatalf("unapplied publication: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	sqltx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	authority := sourceAuthority{reader: postgresSourceAuthority{tx: sqltx, config: config}}
	if _, err := authority.watermark(ctx); err == nil {
		t.Fatal("missing source watermark authority accepted")
	}
	if err := sqltx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := (sourceAuthority{}).authorizeFill(ctx, accounting.FillV1{}); !errors.Is(err, ErrAccountingAuthority) {
		t.Fatalf("invalid source fill: %v", err)
	}
	if err := (sourceAuthority{}).authorizePublication(ctx, accounting.PublicationV1{}); err == nil {
		t.Fatal("invalid source publication accepted")
	}
	pool.Close()
	if _, err := persistence.Begin(ctx); err == nil {
		t.Fatal("closed completeness store accepted")
	}
}
