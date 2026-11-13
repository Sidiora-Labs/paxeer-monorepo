// Package cleanup reports errors from deferred resource cleanup.
package cleanup

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

// Report records a cleanup error with its operation.
func Report(operation string, err error) {
	if err != nil {
		slog.Warn("gateway cleanup", "operation", operation, "error", err)
	}
}

// Close closes a resource and reports a close error.
func Close(resource io.Closer) {
	Report("close resource", resource.Close())
}

// Rollback releases an open transaction and reports unexpected rollback errors.
func Rollback(ctx context.Context, tx interface{ Rollback(context.Context) error }) {
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		Report("rollback transaction", err)
	}
}
