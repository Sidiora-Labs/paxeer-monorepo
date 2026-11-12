package voice

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

func rollbackTx(ctx context.Context, tx interface{ Rollback(context.Context) error }) {
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		slog.Error("social voice transaction rollback failed", "error", err)
	}
}
