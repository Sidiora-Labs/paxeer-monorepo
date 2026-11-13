package checked

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/jackc/pgx/v5"
)

// Close reports a resource close error after the caller has finished using it.
func Close(resource io.Closer) {
	if err := resource.Close(); err != nil {
		log.Printf("indexer: resource close: %v", err)
	}
}

// CloseContext reports a context-aware resource close error.
func CloseContext(resource interface{ Close(context.Context) error }, ctx context.Context) {
	if err := resource.Close(ctx); err != nil {
		log.Printf("indexer: resource close: %v", err)
	}
}

// Rollback releases an unfinished transaction and reports cleanup errors.
func Rollback(tx pgx.Tx, ctx context.Context) {
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		log.Printf("indexer: transaction rollback: %v", err)
	}
}

// Write enforces the hash writer contract for deterministic payloads.
func Write(_ int, err error) {
	if err != nil {
		panic(fmt.Errorf("indexer: deterministic payload write: %w", err))
	}
}

// Report logs an asynchronous stream error discovered during cleanup.
func Report(err error) {
	if err != nil {
		log.Printf("indexer: stream cleanup: %v", err)
	}
}
