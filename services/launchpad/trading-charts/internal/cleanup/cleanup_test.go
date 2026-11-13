package cleanup_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/cleanup"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/jackc/pgx/v5"
)

func TestCloseAndRollbackRealResources(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	cleanup.Close(file)
	if _, err := file.WriteString("after close"); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("file was not closed: %v", err)
	}
	cleanup.Close(file)
	if _, err := file.WriteString("after repeated close"); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("repeated close changed file state: %v", err)
	}
	dsn := internaltest.NewDatabase(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cleanup.Rollback(ctx, tx)
	if err := tx.Commit(ctx); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("transaction was not rolled back: %v", err)
	}
	cleanup.CloseContext(ctx, conn)
	if !conn.IsClosed() {
		t.Fatal("connection was not closed")
	}
	cleanup.CloseContext(ctx, conn)
	if !conn.IsClosed() {
		t.Fatal("repeated context close reopened connection")
	}
	other, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup.CloseContext(ctx, other)
	cancelledTx, err := other.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	cleanup.Rollback(cancelled, cancelledTx)
	if err := cancelledTx.Commit(ctx); err == nil {
		t.Fatal("rollback with canceled context left a committable transaction")
	}
}
