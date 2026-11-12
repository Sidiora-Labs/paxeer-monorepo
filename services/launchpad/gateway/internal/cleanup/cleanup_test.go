package cleanup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestCloseClosesResource(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "resource"))
	if err != nil {
		t.Fatal(err)
	}
	Close(f)
	if _, err := f.WriteString("closed"); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("write after Close = %v, want closed file", err)
	}
	Close(f)
	if _, err := f.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("file state after repeated Close = %v, want closed file", err)
	}
}

func TestReportAcceptsCleanupOutcome(t *testing.T) {
	Report("closed resource", nil)
}

func TestRollbackReleasesPostgresTransaction(t *testing.T) {
	dsn := os.Getenv("L41_POSTGRES_URL")
	if dsn == "" {
		t.Fatal("L41_POSTGRES_URL is required for the gateway storage fixture")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	Rollback(ctx, tx)
	if _, err := tx.Exec(ctx, "SELECT 1"); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("query after rollback = %v, want closed transaction", err)
	}
	Rollback(ctx, tx)
	other, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	Rollback(canceled, other)
	if _, err := other.Exec(ctx, "SELECT 1"); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("query after canceled rollback = %v, want closed transaction", err)
	}
}
