package checked

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestCloseAndRollbackRealResources(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("INDEXER_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Fatal("INDEXER_TEST_POSTGRES_URL is required")
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		CloseContext(conn, ctx)
		t.Fatal(err)
	}
	Rollback(tx, ctx)
	if err := tx.Rollback(ctx); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("transaction remains open: %v", err)
	}
	CloseContext(conn, ctx)
	if !conn.IsClosed() {
		t.Fatal("database connection remains open")
	}

	file, err := os.CreateTemp(t.TempDir(), "closed-file")
	if err != nil {
		t.Fatal(err)
	}
	Close(file)
	if _, err := file.Write([]byte("closed")); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("file remains open: %v", err)
	}
}

func TestWriteReportsDigestFailure(t *testing.T) {
	Write(3, nil)
	want := errors.New("digest write failed")
	defer func() {
		got := recover()
		if got == nil {
			t.Fatal("write error was accepted")
		}
		failure, ok := got.(error)
		if !ok || !errors.Is(failure, want) {
			t.Fatalf("panic %v does not retain write error", got)
		}
	}()
	Write(0, want)
}

func TestCleanupReportsActualClosedResources(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)
	file, err := os.CreateTemp(t.TempDir(), "resource")
	if err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	Close(file)
	if !strings.Contains(output.String(), "resource close") {
		t.Fatalf("close diagnostic %q", output.String())
	}
	output.Reset()
	Report(nil)
	if output.Len() != 0 {
		t.Fatal("successful cleanup emitted a diagnostic")
	}
	_, err = file.Read(make([]byte, 1))
	if !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
	Report(err)
	if !strings.Contains(output.String(), "stream cleanup") || !strings.Contains(output.String(), "file already closed") {
		t.Fatalf("cleanup diagnostic %q", output.String())
	}
	output.Reset()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("INDEXER_TEST_POSTGRES_URL"))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = conn.Close(ctx); err != nil {
		t.Fatal(err)
	}
	Rollback(tx, ctx)
	if !strings.Contains(output.String(), "transaction rollback") {
		t.Fatalf("rollback diagnostic %q", output.String())
	}
}
